package visualtest_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	"github.com/energye/gpui/ui/visualtest"
)

// frameIndicator centers a fixed-size indicator on a white canvas.
func frameIndicator(indicator core.Node, canvasW, canvasH, indW, indH float64) *primitive.Box {
	px := (canvasW - indW) / 2
	py := (canvasH - indH) / 2
	if px < 0 {
		px = 0
	}
	if py < 0 {
		py = 0
	}
	b := primitive.NewBox(indicator)
	b.Width, b.Height = canvasW, canvasH
	b.Padding = primitive.EdgeInsets{Left: px, Top: py, Right: px, Bottom: py}
	b.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	return b
}

func assertIndicator(t *testing.T, id string, root core.Node, w, h int) {
	t.Helper()
	img := visualtest.CaptureTree(w, h, root, kit.DefaultTheme())
	if img == nil {
		t.Fatal("nil capture")
	}
	visualtest.AssertScenario(t, id, img, visualtest.DefaultCompare)
}

func TestScenario_CheckboxOff(t *testing.T) {
	cb := kit.NewCheckbox("")
	root := frameIndicator(cb.IndicatorNode(), 32, 32, 16, 16)
	assertIndicator(t, "checkbox_off", root, 32, 32)
}

func TestScenario_CheckboxOn(t *testing.T) {
	cb := kit.NewCheckbox("")
	cb.SetChecked(true)
	root := frameIndicator(cb.IndicatorNode(), 32, 32, 16, 16)
	assertIndicator(t, "checkbox_on", root, 32, 32)
}

func TestScenario_CheckboxIndeterminate(t *testing.T) {
	cb := kit.NewCheckbox("")
	cb.SetIndeterminate(true)
	// Primary chrome for mixed state (applyChrome treats Indeterminate like checked fill).
	root := frameIndicator(cb.IndicatorNode(), 32, 32, 16, 16)
	assertIndicator(t, "checkbox_indeterminate", root, 32, 32)
}

func TestScenario_RadioOff(t *testing.T) {
	r := kit.NewRadio("a", "")
	root := frameIndicator(r.IndicatorNode(), 32, 32, 16, 16)
	assertIndicator(t, "radio_off", root, 32, 32)
}

func TestScenario_RadioOn(t *testing.T) {
	r := kit.NewRadio("a", "")
	r.SetSelected(true)
	root := frameIndicator(r.IndicatorNode(), 32, 32, 16, 16)
	assertIndicator(t, "radio_on", root, 32, 32)
}

func TestScenario_SwitchOff(t *testing.T) {
	sw := kit.NewSwitch()
	// track 44×22 centered on 56×32
	root := frameIndicator(sw.IndicatorNode(), 56, 32, 44, 22)
	assertIndicator(t, "switch_off", root, 56, 32)
}

func TestScenario_SwitchOn(t *testing.T) {
	sw := kit.NewSwitch()
	sw.SetChecked(true)
	root := frameIndicator(sw.IndicatorNode(), 56, 32, 44, 22)
	assertIndicator(t, "switch_on", root, 56, 32)
}

// Headless state: click checkbox/radio/switch without a real window.
func TestIndicators_HeadlessToggle(t *testing.T) {
	cb := kit.NewCheckbox("x")
	tree := core.NewTree(cb.Node())
	tree.Layout(core.Size{Width: 200, Height: 40})
	x, y := cb.Root.Size().Width/2, cb.Root.Size().Height/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if !cb.Checked {
		t.Fatal("checkbox not checked after click")
	}

	a := kit.NewRadio("a", "A")
	b := kit.NewRadio("b", "B")
	g := kit.NewRadioGroup(a, b)
	g.Select("a")
	if !a.Selected || b.Selected {
		t.Fatalf("radio group a=%v b=%v", a.Selected, b.Selected)
	}
	g.Select("b")
	if a.Selected || !b.Selected || g.Value != "b" {
		t.Fatalf("radio group after b: a=%v b=%v v=%s", a.Selected, b.Selected, g.Value)
	}

	sw := kit.NewSwitch()
	st := core.NewTree(sw.Node())
	st.Layout(core.Size{Width: 80, Height: 40})
	sx, sy := sw.Root.Size().Width/2, sw.Root.Size().Height/2
	st.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: sx, Y: sy, Button: core.ButtonLeft})
	st.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: sx, Y: sy, Button: core.ButtonLeft})
	if !sw.Checked {
		t.Fatal("switch not checked after click")
	}
}
