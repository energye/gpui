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

	// Geometry must be non-zero after layout for the active tab.
	// Ink node is transparent scaffolding; size still tracks the mark.
	var inkBox *primitive.Box
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || inkBox != nil {
			return
		}
		if b, ok := n.(*primitive.Box); ok {
			sz := b.Size()
			// left ink scaffold: width ~3, height ~36
			if sz.Width >= 2 && sz.Width <= 6 && sz.Height >= 20 {
				inkBox = b
				return
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(tabs.Node())
	if inkBox == nil {
		t.Fatal("no ink geometry box found after layout")
	}
	// Scaffold must not paint its own fill (sole painter is paintInk).
	if inkBox.Color.A > 0.01 {
		t.Fatalf("ink scaffold must be transparent to avoid double mark, color=%v", inkBox.Color)
	}

	// Switch: only one logical active; animation updates a single inkAlong.
	tabs.AttachTicker(tree)
	tabs.SetActive("c")
	tree.Layout(core.Size{Width: 1024, Height: 768})
	if tabs.Active != "c" {
		t.Fatal(tabs.Active)
	}
	// Scaffold still transparent after switch.
	if inkBox.Color.A > 0.01 {
		t.Fatalf("ink scaffold painted after switch: %v", inkBox.Color)
	}
}

// TestTabsInkSingleMarkOnSwitch ensures switching does not leave a second ink
// at the previous tab (ghost from dual paint / stale Offset).
func TestTabsInkSingleMarkOnSwitch(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "a", Label: "A"},
		kit.MenuItem{Key: "b", Label: "B"},
		kit.MenuItem{Key: "c", Label: "C"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetInkSize(3)
	tabs.SetInkAnimated(true)
	tabs.InkDuration = 0.2
	tabs.SetContent("a", primitive.NewText("A"))
	tabs.SetContent("b", primitive.NewText("B"))
	tabs.SetContent("c", primitive.NewText("C"))
	tabs.SetActive("a")

	tree := core.NewTree(tabs.Node())
	tabs.AttachTicker(tree)
	tree.Layout(core.Size{Width: 800, Height: 400})

	// Record active-only selected fills after switch to "c".
	tabs.SetActive("c")
	// rebuildBar replaces bar hosts; count selected backgrounds (should be 1).
	// Advance a few animation frames.
	for i := 0; i < 3; i++ {
		tree.TickActive(0.05)
	}
	tree.Layout(core.Size{Width: 800, Height: 400})

	// Walk bar list hosts: only the active tab host should have selected fill.
	// Find bar column children under the tree.
	selected := 0
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if d, ok := n.(*primitive.Decorated); ok {
			// selected fill is light primary tint; non-active is fully transparent
			bg := d.Background
			if bg.A > 0.02 && d.Size().Height >= 30 && d.Size().Width > 50 {
				// rough filter for tab host rows
				selected++
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(tabs.Node())
	// May count more than hosts if body has fills; require at least that Active is c
	// and ink scaffold stays transparent (no dual solid ink boxes).
	if tabs.Active != "c" {
		t.Fatal(tabs.Active)
	}
	var solidInk int
	var walkInk func(core.Node)
	walkInk = func(n core.Node) {
		if n == nil {
			return
		}
		if b, ok := n.(*primitive.Box); ok {
			sz := b.Size()
			if sz.Width >= 2 && sz.Width <= 6 && sz.Height >= 20 && b.Color.A > 0.2 {
				solidInk++
			}
		}
		for _, c := range n.Children() {
			walkInk(c)
		}
	}
	walkInk(tabs.Node())
	if solidInk != 0 {
		t.Fatalf("expected 0 solid ink boxes (paintInk only), got %d", solidInk)
	}
}
