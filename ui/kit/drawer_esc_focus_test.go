package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestDrawer_EscapeClosesAndOnClose(t *testing.T) {
	d := kit.NewDrawer("D")
	d.SetContent(kit.NewText("body").Node())
	d.Viewport = core.Size{Width: 800, Height: 600}
	var closed bool
	d.OnClose = func() { closed = true }

	bg := primitive.NewPressable(primitive.NewText("bg"))
	root := primitive.Column(bg, d.Node())
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.SetFocus(bg)

	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Overlays().Len() < 1 {
		t.Fatal("drawer overlay missing")
	}
	if tree.Focus() == bg {
		t.Fatal("focus should enter drawer trap")
	}

	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if d.Open {
		t.Fatal("Escape should close drawer")
	}
	if !closed {
		t.Fatal("OnClose not called")
	}
	if tree.Focus() != bg {
		t.Fatalf("focus restore failed: %v", tree.Focus())
	}
}

func TestDrawer_TabStaysInTrap(t *testing.T) {
	d := kit.NewDrawer("D")
	d.SetContent(kit.NewText("body").Node())
	d.Viewport = core.Size{Width: 800, Height: 600}
	bg := primitive.NewPressable(primitive.NewText("bg"))
	root := primitive.Column(bg, d.Node())
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.SetFocus(bg)
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})

	for i := 0; i < 4; i++ {
		tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Tab"})
		if tree.Focus() == bg {
			t.Fatalf("Tab %d escaped onto background", i)
		}
		if d.Scope == nil || !d.Scope.ContainsFocus(tree.Focus()) {
			t.Fatalf("Tab %d left drawer scope", i)
		}
	}
}

func TestDrawer_MaskBlocksBackgroundHit(t *testing.T) {
	d := kit.NewDrawer("D")
	d.Viewport = core.Size{Width: 800, Height: 600}
	d.SetContent(kit.NewText("body").Node())
	tree := core.NewTree(d.Node())
	tree.Layout(core.Size{Width: 800, Height: 600})
	d.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})

	// Left side (empty when drawer is right) should hit mask.
	hit := tree.HitTest(core.Point{X: 20, Y: 300})
	found := false
	for n := hit; n != nil; n = n.Parent() {
		if _, ok := n.(*primitive.Mask); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want mask hit on free area, got %v TypeID=%q", hit, func() string {
			if hit == nil {
				return "nil"
			}
			return hit.TypeID()
		}())
	}
}

func TestTour_EscapeCloses(t *testing.T) {
	tour := kit.NewTour(kit.TourStep{Title: "S1", Body: "b1"})
	tour.Viewport = core.Size{Width: 640, Height: 480}
	var closed bool
	tour.OnClose = func() { closed = true }
	tree := core.NewTree(tour.Node())
	tree.Layout(core.Size{Width: 640, Height: 480})
	tour.SetOpen(true)
	tree.Layout(core.Size{Width: 640, Height: 480})
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if tour.Open {
		t.Fatal("Escape should end tour")
	}
	if !closed {
		t.Fatal("OnClose expected")
	}
}

func TestOverlayZOrderConstants(t *testing.T) {
	if !(kit.OverlayZDrawer < kit.OverlayZModal && kit.OverlayZModal < kit.OverlayZMessage && kit.OverlayZMessage < kit.OverlayZTour) {
		t.Fatalf("z ladder: drawer=%d modal=%d message=%d tour=%d",
			kit.OverlayZDrawer, kit.OverlayZModal, kit.OverlayZMessage, kit.OverlayZTour)
	}
	m := kit.NewModal("m")
	d := kit.NewDrawer("d")
	if m.Portal.ZOrder != kit.OverlayZModal || d.Portal.ZOrder != kit.OverlayZDrawer {
		t.Fatal("portal ZOrder not using constants")
	}
}
