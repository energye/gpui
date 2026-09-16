package video

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/h265"
	"github.com/energye/gpui/video/mp4"
)

// faultProbe opens a clip with a frozen clock for fault gates.
func faultProbe(path string) (*Player, error) {
	return OpenFile(path, Options{NowMs: func() int64 { return 0 }})
}

// TestClassifyTable pins the VR6 triage: every layer's sentinel lands in
// its documented bucket, so the window can name layer + tool.
func TestClassifyTable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"box-trunc", fmt.Errorf("x: %w", mp4.ErrTruncated), KindTruncated},
		{"box-bad", fmt.Errorf("x: %w", mp4.ErrBadBox), KindBadBox},
		{"box-nomoov", fmt.Errorf("x: %w", mp4.ErrNoMoov), KindBadBox},
		{"box-novideo", fmt.Errorf("x: %w", mp4.ErrNoVideoTrack), KindBadBox},
		{"box-frag", fmt.Errorf("x: %w", mp4.ErrFragmented), KindBadBox},
		{"params-sps", fmt.Errorf("x: %w", h264.ErrMissingSPS), KindMissingParam},
		{"params-pps", fmt.Errorf("x: %w", h264.ErrMissingPPS), KindMissingParam},
		{"params-avcc", fmt.Errorf("x: %w", h264.ErrBadAVCC), KindMissingParam},
		{"f17-groups", fmt.Errorf("x: %w", h264.ErrSliceGroups), KindF17},
		{"f17-part", fmt.Errorf("x: %w", h264.ErrDataPartitioning), KindF17},
		{"f17-redundant", fmt.Errorf("x: %w", h264.ErrRedundantPic), KindF17},
		{"f17-ext", fmt.Errorf("x: %w", h264.ErrUnsupportedNAL), KindF17},
		{"f20-lost", fmt.Errorf("x: %w", h264.ErrLostReference), KindF20},
		{"f20-slice", fmt.Errorf("x: %w", h264.ErrBadSliceHeader), KindF20},
		{"level", fmt.Errorf("x: %w", h264.ErrUnsupportedLevel), KindLevel},
		{"h265-headers", fmt.Errorf("x: %w", h265.ErrNotDecodable), KindH265},
		{"h265-hvcc", fmt.Errorf("x: %w", h265.ErrBadHVCC), KindH265},
		{"color-matrix", fmt.Errorf("x: %w", color.ErrUnsupportedMatrix), KindColor},
		{"badclip", fmt.Errorf("x: %w", ErrNoVideo), KindBadClip},
		{"missing-file", fmt.Errorf("x: %w", os.ErrNotExist), KindBadClip},
	}
	for _, c := range cases {
		if got := Classify(c.err).Kind; got != c.want {
			t.Errorf("%s: kind = %q, want %q", c.name, got, c.want)
		}
	}
	// StageScope splits by tool marker: F12 vs profile.
	f12 := fmt.Errorf("x: %w: F12 interlace field picture", h264.ErrStageScope)
	if got := Classify(f12).Kind; got != KindInterlace {
		t.Errorf("f12: kind = %q, want %q", got, KindInterlace)
	}
	prof := fmt.Errorf("x: %w: profile XYZ needs later", h264.ErrStageScope)
	if got := Classify(prof).Kind; got != KindProfile {
		t.Errorf("profile: kind = %q, want %q", got, KindProfile)
	}
	// Every bucket names layer + tool for the window list.
	for _, c := range cases {
		f := Classify(c.err)
		if f.Layer == "" || f.CN == "" {
			t.Errorf("%s: fault missing layer/text: %+v", c.name, f)
		}
	}
}

// TestFaultH265Headers pins zero false positives on the V2-1 path: the
// H.265 clip probes as mp4/h265, headers read, and the open refuses with
// the H.265 bucket (never silent, never a crash); good H.264 clips keep
// zero concealment.
func TestFaultH265Headers(t *testing.T) {
	container, codec, err := ProbeFile("testdata/v2_h265.mp4")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if container != ContainerMP4 || codec != CodecH265 {
		t.Fatalf("probe = %q/%q, want mp4/h265", container, codec)
	}
	if _, err := OpenFile("testdata/v2_h265.mp4", Options{NowMs: func() int64 { return 0 }}); err == nil {
		t.Fatal("h265 headers open as pixels")
	} else if got := Classify(err).Kind; got != KindH265 {
		t.Fatalf("kind = %q, want %q (%v)", got, KindH265, err)
	}
}

// TestFaultCleanClips pins zero false positives: good clips open with no
// concealment and no fault text (VR2/VR4/VR5 unaffected).
func TestFaultCleanClips(t *testing.T) {
	for _, n := range []string{"testdata/vr2_m_bframes.mp4", "testdata/vr5_seek.mp4", "testdata/vr2_480p.mp4"} {
		p, err := faultProbe(n)
		if err != nil {
			t.Fatalf("%s: clean clip fails: %v", n, err)
		}
		if p.Info().Concealed != 0 || p.Info().Fault != "" {
			t.Fatalf("%s: concealed=%d fault=%q, want 0/empty", n, p.Info().Concealed, p.Info().Fault)
		}
		if p.Stats().Concealed != 0 {
			t.Fatalf("%s: stats concealed=%d, want 0", n, p.Stats().Concealed)
		}
		p.Close()
	}
}

// TestFaultMissingFile pins io faults: no panic, namable kind.
func TestFaultMissingFile(t *testing.T) {
	_, err := faultProbe("testdata/does-not-exist.mp4")
	if err == nil {
		t.Fatal("missing file opens")
	}
	if got := Classify(err).Kind; got != KindBadClip {
		t.Fatalf("kind = %q, want %q (%v)", got, KindBadClip, err)
	}
}

// TestFaultNonMP4 pins junk input: any readable failure with a known
// bucket counts (parser reports truncated/bad-box for junk bytes).
func TestFaultNonMP4(t *testing.T) {
	_, err := faultProbe("fault.go")
	if err == nil {
		t.Fatal("go source opens as mp4")
	}
	switch got := Classify(err).Kind; got {
	case KindTruncated, KindBadBox, KindBadClip:
	default:
		t.Fatalf("kind = %q, want truncated/bad-box/bad-clip (%v)", got, err)
	}
}

// TestFaultTruncatedTail pins truncation: cutting the tail damages the
// trailing moov, so open fails readable as truncated (no crash).
func TestFaultTruncatedTail(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "trunc.mp4")
	if err := os.WriteFile(bad, raw[:len(raw)-200], 0o644); err != nil {
		t.Fatalf("write trunc: %v", err)
	}
	_, err = faultProbe(bad)
	if err == nil {
		t.Fatal("truncated tail opens")
	}
	if !errors.Is(err, mp4.ErrTruncated) {
		t.Fatalf("err = %v, want truncated chain", err)
	}
	if got := Classify(err).Kind; got != KindTruncated {
		t.Fatalf("kind = %q, want %q", got, KindTruncated)
	}
}

// firstPSlice extracts the second sample (a P slice) from the gate clip.
func firstPSlice(t *testing.T) ([][]byte, *h264.AVCC) {
	t.Helper()
	m, err := mp4.ParseFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	avcc, err := h264.ParseAVCC(m.Video.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("open base: %v", err)
	}
	defer f.Close()
	s := m.Video.Samples[1]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		t.Fatalf("read sample: %v", err)
	}
	units, err := h264.SplitAVCC(buf, avcc.LengthSize)
	if err != nil || len(units) == 0 {
		t.Fatalf("split: %v %d", err, len(units))
	}
	return units, avcc
}

// TestFaultMissingParams pins缺参数: a slice without sets fails namable.
func TestFaultMissingParams(t *testing.T) {
	units, _ := firstPSlice(t)
	dec := h264.NewDecoder(nil)
	err := dec.DecodeNALU(units[0])
	if !errors.Is(err, h264.ErrMissingPPS) && !errors.Is(err, h264.ErrMissingSPS) {
		t.Fatalf("err = %v, want missing params", err)
	}
	if got := Classify(err).Kind; got != KindMissingParam {
		t.Fatalf("kind = %q, want %q", got, KindMissingParam)
	}
}

// TestFaultF20LostReference pins坏参考帧: a P slice with params but no
// reference fails as F20 (isolate + continue, never flower).
func TestFaultF20LostReference(t *testing.T) {
	units, avcc := firstPSlice(t)
	dec := h264.NewDecoder(nil)
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
	err := dec.DecodeNALU(units[0])
	if !errors.Is(err, h264.ErrLostReference) {
		t.Fatalf("err = %v, want lost reference", err)
	}
	if got := Classify(err).Kind; got != KindF20 {
		t.Fatalf("kind = %q, want %q", got, KindF20)
	}
}

// TestFaultF17Partition pins老容错件: partition and extension NALUs stop
// the cut with F17 instead of guessing.
func TestFaultF17Partition(t *testing.T) {
	if _, err := h264.SplitFrames([][]byte{{0x42, 0x00}}); !errors.Is(err, h264.ErrDataPartitioning) {
		t.Fatalf("partition err = %v, want F17", err)
	}
	if _, err := h264.SplitFrames([][]byte{{0x74, 0x00}}); !errors.Is(err, h264.ErrUnsupportedNAL) {
		t.Fatalf("ext err = %v, want F17", err)
	}
}

// TestFaultLevelOverLimit pins超限等级: levels past 5.2 fail fast namable.
func TestFaultLevelOverLimit(t *testing.T) {
	sps := &h264.SPS{ProfileIDC: 77, Profile: "Main", LevelIDC: 60, Level: "6.0"}
	err := checkStreamLimits(sps, "test.mp4")
	if !errors.Is(err, h264.ErrUnsupportedLevel) {
		t.Fatalf("err = %v, want unsupported level", err)
	}
	if got := Classify(err).Kind; got != KindLevel {
		t.Fatalf("kind = %q, want %q", got, KindLevel)
	}
	// Profile beyond B/M/H is a separate bucket.
	sps = &h264.SPS{ProfileIDC: 110, Profile: "High10", LevelIDC: 40, Level: "4.0"}
	if err := checkStreamLimits(sps, "test.mp4"); !errors.Is(err, h264.ErrStageScope) {
		t.Fatalf("profile err = %v, want stage scope", err)
	}
}

// TestFaultFlowerIsolation pins花屏流/F20 end to end: zeroing a middle P
// sample isolates 2 bad frames, the dual-IDR tail still plays to the end
// (8/10 frames, pts prove no flower, no crash).
func TestFaultFlowerIsolation(t *testing.T) {
	m, err := mp4.ParseFile("testdata/vr5_seek.mp4")
	if err != nil {
		t.Fatalf("parse base: %v", err)
	}
	raw, err := os.ReadFile("testdata/vr5_seek.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	cp := append([]byte(nil), raw...)
	s := m.Video.Samples[1]
	for i := int64(0); i < int64(s.Size); i++ {
		cp[int64(s.Offset)+i] = 0
	}
	bad := filepath.Join(t.TempDir(), "flower.mp4")
	if err := os.WriteFile(bad, cp, 0o644); err != nil {
		t.Fatalf("write flower: %v", err)
	}
	h := &handClock{}
	p, err := OpenFile(bad, Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("flower clip fails to open: %v", err)
	}
	defer p.Close()
	if p.Info().Concealed != 2 {
		t.Fatalf("concealed = %d, want 2", p.Info().Concealed)
	}
	if got := Classify(p.ConcealedFault()).Kind; got != KindF20 {
		t.Fatalf("fault kind = %q, want %q (%v)", got, KindF20, p.ConcealedFault())
	}
	// Play to the end: 8 frames, second GOP intact, ends clean.
	var pts []int64
	for i := 0; i < 30; i++ {
		h.now += 200
		f, done := p.Poll()
		if f != nil {
			pts = append(pts, f.PTSMs)
		}
		if done {
			break
		}
	}
	want := []int64{400, 600, 800, 1400, 1600, 1800, 2000, 2200}
	if len(pts) != len(want) {
		t.Fatalf("pts = %v, want %v", pts, want)
	}
	for i := range want {
		if pts[i] != want[i] {
			t.Fatalf("pts = %v, want %v", pts, want)
		}
	}
	if !p.Stats().Ended {
		t.Fatal("flower clip never ends")
	}
	if p.Stats().Concealed != 2 {
		t.Fatalf("stats concealed = %d, want 2", p.Stats().Concealed)
	}
}

// TestFaultRapidBadOpens pins连续坏输入不卡死: alternating good and bad
// opens never hang and every error stays readable.
func TestFaultRapidBadOpens(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			p, err := faultProbe("testdata/vr2_m_bframes.mp4")
			if err != nil {
				t.Fatalf("good open %d: %v", i, err)
			}
			p.Close()
			continue
		}
		bad := filepath.Join(dir, fmt.Sprintf("bad%d.mp4", i))
		_ = os.WriteFile(bad, raw[:len(raw)-200], 0o644)
		if _, err := faultProbe(bad); err == nil {
			t.Fatalf("bad open %d succeeds", i)
		} else if Classify(err).Kind == KindUnknown {
			t.Fatalf("bad open %d unknown kind: %v", i, err)
		}
	}
}
