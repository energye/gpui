package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// Generational eviction: entries untouched for >120 presents are
// swept (~2s at 60Hz); boundaries touched every frame are never evicted. A
// scrolled-out VirtualList cell must release its recorded Picture instead of
// being retained forever.
func TestBoundaryCache_GenerationalEviction(t *testing.T) {
	b := complexBoundary(40, 40, complexFills)
	root := newPolicyTree(200, 200, b)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()

	// Warm-up: the boundary enters the cache once its content has been stable
	// for accessThreshold+1 frames.
	warmBoundaryCache(t, root, cache, 200, 200)
	if !cache.HasValid(b) {
		t.Fatal("entry must be valid after the access threshold")
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

	// Re-entry: the boundary live-paints again and re-enters the cache after
	// the access threshold (honest remount cost, counted as rerecord).
	rrBefore := cache.Rerecord
	for i := 0; i < 5; i++ {
		paintWithCache(t, root, cache, 200, 200)
	}
	if !cache.HasValid(b) {
		t.Fatal("re-entered boundary must be cached again after the access threshold")
	}
	if got := cache.Rerecord - rrBefore; got != 1 {
		t.Fatalf("remount rerecord=%d want 1 (one re-record, then Replay)", got)
	}
	if _, sk := paintWithCache(t, root, cache, 200, 200); sk != 1 {
		t.Fatalf("post-remount skip=%d want 1", sk)
	}
}
