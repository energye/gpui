package main

import (
	"os"
	"testing"

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
	_ = render.FormatRGBA8
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
