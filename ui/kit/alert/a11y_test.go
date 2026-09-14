package alert_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// ALT-18 is N/A by spec: antd Alert has no disabled. The case asserts the
// exemption instead of inventing a disabled look.
func TestAlert_PRD_ALT18_DisabledNA(t *testing.T) {
	a := alert.NewAlert("no disabled in spec")
	// No SetDisabled API on purpose; closable still works and root never
	// takes focus (decorative display, interactive close only).
	if a.Focusable() {
		t.Fatal("alert root must not steal Focus")
	}
	if a.Role() != "alert" {
		t.Fatalf("Role=%q want alert", a.Role())
	}
	if a.AriaLabel() != "" {
		t.Fatalf("default Aria empty, got %q", a.AriaLabel())
	}
}

// Keyboard + focus main path: close is Tab reachable, Enter/Space closes.
func TestAlert_PRD_ALT19_KeyboardFocus(t *testing.T) {
	a := alert.NewAlert("focus me")
	a.SetClosable(true)
	a.SetCloseAria("Close")
	mgr := focus.NewManager()
	n := a.CloseFocusNode()
	mgr.Register(n)
	if !a.CloseFocusable() {
		t.Fatal("close Focusable")
	}
	if n.TabIndex < 0 || !n.Enabled {
		t.Fatal("close node must be Tab reachable")
	}
	if !mgr.RequestFocus(n) || !n.HasFocus() {
		t.Fatal("RequestFocus")
	}
	if a.CloseRole() != "button" {
		t.Fatalf("Close Role=%q", a.CloseRole())
	}
	// Enter via manager activation closes.
	if !mgr.HandleKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true}) {
		t.Fatal("Enter should be consumed")
	}
	if a.Visible() {
		t.Fatal("Enter must close")
	}
	// Space path on a fresh alert.
	b := alert.NewAlert("space")
	b.SetClosable(true)
	mgr2 := focus.NewManager()
	nb := b.CloseFocusNode()
	mgr2.Register(nb)
	mgr2.RequestFocus(nb)
	if !mgr2.HandleKey(focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true}) {
		t.Fatal("Space should be consumed")
	}
	if b.Visible() {
		t.Fatal("Space must close")
	}
	// Direct key helper parity.
	c := alert.NewAlert("direct")
	c.SetClosable(true)
	if !c.PressCloseKey("Enter") || c.Visible() {
		t.Fatal("PressCloseKey Enter")
	}
}

// Four rulers: 44px target, contrast, named readability, reader tree.
func TestAlert_A11y_TargetContrastSemantics(t *testing.T) {
	a := alert.NewAlert("A11y title")
	a.SetDescription("helper")
	a.SetShowIcon(true)
	a.SetClosable(true)
	a.SetCloseAria("Close alert")
	a.SetAriaLabel("Network warning")
	// 1. Minimum 44px close target (hit == layout == paint).
	if a.CloseHitSize() < 44-1e-9 {
		t.Fatalf("close hit=%v want >=44", a.CloseHitSize())
	}
	cn := a.CloseNode()
	sz := cn.Layout(rendering.Loose(100, 100))
	if math.Abs(sz.Width-44) > 0.5 || math.Abs(sz.Height-44) > 0.5 {
		t.Fatalf("CloseNode=%v want 44x44", sz)
	}
	// 2. Text contrast vs shell (WCAG AA 4.5 for normal text).
	tok := theme.Default.Current()
	fg := render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: 1}
	if r := contrast(fg, a.Background()); r < 4.5 {
		t.Fatalf("contrast=%v want >=4.5", r)
	}
	// 3. Named readability.
	if a.AriaLabel() != "Network warning" || a.CloseAria() != "Close alert" {
		t.Fatalf("Aria labels %q/%q", a.AriaLabel(), a.CloseAria())
	}
	// 4. Reader tree: alert root with button child, display never Focuses itself.
	tree := a.Semantics()
	if tree == nil || string(tree.Role) != "alert" {
		t.Fatalf("root Role=%+v", tree)
	}
	foundBtn := false
	for _, ch := range tree.Children {
		if string(ch.Role) == "button" && ch.Label == "Close alert" && ch.Focusable {
			foundBtn = true
		}
	}
	if !foundBtn {
		t.Fatalf("reader tree missing close button: %+v", tree.Children)
	}
	if a.Focusable() {
		t.Fatal("display must not grab Focus")
	}
	if !a.CloseFocusNode().Enabled {
		t.Fatal("close Focus node must be enabled")
	}
}

func lumChan(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func luminance(c render.RGBA) float64 {
	return 0.2126*lumChan(c.R) + 0.7152*lumChan(c.G) + 0.0722*lumChan(c.B)
}

func contrast(fg, bg render.RGBA) float64 {
	l1, l2 := luminance(fg), luminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
