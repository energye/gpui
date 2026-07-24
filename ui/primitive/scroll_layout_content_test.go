package primitive_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// ScrollViewport (vertical) + ExpandFill Slot used to lay out children with
// MaxWidth only (MinWidth=0). Flex then hugged content and space-between had
// zero free space — justify looked broken until a window resize "fixed" it.
func TestSlot_ExpandFill_PassesTightWidthToChild(t *testing.T) {
	a := primitive.NewDecorated(nil)
	a.Width, a.Height = 40, 20
	b := primitive.NewDecorated(nil)
	b.Width, b.Height = 40, 20
	row := primitive.Row(a, b)
	row.MainAlign = core.MainSpaceBetween
	row.ExpandMax = true

	slot := primitive.NewSlot("body", row)
	slot.ExpandFill = true

	// Mimic vertical ScrollViewport content constraints: bounded max, loose min.
	_ = slot.Layout(core.Constraints{MaxWidth: 200, MaxHeight: core.Unbounded})
	if row.Base().Size().Width < 199 {
		t.Fatalf("row width=%v want fill 200", row.Base().Size().Width)
	}
	if b.Base().Offset().X < 150 {
		t.Fatalf("space-between second x=%v want ~160", b.Base().Offset().X)
	}
}

func TestFlex_ExpandMax_SpaceBetweenUnderLooseMax(t *testing.T) {
	a := primitive.NewDecorated(nil)
	a.Width, a.Height = 40, 20
	b := primitive.NewDecorated(nil)
	b.Width, b.Height = 40, 20
	row := primitive.Row(a, b)
	row.MainAlign = core.MainSpaceBetween
	row.ExpandMax = true

	_ = row.Layout(core.Constraints{MaxWidth: 200, MaxHeight: 100})
	if row.Base().Size().Width < 199 {
		t.Fatalf("width=%v want 200", row.Base().Size().Width)
	}
	if b.Base().Offset().X < 150 {
		t.Fatalf("second x=%v want ~160", b.Base().Offset().X)
	}
}

func TestFlex_WithoutExpandMax_HugsContent(t *testing.T) {
	// Button chrome rows must not expand to full parent width.
	a := primitive.NewDecorated(nil)
	a.Width, a.Height = 40, 20
	b := primitive.NewDecorated(nil)
	b.Width, b.Height = 40, 20
	row := primitive.Row(a, b)
	row.MainAlign = core.MainSpaceBetween
	// ExpandMax false (default)
	_ = row.Layout(core.Constraints{MaxWidth: 200, MaxHeight: 100})
	if row.Base().Size().Width > 90 {
		t.Fatalf("hug content width=%v want ~80", row.Base().Size().Width)
	}
}

func TestScrollViewport_KitLikeJustifyAfterMarkNeedsLayout(t *testing.T) {
	a := primitive.NewDecorated(nil)
	a.Width, a.Height = 40, 20
	b := primitive.NewDecorated(nil)
	b.Width, b.Height = 40, 20
	row := primitive.Row(a, b)
	row.MainAlign = core.MainStart
	row.ExpandMax = true

	body := primitive.NewSlot("b", row)
	body.ExpandFill = true
	sv := primitive.NewScrollViewport(body)
	sv.Width, sv.Height = 200, 80
	sv.SetAxis(true, false)

	tree := core.NewTree(sv)
	tree.Layout(core.Size{Width: 200, Height: 80})
	if b.Base().Offset().X > 50 {
		t.Fatalf("start: second x=%v", b.Base().Offset().X)
	}

	row.MainAlign = core.MainSpaceBetween
	row.MarkNeedsLayout()
	tree.Layout(core.Size{Width: 200, Height: 80})
	if b.Base().Offset().X < 150 {
		t.Fatalf("after SetJustify-like change second x=%v want ~160", b.Base().Offset().X)
	}
	if !sv.Base().NeedsPaint() && !row.Base().NeedsPaint() {
		t.Fatal("paint dirty lost after layout; composite path would skip refresh")
	}
}
