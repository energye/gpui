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
	panel := primitive.NewDecorated()
	panel.Width, panel.Height = 400, 300
	tabs.SetContent("a", panel)
	tabs.SetContent("b", panel)
	tabs.SetContent("c", panel)
	tabs.SetActive("a")

	root := tabs.Node()
	// Parent gives tall height — tabs must not stretch each item to fill.
	_ = root.Layout(core.Tight(800, 600))

	// Root is Row: rail | div | body
	kids := root.Children()
	if len(kids) < 1 {
		t.Fatal("no children")
	}
	// First child should be rail Decorated ~160 wide
	rail, ok := kids[0].(*primitive.Decorated)
	if !ok {
		t.Fatalf("rail type %T", kids[0])
	}
	if rail.Size().Width < 150 || rail.Size().Width > 170 {
		t.Fatalf("rail width=%v want ~160", rail.Size().Width)
	}
	// Bar is inside rail — sum of item heights should be ~3*40, not ~600
	bar := rail.Children()
	if len(bar) < 1 {
		t.Fatal("empty rail")
	}
	col, ok := bar[0].(*primitive.Flex)
	if !ok {
		// might be nested
		t.Logf("rail child0 %T size=%v", bar[0], bar[0].Base().Size())
	} else {
		// column of 3 items
		items := col.Children()
		if len(items) != 3 {
			t.Fatalf("tab items=%d want 3", len(items))
		}
		var sum float64
		for _, it := range items {
			h := it.Base().Size().Height
			sum += h
			if h > 50 {
				t.Fatalf("tab item height=%v should be ~40, not filling rail", h)
			}
		}
		if sum > 150 {
			t.Fatalf("sum item heights=%v want ~120", sum)
		}
	}
}
