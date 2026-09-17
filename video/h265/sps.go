package h265

import (
	"fmt"
)

// SPS is the parsed sequence parameter set: coded/display size,
// block sizes for the V2-2 pixel stage, reference depth, VUI timing.
// Peer (read-only): libavcodec/hevc/ps.c:1239 ff_hevc_parse_sps
// (field order, conformance window, block-size gates, RPS skip,
// VUI + extension flags).
type SPS struct {
	ID              uint32
	VPSID           uint32
	MaxSubLayers    uint32
	ChromaFormat    uint32
	Width           uint32
	Height          uint32
	DispWidth       uint32
	DispHeight      uint32
	HasConformWin   bool
	WinLeft         uint32
	WinRight        uint32
	WinTop          uint32
	WinBottom       uint32
	BitDepth        uint32
	BitDepthChroma  uint32
	Log2MaxPOCLsb   uint32
	MaxBuffering    uint32
	NumReorder      uint32
	Log2MinCB       uint32
	Log2MaxCB       uint32
	Log2MinTB       uint32
	Log2MaxTB       uint32
	MaxTIDepthInter uint32
	MaxTIDepthIntra uint32
	AMPEnabled      bool
	SAOEnabled      bool
	NumShortTermRPS uint32
	TemporalMVP     bool
	SmoothIntra     bool
	VUIPresent      bool
	// VUI timing + color truth the player/color chain consume.
	SARWidth      uint32
	SARHeight     uint32
	FullRange     bool
	ColourPresent bool
	ColourMatrix  uint32
	TimingPresent bool
	NumUnitsTick  uint32
	TimeScale     uint32
	NumRefFrames  uint32
	// Long-term references configured in the SPS (empty on our clip;
	// the slice header's LT tail reads against these).
	LTPresent bool
	LTPoc     []uint32
	LTUsed    []bool
	PTL       *PTL
	Raw       []byte
}

// readWindow reads a conformance/default-display window (four ue(v)).
func readWindow(r *Reader, what error) (l, rr, t, b uint32, err error) {
	if l, err = r.ReadUE(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("%w: window left: %v", what, err)
	}
	if rr, err = r.ReadUE(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("%w: window right: %v", what, err)
	}
	if t, err = r.ReadUE(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("%w: window top: %v", what, err)
	}
	if b, err = r.ReadUE(); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("%w: window bottom: %v", what, err)
	}
	return l, rr, t, b, nil
}

// ParseSPS parses one SPS NALU (2-byte HEVC header included).
func ParseSPS(nalu []byte) (*SPS, error) {
	typ, layer, _, err := nalHeader2(nalu)
	if err != nil {
		return nil, err
	}
	if typ != NALSPS {
		return nil, fmt.Errorf("%w: type %d is not SPS", ErrBadSPS, typ)
	}
	if layer != 0 {
		return nil, fmt.Errorf("%w: layer %d", ErrBadSPS, layer)
	}
	r := NewReader(UnescapeRBSP(nalu[2:]))
	s := &SPS{}
	vpsID, err := r.ReadBits(4)
	if err != nil {
		return nil, fmt.Errorf("%w: vps id: %v", ErrBadSPS, err)
	}
	s.VPSID = vpsID
	ms, err := r.ReadBits(3)
	if err != nil {
		return nil, fmt.Errorf("%w: max sub layers: %v", ErrBadSPS, err)
	}
	s.MaxSubLayers = ms + 1
	if s.MaxSubLayers > 7 {
		return nil, fmt.Errorf("%w: sub layers %d", ErrBadSPS, s.MaxSubLayers)
	}
	if _, err := r.ReadBit(); err != nil { // temporal_id_nesting
		return nil, fmt.Errorf("%w: nesting: %v", ErrBadSPS, err)
	}
	ptl, err := readPTL(r, s.MaxSubLayers)
	if err != nil {
		return nil, err
	}
	s.PTL = ptl
	id, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: sps id: %v", ErrBadSPS, err)
	}
	if id >= 16 {
		return nil, fmt.Errorf("%w: sps id %d", ErrBadSPS, id)
	}
	s.ID = id
	chroma, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: chroma: %v", ErrBadSPS, err)
	}
	if chroma > 3 {
		return nil, fmt.Errorf("%w: chroma %d", ErrBadSPS, chroma)
	}
	s.ChromaFormat = chroma
	if chroma == 3 {
		if _, err := r.ReadBit(); err != nil { // separate plane
			return nil, fmt.Errorf("%w: separate plane: %v", ErrBadSPS, err)
		}
	}
	w, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: width: %v", ErrBadSPS, err)
	}
	h, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: height: %v", ErrBadSPS, err)
	}
	if w == 0 || h == 0 || w > 16888 || h > 16888 {
		return nil, fmt.Errorf("%w: size %dx%d", ErrBadSPS, w, h)
	}
	s.Width, s.Height = w, h
	cw, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: conformance flag: %v", ErrBadSPS, err)
	}
	if cw != 0 {
		s.HasConformWin = true
		l, rr, t, b, err := readWindow(r, ErrBadSPS)
		if err != nil {
			return nil, err
		}
		s.WinLeft, s.WinRight, s.WinTop, s.WinBottom = l, rr, t, b
	}
	bd, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: luma depth: %v", ErrBadSPS, err)
	}
	s.BitDepth = bd + 8
	if s.BitDepth > 16 {
		return nil, fmt.Errorf("%w: luma depth %d", ErrBadSPS, s.BitDepth)
	}
	bdc, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: chroma depth: %v", ErrBadSPS, err)
	}
	s.BitDepthChroma = bdc + 8
	if s.BitDepthChroma > 16 {
		return nil, fmt.Errorf("%w: chroma depth %d", ErrBadSPS, s.BitDepthChroma)
	}
	if chroma != 0 && s.BitDepthChroma != s.BitDepth {
		return nil, fmt.Errorf("%w: luma %d vs chroma %d", ErrBadSPS, s.BitDepth, s.BitDepthChroma)
	}
	poc, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: poc lsb: %v", ErrBadSPS, err)
	}
	s.Log2MaxPOCLsb = poc + 4
	if s.Log2MaxPOCLsb > 16 {
		return nil, fmt.Errorf("%w: poc lsb %d", ErrBadSPS, s.Log2MaxPOCLsb)
	}
	subOrd, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: ordering flag: %v", ErrBadSPS, err)
	}
	start := uint32(0)
	if subOrd == 0 {
		start = s.MaxSubLayers - 1
	}
	for i := start; i < s.MaxSubLayers; i++ {
		buf, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: buffering %d: %v", ErrBadSPS, i, err)
		}
		s.MaxBuffering = buf + 1
		if s.MaxBuffering == 0 || s.MaxBuffering > 16 {
			return nil, fmt.Errorf("%w: buffering %d", ErrBadSPS, s.MaxBuffering)
		}
		reo, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: reorder %d: %v", ErrBadSPS, i, err)
		}
		if reo > s.MaxBuffering-1 {
			return nil, fmt.Errorf("%w: reorder %d > buffering %d", ErrBadSPS, reo, s.MaxBuffering)
		}
		s.NumReorder = reo
		if _, err := r.ReadUE(); err != nil { // latency
			return nil, fmt.Errorf("%w: latency %d: %v", ErrBadSPS, i, err)
		}
	}
	// Inferred reference depth the V2-2 pixel stage sizes its DPB with.
	s.NumRefFrames = s.MaxBuffering
	minCB, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: min cb: %v", ErrBadSPS, err)
	}
	s.Log2MinCB = minCB + 3
	diffCB, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: diff cb: %v", ErrBadSPS, err)
	}
	if s.Log2MinCB < 3 || s.Log2MinCB > 30 || diffCB > 30 {
		return nil, fmt.Errorf("%w: cb sizes %d+%d", ErrBadSPS, s.Log2MinCB, diffCB)
	}
	s.Log2MaxCB = s.Log2MinCB + diffCB
	minTB, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: min tb: %v", ErrBadSPS, err)
	}
	s.Log2MinTB = minTB + 2
	diffTB, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: diff tb: %v", ErrBadSPS, err)
	}
	if s.Log2MinTB >= s.Log2MinCB || s.Log2MinTB < 2 || diffTB > 30 {
		return nil, fmt.Errorf("%w: tb sizes %d+%d", ErrBadSPS, s.Log2MinTB, diffTB)
	}
	s.Log2MaxTB = s.Log2MinTB + diffTB
	ti, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ti depth: %v", ErrBadSPS, err)
	}
	ta, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: ta depth: %v", ErrBadSPS, err)
	}
	s.MaxTIDepthInter, s.MaxTIDepthIntra = ti, ta
	scal, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: scaling flag: %v", ErrBadSPS, err)
	}
	if scal != 0 {
		present, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: scaling present: %v", ErrBadSPS, err)
		}
		if present != 0 {
			return nil, fmt.Errorf("%w: scaling lists not supported", ErrBadSPS)
		}
	}
	amp, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: amp: %v", ErrBadSPS, err)
	}
	s.AMPEnabled = amp != 0
	sao, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: sao: %v", ErrBadSPS, err)
	}
	s.SAOEnabled = sao != 0
	pcm, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: pcm flag: %v", ErrBadSPS, err)
	}
	if pcm != 0 {
		return nil, fmt.Errorf("%w: pcm not supported", ErrBadSPS)
	}
	nrps, err := r.ReadUE()
	if err != nil {
		return nil, fmt.Errorf("%w: st rps: %v", ErrBadSPS, err)
	}
	if nrps > 64 {
		return nil, fmt.Errorf("%w: st rps %d", ErrBadSPS, nrps)
	}
	s.NumShortTermRPS = nrps
	// RPS bodies are reference-list truth for V2-2 step 3 (P/B);
	// step 1 skips them with the same field order as the peer so
	// multi-RPS clips stay byte-aligned. Predicted sets need the
	// previous set's picture count, hence the running list.
	prevCounts := make([]uint32, 0, nrps)
	for i := uint32(0); i < nrps; i++ {
		n, err := skipShortTermRPS(r, i, prevCounts)
		if err != nil {
			return nil, err
		}
		prevCounts = append(prevCounts, n)
	}
	lt, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: lt flag: %v", ErrBadSPS, err)
	}
	if lt != 0 {
		nlt, err := r.ReadUE()
		if err != nil {
			return nil, fmt.Errorf("%w: lt count: %v", ErrBadSPS, err)
		}
		if nlt > 32 {
			return nil, fmt.Errorf("%w: lt %d", ErrBadSPS, nlt)
		}
		s.LTPresent = true
		for i := uint32(0); i < nlt; i++ {
			poc, err := r.ReadBits(int(s.Log2MaxPOCLsb))
			if err != nil {
				return nil, fmt.Errorf("%w: lt poc %d: %v", ErrBadSPS, i, err)
			}
			u, err := r.ReadBit()
			if err != nil {
				return nil, fmt.Errorf("%w: lt used %d: %v", ErrBadSPS, i, err)
			}
			s.LTPoc = append(s.LTPoc, poc)
			s.LTUsed = append(s.LTUsed, u != 0)
		}
	}
	tm, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: temporal mvp: %v", ErrBadSPS, err)
	}
	s.TemporalMVP = tm != 0
	sm, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: smoothing: %v", ErrBadSPS, err)
	}
	s.SmoothIntra = sm != 0
	vp, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: vui flag: %v", ErrBadSPS, err)
	}
	if vp != 0 {
		s.VUIPresent = true
		if err := parseVUI(r, s); err != nil {
			return nil, err
		}
	}
	ext, err := r.ReadBit()
	if err != nil {
		return nil, fmt.Errorf("%w: extension flag: %v", ErrBadSPS, err)
	}
	if ext != 0 {
		bits := []string{"range", "multilayer", "3d", "scc"}
		vals := make([]uint32, 4)
		for i, n := range bits {
			v, err := r.ReadBit()
			if err != nil {
				return nil, fmt.Errorf("%w: ext %s: %v", ErrBadSPS, n, err)
			}
			vals[i] = v
		}
		if _, err := r.ReadBits(4); err != nil {
			return nil, fmt.Errorf("%w: ext reserved: %v", ErrBadSPS, err)
		}
		for i, v := range vals {
			if v != 0 {
				return nil, fmt.Errorf("%w: %s extension not supported", ErrBadSPS, bits[i])
			}
		}
	}
	// Display size: conformance window crops the coded size in chroma
	// units (same rule as H.264 cropUnit for 4:2:0).
	dw, dh := w, h
	if s.HasConformWin {
		ux, uy := uint32(1), uint32(1)
		switch chroma {
		case 1:
			ux, uy = 2, 2
		case 2:
			ux, uy = 2, 1
		}
		if (s.WinLeft+s.WinRight)*ux >= w || (s.WinTop+s.WinBottom)*uy >= h {
			return nil, fmt.Errorf("%w: window bigger than picture", ErrBadSPS)
		}
		dw = w - (s.WinLeft+s.WinRight)*ux
		dh = h - (s.WinTop+s.WinBottom)*uy
	}
	s.DispWidth, s.DispHeight = dw, dh
	s.Raw = append([]byte(nil), nalu...)
	return s, nil
}

// skipShortTermRPS consumes one st_ref_pic_set body and reports its
// picture count for the next predicted set.
// Peer: ps.c:113 ff_hevc_decode_short_term_rps (predicted flag,
// delta chain, used/delta_poc reads). SPS path only (slice headers
// own their delta_idx variant in step 3).
func skipShortTermRPS(r *Reader, idx uint32, prev []uint32) (uint32, error) {
	var inter uint32
	if idx != 0 {
		b, err := r.ReadBit()
		if err != nil {
			return 0, fmt.Errorf("%w: rps %d inter: %v", ErrBadSPS, idx, err)
		}
		inter = b
	}
	if inter != 0 {
		if int(idx-1) >= len(prev) {
			return 0, fmt.Errorf("%w: rps %d no predicted set", ErrBadSPS, idx)
		}
		sign, err := r.ReadBit()
		if err != nil {
			return 0, fmt.Errorf("%w: rps %d delta sign: %v", ErrBadSPS, idx, err)
		}
		_ = sign
		delta, err := r.ReadUE()
		if err != nil {
			return 0, fmt.Errorf("%w: rps %d delta: %v", ErrBadSPS, idx, err)
		}
		if delta+1 > 32768 {
			return 0, fmt.Errorf("%w: rps %d delta %d", ErrBadSPS, idx, delta+1)
		}
		// One used bit per picture of the predicted set plus one.
		n := prev[idx-1] + 1
		if n > 32 {
			return 0, fmt.Errorf("%w: rps %d pred pics %d", ErrBadSPS, idx, n)
		}
		var kept uint32
		for i := uint32(0); i < n; i++ {
			used, err := r.ReadBit()
			if err != nil {
				return 0, fmt.Errorf("%w: rps %d pred used %d: %v", ErrBadSPS, idx, i, err)
			}
			useDelta := uint32(0)
			if used == 0 {
				ud, err := r.ReadBit()
				if err != nil {
					return 0, fmt.Errorf("%w: rps %d pred delta %d: %v", ErrBadSPS, idx, i, err)
				}
				useDelta = ud
			}
			if used != 0 || useDelta != 0 {
				kept++
			}
		}
		if kept >= 32 {
			return 0, fmt.Errorf("%w: rps %d pics %d", ErrBadSPS, idx, kept)
		}
		return kept, nil
	}
	neg, err := r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("%w: rps %d neg: %v", ErrBadSPS, idx, err)
	}
	pos, err := r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("%w: rps %d pos: %v", ErrBadSPS, idx, err)
	}
	if neg >= 16 || pos >= 16 {
		return 0, fmt.Errorf("%w: rps %d pics %d+%d", ErrBadSPS, idx, neg, pos)
	}
	for i := uint32(0); i < neg+pos; i++ {
		d, err := r.ReadUE() // delta_poc_minus1
		if err != nil {
			return 0, fmt.Errorf("%w: rps %d poc %d: %v", ErrBadSPS, idx, i, err)
		}
		if d+1 < 1 || d+1 > 32768 {
			return 0, fmt.Errorf("%w: rps %d poc %d bad %d", ErrBadSPS, idx, i, d+1)
		}
		if _, err := r.ReadBit(); err != nil { // used
			return 0, fmt.Errorf("%w: rps %d used %d: %v", ErrBadSPS, idx, i, err)
		}
	}
	return neg + pos, nil
}

// parseVUI reads the SPS VUI: common color/timing truth plus HEVC tail.
// Peer (read-only): h2645_vui.c:37 ff_h2645_decode_common_vui_params
// (aspect/video-signal/chroma-loc order) + ps.c:961 decode_vui
// (neutra/field flags, display window, timing, restriction).
func parseVUI(r *Reader, s *SPS) error {
	ar, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui aspect flag: %v", ErrBadSPS, err)
	}
	if ar != 0 {
		idc, err := r.ReadBits(8)
		if err != nil {
			return fmt.Errorf("%w: vui aspect idc: %v", ErrBadSPS, err)
		}
		if idc == 255 {
			w, err := r.ReadBits(16)
			if err != nil {
				return fmt.Errorf("%w: vui sar w: %v", ErrBadSPS, err)
			}
			h, err := r.ReadBits(16)
			if err != nil {
				return fmt.Errorf("%w: vui sar h: %v", ErrBadSPS, err)
			}
			s.SARWidth, s.SARHeight = w, h
		} else {
			s.SARWidth, s.SARHeight = idc, 1
		}
	}
	over, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui overscan flag: %v", ErrBadSPS, err)
	}
	if over != 0 {
		if _, err := r.ReadBit(); err != nil {
			return fmt.Errorf("%w: vui overscan: %v", ErrBadSPS, err)
		}
	}
	sig, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui signal flag: %v", ErrBadSPS, err)
	}
	if sig != 0 {
		if _, err := r.ReadBits(3); err != nil {
			return fmt.Errorf("%w: vui format: %v", ErrBadSPS, err)
		}
		fr, err := r.ReadBit()
		if err != nil {
			return fmt.Errorf("%w: vui range: %v", ErrBadSPS, err)
		}
		s.FullRange = fr != 0
		cd, err := r.ReadBit()
		if err != nil {
			return fmt.Errorf("%w: vui colour flag: %v", ErrBadSPS, err)
		}
		if cd != 0 {
			s.ColourPresent = true
			if _, err := r.ReadBits(8); err != nil {
				return fmt.Errorf("%w: vui primaries: %v", ErrBadSPS, err)
			}
			if _, err := r.ReadBits(8); err != nil {
				return fmt.Errorf("%w: vui transfer: %v", ErrBadSPS, err)
			}
			mc, err := r.ReadBits(8)
			if err != nil {
				return fmt.Errorf("%w: vui matrix: %v", ErrBadSPS, err)
			}
			s.ColourMatrix = mc
		}
	}
	cl, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui chroma loc flag: %v", ErrBadSPS, err)
	}
	if cl != 0 {
		if _, err := r.ReadUE(); err != nil {
			return fmt.Errorf("%w: vui chroma top: %v", ErrBadSPS, err)
		}
		if _, err := r.ReadUE(); err != nil {
			return fmt.Errorf("%w: vui chroma bottom: %v", ErrBadSPS, err)
		}
	}
	for _, n := range []string{"neutra", "fieldseq", "fieldinfo"} {
		if _, err := r.ReadBit(); err != nil {
			return fmt.Errorf("%w: vui %s: %v", ErrBadSPS, n, err)
		}
	}
	ddw, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui display window: %v", ErrBadSPS, err)
	}
	if ddw != 0 {
		l, rr, t, b, err := readWindow(r, ErrBadSPS)
		if err != nil {
			return err
		}
		_ = l
		_ = rr
		_ = t
		_ = b
	}
	ti, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui timing flag: %v", ErrBadSPS, err)
	}
	if ti != 0 {
		s.TimingPresent = true
		tick, err := r.ReadBits(32)
		if err != nil {
			return fmt.Errorf("%w: vui tick: %v", ErrBadSPS, err)
		}
		scale, err := r.ReadBits(32)
		if err != nil {
			return fmt.Errorf("%w: vui scale: %v", ErrBadSPS, err)
		}
		s.NumUnitsTick, s.TimeScale = tick, scale
		poc, err := r.ReadBit()
		if err != nil {
			return fmt.Errorf("%w: vui poc: %v", ErrBadSPS, err)
		}
		if poc != 0 {
			if _, err := r.ReadUE(); err != nil {
				return fmt.Errorf("%w: vui poc diff: %v", ErrBadSPS, err)
			}
		}
		hrd, err := r.ReadBit()
		if err != nil {
			return fmt.Errorf("%w: vui hrd: %v", ErrBadSPS, err)
		}
		if hrd != 0 {
			return fmt.Errorf("%w: vui hrd not supported", ErrBadSPS)
		}
	}
	rest, err := r.ReadBit()
	if err != nil {
		return fmt.Errorf("%w: vui restriction: %v", ErrBadSPS, err)
	}
	if rest != 0 {
		for _, n := range []string{"tiles", "mvbound", "reflists"} {
			if _, err := r.ReadBit(); err != nil {
				return fmt.Errorf("%w: vui %s: %v", ErrBadSPS, n, err)
			}
		}
		for _, n := range []string{"segidc", "bytesdenom", "bitsdenom", "mvlenh", "mvlenv"} {
			if _, err := r.ReadUE(); err != nil {
				return fmt.Errorf("%w: vui %s: %v", ErrBadSPS, n, err)
			}
		}
	}
	return nil
}
