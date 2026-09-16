package h264

import (
	"os"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// decodeClip runs the real path (demux + split + decode) over every sample
// of an MP4. wrap converts one sample's NALUs to the input packing under
// test (length-prefixed or start-code). It returns one picture per sample
// plus each frame's skip count.
func decodeClip(t *testing.T, mp4Path string, wrap func([][]byte) ([]byte, func([]byte) ([][]byte, error))) ([]*Picture, []int) {
	t.Helper()
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	if v == nil {
		t.Fatal("no video track")
	}
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
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
	var pics []*Picture
	var skips []int
	for _, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample %d: %v", s.Number, err)
		}
		packed, split := wrap(units)
		ins, err := split(packed)
		if err != nil {
			t.Fatalf("re-split sample %d: %v", s.Number, err)
		}
		for _, u := range ins {
			if err := dec.DecodeNALU(u); err != nil {
				t.Fatalf("sample %d: %v", s.Number, err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			t.Fatalf("finish sample %d: %v", s.Number, err)
		}
		pics = append(pics, pic)
		skips = append(skips, dec.skipCnt)
	}
	return pics, skips
}

func avccWrap(units [][]byte) ([]byte, func([]byte) ([][]byte, error)) {
	var buf []byte
	for _, u := range units {
		n := len(u)
		buf = append(buf, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
		buf = append(buf, u...)
	}
	return buf, func(b []byte) ([][]byte, error) { return SplitAVCC(b, 4) }
}

func annexBWrap(units [][]byte) ([]byte, func([]byte) ([][]byte, error)) {
	var buf []byte
	for _, u := range units {
		buf = append(buf, 0, 0, 0, 1)
		buf = append(buf, u...)
	}
	return buf, SplitAnnexB
}

// assertClipExact checks every decoded frame against the ffmpeg oracle.
func assertClipExact(t *testing.T, pics []*Picture, yuvPath string, w, h int) {
	t.Helper()
	buf, err := os.ReadFile(yuvPath)
	if err != nil {
		t.Skipf("oracle missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	fs := w * h * 3 / 2
	if len(buf) < fs*len(pics) {
		t.Fatalf("oracle short: %d", len(buf))
	}
	for fi, pic := range pics {
		if pic.Width != uint32(w) || pic.Height != uint32(h) {
			t.Fatalf("frame %d size = %dx%d", fi, pic.Width, pic.Height)
		}
		ey := buf[fi*fs : fi*fs+w*h]
		ecb := buf[fi*fs+w*h : fi*fs+w*h+w*h/4]
		ecr := buf[fi*fs+w*h+w*h/4 : (fi+1)*fs]
		for i := range ey {
			if pic.Y[i] != ey[i] {
				t.Fatalf("frame %d luma diff at %d (x=%d y=%d) got=%d want=%d",
					fi, i, i%w, i/w, pic.Y[i], ey[i])
			}
		}
		for i := range ecb {
			if pic.Cb[i] != ecb[i] || pic.Cr[i] != ecr[i] {
				t.Fatalf("frame %d chroma diff at %d got=%d/%d want=%d/%d",
					fi, i, pic.Cb[i], pic.Cr[i], ecb[i], ecr[i])
			}
		}
	}
}

// VR2b gate: mixed I+4P Baseline clip decodes pixel-exact, both packings.
func TestDecodeBMixedExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_b_mixed.mp4", avccWrap)
	if len(pics) != 5 {
		t.Fatalf("frames = %d want 5", len(pics))
	}
	assertClipExact(t, pics, "../testdata/vr2_b_mixed.yuv", 96, 96)
}

func TestDecodeBMixedExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_b_mixed.mp4", annexBWrap)
	if len(pics) != 5 {
		t.Fatalf("frames = %d want 5", len(pics))
	}
	assertClipExact(t, pics, "../testdata/vr2_b_mixed.yuv", 96, 96)
}

// VR2b gate: static-grey clip decodes pixel-exact; its P frames must ride
// the skip path (skip count is a self-reported aux item).
func TestDecodeBSkipExact(t *testing.T) {
	pics, skips := decodeClip(t, "../testdata/vr2_b_skip.mp4", avccWrap)
	if len(pics) != 5 {
		t.Fatalf("frames = %d want 5", len(pics))
	}
	assertClipExact(t, pics, "../testdata/vr2_b_skip.yuv", 96, 96)
	t.Logf("skip counts per frame: %v", skips)
	if skips[0] != 0 {
		t.Fatalf("IDR frame skips = %d want 0", skips[0])
	}
	total := 0
	for _, n := range skips[1:] {
		total += n
	}
	if total == 0 {
		t.Fatal("P frames used no skips")
	}
}

// VR2c gate: Main-profile CABAC clip decodes pixel-exact, both packings.
// Covers CABAC entropy, explicit weighted prediction, list reordering
// and sliding-window marking (I + 4P, 96x96).
func TestDecodeMMainExactAVCC(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_main.mp4", avccWrap)
	if len(pics) != 5 {
		t.Fatalf("frames = %d want 5", len(pics))
	}
	assertClipExact(t, pics, "../testdata/vr2_m_main.yuv", 96, 96)
}

func TestDecodeMMainExactAnnexB(t *testing.T) {
	pics, _ := decodeClip(t, "../testdata/vr2_m_main.mp4", annexBWrap)
	if len(pics) != 5 {
		t.Fatalf("frames = %d want 5", len(pics))
	}
	assertClipExact(t, pics, "../testdata/vr2_m_main.yuv", 96, 96)
}

// Reorder unit: the gate clip's frame-2 header moves fn1 to the front
// twice, so list 0 aliases one picture under indices 0 and 1.
func TestRefListReorder(t *testing.T) {
	mk := func(fn uint32) *Picture {
		p, err := NewPicture(16, 16)
		if err != nil {
			t.Fatal(err)
		}
		p.FrameNum = fn
		return p
	}
	d := NewDecoder(nil)
	d.sps = &SPS{Log2MaxFrameNum: 0}
	p0, p1 := mk(0), mk(1)
	d.dpb.Store(p0, true)
	d.dpb.Store(p1, true)
	h := &SliceHeader{FrameNum: 2, RefL0Count: 3, RefModL0: []RefModOp{
		{IDC: 0, Arg: 0}, {IDC: 0, Arg: 15}, {IDC: 0, Arg: 0},
	}}
	list, err := d.buildRefList0(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0] != p1 || list[1] != p1 || list[2] != p0 {
		t.Fatalf("reordered list = %v want [fn1 fn1 fn0]", list)
	}
}

// Strength unit: two indices into the same picture are one reference,
// so the edge between them filters only on motion difference.
func TestInterBSAlias(t *testing.T) {
	p, err := NewPicture(32, 16)
	if err != nil {
		t.Fatal(err)
	}
	mbW, mbH := 2, 1
	stride := mbW * 4
	n4 := stride * mbH * 4
	mbIntra := make([]bool, mbW*mbH)
	nnz := make([]int8, n4)
	mvX := make([]int16, n4)
	mvY := make([]int16, n4)
	refs := make([]int8, n4)
	for i := range refs {
		refs[i] = 0
	}
	// Right half rides index 1; both indices resolve to one picture.
	for y := 0; y < 4; y++ {
		for x := 4; x < 8; x++ {
			refs[y*stride+x] = 1
		}
	}
	list := []*Picture{p, p}
	// sx=16 is the middle vertical edge (x=16), seg row 0.
	if got := interBS(mbIntra, nnz, mvX, mvY, refs, list, mbW, mbH, 16, 0, false, false, nil, nil, nil, nil, nil, nil); got != 0 {
		t.Fatalf("aliased edge bS = %d want 0", got)
	}
	if got := interBS(mbIntra, nnz, mvX, mvY, refs, nil, mbW, mbH, 16, 0, false, false, nil, nil, nil, nil, nil, nil); got != 1 {
		t.Fatalf("index-compare edge bS = %d want 1", got)
	}
}

// Right-half predictor unit: the directional neighbour of P_8x16's
// right half is C (above-right) with D (above-left) standing in when C
// is unavailable (spec 8.4.1.3.2). At the picture's right edge C is
// always out of frame, so reading it raw mispredicts (oceans s100:
// median (-10,65) instead of D (-8,65), 176 luma diffs in x950-959).
func TestPred8x16RightEdgeFallback(t *testing.T) {
	mk := func() *Decoder {
		d := NewDecoder(nil)
		d.mbW, d.mbH = 3, 2
		n4 := d.mbW * 4 * d.mbH * 4
		d.mvX = make([]int16, n4)
		d.mvY = make([]int16, n4)
		d.refIdx = make([]int8, n4)
		for i := range d.refIdx {
			d.refIdx[i] = -1
		}
		d.refIdx1 = make([]int8, n4)
		for i := range d.refIdx1 {
			d.refIdx1[i] = -1
		}
		d.mvX1 = make([]int16, n4)
		d.mvY1 = make([]int16, n4)
		d.useM = make([]uint8, n4)
		d.mbSlice = make([]int, d.mbW*d.mbH)
		return d
	}
	put := func(d *Decoder, bx, by int, mx, my int16, ref int8) {
		i := by*d.mbW*4 + bx
		d.mvX[i], d.mvY[i], d.refIdx[i] = mx, my, ref
		d.useM[i] |= useL0
	}
	// Right-edge MB (2,1): C (12,3) is out of frame, D (9,3) matches.
	// A and B also match (flat neighbours), so the median path would
	// answer (0,0) — only the D fallback gives (-8,65).
	d := mk()
	put(d, 9, 4, 0, 0, 0)
	put(d, 10, 3, 0, 0, 0)
	put(d, 9, 3, -8, 65, 0)
	if mx, my := d.pred8x16Right(10, 4, 0); mx != -8 || my != 65 {
		t.Fatalf("edge right = (%d,%d) want D (-8,65)", mx, my)
	}
	// Interior MB (1,1): C (8,3) matches, returns C directly.
	d2 := mk()
	put(d2, 8, 3, 5, 6, 0)
	if mx, my := d2.pred8x16Right(6, 4, 0); mx != 5 || my != 6 {
		t.Fatalf("interior right = (%d,%d) want C (5,6)", mx, my)
	}
}

// Marking unit: reference roster keeps newest-first order, drops
// disposable pictures, slides the window, and flushes on IDR.
func TestDPBMarkingOps(t *testing.T) {
	mk := func(poc int32, idr bool) *Picture {
		p, err := NewPicture(16, 16)
		if err != nil {
			t.Fatal(err)
		}
		p.POC, p.IsIDR = poc, idr
		return p
	}
	d := NewDPB(2)
	a, b := mk(0, true), mk(2, false)
	c, e := mk(4, false), mk(6, true)
	d.Store(a, true)
	d.Store(b, true)
	if got := d.List0(2); len(got) != 2 || got[0] != b || got[1] != a {
		t.Fatalf("list0 order wrong")
	}
	d.Store(c, false)
	if d.Len() != 2 || d.Latest() != b {
		t.Fatal("disposable picture must not join the roster")
	}
	f := mk(4, false)
	d.Store(f, true)
	if d.Len() != 2 || d.ByPOC(0) != nil || d.ByPOC(4) != f {
		t.Fatal("window must evict the oldest reference")
	}
	d.Store(e, true)
	if d.Len() != 1 || d.Latest() != e {
		t.Fatal("IDR must flush the roster")
	}
}

// P header tracking: mixed clip frame order, type, and picture numbers.
func TestSliceHeaderPTracking(t *testing.T) {
	const mp4Path = "../testdata/vr2_b_mixed.mp4"
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	ps, _, slices := firstSliceNALUs(t, mp4Path, 5)
	q, s := activeSets(t, ps)
	if len(slices) != 5 {
		t.Fatalf("slices = %d want 5", len(slices))
	}
	for i, raw := range slices {
		h, _, err := ParseSliceHeader(raw, q, s)
		if err != nil {
			t.Fatalf("slice %d: %v", i, err)
		}
		if i == 0 {
			if !h.IsI() || !h.IsIDR || h.FrameNum != 0 || h.POC != 0 {
				t.Fatalf("slice 0 = I/IDR fn=%d poc=%d", h.FrameNum, h.POC)
			}
			continue
		}
		if !h.IsP() {
			t.Fatalf("slice %d type = %d want P", i, h.Type)
		}
		if h.FrameNum != uint32(i) || h.POC != int32(2*i) {
			t.Fatalf("slice %d fn=%d poc=%d want fn=%d poc=%d",
				i, h.FrameNum, h.POC, i, 2*i)
		}
		if h.NalRefIDC == 0 {
			t.Fatalf("slice %d must be a reference picture", i)
		}
		if h.RefL0Count < 1 {
			t.Fatalf("slice %d refs = %d", i, h.RefL0Count)
		}
		if len(h.MMCO) != 0 || h.AdaptiveMarking {
			t.Fatalf("slice %d should ride the sliding window (no marking ops)", i)
		}
		t.Logf("slice %d qpD=%d", i, h.QPDelta)
	}
}
