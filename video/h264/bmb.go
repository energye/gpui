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
type bInterSrc struct {
	ref0 func(bx, by int) (int8, error)
	ref1 func(bx, by int) (int8, error)
	mvd  func(list, bx, by int, px, py int16) (mx, my, dx, dy int16, err error)
	sub  func() (uint32, error)
	cbp  func() (uint32, error)
	qpD  func() (int32, error)
	t8   func(cbp uint32, subs []uint32) (bool, error)
}

// bRefReader builds one list's CAVLC reference reader: no code for a
// single reference, one inverted bit for two, Exp-Golomb above that.
func bRefReader(r *Reader, h *SliceHeader, list int) func(bx, by int) (int8, error) {
	return func(bx, by int) (int8, error) {
		var n uint32
		if list == 0 {
			n = h.RefL0Count
		} else {
			n = h.RefL1Count
		}
		if n <= 1 {
			return 0, nil
		}
		if n == 2 {
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
		if v >= n {
			return 0, fmt.Errorf("%w: B ref l%d idx %d", ErrBadSliceHeader, list, v)
		}
		return int8(v), nil
	}
}

// cavlcBInterSrc reads B inter syntax with Exp-Golomb codes.
func (d *Decoder) cavlcBInterSrc(r *Reader, h *SliceHeader, pps *PPS) *bInterSrc {
	readMVD := func(list, bx, by int, predX, predY int16) (int16, int16, int16, int16, error) {
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
	return &bInterSrc{
		ref0: bRefReader(r, h, 0),
		ref1: bRefReader(r, h, 1),
		mvd:  readMVD,
		sub: func() (uint32, error) {
			v, err := r.ReadUE()
			if err != nil {
				return 0, fmt.Errorf("B sub type: %w", err)
			}
			if v > 12 {
				return 0, fmt.Errorf("%w: B sub type %d", ErrBadSliceHeader, v)
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

// decodeSkipB decodes a B_Skip macroblock: spatial-direct motion, no
// residual. Every 4x4 records the derived direction for neighbours,
// deblocking and CABAC contexts.
func (d *Decoder) decodeSkipB(h *SliceHeader, addr int, rs *residSrc) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	if len(d.refList1) == 0 || d.refList1[0] == nil {
		return fmt.Errorf("%w: B skip without list-1 picture", ErrBadSliceHeader)
	}
	dm, err := d.bDirectMB(mbx, mby, h, d.refList1[0])
	if err != nil {
		return err
	}
	d.storeDirectB(mbx, mby, dm)
	var predY [256]uint8
	var predCb, predCr [64]uint8
	fillPred(&predY, &predCb, &predCr)
	parts := [1]part{directPart(mbx, mby, dm)}
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

// directPart packs one 16x16 direct block's motion into a part.
func directPart(mbx, mby int, dm [2]bDirectMV) part {
	p := part{px: mbx * 16, py: mby * 16, w: 16, h: 16}
	if dm[0].use {
		p.mx, p.my, p.ref = dm[0].mx, dm[0].my, dm[0].ref
	}
	if dm[1].use {
		p.mx1, p.my1, p.ref1 = dm[1].mx, dm[1].my, dm[1].ref
	}
	p.use0, p.use1 = dm[0].use, dm[1].use
	return p
}

// storeDirectB records one 16x16 direct block's motion into every 4x4.
func (d *Decoder) storeDirectB(mbx, mby int, dm [2]bDirectMV) {
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			i := (mby*4+y)*stride + mbx*4 + x
			if dm[0].use {
				d.mvX[i], d.mvY[i] = dm[0].mx, dm[0].my
				d.mvdX[i], d.mvdY[i] = 0, 0
				d.refIdx[i] = dm[0].ref
				d.useM[i] |= useL0
			} else {
				d.mvX[i], d.mvY[i] = 0, 0
				d.mvdX[i], d.mvdY[i] = 0, 0
			}
			if dm[1].use {
				d.mvX1[i], d.mvY1[i] = dm[1].mx, dm[1].my
				d.mvdX1[i], d.mvdY1[i] = 0, 0
				d.refIdx1[i] = dm[1].ref
				d.useM[i] |= useL1
			} else {
				d.mvX1[i], d.mvY1[i] = 0, 0
				d.mvdX1[i], d.mvdY1[i] = 0, 0
			}
			d.direct4[i] = true
		}
	}
}

// storeDirectB8 records one 8x8 direct sub-block's motion (2x2 slots at
// 4x4 origin (qx, qy)), including early scratch so same-MB CABAC
// reference contexts read the derived indices.
func (d *Decoder) storeDirectB8(qx, qy int, dm [2]bDirectMV) {
	stride := d.mbW * 4
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			i := (qy+y)*stride + qx + x
			if dm[0].use {
				d.mvX[i], d.mvY[i] = dm[0].mx, dm[0].my
				d.mvdX[i], d.mvdY[i] = 0, 0
				d.refIdx[i] = dm[0].ref
				d.refTmp[(qy+y)*stride+qx+x] = dm[0].ref
				d.useM[i] |= useL0
			} else {
				d.mvX[i], d.mvY[i] = 0, 0
				d.mvdX[i], d.mvdY[i] = 0, 0
			}
			if dm[1].use {
				d.mvX1[i], d.mvY1[i] = dm[1].mx, dm[1].my
				d.mvdX1[i], d.mvdY1[i] = 0, 0
				d.refIdx1[i] = dm[1].ref
				d.refTmp1[(qy+y)*stride+qx+x] = dm[1].ref
				d.useM[i] |= useL1
			} else {
				d.mvX1[i], d.mvY1[i] = 0, 0
				d.mvdX1[i], d.mvdY1[i] = 0, 0
			}
			d.direct4[i] = true
		}
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
func (d *Decoder) decodeMBBParts(h *SliceHeader, pps *PPS, addr, mbx, mby int, mbType uint32, is *bInterSrc, rs *residSrc) error {
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	d.poisonDiagSlots(mbx, mby)
	var parts []part
	var subList []uint32
	hasSubs := false
	allDirect := false
	switch {
	case mbType == 0:
		if len(d.refList1) == 0 || d.refList1[0] == nil {
			return fmt.Errorf("%w: B direct without list-1 picture", ErrBadSliceHeader)
		}
		dm, err := d.bDirectMB(mbx, mby, h, d.refList1[0])
		if err != nil {
			return err
		}
		d.storeDirectB(mbx, mby, dm)
		parts = append(parts, directPart(mbx, mby, dm))
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
			bx8, by8 := i%2, i/2
			qx, qy := x0+bx8*2, y0+by8*2
			ox, oy := px0+bx8*8, py0+by8*8
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
func (d *Decoder) decodeBRefs(is *bInterSrc, descs []bPartDesc) ([]bRefPair, error) {
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
func (d *Decoder) decodeBMVDs(is *bInterSrc, descs []bPartDesc) ([]bMVDDiff, error) {
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
	if mx, my, cr := d.mvNeighbourL(list, px+2, py-1); cr == ref {
		return mx, my
	}
	return d.predMotionL(list, 4, px, py, 2, ref)
}
