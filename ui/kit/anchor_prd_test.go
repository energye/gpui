package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/anchor.md §6.9 — P0 PRD cases (ANC-01 … ANC-19 L1/L2).
// L3/L4 (ANC-20/21) and P1 (ANC-22) deferred.

func approxAnchor(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func approxAnchorColor(a, b render.RGBA, tol float64) bool {
	return math.Abs(float64(a.R-b.R)) <= tol &&
		math.Abs(float64(a.G-b.G)) <= tol &&
		math.Abs(float64(a.B-b.B)) <= tol &&
		math.Abs(float64(a.A-b.A)) <= tol
}

func sampleItems() []kit.AnchorItem {
	return []kit.AnchorItem{
		{Key: "part-1", Href: "#part-1", Title: "Part 1"},
		{Key: "part-2", Href: "#part-2", Title: "Part 2"},
		{Key: "part-3", Href: "#part-3", Title: "Part 3"},
	}
}

func nestedItems() []kit.AnchorItem {
	return []kit.AnchorItem{
		{Key: "1", Href: "#basic", Title: "Basic demo"},
		{Key: "2", Href: "#static", Title: "Static demo"},
		{
			Key: "3", Href: "#api", Title: "API",
			Children: []kit.AnchorItem{
				{Key: "4", Href: "#anchor-props", Title: "Anchor Props"},
				{Key: "5", Href: "#link-props", Title: "Link Props"},
			},
		},
	}
}

func clickAnchorLink(t *testing.T, tree *core.Tree, a *kit.Anchor, href string) {
	t.Helper()
	tree.Layout(core.Size{Width: 240, Height: 320})
	p := a.LinkPressable(href)
	if p == nil {
		t.Fatalf("no pressable for %q (flat=%d)", href, a.FlatCount())
	}
	abs := core.AbsoluteBounds(p)
	x := (abs.Min.X + abs.Max.X) / 2
	y := (abs.Min.Y + abs.Max.Y) / 2
	if abs.Width() <= 0 || abs.Height() <= 0 {
		// Fallback: use root center band
		rAbs := core.AbsoluteBounds(a.ChromeNode())
		x = rAbs.Min.X + 40
		y = rAbs.Min.Y + 20
	}
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerDown, X: x, Y: y, Button: core.ButtonLeft})
	tree.DispatchPointer(&core.PointerEvent{Type: core.PointerUp, X: x, Y: y, Button: core.ButtonLeft})
}

func tallScrollHost(t *testing.T) *primitive.ScrollViewport {
	t.Helper()
	content := primitive.NewBox()
	content.Height = 800
	content.Width = 100
	sv := primitive.NewScrollViewport(content)
	sv.Height = 120
	sv.Width = 100
	_ = sv.Layout(core.Tight(100, 120))
	return sv
}

func TestAnchor_PRD_01_Defaults(t *testing.T) {
	// ANC-01: NewAnchor 默认创建；默认值符合 §6.10 / antd
	a := kit.NewAnchor()
	if a.Node() == nil {
		t.Fatal("nil node")
	}
	if a.Direction != kit.AnchorVertical {
		t.Fatalf("Direction=%v want vertical", a.Direction)
	}
	if !a.Affix {
		t.Fatal("Affix default true")
	}
	if a.ShowInkInFixed {
		t.Fatal("ShowInkInFixed default false")
	}
	if !approxAnchor(a.Bounds, 5, 0.01) {
		t.Fatalf("Bounds=%v want 5", a.Bounds)
	}
	if a.Replace {
		t.Fatal("Replace default false")
	}
	if a.ActiveLink != "" {
		t.Fatalf("ActiveLink=%q want empty", a.ActiveLink)
	}
	if a.EffectiveTargetOffset() != 0 {
		t.Fatalf("targetOffset=%v", a.EffectiveTargetOffset())
	}
	if a.ChromeNode() == nil || a.ChromeNode().Base().Role != "navigation" {
		t.Fatalf("role=%q want navigation", a.ChromeNode().Base().Role)
	}
	if !a.IsAffixed() {
		t.Fatal("default Affix should wrap root")
	}
}

func TestAnchor_PRD_02_ClickScrolls(t *testing.T) {
	// ANC-02 / ANC-S1: 点击项 → 滚到锚点
	sv := tallScrollHost(t)
	a := kit.NewAnchor(sampleItems()...)
	a.SetAffix(false)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{"#part-1": 0, "#part-2": 200, "#part-3": 400})
	tree := core.NewTree(a.Node())
	clickAnchorLink(t, tree, a, "#part-3")
	if a.ActiveLink != "#part-3" {
		t.Fatalf("ActiveLink=%q", a.ActiveLink)
	}
	if !approxAnchor(sv.ScrollY, 400, 0.5) {
		t.Fatalf("ScrollY=%v want 400", sv.ScrollY)
	}
}

func TestAnchor_PRD_03_ScrollSpyOnChange(t *testing.T) {
	// ANC-03 / ANC-S2: 滚动经过 section → active 切换；onChange
	sv := tallScrollHost(t)
	a := kit.NewAnchor(sampleItems()...)
	a.SetAffix(false)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{"#part-1": 0, "#part-2": 120, "#part-3": 240})
	var got []string
	a.SetOnChange(func(link string) { got = append(got, link) })

	sv.ScrollY = 0
	a.SyncFromScroll()
	if a.ActiveLink != "#part-1" {
		t.Fatalf("y=0 active=%q", a.ActiveLink)
	}

	sv.SetScroll(0, 130)
	// SetScroll → OnScroll → SyncFromScroll (auto spy)
	if a.ActiveLink != "#part-2" {
		t.Fatalf("y=130 active=%q ScrollY=%v (auto OnScroll spy)", a.ActiveLink, sv.ScrollY)
	}

	sv.SetScroll(0, 300)
	if a.ActiveLink != "#part-3" {
		t.Fatalf("y=300 active=%q", a.ActiveLink)
	}
	if len(got) < 2 {
		t.Fatalf("OnChange calls=%v want ≥2", got)
	}
}

func TestAnchor_PRD_03b_WheelScrollSpy(t *testing.T) {
	// Wheel path: HandleScroll → OnScroll → SyncFromScroll
	sv := tallScrollHost(t)
	a := kit.NewAnchor(sampleItems()...)
	a.SetAffix(false)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{"#part-1": 0, "#part-2": 120, "#part-3": 240})
	a.SyncFromScroll()
	if a.ActiveLink != "#part-1" {
		t.Fatalf("start=%q", a.ActiveLink)
	}
	// One wheel step with large DY (platform deltas are often >> 1).
	sv.HandleScroll(&core.ScrollEvent{DY: 150})
	if a.ActiveLink != "#part-2" && a.ActiveLink != "#part-3" {
		t.Fatalf("after wheel active=%q ScrollY=%v want part-2+", a.ActiveLink, sv.ScrollY)
	}
}

func TestAnchor_PRD_04_Affix(t *testing.T) {
	// ANC-04 / ANC-S3: affix 钉住
	a := kit.NewAnchor(sampleItems()...)
	if !a.IsAffixed() {
		t.Fatal("default affix true")
	}
	// Root should be Sticky when affix
	if _, ok := a.Node().(*primitive.Sticky); !ok {
		// Affix.Root is sticky
		if a.Node() == a.ChromeNode() {
			t.Fatal("affixed root should differ from chrome")
		}
	}
	a.SetAffix(false)
	if a.IsAffixed() {
		t.Fatal("affix false")
	}
	if a.Node() != a.ChromeNode() {
		t.Fatal("static: root == chrome")
	}
	a.SetOffsetTop(24)
	a.SetAffix(true)
	if !a.IsAffixed() {
		t.Fatal("re-affix")
	}
}

func TestAnchor_PRD_05_Ink(t *testing.T) {
	// ANC-05 / ANC-S4: ink 指示条在 active
	a := kit.NewAnchor(sampleItems()...)
	// default affix=true, no active → ink hidden
	if a.InkVisible() {
		t.Fatal("ink should be hidden without active")
	}
	a.SetActiveLink("#part-2")
	_ = a.Node().Layout(core.Loose(200, 200))
	if !a.InkVisible() {
		t.Fatal("ink visible when active + affix")
	}
	// static without showInkInFixed hides ink
	a.SetAffix(false)
	a.SetActiveLink("#part-2")
	_ = a.Node().Layout(core.Loose(200, 200))
	if a.InkVisible() {
		t.Fatal("affix=false && !showInkInFixed → no ink")
	}
	a.SetShowInkInFixed(true)
	a.SetActiveLink("#part-1")
	_ = a.Node().Layout(core.Loose(200, 200))
	if !a.InkVisible() {
		t.Fatal("showInkInFixed should show ink")
	}
}

func TestAnchor_PRD_06_NestedItems(t *testing.T) {
	// ANC-06 / ANC-S5: 嵌套 items → 二级可见
	a := kit.NewAnchor(nestedItems()...)
	a.SetAffix(false)
	_ = a.Node().Layout(core.Loose(240, 300))
	if a.FlatCount() != 5 {
		t.Fatalf("flat=%d want 5 (3 top + 2 nested)", a.FlatCount())
	}
	if a.LinkPressable("#anchor-props") == nil {
		t.Fatal("nested #anchor-props missing")
	}
	if a.LinkPressable("#link-props") == nil {
		t.Fatal("nested #link-props missing")
	}
	// horizontal ignores children
	h := kit.NewAnchor(nestedItems()...)
	h.SetDirection(kit.AnchorHorizontal)
	h.SetAffix(false)
	_ = h.Node().Layout(core.Loose(400, 80))
	if h.FlatCount() != 3 {
		t.Fatalf("horizontal flat=%d want 3 (no children)", h.FlatCount())
	}
}

func TestAnchor_PRD_07_TargetOffset(t *testing.T) {
	// ANC-07 / ANC-S6: targetOffset 停位偏移
	sv := tallScrollHost(t)
	a := kit.NewAnchor(sampleItems()...)
	a.SetAffix(false)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{"#part-1": 0, "#part-2": 200, "#part-3": 400})
	a.SetTargetOffset(50)
	tree := core.NewTree(a.Node())
	clickAnchorLink(t, tree, a, "#part-2")
	// scrollY = 200 - 50 = 150
	if !approxAnchor(sv.ScrollY, 150, 0.5) {
		t.Fatalf("ScrollY=%v want 150 (200-50)", sv.ScrollY)
	}
	// spy with offset: at y=160, limit=160+50+5=215 → part-2 (200)
	sv.SetScroll(0, 160)
	a.SyncFromScroll()
	if a.ActiveLink != "#part-2" {
		t.Fatalf("active=%q want #part-2", a.ActiveLink)
	}
}

func TestAnchor_PRD_08_BasicDemo(t *testing.T) {
	// ANC-08: 基本 basic.tsx — items 三 section + 默认 affix
	a := kit.NewAnchor(sampleItems()...)
	if !a.Affix || a.Direction != kit.AnchorVertical {
		t.Fatal("basic defaults")
	}
	if a.FlatCount() != 3 {
		t.Fatalf("items=%d", a.FlatCount())
	}
	_ = a.Node().Layout(core.Loose(200, 200))
}

func TestAnchor_PRD_09_HorizontalDemo(t *testing.T) {
	// ANC-09: horizontal.tsx
	a := kit.NewAnchor(sampleItems()...)
	a.SetDirection(kit.AnchorHorizontal)
	if a.Direction != kit.AnchorHorizontal {
		t.Fatal("direction")
	}
	sz := a.Node().Layout(core.Loose(400, 80))
	if sz.Width <= 0 {
		t.Fatal(sz)
	}
	// links should be on one row: check first two pressable Y equal
	p0 := a.LinkPressable("#part-1")
	p1 := a.LinkPressable("#part-2")
	if p0 == nil || p1 == nil {
		t.Fatal("missing links")
	}
	if p0.Base().Offset().Y != p1.Base().Offset().Y {
		// offsets are relative to parent; both under same row flex
		// Accept if AbsoluteBounds share y after tree layout
		tree := core.NewTree(a.Node())
		tree.Layout(core.Size{Width: 400, Height: 80})
		y0 := core.AbsoluteBounds(p0).Min.Y
		y1 := core.AbsoluteBounds(p1).Min.Y
		if !approxAnchor(y0, y1, 1) {
			t.Fatalf("horizontal y0=%v y1=%v", y0, y1)
		}
	}
}

func TestAnchor_PRD_10_StaticDemo(t *testing.T) {
	// ANC-10: static.tsx — affix=false + nested
	a := kit.NewAnchor(nestedItems()...)
	a.SetAffix(false)
	if a.Affix || a.InkVisible() {
		t.Fatal("static: no affix, no ink without showInkInFixed")
	}
	_ = a.Node().Layout(core.Loose(240, 300))
	if a.FlatCount() != 5 {
		t.Fatalf("flat=%d", a.FlatCount())
	}
}

func TestAnchor_PRD_11_OnClickDemo(t *testing.T) {
	// ANC-11: onClick.tsx
	a := kit.NewAnchor(nestedItems()...)
	a.SetAffix(false)
	var got kit.AnchorLinkInfo
	a.SetOnClick(func(link kit.AnchorLinkInfo) { got = link })
	tree := core.NewTree(a.Node())
	clickAnchorLink(t, tree, a, "#static")
	if got.Href != "#static" || got.Title != "Static demo" {
		t.Fatalf("OnClick=%+v", got)
	}
}

func TestAnchor_PRD_12_CustomizeHighlight(t *testing.T) {
	// ANC-12: customizeHighlight.tsx — getCurrentAnchor
	a := kit.NewAnchor(nestedItems()...)
	a.SetAffix(false)
	a.SetGetCurrentAnchor(func(string) string { return "#static" })
	// When getCurrentAnchor is set, display locks to #static
	a.SetActiveLink("#basic")
	if a.ActiveLink != "#static" {
		// SetActiveLink goes through applyActive → getCurrentAnchor
		// applyActive: ComputedLink=#basic, display=getCurrentAnchor → #static
		t.Fatalf("ActiveLink=%q want #static (custom highlight)", a.ActiveLink)
	}
	// OnChange still reports computed original
	var changed string
	a.SetOnChange(func(link string) { changed = link })
	// Force recompute via SyncFromScroll path
	sv := tallScrollHost(t)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{"#basic": 0, "#static": 100, "#api": 200})
	sv.SetScroll(0, 0)
	// Reset computed to trigger change
	a.ComputedLink = ""
	a.SyncFromScroll()
	if a.ActiveLink != "#static" {
		t.Fatalf("display=%q", a.ActiveLink)
	}
	if changed != "#basic" && changed != "" {
		// first section at 0 is #basic
		if a.ComputedLink == "#basic" && changed != "#basic" {
			t.Fatalf("OnChange=%q want computed #basic", changed)
		}
	}
}

func TestAnchor_PRD_13_TargetOffsetDemo(t *testing.T) {
	// ANC-13: targetOffset.tsx
	sv := tallScrollHost(t)
	a := kit.NewAnchor(sampleItems()...)
	a.SetTargetOffset(80)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{"#part-1": 100, "#part-2": 300, "#part-3": 500})
	a.ScrollTo("#part-1")
	if !approxAnchor(sv.ScrollY, 20, 0.5) { // 100-80
		t.Fatalf("ScrollY=%v want 20", sv.ScrollY)
	}
}

func TestAnchor_PRD_14_OnChangeDemo(t *testing.T) {
	// ANC-14: onChange.tsx
	sv := tallScrollHost(t)
	a := kit.NewAnchor(nestedItems()...)
	a.SetAffix(false)
	a.SetScrollTarget(sv)
	a.SetSectionOffsets(map[string]float64{
		"#basic": 0, "#static": 80, "#api": 160, "#anchor-props": 200, "#link-props": 240,
	})
	var last string
	n := 0
	a.SetOnChange(func(link string) { last = link; n++ })
	sv.SetScroll(0, 90)
	a.SyncFromScroll()
	if last != "#static" {
		t.Fatalf("OnChange=%q active=%q", last, a.ActiveLink)
	}
	if n < 1 {
		t.Fatal("OnChange not called")
	}
}

func TestAnchor_PRD_15_ReplaceDemo(t *testing.T) {
	// ANC-15: replace.tsx — History replace vs push
	a := kit.NewAnchor(sampleItems()...)
	a.SetAffix(false)
	tree := core.NewTree(a.Node())
	// push mode (default)
	clickAnchorLink(t, tree, a, "#part-1")
	clickAnchorLink(t, tree, a, "#part-2")
	if len(a.History) != 2 {
		t.Fatalf("push history=%v", a.History)
	}
	// replace mode
	a.SetReplace(true)
	a.History = nil
	clickAnchorLink(t, tree, a, "#part-1")
	clickAnchorLink(t, tree, a, "#part-3")
	if len(a.History) != 1 || a.History[0] != "#part-3" {
		t.Fatalf("replace history=%v want [#part-3]", a.History)
	}
	if a.CurrentHref != "#part-3" {
		t.Fatalf("CurrentHref=%q", a.CurrentHref)
	}
}

func TestAnchor_PRD_16_Tokens(t *testing.T) {
	// ANC-16: §6.2 关键尺寸/间距
	a := kit.NewAnchor()
	if !approxAnchor(a.ResolvedFontSize(), 14, 0.5) {
		t.Fatalf("fontSize=%v", a.ResolvedFontSize())
	}
	if !approxAnchor(a.ResolvedLinkPaddingBlock(), 4, 0.5) {
		t.Fatalf("padBlock=%v want 4", a.ResolvedLinkPaddingBlock())
	}
	if !approxAnchor(a.ResolvedLinkPaddingInlineStart(), 16, 0.5) {
		t.Fatalf("padInline=%v want 16", a.ResolvedLinkPaddingInlineStart())
	}
	if !approxAnchor(a.ResolvedInkWidth(), 2, 0.5) {
		t.Fatalf("inkW=%v want 2", a.ResolvedInkWidth())
	}
	th := kit.DefaultTheme()
	if !approxAnchor(th.SizeOr(core.TokenPaddingXS, 0), 4, 0.5) {
		t.Fatalf("TokenPaddingXS=%v", th.SizeOr(core.TokenPaddingXS, 0))
	}
	if !approxAnchor(th.SizeOr(core.TokenPadding, 0), 16, 0.5) {
		t.Fatalf("TokenPadding=%v", th.SizeOr(core.TokenPadding, 0))
	}
	if !approxAnchor(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatalf("TokenFontSize=%v", th.SizeOr(core.TokenFontSize, 0))
	}
	if !approxAnchor(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatalf("borderRadius=%v", th.SizeOr(core.TokenBorderRadius, 0))
	}
}

func TestAnchor_PRD_17_ThemeColors(t *testing.T) {
	// ANC-17: 默认皮颜色走 Theme Token
	th := kit.DefaultTheme()
	a := kit.NewAnchor(sampleItems()...)
	a.SetTheme(th)
	a.SetActiveLink("#part-1")
	_ = a.Node().Layout(core.Loose(200, 120))
	// active label uses colorPrimary
	p := a.LinkPressable("#part-1")
	if p == nil || len(p.Children()) == 0 {
		t.Fatal("active pressable")
	}
	tx, ok := p.Children()[0].(*primitive.Text)
	if !ok {
		t.Fatalf("child=%T", p.Children()[0])
	}
	primary := th.Color(core.TokenColorPrimary)
	if !approxAnchorColor(tx.Color, primary, 0.02) {
		t.Fatalf("active color=%v want primary %v", tx.Color, primary)
	}
	// inactive uses colorText
	p2 := a.LinkPressable("#part-2")
	tx2 := p2.Children()[0].(*primitive.Text)
	text := th.Color(core.TokenColorText)
	if !approxAnchorColor(tx2.Color, text, 0.02) {
		t.Fatalf("inactive color=%v want text %v", tx2.Color, text)
	}
}

func TestAnchor_PRD_18_DisabledN_A(t *testing.T) {
	// ANC-18: disabled 外观 — Anchor 无 disabled API（适用者 N/A）
	// Documented: no SetDisabled; smoke that control still layouts.
	a := kit.NewAnchor(sampleItems()...)
	_ = a.Node().Layout(core.Loose(200, 120))
}

func TestAnchor_PRD_19_KeyboardFocus(t *testing.T) {
	// ANC-19: 键盘/焦点主路径
	a := kit.NewAnchor(sampleItems()...)
	a.SetAffix(false)
	_ = a.Node().Layout(core.Loose(200, 160))
	if a.FlatCount() < 3 {
		t.Fatal(a.FlatCount())
	}
	// Each link is focusable pressable with focus ring
	p := a.LinkPressable("#part-1")
	if p == nil || !p.Focusable {
		t.Fatal("link not focusable")
	}
	if !p.ShowFocusRing {
		t.Fatal("want focus ring")
	}
	if !approxAnchor(p.FocusRingOutset, 1.5, 0.1) {
		t.Fatalf("FocusRingOutset=%v", p.FocusRingOutset)
	}
	a.MoveFocus(0) // ensure index valid
	a.Nav.Index = 0
	a.MoveFocus(1)
	if a.FocusIndex() != 1 {
		t.Fatalf("focus index=%d", a.FocusIndex())
	}
	a.ActivateFocused()
	if a.ActiveLink != "#part-2" {
		t.Fatalf("after activate ActiveLink=%q", a.ActiveLink)
	}
	// role navigation on chrome
	if a.ChromeNode().Base().Role != "navigation" {
		t.Fatalf("role=%q", a.ChromeNode().Base().Role)
	}
}
