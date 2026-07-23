package primitive_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestAnchoredPopupFlipBottomToTop(t *testing.T) {
	panel := primitive.NewBox()
	panel.Width, panel.Height = 80, 60
	panel.Color = render.RGBA{A: 1}
	pop := primitive.NewAnchoredPopup(panel)
	pop.Placement = primitive.PlaceBottom
	pop.Gap = 4
	// Anchor near bottom of 100×100 viewport → bottom placement overflows → flip up.
	pop.Anchor = core.Rect{Min: core.Point{X: 10, Y: 70}, Max: core.Point{X: 50, Y: 90}}
	pop.Viewport = core.Size{Width: 100, Height: 100}
	pop.SetOpen(true)
	// Force measure + reposition
	_ = panel.Layout(core.Loose(400, 400))
	// Call Layout on pop via tree
	root := primitive.NewBox(pop)
	root.Width, root.Height = 100, 100
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 100, Height: 100})
	off := panel.Base().Offset()
	// Flipped above anchor: y ≈ 70-4-60 = 6
	if off.Y > 30 {
		t.Fatalf("expected flip above anchor, offset=%v", off)
	}
	if off.Y+60 > 100+0.5 {
		t.Fatalf("panel still overflows viewport: offset=%v", off)
	}
}

func TestAnchoredPopupViewportFromTree(t *testing.T) {
	panel := primitive.NewBox()
	panel.Width, panel.Height = 40, 40
	pop := primitive.NewAnchoredPopup(panel)
	pop.Placement = primitive.PlaceBottom
	// No explicit Viewport — should use Tree.Viewport after layout.
	pop.Anchor = core.Rect{Min: core.Point{X: 0, Y: 0}, Max: core.Point{X: 20, Y: 20}}
	pop.SetOpen(true)
	root := primitive.NewBox(pop)
	root.Width, root.Height = 50, 50
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 50, Height: 50})
	// Panel at bottom of anchor would be y=24, height 40 → overflows 50 → clamp/flip
	off := panel.Base().Offset()
	if off.Y+40 > 50.5 {
		t.Fatalf("viewport from tree not applied: offset=%v size 40 in 50", off)
	}
}

func TestOutsideDismissClosesPopup(t *testing.T) {
	panel := primitive.NewBox()
	panel.Width, panel.Height = 40, 40
	panel.Hit = core.HitBlock
	anchor := primitive.NewBox()
	anchor.Width, anchor.Height = 30, 20
	anchor.Hit = core.HitBlock

	pop := primitive.NewAnchoredPopup(panel)
	pop.DismissOnOutside = true
	dismissed := false
	pop.OnDismiss = func() { dismissed = true }
	pop.UpdateAnchorFromNode(anchor)

	col := primitive.Column(anchor, pop)
	root := primitive.NewBox(col)
	root.Width, root.Height = 200, 200
	// Add a blank area sibling for outside hit — root itself is large.
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 200, Height: 200})
	pop.SetOpen(true)
	// Layout again so outside dismiss registers after open.
	tree.Layout(core.Size{Width: 200, Height: 200})

	if !pop.Open {
		t.Fatal("want open")
	}

	// Pointer down far from anchor/content.
	tree.DispatchPointer(&core.PointerEvent{
		Type: core.PointerDown,
		X:    180, Y: 180,
	})
	if pop.Open {
		t.Fatal("outside pointer should dismiss popup")
	}
	if !dismissed {
		t.Fatal("OnDismiss not called")
	}
}

func TestOutsideDismissKeepsAnchorClick(t *testing.T) {
	panel := primitive.NewBox()
	panel.Width, panel.Height = 40, 40
	panel.Hit = core.HitBlock
	anchor := primitive.NewBox()
	anchor.Width, anchor.Height = 80, 40
	anchor.Hit = core.HitBlock

	pop := primitive.NewAnchoredPopup(panel)
	pop.DismissOnOutside = true
	pop.UpdateAnchorFromNode(anchor)

	root := primitive.NewBox(primitive.Column(anchor, pop))
	root.Width, root.Height = 200, 200
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 200, Height: 200})
	pop.SetOpen(true)
	tree.Layout(core.Size{Width: 200, Height: 200})

	abs := core.AbsoluteBounds(anchor)
	cx := (abs.Min.X + abs.Max.X) / 2
	cy := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: cx, Y: cy})
	if !pop.Open {
		t.Fatal("click on anchor must not outside-dismiss")
	}
}

func TestTriggerHoverDelay(t *testing.T) {
	child := primitive.NewBox()
	child.Width, child.Height = 20, 20
	tr := primitive.NewTrigger(child)
	tr.Mode = primitive.TriggerHover
	tr.DelayMs = 100
	opened := 0
	tr.OnOpenChange = func(open bool) {
		if open {
			opened++
		}
	}
	root := primitive.NewBox(tr)
	root.Width, root.Height = 100, 100
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 100, Height: 100})

	tr.SetHovered(true)
	if tr.Open {
		t.Fatal("must not open immediately when DelayMs>0")
	}
	if !tree.HasActiveTickers() {
		t.Fatal("delay should register ticker")
	}
	// 50ms — still waiting
	tree.TickActive(0.05)
	if tr.Open {
		t.Fatal("opened too early")
	}
	// another 60ms → past 100ms
	tree.TickActive(0.06)
	if !tr.Open || opened != 1 {
		t.Fatalf("open=%v opened=%d", tr.Open, opened)
	}
	// leave closes
	tr.SetHovered(false)
	if tr.Open {
		t.Fatal("leave should close")
	}
}

func TestTriggerImmediateWhenNoDelay(t *testing.T) {
	child := primitive.NewBox()
	child.Width, child.Height = 10, 10
	tr := primitive.NewTrigger(child)
	tr.Mode = primitive.TriggerHover
	tr.DelayMs = 0
	tr.SetHovered(true)
	if !tr.Open {
		t.Fatal("DelayMs=0 must open immediately")
	}
}
