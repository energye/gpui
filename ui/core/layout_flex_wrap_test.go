package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// TestFlexWrap_TwoLines packs fixed-size children onto a second line when
// the main axis max is tight.
func TestFlexWrap_TwoLines(t *testing.T) {
	// 3× (40×20) with gap 8 → single line needs 40*3+8*2 = 136; max 100 → 2 lines.
	a := newLeaf("a", 40, 20)
	b := newLeaf("b", 40, 20)
	c := newLeaf("c", 40, 20)
	f := &wrapFlex{axis: core.AxisHorizontal, gap: 8, wrap: true}
	f.Init(f)
	f.Hit = core.HitDefer
	f.AddChild(a)
	f.AddChild(b)
	f.AddChild(c)

	sz := f.Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	// Line1: a,b → 40+8+40 = 88; Line2: c → 40. Height: 20+8+20 = 48.
	if sz.Width < 88-0.5 || sz.Width > 100+0.5 {
		t.Fatalf("width got %v want ~88..100", sz.Width)
	}
	if sz.Height < 47.5 || sz.Height > 48.5 {
		t.Fatalf("height got %v want 48", sz.Height)
	}
	// a at (0,0), b at (48,0), c at (0,28)
	if a.Offset().Y != 0 || b.Offset().Y != 0 {
		t.Fatalf("line0 y: a=%v b=%v", a.Offset(), b.Offset())
	}
	if c.Offset().Y < 27.5 || c.Offset().Y > 28.5 {
		t.Fatalf("c.Y got %v want 28", c.Offset().Y)
	}
	if c.Offset().X != 0 {
		t.Fatalf("c.X got %v want 0", c.Offset().X)
	}
}

// TestFlexWrap_UnboundedMainDoesNotWrap keeps single-line when max main is infinite.
func TestFlexWrap_UnboundedMainDoesNotWrap(t *testing.T) {
	a := newLeaf("a", 40, 20)
	b := newLeaf("b", 40, 20)
	c := newLeaf("c", 40, 20)
	f := &wrapFlex{axis: core.AxisHorizontal, gap: 8, wrap: true}
	f.Init(f)
	f.AddChild(a)
	f.AddChild(b)
	f.AddChild(c)
	sz := f.Layout(core.Constraints{MaxWidth: core.Unbounded, MaxHeight: 400})
	// Single line: 40*3+16 = 136, height 20.
	if sz.Width < 135.5 || sz.Width > 136.5 {
		t.Fatalf("width got %v want 136", sz.Width)
	}
	if sz.Height < 19.5 || sz.Height > 20.5 {
		t.Fatalf("height got %v want 20", sz.Height)
	}
	if c.Offset().Y != 0 {
		t.Fatalf("c should stay on first line, y=%v", c.Offset().Y)
	}
}

// TestFlexWrap_HitMatchesOffset ensures hit path uses wrapped offsets.
func TestFlexWrap_HitMatchesOffset(t *testing.T) {
	a := newLeaf("a", 40, 20)
	b := newLeaf("b", 40, 20)
	c := newLeaf("c", 40, 20)
	f := &wrapFlex{axis: core.AxisHorizontal, gap: 8, wrap: true}
	f.Init(f)
	f.Hit = core.HitDefer
	f.AddChild(a)
	f.AddChild(b)
	f.AddChild(c)
	_ = f.Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	// Click center of c (local 0,28 → 20,38 relative to flex).
	hit := f.HitTest(core.Point{X: c.Offset().X + 10, Y: c.Offset().Y + 10})
	if hit != c {
		t.Fatalf("hit got %v want c", hit)
	}
}

type wrapFlex struct {
	core.NodeBase
	axis core.Axis
	gap  float64
	wrap bool
}

func (f *wrapFlex) TypeID() string { return "test.wrapflex" }
func (f *wrapFlex) Layout(c core.Constraints) core.Size {
	return core.LayoutFlex(&f.NodeBase, c, core.FlexLayoutParams{
		Axis: f.axis, Gap: f.gap, Wrap: f.wrap, CrossAlign: core.CrossStart,
	})
}
func (f *wrapFlex) Paint(pc *core.PaintContext)    { f.DefaultPaintChildren(pc) }
func (f *wrapFlex) HitTest(p core.Point) core.Node { return f.DefaultHitTest(p) }
