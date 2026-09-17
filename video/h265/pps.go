package h265

import (
	"fmt"
)

// PPS is the parsed picture parameter set: slice defaults,chroma
// offsets, tile/wave flags for the V2-2 pixel stage.
// Peer (read-only): libavcodec/hevc/ps.c:2069 setup_pps (inferred
// tile defaults) + :2201 ff_hevc_decode_nal_pps (field order,
// default-active bounds, qp-delta depth gate, extension refusal).
type PPS struct {
	ID                 uint32
	SPSID              uint32
	DependentSlices    bool
	OutputFlagPresent  bool
	NumExtraBits       uint32
	SignHiding         bool
	CabacInitPresent   bool
	RefL0Default       uint32
	RefL1Default       uint32
	PicInitQP          int32
	ConstrainedIntra   bool
	TransformSkip      bool
	CUQPDelta          bool
	DiffQPDelayDepth   uint32
	CbQPOffset         int32
	CrQPOffset         int32
	SliceChromaOffsets bool
	WeightedPred       bool
	WeightedBiPred     bool
	TransquantBypass   bool
	TilesEnabled       bool
	EntropySync        bool
	LoopAcrossSlices   bool
	// Deblock control (peer ps.c setup_pps order): control-present
	// gates override-enabled/disable/beta/tc. Our clip sends
	// control-present 0 (deblock on, zero offsets); other clips
	// refuse honestly below.
	DeblockControl    bool
	DeblockOverrideEn bool
	DisableDbf        bool
	BetaOffset        int32
	TcOffset          int32
	ScalingPresent    bool
	ListsModPresent   bool
	Log2ParallelMerge uint32
	SliceExtPresent   bool
	// CrossCompPred enables cross-component chroma prediction (PPS
	// range extension; off on every clip we take since extensions
	// refuse below — the P pixel stage reads this gate).
	CrossCompPred bool
	ExtPresent    bool
	Raw           []byte
}

// ParsePPS parses one PPS NALU (2-byte HEVC header included). The SPS
// behind pps_seq_parameter_set_id must already be known so tile math
// and depth gates check against truth, like the peer's ps lookup.
func ParsePPS(nalu []byte, spsOf func(id uint32) *SPS) (*PPS, error) {
	typ, layer, _, err := nalHeader2(nalu)
	if err != nil {
		return nil, err
	}
	if typ != NALPPS {
		return nil, fmt.Errorf("%w: type %d is not PPS", ErrBadPPS, typ)
	}
	if layer != 0 {
		return nil, fmt.Errorf("%w: layer %d", ErrBadPPS, layer)
	}
	r := NewReader(UnescapeRBSP(nalu[2:]))
	id, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: pps id: %v", ErrBadPPS, err)
	}
	if id >= 64 {
		return nil, fmt.Errorf("%w: pps id %d", ErrBadPPS, id)
	}
	sid, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: sps id: %v", ErrBadPPS, err)
	}
	if sid >= 16 {
		return nil, fmt.Errorf("%w: sps id %d", ErrBadPPS, sid)
	}
	p := &PPS{ID: id, SPSID: sid}
	var sps *SPS
	if spsOf != nil {
		sps = spsOf(sid)
	}
	if sps == nil {
		return nil, fmt.Errorf("%w: sps %d", ErrMissingSPS, sid)
	}
	flag := func(name string) (bool, error) {
		b, err := r.ReadBit()
		if err != nil {
			return false, fmt.Errorf("%w: %s: %v", ErrBadPPS, name, err)
		}
		return b != 0, nil
	}
	if p.DependentSlices, err = flag("dependent slices"); err != nil {
		return nil, err
	}
	if p.OutputFlagPresent, err = flag("output flag"); err != nil {
		return nil, err
	}
	nb, err := r.ReadBits(3)
	if err != nil {
		return nil, fmt.Errorf("%w: extra bits: %v", ErrBadPPS, err)
	}
	p.NumExtraBits = nb
	if p.SignHiding, err = flag("sign hiding"); err != nil {
		return nil, err
	}
	if p.CabacInitPresent, err = flag("cabac init"); err != nil {
		return nil, err
	}
	l0, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ref l0: %v", ErrBadPPS, err)
	}
	l1, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ref l1: %v", ErrBadPPS, err)
	}
	p.RefL0Default, p.RefL1Default = l0+1, l1+1
	if p.RefL0Default >= 16 || p.RefL1Default >= 16 {
		return nil, fmt.Errorf("%w: default refs %d/%d", ErrBadPPS, p.RefL0Default, p.RefL1Default)
	}
	qp, err := r.ReadSE()
	if err != nil {
		return nil, fmt.Errorf("%w: init qp: %v", ErrBadPPS, err)
	}
	p.PicInitQP = qp
	if p.ConstrainedIntra, err = flag("constrained intra"); err != nil {
		return nil, err
	}
	if p.TransformSkip, err = flag("transform skip"); err != nil {
		return nil, err
	}
	cu, err := flag("cu qp delta")
	if err != nil {
		return nil, err
	}
	p.CUQPDelta = cu
	if cu {
		d, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: qp delta depth: %v", ErrBadPPS, err)
		}
		p.DiffQPDelayDepth = d
		maxDepth := sps.Log2MaxCB - sps.Log2MinCB
		if d > maxDepth {
			return nil, fmt.Errorf("%w: qp delta depth %d > %d", ErrBadPPS, d, maxDepth)
		}
	}
	cb, err := r.ReadSE()
	if err != nil {
		return nil, fmt.Errorf("%w: cb offset: %v", ErrBadPPS, err)
	}
	cr, err := r.ReadSE()
	if err != nil {
		return nil, fmt.Errorf("%w: cr offset: %v", ErrBadPPS, err)
	}
	p.CbQPOffset, p.CrQPOffset = cb, cr
	if p.SliceChromaOffsets, err = flag("slice chroma offsets"); err != nil {
		return nil, err
	}
	if p.WeightedPred, err = flag("weighted pred"); err != nil {
		return nil, err
	}
	if p.WeightedBiPred, err = flag("weighted bipred"); err != nil {
		return nil, err
	}
	if p.TransquantBypass, err = flag("transquant bypass"); err != nil {
		return nil, err
	}
	if p.TilesEnabled, err = flag("tiles"); err != nil {
		return nil, err
	}
	if p.EntropySync, err = flag("entropy sync"); err != nil {
		return nil, err
	}
	if p.TilesEnabled || p.EntropySync {
		return nil, fmt.Errorf("%w: tiles/wavefront not supported", ErrBadPPS)
	}
	if p.LoopAcrossSlices, err = flag("loop across slices"); err != nil {
		return nil, err
	}
	if p.DeblockControl, err = flag("deblock control"); err != nil {
		return nil, err
	}
	if p.DeblockControl {
		// Non-default deblock control stays refused (pixel stage
		// owns the override path next); this clip sends 0.
		return nil, fmt.Errorf("%w: deblock control not supported", ErrBadPPS)
	}
	if p.ScalingPresent, err = flag("scaling present"); err != nil {
		return nil, err
	}
	if p.ScalingPresent {
		return nil, fmt.Errorf("%w: scaling lists not supported", ErrBadPPS)
	}
	if p.ListsModPresent, err = flag("lists mod"); err != nil {
		return nil, err
	}
	pm, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: parallel merge: %v", ErrBadPPS, err)
	}
	p.Log2ParallelMerge = pm + 2
	if p.SliceExtPresent, err = flag("slice ext"); err != nil {
		return nil, err
	}
	if p.ExtPresent, err = flag("extension"); err != nil {
		return nil, err
	}
	if p.ExtPresent {
		return nil, fmt.Errorf("%w: pps extension not supported", ErrBadPPS)
	}
	p.Raw = append([]byte(nil), nalu...)
	return p, nil
}
