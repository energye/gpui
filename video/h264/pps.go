package h264

import "fmt"

// PPS is the parsed picture parameter set.
type PPS struct {
	ID                   uint32
	SPSID                uint32
	EntropyCABAC         bool
	BottomOrderPresent   bool
	NumSliceGroups       uint32
	RefL0Default         uint32
	RefL1Default         uint32
	WeightedPred         bool
	WeightedBiPred       uint32
	PicInitQP            int32
	PicInitQS            int32
	ChromaQPOffset       int32
	DeblockingPresent    bool
	ConstrainedIntra     bool
	RedundantPicPresent  bool
	Transform8x8         bool
	HasScalingMatrix     bool
	SecondChromaQPOffset int32
	Raw                  []byte
}

// ParsePPS parses one PPS NALU (header byte included).
func ParsePPS(nalu []byte) (*PPS, error) {
	forbidden, _, typ, err := NALUHeader(nalu)
	if err != nil {
		return nil, err
	}
	if forbidden {
		return nil, fmt.Errorf("%w: forbidden bit set", ErrBadPPS)
	}
	if typ != NALPPS {
		return nil, fmt.Errorf("%w: type %d is not PPS", ErrBadPPS, typ)
	}
	r := NewReader(UnescapeRBSP(nalu[1:]))
	ppsID, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: pps id: %v", ErrBadPPS, err)
	}
	spsID, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: sps id: %v", ErrBadPPS, err)
	}
	p := &PPS{ID: ppsID, SPSID: spsID}
	entropy, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: entropy flag: %v", ErrBadPPS, err)
	}
	p.EntropyCABAC = entropy != 0
	bottomOrder, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: bottom field order flag: %v", ErrBadPPS, err)
	}
	p.BottomOrderPresent = bottomOrder != 0
	groups, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: slice groups: %v", ErrBadPPS, err)
	}
	p.NumSliceGroups = groups + 1
	if groups > 0 {
		return nil, fmt.Errorf("%w: %d groups", ErrSliceGroups, p.NumSliceGroups)
	}
	l0, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ref l0: %v", ErrBadPPS, err)
	}
	p.RefL0Default = l0 + 1
	l1, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ref l1: %v", ErrBadPPS, err)
	}
	p.RefL1Default = l1 + 1
	weighted, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: weighted pred: %v", ErrBadPPS, err)
	}
	p.WeightedPred = weighted != 0
	bipred, err := r.ReadBits(2)
	if err != nil {
		return nil, fmt.Errorf("%w: weighted bipred: %v", ErrBadPPS, err)
	}
	p.WeightedBiPred = bipred
	if qp, err := r.ReadSE(); err != nil {
		return nil, fmt.Errorf("%w: init qp: %v", ErrBadPPS, err)
	} else {
		p.PicInitQP = qp
	}
	if qs, err := r.ReadSE(); err != nil {
		return nil, fmt.Errorf("%w: init qs: %v", ErrBadPPS, err)
	} else {
		p.PicInitQS = qs
	}
	if off, err := r.ReadSE(); err != nil {
		return nil, fmt.Errorf("%w: chroma qp offset: %v", ErrBadPPS, err)
	} else {
		p.ChromaQPOffset = off
	}
	deblock, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: deblock present: %v", ErrBadPPS, err)
	}
	p.DeblockingPresent = deblock != 0
	constrained, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: constrained intra: %v", ErrBadPPS, err)
	}
	p.ConstrainedIntra = constrained != 0
	redundant, err := r.ReadBits(1)
	if err != nil {
		return nil, fmt.Errorf("%w: redundant pic: %v", ErrBadPPS, err)
	}
	p.RedundantPicPresent = redundant != 0
	if r.MoreRBSPData() {
		t8, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: transform8x8: %v", ErrBadPPS, err)
		}
		p.Transform8x8 = t8 != 0
		scaling, err := r.ReadBits(1)
		if err != nil {
			return nil, fmt.Errorf("%w: scaling present: %v", ErrBadPPS, err)
		}
		if scaling != 0 {
			p.HasScalingMatrix = true
			n := 6 + 2*boolToInt(t8 != 0)
			if err := skipScalingLists(r, n); err != nil {
				return nil, err
			}
		}
		if off, err := r.ReadSE(); err == nil {
			p.SecondChromaQPOffset = off
		}
	}
	p.Raw = append([]byte(nil), nalu...)
	return p, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
