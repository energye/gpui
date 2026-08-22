package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// Generational eviction (R7 RSS hole): entries untouched for >120 presents are
// swept (~2s at 60Hz); boundaries touched every frame are never evicted. A
// scrolled-out VirtualList cell must release its recorded Picture instead of
// being retained forever.
func TestBoundaryCache_GenerationalEviction(t *testing.T) {
	b := rendering.NewRenderColorBox(40, 40, 0.2, 0.6, 0.9, 1)
	b.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(b, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()

	// Frame 1: cold record.
	paintWithCache(t, root, cache, 200, 200)
	if !cache.HasValid(b) {
		t.Fatal("entry must be valid after first store")
	}

	// Mounted and painted every frame for 200 presents (> eviction window):
	// a live boundary must survive every sweep.
	for i := 0; i < 200; i++ {
		paintWithCache(t, root, cache, 200, 200)
	}
	if !cache.HasValid(b) {
		t.Fatal("live (touched) entry must never be evicted")
	}

	// Scroll-out simulation: generations advance (BeginFrame) without the
	// entry being touched. Sweeps run every 30 frames; after 150 untouched
	// frames the stale entry must be gone.
	for i := 0; i < 150; i++ {
		cache.BeginFrame()
	}
	if cache.Len() != 0 {
		t.Fatalf("stale entry must be evicted after 150 untouched frames (len=%d)", cache.Len())
	}
	if cache.HasValid(b) {
		t.Fatal("evicted entry must not report valid")
	}

	// Re-entry records once again (honest remount cost, counted as rerecord).
	b.MarkNeedsPaint()
	rr, _ := paintWithCache(t, root, cache, 200, 200)
	if rr < 1 {
		t.Fatal("re-entered boundary must re-record after eviction")
	}
}
