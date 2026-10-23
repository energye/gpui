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
	// Content region inside each card: the dark panel carries the title on
	// top, the clip-op demo lives in the inset area (ui_render_base_* house
	// style).
	contentX0, contentY0 = 12.0, 36.0
	contentW, contentH   = cardW - 2*contentX0, cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Flat pixel probes read these same constants, so a palette edit can
// never desync the drawn pixels from the asserted ones. Fractional AA-edge
// probe expectations are computed from the card-local hole geometry below.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colFillA  = [3]float64{0.18, 0.78, 0.70} // teal
	colFillB  = [3]float64{0.85, 0.45, 0.20} // orange
	colFillC1 = [3]float64{0.30, 0.55, 0.95} // blue
	colFillC2 = [3]float64{0.80, 0.35, 0.85} // purple
	colFillD  = [3]float64{0.55, 0.85, 0.30} // green
)

// ---- card geometry (card-local) ----

const (
	// A: rect difference holes — one integer, one fractional (AA edges).
	aHole1X, aHole1Y, aHole1W, aHole1H = 40.0, 40.0, 70.0, 70.0
	aHole2X, aHole2Y, aHole2W, aHole2H = 150.25, 40.25, 70.0, 70.0
	// B: replace discards the previous clip.
	bOldW        = 215.0
	bNewX, bNewY = 150.0, 60.0
	bNewW, bNewH = 200.0, 110.0
	// C: nested same-direction rects (outer + inner), NonZero vs EvenOdd.
	cOuterW   = 150.0
	cInnerOff = 40.0
	cInnerW   = 70.0
	cShiftX   = 225.0
	// D: disjoint rects + open (unclosed) triangle.
	dRect1X, dRect1Y = 20.0, 30.0
	dRect2X          = 150.0
	dRectW, dRectH   = 90.0, 90.0
	dTriX0, dTriY0   = 330.0, 120.0
	dTriX1, dTriY1   = 280.0, 170.0
	dTriX2, dTriY2   = 390.0, 170.0
)

// ---- pixel probe spec ----

// band is an independent channel bound on the sampled pixel (0..1 scale), on
// top of the oracle match.
type band struct {
	ch       int
	min, max float64
}

// b8 builds a band in byte units for readability.
func b8(ch int, lo, hi byte) band {
	return band{ch: ch, min: float64(lo) / 255, max: float64(hi) / 255}
}

// probeSpec is one end-of-run pixel probe: card index, card-local sample
// point, the expected-color oracle (device-pixel aware for fractional AA
// edges) and channel bands.
type probeSpec struct {
	key, desc string
	card      int
	pt        render.Point
	want      func(ox, oy, dpr float64, devPX, devPY int) (r, g, b float64)
	bands     []band
}

// overlap1D returns the length of [a0,a1] ∩ [b0,b1].
func overlap1D(a0, a1, b0, b1 float64) float64 {
	lo, hi := math.Max(a0, b0), math.Min(a1, b1)
	if hi <= lo {
		return 0
	}
	return hi - lo
}

// devCov returns the fraction of the device pixel [px,px+1)x[py,py+1) covered
// by the card-local rect (rx,ry,rw,rh). The card origin sits at logical
// (ox, oy) and the snapshot may be at dpr > 1, so both the geometry and the
// sampled pixel are resolved in device space — the AA clip mask is generated
// there, and its analytic coverage must match this model.
func devCov(rx, ry, rw, rh, ox, oy, dpr float64, px, py int) float64 {
	x0, y0 := (ox+rx)*dpr, (oy+ry)*dpr
	x1, y1 := (ox+rx+rw)*dpr, (oy+ry+rh)*dpr
	return overlap1D(float64(px), float64(px+1), x0, x1) *
		overlap1D(float64(py), float64(py+1), y0, y1)
}

// blend returns the color of fill over bg with the given fill coverage cov.
func blend(bg, fill [3]float64, cov float64) (r, g, b float64) {
	f := func(i int) float64 { return bg[i]*(1-cov) + fill[i]*cov }
	return f(0), f(1), f(2)
}

const tolSolid = 8.0 / 255

var probeSpecs []probeSpec

func init() {
	solid := func(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(float64, float64, float64, int, int) (float64, float64, float64) {
			return c[0], c[1], c[2]
		}
	}
	// holeProbe expects fill everywhere except the difference hole: pixel
	// coverage of the fill is 1 − hole coverage (analytic, device-space).
	holeProbe := func(hx, hy, hw, hh float64, fill [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(ox, oy, dpr float64, px, py int) (float64, float64, float64) {
			hole := devCov(hx, hy, hw, hh, ox, oy, dpr, px, py)
			return blend(colCardBG, fill, 1-hole)
		}
	}

	add := func(key, desc string, card int, pt render.Point,
		want func(float64, float64, float64, int, int) (float64, float64, float64), bands ...band) {
		probeSpecs = append(probeSpecs, probeSpec{key, desc, card, pt, want, bands})
	}

	// A: rect difference — integer hole exact, fractional hole AA edges.
	add("a_solid", "A outside holes = teal", 0, render.Pt(15, 170),
		solid(colFillA), b8(1, 190, 210), b8(0, 35, 60), b8(2, 165, 195))
	add("a_hole_int", "A integer hole = card BG", 0, render.Pt(75, 75),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("a_frac_inside", "A fractional hole interior = card BG", 0, render.Pt(200, 105),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("a_frac_left", "A frac hole left edge x=150.25 → 25% fill", 0, render.Pt(150, 105),
		holeProbe(aHole2X, aHole2Y, aHole2W, aHole2H, colFillA), band{ch: 1, min: 0.20, max: 0.40})
	add("a_frac_right", "A frac hole right edge x=220.25 → 75% fill", 0, render.Pt(220, 105),
		holeProbe(aHole2X, aHole2Y, aHole2W, aHole2H, colFillA), band{ch: 1, min: 0.55, max: 0.70})
	add("a_frac_corner", "A frac hole top-left corner → 43.75% fill", 0, render.Pt(150, 40),
		holeProbe(aHole2X, aHole2Y, aHole2W, aHole2H, colFillA), band{ch: 1, min: 0.34, max: 0.50})
	add("a_frac_bottom", "A frac hole bottom-left edge y=110.25 → 81.25% fill", 0, render.Pt(150, 110),
		holeProbe(aHole2X, aHole2Y, aHole2W, aHole2H, colFillA), band{ch: 1, min: 0.58, max: 0.75})

	// B: replace — new region painted, old clip discarded.
	add("b_new", "B replaced region = orange", 1, render.Pt(250, 100),
		solid(colFillB), b8(0, 205, 230), b8(1, 100, 130), b8(2, 40, 65))
	add("b_old", "B old clip region discarded = BG", 1, render.Pt(40, 100),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("b_below", "B below replaced band = BG", 1, render.Pt(250, 178),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("b_above", "B above replaced band = BG", 1, render.Pt(250, 30),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))

	// C: path difference under NonZero vs EvenOdd for nested same-direction
	// rects. NonZero subtracts the solid union (center empty); EvenOdd
	// subtracts only the ring (center stays filled).
	add("c1_center", "C1 NonZero nested diff → center BG", 2, render.Pt(95, 95),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("c1_ring", "C1 NonZero nested diff → ring BG", 2, render.Pt(40, 95),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("c1_out", "C1 outside path = blue", 2, render.Pt(10, 178),
		solid(colFillC1), b8(2, 225, 255), b8(0, 55, 90), b8(1, 115, 155))
	add("c2_center", "C2 EvenOdd nested diff → center filled", 2, render.Pt(320, 95),
		solid(colFillC2), b8(0, 185, 215), b8(2, 200, 230), b8(1, 70, 105))
	add("c2_ring", "C2 EvenOdd nested diff → ring BG", 2, render.Pt(265, 95),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("c2_out", "C2 outside path = purple", 2, render.Pt(400, 178),
		solid(colFillC2), b8(0, 185, 215), b8(2, 200, 230), b8(1, 70, 105))

	// D: path replace — disjoint subpaths + open (unclosed) triangle.
	add("d_r1", "D first subpath filled = green", 3, render.Pt(65, 75),
		solid(colFillD), b8(1, 200, 230), b8(0, 125, 155), b8(2, 60, 95))
	add("d_r2", "D second subpath filled = green", 3, render.Pt(195, 75),
		solid(colFillD), b8(1, 200, 230), b8(0, 125, 155), b8(2, 60, 95))
	add("d_gap", "D gap between subpaths = BG", 3, render.Pt(130, 75),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("d_tri_in", "D open triangle interior = green (implicit close)", 3, render.Pt(335, 155),
		solid(colFillD), b8(1, 200, 230), b8(0, 125, 155), b8(2, 60, 95))
	add("d_tri_out", "D below open triangle = BG", 3, render.Pt(300, 178),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
	add("d_gap2", "D between rect2 and triangle = BG", 3, render.Pt(250, 100),
		solid(colCardBG), b8(0, 20, 42), b8(1, 24, 45), b8(2, 32, 52))
}

// ---- logic probes (value-level, run once at startup) ----

// logic probed via an offscreen CPU context: same public clip-op contract the
// window draws with, checked against analytic coverage (independent math).

func probeCtx() *render.Context {
	dc := render.NewContext(140, 140)
	dc.ClearWithColor(render.White)
	return dc
}

func sampleRGB(dc *render.Context, x, y int) (r, g, b float64) {
	img := dc.Image()
	r32, g32, b32, _ := img.At(x, y).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255
}

func fillBlue(dc *render.Context, w, h float64) {
	dc.SetRGBA(0, 0, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
}

// near3 compares three channels against expected values.
func near3(r, g, b, wr, wg, wb, tol float64) bool {
	return math.Abs(r-wr) <= tol && math.Abs(g-wg) <= tol && math.Abs(b-wb) <= tol
}

// nestedSameDir builds outer + inner same-direction (clockwise) rect subpaths,
// each closed. Path coordinates are canvas-absolute for the 140x140 probe ctx.
func nestedSameDir(dc *render.Context, x0, y0, shift float64) {
	rect := func(x, y, s float64) {
		dc.MoveTo(x, y)
		dc.LineTo(x+s, y)
		dc.LineTo(x+s, y+s)
		dc.LineTo(x, y+s)
		dc.ClosePath()
	}
	rect(x0, y0, cOuterW)
	rect(x0+shift, y0+shift, cInnerW)
}

// runLogicProbes checks ClipRectOp/ClipPathOp value-level semantics on a CPU
// offscreen context. Fractional expectations come from closed-form pixel
// overlap of the analytic difference hole (no render internals involved).
func runLogicProbes() (probes, failed []string) {
	check := func(key string, pass bool) {
		probes = append(probes, key)
		if !pass {
			failed = append(failed, key)
		}
	}
	const tol = 2.0 / 255
	mixW := func(cov float64) float64 { return 1 - cov } // white bg = 1.0 per channel

	// 1) rect difference: integer hole exact.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 140, 140)
		dc.ClipRectOp(30, 30, 40, 40, render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 50, 50)
		r2, g2, b2 := sampleRGB(dc, 10, 10)
		check("rect_diff_hole_int", near3(r1, g1, b1, 1, 1, 1, tol) && near3(r2, g2, b2, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 2) fractional left edge x=30.25: fill coverage 0.25.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 140, 140)
		dc.ClipRectOp(30.25, 30.25, 30, 30, render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r, g, b := sampleRGB(dc, 30, 45)
		check("rect_diff_frac_left25", near3(r, g, b, mixW(0.25), mixW(0.25), 1, tol))
		_ = dc.Close()
	}

	// 3) fractional right edge x=60.25: fill coverage 0.75.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 140, 140)
		dc.ClipRectOp(30.25, 30.25, 30, 30, render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r, g, b := sampleRGB(dc, 60, 45)
		check("rect_diff_frac_right75", near3(r, g, b, mixW(0.75), mixW(0.75), 1, tol))
		_ = dc.Close()
	}

	// 4) corner pixel: both axes fractional → 1 − 0.75·0.75 = 0.4375.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 140, 140)
		dc.ClipRectOp(30.25, 30.25, 30, 30, render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r, g, b := sampleRGB(dc, 30, 30)
		check("rect_diff_frac_corner", near3(r, g, b, mixW(0.4375), mixW(0.4375), 1, tol))
		_ = dc.Close()
	}

	// 5) overlapping difference rects subtract their union.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 140, 140)
		dc.ClipRectOp(20, 20, 40, 40, render.ClipOpDifference)
		dc.ClipRectOp(40, 40, 40, 40, render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 50, 50) // overlap → hole
		r2, g2, b2 := sampleRGB(dc, 35, 35) // hole 1
		r3, g3, b3 := sampleRGB(dc, 70, 70) // hole 2
		r4, g4, b4 := sampleRGB(dc, 30, 70) // x in hole 1 band but y below → filled
		check("rect_diff_union_overlap",
			near3(r1, g1, b1, 1, 1, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol) &&
				near3(r3, g3, b3, 1, 1, 1, tol) && near3(r4, g4, b4, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 6) replace discards the previous clip.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 70, 140)
		dc.ClipRectOp(30, 20, 60, 60, render.ClipOpReplace)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 10, 70) // old clip region → discarded
		r2, g2, b2 := sampleRGB(dc, 50, 40) // replaced region → filled
		r3, g3, b3 := sampleRGB(dc, 100, 40)
		check("rect_replace_discards",
			near3(r1, g1, b1, 1, 1, 1, tol) && near3(r2, g2, b2, 0, 0, 1, tol) &&
				near3(r3, g3, b3, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 7) path difference NonZero: nested same-direction rects subtract solid.
	{
		dc := probeCtx()
		dc.MoveTo(20, 20)
		dc.LineTo(120, 20)
		dc.LineTo(120, 120)
		dc.LineTo(20, 120)
		dc.ClosePath()
		dc.MoveTo(60, 60)
		dc.LineTo(100, 60)
		dc.LineTo(100, 100)
		dc.LineTo(60, 100)
		dc.ClosePath()
		dc.ClipPathOp(render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 80, 80)  // center → subtracted (solid union)
		r2, g2, b2 := sampleRGB(dc, 40, 80)  // ring → subtracted
		r3, g3, b3 := sampleRGB(dc, 80, 130) // below path → filled
		check("path_diff_nonzero_nested",
			near3(r1, g1, b1, 1, 1, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol) &&
				near3(r3, g3, b3, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 8) path difference EvenOdd: same path subtracts only the ring.
	{
		dc := probeCtx()
		dc.SetFillRule(render.FillRuleEvenOdd)
		dc.MoveTo(20, 20)
		dc.LineTo(120, 20)
		dc.LineTo(120, 120)
		dc.LineTo(20, 120)
		dc.ClosePath()
		dc.MoveTo(60, 60)
		dc.LineTo(100, 60)
		dc.LineTo(100, 100)
		dc.LineTo(60, 100)
		dc.ClosePath()
		dc.ClipPathOp(render.ClipOpDifference)
		dc.SetFillRule(render.FillRuleNonZero)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 80, 80)  // center → NOT subtracted under EvenOdd
		r2, g2, b2 := sampleRGB(dc, 40, 80)  // ring → subtracted
		r3, g3, b3 := sampleRGB(dc, 80, 130) // outside → filled
		check("path_diff_evenodd_nested",
			near3(r1, g1, b1, 0, 0, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol) &&
				near3(r3, g3, b3, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 9) path replace with two disjoint rect subpaths.
	{
		dc := probeCtx()
		dc.MoveTo(10, 10)
		dc.LineTo(50, 10)
		dc.LineTo(50, 50)
		dc.LineTo(10, 50)
		dc.ClosePath()
		dc.MoveTo(70, 10)
		dc.LineTo(110, 10)
		dc.LineTo(110, 50)
		dc.LineTo(70, 50)
		dc.ClosePath()
		dc.ClipPathOp(render.ClipOpReplace)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 30, 30)
		r2, g2, b2 := sampleRGB(dc, 90, 30)
		r3, g3, b3 := sampleRGB(dc, 60, 30) // gap between subpaths
		r4, g4, b4 := sampleRGB(dc, 30, 90) // below both
		check("path_replace_disjoint",
			near3(r1, g1, b1, 0, 0, 1, tol) && near3(r2, g2, b2, 0, 0, 1, tol) &&
				near3(r3, g3, b3, 1, 1, 1, tol) && near3(r4, g4, b4, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 10) path replace with an open (unclosed) triangle: implicit close.
	{
		dc := probeCtx()
		dc.MoveTo(70, 20)
		dc.LineTo(20, 110)
		dc.LineTo(120, 110)
		// deliberately no ClosePath
		dc.ClipPathOp(render.ClipOpReplace)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 70, 80)  // inside triangle
		r2, g2, b2 := sampleRGB(dc, 10, 10)  // outside
		r3, g3, b3 := sampleRGB(dc, 70, 125) // below base
		check("path_replace_open_triangle",
			near3(r1, g1, b1, 0, 0, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol) &&
				near3(r3, g3, b3, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 11) disjoint subpaths in a difference: gap between them stays filled.
	{
		dc := probeCtx()
		dc.MoveTo(10, 10)
		dc.LineTo(40, 10)
		dc.LineTo(40, 40)
		dc.LineTo(10, 40)
		dc.ClosePath()
		dc.MoveTo(80, 10)
		dc.LineTo(110, 10)
		dc.LineTo(110, 40)
		dc.LineTo(80, 40)
		dc.ClosePath()
		dc.ClipPathOp(render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 25, 25)
		r2, g2, b2 := sampleRGB(dc, 95, 25)
		r3, g3, b3 := sampleRGB(dc, 60, 25) // gap kept
		r4, g4, b4 := sampleRGB(dc, 60, 90)
		check("path_diff_disjoint_kept",
			near3(r1, g1, b1, 1, 1, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol) &&
				near3(r3, g3, b3, 0, 0, 1, tol) && near3(r4, g4, b4, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 12) ResetClip restores full coverage.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 20, 140)
		dc.ResetClip()
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 100, 70)
		check("reset_clip_restores", near3(r1, g1, b1, 0, 0, 1, tol))
		_ = dc.Close()
	}

	// 13) ClipRectOp default (intersect) intersects both clips.
	{
		dc := probeCtx()
		dc.ClipRect(0, 0, 80, 140)
		dc.ClipRectOp(40, 20, 60, 60, render.ClipOpIntersect)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 50, 40) // in both
		r2, g2, b2 := sampleRGB(dc, 90, 40) // only in the second → outside intersection
		check("clip_rect_intersect",
			near3(r1, g1, b1, 0, 0, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol))
		_ = dc.Close()
	}

	// 14) difference without a base clip: universe (canvas) minus rect.
	{
		dc := probeCtx()
		dc.ClipRectOp(30, 30, 20, 20, render.ClipOpDifference)
		fillBlue(dc, 140, 140)
		r1, g1, b1 := sampleRGB(dc, 10, 10)
		r2, g2, b2 := sampleRGB(dc, 40, 40)
		check("rect_diff_from_no_base",
			near3(r1, g1, b1, 0, 0, 1, tol) && near3(r2, g2, b2, 1, 1, 1, tol))
		_ = dc.Close()
	}

	return probes, failed
}

// ---- scene ----

// buildClipOpScene fills the shell body with the four clip-op demo cards.
func buildClipOpScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  ClipRectOp difference — integer + fractional (AA) holes", paintRectDiffCard)
	card(1, "B  ClipRectOp replace — previous clip discarded", paintRectReplaceCard)
	card(2, "C  ClipPathOp difference × NonZero (left) / EvenOdd (right)", paintPathRuleCard)
	card(3, "D  ClipPathOp replace — disjoint subpaths + open triangle", paintPathReplaceCard)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

// paintRectDiffCard: difference-clip two holes out of an explicit base clip.
// Hole 2 sits on fractional coordinates so its edges carry partial coverage
// (the regression pin for the integer-hard-edge bug).
func paintRectDiffCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	dc.ClipRect(cx, cy, contentW, contentH)
	dc.ClipRectOp(cx+aHole1X, cy+aHole1Y, aHole1W, aHole1H, render.ClipOpDifference)
	dc.ClipRectOp(cx+aHole2X, cy+aHole2Y, aHole2W, aHole2H, render.ClipOpDifference)
	dc.SetRGBA(colFillA[0], colFillA[1], colFillA[2], 1)
	dc.DrawRectangle(cx, cy, contentW, contentH)
	_ = dc.Fill()
}

// paintRectReplaceCard: an old left-half clip must be gone after replace.
func paintRectReplaceCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	dc.ClipRect(cx, cy, bOldW, contentH)
	dc.ClipRectOp(cx+bNewX, cy+bNewY, bNewW, bNewH, render.ClipOpReplace)
	dc.SetRGBA(colFillB[0], colFillB[1], colFillB[2], 1)
	dc.DrawRectangle(cx, cy, contentW, contentH)
	_ = dc.Fill()
}

// paintPathRuleCard: the same nested same-direction rects as a difference
// clip under NonZero (solid subtraction) and EvenOdd (ring subtraction).
func paintPathRuleCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	rect := func(x, y float64) {
		dc.MoveTo(cx+x, cy+y)
		dc.LineTo(cx+x+cOuterW, cy+y)
		dc.LineTo(cx+x+cOuterW, cy+y+cOuterW)
		dc.LineTo(cx+x, cy+y+cOuterW)
		dc.ClosePath()
	}
	inner := func(x, y float64) {
		dc.MoveTo(cx+x+cInnerOff, cy+y+cInnerOff)
		dc.LineTo(cx+x+cInnerOff+cInnerW, cy+y+cInnerOff)
		dc.LineTo(cx+x+cInnerOff+cInnerW, cy+y+cInnerOff+cInnerW)
		dc.LineTo(cx+x+cInnerOff, cy+y+cInnerOff+cInnerW)
		dc.ClosePath()
	}

	// C1 left: NonZero — both subpaths same direction → solid union subtracted.
	dc.ClipRect(cx, cy, cShiftX, contentH)
	rect(20, 20)
	inner(20, 20)
	dc.ClipPathOp(render.ClipOpDifference)
	dc.SetRGBA(colFillC1[0], colFillC1[1], colFillC1[2], 1)
	dc.DrawRectangle(cx, cy, contentW, contentH)
	_ = dc.Fill()

	// C2 right: EvenOdd — inner contour punched out of the subtraction.
	dc.ResetClip()
	dc.SetFillRule(render.FillRuleEvenOdd)
	dc.ClipRect(cx+cShiftX, cy, contentW-cShiftX, contentH)
	rect(20+cShiftX, 20)
	inner(20+cShiftX, 20)
	dc.ClipPathOp(render.ClipOpDifference)
	dc.SetFillRule(render.FillRuleNonZero)
	dc.SetRGBA(colFillC2[0], colFillC2[1], colFillC2[2], 1)
	dc.DrawRectangle(cx, cy, contentW, contentH)
	_ = dc.Fill()
	dc.ResetClip()
}

// paintPathReplaceCard: replace-clip to two disjoint rects plus an open
// triangle subpath (implicit close), all in one path.
func paintPathReplaceCard(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	rect := func(x, y float64) {
		dc.MoveTo(cx+x, cy+y)
		dc.LineTo(cx+x+dRectW, cy+y)
		dc.LineTo(cx+x+dRectW, cy+y+dRectH)
		dc.LineTo(cx+x, cy+y+dRectH)
		dc.ClosePath()
	}
	rect(dRect1X, dRect1Y)
	rect(dRect2X, dRect1Y)
	dc.MoveTo(cx+dTriX0, cy+dTriY0)
	dc.LineTo(cx+dTriX1, cy+dTriY1)
	dc.LineTo(cx+dTriX2, cy+dTriY2)
	// deliberately no ClosePath: fill must implicitly close the subpath
	dc.ClipPathOp(render.ClipOpReplace)
	dc.SetRGBA(colFillD[0], colFillD[1], colFillD[2], 1)
	dc.DrawRectangle(cx, cy, contentW, contentH)
	_ = dc.Fill()
}
