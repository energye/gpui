package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

func TestWave3_SkinPaintersRegistered(t *testing.T) {
	th := skindefault.Theme()
	for _, id := range []string{
		kit.TypeTabs, kit.TypeMenu, kit.TypeDropdown, kit.TypePagination,
		kit.TypeSteps, kit.TypeAnchor, kit.TypeSpace, kit.TypeKitFlex,
		kit.TypeScroll, kit.TypeLayout, kit.TypeSplitter,
	} {
		if th.Painter(id) == nil {
			t.Errorf("missing painter %s", id)
		}
	}
}

func TestWave3_EnsureBuilt_Nodes(t *testing.T) {
	cases := []struct {
		name string
		node func() core.Node
	}{
		{"Tabs", func() core.Node { return kit.NewTabs(kit.TabItem{Key: "a", Label: "A"}).Node() }},
		{"Menu", func() core.Node { return kit.NewMenu(kit.MenuItem{Key: "a", Label: "A"}).Node() }},
		{"Dropdown", func() core.Node { return kit.NewDropdown("D", kit.MenuItem{Key: "a", Label: "A"}).Node() }},
		{"Pagination", func() core.Node { return kit.NewPagination().Node() }},
		{"Steps", func() core.Node { return kit.NewSteps(kit.StepItem{Title: "T"}).Node() }},
		{"Anchor", func() core.Node { return kit.NewAnchor(kit.AnchorItem{Key: "a", Title: "A"}).Node() }},
		{"Space", func() core.Node { return kit.NewSpace().Node() }},
		{"Flex", func() core.Node { return kit.NewFlex().Node() }},
		{"Scroll", func() core.Node { return kit.NewScroll(nil).Node() }},
		{"Layout", func() core.Node { return kit.NewLayout().Node() }},
		{"Splitter", func() core.Node {
			return kit.NewSplitter(kit.NewSplitterPanel(nil), kit.NewSplitterPanel(nil)).Node()
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := tc.node()
			if n == nil {
				t.Fatal("nil node")
			}
		})
	}
}
