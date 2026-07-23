package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestPrimitiveFlex_WrapGeometry(t *testing.T) {
	mk := func(w, h float64) *primitive.Box {
		b := primitive.NewBox()
		b.Width, b.Height = w, h
		return b
	}
	a, b, c := mk(40, 20), mk(40, 20), mk(40, 20)
	row := primitive.Row(a, b, c)
	row.Gap = 8
	row.Wrap = true
	sz := row.Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	if sz.Height < 47.5 || sz.Height > 48.5 {
		t.Fatalf("height got %v want 48", sz.Height)
	}
	if c.Offset().Y < 27.5 || c.Offset().Y > 28.5 {
		t.Fatalf("c.Y got %v want 28", c.Offset().Y)
	}
}
