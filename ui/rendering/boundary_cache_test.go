package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintWithCache runs a FullPaint-style walk with a shared BoundaryCache
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

// TestBoundaryCache_StaticSkipsHotStaysLive: a settled boundary Replays while a
// sibling whose content changes repaints live; the changed sibling re-enters
// the cache only after its content stays unchanged (access threshold).
func TestBoundaryCache_StaticSkipsHotStaysLive(t *testing.T) {
	staticB := complexBoundary(120, 60, complexFills)
	hotB := complexBoundary(120, 60, complexFills)
	root := newPolicyTree(300, 200, staticB, hotB)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 300, Height: 200}, true)
	cache := owner.BoundaryCache()
	if cache == nil {
		t.Fatal("BoundaryCache must be non-nil on PipelineOwner")
	}

	// Warm: both boundaries enter the cache after the access threshold.
	warmBoundaryCache(t, root, cache, 300, 200)
	if !cache.HasValid(staticB) || !cache.HasValid(hotB) {
		t.Fatal("both boundaries must be cached after warm-up")
	}
	if rr, sk := paintWithCache(t, root, cache, 300, 200); rr != 0 || sk != 2 {
		t.Fatalf("steady rr=%d sk=%d want 0/2", rr, sk)
	}

	// Hot changes content: live paint; its re-store is gated by will_change.
	hotB.Background.R = 0.95
	hotB.MarkNeedsPaint()
	if staticB.NeedsPaint() {
		t.Fatal("static must stay clean when the hot boundary repaints")
	}
	rr, sk := paintWithCache(t, root, cache, 300, 200)
	if sk != 1 {
		t.Fatalf("static must Replay while hot repaints (sk=%d want 1)", sk)
	}
	if rr != 0 {
		t.Fatalf("hot re-store must be held back by will_change (rr=%d want 0)", rr)
	}

	// The hot boundary settles: 3 held-back frames + the admitting frame.
	for i := 0; i < 4; i++ {
		paintWithCache(t, root, cache, 300, 200)
	}
	if !cache.HasValid(hotB) {
		t.Fatal("hot boundary must be re-cached once its content settles")
	}
	if rr, sk := paintWithCache(t, root, cache, 300, 200); rr != 0 || sk != 2 {
		t.Fatalf("settled rr=%d sk=%d want 0/2", rr, sk)
	}
}

// TestBoundaryCache_NestedAbsoluteBoxCleanReplayKeepsMidContent: outer own-content
// Replay + walk nested RB children so mid/leaf still paint (mid not baked in outer).
func TestBoundaryCache_NestedAbsoluteBoxCleanReplayKeepsMidContent(t *testing.T) {
	const W, H = 200, 200
	outer := complexBoundary(160, 160, complexFills)
	outer.Background = &rendering.Color{R: 0.1, G: 0.1, B: 0.5, A: 1} // blue outer

	mid := complexBoundary(100, 100, complexFills)
	mid.Background = &rendering.Color{R: 0.0, G: 0.6, B: 0.6, A: 1} // teal mid

	leaf := complexBoundary(40, 40, complexFills)
	leaf.Background = &rendering.Color{R: 0.0, G: 0.9, B: 0.1, A: 1} // bright green

	mid.Place(leaf, 20, 20)
	outer.Place(mid, 20, 20)

	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.Place(outer, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	// Warm admits all three (own-content caches, not one outer bake).
	warmBoundaryCache(t, root, cache, W, H)
	if !cache.HasValid(outer) || !cache.HasValid(mid) || !cache.HasValid(leaf) {
		t.Fatal("warm must cache outer, mid, and leaf separately")
	}

	// Fully clean + wipe: outer/mid/leaf each Replay; mid+leaf must still be visible.
	dc2 := render.NewContext(W, H)
	dc2.BeginFrame()
	dc2.ClearWithColor(render.White)
	pc2 := rendering.NewPaintContext(dc2, 1)
	pc2.BoundaryCache, pc2.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc2)
	if cache.FrameSkip < 3 {
		t.Fatalf("clean frame skip=%d want ≥3 (outer+mid+leaf)", cache.FrameSkip)
	}
	if cache.FrameRerecord != 0 {
		t.Fatalf("clean frame rerecord=%d want 0", cache.FrameRerecord)
	}

	img := dc2.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	// Layout: outer@(10,10), mid@(30,30), leaf@(50,50) 40×40.
	mr, mg, mb, _ := img.At(40, 40).RGBA()
	if mg < 0x8000 || mb < 0x8000 || mr > 0x6000 {
		t.Fatalf("mid-only (40,40)=#%04x%04x%04x want teal (mid own Replay after outer)", mr, mg, mb)
	}
	lr, lg, lb, _ := img.At(70, 70).RGBA()
	if lg < 0xA000 || lr > 0x4000 {
		t.Fatalf("leaf (70,70)=#%04x%04x%04x want green", lr, lg, lb)
	}
	or, og, ob, _ := img.At(20, 20).RGBA()
	if ob < 0x6000 || or > 0x4000 {
		t.Fatalf("outer-only (20,20)=#%04x%04x%04x want blue", or, og, ob)
	}
}

// TestBoundaryCache_InnerChangeThenCleanReplayShowsFresh: leaf owns its Picture;
// outer must not bake leaf. After red→green the clean Replay shows green
// without ancestor invalidate / outer rerecord.
func TestBoundaryCache_InnerChangeThenCleanReplayShowsFresh(t *testing.T) {
	const W, H = 200, 200
	outer := complexBoundary(160, 160, complexFills)
	outer.Background = &rendering.Color{R: 0.1, G: 0.1, B: 0.5, A: 1}

	mid := complexBoundary(100, 100, complexFills)
	mid.Background = &rendering.Color{R: 0.0, G: 0.5, B: 0.5, A: 1}

	leaf := complexBoundary(40, 40, complexFills)
	leaf.Background = &rendering.Color{R: 0.95, G: 0.05, B: 0.05, A: 1} // red warm
	mid.Place(leaf, 20, 20)
	outer.Place(mid, 20, 20)

	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.Place(outer, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	warmBoundaryCache(t, root, cache, W, H)
	if !cache.HasValid(outer) || !cache.HasValid(mid) || !cache.HasValid(leaf) {
		t.Fatal("warm must cache outer, mid, and leaf")
	}
	outerRRWarm := cache.Rerecord

	leaf.Background.R, leaf.Background.G, leaf.Background.B = 0.05, 0.95, 0.1 // green
	leaf.MarkNeedsPaint()
	if outer.NeedsPaint() {
		t.Fatal("outer must stay clean when only the leaf boundary is marked")
	}
	dc2 := render.NewContext(W, H)
	pc2 := rendering.NewPaintContext(dc2, 1)
	pc2.BoundaryCache, pc2.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc2)
	if cache.FrameRerecord != 0 {
		t.Fatalf("leaf dirty frame rerecord=%d want 0 (outer/mid must not re-store)", cache.FrameRerecord)
	}
	if cache.Skip < 2 {
		t.Fatalf("leaf dirty frame skip=%d want ≥2 (outer+mid own Replay)", cache.Skip)
	}
	if !cache.HasValid(outer) {
		t.Fatal("outer own-content cache must remain valid (not invalidated by leaf repaint)")
	}
	// The leaf re-enters the cache after three unchanged frames + the admit frame.
	for i := 0; i < 4; i++ {
		paintWithCache(t, root, cache, W, H)
	}
	if cache.Rerecord != outerRRWarm+1 {
		t.Fatalf("lifetime Rerecord=%d want warm(%d)+1 (leaf only; outer/mid never re-store)", cache.Rerecord, outerRRWarm)
	}

	dc3 := render.NewContext(W, H)
	dc3.BeginFrame()
	dc3.ClearWithColor(render.White)
	pc3 := rendering.NewPaintContext(dc3, 1)
	pc3.BoundaryCache, pc3.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc3)
	if cache.FrameRerecord != 0 {
		t.Fatalf("clean frame rerecord=%d want 0", cache.FrameRerecord)
	}

	img := dc3.Image()
	if img == nil {
		t.Fatal("nil image")
	}
	lr, lg, lb, _ := img.At(70, 70).RGBA()
	if lg < 0xA000 || lr > 0x5000 {
		t.Fatalf("leaf (70,70)=#%04x%04x%04x want green (leaf own cache, not stale outer bake)", lr, lg, lb)
	}
	mr, mg, mb, _ := img.At(40, 40).RGBA()
	if mg < 0x6000 || mr > 0x6000 {
		t.Fatalf("mid (40,40)=#%04x%04x%04x want mid bg", mr, mg, mb)
	}
}

func TestBoundaryCache_NestedOuterNoRerecordWhenInnerDirty(t *testing.T) {
	// Plan criterion: only-inner-dirty must NOT grow outer lifetime Rerecord.
	// Own-content Pictures + walk nested RB after tryReplay.
	outer := complexBoundary(120, 120, complexFills)
	outer.Background = &rendering.Color{R: 0.2, G: 0.2, B: 0.25, A: 1}

	staticLeaf := complexBoundary(20, 20, complexFills)
	hotLeaf := complexBoundary(20, 20, complexFills)
	outer.Place(staticLeaf, 8, 8)
	outer.Place(hotLeaf, 60, 60)

	root := rendering.NewAbsoluteBox(200, 200)
	root.Place(outer, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()

	warmBoundaryCache(t, root, cache, 200, 200)
	if !cache.HasValid(outer) || !cache.HasValid(staticLeaf) || !cache.HasValid(hotLeaf) {
		t.Fatal("warm must cache outer, static leaf, and hot leaf")
	}
	outerRRAfterWarm := cache.Rerecord

	hotLeaf.Background.R = 0.5
	hotLeaf.MarkNeedsPaint()
	if outer.NeedsPaint() {
		t.Fatal("outer must not be paint-dirty when only an inner boundary is marked")
	}
	if staticLeaf.NeedsPaint() {
		t.Fatal("static leaf must stay clean")
	}

	rr1, sk1 := paintWithCache(t, root, cache, 200, 200)
	if rr1 != 0 {
		t.Fatalf("inner dirty frame rerecord=%d want 0 (changed content is not cached)", rr1)
	}
	// Outer own Replay (skip) + static leaf skip while the hot leaf repaints.
	if sk1 < 2 {
		t.Fatalf("skip=%d want ≥2 (outer own + static leaf)", sk1)
	}
	if grew := cache.Rerecord - outerRRAfterWarm; grew != 0 {
		t.Fatalf("lifetime Rerecord grew by %d want 0 (outer must NOT re-store)", grew)
	}
	if !cache.HasValid(outer) {
		t.Fatal("outer cache must stay valid across inner-only dirty")
	}

	// Hot leaf settles: only it re-enters, outer/static never re-store.
	for i := 0; i < 4; i++ {
		paintWithCache(t, root, cache, 200, 200)
	}
	if grew := cache.Rerecord - outerRRAfterWarm; grew != 1 {
		t.Fatalf("lifetime Rerecord grew by %d want exactly 1 (hot leaf only)", grew)
	}
	rr2, sk2 := paintWithCache(t, root, cache, 200, 200)
	if rr2 != 0 {
		t.Fatalf("clean frame rerecord=%d want 0", rr2)
	}
	if sk2 < 3 {
		t.Fatalf("clean frame skip=%d want ≥3", sk2)
	}
}

func TestCountRepaintBoundaries_NestedDepth(t *testing.T) {
	// root (no) → outer (yes) → mid (yes) → leaf (yes) depth 3
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
	box := complexBoundary(20, 20, complexFills)
	root := newPolicyTree(100, 100, box)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	cache := owner.BoundaryCache()

	warmBoundaryCache(t, root, cache, 100, 100)
	if !cache.HasValid(box) {
		t.Fatal("expected valid cache after warm-up")
	}

	_, willChangeBefore, _, _, _ := cache.PolicyCounts()

	// Resize: layout marks the resized node paint dirty (Base.setSize →
	// MarkNeedsPaint), so the frame is will_change — it live-paints, cannot
	// re-store, and drops the previous-generation entry, leaving nothing stale
	// to replay at the old size.
	box.FixedWidth, box.FixedHeight = 40, 40
	box.MarkNeedsLayout()
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	if !box.NeedsPaint() {
		t.Fatal("resize must mark the boundary paint dirty (the will_change signal)")
	}
	rr, sk := paintWithCache(t, root, cache, 100, 100)
	if sk != 0 {
		t.Fatalf("after size change skip=%d want 0", sk)
	}
	if rr != 0 {
		t.Fatalf("after size change rerecord=%d want 0 (changed content held back)", rr)
	}
	if cache.HasValid(box) {
		t.Fatal("the changed boundary's entry must be dropped")
	}
	if _, wc, _, _, _ := cache.PolicyCounts(); wc != willChangeBefore+1 {
		t.Fatalf("will_change_rejects=%d want %d", wc, willChangeBefore+1)
	}
	// Settle: re-admission needs accessThreshold+1 unchanged live paints.
	settle(t, root, cache, 100, 100, cache.AccessThreshold()+1)
	if !cache.HasValid(box) {
		t.Fatal("resized boundary must be re-cached once settled")
	}
	if _, sk := paintWithCache(t, root, cache, 100, 100); sk != 1 {
		t.Fatalf("resized steady skip=%d want 1", sk)
	}

	// Same cycle again: the drop is per content generation, not a one-shot.
	box.FixedWidth, box.FixedHeight = 60, 60
	box.MarkNeedsLayout()
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	if _, sk := paintWithCache(t, root, cache, 100, 100); sk != 0 {
		t.Fatalf("second resize skip=%d want 0", sk)
	}
	if cache.HasValid(box) {
		t.Fatal("the re-resized boundary's entry must be dropped")
	}
	settle(t, root, cache, 100, 100, cache.AccessThreshold()+1)
	if !cache.HasValid(box) {
		t.Fatal("re-resized boundary must be re-cached once settled")
	}
	if _, sk := paintWithCache(t, root, cache, 100, 100); sk != 1 {
		t.Fatalf("re-resized steady skip=%d want 1", sk)
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

// TestBoundaryCache_NonCacheableNeverReplays: a boundary whose own content
// contains RO types the MVP recorder cannot capture (here a Viewport) must not
// be cached — tryReplay must always miss so live content is never dropped from
// a stale-frame Replay. The Viewport's parent owns no own content (every draw
// lives in the nested boundary), so it scores 0 and the complexity gate keeps
// it out as well — no entry exists for either.
func TestBoundaryCache_NonCacheableNeverReplays(t *testing.T) {
	inner := rendering.NewRenderColorBox(10, 10, 1, 0, 0, 1)
	vp := rendering.NewRenderViewport(inner)
	outer := rendering.NewAbsoluteBox(50, 50)
	outer.SetRepaintBoundary(true)
	outer.Place(vp, 5, 5)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(outer, 0, 0)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	cache := owner.BoundaryCache()

	for i := 0; i < 6; i++ {
		rr, sk := paintWithCache(t, root, cache, 100, 100)
		if sk != 0 {
			t.Fatalf("frame %d: non-cacheable boundary skip=%d want 0 (always live)", i, sk)
		}
		if rr != 0 {
			t.Fatalf("frame %d: non-cacheable boundary rerecord=%d want 0 (nothing cached)", i, rr)
		}
	}
	if cache.HasValid(vp) {
		t.Fatal("viewport must never be cached (would replay stale)")
	}
	if cache.HasValid(outer) {
		t.Fatal("parent of a viewport owns no cacheable own content")
	}
	if got := cache.Len(); got != 0 {
		t.Fatalf("entries=%d want 0", got)
	}
	_, willChange, complexity, _, maxScore := cache.PolicyCounts()
	if maxScore != 0 {
		t.Fatalf("max_score=%d want 0 (no cacheable own content anywhere)", maxScore)
	}
	if willChange != 1 || complexity < 5 {
		t.Fatalf("gate must keep the empty-own-content parent out: will_change=%d complexity=%d",
			willChange, complexity)
	}
}

// TestBoundaryCache_DPRInvalidateReRecords: R11 — programmatic full
// invalidation (DPR change path, PipelineApp.InvalidateBoundaryCache → Clear)
// must drop all entries and restart the access count: the next frames live-paint
// (no skip), then steady frames Replay again.
func TestBoundaryCache_DPRInvalidateReRecords(t *testing.T) {
	box := complexBoundary(20, 20, complexFills)
	root := newPolicyTree(100, 100, box)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	cache := owner.BoundaryCache()

	warmBoundaryCache(t, root, cache, 100, 100)
	if _, sk := paintWithCache(t, root, cache, 100, 100); sk != 1 {
		t.Fatalf("steady skip=%d want 1 before invalidation", sk)
	}

	// DPR change → Clear() (InvalidateBoundaryCache path): one wave re-record.
	cache.Clear()
	if cache.HasValid(box) {
		t.Fatal("Clear must drop entries")
	}
	rr, sk := paintWithCache(t, root, cache, 100, 100)
	if sk != 0 {
		t.Fatalf("invalidation frame skip=%d want 0", sk)
	}
	if rr != 0 {
		t.Fatalf("invalidation frame rerecord=%d want 0 (access count restarts)", rr)
	}

	// Back to steady: the boundary is re-admitted after the access threshold.
	for i := 0; i < 2; i++ {
		paintWithCache(t, root, cache, 100, 100)
	}
	if rr, _ := paintWithCache(t, root, cache, 100, 100); rr != 1 {
		t.Fatalf("re-admit frame rerecord=%d want 1", rr)
	}
	if _, sk2 := paintWithCache(t, root, cache, 100, 100); sk2 != 1 {
		t.Fatalf("post-invalidation steady skip=%d want 1", sk2)
	}
}

func TestCompositingBits_IncrementalFlush(t *testing.T) {
	// root → outer(boundary) → mid(boundary) → leaf(boundary): depth 3, count 3
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

	owner := rendering.NewPipelineOwner(root)

	// First flush: everything fresh, bits must propagate (R3b boundary_count).
	count, depth := rendering.CountRepaintBoundaries(root)
	if count != 3 || depth != 3 {
		t.Fatalf("boundary_count=%d depth=%d want 3/3", count, depth)
	}
	owner.UpdateCompositingBits()
	if !leaf.NeedsCompositing() || !mid.NeedsCompositing() || !outer.NeedsCompositing() {
		t.Fatal("all boundaries must need compositing after first flush")
	}
	if !root.NeedsCompositing() {
		t.Fatal("root must need compositing (nested boundaries)")
	}

	// Second flush with no changes: clean, no recompute needed (bits stable).
	if root.NeedsCompositingBitsUpdate() || outer.NeedsCompositingBitsUpdate() {
		t.Fatal("no structural change: compositing bits must be clean")
	}
	owner.UpdateCompositingBits()
	if !outer.NeedsCompositing() {
		t.Fatal("clean flush must not clear bits")
	}

	// New child added to mid: MarkNeedsCompositingBitsUpdate dirties chain to root.
	extra := rendering.NewRenderColorBox(6, 6, 0, 0, 1, 1)
	extra.SetRepaintBoundary(true)
	mid.AddChild(extra)
	if !mid.NeedsCompositingBitsUpdate() || !outer.NeedsCompositingBitsUpdate() || !root.NeedsCompositingBitsUpdate() {
		t.Fatal("child add must mark compositing bits dirty up the chain")
	}
	count, _ = rendering.CountRepaintBoundaries(root)
	if count != 4 {
		t.Fatalf("boundary_count after add=%d want 4", count)
	}
	owner.UpdateCompositingBits()
	if !extra.NeedsCompositing() {
		t.Fatal("new boundary must need compositing after flush")
	}
	if root.NeedsCompositingBitsUpdate() {
		t.Fatal("flush must clear dirty markers")
	}
}

// TestBoundaryCache_ShellBodyPartition: shell-tagged boundaries count in the
// shell bucket separately from body boundaries.
// The body's content change must never re-record the shell; the shell Replays
// (shell skip +1) with shell rerecord staying 0.
func TestBoundaryCache_ShellBodyPartition(t *testing.T) {
	shellB := complexBoundary(400, 50, complexFills)
	shellB.SetShellBoundary(true)
	bodyB := complexBoundary(400, 300, complexFills)

	root := rendering.NewAbsoluteBox(600, 400)
	root.Place(shellB, 0, 0)
	root.Place(bodyB, 0, 60)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()

	// Warm: both admitted once their content has been stable long enough.
	warmBoundaryCache(t, root, cache, 200, 400)
	if cache.ShellRerecord != 1 {
		t.Fatalf("lifetime shell_rerecord=%d want 1 (cold shell record only)", cache.ShellRerecord)
	}
	if !cache.HasValid(shellB) || !cache.HasValid(bodyB) {
		t.Fatal("both boundaries must be cached after warm-up")
	}

	// Body scrolls (its content changes) → shell Replays, body live-paints.
	bodyB.Background.R, bodyB.Background.G = 0.1, 0.3
	bodyB.MarkNeedsPaint()
	if shellB.NeedsPaint() {
		t.Fatal("shell must stay clean when the body (sibling boundary) dirties")
	}
	paintWithCache(t, root, cache, 200, 400)
	srr, ssk := cache.ShellFrameCounts()
	if srr != 0 {
		t.Fatalf("shell_rerecord=%d want 0 (body scroll must not re-record shell)", srr)
	}
	if ssk != 1 {
		t.Fatalf("shell_skip=%d want 1 (shell Picture replays)", ssk)
	}
	if cache.FrameRerecord != 0 {
		t.Fatalf("body rerecord=%d want 0 (changed body content is held back)", cache.FrameRerecord)
	}
	// Global counters still include shell.
	if cache.ShellRerecord != 1 {
		t.Fatalf("lifetime shell_rerecord=%d want 1", cache.ShellRerecord)
	}

	// The body settles and re-enters; the shell keeps replaying throughout.
	for i := 0; i < 4; i++ {
		paintWithCache(t, root, cache, 200, 400)
	}
	if !cache.HasValid(bodyB) {
		t.Fatal("body must be re-cached once its content settles")
	}
	_, ssk = cache.ShellFrameCounts()
	if ssk != 1 {
		t.Fatalf("shell_skip after body settle=%d want 1", ssk)
	}
	if cache.ShellRerecord != 1 {
		t.Fatalf("lifetime shell_rerecord=%d want 1 (shell never re-recorded)", cache.ShellRerecord)
	}
}

// TestBase_SetShellBoundary_Toggle: the tag survives toggling and reports false
// by default; re-tagging is idempotent.
func TestBase_SetShellBoundary_Toggle(t *testing.T) {
	b := rendering.NewRenderColorBox(10, 10, 1, 0, 0, 1)
	if b.IsShellBoundary() {
		t.Fatal("untagged boundary must not be shell")
	}
	b.SetShellBoundary(true)
	if !b.IsShellBoundary() {
		t.Fatal("SetShellBoundary(true) must tag the boundary")
	}
	b.SetShellBoundary(true)
	if !b.IsShellBoundary() {
		t.Fatal("re-tagging must stay applied")
	}
	b.SetShellBoundary(false)
	if b.IsShellBoundary() {
		t.Fatal("SetShellBoundary(false) must clear the tag")
	}
}
