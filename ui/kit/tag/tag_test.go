package tag_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/tag"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type tagFile struct {
	FontSize    float64  `json:"fontSize"`
	Radius      float64  `json:"radius"`
	PadH        float64  `json:"padH"`
	PadV        float64  `json:"padV"`
	ContentH    float64  `json:"contentH"`
	HeightMin   float64  `json:"heightMin"`
	HeightMax   float64  `json:"heightMax"`
	IconMin     float64  `json:"iconSizeMin"`
	IconMax     float64  `json:"iconSizeMax"`
	IconGap     float64  `json:"iconGap"`
	CloseGap    float64  `json:"closeGap"`
	GroupGap    float64  `json:"groupGap"`
	Tolerance   float64  `json:"tolerance"`
	Presets     []string `json:"presets"`
	Statuses    []string `json:"statuses"`
	Variants    []string `json:"variants"`
}

func loadTag(t *testing.T) tagFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "tag.json"))
	if err != nil {
		t.Fatalf("read tag.json: %v", err)
	}
	var f tagFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse tag.json: %v", err)
	}
	if f.Tolerance <= 0 || len(f.Presets) == 0 {
		t.Fatal("bad tag.json")
	}
	return f
}

func approx(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func paintTag(t *testing.T, n rendering.RenderObject) {
	t.Helper()
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	n.Paint(rendering.NewPaintContext(dc, 1))
}

func TestTag_PRD_TAG01_Defaults(t *testing.T) {
	tg := tag.NewTag("Tag")
	if tg.Variant() != tag.TagVariantFilled || tg.EffectiveVariant() != tag.TagVariantFilled {
		t.Fatalf("variant=%q want filled", tg.Variant())
	}
	if tg.Closable() || tg.Disabled() || tg.Hidden() || !tg.Visible() {
		t.Fatal("defaults: non-closable non-disabled visible")
	}
	if tg.EffectiveBorderWidth() != 0 {
		t.Fatalf("border=%v want 0", tg.EffectiveBorderWidth())
	}
	if tg.HasIcon() || tg.CloseNode() != nil {
		t.Fatal("defaults: no icon no close")
	}
	if tg.Focusable() || tg.Role() != "" {
		t.Fatalf("static focus=%v role=%q", tg.Focusable(), tg.Role())
	}
	if tg.AriaLabel() != "Tag" {
		t.Fatalf("aria=%q want label", tg.AriaLabel())
	}
	if tg.Node() == nil || tg.ChromeNode() == nil {
		t.Fatal("nodes nil")
	}
	_ = theme.Default.Current()
}

func TestTag_PRD_TAG02_PresetRed(t *testing.T) {
	def := tag.NewTag("Tag").EffectiveChrome()
	tg := tag.NewTag("Tag")
	tg.SetColor("red")
	got := tg.EffectiveChrome()
	if got.Bg == def.Bg && got.Text == def.Text {
		t.Fatal("red skin must differ from default gray")
	}
	f := loadTag(t)
	found := false
	for _, p := range f.Presets {
		if p == "red" {
			found = true
		}
	}
	if !found {
		t.Fatal("red missing in tag.json presets")
	}
	_ = theme.Default.Current()
}

func TestTag_PRD_TAG03_ClosableClose(t *testing.T) {
	tg := tag.NewTag("Deletable")
	tg.SetClosable(true)
	if tg.CloseNode() == nil || !tg.HasClose() {
		t.Fatal("close affordance missing")
	}
	calls := 0
	tg.OnClose = func(*tag.TagCloseEvent) { calls++ }
	tg.Close()
	if calls != 1 || !tg.Hidden() || tg.Visible() {
		t.Fatalf("close calls=%d hidden=%v", calls, tg.Hidden())
	}
	// PreventDefault keeps visible (TAG-S3).
	tg2 := tag.NewTag("Keep")
	tg2.SetClosable(true)
	tg2.OnClose = func(e *tag.TagCloseEvent) { e.PreventDefault() }
	tg2.Close()
	if tg2.Hidden() {
		t.Fatal("PreventDefault must keep visible")
	}
	if !tg2.Focusable() || tg2.Role() != "button" {
		t.Fatalf("closable focus=%v role=%q", tg2.Focusable(), tg2.Role())
	}
}

func TestTag_PRD_TAG04_CheckableToggle(t *testing.T) {
	ct := tag.NewCheckableTag("Pick")
	if ct.Checked() {
		t.Fatal("default unchecked")
	}
	var got []bool
	ct.SetOnChange(func(b bool) { got = append(got, b) })
	ct.Toggle()
	if !ct.Checked() || len(got) != 1 || !got[0] {
		t.Fatalf("toggle checked=%v calls=%v", ct.Checked(), got)
	}
	ct.Press()
	if ct.Checked() || len(got) != 2 || got[1] {
		t.Fatalf("second toggle checked=%v calls=%v", ct.Checked(), got)
	}
	if ct.Role() != "checkbox" || ct.AriaLabel() != "Pick" || !ct.Focusable() {
		t.Fatalf("a11y role=%q aria=%q focus=%v", ct.Role(), ct.AriaLabel(), ct.Focusable())
	}
}

func TestTag_PRD_TAG05_BorderedVariant(t *testing.T) {
	a := tag.NewTag("A")
	a.SetBordered(false)
	if a.EffectiveBorderWidth() != 0 || a.EffectiveVariant() != tag.TagVariantFilled {
		t.Fatalf("bordered=false border=%v variant=%q", a.EffectiveBorderWidth(), a.EffectiveVariant())
	}
	b := tag.NewTag("B")
	b.SetVariant(tag.TagVariantFilled)
	if b.EffectiveBorderWidth() != 0 {
		t.Fatalf("filled border=%v want 0", b.EffectiveBorderWidth())
	}
	c := tag.NewTag("C")
	c.SetVariant(tag.TagVariantOutlined)
	if c.EffectiveBorderWidth() != 1 {
		t.Fatalf("outlined border=%v want 1", c.EffectiveBorderWidth())
	}
	d := tag.NewTag("D")
	d.SetVariant(tag.TagVariantSolid)
	if d.EffectiveBorderWidth() != 0 {
		t.Fatalf("solid border=%v want 0", d.EffectiveBorderWidth())
	}
	_ = theme.Default.Current()
}

func TestTag_PRD_TAG06_Icon(t *testing.T) {
	tg := tag.NewTag("Msg")
	if tg.HasIcon() {
		t.Fatal("no icon default")
	}
	w0 := tg.PreferredSize().Width
	tg.SetIcon("check")
	if !tg.HasIcon() {
		t.Fatal("icon missing")
	}
	if tg.PreferredSize().Width <= w0 {
		t.Fatal("icon must widen tag")
	}
	tg2 := tag.NewTag("Node")
	tg2.SetIconNode(rendering.NewRenderColorBox(10, 10, 0.2, 0.3, 0.8, 1))
	if !tg2.HasIcon() {
		t.Fatal("icon node missing")
	}
	tg.Layout(rendering.Loose(300, 100))
	paintTag(t, tg.Node())
}

func TestTag_PRD_TAG07_CustomHex(t *testing.T) {
	tg := tag.NewTag("Custom")
	tg.SetColor("#f50")
	got := tg.EffectiveChrome()
	want := render.Hex("#ff5500")
	if got.Text != want {
		t.Fatalf("filled hex text=%+v want %+v", got.Text, want)
	}
	def := tag.NewTag("Tag").EffectiveChrome()
	if got.Bg == def.Bg {
		t.Fatal("hex bg must derive, not default gray")
	}
	// Solid derives opaque base.
	s := tag.NewTag("S")
	s.SetVariant(tag.TagVariantSolid)
	s.SetColor("#f50")
	if sg := s.EffectiveChrome(); sg.Bg != want {
		t.Fatalf("solid hex bg=%+v want %+v", sg.Bg, want)
	}
	// Outlined derives colored border.
	o := tag.NewTag("O")
	o.SetVariant(tag.TagVariantOutlined)
	o.SetColor("#f50")
	if og := o.EffectiveChrome(); og.BorderWidth != 1 || og.Border == def.Border {
		t.Fatalf("outlined hex border=%+v width=%v", og.Border, og.BorderWidth)
	}
}

func TestTag_PRD_TAG08_BasicExample(t *testing.T) {
	plain := tag.NewTag("Tag 1")
	plain.Layout(rendering.Loose(300, 100))
	paintTag(t, plain.Node())
	linked := tag.NewTag("Link")
	linked.SetOnClick(func() {})
	linked.Layout(rendering.Loose(300, 100))
	paintTag(t, linked.Node())
	closable := tag.NewTag("Closable")
	closable.SetClosable(true)
	closable.Layout(rendering.Loose(300, 100))
	paintTag(t, closable.Node())
	if !linked.Focusable() || linked.Role() != "button" {
		t.Fatal("clickable tag must be button focusable")
	}
}

func TestTag_PRD_TAG09_ColorfulExample(t *testing.T) {
	f := loadTag(t)
	variants := map[string]tag.TagVariant{"filled": tag.TagVariantFilled, "solid": tag.TagVariantSolid, "outlined": tag.TagVariantOutlined}
	for _, p := range f.Presets {
		for _, v := range f.Variants {
			tg := tag.NewTag(p)
			tg.SetVariant(variants[v])
			tg.SetColor(p)
			tg.Layout(rendering.Loose(400, 100))
			paintTag(t, tg.Node())
		}
	}
	// Custom colors from the doc example.
	for _, hex := range []string{"#f50", "#2db7f5", "#87d068", "#108ee9"} {
		tg := tag.NewTag(hex)
		tg.SetColor(hex)
		tg.Layout(rendering.Loose(400, 100))
		paintTag(t, tg.Node())
	}
	_ = theme.Default.Current()
}

func TestTag_PRD_TAG10_ControlDynamic(t *testing.T) {
	tags := []*tag.Tag{tag.NewTag("A"), tag.NewTag("B"), tag.NewTag("C")}
	for _, tg := range tags {
		tg.SetClosable(true)
		tg.Layout(rendering.Loose(400, 100))
	}
	// Delete middle (instant, P0 allows instant).
	tags[1].Close()
	visible := 0
	for _, tg := range tags {
		if tg.Visible() {
			visible++
		}
	}
	if visible != 2 {
		t.Fatalf("visible=%d want 2 after close", visible)
	}
	// Add new.
	tags = append(tags, tag.NewTag("D"))
	visible = 0
	for _, tg := range tags {
		tg.Layout(rendering.Loose(400, 100))
		if tg.Visible() {
			visible++
		}
	}
	if visible != 3 {
		t.Fatalf("visible=%d want 3 after add", visible)
	}
}

func TestTag_PRD_TAG11_CheckableGroup(t *testing.T) {
	// Single.
	g := tag.NewCheckableTagGroup(tag.TagOption{Label: "A", Value: "a"}, tag.TagOption{Label: "B", Value: "b"})
	var single string
	g.SetOnChange(func(v string) { single = v })
	g.PressOption("b")
	if g.Value() != "b" || single != "b" {
		t.Fatalf("single value=%q call=%q", g.Value(), single)
	}
	g.PressOption("a")
	if g.Value() != "a" {
		t.Fatalf("single reselect=%q", g.Value())
	}
	// Multi.
	m := tag.NewCheckableTagGroup(tag.TagOption{Label: "A", Value: "a"}, tag.TagOption{Label: "B", Value: "b"})
	m.SetMultiple(true)
	var multi []string
	m.SetOnChangeMulti(func(vs []string) { multi = append([]string(nil), vs...) })
	m.PressOption("a")
	m.PressOption("b")
	if len(m.Values()) != 2 || len(multi) != 2 {
		t.Fatalf("multi values=%v calls=%v", m.Values(), multi)
	}
	m.PressOption("a")
	if len(m.Values()) != 1 || m.Values()[0] != "b" {
		t.Fatalf("multi toggle off=%v", m.Values())
	}
	// Default value path.
	d := tag.NewCheckableTagGroup(tag.TagOption{Label: "A", Value: "a"}, tag.TagOption{Label: "B", Value: "b"})
	d.SetDefaultValue("a")
	if d.Value() != "a" {
		t.Fatalf("default value=%q", d.Value())
	}
	g.Layout(rendering.Loose(400, 200))
	m.Layout(rendering.Loose(400, 200))
}

func TestTag_PRD_TAG12_AnimationInstant(t *testing.T) {
	// P0 allows instant add/remove; assert synchronous hide/show.
	tg := tag.NewTag("Anim")
	tg.SetClosable(true)
	tg.Layout(rendering.Loose(200, 100))
	tg.Close()
	if !tg.Hidden() {
		t.Fatal("close must hide instantly")
	}
	tg.Show()
	if tg.Hidden() || !tg.Visible() {
		t.Fatal("show must restore instantly")
	}
	paintTag(t, tg.Node())
}

func TestTag_PRD_TAG13_IconExample(t *testing.T) {
	tg := tag.NewTag("Icon tag")
	tg.SetIcon("check")
	if !tg.HasIcon() {
		t.Fatal("tag icon missing")
	}
	tg.Layout(rendering.Loose(300, 100))
	paintTag(t, tg.Node())
	ct := tag.NewCheckableTag("Icon check")
	ct.SetIcon("star")
	if !ct.HasIcon() {
		t.Fatal("checkable icon missing")
	}
	ct.Layout(rendering.Loose(300, 100))
	paintTag(t, ct.Node())
	_ = theme.Default.Current()
}

func TestTag_PRD_TAG14_StatusVariant(t *testing.T) {
	f := loadTag(t)
	tok := theme.Default.Current()
	statusTone := map[string]render.RGBA{
		"success": {R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: tok.ColorSuccess.A},
		"error":   {R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A},
		"warning": {R: tok.ColorWarning.R, G: tok.ColorWarning.G, B: tok.ColorWarning.B, A: tok.ColorWarning.A},
	}
	for _, s := range f.Statuses {
		for _, v := range f.Variants {
			tg := tag.NewTag(s)
			switch v {
			case "solid":
				tg.SetVariant(tag.TagVariantSolid)
			case "outlined":
				tg.SetVariant(tag.TagVariantOutlined)
			default:
				tg.SetVariant(tag.TagVariantFilled)
			}
			tg.SetColor(s)
			ch := tg.EffectiveChrome()
			tg.Layout(rendering.Loose(300, 100))
			paintTag(t, tg.Node())
			if s == "processing" {
				continue
			}
			if want, ok := statusTone[s]; ok && v == "filled" && ch.Text != want {
				t.Fatalf("%s filled text=%+v want %+v", s, ch.Text, want)
			}
		}
	}
}

func TestTag_PRD_TAG15_DraggableReorder(t *testing.T) {
	g := tag.NewCheckableTagGroup(
		tag.TagOption{Label: "A", Value: "a"},
		tag.TagOption{Label: "B", Value: "b"},
		tag.TagOption{Label: "C", Value: "c"},
	)
	before := g.Order()
	if len(before) != 3 || before[0] != "a" {
		t.Fatalf("order=%v", before)
	}
	g.Reorder(0, 2)
	after := g.Order()
	if len(after) != 3 || after[2] != "a" || after[0] != "b" {
		t.Fatalf("reordered=%v want b,c,a", after)
	}
	g.Layout(rendering.Loose(400, 200))
	paintTag(t, g.Node())
}

func TestTag_PRD_TAG16_MetricsLayoutMatrix(t *testing.T) {
	f := loadTag(t)
	tg := tag.NewTag("Metrics")
	if !approx(tg.EffectiveFontSize(), f.FontSize, f.Tolerance) {
		t.Fatalf("font=%v want %v", tg.EffectiveFontSize(), f.FontSize)
	}
	if !approx(tg.EffectiveRadius(), f.Radius, f.Tolerance) {
		t.Fatalf("radius=%v want %v", tg.EffectiveRadius(), f.Radius)
	}
	if !approx(tg.EffectivePadH(), f.PadH, f.Tolerance) {
		t.Fatalf("padH=%v want %v", tg.EffectivePadH(), f.PadH)
	}
	h := tg.EffectiveHeight()
	if h < f.HeightMin-f.Tolerance || h > f.HeightMax+f.Tolerance {
		t.Fatalf("height=%v want %v..%v", h, f.HeightMin, f.HeightMax)
	}
	sz := tg.Layout(rendering.Loose(400, 100))
	if math.Abs(sz.Height-h) > f.Tolerance {
		t.Fatalf("loose height=%v want %v", sz.Height, h)
	}
	if sz.Width <= 0 {
		t.Fatal("loose width must be positive")
	}
	// Matrix: Exact forces, Loose contents, MinMax clamps.
	exact := tg.Layout(rendering.Tight(200, 40))
	if math.Abs(exact.Width-200) > f.Tolerance || math.Abs(exact.Height-40) > f.Tolerance {
		t.Fatalf("exact=%v want 200x40", exact)
	}
	loose := tg.Layout(rendering.Loose(400, 100))
	if loose.Width <= 0 || math.Abs(loose.Height-h) > f.Tolerance {
		t.Fatalf("loose=%v height want %v", loose, h)
	}
	mm := tg.Layout(rendering.Constraints{MinWidth: 200, MaxWidth: 400, MinHeight: 0, MaxHeight: 100})
	if mm.Width < 200-f.Tolerance || mm.Width > 400+f.Tolerance {
		t.Fatalf("minmax=%v want 200..400", mm)
	}
	// Icon/group gaps from file.
	if !approx(tg.EffectiveCloseGap(), f.CloseGap, f.Tolerance) {
		t.Fatalf("closeGap=%v want %v", tg.EffectiveCloseGap(), f.CloseGap)
	}
	g := tag.NewCheckableTagGroup(tag.TagOption{Label: "A"})
	if !approx(g.EffectiveGroupGap(), f.GroupGap, f.Tolerance) {
		t.Fatalf("groupGap=%v want %v", g.EffectiveGroupGap(), f.GroupGap)
	}
	ct := tag.NewCheckableTag("C")
	if ct.EffectiveIconSize() < f.IconMin-f.Tolerance || ct.EffectiveIconSize() > f.IconMax+f.Tolerance {
		t.Fatalf("icon=%v want %v..%v", ct.EffectiveIconSize(), f.IconMin, f.IconMax)
	}
	_ = theme.Default.Current()
}

func TestTag_PRD_TAG17_ThemeDefault(t *testing.T) {
	tok := theme.Default.Current()
	def := tag.NewTag("Tag").EffectiveChrome()
	wantBg := render.RGBA{R: tok.ColorFillTertiary.R, G: tok.ColorFillTertiary.G, B: tok.ColorFillTertiary.B, A: tok.ColorFillTertiary.A}
	wantText := render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: tok.ColorText.A}
	if def.Bg != wantBg {
		t.Fatalf("default bg=%+v want FillTertiary %+v", def.Bg, wantBg)
	}
	if def.Text != wantText {
		t.Fatalf("default text=%+v want ColorText %+v", def.Text, wantText)
	}
	// Brand must not leak into the default skin.
	alt := tok
	alt.ColorPrimary = theme.Hex("#ff4d4f")
	tg := tag.NewTag("Tag")
	tg.SetTheme(&alt)
	if got := tg.EffectiveChrome(); got.Bg != wantBg {
		t.Fatalf("default bg follows brand %+v", got.Bg)
	}
	// Checked state does follow the brand.
	ct := tag.NewCheckableTag("C")
	ct.SetChecked(true)
	ct.SetTheme(&alt)
	if got := ct.EffectiveChrome(); got.Bg.R < 0.9 || got.Bg.G > 0.4 {
		t.Fatalf("checked bg=%+v should follow red brand", got.Bg)
	}
}

func TestTag_PRD_TAG18_DisabledLook(t *testing.T) {
	tok := theme.Default.Current()
	tg := tag.NewTag("Off")
	tg.SetDisabled(true)
	tg.SetClosable(true)
	ch := tg.EffectiveChrome()
	wantText := render.RGBA{R: tok.ColorTextDisabled.R, G: tok.ColorTextDisabled.G, B: tok.ColorTextDisabled.B, A: tok.ColorTextDisabled.A}
	if ch.Text != wantText {
		t.Fatalf("disabled text=%+v want %+v", ch.Text, wantText)
	}
	calls := 0
	tg.OnClose = func(*tag.TagCloseEvent) { calls++ }
	tg.Close()
	if calls != 0 || tg.Hidden() {
		t.Fatal("disabled must swallow close")
	}
	clicks := 0
	tg.SetOnClick(func() { clicks++ })
	tg.Click()
	if clicks != 0 {
		t.Fatal("disabled must swallow click")
	}
	if tg.Focusable() || tg.Role() != "" {
		t.Fatalf("disabled focus=%v role=%q", tg.Focusable(), tg.Role())
	}
	ct := tag.NewCheckableTag("Off")
	ct.SetDisabled(true)
	ct.Toggle()
	if ct.Checked() {
		t.Fatal("disabled checkable must not toggle")
	}
	if ct.Focusable() {
		t.Fatal("disabled checkable must not focus")
	}
}

func TestTag_PRD_TAG19_KeyboardFocusRing(t *testing.T) {
	ct := tag.NewCheckableTag("Keys")
	if !ct.Focusable() {
		t.Fatal("checkable must take Tab")
	}
	if ct.FocusRingVisible() {
		t.Fatal("ring off before focus")
	}
	ct.Focus()
	if !ct.Focused() || !ct.FocusRingVisible() {
		t.Fatal("ring must show after Focus")
	}
	ct.PressKey("Space")
	if !ct.Checked() {
		t.Fatal("Space must toggle")
	}
	ct.PressKey("Enter")
	if ct.Checked() {
		t.Fatal("Enter must toggle back")
	}
	ct.Blur()
	if ct.FocusRingVisible() {
		t.Fatal("ring off after Blur")
	}
	if ct.Role() != "checkbox" || ct.AriaLabel() != "Keys" {
		t.Fatalf("role=%q aria=%q", ct.Role(), ct.AriaLabel())
	}
	// Disabled keyboard swallowed.
	ct.SetDisabled(true)
	ct.PressKey("Space")
	if ct.Checked() {
		t.Fatal("disabled must swallow keys")
	}
	// Contrast probe: default checkable text on bg must be readable
	// (composite translucent tokens over white first).
	def := tag.NewCheckableTag("C").EffectiveChrome()
	flatBg := overWhite(def.Bg)
	flatText := over(def.Text, flatBg)
	if contrastRatio(flatBg, flatText) < 3.0 {
		t.Fatalf("contrast=%v want >=3", contrastRatio(flatBg, flatText))
	}
	_ = theme.Default.Current()
}

func over(fg, bg render.RGBA) render.RGBA {
	a := fg.A + bg.A*(1-fg.A)
	if a <= 0 {
		return render.RGBA{}
	}
	return render.RGBA{
		R: (fg.R*fg.A + bg.R*bg.A*(1-fg.A)) / a,
		G: (fg.G*fg.A + bg.G*bg.A*(1-fg.A)) / a,
		B: (fg.B*fg.A + bg.B*bg.A*(1-fg.A)) / a,
		A: a,
	}
}

func overWhite(c render.RGBA) render.RGBA { return over(c, render.White) }

func contrastRatio(a, b render.RGBA) float64 {
	lum := func(c render.RGBA) float64 {
		lin := func(v float64) float64 {
			if v <= 0.03928 {
				return v / 12.92
			}
			return math.Pow((v+0.055)/1.055, 2.4)
		}
		return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
	}
	la, lb := lum(a), lum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
