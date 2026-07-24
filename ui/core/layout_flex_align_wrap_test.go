package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// FLX-S6 / FLX-07: align=center in a tall single-line flex (playground h=120).
// Even with Wrap=true, one line must center on the container cross axis.
func TestFlexWrap_SingleLine_AlignCenterInTallBox(t *testing.T) {
	child := primitive.NewDecorated(nil)
	child.Width, child.Height = 60, 32
	row := primitive.Row(child)
	row.Wrap = true
	row.CrossAlign = core.CrossCenter
	row.ExpandMax = true
	// tight 200×120 like StretchChild playground
	_ = row.Layout(core.Constraints{MinWidth: 200, MaxWidth: 200, MinHeight: 120, MaxHeight: 120})
	y := child.Base().Offset().Y
	// (120-32)/2 = 44
	if y < 40 || y > 50 {
		t.Fatalf("align center in tall wrap flex: y=%v want ~44", y)
	}
}

func TestFlex_NoWrap_AlignCenterInTallBox(t *testing.T) {
	child := primitive.NewDecorated(nil)
	child.Width, child.Height = 60, 32
	row := primitive.Row(child)
	row.Wrap = false
	row.CrossAlign = core.CrossCenter
	row.ExpandMax = true
	_ = row.Layout(core.Constraints{MinWidth: 200, MaxWidth: 200, MinHeight: 120, MaxHeight: 120})
	y := child.Base().Offset().Y
	if y < 40 || y > 50 {
		t.Fatalf("y=%v want ~44", y)
	}
}
