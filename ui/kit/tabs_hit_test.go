package kit_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Reproduces gallery: left tabs + Flexible host + button in panel.
func TestTabsContentButtonHitAndClick(t *testing.T) {
	clicks := 0
	btn := kit.NewButton("ClickMe")
	btn.SetOnClick(func() { clicks++ })

	panel := primitive.Column(btn.Node())
	panel.Padding = primitive.All(12)
	panel.CrossAlign = core.CrossStart

	tabs := kit.NewTabs(
		kit.MenuItem{Key: "btn", Label: "Button"},
		kit.MenuItem{Key: "other", Label: "Other"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("btn", panel)
	tabs.SetContent("other", primitive.NewText("other"))
	tabs.SetActive("btn")

	host := primitive.NewFlexible(1, tabs.Node())
	col := primitive.Column(primitive.NewText("title"), host)
	col.CrossAlign = core.CrossStretch
	root := primitive.NewBox(col)
	root.Width, root.Height = 1024, 768

	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 1024, Height: 768})

	// Find button pressable absolute bounds
	var target *primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			// Prefer button root (has FocusRingRadius or label via tree)
			abs := core.AbsoluteBounds(p)
			if abs.Width() > 40 && abs.Min.X > 160 { // in content area
				if target == nil || abs.Min.Y < core.AbsoluteBounds(target).Min.Y {
					target = p
				}
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)

	if target == nil {
		// dump content side
		fmt.Println("no pressable in content; dump:")
		var dump func(core.Node, int)
		dump = func(n core.Node, d int) {
			if n == nil || d > 6 {
				return
			}
			abs := core.AbsoluteBounds(n)
			fmt.Printf("%*s%s size=%v abs=%v hit=%v\n", d*2, "", n.TypeID(), n.Base().Size(), abs, n.Base().Hit)
			for _, c := range n.Children() {
				dump(c, d+1)
			}
		}
		dump(tabs.Node(), 0)
		t.Fatal("button pressable not found in content")
	}
	abs := core.AbsoluteBounds(target)
	fmt.Printf("button abs=%v size=%v\n", abs, target.Size())
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2

	hit := tree.HitTest(core.Point{X: x, Y: y})
	fmt.Printf("hit(%.0f,%.0f)=%T\n", x, y, hit)
	if hit != target {
		t.Fatalf("hit=%T want button pressable", hit)
	}

	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: x, Y: y})
	if !target.State.Hovered {
		t.Fatal("button not hovered after move")
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if clicks != 1 {
		t.Fatalf("clicks=%d want 1", clicks)
	}
}
