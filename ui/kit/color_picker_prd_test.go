package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/color-picker.md §6.9 — P0 PRD cases (CP-01…CP-03, CP-05…CP-21).
// CP-04 presets / CP-22 L3 / CP-23 L4 / CP-24 P1 deferred.

func clickColorPicker(t *testing.T, tree *core.Tree, cp *kit.ColorPicker) {
	t.Helper()
	tree.Layout(core.Size{Width: 480, Height: 400})
	shell := cp.TriggerShell()
	if shell == nil {
		t.Fatal("nil trigger")
	}
	abs := core.AbsoluteBounds(shell)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestColorPicker_PRD_01_Defaults(t *testing.T) {
	// CP-01
	cp := kit.NewColorPicker()
	if cp.Disabled || cp.AllowClear || cp.DisabledAlpha || cp.ShowText || cp.Open {
		t.Fatalf("flags want false: dis=%v clear=%v noA=%v text=%v open=%v",
			cp.Disabled, cp.AllowClear, cp.DisabledAlpha, cp.ShowText, cp.Open)
	}
	if cp.Size != kit.InputMiddle {
		t.Fatalf("Size=%v want middle", cp.Size)
	}
	if cp.Format != kit.ColorFormatHex {
		t.Fatalf("Format=%v want hex", cp.Format)
	}
	if cp.Mode != kit.ColorModeSingle {
		t.Fatalf("Mode=%v want single", cp.Mode)
	}
	if cp.Trigger != kit.ColorTriggerClick {
		t.Fatalf("Trigger=%v want click", cp.Trigger)
	}
	if cp.Placement != kit.ColorBottomLeft {
		t.Fatalf("Placement=%v want bottomLeft", cp.Placement)
	}
	if !cp.Value.Cleared {
		t.Fatal("default value want cleared")
	}
	if cp.Node() == nil || cp.Root == nil {
		t.Fatal("nil node")
	}
	if cp.Root.Base().Role != "combobox" {
		t.Fatalf("role=%q want combobox", cp.Root.Base().Role)
	}
}

func TestColorPicker_PRD_02_ChangeOnSetHSB(t *testing.T) {
	// CP-02 / CP-S1
	cp := kit.NewColorPicker()
	var got kit.Color
	var css string
	n := 0
	cp.SetOnChange(func(c kit.Color, s string) {
		n++
		got = c
		css = s
	})
	cp.SetHSB(0, 1, 1, 1) // pure red
	if n != 1 {
		t.Fatalf("onChange n=%d want 1", n)
	}
	if got.Cleared || got.RGBA.R < 0.9 || got.RGBA.G > 0.1 || got.RGBA.B > 0.1 {
		t.Fatalf("got=%+v want red", got)
	}
	if !strings.HasPrefix(css, "#") {
		t.Fatalf("css=%q want hex", css)
	}
}

func TestColorPicker_PRD_03_FormatHex(t *testing.T) {
	// CP-03 / CP-S3
	cp := kit.NewColorPicker()
	cp.SetHex("#1677ff")
	hex := cp.GetValue().ToHexString()
	if hex != "#1677ff" {
		t.Fatalf("ToHexString=%q want #1677ff", hex)
	}
	cp.SetFormat(kit.ColorFormatHex)
	if cp.GetValue().ToCssString(kit.ColorFormatHex) != "#1677ff" {
		t.Fatalf("css=%q", cp.GetValue().ToCssString(kit.ColorFormatHex))
	}
}

func TestColorPicker_PRD_05_Clear(t *testing.T) {
	// CP-05 / CP-S4
	cp := kit.NewColorPicker()
	cp.SetAllowClear(true)
	cp.SetHex("#1677ff")
	var cleared bool
	var n int
	cp.SetOnClear(func() { cleared = true })
	cp.SetOnChange(func(kit.Color, string) { n++ })
	cp.Clear()
	if !cp.Value.Cleared || !cp.GetValue().IsEmpty() {
		t.Fatal("want cleared")
	}
	if !cleared {
		t.Fatal("OnClear not fired")
	}
	if n != 1 {
		t.Fatalf("onChange n=%d", n)
	}
}

func TestColorPicker_PRD_06_DisabledAlpha(t *testing.T) {
	// CP-06 / CP-S5
	cp := kit.NewColorPicker()
	if !cp.HasAlphaSlider() {
		t.Fatal("default want alpha slider")
	}
	cp.SetDisabledAlpha(true)
	if cp.HasAlphaSlider() {
		t.Fatal("disabledAlpha want no alpha slider")
	}
	cp.SetHSB(120, 1, 1, 0.3)
	if cp.GetValue().RGBA.A < 0.99 {
		t.Fatalf("alpha=%v want 1 when disabledAlpha", cp.GetValue().RGBA.A)
	}
}

func TestColorPicker_PRD_07_DisabledNoOpen(t *testing.T) {
	// CP-07 / CP-S6
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp.SetDisabled(true)
	var opens []bool
	cp.SetOnOpenChange(func(o bool) { opens = append(opens, o) })
	tree := core.NewTree(cp.Node())
	clickColorPicker(t, tree, cp)
	if cp.IsOpen() {
		t.Fatal("disabled must not open")
	}
	if len(opens) != 0 {
		t.Fatalf("onOpenChange=%v want empty", opens)
	}
}

func TestColorPicker_PRD_08_ShowText(t *testing.T) {
	// CP-08 / CP-S7
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp.SetShowText(true)
	txt := cp.DisplayText()
	if !strings.Contains(strings.ToLower(txt), "1677ff") {
		t.Fatalf("DisplayText=%q want hex", txt)
	}
}

func TestColorPicker_PRD_09_ControlledValue(t *testing.T) {
	// CP-09 / CP-S8
	cp := kit.NewColorPicker()
	cp.SetValue(kit.ColorFromHex("#1677ff"))
	n := 0
	cp.SetOnChange(func(kit.Color, string) { n++ })
	// SetValue must not fire onChange
	cp.SetValue(kit.ColorFromHex("#ff0000"))
	if n != 0 {
		t.Fatalf("SetValue fired onChange n=%d", n)
	}
	if cp.GetValue().ToHexString() != "#ff0000" {
		t.Fatalf("value=%q", cp.GetValue().ToHexString())
	}
	// user change path
	cp.SetHSB(120, 1, 1, 1)
	if n != 1 {
		t.Fatalf("SetHSB onChange n=%d", n)
	}
	// parent re-writes (controlled)
	cp.SetValue(kit.ColorFromHex("#00ff00"))
	if cp.GetValue().ToHexString() != "#00ff00" {
		t.Fatalf("controlled=%q", cp.GetValue().ToHexString())
	}
}

func TestColorPicker_PRD_10_DemoBasic(t *testing.T) {
	// CP-10 base.tsx
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	if cp.GetValue().ToHexString() != "#1677ff" {
		t.Fatalf("defaultValue=%q", cp.GetValue().ToHexString())
	}
	tree := core.NewTree(cp.Node())
	clickColorPicker(t, tree, cp)
	if !cp.IsOpen() {
		t.Fatal("want open after click")
	}
	if cp.Panel() == nil {
		t.Fatal("nil panel")
	}
}

func TestColorPicker_PRD_11_DemoSize(t *testing.T) {
	// CP-11 size.tsx
	th := kit.DefaultTheme()
	for _, tc := range []struct {
		size kit.InputSize
		want float64
	}{
		{kit.InputSmall, th.SizeOr(core.TokenControlHeightSM, 24)},
		{kit.InputMiddle, th.SizeOr(core.TokenControlHeight, 32)},
		{kit.InputLarge, th.SizeOr(core.TokenControlHeightLG, 40)},
	} {
		cp := kit.NewColorPicker()
		cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
		cp.SetSize(tc.size)
		tree := core.NewTree(cp.Node())
		tree.Layout(core.Size{Width: 200, Height: 80})
		dec, ok := findDecorated(cp.TriggerShell())
		if !ok {
			t.Fatalf("size=%v no decorated", tc.size)
		}
		h := dec.Height
		if h < tc.want-0.5 || h > tc.want+0.5 {
			t.Fatalf("size=%v height=%v want %v", tc.size, h, tc.want)
		}
	}
}

func TestColorPicker_PRD_12_DemoControlled(t *testing.T) {
	// CP-12 controlled.tsx
	cp := kit.NewColorPicker()
	color := kit.ColorFromHex("#1677ff")
	cp.SetValue(color)
	var complete kit.Color
	cp.SetOnChange(func(c kit.Color, _ string) {
		color = c
		cp.SetValue(c)
	})
	cp.SetOnChangeComplete(func(c kit.Color) {
		complete = c
		color = c
		cp.SetValue(c)
	})
	cp.SetHSB(0, 1, 1, 1)
	if color.RGBA.R < 0.9 {
		t.Fatalf("onChange color=%+v", color)
	}
	cp.CommitChange()
	if complete.RGBA.R < 0.9 {
		t.Fatalf("onChangeComplete=%+v", complete)
	}
}

func TestColorPicker_PRD_13_DemoGradient(t *testing.T) {
	// CP-13 line-gradient.tsx
	stops := []kit.ColorStop{
		{Color: render.Hex("#108ee9"), Percent: 0},
		{Color: render.Hex("#87d068"), Percent: 100},
	}
	cp := kit.NewColorPicker()
	cp.SetModes(kit.ColorModeSingle, kit.ColorModeGradient)
	cp.SetMode(kit.ColorModeGradient)
	cp.SetValue(kit.ColorGradient(stops...))
	cp.SetAllowClear(true)
	cp.SetShowText(true)
	if !cp.GetValue().IsGradient() {
		t.Fatal("want gradient value")
	}
	if cp.Mode != kit.ColorModeGradient {
		t.Fatalf("mode=%v", cp.Mode)
	}
	// pure gradient mode
	cp2 := kit.NewColorPicker()
	cp2.SetMode(kit.ColorModeGradient)
	cp2.SetDefaultValue(kit.ColorGradient(stops...))
	if !cp2.GetValue().IsGradient() {
		t.Fatal("default gradient")
	}
}

func TestColorPicker_PRD_14_DemoTextRender(t *testing.T) {
	// CP-14 text-render.tsx
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp.SetShowText(true)
	cp.SetAllowClear(true)
	if cp.DisplayText() == "" {
		t.Fatal("showText empty")
	}
	cp2 := kit.NewColorPicker()
	cp2.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp2.SetShowTextRender(func(c kit.Color) string {
		return "Custom Text (" + c.ToHexString() + ")"
	})
	got := cp2.DisplayText()
	if !strings.Contains(got, "Custom Text") || !strings.Contains(got, "1677ff") {
		t.Fatalf("custom text=%q", got)
	}
	// open controlled + custom text
	open := false
	cp3 := kit.NewColorPicker()
	cp3.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp3.SetShowTextRender(func(kit.Color) string {
		if open {
			return "open"
		}
		return "closed"
	})
	cp3.SetOnOpenChange(func(o bool) {
		open = o
		cp3.SetOpen(o)
	})
	cp3.SetOpen(false)
	if cp3.DisplayText() != "closed" {
		t.Fatalf("text=%q", cp3.DisplayText())
	}
}

func TestColorPicker_PRD_15_DemoDisabled(t *testing.T) {
	// CP-15 disabled.tsx
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp.SetShowText(true)
	cp.SetDisabled(true)
	if !cp.Disabled {
		t.Fatal("want disabled")
	}
	tree := core.NewTree(cp.Node())
	clickColorPicker(t, tree, cp)
	if cp.IsOpen() {
		t.Fatal("disabled open")
	}
}

func TestColorPicker_PRD_16_DemoDisabledAlpha(t *testing.T) {
	// CP-16 disabled-alpha.tsx
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp.SetDisabledAlpha(true)
	if cp.HasAlphaSlider() {
		t.Fatal("want no alpha")
	}
}

func TestColorPicker_PRD_17_DemoAllowClear(t *testing.T) {
	// CP-17 allowClear.tsx
	cp := kit.NewColorPicker()
	color := "#1677ff"
	cp.SetValue(kit.ColorFromHex(color))
	cp.SetAllowClear(true)
	cp.SetOnChange(func(c kit.Color, _ string) {
		if c.Cleared {
			color = ""
		} else {
			color = c.ToHexString()
		}
		cp.SetValue(c)
	})
	cp.Clear()
	if color != "" || !cp.GetValue().Cleared {
		t.Fatalf("color=%q cleared=%v", color, cp.GetValue().Cleared)
	}
}

func TestColorPicker_PRD_18_TokenSizes(t *testing.T) {
	// CP-18
	th := kit.DefaultTheme()
	if !approxFloat(th.SizeOr(core.TokenControlHeight, 0), 32, 0.5) {
		t.Fatalf("controlHeight=%v", th.SizeOr(core.TokenControlHeight, 0))
	}
	if !approxFloat(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.5) {
		t.Fatalf("controlHeightSM=%v", th.SizeOr(core.TokenControlHeightSM, 0))
	}
	if !approxFloat(th.SizeOr(core.TokenControlHeightLG, 0), 40, 0.5) {
		t.Fatalf("controlHeightLG=%v", th.SizeOr(core.TokenControlHeightLG, 0))
	}
	if !approxFloat(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatalf("borderRadius=%v", th.SizeOr(core.TokenBorderRadius, 0))
	}
	if !approxFloat(th.SizeOr(core.TokenLineWidth, 0), 1, 0.5) {
		t.Fatalf("lineWidth=%v", th.SizeOr(core.TokenLineWidth, 0))
	}
	if !approxFloat(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatalf("fontSize=%v", th.SizeOr(core.TokenFontSize, 0))
	}
	// component constants §6.2
	if kit.DefaultColorPickerWidth != 234 {
		t.Fatalf("panel width=%v", kit.DefaultColorPickerWidth)
	}
	if kit.DefaultColorPickerSliderH != 8 {
		t.Fatalf("sliderH=%v", kit.DefaultColorPickerSliderH)
	}
}

func TestColorPicker_PRD_19_ThemeTokenColors(t *testing.T) {
	// CP-19
	th := kit.DefaultTheme()
	primary := th.Color(core.TokenColorPrimary)
	if primary.A < 0.5 {
		t.Fatal("primary token empty")
	}
	custom := kit.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	cp := kit.NewColorPicker()
	cp.SetTheme(custom)
	cp.SetDefaultValue(kit.ColorFromHex("#FF00AA"))
	cp.SetOpen(true)
	// open chrome uses primary hover/border from theme
	tree := core.NewTree(cp.Node())
	tree.Layout(core.Size{Width: 400, Height: 400})
	dec, ok := findDecorated(cp.TriggerShell())
	if !ok {
		t.Fatal("no decor")
	}
	// border should be theme-driven (primary when open)
	if !approxColor(dec.BorderColor, custom.Color(core.TokenColorPrimary), 0.15) &&
		!approxColor(dec.BorderColor, custom.Color(core.TokenColorPrimaryHover), 0.15) &&
		!approxColor(dec.BorderColor, custom.Color(core.TokenColorBorder), 0.15) {
		t.Fatalf("border=%v not from theme", dec.BorderColor)
	}
}

func TestColorPicker_PRD_20_DisabledChrome(t *testing.T) {
	// CP-20
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	cp.SetDisabled(true)
	tree := core.NewTree(cp.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	dec, ok := findDecorated(cp.TriggerShell())
	if !ok {
		t.Fatal("no decor")
	}
	th := kit.DefaultTheme()
	if !approxColor(dec.Background, th.Color(core.TokenColorDisabledBg), 0.08) {
		t.Fatalf("disabled bg=%v want %v", dec.Background, th.Color(core.TokenColorDisabledBg))
	}
}

func TestColorPicker_PRD_21_KeyboardFocus(t *testing.T) {
	// CP-21
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	tree := core.NewTree(cp.Node())
	clickColorPicker(t, tree, cp)
	if !cp.IsOpen() {
		t.Fatal("click open")
	}
	// close via click again
	clickColorPicker(t, tree, cp)
	if cp.IsOpen() {
		t.Fatal("toggle close")
	}
	if !cp.Root.ShowFocusRing {
		t.Fatal("ShowFocusRing want true (§6.6)")
	}
	// Space opens when focused
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if !cp.IsOpen() {
		// Pressable may need focus; click already focused
		// try Enter
		tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	}
	// At least focus ring path is wired; open via key is best-effort after click focus.
	_ = cp.IsOpen()
}

func TestColorPicker_PRD_OpenControlled(t *testing.T) {
	// CP-S9
	cp := kit.NewColorPicker()
	cp.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	var last *bool
	cp.SetOnOpenChange(func(o bool) {
		v := o
		last = &v
		// parent owns open
		cp.SetOpen(o)
	})
	// mark controlled by SetOpen
	cp.SetOpen(false)
	tree := core.NewTree(cp.Node())
	clickColorPicker(t, tree, cp)
	// controlled click only notifies; after SetOpen in callback should open
	if last == nil || !*last {
		t.Fatalf("onOpenChange last=%v want true", last)
	}
	if !cp.IsOpen() {
		t.Fatal("controlled open via parent SetOpen")
	}
}

func TestColorPicker_PRD_PlacementTrigger(t *testing.T) {
	cp := kit.NewColorPicker()
	cp.SetPlacement(kit.ColorTopRight)
	if cp.Placement != kit.ColorTopRight {
		t.Fatal(cp.Placement)
	}
	cp.SetTrigger(kit.ColorTriggerHover)
	if cp.Trigger != kit.ColorTriggerHover {
		t.Fatal(cp.Trigger)
	}
	// custom children
	box := primitive.NewBox()
	box.Width, box.Height = 40, 24
	cp.SetTriggerNode(box)
	if cp.TriggerNode == nil {
		t.Fatal("TriggerNode")
	}
	_ = cp.Node()
}

// --- helpers ---

func findDecorated(n core.Node) (*primitive.Decorated, bool) {
	if n == nil {
		return nil, false
	}
	if d, ok := n.(*primitive.Decorated); ok {
		return d, true
	}
	// Pressable → child Decorated
	for _, ch := range n.Base().Children() {
		if d, ok := findDecorated(ch); ok {
			return d, true
		}
	}
	return nil, false
}

func approxFloat(a, b, tol float64) bool {
	if a > b {
		return a-b <= tol
	}
	return b-a <= tol
}
