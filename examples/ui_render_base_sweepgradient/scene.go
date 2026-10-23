package main

import (
	"math"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Flat-stop pixel probes read these same constants, so a palette edit
// can never desync the drawn pixels from the asserted ones. Interpolated
// samples use the CPU brush oracle (ColorAt at the pixel center) instead.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
)

// ---- card geometry (card-local) ----

const (
	// A: rainbow wheel, full 360°, start angle 0 (east).
	aCX, aCY, aRad = 215.0, 136.0, 88.0
	// B: facade FillSweepGradient on a rect, 2 stops, full turn.
	bX, bY, bW, bH = 24.0, 50.0, 382.0, 164.0
	// C: start-angle (π/2) and negative full turn (-2π) wheels.
	c1CX, c1CY, c2CX, c2CY, cRad = 110.0, 138.0, 320.0, 138.0, 70.0
	// D: partial sweep 0→π/2 under the three extend modes.
	dRad = 60.0
)

const (
	aProbeR = 64.0
	bProbeR = 64.0
	cProbeR = 50.0
	dProbeR = 40.0
)

var (
	bCX, bCY = bX + bW/2, bY + bH/2
	dCenters = [3][2]float64{{85, 140}, {215, 140}, {345, 140}}
	dExtend  = [3]render.ExtendMode{render.ExtendPad, render.ExtendRepeat, render.ExtendReflect}
)

// ---- brush builders (single source for painting AND the CPU oracle) ----

// buildWheel is card A: a 7-stop rainbow over the full turn from east (0).
func buildWheel(ox, oy float64) *render.SweepGradientBrush {
	return render.NewSweepGradientBrush(ox+aCX, oy+aCY, 0).
		AddColorStop(0, render.Red).
		AddColorStop(1.0/6, render.Yellow).
		AddColorStop(2.0/6, render.Green).
		AddColorStop(3.0/6, render.Cyan).
		AddColorStop(4.0/6, render.Blue).
		AddColorStop(5.0/6, render.Magenta).
		AddColorStop(1, render.Red)
}

// buildFacadeMirror has exactly the parameters rendering.FillSweepGradient
// uses for card B (2 stops red→blue, full turn): the facade builds its own
// internal brush, the probe oracle rebuilds the same one.
func buildFacadeMirror(ox, oy float64) *render.SweepGradientBrush {
	return render.NewSweepGradientBrush(ox+bCX, oy+bCY, 0).
		AddColorStop(0, render.RGBA{R: 1, G: 0, B: 0, A: 1}).
		AddColorStop(1, render.RGBA{R: 0, G: 0, B: 1, A: 1})
}

// buildRot is card C1: start angle π/2 (screen down), full turn green→red.
func buildRot(ox, oy float64) *render.SweepGradientBrush {
	return render.NewSweepGradientBrush(ox+c1CX, oy+c1CY, math.Pi/2).
		AddColorStop(0, render.Green).
		AddColorStop(1, render.Red)
}

// buildNeg is card C2: negative full turn (end = -2π) red→blue.
func buildNeg(ox, oy float64) *render.SweepGradientBrush {
	return render.NewSweepGradientBrush(ox+c2CX, oy+c2CY, 0).
		SetEndAngle(-2 * math.Pi).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue)
}

// buildPartial is card D circle i: partial sweep 0→π/2, red→blue, one of
// Pad / Repeat / Reflect. Non-pad modes must fold t in-shader (2026-10-19 fix).
func buildPartial(ox, oy float64, i int) *render.SweepGradientBrush {
	return render.NewSweepGradientBrush(ox+dCenters[i][0], oy+dCenters[i][1], 0).
		SetEndAngle(math.Pi / 2).
		SetExtend(dExtend[i]).
		AddColorStop(0, render.Red).
		AddColorStop(1, render.Blue)
}

// ---- pixel probe spec ----

// band is an independent channel bound on the sampled pixel (0..1 scale), on
// top of the oracle match: catches a wrong-color render even if the oracle
// itself regressed.
type band struct {
	ch       int
	min, max float64
}

// b8 builds a band in byte units for readability.
func b8(ch int, lo, hi byte) band {
	return band{ch: ch, min: float64(lo) / 255, max: float64(hi) / 255}
}

// probeSpec is one end-of-run pixel probe: card index, card-local sample
// point, the oracle brush builder (same params as the paint) and channel bands.
type probeSpec struct {
	key, desc string
	card      int
	pt        render.Point
	brush     func(ox, oy float64) *render.SweepGradientBrush
	bands     []band
}

// tolGrad matches render/p1_capability_matrix_closers_test.go's sweep tile
// tolerance: ramp texel quantization + pixel-center oracle, 12/255.
const tolGrad = 12.0 / 255

// ptAt returns a card-local point at radius r and angle deg (screen degrees,
// y-down: 0° = east, 90° = down).
func ptAt(cx, cy, r, deg float64) render.Point {
	a := deg * math.Pi / 180
	return render.Pt(cx+r*math.Cos(a), cy+r*math.Sin(a))
}

var probeSpecs []probeSpec

func init() {
	add := func(key, desc string, card int, pt render.Point,
		brush func(ox, oy float64) *render.SweepGradientBrush, bands ...band) {
		probeSpecs = append(probeSpecs, probeSpec{key, desc, card, pt, brush, bands})
	}

	// A: six stop directions (60° apart from east) + two in-between angles.
	// Stop colors are exact primaries: bands stay wide enough for the ±1px
	// floor-to-pixel-center shift (~0.9° at probe radius 64).
	add("a_east", "A 0° = stop red", 0, ptAt(aCX, aCY, aProbeR, 0),
		buildWheel, b8(0, 240, 255), b8(1, 0, 30), b8(2, 0, 30))
	add("a_60", "A 60° = stop yellow", 0, ptAt(aCX, aCY, aProbeR, 60),
		buildWheel, b8(0, 240, 255), b8(1, 240, 255), b8(2, 0, 30))
	add("a_120", "A 120° = stop green", 0, ptAt(aCX, aCY, aProbeR, 120),
		buildWheel, b8(1, 240, 255), b8(0, 0, 40), b8(2, 0, 30))
	add("a_180", "A 180° = stop cyan", 0, ptAt(aCX, aCY, aProbeR, 180),
		buildWheel, b8(1, 240, 255), b8(2, 240, 255), b8(0, 0, 30))
	add("a_240", "A 240° = stop blue", 0, ptAt(aCX, aCY, aProbeR, 240),
		buildWheel, b8(2, 240, 255), b8(0, 0, 40), b8(1, 0, 30))
	add("a_300", "A 300° = stop magenta", 0, ptAt(aCX, aCY, aProbeR, 300),
		buildWheel, b8(0, 240, 255), b8(2, 240, 255), b8(1, 0, 30))
	add("a_90", "A 90° = yellow→green linear mid", 0, ptAt(aCX, aCY, aProbeR, 90),
		buildWheel, b8(1, 240, 255), b8(0, 176, 200), b8(2, 0, 30))
	add("a_270", "A 270° = blue→magenta linear mid", 0, ptAt(aCX, aCY, aProbeR, 270),
		buildWheel, b8(2, 240, 255), b8(0, 176, 200), b8(1, 0, 30))

	// B: facade rect fill, 2 stops red→blue. 15° keeps clear of the east seam
	// (hard wrap between t=1 and t=0).
	add("b_90", "B 90° = t 0.25", 1, ptAt(bCX, bCY, bProbeR, 90),
		buildFacadeMirror, b8(0, 207, 243), b8(2, 119, 155), b8(1, 0, 30))
	add("b_180", "B 180° = t 0.5 linear mid", 1, ptAt(bCX, bCY, bProbeR, 180),
		buildFacadeMirror, b8(0, 176, 200), b8(2, 176, 200), b8(1, 0, 30))
	add("b_270", "B 270° = t 0.75", 1, ptAt(bCX, bCY, bProbeR, 270),
		buildFacadeMirror, b8(0, 119, 155), b8(2, 207, 243), b8(1, 0, 30))
	add("b_15", "B 15° = t 0.042 red side", 1, ptAt(bCX, bCY, bProbeR, 15),
		buildFacadeMirror, b8(0, 235, 255), b8(2, 40, 80), b8(1, 0, 30))

	// C: start angle π/2 (down) and negative full turn. C1's start seam runs
	// down the center column, so the green-side sample is shifted 1px west
	// (pixel center then sits on the t>0 side of the seam).
	add("c1_down", "C1 90° = start angle 0 → green", 2, render.Pt(c1CX-1, c1CY+cProbeR),
		buildRot, b8(1, 240, 255), b8(0, 0, 30), b8(2, 0, 30))
	add("c1_east", "C1 0° = t 0.75 (rotated start)", 2, ptAt(c1CX, c1CY, cProbeR, 0),
		buildRot, b8(0, 207, 243), b8(1, 119, 155), b8(2, 0, 30))
	add("c2_down", "C2 90° = t 0.75 (negative turn)", 2, ptAt(c2CX, c2CY, cProbeR, 90),
		buildNeg, b8(0, 119, 155), b8(2, 207, 243), b8(1, 0, 30))
	add("c2_east", "C2 0° = t≈1 → blue near seam", 2, ptAt(c2CX, c2CY, cProbeR, 0),
		buildNeg, b8(2, 235, 255), b8(0, 0, 40), b8(1, 0, 30))

	// D: partial sweep in/out of range per extend mode. The out-of-range
	// probes are the regression pins for the 2026-10-19 in-shader t fold:
	// pre-fix all three rendered pad (blue at 112.5°/157.5°).
	part := func(i int) func(ox, oy float64) *render.SweepGradientBrush {
		return func(ox, oy float64) *render.SweepGradientBrush { return buildPartial(ox, oy, i) }
	}
	add("d_pad_in", "D Pad 45° = t 0.5 in range", 3, ptAt(dCenters[0][0], dCenters[0][1], dProbeR, 45),
		part(0), b8(0, 176, 200), b8(2, 176, 200), b8(1, 0, 30))
	add("d_pad_out", "D Pad 112.5° = clamp to end blue", 3, ptAt(dCenters[0][0], dCenters[0][1], dProbeR, 112.5),
		part(0), b8(2, 240, 255), b8(0, 0, 30), b8(1, 0, 30))
	add("d_repeat_out", "D Repeat 112.5° = frac 0.25", 3, ptAt(dCenters[1][0], dCenters[1][1], dProbeR, 112.5),
		part(1), b8(0, 207, 243), b8(2, 119, 155), b8(1, 0, 30))
	add("d_repeat_out157", "D Repeat 157.5° = frac 0.75", 3, ptAt(dCenters[1][0], dCenters[1][1], dProbeR, 157.5),
		part(1), b8(0, 119, 155), b8(2, 207, 243), b8(1, 0, 30))
	add("d_reflect_out", "D Reflect 112.5° = triangle 0.75", 3, ptAt(dCenters[2][0], dCenters[2][1], dProbeR, 112.5),
		part(2), b8(0, 119, 155), b8(2, 207, 243), b8(1, 0, 30))
	add("d_reflect_out157", "D Reflect 157.5° = triangle 0.25", 3, ptAt(dCenters[2][0], dCenters[2][1], dProbeR, 157.5),
		part(2), b8(0, 207, 243), b8(2, 119, 155), b8(1, 0, 30))
}

// ---- CPU-model independent math (standard sRGB transfer, re-implemented) ----

func srgbToLin(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linToSrgb(l float64) float64 {
	if l <= 0.0031308 {
		return l * 12.92
	}
	return 1.055*math.Pow(l, 1.0/2.4) - 0.055
}

// mixSRGB interpolates two sRGB channel values in linear light (the documented
// gradient model) and returns the sRGB result.
func mixSRGB(c1, c2, f float64) float64 {
	return linToSrgb(srgbToLin(c1)*(1-f) + srgbToLin(c2)*f)
}

func chClose(v, want, tol float64) bool { return math.Abs(v-want) <= tol }

// ---- logic probes (value-level, run once at startup) ----

// runLogicProbes checks the sweep-gradient contract against independent math
// (documented sRGB transfer + linear-light interpolation + the extend folds).
func runLogicProbes() (probes, failed []string) {
	check := func(key string, pass bool) {
		probes = append(probes, key)
		if !pass {
			failed = append(failed, key)
		}
	}
	const tolMid = 2e-3

	rb := func() *render.SweepGradientBrush {
		return render.NewSweepGradientBrush(500, 500, 0).
			AddColorStop(0, render.Red).
			AddColorStop(1, render.Blue)
	}

	// Full turn: t=0 at east returns the first stop exactly.
	{
		c := rb().ColorAt(600, 500)
		check("full_stop0_east", chClose(c.R, 1, 1e-6) && chClose(c.G, 0, 1e-6) && chClose(c.B, 0, 1e-6))
	}

	// Full turn: t=0.5 interpolates in linear light → 0.7354 sRGB, not 0.5.
	{
		c := rb().ColorAt(400, 500)
		mid := mixSRGB(1, 0, 0.5)
		check("full_mid_linear", chClose(c.R, mid, tolMid) && chClose(c.G, 0, 1e-6) && chClose(c.B, mid, tolMid))
	}

	// 7-stop wheel: 60° lands exactly on the yellow stop.
	{
		pt := ptAt(aCX, aCY, 100, 60)
		c := buildWheel(0, 0).ColorAt(pt.X, pt.Y)
		check("full_stop_frac", chClose(c.R, 1, tolMid) && chClose(c.G, 1, tolMid) && chClose(c.B, 0, 1e-6))
	}

	// Center is the documented degenerate case: first stop color.
	{
		c := buildWheel(0, 0).ColorAt(aCX, aCY)
		check("full_center_first", chClose(c.R, 1, 1e-6) && chClose(c.G, 0, 1e-6) && chClose(c.B, 0, 1e-6))
	}

	// Start angle π/2: down = t 0 (green), east = t 0.75.
	{
		down := buildRot(0, 0).ColorAt(c1CX, c1CY+100)
		east := buildRot(0, 0).ColorAt(c1CX+100, c1CY)
		mid := mixSRGB(0, 1, 0.75)
		check("startangle_rotate",
			chClose(down.R, 0, 1e-6) && chClose(down.G, 1, 1e-6) && chClose(down.B, 0, 1e-6) &&
				chClose(east.R, mid, tolMid) && chClose(east.G, mixSRGB(1, 0, 0.75), tolMid) && chClose(east.B, 0, 1e-6))
	}

	// Negative full turn: down = 0.75, up = 0.25, east = t 0 (red).
	{
		br := buildNeg(0, 0)
		down := br.ColorAt(c2CX, c2CY+100)
		up := br.ColorAt(c2CX, c2CY-100)
		east := br.ColorAt(c2CX+100, c2CY)
		check("neg_full_mirror",
			chClose(down.R, mixSRGB(1, 0, 0.75), tolMid) && chClose(down.B, mixSRGB(0, 1, 0.75), tolMid) &&
				chClose(up.R, mixSRGB(1, 0, 0.25), tolMid) && chClose(up.B, mixSRGB(0, 1, 0.25), tolMid) &&
				chClose(east.R, 1, 1e-6) && chClose(east.B, 0, 1e-6))
	}

	// Partial sweep 0→π/2: outside the range the three extend modes must
	// diverge. Raw t at 112.5° = 1.25, at 157.5° = 1.75.
	partAt := func(ext render.ExtendMode, deg float64) render.RGBA {
		br := render.NewSweepGradientBrush(500, 500, 0).
			SetEndAngle(math.Pi / 2).
			SetExtend(ext).
			AddColorStop(0, render.Red).
			AddColorStop(1, render.Blue)
		pt := ptAt(500, 500, 100, deg)
		return br.ColorAt(pt.X, pt.Y)
	}
	{
		c := partAt(render.ExtendPad, 112.5)
		check("partial_pad_clamp", chClose(c.R, 0, 1e-6) && chClose(c.B, 1, 1e-6))
	}
	{
		c := partAt(render.ExtendRepeat, 112.5) // raw 1.25 → frac 0.25 → R keeps 0.75 linear
		check("partial_repeat_frac",
			chClose(c.R, mixSRGB(1, 0, 0.25), tolMid) && chClose(c.B, mixSRGB(0, 1, 0.25), tolMid))
	}
	{
		c := partAt(render.ExtendReflect, 112.5) // 1.25 → odd period → 1-0.25 = 0.75
		check("partial_reflect_tri",
			chClose(c.R, mixSRGB(1, 0, 0.75), tolMid) && chClose(c.B, mixSRGB(0, 1, 0.75), tolMid))
	}
	{
		c := partAt(render.ExtendReflect, 157.5) // 1.75 → odd period → 1-0.75 = 0.25
		check("partial_reflect_157",
			chClose(c.R, mixSRGB(1, 0, 0.25), tolMid) && chClose(c.B, mixSRGB(0, 1, 0.25), tolMid))
	}

	// Full turn is the identity case: every extend mode outputs the same
	// values at the same angles (the ramp fix must not touch it).
	{
		mk := func(ext render.ExtendMode) *render.SweepGradientBrush {
			return render.NewSweepGradientBrush(500, 500, 0).
				SetExtend(ext).
				AddColorStop(0, render.Red).
				AddColorStop(1, render.Blue)
		}
		pad, rep, ref := mk(render.ExtendPad), mk(render.ExtendRepeat), mk(render.ExtendReflect)
		ok := true
		for _, deg := range []float64{15, 60, 90, 120, 180, 225, 270, 330} {
			pt := ptAt(500, 500, 100, deg)
			a, b, c := pad.ColorAt(pt.X, pt.Y), rep.ColorAt(pt.X, pt.Y), ref.ColorAt(pt.X, pt.Y)
			if !chClose(a.R, b.R, 1e-9) || !chClose(a.G, b.G, 1e-9) || !chClose(a.B, b.B, 1e-9) ||
				!chClose(a.R, c.R, 1e-9) || !chClose(a.G, c.G, 1e-9) || !chClose(a.B, c.B, 1e-9) {
				ok = false
				break
			}
		}
		check("extend_full_identity", ok)
	}

	// Unsorted stops are sorted lazily: same values as the sorted brush.
	{
		unsorted := render.NewSweepGradientBrush(500, 500, 0).
			AddColorStop(1, render.Blue).
			AddColorStop(0, render.Red)
		a, b := rb().ColorAt(400, 500), unsorted.ColorAt(400, 500)
		e := unsorted.ColorAt(600, 500)
		check("stops_unsorted",
			chClose(a.R, b.R, 1e-6) && chClose(a.B, b.B, 1e-6) &&
				chClose(e.R, 1, 1e-6) && chClose(e.B, 0, 1e-6))
	}

	// Single stop: constant everywhere.
	{
		br := render.NewSweepGradientBrush(500, 500, 0).AddColorStop(0.5, render.Green)
		ok := true
		for _, pt := range []render.Point{render.Pt(600, 500), render.Pt(500, 600), render.Pt(400, 500)} {
			c := br.ColorAt(pt.X, pt.Y)
			if !chClose(c.R, 0, 1e-6) || !chClose(c.G, 1, 1e-6) || !chClose(c.B, 0, 1e-6) {
				ok = false
			}
		}
		check("single_stop_const", ok)
	}

	// Zero sweep range: degenerate, always the first stop color.
	{
		br := render.NewSweepGradientBrush(500, 500, 1.234).
			SetEndAngle(1.234).
			AddColorStop(0, render.Red).
			AddColorStop(1, render.Blue)
		a, b := br.ColorAt(600, 500), br.ColorAt(500, 600)
		check("zero_sweep", chClose(a.R, 1, 1e-6) && chClose(b.R, 1, 1e-6) && chClose(b.B, 0, 1e-6))
	}

	return probes, failed
}

// ---- scene ----

// buildSweepScene fills the shell body with the four demo cards. The card
// background is painted by the card's own OnPaint (first op), so the
// background-reading probes see exactly colCardBG.
func buildSweepScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  full 360° wheel — 7 stops, start 0°(east)", paintWheelCard)
	card(1, "B  FillSweepGradient facade — 2 stops on a rect", paintFacadeCard)
	card(2, "C  start angle π/2 (left) · negative turn −2π (right)", paintTurnCards)
	card(3, "D  partial 0→π/2 × Pad / Repeat / Reflect", paintExtendCards)

	// D sub-labels sit under each circle (UI labels, placed like the card titles).
	for i, name := range []string{"Pad", "Repeat", "Reflect"} {
		shell.Body.Place(wrkit.Label(name, 11, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[3][0]+dCenters[i][0]-float64(len(name))*3.4, cardAt[3][1]+206)
	}
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

// paintWheelCard: 7-stop rainbow wheel; the six exact stop directions and two
// in-between angles are pixel-probed.
func paintWheelCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	dc.SetFillBrush(buildWheel(ox, oy))
	dc.DrawCircle(ox+aCX, oy+aCY, aRad)
	_ = dc.Fill()
}

// paintFacadeCard: the public ui/rendering facade (FS-SWEEP), draws the rect
// itself; probes rebuild the same 2-stop brush as oracle.
func paintFacadeCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	rendering.FillSweepGradient(pc, bX, bY, bW, bH, bCX, bCY, 0, 1, 0, 0, 1, 0, 0, 1, 1)
}

// paintTurnCards: C1 start angle π/2 (full turn green→red), C2 negative full
// turn −2π (red→blue).
func paintTurnCards(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	dc.SetFillBrush(buildRot(ox, oy))
	dc.DrawCircle(ox+c1CX, oy+c1CY, cRad)
	_ = dc.Fill()
	dc.SetFillBrush(buildNeg(ox, oy))
	dc.DrawCircle(ox+c2CX, oy+c2CY, cRad)
	_ = dc.Fill()
}

// paintExtendCards: same partial sweep (0→π/2) under Pad / Repeat / Reflect.
// Only the out-of-range angular region differs between the three circles.
func paintExtendCards(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	for i := range dCenters {
		dc.SetFillBrush(buildPartial(ox, oy, i))
		dc.DrawCircle(ox+dCenters[i][0], oy+dCenters[i][1], dRad)
		_ = dc.Fill()
	}
}
