package h264

import "fmt"

// B-slice macroblock decoding: skip, direct, explicit partitions (both
// prediction directions), intra fallback. Partition shapes follow the
// same tables as the reference decoder (CAVLC Table 7-11/7-17); CABAC
// binarizations mirror its B paths. Reimplemented in pure Go.

// bDir is one B partition's prediction direction.
type bDir uint8

const (
	bL0 bDir = iota
	bL1
	bBi
	bDirect
)

func (dir bDir) usesL0() bool { return dir == bL0 || dir == bBi || dir == bDirect }
func (dir bDir) usesL1() bool { return dir == bL1 || dir == bBi || dir == bDirect }

// bMBDirs gives both halves' directions for B types 4..21 (even 16x8,
// odd 8x16); types 1..3 are single 16x16 partitions.
var bMBDirs = [22][2]bDir{
	1: {bL0}, 2: {bL1}, 3: {bBi},
	4:  {bL0, bL0},
	5:  {bL0, bL0},
	6:  {bL1, bL1},
	7:  {bL1, bL1},
	8:  {bL0, bL1},
	9:  {bL0, bL1},
	10: {bL1, bL0},
	11: {bL1, bL0},
	12: {bL0, bBi},
	13: {bL0, bBi},
	14: {bL1, bBi},
	15: {bL1, bBi},
	16: {bBi, bL0},
	17: {bBi, bL0},
	18: {bBi, bL1},
	19: {bBi, bL1},
	20: {bBi, bBi},
	21: {bBi, bBi},
}

// bSubDirShape gives one B 8x8 sub-block's direction and sub-shape
// (0 = 8x8, 1 = 8x4 pair, 2 = 4x8 pair, 3 = 4x4 quad).
func bSubDirShape(sub uint32) (bDir, int, error) {
	switch sub {
	case 0:
		return bDirect, 0, nil
	case 1:
		return bL0, 0, nil
	case 2:
		return bL1, 0, nil
	case 3:
		return bBi, 0, nil
	case 4:
		return bL0, 1, nil
	case 5:
		return bL0, 2, nil
	case 6:
		return bL1, 1, nil
	case 7:
		return bL1, 2, nil
	case 8:
		return bBi, 1, nil
	case 9:
		return bBi, 2, nil
	case 10:
		return bL0, 3, nil
	case 11:
		return bL1, 3, nil
	case 12:
		return bBi, 3, nil
	}
	return 0, 0, fmt.Errorf("%w: B sub type %d", ErrBadSliceHeader, sub)
}

// bInterSrc provides B partition-level syntax; CAVLC reads bits, CABAC
// bins. ref0/ref1 take the partition origin in 4x4 units.
//
// S1b-G: zero per-MB heap, same treatment as residSrc (S1b-F): methods
// on a stack value instead of heap closures per macroblock.
type bInterSrc struct {
	d              *Decoder
	r              *Reader // CAVLC bitstream (nil for CABAC)
	h              *SliceHeader
	pps            *PPS
	addr, mbx, mby int
	cabac          bool
}

// cavlcBInterSrc reads B inter syntax with Exp-Golomb codes.
func (d *Decoder) cavlcBInterSrc(r *Reader, h *SliceHeader, pps *PPS) bInterSrc {
	return bInterSrc{d: d, r: r, h: h, pps: pps}
}

// ref0 reads one list-0 reference index.
func (s *bInterSrc) ref0(bx, by int) (int8, error) {
	if s.cabac {
		if s.h.RefL0Count <= 1 {
			return 0, nil
		}
		return s.d.cabacRefIdxB(s.addr, s.mbx, s.mby, bx, by, 0)
	}
	return s.cavlcBRef(bx, by, 0)
}

// ref1 reads one list-1 reference index.
func (s *bInterSrc) ref1(bx, by int) (int8, error) {
	if s.cabac {
		if s.h.RefL1Count <= 1 {
			return 0, nil
		}
		return s.d.cabacRefIdxB(s.addr, s.mbx, s.mby, bx, by, 1)
	}
	return s.cavlcBRef(bx, by, 1)
}

// cavlcBRef reads one list's CAVLC reference: no code for a single
// reference, one inverted bit for two, Exp-Golomb above that.
func (s *bInterSrc) cavlcBRef(bx, by int, list int) (int8, error) {
	var n uint32
	if list == 0 {
		n = s.h.RefL0Count
	} else {
		n = s.h.RefL1Count
	}
	if n <= 1 {
		return 0, nil
	}
	if n == 2 {
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
	if v >= n {
		return 0, fmt.Errorf("%w: B ref l%d idx %d", ErrBadSliceHeader, list, v)
	}
	return int8(v), nil
}

// mvd reads one motion-vector difference of one list and adds it.
func (s *bInterSrc) mvd(list, bx, by int, predX, predY int16) (mx, my, dx, dy int16, err error) {
	if s.cabac {
		dx, err = s.d.cabacMVD(bx, by, 0, list)
		if err != nil {
			return 0, 0, 0, 0, err
		}
		dy, err = s.d.cabacMVD(bx, by, 1, list)
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

// sub reads the B sub-macroblock type.
func (s *bInterSrc) sub() (uint32, error) {
	if s.cabac {
		return s.d.cabacBSubType()
	}
	v, err := s.r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("B sub type: %w", err)
	}
	if v > 12 {
		return 0, fmt.Errorf("%w: B sub type %d", ErrBadSliceHeader, v)
	}
	return v, nil
}

// cbp reads the B coded-block pattern.
func (s *bInterSrc) cbp() (uint32, error) {
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
func (s *bInterSrc) qpD() (int32, error) {
	if s.cabac {
		return s.d.cabacQPDelta()
	}
	delta, err := s.r.ReadSE()
	if err != nil {
		return 0, fmt.Errorf("qp delta: %w", err)
	}
	return delta, nil
}

// t8 reads the transform_size_8x8_flag for B blocks.
func (s *bInterSrc) t8(cbp uint32, subs []uint32) (bool, error) {
	pps := s.pps
	if pps == nil || !pps.Transform8x8 || cbp&15 == 0 {
		return false, nil
	}
	if len(subs) == 4 {
		if s.cabac {
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
		} else {
			for _, sb := range subs {
				if sb != 0 {
					return false, nil
				}
			}
		}
	}
	if s.cabac {
		return s.d.cabBin(399+uint16(s.d.neighborT8(s.addr, s.mbx, s.mby))) != 0, nil
	}
	b, err := s.r.ReadBits(1)
	if err != nil {
		return false, fmt.Errorf("t8 flag: %w", err)
	}
	return b != 0, nil
}

// decodeSkipB decodes a B_Skip macroblock: spatial-direct motion, no
// residual. Every 4x4 records the derived direction for neighbours,
// deblocking and CABAC contexts.
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//
//	libavcodec/h264_mvpred.h:950-984 decode_mb_skip (B path: mark
//	L0L1|DIRECT2|SKIP, fill_decode_neighbors/caches when spatial,
//	ff_h264_pred_direct_motion, write_back_motion) +
//	libavcodec/h264_direct.c:208-494 pred_spatial_direct_motion
//	(unsigned-minimum ref = FFMIN3, median/single-match motion, zero
//	fast lane, stationary-colocated zeroing; our bDirectMBBlocks +
//	applyDirectStationary below); oracle is VR2 exact (pixels).
//
// Hot path (S1b-B, ENERGY 1536x864: decodeSkipB ~45% of H.264 math):
// skip blocks carry no residual, so the whole block is motion +
// zero-fill + state marks. This entry keeps the slow readable order
// and delegates the bulk to skipBFast (bulk path below); outputs are
// bit-identical (skip_b test pins MB state + pixels).
func (d *Decoder) decodeSkipB(h *SliceHeader, addr int, rs residSrc) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	if len(d.refList1) == 0 || d.refList1[0] == nil {
		return fmt.Errorf("%w: B skip without list-1 picture", ErrBadSliceHeader)
	}
	dms, err := d.bDirectMBBlocks(mbx, mby, h, d.refList1[0])
	if err != nil {
		return err
	}
	if d.skipBFast(h, addr, mbx, mby, dms, rs) {
		return nil
	}
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	var parts [4]part
	for i := 0; i < 4; i++ {
		parts[i] = directPartB8(px0+(i%2)*8, py0+(i/2)*8, dms[i])
	}
	// Uniform-motion shortcut: all four 8x8 parts share one motion
	// (same use flags, refs, vectors). Prediction tiles one 16x16
	// call instead of four 8x8 calls; motion stores still run per
	// 8x8 below (neighbours read derived indices per block).
	//
	// NOTE (S1b-J tried, reverted): a 16-slot branch-free bulk store
	// (storeDirectUniform) measured zero gain — the stores are memory
	// bound (176 writes/MB), the per-slot branches perfectly
	// predicted; branch elimination saves nothing measurable. Keep
	// the simple per-8x8 stores.
	dm0 := dms[0]
	if dms[1] == dm0 && dms[2] == dm0 && dms[3] == dm0 && !d.wBiExpl {
		for i := 0; i < 4; i++ {
			qx := x0 + (i%2)*2
			qy := y0 + (i/2)*2
			d.storeDirectB8(qx, qy, dms[i])
		}
		if d.skipBUniform(h, addr, mbx, mby, dm0, dm0[0].use, dm0[1].use, px0, py0, rs) {
			return nil
		}
		return fmt.Errorf("%w: skip uniform failed", ErrBadSliceHeader)
	}
	for i := 0; i < 4; i++ {
		qx := x0 + (i%2)*2
		qy := y0 + (i/2)*2
		d.storeDirectB8(qx, qy, dms[i])
	}
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	if err := d.mcParts(mbx, mby, parts[:], &predY, &predCb, &predCr); err != nil {
		return err
	}
	d.finishPMB(h, addr, mbx, mby)
	d.skipped[addr] = true
	d.mbDirect[addr] = true
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

// skipBFast is the S1b-B bulk path for B_Skip: motion stores + MC +
// zero-residual reconstruction + state marks in one pass. It returns
// false when it cannot run (explicit bipred weights on this slice),
// and the caller falls back to the slow readable path above with
// identical output.
//
// Why this shape (not just a faster mcParts): a skip block is four
// fixed 8x8 direct parts at fixed offsets with cbp==0, so the whole
// block skips every per-partition branch (no use0/use1 dispatch, no
// weight dispatch per part, no residual switch, no nnz switch) and
// folds the three loops (store 4x4 motion + predict + copy + zero-fill)
// into fixed-offset writes. The implicit-weight call hoists out of
// the part loop (same poc/ref pair for all four parts here).
func (d *Decoder) skipBFast(h *SliceHeader, addr, mbx, mby int, dms [4][2]bDirectMV, rs residSrc) bool {
	// Explicit bipred weights need per-part table lookups; stay slow.
	if d.wBiExpl {
		return false
	}
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	// Uniformity first (cheap struct compares): all four 8x8 parts
	// sharing one motion is the overwhelming case on still content.
	// Uniform blocks store per 8x8 below like mixed shapes (a 16-slot
	// branch-free bulk store measured zero gain here — the stores are
	// memory bound, branches predicted; see the uniform shortcut in
	// decodeSkipB) and predict once at 16x16 (skipBUniform). Motion
	// stores run before any prediction either way (same order as the
	// slow path, so intra-MB neighbours read the same derived
	// indices).
	u0 := dms[0][0].use && dms[1][0].use && dms[2][0].use && dms[3][0].use
	u1 := dms[0][1].use && dms[1][1].use && dms[2][1].use && dms[3][1].use
	sameRef := dms[0][0].ref == dms[1][0].ref && dms[0][0].ref == dms[2][0].ref && dms[0][0].ref == dms[3][0].ref &&
		dms[0][1].ref == dms[1][1].ref && dms[0][1].ref == dms[2][1].ref && dms[0][1].ref == dms[3][1].ref
	sameMV := dms[0][0].mx == dms[1][0].mx && dms[0][0].mx == dms[2][0].mx && dms[0][0].mx == dms[3][0].mx &&
		dms[0][0].my == dms[1][0].my && dms[0][0].my == dms[2][0].my && dms[0][0].my == dms[3][0].my &&
		dms[0][1].mx == dms[1][1].mx && dms[0][1].mx == dms[2][1].mx && dms[0][1].mx == dms[3][1].mx &&
		dms[0][1].my == dms[1][1].my && dms[0][1].my == dms[2][1].my && dms[0][1].my == dms[3][1].my
	if u0 == (dms[0][0].use || dms[1][0].use || dms[2][0].use || dms[3][0].use) &&
		u1 == (dms[0][1].use || dms[1][1].use || dms[2][1].use || dms[3][1].use) &&
		sameRef && sameMV {
		d.storeDirectB8(x0, y0, dms[0])
		d.storeDirectB8(x0+2, y0, dms[1])
		d.storeDirectB8(x0, y0+2, dms[2])
		d.storeDirectB8(x0+2, y0+2, dms[3])
		if d.skipBUniform(h, addr, mbx, mby, dms[0], u0, u1, px0, py0, rs) {
			return true
		}
		return false
	}
	// Mixed shapes: per-8x8 stores (unrolled, no loop counter), then
	// the per-part predict loop below.
	d.storeDirectB8(x0, y0, dms[0])
	d.storeDirectB8(x0+2, y0, dms[1])
	d.storeDirectB8(x0, y0+2, dms[2])
	d.storeDirectB8(x0+2, y0+2, dms[3])
	// Implicit weight is constant across the four parts here: same
	// current POC and same ref pair per list when both lists used.
	// (Mixed use shapes still go per-part below; the common skip
	// shape is all-four-bipred or all-four-single-list.)
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	rw, rh := int(d.pic.Width/2), int(d.pic.Height/2)
	var blk [256]uint8
	var blk1 [256]uint8
	var cb, cr [64]uint8
	var cb1, cr1 [64]uint8
	for i := 0; i < 4; i++ {
		dm := dms[i]
		ox, oy := (i%2)*8, (i/2)*8
		// Luma.
		switch {
		case dm[0].use && dm[1].use:
			rp0, err := d.refFor(dm[0].ref)
			if err != nil {
				return false
			}
			rp1, err := d.refFor1(dm[1].ref)
			if err != nil {
				return false
			}
			// S1b-W2 dual direct-write, mixed 8x8 (same shape as
			// mcParts dual: list 0 straight into predY, list 1
			// into one temp, per-row mix in place; implicit only
			// here, explicit stays on the slow mcParts path).
			predictLumaBlockInto(rp0, px0+ox, py0+oy, 8, 8, dm[0].mx, dm[0].my, predY[oy*16+ox:], 16)
			predictLumaBlock(rp1, px0+ox, py0+oy, 8, 8, dm[1].mx, dm[1].my, blk1[:64])
			w := d.bipredWeight(d.pic, rp0, rp1, dm[0].ref, dm[1].ref)
			for y := 0; y < 8; y++ {
				bipredAvg(predY[(oy+y)*16+ox:(oy+y)*16+ox+8], blk1[y*8:(y+1)*8], w)
			}
		case dm[0].use:
			rp, err := d.refFor(dm[0].ref)
			if err != nil {
				return false
			}
			predictLumaBlock(rp, px0+ox, py0+oy, 8, 8, dm[0].mx, dm[0].my, blk[:64])
			d.weightLuma(blk[:64], dm[0].ref)
			for y := 0; y < 8; y++ {
				copy(predY[(oy+y)*16+ox:(oy+y)*16+ox+8], blk[y*8:(y+1)*8])
			}
		case dm[1].use:
			rp, err := d.refFor1(dm[1].ref)
			if err != nil {
				return false
			}
			predictLumaBlock(rp, px0+ox, py0+oy, 8, 8, dm[1].mx, dm[1].my, blk[:64])
			d.weightLuma1(blk[:64], dm[1].ref)
			for y := 0; y < 8; y++ {
				copy(predY[(oy+y)*16+ox:(oy+y)*16+ox+8], blk[y*8:(y+1)*8])
			}
		default:
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					predY[(oy+y)*16+ox+x] = 128
				}
			}
		}
		// Chroma (half offsets).
		cw, ch := 4, 4
		cx, cy := ox/2, oy/2
		ccx, ccy := mbx*8+cx, mby*8+cy
		switch {
		case dm[0].use && dm[1].use:
			rp0, err := d.refFor(dm[0].ref)
			if err != nil {
				return false
			}
			rp1, err := d.refFor1(dm[1].ref)
			if err != nil {
				return false
			}
			// S1b-W2 dual direct-write, mixed chroma (same shape as
			// luma above, stride 8).
			predictChromaBlockInto(rp0.Cb, rw, rh, ccx, ccy, cw, ch, dm[0].mx, dm[0].my, predCb[cy*8+cx:], 8)
			predictChromaBlockInto(rp0.Cr, rw, rh, ccx, ccy, cw, ch, dm[0].mx, dm[0].my, predCr[cy*8+cx:], 8)
			predictChromaBlock(rp1.Cb, rw, rh, ccx, ccy, cw, ch, dm[1].mx, dm[1].my, cb1[:16])
			predictChromaBlock(rp1.Cr, rw, rh, ccx, ccy, cw, ch, dm[1].mx, dm[1].my, cr1[:16])
			w := d.bipredWeight(d.pic, rp0, rp1, dm[0].ref, dm[1].ref)
			for y := 0; y < 4; y++ {
				bipredAvg(predCb[(cy+y)*8+cx:(cy+y)*8+cx+4], cb1[y*4:(y+1)*4], w)
				bipredAvg(predCr[(cy+y)*8+cx:(cy+y)*8+cx+4], cr1[y*4:(y+1)*4], w)
			}
			// Already mixed in place at its final address: skip the
			// shared trailing copy below (it moves cb/cr, which this
			// lane never filled, and would clobber the result).
			// continue here targets the part loop, not the switch.
			continue
		case dm[0].use:
			rp, err := d.refFor(dm[0].ref)
			if err != nil {
				return false
			}
			predictChromaBlock(rp.Cb, rw, rh, ccx, ccy, cw, ch, dm[0].mx, dm[0].my, cb[:16])
			predictChromaBlock(rp.Cr, rw, rh, ccx, ccy, cw, ch, dm[0].mx, dm[0].my, cr[:16])
			d.weightChroma(cb[:16], cr[:16], dm[0].ref)
		case dm[1].use:
			rp, err := d.refFor1(dm[1].ref)
			if err != nil {
				return false
			}
			predictChromaBlock(rp.Cb, rw, rh, ccx, ccy, cw, ch, dm[1].mx, dm[1].my, cb[:16])
			predictChromaBlock(rp.Cr, rw, rh, ccx, ccy, cw, ch, dm[1].mx, dm[1].my, cr[:16])
			d.weightChroma1(cb[:16], cr[:16], dm[1].ref)
		default:
			for k := range cb[:16] {
				cb[k], cr[k] = 128, 128
			}
		}
		for y := 0; y < 4; y++ {
			copy(predCb[(cy+y)*8+cx:(cy+y)*8+cx+4], cb[y*4:(y+1)*4])
			copy(predCr[(cy+y)*8+cx:(cy+y)*8+cx+4], cr[y*4:(y+1)*4])
		}
	}
	d.finishPMB(h, addr, mbx, mby)
	d.skipped[addr] = true
	d.mbDirect[addr] = true
	d.mbSlice[addr] = d.slices
	if addr >= 0 && addr < len(d.mbT8) {
		d.mbT8[addr] = false
	}
	if err := d.reconstructInterWith(mbx, mby, 0, &predY, &predCb, &predCr, rs); err != nil {
		return false
	}
	d.skipCnt++
	return true
}

// skipBUniform is the uniform-motion lane: all four 8x8 parts share dm
// (same use flags, refs, vectors — still content, so the four 8x8
// predictions tile one 16x16 prediction). Reference lookups and the
// implicit weight resolve once; prediction runs at 16x16 (one call per
// plane-pair instead of four), then slices into the MB buffers. Output
// equals four separate 8x8 predictions (same function, adjacent
// offsets, no overlap), verified by the skip uniform gate.
func (d *Decoder) skipBUniform(h *SliceHeader, addr, mbx, mby int, dm [2]bDirectMV, u0, u1 bool, px0, py0 int, rs residSrc) bool {
	var predY [256]uint8
	var predCb, predCr [64]uint8
	rw, rh := int(d.pic.Width/2), int(d.pic.Height/2)
	switch {
	case u0 && u1:
		rp0, err := d.refFor(dm[0].ref)
		if err != nil {
			return false
		}
		rp1, err := d.refFor1(dm[1].ref)
		if err != nil {
			return false
		}
		var blk1 [256]uint8
		predictLumaBlock(rp0, px0, py0, 16, 16, dm[0].mx, dm[0].my, predY[:])
		predictLumaBlock(rp1, px0, py0, 16, 16, dm[1].mx, dm[1].my, blk1[:])
		w := d.bipredWeight(d.pic, rp0, rp1, dm[0].ref, dm[1].ref)
		bipredAvg(predY[:], blk1[:], w)
		ccx, ccy := mbx*8, mby*8
		var cb1, cr1 [64]uint8
		predictChromaBlock(rp0.Cb, rw, rh, ccx, ccy, 8, 8, dm[0].mx, dm[0].my, predCb[:])
		predictChromaBlock(rp0.Cr, rw, rh, ccx, ccy, 8, 8, dm[0].mx, dm[0].my, predCr[:])
		predictChromaBlock(rp1.Cb, rw, rh, ccx, ccy, 8, 8, dm[1].mx, dm[1].my, cb1[:])
		predictChromaBlock(rp1.Cr, rw, rh, ccx, ccy, 8, 8, dm[1].mx, dm[1].my, cr1[:])
		bipredAvg(predCb[:], cb1[:], w)
		bipredAvg(predCr[:], cr1[:], w)
	case u0:
		rp, err := d.refFor(dm[0].ref)
		if err != nil {
			return false
		}
		predictLumaBlock(rp, px0, py0, 16, 16, dm[0].mx, dm[0].my, predY[:])
		d.weightLuma(predY[:], dm[0].ref)
		ccx, ccy := mbx*8, mby*8
		predictChromaBlock(rp.Cb, rw, rh, ccx, ccy, 8, 8, dm[0].mx, dm[0].my, predCb[:])
		predictChromaBlock(rp.Cr, rw, rh, ccx, ccy, 8, 8, dm[0].mx, dm[0].my, predCr[:])
		d.weightChroma(predCb[:], predCr[:], dm[0].ref)
	case u1:
		rp, err := d.refFor1(dm[1].ref)
		if err != nil {
			return false
		}
		predictLumaBlock(rp, px0, py0, 16, 16, dm[1].mx, dm[1].my, predY[:])
		d.weightLuma1(predY[:], dm[1].ref)
		ccx, ccy := mbx*8, mby*8
		predictChromaBlock(rp.Cb, rw, rh, ccx, ccy, 8, 8, dm[1].mx, dm[1].my, predCb[:])
		predictChromaBlock(rp.Cr, rw, rh, ccx, ccy, 8, 8, dm[1].mx, dm[1].my, predCr[:])
		d.weightChroma1(predCb[:], predCr[:], dm[1].ref)
	default:
		fillPred(&predY, &predCb, &predCr)
	}
	d.finishPMB(h, addr, mbx, mby)
	d.skipped[addr] = true
	d.mbDirect[addr] = true
	d.mbSlice[addr] = d.slices
	if addr >= 0 && addr < len(d.mbT8) {
		d.mbT8[addr] = false
	}
	// S1b-E: all four 8x8 parts share dm (checked by the caller), so
	// all 16 4x4 motion slots are identical — record it for the
	// deblocker's uniform-edge shortcut (cbp==0 below zeroes nnz, so
	// the block is also clean by construction).
	if addr >= 0 && addr < len(d.uniSkip) {
		d.uniSkip[addr] = true
		d.uniDM[addr] = dm
	}
	if err := d.reconstructInterWith(mbx, mby, 0, &predY, &predCb, &predCr, rs); err != nil {
		return false
	}
	d.skipCnt++
	return true
}

// storeDirectB8 records one 8x8 direct sub-block's motion (2x2 slots at
// 4x4 origin (qx, qy)), including early scratch so same-MB CABAC
// reference contexts read the derived indices.
// storeDirectB8 records one 8x8 direct sub-block's motion (2x2 slots at
// 4x4 origin (qx, qy)), including early scratch so same-MB CABAC
// reference contexts read the derived indices.
//
// S1b-M loop shape (values bit-identical to the old y/x loop): the two
// list-use flags are block-invariant, so the per-slot branches hoist
// out of the slot loop; the four slot indices unroll from one base
// (the old inner recompute of (qy+y)*stride+qx+x per store is gone,
// and refTmp reuses the same slot index instead of recomputing it);
// the two useM read-modify-writes fuse into one OR (sequential |= of
// disjoint bits equals one |= of their union; |=0 is a no-op so the
// neither-list case stores the same value). No pixel writes: motion
// bookkeeping only, so the win cannot drift the picture (B-direct
// pixel locks in bdirect/bframes tests pin the downstream effect).
func (d *Decoder) storeDirectB8(qx, qy int, dm [2]bDirectMV) {
	stride := d.mbW * 4
	base := qy*stride + qx
	u0, u1 := dm[0].use, dm[1].use
	mx0, my0, rf0 := dm[0].mx, dm[0].my, dm[0].ref
	mx1, my1, rf1 := dm[1].mx, dm[1].my, dm[1].ref
	var bits uint8
	if u0 {
		bits |= useL0
	}
	if u1 {
		bits |= useL1
	}
	// Unrolled 2x2 slots (offsets fixed: 0, 1, stride, stride+1).
	i0 := base
	i1 := base + 1
	i2 := base + stride
	i3 := base + stride + 1
	if u0 && u1 {
		d.mvX[i0], d.mvY[i0] = mx0, my0
		d.mvdX[i0], d.mvdY[i0] = 0, 0
		d.refIdx[i0] = rf0
		d.refTmp[i0] = rf0
		d.mvX1[i0], d.mvY1[i0] = mx1, my1
		d.mvdX1[i0], d.mvdY1[i0] = 0, 0
		d.refIdx1[i0] = rf1
		d.refTmp1[i0] = rf1
		d.useM[i0] |= bits
		d.direct4[i0] = true
		d.mvX[i1], d.mvY[i1] = mx0, my0
		d.mvdX[i1], d.mvdY[i1] = 0, 0
		d.refIdx[i1] = rf0
		d.refTmp[i1] = rf0
		d.mvX1[i1], d.mvY1[i1] = mx1, my1
		d.mvdX1[i1], d.mvdY1[i1] = 0, 0
		d.refIdx1[i1] = rf1
		d.refTmp1[i1] = rf1
		d.useM[i1] |= bits
		d.direct4[i1] = true
		d.mvX[i2], d.mvY[i2] = mx0, my0
		d.mvdX[i2], d.mvdY[i2] = 0, 0
		d.refIdx[i2] = rf0
		d.refTmp[i2] = rf0
		d.mvX1[i2], d.mvY1[i2] = mx1, my1
		d.mvdX1[i2], d.mvdY1[i2] = 0, 0
		d.refIdx1[i2] = rf1
		d.refTmp1[i2] = rf1
		d.useM[i2] |= bits
		d.direct4[i2] = true
		d.mvX[i3], d.mvY[i3] = mx0, my0
		d.mvdX[i3], d.mvdY[i3] = 0, 0
		d.refIdx[i3] = rf0
		d.refTmp[i3] = rf0
		d.mvX1[i3], d.mvY1[i3] = mx1, my1
		d.mvdX1[i3], d.mvdY1[i3] = 0, 0
		d.refIdx1[i3] = rf1
		d.refTmp1[i3] = rf1
		d.useM[i3] |= bits
		d.direct4[i3] = true
		return
	}
	// Single-list (or neither) lanes keep the exact old semantics:
	// used list stores motion+ref+scratch, unused list zeroes mv/mvd
	// only (ref/useM untouched), useM ORs once, direct4 always set.
	for _, i := range [4]int{i0, i1, i2, i3} {
		if u0 {
			d.mvX[i], d.mvY[i] = mx0, my0
			d.mvdX[i], d.mvdY[i] = 0, 0
			d.refIdx[i] = rf0
			d.refTmp[i] = rf0
		} else {
			d.mvX[i], d.mvY[i] = 0, 0
			d.mvdX[i], d.mvdY[i] = 0, 0
		}
		if u1 {
			d.mvX1[i], d.mvY1[i] = mx1, my1
			d.mvdX1[i], d.mvdY1[i] = 0, 0
			d.refIdx1[i] = rf1
			d.refTmp1[i] = rf1
		} else {
			d.mvX1[i], d.mvY1[i] = 0, 0
			d.mvdX1[i], d.mvdY1[i] = 0, 0
		}
		if bits != 0 {
			d.useM[i] |= bits
		}
		d.direct4[i] = true
	}
}

// directPartB8 packs one 8x8 direct sub-block into an 8x8 part.
func directPartB8(ox, oy int, dm [2]bDirectMV) part {
	p := part{px: ox, py: oy, w: 8, h: 8}
	if dm[0].use {
		p.mx, p.my, p.ref = dm[0].mx, dm[0].my, dm[0].ref
	}
	if dm[1].use {
		p.mx1, p.my1, p.ref1 = dm[1].mx, dm[1].my, dm[1].ref
	}
	p.use0, p.use1 = dm[0].use, dm[1].use
	return p
}

// decodeMBB is the CAVLC B-slice macroblock entry: skip runs land in the
// caller; here 0..22 are inter, 23..48 the intra escape (48 = PCM).
func (d *Decoder) decodeMBB(r *Reader, pps *PPS, h *SliceHeader, addr int) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	mbType, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("mb type: %w", err)
	}
	if mbType > 48 {
		return fmt.Errorf("%w: mb type %d in B slice", ErrBadSliceHeader, mbType)
	}
	if mbType >= 23 {
		intraType := mbType - 23
		if intraType == 25 {
			return d.decodePCM(r, mbx, mby)
		}
		return d.decodeIntraMB(r, pps, h, addr, mbx, mby, intraType)
	}
	return d.decodeMBBParts(h, pps, addr, mbx, mby, mbType, d.cavlcBInterSrc(r, h, pps), d.cavlcSrc(r))
}

// decodeMBBParts decodes one explicit B macroblock: references of both
// lists first (list 0, then list 1, in partition order), motion
// differences after, then CBP and residual like P.
func (d *Decoder) decodeMBBParts(h *SliceHeader, pps *PPS, addr, mbx, mby int, mbType uint32, is bInterSrc, rs residSrc) error {
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	d.poisonDiagSlots(mbx, mby)
	// S1b-J: same pre-size as decodeMBPParts (growslice memmove).
	parts := make([]part, 0, 4)
	var subList []uint32
	hasSubs := false
	allDirect := false
	switch {
	case mbType == 0:
		if len(d.refList1) == 0 || d.refList1[0] == nil {
			return fmt.Errorf("%w: B direct without list-1 picture", ErrBadSliceHeader)
		}
		dms, err := d.bDirectMBBlocks(mbx, mby, h, d.refList1[0])
		if err != nil {
			return err
		}
		for i := 0; i < 4; i++ {
			qx := x0 + (i%2)*2
			qy := y0 + (i/2)*2
			d.storeDirectB8(qx, qy, dms[i])
			parts = append(parts, directPartB8(px0+(i%2)*8, py0+(i/2)*8, dms[i]))
		}
		allDirect = true
	case mbType <= 3:
		dir := bMBDirs[mbType][0]
		ds := bPartDesc{qx: x0, qy: y0, w4: 4, h4: 4, dir: dir}
		refs, err := d.decodeBRefs(is, []bPartDesc{ds})
		if err != nil {
			return err
		}
		diffs, err := d.decodeBMVDs(is, []bPartDesc{ds})
		if err != nil {
			return err
		}
		pt, err := d.decodeBPartMVD(refs, diffs, 0, ds, bk16, px0, py0)
		if err != nil {
			return err
		}
		parts = append(parts, pt)
	case mbType <= 21:
		dirs := bMBDirs[mbType]
		wide := mbType%2 == 0 // even 16x8 halves, odd 8x16 halves
		kind := bk16x8
		var descs []bPartDesc
		if wide {
			kind = bk16x8
			descs = []bPartDesc{
				{qx: x0, qy: y0, w4: 4, h4: 2, dir: dirs[0]},
				{qx: x0, qy: y0 + 2, w4: 4, h4: 2, dir: dirs[1]},
			}
		} else {
			kind = bk8x16
			descs = []bPartDesc{
				{qx: x0, qy: y0, w4: 2, h4: 4, dir: dirs[0]},
				{qx: x0 + 2, qy: y0, w4: 2, h4: 4, dir: dirs[1]},
			}
		}
		refs, err := d.decodeBRefs(is, descs)
		if err != nil {
			return err
		}
		diffs, err := d.decodeBMVDs(is, descs)
		if err != nil {
			return err
		}
		for i, ds := range descs {
			px, py := px0, py0
			if wide {
				py += i * 8
			} else {
				px += i * 8
			}
			pt, err := d.decodeBPartMVD(refs, diffs, i, ds, kind, px, py)
			if err != nil {
				return err
			}
			parts = append(parts, pt)
		}
	default: // 22: B_8x8
		var subs [4]uint32
		for i := 0; i < 4; i++ {
			v, err := is.sub()
			if err != nil {
				return err
			}
			subs[i] = v
		}
		hasSubs = true
		subList = subs[:]
		var descs []bPartDesc
		var kinds []bKind
		var poses [][2]int
		b8direct := true
		for i := 0; i < 4; i++ {
			dir, shape, err := bSubDirShape(subs[i])
			if err != nil {
				return err
			}
			if dir != bDirect {
				b8direct = false
				bx8, by8 := i%2, i/2
				qx, qy := x0+bx8*2, y0+by8*2
				ox, oy := px0+bx8*8, py0+by8*8
				switch shape {
				case 0: // 8x8
					descs = append(descs, bPartDesc{qx: qx, qy: qy, w4: 2, h4: 2, n: 4 * i, dir: dir})
					kinds = append(kinds, bkSub)
					poses = append(poses, [2]int{ox, oy})
				case 1: // 8x4 pair
					for k := 0; k < 2; k++ {
						descs = append(descs, bPartDesc{qx: qx, qy: qy + k, w4: 2, h4: 1, n: 4*i + 2*k, dir: dir})
						kinds = append(kinds, bkSub)
						poses = append(poses, [2]int{ox, oy + k*4})
					}
				case 2: // 4x8 pair
					for k := 0; k < 2; k++ {
						descs = append(descs, bPartDesc{qx: qx + k, qy: qy, w4: 1, h4: 2, n: 4*i + k, dir: dir})
						kinds = append(kinds, bkSub)
						poses = append(poses, [2]int{ox + k*4, oy})
					}
				default: // 4x4 quad
					for k := 0; k < 4; k++ {
						descs = append(descs, bPartDesc{qx: qx + (k % 2), qy: qy + (k / 2), w4: 1, h4: 1, n: 4*i + k, dir: dir})
						kinds = append(kinds, bkSub)
						poses = append(poses, [2]int{ox + (k%2)*4, oy + (k/2)*4})
					}
				}
			}
		}
		// Direct sub-blocks derive and store first: later explicit
		// partitions predict from their motion, mirroring the reference
		// order (macroblock-level derivation before any explicit
		// decode). They share one macroblock-level derivation (plus
		// each sub-block's own stationary check), not per-sub-block
		// neighbours.
		var mbDM [2]bDirectMV
		mbDMDone := false
		for i := 0; i < 4; i++ {
			dir, _, err := bSubDirShape(subs[i])
			if err != nil {
				return err
			}
			if dir != bDirect {
				continue
			}
			bx8, by8 := i%2, i/2
			qx, qy := x0+bx8*2, y0+by8*2
			ox, oy := px0+bx8*8, py0+by8*8
			// Temporal direct has no macroblock-level derivation: each
			// 8x8 scales its own colocated block (bTempDirect8), so it
			// derives per sub-block here instead of sharing mbDM.
			if !h.DirectSpatial {
				if len(d.refList1) == 0 || d.refList1[0] == nil {
					return fmt.Errorf("%w: B direct without list-1 picture", ErrBadSliceHeader)
				}
				dm, err := d.bTempDirect8(qx, qy, h, d.refList1[0])
				if err != nil {
					return err
				}
				d.storeDirectB8(qx, qy, dm)
				parts = append(parts, directPartB8(ox, oy, dm))
				continue
			}
			if !mbDMDone {
				if len(d.refList1) == 0 || d.refList1[0] == nil {
					return fmt.Errorf("%w: B direct without list-1 picture", ErrBadSliceHeader)
				}
				mbDM, err = d.bDirectMBLevel(mbx, mby, h)
				if err != nil {
					return err
				}
				mbDMDone = true
			}
			dm := d.bDirectB8(mbDM, qx, qy, d.refList1[0])
			d.storeDirectB8(qx, qy, dm)
			parts = append(parts, directPartB8(ox, oy, dm))
		}
		refs, err := d.decodeBRefs(is, descs)
		if err != nil {
			return err
		}
		diffs, err := d.decodeBMVDs(is, descs)
		if err != nil {
			return err
		}
		for i, ds := range descs {
			pt, err := d.decodeBPartMVD(refs, diffs, i, ds, kinds[i], poses[i][0], poses[i][1])
			if err != nil {
				return err
			}
			parts = append(parts, pt)
		}
		allDirect = b8direct
	}
	// Motion compensation into full-MB prediction buffers.
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	if err := d.mcParts(mbx, mby, parts, &predY, &predCb, &predCr); err != nil {
		return err
	}
	// CBP + 8x8 flag + QP delta + residual (same order as P).
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
	// B type contexts read directness: skip, whole-direct and
	// all-direct-subblock B_8x8 count; anything with an explicit
	// partition does not (matches the reference DIRECT2 bit).
	d.mbDirect[addr] = allDirect || mbType == 0
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

// bPartDesc is one B partition's geometry in 4x4 units (origin, size)
// plus its direction. Reference reading consumes a desc list; motion
// decoding consumes the same list, so origins stay identical for CABAC
// neighbour contexts. n is the grouped 4x4 index for diagonal prediction.
type bPartDesc struct {
	qx, qy, w4, h4 int
	n              int
	dir            bDir
}

// bRefPair holds one partition's reference indices per used list.
type bRefPair struct {
	r0, r1 int8
}

// bKind selects the motion-prediction shape rule for one partition.
type bKind uint8

const (
	bk16 bKind = iota
	bk16x8
	bk8x16
	bkSub
)

// decodeBRefs reads both lists' references for ordered partitions and
// stores them into early scratch at once (later same-MB partitions read
// them for neighbour contexts): every list-0 reference first, then
// every list-1 reference.
func (d *Decoder) decodeBRefs(is bInterSrc, descs []bPartDesc) ([]bRefPair, error) {
	refs := make([]bRefPair, len(descs))
	for i, ds := range descs {
		if !ds.dir.usesL0() {
			continue
		}
		r, err := is.ref0(ds.qx, ds.qy)
		if err != nil {
			return nil, err
		}
		refs[i].r0 = r
		d.storeRef(ds.qx, ds.qy, ds.w4, ds.h4, r)
	}
	for i, ds := range descs {
		if !ds.dir.usesL1() {
			continue
		}
		r, err := is.ref1(ds.qx, ds.qy)
		if err != nil {
			return nil, err
		}
		refs[i].r1 = r
		d.storeRef1(ds.qx, ds.qy, ds.w4, ds.h4, r)
	}
	return refs, nil
}

// bMVDDiff holds one partition's decoded differences per used list.
type bMVDDiff struct {
	dx0, dy0, dx1, dy1 int16
}

// decodeBMVDs reads every partition's motion differences list-outer
// (every list-0 difference, then every list-1 difference, each in
// partition order): the bitstream groups differences by list, so mixed-
// direction blocks must not read partition-by-partition. Differences
// land in the raw MVD slots at read time, so later same-list partitions
// see them for neighbour contexts; prediction still runs in partition
// order afterwards.
func (d *Decoder) decodeBMVDs(is bInterSrc, descs []bPartDesc) ([]bMVDDiff, error) {
	out := make([]bMVDDiff, len(descs))
	for i, ds := range descs {
		if !ds.dir.usesL0() {
			continue
		}
		_, _, dx, dy, err := is.mvd(0, ds.qx, ds.qy, 0, 0)
		if err != nil {
			return nil, err
		}
		out[i].dx0, out[i].dy0 = dx, dy
		d.storeBMVD(ds, 0, dx, dy)
	}
	for i, ds := range descs {
		if !ds.dir.usesL1() {
			continue
		}
		_, _, dx, dy, err := is.mvd(1, ds.qx, ds.qy, 0, 0)
		if err != nil {
			return nil, err
		}
		out[i].dx1, out[i].dy1 = dx, dy
		d.storeBMVD(ds, 1, dx, dy)
	}
	return out, nil
}

// storeBMVD records one partition's decoded differences into the raw
// MVD slots of one list at read time (motion vectors follow later, in
// partition order, and overwrite nothing here).
func (d *Decoder) storeBMVD(ds bPartDesc, list int, dx, dy int16) {
	stride := d.mbW * 4
	for y := ds.qy; y < ds.qy+ds.h4; y++ {
		for x := ds.qx; x < ds.qx+ds.w4; x++ {
			i := y*stride + x
			if list == 0 {
				d.mvdX[i], d.mvdY[i] = dx, dy
			} else {
				d.mvdX1[i], d.mvdY1[i] = dx, dy
			}
		}
	}
}

// decodeBPartMVD predicts and stores one B partition whose references
// and differences are already read: motion prediction per used list,
// then stores.
func (d *Decoder) decodeBPartMVD(refs []bRefPair, diffs []bMVDDiff, pi int, ds bPartDesc, kind bKind, px, py int) (part, error) {
	var pt part
	pt.px, pt.py, pt.w, pt.h = px, py, ds.w4*4, ds.h4*4
	if ds.dir.usesL0() {
		ref := refs[pi].r0
		pvx, pvy := d.predBPart(0, kind, pi, ds, ref)
		mx, my := pvx+diffs[pi].dx0, pvy+diffs[pi].dy0
		d.storeMV(px, py, ds.w4*4, ds.h4*4, mx, my, diffs[pi].dx0, diffs[pi].dy0, ref)
		pt.mx, pt.my, pt.ref = mx, my, ref
		pt.use0 = true
	} else {
		d.zeroMVL(px, py, ds.w4*4, ds.h4*4, 0)
	}
	if ds.dir.usesL1() {
		ref := refs[pi].r1
		pvx, pvy := d.predBPart(1, kind, pi, ds, ref)
		mx, my := pvx+diffs[pi].dx1, pvy+diffs[pi].dy1
		d.storeMV1(px, py, ds.w4*4, ds.h4*4, mx, my, diffs[pi].dx1, diffs[pi].dy1, ref)
		pt.mx1, pt.my1, pt.ref1 = mx, my, ref
		pt.use1 = true
	} else {
		d.zeroMVL(px, py, ds.w4*4, ds.h4*4, 1)
	}
	return pt, nil
}

// predBPart predicts one B partition from one list by shape kind.
func (d *Decoder) predBPart(list int, kind bKind, pi int, ds bPartDesc, ref int8) (int16, int16) {
	qx, qy := ds.qx, ds.qy
	switch kind {
	case bk16:
		return d.predMotionL(list, 0, qx, qy, 4, ref)
	case bk16x8:
		if pi == 0 {
			return d.predB16x8(list, true, qx, qy, ref)
		}
		return d.predB16x8(list, false, qx, qy, ref)
	case bk8x16:
		if pi == 0 {
			return d.predB8x16(list, true, qx, qy, ref)
		}
		return d.predB8x16(list, false, qx, qy, ref)
	default:
		return d.predMotionL(list, ds.n, qx, qy, ds.w4, ref)
	}
}

// predB16x8 predicts one 16x8 half from one list, mirroring the P rules.
func (d *Decoder) predB16x8(list int, top bool, x0, y0 int, ref int8) (int16, int16) {
	if top {
		if _, _, br := d.mvNeighbourL(list, x0, y0-1); br == ref {
			mx, my, _ := d.mvNeighbourL(list, x0, y0-1)
			return mx, my
		}
		return d.predMotionL(list, 0, x0, y0, 4, ref)
	}
	if _, _, ar := d.mvNeighbourL(list, x0-1, y0); ar == ref {
		mx, my, _ := d.mvNeighbourL(list, x0-1, y0)
		return mx, my
	}
	return d.predMotionL(list, 8, x0, y0, 4, ref)
}

// predB8x16 predicts one 8x16 half from one list, mirroring the P rules.
func (d *Decoder) predB8x16(list int, left bool, px, py int, ref int8) (int16, int16) {
	if left {
		if _, _, ar := d.mvNeighbourL(list, px-1, py); ar == ref {
			mx, my, _ := d.mvNeighbourL(list, px-1, py)
			return mx, my
		}
		return d.predMotionL(list, 0, px, py, 2, ref)
	}
	// Right half: directional neighbour is C (above-right) with D
	// standing in when C is unavailable (spec 8.4.1.3.2, same fix as
	// the P-side pred8x16Right).
	if mx, my, cr := d.diagNeighbourL(list, 4, 2, px, py); true {
		if cr == partNotAvailable {
			mx, my, cr = d.topLeftNeighbourL(list, px, py)
		}
		if cr == ref {
			return mx, my
		}
	}
	return d.predMotionL(list, 4, px, py, 2, ref)
}
