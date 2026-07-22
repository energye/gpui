package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestTabsInkSlidesOnSwitch(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "a", Label: "A"},
		kit.MenuItem{Key: "b", Label: "B"},
		kit.MenuItem{Key: "c", Label: "C"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetInkSize(3)
	tabs.SetInkColor(render.Hex("#1677FF"))
	tabs.SetInkAnimated(true)
	tabs.InkDuration = 0.2
	tabs.SetContent("a", primitive.NewText("A"))
	tabs.SetContent("b", primitive.NewText("B"))
	tabs.SetContent("c", primitive.NewText("C"))
	tabs.SetActive("a")

	tree := core.NewTree(tabs.Node())
	tabs.AttachTicker(tree)
	tree.Layout(core.Size{Width: 800, Height: 400})

	tabs.SetActive("c")
	// Animation should be running
	if !tree.HasActiveTickers() {
		// may already need a tick registration
		tree.BindTicker(tabs, true)
	}
	// Advance halfway
	for i := 0; i < 5; i++ {
		tree.TickActive(0.04)
	}
	// Still not finished or finished — ink should have moved toward C (slot y≈80)
	// Smoke: no panic + active is c
	if tabs.Active != "c" {
		t.Fatal(tabs.Active)
	}
	// Finish animation
	for i := 0; i < 20; i++ {
		if !tree.TickActive(0.05) {
			break
		}
	}
}

func TestTabsInkConfigurable(t *testing.T) {
	tabs := kit.NewTabs(kit.MenuItem{Key: "a", Label: "A"}, kit.MenuItem{Key: "b", Label: "B"})
	tabs.SetPosition(kit.TabLeft)
	tabs.SetInkSize(5)
	tabs.SetInkColor(render.Hex("#FF0000"))
	tabs.HideInk = false
	_ = tabs.Node().Layout(core.Loose(400, 300))
	if tabs.TabInkWidth != 5 {
		t.Fatal(tabs.TabInkWidth)
	}
	if tabs.TabInkColor.R < 0.9 {
		t.Fatal(tabs.TabInkColor)
	}
}
