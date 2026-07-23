package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestModal_EscapeClosesAndCallsOnCancel(t *testing.T) {
	modal := kit.NewModal("T")
	modal.SetContent(kit.NewText("body").Node())
	modal.Viewport = core.Size{Width: 800, Height: 600}
	var canceled bool
	modal.OnCancel = func() { canceled = true }

	// Background focusable (should not receive keys while modal open).
	bg := primitive.NewPressable(primitive.NewText("bg"))
	root := primitive.Column(bg, modal.Node())
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.SetFocus(bg)

	modal.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})
	if tree.Overlays().Len() < 1 {
		t.Fatal("overlay missing")
	}
	// Focus should move into modal trap.
	if tree.Focus() == bg {
		t.Fatal("focus still on background after open")
	}
	if tree.Focus() == nil {
		t.Fatal("expected focus inside modal")
	}

	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if modal.Open {
		t.Fatal("Escape should close modal")
	}
	if !canceled {
		t.Fatal("OnCancel not called")
	}
	// Focus restored to background.
	if tree.Focus() != bg {
		t.Fatalf("focus restore: got %v want bg", tree.Focus())
	}
}

func TestModal_TabStaysInsideFocusTrap(t *testing.T) {
	modal := kit.NewModal("T")
	modal.SetContent(kit.NewText("body").Node())
	modal.Viewport = core.Size{Width: 800, Height: 600}

	bg := primitive.NewPressable(primitive.NewText("bg"))
	root := primitive.Column(bg, modal.Node())
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 800, Height: 600})
	tree.SetFocus(bg)

	modal.SetOpen(true)
	tree.Layout(core.Size{Width: 800, Height: 600})

	// Tab several times — never land on background.
	for i := 0; i < 6; i++ {
		tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Tab"})
		if tree.Focus() == bg {
			t.Fatalf("Tab %d escaped trap onto background", i)
		}
		if tree.Focus() == nil {
			t.Fatalf("Tab %d cleared focus", i)
		}
		// Focus must stay under FocusScope
		if modal.Scope == nil || !modal.Scope.ContainsFocus(tree.Focus()) {
			t.Fatalf("Tab %d focus left modal scope: %v", i, tree.Focus())
		}
	}
}

func TestModal_EscapeWorksWhenFocusNil(t *testing.T) {
	modal := kit.NewModal("T")
	modal.SetContent(kit.NewText("body").Node())
	modal.Viewport = core.Size{Width: 640, Height: 480}
	tree := core.NewTree(modal.Node())
	tree.Layout(core.Size{Width: 640, Height: 480})
	modal.SetOpen(true)
	tree.Layout(core.Size{Width: 640, Height: 480})
	tree.SetFocus(nil) // adversarial
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if modal.Open {
		t.Fatal("Escape should close even if focus was cleared")
	}
}
