package h265

// V2-2 step 1: VPS/SPS/PPS read-full parity. The hvcC carries only
// profile/level/length; the real width, block sizes and reference
// depth live inside the parameter sets. This gate pins them against
// the same clip's ffmpeg truth, without touching pixels.
//
// Baseline: ../testdata/v2_ffmpeg.json (ffprobe 4.4.2 + ffmpeg 4.4.2,
// §12 V2 row). Trace source: `ffmpeg -i <clip> -c:v copy
// -bsf:v trace_headers -f null -` (VPS/SPS/PPS field dumps).
//
// Peer (read-only, no code copied): libavcodec/hevc/ps.c:786
// ff_hevc_decode_nal_vps + :1239 ff_hevc_parse_sps + :2201
// ff_hevc_decode_nal_pps + :262 decode_profile_tier_level +
// :113 ff_hevc_decode_short_term_rps + h2645_vui.c:37 common VUI.

import (
	"testing"

	"github.com/energye/gpui/video/mp4"
)

func loadStep1Sets(t *testing.T) (*HVCC, *ParamSets) {
	t.Helper()
	b := loadV2Baseline(t)
	clip := b.Clips[0]
	m, err := mp4.ParseFile("../testdata/" + clip.File)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	h, err := ParseHVCC(m.Video.HEVCConfig)
	if err != nil {
		t.Fatalf("hvcC: %v", err)
	}
	ps := NewParamSets()
	if err := ps.FromHVCC(h); err != nil {
		t.Fatalf("param sets: %v", err)
	}
	return h, ps
}

// TestV22VPSFull pins VPS truth: single base layer, Main profile,
// level 30, DPB 5 / reorder 2 (trace_headers VPS dump).
func TestV22VPSFull(t *testing.T) {
	_, ps := loadStep1Sets(t)
	v, ok := ps.VPS[0]
	if !ok {
		t.Fatal("no VPS 0")
	}
	if v.MaxLayers != 1 || v.MaxSubLayers != 1 {
		t.Fatalf("layers %d sub %d, want 1/1", v.MaxLayers, v.MaxSubLayers)
	}
	if v.PTL == nil || v.PTL.ProfileIDC != 1 || v.PTL.LevelIDC != 30 {
		t.Fatalf("ptl %+v, want profile 1 level 30", v.PTL)
	}
	if v.MaxBuffering != 5 || v.NumReorder != 2 {
		t.Fatalf("dpb %d reorder %d, want 5/2", v.MaxBuffering, v.NumReorder)
	}
	if v.TimingPresent {
		t.Fatal("VPS timing present, want absent")
	}
}

// TestV22SPSFull pins SPS truth: 8-bit 4:2:0, coded 96x96 = display
// 96x96 (no conformance crop), CTB 64 (log2 3..6), TB 4..32,
// reference depth 5, SAO on, no short-term RPS, VUI SAR 1:1 +
// timing 1/5 (trace_headers SPS dump + v2_ffmpeg 5fps).
func TestV22SPSFull(t *testing.T) {
	_, ps := loadStep1Sets(t)
	s, ok := ps.SPS[0]
	if !ok {
		t.Fatal("no SPS 0")
	}
	if s.ChromaFormat != 1 || s.BitDepth != 8 || s.BitDepthChroma != 8 {
		t.Fatalf("chroma %d depth %d/%d, want 1 8/8", s.ChromaFormat, s.BitDepth, s.BitDepthChroma)
	}
	if s.Width != 96 || s.Height != 96 || s.DispWidth != 96 || s.DispHeight != 96 {
		t.Fatalf("coded %dx%d disp %dx%d, want 96x96/96x96", s.Width, s.Height, s.DispWidth, s.DispHeight)
	}
	if s.HasConformWin {
		t.Fatal("conformance window present, want absent")
	}
	if s.Log2MinCB != 3 || s.Log2MaxCB != 6 {
		t.Fatalf("cb %d..%d, want 3..6", s.Log2MinCB, s.Log2MaxCB)
	}
	if s.Log2MinTB != 2 || s.Log2MaxTB != 5 {
		t.Fatalf("tb %d..%d, want 2..5", s.Log2MinTB, s.Log2MaxTB)
	}
	if s.NumRefFrames != 5 || s.MaxBuffering != 5 || s.NumReorder != 2 {
		t.Fatalf("refs %d buf %d reo %d, want 5/5/2", s.NumRefFrames, s.MaxBuffering, s.NumReorder)
	}
	if s.AMPEnabled || !s.SAOEnabled {
		t.Fatalf("amp %v sao %v, want false/true", s.AMPEnabled, s.SAOEnabled)
	}
	if s.NumShortTermRPS != 0 {
		t.Fatalf("st rps %d, want 0", s.NumShortTermRPS)
	}
	if !s.VUIPresent || s.SARWidth != 1 || s.SARHeight != 1 {
		t.Fatalf("vui %v sar %d:%d, want true 1:1", s.VUIPresent, s.SARWidth, s.SARHeight)
	}
	if !s.TimingPresent || s.NumUnitsTick != 1 || s.TimeScale != 5 {
		t.Fatalf("timing %v %d/%d, want true 1/5", s.TimingPresent, s.NumUnitsTick, s.TimeScale)
	}
	if s.Log2MaxPOCLsb != 8 {
		t.Fatalf("poc lsb %d, want 8", s.Log2MaxPOCLsb)
	}
}

// TestV22PPSFull pins PPS truth: SPS 0 chain, default refs 1/1,
// weighted-pred on (trace shows weighted_pred_flag 1), tiles and
// wavefront off, parallel-merge 2 (trace_headers PPS dump).
func TestV22PPSFull(t *testing.T) {
	_, ps := loadStep1Sets(t)
	q, ok := ps.PPS[0]
	if !ok {
		t.Fatal("no PPS 0")
	}
	if q.SPSID != 0 {
		t.Fatalf("sps %d, want 0", q.SPSID)
	}
	if q.RefL0Default != 1 || q.RefL1Default != 1 {
		t.Fatalf("refs %d/%d, want 1/1", q.RefL0Default, q.RefL1Default)
	}
	if !q.WeightedPred || q.WeightedBiPred {
		t.Fatalf("weighted %v/%v, want true/false", q.WeightedPred, q.WeightedBiPred)
	}
	if q.TilesEnabled || q.EntropySync {
		t.Fatal("tiles/wavefront present, want off")
	}
	if q.Log2ParallelMerge != 2 {
		t.Fatalf("merge %d, want 2", q.Log2ParallelMerge)
	}
	if _, _, _, err := ps.RequireForSlice(0); err != nil {
		t.Fatalf("chain: %v", err)
	}
}

// TestV22ChainRefusal pins readable failure: unknown PPS, PPS->missing
// SPS, SPS->missing VPS each fail with the namable bucket, never a panic.
func TestV22ChainRefusal(t *testing.T) {
	_, ps := loadStep1Sets(t)
	if _, _, _, err := ps.RequireForSlice(99); err == nil {
		t.Fatal("unknown PPS resolves")
	}
	// PPS 1 -> SPS 7 (absent).
	ps.PPS[1] = &PPS{ID: 1, SPSID: 7}
	if _, _, _, err := ps.RequireForSlice(1); err == nil {
		t.Fatal("missing SPS resolves")
	}
	delete(ps.PPS, 1)
	// SPS 7 -> VPS 7 (absent), via PPS 2.
	ps.SPS[7] = &SPS{ID: 7, VPSID: 7}
	ps.PPS[2] = &PPS{ID: 2, SPSID: 7}
	if _, _, _, err := ps.RequireForSlice(2); err == nil {
		t.Fatal("missing VPS resolves")
	}
}
