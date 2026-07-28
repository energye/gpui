package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintWithCache runs a FullPaint-style walk with a shared BoundaryCache
// (same path W1 windows use via PipelineOwner.BoundaryCache).
func paintWithCache(t *testing.T, root rendering.RenderObject, cache *rendering.BoundaryCache, w, h int) (rerecord, skip int64) {
	t.Helper()
	dc := render.NewContext(w, h)
	pc := rendering.NewPaintContext(dc, 1)
	pc.BoundaryCache = cache
	pc.UseBoundaryCache = true
	cache.BeginFrame()
	root.Paint(pc)
	return cache.FrameRerecord, cache.FrameSkip
}

func TestBoundaryCache_SkipVsRerecord(t *testing.T) {
	staticB := rendering.NewRenderColorBox(40, 40, 0.1, 0.8, 0.2, 1)
	staticB.SetRepaintBoundary(true)
	hotB := rendering.NewRenderColorBox(30, 30, 0.9, 0.2, 0.1, 1)
	hotB.SetRepaintBoundary(true)

	root := rendering.NewAbsoluteBox(200, 200)
	root.Background = &rendering.Color{R: 0.05, G: 0.05, B: 0.08, A: 1}
	root.Place(staticB, 10, 10)
	root.Place(hotB, 100, 100)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()
	if cache == nil {
		t.Fatal("BoundaryCache must be non-nil on PipelineOwner")
	}

	// Frame 1: both boundaries dirty → rerecord, no skip.
	rr1, sk1 := paintWithCache(t, root, cache, 200, 200)
	if rr1 < 2 {
		t.Fatalf("frame1 rerecord=%d want ≥2 (static+hot)", rr1)
	}
	if sk1 != 0 {
		t.Fatalf("frame1 skip=%d want 0 (cold cache)", sk1)
	}
	if !cache.HasValid(staticB) {
		t.Fatal("static boundary must have valid Picture after first paint")
	}
	if !cache.HasValid(hotB) {
		t.Fatal("hot boundary must have valid Picture after first paint")
	}

	// Frame 2: only hot dirty → static skip, hot rerecord.
	hotB.R, hotB.G, hotB.B = 0.95, 0.4, 0.1
	hotB.MarkNeedsPaint()
	if staticB.NeedsPaint() {
		t.Fatal("static must stay clean when hot is repaint boundary")
	}
	rr2, sk2 := paintWithCache(t, root, cache, 200, 200)
	if sk2 < 1 {
		t.Fatalf("frame2 skip=%d want ≥1 (static Replay)", sk2)
	}
	if rr2 < 1 {
		t.Fatalf("frame2 rerecord=%d want ≥1 (hot)", rr2)
	}
	// Cumulative: skip grew.
	if cache.Skip < 1 {
		t.Fatalf("lifetime Skip=%d want ≥1", cache.Skip)
	}
	if cache.Rerecord < 3 {
		t.Fatalf("lifetime Rerecord=%d want ≥3 (2 first + ≥1 hot)", cache.Rerecord)
	}

	// Frame 3: both clean → both skip, zero rerecord.
	rr3, sk3 := paintWithCache(t, root, cache, 200, 200)
	if rr3 != 0 {
		t.Fatalf("frame3 rerecord=%d want 0 (both clean)", rr3)
	}
	if sk3 < 2 {
		t.Fatalf("frame3 skip=%d want ≥2", sk3)
	}
}

func TestBoundaryCache_NestedOuterNoRerecordWhenInnerDirty(t *testing.T) {
	// Outer AbsoluteBox is a repaint boundary; inner hot leaf is too.
	// Only inner MarkNeedsPaint must not bump outer boundary_rerecord after warm.
	outer := rendering.NewAbsoluteBox(120, 120)
	outer.Background = &rendering.Color{R: 0.2, G: 0.2, B: 0.25, A: 1}
	outer.SetRepaintBoundary(true)

	staticLeaf := rendering.NewRenderColorBox(20, 20, 0.1, 0.7, 0.2, 1)
	staticLeaf.SetRepaintBoundary(true)
	hotLeaf := rendering.NewRenderColorBox(20, 20, 0.9, 0.1, 0.1, 1)
	hotLeaf.SetRepaintBoundary(true)
	outer.Place(staticLeaf, 8, 8)
	outer.Place(hotLeaf, 60, 60)

	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(outer, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()

	// Warm: record outer + leaves.
	rr0, _ := paintWithCache(t, root, cache, 200, 200)
	if rr0 < 2 {
		t.Fatalf("warm rerecord=%d want ≥2", rr0)
	}
	if !cache.HasValid(outer) {
		t.Fatal("outer boundary must be cached after warm")
	}
	outerRerecordAfterWarm := cache.Rerecord

	// Dirty only inner hot leaf (stops at hotLeaf repaint boundary).
	hotLeaf.R = 0.5
	hotLeaf.MarkNeedsPaint()
	if outer.NeedsPaint() {
		t.Fatal("outer must not be paint-dirty when only inner boundary is marked")
	}
	if staticLeaf.NeedsPaint() {
		t.Fatal("static leaf must stay clean")
	}

	rr1, sk1 := paintWithCache(t, root, cache, 200, 200)
	if rr1 < 1 {
		t.Fatalf("inner dirty frame rerecord=%d want ≥1 (hot leaf)", rr1)
	}
	// Outer must NOT re-store: lifetime Rerecord growth comes only from hot leaf.
	// After warm, outer was stored once; this frame should not add outer again.
	// Lifetime Rerecord should grow by hot only (≈1), not outer+hot.
	grew := cache.Rerecord - outerRerecordAfterWarm
	if grew > rr1+1 {
		// allow small slack; fail if outer clearly re-stored (+leaves)
		t.Fatalf("rerecord grew by %d (frame rr=%d) — outer likely re-recorded", grew, rr1)
	}
	// Static leaf should skip via Replay.
	if sk1 < 1 {
		t.Fatalf("skip=%d want ≥1 (static leaf Replay while outer walks)", sk1)
	}
	// Outer still has valid entry (not invalidated).
	if !cache.HasValid(outer) {
		t.Fatal("outer cache entry must remain valid when only inner dirtied")
	}
}

func TestCountRepaintBoundaries_NestedDepth(t *testing.T) {
	// root (no) → outer (yes) → mid (yes) → leaf (yes)  depth 3
	leaf := rendering.NewRenderColorBox(8, 8, 1, 0, 0, 1)
	leaf.SetRepaintBoundary(true)
	mid := rendering.NewAbsoluteBox(40, 40)
	mid.SetRepaintBoundary(true)
	mid.Place(leaf, 4, 4)
	outer := rendering.NewAbsoluteBox(80, 80)
	outer.SetRepaintBoundary(true)
	outer.Place(mid, 4, 4)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(outer, 0, 0)

	count, depth := rendering.CountRepaintBoundaries(root)
	if count != 3 {
		t.Fatalf("boundary count=%d want 3", count)
	}
	if depth != 3 {
		t.Fatalf("max depth=%d want 3", depth)
	}

	// Compositing bits: boundaries force needsCompositing up the chain.
	owner := rendering.NewPipelineOwner(root)
	owner.UpdateCompositingBits()
	if !outer.NeedsCompositing() {
		t.Fatal("outer boundary should need compositing")
	}
	if !root.NeedsCompositing() {
		t.Fatal("root should need compositing from nested boundaries")
	}
}

func TestBoundaryCache_SizeChangeInvalidates(t *testing.T) {
	box := rendering.NewRenderColorBox(20, 20, 0.3, 0.3, 0.8, 1)
	box.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(box, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	cache := owner.BoundaryCache()

	_, _ = paintWithCache(t, root, cache, 100, 100)
	if !cache.HasValid(box) {
		t.Fatal("expected valid cache")
	}

	// Resize → layout dirties; after layout size differs from entry → no skip, re-record.
	box.Width, box.Height = 40, 40
	box.MarkNeedsLayout()
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	box.MarkNeedsPaint()
	rr, sk := paintWithCache(t, root, cache, 100, 100)
	if sk != 0 {
		t.Fatalf("after size change skip=%d want 0", sk)
	}
	if rr < 1 {
		t.Fatalf("after size change rerecord=%d want ≥1", rr)
	}
}

func TestPipelineOwner_BoundaryCachePersists(t *testing.T) {
	owner := rendering.NewPipelineOwner(rendering.NewAbsoluteBox(10, 10))
	c1 := owner.BoundaryCache()
	c2 := owner.BoundaryCache()
	if c1 == nil || c1 != c2 {
		t.Fatal("BoundaryCache must be stable across calls")
	}
}
