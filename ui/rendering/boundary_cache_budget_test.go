package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// newBoundedBoundaryTree builds n repaint-boundary color boxes under a plain
// AbsoluteBox (the boxes' own Paint path stores into the shared cache).
func newBoundedBoundaryTree(n int) rendering.RenderObject {
	root := rendering.NewAbsoluteBox(600, 40)
	for i := 0; i < n; i++ {
		box := rendering.NewRenderColorBox(20, 20, 0.5, 0.3, 0.2, 1)
		box.SetRepaintBoundary(true)
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

// TestBoundaryCache_BudgetEvictsOldest: the explicit R14 budget caps the entry
// map. Under sustained store pressure (3-box tree, cap 2) the invariants are:
// exactly N stores per frame, Len pinned at the budget, cumulative evictions
// monotonically increasing. The per-frame eviction count itself is NOT
// deterministic: mid-walk replays touch survivors and shift the victim set.
func TestBoundaryCache_BudgetEvictsOldest(t *testing.T) {
	cache := rendering.NewBoundaryCache()
	cache.SetMaxEntries(2)
	if cache.MaxEntries() != 2 {
		t.Fatalf("MaxEntries=%d want 2", cache.MaxEntries())
	}
	root := newBoundedBoundaryTree(3)

	paintBudgetFrame(t, root, cache, 620, 60)
	if got := cache.Len(); got != 2 {
		t.Fatalf("Len after capped frame=%d want 2", got)
	}
	if got := cache.Evictions; got != 1 {
		t.Fatalf("Evictions=%d want 1", got)
	}
	if got := cache.Rerecord; got != 3 {
		t.Fatalf("Rerecord=%d want 3", got)
	}
	prev := cache.Evictions

	// All-dirty frames under cap: Len never exceeds the budget and the
	// eviction counter keeps growing (pressure keeps dropping entries).
	for i := 0; i < 3; i++ {
		markAllBoundaryDirty(root)
		paintBudgetFrame(t, root, cache, 620, 60)
		if got := cache.FrameRerecord; got != 3 {
			t.Fatalf("frame %d FrameRerecord=%d want 3", i, got)
		}
		if got := cache.Len(); got != 2 {
			t.Fatalf("frame %d Len=%d want 2 (budget enforced)", i, got)
		}
		if got := cache.Evictions; got <= prev {
			t.Fatalf("frame %d Evictions=%d not growing (prev=%d)", i, got, prev)
		}
		prev = cache.Evictions
	}

	// Uncapped default: unlimited (0), existing behavior unchanged — clean
	// frames replay all three without any drop.
	free := rendering.NewBoundaryCache()
	if free.MaxEntries() != 0 {
		t.Fatalf("default MaxEntries=%d want 0", free.MaxEntries())
	}
	paintBudgetFrame(t, root, free, 620, 60)
	if got := free.Len(); got != 3 {
		t.Fatalf("uncapped Len=%d want 3", got)
	}
	if got := free.Evictions; got != 0 {
		t.Fatalf("uncapped Evictions=%d want 0", got)
	}
	paintBudgetFrame(t, root, free, 620, 60)
	if got := free.FrameSkip; got != 3 {
		t.Fatalf("uncapped FrameSkip=%d want 3 (clean replays)", got)
	}
	if got := free.Evictions; got != 0 {
		t.Fatalf("uncapped Evictions=%d want 0", got)
	}
}
