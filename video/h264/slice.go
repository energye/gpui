package h264

import "fmt"

// Slice types after modulo 5 (Table 7-6).
const (
	SliceP  = 0
	SliceB  = 1
	SliceI  = 2
	SliceSP = 3
	SliceSI = 4
)

// MMCOOp is one decoded-reference marking operation (Annex C control).
type MMCOOp struct {
	Op   uint32
	Arg1 int32
	Arg2 uint32
}

// SliceHeader is the parsed slice header: addressing, type, quant start,
// filter switches and reference-marking ops. MB payload follows.
type SliceHeader struct {
	FirstMB         uint32
	Type            uint32
	PPSID           uint32
	FrameNum        uint32
	FieldPic        bool
	BottomField     bool
	IDRPicID        uint32
	POC             int32
	RedundantCnt    uint32
	RefL0Count      uint32
	RefL1Count      uint32
	DirectSpatial   bool
	CabacInitIDC    uint32
	QPDelta         int32
	DisableFilter   uint32
	FilterAlpha     int32
	FilterBeta      int32
	NalRefIDC       int
	NoOutputPrior   bool
	LongTermRef     bool
	AdaptiveMarking bool
	MMCO            []MMCOOp
	IsIDR           bool
	BitsUsed        int
}

// IsI/IsP/IsB classify the slice coding type.
func (s *SliceHeader) IsI() bool { return s != nil && s.Type == SliceI }
func (s *SliceHeader) IsP() bool { return s != nil && (s.Type == SliceP || s.Type == SliceSP) }
func (s *SliceHeader) IsB() bool { return s != nil && s.Type == SliceB }

// ParseSliceHeader parses one slice NALU header (header byte included).
// pps/sps select the active sets. It also returns the bit reader
// positioned at the first macroblock bit, so slice payload decoding can
// continue without re-reading.
func ParseSliceHeader(nalu []byte, pps *PPS, sps *SPS) (*SliceHeader, *Reader, error) {
	forbidden, refIDC, typ, err := NALUHeader(nalu)
	if err != nil {
		return nil, nil, err
	}
	if forbidden {
		return nil, nil, fmt.Errorf("%w: forbidden bit set", ErrBadSliceHeader)
	}
	if typ != NALSliceNonIDR && typ != NALSliceIDR {
		return nil, nil, fmt.Errorf("%w: type %d is not a base slice", ErrBadSliceHeader, typ)
	}
	if pps == nil || sps == nil {
		return nil, nil, fmt.Errorf("%w: slice without active sets", ErrMissingPPS)
	}
	r := NewReader(UnescapeRBSP(nalu[1:]))
	h := &SliceHeader{
		IsIDR:      typ == NALSliceIDR,
		NalRefIDC:  refIDC,
		RefL0Count: pps.RefL0Default,
		RefL1Count: pps.RefL1Default,
	}
	if h.FirstMB, err = r.ReadUE(); err != nil {
		return nil, nil, fmt.Errorf("%w: first mb: %v", ErrBadSliceHeader, err)
	}
	rawType, err := r.ReadUE()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: slice type: %v", ErrBadSliceHeader, err)
	}
	if rawType > 9 {
		return nil, nil, fmt.Errorf("%w: slice type %d", ErrBadSliceHeader, rawType)
	}
	h.Type = rawType % 5
	if h.PPSID, err = r.ReadUE(); err != nil {
		return nil, nil, fmt.Errorf("%w: pps id: %v", ErrBadSliceHeader, err)
	}
	if h.PPSID != pps.ID {
		return nil, nil, fmt.Errorf("%w: slice pps %d, active %d", ErrBadSliceHeader, h.PPSID, pps.ID)
	}
	fnBits, err := frameNumBits(sps)
	if err != nil {
		return nil, nil, err
	}
	if h.FrameNum, err = r.ReadBits(fnBits); err != nil {
		return nil, nil, fmt.Errorf("%w: frame num: %v", ErrBadSliceHeader, err)
	}
	if !sps.FrameMBsOnly {
		f, err := r.ReadBits(1)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: field pic: %v", ErrBadSliceHeader, err)
		}
		h.FieldPic = f != 0
		if h.FieldPic {
			b, err := r.ReadBits(1)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: bottom field: %v", ErrBadSliceHeader, err)
			}
			h.BottomField = b != 0
		}
	}
	if h.IsIDR {
		if h.IDRPicID, err = r.ReadUE(); err != nil {
			return nil, nil, fmt.Errorf("%w: idr pic id: %v", ErrBadSliceHeader, err)
		}
	}
	if err := parsePOCLSB(r, h, pps, sps); err != nil {
		return nil, nil, err
	}
	if pps.RedundantPicPresent {
		rc, err := r.ReadUE()
		if err != nil {
			return nil, nil, fmt.Errorf("%w: redundant pic: %v", ErrBadSliceHeader, err)
		}
		h.RedundantCnt = rc
	}
	if err := finishSliceHeader(r, h, pps, sps); err != nil {
		return nil, nil, err
	}
	return h, r, nil
}

func frameNumBits(sps *SPS) (int, error) {
	return int(sps.Log2MaxFrameNum) + 4, nil
}

func parsePOCLSB(r *Reader, h *SliceHeader, pps *PPS, sps *SPS) error {
	pocType, err := spsPOCType(sps)
	if err != nil {
		return err
	}
	switch pocType {
	case 0:
		lsbBits, err := spsPOCLSB(sps)
		if err != nil {
			return err
		}
		lsb, err := r.ReadBits(lsbBits)
		if err != nil {
			return fmt.Errorf("%w: poc lsb: %v", ErrBadSliceHeader, err)
		}
		h.POC = int32(lsb)
		if pps.BottomOrderPresent && !h.FieldPic {
			d, err := r.ReadSE()
			if err != nil {
				return fmt.Errorf("%w: poc bottom: %v", ErrBadSliceHeader, err)
			}
			_ = d
		}
	case 1:
		// Full poc_type 1 reconstruction needs stream state; record zero
		// and let the DPB order by frame_num until VR2d wires B reordering.
		h.POC = int32(h.FrameNum) * 2
	case 2:
		h.POC = int32(h.FrameNum) * 2
	}
	return nil
}

func finishSliceHeader(r *Reader, h *SliceHeader, pps *PPS, sps *SPS) error {
	_ = sps
	if h.Type == SliceB {
		d, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: direct flag: %v", ErrBadSliceHeader, err)
		}
		h.DirectSpatial = d != 0
	}
	if h.Type == SliceP || h.Type == SliceSP || h.Type == SliceB {
		over, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: ref override: %v", ErrBadSliceHeader, err)
		}
		if over != 0 {
			l0, err := r.ReadUE()
			if err != nil {
				return fmt.Errorf("%w: ref l0 active: %v", ErrBadSliceHeader, err)
			}
			h.RefL0Count = l0 + 1
			if h.Type == SliceB {
				l1, err := r.ReadUE()
				if err != nil {
					return fmt.Errorf("%w: ref l1 active: %v", ErrBadSliceHeader, err)
				}
				h.RefL1Count = l1 + 1
			}
		}
		if err := skipRefPicListMod(r, h.Type); err != nil {
			return err
		}
	}
	if (pps.WeightedPred && (h.Type == SliceP || h.Type == SliceSP)) ||
		(h.Type == SliceB && pps.WeightedBiPred == 1) {
		if err := skipWeightTable(r, h, sps); err != nil {
			return err
		}
	}
	if err := parseMarking(r, h); err != nil {
		return err
	}
	if pps.EntropyCABAC && h.Type != SliceI && h.Type != SliceSI {
		c, err := r.ReadUE()
		if err != nil {
			return fmt.Errorf("%w: cabac init: %v", ErrBadSliceHeader, err)
		}
		if c > 2 {
			return fmt.Errorf("%w: cabac init %d", ErrBadSliceHeader, c)
		}
		h.CabacInitIDC = c
	}
	qp, err := r.ReadSE()
	if err != nil {
		return fmt.Errorf("%w: qp delta: %v", ErrBadSliceHeader, err)
	}
	if qp < -26-26 || qp > 25 {
		return fmt.Errorf("%w: qp delta %d", ErrBadSliceHeader, qp)
	}
	h.QPDelta = qp
	if h.Type == SliceSP || h.Type == SliceSI {
		if _, err := r.ReadBits(1); err != nil {
			return fmt.Errorf("%w: sp for switch: %v", ErrBadSliceHeader, err)
		}
		if _, err := r.ReadUE(); err != nil {
			return fmt.Errorf("%w: sp qs: %v", ErrBadSliceHeader, err)
		}
	}
	if pps.DeblockingPresent {
		df, err := r.ReadUE()
		if err != nil {
			return fmt.Errorf("%w: deblock idc: %v", ErrBadSliceHeader, err)
		}
		if df > 2 {
			return fmt.Errorf("%w: deblock idc %d", ErrBadSliceHeader, df)
		}
		h.DisableFilter = df
		if df != 1 {
			a, err := r.ReadSE()
			if err != nil {
				return fmt.Errorf("%w: alpha offset: %v", ErrBadSliceHeader, err)
			}
			b, err := r.ReadSE()
			if err != nil {
				return fmt.Errorf("%w: beta offset: %v", ErrBadSliceHeader, err)
			}
			if a < -6 || a > 6 || b < -6 || b > 6 {
				return fmt.Errorf("%w: filter offsets %d/%d", ErrBadSliceHeader, a, b)
			}
			h.FilterAlpha, h.FilterBeta = a, b
		}
	}
	h.BitsUsed = 0
	return nil
}

func skipRefPicListMod(r *Reader, sliceType uint32) error {
	for list := 0; list < 2; list++ {
		if sliceType == SliceB || list == 0 {
			flag, err := r.ReadBits(1)
			if err != nil {
				return fmt.Errorf("%w: ref mod flag: %v", ErrBadSliceHeader, err)
			}
			if flag == 0 {
				continue
			}
			for {
				idc, err := r.ReadUE()
				if err != nil {
					return fmt.Errorf("%w: ref mod idc: %v", ErrBadSliceHeader, err)
				}
				if idc == 3 {
					break
				}
				if idc > 5 {
					return fmt.Errorf("%w: ref mod idc %d", ErrBadSliceHeader, idc)
				}
				if _, err := r.ReadUE(); err != nil {
					return fmt.Errorf("%w: ref mod arg: %v", ErrBadSliceHeader, err)
				}
				if idc == 2 {
					if _, err := r.ReadUE(); err != nil {
						return fmt.Errorf("%w: ref mod view: %v", ErrBadSliceHeader, err)
					}
				}
			}
		}
	}
	return nil
}

func skipWeightTable(r *Reader, h *SliceHeader, sps *SPS) error {
	denom, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("%w: luma denom: %v", ErrBadSliceHeader, err)
	}
	if denom > 7 {
		return fmt.Errorf("%w: luma denom %d", ErrBadSliceHeader, denom)
	}
	chroma := sps.ChromaFormat != 0
	if chroma {
		cdenom, err := r.ReadUE()
		if err != nil {
			return fmt.Errorf("%w: chroma denom: %v", ErrBadSliceHeader, err)
		}
		if cdenom > 7 {
			return fmt.Errorf("%w: chroma denom %d", ErrBadSliceHeader, cdenom)
		}
	}
	lists := []uint32{h.RefL0Count}
	if h.Type == SliceB {
		lists = append(lists, h.RefL1Count)
	}
	for _, n := range lists {
		if n > 32 {
			return fmt.Errorf("%w: ref count %d", ErrBadSliceHeader, n)
		}
		for i := uint32(0); i < n; i++ {
			f, err := r.ReadBits(1)
			if err != nil {
				return fmt.Errorf("%w: luma weight flag: %v", ErrBadSliceHeader, err)
			}
			if f != 0 {
				if _, err := r.ReadSE(); err != nil {
					return fmt.Errorf("%w: luma weight: %v", ErrBadSliceHeader, err)
				}
				if _, err := r.ReadSE(); err != nil {
					return fmt.Errorf("%w: luma offset: %v", ErrBadSliceHeader, err)
				}
			}
			f, err = r.ReadBits(1)
			if err != nil {
				return fmt.Errorf("%w: chroma weight flag: %v", ErrBadSliceHeader, err)
			}
			if f != 0 {
				for k := 0; k < 4; k++ {
					if _, err := r.ReadSE(); err != nil {
						return fmt.Errorf("%w: chroma weight %d: %v", ErrBadSliceHeader, k, err)
					}
				}
			}
		}
	}
	return nil
}

func parseMarking(r *Reader, h *SliceHeader) error {
	// dec_ref_pic_marking() exists only when nal_ref_idc != 0: reference
	// management has nothing to say about a disposable picture.
	if h.NalRefIDC == 0 {
		return nil
	}
	if h.IsIDR {
		n, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: no output prior: %v", ErrBadSliceHeader, err)
		}
		l, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: long term ref: %v", ErrBadSliceHeader, err)
		}
		h.NoOutputPrior = n != 0
		h.LongTermRef = l != 0
		return nil
	}
	a, err := r.ReadBits(1)
	if err != nil {
		return fmt.Errorf("%w: adaptive marking: %v", ErrBadSliceHeader, err)
	}
	if a == 0 {
		return nil
	}
	h.AdaptiveMarking = true
	for {
		op, err := r.ReadUE()
		if err != nil {
			return fmt.Errorf("%w: mmco: %v", ErrBadSliceHeader, err)
		}
		if op == 0 {
			break
		}
		if op > 6 {
			return fmt.Errorf("%w: mmco %d", ErrBadSliceHeader, op)
		}
		m := MMCOOp{Op: op}
		switch op {
		case 1, 3:
			v, err := r.ReadUE()
			if err != nil {
				return fmt.Errorf("%w: mmco arg: %v", ErrBadSliceHeader, err)
			}
			m.Arg1 = int32(v)
		case 2:
			v, err := r.ReadUE()
			if err != nil {
				return fmt.Errorf("%w: mmco arg: %v", ErrBadSliceHeader, err)
			}
			m.Arg1 = int32(v)
		case 4:
			v, err := r.ReadUE()
			if err != nil {
				return fmt.Errorf("%w: mmco arg: %v", ErrBadSliceHeader, err)
			}
			m.Arg2 = v
		}
		h.MMCO = append(h.MMCO, m)
	}
	return nil
}

// SPS re-read helpers: the SPS struct keeps decoded values, but a few
// header-only fields (poc type, lsb width) are needed by slice parsing
// without growing the struct in VR1. They are re-read from the stored raw
// bytes so engine state stays single-sourced.
// SPS header fields live on the struct since VR2a (single-sourced at
// parse time); the helpers below used to re-read raw bytes and skewed on
// non-zero fields, so they now return stored values.
func spsPOCType(sps *SPS) (uint32, error) {
	return sps.POCType, nil
}

func spsPOCLSB(sps *SPS) (int, error) {
	if sps.POCType != 0 {
		return 0, fmt.Errorf("%w: poc %d lsb width", ErrBadSliceHeader, sps.POCType)
	}
	return int(sps.Log2MaxPOCLsb) + 4, nil
}
