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
