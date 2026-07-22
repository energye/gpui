package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestTabsLeftItemHeightNotFillRail(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "a", Label: "A"},
		kit.MenuItem{Key: "b", Label: "B"},
		kit.MenuItem{Key: "c", Label: "C"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("a", primitive.NewText("panel-A"))
	tabs.SetContent("b", primitive.NewText("panel-B"))
	tabs.SetContent("c", primitive.NewText("panel-C"))
	tabs.SetActive("a")

	root := tabs.Node()
	_ = root.Layout(core.Tight(800, 600))

	kids := root.Children()
	rail, ok := kids[0].(*primitive.Decorated)
	if !ok {
		t.Fatalf("rail type %T", kids[0])
	}
	if rail.Size().Width < 150 || rail.Size().Width > 170 {
		t.Fatalf("rail width=%v want ~160", rail.Size().Width)
	}
	// rail → ScrollViewport → Stack(barList, ink) → Flex(barList)
	scroll, ok := rail.Children()[0].(*primitive.ScrollViewport)
	if !ok {
		t.Fatalf("rail child type %T want ScrollViewport", rail.Children()[0])
	}
	// scroll child is kit.tabsBarHost (Stack wrapper) — walk for Flex barList
	host := scroll.Children()[0]
	var bar *primitive.Flex
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || bar != nil {
			return
		}
		if f, ok := n.(*primitive.Flex); ok && len(f.Children()) >= 1 {
			// Prefer the flex that holds tab hosts (Decorated rows)
			if _, ok := f.Children()[0].(*primitive.Decorated); ok || len(f.Children()) == 3 {
				bar = f
				return
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(host)
	if bar == nil {
		// fallback: first flex under host
		walk = func(n core.Node) {
			if n == nil || bar != nil {
				return
			}
			if f, ok := n.(*primitive.Flex); ok {
				bar = f
				return
			}
			for _, c := range n.Children() {
				walk(c)
			}
		}
		walk(host)
	}
	if bar == nil {
		t.Fatalf("bar Flex not found under %T", host)
	}
	items := bar.Children()
	if len(items) != 3 {
		t.Fatalf("tab items=%d want 3", len(items))
	}
	var sum float64
	var prevY float64 = -1
	for i, it := range items {
		h := it.Base().Size().Height
		y := it.Base().Offset().Y
		sum += h
		if h > 50 {
			t.Fatalf("item %d height=%v should be ~40", i, h)
		}
		if prevY >= 0 && y <= prevY {
			t.Fatalf("item %d y=%v should be below previous y=%v (merged?)", i, y, prevY)
		}
		prevY = y
	}
	if sum < 90 || sum > 150 {
		t.Fatalf("sum heights=%v want ~120", sum)
	}
	// First item near top (rail pad 8), not vertically centered in 600.
	if items[0].Base().Offset().Y > 20 {
		t.Fatalf("first tab y=%v should be near top (~0), not centered", items[0].Base().Offset().Y)
	}
}

func TestTabsLeftClickSwitches(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "a", Label: "Alpha"},
		kit.MenuItem{Key: "b", Label: "Beta"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("a", primitive.NewText("AAA"))
	tabs.SetContent("b", primitive.NewText("BBB"))
	tabs.SetActive("a")

	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 800, Height: 600})

	// First tab: y ≈ 8..48 (rail top pad 8 + height 40)
	// Second tab: y ≈ 48..88
	x, y := 80.0, 68.0
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})

	if tabs.Active != "b" {
		// dump hit
		hit := tree.HitTest(core.Point{X: x, Y: y})
		t.Fatalf("after click active=%q want b; hit=%T abs=%v", tabs.Active, hit, core.AbsoluteBounds(hit))
	}
}
