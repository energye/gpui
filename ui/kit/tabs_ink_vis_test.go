package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestTabsInkVisibleAfterLayout(t *testing.T) {
	// Mimic gallery: many left tabs with headers
	items := []kit.MenuItem{
		{Key: "cat", Label: "General", Disabled: true},
		{Divider: true},
		{Key: "a", Label: "Button"},
		{Key: "b", Label: "Icon"},
		{Key: "c", Label: "Typography"},
	}
	tabs := kit.NewTabs(items...)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 168
	tabs.TabItemHeight = 36
	tabs.SetInkSize(3)
	for _, it := range items {
		if it.Selectable() {
			tabs.SetContent(it.Key, kit.NewText(it.Label).Node())
		}
	}
	tabs.SetActive("a")
	tree := core.NewTree(tabs.Node())
	tree.Layout(core.Size{Width: 1024, Height: 768})

	// Find blue-ish box: walk for Box with small width ~3 and primary color
	var found bool
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || found {
			return
		}
		if b, ok := n.(*primitive.Box); ok {
			sz := b.Size()
			// left ink: width ~3, height ~36
			if sz.Width >= 2 && sz.Width <= 6 && sz.Height >= 20 {
				if b.Color.A > 0.2 {
					found = true
					t.Logf("ink box size=%v color=%v offset=%v abs=%v", sz, b.Color, b.Offset(), core.AbsoluteBounds(b))
					return
				}
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(tabs.Node())
	if !found {
		t.Fatal("no ink indicator box found after layout")
	}

	// Switch and ensure animation ticker registers
	tabs.AttachTicker(tree)
	tabs.SetActive("c")
	// After SetActive with anim, ticker should be active
	tree.Layout(core.Size{Width: 1024, Height: 768})
	if tabs.Active != "c" {
		t.Fatal(tabs.Active)
	}
}
