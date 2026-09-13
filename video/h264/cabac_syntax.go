package h264

import "fmt"

// CABAC slice wiring and per-MB syntax (Main/High entropy path).
// Context arithmetic lives in cabac.go, numeric tables in
// cabac_tables.go; this file only walks macroblocks the way the
// bitstream orders them. Element binarizations follow H.264 §9.3.

// cabacInitSlice byte-aligns the reader past the slice header, starts
// the arithmetic engine on the payload tail and seeds the 1024 packed
// contexts for this slice type.
func (d *Decoder) cabacInitSlice(h *SliceHeader, r *Reader, pps *PPS) error {
	if h.Type == SliceSP || h.Type == SliceSI {
		return fmt.Errorf("%w: cabac SP/SI slice", ErrStageScope)
	}
	r.AlignToByte()
	d.cabByte0 = r.pos / 8
	tail := append(append([]byte(nil), r.data[d.cabByte0:]...), make([]byte, 16)...)
	cab, err := newCabacDec(tail)
	if err != nil {
		return err
	}
	d.cab = cab
	tab := cabacInitI[:]
	if !h.IsI() {
		tab = cabacInitPB[int(h.CabacInitIDC)*2048 : (int(h.CabacInitIDC)+1)*2048]
	}
	d.sliceQP = d.qpY
	// No delta precedes the first macroblock.
	d.lastQPD = 0
	initCabacCtx(&d.cabCtx, tab, d.sliceQP)
	return nil
}

// cabSameSlice reports whether addr belongs to the slice being decoded.
func (d *Decoder) cabSameSlice(addr int) bool {
	return addr >= 0 && addr < len(d.mbSlice) && d.mbSlice[addr] == d.slices
}

// cabLeftTop returns same-slice left/top macroblock addresses
// (-1 where the neighbour is missing or from another slice).
func (d *Decoder) cabLeftTop(addr, mbx, mby int) (int, int) {
	left, top := -1, -1
	if mbx > 0 && d.cabSameSlice(addr-1) {
		left = addr - 1
	}
	if mby > 0 && d.cabSameSlice(addr-d.mbW) {
		top = addr - d.mbW
	}
	return left, top
}

// cabBin reads one bin with context ctx.
func (d *Decoder) cabBin(ctx uint16) int {
	return d.cab.bin(&d.cabCtx[ctx])
}

// decodeSliceCabac walks one CABAC slice: per-MB skip/type bins, shared
// partition cores, and one end-of-slice flag after every macroblock.
func (d *Decoder) decodeSliceCabac(h *SliceHeader, pps *PPS, r *Reader, addr, total int) error {
	for addr < total {
		var err error
		if h.IsI() {
			err = d.decodeMBCabac(h, r, pps, addr)
		} else if h.IsB() {
			err = d.decodeMBBCabac(h, r, pps, addr)
		} else {
			err = d.decodeMBPCabac(h, r, pps, addr)
		}
		if err != nil {
			return fmt.Errorf("mb %d: %w", addr, err)
		}
		d.decoded++
		addr++
		if d.cab.terminate() != 0 {
			break
		}
	}
	d.slices++
	return nil
}

func (d *Decoder) decodeMBCabac(h *SliceHeader, r *Reader, pps *PPS, addr int) error {
	// lastQPD carries over from the previous block until this one
	// decodes (or skips) its own delta. Mark slice ownership early:
	// same-block neighbours are already decoded mid-block.
	d.lastQPDHit = false
	d.mbSlice[addr] = d.slices
	mbx, mby := addr%d.mbW, addr/d.mbW
	mbType, err := d.cabacMBTypeI(mbx, mby, addr)
	if err != nil {
		return err
	}
	if mbType == 25 {
		return d.cabacDecodePCM(r, mbx, mby)
	}
	err = d.decodeIntraMBCore(h, pps, addr, mbx, mby, mbType, d.cabacIntraSrc(addr, mbx, mby, pps), d.cabacResidSrc(addr, mbx, mby, true))
	if !d.lastQPDHit {
		d.lastQPD = 0
	}
	return err
}

// decodeMBBCabac decodes one CABAC B macroblock: B skip flag, B type,
// then the shared B partition core or the intra escape (48 = PCM).
func (d *Decoder) decodeMBBCabac(h *SliceHeader, r *Reader, pps *PPS, addr int) error {
	d.lastQPDHit = false
	d.mbSlice[addr] = d.slices
	mbx, mby := addr%d.mbW, addr/d.mbW
	skip, err := d.cabacSkipFlagB(mbx, mby, addr)
	if err != nil {
		return err
	}
	if skip {
		d.skipped[addr] = true
		d.mbSlice[addr] = d.slices
		d.lastQPD = 0
		return d.decodeSkipB(h, addr, d.cabacResidSrc(addr, mbx, mby, false))
	}
	mbType, err := d.cabacMBTypeB(mbx, mby, addr)
	if err != nil {
		return err
	}
	if mbType >= 23 {
		intraType := mbType - 23
		if intraType == 25 {
			return d.cabacDecodePCM(r, mbx, mby)
		}
		err = d.decodeIntraMBCore(h, pps, addr, mbx, mby, intraType, d.cabacIntraSrc(addr, mbx, mby, pps), d.cabacResidSrc(addr, mbx, mby, true))
	} else {
		err = d.decodeMBBParts(h, pps, addr, mbx, mby, mbType, d.cabacBInterSrc(addr, mbx, mby, h, pps), d.cabacResidSrc(addr, mbx, mby, false))
	}
	if !d.lastQPDHit {
		d.lastQPD = 0
	}
	return err
}

func (d *Decoder) decodeMBPCabac(h *SliceHeader, r *Reader, pps *PPS, addr int) error {
	d.lastQPDHit = false
	d.mbSlice[addr] = d.slices
	mbx, mby := addr%d.mbW, addr/d.mbW
	skip, err := d.cabacSkipFlag(mbx, mby, addr)
	if err != nil {
		return err
	}
	if skip {
		d.skipped[addr] = true
		d.mbSlice[addr] = d.slices
		d.lastQPD = 0
		return d.decodeSkip(h, addr, d.cabacResidSrc(addr, mbx, mby, false))
	}
	mbType, err := d.cabacMBTypeP(mbx, mby, addr, h)
	if err != nil {
		return err
	}
	if mbType >= 5 {
		intraType := mbType - 5
		if intraType == 25 {
			return d.cabacDecodePCM(r, mbx, mby)
		}
		err = d.decodeIntraMBCore(h, pps, addr, mbx, mby, intraType, d.cabacIntraSrc(addr, mbx, mby, pps), d.cabacResidSrc(addr, mbx, mby, true))
	} else {
		err = d.decodeMBPParts(h, pps, addr, mbx, mby, mbType, d.cabacInterSrc(addr, mbx, mby, h, pps), d.cabacResidSrc(addr, mbx, mby, false))
	}
	if !d.lastQPDHit {
		d.lastQPD = 0
	}
	return err
}

// cabacDecodePCM terminates arithmetic decoding, hands the byte reader
// over at the exact consumed position and stores raw samples through
// the shared PCM path.
func (d *Decoder) cabacDecodePCM(r *Reader, mbx, mby int) error {
	d.lastQPD = 0
	r.pos = (d.cabByte0 + d.cab.consumedBytes()) * 8
	r.AlignToByte()
	raw, err := r.ReadBytes(384)
	if err != nil {
		return fmt.Errorf("pcm: %w", err)
	}
	d.storePCM(raw, mbx, mby)
	return nil
}

// cabacSkipFlag reads mb_skip_flag: context 11 plus one per same-slice
// non-skip left/top neighbour.
func (d *Decoder) cabacSkipFlag(mbx, mby, addr int) (bool, error) {
	left, top := d.cabLeftTop(addr, mbx, mby)
	ctx := uint16(11)
	if left >= 0 && !d.skipped[left] {
		ctx++
	}
	if top >= 0 && !d.skipped[top] {
		ctx++
	}
	return d.cabBin(ctx) != 0, nil
}

// cabacSkipFlagB reads a B-slice skip flag: same neighbours as P, base
// context 24 instead of 11.
func (d *Decoder) cabacSkipFlagB(mbx, mby, addr int) (bool, error) {
	left, top := d.cabLeftTop(addr, mbx, mby)
	ctx := uint16(24)
	if left >= 0 && !d.skipped[left] {
		ctx++
	}
	if top >= 0 && !d.skipped[top] {
		ctx++
	}
	return d.cabBin(ctx) != 0, nil
}

// cabacMBTypeB reads a B-slice mb_type (0..22 inter, 23..48 intra
// escape/PCM). Directness of left/top neighbours picks the first context.
func (d *Decoder) cabacMBTypeB(mbx, mby, addr int) (uint32, error) {
	left, top := d.cabLeftTop(addr, mbx, mby)
	ctx := uint16(27)
	if left >= 0 && !d.mbDirect[left] {
		ctx++
	}
	if top >= 0 && !d.mbDirect[top] {
		ctx++
	}
	if d.cabBin(ctx) == 0 {
		return 0, nil // B_Direct_16x16
	}
	if d.cabBin(30) == 0 {
		return uint32(1 + d.cabBin(32)), nil // B_L0/L1_16x16
	}
	bits := d.cabBin(31)<<3 | d.cabBin(32)<<2 | d.cabBin(32)<<1 | d.cabBin(32)
	switch {
	case bits < 8:
		return uint32(bits + 3), nil // B_Bi_16x16 through B_L1_L0_16x8
	case bits == 14:
		return 11, nil // B_L1_L0_8x16
	case bits == 15:
		return 22, nil // B_8x8
	case bits == 13:
		// Intra escape reads the I4x4 flag first, then the shared
		// body (PCM terminate first inside); 23 = Intra4x4.
		if d.cabBin(32) == 0 {
			return 23, nil
		}
		t, err := d.cabacIntraType(32, false)
		if err != nil {
			return 0, err
		}
		if t > 25 {
			return 0, fmt.Errorf("%w: cabac B intra %d", ErrBadSliceHeader, t)
		}
		return 23 + t, nil
	default:
		bits = (bits << 1) + d.cabBin(32)
		return uint32(bits - 4), nil // B_L0_Bi_* through B_Bi_Bi_*
	}
}

// cabacBSubType reads one B 8x8 sub-block type (0..12, Table 7-17).
func (d *Decoder) cabacBSubType() (uint32, error) {
	if d.cabBin(36) == 0 {
		return 0, nil // B_Direct_8x8
	}
	if d.cabBin(37) == 0 {
		return uint32(1 + d.cabBin(39)), nil // B_L0/L1_8x8
	}
	typ := 3
	if d.cabBin(38) != 0 {
		if d.cabBin(39) != 0 {
			return uint32(11 + d.cabBin(39)), nil // B_L1/Bi_4x4
		}
		typ += 4
	}
	typ += 2*d.cabBin(39) + d.cabBin(39)
	return uint32(typ), nil
}

// cabacIntraType reads the shared Intra16x16/PCM body after the caller
// consumed the I4x4 flag: one terminate bin for PCM, then luma flag,
// chroma pair and two prediction bits. base selects contexts 3 (I slice)
// or 17/32 (P/B-slice escape); intra picks the second chroma/pred
// contexts.
func (d *Decoder) cabacIntraType(base uint16, intra bool) (uint32, error) {
	// The I-slice first bin consumes the base pair before the body;
	// the P escape reads its I4x4 flag straight from the base.
	st := base
	if intra {
		st = base + 2
	}
	if d.cab.terminate() != 0 {
		return 25, nil
	}
	mbType := uint32(1)
	mbType += 12 * uint32(d.cabBin(st+1))
	if d.cabBin(st+2) != 0 {
		// I slices step to the next chroma context; the P escape
		// reuses the first one twice.
		n := st + 2
		if intra {
			n = st + 3
		}
		mbType += 4 + 4*uint32(d.cabBin(n))
	}
	p := st + 3
	q := st + 3
	if intra {
		p, q = st+4, st+5
	}
	mbType += 2*uint32(d.cabBin(p)) + uint32(d.cabBin(q))
	return mbType, nil
}

// cabacMBTypeI reads an I-slice mb_type (0 = Intra4x4, 1..24 =
// Intra16x16, 25 = PCM).
func (d *Decoder) cabacMBTypeI(mbx, mby, addr int) (uint32, error) {
	left, top := d.cabLeftTop(addr, mbx, mby)
	ctx := uint16(3)
	if left >= 0 && d.mbI16[left] {
		ctx++
	}
	if top >= 0 && d.mbI16[top] {
		ctx++
	}
	if d.cabBin(ctx) == 0 {
		return 0, nil
	}
	return d.cabacIntraType(3, true)
}

// cabacMBTypeP reads a P-slice mb_type (0..3 inter partitions, 5..30
// intra escape/PCM). Value 3 folds to the ref0-only engine case when
// the slice owns a single reference.
func (d *Decoder) cabacMBTypeP(mbx, mby, addr int, h *SliceHeader) (uint32, error) {
	if d.cabBin(14) == 0 {
		if d.cabBin(15) == 0 {
			return uint32(3 * d.cabBin(16)), nil
		}
		return uint32(2 - d.cabBin(17)), nil
	}
	if d.cabBin(17) == 0 {
		return 5, nil
	}
	t, err := d.cabacIntraType(17, false)
	if err != nil {
		return 0, err
	}
	if t > 25 {
		return 0, fmt.Errorf("%w: cabac P intra %d", ErrBadSliceHeader, t)
	}
	// Escape values share the engine's 0..25 intra numbering (25 = PCM
	// folds to 30, the P-slice PCM type).
	return 5 + t, nil
}

// cabacIntraSrc provides intra syntax from CABAC bins.
func (d *Decoder) cabacIntraSrc(addr, mbx, mby int, pps *PPS) *intraSrc {
	return &intraSrc{
		t8: func() (bool, error) {
			if pps == nil || !pps.Transform8x8 {
				return false, nil
			}
			return d.cabBin(399+uint16(d.neighborT8(addr, mbx, mby))) != 0, nil
		},
		mode: func(bx, by, _ int) (int, error) {
			pred := d.intraMostProbable(bx, by)
			if d.cabBin(68) != 0 {
				return pred, nil
			}
			rem := d.cabBin(69) + 2*d.cabBin(69) + 4*d.cabBin(69)
			if rem >= pred {
				rem++
			}
			return rem, nil
		},
		mode8: func(bx, by int) (int, error) {
			pred := d.intraMostProbable(bx, by)
			if d.cabBin(68) != 0 {
				return pred, nil
			}
			rem := d.cabBin(69) + 2*d.cabBin(69) + 4*d.cabBin(69)
			if rem >= pred {
				rem++
			}
			return rem, nil
		},
		chroma: func() (uint32, error) {
			left, top := d.cabLeftTop(addr, mbx, mby)
			ctx := uint16(64)
			if left >= 0 && d.mbIntra[left] && d.cmode[left] != 2 {
				ctx++
			}
			if top >= 0 && d.mbIntra[top] && d.cmode[top] != 2 {
				ctx++
			}
			if d.cabBin(ctx) == 0 {
				return 2, nil
			}
			if d.cabBin(67) == 0 {
				return 1, nil
			}
			if d.cabBin(67) == 0 {
				return 0, nil
			}
			return 3, nil
		},
		cbp: func() (uint32, error) {
			return d.cabacCBP(addr, mbx, mby, true)
		},
		qpD: func() (int32, error) {
			return d.cabacQPDelta()
		},
	}
}

// cabacMissingCBP is the CABAC unavailable-neighbour default: all luma
// bits (plus DC presence) for intra blocks, luma-only for inter blocks.
func cabacMissingCBP(intra bool) uint32 {
	if intra {
		return 0x7CF
	}
	return 0x00F
}

// cabacCBP reads luma (4 chained bins from 73) and chroma (up to two
// bins from 77) pattern bits into the shared CBP numbering.
func (d *Decoder) cabacCBP(addr, mbx, mby int, intra bool) (uint32, error) {
	left, top := d.cabLeftTop(addr, mbx, mby)
	lc, tc := cabacMissingCBP(intra), cabacMissingCBP(intra)
	if left >= 0 {
		lc = uint32(d.cbpArr[left])
	}
	if top >= 0 {
		tc = uint32(d.cbpArr[top])
	}
	bit := func(v uint32) uint16 {
		if v != 0 {
			return 0
		}
		return 1
	}
	cbp := uint32(d.cabBin(73 + bit(lc&0x02) + 2*bit(tc&0x04)))
	cbp |= uint32(d.cabBin(73+bit(cbp&0x01)+2*bit(tc&0x08))) << 1
	cbp |= uint32(d.cabBin(73+bit(lc&0x08)+2*bit(cbp&0x01))) << 2
	cbp |= uint32(d.cabBin(73+bit(cbp&0x04)+2*bit(cbp&0x02))) << 3
	la, ta := (lc>>4)&3, (tc>>4)&3
	ctx := uint16(77)
	if la > 0 {
		ctx++
	}
	if ta > 0 {
		ctx += 2
	}
	if d.cabBin(ctx) == 0 {
		return cbp, nil
	}
	ctx = 81
	if la == 2 {
		ctx++
	}
	if ta == 2 {
		ctx += 2
	}
	cbp |= uint32(1+d.cabBin(uint16(ctx))) << 4
	return cbp, nil
}

// cabacQPDelta reads mb_qp_delta (base context 60) and records it for
// the next block's context.
func (d *Decoder) cabacQPDelta() (int32, error) {
	d.lastQPDHit = true
	ctx := uint16(60)
	if d.lastQPD != 0 {
		ctx++
	}
	if d.cabBin(ctx) == 0 {
		d.lastQPD = 0
		return 0, nil
	}
	val := 1
	ctx = 62
	for d.cabBin(ctx) != 0 {
		ctx = 63
		val++
		if val > 102 {
			return 0, fmt.Errorf("%w: cabac qp delta overflow", ErrBadSliceHeader)
		}
	}
	var delta int32
	if val&1 != 0 {
		delta = int32((val + 1) >> 1)
	} else {
		delta = -int32((val + 1) >> 1)
	}
	d.lastQPD = delta
	return delta, nil
}

// cabacInterSrc provides partition-level syntax from CABAC bins.
func (d *Decoder) cabacInterSrc(addr, mbx, mby int, h *SliceHeader, pps *PPS) *interSrc {
	return &interSrc{
		ref: func(bx, by int) (int8, error) {
			if h.RefL0Count <= 1 {
				return 0, nil
			}
			return d.cabacRefIdx(addr, mbx, mby, bx, by)
		},
		mvd: func(bx, by int, px, py int16) (mx, my, dx, dy int16, err error) {
			dx, err = d.cabacMVD(bx, by, 0, 0)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			dy, err = d.cabacMVD(bx, by, 1, 0)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			return px + dx, py + dy, dx, dy, nil
		},
		sub: func() (uint32, error) {
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
		},
		cbp: func() (uint32, error) {
			return d.cabacCBP(addr, mbx, mby, false)
		},
		qpD: func() (int32, error) {
			return d.cabacQPDelta()
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
			return d.cabBin(399+uint16(d.neighborT8(addr, mbx, mby))) != 0, nil
		},
	}
}

// cabacBInterSrc reads B inter syntax from bins: per-list reference
// indices, shared motion differences, B sub-block types.
func (d *Decoder) cabacBInterSrc(addr, mbx, mby int, h *SliceHeader, pps *PPS) *bInterSrc {
	return &bInterSrc{
		ref0: func(bx, by int) (int8, error) {
			if h.RefL0Count <= 1 {
				return 0, nil
			}
			return d.cabacRefIdxB(addr, mbx, mby, bx, by, 0)
		},
		ref1: func(bx, by int) (int8, error) {
			if h.RefL1Count <= 1 {
				return 0, nil
			}
			return d.cabacRefIdxB(addr, mbx, mby, bx, by, 1)
		},
		mvd: func(list, bx, by int, px, py int16) (mx, my, dx, dy int16, err error) {
			dx, err = d.cabacMVD(bx, by, 0, list)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			dy, err = d.cabacMVD(bx, by, 1, list)
			if err != nil {
				return 0, 0, 0, 0, err
			}
			return px + dx, py + dy, dx, dy, nil
		},
		sub: func() (uint32, error) {
			return d.cabacBSubType()
		},
		cbp: func() (uint32, error) {
			return d.cabacCBP(addr, mbx, mby, false)
		},
		qpD: func() (int32, error) {
			return d.cabacQPDelta()
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
			return d.cabBin(399+uint16(d.neighborT8(addr, mbx, mby))) != 0, nil
		},
	}
}

// cabacRefAtB reads one 4x4 reference index of one list for neighbour
// contexts: unavailable, cross-slice or non-using slots report -1.
func (d *Decoder) cabacRefAtB(bx, by, list int) int {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return -1
	}
	if !d.cabSameSlice((by/4)*d.mbW + bx/4) {
		return -1
	}
	i := by*d.mbW*4 + bx
	if d.useM[i]&(1<<uint(list)) == 0 {
		return -1
	}
	if list == 0 {
		return int(d.refIdx[i])
	}
	return int(d.refIdx1[i])
}

// cabacRefIdxB reads one B reference index of one list: neighbours above
// zero count unless direct-derived; unary bins from 54+i. Slots inside
// the current macroblock come from early scratch.
func (d *Decoder) cabacRefIdxB(addr, mbx, mby, bx, by, list int) (int8, error) {
	at := func(x, y int) (int, bool) {
		if x < 0 || y < 0 || x >= d.mbW*4 || y >= d.mbH*4 {
			return -1, false
		}
		if x/4 == mbx && y/4 == mby {
			if list == 0 {
				return int(d.refTmp[y*d.mbW*4+x]), true
			}
			return int(d.refTmp1[y*d.mbW*4+x]), true
		}
		if !d.cabSameSlice((y/4)*d.mbW + x/4) {
			return -1, false
		}
		i := y*d.mbW*4 + x
		var r int8
		if list == 0 {
			r = d.refIdx[i]
		} else {
			r = d.refIdx1[i]
		}
		return int(r), !d.direct4[i]
	}
	off := uint16(0)
	if v, direct := at(bx-1, by); v > 0 && direct {
		off++
	}
	if v, direct := at(bx, by-1); v > 0 && direct {
		off += 2
	}
	ref := 0
	for d.cabBin(54+off) != 0 {
		ref++
		off = (off >> 2) + 4
		if ref >= 32 {
			return 0, fmt.Errorf("%w: cabac B ref l%d overflow", ErrBadSliceHeader, list)
		}
	}
	return int8(ref), nil
}

// cabacRefAt reads one 4x4 reference index for neighbour contexts:
// unavailable or intra slots report -1 (never greater than zero).
func (d *Decoder) cabacRefAt(bx, by int) int {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return -1
	}
	if !d.cabSameSlice((by/4)*d.mbW + bx/4) {
		return -1
	}
	return int(d.refIdx[by*d.mbW*4+bx])
}

// cabacRefIdx reads one reference index: unary bins from 54+i with the
// offset stepping to 4 then 5 (indices 58 then 59). Slots inside the
// current macroblock come from early scratch (references land before
// motion vectors); everything else uses the committed index.
func (d *Decoder) cabacRefIdx(addr, mbx, mby, bx, by int) (int8, error) {
	at := func(x, y int) int {
		if x/4 == mbx && y/4 == mby && x >= 0 && y >= 0 && x < d.mbW*4 && y < d.mbH*4 {
			return int(d.refTmp[y*d.mbW*4+x])
		}
		return d.cabacRefAt(x, y)
	}
	off := uint16(0)
	if at(bx-1, by) > 0 {
		off++
	}
	if at(bx, by-1) > 0 {
		off += 2
	}
	ref := 0
	for d.cabBin(54+off) != 0 {
		ref++
		off = (off >> 2) + 4
		if ref >= 32 {
			return 0, fmt.Errorf("%w: cabac ref idx overflow", ErrBadSliceHeader)
		}
	}
	return int8(ref), nil
}

// cabacMVDAt reads one signed 4x4 motion difference of one list for
// neighbour sums: P slices only own list 0; B slices keep both.
func (d *Decoder) cabacMVDAt(bx, by, comp, list int) int {
	if bx < 0 || by < 0 || bx >= d.mbW*4 || by >= d.mbH*4 {
		return 0
	}
	if !d.cabSameSlice((by/4)*d.mbW + bx/4) {
		return 0
	}
	var v int
	if list == 0 {
		v = int(d.mvdX[by*d.mbW*4+bx])
		if comp == 1 {
			v = int(d.mvdY[by*d.mbW*4+bx])
		}
	} else {
		v = int(d.mvdX1[by*d.mbW*4+bx])
		if comp == 1 {
			v = int(d.mvdY1[by*d.mbW*4+bx])
		}
	}
	if v < 0 {
		v = -v
	}
	return v
}

// cabacMVD reads one motion vector difference of one list: leading bin
// picks one of three contexts by neighbour magnitude, short unary runs
// to 8, longer values continue Exp-Golomb order 3 in bypass, sign closes
// the code.
func (d *Decoder) cabacMVD(bx, by, comp, list int) (int16, error) {
	base := uint16(40)
	if comp == 1 {
		base = 47
	}
	sum := d.cabacMVDAt(bx-1, by, comp, list) + d.cabacMVDAt(bx, by-1, comp, list)
	ctx := base
	if sum > 2 {
		ctx++
	}
	if sum > 32 {
		ctx++
	}
	if d.cabBin(ctx) == 0 {
		return 0, nil
	}
	mvd := 1
	ctx = base + 3
	for mvd < 9 && d.cabBin(ctx) != 0 {
		if mvd < 4 {
			ctx++
		}
		mvd++
	}
	if mvd >= 9 {
		k := 3
		for d.cab.bypass() != 0 {
			mvd += 1 << uint(k)
			k++
			if k > 24 {
				return 0, fmt.Errorf("%w: cabac mvd overflow", ErrBadSliceHeader)
			}
		}
		for k > 0 {
			k--
			mvd += d.cab.bypass() << uint(k)
		}
	}
	return int16(d.cab.bypassSign(int32(-mvd))), nil
}

// cabacCBF reads one coded_block_flag. DC blocks consult neighbour CBP
// presence bits; AC blocks consult neighbour nonzero counts.
func (d *Decoder) cabacCBF(cat int, nza, nzb int) bool {
	ctx := uint16(cabacCBFBase[cat])
	if nza > 0 {
		ctx++
	}
	if nzb > 0 {
		ctx += 2
	}
	return d.cabBin(ctx) != 0
}

// cabacNNZAt reads one 4x4 nonzero count for AC flag contexts (unit =
// blocks per macroblock side: 4 luma, 2 chroma). Out-of-picture or
// foreign-slice slots report the unavailable marker: 64 (counts as
// present) for intra blocks, 0 for inter blocks.
func (d *Decoder) cabacNNZAt(grid []int8, stride, height, unit, bx, by int, intra bool) int {
	if bx < 0 || by < 0 || bx >= stride || by >= height {
		return cabacMissingNNZ(intra)
	}
	if !d.cabSameSlice((by/unit)*d.mbW + bx/unit) {
		return cabacMissingNNZ(intra)
	}
	return int(grid[by*stride+bx])
}

// cabacMissingNNZ is the unavailable-slot nonzero marker.
func cabacMissingNNZ(intra bool) int {
	if intra {
		return 64
	}
	return 0
}

// cabacCoeffData reads significance map plus levels for one transform
// block into scan order: positions run 0..maxCoeff-1 shifted by scanOff
// (AC-only blocks skip the DC slot), levels close in reverse with the
// node state machine and bypass suffixes.
func (d *Decoder) cabacCoeffData(cat, maxCoeff, scanOff int) ([16]int32, int, error) {
	var out [16]int32
	sigBase := uint16(cabacSigOffset[cat])
	lastBase := uint16(cabacLastOffset[cat])
	lvlBase := uint16(cabacLevelOffset[cat])
	var index [16]int
	cc := 0
	last := 0
	for ; last < maxCoeff-1; last++ {
		if d.cabBin(sigBase+uint16(last)) == 0 {
			continue
		}
		index[cc] = last
		cc++
		if d.cabBin(lastBase+uint16(last)) != 0 {
			last = maxCoeff
			break
		}
	}
	if last == maxCoeff-1 {
		index[cc] = last
		cc++
	}
	tc := cc
	node := 0
	for cc > 0 {
		cc--
		j := index[cc] + scanOff
		lvl := 1
		if d.cabBin(lvlBase+uint16(cabacLevel1Ctx[node])) == 0 {
			node = int(cabacLevelTrans[node])
		} else {
			lvl = 2
			gctx := lvlBase + uint16(cabacLevelGt1Ctx[node])
			node = int(cabacLevelTrans[8+node])
			for lvl < 15 && d.cabBin(gctx) != 0 {
				lvl++
			}
			if lvl >= 15 {
				k := 0
				for d.cab.bypass() != 0 && k < 23 {
					k++
				}
				lvl = 1
				for k > 0 {
					k--
					lvl += lvl + d.cab.bypass()
				}
				lvl += 14
			}
		}
		out[j] = d.cab.bypassSign(int32(-lvl))
	}
	return out, tc, nil
}

// cabacCoeffData8x8 reads one 8x8 luma significance map plus levels
// into zigzag scan order (no coded-block flag: CBP gates the block).
func (d *Decoder) cabacCoeffData8x8() ([64]int32, int, error) {
	var out [64]int32
	const sigBase, lastBase, lvlBase = uint16(402), uint16(417), uint16(426)
	var index [64]int
	cc, last := 0, 0
	for ; last < 63; last++ {
		if d.cabBin(sigBase+cabacSigOffset8x8[last]) == 0 {
			continue
		}
		index[cc] = last
		cc++
		if d.cabBin(lastBase+uint16(cabacLastCoeffOffset8x8[last])) != 0 {
			last = 64
			break
		}
	}
	if last == 63 {
		index[cc] = last
		cc++
	}
	tc := cc
	node := 0
	for cc > 0 {
		cc--
		j := index[cc]
		lvl := 1
		if d.cabBin(lvlBase+uint16(cabacLevel1Ctx[node])) == 0 {
			node = int(cabacLevelTrans[node])
		} else {
			lvl = 2
			gctx := lvlBase + uint16(cabacLevelGt1Ctx[node])
			node = int(cabacLevelTrans[8+node])
			for lvl < 15 && d.cabBin(gctx) != 0 {
				lvl++
			}
			if lvl >= 15 {
				k := 0
				for d.cab.bypass() != 0 && k < 23 {
					k++
				}
				lvl = 1
				for k > 0 {
					k--
					lvl += lvl + d.cab.bypass()
				}
				lvl += 14
			}
		}
		out[j] = d.cab.bypassSign(int32(-lvl))
	}
	return out, tc, nil
}

// cabacResidSrc provides residual blocks from CABAC bins. Each closure
// reads its own flag first, so blocks the shared reconstruction loops
// skip (zero CBP) cost no bins and zero-flagged blocks cost one.
// intra selects the unavailable-neighbour defaults (full CBP / 64).
func (d *Decoder) cabacResidSrc(addr, mbx, mby int, intra bool) *residSrc {
	ys, yh := d.mbW*4, d.mbH*4
	cs, ch := d.mbW*2, d.mbH*2
	// dcBit reports one neighbour DC-presence bit for flag contexts.
	dcBit := func(naddr int, bit uint16) int {
		if d.cabSameSlice(naddr) {
			if d.cbpArr[naddr]&bit != 0 {
				return 1
			}
			return 0
		}
		if intra {
			return 1
		}
		return 0
	}
	return &residSrc{
		// Shared loops call lumaAC for Intra4x4/inter blocks only,
		// which always use category 2 (the cat argument stays for
		// the common signature).
		lumaAC: func(bx, by, cat int) ([16]int32, int, error) {
			nza := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx-1, by, intra)
			nzb := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx, by-1, intra)
			if !d.cabacCBF(2, nza, nzb) {
				return [16]int32{}, 0, nil
			}
			return d.cabacCoeffData(2, 16, 0)
		},
		lumaAC15: func(bx, by int) ([16]int32, int, error) {
			nza := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx-1, by, intra)
			nzb := d.cabacNNZAt(d.nnzY, ys, yh, 4, bx, by-1, intra)
			if !d.cabacCBF(1, nza, nzb) {
				return [16]int32{}, 0, nil
			}
			return d.cabacCoeffData(1, 15, 1)
		},
		lumaDC: func(mbx, mby int) ([16]int32, error) {
			nza, nzb := 0, 0
			if mbx > 0 {
				nza = dcBit(addr-1, 0x100)
			} else if intra {
				nza = 1
			}
			if mby > 0 {
				nzb = dcBit(addr-d.mbW, 0x100)
			} else if intra {
				nzb = 1
			}
			if !d.cabacCBF(0, nza, nzb) {
				return [16]int32{}, nil
			}
			// A coded DC block marks presence for later neighbours.
			d.cbpArr[addr] |= 0x100
			out, _, err := d.cabacCoeffData(0, 16, 0)
			return out, err
		},
		chromaDC: func(comp int) ([4]int32, error) {
			bit := uint16(0x40 << uint(comp))
			nza, nzb := 0, 0
			if mbx > 0 {
				nza = dcBit(addr-1, bit)
			} else if intra {
				nza = 1
			}
			if mby > 0 {
				nzb = dcBit(addr-d.mbW, bit)
			} else if intra {
				nzb = 1
			}
			if !d.cabacCBF(3, nza, nzb) {
				return [4]int32{}, nil
			}
			d.cbpArr[addr] |= bit
			out, _, err := d.cabacCoeffData(3, 4, 0)
			if err != nil {
				return [4]int32{}, err
			}
			var dc [4]int32
			copy(dc[:], out[:4])
			return dc, nil
		},
		chromaAC: func(mbx, mby, comp, b int) ([16]int32, int, error) {
			grid := d.nnzCb
			if comp == 1 {
				grid = d.nnzCr
			}
			bx, by := mbx*2+b%2, mby*2+b/2
			nza := d.cabacNNZAt(grid, cs, ch, 2, bx-1, by, intra)
			nzb := d.cabacNNZAt(grid, cs, ch, 2, bx, by-1, intra)
			if !d.cabacCBF(4, nza, nzb) {
				return [16]int32{}, 0, nil
			}
			return d.cabacCoeffData(4, 15, 1)
		},
		luma8x8: func(mbx, mby, i8 int) ([64]int32, [4]int, error) {
			out, tc, err := d.cabacCoeffData8x8()
			if err != nil {
				return out, [4]int{}, err
			}
			return out, [4]int{tc, tc, tc, tc}, nil
		},
	}
}
