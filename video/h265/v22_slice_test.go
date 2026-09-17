package h265

// V2-2 step 2: slice header parity. Each sample of the clip holds one
// coded slice (IDR + 4 P); the header carries type, POC, reference
// counts and quant switches the pixel stage consumes. This gate pins
// all five against the same clip's ffmpeg truth, without decoding
// coefficients yet (Decoder still refuses pixels; V2-1 stays green).
//
// Baseline: ../testdata/v2_ffmpeg.json (§12 V2 row). Trace source:
// `ffmpeg -i <clip> -c:v copy -bsf:v trace_headers -f null -`
// (Slice Segment Header dumps per packet).
//
// Peer (read-only, no code copied): libavcodec/hevc/hevcdec.c:774
// hls_slice_header + :175 pred_weight_table + decode_lt_rps +
// ps.c:2485 ff_hevc_compute_poc2 + hevc.h NAL enum + hevcdec.h:74
// IS_IRAP/IS_IDR.

import (
	"os"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

func loadStep2Headers(t *testing.T) (*HVCC, *ParamSets, [][]byte) {
	t.Helper()
	b := loadV2Baseline(t)
	clip := b.Clips[0]
	path := "../testdata/" + clip.File
	m, err := mp4.ParseFile(path)
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
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	var units [][]byte
	for _, s := range m.Video.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		u, err := SplitHVCC(buf, h.LengthSize)
		if err != nil {
			t.Fatalf("split %d: %v", s.Number, err)
		}
		if len(u) != 1 {
			t.Fatalf("sample %d units %d, want 1", s.Number, len(u))
		}
		units = append(units, u[0])
	}
	return h, ps, units
}

// TestV22SliceSequence pins framing: one slice per sample, IDR first,
// all VCL slice types.
func TestV22SliceSequence(t *testing.T) {
	_, _, units := loadStep2Headers(t)
	want := []int{NALIdrNLp, NALTrailR, NALTrailR, NALTrailR, NALTrailR}
	if len(units) != len(want) {
		t.Fatalf("slices %d, want %d", len(units), len(want))
	}
	for i, u := range units {
		typ, ok := NALType(u)
		if !ok {
			t.Fatalf("slice %d: no header", i)
		}
		if typ != want[i] {
			t.Fatalf("slice %d type %d (%s), want %d", i, typ, NALName(typ), want[i])
		}
		if !IsSlice(typ) {
			t.Fatalf("slice %d type %d not VCL", i, typ)
		}
	}
	if !IsIDR(want[0]) || IsIDR(want[1]) {
		t.Fatal("IRAP shape wrong: want IDR first only")
	}
}

// TestV22IDRHeader pins the intra frame's header: first slice, PPS 0,
// I type, POC 0 with no RPS on the wire, SAO on, QP 28 (26+0+2).
func TestV22IDRHeader(t *testing.T) {
	_, ps, units := loadStep2Headers(t)
	sh, err := ParseSliceHeader(units[0], ps, 0)
	if err != nil {
		t.Fatalf("idr: %v", err)
	}
	if !sh.First || sh.NoOutput || sh.PPSID != 0 {
		t.Fatalf("first %v noop %v pps %d, want true/false/0", sh.First, sh.NoOutput, sh.PPSID)
	}
	if sh.Type != SliceI {
		t.Fatalf("type %s, want I", SliceTypeName(sh.Type))
	}
	if sh.POC != 0 || sh.POCLsb != 0 || sh.RPS != nil || sh.LTRefs != 0 {
		t.Fatalf("poc %d lsb %d rps %v lt %d, want 0/0/nil/0", sh.POC, sh.POCLsb, sh.RPS, sh.LTRefs)
	}
	if !sh.SAOLuma || !sh.SAOChroma {
		t.Fatal("sao off, want luma+chroma on")
	}
	if sh.RefL0 != 0 || sh.RefL1 != 0 || sh.MergeCand != 0 {
		t.Fatalf("refs %d/%d merge %d, want 0/0/0", sh.RefL0, sh.RefL1, sh.MergeCand)
	}
	if sh.QpDelta != 2 || sh.SliceQP != 28 {
		t.Fatalf("qp delta %d slice %d, want 2/28", sh.QpDelta, sh.SliceQP)
	}
	if !sh.LoopAcross || !sh.PicOutput {
		t.Fatal("loop/output off, want on")
	}
}

// TestV22PHeaders pins the four inter headers: P type, POC 1..4 via
// chained pocTid0, explicit RPS growing 1..4 (all delta -1, all used),
// default refs on frame 1 then override 2/3/3, weight denoms 7/6 with
// all flags off, 3 merge cands, QP 28 throughout.
func TestV22PHeaders(t *testing.T) {
	_, ps, units := loadStep2Headers(t)
	wantNeg := []uint32{1, 2, 3, 4}
	wantRef := []uint32{1, 2, 3, 3}
	wantOver := []bool{false, true, true, true}
	pocTid0 := 0
	for i := 1; i <= 4; i++ {
		sh, err := ParseSliceHeader(units[i], ps, pocTid0)
		if err != nil {
			t.Fatalf("p%d: %v", i, err)
		}
		if sh.Type != SliceP {
			t.Fatalf("p%d type %s, want P", i, SliceTypeName(sh.Type))
		}
		if sh.POC != i || int(sh.POCLsb) != i {
			t.Fatalf("p%d poc %d lsb %d, want %d", i, sh.POC, sh.POCLsb, i)
		}
		pocTid0 = sh.POC
		if sh.RPSSPSFlag || sh.RPS == nil {
			t.Fatalf("p%d rps sps %v nil %v, want explicit", i, sh.RPSSPSFlag, sh.RPS == nil)
		}
		if sh.RPS.Neg != wantNeg[i-1] || sh.RPS.Pos != 0 {
			t.Fatalf("p%d rps %d+%d, want %d+0", i, sh.RPS.Neg, sh.RPS.Pos, wantNeg[i-1])
		}
		for k, d := range sh.RPS.DeltaPOC {
			if d != -1 || !sh.RPS.Used[k] {
				t.Fatalf("p%d rps[%d] %d/%v, want -1/true", i, k, d, sh.RPS.Used[k])
			}
		}
		if sh.RefL0 != wantRef[i-1] || sh.RefL1 != 0 || sh.Override != wantOver[i-1] {
			t.Fatalf("p%d refs %d/%d over %v, want %d/0/%v", i, sh.RefL0, sh.RefL1, sh.Override, wantRef[i-1], wantOver[i-1])
		}
		if sh.LumaDenom != 7 || sh.ChromaDenom != 6 {
			t.Fatalf("p%d denom %d/%d, want 7/6", i, sh.LumaDenom, sh.ChromaDenom)
		}
		for k := range sh.L0LumaUsed {
			if sh.L0LumaUsed[k] || sh.L0ChromaUsed[k] {
				t.Fatalf("p%d weight l0[%d] on, want off", i, k)
			}
		}
		if sh.MergeCand != 3 {
			t.Fatalf("p%d merge %d, want 3", i, sh.MergeCand)
		}
		if !sh.TemporalMVP || !sh.SAOLuma || !sh.SAOChroma {
			t.Fatalf("p%d mvp/sao off", i)
		}
		if sh.QpDelta != 2 || sh.SliceQP != 28 {
			t.Fatalf("p%d qp delta %d slice %d, want 2/28", i, sh.QpDelta, sh.SliceQP)
		}
	}
}

// TestV22SliceRefusal pins readable failure: non-slice NAL, unknown
// PPS and truncated input fail namably, never a panic.
func TestV22SliceRefusal(t *testing.T) {
	h, ps, units := loadStep2Headers(t)
	if _, err := ParseSliceHeader(h.VPS[0], ps, 0); err == nil {
		t.Fatal("VPS parses as slice")
	}
	bad := append([]byte(nil), units[1]...)
	if len(bad) < 4 {
		t.Fatal("slice too short to truncate")
	}
	if _, err := ParseSliceHeader(bad[:3], ps, 0); err == nil {
		t.Fatal("truncated slice parses")
	}
	// Unknown PPS: parameter sets carrying VPS/SPS but no PPS 0.
	bare := NewParamSets()
	for _, raw := range h.VPS {
		if err := bare.AddVPS(raw); err != nil {
			t.Fatalf("vps: %v", err)
		}
	}
	for _, raw := range h.SPS {
		if err := bare.AddSPS(raw); err != nil {
			t.Fatalf("sps: %v", err)
		}
	}
	if _, err := ParseSliceHeader(units[1], bare, 0); err == nil {
		t.Fatal("unknown PPS resolves")
	}
}
