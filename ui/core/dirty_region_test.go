package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestLayoutEarlyOut_PaintOnlyDirty(t *testing.T) {
	child := primitive.NewBox()
	child.Width, child.Height = 40, 20
	root := primitive.NewBox(child)
	root.Width, root.Height = 100, 80
	tree := core.NewTree(root)
	vp := core.Size{Width: 100, Height: 80}
	tree.Layout(vp)
	if root.NeedsLayout() || child.NeedsLayout() {
		t.Fatal("expected layout clean after first pass")
	}
	// Paint-only dirty must not force relayout.
	child.MarkNeedsPaint()
	if child.NeedsLayout() {
		t.Fatal("MarkNeedsPaint must not set needsLayout")
	}
	tree.Layout(vp)
	if root.NeedsLayout() {
		t.Fatal("paint-only dirty should keep layout early-out")
	}
}

func TestRepaintBoundaryStopsPaintBubble(t *testing.T) {
	inner := primitive.NewBox()
	inner.Width, inner.Height = 10, 10
	boundary := primitive.NewRepaintBoundary(inner)
	outer := primitive.NewBox(boundary)
	outer.Width, outer.Height = 50, 50
	tree := core.NewTree(outer)
	tree.Layout(core.Size{Width: 50, Height: 50})
	// Clear paint dirty from mount/layout.
	tree.Frame(&core.PaintContext{}, core.Size{Width: 50, Height: 50})
	if tree.Dirty() {
		t.Fatal("clean after frame")
	}
	inner.MarkNeedsPaint()
	if !inner.NeedsPaint() {
		t.Fatal("inner should be paint dirty")
	}
	if !boundary.NeedsPaint() {
		t.Fatal("boundary should be paint dirty")
	}
	if outer.NeedsPaint() {
		t.Fatal("paint dirty must stop at RepaintBoundary (outer clean)")
	}
	if !tree.Dirty() {
		t.Fatal("tree must still schedule a frame")
	}
}

func TestCollectPaintDamage(t *testing.T) {
	inner := primitive.NewBox()
	inner.Width, inner.Height = 20, 12
	boundary := primitive.NewRepaintBoundary(inner)
	root := primitive.NewBox(boundary)
	root.Width, root.Height = 200, 100
	tree := core.NewTree(root)
	tree.Frame(&core.PaintContext{}, core.Size{Width: 200, Height: 100})
	// After full paint, FullPaintRequired is false.
	if tree.FullPaintRequired() {
		t.Fatal("full paint should clear after first paint")
	}
	inner.MarkNeedsPaint()
	rects := tree.CollectPaintDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects for dirty nodes")
	}
}

func TestProgressSetPercentStableRoot(t *testing.T) {
	// living in kit package — use primitive only here; kit test covers progress.
}

func TestCompositeOnlySkipsCleanLeaves(t *testing.T) {
	inner := primitive.NewBox()
	inner.Width, inner.Height = 20, 12
	boundary := primitive.NewRepaintBoundary(inner)
	static := primitive.NewBox()
	static.Width, static.Height = 40, 20
	root := primitive.NewBox(boundary, static)
	root.Width, root.Height = 200, 100
	tree := core.NewTree(root)
	vp := core.Size{Width: 200, Height: 100}
	// First full paint.
	tree.Frame(&core.PaintContext{}, vp)
	if tree.FullPaintRequired() {
		t.Fatal("full paint should clear")
	}
	// Dirty only the animated branch.
	inner.MarkNeedsPaint()
	if static.NeedsPaint() {
		t.Fatal("static must stay clean")
	}
	// Composite-only frame: clean static leaf must not need paint flag.
	pc := &core.PaintContext{CompositeOnly: true}
	tree.Frame(pc, vp)
	if tree.Dirty() {
		t.Fatal("frame should clear tree dirty")
	}
	if static.NeedsPaint() {
		t.Fatal("static should remain paint-clean after composite-only frame")
	}
}
