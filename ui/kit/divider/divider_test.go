package divider_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/divider"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type dividerSpec struct {
	MarginBlock struct {
		Unset         float64 `json:"unset"`
		Small         float64 `json:"small"`
		Medium        float64 `json:"medium"`
		Large         float64 `json:"large"`
		WithTextUnset float64 `json:"withTextUnset"`
	} `json:"marginBlock"`
	LineWidth   float64 `json:"lineWidth"`
	TitleFontSZ struct {
		Plain   float64 `json:"plain"`
		Default float64 `json:"default"`
	} `json:"titleFontSize"`
	OrientationMarginRatio float64 `json:"orientationMarginRatio"`
	Vertical               struct {
		HeightFactor float64 `json:"heightFactor"`
		MarginInline float64 `json:"marginInline"`
		FontSize     float64 `json:"fontSize"`
	} `json:"vertical"`
}

func loadDividerSpec(t *testing.T) dividerSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "divider.json"))
	if err != nil {
		t.Fatalf("read divider.json: %v", err)
	}
	var s dividerSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse divider.json: %v", err)
	}
	return s
}

func paintDivider(d *divider.Divider, w, h int) {
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	d.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func TestDivider_PRD_DIV01_Defaults(t *testing.T) {
	d := divider.NewDivider()
	if d.EffectiveOrientation() != divider.Horizontal {
		t.Fatal("default orientation horizontal")
	}
	if d.EffectiveVariant() != divider.Solid {
		t.Fatal("default variant solid")
	}
	if d.Size() != divider.SizeUnset {
		t.Fatalf("default size unset, got %v", d.Size())
	}
	if d.Plain() || d.Dashed() {
		t.Fatal("default plain/dashed false")
	}
	if d.Title() != "" || d.HasTitle() {
		t.Fatal("default no title")
	}
	if d.TitlePlacement() != divider.Center {
		t.Fatal("default placement center")
	}
	if d.Node() == nil || d.ChromeNode() == nil {
		t.Fatal("node/chrome nil")
	}
	if d.MarginBlock() < 23.5 || d.MarginBlock() > 24.5 {
		t.Fatalf("default marginBlock=%v want 24", d.MarginBlock())
	}
}

func TestDivider_PRD_DIV02_DefaultLayoutMatrix(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDivider()
	// Loose bounded fills width; height = 2*margin + line.
	sz := d.Layout(rendering.Loose(300, 100))
	wantH := 2*spec.MarginBlock.Unset + spec.LineWidth
	if math.Abs(sz.Width-300) > 0.5 || math.Abs(sz.Height-wantH) > 0.5 {
		t.Fatalf("loose layout=%v want 300x%v", sz, wantH)
	}
	// Exact forces size.
	sz = d.Layout(rendering.Tight(240, wantH))
	if math.Abs(sz.Width-240) > 0.5 || math.Abs(sz.Height-wantH) > 0.5 {
		t.Fatalf("exact layout=%v", sz)
	}
	// Unbounded falls back to 200 wide.
	sz = d.Layout(rendering.Expand())
	if math.Abs(sz.Width-200) > 0.5 || math.Abs(sz.Height-wantH) > 0.5 {
		t.Fatalf("expand layout=%v want 200x%v", sz, wantH)
	}
	paintDivider(d, 120, 60)
}

func TestDivider_PRD_DIV03_Vertical(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDivider()
	d.SetVertical(true)
	if !d.IsVertical() || d.EffectiveOrientation() != divider.Vertical {
		t.Fatal("vertical flag")
	}
	sz := d.Layout(rendering.Loose(100, 100))
	wantW := 2*spec.Vertical.MarginInline + spec.LineWidth
	wantH := spec.Vertical.HeightFactor * spec.Vertical.FontSize
	if math.Abs(sz.Width-wantW) > 0.5 || math.Abs(sz.Height-wantH) > 0.5 {
		t.Fatalf("vertical layout=%v want %vx%v", sz, wantW, wantH)
	}
	if d.MarginBlock() != 0 {
		t.Fatalf("vertical marginBlock=%v want 0", d.MarginBlock())
	}
	if math.Abs(d.MarginInline()-spec.Vertical.MarginInline) > 0.5 {
		t.Fatalf("marginInline=%v", d.MarginInline())
	}
	// Orientation wins over vertical flag.
	d.SetOrientation(divider.Horizontal)
	d.SetVertical(true)
	if d.IsVertical() {
		t.Fatal("orientation should win over vertical flag")
	}
	d.SetOrientation(divider.Vertical)
	if !d.IsVertical() {
		t.Fatal("explicit orientation vertical")
	}
	paintDivider(d, 40, 40)
}

func TestDivider_PRD_DIV04_Dashed(t *testing.T) {
	d := divider.NewDivider()
	d.SetDashed(true)
	if d.EffectiveVariant() != divider.Dashed {
		t.Fatal("dashed sugar")
	}
	d.SetDashed(false)
	d.SetVariant(divider.Dashed)
	if d.EffectiveVariant() != divider.Dashed {
		t.Fatal("variant dashed")
	}
	paintDivider(d, 120, 40)
}

func TestDivider_PRD_DIV05_Dotted(t *testing.T) {
	d := divider.NewDivider()
	d.SetVariant(divider.Dotted)
	if d.EffectiveVariant() != divider.Dotted {
		t.Fatal("dotted")
	}
	// Dotted wins over dashed sugar.
	d.SetDashed(true)
	if d.EffectiveVariant() != divider.Dotted {
		t.Fatal("dotted should win over dashed")
	}
	paintDivider(d, 120, 40)
}

func TestDivider_PRD_DIV06_TitleCenter(t *testing.T) {
	d := divider.NewDividerWithTitle("Text")
	if !d.HasTitle() {
		t.Fatal("has title")
	}
	gs, ge := d.RailGrows()
	if gs != 1 || ge != 1 {
		t.Fatalf("center grows=%v/%v", gs, ge)
	}
	sz := d.Layout(rendering.Loose(300, 100))
	if math.Abs(sz.Width-300) > 0.5 {
		t.Fatalf("with-text width=%v", sz.Width)
	}
	if math.Abs(d.RailStartWidth()-d.RailEndWidth()) > 0.5 {
		t.Fatalf("center rails %v vs %v", d.RailStartWidth(), d.RailEndWidth())
	}
	if d.TitleBlockWidth() <= 0 {
		t.Fatal("title block width")
	}
	paintDivider(d, 160, 60)
}

func TestDivider_PRD_DIV07_TitleStart(t *testing.T) {
	d := divider.NewDividerWithTitle("Text")
	d.SetTitlePlacement(divider.Start)
	gs, ge := d.RailGrows()
	if math.Abs(gs-0.05) > 1e-9 || math.Abs(ge-0.95) > 1e-9 {
		t.Fatalf("start grows=%v/%v", gs, ge)
	}
	d.Layout(rendering.Loose(300, 100))
	if d.RailStartWidth() >= d.RailEndWidth() {
		t.Fatalf("start rail %v should be shorter than %v", d.RailStartWidth(), d.RailEndWidth())
	}
	// Custom ratio path.
	d.SetOrientationMargin(0.2)
	if math.Abs(d.OrientationMarginRatio()-0.2) > 1e-9 {
		t.Fatalf("ratio=%v", d.OrientationMarginRatio())
	}
	paintDivider(d, 160, 60)
}

func TestDivider_PRD_DIV08_TitleEnd(t *testing.T) {
	d := divider.NewDividerWithTitle("Text")
	d.SetTitlePlacement(divider.End)
	gs, ge := d.RailGrows()
	if math.Abs(gs-0.95) > 1e-9 || math.Abs(ge-0.05) > 1e-9 {
		t.Fatalf("end grows=%v/%v", gs, ge)
	}
	d.Layout(rendering.Loose(300, 100))
	if d.RailEndWidth() >= d.RailStartWidth() {
		t.Fatalf("end rail %v should be shorter than %v", d.RailEndWidth(), d.RailStartWidth())
	}
	paintDivider(d, 160, 60)
}

func TestDivider_PRD_DIV09_Plain(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDividerWithTitle("Text")
	d.SetPlain(true)
	if math.Abs(d.TitleFontSize()-spec.TitleFontSZ.Plain) > 0.5 {
		t.Fatalf("plain size=%v", d.TitleFontSize())
	}
	paintDivider(d, 120, 50)
}

func TestDivider_PRD_DIV10_NonPlain(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDividerWithTitle("Text")
	if math.Abs(d.TitleFontSize()-spec.TitleFontSZ.Default) > 0.5 {
		t.Fatalf("default title size=%v", d.TitleFontSize())
	}
	paintDivider(d, 120, 50)
}

func TestDivider_PRD_DIV11_Size档(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDivider()
	d.SetSize(divider.Small)
	if math.Abs(d.MarginBlock()-spec.MarginBlock.Small) > 0.5 {
		t.Fatalf("small=%v", d.MarginBlock())
	}
	d.SetSize(divider.Medium)
	if math.Abs(d.MarginBlock()-spec.MarginBlock.Medium) > 0.5 {
		t.Fatalf("medium=%v", d.MarginBlock())
	}
	d.SetSize(divider.Large)
	if math.Abs(d.MarginBlock()-spec.MarginBlock.Large) > 0.5 {
		t.Fatalf("large=%v", d.MarginBlock())
	}
	// Unset without title equals large rhythm.
	d2 := divider.NewDivider()
	if math.Abs(d2.MarginBlock()-spec.MarginBlock.Unset) > 0.5 {
		t.Fatalf("unset=%v", d2.MarginBlock())
	}
}

func TestDivider_PRD_DIV12_LineWidth(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDivider()
	if math.Abs(d.LineWidth()-spec.LineWidth) > 0.5 {
		t.Fatalf("lineWidth=%v", d.LineWidth())
	}
	tok := theme.Default.Current()
	if tok.LineWidth > 0 && d.LineWidth() != tok.LineWidth {
		t.Fatalf("lineWidth should follow theme %v", tok.LineWidth)
	}
}

func TestDivider_PRD_DIV13_LineColor(t *testing.T) {
	d := divider.NewDivider()
	tok := theme.Default.Current()
	want := render.RGBA{R: tok.ColorBorderSecondary.R, G: tok.ColorBorderSecondary.G, B: tok.ColorBorderSecondary.B, A: tok.ColorBorderSecondary.A}
	if got := d.LineColor(); got != want {
		t.Fatalf("lineColor=%+v want theme ColorBorderSecondary %+v", got, want)
	}
	c := render.RGBA{R: 1, G: 0, B: 0, A: 1}
	d.SetStyle(divider.Style{Border: c})
	if got := d.LineColor(); got != c {
		t.Fatalf("style border=%+v", got)
	}
}

func TestDivider_PRD_DIV14_A11y(t *testing.T) {
	d := divider.NewDivider()
	if d.Role() != "separator" {
		t.Fatalf("role=%q", d.Role())
	}
	if d.Focusable() {
		t.Fatal("divider must not take focus")
	}
	d.SetAriaLabel("section")
	if d.AriaLabel() != "section" {
		t.Fatalf("aria=%q", d.AriaLabel())
	}
	if d.Role() != "separator" {
		t.Fatal("role stays separator with label")
	}
}

func TestDivider_PRD_DIV15_HorizontalCombo(t *testing.T) {
	a := divider.NewDivider()
	b := divider.NewDivider()
	b.SetDashed(true)
	for _, d := range []*divider.Divider{a, b} {
		sz := d.Layout(rendering.Loose(240, 80))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("combo layout=%v", sz)
		}
		if d.Node() == nil || d.ChromeNode() == nil {
			t.Fatal("combo node nil")
		}
		paintDivider(d, 120, 40)
	}
}

func TestDivider_PRD_DIV16_WithTextCombo(t *testing.T) {
	for _, p := range []divider.DividerTitlePlacement{divider.Center, divider.Start, divider.End} {
		d := divider.NewDividerWithTitle("Title")
		d.SetTitlePlacement(p)
		sz := d.Layout(rendering.Loose(280, 90))
		if sz.Width <= 0 || !d.HasTitle() {
			t.Fatalf("placement %v layout=%v", p, sz)
		}
		paintDivider(d, 140, 50)
	}
}

func TestDivider_PRD_DIV17_SizeCombo(t *testing.T) {
	spec := loadDividerSpec(t)
	want := map[divider.DividerSize]float64{
		divider.Small:  spec.MarginBlock.Small,
		divider.Medium: spec.MarginBlock.Medium,
		divider.Large:  spec.MarginBlock.Large,
	}
	for s, w := range want {
		d := divider.NewDivider()
		d.SetSize(s)
		if math.Abs(d.MarginBlock()-w) > 0.5 {
			t.Fatalf("size %v margin=%v want %v", s, d.MarginBlock(), w)
		}
		d.Layout(rendering.Loose(220, 80))
		paintDivider(d, 110, 40)
	}
}

func TestDivider_PRD_DIV18_PlainCombo(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDividerWithTitle("Body")
	d.SetPlain(true)
	if math.Abs(d.TitleFontSize()-spec.TitleFontSZ.Plain) > 0.5 {
		t.Fatalf("plain combo size=%v", d.TitleFontSize())
	}
	tok := theme.Default.Current()
	tc := d.TitleColor()
	if tc != (render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: tok.ColorText.A}) {
		t.Fatalf("plain color=%+v", tc)
	}
	paintDivider(d, 120, 50)
}

func TestDivider_PRD_DIV19_VerticalCombo(t *testing.T) {
	d := divider.NewDivider()
	d.SetOrientation(divider.Vertical)
	sz := d.Layout(rendering.Loose(60, 60))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("vertical combo=%v", sz)
	}
	// Vertical ignores title text.
	d.SetTitle("Ignored")
	if d.HasTitle() {
		t.Fatal("vertical should ignore title")
	}
	paintDivider(d, 30, 30)
}

func TestDivider_PRD_DIV20_VariantCombo(t *testing.T) {
	kinds := map[divider.DividerVariant]divider.DividerVariant{
		divider.Solid:  divider.Solid,
		divider.Dashed: divider.Dashed,
		divider.Dotted: divider.Dotted,
	}
	for v, want := range kinds {
		d := divider.NewDivider()
		d.SetVariant(v)
		if d.EffectiveVariant() != want {
			t.Fatalf("variant %v got %v", v, d.EffectiveVariant())
		}
		d.Layout(rendering.Loose(200, 60))
		paintDivider(d, 120, 40)
	}
}

func TestDivider_PRD_DIV21_SemanticMount(t *testing.T) {
	// P0 only mounts structure; class string depth stays P1.
	d := divider.NewDividerWithTitle("Text")
	hook := rendering.NewRenderColorBox(24, 8, 0.2, 0.4, 0.8, 1)
	d.SetTitleNode(hook)
	if !d.HasTitle() {
		t.Fatal("custom node counts as title")
	}
	sz := d.Layout(rendering.Loose(260, 80))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("custom layout=%v", sz)
	}
	found := false
	for _, c := range d.Node().Children() {
		if c == rendering.RenderObject(hook) {
			found = true
		}
	}
	if !found {
		t.Fatal("custom node should be mounted")
	}
	paintDivider(d, 130, 50)
}

func TestDivider_PRD_DIV22_Metrics(t *testing.T) {
	spec := loadDividerSpec(t)
	d := divider.NewDivider()
	if math.Abs(d.MarginBlock()-spec.MarginBlock.Unset) > 0.5 {
		t.Fatalf("unset margin=%v", d.MarginBlock())
	}
	dt := divider.NewDividerWithTitle("T")
	if math.Abs(dt.MarginBlock()-spec.MarginBlock.WithTextUnset) > 0.5 {
		t.Fatalf("with-text unset=%v", dt.MarginBlock())
	}
	if math.Abs(d.LineWidth()-spec.LineWidth) > 0.5 {
		t.Fatalf("line=%v", d.LineWidth())
	}
	if math.Abs(d.TitleFontSize()-spec.TitleFontSZ.Default) > 0.5 {
		t.Fatalf("title=%v", d.TitleFontSize())
	}
	dt.SetPlain(true)
	if math.Abs(dt.TitleFontSize()-spec.TitleFontSZ.Plain) > 0.5 {
		t.Fatalf("plain=%v", dt.TitleFontSize())
	}
	if math.Abs(d.OrientationMarginRatio()-spec.OrientationMarginRatio) > 1e-9 {
		t.Fatalf("ratio=%v", d.OrientationMarginRatio())
	}
}

func TestDivider_PRD_DIV23_NoBrandColor(t *testing.T) {
	d := divider.NewDivider()
	primary := render.Hex("#1677ff")
	if d.LineColor() == primary {
		t.Fatal("line must not hardcode brand primary")
	}
	tok := theme.Default.Current()
	// Theme font + border tokens drive the look, not a fixed hex.
	if d.TitleFontSize() <= 0 || d.LineWidth() <= 0 {
		t.Fatal("theme-driven metrics")
	}
	_ = tok
}
