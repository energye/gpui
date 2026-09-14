package progress_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/progress"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseSpec struct {
	CanvasW int     `json:"canvasW"`
	Margin  float64 `json:"margin"`
	Gap     float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadProgressShowcaseSpec(t *testing.T) showcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s showcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadProgressShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("showcase needs a system face for real glyphs: %v", err)
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// TestProgress_Showcase_P0P1 lays §6.8 P0+P1 on one canvas: line normal /
// active sweep / exception / success / hideInfo / small / format, steps,
// gradient, butt cap, inner/outer positions, success split, circle trio /
// micro / mini trio / circle-steps, dashboard default + top gap.
// Three evidences: logic probe (status/size/steps/positions), pixel
// assertions (rail vs fill, steps blocks, gradient ends, text ink),
// golden compare (tolerance from testdata/showcase_spec.json).
// Regenerate with UPDATE_GOLDEN=1.
func TestProgress_Showcase_P0P1(t *testing.T) {
	spec := loadProgressShowcaseSpec(t)
	face := loadProgressShowcaseFace(t)

	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin

	mk := func(p *progress.Progress) *progress.Progress {
		p.SetTextFace(face)
		return p
	}
	tok := theme.Default.Current()
	primary := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: 1}
	_ = primary

	// P0 lines.
	l30 := mk(progress.NewProgress(30))
	lActive := mk(progress.NewProgress(50))
	lActive.SetStatus(progress.StatusActive)
	lActive.Tick(0.42)
	lExc := mk(progress.NewProgress(70))
	lExc.SetStatus(progress.StatusException)
	l100 := mk(progress.NewProgress(100))
	lHide := mk(progress.NewProgress(50))
	lHide.SetShowInfo(false)
	lSmall := mk(progress.NewProgress(40))
	lSmall.SetSize(progress.SizeSmall)
	lFmt := mk(progress.NewProgress(30))
	lFmt.SetFormat(func(percent, _ float64) string { return "Done" })

	// P1 lines.
	lSteps := mk(progress.NewProgress(50))
	lSteps.SetSteps(5)
	lGrad := mk(progress.NewProgress(50))
	lGrad.SetStrokeGradient(
		render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: 1},
		render.RGBA{R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: 1},
		"to right")
	lButt := mk(progress.NewProgress(75))
	lButt.SetStrokeLinecap(progress.LinecapButt)
	lInner := mk(progress.NewProgress(50))
	lInner.SetPercentPosition(progress.AlignCenter, progress.PosInner)
	lOuterStart := mk(progress.NewProgress(60))
	lOuterStart.SetPercentPosition(progress.AlignStart, progress.PosOuter)
	lSucc := mk(progress.NewProgress(60))
	lSucc.SetSuccessPercent(30)

	// P0 circles.
	c75 := mk(progress.NewProgress(75))
	c75.SetType(progress.TypeCircle)
	cExc := mk(progress.NewProgress(70))
	cExc.SetType(progress.TypeCircle)
	cExc.SetStatus(progress.StatusException)
	c100 := mk(progress.NewProgress(100))
	c100.SetType(progress.TypeCircle)
	cMicro := mk(progress.NewProgress(65))
	cMicro.SetType(progress.TypeCircle)
	cMicro.SetSizePx(48.5)
	cMicro.SetStrokeWidth(8)
	cMicro.SetFormat(func(percent, _ float64) string { return "custom" })
	cMini30 := mk(progress.NewProgress(30))
	cMini30.SetType(progress.TypeCircle)
	cMini30.SetSizePx(80)
	cMini60 := mk(progress.NewProgress(60))
	cMini60.SetType(progress.TypeCircle)
	cMini60.SetSizePx(80)
	cMini90 := mk(progress.NewProgress(90))
	cMini90.SetType(progress.TypeCircle)
	cMini90.SetSizePx(80)

	// P1 circle steps.
	cSteps := mk(progress.NewProgress(100))
	cSteps.SetType(progress.TypeCircle)
	cSteps.SetSteps(5)
	cSteps.SetStepGap(7)

	// Dashboards.
	dDef := mk(progress.NewProgress(70))
	dDef.SetType(progress.TypeDashboard)
	dTop := mk(progress.NewProgress(50))
	dTop.SetType(progress.TypeDashboard)
	dTop.SetGapPlacement(progress.GapTop)

	items := []*progress.Progress{
		l30, lActive, lExc, l100, lHide, lSmall, lFmt,
		lSteps, lGrad, lButt, lInner, lOuterStart, lSucc,
		c75, cExc, c100, cMicro, cMini30, cMini60, cMini90, cSteps,
		dDef, dTop,
	}

	// Logic probe before paint.
	if l30.EffectiveStatus() != progress.StatusNormal {
		t.Fatal("l30 status")
	}
	if !lActive.WantsFrame() || lActive.EffectiveStatus() != progress.StatusActive {
		t.Fatal("active sweep probe")
	}
	if l100.EffectiveStatus() != progress.StatusSuccess {
		t.Fatal("100 auto success")
	}
	if lHide.InfoText() != "" || lHide.HasInfoNode() {
		t.Fatal("hideInfo probe")
	}
	if lSteps.Steps() != 5 || lSteps.ActiveSteps() != 3 {
		t.Fatalf("steps active=%d", lSteps.ActiveSteps())
	}
	if !lGrad.HasStrokeGradient() {
		t.Fatal("gradient probe")
	}
	if lButt.EffectiveStrokeLinecap() != progress.LinecapButt {
		t.Fatal("linecap probe")
	}
	if !lInner.IsInnerInfo() || lOuterStart.IsInnerInfo() {
		t.Fatal("percentPosition probe")
	}
	if lSucc.EffectiveSuccessPercent() != 30 {
		t.Fatal("success probe")
	}
	if cMicro.InfoText() != "custom" {
		t.Fatal("micro format probe")
	}
	if cSteps.ActiveSteps() != 5 {
		t.Fatal("circle steps probe")
	}
	if dDef.EffectiveGapDegree() != 75 || dDef.EffectiveGapPlacement() != progress.GapBottom {
		t.Fatal("dashboard defaults probe")
	}
	if dTop.EffectiveGapPlacement() != progress.GapTop {
		t.Fatal("dashboard top probe")
	}

	type placed struct {
		p *progress.Progress
		x float64
		y float64
		w float64
		h float64
	}
	var laid []placed
	y := margin
	for i, p := range items {
		sz := p.Layout(rendering.Loose(rowW, 800))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("row %d layout=%+v", i, sz)
		}
		if p.Node() == nil {
			t.Fatalf("row %d nil node", i)
		}
		laid = append(laid, placed{p: p, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range laid {
		it.p.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel 1: line rail vs fill (row 0, 30% on 160 fallback).
	r0 := laid[0]
	midY := int(r0.y + r0.h/2)
	fillPx := func(x int) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(x, midY).RGBA()
		return r / 257, g / 257, b / 257
	}
	fr, fg, fb := fillPx(int(r0.x + 20))
	if !(fr < 60 && fg > 90 && fg < 150 && fb > 220) {
		t.Fatalf("line fill #%02x%02x%02x want ~primary #1677ff", fr, fg, fb)
	}
	rr, rg, rb := fillPx(int(r0.x + 140))
	if rr < 0xD0 || rg < 0xD0 || rb < 0xD0 {
		t.Fatalf("line rail #%02x%02x%02x want bright rail", rr, rg, rb)
	}
	if fr == rr && fg == rg && fb == rb {
		t.Fatal("rail must differ from fill")
	}

	// Pixel 2: steps blocks (row 7, 5 steps 50% => 3 fill, 2 rail).
	rs := laid[7]
	sMidY := int(rs.y + rs.h/2)
	at := func(x int) (uint32, uint32, uint32) {
		r, g, b, _ := got.At(x, sMidY).RGBA()
		return r / 257, g / 257, b / 257
	}
	// Block pitch 16 (14+2): block0 center ~x+7 fill, block4 center ~x+71 rail.
	b0r, b0g, b0b := at(int(rs.x + 7))
	b4r, b4g, b4b := at(int(rs.x + 71))
	if b0r == b4r && b0g == b4g && b0b == b4b {
		t.Fatalf("steps blocks must differ fill #%02x%02x%02x vs rail #%02x%02x%02x", b0r, b0g, b0b, b4r, b4g, b4b)
	}
	if !(b0r < 60 && b0b > 220) {
		t.Fatalf("steps first block #%02x%02x%02x want primary fill", b0r, b0g, b0b)
	}

	// Pixel 3: gradient ends differ (row 8).
	rgd := laid[8]
	gMidY := int(rgd.y + rgd.h/2)
	g0r, g0g, g0b, _ := got.At(int(rgd.x+8), gMidY).RGBA()
	g1r, g1g, g1b, _ := got.At(int(rgd.x+72), gMidY).RGBA()
	dg := absDiff(g0r, g1r) + absDiff(g0g, g1g) + absDiff(g0b, g1b)
	if dg/257 < 20 {
		t.Fatalf("gradient ends must differ, delta=%d", dg/257)
	}

	// Pixel 4: text zone has real glyph ink, not empty and not solid bar.
	// Row 0 info sits at the right end of the host.
	dark, light := 0, 0
	x0 := int(r0.x + r0.w - 60)
	if x0 < int(r0.x) {
		x0 = int(r0.x)
	}
	x1 := int(r0.x + r0.w - 2)
	for yy := int(r0.y + 2); yy < int(r0.y+r0.h-2); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			r8, g8, b8 := r/257, g/257, b/257
			if r8 < 110 && g8 < 110 && b8 < 110 {
				dark++
			} else if r8 > 200 && g8 > 200 && b8 > 200 {
				light++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("info text dark=%d want >=30 (real glyphs missing?)", dark)
	}
	if light < 30 {
		t.Fatalf("info zone light=%d want >=30 (solid black bar instead of glyphs?)", light)
	}

	path := filepath.Join("testdata", "showcase_progress.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		f.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode showcase: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("showcase bounds %v want %v (regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4p(absDiff(r1, r2), absDiff(g1, g2), absDiff(b1, b2), absDiff(a1, a2))
			if m > hardCap {
				t.Fatalf("showcase pixel (%d,%d) diff %d exceeds hard cap", xx, yy, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > spec.Tolerance.BadFrac {
		t.Fatalf("showcase bad pixels %d/%d exceed %.1f%%", bad, total, spec.Tolerance.BadFrac*100)
	}
}

// TestProgress_InfoNoFaceNoBar guards the no-face path: without SetTextFace
// the info node still lays out non-zero but paints no solid black bar.
func TestProgress_InfoNoFaceNoBar(t *testing.T) {
	p := progress.NewProgress(30)
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%+v", sz)
	}
	if p.Node() == nil {
		t.Fatal("nil node")
	}
	const cw, ch = 220, 40
	dc := render.NewContext(cw, ch)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	offX := (float64(cw) - sz.Width) / 2
	offY := (float64(ch) - sz.Height) / 2
	p.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(offX, offY))
	img := dc.Image()
	// Right-end info zone must not be a solid black bar.
	dark, total := 0, 0
	x0 := int(offX + sz.Width - 60)
	if x0 < 0 {
		x0 = 0
	}
	for yy := 2; yy < ch-2; yy++ {
		for xx := x0; xx < cw-2; xx++ {
			r, g, b, _ := img.At(xx, yy).RGBA()
			total++
			if r/257 < 30 && g/257 < 30 && b/257 < 30 {
				dark++
			}
		}
	}
	if total > 0 && float64(dark)/float64(total) > 0.5 {
		t.Fatalf("no-face info zone looks like a black bar: dark %d/%d", dark, total)
	}
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4p(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
