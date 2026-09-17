package h265

// V2-2 step 3: I-frame byte-exact syntax gate. The first sample (IDR) must
// walk from the slice header to the last CTB's end_of_slice flag with no
// refusal, landing exactly where the reference decoder lands.
//
// Numbers below were verified bin-by-bin against a pristine ffmpeg build
// (hevc/cabac.c + hevcdec.c, only logging added): all 10218 context and
// bypass bins identical, 117/117 CUs identical (position/size/part plus
// every coded intra mode). Three bugs died for this (all found by BIN
// diffing, ideas only, no code copied):
//   1. merged SAO CTBs read type_idx bins the peer never reads (its
//      SET_SAO macro args stay unevaluated on merge) — symptom: split
//      at (64,0,16) flipped 1->0 two bins later;
//   2. last-significant X suffix was read before the Y prefix; the peer
//      reads both prefixes first (ff_hevc_hls_residual_coding);
//   3. the tail is NOT textbook stop-bit-plus-zeros on real x265 output
//      (this clip ends ...57 FF, fresh encodes end mid-0x5E), and the
//      peer never validates it — so the gate pins the exact stop point
//      (consumed/payload/firstOne) instead of demanding zeros.
//
// Baseline: ../testdata/v2_ffmpeg.json (same clip as the header gates).
// Peer (read-only): hevcdec.c hls_coding_quadtree/hls_coding_unit/
// hls_transform_tree/hls_transform_unit/intra_prediction_unit/
// hls_sao_param + cabac.c ff_hevc_hls_residual_coding and the syntax
// decoders. P slices parse (CU/PU/motion + residual gate); B stays
// refused until the B stage.

import (
	"os"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

func loadIDR(t *testing.T) (*ParamSets, []byte) {
	t.Helper()
	b := loadV2Baseline(t)
	path := "../testdata/" + b.Clips[0].File
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
	s := m.Video.Samples[0]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		t.Fatalf("sample: %v", err)
	}
	u, err := SplitHVCC(buf, h.LengthSize)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	for _, nalu := range u {
		typ, ok := NALType(nalu)
		if !ok || !IsSlice(typ) {
			continue
		}
		return ps, nalu
	}
	t.Fatal("no slice in first sample")
	return nil, nil
}

// TestV22IDRExact pins the full IDR walk: no refusal, 4 CTBs, 117 CUs,
// 232 stored residuals, CABAC stopping at bit 9663 of 9672 (first tail
// bit 9663: encoder 0xFF pad, see note above), plus the three structural
// witnesses (first CU, the (64,0) split node, last CU) and the SAO merge
// that caught bug 1.
func TestV22IDRExact(t *testing.T) {
	ps, nalu := loadIDR(t)
	fs, err := ParseFrameSyntax(nalu, ps, 0)
	if err != nil {
		t.Fatalf("idr: %v", err)
	}
	if fs.CTUs != 4 {
		t.Fatalf("CTUs %d, want 4", fs.CTUs)
	}
	if len(fs.CUs) != 117 {
		t.Fatalf("CUs %d, want 117", len(fs.CUs))
	}
	if len(fs.TUs) != 232 {
		t.Fatalf("TUs %d, want 232", len(fs.TUs))
	}
	if fs.Consumed != 9663 || fs.PayloadBits != 9672 || fs.TailFirstOne != 9663 {
		t.Fatalf("stop %d/%d firstOne %d, want 9663/9672/9663",
			fs.Consumed, fs.PayloadBits, fs.TailFirstOne)
	}
	c0 := fs.CUs[0]
	if c0.X0 != 0 || c0.Y0 != 0 || c0.Log2Size != 3 || c0.Part != Part2Nx2N ||
		c0.Luma[0] != 0 || c0.ChromaLuma != 0 {
		t.Fatalf("CU0 %+v, want (0,0,8) 2Nx2N mode0 chroma0", c0)
	}
	c40 := fs.CUs[40]
	if c40.X0 != 64 || c40.Y0 != 0 || c40.Log2Size != 3 || c40.Part != PartNxN ||
		c40.Luma != [4]uint8{14, 14, 3, 15} || c40.ChromaLuma != 14 {
		t.Fatalf("CU40 %+v, want (64,0,8) NxN modes 14/14/3/15 chroma14", c40)
	}
	cl := fs.CUs[len(fs.CUs)-1]
	if cl.X0 != 88 || cl.Y0 != 88 || cl.Log2Size != 3 || cl.Luma[0] != 26 ||
		cl.ChromaLuma != 10 {
		t.Fatalf("last %+v, want (88,88,8) mode26 chroma10", cl)
	}
	if len(fs.SAO) != 4 {
		t.Fatalf("SAO %d entries, want 4", len(fs.SAO))
	}
	if fs.SAO[0].Type != [3]uint8{SAOEdge, SAOEdge, SAOEdge} {
		t.Fatalf("SAO0 %+v, want edge/edge/edge", fs.SAO[0].Type)
	}
	if !fs.SAO[1].MergeLeft {
		t.Fatal("SAO1 merge left off, want on")
	}
}

// TestV22PSyntaxCounts pins the P-slice walk: all 4 P frames parse with
// the peer's CU/PU/leaf counts (verified CU-by-CU against a pristine
// ffmpeg build: hevcdec.c hls_coding_unit/prediction_unit + cabac.c
// skip/merge/ref/mvd decoders, ideas only, no code copied).
// P1 57/57 (skip44 inter13, 10 leaves, stop 757/768 firstOne 757),
// P2 51/51 (skip41 inter10, 10 leaves, stop 722/736 firstOne 723),
// P3 51/51 (skip39 inter12, 12 leaves, stop 627/640 firstOne 630),
// P4 51/51 (skip41 inter10, 10 leaves, stop 595/608 firstOne 595).
// B slices still refuse with the namable pixel-stage error.
func TestV22PSyntaxCounts(t *testing.T) {
	b := loadV2Baseline(t)
	path := "../testdata/" + b.Clips[0].File
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
	want := []struct {
		cus, pus, skip, inter, leaves int
		stop, payload, firstOne       int
	}{
		{57, 57, 44, 13, 10, 757, 768, 757},
		{51, 51, 41, 10, 10, 722, 736, 723},
		{51, 51, 39, 12, 12, 627, 640, 630},
		{51, 51, 41, 10, 10, 595, 608, 595},
	}
	for i, w := range want {
		s := m.Video.Samples[i+1]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("P%d sample: %v", i+1, err)
		}
		u, err := SplitHVCC(buf, h.LengthSize)
		if err != nil {
			t.Fatalf("P%d split: %v", i+1, err)
		}
		fs, err := ParseFrameSyntax(u[0], ps, i)
		if err != nil {
			t.Fatalf("P%d: %v", i+1, err)
		}
		npu, nskip, ninter := 0, 0, 0
		for _, c := range fs.CUs {
			npu += len(c.PUs)
			switch c.Pred {
			case PredSkip:
				nskip++
			case PredInter:
				ninter++
			}
		}
		if len(fs.CUs) != w.cus || npu != w.pus || nskip != w.skip ||
			ninter != w.inter || len(fs.Leaves) != w.leaves ||
			fs.Consumed != w.stop || fs.PayloadBits != w.payload ||
			fs.TailFirstOne != w.firstOne {
			t.Fatalf("P%d CUs=%d PU=%d skip=%d inter=%d leaves=%d stop=%d/%d firstOne=%d, want %d/%d/%d/%d/%d %d/%d/%d",
				i+1, len(fs.CUs), npu, nskip, ninter, len(fs.Leaves),
				fs.Consumed, fs.PayloadBits, fs.TailFirstOne,
				w.cus, w.pus, w.skip, w.inter, w.leaves, w.stop, w.payload, w.firstOne)
		}
	}
}

// TestV22InterRefused keeps the B staging line: B slices still refuse
// with the namable pixel-stage error (their path is a later step).
func TestV22InterRefused(t *testing.T) {
	b := loadV2Baseline(t)
	path := "../testdata/" + b.Clips[0].File
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
	s := m.Video.Samples[1]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		t.Fatalf("sample: %v", err)
	}
	u, err := SplitHVCC(buf, h.LengthSize)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if _, err := ParseFrameSyntax(u[0], ps, 0); err != nil {
		t.Fatalf("P slice err = %v, want nil (P parses since v0.90)", err)
	}
	if SliceTypeName(SliceB) != "B" {
		t.Fatalf("B type name = %q, want B", SliceTypeName(SliceB))
	}
}
