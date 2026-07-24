package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// PaintOrder: higher sibling paints above and is hit first (hit == paint z-order).
// Covers overflow chrome (e.g. splitter bars over next panel) without TypeID hacks.
func TestPaintOrder_HitAndPaintStack(t *testing.T) {
	root := primitive.NewBox()
	root.Hit = core.HitDefer
	root.Width, root.Height = 100, 40

	// two overlapping full-size children; later in slice would normally be top.
	a := primitive.NewBox()
	a.Width, a.Height = 100, 40
	a.Hit = core.HitTarget
	b := primitive.NewBox()
	b.Width, b.Height = 100, 40
	b.Hit = core.HitTarget
	// Raise a above b via PaintOrder even though a is first in children.
	a.PaintOrder = 1
	b.PaintOrder = 0

	root.AddChild(a)
	root.AddChild(b)
	_ = root.Layout(core.Tight(100, 40))
	a.SetOffset(core.Point{})
	b.SetOffset(core.Point{})

	hit := root.HitTest(core.Point{X: 50, Y: 20})
	if hit != a {
		t.Fatalf("hit=%v want a (PaintOrder 1 above b)", hit)
	}

	// All-zero PaintOrder keeps historical reverse-index hit (last child wins).
	root2 := primitive.NewBox()
	root2.Hit = core.HitDefer
	c1 := primitive.NewBox()
	c1.Width, c1.Height = 100, 40
	c1.Hit = core.HitTarget
	c2 := primitive.NewBox()
	c2.Width, c2.Height = 100, 40
	c2.Hit = core.HitTarget
	root2.AddChild(c1)
	root2.AddChild(c2)
	_ = root2.Layout(core.Tight(100, 40))
	c1.SetOffset(core.Point{})
	c2.SetOffset(core.Point{})
	hit2 := root2.HitTest(core.Point{X: 10, Y: 10})
	if hit2 != c2 {
		t.Fatalf("all-zero PaintOrder hit=%v want last child c2", hit2)
	}
}
