package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// newBoundedBoundaryTree builds n complex repaint boundaries under a plain
// AbsoluteBox (each is worth caching: 1 background + complexFills = 7 ops).
func newBoundedBoundaryTree(n int) rendering.RenderObject {
	root := rendering.NewAbsoluteBox(600, 40)
	for i := 0; i < n; i++ {
		box := complexBoundary(20, 20, complexFills)
		root.Place(box, float64(i*22), 10)
	}
	return root
}

// paintBudgetFrame runs one boundary-cache paint walk over the tree.
func paintBudgetFrame(t *testing.T, root rendering.RenderObject, cache *rendering.BoundaryCache, w, h int) {
	t.Helper()
	dc := render.NewContext(w, h)
	pc := rendering.NewPaintContext(dc, 1)
	pc.BoundaryCache = cache
	pc.UseBoundaryCache = true
	cache.BeginFrame()
	root.Paint(pc)
}

// markAllBoundaryDirty marks every repaint boundary in the tree dirty.
func markAllBoundaryDirty(root rendering.RenderObject) {
	var mark func(n rendering.RenderObject)
	mark = func(n rendering.RenderObject) {
		if n.IsRepaintBoundary() {
			n.MarkNeedsPaint()
		}
		for _, ch := range n.Children() {
			mark(ch)
		}
	}
	mark(root)
}

// TestBoundaryCache_BudgetCapsEntries: the explicit R14 budget caps the entry
// map. Entries enter through the access threshold (warm-up); a dirty frame
// drops its own entry instead of adding one (will_change); the cap evicts only
// entries not used in the current frame (the retained texture cache's LRU
// rule), and when the live set alone fills the cap, new entries are refused —
// so the map settles at the live-set size with steady replays and no re-record
// thrash.
func TestBoundaryCache_BudgetCapsEntries(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	if cache.MaxEntries() != 0 {
		t.Fatalf("default MaxEntries=%d want 0 (unlimited)", cache.MaxEntries())
	}
	root := newBoundedBoundaryTree(3)

	// Warm: all three boundaries enter the cache (dirty frame + 3 held-back
	// frames + the admitting frame).
	for i := 0; i < 5; i++ {
		paintBudgetFrame(t, root, cache, 620, 60)
	}
	if got := cache.Len(); got != 3 {
		t.Fatalf("uncapped Len=%d want 3", got)
	}
	if got := cache.Evictions; got != 0 {
		t.Fatalf("uncapped Evictions=%d want 0", got)
	}
	paintBudgetFrame(t, root, cache, 620, 60)
	if got := cache.FrameSkip; got != 3 {
		t.Fatalf("uncapped FrameSkip=%d want 3 (clean replays)", got)
	}

	// Cap below the entry count: entries used in the current frame are never
	// sacrificed, so the cap binds from the next frame on.
	cache.SetMaxEntries(2)
	if got := cache.Len(); got != 3 {
		t.Fatalf("Len right after the cap=%d want 3 (in-use entries kept)", got)
	}
	paintBudgetFrame(t, root, cache, 620, 60)
	if got := cache.Len(); got != 2 {
		t.Fatalf("Len after the next frame=%d want 2 (oldest unused entry dropped)", got)
	}
	if got := cache.Evictions; got < 1 {
		t.Fatalf("Evictions=%d want >=1 (cap enforced)", got)
	}

	// Sustained dirty frames must not re-grow the map: every store attempt is
	// gated by will_change and drops the previous-generation entry.
	for i := 0; i < 3; i++ {
		markAllBoundaryDirty(root)
		paintBudgetFrame(t, root, cache, 620, 60)
		if got := cache.FrameRerecord; got != 0 {
			t.Fatalf("dirty frame %d FrameRerecord=%d want 0 (changed content is not cached)", i, got)
		}
		if got := cache.Len(); got > 2 {
			t.Fatalf("dirty frame %d Len=%d exceeds the budget", i, got)
		}
	}
	if got := cache.Len(); got != 0 {
		t.Fatalf("entries after the dirty burst=%d want 0 (changed content drops its entry)", got)
	}

	// Steady frames re-fill at the access threshold; exactly the permitted two
	// are admitted and the third is refused (live paint, no store, no thrash).
	for i := 0; i < 4; i++ {
		paintBudgetFrame(t, root, cache, 620, 60)
		if got := cache.Len(); got > 2 {
			t.Fatalf("steady frame %d Len=%d exceeds the budget", i, got)
		}
	}
	if got := cache.Len(); got != 2 {
		t.Fatalf("steady Len=%d want 2 (capped survivors)", got)
	}
	_, _, _, limit, _ := cache.PolicyCounts()
	if limit < 1 {
		t.Fatalf("limit_rejects=%d want >=1 (budget deferred the third entry)", limit)
	}
	prevLimit := limit
	for i := 0; i < 3; i++ {
		paintBudgetFrame(t, root, cache, 620, 60)
		rr, sk := cache.FrameRerecord, cache.FrameSkip
		if rr != 0 || sk != 2 {
			t.Fatalf("steady frame %d rr=%d sk=%d want 0/2 (survivors replay, third stays live)", i, rr, sk)
		}
		if got := cache.Len(); got != 2 {
			t.Fatalf("steady frame %d Len=%d want 2", i, got)
		}
		_, _, _, limit, _ = cache.PolicyCounts()
		if limit <= prevLimit {
			t.Fatalf("steady frame %d limit_rejects=%d want > %d (third entry deferred every frame)", i, limit, prevLimit)
		}
		prevLimit = limit
	}
}
