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
// decoders. P/B slices stay refused (pixel stage owns them next).

import (
	"errors"
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

// TestV22InterRefused pins the staging line: P/B slices still refuse with
// the namable pixel-stage error (their motion path is the next step).
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
	if _, err := ParseFrameSyntax(u[0], ps, 0); !errors.Is(err, ErrNotDecodable) {
		t.Fatalf("P slice err = %v, want ErrNotDecodable", err)
	}
}
