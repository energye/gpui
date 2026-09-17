package h265

import (
	"fmt"
)

// VPS is the parsed video parameter set: layering, DPB depth for the
// V2-2 pixel stage, and timing for the clock.
// Peer (read-only): libavcodec/hevc/ps.c:786 ff_hevc_decode_nal_vps
// (field order, reserved ffff check, buffering/reorder/latency loops,
// timing + hrd skip, extension honest refusal).
type VPS struct {
	ID            uint32
	MaxLayers     uint32
	MaxSubLayers  uint32
	NestingFlag   bool
	PTL           *PTL
	MaxBuffering  uint32
	NumReorder    uint32
	MaxLatency    int32
	MaxLayerID    uint32
	TimingPresent bool
	NumUnitsTick  uint32
	TimeScale     uint32
	Raw           []byte
}

// ParseVPS parses one VPS NALU (2-byte HEVC header included).
func ParseVPS(nalu []byte) (*VPS, error) {
	typ, layer, _, err := nalHeader2(nalu)
	if err != nil {
		return nil, err
	}
	if typ != NALVPS {
		return nil, fmt.Errorf("%w: type %d is not VPS", ErrBadVPS, typ)
	}
	if layer != 0 {
		return nil, fmt.Errorf("%w: layer %d", ErrBadVPS, layer)
	}
	r := NewReader(UnescapeRBSP(nalu[2:]))
	id, err := r.ReadBits(4)
	if err != nil {
		return nil, fmt.Errorf("%w: id: %v", ErrBadVPS, err)
	}
	v := &VPS{ID: id}
	baseIn, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: base internal: %v", ErrBadVPS, err)
	}
	baseAv, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: base available: %v", ErrBadVPS, err)
	}
	if baseIn == 0 || baseAv == 0 {
		return nil, fmt.Errorf("%w: base layer flags %d/%d", ErrBadVPS, baseIn, baseAv)
	}
	ml, err := r.ReadBits(6)
	if err != nil {
		return nil, fmt.Errorf("%w: max layers: %v", ErrBadVPS, err)
	}
	v.MaxLayers = ml + 1
	ms, err := r.ReadBits(3)
	if err != nil {
		return nil, fmt.Errorf("%w: max sub layers: %v", ErrBadVPS, err)
	}
	v.MaxSubLayers = ms + 1
	if v.MaxSubLayers > 7 {
		return nil, fmt.Errorf("%w: sub layers %d", ErrBadVPS, v.MaxSubLayers)
	}
	nest, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: nesting: %v", ErrBadVPS, err)
	}
	v.NestingFlag = nest != 0
	res, err := r.ReadBits(16)
	if err != nil {
		return nil, fmt.Errorf("%w: reserved: %v", ErrBadVPS, err)
	}
	if res != 0xffff {
		return nil, fmt.Errorf("%w: reserved ffff is %#x", ErrBadVPS, res)
	}
	ptl, err := readPTL(r, v.MaxSubLayers)
	if err != nil {
		// PTL errors carry the SPS bucket from the shared reader;
		// relabel them to VPS so Classify keeps the layer.
		return nil, fmt.Errorf("%w: ptl: %v", ErrBadVPS, err)
	}
	v.PTL = ptl
	subFlag, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: ordering flag: %v", ErrBadVPS, err)
	}
	start := uint32(0)
	if subFlag == 0 {
		start = v.MaxSubLayers - 1
	}
	for i := start; i < v.MaxSubLayers; i++ {
		buf, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: buffering %d: %v", ErrBadVPS, i, err)
		}
		v.MaxBuffering = buf + 1
		if v.MaxBuffering == 0 || v.MaxBuffering > 16 {
			return nil, fmt.Errorf("%w: buffering %d", ErrBadVPS, v.MaxBuffering)
		}
		reo, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: reorder %d: %v", ErrBadVPS, i, err)
		}
		if reo > v.MaxBuffering-1 {
			return nil, fmt.Errorf("%w: reorder %d > buffering %d", ErrBadVPS, reo, v.MaxBuffering)
		}
		v.NumReorder = reo
		lat, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: latency %d: %v", ErrBadVPS, i, err)
		}
		v.MaxLatency = int32(lat) - 1
	}
	lid, err := r.ReadBits(6)
	if err != nil {
		return nil, fmt.Errorf("%w: layer id: %v", ErrBadVPS, err)
	}
	v.MaxLayerID = lid
	nsets, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: layer sets: %v", ErrBadVPS, err)
	}
	nsets++
	if nsets < 1 || nsets > 1024 || int64(nsets-1)*int64(v.MaxLayerID+1) > int64(r.BitsLeft()) {
		return nil, fmt.Errorf("%w: layer sets %d", ErrBadVPS, nsets)
	}
	// Single base layer: first set is implicit, extra sets are skipped.
	for s := uint32(1); s < nsets; s++ {
		for l := uint32(0); l <= v.MaxLayerID; l++ {
			if _, err := r.ReadBit(); err != nil {
				return nil, fmt.Errorf("%w: layer set %d: %v", ErrBadVPS, s, err)
			}
		}
	}
	tp, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: timing flag: %v", ErrBadVPS, err)
	}
	if tp != 0 {
		v.TimingPresent = true
		tick, err := r.ReadBits(32)
		if err != nil {
			return nil, fmt.Errorf("%w: tick: %v", ErrBadVPS, err)
		}
		scale, err := r.ReadBits(32)
		if err != nil {
			return nil, fmt.Errorf("%w: scale: %v", ErrBadVPS, err)
		}
		v.NumUnitsTick = tick
		v.TimeScale = scale
		poc, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: poc timing: %v", ErrBadVPS, err)
		}
		if poc != 0 {
			pd, err := r.ReadUE()
			if err != nil {
				return nil, fmt.Errorf("%w: poc diff: %v", ErrBadVPS, err)
			}
			_ = pd + 1
		}
		nhrd, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: hrd count: %v", ErrBadVPS, err)
		}
		if nhrd > nsets {
			return nil, fmt.Errorf("%w: hrd %d > sets %d", ErrBadVPS, nhrd, nsets)
		}
		// HRD bodies are clock tuning, not pixel truth: skip only when
		// a clip actually carries them (ours carry none).
		if nhrd > 0 {
			return nil, fmt.Errorf("%w: hrd bodies not supported", ErrBadVPS)
		}
	}
	if v.MaxLayers > 1 {
		ext, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: extension flag: %v", ErrBadVPS, err)
		}
		if ext != 0 {
			return nil, fmt.Errorf("%w: multi-layer extension not supported", ErrBadVPS)
		}
	}
	v.Raw = append([]byte(nil), nalu...)
	return v, nil
}
