package h265

import (
	"fmt"
)

// ParamSets collects VPS/SPS/PPS from out-of-band (hvcC) and in-band
// (sample NALUs) sources. Slices can only be trusted once the full
// chain is known: PPS -> SPS -> VPS.
// Peer shape (read-only): libavcodec/hevc/ps.c VPS/SPS/PPS lists
// (vps_list/sps_list/pps_list + missing-set errors) + hevcdec.c:4190
// hevc_decode_init (extradata first, then slices).
type ParamSets struct {
	VPS map[uint32]*VPS
	SPS map[uint32]*SPS
	PPS map[uint32]*PPS
}

// NewParamSets builds an empty collection.
func NewParamSets() *ParamSets {
	return &ParamSets{
		VPS: map[uint32]*VPS{},
		SPS: map[uint32]*SPS{},
		PPS: map[uint32]*PPS{},
	}
}

// FromHVCC loads the out-of-band sets from a parsed hvcC box.
func (p *ParamSets) FromHVCC(h *HVCC) error {
	if p == nil || h == nil {
		return fmt.Errorf("%w: nil sets or hvcc", ErrNoParamSets)
	}
	for _, raw := range h.VPS {
		if err := p.AddVPS(raw); err != nil {
			return err
		}
	}
	for _, raw := range h.SPS {
		if err := p.AddSPS(raw); err != nil {
			return err
		}
	}
	spsOf := func(id uint32) *SPS {
		if p.SPS == nil {
			return nil
		}
		return p.SPS[id]
	}
	for _, raw := range h.PPS {
		if err := p.AddPPS(raw, spsOf); err != nil {
			return err
		}
	}
	return nil
}

// AddVPS parses and stores one VPS NALU.
func (p *ParamSets) AddVPS(nalu []byte) error {
	v, err := ParseVPS(nalu)
	if err != nil {
		return err
	}
	if p.VPS == nil {
		p.VPS = map[uint32]*VPS{}
	}
	p.VPS[v.ID] = v
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

// AddPPS parses and stores one PPS NALU (SPS must be known first).
func (p *ParamSets) AddPPS(nalu []byte, spsOf func(id uint32) *SPS) error {
	if spsOf == nil {
		spsOf = func(id uint32) *SPS {
			if p.SPS == nil {
				return nil
			}
			return p.SPS[id]
		}
	}
	q, err := ParsePPS(nalu, spsOf)
	if err != nil {
		return err
	}
	if p.PPS == nil {
		p.PPS = map[uint32]*PPS{}
	}
	p.PPS[q.ID] = q
	return nil
}

// AddNALU stores VPS/SPS/PPS units and ignores the rest.
// It returns true when the unit was a parameter set.
func (p *ParamSets) AddNALU(nalu []byte) (bool, error) {
	t, ok := NALType(nalu)
	if !ok {
		return false, fmt.Errorf("%w: empty unit", ErrBadNALU)
	}
	switch t {
	case NALVPS:
		return true, p.AddVPS(nalu)
	case NALSPS:
		return true, p.AddSPS(nalu)
	case NALPPS:
		return true, p.AddPPS(nalu, nil)
	default:
		return false, nil
	}
}

// RequireForSlice checks the full chain a slice header points at:
// PPS -> SPS -> VPS. Missing sets mean the frame cannot be trusted.
func (p *ParamSets) RequireForSlice(ppsID uint32) (*PPS, *SPS, *VPS, error) {
	q, ok := p.PPS[ppsID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: pps %d", ErrMissingPPS, ppsID)
	}
	s, ok := p.SPS[q.SPSID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: sps %d for pps %d", ErrMissingSPS, q.SPSID, ppsID)
	}
	v, ok := p.VPS[s.VPSID]
	if !ok {
		return nil, nil, nil, fmt.Errorf("%w: vps %d for sps %d", ErrMissingVPS, s.VPSID, s.ID)
	}
	return q, s, v, nil
}

// HasVPS/HasSPS/HasPPS report whether at least one set is known.
func (p *ParamSets) HasVPS() bool { return p != nil && len(p.VPS) > 0 }
func (p *ParamSets) HasSPS() bool { return p != nil && len(p.SPS) > 0 }
func (p *ParamSets) HasPPS() bool { return p != nil && len(p.PPS) > 0 }
