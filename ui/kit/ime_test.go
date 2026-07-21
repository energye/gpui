package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

// C4: composition sequence via platform.Dispatch (Headless CapIME path).
// Sequence: preedit "ni" → preedit "nihao" → End clear → TextInput commit "你好" → backspace.
func TestIME_CompositionSequence_Headless(t *testing.T) {
	h := platform.NewHeadless(320, 80)
	defer h.Close()
	if !h.Caps().Has(platform.CapIME) {
		t.Fatal("Headless must advertise CapIME for degraded IME tests")
	}

	in := kit.NewInput("")
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())

	// preedit
	h.InjectIME("ni", false)
	for _, ev := range h.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if in.Editor().Preedit() != "ni" {
		t.Fatalf("preedit=%q want ni", in.Editor().Preedit())
	}

	h.InjectIME("nihao", false)
	for _, ev := range h.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if in.Editor().Preedit() != "nihao" {
		t.Fatalf("preedit=%q want nihao", in.Editor().Preedit())
	}

	// End without commit text → clear preedit
	h.InjectIME("", true)
	for _, ev := range h.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if in.Editor().Preedit() != "" {
		t.Fatalf("preedit after End=%q", in.Editor().Preedit())
	}
	if in.Value != "" {
		t.Fatalf("value should still be empty, got %q", in.Value)
	}

	// Commit via TextInput
	h.InjectText("你好")
	for _, ev := range h.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if in.Value != "你好" {
		t.Fatalf("value=%q want 你好", in.Value)
	}

	// Backspace removes one rune
	h.InjectKey("Backspace", true)
	for _, ev := range h.PumpEvents() {
		platform.Dispatch(tree, ev)
	}
	if in.Value != "你" {
		t.Fatalf("after bs value=%q want 你", in.Value)
	}
}

// End with Text in the same event commits (host-dependent convenience path).
func TestIME_EndWithCommitText(t *testing.T) {
	ed := primitive.NewEditableText()
	tree := core.NewTree(ed)
	tree.Layout(core.Size{Width: 100, Height: 30})
	tree.SetFocus(ed)
	tree.DispatchIME(&core.IMECompositionEvent{Text: "zhong"})
	if ed.Preedit() != "zhong" {
		t.Fatal(ed.Preedit())
	}
	tree.DispatchIME(&core.IMECompositionEvent{Text: "中", End: true})
	if ed.Preedit() != "" {
		t.Fatalf("preedit=%q", ed.Preedit())
	}
	if ed.Value != "中" {
		t.Fatalf("value=%q", ed.Value)
	}
}

// C3: caret geometry + optional SetIMEPosition on Headless.
func TestIME_CaretPosition_SetIMEPosition(t *testing.T) {
	h := platform.NewHeadless(200, 40)
	defer h.Close()
	ed := primitive.NewEditableText()
	ed.SetValue("ab")
	tree := core.NewTree(ed)
	tree.Layout(core.Size{Width: 200, Height: 40})
	tree.SetFocus(ed)
	lx, ly := ed.CaretLocalPos()
	abs := core.AbsoluteOffset(ed)
	// Forward to host if CapIME/IMEPositioner available.
	ok := platform.SetIMEPositionIfSupported(h, abs.X+lx, abs.Y+ly)
	if !ok {
		t.Fatal("Headless IMEPositioner expected")
	}
	x, y, n := h.LastIMEPosition()
	if n != 1 || x != abs.X+lx || y != abs.Y+ly {
		t.Fatalf("pos=(%v,%v) n=%d abs=(%v,%v) local=(%v,%v)", x, y, n, abs.X, abs.Y, lx, ly)
	}
}

// Document Linux CapIME degradation: true-window Caps (when constructible) omit CapIME.
// We only assert the contract string for Headless vs documented LinuxHost Caps bits.
func TestIME_LinuxCapsContract_Documented(t *testing.T) {
	// LinuxHost Caps bits (no CapIME) — mirrored from linux_host.go Caps().
	linuxCaps := platform.CapWindow | platform.CapPointer | platform.CapKeyboard |
		platform.CapTextInput | platform.CapPresent | platform.CapSurfaceLifecycle | platform.CapCursor
	if linuxCaps.Has(platform.CapIME) {
		t.Fatal("documented LinuxHost caps must not include CapIME until XIM is wired")
	}
	if !platform.HeadlessCaps.Has(platform.CapIME) {
		t.Fatal("Headless is the injectable CapIME path")
	}
}
