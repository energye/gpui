package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// flexGrowShrink is a flex child with configurable grow/shrink and preferred size.
type flexGrowShrink struct {
	core.NodeBase
	w, h   float64
	grow   float64
	shrink float64
}

func newFlexGS(w, h, grow, shrink float64) *flexGrowShrink {
	n := &flexGrowShrink{w: w, h: h, grow: grow, shrink: shrink}
	n.Init(n)
	n.Hit = core.HitDefer
	return n
}

func (n *flexGrowShrink) TypeID() string           { return "test.flexgs" }
func (n *flexGrowShrink) FlexGrow() float64        { return n.grow }
func (n *flexGrowShrink) FlexShrink() float64      { return n.shrink }
func (n *flexGrowShrink) Paint(*core.PaintContext) {}
func (n *flexGrowShrink) HitTest(p core.Point) core.Node {
	return n.DefaultHitTest(p)
}
func (n *flexGrowShrink) Layout(c core.Constraints) core.Size {
	// Prefer intrinsic, but honor tight/capped main.
	pref := core.Size{Width: n.w, Height: n.h}
	out := c.Tighten(pref)
	n.SetSize(out)
	return out
}

// TestFlexShrink_Overflow: two items prefer 100 each (grow=0,shrink=1) in max 120 → each ~60.
func TestFlexShrink_Overflow(t *testing.T) {
	a := newFlexGS(100, 20, 0, 1)
	b := newFlexGS(100, 20, 0, 1)
	// Use wrapFlex-like host without wrap
	f := &wrapFlex{axis: core.AxisHorizontal, gap: 0, wrap: false}
	// Need a flex that passes grow/shrink - wrapFlex only uses LayoutFlex without custom grow from children that implement FlexFactorNode
	f.Init(f)
	f.AddChild(a)
	f.AddChild(b)
	sz := f.Layout(core.Constraints{MaxWidth: 120, MaxHeight: 100})
	if sz.Width > 120.5 {
		t.Fatalf("parent width %v > 120", sz.Width)
	}
	// Equal shrink from 100+100=200 deficit 80 → each loses 40 → 60
	if a.Size().Width < 55 || a.Size().Width > 65 {
		t.Fatalf("a width %v want ~60", a.Size().Width)
	}
	if b.Size().Width < 55 || b.Size().Width > 65 {
		t.Fatalf("b width %v want ~60", b.Size().Width)
	}
}

// TestFlexShrink_ZeroDoesNotShrink: shrink=0 keeps preferred size (may overflow parent).
func TestFlexShrink_ZeroDoesNotShrink(t *testing.T) {
	a := newFlexGS(80, 20, 0, 0)
	b := newFlexGS(80, 20, 1, 1)
	f := &wrapFlex{axis: core.AxisHorizontal, gap: 0, wrap: false}
	f.Init(f)
	f.AddChild(a)
	f.AddChild(b)
	_ = f.Layout(core.Constraints{MaxWidth: 100, MaxHeight: 100})
	if a.Size().Width < 79 {
		t.Fatalf("a should not shrink, got %v", a.Size().Width)
	}
	// b takes remaining ~20
	if b.Size().Width > 25 {
		t.Fatalf("b should absorb shrink, got %v", b.Size().Width)
	}
}
