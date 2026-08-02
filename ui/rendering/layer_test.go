package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

func TestRepaintBoundary_StopsPaintBubble(t *testing.T) {
	leaf := rendering.NewRenderColorBox(10, 10, 1, 0, 0, 1)
	leaf.SetRepaintBoundary(true)
	root := rendering.NewRenderBox(leaf)
	root.FixedWidth, root.FixedHeight = 100, 100
	_ = rendering.NewPipelineOwner(root)
	root.Layout(rendering.Tight(100, 100))
	// Clear paint dirty from layout marks.
	leaf.Paint(&rendering.PaintContext{})
	root.Paint(&rendering.PaintContext{})

	leaf.MarkNeedsPaint()
	if !leaf.NeedsPaint() {
		t.Fatal("leaf should be paint dirty")
	}
	if root.NeedsPaint() {
		t.Fatal("root must not be paint dirty when leaf is repaint boundary")
	}
}

// TestMarkNeedsPaint_NestedBoundaryIsolation (R2 strict Flutter semantics):
// dirtying a node inside a nested RepaintBoundary dirties only up to that
// boundary — neither the outer boundary nor the root may become dirty, and
// idempotent re-marking must not extend the dirty chain.
func TestMarkNeedsPaint_NestedBoundaryIsolation(t *testing.T) {
	inner := rendering.NewRenderColorBox(4, 4, 1, 0, 0, 1)
	inner.SetRepaintBoundary(true)
	outer := rendering.NewRenderBox(inner)
	outer.FixedWidth, outer.FixedHeight = 20, 20
	outer.SetRepaintBoundary(true)
	root := rendering.NewRenderBox(outer)
	root.FixedWidth, root.FixedHeight = 100, 100
	_ = rendering.NewPipelineOwner(root)
	root.Layout(rendering.Tight(100, 100))
	root.Paint(&rendering.PaintContext{})
	outer.Paint(&rendering.PaintContext{})
	inner.Paint(&rendering.PaintContext{})

	inner.MarkNeedsPaint()
	if !inner.NeedsPaint() {
		t.Fatal("inner should be paint dirty")
	}
	if outer.NeedsPaint() {
		t.Fatal("outer boundary must not be dirtied by inner child paint")
	}
	if root.NeedsPaint() {
		t.Fatal("root must not be dirtied through outer boundary")
	}
}

// TestCompositeOnly_OnlyDirtyPathVisits (R2): with several isolated
// boundaries, dirtying one must produce visits proportional to that path —
// static siblings stay at NeedsPaint()==false and are never visited.
func TestCompositeOnly_OnlyDirtyPathVisits(t *testing.T) {
	const n = 50
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = 800, 400
	var hot *rendering.RenderColorBox
	for i := 0; i < n; i++ {
		c := rendering.NewRenderColorBox(8, 8, 0.5, 0.5, 0.5, 1)
		c.SetRepaintBoundary(true)
		root.AddChild(c)
		if i == 7 {
			hot = c
		}
	}
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 800, Height: 400}, true)
	var visits int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, true)

	// All static boundaries must be clean after full paint.
	for _, c := range root.Children() {
		if c.NeedsPaint() {
			t.Fatal("static boundary must be clean after full paint")
		}
	}
	hot.MarkNeedsPaint()
	visits = 0
	if !owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false) {
		t.Fatal("expected partial paint")
	}
	// Root + hot path only; 48 static siblings skipped.
	if visits > 5 {
		t.Fatalf("visits=%d want ≤5 (root+hot path)", visits)
	}
	if hot.NeedsPaint() {
		t.Fatal("hot boundary must be cleaned after its partial paint")
	}
}

func TestCompositeOnly_SkipsCleanLeaves(t *testing.T) {
	const n = 200 // enough to prove skip; 10k is fine too but slower in -race
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = 1000, 1000
	var spinner *rendering.RenderColorBox
	for i := 0; i < n; i++ {
		c := rendering.NewRenderColorBox(2, 2, 0.5, 0.5, 0.5, 1)
		if i == n/2 {
			c.SetRepaintBoundary(true)
			spinner = c
		}
		root.AddChild(c)
	}
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 1000, Height: 1000}, true)

	// Full paint once to clear dirty flags.
	var visits int64
	pc := &rendering.PaintContext{CompositeOnly: false, PaintVisits: &visits}
	owner.FlushPaint(pc, true)
	fullVisits := visits
	if fullVisits < int64(n) {
		t.Fatalf("full paint visits=%d want ≥%d", fullVisits, n)
	}

	// Dirty only spinner boundary.
	spinner.MarkNeedsPaint()
	visits = 0
	pc2 := &rendering.PaintContext{PaintVisits: &visits}
	// force=false enables CompositeOnly inside FlushPaint
	if !owner.FlushPaint(pc2, false) {
		t.Fatal("expected partial paint")
	}
	// Should visit root + path to spinner, not all n leaves.
	if visits >= fullVisits/2 {
		t.Fatalf("composite-only visits=%d still too high (full=%d)", visits, fullVisits)
	}
	if visits < 1 {
		t.Fatal("expected some visits")
	}
}

func TestCompositeOnly_10kBoundary(t *testing.T) {
	const n = 10000
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = 4000, 4000
	var hot *rendering.RenderColorBox
	for i := 0; i < n; i++ {
		c := rendering.NewRenderColorBox(1, 1, 0.2, 0.2, 0.2, 1)
		if i == n-1 {
			c.SetRepaintBoundary(true)
			hot = c
		}
		root.AddChild(c)
	}
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 4000, Height: 4000}, true)
	var visits int64
	pc := &rendering.PaintContext{PaintVisits: &visits}
	owner.FlushPaint(pc, true)

	hot.MarkNeedsPaint()
	visits = 0
	pc2 := &rendering.PaintContext{PaintVisits: &visits}
	owner.FlushPaint(pc2, false)
	// Root visited + hot boundary; clean siblings skipped.
	if visits > 20 {
		t.Fatalf("10k tree partial paint visits=%d want small (≪10000)", visits)
	}
}

func TestBuildLayerTree_BoundaryDirtyIDs(t *testing.T) {
	scene.ResetLayerIDGen()
	leaf := rendering.NewRenderColorBox(8, 8, 1, 0, 0, 1)
	leaf.SetRepaintBoundary(true)
	root := rendering.NewRenderBox(leaf)
	root.FixedWidth, root.FixedHeight = 64, 64
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 64, Height: 64}, true)
	// Clear paint then dirty leaf only.
	var v int64
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &v}, true)
	leaf.MarkNeedsPaint()

	b := rendering.BuildLayerTree(root)
	pkt := b.BuildPacket(1, 1, 64, 64)
	if pkt.Root == nil {
		t.Fatal("nil root")
	}
	if pkt.Overlay == nil {
		t.Fatal("overlay band must be reserved")
	}
	if len(pkt.DirtyLayerIDs) < 1 {
		t.Fatalf("expected dirty boundary ids, got %v", pkt.DirtyLayerIDs)
	}
	// COW: second packet shares root if we rebuild from same builder without structural change —
	// BuildLayerTree creates a new tree each call; ShareRoot applies to CloneShallow.
	p2 := pkt.CloneShallow()
	if !scene.ShareRoot(pkt, p2) {
		t.Fatal("clone must share root")
	}
}

func TestCompositingBits_Propagate(t *testing.T) {
	leaf := rendering.NewRenderColorBox(4, 4, 1, 1, 1, 1)
	leaf.SetAlwaysNeedsCompositing(true)
	mid := rendering.NewRenderBox(leaf)
	root := rendering.NewRenderBox(mid)
	owner := rendering.NewPipelineOwner(root)
	owner.UpdateCompositingBits()
	if !root.NeedsCompositing() {
		// NeedsCompositing is on Base — use type assert
	}
	// Access via embedding
	if !root.Base.NeedsCompositing() {
		t.Fatal("root should need compositing from descendant")
	}
	if !mid.Base.NeedsCompositing() {
		t.Fatal("mid should need compositing")
	}
	if !leaf.Base.NeedsCompositing() {
		t.Fatal("leaf alwaysNeeds")
	}
}
