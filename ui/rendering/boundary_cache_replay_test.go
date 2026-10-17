package rendering

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/scene"
)

// TestTryReplay_SizeMismatchInvalidates covers the entry-size gate directly.
//
// The layout path always marks a resized node paint dirty (Base.setSize →
// MarkNeedsPaint), so a resize reaches the cache as will_change and the
// external test (TestBoundaryCache_SizeChangeInvalidates) pins that path. The
// size check inside tryReplay is the safety net for a stored Picture whose size
// no longer matches the node: replaying it would draw at the wrong extent, so
// the entry must be invalidated and counted as a miss instead. Size is changed
// here without the paint flag, which no public API does today — that is exactly
// the state this net exists for.
func TestTryReplay_SizeMismatchInvalidates(t *testing.T) {
	cache := NewBoundaryCache()
	box := NewRenderColorBox(30, 20, 0.4, 0.6, 0.8, 1)
	box.SetRepaintBoundary(true)
	box.setSize(Size{Width: 30, Height: 20})
	box.clearPaintDirty()

	dc := render.NewContext(60, 40)
	pc := NewPaintContext(dc, 1)
	pc.BoundaryCache = cache
	pc.UseBoundaryCache = true

	// 7 ops > the naive complexity gate 5, so the boundary is worth caching.
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		for i := 0; i < 7; i++ {
			r.FillRect(float64(i*4), 0, 3, 20, 0.4, 0.6, 0.8, 1)
		}
	})
	for i := 0; i < cache.AccessThreshold()+2; i++ {
		cache.BeginFrame()
		cache.StorePicture(pc, box, pic)
	}
	if !cache.HasValid(box) {
		t.Fatal("boundary must be cached after the access threshold")
	}

	// Size changes without a paint flag: the stored entry no longer matches.
	box.size = Size{Width: 40, Height: 20}
	cache.BeginFrame()
	if cache.tryReplay(pc, box) {
		t.Fatal("size-mismatched entry must not Replay")
	}
	if cache.FrameMiss != 1 {
		t.Fatalf("frame_miss=%d want 1", cache.FrameMiss)
	}
	if cache.HasValid(box) {
		t.Fatal("size-mismatched entry must be invalidated")
	}
}
