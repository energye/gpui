package border_beam_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	borderbeam "github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type beamFile struct {
	Duration       float64 `json:"duration"`
	Size           float64 `json:"size"`
	LineWidth      float64 `json:"lineWidth"`
	Outset         float64 `json:"outset"`
	BorderRadius   float64 `json:"borderRadius"`
	FontSize       float64 `json:"fontSize"`
	MaxStopPercent float64 `json:"maxStopPercent"`
}

func loadBeamFile(t *testing.T) beamFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "border_beam.json"))
	if err != nil {
		t.Fatalf("read border_beam.json: %v", err)
	}
	var f beamFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse border_beam.json: %v", err)
	}
	return f
}

func boxChild(w, h float64) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, 0.97, 0.97, 0.97, 1)
}

func paintBeamOK(t *testing.T, b *borderbeam.BorderBeam, w, h int) {
	t.Helper()
	b.Layout(rendering.Loose(float64(w), float64(h)))
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	b.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func TestBorderBeam_PRD_BB01(t *testing.T) {
	b := borderbeam.NewBorderBeam(nil)
	if b == nil || b.Node() == nil {
		t.Fatal("nil beam/node")
	}
	if b.ResolvedDuration() != 6 || b.ResolvedSize() != 100 || b.ResolvedLineWidth() != 1 {
		t.Fatalf("defaults d=%v s=%v w=%v", b.ResolvedDuration(), b.ResolvedSize(), b.ResolvedLineWidth())
	}
	if b.ResolvedOutset() != 0 || b.HasOutset() {
		t.Fatalf("outset=%v has=%v", b.ResolvedOutset(), b.HasOutset())
	}
	sz := b.Layout(rendering.Loose(200, 200))
	if sz.Width != 0 || sz.Height != 0 {
		t.Fatalf("empty layout=%vx%v", sz.Width, sz.Height)
	}
	paintBeamOK(t, b, 64, 64)
}

func TestBorderBeam_PRD_BB02(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	if !b.IsBeamVisible() {
		t.Fatal("default beam should be visible")
	}
	if !b.WantsFrame() {
		t.Fatal("running beam should want frames")
	}
	p0 := b.Phase()
	b.Tick(1.0)
	p1 := b.Phase()
	if p1 == p0 {
		t.Fatalf("phase stuck at %v", p0)
	}
	if math.Abs(p1-1.0/6) > 1e-9 {
		t.Fatalf("phase=%v want 1/6", p1)
	}
}

func TestBorderBeam_PRD_BB03(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	b.SetReduceMotion(true)
	if b.IsBeamVisible() {
		t.Fatal("reduced-motion must hide the beam")
	}
	if b.WantsFrame() {
		t.Fatal("hidden beam must not want frames")
	}
	b.Tick(1.0)
	if b.Phase() != 0 {
		t.Fatalf("phase=%v want frozen 0", b.Phase())
	}
	paintBeamOK(t, b, 160, 120)
}

func TestBorderBeam_PRD_BB04(t *testing.T) {
	fast := borderbeam.NewBorderBeam(boxChild(120, 60))
	slow := borderbeam.NewBorderBeam(boxChild(120, 60))
	fast.SetDuration(3)
	slow.SetDuration(12)
	fast.Tick(1.5)
	slow.Tick(1.5)
	if math.Abs(fast.Phase()-0.5) > 1e-9 || math.Abs(slow.Phase()-0.125) > 1e-9 {
		t.Fatalf("phases %v/%v want 0.5/0.125", fast.Phase(), slow.Phase())
	}
}

func TestBorderBeam_PRD_BB05(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	red := render.RGBA{R: 1, G: 0, B: 0, A: 1}
	b.SetColor(red)
	st := b.ResolvedColorStops()
	if len(st) != 1 || st[0].Color != red {
		t.Fatalf("single stops=%+v", st)
	}
	a := render.Hex("#1677ff")
	c := render.Hex("#52c41a")
	b.SetColorStops(
		borderbeam.BorderBeamColorStop{Color: a, Percent: 0},
		borderbeam.BorderBeamColorStop{Color: c, Percent: 100},
	)
	st = b.ResolvedColorStops()
	if len(st) != 2 || st[0].Color != a || st[1].Color != c {
		t.Fatalf("stops=%+v", st)
	}
	b.ClearColor()
	if len(b.ResolvedColorStops()) != 2 {
		t.Fatal("cleared beam should fall back to the theme gradient")
	}
	paintBeamOK(t, b, 160, 120)
}

func TestBorderBeam_PRD_BB06(t *testing.T) {
	child := boxChild(120, 60)
	b := borderbeam.NewBorderBeam(child)
	if b.Child() == nil {
		t.Fatal("child missing")
	}
	found := false
	for _, ch := range b.Node().Children() {
		if ch == rendering.RenderObject(child) {
			found = true
		}
	}
	if !found {
		t.Fatal("child not in tree")
	}
	sz := b.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-120) > 0.5 || math.Abs(sz.Height-60) > 0.5 {
		t.Fatalf("layout=%vx%v want child box", sz.Width, sz.Height)
	}
	paintBeamOK(t, b, 160, 120)
}

func TestBorderBeam_PRD_BB07(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	if !b.IsBeamVisible() {
		t.Fatal("basic beam should show")
	}
	if len(b.Node().Children()) != 2 {
		t.Fatalf("children=%d want child+beam", len(b.Node().Children()))
	}
	sz := b.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-200) > 0.5 || math.Abs(sz.Height-120) > 0.5 {
		t.Fatalf("layout=%vx%v", sz.Width, sz.Height)
	}
	paintBeamOK(t, b, 240, 160)
}

func TestBorderBeam_PRD_BB08(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	b.SetShowOnHover(true)
	if b.IsBeamVisible() {
		t.Fatal("hover-only beam must hide before hover")
	}
	if b.WantsFrame() {
		t.Fatal("hidden beam must not want frames")
	}
	b.SetHovered(true)
	if !b.IsBeamVisible() {
		t.Fatal("hovered beam should show")
	}
	b.SetHovered(false)
	if b.IsBeamVisible() {
		t.Fatal("unhovered beam should hide again")
	}
	paintBeamOK(t, b, 240, 160)
}

func TestBorderBeam_PRD_BB09(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	b.SetBorderRadius(8)
	if math.Abs(b.ResolvedBorderRadius()-8) > 1e-9 {
		t.Fatalf("radius=%v want 8", b.ResolvedBorderRadius())
	}
	sz := b.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-200) > 0.5 || math.Abs(sz.Height-120) > 0.5 {
		t.Fatalf("layout=%vx%v", sz.Width, sz.Height)
	}
	paintBeamOK(t, b, 240, 160)
}

func TestBorderBeam_PRD_BB10(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	b.SetColorStops(
		borderbeam.BorderBeamColorStop{Color: render.Hex("#ff4d4f"), Percent: 0},
		borderbeam.BorderBeamColorStop{Color: render.Hex("#faad14"), Percent: 50},
		borderbeam.BorderBeamColorStop{Color: render.Hex("#1677ff"), Percent: 100},
	)
	if len(b.ResolvedColorStops()) != 3 {
		t.Fatalf("stops=%+v want 3", b.ResolvedColorStops())
	}
	paintBeamOK(t, b, 240, 160)
}

func TestBorderBeam_PRD_BB11(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	for _, d := range []float64{3, 6, 12} {
		b.SetDuration(d)
		if b.ResolvedDuration() != d {
			t.Fatalf("duration=%v want %v", b.ResolvedDuration(), d)
		}
	}
	b.SetDuration(0)
	if b.ResolvedDuration() != 6 {
		t.Fatalf("bad duration should fall back to 6, got %v", b.ResolvedDuration())
	}
}

func TestBorderBeam_PRD_BB12(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	for _, s := range []float64{100, 56, 160} {
		b.SetSize(s)
		if b.ResolvedSize() != s {
			t.Fatalf("size=%v want %v", b.ResolvedSize(), s)
		}
		paintBeamOK(t, b, 200, 120)
	}
}

func TestBorderBeam_PRD_BB13(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	b.SetLineWidth(2)
	if b.ResolvedLineWidth() != 2 {
		t.Fatalf("lineWidth=%v want 2", b.ResolvedLineWidth())
	}
	b.SetOutset(0)
	if !b.HasOutset() || b.ResolvedOutset() != 0 {
		t.Fatal("explicit outset 0 must stick")
	}
	b.ClearOutset()
	if b.HasOutset() {
		t.Fatal("cleared outset must fall back")
	}
	paintBeamOK(t, b, 160, 120)
}

func TestBorderBeam_PRD_BB14(t *testing.T) {
	f := loadBeamFile(t)
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	eq := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.5 {
			t.Fatalf("%s=%v want %v", name, got, want)
		}
	}
	eq("duration", b.ResolvedDuration(), f.Duration)
	eq("size", b.ResolvedSize(), f.Size)
	eq("lineWidth", b.ResolvedLineWidth(), f.LineWidth)
	eq("radius", b.ResolvedBorderRadius(), f.BorderRadius)
	eq("fontSize", b.ContentFontSize(), f.FontSize)
	eq("maxStopPercent", borderbeam.MaxStopPercent, f.MaxStopPercent)
}

func TestBorderBeam_PRD_BB15(t *testing.T) {
	var toks theme.Tokens = theme.DefaultTokens()
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	b.SetProvider(theme.NewProvider(toks))
	got := b.ResolvedColorStops()
	want := render.RGBA{R: toks.ColorPrimary.R, G: toks.ColorPrimary.G, B: toks.ColorPrimary.B, A: toks.ColorPrimary.A}
	if len(got) == 0 || got[0].Color != want {
		t.Fatalf("default head=%+v want theme primary %+v", got, want)
	}
	other := toks
	other.ColorPrimary = theme.Hex("#ff4d4f")
	b.SetTheme(&other)
	got2 := b.ResolvedColorStops()
	want2 := render.Hex("#ff4d4f")
	if len(got2) == 0 || got2[0].Color != want2 {
		t.Fatalf("themed head=%+v want %+v (must follow Theme, not a fixed color)", got2, want2)
	}
}

// Layout matrix: Exact / Min / Max constraints in one pass.
func TestBorderBeam_LayoutMatrix(t *testing.T) {
	mk := func() *borderbeam.BorderBeam {
		return borderbeam.NewBorderBeam(boxChild(120, 60))
	}
	exact := mk().Layout(rendering.Tight(200, 100))
	if math.Abs(exact.Width-200) > 0.5 || math.Abs(exact.Height-100) > 0.5 {
		t.Fatalf("exact=%vx%v want 200x100", exact.Width, exact.Height)
	}
	min := mk().Layout(rendering.Loose(40, 40))
	if math.Abs(min.Width-40) > 0.5 || math.Abs(min.Height-40) > 0.5 {
		t.Fatalf("min=%vx%v want 40x40", min.Width, min.Height)
	}
	max := mk().Layout(rendering.Loose(400, 400))
	if math.Abs(max.Width-120) > 0.5 || math.Abs(max.Height-60) > 0.5 {
		t.Fatalf("max=%vx%v want 120x60", max.Width, max.Height)
	}
}

// Accessibility: decorative by default, named on demand, never focusable.
// 44px touch target is N/A here: the beam takes no hits, children own input.
func TestBorderBeam_Accessibility(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	if b.Focusable() {
		t.Fatal("beam must never take focus")
	}
	if b.Role() != "presentation" {
		t.Fatalf("role=%q want presentation", b.Role())
	}
	if !b.AriaHidden() || b.AriaLabel() != "" {
		t.Fatal("default beam should be aria-hidden without a name")
	}
	b.SetAriaLabel("promo")
	if b.AriaHidden() || b.AriaLabel() != "promo" || b.Role() == "" {
		t.Fatalf("named beam role=%q hidden=%v", b.Role(), b.AriaHidden())
	}
	b.Layout(rendering.Loose(400, 400))
	hit := b.Node().HitTest(rendering.Point{X: 60, Y: 30})
	if hit == nil {
		t.Fatal("child area should hit")
	}
	kids := b.Node().Children()
	if beamNode := kids[len(kids)-1]; hit == beamNode {
		t.Fatal("beam layer must not steal hits from the child")
	}
}

// Theme variants plus the RTL mirror idea: direction flips, layout holds.
func TestBorderBeam_ThemeAndRTL(t *testing.T) {
	var toks theme.Tokens = theme.DefaultTokens()
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	b.SetProvider(theme.NewProvider(toks))
	dark := toks
	dark.ColorPrimary = theme.Hex("#177ddc")
	dark.ColorBgContainer = theme.Hex("#141414")
	b.SetTheme(&dark)
	head := b.ResolvedColorStops()[0].Color
	if head != (render.RGBA{R: dark.ColorPrimary.R, G: dark.ColorPrimary.G, B: dark.ColorPrimary.B, A: dark.ColorPrimary.A}) {
		t.Fatalf("dark head=%+v", head)
	}
	compact := toks
	compact.FontSize = 12
	b.SetTheme(&compact)
	if b.ContentFontSize() != 12 {
		t.Fatalf("compact font=%v want 12", b.ContentFontSize())
	}
	b.SetTheme(nil)
	fwd := borderbeam.NewBorderBeam(boxChild(120, 60))
	rev := borderbeam.NewBorderBeam(boxChild(120, 60))
	rev.SetRTL(true)
	fwd.Tick(1.0)
	rev.Tick(1.0)
	if math.Abs(rev.EffectivePhase()-(1-fwd.EffectivePhase())) > 1e-9 {
		t.Fatalf("rtl=%v want mirror of %v", rev.EffectivePhase(), fwd.EffectivePhase())
	}
	sz := rev.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-120) > 0.5 {
		t.Fatalf("rtl layout=%vx%v", sz.Width, sz.Height)
	}
	paintBeamOK(t, b, 160, 120)
}
