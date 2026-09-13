package h264

import (
	"os"
	"sort"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// TestBRefLists pins the B-slice reference tables of the VR2d gate clip
// (vr2_m_bframes, decode order I0/P6/B2/B4/P8, display POC 0/2/4/6/8).
// Lists are checked by display order; macroblock payload still stops at
// the VR2d stage gate.
func TestBRefLists(t *testing.T) {
	mp4Path := "../testdata/vr2_m_bframes.mp4"
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	sampleUnits := func(i int) [][]byte {
		t.Helper()
		s := v.Samples[i]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample %d: %v", i, err)
		}
		return units
	}
	sliceOf := func(i int) []byte {
		t.Helper()
		for _, u := range sampleUnits(i) {
			if typ, _ := NALType(u); typ == NALSliceNonIDR || typ == NALSliceIDR {
				return u
			}
		}
		t.Fatalf("sample %d: no slice", i)
		return nil
	}

	dec := NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			t.Fatalf("sps: %v", err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			t.Fatalf("pps: %v", err)
		}
	}
	pps, sps, err := dec.ps.RequireForSlice(0)
	if err != nil {
		t.Fatalf("sets: %v", err)
	}
	// Samples 0 (IDR) and 1 (P) decode end to end, filling the buffer.
	for _, i := range []int{0, 1} {
		for _, u := range sampleUnits(i) {
			if err := dec.DecodeNALU(u); err != nil {
				t.Fatalf("sample %d: %v", i, err)
			}
		}
		if _, err := dec.FinishPicture(); err != nil {
			t.Fatalf("finish %d: %v", i, err)
		}
	}
	pocs := func(list []*Picture) []int32 {
		var out []int32
		for _, p := range list {
			if p == nil {
				out = append(out, -1)
				continue
			}
			out = append(out, p.POC)
		}
		return out
	}
	eq := func(tag string, got []int32, want ...int32) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s length = %v, want %v", tag, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s = %v, want %v", tag, got, want)
			}
		}
	}

	// Sample 2 (B poc2): past [I0], future [P6].
	h2, _, err := ParseSliceHeader(sliceOf(2), pps, sps)
	if err != nil {
		t.Fatalf("b2 header: %v", err)
	}
	if !h2.IsB() || h2.POC != 2 {
		t.Fatalf("b2 = type %d poc %d, want B poc 2", h2.Type, h2.POC)
	}
	dec.fixPOC(h2, sps)
	l0, l1, err := dec.buildRefListsB(h2)
	if err != nil {
		t.Fatalf("b2 lists: %v", err)
	}
	eq("b2 l0", pocs(l0), 0)
	eq("b2 l1", pocs(l1), 6)

	// Payload decodes end to end now (macroblock stage landed), before
	// the synthetic B2 below joins the buffer.
	if err := dec.DecodeNALU(sliceOf(2)); err != nil {
		t.Fatalf("b2 payload: %v", err)
	}

	// Sample 3 (B poc4, non-reference): buffer also holds the B2 ref.

	// Sample 3 (B poc4, non-reference): buffer also holds the B2 ref.
	dec.dpb.Store(&Picture{Width: 96, Height: 96, Y: make([]uint8, 96*96),
		Cb: make([]uint8, 48*48), Cr: make([]uint8, 48*48),
		FrameNum: 2, POC: 2}, true)
	h3, _, err := ParseSliceHeader(sliceOf(3), pps, sps)
	if err != nil {
		t.Fatalf("b3 header: %v", err)
	}
	dec.fixPOC(h3, sps)
	l0, l1, err = dec.buildRefListsB(h3)
	if err != nil {
		t.Fatalf("b3 lists: %v", err)
	}
	eq("b3 l0", pocs(l0), 2, 0)
	eq("b3 l1", pocs(l1), 6)
}

// VR2d gate: Main-profile B clip decodes pixel-exact in display order,
// both packings. Decode order is I0/P6/B2/B4/P8 (POC 0/6/2/4/8); display
// order is POC 0/2/4/6/8. Covers B skip/direct/partitions, spatial
// direct, implicit bipred weighting and two-list deblocking.
func TestDecodeMBFramesExactAVCC(t *testing.T) {
	pics, skips := decodeClip(t, "../testdata/vr2_m_bframes.mp4", avccWrap)
	assertBFramesExact(t, pics, skips)
}

func TestDecodeMBFramesExactAnnexB(t *testing.T) {
	pics, skips := decodeClip(t, "../testdata/vr2_m_bframes.mp4", annexBWrap)
	assertBFramesExact(t, pics, skips)
}

// VR2d widening gate: cropped 480p Main clip (854x480 display over
// 864x480 coded, decode order I0/P4/B2/P8/B6, display POC 0/2/4/6/8)
// decodes pixel-exact in display order, both packings. Nets the B intra
// escape (I4x4 flag before the PCM check) and list-grouped B motion
// differences, both first hit by real-encoder B blocks this clip carries.
func TestDecode480pExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_480p.mp4", avccWrap)
	assert480pExact(t, pics)
}

func TestDecode480pExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_480p.mp4", annexBWrap)
	assert480pExact(t, pics)
}

func assert480pExact(t *testing.T, pics []*Picture) {
	t.Helper()
	assertCropExact(t, pics, "../testdata/vr2_480p.yuv", 854, 480, []int32{0, 4, 2, 8, 6})
}

// VR2d widening gate: 720p Main clip (1280x720, same I0/P4/B2/P8/B6
// structure as 480p) decodes pixel-exact in display order, both
// packings. Nets the P-skip zero shortcut and macroblock-level B_8x8
// direct derivation, both first hit by real-encoder blocks this clip
// carries.
func TestDecode720pExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_720p.mp4", avccWrap)
	assertCropExact(t, pics, "../testdata/vr2_720p.yuv", 1280, 720, []int32{0, 4, 2, 8, 6})
}

func TestDecode720pExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_720p.mp4", annexBWrap)
	assertCropExact(t, pics, "../testdata/vr2_720p.yuv", 1280, 720, []int32{0, 4, 2, 8, 6})
}

// VR2d d3 gate: B-pyramid clip (96x96, decode I0/P8/B4/B2/B6; the middle
// B is a reference, so later Bs see two future refs). Nets multi-ref
// list-1 indices and motion, both packings.
func TestDecodeBpyrExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_bpyr.mp4", avccWrap)
	assertCropExact(t, pics, "../testdata/vr2_m_bpyr.yuv", 96, 96, []int32{0, 8, 4, 2, 6})
}

func TestDecodeBpyrExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_bpyr.mp4", annexBWrap)
	assertCropExact(t, pics, "../testdata/vr2_m_bpyr.yuv", 96, 96, []int32{0, 8, 4, 2, 6})
}

// VR2d d3 gate: CAVLC-entropy B clip (96x96, decode I0/P6/B2/B4/P8).
// Nets the CAVLC B-macroblock path (skip/direct/partitions/residual)
// and direct-sub-block-first ordering, both packings.
func TestDecodeBcavlcExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_bcavlc.mp4", avccWrap)
	assertCropExact(t, pics, "../testdata/vr2_m_bcavlc.yuv", 96, 96, []int32{0, 6, 2, 4, 8})
}

func TestDecodeBcavlcExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_bcavlc.mp4", annexBWrap)
	assertCropExact(t, pics, "../testdata/vr2_m_bcavlc.yuv", 96, 96, []int32{0, 6, 2, 4, 8})
}

// VR2d d3 gate: explicit-B-weight vector (96x96, decode I0/P6/B2/B4/P8).
// x264 does not emit explicit B tables on tried content, so the vector
// is built by NAL surgery on vr2_m_bframes.mp4 samples (recipe, kept in
// sync with video/testdata/gen_vr2.sh comments): the PPS bipred flag is
// flipped 2->1; each B slice gets one weight table inserted after its
// reference-modification steps (P slices keep their own tables
// verbatim, since PPS weighted_pred=1 already gives them denom-0
// tables); every inserted table is a multiple of 8 bits so the CABAC
// payload that follows stays byte-aligned. B2 (one ref per list):
// denoms 5/5, L0[0] weight 48 offset 0 with chroma 48 offset Cb+1/Cr+0,
// L1[0] weight 16 offset +1 with chroma 16 offset Cb+2/Cr-2 (104 bits).
// B4 (two L0 refs): denoms 5/5, L0[0] weight 20 offset 0 with chroma 20
// offset Cb+0/Cr-1, L0[1] default, L1[0] weight 44 offset -2 with
// default chroma (72 bits). Wire values are used as-is (a fade-out
// clip proves it: wire +115 at denom 7 dims 124 to 113). Nets explicit
// list-0/list-1 weights, explicit bipred (luma and chroma), default
// entries and negative offsets.
func TestDecodeBExplicitExact(t *testing.T) {
	raw, err := os.ReadFile("testdata/b_explicit.h264")
	if err != nil {
		t.Skipf("vector missing: %v", err)
	}
	units, err := SplitAnnexB(raw)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	dec := NewDecoder(nil)
	var sps *SPS
	var pps *PPS
	for _, u := range units {
		typ, _ := NALType(u)
		if typ == NALSPS {
			if sps, err = ParseSPS(u); err != nil {
				t.Fatalf("sps: %v", err)
			}
		}
		if typ == NALPPS {
			if pps, err = ParsePPS(u); err != nil {
				t.Fatalf("pps: %v", err)
			}
		}
	}
	if pps == nil || sps == nil {
		t.Fatalf("vector has no parameter sets")
	}
	if pps.WeightedBiPred != 1 {
		t.Fatalf("pps bipred = %d, want 1", pps.WeightedBiPred)
	}
	assertBExplicitTables(t, units, pps, sps)
	var pics []*Picture
	for _, u := range units {
		typ, _ := NALType(u)
		if typ != NALSliceNonIDR && typ != NALSliceIDR {
			if err := dec.DecodeNALU(u); err != nil {
				t.Fatalf("param: %v", err)
			}
			continue
		}
		if err := dec.DecodeNALU(u); err != nil {
			t.Fatalf("slice: %v", err)
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			t.Fatalf("finish: %v", err)
		}
		pics = append(pics, pic)
	}
	assertCropExact(t, pics, "testdata/b_explicit.yuv", 96, 96, []int32{0, 6, 2, 4, 8})
	byDisplay := append([]*Picture(nil), pics...)
	sort.Slice(byDisplay, func(i, j int) bool { return byDisplay[i].POC < byDisplay[j].POC })
	assertClipExact(t, byDisplay, "testdata/b_explicit.yuv", 96, 96)
}

// assertBExplicitTables pins the surgical weight values each B slice
// carries (display POC selects the slice).
func assertBExplicitTables(t *testing.T, units [][]byte, pps *PPS, sps *SPS) {
	t.Helper()
	seen := map[int32]bool{}
	for _, u := range units {
		typ, _ := NALType(u)
		if typ != NALSliceNonIDR && typ != NALSliceIDR {
			continue
		}
		h, _, err := ParseSliceHeader(u, pps, sps)
		if err != nil {
			t.Fatalf("header: %v", err)
		}
		if !h.IsB() {
			continue
		}
		seen[h.POC] = true
		if h.LumaDenom != 5 || h.ChromaDenom != 5 {
			t.Fatalf("poc %d denoms = %d/%d, want 5/5", h.POC, h.LumaDenom, h.ChromaDenom)
		}
		switch h.POC {
		case 2:
			if h.RefL0Count != 1 || h.RefL1Count != 1 {
				t.Fatalf("poc 2 refs = %d/%d, want 1/1", h.RefL0Count, h.RefL1Count)
			}
			if h.LumaW0[0] != 48 || h.LumaO0[0] != 0 || h.LumaW1[0] != 16 || h.LumaO1[0] != 1 {
				t.Fatalf("poc 2 luma = %d/%d %d/%d, want 48/0 16/1",
					h.LumaW0[0], h.LumaO0[0], h.LumaW1[0], h.LumaO1[0])
			}
			if h.ChromaW0[0] != [2]int32{48, 48} || h.ChromaO0[0] != [2]int32{1, 0} {
				t.Fatalf("poc 2 chroma L0 = %v/%v", h.ChromaW0[0], h.ChromaO0[0])
			}
			if h.ChromaW1[0] != [2]int32{16, 16} || h.ChromaO1[0] != [2]int32{2, -2} {
				t.Fatalf("poc 2 chroma L1 = %v/%v", h.ChromaW1[0], h.ChromaO1[0])
			}
		case 4:
			if h.RefL0Count != 2 || h.RefL1Count != 1 {
				t.Fatalf("poc 4 refs = %d/%d, want 2/1", h.RefL0Count, h.RefL1Count)
			}
			if h.LumaW0[0] != 20 || h.LumaO0[0] != 0 || h.LumaW0[1] != 32 || h.LumaO0[1] != 0 {
				t.Fatalf("poc 4 luma L0 = %d/%d %d/%d", h.LumaW0[0], h.LumaO0[0], h.LumaW0[1], h.LumaO0[1])
			}
			if h.LumaW1[0] != 44 || h.LumaO1[0] != -2 {
				t.Fatalf("poc 4 luma L1 = %d/%d", h.LumaW1[0], h.LumaO1[0])
			}
		default:
			t.Fatalf("unexpected B poc %d", h.POC)
		}
	}
	if len(seen) != 2 {
		t.Fatalf("B slices = %v, want poc 2 and 4", seen)
	}
}

// VR2d d3 gate: real-encoder fade-out without B frames (96x96, two GOPs
// back to back, decode order == display order within each GOP). x264
// emits explicit P tables here (e.g. wire +115 at denom 7 on bright
// refs, dimming 124 to 113), pinning the wire-direct convention on top
// of the surgical B gate above. No POC sorting: the second GOP reuses
// POC 0/2, so frames compare in file order.
func TestDecodeFadeoutExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_fadeout.mp4", avccWrap)
	assertFadeoutExact(t, pics)
}

func TestDecodeFadeoutExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_fadeout.mp4", annexBWrap)
	assertFadeoutExact(t, pics)
}

func assertFadeoutExact(t *testing.T, pics []*Picture) {
	t.Helper()
	wantPOC := []int32{0, 2, 4, 6, 8, 10, 12, 14, 0, 2}
	if len(pics) != len(wantPOC) {
		t.Fatalf("frames = %d want %d", len(pics), len(wantPOC))
	}
	for i, p := range pics {
		if p.POC != wantPOC[i] {
			t.Fatalf("frame %d poc = %d want %d", i, p.POC, wantPOC[i])
		}
		if p.Width != 96 || p.Height != 96 {
			t.Fatalf("frame %d size = %dx%d", i, p.Width, p.Height)
		}
	}
	assertClipExact(t, pics, "../testdata/vr2_m_fadeout.yuv", 96, 96)
}

func assertCropExact(t *testing.T, pics []*Picture, yuvPath string, w, h int, wantDecode []int32) {
	t.Helper()
	if len(pics) != len(wantDecode) {
		t.Fatalf("frames = %d want %d", len(pics), len(wantDecode))
	}
	var gotPOC []int32
	for _, p := range pics {
		gotPOC = append(gotPOC, p.POC)
		if p.Width != uint32(w) || p.Height != uint32(h) {
			t.Fatalf("frame poc %d size = %dx%d, want %dx%d", p.POC, p.Width, p.Height, w, h)
		}
	}
	for i := range wantDecode {
		if gotPOC[i] != wantDecode[i] {
			t.Fatalf("decode order POC = %v want %v", gotPOC, wantDecode)
		}
	}
	byDisplay := append([]*Picture(nil), pics...)
	sort.Slice(byDisplay, func(i, j int) bool { return byDisplay[i].POC < byDisplay[j].POC })
	assertClipExact(t, byDisplay, yuvPath, w, h)
}

func assertBFramesExact(t *testing.T, pics []*Picture, skips []int) {
	t.Helper()
	if len(pics) != 5 {
		t.Fatalf("frames = %d want 5", len(pics))
	}
	var gotPOC []int32
	for _, p := range pics {
		gotPOC = append(gotPOC, p.POC)
	}
	wantDecode := []int32{0, 6, 2, 4, 8}
	for i := range wantDecode {
		if gotPOC[i] != wantDecode[i] {
			t.Fatalf("decode order POC = %v want %v", gotPOC, wantDecode)
		}
	}
	byDisplay := append([]*Picture(nil), pics...)
	sort.Slice(byDisplay, func(i, j int) bool { return byDisplay[i].POC < byDisplay[j].POC })
	for i, p := range byDisplay {
		if p.POC != int32(i*2) {
			t.Fatalf("display order POC = %v want 0/2/4/6/8", gotPOC)
		}
	}
	assertClipExact(t, byDisplay, "../testdata/vr2_m_bframes.yuv", 96, 96)
	t.Logf("skip counts per sample (decode order): %v", skips)
	if skips[0] != 0 {
		t.Fatalf("IDR frame skips = %d want 0", skips[0])
	}
	for _, i := range []int{2, 3} {
		if skips[i] == 0 {
			t.Fatalf("B sample %d used no skips", i)
		}
	}
}
