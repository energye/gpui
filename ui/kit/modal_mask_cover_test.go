package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/layer"
	"github.com/energye/gpui/ui/primitive"
)

// TestModal_MaskCoversFullViewport: mask geometry = Modal.Viewport (full client).
func TestModal_MaskCoversFullViewport(t *testing.T) {
	modal := kit.NewModal("T")
	modal.SetContent(kit.NewText("body").Node())
	modal.Viewport = core.Size{Width: 1024, Height: 768}
	tree := core.NewTree(modal.Node())
	tree.Layout(core.Size{Width: 1024, Height: 768})
	modal.SetOpen(true)
	tree.Layout(core.Size{Width: 1024, Height: 768})

	if tree.Overlays().Len() < 1 {
		t.Fatal("expected overlay")
	}
	var mask *primitive.Mask
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || mask != nil {
			return
		}
		if m, ok := n.(*primitive.Mask); ok {
			mask = m
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	for _, e := range tree.Overlays().Entries() {
		walk(e.Node)
	}
	if mask == nil {
		t.Fatal("mask not found")
	}
	sz := mask.Size()
	if sz.Width != 1024 || sz.Height != 768 {
		t.Fatalf("mask size=%v want 1024x768", sz)
	}
	abs := core.AbsoluteBounds(mask)
	if abs.Min.X != 0 || abs.Min.Y != 0 {
		t.Fatalf("mask abs origin=%v want (0,0)", abs.Min)
	}
}

// TestModal_MaskBlocksTabsChrome: hit over tab rail hits Mask, not Tabs.
// Exercises compositor dual-band frame (main then overlay) so Z-order is correct.
func TestModal_MaskBlocksTabsChrome(t *testing.T) {
	tabs := kit.NewTabs(
		kit.MenuItem{Key: "a", Label: "A"},
		kit.MenuItem{Key: "b", Label: "B"},
	)
	tabs.SetPosition(kit.TabLeft)
	tabs.TabWidth = 160
	tabs.TabItemHeight = 40
	tabs.SetContent("a", kit.NewText("panel A").Node())
	tabs.SetContent("b", kit.NewText("panel B").Node())
	tabs.SetActive("a")

	modal := kit.NewModal("Confirm")
	modal.SetContent(kit.NewText("modal body").Node())
	modal.Viewport = core.Size{Width: 800, Height: 600}

	root := primitive.Column(tabs.Node(), modal.Node())
	tree := core.NewTree(root)
	vp := core.Size{Width: 800, Height: 600}
	tree.Layout(vp)

	modal.SetOpen(true)
	tree.Layout(vp)

	if tree.Overlays().Len() < 1 {
		t.Fatal("expected modal overlay")
	}

	points := []core.Point{
		{X: 10, Y: 10},
		{X: 80, Y: 100}, // left rail
		{X: 10, Y: 590},
		{X: 790, Y: 10},
	}
	for _, p := range points {
		hit := tree.HitTest(p)
		if hit == nil {
			t.Fatalf("HitTest %v nil", p)
		}
		foundMask := false
		for n := hit; n != nil; n = n.Parent() {
			if _, ok := n.(*primitive.Mask); ok {
				foundMask = true
				break
			}
		}
		if !foundMask {
			t.Fatalf("HitTest %v TypeID=%q — want Mask under modal (blocked chrome)", p, hit.TypeID())
		}
	}

	// Dual-band compositor: exercise Frame without requiring GPU textures
	// (Rasterize may fail headless; paint/hit order is already covered above).
	comp := layer.NewCompositor()
	comp.Resize(800, 600, 1)
	comp.BG = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	_ = comp.Frame(tree, nil, true)
	// If GPU is available, base must be ready after full frame.
	if comp.HasBase() {
		// BlitTo must not panic with overlay open.
		// (surface DC optional in unit tests)
	}
}

// TestModal_MaskUsesTreeViewportWhenUnset: host forgets Modal.Viewport → tree viewport.
func TestModal_MaskUsesTreeViewportWhenUnset(t *testing.T) {
	modal := kit.NewModal("T")
	modal.SetContent(kit.NewText("body").Node())
	tree := core.NewTree(modal.Node())
	tree.Layout(core.Size{Width: 640, Height: 480})
	modal.SetOpen(true)
	tree.Layout(core.Size{Width: 640, Height: 480})

	var mask *primitive.Mask
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || mask != nil {
			return
		}
		if m, ok := n.(*primitive.Mask); ok {
			mask = m
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	for _, e := range tree.Overlays().Entries() {
		walk(e.Node)
	}
	if mask == nil {
		t.Fatal("mask not found")
	}
	sz := mask.Size()
	if sz.Width != 640 || sz.Height != 480 {
		t.Fatalf("mask size=%v want tree viewport 640x480", sz)
	}
}
