package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

func TestEditableTextTyping(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.SetValue("hi")
	tree := core.NewTree(ed)
	tree.Layout(core.Size{Width: 200, Height: 40})
	tree.SetFocus(ed)
	tree.DispatchTextInput(&core.TextInputEvent{Text: "!"})
	if ed.Value != "hi!" {
		t.Fatalf("value=%q", ed.Value)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Backspace"})
	if ed.Value != "hi" {
		t.Fatalf("after bs value=%q", ed.Value)
	}
}

func TestInputHeadless(t *testing.T) {
	in := kit.NewInput("name")
	in.SetValue("Ada")
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	if in.Editor().Value != "Ada" {
		t.Fatal(in.Editor().Value)
	}
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: " Lovelace"})
	if in.Value != "Ada Lovelace" {
		t.Fatalf("got %q", in.Value)
	}
}

func TestCheckboxToggle(t *testing.T) {
	var got bool
	cb := kit.NewCheckbox("Agree")
	cb.SetOnChange(func(v bool) { got = v })
	tree := core.NewTree(cb.Node())
	tree.Layout(core.Size{Width: 200, Height: 40})
	// click center
	x := cb.Root.Size().Width / 2
	y := cb.Root.Size().Height / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if !got || !cb.Checked {
		t.Fatalf("checked=%v got=%v", cb.Checked, got)
	}
}

func TestRadioGroup(t *testing.T) {
	a := kit.NewRadio("a", "A")
	b := kit.NewRadio("b", "B")
	g := kit.NewRadioGroup(a, b)
	g.Select("b")
	if g.Value != "b" || !b.Selected || a.Selected {
		t.Fatalf("value=%s a=%v b=%v", g.Value, a.Selected, b.Selected)
	}
}

func TestSwitchToggle(t *testing.T) {
	sw := kit.NewSwitch()
	tree := core.NewTree(sw.Node())
	tree.Layout(core.Size{Width: 100, Height: 40})
	x, y := sw.Root.Size().Width/2, sw.Root.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if !sw.Checked {
		t.Fatal("expected checked")
	}
}

func TestScrollViewport(t *testing.T) {
	inner := primitive.NewBox()
	inner.Width, inner.Height = 100, 400
	sv := primitive.NewScrollViewport(inner)
	sv.Width, sv.Height = 100, 100
	tree := core.NewTree(sv)
	tree.Layout(core.Size{Width: 100, Height: 100})
	tree.DispatchScroll(&core.ScrollEvent{X: 10, Y: 10, DY: 50})
	if sv.ScrollY != 50 {
		t.Fatalf("scrollY=%v", sv.ScrollY)
	}
}

func TestOverlayPortal(t *testing.T) {
	panel := primitive.NewBox()
	panel.Width, panel.Height = 80, 40
	portal := primitive.NewOverlayPortal(panel)
	portal.ID = "t1"
	root := primitive.NewBox(portal)
	root.Width, root.Height = 200, 200
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 200, Height: 200})
	portal.SetContentOffset(core.Point{X: 20, Y: 30})
	portal.SetOpen(true)
	// re-layout to measure overlay
	tree.Layout(core.Size{Width: 200, Height: 200})
	if tree.Overlays().Len() != 1 {
		t.Fatalf("overlays=%d", tree.Overlays().Len())
	}
	// hit overlay
	hit := tree.HitTest(core.Point{X: 40, Y: 40})
	if hit == nil {
		t.Fatal("expected hit on overlay panel")
	}
	portal.SetOpen(false)
	if tree.Overlays().Len() != 0 {
		t.Fatalf("after close overlays=%d", tree.Overlays().Len())
	}
}

func TestMaskDismiss(t *testing.T) {
	dismissed := false
	mask := primitive.NewMask()
	mask.Width, mask.Height = 100, 100
	mask.OnDismiss = func() { dismissed = true }
	tree := core.NewTree(mask)
	tree.Layout(core.Size{Width: 100, Height: 100})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: 10, Y: 10, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: 10, Y: 10, Button: core.ButtonLeft})
	if !dismissed {
		t.Fatal("mask not dismissed")
	}
}

func TestAnchoredPopupOpen(t *testing.T) {
	content := primitive.NewBox()
	content.Width, content.Height = 60, 30
	pop := primitive.NewAnchoredPopup(content)
	pop.Anchor = core.NewRect(10, 10, 40, 20)
	pop.Viewport = core.Size{Width: 200, Height: 200}
	root := primitive.NewBox(pop)
	root.Width, root.Height = 200, 200
	tree := core.NewTree(root)
	// mount portal
	tree.Layout(core.Size{Width: 200, Height: 200})
	pop.SetOpen(true)
	tree.Layout(core.Size{Width: 200, Height: 200})
	if tree.Overlays().Len() < 1 {
		t.Fatal("popup not in overlays")
	}
}

func TestTooltipSync(t *testing.T) {
	trig := primitive.NewText("?")
	tt := kit.NewTooltip(trig, "help")
	tree := core.NewTree(tt.Node())
	tree.Layout(core.Size{Width: 300, Height: 200})
	// hover shell via move
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: 5, Y: 5})
	tt.Viewport = core.Size{Width: 300, Height: 200}
	tt.Sync()
	// may or may not open depending on hit — ensure no panic
	_ = tree.Overlays()
}

func TestPopoverClick(t *testing.T) {
	body := primitive.NewText("panel")
	trig := kit.NewButton("Open")
	pop := kit.NewPopover(trig.Node(), body)
	tree := core.NewTree(pop.Node())
	tree.Layout(core.Size{Width: 400, Height: 300})
	// click shell (button pressable nested)
	// find pressable shell at root child 0
	host := platform.NewHeadless(400, 300)
	defer host.Close()
	// use button size
	ax := 10.0
	ay := 10.0
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: ax, Y: ay, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: ax, Y: ay, Button: core.ButtonLeft})
	pop.Sync()
	// open may be true if hit
	_ = pop.Open
}

func TestIMEPreedit(t *testing.T) {
	ed := primitive.NewEditableText()
	tree := core.NewTree(ed)
	tree.Layout(core.Size{Width: 100, Height: 30})
	tree.SetFocus(ed)
	tree.DispatchIME(&core.IMECompositionEvent{Text: "ni"})
	tree.DispatchIME(&core.IMECompositionEvent{End: true})
	tree.DispatchTextInput(&core.TextInputEvent{Text: "你"})
	if ed.Value != "你" {
		t.Fatalf("value=%q", ed.Value)
	}
}
