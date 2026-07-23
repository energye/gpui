package primitive

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

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
