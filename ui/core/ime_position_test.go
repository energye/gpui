package core_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/primitive"
)

// F5: after IME / focus, tree reports caret position to host via OnIMEPosition.
func TestIMEPositionCallback(t *testing.T) {
	ed := primitive.NewEditableText()
	ed.Width, ed.Height = 120, 32
	ed.SetValue("hi")
	root := primitive.NewBox(ed)
	root.Width, root.Height = 120, 32
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 120, Height: 32})

	var lastX, lastY float64
	var n int
	tree.SetOnIMEPosition(func(x, y float64) {
		lastX, lastY = x, y
		n++
	})
	tree.SetFocus(ed)
	if n < 1 {
		t.Fatal("SetFocus should update IME position")
	}
	// Caret near start of text
	if lastX < 0 || lastY < 0 {
		t.Fatalf("ime pos negative: %v,%v", lastX, lastY)
	}

	// IME preedit should refresh position
	n0 := n
	tree.DispatchIME(&core.IMECompositionEvent{Text: "ni", End: false})
	if n <= n0 {
		t.Fatal("DispatchIME should refresh IME position")
	}

	// Headless path: app-level bridge tested via platform helper
	h := platform.NewHeadless(100, 80)
	if !h.Caps().Has(platform.CapIME) {
		t.Fatal("Headless CapIME")
	}
	_ = lastX
	_ = lastY
}

func TestLinuxHostNoCapIMEDocumented(t *testing.T) {
	// Compile-time/doc contract: CapIME only when host advertises it.
	// Headless has it; we only assert Headless here (Linux needs DISPLAY).
	h := platform.NewHeadless(10, 10)
	if !h.Caps().Has(platform.CapIME) {
		t.Fatal("Headless must keep CapIME for CI composition")
	}
}
