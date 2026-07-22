package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestScrollViewportOverflowAndThumb(t *testing.T) {
	// Tall content in short viewport.
	col := primitive.Column()
	for i := 0; i < 20; i++ {
		b := primitive.NewBox()
		b.Width, b.Height = 100, 40
		col.AddChild(b)
	}
	sv := primitive.NewScrollViewport(col)
	sv.Width, sv.Height = 120, 100
	sz := sv.Layout(core.Tight(120, 100))
	if sz.Height != 100 {
		t.Fatalf("viewport h=%v", sz.Height)
	}
	if !sv.OverflowY() {
		t.Fatalf("want overflow ContentH=%v", sv.ContentH)
	}
	// Wheel down
	sv.HandleScroll(&core.ScrollEvent{DY: 50})
	if sv.ScrollY < 49 {
		t.Fatalf("ScrollY=%v", sv.ScrollY)
	}
	// Clamp at max
	sv.SetScroll(0, 1e9)
	if sv.ScrollY > sv.ContentH-sv.Size().Height+0.5 {
		t.Fatalf("not clamped ScrollY=%v max=%v", sv.ScrollY, sv.ContentH-100)
	}
}

func TestScrollViewportHorizontal(t *testing.T) {
	row := primitive.Row()
	for i := 0; i < 10; i++ {
		b := primitive.NewBox()
		b.Width, b.Height = 80, 30
		row.AddChild(b)
	}
	sv := primitive.NewScrollViewport(row)
	sv.SetAxis(false, true)
	sv.Width, sv.Height = 200, 40
	_ = sv.Layout(core.Tight(200, 40))
	if !sv.OverflowX() {
		t.Fatalf("want overflowX ContentW=%v", sv.ContentW)
	}
	sv.HandleScroll(&core.ScrollEvent{DX: 40})
	if sv.ScrollX < 39 {
		t.Fatalf("ScrollX=%v", sv.ScrollX)
	}
}
