package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// F3: kit floating shells use AnchoredPopup.DismissOnOutside (default true).
// Pointer-down on empty space closes; pointer-down on trigger does not.

func mountSelect(t *testing.T) (*kit.Select, *core.Tree) {
	t.Helper()
	sel := kit.NewSelect("pick",
		kit.SelectOption{Value: "a", Label: "Alpha"},
		kit.SelectOption{Value: "b", Label: "Beta"},
	)
	sel.Viewport = core.Size{Width: 400, Height: 300}
	// Large box so outside clicks hit the root, not the trigger.
	bg := primitive.NewBox(sel.Node())
	bg.Width, bg.Height = 400, 300
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: 400, Height: 300})
	return sel, tree
}

func TestSelect_OutsideDismiss(t *testing.T) {
	sel, tree := mountSelect(t)
	sel.SetOpen(true)
	// Layout after open so outside-dismiss registers on the tree.
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !sel.Open {
		t.Fatal("want open")
	}
	if !sel.Popup().Open {
		t.Fatal("popup should be open")
	}
	// Far corner — outside trigger and panel.
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 380, Y: 280})
	if sel.Popup().Open {
		t.Fatal("outside pointer should dismiss Select popup")
	}
	if sel.Open {
		t.Fatal("Select.Open should sync false via OnDismiss")
	}
}

func TestSelect_OutsideDismiss_KeepsTrigger(t *testing.T) {
	sel, tree := mountSelect(t)
	sel.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})

	// Click center of trigger (Select root pressable).
	abs := core.AbsoluteBounds(sel.Root)
	cx := (abs.Min.X + abs.Max.X) / 2
	cy := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: cx, Y: cy})
	if !sel.Popup().Open {
		t.Fatal("click on trigger must not outside-dismiss (toggle is separate click)")
	}
}

func TestDropdown_OutsideDismiss(t *testing.T) {
	dd := kit.NewDropdown("Menu",
		kit.MenuItem{Key: "1", Label: "One"},
		kit.MenuItem{Key: "2", Label: "Two"},
	)
	dd.Viewport = core.Size{Width: 400, Height: 300}
	bg := primitive.NewBox(dd.Node())
	bg.Width, bg.Height = 400, 300
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: 400, Height: 300})
	dd.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !dd.Popup().Open {
		t.Fatal("want open")
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 350, Y: 280})
	if dd.Popup().Open {
		t.Fatal("outside should dismiss Dropdown")
	}
	if dd.Open {
		t.Fatal("Dropdown.Open should sync false")
	}
}

func TestPopover_OutsideDismiss(t *testing.T) {
	body := primitive.NewText("panel body")
	trig := primitive.NewPressable(primitive.NewText("open"))
	po := kit.NewPopover(trig, body)
	po.Viewport = core.Size{Width: 400, Height: 300}
	bg := primitive.NewBox(po.Node())
	bg.Width, bg.Height = 400, 300
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: 400, Height: 300})
	po.SetOpen(true)
	tree.Layout(core.Size{Width: 400, Height: 300})
	if !po.Popup.Open {
		t.Fatal("want open")
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 380, Y: 280})
	if po.Popup.Open {
		t.Fatal("outside should dismiss Popover")
	}
	if po.Open {
		t.Fatal("Popover.Open should sync false")
	}
}

func TestSelect_DismissOnOutsideDefault(t *testing.T) {
	sel := kit.NewSelect("x", kit.SelectOption{Value: "1", Label: "One"})
	// Ensure rebuild created popup with default DismissOnOutside.
	_ = sel.Node()
	if sel.Popup() == nil {
		t.Fatal("nil popup")
	}
	if !sel.Popup().DismissOnOutside {
		t.Fatal("Select popup should default DismissOnOutside=true")
	}
}
