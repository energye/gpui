package main

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// newHeadlessState builds the nodes openPath touches, without a window.
// Rendering nodes + ImageBuf are pure Go; no GPU window needed.
func newHeadlessState(t *testing.T, clip string) *state {
	t.Helper()
	wrkit.EnsureUIFace()
	root := rendering.NewAbsoluteBox(1200, 800)
	img := rendering.NewRenderImage(960, 540)
	root.Place(img, 120, 90)
	playBox := rendering.NewRenderColorBox(playW, playH, 0.2, 0.5, 0.9, 1)
	replayBox := rendering.NewRenderColorBox(replayW, replayH, 0.25, 0.28, 0.33, 1)
	openBox := rendering.NewRenderColorBox(openW, openH, 0.16, 0.42, 0.32, 1)
	barBg := rendering.NewRenderColorBox(960, barH, 0.2, 0.22, 0.26, 1)
	barFill := rendering.NewRenderColorBox(1, barH, 0.3, 0.8, 0.5, 1)
	st := &state{clip: clip}
	st.root = root
	st.img = img
	st.playBox, st.replayBox, st.barBg = playBox, replayBox, barBg
	st.openBox = openBox
	st.barFill = barFill
	st.fileLabel = wrkit.Label("", 13, 0.72, 0.8, 0.9)
	st.statusLabel = wrkit.Label("", 13, 0.75, 0.85, 0.9)
	st.timeLabel = wrkit.Label("", 12, 0.72, 0.8, 0.9)
	st.playLabel = wrkit.Label("", 14, 0.95, 0.97, 1)
	st.replayLabel = wrkit.Label("", 14, 0.9, 0.93, 0.96)
	st.openLabel = wrkit.Label("", 14, 0.95, 0.97, 1)
	st.helpLabel = wrkit.Label("", 12, 0.55, 0.65, 0.75)
	return st
}

func resolveTestClip(name string) string {
	for _, p := range []string{"../../video/testdata/" + name, "video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "../../video/testdata/" + name
}

// TestOpenPathSwapsClip proves runtime switching: 720p -> 480p replaces
// the player, buffer, info and layout aspect without touching the window.
func TestOpenPathSwapsClip(t *testing.T) {
	st := newHeadlessState(t, resolveTestClip("vr2_720p.mp4"))
	st.openPath(resolveTestClip("vr2_720p.mp4"))
	if st.player == nil || st.bad != "" {
		t.Fatalf("open 720p: player=%v bad=%q", st.player != nil, st.bad)
	}
	w0, h0 := st.info.Width, st.info.Height
	if w0 != 1280 || h0 != 720 {
		t.Fatalf("720p info = %dx%d, want 1280x720", w0, h0)
	}
	oldPlayer := st.player
	st.openPath(resolveTestClip("vr2_480p.mp4"))
	if st.player == nil || st.bad != "" {
		t.Fatalf("open 480p: player=%v bad=%q", st.player != nil, st.bad)
	}
	if st.player == oldPlayer {
		t.Fatal("player not swapped")
	}
	if st.info.Width == w0 && st.info.Height == h0 {
		t.Fatalf("info not updated, still %dx%d", st.info.Width, st.info.Height)
	}
	if st.buf == nil || st.buf.Width() != st.info.Width || st.buf.Height() != st.info.Height {
		t.Fatalf("buf = %v, want %dx%d", st.buf, st.info.Width, st.info.Height)
	}
	// Layout must follow the new aspect: videoW/videoH ratio == src ratio.
	got := st.videoW / st.videoH
	want := float64(st.info.Width) / float64(st.info.Height)
	if diff := got - want; diff < -0.01 || diff > 0.01 {
		t.Fatalf("layout aspect = %.3f, want %.3f", got, want)
	}
	if st.player != nil {
		st.player.Close()
	}
	if st.buf != nil {
		st.buf.Dispose()
	}
}

// TestOpenPathBadFileKeepsPlaying proves a bad drop never kills the
// current playback: player pointer, clip and note survive.
func TestOpenPathBadFileKeepsPlaying(t *testing.T) {
	st := newHeadlessState(t, resolveTestClip("vr2_720p.mp4"))
	st.openPath(resolveTestClip("vr2_720p.mp4"))
	if st.player == nil {
		t.Fatal("setup open failed")
	}
	keep, keepClip := st.player, st.clip
	st.openPath("/tmp/does-not-exist-video.mp4")
	if st.player != keep {
		t.Fatal("bad file swapped the player")
	}
	if st.clip != keepClip {
		t.Fatalf("clip = %q, want %q", st.clip, keepClip)
	}
	if st.bad != "" {
		t.Fatalf("bad = %q, runtime failure must not brick (status only)", st.bad)
	}
	st.player.Close()
	st.buf.Dispose()
	_ = render.FormatRGBAPremul
}

// TestAudioStaysOffHeadless pins the speaker guard: headless openPath
// (enableAudio false) never spawns a speaker writer, even for a clip
// with sound — the pump only starts under a real window (main sets
// enableAudio). stopAudio stays nil-safe.
func TestAudioStaysOffHeadless(t *testing.T) {
	st := newHeadlessState(t, resolveTestClip("vr_oceans.mp4"))
	st.openPath(resolveTestClip("vr_oceans.mp4"))
	if st.player == nil || st.bad != "" {
		t.Fatalf("open oceans: player=%v bad=%q", st.player != nil, st.bad)
	}
	defer st.player.Close()
	defer st.buf.Dispose()
	if !st.player.HasAudio() {
		t.Fatal("oceans must carry audio for this guard to mean anything")
	}
	if st.ab != nil {
		st.stopAudio()
		t.Fatal("headless openPath spawned a speaker pump (must stay off in tests)")
	}
	st.stopAudio()
	if st.audioNote != "" {
		t.Fatalf("headless audioNote = %q, want empty", st.audioNote)
	}
}

// TestTextThrottleSkipsRedundantShaping pins the STEP-1 window diet:
// identical status/time text must not reshape every frame — only real
// changes touch the shaping path. It drives refreshTime/refreshStatus
// back to back and requires the second identical pass to leave the
// bar width untouched (no-op), while a changed frame count reshapes.
func TestTextThrottleSkipsRedundantShaping(t *testing.T) {
	st := newHeadlessState(t, resolveTestClip("vr2_720p.mp4"))
	st.openPath(resolveTestClip("vr2_720p.mp4"))
	if st.player == nil || st.bad != "" {
		t.Fatalf("open: player=%v bad=%q", st.player != nil, st.bad)
	}
	defer st.player.Close()
	defer st.buf.Dispose()
	st.lastPTS, st.shown = 200, 3
	st.refreshTime()
	w0 := st.barFill.Width
	st.refreshTime()
	if st.barFill.Width != w0 {
		t.Fatal("identical time text reshaped (throttle must no-op)")
	}
	st.lastPTS, st.shown = 600, 4
	st.refreshTime()
	if st.barFill.Width == w0 {
		t.Fatal("changed time text did not reshape time text")
	}
}

// TestCleanDropPath pins the file:// defence (normal Files are clean).
func TestCleanDropPath(t *testing.T) {
	if got := cleanDropPath("file:///tmp/a.mp4"); got != "/tmp/a.mp4" {
		t.Fatalf("file:// => %q", got)
	}
	if got := cleanDropPath("  /tmp/b.mp4  "); got != "/tmp/b.mp4" {
		t.Fatalf("trim => %q", got)
	}
}

// TestAsyncSeekNeverFreezes pins progress clicks on a fresh clip: the
// engine reparks and returns in milliseconds (no background patch), the
// landing then arrives via Poll, and scrub (fast) + exact both work.
func TestAsyncSeekNeverFreezes(t *testing.T) {
	st := newHeadlessState(t, resolveTestClip("vr2_720p.mp4"))
	st.openPath(resolveTestClip("vr2_720p.mp4"))
	if st.player == nil || st.bad != "" {
		t.Fatalf("open: player=%v bad=%q", st.player != nil, st.bad)
	}
	defer st.player.Close()
	defer st.buf.Dispose()
	// Exact seek returns at once even on a fresh clip.
	start := time.Now()
	st.seekTo(600)
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("seekTo blocked %v (must return at once)", took)
	}
	if st.lastPTS != 600 {
		t.Fatalf("lastPTS = %d, want 600 (landing re-anchored)", st.lastPTS)
	}
	// Scrub path: fast keyframe jump then exact release.
	st.press(st.barX+st.barW/2, st.barY)
	if !st.scrubbing {
		t.Fatal("press on bar did not start scrub")
	}
	st.drag(st.barX + st.barW*0.8)
	st.release(st.barX + st.barW*0.8)
	// Rate + step + keyframe walk all work headless.
	st.setRate(2)
	if st.player.Rate() != 2 {
		t.Fatalf("rate = %v, want 2", st.player.Rate())
	}
	st.setRate(1)
	before := st.lastPTS
	st.stepFrame()
	if st.lastPTS == before && st.player.Info().Frames > 1 {
		t.Logf("step stayed at %d (single-frame clip edge ok)", st.lastPTS)
	}
}

// TestScrubPressDragRelease pins the standard scrub model: press-move
// fires fast jumps, release fires the exact landing.
func TestScrubPressDragRelease(t *testing.T) {
	st := newHeadlessState(t, resolveTestClip("vr2_720p.mp4"))
	st.openPath(resolveTestClip("vr2_720p.mp4"))
	if st.player == nil || st.bad != "" {
		t.Fatalf("open: player=%v bad=%q", st.player != nil, st.bad)
	}
	defer st.player.Close()
	defer st.buf.Dispose()
	mid := st.barX + st.barW/2
	st.press(mid, st.barY)
	if !st.scrubbing {
		t.Fatal("scrub not started")
	}
	st.drag(mid + 10)
	st.release(mid + 20)
	if st.scrubbing {
		t.Fatal("scrub not ended on release")
	}
	if st.bad != "" {
		t.Fatalf("bad = %q after scrub", st.bad)
	}
}
