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

type floatSpec struct {
	Size            float64 `json:"size"`
	IconSize        float64 `json:"iconSize"`
	ContentFontSize float64 `json:"contentFontSize"`
	RadiusCircle    float64 `json:"radiusCircle"`
	RadiusSquare    float64 `json:"radiusSquare"`
	GroupGap        float64 `json:"groupGap"`
}

func loadFloatSpec(t *testing.T) floatSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "float-button.json"))
	if err != nil {
		t.Fatalf("read float-button.json: %v", err)
	}
	var f floatSpec
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse float-button.json: %v", err)
	}
	if f.Size != 40 || f.RadiusSquare != 8 || f.GroupGap != 16 {
		t.Fatalf("spec numbers %+v want size=40 square=8 gap=16", f)
	}
	return f
}

func layoutSize(t *testing.T, n rendering.RenderObject, c rendering.Constraints) rendering.Size {
	t.Helper()
	return n.Layout(c)
}

func paintButton(b *float_button.FloatButton, size int) {
	dc := render.NewContext(size, size)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	b.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func TestFloatButton_PRD_FB01(t *testing.T) {
	b := float_button.NewFloatButton()
	if b.Type() != float_button.ButtonTypeDefault {
		t.Fatalf("type=%q want default", b.Type())
	}
	if b.Shape() != float_button.FloatButtonShapeCircle {
		t.Fatalf("shape=%q want circle", b.Shape())
	}
	if b.Disabled() || b.Loading() {
		t.Fatal("disabled/loading default false")
	}
	if !b.Click() {
		t.Fatal("default button must be clickable")
	}
	sz := layoutSize(t, b.Node(), rendering.Tight(40, 40))
	if math.Abs(sz.Width-40) > 0.5 || math.Abs(sz.Height-40) > 0.5 {
		t.Fatalf("layout=%vx%v want 40x40", sz.Width, sz.Height)
	}
	paintButton(b, 64)
}

func TestFloatButton_PRD_FB02(t *testing.T) {
	b := float_button.NewFloatButton()
	n := 0
	b.SetOnClick(func() { n++ })
	if !b.Click() {
		t.Fatal("click must activate")
	}
	if n != 1 {
		t.Fatalf("onClick=%d want 1", n)
	}
}

func TestFloatButton_PRD_FB03(t *testing.T) {
	b := float_button.NewFloatButton()
	n := 0
	b.SetOnClick(func() { n++ })
	b.SetDisabled(true)
	if b.Click() {
		t.Fatal("disabled must swallow click")
	}
	if n != 0 {
		t.Fatalf("onClick=%d want 0", n)
	}
	if b.KeyActivate("Enter") {
		t.Fatal("disabled must swallow keyboard")
	}
	b.SetHover(true)
	if b.TooltipVisible() {
		t.Fatal("disabled must not show tooltip")
	}
}

func TestFloatButton_PRD_FB04(t *testing.T) {
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	d := float_button.NewFloatButton()
	if got := d.EffectiveBackground(); got != toRGBA(tok.ColorBgContainer) {
		t.Fatalf("default bg=%+v want ColorBgContainer", got)
	}
	if got := d.EffectiveForeground(); got != toRGBA(tok.ColorText) {
		t.Fatalf("default fg=%+v want ColorText", got)
	}
	p := float_button.NewFloatButton()
	p.SetType(float_button.ButtonTypePrimary)
	if got := p.EffectiveBackground(); got != toRGBA(tok.ColorPrimary) {
		t.Fatalf("primary bg=%+v want ColorPrimary", got)
	}
	if got := p.EffectiveForeground(); got != toRGBA(tok.ColorWhite) {
		t.Fatalf("primary fg=%+v want ColorWhite", got)
	}
}

func TestFloatButton_PRD_FB05(t *testing.T) {
	f := loadFloatSpec(t)
	b := float_button.NewFloatButton()
	if math.Abs(b.EffectiveRadius()-f.RadiusCircle) > 0.5 {
		t.Fatalf("circle r=%v want %v", b.EffectiveRadius(), f.RadiusCircle)
	}
	b.SetShape(float_button.FloatButtonShapeSquare)
	if math.Abs(b.EffectiveRadius()-f.RadiusSquare) > 0.5 {
		t.Fatalf("square r=%v want %v", b.EffectiveRadius(), f.RadiusSquare)
	}
}

func TestFloatButton_PRD_FB06(t *testing.T) {
	f := loadFloatSpec(t)
	b := float_button.NewFloatButton()
	exact := layoutSize(t, b.Node(), rendering.Tight(f.Size, f.Size))
	if math.Abs(exact.Width-f.Size) > 0.5 || math.Abs(exact.Height-f.Size) > 0.5 {
		t.Fatalf("exact=%vx%v want 40x40", exact.Width, exact.Height)
	}
	loose := layoutSize(t, b.Node(), rendering.Loose(200, 200))
	if math.Abs(loose.Width-f.Size) > 0.5 || math.Abs(loose.Height-f.Size) > 0.5 {
		t.Fatalf("loose=%vx%v want 40x40", loose.Width, loose.Height)
	}
	open := layoutSize(t, b.Node(), rendering.Expand())
	if math.Abs(open.Width-f.Size) > 0.5 || math.Abs(open.Height-f.Size) > 0.5 {
		t.Fatalf("expand=%vx%v want 40x40", open.Width, open.Height)
	}
}

func TestFloatButton_PRD_FB07(t *testing.T) {
	a := float_button.NewFloatButton()
	c := float_button.NewFloatButton()
	g := float_button.NewFloatButtonGroup(a, c)
	g.SetTrigger(float_button.FloatButtonTriggerClick)
	if g.Open() {
		t.Fatal("menu starts closed")
	}
	if len(g.VisibleChildren()) != 0 {
		t.Fatal("closed menu hides children")
	}
	if !g.ClickTrigger() {
		t.Fatal("trigger click must open")
	}
	if !g.Open() || len(g.VisibleChildren()) != 2 {
		t.Fatalf("open=%v visible=%d want true/2", g.Open(), len(g.VisibleChildren()))
	}
}

func TestFloatButton_PRD_FB08(t *testing.T) {
	g := float_button.NewFloatButtonGroup(float_button.NewFloatButton())
	g.SetTrigger(float_button.FloatButtonTriggerClick)
	g.SetOpen(true)
	if !g.Open() {
		t.Fatal("SetOpen(true) must open")
	}
	g.SetOpen(false)
	if g.Open() {
		t.Fatal("controlled open=false must close")
	}
	if len(g.VisibleChildren()) != 0 {
		t.Fatal("controlled close hides children")
	}
	if !g.IsControlled() {
		t.Fatal("SetOpen marks controlled")
	}
}

func TestFloatButton_PRD_FB09(t *testing.T) {
	a := float_button.NewFloatButton()
	c := float_button.NewFloatButton()
	g := float_button.NewFloatButtonGroup(a, c)
	g.SetTrigger(float_button.FloatButtonTriggerClick)
	g.SetPlacement(float_button.FloatButtonPlacementLeft)
	g.SetOpen(true)
	sz := layoutSize(t, g.Node(), rendering.Loose(800, 800))
	if math.Abs(sz.Height-40) > 0.5 || sz.Width <= 40 {
		t.Fatalf("left layout=%vx%v want horizontal row", sz.Width, sz.Height)
	}
	trig := g.Node().Children()[0].Offset()
	kid := a.Node().Offset()
	if !(kid.X < trig.X) {
		t.Fatalf("left placement: child x=%v trigger x=%v want child left", kid.X, trig.X)
	}
}

func TestFloatButton_PRD_FB13(t *testing.T) {
	b := float_button.NewFloatButton()
	if !b.NeedAriaLabel() {
		t.Fatal("icon-only button must require AriaLabel")
	}
	if b.Role() != "button" {
		t.Fatalf("role=%q want button", b.Role())
	}
	b.SetAriaLabel("back to top")
	if b.NeedAriaLabel() {
		t.Fatal("label clears the requirement")
	}
	if b.AccessibleName() != "back to top" {
		t.Fatalf("name=%q", b.AccessibleName())
	}
	if b.AriaLabel() != "back to top" {
		t.Fatalf("label=%q", b.AriaLabel())
	}
}

func TestFloatButton_PRD_FB14(t *testing.T) {
	f := loadFloatSpec(t)
	b := float_button.NewFloatButton()
	n := 0
	b.SetOnClick(func() { n++ })
	if !b.Click() || n != 1 {
		t.Fatalf("basic click n=%d", n)
	}
	sz := layoutSize(t, b.Node(), rendering.Tight(f.Size, f.Size))
	if math.Abs(sz.Width-f.Size) > 0.5 {
		t.Fatalf("edge=%v want 40", sz.Width)
	}
}

func TestFloatButton_PRD_FB15(t *testing.T) {
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	d := float_button.NewFloatButton()
	p := float_button.NewFloatButton()
	p.SetType(float_button.ButtonTypePrimary)
	if d.EffectiveBackground() != toRGBA(tok.ColorBgContainer) {
		t.Fatal("default shows container bg")
	}
	if p.EffectiveBackground() != toRGBA(tok.ColorPrimary) {
		t.Fatal("primary shows ColorPrimary")
	}
	if d.EffectiveForeground() == p.EffectiveForeground() {
		t.Fatal("text colors must differ between types")
	}
}

func TestFloatButton_PRD_FB16(t *testing.T) {
	f := loadFloatSpec(t)
	c := float_button.NewFloatButton()
	c.SetShape(float_button.FloatButtonShapeCircle)
	s := float_button.NewFloatButton()
	s.SetShape(float_button.FloatButtonShapeSquare)
	if math.Abs(c.EffectiveRadius()-f.Size/2) > 0.5 {
		t.Fatalf("circle r=%v want 20", c.EffectiveRadius())
	}
	if math.Abs(s.EffectiveRadius()-f.RadiusSquare) > 0.5 {
		t.Fatalf("square r=%v want 8", s.EffectiveRadius())
	}
	paintButton(c, 64)
	paintButton(s, 64)
}

func TestFloatButton_PRD_FB17(t *testing.T) {
	f := loadFloatSpec(t)
	b := float_button.NewFloatButton()
	b.SetContent("help")
	if b.Content() != "help" {
		t.Fatalf("content=%q", b.Content())
	}
	if math.Abs(b.ContentFontSize()-f.ContentFontSize) > 0.5 {
		t.Fatalf("font=%v want 12", b.ContentFontSize())
	}
	if b.AccessibleName() != "help" {
		t.Fatalf("name=%q defaults to content", b.AccessibleName())
	}
	paintButton(b, 64)
}

func TestFloatButton_PRD_FB18(t *testing.T) {
	b := float_button.NewFloatButton()
	b.SetTooltip("bubble tip")
	n := 0
	b.SetOnClick(func() { n++ })
	b.SetHover(true)
	if !b.TooltipVisible() {
		t.Fatal("hover must show string bubble")
	}
	if !b.Click() || n != 1 {
		t.Fatalf("bubble must not steal main click n=%d", n)
	}
}

func TestFloatButton_PRD_FB19(t *testing.T) {
	f := loadFloatSpec(t)
	kids := []*float_button.FloatButton{
		float_button.NewFloatButton(),
		float_button.NewFloatButton(),
		float_button.NewFloatButton(),
	}
	g := float_button.NewFloatButtonGroup(kids...)
	if len(g.VisibleChildren()) != 3 {
		t.Fatalf("plain group shows %d want 3", len(g.VisibleChildren()))
	}
	layoutSize(t, g.Node(), rendering.Loose(800, 800))
	step := f.Size + f.GroupGap
	for i := 1; i < 3; i++ {
		dy := kids[i].Node().Offset().Y - kids[i-1].Node().Offset().Y
		if math.Abs(dy-step) > 0.5 {
			t.Fatalf("gap %d->%d =%v want %v", i-1, i, dy, step)
		}
	}
}

func TestFloatButton_PRD_FB20(t *testing.T) {
	g := float_button.NewFloatButtonGroup(
		float_button.NewFloatButton(), float_button.NewFloatButton())
	g.SetTrigger(float_button.FloatButtonTriggerClick)
	var seq []bool
	g.SetOnOpenChange(func(v bool) { seq = append(seq, v) })
	g.ClickTrigger()
	if !g.Open() {
		t.Fatal("trigger opens menu")
	}
	if !g.HandleOutsideClick() {
		t.Fatal("outside click must close")
	}
	if g.Open() {
		t.Fatal("outside close hides menu")
	}
	if len(seq) != 2 || !seq[0] || seq[1] {
		t.Fatalf("onOpenChange=%v want [true false]", seq)
	}
	// Hover trigger pair.
	h := float_button.NewFloatButtonGroup(float_button.NewFloatButton())
	h.SetTrigger(float_button.FloatButtonTriggerHover)
	h.SetHover(true)
	if !h.Open() {
		t.Fatal("hover opens menu")
	}
	h.SetHover(false)
	if h.Open() {
		t.Fatal("hover leave closes menu")
	}
}

func TestFloatButton_PRD_FB21(t *testing.T) {
	g := float_button.NewFloatButtonGroup(float_button.NewFloatButton())
	g.SetTrigger(float_button.FloatButtonTriggerClick)
	g.SetOpen(true)
	if !g.Open() || len(g.VisibleChildren()) != 1 {
		t.Fatal("controlled open shows children")
	}
	g.SetOpen(false)
	if g.Open() || len(g.VisibleChildren()) != 0 {
		t.Fatal("controlled close hides children")
	}
}

func TestFloatButton_PRD_FB22(t *testing.T) {
	f := loadFloatSpec(t)
	b := float_button.NewFloatButton()
	b.SetShape(float_button.FloatButtonShapeSquare)
	if math.Abs(b.EffectiveSize()-f.Size) > 0.5 {
		t.Fatalf("edge=%v want 40", b.EffectiveSize())
	}
	if math.Abs(b.EffectiveRadius()-f.RadiusSquare) > 0.5 {
		t.Fatalf("square r=%v want 8", b.EffectiveRadius())
	}
}

func TestFloatButton_PRD_FB23(t *testing.T) {
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	b := float_button.NewFloatButton()
	if got := b.EffectiveBackground(); got != toRGBA(tok.ColorBgContainer) {
		t.Fatalf("default skin must walk Theme Token, bg=%+v", got)
	}
	// Custom provider recolors the button: proves Token wiring, not hardcode.
	base := theme.DefaultTokens()
	base.ColorPrimary = theme.RGBA(255, 0, 0, 1)
	p := theme.NewProvider(base)
	pb := float_button.NewFloatButton()
	pb.SetType(float_button.ButtonTypePrimary)
	pb.SetProvider(p)
	if got := pb.EffectiveBackground(); got != (render.RGBA{R: 1, G: 0, B: 0, A: 1}) {
		t.Fatalf("custom Token primary bg=%+v", got)
	}
}

func TestFloatButton_PRD_FB24(t *testing.T) {
	tok := theme.Default.Current()
	toRGBA := func(c theme.Color) render.RGBA {
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
	}
	b := float_button.NewFloatButton()
	b.SetDisabled(true)
	if got := b.EffectiveForeground(); got != toRGBA(tok.ColorTextDisabled) {
		t.Fatalf("disabled fg=%+v want ColorTextDisabled", got)
	}
	if b.HasHoverHighlight() {
		t.Fatal("disabled must not highlight on hover")
	}
	plain := b.EffectiveBackground()
	b.SetHover(true)
	if got := b.EffectiveBackground(); got != plain {
		t.Fatal("disabled hover must not recolor")
	}
}

func TestFloatButton_PRD_FB25(t *testing.T) {
	b := float_button.NewFloatButton()
	if !b.Focusable() {
		t.Fatal("button must take Focus")
	}
	if !b.Focus() || !b.Focused() {
		t.Fatal("Focus must latch")
	}
	if !b.FocusRingVisible() {
		t.Fatal("Focus ring must be visible")
	}
	n := 0
	b.SetOnClick(func() { n++ })
	if !b.KeyActivate("Enter") || n != 1 {
		t.Fatalf("Enter activates n=%d", n)
	}
	if !b.KeyActivate("Space") || n != 2 {
		t.Fatalf("Space activates n=%d", n)
	}
	b.Blur()
	if b.Focused() || b.FocusRingVisible() {
		t.Fatal("Blur clears Focus ring")
	}
	d := float_button.NewFloatButton()
	d.SetDisabled(true)
	if d.Focusable() || d.Focus() {
		t.Fatal("disabled skips Focus")
	}
}

func TestFloatButton_PRD_FB26(t *testing.T) {
	b := float_button.NewFloatButton()
	n := 0
	b.SetOnClick(func() { n++ })
	b.SetLoading(true)
	if b.Click() {
		t.Fatal("loading must swallow onClick")
	}
	if n != 0 {
		t.Fatalf("n=%d want 0", n)
	}
	if !b.WantsFrame() {
		t.Fatal("spinner wants frames")
	}
	a0 := b.Phase()
	b.Tick(0.25)
	if b.Phase() == a0 {
		t.Fatal("Tick must advance spinner")
	}
	paintButton(b, 64)
}
