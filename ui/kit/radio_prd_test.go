package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/radio.md §6.9 — P0 PRD cases (RDO-01 … RDO-21).
// L3/L4 (RDO-22/23) and P1 (RDO-24) deferred.

func clickRadio(t *testing.T, tree *core.Tree, r *kit.Radio) {
	t.Helper()
	tree.Layout(core.Size{Width: 480, Height: 200})
	abs := core.AbsoluteBounds(r.Root)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestRadio_PRD_01_Defaults(t *testing.T) {
	// RDO-01: NewRadio 默认创建
	r := kit.NewRadio("Radio")
	if r.Checked || r.Disabled || r.Controlled || r.ButtonMode {
		t.Fatalf("flags want false: checked=%v dis=%v ctrl=%v btn=%v",
			r.Checked, r.Disabled, r.Controlled, r.ButtonMode)
	}
	if r.Label != "Radio" {
		t.Fatalf("Label=%q", r.Label)
	}
	if r.Node() == nil || r.Root == nil {
		t.Fatal("nil node")
	}
	if r.Root.Base().Role != "radio" {
		t.Fatalf("role=%q want radio", r.Root.Base().Role)
	}
	if r.Root.Base().Label != "Radio" {
		t.Fatalf("a11y name=%q want Radio", r.Root.Base().Label)
	}
}

func TestRadio_PRD_02_TwoOptionsExclusive(t *testing.T) {
	// RDO-02 / RDO-S1: 两点不同 option → 仅后者选中
	g := kit.NewRadioGroup()
	g.SetStringOptions("A", "B")
	var last string
	g.SetOnChange(func(v string) { last = v })
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[0])
	if g.Value != "A" || !g.Items[0].Checked || g.Items[1].Checked {
		t.Fatalf("after A: value=%s a=%v b=%v", g.Value, g.Items[0].Checked, g.Items[1].Checked)
	}
	clickRadio(t, tree, g.Items[1])
	if g.Value != "B" || g.Items[0].Checked || !g.Items[1].Checked {
		t.Fatalf("after B: value=%s a=%v b=%v", g.Value, g.Items[0].Checked, g.Items[1].Checked)
	}
	if last != "B" {
		t.Fatalf("onChange last=%q", last)
	}
	// Re-click selected keeps selection (no deselect).
	n := 0
	g.SetOnChange(func(string) { n++ })
	clickRadio(t, tree, g.Items[1])
	if n != 0 || g.Value != "B" {
		t.Fatalf("reclick n=%d value=%s", n, g.Value)
	}
}

func TestRadio_PRD_03_OptionTypeButton(t *testing.T) {
	// RDO-03 / RDO-S2: optionType=button → 按钮组外观
	g := kit.NewRadioGroup()
	g.SetOptionType(kit.RadioOptionButton)
	g.SetStringOptions("Hangzhou", "Shanghai")
	g.SetDefaultValue("Hangzhou")
	if len(g.Items) != 2 {
		t.Fatalf("items=%d", len(g.Items))
	}
	if !g.Items[0].ButtonMode {
		t.Fatal("optionType=button should set ButtonMode on options")
	}
	// Button mode: indicator is the button Decorated (MinHeight = controlHeight).
	ind := g.Items[0].IndicatorNode().(*primitive.Decorated)
	if ind.Height < 32-0.5 || ind.Height > 32+0.5 {
		t.Fatalf("button height=%v want 32", ind.Height)
	}
	if !g.Items[0].Checked || g.Items[1].Checked {
		t.Fatalf("default Hangzhou: a=%v b=%v", g.Items[0].Checked, g.Items[1].Checked)
	}
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[1])
	if g.Value != "Shanghai" || !g.Items[1].Checked {
		t.Fatalf("value=%s shanghai=%v", g.Value, g.Items[1].Checked)
	}
}

func TestRadio_PRD_04_ButtonStyleSolid(t *testing.T) {
	// RDO-04 / RDO-S3: buttonStyle=solid → 实心选中态
	g := kit.NewRadioGroup()
	g.SetOptionType(kit.RadioOptionButton)
	g.SetButtonStyle(kit.RadioButtonSolid)
	g.SetStringOptions("Apple", "Pear")
	g.SetDefaultValue("Apple")
	_ = g.Node().Layout(core.Loose(400, 80))
	ind := g.Items[0].IndicatorNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	if !approxColor(ind.Background, th.Color(core.TokenColorPrimary), 0.05) {
		t.Fatalf("solid checked fill=%v want primary", ind.Background)
	}
}

func TestRadio_PRD_05_DisabledOption(t *testing.T) {
	// RDO-05 / RDO-S4: disabled 项不可选
	g := kit.NewRadioGroup()
	g.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear", Disabled: true},
		kit.RadioOption{Label: "Orange", Value: "Orange"},
	)
	g.SetDefaultValue("Apple")
	n := 0
	g.SetOnChange(func(string) { n++ })
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[1])
	if n != 0 || g.Value != "Apple" || g.Items[1].Checked {
		t.Fatalf("disabled pear selected n=%d value=%s pear=%v", n, g.Value, g.Items[1].Checked)
	}
	// Whole group disabled.
	g2 := kit.NewRadioGroup()
	g2.SetStringOptions("A", "B")
	g2.SetDefaultValue("A")
	g2.SetDisabled(true)
	n2 := 0
	g2.SetOnChange(func(string) { n2++ })
	tree2 := core.NewTree(g2.Node())
	clickRadio(t, tree2, g2.Items[1])
	if n2 != 0 || g2.Value != "A" {
		t.Fatalf("group disabled n=%d value=%s", n2, g2.Value)
	}
}

func TestRadio_PRD_06_ControlledValue(t *testing.T) {
	// RDO-06 / RDO-S5: 受控 value 外部优先
	g := kit.NewRadioGroup()
	g.SetStringOptions("A", "B", "C")
	g.SetValue("A")
	var got []string
	g.SetOnChange(func(v string) { got = append(got, v) })
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[1])
	// Controlled: local Value stays until parent SetValue.
	if g.Value != "A" {
		t.Fatalf("controlled local value=%s want A", g.Value)
	}
	if len(got) != 1 || got[0] != "B" {
		t.Fatalf("onChange=%v want [B]", got)
	}
	if g.Items[1].Checked {
		t.Fatal("controlled: item must not auto-check until parent sets")
	}
	// Parent applies.
	g.SetValue("B")
	if g.Value != "B" || !g.Items[1].Checked || g.Items[0].Checked {
		t.Fatalf("after parent set: value=%s a=%v b=%v", g.Value, g.Items[0].Checked, g.Items[1].Checked)
	}
}

func TestRadio_PRD_07_IndicatorSize(t *testing.T) {
	// RDO-07 / RDO-S6: 圆点尺寸 16
	r := kit.NewRadio("c")
	ind := r.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Width < 16-0.5 || ind.Width > 16+0.5 || ind.Height < 16-0.5 || ind.Height > 16+0.5 {
		t.Fatalf("indicator %vx%v want 16x16", ind.Width, ind.Height)
	}
}

func TestRadio_PRD_08_ButtonHeightMiddle(t *testing.T) {
	// RDO-08 / RDO-S7: Radio.Button 高度 middle 32
	btn := kit.NewRadioButton("Hangzhou")
	ind := btn.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(200, 100))
	if ind.Height < 32-0.5 || ind.Height > 32+0.5 {
		t.Fatalf("button height=%v want 32", ind.Height)
	}
	// Group size small → 24
	g := kit.NewRadioGroup()
	g.SetOptionType(kit.RadioOptionButton)
	g.SetSize(kit.RadioSmall)
	g.SetStringOptions("a", "b")
	ind2 := g.Items[0].IndicatorNode().(*primitive.Decorated)
	_ = ind2.Layout(core.Loose(200, 100))
	if ind2.Height < 24-0.5 || ind2.Height > 24+0.5 {
		t.Fatalf("small button height=%v want 24", ind2.Height)
	}
	// large → 40
	g.SetSize(kit.RadioLarge)
	ind3 := g.Items[0].IndicatorNode().(*primitive.Decorated)
	_ = ind3.Layout(core.Loose(200, 100))
	if ind3.Height < 40-0.5 || ind3.Height > 40+0.5 {
		t.Fatalf("large button height=%v want 40", ind3.Height)
	}
}

func TestRadio_PRD_09_KeyboardArrows(t *testing.T) {
	// RDO-09 / RDO-S8: 键盘方向移动选中
	g := kit.NewRadioGroup()
	g.SetStringOptions("A", "B", "C")
	g.SetDefaultValue("A")
	if !g.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowRight"}) {
		t.Fatal("ArrowRight should be handled")
	}
	if g.Value != "B" || !g.Items[1].Checked {
		t.Fatalf("after Right value=%s b=%v", g.Value, g.Items[1].Checked)
	}
	g.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowLeft"})
	if g.Value != "A" {
		t.Fatalf("after Left value=%s", g.Value)
	}
	// Vertical orientation uses Up/Down.
	gv := kit.NewRadioGroup()
	gv.SetVertical(true)
	gv.SetStringOptions("X", "Y")
	gv.SetDefaultValue("X")
	if !gv.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"}) {
		t.Fatal("ArrowDown vertical")
	}
	if gv.Value != "Y" {
		t.Fatalf("vertical Down value=%s", gv.Value)
	}
}

func TestRadio_PRD_10_DemoBasic(t *testing.T) {
	// RDO-10 basic.tsx
	r := kit.NewRadio("Radio")
	n := 0
	r.SetOnChange(func(bool) { n++ })
	tree := core.NewTree(r.Node())
	clickRadio(t, tree, r)
	if !r.Checked || n != 1 {
		t.Fatalf("basic checked=%v n=%d", r.Checked, n)
	}
	// Re-click does not uncheck.
	clickRadio(t, tree, r)
	if !r.Checked || n != 1 {
		t.Fatalf("reclick checked=%v n=%d", r.Checked, n)
	}
}

func TestRadio_PRD_11_DemoDisabled(t *testing.T) {
	// RDO-11 disabled.tsx: off / on both disabled + toggle
	off := kit.NewRadio("Disabled")
	off.SetDefaultChecked(false)
	off.SetDisabled(true)
	on := kit.NewRadio("Disabled")
	on.SetDefaultChecked(true)
	on.SetDisabled(true)

	n := 0
	off.SetOnChange(func(bool) { n++ })
	on.SetOnChange(func(bool) { n++ })

	tree := core.NewTree(primitive.Column(off.Node(), on.Node()))
	clickRadio(t, tree, off)
	clickRadio(t, tree, on)
	if n != 0 {
		t.Fatalf("disabled demos fired n=%d", n)
	}
	if off.Checked || !on.Checked {
		t.Fatalf("state off=%v on=%v", off.Checked, on.Checked)
	}
	// Toggle disabled → can select off.
	off.SetDisabled(false)
	clickRadio(t, tree, off)
	if !off.Checked || n != 1 {
		t.Fatalf("enabled click checked=%v n=%d", off.Checked, n)
	}
}

func TestRadio_PRD_12_DemoRadioGroup(t *testing.T) {
	// RDO-12 radiogroup.tsx: options + controlled value
	g := kit.NewRadioGroup()
	g.SetOptions(
		kit.RadioOption{Label: "LineChart", Value: "1"},
		kit.RadioOption{Label: "DotChart", Value: "2"},
		kit.RadioOption{Label: "BarChart", Value: "3"},
		kit.RadioOption{Label: "PieChart", Value: "4"},
	)
	g.SetValue("1")
	var last string
	g.SetOnChange(func(v string) {
		last = v
		g.SetValue(v) // controlled parent
	})
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[2])
	if last != "3" || g.Value != "3" || !g.Items[2].Checked {
		t.Fatalf("value=%s last=%s bar=%v", g.Value, last, g.Items[2].Checked)
	}
}

func TestRadio_PRD_13_DemoVertical(t *testing.T) {
	// RDO-13 radiogroup-more.tsx: vertical + button vertical
	g := kit.NewRadioGroup()
	g.SetVertical(true)
	g.SetOptions(
		kit.RadioOption{Label: "Option A", Value: "1"},
		kit.RadioOption{Label: "Option B", Value: "2"},
		kit.RadioOption{Label: "Option C", Value: "3"},
		kit.RadioOption{Label: "More...", Value: "4"},
	)
	g.SetDefaultValue("1")
	if fl, ok := g.Node().(*primitive.Flex); !ok || fl.Axis != core.AxisVertical {
		t.Fatal("want vertical axis")
	}
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[1])
	if g.Value != "2" || !g.Items[1].Checked {
		t.Fatalf("vertical value=%s b=%v", g.Value, g.Items[1].Checked)
	}

	gb := kit.NewRadioGroup()
	gb.SetOptionType(kit.RadioOptionButton)
	gb.SetVertical(true)
	gb.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Title: "Orange"},
	)
	if fl, ok := gb.Node().(*primitive.Flex); !ok || fl.Axis != core.AxisVertical {
		t.Fatal("button vertical axis")
	}
	if gb.Items[2].Title != "Orange" {
		t.Fatalf("title=%q", gb.Items[2].Title)
	}
}

func TestRadio_PRD_14_DemoBlock(t *testing.T) {
	// RDO-14 radiogroup-block.tsx
	opts := []kit.RadioOption{
		{Label: "Apple", Value: "Apple"},
		{Label: "Pear", Value: "Pear"},
		{Label: "Orange", Value: "Orange"},
	}
	g1 := kit.NewRadioGroup()
	g1.SetBlock(true)
	g1.SetOptions(opts...)
	g1.SetDefaultValue("Apple")
	if !g1.Block {
		t.Fatal("block flag")
	}
	g2 := kit.NewRadioGroup()
	g2.SetBlock(true)
	g2.SetOptionType(kit.RadioOptionButton)
	g2.SetButtonStyle(kit.RadioButtonSolid)
	g2.SetOptions(opts...)
	g2.SetDefaultValue("Apple")
	_ = g2.Node().Layout(core.Loose(400, 80))
	ind := g2.Items[0].IndicatorNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	if !approxColor(ind.Background, th.Color(core.TokenColorPrimary), 0.05) {
		t.Fatalf("block solid fill=%v", ind.Background)
	}
	g3 := kit.NewRadioGroup()
	g3.SetBlock(true)
	g3.SetOptionType(kit.RadioOptionButton)
	g3.SetOptions(opts...)
	g3.SetDefaultValue("Pear")
	if !g3.Items[1].Checked {
		t.Fatal("default Pear")
	}
}

func TestRadio_PRD_15_DemoOptions(t *testing.T) {
	// RDO-15 radiogroup-options.tsx
	plain := []string{"Apple", "Pear", "Orange"}
	g1 := kit.NewRadioGroup()
	g1.SetStringOptions(plain...)
	g1.SetValue("Apple")
	var v1 string
	g1.SetOnChange(func(v string) {
		v1 = v
		g1.SetValue(v)
	})

	g2 := kit.NewRadioGroup()
	g2.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Disabled: true},
	)
	g2.SetValue("Apple")

	g3 := kit.NewRadioGroup()
	g3.SetOptionType(kit.RadioOptionButton)
	g3.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Title: "Orange"},
	)
	g3.SetValue("Apple")

	g4 := kit.NewRadioGroup()
	g4.SetOptionType(kit.RadioOptionButton)
	g4.SetButtonStyle(kit.RadioButtonSolid)
	g4.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Disabled: true},
	)
	g4.SetValue("Apple")

	tree := core.NewTree(primitive.Column(g1.Node(), g2.Node(), g3.Node(), g4.Node()))
	clickRadio(t, tree, g1.Items[1])
	if v1 != "Pear" || g1.Value != "Pear" {
		t.Fatalf("plain onChange=%s value=%s", v1, g1.Value)
	}
	n := 0
	g2.SetOnChange(func(string) { n++ })
	clickRadio(t, tree, g2.Items[2])
	if n != 0 {
		t.Fatal("disabled orange fired")
	}
	clickRadio(t, tree, g3.Items[1])
	g3.SetValue("Pear")
	if !g3.Items[1].Checked {
		t.Fatal("button options pear")
	}
}

func TestRadio_PRD_16_DemoRadioButton(t *testing.T) {
	// RDO-16 radiobutton.tsx: Radio.Button children + disabled item/group
	g1 := kit.NewRadioGroup()
	a := kit.NewRadioButton("Hangzhou")
	a.SetValue("a")
	b := kit.NewRadioButton("Shanghai")
	b.SetValue("b")
	c := kit.NewRadioButton("Beijing")
	c.SetValue("c")
	d := kit.NewRadioButton("Chengdu")
	d.SetValue("d")
	g1.Add(a, b, c, d)
	g1.SetDefaultValue("a")
	var last string
	g1.SetOnChange(func(v string) { last = v })
	tree := core.NewTree(g1.Node())
	clickRadio(t, tree, c)
	if last != "c" || g1.Value != "c" {
		t.Fatalf("value=%s last=%s", g1.Value, last)
	}

	g2 := kit.NewRadioGroup()
	a2 := kit.NewRadioButton("Hangzhou")
	a2.SetValue("a")
	b2 := kit.NewRadioButton("Shanghai")
	b2.SetValue("b")
	b2.SetDisabled(true)
	c2 := kit.NewRadioButton("Beijing")
	c2.SetValue("c")
	g2.Add(a2, b2, c2)
	g2.SetDefaultValue("a")
	n := 0
	g2.SetOnChange(func(string) { n++ })
	tree2 := core.NewTree(g2.Node())
	clickRadio(t, tree2, b2)
	if n != 0 {
		t.Fatal("disabled shanghai fired")
	}

	g3 := kit.NewRadioGroup()
	g3.SetDisabled(true)
	for _, lab := range []struct{ v, l string }{{"a", "Hangzhou"}, {"b", "Shanghai"}} {
		rb := kit.NewRadioButton(lab.l)
		rb.SetValue(lab.v)
		g3.Add(rb)
	}
	g3.SetDefaultValue("a")
	n3 := 0
	g3.SetOnChange(func(string) { n3++ })
	tree3 := core.NewTree(g3.Node())
	clickRadio(t, tree3, g3.Items[1])
	if n3 != 0 {
		t.Fatal("group disabled button fired")
	}
}

func TestRadio_PRD_17_DemoWithName(t *testing.T) {
	// RDO-17 radiogroup-with-name.tsx
	g := kit.NewRadioGroup()
	g.SetName("radiogroup")
	g.SetOptions(
		kit.RadioOption{Label: "A", Value: "1"},
		kit.RadioOption{Label: "B", Value: "2"},
		kit.RadioOption{Label: "C", Value: "3"},
		kit.RadioOption{Label: "D", Value: "4"},
	)
	g.SetDefaultValue("1")
	if g.Name != "radiogroup" {
		t.Fatalf("name=%q", g.Name)
	}
	if g.Node().Base().Role != "radiogroup" {
		t.Fatalf("role=%q", g.Node().Base().Role)
	}
	if g.Node().Base().Label != "radiogroup" {
		t.Fatalf("a11y label=%q want name", g.Node().Base().Label)
	}
	if !g.Items[0].Checked {
		t.Fatal("default 1")
	}
	tree := core.NewTree(g.Node())
	clickRadio(t, tree, g.Items[3])
	if g.Value != "4" {
		t.Fatalf("value=%s", g.Value)
	}
}

func TestRadio_PRD_18_TokenMetrics(t *testing.T) {
	// RDO-18 §6.2 key sizes
	th := kit.DefaultTheme()
	if th.SizeOr(core.TokenSizeIndicator, 0) != 16 {
		t.Fatalf("TokenSizeIndicator=%v want 16", th.SizeOr(core.TokenSizeIndicator, 0))
	}
	if th.SizeOr(core.TokenControlHeight, 0) != 32 {
		t.Fatalf("controlHeight=%v want 32", th.SizeOr(core.TokenControlHeight, 0))
	}
	if th.SizeOr(core.TokenControlHeightSM, 0) != 24 {
		t.Fatalf("controlHeightSM=%v want 24", th.SizeOr(core.TokenControlHeightSM, 0))
	}
	if th.SizeOr(core.TokenControlHeightLG, 0) != 40 {
		t.Fatalf("controlHeightLG=%v want 40", th.SizeOr(core.TokenControlHeightLG, 0))
	}
	if th.SizeOr(core.TokenFontSize, 0) != 14 {
		t.Fatalf("fontSize=%v want 14", th.SizeOr(core.TokenFontSize, 0))
	}
	if th.SizeOr(core.TokenBorderRadius, 0) != 6 {
		t.Fatalf("borderRadius=%v want 6", th.SizeOr(core.TokenBorderRadius, 0))
	}
	if th.SizeOr(core.TokenLineWidth, 0) != 1 {
		t.Fatalf("lineWidth=%v want 1", th.SizeOr(core.TokenLineWidth, 0))
	}
	r := kit.NewRadio("m")
	ind := r.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Width != 16 || ind.Height != 16 {
		t.Fatalf("geom %vx%v", ind.Width, ind.Height)
	}
}

func TestRadio_PRD_19_ThemeColors(t *testing.T) {
	// RDO-19 default skin uses Theme tokens
	r := kit.NewRadio("x")
	r.SetChecked(true)
	_ = r.Node().Layout(core.Loose(120, 40))
	// Indicator paints via PainterNode; label uses token text.
	lab := r.LabelNode().(*primitive.Text)
	th := kit.DefaultTheme()
	if !approxColor(lab.Color, th.Color(core.TokenColorText), 0.02) {
		t.Fatalf("label=%v want colorText", lab.Color)
	}
	// Button solid uses primary fill.
	g := kit.NewRadioGroup()
	g.SetOptionType(kit.RadioOptionButton)
	g.SetButtonStyle(kit.RadioButtonSolid)
	g.SetStringOptions("A", "B")
	g.SetDefaultValue("A")
	_ = g.Node().Layout(core.Loose(300, 80))
	ind := g.Items[0].IndicatorNode().(*primitive.Decorated)
	if !approxColor(ind.Background, th.Color(core.TokenColorPrimary), 0.02) {
		t.Fatalf("solid fill=%v want primary", ind.Background)
	}
	// No hard-coded brand as sole path: custom theme primary.
	custom := kit.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorPrimary] = th.Color(core.TokenColorSuccess)
	g2 := kit.NewRadioGroup()
	g2.SetTheme(custom)
	g2.SetOptionType(kit.RadioOptionButton)
	g2.SetButtonStyle(kit.RadioButtonSolid)
	g2.SetStringOptions("A")
	g2.SetDefaultValue("A")
	_ = g2.Node().Layout(core.Loose(200, 80))
	ind2 := g2.Items[0].IndicatorNode().(*primitive.Decorated)
	if !approxColor(ind2.Background, custom.Color(core.TokenColorPrimary), 0.05) {
		t.Fatalf("custom theme fill=%v", ind2.Background)
	}
}

func TestRadio_PRD_20_DisabledChrome(t *testing.T) {
	// RDO-20 disabled appearance: disabled colors; no hover highlight
	r := kit.NewRadio("x")
	r.SetDisabled(true)
	_ = r.Node().Layout(core.Loose(120, 40))
	lab := r.LabelNode().(*primitive.Text)
	th := kit.DefaultTheme()
	if !approxColor(lab.Color, th.Color(core.TokenColorDisabledText), 0.08) {
		t.Fatalf("disabled label=%v", lab.Color)
	}
	// Hover must not change disabled label to primary.
	r.Root.State.Hovered = true
	r.SyncState()
	if approxColor(lab.Color, th.Color(core.TokenColorPrimary), 0.05) {
		t.Fatal("disabled hover must not use primary label")
	}
	// Button disabled unchecked: disabled bg.
	g := kit.NewRadioGroup()
	g.SetOptionType(kit.RadioOptionButton)
	g.SetStringOptions("A", "B")
	g.SetDefaultValue("B")
	g.Items[0].SetDisabled(true)
	_ = g.Node().Layout(core.Loose(300, 80))
	ind := g.Items[0].IndicatorNode().(*primitive.Decorated)
	if !approxColor(ind.Background, th.Color(core.TokenColorDisabledBg), 0.08) {
		t.Fatalf("disabled btn fill=%v", ind.Background)
	}
}

func TestRadio_PRD_21_A11yFocus(t *testing.T) {
	// RDO-21 keyboard/focus main path
	r := kit.NewRadio("")
	r.SetTitle("opt-title")
	if r.Root.Base().Label != "opt-title" {
		t.Fatalf("title a11y fallback=%q", r.Root.Base().Label)
	}
	r.SetAriaLabel("custom")
	if r.Root.Base().Label != "custom" {
		t.Fatalf("aria=%q", r.Root.Base().Label)
	}
	if r.Root.Base().Role != "radio" {
		t.Fatalf("role=%q", r.Root.Base().Role)
	}
	if !r.Root.Focusable || !r.Root.ShowFocusRing {
		t.Fatal("focusable + focus ring required")
	}
	// Space/Enter activate standalone.
	r2 := kit.NewRadio("x")
	n := 0
	r2.SetOnChange(func(bool) { n++ })
	tree := core.NewTree(r2.Node())
	// Focus via click first.
	clickRadio(t, tree, r2)
	if n != 1 || !r2.Checked {
		t.Fatalf("after click n=%d checked=%v", n, r2.Checked)
	}
	// Already checked: Space no-ops (cannot uncheck).
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if n != 1 || !r2.Checked {
		t.Fatalf("Space on checked n=%d checked=%v", n, r2.Checked)
	}
	// Group a11y.
	g := kit.NewRadioGroup()
	g.SetAriaLabel("fruit")
	g.SetStringOptions("A", "B")
	if g.Node().Base().Role != "radiogroup" || g.Node().Base().Label != "fruit" {
		t.Fatalf("group role/label %q %q", g.Node().Base().Role, g.Node().Base().Label)
	}
}
