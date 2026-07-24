package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func mkBox(w, h float64) core.Node {
	d := primitive.NewDecorated(nil)
	d.Width, d.Height = w, h
	return d
}

// Gallery-like: Tabs body = ScrollViewport + ExpandFill Slot + kit.Flex
func galleryHost(child core.Node, w, h float64) (*core.Tree, *primitive.ScrollViewport) {
	slot := primitive.NewSlot("tab-body", child)
	slot.ExpandFill = true
	sv := primitive.NewScrollViewport(slot)
	sv.Width, sv.Height = w, h
	sv.SetAxis(true, false)
	tree := core.NewTree(sv)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree, sv
}

func TestFlex_PRD_AlignCenter_UnderScroll(t *testing.T) {
	// FLX-07 / FLX-S6: align=center cross-axis center in bounded height playground
	a := mkBox(40, 20)
	f := kit.NewFlex(a)
	f.SetAlign(kit.FlexAlignCenter)
	// playground fixed height like align demo
	play := primitive.NewDecorated(f.Node())
	play.ExpandWidth = true
	play.Height = 120
	play.StretchChild = true
	tree, _ := galleryHost(play, 400, 300)
	tree.Layout(core.Size{Width: 400, Height: 300})
	y := a.(*primitive.Decorated).Base().Offset().Y
	// under StretchChild host 120, center of 20-tall → y≈50
	// but Offset is relative to flex parent; flex is child of play with StretchChild
	// Absolute-ish: child of flex
	kids := f.Root.Children()
	if len(kids) == 0 {
		t.Fatal("no kids")
	}
	y = kids[0].Base().Offset().Y
	if y < 40 || y > 60 {
		t.Fatalf("FLX-07 align center y=%v want ~50 (host h=120 child h=20)", y)
	}
}

func TestFlex_PRD_JustifyLive_UnderScroll(t *testing.T) {
	// FLX-06 / FLX-S5: justify change must apply on next layout without resize
	a, b := mkBox(40, 20), mkBox(40, 20)
	f := kit.NewFlex(a, b)
	f.SetJustify(kit.FlexJustifyStart)
	play := primitive.NewDecorated(f.Node())
	play.ExpandWidth = true
	play.Height = 40
	play.StretchChild = true
	tree, sv := galleryHost(play, 400, 200)
	tree.Layout(core.Size{Width: 400, Height: 200})
	x0 := f.Root.Children()[1].Base().Offset().X

	f.SetJustify(kit.FlexJustifySpaceBetween)
	// demand path: Frame layout if dirty
	if !tree.Dirty() {
		t.Fatal("tree not dirty after SetJustify")
	}
	tree.Frame(nil, core.Size{Width: 400, Height: 200})
	x1 := f.Root.Children()[1].Base().Offset().X
	if x1 <= x0+50 {
		t.Fatalf("FLX-06 live justify: before=%v after=%v want space-between ~ near right", x0, x1)
	}
	// paint dirty on scroll for composite path
	if !sv.Base().NeedsPaint() && !f.Root.Base().NeedsPaint() {
		t.Fatal("no paint dirty after layout change — UI won't refresh until full paint/resize")
	}
}

func TestFlex_PRD_AlignLive_UnderScroll(t *testing.T) {
	// FLX-07 live: SetAlign after first layout must re-center without resize
	a := mkBox(40, 20)
	f := kit.NewFlex(a)
	f.SetAlign(kit.FlexAlignStart)
	play := primitive.NewDecorated(f.Node())
	play.ExpandWidth = true
	play.Height = 120
	play.StretchChild = true
	tree, _ := galleryHost(play, 400, 300)
	tree.Layout(core.Size{Width: 400, Height: 300})
	y0 := f.Root.Children()[0].Base().Offset().Y
	f.SetAlign(kit.FlexAlignCenter)
	tree.Frame(nil, core.Size{Width: 400, Height: 300})
	y1 := f.Root.Children()[0].Base().Offset().Y
	if y1 < 40 || y1 > 60 {
		t.Fatalf("FLX-07 live align center: y0=%v y1=%v want ~50", y0, y1)
	}
}

func TestSlider_TrackVisibleWithExpandWidth(t *testing.T) {
	// customize gap slider in gallery: Width=0 + ExpandWidth host
	s := kit.NewSlider(16)
	s.Min, s.Max = 0, 64
	s.Width = 0
	host := primitive.NewDecorated(s.Node())
	host.ExpandWidth = true
	host.Height = 32
	host.StretchChild = true
	sz := host.Layout(core.Constraints{MinWidth: 280, MaxWidth: 280, MaxHeight: 40})
	if sz.Width < 100 {
		t.Fatalf("slider host width=%v", sz.Width)
	}
	// slider root size
	ss := s.Node().Base().Size()
	t.Logf("slider node size=%v host=%v Width field=%v", ss, sz, s.Width)
	if ss.Width < 50 {
		t.Fatalf("slider laid out too narrow w=%v — track invisible (only thumb)", ss.Width)
	}
}
