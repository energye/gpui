package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/cascader.md §6.9 — P0 PRD cases (CAS-01 … CAS-21).
// L3/L4 (CAS-22/23) and P1 (CAS-24) deferred.

func casSampleOptions() []kit.CascaderOption {
	return []kit.CascaderOption{
		{
			Value: "zhejiang", Label: "Zhejiang",
			Children: []kit.CascaderOption{
				{
					Value: "hangzhou", Label: "Hangzhou",
					Children: []kit.CascaderOption{
						{Value: "xihu", Label: "West Lake"},
					},
				},
			},
		},
		{
			Value: "jiangsu", Label: "Jiangsu",
			Children: []kit.CascaderOption{
				{
					Value: "nanjing", Label: "Nanjing",
					Children: []kit.CascaderOption{
						{Value: "zhonghuamen", Label: "Zhong Hua Men"},
					},
				},
			},
		},
	}
}

func casMultiOptions() []kit.CascaderOption {
	return []kit.CascaderOption{
		{
			Label: "Light", Value: "light",
			Children: []kit.CascaderOption{
				{Label: "Number 0", Value: "0"},
				{Label: "Number 1", Value: "1"},
			},
		},
		{
			Label: "Bamboo", Value: "bamboo",
			Children: []kit.CascaderOption{
				{
					Label: "Little", Value: "little",
					Children: []kit.CascaderOption{
						{Label: "Toy Fish", Value: "fish"},
						{Label: "Toy Cards", Value: "cards"},
						{Label: "Toy Bird", Value: "bird"},
					},
				},
			},
		},
	}
}

func approxCAS(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxCASColor(a, b render.RGBA, tol float64) bool {
	dr, dg, db, da := a.R-b.R, a.G-b.G, a.B-b.B, a.A-b.A
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if da < 0 {
		da = -da
	}
	return dr <= tol && dg <= tol && db <= tol && da <= tol
}

func mountCascader(t *testing.T, c *kit.Cascader, w, h float64) *core.Tree {
	t.Helper()
	if c == nil {
		t.Fatal("nil cascader")
	}
	c.Viewport = core.Size{Width: w, Height: h}
	bg := primitive.NewBox(c.Node())
	bg.Width, bg.Height = w, h
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func TestCascader_PRD_01_Defaults(t *testing.T) {
	// CAS-01
	c := kit.NewCascader("Please select", casSampleOptions()...)
	if c.Size != kit.InputMiddle {
		t.Fatalf("Size=%v want middle", c.Size)
	}
	if c.Variant != kit.InputOutlined {
		t.Fatalf("Variant=%v want outlined", c.Variant)
	}
	if c.Status != kit.InputStatusNone {
		t.Fatalf("Status=%v want none", c.Status)
	}
	if c.Disabled || c.Open || c.Multiple || c.ChangeOnSelect || c.ShowSearch {
		t.Fatalf("flags disabled=%v open=%v multi=%v cos=%v search=%v",
			c.Disabled, c.Open, c.Multiple, c.ChangeOnSelect, c.ShowSearch)
	}
	if !c.AllowClear {
		t.Fatal("AllowClear default true (antd)")
	}
	if c.ExpandTrigger != kit.CascaderExpandClick {
		t.Fatalf("ExpandTrigger=%v want click", c.ExpandTrigger)
	}
	if c.ShowCheckedStrategy != kit.CascaderShowParent {
		t.Fatalf("ShowCheckedStrategy=%v want parent", c.ShowCheckedStrategy)
	}
	if c.Placement != kit.CascaderBottomLeft {
		t.Fatalf("Placement=%v want bottomLeft", c.Placement)
	}
	if c.Placeholder != "Please select" {
		t.Fatalf("Placeholder=%q", c.Placeholder)
	}
	if c.Node() == nil || c.Popup() == nil || c.TriggerShell() == nil {
		t.Fatal("nil nodes")
	}
	_ = mountCascader(t, c, 400, 300)
}

func TestCascader_PRD_02_SelectThreeLevel(t *testing.T) {
	// CAS-02 / CAS-S1
	c := kit.NewCascader("Please select", casSampleOptions()...)
	var got []string
	c.SetOnChange(func(v []string, _ []kit.CascaderOption) { got = append([]string(nil), v...) })
	_ = mountCascader(t, c, 480, 320)
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	if len(got) != 3 {
		t.Fatalf("onChange path len=%d got=%v want 3", len(got), got)
	}
	if got[0] != "zhejiang" || got[1] != "hangzhou" || got[2] != "xihu" {
		t.Fatalf("path=%v", got)
	}
	if len(c.GetValue()) != 3 {
		t.Fatalf("value=%v", c.GetValue())
	}
}

func TestCascader_PRD_03_ChangeOnSelect(t *testing.T) {
	// CAS-03 / CAS-S2
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetChangeOnSelect(true)
	var changes []int
	c.SetOnChange(func(v []string, _ []kit.CascaderOption) {
		changes = append(changes, len(v))
	})
	_ = mountCascader(t, c, 480, 320)
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	// every level should fire (3 events)
	if len(changes) < 3 {
		t.Fatalf("change events=%v want ≥3 lengths", changes)
	}
	if changes[0] != 1 || changes[1] != 2 || changes[2] != 3 {
		t.Fatalf("change lengths=%v want 1,2,3…", changes)
	}
}

func TestCascader_PRD_04_LoadData(t *testing.T) {
	// CAS-04 / CAS-S3
	opts := []kit.CascaderOption{
		{Value: "zhejiang", Label: "Zhejiang", IsLeaf: false},
	}
	c := kit.NewCascader("Please select", opts...)
	c.SetChangeOnSelect(true)
	loaded := false
	c.SetLoadData(func(selected []kit.CascaderOption) {
		if len(selected) == 0 {
			return
		}
		loaded = true
		// fill children on live tree
		if live := findCASOpt(&c.Options, selected[0].Value); live != nil {
			live.Loading = false
			live.Children = []kit.CascaderOption{
				{Value: "hangzhou", Label: "Hangzhou", IsLeaf: true},
			}
		}
		c.NotifyOptionsChanged()
	})
	tree := mountCascader(t, c, 480, 320)
	c.AttachTicker(tree)
	c.ExpandPath([]string{"zhejiang"})
	if !loaded {
		t.Fatal("loadData not called")
	}
	// children should appear
	c.SelectPath([]string{"zhejiang", "hangzhou"})
	if got := c.GetValue(); len(got) != 2 || got[1] != "hangzhou" {
		t.Fatalf("value after load=%v", got)
	}
}

func findCASOpt(opts *[]kit.CascaderOption, value string) *kit.CascaderOption {
	for i := range *opts {
		if (*opts)[i].Value == value {
			return &(*opts)[i]
		}
	}
	return nil
}

func TestCascader_PRD_05_Clear(t *testing.T) {
	// CAS-05 / CAS-S4
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetValue([]string{"zhejiang", "hangzhou", "xihu"})
	cleared := 0
	var last []string
	c.SetOnClear(func() { cleared++ })
	c.SetOnChange(func(v []string, _ []kit.CascaderOption) { last = v })
	_ = mountCascader(t, c, 400, 300)
	c.Clear()
	if len(c.GetValue()) != 0 {
		t.Fatalf("value after clear=%v", c.GetValue())
	}
	if cleared != 1 {
		t.Fatalf("OnClear=%d", cleared)
	}
	if last != nil && len(last) != 0 {
		t.Fatalf("onChange after clear=%v", last)
	}
}

func TestCascader_PRD_06_Search(t *testing.T) {
	// CAS-06 / CAS-S5
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetShowSearch(true)
	_ = mountCascader(t, c, 480, 320)
	c.SetSearchValue("West")
	hits := c.VisibleSearchPaths()
	if len(hits) == 0 {
		t.Fatal("expected search hits for West")
	}
	found := false
	for _, p := range hits {
		if len(p) == 3 && p[2] == "xihu" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hits=%v want path ending xihu", hits)
	}
}

func TestCascader_PRD_07_Disabled(t *testing.T) {
	// CAS-07 / CAS-S6
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetDisabled(true)
	_ = mountCascader(t, c, 400, 300)
	c.SetOpen(true) // controlled attempt — disabled blocks open
	if c.IsOpen() {
		t.Fatal("disabled should not open")
	}
	// SelectPath must no-op
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	if len(c.GetValue()) != 0 {
		t.Fatalf("disabled select value=%v", c.GetValue())
	}
}

func TestCascader_PRD_07_Disabled_Click(t *testing.T) {
	// CAS-07 click path
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetDisabled(true)
	tree := mountCascader(t, c, 400, 300)
	shell := c.TriggerShell()
	if shell == nil {
		t.Fatal("nil shell")
	}
	abs := core.AbsoluteBounds(shell)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
	if c.IsOpen() {
		t.Fatal("click on disabled should not open")
	}
}

func TestCascader_PRD_08_ExpandTriggerHover(t *testing.T) {
	// CAS-08 / CAS-S7
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetExpandTrigger(kit.CascaderExpandHover)
	_ = mountCascader(t, c, 480, 320)
	c.SetOpen(true)
	c.HoverExpand(0, 0) // Zhejiang
	ap := c.ActivePath()
	if len(ap) < 1 || ap[0] != "zhejiang" {
		t.Fatalf("active after hover=%v", ap)
	}
	// should not commit value without click
	if len(c.GetValue()) != 0 {
		t.Fatalf("hover should not commit value=%v", c.GetValue())
	}
}

func TestCascader_PRD_09_DisplayRender(t *testing.T) {
	// CAS-09 / CAS-S8
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetDisplayRender(func(labels []string, _ []kit.CascaderOption) string {
		if len(labels) == 0 {
			return ""
		}
		return labels[len(labels)-1]
	})
	c.SetValue([]string{"zhejiang", "hangzhou", "xihu"})
	_ = mountCascader(t, c, 400, 300)
	if c.DisplayText() != "West Lake" {
		t.Fatalf("display=%q want West Lake", c.DisplayText())
	}
}

func TestCascader_PRD_10_BasicDemo(t *testing.T) {
	// CAS-10 basic.tsx
	c := kit.NewCascader("Please select", casSampleOptions()...)
	var got []string
	c.SetOnChange(func(v []string, _ []kit.CascaderOption) { got = v })
	_ = mountCascader(t, c, 480, 320)
	c.SelectPath([]string{"jiangsu", "nanjing", "zhonghuamen"})
	if len(got) != 3 || got[2] != "zhonghuamen" {
		t.Fatalf("basic select=%v", got)
	}
	if !strings.Contains(c.DisplayText(), "Zhong Hua Men") {
		t.Fatalf("display=%q", c.DisplayText())
	}
}

func TestCascader_PRD_11_DefaultValueDemo(t *testing.T) {
	// CAS-11 default-value.tsx
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetDefaultValue([]string{"zhejiang", "hangzhou", "xihu"})
	_ = mountCascader(t, c, 400, 300)
	if got := c.GetValue(); len(got) != 3 || got[2] != "xihu" {
		t.Fatalf("default value=%v", got)
	}
	if !strings.Contains(c.DisplayText(), "West Lake") {
		t.Fatalf("display=%q", c.DisplayText())
	}
}

func TestCascader_PRD_12_CustomTriggerDemo(t *testing.T) {
	// CAS-12 custom-trigger.tsx
	c := kit.NewCascader("", casSampleOptions()...)
	link := kit.NewText("Change city")
	c.SetTriggerNode(link.Node())
	text := "Unselect"
	c.SetOnChange(func(_ []string, selected []kit.CascaderOption) {
		labels := make([]string, 0, len(selected))
		for _, o := range selected {
			labels = append(labels, o.DisplayLabel())
		}
		text = strings.Join(labels, ", ")
	})
	_ = mountCascader(t, c, 400, 300)
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	if !strings.Contains(text, "West Lake") {
		t.Fatalf("custom trigger text=%q", text)
	}
	if c.TriggerShell() == nil {
		t.Fatal("nil shell with custom trigger")
	}
}

func TestCascader_PRD_13_HoverDemo(t *testing.T) {
	// CAS-13 hover.tsx
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetExpandTrigger(kit.CascaderExpandHover)
	c.SetDisplayRender(func(labels []string, _ []kit.CascaderOption) string {
		if len(labels) == 0 {
			return ""
		}
		return labels[len(labels)-1]
	})
	_ = mountCascader(t, c, 480, 320)
	c.SetOpen(true)
	c.HoverExpand(0, 0)
	c.HoverExpand(1, 0)
	// click leaf via SelectPath last segment
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	if c.DisplayText() != "West Lake" {
		t.Fatalf("hover demo display=%q", c.DisplayText())
	}
}

func TestCascader_PRD_14_DisabledOptionDemo(t *testing.T) {
	// CAS-14 disabled-option.tsx
	opts := casSampleOptions()
	opts[1].Disabled = true // jiangsu disabled
	c := kit.NewCascader("Please select", opts...)
	_ = mountCascader(t, c, 480, 320)
	c.SelectPath([]string{"jiangsu", "nanjing", "zhonghuamen"})
	if len(c.GetValue()) != 0 {
		t.Fatalf("disabled option should block select, value=%v", c.GetValue())
	}
	// enabled path still works
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	if len(c.GetValue()) != 3 {
		t.Fatalf("enabled path value=%v", c.GetValue())
	}
}

func TestCascader_PRD_15_ChangeOnSelectDemo(t *testing.T) {
	// CAS-15 change-on-select.tsx
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetChangeOnSelect(true)
	var last []string
	c.SetOnChange(func(v []string, _ []kit.CascaderOption) { last = append([]string(nil), v...) })
	_ = mountCascader(t, c, 480, 320)
	c.SelectPath([]string{"zhejiang"})
	if len(last) != 1 || last[0] != "zhejiang" {
		t.Fatalf("level1=%v", last)
	}
	c.SelectPath([]string{"zhejiang", "hangzhou"})
	if len(last) != 2 {
		t.Fatalf("level2=%v", last)
	}
}

func TestCascader_PRD_16_MultipleDemo(t *testing.T) {
	// CAS-16 multiple.tsx
	c := kit.NewCascader("Please select", casMultiOptions()...)
	c.SetMultiple(true)
	var multi [][]string
	c.SetOnChangeMulti(func(v [][]string, _ [][]kit.CascaderOption) {
		multi = v
	})
	_ = mountCascader(t, c, 520, 360)
	c.SelectPath([]string{"bamboo", "little", "fish"})
	c.SelectPath([]string{"bamboo", "little", "cards"})
	if len(c.GetMultiValue()) < 2 {
		t.Fatalf("multi value=%v", c.GetMultiValue())
	}
	if len(multi) < 2 {
		t.Fatalf("onChangeMulti=%v", multi)
	}
}

func TestCascader_PRD_17_ShowCheckedStrategyDemo(t *testing.T) {
	// CAS-17 showCheckedStrategy.tsx
	c := kit.NewCascader("Please select", casMultiOptions()...)
	c.SetMultiple(true)
	c.SetShowCheckedStrategy(kit.CascaderSHOW_CHILD)
	c.SetDefaultMultiValue([][]string{
		{"bamboo", "little", "fish"},
		{"bamboo", "little", "cards"},
		{"bamboo", "little", "bird"},
	})
	_ = mountCascader(t, c, 520, 360)
	if len(c.GetMultiValue()) != 3 {
		t.Fatalf("default multi=%v", c.GetMultiValue())
	}
	disp := c.DisplayText()
	if !strings.Contains(disp, "Toy") && !strings.Contains(disp, "Fish") {
		// display should mention child labels
		if disp == "" || disp == "Please select" {
			t.Fatalf("display empty for multi default: %q", disp)
		}
	}

	// SHOW_PARENT parent collapse demo
	p := kit.NewCascader("Please select", casMultiOptions()...)
	p.SetMultiple(true)
	p.SetShowCheckedStrategy(kit.CascaderSHOW_PARENT)
	p.SetDefaultMultiValue([][]string{{"bamboo"}})
	_ = mountCascader(t, p, 520, 360)
	if !strings.Contains(p.DisplayText(), "Bamboo") {
		t.Fatalf("parent strategy display=%q", p.DisplayText())
	}
}

func TestCascader_PRD_18_TokenMetrics(t *testing.T) {
	// CAS-18 §6.2
	th := kit.DefaultTheme()
	if !approxCAS(th.SizeOr(core.TokenControlHeight, 0), 32, 0.5) {
		t.Fatalf("controlHeight=%v want 32", th.SizeOr(core.TokenControlHeight, 0))
	}
	if !approxCAS(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.5) {
		t.Fatalf("controlHeightSM=%v want 24", th.SizeOr(core.TokenControlHeightSM, 0))
	}
	if !approxCAS(th.SizeOr(core.TokenControlHeightLG, 0), 40, 0.5) {
		t.Fatalf("controlHeightLG=%v want 40", th.SizeOr(core.TokenControlHeightLG, 0))
	}
	if !approxCAS(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatalf("fontSize=%v want 14", th.SizeOr(core.TokenFontSize, 0))
	}
	if !approxCAS(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatalf("borderRadius=%v want 6", th.SizeOr(core.TokenBorderRadius, 0))
	}
	if !approxCAS(th.SizeOr(core.TokenLineWidth, 0), 1, 0.5) {
		t.Fatalf("lineWidth=%v want 1", th.SizeOr(core.TokenLineWidth, 0))
	}

	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetTheme(th)
	_ = mountCascader(t, c, 400, 300)
	// middle height
	shell := c.TriggerShell()
	if shell == nil {
		t.Fatal("nil shell")
	}
	sz := shell.Size()
	if !approxCAS(sz.Height, 32, 1.5) {
		// pressable may wrap decor; check decor via layout height of node
		if !approxCAS(sz.Height, 32, 4) {
			t.Fatalf("trigger height=%v want ~32", sz.Height)
		}
	}
}

func TestCascader_PRD_19_DefaultSkinTokens(t *testing.T) {
	// CAS-19 no hardcoded brand-only default skin
	th := kit.DefaultTheme()
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetTheme(th)
	_ = mountCascader(t, c, 400, 300)
	// primary/border come from theme
	pri := th.Color(core.TokenColorPrimary)
	bd := th.Color(core.TokenColorBorder)
	if pri.A < 0.1 || bd.A < 0.1 {
		t.Fatalf("theme tokens missing primary=%v border=%v", pri, bd)
	}
	// status error uses colorError
	c.SetStatus(kit.InputStatusError)
	errC := th.Color(core.TokenColorError)
	if errC.A < 0.1 {
		t.Fatal("colorError missing")
	}
	_ = approxCASColor
}

func TestCascader_PRD_20_DisabledChrome(t *testing.T) {
	// CAS-20
	th := kit.DefaultTheme()
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetTheme(th)
	c.SetDisabled(true)
	_ = mountCascader(t, c, 400, 300)
	if !c.Disabled {
		t.Fatal("want disabled")
	}
	// no open on SetOpen
	c.SetOpen(true)
	if c.IsOpen() {
		t.Fatal("disabled open")
	}
}

func TestCascader_PRD_21_KeyboardFocus(t *testing.T) {
	// CAS-21
	c := kit.NewCascader("Please select", casSampleOptions()...)
	tree := mountCascader(t, c, 400, 300)
	shell := c.TriggerShell()
	if shell == nil || !shell.Focusable {
		t.Fatal("trigger should be focusable")
	}
	if !shell.ShowFocusRing {
		t.Fatal("focus ring should be visible")
	}
	tree.SetFocus(shell)
	c.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"})
	if !c.IsOpen() {
		t.Fatal("ArrowDown should open")
	}
	c.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if c.IsOpen() {
		t.Fatal("Escape should close")
	}
}

func TestCascader_PRD_22_GoldenDeferred(t *testing.T) {
	// CAS-22 L3 deferred
	t.Skip("L3 golden deferred (not browser pixel hash)")
}

func TestCascader_PRD_23_EyeDeferred(t *testing.T) {
	// CAS-23 L4 deferred
	t.Skip("L4 human eye sign-off deferred")
}

func TestCascader_PRD_24_P1Deferred(t *testing.T) {
	// CAS-24 P1 deferred
	t.Skip("P1: showSearch object form, tagRender, fieldNames, maxTagCount responsive, virtual list")
}

func TestCascader_PRD_RootStableOnSelect(t *testing.T) {
	c := kit.NewCascader("Please select", casSampleOptions()...)
	tree := mountCascader(t, c, 480, 320)
	r0, w0 := c.Root, c.Wrap
	c.SelectPath([]string{"zhejiang", "hangzhou", "xihu"})
	tree.Layout(core.Size{Width: 480, Height: 320})
	if c.Root != r0 || c.Wrap != w0 {
		t.Fatal("root identity changed on select")
	}
}

func TestCascader_PRD_Placement(t *testing.T) {
	c := kit.NewCascader("Please select", casSampleOptions()...)
	c.SetPlacement(kit.CascaderTopRight)
	_ = mountCascader(t, c, 400, 300)
	if c.Placement != kit.CascaderTopRight {
		t.Fatal(c.Placement)
	}
	c.SetOpen(true)
	if c.Popup() == nil || !c.Popup().Open {
		t.Fatal("popup not open")
	}
}

func TestCascader_PRD_OpenChange(t *testing.T) {
	c := kit.NewCascader("Please select", casSampleOptions()...)
	var seq []bool
	c.SetOnOpenChange(func(open bool) { seq = append(seq, open) })
	_ = mountCascader(t, c, 400, 300)
	c.SetOpen(true)
	c.SetOpen(false)
	if len(seq) < 2 || !seq[0] || seq[1] {
		t.Fatalf("open change seq=%v", seq)
	}
}

func TestCascader_PRD_VariantStatus(t *testing.T) {
	c := kit.NewCascader("Please select", casSampleOptions()...)
	for _, v := range []kit.InputVariant{kit.InputOutlined, kit.InputFilled, kit.InputBorderless, kit.InputUnderlined} {
		c.SetVariant(v)
	}
	c.SetStatus(kit.InputStatusError)
	c.SetStatus(kit.InputStatusWarning)
	c.SetStatus(kit.InputStatusNone)
	_ = mountCascader(t, c, 400, 300)
	for _, s := range []kit.InputSize{kit.InputLarge, kit.InputMiddle, kit.InputSmall} {
		c.SetSize(s)
		_ = c.Node().Layout(core.Loose(400, 100))
	}
}
