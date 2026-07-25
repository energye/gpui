package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/collapse.md §6.9 — P0 PRD cases (COL-01 … COL-21).
// L3/L4 (COL-22/23) and P1 (COL-24) are deferred.

func approxCollapse(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func collapseDogText() string {
	return "A dog is a type of domesticated animal. Known for its loyalty and faithfulness."
}

func collapseItems3() []kit.CollapseItem {
	body := func() core.Node { return kit.NewText(collapseDogText()).Node() }
	return []kit.CollapseItem{
		{Key: "1", Label: "This is panel header 1", Children: body()},
		{Key: "2", Label: "This is panel header 2", Children: body()},
		{Key: "3", Label: "This is panel header 3", Children: body()},
	}
}

func clickCollapseHeader(t *testing.T, c *kit.Collapse, key string) {
	t.Helper()
	p := c.HeaderPress(key)
	if p == nil {
		p = c.IconPress(key)
	}
	if p == nil {
		t.Fatalf("no pressable for key %q", key)
	}
	if p.Click != nil {
		p.Click()
	}
}

func TestCollapse_PRD_01_Defaults(t *testing.T) {
	// COL-01: NewCollapse 默认创建
	c := kit.NewCollapse(collapseItems3()...)
	if c.Size != kit.CollapseMiddle {
		t.Fatalf("Size=%v want middle", c.Size)
	}
	if !c.Bordered {
		t.Fatal("Bordered default false")
	}
	if c.Ghost || c.Accordion || c.DestroyOnHidden {
		t.Fatalf("flags ghost=%v accordion=%v destroy=%v", c.Ghost, c.Accordion, c.DestroyOnHidden)
	}
	if c.Collapsible != kit.CollapseCollapsibleHeader {
		t.Fatalf("Collapsible=%v", c.Collapsible)
	}
	if c.ExpandIconPlacement != kit.CollapseIconStart {
		t.Fatalf("placement=%v", c.ExpandIconPlacement)
	}
	if len(c.ActiveKeys()) != 0 {
		t.Fatalf("active=%v", c.ActiveKeys())
	}
	if c.Node() == nil || c.Root == nil {
		t.Fatal("nil node")
	}
	_ = c.Node().Layout(core.Loose(400, 300))
}

func TestCollapse_PRD_02_ExpandOne(t *testing.T) {
	// COL-02: 展开一项 → 内容可见；onChange
	var got []string
	c := kit.NewCollapse(collapseItems3()...)
	c.SetOnChange(func(keys []string) { got = append([]string(nil), keys...) })
	tree := core.NewTree(c.Node())
	tree.Layout(core.Size{Width: 400, Height: 400})
	clickCollapseHeader(t, c, "1")
	if !c.IsActive("1") {
		t.Fatal("not active")
	}
	if len(got) == 0 || got[0] != "1" {
		t.Fatalf("onChange=%v", got)
	}
	// Open panel contributes more height than all-collapsed.
	openH := c.Node().Layout(core.Loose(400, 800)).Height
	c.SetActive()
	closedH := c.Node().Layout(core.Loose(400, 800)).Height
	if openH <= closedH {
		t.Fatalf("openH=%v closedH=%v want open taller", openH, closedH)
	}
}

func TestCollapse_PRD_03_Accordion(t *testing.T) {
	// COL-03: accordion 开第二项 → 第一项收起
	c := kit.NewCollapse(collapseItems3()...)
	c.SetAccordion(true)
	c.SetActive("1")
	if !c.IsActive("1") || c.IsActive("2") {
		t.Fatalf("keys=%v", c.ActiveKeys())
	}
	clickCollapseHeader(t, c, "2")
	if c.IsActive("1") {
		t.Fatal("1 still open")
	}
	if !c.IsActive("2") {
		t.Fatal("2 not open")
	}
	if len(c.ActiveKeys()) != 1 {
		t.Fatalf("keys=%v", c.ActiveKeys())
	}
}

func TestCollapse_PRD_04_CollapsibleDisabled(t *testing.T) {
	// COL-04: collapsible=disabled → 不可点
	c := kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "locked", Children: kit.NewText("body").Node(),
	})
	c.SetCollapsible(kit.CollapseCollapsibleDisabled)
	c.SetDefaultActiveKey("1")
	before := append([]string(nil), c.ActiveKeys()...)
	// No header pressable when disabled.
	if p := c.HeaderPress("1"); p != nil && p.Click != nil {
		p.Click()
	}
	if p := c.IconPress("1"); p != nil && p.Click != nil {
		p.Click()
	}
	after := c.ActiveKeys()
	if len(before) != len(after) {
		t.Fatalf("toggled while disabled before=%v after=%v", before, after)
	}
	// Still open from default; cannot close via UI.
	if !c.IsActive("1") {
		t.Fatal("default active lost")
	}
}

func TestCollapse_PRD_05_Ghost(t *testing.T) {
	// COL-05: ghost → 无边框背景弱
	c := kit.NewCollapse(collapseItems3()...)
	c.SetGhost(true)
	c.SetDefaultActiveKey("1")
	_ = c.Node().Layout(core.Loose(400, 300))
	if c.Root.BorderWidth != 0 {
		t.Fatalf("border=%v", c.Root.BorderWidth)
	}
	if c.LineWidth() != 0 {
		t.Fatalf("LineWidth=%v", c.LineWidth())
	}
	// Transparent root bg (A==0).
	if c.Root.Background.A > 0.01 {
		t.Fatalf("ghost bg alpha=%v", c.Root.Background.A)
	}
}

func TestCollapse_PRD_06_Borderless(t *testing.T) {
	// COL-06: bordered=false → 无边框
	c := kit.NewCollapse(collapseItems3()...)
	c.SetBordered(false)
	c.SetDefaultActiveKey("1")
	_ = c.Node().Layout(core.Loose(400, 300))
	if c.Root.BorderWidth != 0 {
		t.Fatalf("border=%v", c.Root.BorderWidth)
	}
	if c.LineWidth() != 0 {
		t.Fatalf("LineWidth=%v", c.LineWidth())
	}
}

func TestCollapse_PRD_07_DestroyOnHidden(t *testing.T) {
	// COL-07: destroyOnHidden → 收起卸载
	body := kit.NewText("secret-body")
	c := kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "H", Children: body.Node(),
	})
	c.SetDestroyOnHidden(true)
	c.SetActive("1")
	_ = c.Node().Layout(core.Loose(400, 300))
	if !c.IsActive("1") {
		t.Fatal("not open")
	}
	c.SetActive() // close
	_ = c.Node().Layout(core.Loose(400, 300))
	// Closed + destroy: body slot should not be in tree under root.
	// Height should match header-only.
	closedH := c.Node().Layout(core.Loose(400, 300)).Height
	c.SetActive("1")
	openH := c.Node().Layout(core.Loose(400, 300)).Height
	if openH <= closedH {
		t.Fatalf("openH=%v closedH=%v", openH, closedH)
	}
}

func TestCollapse_PRD_08_ControlledActiveKey(t *testing.T) {
	// COL-08: 受控 activeKey → 外部优先
	var changes int
	c := kit.NewCollapse(collapseItems3()...)
	c.SetActiveKey("1")
	c.SetOnChange(func(keys []string) { changes++ })
	if !c.IsActive("1") {
		t.Fatal("controlled not applied")
	}
	clickCollapseHeader(t, c, "1") // would close if uncontrolled
	if !c.IsActive("1") {
		t.Fatal("controlled mutated on click")
	}
	if changes != 1 {
		t.Fatalf("onChange count=%d", changes)
	}
	// Parent applies new keys.
	c.SetActiveKey()
	if c.IsActive("1") {
		t.Fatal("still active after SetActiveKey clear")
	}
}

func TestCollapse_PRD_09_Keyboard(t *testing.T) {
	// COL-09: 键盘可激活头
	c := kit.NewCollapse(collapseItems3()...)
	tree := core.NewTree(c.Node())
	tree.Layout(core.Size{Width: 400, Height: 400})
	p := c.HeaderPress("1")
	if p == nil {
		t.Fatal("no header press")
	}
	if !p.CanFocus() {
		t.Fatal("header not focusable")
	}
	p.SetFocused(true)
	p.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if !c.IsActive("1") {
		t.Fatal("Enter did not expand")
	}
}

func TestCollapse_PRD_10_DemoBasic(t *testing.T) {
	// COL-10: basic.tsx
	c := kit.NewCollapse(collapseItems3()...)
	c.SetDefaultActiveKey("1")
	var last []string
	c.SetOnChange(func(keys []string) { last = keys })
	_ = c.Node().Layout(core.Loose(400, 400))
	if !c.IsActive("1") {
		t.Fatal("defaultActiveKey")
	}
	clickCollapseHeader(t, c, "2")
	if !c.IsActive("1") || !c.IsActive("2") {
		t.Fatalf("multi open keys=%v", c.ActiveKeys())
	}
	if len(last) < 2 {
		t.Fatalf("onChange=%v", last)
	}
}

func TestCollapse_PRD_11_DemoSize(t *testing.T) {
	// COL-11: size.tsx
	mk := func(sz kit.CollapseSize) *kit.Collapse {
		c := kit.NewCollapse(kit.CollapseItem{
			Key: "1", Label: "size panel", Children: kit.NewText(collapseDogText()).Node(),
		})
		c.SetSize(sz)
		return c
	}
	mid := mk(kit.CollapseMiddle)
	sm := mk(kit.CollapseSmall)
	lg := mk(kit.CollapseLarge)
	mv, mh := mid.HeaderPadding()
	sv, sh := sm.HeaderPadding()
	lv, lh := lg.HeaderPadding()
	if !approxCollapse(mv, kit.DefaultCollapseHeaderPadV, 0.5) || !approxCollapse(mh, kit.DefaultCollapseHeaderPadH, 0.5) {
		t.Fatalf("mid header pad %v,%v", mv, mh)
	}
	if !approxCollapse(sv, kit.DefaultCollapseHeaderPadVSM, 0.5) || !approxCollapse(sh, kit.DefaultCollapseHeaderPadHSM, 0.5) {
		t.Fatalf("sm header pad %v,%v", sv, sh)
	}
	if !approxCollapse(lv, kit.DefaultCollapseHeaderPadVLG, 0.5) || !approxCollapse(lh, kit.DefaultCollapseHeaderPadHLG, 0.5) {
		t.Fatalf("lg header pad %v,%v", lv, lh)
	}
	if !approxCollapse(sm.ContentPadding(), kit.DefaultCollapseContentPadSM, 0.5) {
		t.Fatalf("sm content=%v", sm.ContentPadding())
	}
	if !approxCollapse(lg.ContentPadding(), kit.DefaultCollapseContentPadLG, 0.5) {
		t.Fatalf("lg content=%v", lg.ContentPadding())
	}
	if !approxCollapse(lg.FontSize(), kit.DefaultCollapseFontSizeLG, 0.5) {
		t.Fatalf("lg font=%v", lg.FontSize())
	}
}

func TestCollapse_PRD_12_DemoAccordion(t *testing.T) {
	// COL-12: accordion.tsx
	c := kit.NewCollapse(collapseItems3()...)
	c.SetAccordion(true)
	_ = c.Node().Layout(core.Loose(400, 300))
	clickCollapseHeader(t, c, "1")
	clickCollapseHeader(t, c, "3")
	if c.IsActive("1") || !c.IsActive("3") {
		t.Fatalf("keys=%v", c.ActiveKeys())
	}
}

func TestCollapse_PRD_13_DemoMixNested(t *testing.T) {
	// COL-13: mix.tsx 面板嵌套
	inner := kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "This is panel nest panel", Children: kit.NewText(collapseDogText()).Node(),
	})
	inner.SetDefaultActiveKey("1")
	outer := kit.NewCollapse(
		kit.CollapseItem{Key: "1", Label: "This is panel header 1", Children: inner.Node()},
		kit.CollapseItem{Key: "2", Label: "This is panel header 2", Children: kit.NewText(collapseDogText()).Node()},
		kit.CollapseItem{Key: "3", Label: "This is panel header 3", Children: kit.NewText(collapseDogText()).Node()},
	)
	_ = outer.Node().Layout(core.Loose(480, 600))
	clickCollapseHeader(t, outer, "1")
	if !outer.IsActive("1") {
		t.Fatal("outer not open")
	}
	if !inner.IsActive("1") {
		t.Fatal("inner default lost")
	}
	sz := outer.Node().Layout(core.Loose(480, 800))
	if sz.Height < 40 {
		t.Fatalf("nested height=%v", sz)
	}
}

func TestCollapse_PRD_14_DemoBorderless(t *testing.T) {
	// COL-14: borderless.tsx
	c := kit.NewCollapse(collapseItems3()...)
	c.SetBordered(false)
	c.SetDefaultActiveKey("1")
	_ = c.Node().Layout(core.Loose(400, 300))
	if c.Bordered || c.Root.BorderWidth != 0 {
		t.Fatal("still bordered")
	}
}

func TestCollapse_PRD_15_DemoCustom(t *testing.T) {
	// COL-15: custom.tsx — expandIcon + borderless + item style
	th := kit.DefaultTheme()
	panelBG := th.Color(core.TokenColorFillSecondary)
	items := make([]kit.CollapseItem, 3)
	for i, key := range []string{"1", "2", "3"} {
		items[i] = kit.CollapseItem{
			Key:      key,
			Label:    "This is panel header " + key,
			Children: kit.NewText(collapseDogText()).Node(),
			Style:    kit.Style{Background: panelBG, Radius: kit.DefaultCollapseRadius, ForceRadius: true},
		}
	}
	c := kit.NewCollapse(items...)
	c.SetBordered(false)
	c.SetDefaultActiveKey("1")
	c.SetExpandIcon(func(active bool, _ kit.CollapseItem) core.Node {
		s := "▷"
		if active {
			s = "▼"
		}
		return primitive.NewText(s)
	})
	_ = c.Node().Layout(core.Loose(400, 400))
	if !c.IsActive("1") {
		t.Fatal("default")
	}
	clickCollapseHeader(t, c, "2")
	if !c.IsActive("2") {
		t.Fatal("custom icon click")
	}
}

func TestCollapse_PRD_16_DemoNoArrow(t *testing.T) {
	// COL-16: noarrow.tsx
	no := false
	c := kit.NewCollapse(
		kit.CollapseItem{Key: "1", Label: "with arrow", Children: kit.NewText("a").Node()},
		kit.CollapseItem{Key: "2", Label: "no arrow", Children: kit.NewText("b").Node(), ShowArrow: &no},
	)
	c.SetDefaultActiveKey("1")
	_ = c.Node().Layout(core.Loose(400, 300))
	clickCollapseHeader(t, c, "2")
	if !c.IsActive("2") {
		t.Fatal("no-arrow panel still collapsible via header")
	}
}

func TestCollapse_PRD_17_DemoExtra(t *testing.T) {
	// COL-17: extra.tsx — extra 不切换；placement end
	extraClicks := 0
	extraBtn := kit.NewButton("⚙")
	extraBtn.SetType(kit.ButtonText)
	extraBtn.SetOnClick(func() { extraClicks++ })
	c := kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "header", Children: kit.NewText("body").Node(), Extra: extraBtn.Node(),
	})
	c.SetDefaultActiveKey("1")
	c.SetExpandIconPlacement(kit.CollapseIconEnd)
	tree := core.NewTree(c.Node())
	tree.Layout(core.Size{Width: 400, Height: 300})
	if c.ExpandIconPlacement != kit.CollapseIconEnd {
		t.Fatal("placement")
	}
	// Extra click should not collapse.
	if extraBtn.Root != nil && extraBtn.Root.Click != nil {
		extraBtn.Root.Click()
	}
	if extraClicks != 1 {
		t.Fatalf("extra clicks=%d", extraClicks)
	}
	if !c.IsActive("1") {
		t.Fatal("extra collapsed panel")
	}
}

func TestCollapse_PRD_18_Metrics(t *testing.T) {
	// COL-18: §6.2 关键尺寸
	c := kit.NewCollapse(kit.CollapseItem{Key: "1", Label: "H", Children: kit.NewText("b").Node()})
	if !approxCollapse(c.Radius(), kit.DefaultCollapseRadius, 0.5) {
		t.Fatalf("radius=%v want %v", c.Radius(), kit.DefaultCollapseRadius)
	}
	if !approxCollapse(c.FontSize(), kit.DefaultCollapseFontSize, 0.5) {
		t.Fatalf("font=%v", c.FontSize())
	}
	if !approxCollapse(c.ContentPadding(), kit.DefaultCollapseContentPad, 0.5) {
		t.Fatalf("contentPad=%v", c.ContentPadding())
	}
	v, h := c.HeaderPadding()
	if !approxCollapse(v, kit.DefaultCollapseHeaderPadV, 0.5) || !approxCollapse(h, kit.DefaultCollapseHeaderPadH, 0.5) {
		t.Fatalf("header pad %v %v", v, h)
	}
	if !approxCollapse(float64(c.LineWidth()), kit.DefaultCollapseLineWidth, 0.5) {
		// bordered default
		t.Fatalf("lineW=%v", c.LineWidth())
	}
}

func TestCollapse_PRD_19_ThemeTokens(t *testing.T) {
	// COL-19: 默认皮颜色走 Theme，无硬编码品牌蓝作为唯一皮
	th := kit.DefaultTheme()
	th.Tokens.Colors[core.TokenColorFillSecondary] = render.RGBA{R: 0.1, G: 0.2, B: 0.3, A: 1}
	th.Tokens.Colors[core.TokenColorBorder] = render.Hex("#112233")
	c := kit.NewCollapse(kit.CollapseItem{Key: "1", Label: "H", Children: kit.NewText("b").Node()})
	c.SetTheme(th)
	_ = c.Node().Layout(core.Loose(300, 200))
	bg := c.Root.Background
	if !approxCollapse(float64(bg.R), 0.1, 0.01) || !approxCollapse(float64(bg.B), 0.3, 0.01) {
		t.Fatalf("bg=%v not theme fill", bg)
	}
	if c.Root.BorderColor != th.Color(core.TokenColorBorder) {
		t.Fatalf("border=%v want theme", c.Root.BorderColor)
	}
}

func TestCollapse_PRD_20_DisabledAppearance(t *testing.T) {
	// COL-20: disabled 外观（collapsible=disabled）
	c := kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "disabled head", Children: kit.NewText("b").Node(),
		Collapsible: kit.CollapseCollapsibleDisabled,
	})
	_ = c.Node().Layout(core.Loose(300, 200))
	if c.HeaderPress("1") != nil {
		t.Fatal("disabled should not expose header press")
	}
}

func TestCollapse_PRD_21_FocusRing(t *testing.T) {
	// COL-21: 可聚焦者 Focus ring；激活键有效
	c := kit.NewCollapse(collapseItems3()...)
	_ = c.Node().Layout(core.Loose(400, 300))
	p := c.HeaderPress("2")
	if p == nil || !p.ShowFocusRing {
		t.Fatal("focus ring off")
	}
	p.SetFocused(true)
	p.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: " "})
	if !c.IsActive("2") {
		t.Fatal("Space did not expand")
	}
}

func TestCollapse_RootStableOnToggle(t *testing.T) {
	c := kit.NewCollapse(kit.CollapseItem{Key: "1", Header: "H", Content: kit.NewText("c").Node()})
	r0 := c.Root
	c.SetActive("1")
	if c.Root != r0 {
		t.Fatal("Root replaced on SetActive")
	}
	clickCollapseHeader(t, c, "1")
	if c.Root != r0 {
		t.Fatal("Root replaced on toggle")
	}
}
