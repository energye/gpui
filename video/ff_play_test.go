package video

import (
	"runtime"
	"testing"
	"time"
)

// Play/seek/don't-crash gate for the ffmpeg backend: open a checked-in
// clip through libgpui_ffmpeg, play to the end, seek, close. Skips when
// the library is absent; per the agreed bar it checks play + seek +
// no-crash (no pixel comparison: the Go-decode pixel gates retire with
// the old path). Hand time is fake but decode costs real time, so each
// tick yields to the background like TestStreamPathPlaysToEnd.
func TestFFmpegPlaySeekClose(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_720p.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Skipf("ffmpeg backend unavailable: %v", err)
	}
	defer p.Close()
	info := p.Info()
	if info.Width != 1280 || info.Height != 720 {
		t.Fatalf("info size %dx%d", info.Width, info.Height)
	}
	if info.Container != "ffmpeg" || info.Codec == "" {
		t.Fatalf("info backend %+v", info)
	}
	// The 720p gate clip is 5fps: tick 200ms per poll.
	shown := 0
	ended := false
	deadline := time.Now().Add(60 * time.Second)
	for !ended {
		if time.Now().After(deadline) {
			t.Fatalf("not ended after full play, shown=%d", shown)
		}
		h.now += 200
		fr, done := p.Poll()
		ended = done
		if fr != nil {
			shown++
			if len(fr.Pix) != fr.Width*fr.Height*4 {
				t.Fatalf("frame pix %d", len(fr.Pix))
			}
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if shown < 3 {
		t.Fatalf("shown=%d, want >=3", shown)
	}
	// Seek to the middle and show one frame after the landing.
	if _, err := p.SeekTo(500); err != nil {
		t.Fatalf("seek: %v", err)
	}
	ok, _, landed, _, _, _ := p.SeekInfo()
	if !ok {
		t.Fatalf("seek not recorded")
	}
	seen := false
	deadline = time.Now().Add(30 * time.Second)
	for !seen {
		if time.Now().After(deadline) {
			t.Fatalf("no frame at/after landing %d", landed)
		}
		h.now += 200
		fr, _ := p.Poll()
		if fr != nil && fr.PTSMs >= landed {
			seen = true
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	st := p.Stats()
	if st.Shown < 1 {
		t.Fatalf("stats %+v", st)
	}
}
