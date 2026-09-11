package h264

import "fmt"

// ParamSets collects SPS/PPS from both out-of-band (avcC) and in-band
// (Annex B / sample NALUs) sources. Slices can only be trusted once the
// PPS they point at, and the SPS that PPS points at, are both known.
type ParamSets struct {
	SPS map[uint32]*SPS
	PPS map[uint32]*PPS
}

// NewParamSets builds an empty collection.
func NewParamSets() *ParamSets {
	return &ParamSets{SPS: map[uint32]*SPS{}, PPS: map[uint32]*PPS{}}
}

// FromAVCC loads the out-of-band sets from a parsed avcC box.
func (p *ParamSets) FromAVCC(a *AVCC) error {
	if p == nil || a == nil {
		return fmt.Errorf("%w: nil sets or avcc", ErrNoParamSets)
	}
	for _, raw := range a.SPS {
		if err := p.AddSPS(raw); err != nil {
			return err
		}
	}
	for _, raw := range a.PPS {
		if err := p.AddPPS(raw); err != nil {
			return err
		}
	}
	return nil
}

// AddSPS parses and stores one SPS NALU.
func (p *ParamSets) AddSPS(nalu []byte) error {
	s, err := ParseSPS(nalu)
	if err != nil {
		return err
	}
	if p.SPS == nil {
		p.SPS = map[uint32]*SPS{}
	}
	p.SPS[s.ID] = s
	return nil
}

// AddPPS parses and stores one PPS NALU.
func (p *ParamSets) AddPPS(nalu []byte) error {
	q, err := ParsePPS(nalu)
	if err != nil {
		return err
	}
	if p.PPS == nil {
		p.PPS = map[uint32]*PPS{}
	}
	p.PPS[q.ID] = q
	return nil
}

// AddNALU stores SPS/PPS units and ignores the rest.
// It returns true when the unit was a parameter set.
func (p *ParamSets) AddNALU(nalu []byte) (bool, error) {
	t, ok := NALType(nalu)
	if !ok {
		return false, fmt.Errorf("%w: empty unit", ErrBadNALU)
	}
	switch t {
	case NALSPS:
		return true, p.AddSPS(nalu)
	case NALPPS:
		return true, p.AddPPS(nalu)
	default:
		return false, nil
	}
}

// RequireForSlice checks the PPS id a slice header points at and the SPS
// behind it. Missing sets mean the frame cannot be trusted.
func (p *ParamSets) RequireForSlice(ppsID uint32) (*PPS, *SPS, error) {
	q, ok := p.PPS[ppsID]
	if !ok {
		return nil, nil, fmt.Errorf("%w: pps %d", ErrMissingPPS, ppsID)
	}
	s, ok := p.SPS[q.SPSID]
	if !ok {
		return nil, nil, fmt.Errorf("%w: sps %d for pps %d", ErrMissingSPS, q.SPSID, ppsID)
	}
	return q, s, nil
}

// HasSPS and HasPPS report whether at least one set is known.
func (p *ParamSets) HasSPS() bool { return p != nil && len(p.SPS) > 0 }
func (p *ParamSets) HasPPS() bool { return p != nil && len(p.PPS) > 0 }

// Frame is one cut access unit: the NALUs that belong to one picture's
// decode step, in stream order.
type Frame struct {
	Units      [][]byte
	Types      []int
	IsIDR      bool
	SliceCount int
	SizeBytes  int
}

// SplitFrames cuts NALUs into frames. AUD delimits units; SPS/PPS/SEI
// attach to the coming picture; a base slice (types 1/5) with
// first_mb_in_slice == 0 starts a new picture when one is already open.
// Slice-partition (2/3/4) and slice-extension (20) units are F17-class:
// they stop the cut with a readable error instead of guessing.
func SplitFrames(units [][]byte) ([]Frame, error) {
	var out []Frame
	cur := Frame{}
	flush := func() {
		if cur.SliceCount > 0 {
			out = append(out, cur)
		}
		cur = Frame{}
	}
	for _, u := range units {
		t, ok := NALType(u)
		if !ok {
			return nil, fmt.Errorf("%w: empty unit", ErrBadNALU)
		}
		switch t {
		case NALSlicePartA, NALSlicePartB, NALSlicePartC:
			return nil, fmt.Errorf("%w: nal type %d (%s)", ErrDataPartitioning, t, TypeName(t))
		case NALSliceExt:
			return nil, fmt.Errorf("%w: nal type 20 (%s)", ErrUnsupportedNAL, TypeName(t))
		case NALAUD:
			flush()
			continue
		case NALEoS, NALEoB, NALFiller:
			continue
		case NALSPS, NALPPS, NALSei:
			if cur.SliceCount > 0 {
				flush()
			}
			cur.Units = append(cur.Units, u)
			cur.Types = append(cur.Types, t)
			cur.SizeBytes += len(u)
			continue
		case NALPrefix:
			continue
		}
		if t != NALSliceNonIDR && t != NALSliceIDR {
			continue
		}
		mb, err := FirstMBInSlice(u)
		if err != nil {
			return nil, err
		}
		if cur.SliceCount > 0 && mb == 0 {
			flush()
		}
		cur.Units = append(cur.Units, u)
		cur.Types = append(cur.Types, t)
		cur.SizeBytes += len(u)
		cur.SliceCount++
		if t == NALSliceIDR {
			cur.IsIDR = true
		}
	}
	flush()
	return out, nil
}
