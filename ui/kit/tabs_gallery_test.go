package kit_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestTabsGalleryLikeSwitch(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "btn", Label: "Button"},
		kit.MenuItem{Key: "input", Label: "Input"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("btn", primitive.NewText("AAA"))
	tabs.SetContent("input", primitive.NewText("BBB"))
	tabs.SetActive("btn")

	host := primitive.NewFlexible(1, tabs.Node())
	col := primitive.Column(primitive.NewText("title"), host)
	col.CrossAlign = core.CrossStretch
	root := primitive.NewBox(col)
	root.Width, root.Height = 800, 600
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})

	// Click second tab via center of absolute bounds
	var pressables []*primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if p, ok := n.(*primitive.Pressable); ok && p.Click != nil {
			abs := core.AbsoluteBounds(p)
			if abs.Max.X <= 160 && abs.Width() > 10 {
				pressables = append(pressables, p)
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(tabs.Node())
	if len(pressables) < 2 {
		t.Fatalf("pressables=%d", len(pressables))
	}
	abs := core.AbsoluteBounds(pressables[1])
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	fmt.Printf("click %.1f,%.1f abs=%v\n", x, y, abs)

	// Force layout dirty flags before click path that rebuilds
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	fmt.Printf("active=%q dirty=%v\n", tabs.Active, tree.Dirty())
	fmt.Printf("root.needsLayout=%v flex.needsLayout=%v\n", root.Base().NeedsLayout(), tabs.Node().Base().NeedsLayout())

	// Must layout after switch
	tree.Layout(core.Size{Width: 800, Height: 600})
	pressables = nil
	walk(tabs.Node())
	for i, p := range pressables {
		fmt.Printf("after[%d] size=%v abs=%v\n", i, p.Size(), core.AbsoluteBounds(p))
	}
	if tabs.Active != "input" {
		t.Fatalf("active=%q", tabs.Active)
	}
	if pressables[0].Size().Width < 100 {
		t.Fatalf("tab not laid out after switch: %v", pressables[0].Size())
	}
}
