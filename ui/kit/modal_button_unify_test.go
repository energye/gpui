package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Modal footer must reuse kit.Button and mount into Tree so hover paints like Button tab.
func TestModalFooterButtonsReuseKitButtonAndMount(t *testing.T) {
	const W, H = 800.0, 600.0
	modal := kit.NewModal("Confirm")
	modal.SetContent(kit.NewText("body").Node())
	modal.Viewport = core.Size{Width: W, Height: H}

	root := primitive.NewBox(modal.Node())
	root.Width, root.Height = W, H
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: W, Height: H})

	modal.SetOpen(true)
	tree.Layout(core.Size{Width: W, Height: H})
	if tree.Overlays().Len() < 1 {
		t.Fatal("no overlay")
	}

	// Find pressables in overlay — both should have tree set (mounted).
	var pressables []*primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			pressables = append(pressables, p)
			if p.Base().Tree() != tree {
				t.Errorf("pressable %q not mounted on tree (hover/paint will not schedule frames)", p.Base().Label)
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	for _, e := range tree.Overlays().Entries() {
		walk(e.Node)
	}
	if len(pressables) < 2 {
		t.Fatalf("want >=2 footer buttons, got %d", len(pressables))
	}

	// Hover chrome: MarkNeedsPaint must dirty the tree when mounted.
	tree.Layout(core.Size{Width: W, Height: H}) // clear dirty from layout
	// Force a clean state then paint-dirty a pressable.
	// After layout full paint may have cleared; mark again.
	p0 := pressables[0]
	// simulate hover state change path
	p0.State.Hovered = true
	p0.MarkNeedsPaint()
	if !tree.Dirty() {
		t.Fatal("tree not dirty after overlay pressable MarkNeedsPaint — mount missing")
	}
}
