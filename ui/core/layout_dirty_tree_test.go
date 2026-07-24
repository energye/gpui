package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Intermediate clean ancestor + LayoutSkipIfClean must not hide a dirty descendant.
func TestTree_DeepMarkNeedsLayout_Remeasures(t *testing.T) {
	a := primitive.NewDecorated(nil)
	a.Width, a.Height = 20, 20
	b := primitive.NewDecorated(nil)
	b.Width, b.Height = 20, 20
	row := primitive.Row(a, b)
	row.MainAlign = core.MainStart
	row.ExpandMax = true

	// multi-level ancestors like Tabs → Scroll → Slot → Decorated → Flex
	inner := primitive.NewDecorated(row)
	inner.Width, inner.Height = 200, 40
	inner.StretchChild = true
	mid := primitive.NewDecorated(inner)
	mid.ExpandWidth = true
	mid.StretchChild = true
	host := primitive.NewDecorated(mid)
	host.Width, host.Height = 200, 40
	host.StretchChild = true

	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 200, Height: 40})
	if b.Base().Offset().X > 30 {
		t.Fatalf("start x=%v", b.Base().Offset().X)
	}

	// Simulate finished frame: clear ALL dirty flags including intermediates.
	var clean func(core.Node)
	clean = func(n core.Node) {
		if n == nil {
			return
		}
		n.Base().ClearLayoutDirty()
		n.Base().ClearPaintDirty()
		for _, c := range n.Children() {
			clean(c)
		}
	}
	clean(host)
	tree.ClearDirty()

	// Only mark the deep row (product Flex.apply path).
	row.MainAlign = core.MainSpaceBetween
	row.MarkNeedsLayout()

	// Critical: intermediate mid/inner must also be dirty, otherwise SkipIfClean
	// skips them and never reaches row.
	if !inner.Base().NeedsLayout() {
		t.Fatal("inner ancestor not needsLayout after deep MarkNeedsLayout — SkipIfClean will skip flex")
	}
	if !mid.Base().NeedsLayout() {
		t.Fatal("mid ancestor not needsLayout")
	}
	if !host.Base().NeedsLayout() {
		t.Fatal("root not needsLayout")
	}

	tree.Frame(nil, core.Size{Width: 200, Height: 40})
	if b.Base().Offset().X < 150 {
		t.Fatalf("remeasure failed: b.x=%v want ~180", b.Base().Offset().X)
	}
}

func TestTree_AlignCenter_Live(t *testing.T) {
	child := primitive.NewDecorated(nil)
	child.Width, child.Height = 40, 20
	row := primitive.Row(child)
	row.CrossAlign = core.CrossStart
	row.ExpandMax = true
	play := primitive.NewDecorated(row)
	play.Width, play.Height = 200, 120
	play.StretchChild = true
	tree := core.NewTree(play)
	tree.Layout(core.Size{Width: 200, Height: 120})
	y0 := child.Base().Offset().Y
	if y0 > 1 {
		t.Fatalf("start y=%v", y0)
	}

	var clean func(core.Node)
	clean = func(n core.Node) {
		if n == nil {
			return
		}
		n.Base().ClearLayoutDirty()
		n.Base().ClearPaintDirty()
		for _, c := range n.Children() {
			clean(c)
		}
	}
	clean(play)
	tree.ClearDirty()

	row.CrossAlign = core.CrossCenter
	row.MarkNeedsLayout()
	tree.Frame(nil, core.Size{Width: 200, Height: 120})
	y1 := child.Base().Offset().Y
	if y1 < 40 || y1 > 60 {
		t.Fatalf("align center live y0=%v y1=%v want ~50", y0, y1)
	}
}
