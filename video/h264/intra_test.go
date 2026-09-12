package h264

import (
	"os"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// decodeFirstFrame runs the real path (demux + split + decode) and returns
// the first decoded picture of an MP4.
func decodeFirstFrame(t *testing.T, mp4Path string) *Picture {
	t.Helper()
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
	// Out-of-band sets travel in avcC, not in mdat: load them first.
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
	s := v.Samples[0]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		t.Fatalf("sample %d: %v", s.Number, err)
	}
	units, err := SplitAVCC(buf, avcc.LengthSize)
	if err != nil {
		t.Fatalf("split sample %d: %v", s.Number, err)
	}
	for _, u := range units {
		if err := dec.DecodeNALU(u); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	var pic *Picture
	pic, err = dec.FinishPicture()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	return pic
}

func readYUVFrame(t *testing.T, path string, w, h, idx int) (y, cb, cr []uint8) {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("oracle missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	fs := w * h * 3 / 2
	if len(buf) < fs*(idx+1) {
		t.Fatalf("oracle short: %d", len(buf))
	}
	off := fs * idx
	return buf[off : off+w*h], buf[off+w*h : off+w*h+w*h/4], buf[off+w*h+w*h/4 : off+fs]
}

// VR2a gate: I-only Baseline clip decodes pixel-exact to ffmpeg output.
func TestDecodeBIntraExact(t *testing.T) {
	const mp4Path = "../testdata/vr2_b_intra.mp4"
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	pic := decodeFirstFrame(t, mp4Path)
	if pic.Width != 96 || pic.Height != 96 {
		t.Fatalf("size = %dx%d", pic.Width, pic.Height)
	}
	ey, ecb, ecr := readYUVFrame(t, "../testdata/vr2_b_intra.yuv", 96, 96, 0)
	bad := 0
	first := -1
	for i := range ey {
		if pic.Y[i] != ey[i] {
			if first < 0 {
				first = i
			}
			bad++
		}
	}
	if bad != 0 {
		t.Fatalf("luma diff pixels = %d/%d first at %d (x=%d y=%d) got=%d want=%d",
			bad, len(ey), first, first%96, first/96, pic.Y[first], ey[first])
	}
	for i := range ecb {
		if pic.Cb[i] != ecb[i] || pic.Cr[i] != ecr[i] {
			t.Fatalf("chroma diff at %d", i)
		}
	}
}

func TestCBPMap(t *testing.T) {
	if golombToIntra4x4CBP[0] != 47 || golombToIntra4x4CBP[3] != 0 {
		t.Fatalf("cbp map = %d/%d", golombToIntra4x4CBP[0], golombToIntra4x4CBP[3])
	}
	if i16PredCycle != [4]int{0, 1, 2, 3} {
		t.Fatalf("pred cycle = %v", i16PredCycle)
	}
}
