package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func TestConstraints_Tighten(t *testing.T) {
	c := rendering.Constraints{MinWidth: 10, MaxWidth: 100, MinHeight: 5, MaxHeight: 50}
	sz := c.Tighten(rendering.Size{Width: 200, Height: 1})
	if sz.Width != 100 || sz.Height != 5 {
		t.Fatalf("%v", sz)
	}
}

func TestConstraints_Tight(t *testing.T) {
	c := rendering.Tight(40, 30)
	if c.MinWidth != 40 || c.MaxHeight != 30 {
		t.Fatalf("%v", c)
	}
}
