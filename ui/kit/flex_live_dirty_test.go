package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func boxWH(w, h float64) core.Node {
	d := primitive.NewDecorated(nil)
	d.Width, d.Height = w, h
	return d
}

func TestFlex_ClickPath_MarksTreeDirty(t *testing.T) {
	f := kit.NewFlex(boxWH(40, 20), boxWH(40, 20))
	host := primitive.NewDecorated(f.Node())
	host.Width, host.Height = 200, 40
	host.StretchChild = true
	tree := core.NewTree(host)
	tree.Layout(core.Size{Width: 200, Height: 40})
	tree.ClearDirty()
	if tree.Dirty() {
		t.Fatal("expected clean after layout+clear")
	}
	// simulate radio/segmented onChange
	f.SetJustify(kit.FlexJustifySpaceBetween)
	if !tree.Dirty() {
		t.Fatal("tree not dirty after SetJustify — frame will not paint")
	}
	if !host.Base().NeedsLayout() && !f.Root.Base().NeedsLayout() {
		t.Fatal("neither host nor flex needs layout")
	}
	// Frame path: needsLayoutPass then layout
	tree.Frame(nil, core.Size{Width: 200, Height: 40})
	x := f.Root.Children()[1].Base().Offset().X
	if x < 150 {
		t.Fatalf("after Frame, second x=%v want ~160", x)
	}
}

func TestFlex_WrapUnderBoundedParent(t *testing.T) {
	// 8 boxes of 72 + gap 8; host max 200 should wrap
	kids := make([]core.Node, 8)
	for i := range kids {
		kids[i] = boxWH(72, 28)
	}
	f := kit.NewFlex(kids...)
	f.SetWrap(true)
	f.SetGap(8)
	// parent with ExpandWidth-like max only
	sz := f.Node().Layout(core.Constraints{MaxWidth: 200, MaxHeight: 400})
	if sz.Height < 50 {
		t.Fatalf("should wrap multi-line h=%v", sz.Height)
	}
	// unbounded max — no wrap
	f2 := kit.NewFlex(kids...)
	f2.SetWrap(true)
	f2.SetGap(8)
	sz2 := f2.Node().Layout(core.Constraints{MaxWidth: core.Unbounded, MaxHeight: 400})
	if sz2.Height > 40 {
		t.Fatalf("unbounded should not wrap h=%v", sz2.Height)
	}
}
