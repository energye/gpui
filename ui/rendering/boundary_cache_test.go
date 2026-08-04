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

// TestBoundaryCache_NestedAbsoluteBoxCleanReplayKeepsMidContent: outer own-content
// Replay + walk nested RB children so mid/leaf still paint (mid not baked in outer).
func TestBoundaryCache_NestedAbsoluteBoxCleanReplayKeepsMidContent(t *testing.T) {
	const W, H = 200, 200
	outer := rendering.NewAbsoluteBox(160, 160)
	outer.Background = &rendering.Color{R: 0.1, G: 0.1, B: 0.5, A: 1} // blue outer
	outer.SetRepaintBoundary(true)

	mid := rendering.NewAbsoluteBox(100, 100)
	mid.Background = &rendering.Color{R: 0.0, G: 0.6, B: 0.6, A: 1} // teal mid
	mid.SetRepaintBoundary(true)

	leaf := rendering.NewRenderColorBox(40, 40, 0.0, 0.9, 0.1, 1) // bright green
	leaf.SetRepaintBoundary(true)
	mid.Place(leaf, 20, 20)
	outer.Place(mid, 20, 20)

	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.Place(outer, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	dc1 := render.NewContext(W, H)
	pc1 := rendering.NewPaintContext(dc1, 1)
	pc1.BoundaryCache, pc1.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc1)
	if cache.FrameRerecord < 3 {
		t.Fatalf("warm rerecord=%d want ≥3 (outer+mid+leaf own caches)", cache.FrameRerecord)
	}
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
// outer must not bake leaf. After red→green rerecord, clean Replay shows green
// without ancestor invalidate / outer rerecord.
func TestBoundaryCache_InnerChangeThenCleanReplayShowsFresh(t *testing.T) {
	const W, H = 200, 200
	outer := rendering.NewAbsoluteBox(160, 160)
	outer.Background = &rendering.Color{R: 0.1, G: 0.1, B: 0.5, A: 1}
	outer.SetRepaintBoundary(true)

	mid := rendering.NewAbsoluteBox(100, 100)
	mid.Background = &rendering.Color{R: 0.0, G: 0.5, B: 0.5, A: 1}
	mid.SetRepaintBoundary(true)

	leaf := rendering.NewRenderColorBox(40, 40, 0.95, 0.05, 0.05, 1) // red warm
	leaf.SetRepaintBoundary(true)
	mid.Place(leaf, 20, 20)
	outer.Place(mid, 20, 20)

	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.Place(outer, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()

	dc1 := render.NewContext(W, H)
	pc1 := rendering.NewPaintContext(dc1, 1)
	pc1.BoundaryCache, pc1.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc1)
	if !cache.HasValid(outer) || !cache.HasValid(leaf) {
		t.Fatal("warm must cache outer and leaf")
	}
	outerRRWarm := cache.Rerecord

	leaf.R, leaf.G, leaf.B, leaf.A = 0.05, 0.95, 0.1, 1 // green
	leaf.MarkNeedsPaint()
	if outer.NeedsPaint() {
		t.Fatal("outer must stay clean when only leaf boundary is marked")
	}
	dc2 := render.NewContext(W, H)
	pc2 := rendering.NewPaintContext(dc2, 1)
	pc2.BoundaryCache, pc2.UseBoundaryCache = cache, true
	cache.BeginFrame()
	root.Paint(pc2)
	if cache.FrameRerecord != 1 {
		t.Fatalf("leaf dirty frame rerecord=%d want exactly 1 (hot only; outer/mid must not re-store)", cache.FrameRerecord)
	}
	if cache.Rerecord != outerRRWarm+1 {
		t.Fatalf("lifetime Rerecord=%d want warm(%d)+1", cache.Rerecord, outerRRWarm)
	}
	if !cache.HasValid(outer) {
		t.Fatal("outer own-content cache must remain valid (not invalidated by leaf store)")
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

	rr0, _ := paintWithCache(t, root, cache, 200, 200)
	if rr0 < 3 {
		t.Fatalf("warm rerecord=%d want ≥3 (outer+static+hot)", rr0)
	}
	if !cache.HasValid(outer) {
		t.Fatal("outer boundary must be cached after warm")
	}
	outerRRAfterWarm := cache.Rerecord

	hotLeaf.R = 0.5
	hotLeaf.MarkNeedsPaint()
	if outer.NeedsPaint() {
		t.Fatal("outer must not be paint-dirty when only inner boundary is marked")
	}
	if staticLeaf.NeedsPaint() {
		t.Fatal("static leaf must stay clean")
	}

	rr1, sk1 := paintWithCache(t, root, cache, 200, 200)
	if rr1 != 1 {
		t.Fatalf("inner dirty frame rerecord=%d want exactly 1 (hot only)", rr1)
	}
	// Outer own Replay (skip) + static leaf skip while hot rerecords.
	if sk1 < 2 {
		t.Fatalf("skip=%d want ≥2 (outer own + static leaf)", sk1)
	}
	grew := cache.Rerecord - outerRRAfterWarm
	if grew != 1 {
		t.Fatalf("lifetime Rerecord grew by %d want exactly 1 (outer must NOT re-store)", grew)
	}
	if !cache.HasValid(outer) {
		t.Fatal("outer cache must stay valid across inner-only dirty")
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

// TestBoundaryCache_NonCacheableNeverReplays: a boundary whose own content
// contains RO types the MVP recorder cannot capture (here a Viewport) must not
// be cached — tryReplay must always miss so live content is never dropped from
// a stale-frame Replay (correctness-first R3 rewrite).
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

	_, _ = paintWithCache(t, root, cache, 100, 100)
	if cache.HasValid(outer) {
		t.Fatal("viewport-containing boundary must never be cached (would replay stale)")
	}
	// Every subsequent frame must still live-paint (no cache Replay ever).
	rr, sk := paintWithCache(t, root, cache, 100, 100)
	if sk != 0 {
		t.Fatalf("non-cacheable boundary skip=%d want 0 (always live)", sk)
	}
	if rr != 0 {
		t.Fatalf("non-cacheable boundary rerecord=%d want 0 (nothing cached)", rr)
	}
}

// TestBoundaryCache_DPRInvalidateReRecords: R11 — programmatic full
// invalidation (DPR change path, PipelineApp.InvalidateBoundaryCache → Clear)
// must drop all entries: next frame re-records (no skip), then steady frames
// replay again (skip resumes). Mirrors TestBoundaryCache_SizeChangeInvalidates
// for the Clear path.
func TestBoundaryCache_DPRInvalidateReRecords(t *testing.T) {
	box := rendering.NewRenderColorBox(20, 20, 0.3, 0.8, 0.2, 1)
	box.SetRepaintBoundary(true)
	root := rendering.NewAbsoluteBox(100, 100)
	root.Place(box, 0, 0)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 100, Height: 100}, true)
	cache := owner.BoundaryCache()

	// Frame 1 records; steady skip accumulates from frame 2.
	if rr, _ := paintWithCache(t, root, cache, 100, 100); rr < 1 {
		t.Fatalf("initial frame rerecord=%d want ≥1", rr)
	}
	if _, sk := paintWithCache(t, root, cache, 100, 100); sk < 1 {
		t.Fatalf("steady skip=%d want ≥1 before invalidation", sk)
	}
	if !cache.HasValid(box) {
		t.Fatal("expected valid cache before invalidation")
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
	if rr < 1 {
		t.Fatalf("invalidation frame rerecord=%d want ≥1 (one wave)", rr)
	}

	// Back to steady: replay resumes.
	_, sk2 := paintWithCache(t, root, cache, 100, 100)
	if sk2 < 1 {
		t.Fatalf("post-invalidation steady skip=%d want ≥1", sk2)
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
// shell bucket separately from body boundaries (R21 shell/content layering).
// Frame 1 cold-records both; frame 2 dirties ONLY the body — the shell boundary
// Replays (shell skip +1) with shell rerecord staying 0.
func TestBoundaryCache_ShellBodyPartition(t *testing.T) {
	shellB := rendering.NewRenderColorBox(400, 50, 0.2, 0.4, 0.9, 1)
	shellB.SetRepaintBoundary(true)
	shellB.SetShellBoundary(true)
	bodyB := rendering.NewRenderColorBox(400, 300, 0.9, 0.6, 0.2, 1)
	bodyB.SetRepaintBoundary(true)

	root := rendering.NewAbsoluteBox(600, 400)
	root.Place(shellB, 0, 0)
	root.Place(bodyB, 0, 60)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()

	// Frame 1: both dirty → 2 rerecords, 1 of them shell.
	paintWithCache(t, root, cache, 200, 400)
	srr, ssk := cache.ShellFrameCounts()
	if srr != 1 {
		t.Fatalf("frame1 shell_rerecord=%d want 1", srr)
	}
	if ssk != 0 {
		t.Fatalf("frame1 shell_skip=%d want 0", ssk)
	}
	if cache.ShellRerecord != 1 || cache.ShellSkip != 0 {
		t.Fatalf("lifetime shell rr/sk=%d/%d want 1/0", cache.ShellRerecord, cache.ShellSkip)
	}

	// Frame 2: body scrolls (changes color) → body rerecords, shell Replays.
	bodyB.R, bodyB.G = 0.1, 0.3
	bodyB.MarkNeedsPaint()
	if shellB.NeedsPaint() {
		t.Fatal("shell must stay clean when the body (sibling boundary) dirties")
	}
	paintWithCache(t, root, cache, 200, 400)
	srr, ssk = cache.ShellFrameCounts()
	if srr != 0 {
		t.Fatalf("frame2 shell_rerecord=%d want 0 (body scroll must not re-record shell)", srr)
	}
	if ssk != 1 {
		t.Fatalf("frame2 shell_skip=%d want 1 (shell Picture replays)", ssk)
	}
	if cache.FrameRerecord < 1 {
		t.Fatalf("frame2 body rerecord=%d want ≥1", cache.FrameRerecord)
	}
	// Global counters still include shell (backward compat with R3/R4b gates).
	if cache.ShellRerecord != 1 {
		t.Fatalf("lifetime shell_rerecord=%d want 1", cache.ShellRerecord)
	}
}

// TestBase_SetShellBoundary_Toggle: the tag survives toggling and reports false
// by default; re-tagging is idempotent (R21).
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
