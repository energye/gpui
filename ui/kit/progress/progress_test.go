package progress_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/progress"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type progressCases struct {
	LineHeightMedium  float64 `json:"lineHeightMedium"`
	LineHeightSmall   float64 `json:"lineHeightSmall"`
	CircleEdgeMedium  float64 `json:"circleEdgeMedium"`
	CircleEdgeSmall   float64 `json:"circleEdgeSmall"`
	InfoGap           float64 `json:"infoGap"`
	LineFallbackWidth float64 `json:"lineFallbackWidth"`
	StrokeWidthPct    float64 `json:"strokeWidthPct"`
	GapDegree         float64 `json:"gapDegree"`
	LineInfoFontSize  float64 `json:"lineInfoFontSize"`
	FillCases         []struct {
		Percent   float64 `json:"percent"`
		FillRatio float64 `json:"fillRatio"`
	} `json:"fillCases"`
}

func loadCases(t *testing.T) progressCases {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "progress_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var c progressCases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	if len(c.FillCases) == 0 {
		t.Fatal("empty fill cases")
	}
	return c
}

func paintProgress(p *progress.Progress, w, h int) {
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	p.Layout(rendering.Loose(float64(w), float64(h)))
	p.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func TestProgress_PRD_PRG01(t *testing.T) {
	c := loadCases(t)
	_ = c
	p := progress.NewProgress(30)
	if p.Type() != progress.TypeLine {
		t.Fatalf("type=%v want line", p.Type())
	}
	if p.Size() != progress.SizeMedium {
		t.Fatalf("size=%v want medium", p.Size())
	}
	if !p.ShowInfo() {
		t.Fatal("showInfo default true")
	}
	if p.Status() != progress.StatusAuto {
		t.Fatalf("status=%q want auto", p.Status())
	}
	if p.Percent() != 30 {
		t.Fatalf("percent=%v want 30", p.Percent())
	}
	if p.EffectiveStatus() != progress.StatusNormal {
		t.Fatalf("effective=%v want normal", p.EffectiveStatus())
	}
	if p.Node() == nil {
		t.Fatal("nil node")
	}
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG02(t *testing.T) {
	c := loadCases(t)
	p := progress.NewProgress(50)
	var want float64 = -1
	for _, fc := range c.FillCases {
		if fc.Percent == 50 {
			want = fc.FillRatio
		}
	}
	if want < 0 {
		t.Fatal("50 case missing in testdata")
	}
	if math.Abs(p.FillRatio()-want) > 1e-9 {
		t.Fatalf("fill=%v want %v", p.FillRatio(), want)
	}
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Width <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG03(t *testing.T) {
	p := progress.NewProgress(100)
	if p.EffectiveStatus() != progress.StatusSuccess {
		t.Fatalf("effective=%v want success", p.EffectiveStatus())
	}
	tok := theme.Default.Current()
	got := p.EffectiveFillColor()
	want := render.RGBA{R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: tok.ColorSuccess.A}
	if got != want {
		t.Fatalf("fill=%+v want success %+v", got, want)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG04(t *testing.T) {
	p := progress.NewProgress(40)
	p.SetStatus(progress.StatusException)
	if p.EffectiveStatus() != progress.StatusException {
		t.Fatalf("effective=%v", p.EffectiveStatus())
	}
	tok := theme.Default.Current()
	got := p.EffectiveFillColor()
	want := render.RGBA{R: tok.ColorError.R, G: tok.ColorError.G, B: tok.ColorError.B, A: tok.ColorError.A}
	if got != want {
		t.Fatalf("fill=%+v want error %+v", got, want)
	}
	igot := p.EffectiveInfoColor()
	if igot != want {
		t.Fatalf("info=%+v want error %+v", igot, want)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG05(t *testing.T) {
	c := loadCases(t)
	p := progress.NewProgress(75)
	p.SetType(progress.TypeCircle)
	sz := p.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-c.CircleEdgeMedium) > 0.5 || math.Abs(sz.Height-c.CircleEdgeMedium) > 0.5 {
		t.Fatalf("circle layout=%vx%v want 120", sz.Width, sz.Height)
	}
	if math.Abs(p.CircleSize()-c.CircleEdgeMedium) > 1e-9 {
		t.Fatalf("edge=%v", p.CircleSize())
	}
	paintProgress(p, 160, 160)
}

func TestProgress_PRD_PRG06(t *testing.T) {
	p := progress.NewProgress(50)
	p.SetShowInfo(false)
	if p.InfoText() != "" {
		t.Fatalf("info=%q want empty", p.InfoText())
	}
	if p.HasInfoNode() {
		t.Fatal("info node should detach when showInfo=false")
	}
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Width <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG07(t *testing.T) {
	c := loadCases(t)
	p := progress.NewProgress(50)
	p.SetSize(progress.SizeMedium)
	p.SetType(progress.TypeLine)
	if math.Abs(p.LineHeight()-c.LineHeightMedium) > 1e-9 {
		t.Fatalf("lineH=%v want %v", p.LineHeight(), c.LineHeightMedium)
	}
	p.Layout(rendering.Loose(500, 100))
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG08(t *testing.T) {
	c := loadCases(t)
	p := progress.NewProgress(60)
	p.SetType(progress.TypeCircle)
	if math.Abs(p.CircleSize()-c.CircleEdgeMedium) > 1e-9 {
		t.Fatalf("edge=%v want %v", p.CircleSize(), c.CircleEdgeMedium)
	}
	sz := p.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-c.CircleEdgeMedium) > 0.5 {
		t.Fatalf("layout w=%v", sz.Width)
	}
}

func TestProgress_PRD_PRG10(t *testing.T) {
	line30 := progress.NewProgress(30)
	active50 := progress.NewProgress(50)
	active50.SetStatus(progress.StatusActive)
	exc70 := progress.NewProgress(70)
	exc70.SetStatus(progress.StatusException)
	done100 := progress.NewProgress(100)
	hidden := progress.NewProgress(50)
	hidden.SetShowInfo(false)
	for i, p := range []*progress.Progress{line30, active50, exc70, done100, hidden} {
		sz := p.Layout(rendering.Loose(500, 100))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("case %d layout=%+v", i, sz)
		}
		paintProgress(p, 200, 32)
	}
	if active50.EffectiveStatus() != progress.StatusActive {
		t.Fatal("active status")
	}
	if done100.EffectiveStatus() != progress.StatusSuccess {
		t.Fatal("100 auto success")
	}
	if hidden.InfoText() != "" {
		t.Fatal("hidden info")
	}
	// Active sweep advances via Tick without layout rebuild of root node.
	n0 := active50.Node()
	active50.Tick(0.2)
	if active50.Node() != n0 {
		t.Fatal("tick must not rebuild root")
	}
	if !active50.WantsFrame() {
		t.Fatal("active line should want frames")
	}
}

func TestProgress_PRD_PRG11(t *testing.T) {
	a := progress.NewProgress(75)
	a.SetType(progress.TypeCircle)
	b := progress.NewProgress(70)
	b.SetType(progress.TypeCircle)
	b.SetStatus(progress.StatusException)
	c := progress.NewProgress(100)
	c.SetType(progress.TypeCircle)
	for i, p := range []*progress.Progress{a, b, c} {
		sz := p.Layout(rendering.Loose(400, 400))
		if math.Abs(sz.Width-120) > 0.5 {
			t.Fatalf("case %d w=%v", i, sz.Width)
		}
		paintProgress(p, 160, 160)
	}
	if c.EffectiveStatus() != progress.StatusSuccess {
		t.Fatal("circle 100 auto success")
	}
}

func TestProgress_PRD_PRG12(t *testing.T) {
	c := loadCases(t)
	p := progress.NewProgress(40)
	p.SetSize(progress.SizeSmall)
	p.SetType(progress.TypeLine)
	if math.Abs(p.LineHeight()-c.LineHeightSmall) > 1e-9 {
		t.Fatalf("small lineH=%v want %v", p.LineHeight(), c.LineHeightSmall)
	}
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Height <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG13(t *testing.T) {
	p := progress.NewProgress(65)
	p.SetType(progress.TypeCircle)
	p.SetSizePx(48.5)
	p.SetStrokeWidth(8)
	p.SetFormat(func(percent, _ float64) string { return "custom" })
	if math.Abs(p.CircleSize()-48.5) > 1e-9 {
		t.Fatalf("edge=%v", p.CircleSize())
	}
	if p.InfoText() != "custom" {
		t.Fatalf("info=%q", p.InfoText())
	}
	sz := p.Layout(rendering.Loose(400, 400))
	if math.Abs(sz.Width-48.5) > 0.5 {
		t.Fatalf("layout w=%v", sz.Width)
	}
	paintProgress(p, 120, 120)
}

func TestProgress_PRD_PRG14(t *testing.T) {
	rings := []*progress.Progress{
		progress.NewProgress(30),
		progress.NewProgress(60),
		progress.NewProgress(90),
	}
	for i, p := range rings {
		p.SetType(progress.TypeCircle)
		p.SetSizePx(80)
		sz := p.Layout(rendering.Loose(400, 400))
		if math.Abs(sz.Width-80) > 0.5 || math.Abs(sz.Height-80) > 0.5 {
			t.Fatalf("ring %d %+v", i, sz)
		}
		paintProgress(p, 120, 120)
	}
}

func TestProgress_PRD_PRG15(t *testing.T) {
	p := progress.NewProgress(50)
	p.Layout(rendering.Loose(500, 100))
	n0 := p.Node()
	p.SetPercent(60)
	if p.Node() != n0 {
		t.Fatal("SetPercent must not rebuild root node")
	}
	if math.Abs(p.FillRatio()-0.6) > 1e-9 {
		t.Fatalf("fill=%v", p.FillRatio())
	}
	p.SetPercent(50)
	if p.Node() != n0 {
		t.Fatal("SetPercent back must not rebuild root")
	}
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Width <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG16(t *testing.T) {
	p := progress.NewProgress(30)
	p.SetFormat(func(percent, _ float64) string { return "done" })
	if p.InfoText() != "done" {
		t.Fatalf("info=%q want done", p.InfoText())
	}
	if !strings.Contains(p.AccessibleName(), "done") {
		t.Fatalf("accessible=%q want format", p.AccessibleName())
	}
	p.Layout(rendering.Loose(500, 100))
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG17(t *testing.T) {
	c := loadCases(t)
	p := progress.NewProgress(70)
	p.SetType(progress.TypeDashboard)
	if math.Abs(p.EffectiveGapDegree()-c.GapDegree) > 1e-9 {
		t.Fatalf("gap=%v want %v", p.EffectiveGapDegree(), c.GapDegree)
	}
	if p.EffectiveGapPlacement() != progress.GapBottom {
		t.Fatalf("placement=%v", p.EffectiveGapPlacement())
	}
	p.SetGapDegree(90)
	if math.Abs(p.EffectiveGapDegree()-90) > 1e-9 {
		t.Fatalf("gap custom=%v", p.EffectiveGapDegree())
	}
	sz := p.Layout(rendering.Loose(400, 400))
	if sz.Width <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	paintProgress(p, 160, 160)
	// Gap placement variants build without panic.
	for _, g := range []progress.ProgressGapPlacement{progress.GapTop, progress.GapBottom, progress.GapStart, progress.GapEnd} {
		q := progress.NewProgress(50)
		q.SetType(progress.TypeDashboard)
		q.SetGapPlacement(g)
		q.Layout(rendering.Loose(400, 400))
		paintProgress(q, 160, 160)
	}
}

func TestProgress_PRD_PRG18(t *testing.T) {
	c := loadCases(t)
	line := progress.NewProgress(50)
	line.SetType(progress.TypeLine)
	line.SetSize(progress.SizeMedium)
	if math.Abs(line.LineHeight()-c.LineHeightMedium) > 1e-9 {
		t.Fatalf("medium H=%v", line.LineHeight())
	}
	line.SetSize(progress.SizeSmall)
	if math.Abs(line.LineHeight()-c.LineHeightSmall) > 1e-9 {
		t.Fatalf("small H=%v", line.LineHeight())
	}
	circle := progress.NewProgress(50)
	circle.SetType(progress.TypeCircle)
	circle.SetSize(progress.SizeMedium)
	if math.Abs(circle.CircleSize()-c.CircleEdgeMedium) > 1e-9 {
		t.Fatalf("circle=%v", circle.CircleSize())
	}
	circle.SetSize(progress.SizeSmall)
	if math.Abs(circle.CircleSize()-c.CircleEdgeSmall) > 1e-9 {
		t.Fatalf("circle small=%v", circle.CircleSize())
	}
	if math.Abs(line.InfoGap()-c.InfoGap) > 0.5 {
		t.Fatalf("gap=%v want %v", line.InfoGap(), c.InfoGap)
	}
	// Layout matrix: Exact / Min / Max each run once, sizes asserted.
	exact := line.Layout(rendering.Tight(200, 20))
	if math.Abs(exact.Width-200) > 0.5 || math.Abs(exact.Height-20) > 0.5 {
		t.Fatalf("exact=%+v", exact)
	}
	minC := rendering.Constraints{MinWidth: 100, MaxWidth: 500, MinHeight: 10, MaxHeight: 100}
	minSz := line.Layout(minC)
	if minSz.Width < 100-0.5 || minSz.Width > 500+0.5 {
		t.Fatalf("min-constrained=%+v", minSz)
	}
	maxSz := line.Layout(rendering.Loose(500, 100))
	if maxSz.Width <= 0 || maxSz.Width > 500+0.5 {
		t.Fatalf("max=%+v", maxSz)
	}
	circle.Layout(rendering.Tight(120, 120))
	circle.Layout(minC)
	circle.Layout(rendering.Loose(400, 400))
}

func TestProgress_PRD_PRG19(t *testing.T) {
	tok := theme.Default.Current()
	norm := progress.NewProgress(40)
	got := norm.EffectiveFillColor()
	want := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if got != want {
		t.Fatalf("normal fill=%+v want primary %+v", got, want)
	}
	succ := progress.NewProgress(100)
	gotS := succ.EffectiveFillColor()
	wantS := render.RGBA{R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: tok.ColorSuccess.A}
	if gotS != wantS {
		t.Fatalf("success fill=%+v want %+v", gotS, wantS)
	}
	rail := norm.EffectiveRailColor()
	wantR := render.RGBA{R: tok.ColorFillSecondary.R, G: tok.ColorFillSecondary.G, B: tok.ColorFillSecondary.B, A: tok.ColorFillSecondary.A}
	if rail != wantR {
		t.Fatalf("rail=%+v want fillSecondary %+v", rail, wantR)
	}
	// Theme override flows through (Token assertion, no hardcoded skin).
	alt := tok
	alt.ColorPrimary = theme.Hex("#123456")
	pr := theme.NewProvider(alt)
	themed := progress.NewProgress(20)
	themed.SetProvider(pr)
	gotT := themed.EffectiveFillColor()
	if gotT != (render.RGBA{R: alt.ColorPrimary.R, G: alt.ColorPrimary.G, B: alt.ColorPrimary.B, A: alt.ColorPrimary.A}) {
		t.Fatalf("provider fill=%+v", gotT)
	}
	override := theme.DefaultTokens()
	override.ColorError = theme.Hex("#654321")
	pinned := progress.NewProgress(10)
	pinned.SetStatus(progress.StatusException)
	pinned.SetTheme(&override)
	gotP := pinned.EffectiveFillColor()
	if gotP != (render.RGBA{R: override.ColorError.R, G: override.ColorError.G, B: override.ColorError.B, A: override.ColorError.A}) {
		t.Fatalf("theme pin=%+v", gotP)
	}
}

func TestProgress_PRD_PRG20(t *testing.T) {
	// Progress has no disabled state: never focusable, always paints.
	p := progress.NewProgress(50)
	if p.Focusable() {
		t.Fatal("progress must not be focusable (no disabled/focus path)")
	}
	p.SetPercent(0)
	p.Layout(rendering.Loose(500, 100))
	paintProgress(p, 200, 32)
	p.SetPercent(100)
	p.Layout(rendering.Loose(500, 100))
	paintProgress(p, 200, 32)
}

func TestProgress_PRD_PRG21(t *testing.T) {
	p := progress.NewProgress(42)
	if p.Role() != "progressbar" {
		t.Fatalf("role=%q", p.Role())
	}
	if p.Focusable() {
		t.Fatal("must not take focus")
	}
	if math.Abs(p.AriaValueNow()-42) > 1e-9 {
		t.Fatalf("aria now=%v", p.AriaValueNow())
	}
	if p.AriaValueMin() != 0 || p.AriaValueMax() != 100 {
		t.Fatalf("min/max=%v/%v", p.AriaValueMin(), p.AriaValueMax())
	}
	if !strings.Contains(p.AccessibleName(), "42") {
		t.Fatalf("accessible=%q want percent", p.AccessibleName())
	}
	p.SetAriaLabel("loading files")
	if p.AriaLabel() != "loading files" || p.AccessibleName() != "loading files" {
		t.Fatalf("label=%q accessible=%q", p.AriaLabel(), p.AccessibleName())
	}
	p.SetPercent(80)
	if math.Abs(p.AriaValueNow()-80) > 1e-9 {
		t.Fatalf("aria after=%v", p.AriaValueNow())
	}
}
