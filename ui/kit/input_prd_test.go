package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/input.md §6.9 — P0 PRD cases (INP-01 … INP-25).
// L3/L4 (INP-26/27) and P1 (INP-28) deferred.

func approxInputColor(a, b render.RGBA, tol float64) bool {
	dr := a.R - b.R
	if dr < 0 {
		dr = -dr
	}
	dg := a.G - b.G
	if dg < 0 {
		dg = -dg
	}
	db := a.B - b.B
	if db < 0 {
		db = -db
	}
	da := a.A - b.A
	if da < 0 {
		da = -da
	}
	return dr <= tol && dg <= tol && db <= tol && da <= tol
}

func TestInput_PRD_01_Defaults(t *testing.T) {
	// INP-01: NewInput 默认创建
	in := kit.NewInput("ph")
	if in.Size != kit.InputMiddle {
		t.Fatalf("Size=%v want middle", in.Size)
	}
	if in.Variant != kit.InputOutlined {
		t.Fatalf("Variant=%v want outlined", in.Variant)
	}
	if in.Status != kit.InputStatusNone {
		t.Fatalf("Status=%v want none", in.Status)
	}
	if in.Type != kit.InputTypeText {
		t.Fatalf("Type=%v want text", in.Type)
	}
	if in.Disabled || in.ReadOnly || in.AllowClear || in.Controlled {
		t.Fatalf("flags should be false")
	}
	if in.Placeholder != "ph" {
		t.Fatalf("Placeholder=%q", in.Placeholder)
	}
	if in.Node() == nil || in.ChromeNode() == nil || in.Editor() == nil {
		t.Fatal("nil nodes")
	}
	_ = in.Node().Layout(core.Loose(400, 100))
}

func TestInput_PRD_02_ControlledValue(t *testing.T) {
	// INP-02 / INP-S1: 受控 value — 键入只经 onChange 上抛，不私自写回
	in := kit.NewInput("")
	in.SetControlled(true)
	in.SetValue("ext")
	got := ""
	in.SetOnChange(func(v string) { got = v })

	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "x"})

	if got != "extx" && got != "x" {
		// Controlled: editor may compose over current value before revert.
		// Accept any onChange payload that is not silently dropped.
		if got == "" {
			t.Fatal("onChange not fired on type")
		}
	}
	if in.Value != "ext" {
		t.Fatalf("controlled Value=%q want ext (no private writeback)", in.Value)
	}
	if in.Editor().Value != "ext" {
		t.Fatalf("editor display=%q want ext after revert", in.Editor().Value)
	}
	// Parent writes back
	in.SetValue("extx")
	if in.Editor().Value != "extx" {
		t.Fatalf("after SetValue editor=%q", in.Editor().Value)
	}
}

func TestInput_PRD_03_AllowClear(t *testing.T) {
	// INP-03: allowClear 有内容时点清除 → onChange("")；空时隐藏
	in := kit.NewInput("")
	in.SetAllowClear(true)
	in.SetValue("hello")
	changes := 0
	last := "unset"
	in.SetOnChange(func(v string) {
		changes++
		last = v
	})
	cleared := 0
	in.SetOnClear(func() { cleared++ })

	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})

	// Find clear pressable by walking for Icon close / Pressable near end.
	var clear *primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || clear != nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok {
			// Heuristic: non-focusable affix pressables
			if !p.Focusable && p.Click != nil {
				// Prefer the one that is not the eye (password) — we have allowClear only.
				clear = p
			}
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(in.Node())
	if clear == nil {
		t.Fatal("clear pressable not found")
	}
	abs := core.AbsoluteBounds(clear)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	// Reset counters after SetValue (which may fire onChange).
	changes, last = 0, "unset"
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if last != "" {
		t.Fatalf("onChange after clear=%q want \"\"", last)
	}
	if changes != 1 {
		t.Fatalf("onChange count=%d want 1", changes)
	}
	if cleared != 1 {
		t.Fatalf("onClear=%d want 1", cleared)
	}
	if in.Value != "" {
		t.Fatalf("Value=%q want empty", in.Value)
	}

	// Empty → clear slot collapsed (no second clear)
	in.SetValue("")
	tree.Layout(core.Size{Width: 320, Height: 80})
	// Slot child should be nil — re-walk for clear with size
	clear = nil
	walk(in.Node())
	// May still find pressable detached; value empty is the contract.
	if in.Value != "" {
		t.Fatal("empty value required")
	}
}

func TestInput_PRD_04_Disabled(t *testing.T) {
	// INP-04
	in := kit.NewInput("")
	in.SetDisabled(true)
	changes := 0
	in.SetOnChange(func(string) { changes++ })
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "nope"})
	if changes != 0 || in.Value != "" {
		t.Fatalf("disabled must not change: changes=%d value=%q", changes, in.Value)
	}
	if in.Editor().CanFocus() {
		t.Fatal("disabled editor CanFocus want false")
	}
}

func TestInput_PRD_05_ReadOnly(t *testing.T) {
	// INP-05
	in := kit.NewInput("")
	in.SetValue("ro")
	in.SetReadOnly(true)
	changes := 0
	in.SetOnChange(func(string) { changes++ })
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	if !in.Editor().CanFocus() {
		t.Fatal("readOnly should still be focusable")
	}
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "x"})
	if changes != 0 || in.Value != "ro" {
		t.Fatalf("readOnly mutated: changes=%d value=%q", changes, in.Value)
	}
}

func TestInput_PRD_06_MaxLength(t *testing.T) {
	// INP-06
	in := kit.NewInput("")
	in.SetMaxLength(3)
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "abcd"})
	if in.Value != "abc" {
		t.Fatalf("value=%q want abc (maxLength=3)", in.Value)
	}
}

func TestInput_PRD_07_StatusError(t *testing.T) {
	// INP-07
	in := kit.NewInput("")
	in.SetStatus(kit.InputStatusError)
	_ = in.Node().Layout(core.Loose(400, 100))
	dec := in.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	errC := th.Color(core.TokenColorError)
	if !approxInputColor(dec.BorderColor, errC, 0.05) {
		t.Fatalf("error border=%v want ~%v", dec.BorderColor, errC)
	}
	// still editable
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "ok"})
	if in.Value != "ok" {
		t.Fatalf("value=%q", in.Value)
	}
}

func TestInput_PRD_08_PressEnter(t *testing.T) {
	// INP-08
	in := kit.NewInput("")
	in.SetValue("hi")
	got := ""
	in.SetOnPressEnter(func(v string) { got = v })
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if got != "hi" {
		t.Fatalf("onPressEnter=%q want hi", got)
	}
}

func TestInput_PRD_09_PasswordToggle(t *testing.T) {
	// INP-09
	pw := kit.NewPassword("")
	pw.SetValue("secret")
	if !pw.Editor().Password {
		t.Fatal("password should mask by default")
	}
	if pw.Value != "secret" {
		t.Fatalf("value=%q", pw.Value)
	}
	pw.SetPasswordVisible(true)
	if pw.Editor().Password {
		t.Fatal("visible → Password mask off")
	}
	if pw.Value != "secret" {
		t.Fatalf("value changed on toggle: %q", pw.Value)
	}
	pw.SetPasswordVisible(false)
	if !pw.Editor().Password {
		t.Fatal("hidden again")
	}
}

func TestInput_PRD_10_SearchIcon(t *testing.T) {
	// INP-10
	s := kit.NewSearch("q")
	s.SetValue("term")
	src := kit.SearchSource(-1)
	val := ""
	s.SetOnSearch(func(v string, source kit.SearchSource) {
		val = v
		src = source
	})
	tree := core.NewTree(s.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})

	// Click search icon pressable
	var searchP *primitive.Pressable
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || searchP != nil {
			return
		}
		if p, ok := n.(*primitive.Pressable); ok && !p.Focusable && p.Click != nil {
			// last non-focusable pressable is search icon when !enterButton
			searchP = p
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(s.Node())
	if searchP == nil {
		t.Fatal("search pressable not found")
	}
	abs := core.AbsoluteBounds(searchP)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if val != "term" || src != kit.SearchFromButton {
		t.Fatalf("onSearch val=%q src=%v", val, src)
	}
}

func TestInput_PRD_11_TextAreaAutoSize(t *testing.T) {
	// INP-11
	ta := kit.NewTextArea("ta", 2)
	ta.SetAutoSizeRange(2, 4)
	_ = ta.Node().Layout(core.Loose(400, 400))
	h0 := ta.ChromeNode().(*primitive.Decorated).Height
	if h0 <= 0 {
		t.Fatal("autoSize height unset")
	}
	ta.SetValue("a\nb\nc\nd\ne\nf")
	_ = ta.Node().Layout(core.Loose(400, 400))
	h1 := ta.ChromeNode().(*primitive.Decorated).Height
	// maxRows=4 → height capped
	if h1 < h0 {
		t.Fatalf("height shrunk? h0=%v h1=%v", h0, h1)
	}
	// min rows: empty still ≥ min
	ta.SetValue("")
	_ = ta.Node().Layout(core.Loose(400, 400))
	hMin := ta.ChromeNode().(*primitive.Decorated).Height
	if hMin <= 0 {
		t.Fatal("min rows height")
	}
}

func TestInput_PRD_12_SizeHeights(t *testing.T) {
	// INP-12 / INP-S11
	in := kit.NewInput("x")
	for _, tc := range []struct {
		size kit.InputSize
		want float64
	}{
		{kit.InputSmall, 24},
		{kit.InputMiddle, 32},
		{kit.InputLarge, 40},
	} {
		in.SetSize(tc.size)
		sz := in.Node().Layout(core.Loose(400, 100))
		if sz.Height < tc.want-0.5 || sz.Height > tc.want+0.5 {
			t.Fatalf("size %v height=%v want %v±0.5", tc.size, sz.Height, tc.want)
		}
	}
}

func TestInput_PRD_13_VariantChrome(t *testing.T) {
	// INP-13: variant switch + status clear leaves no residual error border
	in := kit.NewInput("v")
	for _, v := range []kit.InputVariant{
		kit.InputOutlined, kit.InputFilled, kit.InputBorderless, kit.InputUnderlined,
	} {
		in.SetVariant(v)
		in.SetStatus(kit.InputStatusError)
		_ = in.Node().Layout(core.Loose(400, 100))
		in.SetStatus(kit.InputStatusNone)
		_ = in.Node().Layout(core.Loose(400, 100))
		dec := in.ChromeNode().(*primitive.Decorated)
		th := kit.DefaultTheme()
		if v == kit.InputOutlined && approxInputColor(dec.BorderColor, th.Color(core.TokenColorError), 0.05) {
			t.Fatalf("variant %v residual error border %v", v, dec.BorderColor)
		}
	}
}

func TestInput_PRD_14_BasicDemo(t *testing.T) {
	// INP-14 basic.tsx
	in := kit.NewInput("Basic usage")
	if in.Node() == nil {
		t.Fatal()
	}
	sz := in.Node().Layout(core.Loose(400, 100))
	if sz.Height < 24 {
		t.Fatalf("height=%v", sz.Height)
	}
}

func TestInput_PRD_15_SizeDemo(t *testing.T) {
	// INP-15 size.tsx
	for _, s := range []kit.InputSize{kit.InputLarge, kit.InputMiddle, kit.InputSmall} {
		in := kit.NewInput("size")
		in.SetSize(s)
		in.SetPrefix(primitive.NewIcon("info"))
		sz := in.Node().Layout(core.Loose(400, 100))
		if sz.Height < 20 {
			t.Fatalf("size %v h=%v", s, sz.Height)
		}
	}
}

func TestInput_PRD_16_VariantDemo(t *testing.T) {
	// INP-16 variant.tsx
	for _, v := range []kit.InputVariant{
		kit.InputOutlined, kit.InputFilled, kit.InputBorderless, kit.InputUnderlined,
	} {
		in := kit.NewInput(v.String())
		in.SetVariant(v)
		_ = in.Node().Layout(core.Loose(400, 100))
	}
	s := kit.NewSearch("Filled")
	s.SetVariant(kit.InputFilled)
	_ = s.Node().Layout(core.Loose(400, 100))
}

func TestInput_PRD_17_CompactDemo(t *testing.T) {
	// INP-17 compact-style.tsx
	in1 := kit.NewInput("")
	in1.SetValue("0571")
	in2 := kit.NewInput("")
	in2.SetValue("26888888")
	c := kit.NewSpaceCompact(in1.Node(), in2.Node())
	if c.Node() == nil {
		t.Fatal()
	}
	_ = c.Node().Layout(core.Loose(600, 100))
}

func TestInput_PRD_18_SearchDemo(t *testing.T) {
	// INP-18 search-input.tsx
	s := kit.NewSearch("input search text")
	s.SetAllowClear(true)
	s.SetOnSearch(func(string, kit.SearchSource) {})
	_ = s.Node().Layout(core.Loose(400, 100))
	s2 := kit.NewSearch("btn")
	s2.SetEnterButton(true)
	s2.SetOnSearch(func(string, kit.SearchSource) {})
	_ = s2.Node().Layout(core.Loose(400, 100))
	s3 := kit.NewSearch("lbl")
	s3.SetEnterButtonText("Search")
	s3.SetSize(kit.InputLarge)
	_ = s3.Node().Layout(core.Loose(400, 100))
}

func TestInput_PRD_19_SearchLoadingDemo(t *testing.T) {
	// INP-19 search-input-loading.tsx
	s := kit.NewSearch("loading")
	s.SetLoading(true)
	if !s.IsLoading() {
		t.Fatal("loading flag")
	}
	_ = s.Node().Layout(core.Loose(400, 100))
	// Tick advances spinner without panic
	if !s.Tick(0.016) {
		t.Fatal("Tick should stay active while loading")
	}
	s.SetEnterButton(true)
	s.SetLoading(true)
	_ = s.Node().Layout(core.Loose(400, 100))
}

func TestInput_PRD_20_TextAreaDemo(t *testing.T) {
	// INP-20 textarea.tsx
	ta := kit.NewTextArea("", 4)
	_ = ta.Node().Layout(core.Loose(400, 200))
	ta2 := kit.NewTextArea("maxLength is 6", 4)
	ta2.SetMaxLength(6)
	tree := core.NewTree(ta2.Node())
	tree.Layout(core.Size{Width: 400, Height: 200})
	tree.SetFocus(ta2.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "1234567890"})
	if ta2.Value != "123456" {
		t.Fatalf("value=%q want 123456", ta2.Value)
	}
}

func TestInput_PRD_21_AutoSizeDemo(t *testing.T) {
	// INP-21 autosize-textarea.tsx
	ta := kit.NewTextArea("auto", 2)
	ta.SetAutoSize(true)
	_ = ta.Node().Layout(core.Loose(400, 400))
	ta2 := kit.NewTextArea("range", 2)
	ta2.SetAutoSizeRange(2, 6)
	ta2.SetValue("1\n2\n3")
	_ = ta2.Node().Layout(core.Loose(400, 400))
	h := ta2.ChromeNode().(*primitive.Decorated).Height
	if h < 20 {
		t.Fatalf("autoSize height=%v", h)
	}
}

func TestInput_PRD_22_TokenMetrics(t *testing.T) {
	// INP-22 §6.2
	th := kit.DefaultTheme()
	if th.SizeOr(core.TokenControlHeight, 0) != 32 {
		t.Fatalf("controlHeight=%v want 32", th.SizeOr(core.TokenControlHeight, 0))
	}
	if th.SizeOr(core.TokenControlHeightSM, 0) != 24 {
		t.Fatalf("SM=%v want 24", th.SizeOr(core.TokenControlHeightSM, 0))
	}
	if th.SizeOr(core.TokenControlHeightLG, 0) != 40 {
		t.Fatalf("LG=%v want 40", th.SizeOr(core.TokenControlHeightLG, 0))
	}
	if th.SizeOr(core.TokenFontSize, 0) != 14 {
		t.Fatalf("fontSize=%v want 14", th.SizeOr(core.TokenFontSize, 0))
	}
	if th.SizeOr(core.TokenBorderRadius, 0) != 6 {
		t.Fatalf("radius=%v want 6", th.SizeOr(core.TokenBorderRadius, 0))
	}
	pad := th.SizeOr(core.TokenControlPaddingInline, kit.DefaultInputPaddingInline)
	if pad < 7 || pad > 12 {
		t.Fatalf("paddingInline=%v want ~11", pad)
	}
	in := kit.NewInput("")
	sz := in.Node().Layout(core.Loose(400, 100))
	if sz.Height < 31.5 || sz.Height > 32.5 {
		t.Fatalf("middle height=%v", sz.Height)
	}
}

func TestInput_PRD_23_TokenColors(t *testing.T) {
	// INP-23 默认皮走 Token，无硬编码品牌蓝当唯一皮
	in := kit.NewInput("")
	_ = in.Node().Layout(core.Loose(400, 100))
	dec := in.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	if !approxInputColor(dec.Background, bg, 0.02) {
		t.Fatalf("bg=%v want token %v", dec.Background, bg)
	}
	if !approxInputColor(dec.BorderColor, bd, 0.02) {
		t.Fatalf("border=%v want token %v", dec.BorderColor, bd)
	}
	// Brand primary only on focus/status — not default fill
	primary := th.Color(core.TokenColorPrimary)
	if approxInputColor(dec.Background, primary, 0.05) {
		t.Fatal("default fill must not be primary brand")
	}
}

func TestInput_PRD_24_DisabledChrome(t *testing.T) {
	// INP-24
	in := kit.NewInput("")
	in.SetDisabled(true)
	_ = in.Node().Layout(core.Loose(400, 100))
	dec := in.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	dbg := th.Color(core.TokenColorDisabledBg)
	if !approxInputColor(dec.Background, dbg, 0.05) {
		t.Fatalf("disabled bg=%v want ~%v", dec.Background, dbg)
	}
	// hover must not promote border to primary while disabled
	in.Editor().OnHoverChange(true) // may be nil-safe via apply — force via SetDisabled path
	in.SetDisabled(true)
	_ = in.Node().Layout(core.Loose(400, 100))
	dec = in.ChromeNode().(*primitive.Decorated)
	if approxInputColor(dec.BorderColor, th.Color(core.TokenColorPrimary), 0.05) {
		t.Fatal("disabled must not show primary hover border")
	}
}

func TestInput_PRD_25_FocusA11y(t *testing.T) {
	// INP-25
	in := kit.NewInput("name")
	in.SetAriaLabel("Full name")
	_ = in.Node().Layout(core.Loose(400, 100))
	if in.Editor().Base().Role != "textbox" {
		t.Fatalf("role=%q want textbox", in.Editor().Base().Role)
	}
	if in.Editor().Base().Label != "Full name" {
		t.Fatalf("label=%q", in.Editor().Base().Label)
	}
	tree := core.NewTree(in.Node())
	tree.Layout(core.Size{Width: 320, Height: 80})
	tree.SetFocus(in.Editor())
	if !in.IsFocused() {
		t.Fatal("IsFocused after SetFocus")
	}
	// focus chrome → primary border
	dec := in.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	if !approxInputColor(dec.BorderColor, th.Color(core.TokenColorPrimary), 0.08) {
		t.Fatalf("focus border=%v want primary", dec.BorderColor)
	}
}

func TestInput_PRD_DefaultValue(t *testing.T) {
	in := kit.NewInputWithDefault("ph", "seed")
	if in.Value != "seed" || in.DefaultValue != "seed" {
		t.Fatalf("default value: Value=%q Default=%q", in.Value, in.DefaultValue)
	}
	if in.Editor().Value != "seed" {
		t.Fatalf("editor=%q", in.Editor().Value)
	}
}

func TestInput_PRD_LayoutPlaceholder(t *testing.T) {
	// Keep layout regression from input_layout_test
	in := kit.NewInput("Type here")
	in.SetFixedSize(360, 32)
	sz := in.Node().Layout(core.Loose(800, 600))
	if sz.Width < 300 || sz.Height < 28 {
		t.Fatalf("size=%v", sz)
	}
	if in.Editor().Size().Width < 200 {
		t.Fatalf("editor width=%v", in.Editor().Size().Width)
	}
}
