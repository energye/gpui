package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// leaf is a fixed-size node for layout/hit tests.
type leaf struct {
	core.NodeBase
	w, h float64
	id   string
}

func newLeaf(id string, w, h float64) *leaf {
	n := &leaf{w: w, h: h, id: id}
	n.Init(n)
	n.Hit = core.HitTarget
	return n
}

func (n *leaf) TypeID() string { return "test." + n.id }
func (n *leaf) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: n.w, Height: n.h})
	n.SetSize(out)
	return out
}
func (n *leaf) Paint(*core.PaintContext)       {}
func (n *leaf) HitTest(p core.Point) core.Node { return n.DefaultHitTest(p) }

type flexBox struct {
	core.NodeBase
	axis core.Axis
	gap  float64
}

func newFlexBox(axis core.Axis, kids ...core.Node) *flexBox {
	f := &flexBox{axis: axis}
	f.Init(f)
	f.Hit = core.HitDefer
	for _, k := range kids {
		f.AddChild(k)
	}
	return f
}
func (f *flexBox) TypeID() string { return "test.flex" }
func (f *flexBox) Layout(c core.Constraints) core.Size {
	return core.LayoutFlex(&f.NodeBase, c, core.FlexLayoutParams{
		Axis: f.axis, Gap: f.gap, CrossAlign: core.CrossStart,
	})
}
func (f *flexBox) Paint(pc *core.PaintContext)    { f.DefaultPaintChildren(pc) }
func (f *flexBox) HitTest(p core.Point) core.Node { return f.DefaultHitTest(p) }

type clickLeaf struct {
	core.NodeBase
	clicks int
}

func newClickLeaf() *clickLeaf {
	c := &clickLeaf{}
	c.Init(c)
	c.Hit = core.HitTarget
	return c
}
func (c *clickLeaf) TypeID() string { return "test.click" }
func (c *clickLeaf) Layout(cons core.Constraints) core.Size {
	out := cons.Tighten(core.Size{Width: 80, Height: 40})
	c.SetSize(out)
	return out
}
func (c *clickLeaf) Paint(*core.PaintContext)       {}
func (c *clickLeaf) HitTest(p core.Point) core.Node { return c.DefaultHitTest(p) }
func (c *clickLeaf) HandlePointer(ev *core.PointerEvent) {
	if ev.Type == core.PointerDown {
		ev.Handled = true
	}
}
func (c *clickLeaf) OnClick(*core.PointerEvent) { c.clicks++ }

func TestConstraintsTighten(t *testing.T) {
	c := core.Constraints{MinWidth: 10, MaxWidth: 100, MinHeight: 5, MaxHeight: 50}
	s := c.Tighten(core.Size{Width: 200, Height: 1})
	if s.Width != 100 || s.Height != 5 {
		t.Fatalf("Tighten = %+v", s)
	}
}

func TestFlexRowLayout(t *testing.T) {
	a := newLeaf("a", 40, 20)
	b := newLeaf("b", 60, 30)
	row := newFlexBox(core.AxisHorizontal, a, b)
	row.gap = 10
	sz := row.Layout(core.Loose(400, 300))
	if sz.Width != 110 {
		t.Fatalf("width=%v want 110", sz.Width)
	}
	if sz.Height != 30 {
		t.Fatalf("height=%v want 30", sz.Height)
	}
	if a.Offset().X != 0 || b.Offset().X != 50 {
		t.Fatalf("offsets a=%v b=%v", a.Offset(), b.Offset())
	}
}

func TestFlexColumnLayout(t *testing.T) {
	a := newLeaf("a", 40, 20)
	b := newLeaf("b", 60, 30)
	col := newFlexBox(core.AxisVertical, a, b)
	sz := col.Layout(core.Loose(400, 300))
	if sz.Height != 50 {
		t.Fatalf("height=%v want 50", sz.Height)
	}
	if b.Offset().Y != 20 {
		t.Fatalf("b.Y=%v want 20", b.Offset().Y)
	}
}

type container struct{ core.NodeBase }

func (c *container) TypeID() string { return "test.container" }
func (c *container) Layout(cons core.Constraints) core.Size {
	out := cons.Tighten(c.Size())
	c.SetSize(out)
	return out
}
func (c *container) Paint(pc *core.PaintContext)    { c.DefaultPaintChildren(pc) }
func (c *container) HitTest(p core.Point) core.Node { return c.DefaultHitTest(p) }

func TestHitTestReverseZ(t *testing.T) {
	root := &container{}
	root.Init(root)
	root.Hit = core.HitDefer

	back := newLeaf("back", 100, 100)
	front := newLeaf("front", 50, 50)
	root.AddChild(back)
	root.AddChild(front)
	root.SetSize(core.Size{Width: 100, Height: 100})
	back.SetSize(core.Size{Width: 100, Height: 100})
	back.SetOffset(core.Point{})
	front.SetSize(core.Size{Width: 50, Height: 50})
	front.SetOffset(core.Point{X: 10, Y: 10})

	if hit := root.HitTest(core.Point{X: 20, Y: 20}); hit != front {
		t.Fatalf("hit=%v want front", hit)
	}
	if hit := root.HitTest(core.Point{X: 80, Y: 80}); hit != back {
		t.Fatalf("hit=%v want back", hit)
	}
}

func TestTreePointerClick(t *testing.T) {
	c := newClickLeaf()
	tree := core.NewTree(c)
	tree.Layout(core.Size{Width: 200, Height: 100})

	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 10, Y: 10, Button: core.ButtonLeft})
	if c.clicks != 1 {
		t.Fatalf("clicks=%d want 1", c.clicks)
	}
}

func TestPluginRegisterNoOverwrite(t *testing.T) {
	h := core.NewPluginHost()
	if err := h.RegisterControl("primitive.Box", 1); err != nil {
		t.Fatal(err)
	}
	if err := h.RegisterControl("primitive.Box", 2); err == nil {
		t.Fatal("expected duplicate register error")
	}
	if err := h.ReplaceControl("primitive.Box", 3); err != nil {
		t.Fatal(err)
	}
	v, ok := h.Control("primitive.Box")
	if !ok || v.(int) != 3 {
		t.Fatalf("got %v", v)
	}
}

func TestStackLayoutAlign(t *testing.T) {
	s := &container{}
	s.Init(s)
	s.Hit = core.HitDefer
	child := newLeaf("c", 20, 10)
	s.AddChild(child)
	sz := core.LayoutStack(&s.NodeBase, core.Tight(100, 80))
	if sz.Width != 100 || sz.Height != 80 {
		t.Fatalf("stack size=%+v", sz)
	}
}
