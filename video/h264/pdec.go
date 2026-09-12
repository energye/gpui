package h264

import "fmt"

// P-slice macroblock decoding: skip, inter partitions, intra fallback.

// part is one motion-compensated rectangle in luma pixels.
type part struct {
	px, py, w, h int
	mx, my       int16
	ref          int8
}

func (d *Decoder) decodeSkip(h *SliceHeader, addr int, rs *residSrc) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	if d.refPic == nil {
		return fmt.Errorf("%w: skip without reference", ErrBadSliceHeader)
	}
	mx, my := d.predMotion(mbx*4, mby*4, 4, 0)
	d.storeMV(mbx*16, mby*16, 16, 16, mx, my, 0, 0, 0)
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	parts := [1]part{{mbx * 16, mby * 16, 16, 16, mx, my, 0}}
	if err := d.mcParts(mbx, mby, parts[:], &predY, &predCb, &predCr); err != nil {
		return err
	}
	d.finishPMB(h, addr, mbx, mby)
	d.skipped[addr] = true
	d.mbSlice[addr] = d.slices
	if addr >= 0 && addr < len(d.mbT8) {
		d.mbT8[addr] = false
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

// fillPred resets MB prediction buffers to mid-grey.
func fillPred(predY *[256]uint8, predCb, predCr *[64]uint8) {
	for i := range predY {
		predY[i] = 128
	}
	for i := range predCb {
		predCb[i], predCr[i] = 128, 128
	}
}

// mcParts runs motion compensation for every partition into MB-sized
// prediction buffers.
func (d *Decoder) mcParts(mbx, mby int, parts []part, predY *[256]uint8, predCb, predCr *[64]uint8) error {
	if len(parts) == 0 {
		return fmt.Errorf("%w: no partitions", ErrBadSliceHeader)
	}
	rw, rh := int(d.pic.Width/2), int(d.pic.Height/2)
	var blk [256]uint8
	var cb, cr [64]uint8
	for _, pt := range parts {
		rp, err := d.refFor(pt.ref)
		if err != nil {
			return err
		}
		predictLumaBlock(rp, pt.px, pt.py, pt.w, pt.h, pt.mx, pt.my, blk[:pt.w*pt.h])
		d.weightLuma(blk[:pt.w*pt.h], pt.ref)
		for y := 0; y < pt.h; y++ {
			for x := 0; x < pt.w; x++ {
				ox, oy := pt.px-mbx*16+x, pt.py-mby*16+y
				predY[oy*16+ox] = blk[y*pt.w+x]
			}
		}
		cw, ch := pt.w/2, pt.h/2
		cx, cy := (pt.px-mbx*16)/2, (pt.py-mby*16)/2
		// Chroma origin in the subsampled plane.
		ccx, ccy := mbx*8+cx, mby*8+cy
		predictChromaBlock(rp.Cb, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my, cb[:cw*ch])
		predictChromaBlock(rp.Cr, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my, cr[:cw*ch])
		d.weightChroma(cb[:cw*ch], cr[:cw*ch], pt.ref)
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				predCb[(cy+y)*8+cx+x] = cb[y*cw+x]
				predCr[(cy+y)*8+cx+x] = cr[y*cw+x]
			}
		}
	}
	return nil
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
type interSrc struct {
	ref func(bx, by int) (int8, error)
	mvd func(bx, by int, px, py int16) (mx, my, dx, dy int16, err error)
	sub func() (uint32, error)
	cbp func() (uint32, error)
	qpD func() (int32, error)
	t8  func(cbp uint32, subs []uint32) (bool, error)
}

// cavlcInterSrc reads inter syntax with Exp-Golomb codes.
func (d *Decoder) cavlcInterSrc(r *Reader, h *SliceHeader, pps *PPS, addr, mbx, mby int) *interSrc {
	readRef := func(bx, by int) (int8, error) {
		if h.RefL0Count <= 1 {
			return 0, nil
		}
		if h.RefL0Count == 2 {
			b, err := r.ReadBits(1)
			if err != nil {
				return 0, err
			}
			return int8(b ^ 1), nil
		}
		v, err := r.ReadUE()
		if err != nil {
			return 0, err
		}
		if v >= h.RefL0Count {
			return 0, fmt.Errorf("%w: ref idx %d", ErrBadSliceHeader, v)
		}
		return int8(v), nil
	}
	readMVD := func(bx, by int, predX, predY int16) (int16, int16, int16, int16, error) {
		dx, err := r.ReadSE()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		dy, err := r.ReadSE()
		if err != nil {
			return 0, 0, 0, 0, err
		}
		return predX + int16(dx), predY + int16(dy), int16(dx), int16(dy), nil
	}
	return &interSrc{
		ref: readRef,
		mvd: readMVD,
		sub: func() (uint32, error) {
			v, err := r.ReadUE()
			if err != nil {
				return 0, fmt.Errorf("sub type: %w", err)
			}
			if v > 3 {
				return 0, fmt.Errorf("%w: sub type %d", ErrBadSliceHeader, v)
			}
			return v, nil
		},
		cbp: func() (uint32, error) {
			cbpUE, err := r.ReadUE()
			if err != nil {
				return 0, fmt.Errorf("cbp: %w", err)
			}
			if cbpUE > 47 {
				return 0, fmt.Errorf("%w: cbp %d", ErrBadSliceHeader, cbpUE)
			}
			return uint32(golombToInterCBP[cbpUE]), nil
		},
		qpD: func() (int32, error) {
			delta, err := r.ReadSE()
			if err != nil {
				return 0, fmt.Errorf("qp delta: %w", err)
			}
			return delta, nil
		},
		t8: func(cbp uint32, subs []uint32) (bool, error) {
			if pps == nil || !pps.Transform8x8 || cbp&15 == 0 {
				return false, nil
			}
			if len(subs) == 4 {
				for _, s := range subs {
					if s != 0 {
						return false, nil
					}
				}
			}
			b, err := r.ReadBits(1)
			if err != nil {
				return false, fmt.Errorf("t8 flag: %w", err)
			}
			return b != 0, nil
		},
	}
}

func (d *Decoder) decodeMBPParts(h *SliceHeader, pps *PPS, addr, mbx, mby int, mbType uint32, is *interSrc, rs *residSrc) error {
	var parts []part
	var subList []uint32
	var subTypes [4]uint32
	hasSubs := false
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	switch mbType {
	case 0: // P_16x16
		ref, err := is.ref(x0, y0)
		if err != nil {
			return err
		}
		d.storeRef(x0, y0, 4, 4, ref)
		px, py := d.predMotion(x0, y0, 4, ref)
		mx, my, mdx, mdy, err := is.mvd(x0, y0, px, py)
		if err != nil {
			return err
		}
		d.storeMV(px0, py0, 16, 16, mx, my, mdx, mdy, ref)
		parts = append(parts, part{px0, py0, 16, 16, mx, my, ref})
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
			parts = append(parts, part{px0, py, 16, 8, mx, my, ref})
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
			parts = append(parts, part{px, py0, 8, 16, mx, my, ref})
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
				px, py := d.predMotion(qx, qy, 2, ref)
				mx, my, mdx, mdy, err := is.mvd(qx, qy, px, py)
				if err != nil {
					return err
				}
				d.storeMV(ox, oy, 8, 8, mx, my, mdx, mdy, ref)
				parts = append(parts, part{ox, oy, 8, 8, mx, my, ref})
			case 1: // 8x4 top,bottom
				for k := 0; k < 2; k++ {
					py := oy + k*4
					sqy := qy + k
					px, pyv := d.predMotion(qx, sqy, 2, ref)
					mx, my, mdx, mdy, err := is.mvd(qx, sqy, px, pyv)
					if err != nil {
						return err
					}
					d.storeMV(ox, py, 8, 4, mx, my, mdx, mdy, ref)
					parts = append(parts, part{ox, py, 8, 4, mx, my, ref})
				}
			case 2: // 4x8 left,right
				for k := 0; k < 2; k++ {
					px := ox + k*4
					sqx := qx + k
					pvx, pvy := d.predMotion(sqx, qy, 1, ref)
					mx, my, mdx, mdy, err := is.mvd(sqx, qy, pvx, pvy)
					if err != nil {
						return err
					}
					d.storeMV(px, oy, 4, 8, mx, my, mdx, mdy, ref)
					parts = append(parts, part{px, oy, 4, 8, mx, my, ref})
				}
			default: // 4x4 raster
				for k := 0; k < 4; k++ {
					px := ox + (k%2)*4
					py := oy + (k/2)*4
					sqx, sqy := qx+(k%2), qy+(k/2)
					pvx, pvy := d.predMotion(sqx, sqy, 1, ref)
					mx, my, mdx, mdy, err := is.mvd(sqx, sqy, pvx, pvy)
					if err != nil {
						return err
					}
					d.storeMV(px, py, 4, 4, mx, my, mdx, mdy, ref)
					parts = append(parts, part{px, py, 4, 4, mx, my, ref})
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

func (d *Decoder) reconstructInterWith(mbx, mby int, cbp uint32, predY *[256]uint8, predCb, predCr *[64]uint8, rs *residSrc) error {
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
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				lx, ly := (bx-mbx*4)*4+x, (by-mby*4)*4+y
				v := int32(predY[ly*16+lx]) + res[y*4+x]
				d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
			}
		}
		d.setNnz(d.nnzY, stride, bx, by, tc)
	}
	return d.reconstructChromaBlocks(mbx, mby, (cbp>>4)&3, predCb, predCr, rs, false)
}

// reconstructInter8x8With adds 8x8-transformed luma residual (CAVLC or
// CABAC 8x8 blocks) to inter prediction.
func (d *Decoder) reconstructInter8x8With(mbx, mby int, cbp uint32, predY *[256]uint8, predCb, predCr *[64]uint8, rs *residSrc) error {
	stride := d.mbW * 4
	w8 := d.sc8[1]
	grouped := [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15}
	for i8 := 0; i8 < 4; i8++ {
		var coeff [64]int32
		var tcs [4]int
		if cbp&(1<<uint(i8)) != 0 {
			c, t, err := rs.luma8x8(mbx, mby, i8)
			if err != nil {
				return err
			}
			coeff, tcs = c, t
		}
		res := ITransform8x8Scaled(coeff, uint32(d.qpY), w8)
		bx0, by0 := mbx*4+(grouped[i8*4]%4), mby*4+(grouped[i8*4]/4)
		_ = bx0
		_ = by0
		ox, oy := (i8%2)*8, (i8/2)*8
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				v := int32(predY[(oy+y)*16+ox+x]) + res[y*8+x]
				d.pic.SetY(uint32(mbx*16+ox+x), uint32(mby*16+oy+y), clipPixel(v))
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
