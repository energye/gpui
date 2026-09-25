package video

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// faultProbe opens a clip with a frozen clock for fault gates.
func faultProbe(path string) (*Player, error) {
	return OpenFile(path, Options{NowMs: func() int64 { return 0 }})
}

// TestClassifyTable pins the VR6 triage on the ffmpeg backend: only the
// shell/clip/cap buckets remain (ffmpeg absorbs decode details
// natively), so the window can name layer + tool.
func TestClassifyTable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"badclip", fmt.Errorf("x: %w", ErrNoVideo), KindBadClip},
		{"noframes", fmt.Errorf("x: %w", ErrNoFrames), KindBadClip},
		{"closed", fmt.Errorf("x: %w", ErrClosed), KindBadClip},
		{"eof", fmt.Errorf("x: %w", ErrDecodeEOF), KindBadClip},
		{"unsupported-container", fmt.Errorf("x: %w", ErrUnsupportedContainer), KindBadClip},
		{"unsupported-codec", fmt.Errorf("x: %w", ErrUnsupportedCodec), KindBadClip},
		{"memovercap", fmt.Errorf("x: %w", ErrMemOverCap), KindMemOverCap},
		{"missing-file", fmt.Errorf("x: %w", os.ErrNotExist), KindBadClip},
		{"ffmpeg-layer", fmt.Errorf("x: ffmpeg: something broke inside native decode"), KindBadClip},
		{"truncated", fmt.Errorf("x: unexpected EOF in stream"), KindTruncated},
	}
	for _, c := range cases {
		if got := Classify(c.err).Kind; got != c.want {
			t.Errorf("%s: kind = %q, want %q", c.name, got, c.want)
		}
	}
	// Every bucket names layer + tool for the window list.
	for _, c := range cases {
		f := Classify(c.err)
		if f.Layer == "" || f.CN == "" {
			t.Errorf("%s: fault missing layer/text: %+v", c.name, f)
		}
	}
}

// TestFaultH265Headers pins the ffmpeg path for H.265: the probe answers
// mp4/h265 through the ffmpeg demuxer and the player decodes it natively
// — hevc is enabled in libgpui_ffmpeg, so the clip plays.
func TestFaultH265Headers(t *testing.T) {
	container, codec, err := ProbeFile("testdata/v2_h265.mp4")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if container != ContainerMP4 || codec != CodecH265 {
		t.Fatalf("probe = %q/%q, want mp4/h265", container, codec)
	}
	h := &handClock{}
	p, err := OpenFile("testdata/v2_h265.mp4", Options{NowMs: h.at, QueueCap: 8})
	if err != nil {
		t.Fatalf("h265 ffmpeg open: %v", err)
	}
	defer p.Close()
	if p.Info().Codec == "" {
		t.Fatalf("codec empty: %+v", p.Info())
	}
	// One frame proves pixels, not just headers.
	deadline := time.Now().Add(60 * time.Second)
	shown := false
	for !shown {
		if time.Now().After(deadline) {
			t.Fatal("h265 clip shows no frame")
		}
		h.now += 200
		if fr, _ := p.Poll(); fr != nil {
			shown = true
			if len(fr.Pix) != fr.Width*fr.Height*4 {
				t.Fatalf("frame pix %d", len(fr.Pix))
			}
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
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
// bucket counts (ffmpeg reports bad-clip for junk bytes).
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

// TestFaultTruncatedTail pins truncation: cutting the tail must never
// crash. ffmpeg owns the box now, so either the open fails readably
// (bad-clip bucket via the ffmpeg fallback) or the clip opens and plays
// a partial tail — both are honest, only a crash or silence is a failure.
func TestFaultTruncatedTail(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	bad := filepath.Join(t.TempDir(), "trunc.mp4")
	if err := os.WriteFile(bad, raw[:len(raw)-200], 0o644); err != nil {
		t.Fatalf("write trunc: %v", err)
	}
	h := &handClock{}
	p, err := OpenFile(bad, Options{NowMs: h.at, QueueCap: 8})
	if err != nil {
		switch got := Classify(err).Kind; got {
		case KindBadClip, KindTruncated, KindBadBox:
		default:
			t.Fatalf("kind = %q, want bad-clip/truncated/bad-box (%v)", got, err)
		}
		return
	}
	defer p.Close()
	// Opened: drain without crashing; partial tail is fine.
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("truncated clip never drains")
		}
		h.now += 200
		_, done := p.Poll()
		if done {
			break
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// TestFaultFlowerIsolation pins坏帧不崩 end to end: corrupting the middle
// of a clip must never crash. ffmpeg conceals corrupt frames inside
// native code — the gate is play-to-end without crashing, with monotonic
// stamps, not an exact frame count. The corruption is blind (no Go box
// parsing): zero a middle 2KB window of the file bytes.
func TestFaultFlowerIsolation(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr5_seek.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	cp := append([]byte(nil), raw...)
	mid := len(cp) / 2
	for i := 0; i < 2048 && mid+i < len(cp); i++ {
		cp[mid+i] = 0
	}
	bad := filepath.Join(t.TempDir(), "flower.mp4")
	if err := os.WriteFile(bad, cp, 0o644); err != nil {
		t.Fatalf("write flower: %v", err)
	}
	h := &handClock{}
	p, err := OpenFile(bad, Options{NowMs: h.at, QueueCap: 8})
	if err != nil {
		// Refusing a corrupted clip readably is also honest.
		switch got := Classify(err).Kind; got {
		case KindBadClip, KindTruncated, KindBadBox:
		default:
			t.Fatalf("kind = %q, want readable bucket (%v)", got, err)
		}
		return
	}
	defer p.Close()
	// Play to the end: frames shown, stamps monotonic, ends clean.
	var pts []int64
	deadline := time.Now().Add(60 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("flower clip never ends, pts=%v", pts)
		}
		h.now += 200
		f, done := p.Poll()
		if f != nil {
			pts = append(pts, f.PTSMs)
		}
		if done {
			break
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if len(pts) == 0 {
		t.Fatal("flower clip shows no frame")
	}
	for i := 1; i < len(pts); i++ {
		if pts[i] <= pts[i-1] {
			t.Fatalf("pts not monotonic: %v", pts)
		}
	}
	if !p.Stats().Ended {
		t.Fatal("flower clip never ends")
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
