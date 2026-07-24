package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/flex.md §6.9 — P0 PRD cases (FLX-01 … FLX-16).
// L3/L4 (FLX-19/20) and P1 (FLX-21) deferred; FLX-17/18 N/A for layout-only.

func approxFlex(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func flexBox(w, h float64) core.Node {
	b := primitive.NewBox()
	b.Width, b.Height = w, h
	return b
}

func TestFlex_PRD_01_Defaults(t *testing.T) {
	// FLX-01: NewFlex 默认创建；默认值符合 §6.10
	f := kit.NewFlex()
	if f.Node() == nil {
		t.Fatal("nil node")
	}
	if f.EffectiveOrientation() != kit.FlexHorizontal {
		t.Fatalf("orientation=%v want horizontal", f.EffectiveOrientation())
	}
	if f.IsVertical() {
		t.Fatal("want horizontal")
	}
	if f.Wrap {
		t.Fatal("wrap default false")
	}
	if f.Justify != kit.FlexJustifyStart {
		t.Fatalf("justify=%v want start", f.Justify)
	}
	if f.Align != kit.FlexAlignAuto {
		t.Fatalf("align=%v want auto", f.Align)
	}
	if f.GapSize != kit.FlexGapUnset {
		t.Fatalf("gapSize=%v want unset", f.GapSize)
	}
	if !approxFlex(f.ResolvedGap(), 0, 0.01) {
		t.Fatalf("gap=%v want 0", f.ResolvedGap())
	}
	if f.ResolvedJustify() != core.MainStart {
		t.Fatalf("main=%v", f.ResolvedJustify())
	}
	if f.ResolvedAlign() != core.CrossStart {
		t.Fatalf("cross=%v want start (horizontal auto)", f.ResolvedAlign())
	}
	if f.Root == nil || f.Root.Axis != core.AxisHorizontal {
		t.Fatal("root axis not horizontal")
	}
}

func TestFlex_PRD_02_DefaultTwoChildrenHorizontal(t *testing.T) {
	// FLX-02 / FLX-S1: 默认两子 → 横向
	a, b := flexBox(40, 20), flexBox(40, 20)
	f := kit.NewFlex(a, b)
	_ = f.Node().Layout(core.Loose(400, 100))
	kids := f.Root.Children()
	if len(kids) != 2 {
		t.Fatalf("kids=%d", len(kids))
	}
	if kids[0].Base().Offset().Y != kids[1].Base().Offset().Y {
		t.Fatalf("not same row: y0=%v y1=%v", kids[0].Base().Offset().Y, kids[1].Base().Offset().Y)
	}
	if kids[1].Base().Offset().X <= kids[0].Base().Offset().X {
		t.Fatalf("second not to the right: x0=%v x1=%v", kids[0].Base().Offset().X, kids[1].Base().Offset().X)
	}
}

func TestFlex_PRD_03_Vertical(t *testing.T) {
	// FLX-03 / FLX-S2
	a, b := flexBox(40, 20), flexBox(40, 20)
	f := kit.NewFlex(a, b)
	f.SetVertical(true)
	if !f.IsVertical() {
		t.Fatal("want vertical")
	}
	_ = f.Node().Layout(core.Loose(200, 200))
	kids := f.Root.Children()
	if kids[1].Base().Offset().Y <= kids[0].Base().Offset().Y {
		t.Fatalf("second not below: y0=%v y1=%v", kids[0].Base().Offset().Y, kids[1].Base().Offset().Y)
	}
	// orientation wins over Vertical sugar
	f2 := kit.NewFlex()
	f2.SetVertical(true)
	f2.SetOrientation(kit.FlexHorizontal)
	if f2.IsVertical() {
		t.Fatal("orientation should win over Vertical sugar")
	}
	// vertical auto align → stretch
	f3 := kit.NewFlex()
	f3.SetOrientation(kit.FlexVertical)
	if f3.ResolvedAlign() != core.CrossStretch {
		t.Fatalf("vertical auto align=%v want stretch", f3.ResolvedAlign())
	}
}

func TestFlex_PRD_04_GapMiddle(t *testing.T) {
	// FLX-04 / FLX-S3: gap middle → 16
	a, b := flexBox(40, 20), flexBox(40, 20)
	f := kit.NewFlex(a, b)
	f.SetGapSize(kit.FlexGapMedium)
	if !approxFlex(f.ResolvedGap(), 16, 0.5) {
		t.Fatalf("gap=%v want 16", f.ResolvedGap())
	}
	_ = f.Node().Layout(core.Loose(400, 100))
	kids := f.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxFlex(dx, 16, 0.5) {
		t.Fatalf("spacing=%v want 16", dx)
	}
}

func TestFlex_PRD_05_GapLarge(t *testing.T) {
	// FLX-05 / FLX-S4
	a, b := flexBox(40, 20), flexBox(40, 20)
	f := kit.NewFlex(a, b)
	f.SetGapSize(kit.FlexGapLarge)
	if !approxFlex(f.ResolvedGap(), 24, 0.5) {
		t.Fatalf("gap=%v want 24", f.ResolvedGap())
	}
	_ = f.Node().Layout(core.Loose(400, 100))
	kids := f.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxFlex(dx, 24, 0.5) {
		t.Fatalf("spacing=%v want 24", dx)
	}
}

func TestFlex_PRD_06_JustifySpaceBetween(t *testing.T) {
	// FLX-06 / FLX-S5
	a, b := flexBox(40, 20), flexBox(40, 20)
	f := kit.NewFlex(a, b)
	f.SetJustify(kit.FlexJustifySpaceBetween)
	_ = f.Node().Layout(core.Tight(200, 40))
	kids := f.Root.Children()
	if !approxFlex(kids[0].Base().Offset().X, 0, 0.5) {
		t.Fatalf("first x=%v want 0", kids[0].Base().Offset().X)
	}
	wantX := 200 - 40
	if !approxFlex(kids[1].Base().Offset().X, float64(wantX), 1.0) {
		t.Fatalf("second x=%v want %v", kids[1].Base().Offset().X, wantX)
	}
}

func TestFlex_PRD_07_AlignCenter(t *testing.T) {
	// FLX-07 / FLX-S6
	a := flexBox(40, 20)
	f := kit.NewFlex(a)
	f.SetAlign(kit.FlexAlignCenter)
	_ = f.Node().Layout(core.Tight(200, 100))
	kids := f.Root.Children()
	// cross center: y ≈ (100-20)/2 = 40
	if !approxFlex(kids[0].Base().Offset().Y, 40, 1.0) {
		t.Fatalf("y=%v want ~40", kids[0].Base().Offset().Y)
	}
}

func TestFlex_PRD_08_WrapNarrow(t *testing.T) {
	// FLX-08 / FLX-S7
	mk := func() core.Node { return flexBox(40, 20) }
	f := kit.NewFlex(mk(), mk(), mk())
	f.SetGap(8)
	f.SetWrap(true)
	sz := f.Node().Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	// 2 per line: 40+8+40=88; third wraps → height 20+8+20=48
	if sz.Height < 47.5 || sz.Height > 48.5 {
		t.Fatalf("height got %v want 48", sz.Height)
	}
	kids := f.Root.Children()
	if kids[2].Base().Offset().Y < 27.5 {
		t.Fatalf("third child should be on line 2, y=%v", kids[2].Base().Offset().Y)
	}
}

func TestFlex_PRD_09_GapNumeric8(t *testing.T) {
	// FLX-09 / FLX-S8
	a, b := flexBox(40, 20), flexBox(40, 20)
	f := kit.NewFlex(a, b)
	f.SetGap(8)
	if !approxFlex(f.ResolvedGap(), 8, 0.01) {
		t.Fatalf("gap=%v want 8", f.ResolvedGap())
	}
	_ = f.Node().Layout(core.Loose(400, 100))
	kids := f.Root.Children()
	dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
	if !approxFlex(dx, 8, 0.5) {
		t.Fatalf("spacing=%v want 8", dx)
	}
}

func TestFlex_PRD_10_BasicDemo(t *testing.T) {
	// FLX-10: 基本布局 basic.tsx — orientation/vertical 切换 4 块
	kids := make([]core.Node, 4)
	for i := range kids {
		kids[i] = flexBox(50, 54)
	}
	f := kit.NewFlex(kids...)
	// horizontal
	_ = f.Node().Layout(core.Loose(400, 200))
	if f.Root.Children()[1].Base().Offset().X <= 0 {
		t.Fatal("horizontal layout failed")
	}
	// vertical
	f.SetVertical(true)
	_ = f.Node().Layout(core.Loose(400, 400))
	if f.Root.Children()[1].Base().Offset().Y <= f.Root.Children()[0].Base().Offset().Y {
		t.Fatal("vertical layout failed")
	}
	// gap medium outer stack as in demo
	outer := kit.NewFlex(f.Node())
	outer.SetGapSize(kit.FlexGapMedium)
	outer.SetVertical(true)
	if !approxFlex(outer.ResolvedGap(), 16, 0.5) {
		t.Fatalf("outer gap=%v", outer.ResolvedGap())
	}
	_ = outer.Node().Layout(core.Loose(400, 400))
}

func TestFlex_PRD_11_AlignDemo(t *testing.T) {
	// FLX-11: 对齐方式 align.tsx — justify + align in bounded box
	btns := []core.Node{flexBox(60, 32), flexBox(60, 32), flexBox(60, 32), flexBox(60, 32)}
	f := kit.NewFlex(btns...)
	f.SetJustify(kit.FlexJustifyCenter)
	f.SetAlign(kit.FlexAlignCenter)
	sz := f.Node().Layout(core.Tight(400, 120))
	if sz.Width < 399 || sz.Height < 119 {
		t.Fatalf("size=%v want tight 400×120", sz)
	}
	// first child roughly centered on main: total content 240, free 160 → lead 80
	x0 := f.Root.Children()[0].Base().Offset().X
	if x0 < 70 || x0 > 90 {
		t.Fatalf("justify center first x=%v want ~80", x0)
	}
	y0 := f.Root.Children()[0].Base().Offset().Y
	if y0 < 40 || y0 > 48 {
		t.Fatalf("align center y=%v want ~44", y0)
	}
}

func TestFlex_PRD_12_GapDemo(t *testing.T) {
	// FLX-12: 设置间隙 gap.tsx — small/medium/large/custom
	mk := func() *kit.Flex {
		return kit.NewFlex(flexBox(40, 32), flexBox(40, 32), flexBox(40, 32))
	}
	for _, tc := range []struct {
		name string
		set  func(*kit.Flex)
		want float64
	}{
		{"small", func(f *kit.Flex) { f.SetGapSize(kit.FlexGapSmall) }, 8},
		{"medium", func(f *kit.Flex) { f.SetGapSize(kit.FlexGapMedium) }, 16},
		{"large", func(f *kit.Flex) { f.SetGapSize(kit.FlexGapLarge) }, 24},
		{"custom", func(f *kit.Flex) { f.SetGap(12) }, 12},
	} {
		f := mk()
		tc.set(f)
		if !approxFlex(f.ResolvedGap(), tc.want, 0.5) {
			t.Fatalf("%s gap=%v want %v", tc.name, f.ResolvedGap(), tc.want)
		}
		_ = f.Node().Layout(core.Loose(400, 80))
		kids := f.Root.Children()
		dx := kids[1].Base().Offset().X - (kids[0].Base().Offset().X + kids[0].Base().Size().Width)
		if !approxFlex(dx, tc.want, 0.5) {
			t.Fatalf("%s spacing=%v want %v", tc.name, dx, tc.want)
		}
	}
}

func TestFlex_PRD_13_WrapDemo(t *testing.T) {
	// FLX-13: 自动换行 wrap.tsx — many items + wrap + gap small
	kids := make([]core.Node, 24)
	for i := range kids {
		kids[i] = flexBox(64, 32) // button-ish
	}
	f := kit.NewFlex(kids...)
	f.SetWrap(true)
	f.SetGapSize(kit.FlexGapSmall)
	sz := f.Node().Layout(core.Constraints{MaxWidth: 300, MaxHeight: 2000})
	if sz.Height <= 40 {
		t.Fatalf("should wrap to multiple rows, h=%v", sz.Height)
	}
	// at least one child on second line
	foundWrap := false
	for _, c := range f.Root.Children() {
		if c.Base().Offset().Y > 20 {
			foundWrap = true
			break
		}
	}
	if !foundWrap {
		t.Fatal("no wrapped child")
	}
}

func TestFlex_PRD_14_CombinationDemo(t *testing.T) {
	// FLX-14: 组合使用 combination.tsx — space-between + nested vertical flex-end
	img := flexBox(120, 100)
	title := flexBox(80, 40)
	btn := flexBox(60, 32)
	inner := kit.NewFlex(title, btn)
	inner.SetVertical(true)
	inner.SetAlign(kit.FlexAlignEnd)
	inner.SetJustify(kit.FlexJustifySpaceBetween)
	// give inner a fixed height playground
	innerHost := primitive.NewBox(inner.Node())
	innerHost.Width = 160
	innerHost.Height = 100

	outer := kit.NewFlex(img, innerHost)
	outer.SetJustify(kit.FlexJustifySpaceBetween)
	_ = outer.Node().Layout(core.Tight(400, 100))
	kids := outer.Root.Children()
	if kids[0].Base().Offset().X > 1 {
		t.Fatalf("img x=%v want 0", kids[0].Base().Offset().X)
	}
	if kids[1].Base().Offset().X < 200 {
		t.Fatalf("inner should be pushed right, x=%v", kids[1].Base().Offset().X)
	}
	_ = inner.Node().Layout(core.Tight(160, 100))
	ik := inner.Root.Children()
	// align end: x ≈ 160-80 = 80 for title
	if ik[0].Base().Offset().X < 70 {
		t.Fatalf("title x=%v want ~80 (align end)", ik[0].Base().Offset().X)
	}
	// space-between vertical: btn near bottom
	if ik[1].Base().Offset().Y < 50 {
		t.Fatalf("btn y=%v want near bottom", ik[1].Base().Offset().Y)
	}
}

func TestFlex_PRD_15_TokenMetrics(t *testing.T) {
	// FLX-15: §6.2 关键尺寸/间距
	th := kit.DefaultTheme()
	if !approxFlex(th.SizeOr(core.TokenFontSize, 0), kit.DefaultFlexFontSize, 0.5) {
		t.Fatalf("fontSize=%v want %v", th.SizeOr(core.TokenFontSize, 0), kit.DefaultFlexFontSize)
	}
	if !approxFlex(th.SizeOr(core.TokenBorderRadius, 0), kit.DefaultFlexBorderRadius, 0.5) {
		t.Fatalf("radius=%v want %v", th.SizeOr(core.TokenBorderRadius, 0), kit.DefaultFlexBorderRadius)
	}
	if !approxFlex(th.SizeOr(core.TokenLineWidth, 0), kit.DefaultFlexLineWidth, 0.5) {
		t.Fatalf("lineWidth=%v", th.SizeOr(core.TokenLineWidth, 0))
	}
	// gap ladder
	f := kit.NewFlex()
	f.SetTheme(th)
	f.SetGapSize(kit.FlexGapSmall)
	if !approxFlex(f.ResolvedGap(), 8, 0.5) {
		t.Fatalf("small=%v", f.ResolvedGap())
	}
	f.SetGapSize(kit.FlexGapMedium)
	if !approxFlex(f.ResolvedGap(), 16, 0.5) {
		t.Fatalf("medium=%v", f.ResolvedGap())
	}
	f.SetGapSize(kit.FlexGapLarge)
	if !approxFlex(f.ResolvedGap(), 24, 0.5) {
		t.Fatalf("large=%v", f.ResolvedGap())
	}
	// medium/large via Theme tokens
	if !approxFlex(th.SizeOr(core.TokenPadding, 0), 16, 0.5) {
		t.Fatalf("TokenPadding=%v", th.SizeOr(core.TokenPadding, 0))
	}
	if !approxFlex(th.SizeOr(core.TokenPaddingLG, 0), 24, 0.5) {
		t.Fatalf("TokenPaddingLG=%v", th.SizeOr(core.TokenPaddingLG, 0))
	}
}

func TestFlex_PRD_16_NoHardcodedBrandSkin(t *testing.T) {
	// FLX-16: 默认皮无硬编码品牌色；布局容器透明
	f := kit.NewFlex(flexBox(20, 20))
	_ = f.Node()
	// primitive.Flex has no Background field — no brand paint on chrome.
	// Theme primary must still be reachable for children, not baked into Flex.
	primary := kit.DefaultTheme().Color(core.TokenColorPrimary)
	if primary.A == 0 {
		t.Fatal("theme primary missing")
	}
	// Ensure we did not invent a brand fill on Root via decoration — Root is *primitive.Flex only.
	if _, ok := any(f.Root).(*primitive.Flex); !ok {
		t.Fatalf("root type %T", f.Root)
	}
}

func TestFlex_PRD_17_DisabledN_A(t *testing.T) {
	// FLX-17: disabled 外观 — 布局容器不适用（无交互态）
	f := kit.NewFlex(flexBox(20, 20))
	_ = f.Node().Layout(core.Loose(100, 40))
	// smoke: still layouts; no Disabled field on product API for Flex
	if f.Root == nil {
		t.Fatal("nil root")
	}
}

func TestFlex_PRD_18_KeyboardN_A(t *testing.T) {
	// FLX-18: 键盘/焦点 — 布局容器不聚焦
	f := kit.NewFlex(flexBox(20, 20))
	f.SetAriaLabel("toolbar region")
	_ = f.Node()
	if f.Root.Base().Label != "toolbar region" {
		t.Fatalf("label=%q", f.Root.Base().Label)
	}
	// no Role forced
	if f.Root.Base().Role != "" {
		t.Fatalf("role=%q want empty for pure layout", f.Root.Base().Role)
	}
}

// Guard: custom SetGap then SetGapSize restores preset.
func TestFlex_PRD_GapPresetOverridesCustom(t *testing.T) {
	f := kit.NewFlex()
	f.SetGap(12)
	if !approxFlex(f.ResolvedGap(), 12, 0.01) {
		t.Fatal(f.ResolvedGap())
	}
	f.SetGapSize(kit.FlexGapSmall)
	if !approxFlex(f.ResolvedGap(), 8, 0.01) {
		t.Fatal(f.ResolvedGap())
	}
}

// Paint smoke: hit == layout size (no extra chrome).
// kit.Flex is block-level (ExpandMax): under bounded MaxWidth it fills parent width
// so justify free-space works (antd Flex display:flex block).
func TestFlex_PRD_HitEqualsLayout(t *testing.T) {
	f := kit.NewFlex(flexBox(30, 20), flexBox(30, 20))
	f.SetGap(8)
	n := f.Node()
	sz := n.Layout(core.Loose(400, 100))
	if !approxFlex(sz.Width, 400, 0.5) {
		t.Fatalf("w=%v want 400 (block fill)", sz.Width)
	}
	if !approxFlex(sz.Height, 20, 0.5) {
		t.Fatalf("h=%v want 20", sz.Height)
	}
	// HitTest at center of first child should hit something under root bounds
	tree := core.NewTree(n)
	tree.Layout(core.Size{Width: 400, Height: 100})
	hit := tree.HitTest(core.Point{X: 10, Y: 10})
	if hit == nil {
		t.Fatal("expected hit inside flex")
	}
	// expanded root is hittable across width (HitDefer still walks children)
	_ = tree.HitTest(core.Point{X: 200, Y: 10})
	_ = render.RGBA{}
}
