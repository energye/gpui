package primitive

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// Drag MarkNeedsPaint must stay inside ScrollViewport (RepaintBoundary).
// Otherwise NonBoundaryPaintDirty forces full window base rebuild every move
// → lag then jump (跟手延迟后猛跳).
func TestScrollDragPaintStaysInsideBoundary(t *testing.T) {
	col := Column()
	for i := 0; i < 30; i++ {
		b := NewBox()
		b.Height = 40
		b.Width = 80
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 150
	sv.Scrollbar().Horizontal = ScrollbarNever
	if !sv.IsRepaintBoundary() {
		t.Fatal("ScrollViewport must be a RepaintBoundary for demand-path drag")
	}
	// Sibling outside the scroll — must not get paint-dirty from scroll drag.
	static := NewBox()
	static.Width, static.Height = 40, 40
	host := NewBox(sv, static)
	host.Width, host.Height = 160, 150
	tree := core.NewTree(host)
	vp := core.Size{Width: 160, Height: 150}
	tree.Frame(&core.PaintContext{}, vp)
	if tree.FullPaintRequired() {
		t.Fatal("full paint should clear after first frame")
	}
	if tree.NonBoundaryPaintDirty() {
		t.Fatal("tree should not have non-boundary paint dirty after full frame")
	}

	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if sv.drag.axis != 1 {
		t.Fatalf("expected vertical drag, axis=%d", sv.drag.axis)
	}
	// Clear paint flags from down, then move.
	tree.Frame(&core.PaintContext{}, vp)

	sv.HandlePointer(&core.PointerEvent{
		Type: core.PointerMove, X: downX, Y: downY + 40, Button: core.ButtonLeft,
	})
	if !sv.NeedsPaint() {
		t.Fatal("scroll viewport should be paint dirty after drag move")
	}
	if static.NeedsPaint() {
		t.Fatal("sibling outside scroll must stay paint-clean (boundary isolation)")
	}
	if host.NeedsPaint() {
		t.Fatal("host above ScrollViewport boundary must stay paint-clean")
	}
	if tree.NonBoundaryPaintDirty() {
		t.Fatal("drag paint must not force full base rebuild (NonBoundaryPaintDirty)")
	}
	if !tree.Dirty() {
		t.Fatal("tree must still schedule a frame for the boundary layer")
	}
}
