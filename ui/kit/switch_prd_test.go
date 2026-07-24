package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/switch.md §6.9 — P0 PRD cases (SW-01 … SW-16, SW-19…22).
// SW-17/18/25 are P1; SW-23/24 are L3/L4.

func clickSwitch(t *testing.T, tree *core.Tree, sw *kit.Switch) {
	t.Helper()
	tree.Layout(core.Size{Width: 200, Height: 80})
	abs := core.AbsoluteBounds(sw.Root)
	x, y := (abs.Min.X+abs.Max.X)/2, (abs.Min.Y+abs.Max.Y)/2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func TestSwitch_PRD_01_Defaults(t *testing.T) {
	// SW-01
	sw := kit.NewSwitch()
	if sw.Checked {
		t.Fatal("default Checked want false")
	}
	if sw.Size != kit.SwitchMedium {
		t.Fatalf("Size=%v want medium", sw.Size)
	}
	if sw.Disabled || sw.Loading || sw.Controlled {
		t.Fatalf("flags want false: disabled=%v loading=%v controlled=%v",
			sw.Disabled, sw.Loading, sw.Controlled)
	}
	if sw.Node() == nil || sw.Root == nil {
		t.Fatal("nil node")
	}
	if sw.Root.Base().Role != "switch" {
		t.Fatalf("role=%q want switch", sw.Root.Base().Role)
	}
}

func TestSwitch_PRD_02_ClickOn(t *testing.T) {
	// SW-02 / SW-S1
	sw := kit.NewSwitch()
	var got []bool
	sw.SetOnChange(func(v bool) { got = append(got, v) })
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if !sw.Checked {
		t.Fatal("want checked after click")
	}
	if len(got) != 1 || !got[0] {
		t.Fatalf("onChange=%v want [true]", got)
	}
	dec := sw.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	if !approxColor(dec.Background, th.Color(core.TokenColorPrimary), 0.05) {
		t.Fatalf("on track=%v want primary", dec.Background)
	}
}

func TestSwitch_PRD_03_ClickOff(t *testing.T) {
	// SW-03 / SW-S2
	sw := kit.NewSwitch()
	sw.SetChecked(true)
	var got []bool
	sw.SetOnChange(func(v bool) { got = append(got, v) })
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if sw.Checked {
		t.Fatal("want unchecked after click")
	}
	if len(got) != 1 || got[0] {
		t.Fatalf("onChange=%v want [false]", got)
	}
}

func TestSwitch_PRD_04_DisabledNoChange(t *testing.T) {
	// SW-04 / SW-S3
	sw := kit.NewSwitch()
	n := 0
	sw.SetOnChange(func(bool) { n++ })
	sw.SetDisabled(true)
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if n != 0 || sw.Checked {
		t.Fatalf("disabled fired change n=%d checked=%v", n, sw.Checked)
	}
}

func TestSwitch_PRD_05_LoadingNoChange(t *testing.T) {
	// SW-05 / SW-S4
	sw := kit.NewSwitch()
	n := 0
	sw.SetOnChange(func(bool) { n++ })
	sw.SetLoading(true)
	if sw.ChromeNode() == nil {
		t.Fatal("nil chrome while loading")
	}
	// Spinner should be present under thumb.
	track := sw.IndicatorNode().(*primitive.Decorated)
	if len(track.Children()) < 1 {
		t.Fatal("no thumb while loading")
	}
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if n != 0 || sw.Checked {
		t.Fatalf("loading fired change n=%d checked=%v", n, sw.Checked)
	}
}

func TestSwitch_PRD_06_Controlled(t *testing.T) {
	// SW-06 / SW-S5
	sw := kit.NewSwitch()
	sw.SetControlled(true)
	sw.SetChecked(false)
	var got []bool
	sw.SetOnChange(func(v bool) { got = append(got, v) })
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if sw.Checked {
		t.Fatal("controlled: local Checked must stay false until parent sets")
	}
	if len(got) != 1 || !got[0] {
		t.Fatalf("onChange=%v want [true]", got)
	}
	// Parent applies.
	sw.SetChecked(true)
	if !sw.Checked {
		t.Fatal("parent SetChecked(true) should stick")
	}
}

func TestSwitch_PRD_07_KeyboardSpace(t *testing.T) {
	// SW-07 / SW-S6 / SW-22
	sw := kit.NewSwitch()
	n := 0
	sw.SetOnChange(func(bool) { n++ })
	tree := core.NewTree(sw.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})
	// Focus via click first.
	clickSwitch(t, tree, sw)
	if n != 1 || !sw.Checked {
		t.Fatalf("after click n=%d checked=%v", n, sw.Checked)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if n != 2 || sw.Checked {
		t.Fatalf("Space n=%d checked=%v want n=2 unchecked", n, sw.Checked)
	}
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if n != 3 || !sw.Checked {
		t.Fatalf("Enter n=%d checked=%v want n=3 checked", n, sw.Checked)
	}
	if !sw.Root.ShowFocusRing {
		t.Fatal("ShowFocusRing want true (§6.6)")
	}
}

func TestSwitch_PRD_08_SizeSmall(t *testing.T) {
	// SW-08 / SW-S7
	sw := kit.NewSwitch()
	sw.SetSize(kit.SwitchSmall)
	ind := sw.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Height < 16-0.5 || ind.Height > 16+0.5 {
		t.Fatalf("small height=%v want 16±0.5", ind.Height)
	}
	if ind.Width < 28-0.5 || ind.Width > 28+1.5 {
		t.Fatalf("small width=%v want ≈28", ind.Width)
	}
}

func TestSwitch_PRD_09_SizeDefault(t *testing.T) {
	// SW-09 / SW-S8
	sw := kit.NewSwitch()
	ind := sw.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Height < 22-0.5 || ind.Height > 22+0.5 {
		t.Fatalf("default height=%v want 22±0.5", ind.Height)
	}
	if ind.Width < 44-0.5 || ind.Width > 44+0.5 {
		t.Fatalf("default width=%v want 44±0.5", ind.Width)
	}
}

func TestSwitch_PRD_10_ChildrenText(t *testing.T) {
	// SW-10 / SW-S9
	sw := kit.NewSwitch()
	sw.SetCheckedChildren("On")
	sw.SetUnCheckedChildren("Off")
	_ = sw.Node().Layout(core.Loose(120, 40))
	// Off initially.
	if sw.Checked {
		t.Fatal("want off")
	}
	// Toggle on and verify chrome path doesn't panic; a11y name updates.
	sw.SetChecked(true)
	if sw.Root.Base().Label != "On" {
		t.Fatalf("a11y label=%q want On", sw.Root.Base().Label)
	}
	sw.SetChecked(false)
	if sw.Root.Base().Label != "Off" {
		t.Fatalf("a11y label=%q want Off", sw.Root.Base().Label)
	}
}

func TestSwitch_PRD_11_PrimaryOnTrack(t *testing.T) {
	// SW-11 / SW-S10
	sw := kit.NewSwitch()
	sw.SetChecked(true)
	_ = sw.Node().Layout(core.Loose(100, 40))
	dec := sw.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	if !approxColor(dec.Background, th.Color(core.TokenColorPrimary), 0.05) {
		t.Fatalf("on track=%v want primary %v", dec.Background, th.Color(core.TokenColorPrimary))
	}
}

func TestSwitch_PRD_12_DemoBasic(t *testing.T) {
	// SW-12 basic.tsx: defaultChecked + onChange
	sw := kit.NewSwitch()
	sw.SetDefaultChecked(true)
	n := 0
	sw.SetOnChange(func(bool) { n++ })
	if !sw.Checked {
		t.Fatal("defaultChecked")
	}
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if sw.Checked || n != 1 {
		t.Fatalf("checked=%v n=%d", sw.Checked, n)
	}
}

func TestSwitch_PRD_13_DemoDisabled(t *testing.T) {
	// SW-13 disabled.tsx
	sw := kit.NewSwitch()
	sw.SetDefaultChecked(true)
	sw.SetDisabled(true)
	n := 0
	sw.SetOnChange(func(bool) { n++ })
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if !sw.Checked || n != 0 {
		t.Fatalf("disabled toggled checked=%v n=%d", sw.Checked, n)
	}
}

func TestSwitch_PRD_14_DemoText(t *testing.T) {
	// SW-14 text.tsx (string children)
	sw := kit.NewSwitch()
	sw.SetCheckedChildren("On")
	sw.SetUnCheckedChildren("Off")
	sw.SetDefaultChecked(true)
	_ = sw.Node().Layout(core.Loose(120, 40))
	if sw.Root.Base().Label != "On" {
		t.Fatalf("label=%q", sw.Root.Base().Label)
	}
	sw2 := kit.NewSwitch()
	sw2.SetCheckedChildren("1")
	sw2.SetUnCheckedChildren("0")
	sw2.SetDefaultChecked(true)
	_ = sw2.Node().Layout(core.Loose(120, 40))
}

func TestSwitch_PRD_15_DemoSize(t *testing.T) {
	// SW-15 size.tsx
	a := kit.NewSwitch()
	a.SetDefaultChecked(true)
	b := kit.NewSwitch()
	b.SetSize(kit.SwitchSmall)
	b.SetDefaultChecked(true)
	ia := a.IndicatorNode().(*primitive.Decorated)
	ib := b.IndicatorNode().(*primitive.Decorated)
	_ = ia.Layout(core.Loose(100, 100))
	_ = ib.Layout(core.Loose(100, 100))
	if ia.Height < 21.5 || ib.Height > 16.5 {
		t.Fatalf("sizes medium=%vx%v small=%vx%v", ia.Width, ia.Height, ib.Width, ib.Height)
	}
}

func TestSwitch_PRD_16_DemoLoading(t *testing.T) {
	// SW-16 loading.tsx
	a := kit.NewSwitch()
	a.SetLoading(true)
	a.SetDefaultChecked(true)
	b := kit.NewSwitch()
	b.SetSize(kit.SwitchSmall)
	b.SetLoading(true)
	_ = a.Node().Layout(core.Loose(100, 40))
	_ = b.Node().Layout(core.Loose(100, 40))
	n := 0
	a.SetOnChange(func(bool) { n++ })
	tree := core.NewTree(a.Node())
	clickSwitch(t, tree, a)
	if n != 0 {
		t.Fatal("loading must block")
	}
	// Tick spinner path.
	a.AttachTicker(tree)
	if !a.Tick(0.016) {
		t.Fatal("Tick while loading should return true")
	}
}

func TestSwitch_PRD_19_Metrics(t *testing.T) {
	// SW-19 L2
	sw := kit.NewSwitch()
	ind := sw.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Width != kit.DefaultSwitchTrackMinWidth || ind.Height != kit.DefaultSwitchTrackHeight {
		t.Fatalf("default track %vx%v", ind.Width, ind.Height)
	}
	sw.SetSize(kit.SwitchSmall)
	ind = sw.IndicatorNode().(*primitive.Decorated)
	_ = ind.Layout(core.Loose(100, 100))
	if ind.Height != kit.DefaultSwitchTrackHeightSM {
		t.Fatalf("sm height=%v", ind.Height)
	}
	if ind.Width < kit.DefaultSwitchTrackMinWidthSM-0.5 {
		t.Fatalf("sm width=%v", ind.Width)
	}
}

func TestSwitch_PRD_20_ThemeTokens(t *testing.T) {
	// SW-20
	sw := kit.NewSwitch()
	sw.SetChecked(true)
	_ = sw.Node().Layout(core.Loose(100, 40))
	on := sw.ChromeNode().(*primitive.Decorated).Background
	th := kit.DefaultTheme()
	if !approxColor(on, th.Color(core.TokenColorPrimary), 0.02) {
		t.Fatalf("on uses non-token primary: %v", on)
	}
	// Style override still allowed.
	custom := render.RGBA{R: 0.1, G: 0.8, B: 0.2, A: 1}
	sw.SetActiveColor(custom)
	_ = sw.Node().Layout(core.Loose(100, 40))
	if !approxColor(sw.ChromeNode().(*primitive.Decorated).Background, custom, 0.02) {
		t.Fatal("SetActiveColor ignored")
	}
}

func TestSwitch_PRD_21_DisabledChrome(t *testing.T) {
	// SW-21
	sw := kit.NewSwitch()
	sw.SetChecked(true)
	_ = sw.Node().Layout(core.Loose(100, 40))
	sw.SetDisabled(true)
	_ = sw.Node().Layout(core.Loose(100, 40))
	dec := sw.ChromeNode().(*primitive.Decorated)
	// hover must not switch to primaryHover
	sw.Root.SetHovered(true)
	sw.SyncState()
	dec2 := sw.ChromeNode().(*primitive.Decorated)
	th := kit.DefaultTheme()
	hover := th.Color(core.TokenColorPrimaryHover)
	if approxColor(dec2.Background, hover, 0.05) {
		t.Fatalf("disabled hover brightened to primaryHover: %v", dec2.Background)
	}
	_ = dec
}

func TestSwitch_PRD_22_FocusA11y(t *testing.T) {
	// SW-22
	sw := kit.NewSwitch()
	sw.SetAriaLabel("启用通知")
	_ = sw.Node()
	if sw.Root.Base().Label != "启用通知" {
		t.Fatalf("aria=%q", sw.Root.Base().Label)
	}
	if sw.Root.Base().Role != "switch" {
		t.Fatalf("role=%q", sw.Root.Base().Role)
	}
	if !sw.Root.Focusable || !sw.Root.ShowFocusRing {
		t.Fatal("focusable + ring required")
	}
	if sw.Root.FocusRingOutset < 1 {
		t.Fatalf("FocusRingOutset=%v want ≥1.5", sw.Root.FocusRingOutset)
	}
}

func TestSwitch_PRD_ValueAliases(t *testing.T) {
	// value / defaultValue aliases
	sw := kit.NewSwitch()
	sw.SetValue(true)
	if !sw.Checked || !sw.Controlled || !sw.Value() {
		t.Fatalf("SetValue: checked=%v controlled=%v", sw.Checked, sw.Controlled)
	}
	sw2 := kit.NewSwitch()
	sw2.SetDefaultValue(true)
	if !sw2.Checked || sw2.Controlled {
		t.Fatalf("SetDefaultValue: checked=%v controlled=%v", sw2.Checked, sw2.Controlled)
	}
}

func TestSwitch_PRD_OnClick(t *testing.T) {
	sw := kit.NewSwitch()
	var click, change []bool
	sw.SetOnClick(func(v bool) { click = append(click, v) })
	sw.SetOnChange(func(v bool) { change = append(change, v) })
	tree := core.NewTree(sw.Node())
	clickSwitch(t, tree, sw)
	if len(click) != 1 || !click[0] || len(change) != 1 || !change[0] {
		t.Fatalf("click=%v change=%v", click, change)
	}
}
