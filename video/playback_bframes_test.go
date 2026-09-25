package video

// B player gate on the ffmpeg backend: the Go B-window machinery
// (bWindows/bSpans, GPUI_B_OFF) retired with the Go decode path —
// ffmpeg reorders B-frames natively, so laps always stay 0 and the
// opt-in changes nothing. What this gate pins instead: the long-group
// B-heavy clip plays fully to Ended with monotonic stamps and
// deterministic pixels across runs.
// Clip: testdata/vr_b_long.mp4 (96x96/Main/80 samples/2 IDR GOPs x40,
// 72% B; missing file FAILs, never Skips).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

type bShown struct {
	pts int64
	pix []byte
}

// bPlayAll plays name to Ended and copies every shown Pix (streaming
// Pix is recycled on the next Poll, so copy during the tick). parallel
// toggles Options.S2Parallel (compat no-op on the ffmpeg backend).
func bPlayAll(t *testing.T, name string, parallel bool) []bShown {
	t.Helper()
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, S2Parallel: parallel})
	if err != nil {
		t.Fatalf("open %s parallel=%v: %v", name, parallel, err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatalf("%s buffered, want streaming", name)
	}
	deadline := time.Now().Add(90 * time.Second)
	var out []bShown
	for {
		if time.Now().After(deadline) {
			t.Fatalf("not ended %s parallel=%v, shown %d", name, parallel, len(out))
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			cp := make([]byte, len(fr.Pix))
			copy(cp, fr.Pix)
			out = append(out, bShown{pts: fr.PTSMs, pix: cp})
		}
		if done {
			if got := p.BWindows(); got != 0 {
				t.Fatalf("bwindows = %d, want 0 (Go B machinery retired, ffmpeg reorders natively)", got)
			}
			return out
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

func bCheckShown(t *testing.T, shown []bShown) {
	t.Helper()
	if len(shown) != 80 {
		t.Fatalf("shown = %d, want 80 (full B-heavy play, no drops on this clip)", len(shown))
	}
	for i := 1; i < len(shown); i++ {
		if shown[i].pts <= shown[i-1].pts {
			t.Fatalf("pts not monotonic at %d: %d <= %d", i, shown[i].pts, shown[i-1].pts)
		}
	}
}

func TestBPlayerExact(t *testing.T) {
	name := filepath.Join("testdata", "vr_b_long.mp4")
	if fi, err := os.Stat(name); err != nil {
		t.Fatalf("long clip absent: %v", err)
	} else if fi.Size() != 20110 {
		t.Fatalf("long clip bytes %d want 20110 (re-record baseline if the clip changed)", fi.Size())
	}
	shown := bPlayAll(t, name, true)
	bCheckShown(t, shown)
	t.Logf("B player 80f/2gops: shown=%d first=%d last=%d", len(shown), shown[0].pts, shown[len(shown)-1].pts)
}

// TestBPlayerOffParity pins the retired kill switch: GPUI_B_OFF=1 with
// the opt-in on plays identically to the default path (same stamps,
// same pixels) — the flag is accepted and ignored on ffmpeg.
func TestBPlayerOffParity(t *testing.T) {
	name := filepath.Join("testdata", "vr_b_long.mp4")
	t.Setenv("GPUI_B_OFF", "1")
	off := bPlayAll(t, name, true)
	bCheckShown(t, off)
	on := bPlayAll(t, name, false)
	bCheckShown(t, on)
	for i := range off {
		if off[i].pts != on[i].pts {
			t.Fatalf("frame %d pts %d != %d (flag must not change output)", i, off[i].pts, on[i].pts)
		}
		if !equalBytes(off[i].pix, on[i].pix) {
			t.Fatalf("frame %d pts %d pixels differ (flag must not change output)", i, off[i].pts)
		}
	}
}
