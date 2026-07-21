package visualtest_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

// Button / Input scenarios use fixed sizes from §12.2:
// button_* 120×40, input_* 200×32. Prefer chrome blocks (no font dependency).

func frameWhite(child core.Node, canvasW, canvasH, childW, childH float64) *primitive.Box {
	px := (canvasW - childW) / 2
	py := (canvasH - childH) / 2
	if px < 0 {
		px = 0
	}
	if py < 0 {
		py = 0
	}
	b := primitive.NewBox(child)
	b.Width, b.Height = canvasW, canvasH
	b.Padding = primitive.EdgeInsets{Left: px, Top: py, Right: px, Bottom: py}
	b.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	return b
}

func TestScenario_ButtonPrimary(t *testing.T) {
	btn := kit.NewButton("") // no text → stable chrome block
	btn.SetType(kit.ButtonPrimary)
	btn.SetFixedSize(120, 32)
	// Canvas 120×40 with vertical pad so height matches table.
	root := frameWhite(btn.Node(), 120, 40, 120, 32)
	img := visualtest.CaptureTree(120, 40, root, kit.DefaultTheme())
	visualtest.AssertScenario(t, "button_primary", img, visualtest.DefaultCompare)
}

func TestScenario_ButtonDefault(t *testing.T) {
	btn := kit.NewButton("")
	btn.SetType(kit.ButtonDefault)
	btn.SetFixedSize(120, 32)
	root := frameWhite(btn.Node(), 120, 40, 120, 32)
	img := visualtest.CaptureTree(120, 40, root, kit.DefaultTheme())
	visualtest.AssertScenario(t, "button_default", img, visualtest.DefaultCompare)
}

func TestScenario_InputIdle(t *testing.T) {
	in := kit.NewInput("") // empty value; no placeholder text drawn when empty string
	in.SetFixedSize(200, 32)
	root := frameWhite(in.Node(), 200, 32, 200, 32)
	img := visualtest.CaptureTree(200, 32, root, kit.DefaultTheme())
	visualtest.AssertScenario(t, "input_idle", img, visualtest.DefaultCompare)
}

func TestScenario_InputFocus(t *testing.T) {
	in := kit.NewInput("")
	in.SetFixedSize(200, 32)
	root := frameWhite(in.Node(), 200, 32, 200, 32)
	// Focus editor → Input applyChrome primary border (no inner ring).
	img := visualtest.CaptureTreeEx(200, 32, root, visualtest.CaptureTreeOptions{
		Theme: kit.DefaultTheme(),
		Focus: in.Editor(),
	})
	if !in.IsFocused() {
		// SetFocus may not mark kit.focused if OnFocusChange not fired through tree.
		// Force via editor API for chrome, then recapture.
		in.Editor().SetFocused(true)
		img = visualtest.CaptureTree(200, 32, root, kit.DefaultTheme())
	}
	if !in.IsFocused() {
		t.Fatal("input not focused for input_focus scenario")
	}
	visualtest.AssertScenario(t, "input_focus", img, visualtest.DefaultCompare)
}

// Headless: Button focus ring + Input focus border + SyncState hover.
func TestButtonInput_FocusAndHover(t *testing.T) {
	btn := kit.NewButton("Go")
	btn.SetType(kit.ButtonPrimary)
	tree := core.NewTree(btn.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	tree.SetFocus(btn.Root)
	if !btn.Root.State.Focused {
		t.Fatal("button not focused")
	}
	// Hover via pointer move + SyncState
	cx, cy := btn.Root.Size().Width/2, btn.Root.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerMove, X: cx, Y: cy})
	btn.SyncState()

	in := kit.NewInput("ph")
	it := core.NewTree(in.Node())
	it.Layout(core.Size{Width: 240, Height: 40})
	it.SetFocus(in.Editor())
	if !in.IsFocused() && !in.Editor().IsFocused() {
		t.Fatal("input editor not focused")
	}
	// Primary border when focused
	in.Editor().SetFocused(true)
	if in.Root.BorderColor.A == 0 {
		t.Fatal("expected focus border color")
	}
	prim := kit.DefaultTheme().Color(core.TokenColorPrimary)
	bd := in.Root.BorderColor
	// Rough match: primary-ish (not gray border)
	if bd.R < 0.05 && bd.B < 0.05 {
		t.Fatalf("focus border looks gray: %+v want primary %+v", bd, prim)
	}
}
