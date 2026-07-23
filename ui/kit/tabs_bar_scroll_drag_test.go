package kit

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Gallery left Tabs + ink animation: thumb MAIN height must stay fixed while dragging.
func TestTabsLeftRailThumbHeightStableDuringDrag(t *testing.T) {
	var items []MenuItem
	for i := 0; i < 45; i++ {
		items = append(items, MenuItem{Key: fmt.Sprintf("k%d", i), Label: fmt.Sprintf("Tab %02d long", i)})
	}
	items = append([]MenuItem{
		{Key: "cat", Label: "General", Disabled: true},
		{Key: "div", Label: "-", Divider: true},
	}, items...)

	tabs := NewTabs(items...)
	tabs.SetPosition(TabLeft)
	tabs.SetTabWidth(168)
	tabs.SetTabItemHeight(36)
	tabs.SetInkSize(3)
	tabs.SetInkAnimated(true)
	for _, it := range items {
		if it.Selectable() {
			tabs.SetContent(it.Key, NewText(it.Label).Node())
		}
	}
	tabs.SetActive(tabs.FirstSelectableKey())

	title := NewText("title").Node()
	status := NewText("status").Node()
	tabsHost := primitive.NewFlexible(1, tabs.Node())
	col := primitive.Column(title, tabsHost, status)
	col.Gap = 12
	col.Padding = primitive.All(16)
	root := primitive.NewBox(col)
	root.Width, root.Height = 1024, 768
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 1024, Height: 768})
	tabs.AttachTicker(tree)

	// Start ink animation (old path MarkNeedsLayout every Tick).
	for _, it := range items {
		if it.Selectable() && it.Key != tabs.Active {
			tabs.SetActive(it.Key)
			break
		}
	}
	tree.Layout(core.Size{Width: 1024, Height: 768})

	sv := tabs.barScroll
	if sv == nil || !sv.OverflowY() {
		t.Fatalf("need overflow ContentH=%v size=%v", sv.ContentH, sv.Size())
	}
	h0 := sv.ThumbMainLength(true)
	if h0 < 8 {
		t.Fatalf("thumb too small h=%v", h0)
	}
	abs := core.AbsoluteBounds(sv)
	gutter := sv.Scrollbar().GutterThickness()
	if gutter < 8 {
		gutter = 8
	}
	// Start near top of track (ScrollY=0 → thumb near top)
	downX := abs.Min.X + sv.Size().Width - gutter/2
	downY := abs.Min.Y + 12
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	if !sv.Dragging() {
		// Try center of first thumb estimate
		downY = abs.Min.Y + h0/2
		tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: downX, Y: downY, Button: core.ButtonLeft})
	}
	if !sv.Dragging() {
		t.Fatal("expected drag start on left rail scrollbar")
	}
	thumb0 := sv.ThumbMainLength(true)
	content0 := sv.ContentH

	var heights, contents, scrolls []float64
	for i := 0; i <= 40; i++ {
		frac := float64(i) / 40
		tree.DispatchPointer(&core.PointerEvent{
			Type: core.PointerMove, X: downX, Y: downY + frac*450, Button: core.ButtonLeft,
		})
		_ = tabs.Tick(0.016)
		tree.Frame(&core.PaintContext{}, core.Size{Width: 1024, Height: 768})
		heights = append(heights, sv.ThumbMainLength(true))
		contents = append(contents, sv.ContentH)
		scrolls = append(scrolls, sv.ScrollY)
	}
	for i, h := range heights {
		if h != thumb0 {
			t.Fatalf("Tabs bar thumb height thrash i=%d h=%.3f want %.3f heights=%v contents=%v",
				i, h, thumb0, heights, contents)
		}
		if contents[i] != content0 {
			t.Fatalf("ContentH thrash: %v", contents)
		}
	}
	for i := 1; i < len(scrolls); i++ {
		if scrolls[i] < scrolls[i-1]-0.01 {
			t.Fatalf("ScrollY backwards: %v", scrolls)
		}
	}
	if scrolls[len(scrolls)-1] < 40 {
		t.Fatalf("scroll barely moved (drag?): last=%v ContentH0=%v", scrolls[len(scrolls)-1], content0)
	}
}
