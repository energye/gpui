package scene_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// Long steady run with caret blinking: B text must never appear left of the
// box (the "B/C spill develops over time with zero input" class). A handful
// of frames never reproduces it — the corruption needs dozens of blink
// re-records — so pump 300 frames like the live window does.
func TestRetainedSpill_LongBlinkRun(t *testing.T) {
	root, owner, box := acceptBoxB(t)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatalf("B focus request failed")
	}
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	tex := scene.NewPictureTextureCache(dc, 256)
	const frames = 300
	for i := 0; i < frames; i++ {
		dc.BeginFrame()
		box.TickCaret(0.6)
		owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
		compositeFrame(t, dc, tex, owner, root, uint64(i+1))
		if i == 0 && tex.Len() == 0 {
			t.Skipf("no GPU texture entries (no GPU?)")
		}
		if i%50 == 49 {
			checkLeftOfBoxClean(t, dc.Image(), i+1)
		}
	}
	checkLeftOfBoxClean(t, dc.Image(), frames)
}

// checkLeftOfBoxClean asserts the region left of the B box (root background
// only — acceptBoxB builds no sidebar) stays flat. Spilled text adds edges.
func checkLeftOfBoxClean(t *testing.T, img image.Image, frame int) {
	t.Helper()
	var lo, hi uint32 = 1 << 30, 0
	for y := 170; y < 230; y++ {
		for x := 0; x < 290; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			v := (r + g + b) / 3
			if v < lo {
				lo = v
			}
			if v > hi {
				hi = v
			}
		}
	}
	if hi-lo > 6000 {
		t.Fatalf("frame %d: text spills left of B box (brightness range %d)", frame, hi-lo)
	}
}

// Scroll far left: the box must still show the same text as a live vector
// repaint. Scrolling past the retained band margin without re-recording
// leaves a stale band blitted at the new offset (empty box). Nothing blinks
// between the two grabs, so the caret bar is identical in both.
func TestRetainedScrollBandExpiry(t *testing.T) {
	root, owner, box := acceptBoxB(t)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatalf("B focus request failed")
	}
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	tex := scene.NewPictureTextureCache(dc, 256)
	// Valid layout exists now (acceptBoxB flushed); re-sync so the first
	// recording holds the tail band, not the pre-layout head band.
	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	compositeFrame(t, dc, tex, owner, root, 1)
	if tex.Len() == 0 {
		t.Skipf("no GPU texture entries (no GPU?)")
	}
	if n := boxInkPixels(t, dc.Image()); n < 500 {
		t.Fatalf("tail frame: want text in B box, got %d ink px", n)
	}
	// Caret near the head via real key presses (the user path): each Left
	// moves one char and re-syncs scroll/hint through OnKey. 260 presses
	// move ~2000px, far past the 200px retained band margin.
	for i := 0; i < 260; i++ {
		box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowLeft})
	}
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	dc.BeginFrame()
	retained := compositeFrame(t, dc, tex, owner, root, 2)
	vectored := vectorFrame(t, owner, root)
	if n := boxInkPixels(t, vectored); n < 500 {
		t.Fatalf("vector reference itself is empty (%d ink px) — cannot validate", n)
	}
	boxR := image.Rect(296, 180, 1176, 220)
	n, bb := pixDiff(retained, vectored, boxR, 2)
	t.Logf("retained-vs-vector diff: %d px bbox=%v", n, bb)
	if n > 3000 {
		t.Fatalf("scrolled retained frame differs from live repaint: %d px bbox=%v (stale band)", n, bb)
	}
}

// Scroll with frames interleaved, starting from the production warm-up
// state (a full vector paint clears the fresh dirty flags before the first
// retained texture exists). Without band establishment the first texture is
// recorded without a note and every later scroll check trusts an unknown
// band (empty box, recordedIn stuck at 1). Presses go through the real OnKey
// path in 5-press batches with a composite between batches.
func TestRetainedScrollBandEstablishAfterWarmup(t *testing.T) {
	root, owner, box := acceptBoxB(t)
	fm := focus.NewManager()
	fm.Register(box.FocusNode())
	if !box.FocusNode().RequestFocus() {
		t.Fatalf("B focus request failed")
	}
	dc := render.NewContext(1200, 800)
	defer dc.Close()
	dc.BeginFrame()
	tex := scene.NewPictureTextureCache(dc, 256)
	box.Sync()
	owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
	owner.FlushPaint(rendering.NewPaintContext(dc, dc.DeviceScale()), true)
	compositeFrame(t, dc, tex, owner, root, 1)
	if tex.Len() == 0 {
		t.Skipf("no GPU texture entries (no GPU?)")
	}
	if n := boxInkPixels(t, dc.Image()); n < 500 {
		t.Fatalf("tail frame: want text in B box, got %d ink px", n)
	}
	var frameID uint64 = 2
	for i := 0; i < 52; i++ {
		for k := 0; k < 5; k++ {
			box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowLeft})
		}
		owner.FlushLayout(rendering.Size{Width: 1200, Height: 800}, false)
		dc.BeginFrame()
		compositeFrame(t, dc, tex, owner, root, frameID)
		frameID++
	}
	retained := dc.Image()
	vectored := vectorFrame(t, owner, root)
	if n := boxInkPixels(t, vectored); n < 500 {
		t.Fatalf("vector reference itself is empty (%d ink px) — cannot validate", n)
	}
	boxR := image.Rect(296, 180, 1176, 220)
	n, bb := pixDiff(retained, vectored, boxR, 2)
	t.Logf("warmup interleaved retained-vs-vector diff: %d px bbox=%v", n, bb)
	if n > 3000 {
		t.Fatalf("warmup interleaved scroll differs from live repaint: %d px bbox=%v", n, bb)
	}
}

// boxInkPixels counts non-background pixels inside the B box rect.
func boxInkPixels(t *testing.T, img image.Image) int {
	t.Helper()
	n := 0
	for y := 180; y < 220; y++ {
		for x := 296; x < 1176; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r > 0x4000 || g > 0x4000 || b > 0x4000 {
				n++
			}
		}
	}
	return n
}
