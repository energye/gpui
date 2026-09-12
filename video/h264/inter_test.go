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
