package primitive

import (
	"testing"

	"github.com/energye/gpui/ui/core"
)

// While thumb-dragging, tree must not remeasure even if another node MarkNeedsLayout
// (Switch/Progress animation). Layout mid-drag shifts AbsoluteBounds → thumb jump.
func TestDragFreezesTreeLayout(t *testing.T) {
	col := Column()
	for i := 0; i < 20; i++ {
		b := NewBox()
		b.Height = 40
		b.Width = 80
		col.AddChild(b)
	}
	sv := NewScrollViewport(col)
	sv.Width, sv.Height = 100, 150
	sv.Scrollbar().Horizontal = ScrollbarNever
	other := NewBox()
	other.Width, other.Height = 20, 20
	host := NewBox(sv, other)
	host.Width, host.Height = 160, 150
	tree := core.NewTree(host)
	vp := core.Size{Width: 160, Height: 150}
	tree.Frame(&core.PaintContext{}, vp)

	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	_, _, y0, _, h0 := sv.vThumbGeom(sv.Size())
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + y0 + h0/2
	sv.HandlePointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.Dragging() {
		t.Fatal("expected drag")
	}
	// Capture is set by tree on real dispatch; simulate capture for needsLayoutPass.
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})

	// Mid-drag: another widget demands layout (animation-style).
	other.MarkNeedsLayout()
	if !other.NeedsLayout() {
		t.Fatal("other should be layout dirty")
	}
	// Tree.Frame must skip layout while capture is dragging.
	oy0 := core.AbsoluteBounds(sv).Min.Y
	// mutate host height slightly via constraints by changing other size demand —
	// only matters if layout runs.
	other.Height = 80
	tree.Frame(&core.PaintContext{}, vp)
	oy1 := core.AbsoluteBounds(sv).Min.Y
	if oy0 != oy1 {
		t.Fatalf("scroll absolute Y jumped during drag layout thrash: %.2f → %.2f", oy0, oy1)
	}
	if other.NeedsLayout() {
		// Layout was skipped so dirty may remain — that is OK while dragging.
	}
}
