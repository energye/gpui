package h264

import "fmt"

// P-slice macroblock decoding: skip, inter partitions, intra fallback.

// part is one motion-compensated rectangle in luma pixels. B partitions
// name which lists they predict from (use0/use1); both means bipred with
// the slice's bipred weights.
type part struct {
	px, py, w, h int
	mx, my       int16
	ref          int8
	mx1, my1     int16
	ref1         int8
	use0, use1   bool
}

func (d *Decoder) decodeSkip(h *SliceHeader, addr int, rs residSrc) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	if d.refPic == nil {
		return fmt.Errorf("%w: skip without reference", ErrBadSliceHeader)
	}
	mx, my := d.predSkipP(mbx, mby)
	d.storeMV(mbx*16, mby*16, 16, 16, mx, my, 0, 0, 0)
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	parts := [1]part{{mbx * 16, mby * 16, 16, 16, mx, my, 0, 0, 0, 0, true, false}}
	if err := d.mcParts(mbx, mby, parts[:], &predY, &predCb, &predCr); err != nil {
		return err
	}
	d.finishPMB(h, addr, mbx, mby)
	d.skipped[addr] = true
	d.mbSlice[addr] = d.slices
	if addr >= 0 && addr < len(d.mbT8) {
		d.mbT8[addr] = false
	}
	// S1b-E: one 16x16 partition means all 16 4x4 slots share (mx,my)
	// (storeMV above) — record it for the deblocker's uniform-edge
	// shortcut (cbp==0 below zeroes nnz, so clean by construction).
	if addr >= 0 && addr < len(d.uniSkip) {
		d.uniSkip[addr] = true
		d.uniDM[addr] = [2]bDirectMV{{ref: 0, mx: mx, my: my, use: true}, {}}
	}
	if err := d.reconstructInterWith(mbx, mby, 0, &predY, &predCb, &predCr, rs); err != nil {
		return err
	}
	d.skipCnt++
	return nil
}

// refFor resolves one reference index against list 0.
func (d *Decoder) refFor(idx int8) (*Picture, error) {
	if len(d.refList) == 0 {
		if d.refPic != nil && idx == 0 {
			return d.refPic, nil
		}
		return nil, fmt.Errorf("%w: ref %d without list", ErrBadSliceHeader, idx)
	}
	if int(idx) < 0 || int(idx) >= len(d.refList) || d.refList[idx] == nil {
		return nil, fmt.Errorf("%w: ref idx %d", ErrBadSliceHeader, idx)
	}
	return d.refList[idx], nil
}

// refFor1 resolves one reference index against list 1 (B slices).
func (d *Decoder) refFor1(idx int8) (*Picture, error) {
	if int(idx) < 0 || int(idx) >= len(d.refList1) || d.refList1[idx] == nil {
		return nil, fmt.Errorf("%w: ref l1 idx %d", ErrBadSliceHeader, idx)
	}
	return d.refList1[idx], nil
}

// bipredWeight returns the implicit list-0 weight (0..64, denom 5) for one
// B partition from display-order distances (spec 8.4.2.3.2): the nearer in
// time, the heavier. Equal split when the lists share a picture or either
// distance is zero.
func (d *Decoder) bipredWeight(poc, p0, p1 *Picture, i0, i1 int8) int32 {
	if poc == nil || p0 == nil || p1 == nil {
		return 32
	}
	td := clipInt32(int32(p1.POC)-int32(p0.POC), -128, 127)
	if td == 0 {
		return 32
	}
	tb := clipInt32(int32(poc.POC)-int32(p0.POC), -128, 127)
	tx := (16384 + (abs32(td) >> 1)) / td
	w := 64 - ((tb*tx + 32) >> 8)
	if w < -64 || w > 128 {
		return 32
	}
	_ = i0
	_ = i1
	return w
}

func clipInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// fillPred resets MB prediction buffers to mid-grey.
//
// S1-F: array assignment from pre-greyed tables (one runtime memmove
// per buffer, SIMD in runtime) instead of 384 per-byte stores with
// loop overhead. Same 128 bytes bit for bit (prefix parity pins
// output); pure Go so amd64/arm64/fallback share one path.
var predGreyY [256]uint8
var predGreyC [64]uint8

func init() {
	for i := range predGreyY {
		predGreyY[i] = 128
	}
	for i := range predGreyC {
		predGreyC[i] = 128
	}
}

func fillPred(predY *[256]uint8, predCb, predCr *[64]uint8) {
	*predY = predGreyY
	*predCb = predGreyC
	*predCr = predGreyC
}

// mcParts runs motion compensation for every partition into MB-sized
// prediction buffers.
func (d *Decoder) mcParts(mbx, mby int, parts []part, predY *[256]uint8, predCb, predCr *[64]uint8) error {
	if len(parts) == 0 {
		return fmt.Errorf("%w: no partitions", ErrBadSliceHeader)
	}
	rw, rh := int(d.pic.Width/2), int(d.pic.Height/2)
	// S1b-W2: only list-1 temps remain (list 0 writes straight into
	// predY/predCb/predCr); blk/cb/cr intermediates are gone.
	var blk1 [256]uint8
	var cb1, cr1 [64]uint8
	for _, pt := range parts {
		if pt.use0 && !pt.use1 {
			rp, err := d.refFor(pt.ref)
			if err != nil {
				return err
			}
			ox0 := pt.px - mbx*16
			oy0 := pt.py - mby*16
			// S1-W direct-write: interpolate straight into predY at
			// its final address (no blk intermediate, no row copy).
			// Integer, arch sub-pel and scalar edge paths all stride
			// out; bit-identical to the old dense predict+copy (the
			// S1-W gate pins it). ox0+w <= 16 holds (partitions sit
			// inside the MB), so the subslice never overruns predY.
			predictLumaBlockInto(rp, pt.px, pt.py, pt.w, pt.h, pt.mx, pt.my, predY[oy0*16+ox0:], 16)
			d.weightLumaRect(predY, ox0, oy0, pt.w, pt.h, pt.ref, false)
		} else if pt.use0 && pt.use1 {
			rp0, err := d.refFor(pt.ref)
			if err != nil {
				return err
			}
			rp1, err := d.refFor1(pt.ref1)
			if err != nil {
				return err
			}
			// S1b-W2 dual direct-write: list 0 straight into predY
			// at its final address, list 1 into one temp, then mix
			// in place (no second temp, no row copy). Same bytes as
			// the old dense predict+mix+copy (S1-W gate pins it).
			ox0 := pt.px - mbx*16
			oy0 := pt.py - mby*16
			predictLumaBlockInto(rp0, pt.px, pt.py, pt.w, pt.h, pt.mx, pt.my, predY[oy0*16+ox0:], 16)
			predictLumaBlock(rp1, pt.px, pt.py, pt.w, pt.h, pt.mx1, pt.my1, blk1[:pt.w*pt.h])
			if d.wBiExpl {
				for y := 0; y < pt.h; y++ {
					for x := 0; x < pt.w; x++ {
						di := (oy0+y)*16 + ox0 + x
						si := y*pt.w + x
						predY[di] = d.bipredExplicitLuma(int32(predY[di]), int32(blk1[si]), pt.ref, pt.ref1)
					}
				}
			} else {
				w := d.bipredWeight(d.pic, rp0, rp1, pt.ref, pt.ref1)
				for y := 0; y < pt.h; y++ {
					bipredAvg(predY[(oy0+y)*16+ox0:(oy0+y)*16+ox0+pt.w], blk1[y*pt.w:(y+1)*pt.w], w)
				}
			}
		} else if pt.use1 {
			rp, err := d.refFor1(pt.ref1)
			if err != nil {
				return err
			}
			ox0 := pt.px - mbx*16
			oy0 := pt.py - mby*16
			// S1-W direct-write, list 1 (same shape as list 0).
			predictLumaBlockInto(rp, pt.px, pt.py, pt.w, pt.h, pt.mx1, pt.my1, predY[oy0*16+ox0:], 16)
			d.weightLumaRect(predY, ox0, oy0, pt.w, pt.h, pt.ref1, true)
		} else {
			return fmt.Errorf("%w: partition uses no list", ErrBadSliceHeader)
		}
		cw, ch := pt.w/2, pt.h/2
		cx, cy := (pt.px-mbx*16)/2, (pt.py-mby*16)/2
		// Chroma origin in the subsampled plane.
		ccx, ccy := mbx*8+cx, mby*8+cy
		if !pt.use0 && pt.use1 {
			rp, err := d.refFor1(pt.ref1)
			if err != nil {
				return err
			}
			// S1-W chroma direct-write (same shape as luma: straight
			// into predCb/predCr, no cb/cr intermediates).
			predictChromaBlockInto(rp.Cb, rw, rh, ccx, ccy, cw, ch, pt.mx1, pt.my1, predCb[cy*8+cx:], 8)
			predictChromaBlockInto(rp.Cr, rw, rh, ccx, ccy, cw, ch, pt.mx1, pt.my1, predCr[cy*8+cx:], 8)
			d.weightChromaRect(predCb, predCr, cx, cy, cw, ch, pt.ref1, true)
		} else if pt.use0 && !pt.use1 {
			rp, err := d.refFor(pt.ref)
			if err != nil {
				return err
			}
			predictChromaBlockInto(rp.Cb, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my, predCb[cy*8+cx:], 8)
			predictChromaBlockInto(rp.Cr, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my, predCr[cy*8+cx:], 8)
			d.weightChromaRect(predCb, predCr, cx, cy, cw, ch, pt.ref, false)
		} else {
			rp0, err := d.refFor(pt.ref)
			if err != nil {
				return err
			}
			rp1, err := d.refFor1(pt.ref1)
			if err != nil {
				return err
			}
			// S1b-W2 dual direct-write, chroma (same shape as luma:
			// list 0 straight into predCb/predCr, list 1 into one
			// temp pair, then per-row mix in place).
			predictChromaBlockInto(rp0.Cb, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my, predCb[cy*8+cx:], 8)
			predictChromaBlockInto(rp0.Cr, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my, predCr[cy*8+cx:], 8)
			predictChromaBlock(rp1.Cb, rw, rh, ccx, ccy, cw, ch, pt.mx1, pt.my1, cb1[:cw*ch])
			predictChromaBlock(rp1.Cr, rw, rh, ccx, ccy, cw, ch, pt.mx1, pt.my1, cr1[:cw*ch])
			if d.wBiExpl {
				for y := 0; y < ch; y++ {
					for x := 0; x < cw; x++ {
						di := (cy+y)*8 + cx + x
						si := y*cw + x
						predCb[di] = d.bipredExplicitChroma(int32(predCb[di]), int32(cb1[si]), pt.ref, pt.ref1, 0)
						predCr[di] = d.bipredExplicitChroma(int32(predCr[di]), int32(cr1[si]), pt.ref, pt.ref1, 1)
					}
				}
			} else {
				w := d.bipredWeight(d.pic, rp0, rp1, pt.ref, pt.ref1)
				for y := 0; y < ch; y++ {
					bipredAvg(predCb[(cy+y)*8+cx:(cy+y)*8+cx+cw], cb1[y*cw:(y+1)*cw], w)
					bipredAvg(predCr[(cy+y)*8+cx:(cy+y)*8+cx+cw], cr1[y*cw:(y+1)*cw], w)
				}
			}
		}
	}
	return nil
}

// weightLumaRect scales the w*h rect at (ox,oy) inside a stride-16
// prediction plane in place (same math as weightLuma, strided
// addressing). Used by the direct-write integer path: the block
// already sits at its final address, so weighting runs there instead
// of on a dense intermediate.
func (d *Decoder) weightLumaRect(pred *[256]uint8, ox, oy, w, h int, ref int8, list1 bool) {
	var ws []int32
	var os []int32
	var denom int32
	if !list1 {
		if int(ref) < 0 || int(ref) >= len(d.wW0) {
			return
		}
		ws, os = d.wW0[:], d.wO0[:]
		denom = d.wDenomL
	} else {
		if int(ref) < 0 || int(ref) >= len(d.wW1) {
			return
		}
		ws, os = d.wW1[:], d.wO1[:]
		denom = d.wDenomL
	}
	wgt, off := ws[ref], os[ref]
	if wgt == 1<<uint(denom) && off == 0 {
		return
	}
	round := int32(0)
	if denom > 0 {
		round = 1 << uint(denom-1)
	}
	for y := 0; y < h; y++ {
		base := (oy+y)*16 + ox
		for x := 0; x < w; x++ {
			i := base + x
			pred[i] = clipPixel((wgt*int32(pred[i])+round)>>uint(denom) + off)
		}
	}
}

// weightChromaRect scales the cw*ch rects at (cx,cy) inside the
// stride-8 chroma prediction planes in place (same math as
// weightChroma/weightChroma1, strided addressing).
func (d *Decoder) weightChromaRect(predCb, predCr *[64]uint8, cx, cy, cw, ch int, ref int8, list1 bool) {
	for comp := 0; comp < 2; comp++ {
		var wgt, off int32
		var denom int32
		if !list1 {
			if int(ref) < 0 || int(ref) >= len(d.wWC0) {
				return
			}
			wgt, off = d.wWC0[ref][comp], d.wOC0[ref][comp]
			denom = d.wDenomC
		} else {
			if int(ref) < 0 || int(ref) >= len(d.wWC1) {
				return
			}
			wgt, off = d.wWC1[ref][comp], d.wOC1[ref][comp]
			denom = d.wDenomC
		}
		if wgt == 1<<uint(denom) && off == 0 {
			continue
		}
		round := int32(0)
		if denom > 0 {
			round = 1 << uint(denom-1)
		}
		var plane *[64]uint8
		if comp == 0 {
			plane = predCb
		} else {
			plane = predCr
		}
		for y := 0; y < ch; y++ {
			base := (cy+y)*8 + cx
			for x := 0; x < cw; x++ {
				i := base + x
				plane[i] = clipPixel((wgt*int32(plane[i])+round)>>uint(denom) + off)
			}
		}
	}
}

// weightLuma scales one predicted luma block by the slice's explicit
// list-0 factors; default tables pass through untouched.
func (d *Decoder) weightLuma(blk []byte, ref int8) {
	if int(ref) < 0 || int(ref) >= len(d.wW0) {
		return
	}
	w, o := d.wW0[ref], d.wO0[ref]
	if w == 1<<uint(d.wDenomL) && o == 0 {
		return
	}
	round := int32(0)
	if d.wDenomL > 0 {
		round = 1 << uint(d.wDenomL-1)
	}
	for i, p := range blk {
		blk[i] = clipPixel((w*int32(p)+round)>>uint(d.wDenomL) + o)
	}
}

// weightChroma scales one predicted chroma block pair the same way.
func (d *Decoder) weightChroma(cb, cr []byte, ref int8) {
	if int(ref) < 0 || int(ref) >= len(d.wWC0) {
		return
	}
	for comp, dst := range [][]byte{cb, cr} {
		w, o := d.wWC0[ref][comp], d.wOC0[ref][comp]
		if w == 1<<uint(d.wDenomC) && o == 0 {
			continue
		}
		round := int32(0)
		if d.wDenomC > 0 {
			round = 1 << uint(d.wDenomC-1)
		}
		for i, p := range dst {
			dst[i] = clipPixel((w*int32(p)+round)>>uint(d.wDenomC) + o)
		}
	}
}

// weightLuma1 scales one predicted luma block by the slice's explicit
// list-1 factors; default tables pass through untouched.
func (d *Decoder) weightLuma1(blk []byte, ref int8) {
	if int(ref) < 0 || int(ref) >= len(d.wW1) {
		return
	}
	w, o := d.wW1[ref], d.wO1[ref]
	if w == 1<<uint(d.wDenomL) && o == 0 {
		return
	}
	round := int32(0)
	if d.wDenomL > 0 {
		round = 1 << uint(d.wDenomL-1)
	}
	for i, p := range blk {
		blk[i] = clipPixel((w*int32(p)+round)>>uint(d.wDenomL) + o)
	}
}

// weightChroma1 scales one predicted chroma block pair the same way.
func (d *Decoder) weightChroma1(cb, cr []byte, ref int8) {
	if int(ref) < 0 || int(ref) >= len(d.wWC1) {
		return
	}
	for comp, dst := range [][]byte{cb, cr} {
		w, o := d.wWC1[ref][comp], d.wOC1[ref][comp]
		if w == 1<<uint(d.wDenomC) && o == 0 {
			continue
		}
		round := int32(0)
		if d.wDenomC > 0 {
			round = 1 << uint(d.wDenomC-1)
		}
		for i, p := range dst {
			dst[i] = clipPixel((w*int32(p)+round)>>uint(d.wDenomC) + o)
		}
	}
}

// bipredExplicitLuma mixes one luma sample pair with the slice's explicit
// bipred factors (offsets average outside the shift).
func (d *Decoder) bipredExplicitLuma(p0, p1 int32, r0, r1 int8) uint8 {
	w0, o0 := d.wW0[r0], d.wO0[r0]
	w1, o1 := d.wW1[r1], d.wO1[r1]
	den := uint(d.wDenomL)
	v := ((p0*w0 + p1*w1 + (1 << den)) >> uint(den+1)) + ((o0 + o1 + 1) >> 1)
	return clipPixel(v)
}

// bipredExplicitChroma mixes one chroma sample pair the same way.
func (d *Decoder) bipredExplicitChroma(p0, p1 int32, r0, r1 int8, comp int) uint8 {
	w0, o0 := d.wWC0[r0][comp], d.wOC0[r0][comp]
	w1, o1 := d.wWC1[r1][comp], d.wOC1[r1][comp]
	den := uint(d.wDenomC)
	v := ((p0*w0 + p1*w1 + (1 << den)) >> uint(den+1)) + ((o0 + o1 + 1) >> 1)
	return clipPixel(v)
}

// finishPMB records the per-MB state shared by all P macroblocks.
func (d *Decoder) finishPMB(h *SliceHeader, addr, mbx, mby int) {
	d.qps[addr] = d.qpY
	d.fIDC[addr] = h.DisableFilter
	d.fA[addr] = h.FilterAlpha
	d.fB[addr] = h.FilterBeta
	d.mbIntra[addr] = false
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			d.modes[(mby*4+y)*stride+mbx*4+x] = -1
		}
	}
}

func (d *Decoder) decodeMBP(r *Reader, pps *PPS, h *SliceHeader, addr int) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	mbType, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("mb type: %w", err)
	}
	if mbType > 30 {
		return fmt.Errorf("%w: mb type %d in P slice", ErrBadSliceHeader, mbType)
	}
	if mbType >= 5 {
		intraType := mbType - 5
		if intraType == 25 {
			return d.decodePCM(r, mbx, mby)
		}
		return d.decodeIntraMB(r, pps, h, addr, mbx, mby, intraType)
	}
	// Inter 0..4.
	return d.decodeMBPParts(h, pps, addr, mbx, mby, mbType, d.cavlcInterSrc(r, h, pps, addr, mbx, mby), d.cavlcSrc(r))
}

// interSrc provides partition-level syntax; CAVLC reads bits, CABAC bins.
// ref/mvd take the partition origin in 4x4 units for neighbour contexts.
//
// S1b-G: zero per-MB heap, same treatment as residSrc (S1b-F): methods
// on a stack value instead of heap closures per macroblock.
type interSrc struct {
	d              *Decoder
	r              *Reader // CAVLC bitstream (nil for CABAC)
	h              *SliceHeader
	pps            *PPS
	addr, mbx, mby int
	cabac          bool
}

// cavlcInterSrc reads inter syntax with Exp-Golomb codes.
func (d *Decoder) cavlcInterSrc(r *Reader, h *SliceHeader, pps *PPS, addr, mbx, mby int) interSrc {
	return interSrc{d: d, r: r, h: h, pps: pps, addr: addr, mbx: mbx, mby: mby}
}

// ref reads one list-0 reference index.
func (s *interSrc) ref(bx, by int) (int8, error) {
	if s.cabac {
		if s.h.RefL0Count <= 1 {
			return 0, nil
		}
		return s.d.cabacRefIdx(s.addr, s.mbx, s.mby, bx, by)
	}
	h := s.h
	if h.RefL0Count <= 1 {
		return 0, nil
	}
	if h.RefL0Count == 2 {
		b, err := s.r.ReadBits(1)
		if err != nil {
			return 0, err
		}
		return int8(b ^ 1), nil
	}
	v, err := s.r.ReadUE()
	if err != nil {
		return 0, err
	}
	if v >= h.RefL0Count {
		return 0, fmt.Errorf("%w: ref idx %d", ErrBadSliceHeader, v)
	}
	return int8(v), nil
}

// mvd reads one motion-vector difference and adds the predictor.
func (s *interSrc) mvd(bx, by int, predX, predY int16) (mx, my, dx, dy int16, err error) {
	if s.cabac {
		dx, err = s.d.cabacMVD(bx, by, 0, 0)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		dy, err = s.d.cabacMVD(bx, by, 1, 0)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		return predX + dx, predY + dy, dx, dy, nil
	}
	dx32, err := s.r.ReadSE()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	dy32, err := s.r.ReadSE()
	if err != nil {
		return 0, 0, 0, 0, err
	}
	dx, dy = int16(dx32), int16(dy32)
	return predX + dx, predY + dy, dx, dy, nil
}

// sub reads the sub-macroblock type.
func (s *interSrc) sub() (uint32, error) {
	if s.cabac {
		d := s.d
		if d.cabBin(21) != 0 {
			return 0, nil
		}
		if d.cabBin(22) == 0 {
			return 1, nil
		}
		if d.cabBin(23) != 0 {
			return 2, nil
		}
		return 3, nil
	}
	v, err := s.r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("sub type: %w", err)
	}
	if v > 3 {
		return 0, fmt.Errorf("%w: sub type %d", ErrBadSliceHeader, v)
	}
	return v, nil
}

// cbp reads the inter coded-block pattern.
func (s *interSrc) cbp() (uint32, error) {
	if s.cabac {
		return s.d.cabacCBP(s.addr, s.mbx, s.mby, false)
	}
	cbpUE, err := s.r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("cbp: %w", err)
	}
	if cbpUE > 47 {
		return 0, fmt.Errorf("%w: cbp %d", ErrBadSliceHeader, cbpUE)
	}
	return uint32(golombToInterCBP[cbpUE]), nil
}

// qpD reads mb_qp_delta.
func (s *interSrc) qpD() (int32, error) {
	if s.cabac {
		return s.d.cabacQPDelta()
	}
	delta, err := s.r.ReadSE()
	if err != nil {
		return 0, fmt.Errorf("qp delta: %w", err)
	}
	return delta, nil
}

// t8 reads the transform_size_8x8_flag for inter blocks.
func (s *interSrc) t8(cbp uint32, subs []uint32) (bool, error) {
	pps := s.pps
	if pps == nil || !pps.Transform8x8 || cbp&15 == 0 {
		return false, nil
	}
	if s.cabac {
		if len(subs) == 4 {
			directInfer := true
			if s.d.sps != nil {
				directInfer = s.d.sps.Direct8x8Infer
			}
			for _, sb := range subs {
				if sb == 0 {
					if !directInfer {
						return false, nil
					}
					continue
				}
				if sb <= 3 {
					continue
				}
				return false, nil
			}
		}
		return s.d.cabBin(399+uint16(s.d.neighborT8(s.addr, s.mbx, s.mby))) != 0, nil
	}
	if len(subs) == 4 {
		for _, sb := range subs {
			if sb != 0 {
				return false, nil
			}
		}
	}
	b, err := s.r.ReadBits(1)
	if err != nil {
		return false, fmt.Errorf("t8 flag: %w", err)
	}
	return b != 0, nil
}

func (d *Decoder) decodeMBPParts(h *SliceHeader, pps *PPS, addr, mbx, mby int, mbType uint32, is interSrc, rs residSrc) error {
	// S1b-J: stack backing for up to 16 sub-8x8 partitions (no heap,
	// no growslice memmove). Same order bit for bit (prefix parity
	// pins output); pure Go so amd64/arm64/fallback share one path.
	var partsBuf [16]part
	parts := partsBuf[:0]
	var subList []uint32
	var subTypes [4]uint32
	hasSubs := false
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	// Right-half top rows start not-available for diagonal prediction
	// until their 8x8 stores land (reference fill semantics).
	d.poisonDiagSlots(mbx, mby)
	switch mbType {
	case 0: // P_16x16
		ref, err := is.ref(x0, y0)
		if err != nil {
			return err
		}
		d.storeRef(x0, y0, 4, 4, ref)
		px, py := d.predMotion(0, x0, y0, 4, ref)
		mx, my, mdx, mdy, err := is.mvd(x0, y0, px, py)
		if err != nil {
			return err
		}
		d.storeMV(px0, py0, 16, 16, mx, my, mdx, mdy, ref)
		parts = append(parts, part{px0, py0, 16, 16, mx, my, ref, 0, 0, 0, true, false})
	case 1: // P_16x8: refs first, then MVDs in order.
		var refs [2]int8
		for i := 0; i < 2; i++ {
			ref, err := is.ref(x0, y0+2*i)
			if err != nil {
				return err
			}
			refs[i] = ref
			d.storeRef(x0, y0+2*i, 4, 2, ref)
		}
		for i := 0; i < 2; i++ {
			ref := refs[i]
			py := py0 + i*8
			var px, pyv int16
			if i == 0 {
				px, pyv = d.pred16x8Top(x0, y0, ref)
			} else {
				px, pyv = d.pred16x8Bottom(x0, y0+2, ref)
			}
			mx, my, mdx, mdy, err := is.mvd(x0, y0+2*i, px, pyv)
			if err != nil {
				return err
			}
			d.storeMV(px0, py, 16, 8, mx, my, mdx, mdy, ref)
			parts = append(parts, part{px0, py, 16, 8, mx, my, ref, 0, 0, 0, true, false})
		}
	case 2: // P_8x16: refs first, then MVDs.
		var refs [2]int8
		for i := 0; i < 2; i++ {
			ref, err := is.ref(x0+2*i, y0)
			if err != nil {
				return err
			}
			refs[i] = ref
			d.storeRef(x0+2*i, y0, 2, 4, ref)
		}
		for i := 0; i < 2; i++ {
			ref := refs[i]
			px := px0 + i*8
			var pxx, pyy int16
			if i == 0 {
				pxx, pyy = d.pred8x16Left(x0, y0, ref)
			} else {
				pxx, pyy = d.pred8x16Right(x0+2, y0, ref)
			}
			mx, my, mdx, mdy, err := is.mvd(x0+2*i, y0, pxx, pyy)
			if err != nil {
				return err
			}
			d.storeMV(px, py0, 8, 16, mx, my, mdx, mdy, ref)
			parts = append(parts, part{px, py0, 8, 16, mx, my, ref, 0, 0, 0, true, false})
		}
	case 3, 4: // P_8x8 / P_8x8ref0
		ref0Only := mbType == 4
		for i := 0; i < 4; i++ {
			v, err := is.sub()
			if err != nil {
				return err
			}
			subTypes[i] = v
		}
		hasSubs = true
		subList = subTypes[:]
		var refs [4]int8
		for i := 0; i < 4; i++ {
			if ref0Only {
				refs[i] = 0
				continue
			}
			ref, err := is.ref(x0+(i%2)*2, y0+(i/2)*2)
			if err != nil {
				return err
			}
			refs[i] = ref
			d.storeRef(x0+(i%2)*2, y0+(i/2)*2, 2, 2, ref)
		}
		// 8x8 order: 0 TL, 1 TR, 2 BL, 3 BR. MVDs follow refs.
		for i := 0; i < 4; i++ {
			bx8, by8 := i%2, i/2
			ox, oy := px0+bx8*8, py0+by8*8
			qx, qy := x0+bx8*2, y0+by8*2
			ref := refs[i]
			switch subTypes[i] {
			case 0: // 8x8
				px, py := d.predMotion(4*i, qx, qy, 2, ref)
				mx, my, mdx, mdy, err := is.mvd(qx, qy, px, py)
				if err != nil {
					return err
				}
				d.storeMV(ox, oy, 8, 8, mx, my, mdx, mdy, ref)
				parts = append(parts, part{ox, oy, 8, 8, mx, my, ref, 0, 0, 0, true, false})
			case 1: // 8x4 top,bottom
				for k := 0; k < 2; k++ {
					py := oy + k*4
					sqy := qy + k
					px, pyv := d.predMotion(4*i+2*k, qx, sqy, 2, ref)
					mx, my, mdx, mdy, err := is.mvd(qx, sqy, px, pyv)
					if err != nil {
						return err
					}
					d.storeMV(ox, py, 8, 4, mx, my, mdx, mdy, ref)
					parts = append(parts, part{ox, py, 8, 4, mx, my, ref, 0, 0, 0, true, false})
				}
			case 2: // 4x8 left,right
				for k := 0; k < 2; k++ {
					px := ox + k*4
					sqx := qx + k
					pvx, pvy := d.predMotion(4*i+k, sqx, qy, 1, ref)
					mx, my, mdx, mdy, err := is.mvd(sqx, qy, pvx, pvy)
					if err != nil {
						return err
					}
					d.storeMV(px, oy, 4, 8, mx, my, mdx, mdy, ref)
					parts = append(parts, part{px, oy, 4, 8, mx, my, ref, 0, 0, 0, true, false})
				}
			default: // 4x4 raster
				for k := 0; k < 4; k++ {
					px := ox + (k%2)*4
					py := oy + (k/2)*4
					sqx, sqy := qx+(k%2), qy+(k/2)
					pvx, pvy := d.predMotion(4*i+k, sqx, sqy, 1, ref)
					mx, my, mdx, mdy, err := is.mvd(sqx, sqy, pvx, pvy)
					if err != nil {
						return err
					}
					d.storeMV(px, py, 4, 4, mx, my, mdx, mdy, ref)
					parts = append(parts, part{px, py, 4, 4, mx, my, ref, 0, 0, 0, true, false})
				}
			}
		}
	}
	// Motion compensation into full-MB prediction buffers.
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	if err := d.mcParts(mbx, mby, parts, &predY, &predCb, &predCr); err != nil {
		return err
	}
	// CBP + 8x8 flag + QP delta + residual (t8 sits between CBP and
	// QP delta in the bitstream).
	cbp, err := is.cbp()
	if err != nil {
		return err
	}
	var subs []uint32
	if hasSubs {
		subs = subList
	}
	use8, err := is.t8(cbp, subs)
	if err != nil {
		return err
	}
	if addr >= 0 && addr < len(d.mbT8) {
		d.mbT8[addr] = use8
	}
	if cbp != 0 {
		delta, err := is.qpD()
		if err != nil {
			return err
		}
		d.qpY += delta
		for d.qpY < 0 {
			d.qpY += 52
		}
		for d.qpY > 51 {
			d.qpY -= 52
		}
	}
	d.finishPMB(h, addr, mbx, mby)
	d.cbpArr[addr] = uint16(cbp)
	d.mbSlice[addr] = d.slices
	if use8 {
		if err := d.reconstructInter8x8With(mbx, mby, cbp, &predY, &predCb, &predCr, rs); err != nil {
			return err
		}
		return nil
	}
	if err := d.reconstructInterWith(mbx, mby, cbp, &predY, &predCb, &predCr, rs); err != nil {
		return err
	}
	return nil
}

func (d *Decoder) reconstructInter(r *Reader, mbx, mby int, cbp uint32, predY *[256]uint8, predCb, predCr *[64]uint8) error {
	return d.reconstructInterWith(mbx, mby, cbp, predY, predCb, predCr, d.cavlcSrc(r))
}

func (d *Decoder) reconstructInterWith(mbx, mby int, cbp uint32, predY *[256]uint8, predCb, predCr *[64]uint8, rs residSrc) error {
	// Fast path: zero residual means prediction IS the reconstruction
	// (skip/direct blocks, the common case). Copy rows and mark all
	// coefficients zero — no per-block inverse transform of all-zero
	// input, no per-pixel clip.
	if cbp == 0 {
		stride := int(d.pic.Width)
		base := mby*16*stride + mbx*16
		// S1b-Z luma zero lane (same 16 bytes per row as copy, no
		// memmove call): 16x16-byte row copies went through
		// runtime.memmove (call overhead dominates a 16-byte move;
		// ~20ms flat + loop overhead per 90f profile). Sixteen
		// plain byte stores compile to inline MOVBs with the
		// bounds checks eliminated (row sliced once, indices
		// 0..15 proven in range), bit-identical bytes on every
		// arch (same shape as the chroma S1b-C lane in mb.go).
		for y := 0; y < 16; y++ {
			d8 := d.pic.Y[base+y*stride : base+y*stride+16]
			s8 := predY[y*16 : y*16+16]
			d8[0], d8[1], d8[2], d8[3], d8[4], d8[5], d8[6], d8[7],
				d8[8], d8[9], d8[10], d8[11], d8[12], d8[13], d8[14], d8[15] =
				s8[0], s8[1], s8[2], s8[3], s8[4], s8[5], s8[6], s8[7],
				s8[8], s8[9], s8[10], s8[11], s8[12], s8[13], s8[14], s8[15]
		}
		// S1b-Z nnz shape (same 16 zero slots as the double loop,
		// no per-element bounds checks): the 4x4 luma slots sit
		// at four row slices, one slice each, identical values.
		ystride := d.mbW * 4
		n0 := (mby*4)*ystride + mbx*4
		r0 := d.nnzY[n0 : n0+4]
		r0[0], r0[1], r0[2], r0[3] = 0, 0, 0, 0
		r1 := d.nnzY[n0+ystride : n0+ystride+4]
		r1[0], r1[1], r1[2], r1[3] = 0, 0, 0, 0
		r2 := d.nnzY[n0+2*ystride : n0+2*ystride+4]
		r2[0], r2[1], r2[2], r2[3] = 0, 0, 0, 0
		r3 := d.nnzY[n0+3*ystride : n0+3*ystride+4]
		r3[0], r3[1], r3[2], r3[3] = 0, 0, 0, 0
		return d.reconstructChromaBlocks(mbx, mby, 0, predCb, predCr, rs, false)
	}
	stride := d.mbW * 4
	wY := d.sc4[3]
	for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
		bx, by := mbx*4+b%4, mby*4+b/4
		var coeff [16]int32
		tc := 0
		i8 := (b/8)*2 + (b%4)/2
		if cbp&(1<<uint(i8)) != 0 {
			c, total, err := rs.lumaAC(bx, by, 2)
			if err != nil {
				return err
			}
			coeff, tc = c, total
		}
		res := ITransform4x4Scaled(coeff, uint32(d.qpY), wY)
		lx0, ly0 := (bx-mbx*4)*4, (by-mby*4)*4
		if !addResidBlock(d.pic.Y, d.pic.Width, uint32(bx*4), uint32(by*4), predY[ly0*16+lx0:], 16, res[:], 4, 4) {
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					lx, ly := (bx-mbx*4)*4+x, (by-mby*4)*4+y
					v := int32(predY[ly*16+lx]) + res[y*4+x]
					d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
				}
			}
		}
		d.setNnz(d.nnzY, stride, bx, by, tc)
	}
	return d.reconstructChromaBlocks(mbx, mby, (cbp>>4)&3, predCb, predCr, rs, false)
}

// reconstructInter8x8With adds 8x8-transformed luma residual (CAVLC or
// CABAC 8x8 blocks) to inter prediction.
//
// S1-H: cbp==0 fast path (prediction IS the reconstruction, like the
// 4x4 reconstructInterWith path) plus per-8x8 empty skip (bit 0 means
// coeff zero, whose transform is zero per TestS18x8ZeroSkipsCore, so
// copy prediction rows instead of running dequant+IDCT+add). Same
// bytes (prefix parity pins every pixel); pure Go so amd64/arm64/
// fallback share one path.
func (d *Decoder) reconstructInter8x8With(mbx, mby int, cbp uint32, predY *[256]uint8, predCb, predCr *[64]uint8, rs residSrc) error {
	stride := d.mbW * 4
	if cbp == 0 {
		picStride := int(d.pic.Width)
		base := mby*16*picStride + mbx*16
		for y := 0; y < 16; y++ {
			copy(d.pic.Y[base+y*picStride:base+y*picStride+16], predY[y*16:(y+1)*16])
		}
		for by := mby * 4; by < mby*4+4; by++ {
			for bx := mbx * 4; bx < mbx*4+4; bx++ {
				d.nnzY[by*stride+bx] = 0
			}
		}
		return d.reconstructChromaBlocks(mbx, mby, 0, predCb, predCr, rs, false)
	}
	w8 := d.sc8[1]
	grouped := [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15}
	for i8 := 0; i8 < 4; i8++ {
		ox, oy := (i8%2)*8, (i8/2)*8
		if cbp&(1<<uint(i8)) == 0 {
			picStride := int(d.pic.Width)
			base := (mby*16+oy)*picStride + mbx*16 + ox
			for y := 0; y < 8; y++ {
				copy(d.pic.Y[base+y*picStride:base+y*picStride+8], predY[(oy+y)*16+ox:(oy+y)*16+ox+8])
			}
			for g := 0; g < 4; g++ {
				b := grouped[i8*4+g]
				bx, by := mbx*4+b%4, mby*4+b/4
				d.setNnz(d.nnzY, stride, bx, by, 0)
			}
			continue
		}
		var coeff [64]int32
		var tcs [4]int
		c, t, err := rs.luma8x8(mbx, mby, i8)
		if err != nil {
			return err
		}
		coeff, tcs = c, t
		res := ITransform8x8Scaled(coeff, uint32(d.qpY), w8)
		bx0, by0 := mbx*4+(grouped[i8*4]%4), mby*4+(grouped[i8*4]/4)
		_ = bx0
		_ = by0
		if !addResidBlock(d.pic.Y, d.pic.Width, uint32(mbx*16+ox), uint32(mby*16+oy), predY[oy*16+ox:], 16, res[:], 8, 8) {
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					v := int32(predY[(oy+y)*16+ox+x]) + res[y*8+x]
					d.pic.SetY(uint32(mbx*16+ox+x), uint32(mby*16+oy+y), clipPixel(v))
				}
			}
		}
		for g := 0; g < 4; g++ {
			b := grouped[i8*4+g]
			bx, by := mbx*4+b%4, mby*4+b/4
			d.setNnz(d.nnzY, stride, bx, by, tcs[g])
		}
	}
	return d.reconstructChromaBlocks(mbx, mby, (cbp>>4)&3, predCb, predCr, rs, false)
}
