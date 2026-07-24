package primitive

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// countingText increments a counter every Paint — used to detect ghost re-paints
// of section labels during nested-boundary composite passes.
type countingText struct {
	core.NodeBase
	paints *int
}

func (t *countingText) TypeID() string { return "test.countingText" }

func (t *countingText) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: 80, Height: 20})
	t.SetSize(out)
	return out
}

func (t *countingText) Paint(pc *core.PaintContext) {
	if t.paints != nil {
		*t.paints++
	}
}

func (t *countingText) HitTest(p core.Point) core.Node { return t.DefaultHitTest(p) }

// Nested RepaintBoundary dirty must not force ScrollViewport needs-raster of content.
func TestScrollNestedBoundaryDoesNotDirtyContent(t *testing.T) {
	inner := NewBox()
	inner.Width, inner.Height = 40, 40
	b := NewRepaintBoundary(inner)
	col := Column(b)
	for i := 0; i < 5; i++ {
		x := NewBox()
		x.Width, x.Height = 40, 40
		col.AddChild(x)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 80
	host := NewBox(sv)
	host.Width, host.Height = 100, 80
	tree := core.NewTree(host)
	tree.Frame(&core.PaintContext{}, core.Size{Width: 100, Height: 80})

	// Dirty only the nested boundary.
	inner.MarkNeedsPaint()
	if !b.NeedsPaint() {
		t.Fatal("boundary should be dirty")
	}
	if sv.NeedsPaint() {
		t.Fatal("scroll must not be paint-dirty from nested boundary (MarkNeedsPaint stops)")
	}
	if scrollNonBoundaryDescPaintDirty(sv) {
		t.Fatal("nested boundary dirty must not count as non-boundary content dirty")
	}
}

// TestScrollNestedBoundaryDoesNotRepaintSiblingText is the Icon-gallery regression:
// section description Text next to Spin/Icon RepaintBoundary must not be re-painted
// on composite-only frames (would ghost labels onto the page background).
func TestScrollNestedBoundaryDoesNotRepaintSiblingText(t *testing.T) {
	var textPaints int
	lab := &countingText{paints: &textPaints}
	lab.Init(lab)
	lab.Hit = core.HitDefer

	inner := NewBox()
	inner.Width, inner.Height = 24, 24
	icon := NewRepaintBoundary(inner)

	// demoSection-like column: title text + icon row
	col := Column(lab, icon)
	col.Gap = 8

	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 200, 120
	host := NewBox(sv)
	host.Width, host.Height = 200, 120
	tree := core.NewTree(host)

	// Full first paint.
	tree.Frame(&core.PaintContext{}, core.Size{Width: 200, Height: 120})
	if textPaints < 1 {
		t.Fatalf("first paint textPaints=%d want >=1", textPaints)
	}
	first := textPaints

	// Dirty only the nested icon boundary (spin tick).
	inner.MarkNeedsPaint()
	// Composite-only style frame: ForceFullPaint false, CompositeOnly true.
	pc := &core.PaintContext{CompositeOnly: true}
	tree.Frame(pc, core.Size{Width: 200, Height: 120})

	if textPaints != first {
		t.Fatalf("sibling Text re-painted on nested-boundary composite frame: paints %d→%d (ghost text bug)",
			first, textPaints)
	}
}
