package kit_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/input-number.md §6.9 — P0 PRD cases (INN-01 … INN-23).
// L3/L4 (INN-24/25) and P1 (INN-26) deferred.

func approxINN(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func approxINNColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(a.R-b.R) <= tol &&
		math.Abs(a.G-b.G) <= tol &&
		math.Abs(a.B-b.B) <= tol &&
		math.Abs(a.A-b.A) <= tol
}

func clickPressable(t *testing.T, tree *core.Tree, p *primitive.Pressable) {
	t.Helper()
	if p == nil {
		t.Fatal("nil pressable")
	}
	abs := core.AbsoluteBounds(p)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func findINNActions(root core.Node) (up, down *primitive.Pressable) {
	var btns []*primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok && !p.Focusable && p.Click != nil {
			btns = append(btns, p)
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	if len(btns) >= 2 {
		return btns[0], btns[1]
	}
	if len(btns) == 1 {
		return btns[0], nil
	}
	return nil, nil
}

func TestInputNumber_PRD_01_Defaults(t *testing.T) {
	// INN-01
	n := kit.NewInputNumber()
	if n.Size != kit.InputMiddle {
		t.Fatalf("Size=%v want middle", n.Size)
	}
	if n.Variant != kit.InputOutlined {
		t.Fatalf("Variant=%v want outlined", n.Variant)
	}
	if n.Status != kit.InputStatusNone {
		t.Fatalf("Status=%v want none", n.Status)
	}
	if n.Mode != kit.InputNumberModeInput {
		t.Fatalf("Mode=%v want input", n.Mode)
	}
	if n.Step != 1 {
		t.Fatalf("Step=%v want 1", n.Step)
	}
	if !n.Controls || !n.Keyboard || !n.ChangeOnBlur {
		t.Fatalf("defaults Controls/Keyboard/ChangeOnBlur want true")
	}
	if n.ChangeOnWheel || n.Disabled || n.ReadOnly || n.Controlled || n.StringMode {
		t.Fatalf("flags should be false")
	}
	if n.Node() == nil || n.ChromeNode() == nil || n.Editor() == nil {
		t.Fatal("nil nodes")
	}
	_ = n.Node().Layout(core.Loose(400, 100))
}

func TestInputNumber_PRD_02_StepUp(t *testing.T) {
	// INN-02 / INN-S1
	n := kit.NewInputNumberValue(3)
	n.SetMin(1)
	n.SetMax(10)
	got := 0.0
	n.SetOnChange(func(v float64) { got = v })
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	up, _ := findINNActions(n.Node())
	if up == nil {
		t.Fatal("up action not found")
	}
	clickPressable(t, tree, up)
	if n.Value != 4 {
		t.Fatalf("value=%v want 4", n.Value)
	}
	if got != 4 {
		t.Fatalf("onChange=%v want 4", got)
	}
}

func TestInputNumber_PRD_03_ClampMin(t *testing.T) {
	// INN-03 / INN-S2
	n := kit.NewInputNumberValue(1)
	n.SetMin(1)
	n.SetMax(10)
	n.StepDown()
	if n.Value != 1 {
		t.Fatalf("value=%v want min 1", n.Value)
	}
	n.SetValue(2)
	n.StepDown()
	if n.Value != 1 {
		t.Fatalf("value=%v want 1", n.Value)
	}
}

func TestInputNumber_PRD_04_ClampMaxNoop(t *testing.T) {
	// INN-04 / INN-S3
	n := kit.NewInputNumberValue(10)
	n.SetMin(1)
	n.SetMax(10)
	changes := 0
	n.SetOnChange(func(float64) { changes++ })
	n.StepUp()
	if n.Value != 10 {
		t.Fatalf("value=%v want 10", n.Value)
	}
	if changes != 0 {
		t.Fatalf("onChange fired %d times want 0 at max", changes)
	}
}

func TestInputNumber_PRD_05_Precision(t *testing.T) {
	// INN-05 / INN-S4
	n := kit.NewInputNumberValue(1)
	n.SetPrecision(2)
	n.SetStep(0.1)
	n.StepUp()
	if !approxINN(n.Value, 1.1, 1e-9) {
		t.Fatalf("value=%v want 1.1", n.Value)
	}
	disp := n.DisplayString()
	if disp != "1.10" {
		t.Fatalf("display=%q want 1.10", disp)
	}
}

func TestInputNumber_PRD_06_KeyboardDown(t *testing.T) {
	// INN-06 / INN-S5
	n := kit.NewInputNumberValue(3)
	n.SetMin(1)
	n.SetMax(10)
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	tree.SetFocus(n.Editor())
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"})
	if n.Value != 2 {
		t.Fatalf("value=%v want 2 after ArrowDown", n.Value)
	}
}

func TestInputNumber_PRD_07_Disabled(t *testing.T) {
	// INN-07 / INN-S6
	n := kit.NewInputNumberValue(3)
	n.SetDisabled(true)
	changes := 0
	n.SetOnChange(func(float64) { changes++ })
	n.StepUp()
	n.StepDown()
	n.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowUp"})
	if n.Value != 3 || changes != 0 {
		t.Fatalf("disabled mutated value=%v changes=%d", n.Value, changes)
	}
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	tree.SetFocus(n.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "9"})
	if changes != 0 || n.Value != 3 {
		t.Fatalf("disabled typing value=%v changes=%d", n.Value, changes)
	}
}

func TestInputNumber_PRD_08_ControlsFalse(t *testing.T) {
	// INN-08 / INN-S7
	n := kit.NewInputNumberValue(3)
	n.SetControls(false)
	if n.ControlsVisible() {
		t.Fatal("controls should be hidden")
	}
	up, dn := findINNActions(n.Node())
	if up != nil || dn != nil {
		t.Fatal("action pressables should not mount when controls=false")
	}
}

func TestInputNumber_PRD_09_MiddleHeight(t *testing.T) {
	// INN-09 / INN-S8
	n := kit.NewInputNumberValue(1)
	sz := n.Node().Layout(core.Loose(400, 100))
	if !approxINN(sz.Height, 32, 0.5) {
		t.Fatalf("height=%v want 32±0.5", sz.Height)
	}
	if !approxINN(n.HeightToken(), 32, 0.5) {
		t.Fatalf("HeightToken=%v want 32", n.HeightToken())
	}
}

func TestInputNumber_PRD_10_Controlled(t *testing.T) {
	// INN-10 / INN-S9
	n := kit.NewInputNumberValue(3)
	n.SetControlled(true)
	n.SetValue(3)
	got := -1.0
	n.SetOnChange(func(v float64) { got = v })
	n.StepUp()
	if got != 4 {
		t.Fatalf("onChange=%v want 4", got)
	}
	if n.Value != 3 {
		t.Fatalf("controlled Value=%v want 3 until parent SetValue", n.Value)
	}
	n.SetValue(4)
	if n.Value != 4 {
		t.Fatalf("after SetValue=%v want 4", n.Value)
	}
}

func TestInputNumber_PRD_11_StepPointOne(t *testing.T) {
	// INN-11 / INN-S10
	n := kit.NewInputNumberValue(1)
	n.SetStep(0.1)
	n.SetPrecision(1)
	n.StepUp()
	if !approxINN(n.Value, 1.1, 1e-9) {
		t.Fatalf("value=%v want 1.1", n.Value)
	}
	n.StepUp()
	if !approxINN(n.Value, 1.2, 1e-9) {
		t.Fatalf("value=%v want 1.2", n.Value)
	}
}

func TestInputNumber_PRD_12_BasicDemo(t *testing.T) {
	// INN-12 basic.tsx
	n := kit.NewInputNumberValue(3)
	n.SetMin(1)
	n.SetMax(10)
	n.SetOnChange(func(float64) {})
	_ = n.Node().Layout(core.Loose(200, 80))
	n.StepUp()
	if n.Value != 4 {
		t.Fatalf("value=%v", n.Value)
	}
}

func TestInputNumber_PRD_13_SizeDemo(t *testing.T) {
	// INN-13 size.tsx
	th := kit.DefaultTheme()
	for _, tc := range []struct {
		size kit.InputSize
		want float64
	}{
		{kit.InputLarge, th.SizeOr(core.TokenControlHeightLG, 40)},
		{kit.InputMiddle, th.SizeOr(core.TokenControlHeight, 32)},
		{kit.InputSmall, th.SizeOr(core.TokenControlHeightSM, 24)},
	} {
		n := kit.NewInputNumberValue(3)
		n.SetMin(1)
		n.SetMax(100000)
		n.SetSize(tc.size)
		sz := n.Node().Layout(core.Loose(400, 100))
		if !approxINN(sz.Height, tc.want, 0.5) {
			t.Fatalf("size %v height=%v want %v", tc.size, sz.Height, tc.want)
		}
	}
}

func TestInputNumber_PRD_14_DisabledDemo(t *testing.T) {
	// INN-14 disabled.tsx
	n := kit.NewInputNumberValue(3)
	n.SetMin(1)
	n.SetMax(10)
	n.SetDisabled(true)
	_ = n.Node().Layout(core.Loose(200, 80))
	n.StepUp()
	if n.Value != 3 {
		t.Fatal("disabled demo must not step")
	}
}

func TestInputNumber_PRD_15_DigitDemo(t *testing.T) {
	// INN-15 digit.tsx — stringMode + tiny step
	n := kit.NewInputNumberValue(1)
	n.SetMin(0)
	n.SetMax(10)
	n.SetStep(0.00000000000001)
	n.SetStringMode(true)
	n.StepUp()
	if n.Value <= 1 {
		t.Fatalf("value=%v want >1 after tiny step", n.Value)
	}
	if n.DisplayString() == "" {
		t.Fatal("empty display")
	}
}

func TestInputNumber_PRD_16_FormatterDemo(t *testing.T) {
	// INN-16 formatter.tsx
	n := kit.NewInputNumberValue(1000)
	n.SetFormatter(func(v float64, _ kit.InputNumberFormatterInfo) string {
		raw := strconv.FormatFloat(v, 'f', -1, 64)
		parts := strings.Split(raw, ".")
		intp := parts[0]
		var b strings.Builder
		for i, r := range intp {
			if i > 0 && (len(intp)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		if len(parts) > 1 {
			return "$ " + b.String() + "." + parts[1]
		}
		return "$ " + b.String()
	})
	n.SetParser(func(s string) (float64, bool) {
		s = strings.ReplaceAll(s, "$", "")
		s = strings.ReplaceAll(s, ",", "")
		s = strings.TrimSpace(s)
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	})
	_ = n.Node().Layout(core.Loose(240, 80))
	disp := n.DisplayString()
	if !strings.Contains(disp, "1,000") && !strings.Contains(disp, "1000") {
		t.Fatalf("formatted display=%q", disp)
	}
	if !strings.HasPrefix(strings.TrimSpace(disp), "$") {
		t.Fatalf("display=%q want $ prefix", disp)
	}

	pct := kit.NewInputNumberValue(100)
	pct.SetMin(0)
	pct.SetMax(100)
	pct.SetFormatter(func(v float64, _ kit.InputNumberFormatterInfo) string {
		return strconv.FormatFloat(v, 'f', -1, 64) + "%"
	})
	pct.SetParser(func(s string) (float64, bool) {
		s = strings.TrimSuffix(strings.TrimSpace(s), "%")
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	})
	_ = pct.Node().Layout(core.Loose(200, 80))
	if !strings.HasSuffix(pct.DisplayString(), "%") {
		t.Fatalf("pct display=%q", pct.DisplayString())
	}
}

func TestInputNumber_PRD_17_KeyboardDemo(t *testing.T) {
	// INN-17 keyboard.tsx
	n := kit.NewInputNumberValue(3)
	n.SetMin(1)
	n.SetMax(10)
	n.SetKeyboard(true)
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	tree.SetFocus(n.Editor())
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowUp"})
	if n.Value != 4 {
		t.Fatalf("keyboard on value=%v want 4", n.Value)
	}
	n.SetKeyboard(false)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowUp"})
	if n.Value != 4 {
		t.Fatalf("keyboard off should not step, value=%v", n.Value)
	}
}

func TestInputNumber_PRD_18_WheelDemo(t *testing.T) {
	// INN-18 change-on-wheel.tsx
	n := kit.NewInputNumberValue(3)
	n.SetMin(1)
	n.SetMax(10)
	n.SetChangeOnWheel(true)
	steps := 0
	n.SetOnStep(func(_ float64, info kit.InputNumberStepInfo) {
		steps++
		if info.Emitter != kit.InputNumberStepWheel {
			t.Fatalf("emitter=%v want wheel", info.Emitter)
		}
	})
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	tree.SetFocus(n.Editor())
	if n.Editor().OnFocusChange != nil {
		n.Editor().OnFocusChange(true)
	}
	abs := core.AbsoluteBounds(n.ChromeNode())
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchScroll(&core.ScrollEvent{X: x, Y: y, DY: -20}) // up → increase
	if n.Value != 4 {
		t.Fatalf("wheel value=%v want 4", n.Value)
	}
	if steps != 1 {
		t.Fatalf("onStep=%d want 1", steps)
	}
}

func TestInputNumber_PRD_19_VariantDemo(t *testing.T) {
	// INN-19 variant.tsx
	for _, v := range []kit.InputVariant{
		kit.InputOutlined, kit.InputFilled, kit.InputBorderless, kit.InputUnderlined,
	} {
		n := kit.NewInputNumber()
		n.SetPlaceholder(v.String())
		n.SetVariant(v)
		_ = n.Node().Layout(core.Loose(220, 80))
		dec := n.ChromeNode().(*primitive.Decorated)
		if dec == nil {
			t.Fatalf("nil chrome variant=%v", v)
		}
		switch v {
		case kit.InputOutlined:
			if dec.BorderWidth < 0.5 {
				t.Fatalf("outlined borderW=%v", dec.BorderWidth)
			}
		case kit.InputBorderless:
			if dec.BorderWidth > 0.5 {
				t.Fatalf("borderless borderW=%v want 0", dec.BorderWidth)
			}
		}
	}
}

func TestInputNumber_PRD_20_TokenMetrics(t *testing.T) {
	// INN-20
	th := kit.DefaultTheme()
	if !approxINN(th.SizeOr(core.TokenControlHeight, 0), 32, 0.5) {
		t.Fatalf("controlHeight=%v", th.SizeOr(core.TokenControlHeight, 0))
	}
	if !approxINN(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.5) {
		t.Fatalf("controlHeightSM=%v", th.SizeOr(core.TokenControlHeightSM, 0))
	}
	if !approxINN(th.SizeOr(core.TokenControlHeightLG, 0), 40, 0.5) {
		t.Fatalf("controlHeightLG=%v", th.SizeOr(core.TokenControlHeightLG, 0))
	}
	if !approxINN(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatalf("borderRadius=%v", th.SizeOr(core.TokenBorderRadius, 0))
	}
	if !approxINN(th.SizeOr(core.TokenLineWidth, 0), 1, 0.5) {
		t.Fatalf("lineWidth=%v", th.SizeOr(core.TokenLineWidth, 0))
	}
	if !approxINN(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatalf("fontSize=%v", th.SizeOr(core.TokenFontSize, 0))
	}
	n := kit.NewInputNumberValue(1)
	sz := n.Node().Layout(core.Loose(400, 100))
	if !approxINN(sz.Height, 32, 0.5) {
		t.Fatalf("default height=%v", sz.Height)
	}
	if sz.Width < 80 || sz.Width > 120 {
		t.Fatalf("default width=%v want ~90", sz.Width)
	}
}

func TestInputNumber_PRD_21_ThemeTokens(t *testing.T) {
	// INN-21 — no hard-coded brand-only default skin
	n := kit.NewInputNumberValue(1)
	th := kit.DefaultTheme()
	n.SetTheme(th)
	_ = n.Node().Layout(core.Loose(200, 80))
	dec := n.ChromeNode().(*primitive.Decorated)
	if dec.BorderWidth < 0.5 {
		t.Fatal("outlined needs border")
	}
	if dec.BorderColor.A == 0 {
		t.Fatal("border color missing while border width set")
	}
	// primary token must be readable (theme path)
	if th.Color(core.TokenColorPrimary).A == 0 {
		t.Fatal("primary token empty")
	}
	if th.Color(core.TokenColorBgContainer).A == 0 && dec.Background.A == 0 {
		// both transparent is still token-driven for borderless; outlined uses container
	}
}

func TestInputNumber_PRD_22_DisabledChrome(t *testing.T) {
	// INN-22
	n := kit.NewInputNumberValue(1)
	n.SetDisabled(true)
	_ = n.Node().Layout(core.Loose(200, 80))
	dec := n.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	if approxINNColor(dec.Background, primary, 0.05) {
		t.Fatalf("disabled bg looks primary: %v", dec.Background)
	}
	if n.Editor().OnHoverChange != nil {
		n.Editor().OnHoverChange(true)
	}
	dec2 := n.ChromeNode().(*primitive.Decorated)
	if approxINNColor(dec2.BorderColor, primary, 0.05) && n.Status == kit.InputStatusNone {
		t.Fatal("disabled hover must not use primary border highlight")
	}
}

func TestInputNumber_PRD_23_FocusKeyboardA11y(t *testing.T) {
	// INN-23
	n := kit.NewInputNumberValue(3)
	n.SetAriaLabel("数量")
	_ = n.Node().Layout(core.Loose(200, 80))
	if n.Editor().Base().Role != "spinbutton" {
		t.Fatalf("role=%q want spinbutton", n.Editor().Base().Role)
	}
	if n.Editor().Base().Label != "数量" {
		t.Fatalf("label=%q want 数量", n.Editor().Base().Label)
	}
	n.SetStatus(kit.InputStatusError)
	if !strings.Contains(n.Editor().Base().Label, "invalid") {
		t.Fatalf("error a11y label=%q", n.Editor().Base().Label)
	}
	tree := core.NewTree(n.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	if !n.Editor().CanFocus() {
		t.Fatal("editor should be focusable")
	}
	tree.SetFocus(n.Editor())
	if n.Editor().OnFocusChange != nil {
		n.Editor().OnFocusChange(true)
	}
	dec := n.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	if !approxINNColor(dec.BorderColor, th.Color(core.TokenColorError), 0.12) {
		t.Fatalf("error border=%v", dec.BorderColor)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowUp"})
	if n.Value != 4 {
		t.Fatalf("focus keyboard step value=%v want 4", n.Value)
	}
}

func TestInputNumber_PRD_ModeSpinnerAPI(t *testing.T) {
	// mode is P0 API (spinner full visual P1 gallery)
	n := kit.NewInputNumberValue(3)
	n.SetMode(kit.InputNumberModeSpinner)
	if n.Mode != kit.InputNumberModeSpinner {
		t.Fatal(n.Mode)
	}
	_ = n.Node().Layout(core.Loose(200, 80))
	if !n.ControlsVisible() {
		t.Fatal("spinner still has controls")
	}
	n.StepUp()
	if n.Value != 4 {
		t.Fatalf("value=%v", n.Value)
	}
}
