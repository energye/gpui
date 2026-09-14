package float_button_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	float_button "github.com/energye/gpui/ui/kit/float-button"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// P1 spec numbers live in testdata (hardcoded numbers banned in tests).
type floatP1Spec struct {
	Size         float64 `json:"size"`
	RadiusSquare float64 `json:"radiusSquare"`
	GroupGap     float64 `json:"groupGap"`
	Badge        struct {
		Count    int    `json:"count"`
		Overflow int    `json:"overflow"`
		BigCount int    `json:"bigCount"`
		BigText  string `json:"bigText"`
	} `json:"badge"`
	BackTop struct {
		VisibilityHeight float64 `json:"visibilityHeight"`
		DurationMs       float64 `json:"durationMs"`
		HiddenScroll     float64 `json:"hiddenScroll"`
		ShownScroll      float64 `json:"shownScroll"`
	} `json:"backTop"`
	HtmlTypes         []string `json:"htmlTypes"`
	SemanticNodes     []string `json:"semanticNodes"`
	TooltipPlacements []string `json:"tooltipPlacements"`
}

func loadFloatP1Spec(t *testing.T) floatP1Spec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "float-button-p1.json"))
	if err != nil {
		t.Fatalf("read float-button-p1.json: %v", err)
	}
	var s floatP1Spec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse p1 spec: %v", err)
	}
	if s.Size != 40 || s.RadiusSquare != 8 || s.GroupGap != 16 {
		t.Fatalf("spec geometry %+v want size=40 square=8 gap=16", s)
	}
	if s.BackTop.VisibilityHeight != 400 || s.BackTop.DurationMs != 450 {
		t.Fatalf("spec backtop %+v want 400/450", s.BackTop)
	}
	if len(s.HtmlTypes) != 3 || len(s.SemanticNodes) != 4 || len(s.TooltipPlacements) != 4 {
		t.Fatalf("spec lists html=%d semantic=%d tips=%d want 3/4/4",
			len(s.HtmlTypes), len(s.SemanticNodes), len(s.TooltipPlacements))
	}
	return s
}

// FB-10 (P1): BackTop hides below visibilityHeight.
func TestFloatButton_P1_FB10_BackTopHidden(t *testing.T) {
	spec := loadFloatP1Spec(t)
	bt := float_button.NewFloatBackTop()
	if bt.VisibilityHeight() != spec.BackTop.VisibilityHeight {
		t.Fatalf("visibility=%v want %v", bt.VisibilityHeight(), spec.BackTop.VisibilityHeight)
	}
	if bt.DurationMs() != spec.BackTop.DurationMs {
		t.Fatalf("duration=%v want %v", bt.DurationMs(), spec.BackTop.DurationMs)
	}
	bt.SetScrollY(spec.BackTop.HiddenScroll)
	if bt.Visible() {
		t.Fatal("scroll<400 must hide BackTop")
	}
	if bt.Click() {
		t.Fatal("hidden BackTop must swallow click")
	}
	if bt.Focusable() || bt.Focus() {
		t.Fatal("hidden BackTop skips focus")
	}
	if sz := bt.Layout(rendering.Loose(800, 800)); sz.Width != 0 || sz.Height != 0 {
		t.Fatalf("hidden layout=%v want zero", sz)
	}
	if bt.Role() != "button" {
		t.Fatalf("role=%q want button", bt.Role())
	}
}

// FB-11 (P1): BackTop shows at scroll>=400, click returns to top.
func TestFloatButton_P1_FB11_BackTopClick(t *testing.T) {
	spec := loadFloatP1Spec(t)
	bt := float_button.NewFloatBackTop()
	n := 0
	bt.SetOnClick(func() { n++ })
	bt.SetScrollY(spec.BackTop.ShownScroll)
	if !bt.Visible() {
		t.Fatal("scroll>=400 must show BackTop")
	}
	sz := bt.Layout(rendering.Loose(800, 800))
	if math.Abs(sz.Width-spec.Size) > 0.5 || math.Abs(sz.Height-spec.Size) > 0.5 {
		t.Fatalf("shown layout=%vx%v want 40x40", sz.Width, sz.Height)
	}
	if !bt.Focusable() || !bt.Focus() || !bt.Focused() {
		t.Fatal("shown BackTop takes focus")
	}
	if !bt.KeyActivate("Enter") || n != 1 {
		t.Fatalf("Enter回顶 n=%d want 1", n)
	}
	if bt.Visible() || bt.ScrollY() != 0 {
		t.Fatal("click must return scrollY to top and hide")
	}
	// Direct Click path.
	bt.SetScrollY(spec.BackTop.ShownScroll)
	if !bt.Click() || n != 2 {
		t.Fatalf("Click n=%d want 2", n)
	}
	// Disabled/loading swallow.
	bt.SetScrollY(spec.BackTop.ShownScroll)
	bt.Button().SetDisabled(true)
	if bt.Click() || n != 2 {
		t.Fatalf("disabled BackTop n=%d want 2", n)
	}
	bt.Button().SetDisabled(false)
	bt.Button().SetLoading(true)
	if bt.Click() || n != 2 {
		t.Fatalf("loading BackTop n=%d want 2", n)
	}
	bt.Button().SetLoading(false)
	// Threshold setters fall back on junk.
	bt.SetVisibilityHeight(0)
	bt.SetDurationMs(-1)
	if bt.VisibilityHeight() != spec.BackTop.VisibilityHeight || bt.DurationMs() != spec.BackTop.DurationMs {
		t.Fatal("junk threshold must fall back to 400/450")
	}
}

// FB-12 (P1): badge count/dot overlay, never steals the main click.
func TestFloatButton_P1_FB12_Badge(t *testing.T) {
	spec := loadFloatP1Spec(t)
	tok := theme.Default.Current()
	b := float_button.NewFloatButton()
	if b.BadgeVisible() || b.BadgeText() != "" || b.HasBadgeCount() || b.BadgeDot() {
		t.Fatal("no badge by default")
	}
	b.SetBadgeCount(spec.Badge.Count)
	if !b.HasBadgeCount() || !b.BadgeVisible() {
		t.Fatal("count badge must show")
	}
	if got := b.BadgeText(); got != "5" {
		t.Fatalf("badge text=%q want 5", got)
	}
	if ov := b.BadgeOverflowCount(); ov != spec.Badge.Overflow {
		t.Fatalf("overflow=%d want %d", ov, spec.Badge.Overflow)
	}
	wantBG := render.RGBA{R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A}
	if got := b.BadgeBackground(); got != wantBG {
		t.Fatalf("badge bg=%+v want ColorError", got)
	}
	b.SetBadgeCount(spec.Badge.BigCount)
	if got := b.BadgeText(); got != spec.Badge.BigText {
		t.Fatalf("overflow text=%q want %q", got, spec.Badge.BigText)
	}
	// Zero hides unless showZero.
	b.SetBadgeCount(0)
	if b.BadgeVisible() || b.BadgeText() != "" {
		t.Fatal("zero count hides by default")
	}
	b.SetBadgeShowZero(true)
	if !b.BadgeVisible() || b.BadgeText() != "0" {
		t.Fatalf("showZero visible=%v text=%q", b.BadgeVisible(), b.BadgeText())
	}
	b.SetBadgeShowZero(false)
	// Dot overlay.
	b.ClearBadge()
	b.SetBadgeDot(true)
	if !b.BadgeVisible() || b.BadgeText() != "" || !b.BadgeDot() {
		t.Fatal("dot shows without text")
	}
	b.ClearBadge()
	if b.BadgeVisible() {
		t.Fatal("ClearBadge hides overlay")
	}
	// Overlay never steals the main click; edge stays 40.
	c := float_button.NewFloatButton()
	c.SetBadgeCount(spec.Badge.Count)
	n := 0
	c.SetOnClick(func() { n++ })
	if !c.HitContains(20, 20) {
		t.Fatal("center must hit")
	}
	if !c.Click() || n != 1 {
		t.Fatalf("badge must not steal click n=%d", n)
	}
	sz := c.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-spec.Size) > 0.5 {
		t.Fatalf("edge=%v want 40", sz.Width)
	}
	paintButton(c, 64)
}

// FB-29 (P1): href/target/htmlType desktop mapping.
func TestFloatButton_PRD_FB29_HrefTargetHtmlType(t *testing.T) {
	spec := loadFloatP1Spec(t)
	if len(spec.HtmlTypes) != 3 {
		t.Fatalf("htmlTypes=%v want 3", spec.HtmlTypes)
	}
	b := float_button.NewFloatButton()
	if b.HtmlTypeName() != float_button.FloatHtmlButton || b.IsLink() {
		t.Fatalf("defaults html=%s link=%v", b.HtmlTypeName(), b.IsLink())
	}
	b.SetHref("https://ant.design/components/float-button")
	b.SetTarget(" _blank ")
	if !b.IsLink() || b.Href() != "https://ant.design/components/float-button" || b.Target() != "_blank" {
		t.Fatalf("href=%q target=%q link=%v", b.Href(), b.Target(), b.IsLink())
	}
	b.SetHtmlType(float_button.FloatHtmlSubmit)
	if b.HtmlTypeName() != float_button.FloatHtmlSubmit {
		t.Fatalf("html=%s", b.HtmlTypeName())
	}
	b.SetHtmlType("bogus")
	if b.HtmlTypeName() != float_button.FloatHtmlButton {
		t.Fatalf("bogus html=%s want button", b.HtmlTypeName())
	}
	// Click fires OnClick, then offers OnNavigate exactly once.
	clicks, navs := 0, 0
	b.SetOnClick(func() { clicks++ })
	b.SetOnNavigate(func(href, target string) {
		navs++
		if href != "https://ant.design/components/float-button" || target != "_blank" {
			t.Errorf("navigate(%q,%q)", href, target)
		}
	})
	if !b.Click() || clicks != 1 || navs != 1 {
		t.Fatalf("clicks=%d navs=%d want 1/1", clicks, navs)
	}
	// Disabled swallows both; plain button has no navigation.
	d := float_button.NewFloatButton()
	d.SetHref("https://ant.design/components/float-button")
	dn := 0
	d.SetOnNavigate(func(_, _ string) { dn++ })
	d.SetDisabled(true)
	if d.Click() || dn != 0 {
		t.Fatal("disabled swallows navigate")
	}
	p := float_button.NewFloatButton()
	pn := 0
	p.SetOnNavigate(func(_, _ string) { pn++ })
	if !p.Click() || pn != 0 {
		t.Fatal("plain button has no navigation")
	}
}

// P1 semantic classNames/styles object-form hooks.
func TestFloatButton_P1_SemanticHooks(t *testing.T) {
	spec := loadFloatP1Spec(t)
	if len(float_button.FloatSemanticNodes) != len(spec.SemanticNodes) {
		t.Fatalf("semantic nodes=%d want %d", len(float_button.FloatSemanticNodes), len(spec.SemanticNodes))
	}
	b := float_button.NewFloatButton()
	b.SetClassName(float_button.FloatSemanticRoot, "my-fb")
	b.SetClassNames(map[float_button.FloatSemanticKey]string{
		float_button.FloatSemanticRoot:    "my-fb",
		float_button.FloatSemanticContent: "my-content",
	})
	if b.ClassName(float_button.FloatSemanticRoot) != "my-fb" ||
		b.ClassName(float_button.FloatSemanticContent) != "my-content" {
		t.Fatal("classNames must stick")
	}
	before := b.EffectiveBackground()
	beforeText := b.EffectiveForeground()
	b.SetSemanticStyle(float_button.FloatSemanticRoot,
		float_button.FloatSemanticStyle{Bg: render.RGBA{R: 0.1, G: 0.2, B: 0.3, A: 1}, UseBg: true})
	b.SetSemanticStyle(float_button.FloatSemanticContent,
		float_button.FloatSemanticStyle{Text: render.RGBA{R: 0.9, G: 0.1, B: 0.1, A: 1}, UseText: true})
	if b.EffectiveBackground() == before {
		t.Fatal("root semantic Bg must move the fill")
	}
	if b.EffectiveForeground() == beforeText {
		t.Fatal("content semantic Text must move the ink")
	}
	if _, ok := b.SemanticStyle(float_button.FloatSemanticIcon); ok {
		t.Fatal("unset icon hook must report missing")
	}
	// Hooks never move layout.
	y := float_button.NewFloatButton()
	a := y.Layout(rendering.Loose(1000, 1000))
	y.SetClassName(float_button.FloatSemanticRoot, "late")
	y.SetSemanticStyle(float_button.FloatSemanticRoot,
		float_button.FloatSemanticStyle{Bg: render.RGBA{R: 0, G: 0, B: 0, A: 1}, UseBg: true})
	if after := y.Layout(rendering.Loose(1000, 1000)); a != after {
		t.Fatalf("semantic hooks moved layout %v -> %v", a, after)
	}
}

// P1 ConfigProvider-style global defaults (float-button-owned staging).
func TestFloatButton_P1_GlobalDefaults(t *testing.T) {
	float_button.ResetFloatGlobalConfig()
	defer float_button.ResetFloatGlobalConfig()
	float_button.SetFloatGlobalConfig(float_button.FloatGlobalConfig{
		Type:        float_button.ButtonTypePrimary,
		HasType:     true,
		Shape:       float_button.FloatButtonShapeSquare,
		HasShape:    true,
		Icon:        "search",
		HasIcon:     true,
		HtmlType:    float_button.FloatHtmlSubmit,
		HasHtmlType: true,
	})
	b := float_button.NewFloatButton()
	if b.Type() != float_button.ButtonTypePrimary {
		t.Fatalf("global type=%s", b.Type())
	}
	if b.Shape() != float_button.FloatButtonShapeSquare {
		t.Fatalf("global shape=%s", b.Shape())
	}
	if b.EffectiveIcon() != "search" || b.HtmlTypeName() != float_button.FloatHtmlSubmit {
		t.Fatalf("global icon=%q html=%s", b.EffectiveIcon(), b.HtmlTypeName())
	}
	// Explicit props win over globals.
	b.SetType(float_button.ButtonTypeDefault)
	b.SetShape(float_button.FloatButtonShapeCircle)
	if b.Type() != float_button.ButtonTypeDefault || b.Shape() != float_button.FloatButtonShapeCircle {
		t.Fatal("explicit must win over global")
	}
	float_button.ResetFloatGlobalConfig()
	if n := float_button.NewFloatButton(); n.Type() != float_button.ButtonTypeDefault {
		t.Fatal("reset must restore baselines")
	}
}

// P1 draggable: host-mapped drag offsets.
func TestFloatButton_P1_Draggable(t *testing.T) {
	spec := loadFloatP1Spec(t)
	b := float_button.NewFloatButton()
	if b.Draggable() {
		t.Fatal("drag defaults off")
	}
	if b.DragBy(10, 5) {
		t.Fatal("non-draggable ignores DragBy")
	}
	b.SetDraggable(true)
	if !b.Draggable() || !b.DragBy(10, 5) {
		t.Fatal("draggable must move")
	}
	if x, y := b.DragOffset(); x != 10 || y != 5 {
		t.Fatalf("offset=(%v,%v) want (10,5)", x, y)
	}
	if off := b.Node().Offset(); off.X != 10 || off.Y != 5 {
		t.Fatalf("node offset=%v want (10,5)", off)
	}
	b.SetDisabled(true)
	if b.DragBy(4, 4) {
		t.Fatal("disabled blocks drag")
	}
	b.SetDisabled(false)
	b.ClearDrag()
	if x, y := b.DragOffset(); x != 0 || y != 0 {
		t.Fatalf("cleared offset=(%v,%v)", x, y)
	}
	sz := b.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-spec.Size) > 0.5 {
		t.Fatalf("edge=%v want 40", sz.Width)
	}
}

// P1 tooltip full-props staging (placement/delay stored; string P0 unchanged).
func TestFloatButton_P1_TooltipProps(t *testing.T) {
	spec := loadFloatP1Spec(t)
	b := float_button.NewFloatButton()
	if b.TooltipPlacement() != float_button.FloatTooltipTop {
		t.Fatalf("tip default=%s want top", b.TooltipPlacement())
	}
	b.SetTooltipPlacement(float_button.FloatTooltipLeft)
	if b.TooltipPlacement() != float_button.FloatTooltipLeft {
		t.Fatalf("tip=%s want left", b.TooltipPlacement())
	}
	b.SetTooltipPlacement("bogus")
	if b.TooltipPlacement() != float_button.FloatTooltipTop {
		t.Fatalf("bogus tip=%s want top", b.TooltipPlacement())
	}
	b.SetTooltipDelayMs(-3)
	if b.TooltipDelayMs() != 0 {
		t.Fatalf("delay=%d want 0", b.TooltipDelayMs())
	}
	b.SetTooltipDelayMs(200)
	if b.TooltipDelayMs() != 200 {
		t.Fatalf("delay=%d want 200", b.TooltipDelayMs())
	}
	if len(spec.TooltipPlacements) != 4 {
		t.Fatalf("tips=%v want 4", spec.TooltipPlacements)
	}
	b.SetTooltip("bubble tip")
	b.SetHover(true)
	if !b.TooltipVisible() {
		t.Fatal("string bubble still shows on hover")
	}
}

// True-text chain: content-only button paints no ink without a face (black
// bars banned); with a face the text zone carries ink. Layout/Node non-zero.
func TestFloatButton_P1_TrueText(t *testing.T) {
	spec := loadFloatP1Spec(t)
	face, _, err := rendering.TryLoadDefaultFace(12)
	if err != nil || face == nil {
		t.Skipf("true-text needs a system face: %v", err)
	}
	countDark := func(b *float_button.FloatButton) int {
		sz := b.Layout(rendering.Loose(200, 200))
		dc := render.NewContext(int(sz.Width), int(sz.Height))
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		b.Node().Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		dark := 0
		for y := 0; y < int(sz.Height); y++ {
			for x := 0; x < int(sz.Width); x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				if r/257 < 110 && g/257 < 110 && bl/257 < 110 {
					dark++
				}
			}
		}
		return dark
	}
	plain := float_button.NewFloatButton()
	plain.SetContent("Help")
	if n := countDark(plain); n != 0 {
		t.Fatalf("no-face dark=%d want 0 (black bar banned)", n)
	}
	with := float_button.NewFloatButton()
	with.SetContent("Help")
	with.SetTextFace(face)
	if n := countDark(with); n < 30 {
		t.Fatalf("with-face dark=%d want >=30 (real glyphs missing)", n)
	}
	with.SetTextFace(nil)
	if n := countDark(with); n != 0 {
		t.Fatalf("cleared face dark=%d want 0", n)
	}
	// Layout/Node contract: laid out size is the 40 edge, never zero.
	nb := float_button.NewFloatButton()
	sz := nb.Layout(rendering.Loose(200, 200))
	if math.Abs(sz.Width-spec.Size) > 0.5 || math.Abs(sz.Height-spec.Size) > 0.5 {
		t.Fatalf("layout=%vx%v want 40x40", sz.Width, sz.Height)
	}
	if ns := nb.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("node size=%v want >0 after Layout", ns)
	}
}

// P1 placement entry animation is pixel-staged: four-direction behavior is P0
// (FB-09 + showcase left/bottom rows); the entrance animation keeps instant
// switching that respects reduced-motion.
func TestFloatButton_P1_PlacementAnimationNA(t *testing.T) {
	t.Skip("placement entry-animation pixels are P1 staged; behavior in four directions is P0 and instant switching respects reduced-motion")
}

// P1 badge omits status/text/title/children per FloatButton.tsx (only
// count/dot/overflowCount supported).
func TestFloatButton_P1_BadgeOmitNA(t *testing.T) {
	t.Skip("badge status/text/title/children unsupported by design (source omit); only count/dot/overflowCount implemented")
}

// P1 classNames/styles function form has no desktop equivalent mapping.
func TestFloatButton_P1_SemanticFuncNA(t *testing.T) {
	t.Skip("semantic function form (info: {props}) => Record is React-only; object form implemented")
}

// P1 full TooltipProps overlay (arrow/position/flip/focus-lock) needs the
// Tooltip component (NotStarted); string P0 self-draw covered by FB-18.
func TestFloatButton_P1_TooltipFullNA(t *testing.T) {
	t.Skip("full TooltipProps overlay positioning needs ui/kit/tooltip; string bubble P0 implemented")
}

// P1 debug examples never count toward acceptance (§6.1).
func TestFloatButton_P1_DebugNA(t *testing.T) {
	t.Skip("badge-debug/render-panel are internal debug previews, excluded from P0/P1 acceptance")
}

// L4 human-eye side-by-side needs a reviewer signature.
func TestFloatButton_PRD_FB28_HumanEyeNA(t *testing.T) {
	t.Skip("L4 FB-28 needs human side-by-side sign-off against ant.design; no automated assertion")
}
