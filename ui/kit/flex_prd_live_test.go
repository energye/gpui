package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func mkDec(w, h float64) *primitive.Decorated {
	d := primitive.NewDecorated(nil)
	d.Width, d.Height = w, h
	return d
}

// gallery-like host: root → mid → scroll → slot → playground → kit.Flex
func mountGalleryLike(flexNode core.Node, vw, vh float64) *core.Tree {
	play := primitive.NewDecorated(flexNode)
	play.ExpandWidth = true
	play.Height = 120
	play.StretchChild = true
	slot := primitive.NewSlot("tab-body", play)
	slot.ExpandFill = true
	sv := primitive.NewScrollViewport(slot)
	sv.SetAxis(true, false)
	mid := primitive.NewDecorated(sv)
	mid.ExpandWidth = true
	mid.StretchChild = true
	root := primitive.NewDecorated(mid)
	root.Width, root.Height = vw, vh
	root.StretchChild = true
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: vw, Height: vh})
	return tree
}

func cleanTree(tree *core.Tree, n core.Node) {
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
	clean(n)
	tree.ClearDirty()
}

// FLX-06 / FLX-S5: justify change must remeasure without viewport resize.
func TestFlex_PRD_06_JustifyLive_NoResize(t *testing.T) {
	a, b := mkDec(40, 20), mkDec(40, 20)
	f := kit.NewFlex(a, b)
	f.SetJustify(kit.FlexJustifyStart)
	tree := mountGalleryLike(f.Node(), 400, 300)
	cleanTree(tree, tree.Root())

	f.SetJustify(kit.FlexJustifySpaceBetween)
	// Must schedule layout without size change
	tree.Frame(nil, core.Size{Width: 400, Height: 300})
	x := f.Root.Children()[1].Base().Offset().X
	if x < 300 {
		t.Fatalf("FLX-06 live space-between x=%v want ≳300 (no resize)", x)
	}
}

// FLX-07 / FLX-S6: align=center live without resize.
func TestFlex_PRD_07_AlignLive_NoResize(t *testing.T) {
	a := mkDec(40, 20)
	f := kit.NewFlex(a)
	f.SetAlign(kit.FlexAlignStart)
	f.SetWrap(false) // antd align playground is single-line
	tree := mountGalleryLike(f.Node(), 400, 300)
	cleanTree(tree, tree.Root())

	f.SetAlign(kit.FlexAlignCenter)
	tree.Frame(nil, core.Size{Width: 400, Height: 300})
	y := f.Root.Children()[0].Base().Offset().Y
	// playground height 120, child 20 → center ~50
	if y < 40 || y > 60 {
		t.Fatalf("FLX-07 live align center y=%v want ~50", y)
	}
}

// FLX-03 / FLX-S2: vertical live.
func TestFlex_PRD_03_VerticalLive_NoResize(t *testing.T) {
	a, b := mkDec(40, 20), mkDec(40, 20)
	f := kit.NewFlex(a, b)
	tree := mountGalleryLike(f.Node(), 400, 300)
	cleanTree(tree, tree.Root())
	f.SetVertical(true)
	tree.Frame(nil, core.Size{Width: 400, Height: 300})
	if f.Root.Children()[1].Base().Offset().Y <= f.Root.Children()[0].Base().Offset().Y {
		t.Fatal("FLX-03 live vertical: second child not below first")
	}
}
