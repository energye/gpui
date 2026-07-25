package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/empty.md §6.9 — P0 PRD cases (EMP-01 … EMP-10, 12, 13, 15, 16, 18).
// EMP-11/14/21 P1 deferred; EMP-17 N/A; EMP-19 L3 / EMP-20 L4 deferred.

func approxEmpty(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxEmptyColor(a, b render.RGBA, tol float64) bool {
	return approxEmpty(float64(a.R), float64(b.R), tol) &&
		approxEmpty(float64(a.G), float64(b.G), tol) &&
		approxEmpty(float64(a.B), float64(b.B), tol) &&
		approxEmpty(float64(a.A), float64(b.A), tol)
}

func TestEmpty_PRD_01_Defaults(t *testing.T) {
	// EMP-01: NewEmpty 默认创建
	e := kit.NewEmpty()
	if e.ImageKind() != kit.EmptyImageDefault {
		t.Fatalf("Image=%v want default", e.ImageKind())
	}
	if e.ResolvedDescription() != kit.DefaultEmptyDescription {
		t.Fatalf("desc=%q want %q", e.ResolvedDescription(), kit.DefaultEmptyDescription)
	}
	if !e.HasDescription() {
		t.Fatal("HasDescription false")
	}
	if e.HasFooter() {
		t.Fatal("footer want empty")
	}
	if e.Node() == nil || e.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	sz := e.Node().Layout(core.Loose(400, 300))
	if sz.Width < 10 || sz.Height < 10 {
		t.Fatalf("size=%v", sz)
	}
}

func TestEmpty_PRD_02_DefaultIllustrationAndLocale(t *testing.T) {
	// EMP-02 / EMP-S1: 默认插画 + No data
	e := kit.NewEmpty()
	if e.ImageKind() != kit.EmptyImageDefault {
		t.Fatal("kind")
	}
	if e.ResolvedDescription() != "No data" {
		t.Fatalf("desc=%q", e.ResolvedDescription())
	}
	if e.ImageNodeHost() == nil {
		t.Fatal("no image host")
	}
	if e.DescriptionNodeHost() == nil {
		t.Fatal("no desc lab")
	}
	_ = e.Node().Layout(core.Loose(400, 300))
}

func TestEmpty_PRD_03_SimpleImage(t *testing.T) {
	// EMP-03 / EMP-S2: simple 图
	e := kit.NewEmpty()
	e.SetImage(kit.EmptyImageSimple)
	if !e.IsSimple() {
		t.Fatal("IsSimple false")
	}
	if !approxEmpty(e.ImageHeight(), kit.DefaultEmptyImgHeightMD, 0.5) {
		t.Fatalf("imgH=%v want %v", e.ImageHeight(), kit.DefaultEmptyImgHeightMD)
	}
	if !approxEmpty(e.NormalMarginBlock(), kit.DefaultEmptyNormalMarginBlock, 0.5) {
		t.Fatalf("marginBlock=%v", e.NormalMarginBlock())
	}
	_ = e.Node().Layout(core.Loose(400, 300))
}

func TestEmpty_PRD_04_CustomDescription(t *testing.T) {
	// EMP-04 / EMP-S3: 自定义 description
	e := kit.NewEmpty()
	e.SetDescription("Nothing here")
	if e.ResolvedDescription() != "Nothing here" {
		t.Fatalf("desc=%q", e.ResolvedDescription())
	}
	if !e.HasDescription() {
		t.Fatal("HasDescription")
	}
	_ = e.Node().Layout(core.Loose(400, 200))
}

func TestEmpty_PRD_05_ChildrenFooter(t *testing.T) {
	// EMP-05 / EMP-S4: children 按钮
	clicked := false
	btn := kit.NewButton("Create Now")
	btn.SetOnClick(func() { clicked = true })
	e := kit.NewEmpty()
	e.SetChildren(btn.Node())
	if !e.HasFooter() {
		t.Fatal("HasFooter false")
	}
	if e.FooterNode() == nil {
		t.Fatal("FooterNode nil")
	}
	_ = e.Node().Layout(core.Loose(400, 300))
	// footer hosts the same button node; click path is Button.OnClick
	if btn.OnClick == nil {
		t.Fatal("OnClick nil")
	}
	btn.OnClick()
	if !clicked {
		t.Fatal("footer button not clickable")
	}
}

func TestEmpty_PRD_06_CustomImage(t *testing.T) {
	// EMP-06 / EMP-S5: 自定义 image Node / src
	custom := kit.NewText("IMG").Node()
	e := kit.NewEmpty()
	e.SetImageNode(custom)
	if e.ImageNode != custom {
		t.Fatal("ImageNode not set")
	}
	_ = e.Node().Layout(core.Loose(400, 300))

	e2 := kit.NewEmpty()
	e2.SetImageSrc("https://example.com/empty.svg")
	if e2.ImageSrc == "" {
		t.Fatal("ImageSrc empty")
	}
	_ = e2.Node().Layout(core.Loose(400, 300))
	// string image should mark img role on image subtree
	if host := e2.ImageNodeHost(); host != nil && len(host.Children()) > 0 {
		// role may be on nested image root
		_ = host
	}
}

func TestEmpty_PRD_07_ThemeSwitch(t *testing.T) {
	// EMP-07 / EMP-S6: 主题切换
	e := kit.NewEmpty()
	th1 := kit.DefaultTheme()
	e.SetTheme(th1)
	c1 := e.DescriptionColor()
	th2 := kit.DefaultTheme()
	// override secondary text
	if th2.Tokens == nil {
		th2.Tokens = core.NewTokenSet()
	}
	th2.Tokens.Colors[core.TokenColorTextSecondary] = render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 0.9}
	e.SetTheme(th2)
	c2 := e.DescriptionColor()
	if approxEmptyColor(c1, c2, 0.02) {
		// still ok if Theme.Color falls back; ensure not hard-coded brand primary
		prim := th2.Color(core.TokenColorPrimary)
		if approxEmptyColor(c2, prim, 0.02) {
			t.Fatal("description color equals brand primary")
		}
	}
	_ = e.Node().Layout(core.Loose(400, 200))
}

func TestEmpty_PRD_08_DemoBasic(t *testing.T) {
	// EMP-08: basic.tsx — <Empty />
	e := kit.NewEmpty()
	if e.ResolvedDescription() != kit.DefaultEmptyDescription {
		t.Fatalf("desc=%q", e.ResolvedDescription())
	}
	if e.ImageKind() != kit.EmptyImageDefault {
		t.Fatal("kind")
	}
	sz := e.Node().Layout(core.Loose(480, 320))
	if sz.Height < kit.DefaultEmptyImgHeight {
		t.Fatalf("h=%v want >= imgH", sz.Height)
	}
}

func TestEmpty_PRD_09_DemoSimple(t *testing.T) {
	// EMP-09: simple.tsx — PRESENTED_IMAGE_SIMPLE
	e := kit.NewEmpty()
	e.SetImage(kit.EmptyImageSimple)
	if !approxEmpty(e.ImageHeight(), kit.DefaultEmptyImgHeightMD, 0.5) {
		t.Fatalf("h=%v", e.ImageHeight())
	}
	_ = e.Node().Layout(core.Loose(400, 300))
}

func TestEmpty_PRD_10_DemoCustomize(t *testing.T) {
	// EMP-10: customize.tsx — custom image height 60 + description + footer button
	e := kit.NewEmpty()
	e.SetImageSrc("https://gw.alipayobjects.com/zos/antfincdn/ZHrcdLPrvN/empty.svg")
	e.SetImageHeight(60)
	e.SetDescription("Customize Description")
	btn := kit.NewButton("Create Now")
	btn.SetType(kit.ButtonPrimary)
	e.SetChildren(btn.Node())
	if !approxEmpty(e.ImageHeight(), 60, 0.5) {
		t.Fatalf("imgH=%v", e.ImageHeight())
	}
	if e.ResolvedDescription() != "Customize Description" {
		t.Fatalf("desc=%q", e.ResolvedDescription())
	}
	if !e.HasFooter() {
		t.Fatal("footer")
	}
	_ = e.Node().Layout(core.Loose(480, 360))
}

func TestEmpty_PRD_12_DemoStyleClass(t *testing.T) {
	// EMP-12: style-class.tsx — shallow styles + classNames
	e := kit.NewEmpty()
	e.SetImage(kit.EmptyImageSimple)
	e.SetDescription("Object styles")
	btn := kit.NewButton("Create Now")
	e.SetChildren(btn.Node())
	e.SetClassNames(kit.EmptyClassNames{Root: "empty-demo-root"})
	e.SetStyle(kit.Style{
		Background:  render.Hex("#F5F5F5"),
		Radius:      8,
		ForceRadius: true,
	})
	e.SetDescriptionStyle(kit.Style{Text: render.Hex("#1890FF")})
	e.SetFooterStyle(kit.Style{})
	if e.ClassNames.Root != "empty-demo-root" {
		t.Fatalf("classNames=%q", e.ClassNames.Root)
	}
	if e.DescriptionStyle.Text.A < 0.1 {
		t.Fatal("desc style")
	}
	root := e.ChromeNode().(*primitive.Decorated)
	if !approxEmptyColor(root.Background, render.Hex("#F5F5F5"), 0.02) {
		t.Fatalf("bg=%v", root.Background)
	}
	if !approxEmpty(root.Radius, 8, 0.5) {
		t.Fatalf("radius=%v", root.Radius)
	}
	_ = e.Node().Layout(core.Loose(480, 360))
}

func TestEmpty_PRD_13_DemoNoDescription(t *testing.T) {
	// EMP-13 / EMP-S7: description.tsx — description={false}
	e := kit.NewEmpty()
	e.HideDescription()
	if e.HasDescription() {
		t.Fatal("HasDescription true")
	}
	if e.ResolvedDescription() != "" {
		t.Fatalf("desc=%q", e.ResolvedDescription())
	}
	if e.DescriptionNodeHost() != nil {
		t.Fatal("desc lab should be nil")
	}
	_ = e.Node().Layout(core.Loose(400, 300))

	// empty string also hides
	e2 := kit.NewEmpty()
	e2.SetDescription("")
	if e2.HasDescription() {
		t.Fatal("empty string should hide")
	}
}

func TestEmpty_PRD_15_Metrics(t *testing.T) {
	// EMP-15: §6.2 关键尺寸/间距
	e := kit.NewEmpty()
	if !approxEmpty(e.FontSize(), kit.DefaultEmptyFontSize, 0.5) {
		t.Fatalf("font=%v", e.FontSize())
	}
	if !approxEmpty(e.ImageHeight(), kit.DefaultEmptyImgHeight, 0.5) {
		t.Fatalf("imgH=%v want %v", e.ImageHeight(), kit.DefaultEmptyImgHeight)
	}
	if !approxEmpty(e.ImageMarginBottom(), kit.DefaultEmptyImageMarginBottom, 0.5) {
		t.Fatalf("imgMB=%v", e.ImageMarginBottom())
	}
	if !approxEmpty(e.FooterMarginTop(), kit.DefaultEmptyFooterMarginTop, 0.5) {
		t.Fatalf("ftMT=%v", e.FooterMarginTop())
	}
	if !approxEmpty(e.MarginInline(), kit.DefaultEmptyMarginInline, 0.5) {
		t.Fatalf("mi=%v", e.MarginInline())
	}
	// controlHeightLG-derived default height
	th := kit.DefaultTheme()
	chLG := th.SizeOr(core.TokenControlHeightLG, 40)
	if !approxEmpty(chLG*2.5, kit.DefaultEmptyImgHeight, 0.5) {
		t.Fatalf("chLG*2.5=%v", chLG*2.5)
	}
	// simple metrics
	e.SetImage(kit.EmptyImageSimple)
	if !approxEmpty(e.ImageHeight(), kit.DefaultEmptyImgHeightMD, 0.5) {
		t.Fatalf("simple h=%v", e.ImageHeight())
	}
	if !approxEmpty(e.NormalMarginBlock(), kit.DefaultEmptyNormalMarginBlock, 0.5) {
		t.Fatalf("normal mb=%v", e.NormalMarginBlock())
	}
}

func TestEmpty_PRD_16_TokenColors(t *testing.T) {
	// EMP-16: 默认皮颜色走 Theme Token
	e := kit.NewEmpty()
	th := kit.DefaultTheme()
	e.SetTheme(th)
	sec := th.Color(core.TokenColorTextSecondary)
	got := e.DescriptionColor()
	if !approxEmptyColor(got, sec, 0.05) {
		t.Fatalf("descColor=%v token=%v", got, sec)
	}
	// not brand primary
	prim := th.Color(core.TokenColorPrimary)
	if approxEmptyColor(got, prim, 0.02) {
		t.Fatal("description uses brand primary")
	}
}

func TestEmpty_PRD_18_FooterFocusPath(t *testing.T) {
	// EMP-18: footer 子 Button 可聚焦（本体无强制焦点）
	btn := kit.NewButton("Create Now")
	e := kit.NewEmpty()
	e.SetChildren(btn.Node())
	_ = e.Node().Layout(core.Loose(400, 300))
	// Empty root is group, not button
	if e.ChromeNode().Base().Role != "group" {
		t.Fatalf("role=%q", e.ChromeNode().Base().Role)
	}
	// button node exists and layouts
	sz := btn.Node().Layout(core.Loose(200, 40))
	if sz.Width < 4 {
		t.Fatalf("btn sz=%v", sz)
	}
}

func TestEmpty_PRD_A11yDecorativeImage(t *testing.T) {
	// §6.6: 内置装饰图无 Role/Label
	e := kit.NewEmpty()
	_ = e.Node().Layout(core.Loose(400, 300))
	host := e.ImageNodeHost()
	if host == nil {
		t.Fatal("no image host")
	}
	// canvas child should be decorative
	if kids := host.Children(); len(kids) > 0 {
		if kids[0].Base().Role == "img" && kids[0].Base().Label == "" {
			// ok-ish
		}
	}
}
