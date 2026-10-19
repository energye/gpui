package main

import (
	"sort"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Pixel-probe expectations read these same constants, so a palette
// edit can never desync the drawn pixels from the asserted ones.
var (
	colCardBG  = [3]float64{0.12, 0.135, 0.165}
	colFillA   = [3]float64{0.20, 0.72, 0.68} // teal
	colStrokeA = [3]float64{0.95, 0.55, 0.20} // orange
	colFillB   = [3]float64{0.18, 0.28, 0.62} // deep blue
	colStrokeB = [3]float64{0.85, 0.35, 0.75} // magenta
	colLineC   = [3]float64{0.45, 0.85, 0.40} // green
	colDotC    = [3]float64{0.98, 0.85, 0.25} // amber
	colFillD   = [3]float64{0.85, 0.30, 0.30} // red
	colTitle   = [3]float64{0.80, 0.86, 0.94}
)

// probeSpec is one end-of-run pixel probe: card index, card-local sample
// point and the expected flat color.
type probeSpec struct {
	card      int
	lx, ly    float64
	key, desc string
	want      [3]float64
	tol       float64
}

// tolColor is the flat-fill tolerance (8/255): fills/strokes are opaque and
// the samples sit in the middle of a region, so only AA edges could differ —
// none of these points touch an edge.
const tolColor = 8.0 / 255

var probeSpecs = []probeSpec{
	{0, 140, 170, "A_fill", "A FillPreserve interior = fill teal", colFillA, tolColor},
	{0, 70, 170, "A_stroke", "A outline = stroke orange (path survived fill)", colStrokeA, tolColor},
	{1, 140, 135, "B_fill", "B interior = fill blue (fill drawn after stroke)", colFillB, tolColor},
	{1, 58, 135, "B_ring", "B visible outer stroke half = magenta", colStrokeB, tolColor},
	{2, 120, 80, "C_dot_line", "C dot at LineTo vertex (returned coord)", colDotC, tolColor},
	{2, 290, 190, "C_dot_cubic", "C dot at CubicTo end (returned coord)", colDotC, tolColor},
	{3, 58, 135, "D_absent", "D no outline where Stroke would be = card bg", colCardBG, tolColor},
}

// probeKeys are the logical probes that must all run and pass.
var probeKeys = []string{
	"fill_preserve_keeps",
	"stroke_consumes",
	"stroke_preserve_keeps",
	"stroke_fill_consumes",
	"cp_move_to",
	"cp_line_to",
	"cp_quad_to",
	"cp_cubic_to",
	"cp_close",
	"clear_path_resets",
	"fill_consumes_d",
	"empty_stroke_still_empty",
}

// paintProbe collects the in-paint logical probes. OnPaint runs on the raster
// worker thread; Run returns before the gates read the counters.
type paintProbe struct {
	mu      sync.Mutex
	ran     map[string]bool
	ok      map[string]bool
	cpCalls int
	cpExact int
}

func newPaintProbe() *paintProbe {
	return &paintProbe{ran: map[string]bool{}, ok: map[string]bool{}}
}

func (p *paintProbe) note(key string, pass bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.ran[key] = true
	if pass {
		p.ok[key] = true
	}
	p.mu.Unlock()
}

// checkCurrentPoint asserts GetCurrentPoint() equals the device-space image of
// logical (lx, ly) under the live DC matrix. Exact equality: the API contract
// is the raw stored point, not a rounded one.
func (p *paintProbe) checkCurrentPoint(key string, dc *render.Context, lx, ly float64) {
	if dc == nil {
		return
	}
	want := dc.GetTransform().TransformPoint(render.Pt(lx, ly))
	gx, gy, has := dc.GetCurrentPoint()
	pass := has && gx == want.X && gy == want.Y
	p.mu.Lock()
	p.ran[key] = true
	if pass {
		p.ok[key] = true
	}
	p.cpCalls++
	if pass {
		p.cpExact++
	}
	p.mu.Unlock()
}

// checkCurrentPointExists asserts only presence/absence of a current point.
func (p *paintProbe) checkCurrentPointExists(key string, dc *render.Context, want bool) {
	if dc == nil {
		return
	}
	_, _, has := dc.GetCurrentPoint()
	p.note(key, has == want)
}

func (p *paintProbe) snapshot() (ran, ok map[string]bool, cpCalls, cpExact int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	ran = make(map[string]bool, len(p.ran))
	for k, v := range p.ran {
		ran[k] = v
	}
	ok = make(map[string]bool, len(p.ok))
	for k, v := range p.ok {
		ok[k] = v
	}
	return ran, ok, p.cpCalls, p.cpExact
}

// failingKeys returns the required probe keys that did not run or did not pass.
func failingKeys(ran, ok map[string]bool) []string {
	var bad []string
	for _, k := range probeKeys {
		if !ran[k] || !ok[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

// buildPathLowScene fills the shell body with the four demo cards. The card
// background is painted by the card's own OnPaint (first op), so the D_absent
// probe reads exactly colCardBG regardless of framework background plumbing.
func buildPathLowScene(shell *wrkit.ShellChrome, probe *paintProbe) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  FillPreserve — fill then outline, same path", paintFillPreserve(probe))
	card(1, "B  StrokePreserve — outline first, fill on top", paintStrokePreserve(probe))
	card(2, "C  GetCurrentPoint — dots at returned coords", paintCurrentPoint(probe))
	card(3, "D  Fill() consumes path — Stroke() stays empty", paintFillConsumes(probe))
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

// Card A: fill with FillPreserve, then stroke the very same path. Both the
// fill teal and the stroke orange must be visible at once — that is what
// preserving proves. Fill() (non-preserving) would clear the path and the
// orange outline would be missing.
func paintFillPreserve(probe *paintProbe) func(*rendering.PaintContext, rendering.Size) {
	pent := [][2]float64{{70, 200}, {70, 120}, {140, 60}, {210, 120}, {210, 200}}
	return func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
		cardBG(dc, ox, oy, sz.Width, sz.Height)

		dc.MoveTo(ox+pent[0][0], oy+pent[0][1])
		for _, q := range pent[1:] {
			dc.LineTo(ox+q[0], oy+q[1])
		}
		dc.ClosePath()
		dc.SetRGBA(colFillA[0], colFillA[1], colFillA[2], 1)
		_ = dc.FillPreserve()
		// ClosePath leaves the current point at the subpath start; FillPreserve
		// must leave the whole path (and that point) in place.
		probe.checkCurrentPoint("fill_preserve_keeps", dc, ox+pent[0][0], oy+pent[0][1])
		dc.SetRGBA(colStrokeA[0], colStrokeA[1], colStrokeA[2], 1)
		dc.SetLineWidth(6)
		_ = dc.Stroke()
		probe.checkCurrentPointExists("stroke_consumes", dc, false)
	}
}

// Card B: stroke with StrokePreserve first, then fill on top. The fill covers
// the inner stroke half; the outer half stays visible as a magenta ring.
func paintStrokePreserve(probe *paintProbe) func(*rendering.PaintContext, rendering.Size) {
	const x0, y0, w0, h0, r0 = 60.0, 70.0, 160.0, 130.0, 20.0
	return func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
		cardBG(dc, ox, oy, sz.Width, sz.Height)

		dc.DrawRoundedRectangle(ox+x0, oy+y0, w0, h0, r0)
		dc.SetRGBA(colStrokeB[0], colStrokeB[1], colStrokeB[2], 1)
		dc.SetLineWidth(8)
		_ = dc.StrokePreserve()
		probe.checkCurrentPointExists("stroke_preserve_keeps", dc, true)
		dc.SetRGBA(colFillB[0], colFillB[1], colFillB[2], 1)
		_ = dc.Fill()
		probe.checkCurrentPointExists("stroke_fill_consumes", dc, false)
	}
}

// Card C: after every path op the current point is read back and compared
// exactly (device space) against the point just submitted, then drawn as a dot
// at the coordinates GetCurrentPoint() itself returned. A wrong return value
// moves the dot off the vertex and fails the exact compare.
func paintCurrentPoint(probe *paintProbe) func(*rendering.PaintContext, rendering.Size) {
	return func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
		cardBG(dc, ox, oy, sz.Width, sz.Height)

		var got []render.Point
		track := func(key string, lx, ly float64) {
			probe.checkCurrentPoint(key, dc, ox+lx, oy+ly)
			if gx, gy, has := dc.GetCurrentPoint(); has {
				got = append(got, render.Pt(gx, gy))
			}
		}
		dc.MoveTo(ox+60, oy+200)
		track("cp_move_to", 60, 200)
		dc.LineTo(ox+120, oy+80)
		track("cp_line_to", 120, 80)
		dc.QuadraticTo(ox+160, oy+40, ox+200, oy+90)
		track("cp_quad_to", 200, 90)
		dc.CubicTo(ox+230, oy+130, ox+260, oy+140, ox+290, oy+190)
		track("cp_cubic_to", 290, 190)
		dc.ClosePath()
		track("cp_close", 60, 200)

		dc.SetRGBA(colLineC[0], colLineC[1], colLineC[2], 1)
		dc.SetLineWidth(3)
		_ = dc.Stroke()
		dc.ClearPath()
		probe.checkCurrentPointExists("clear_path_resets", dc, false)

		// Dots at the exact returned coordinates: Push + identity matrix puts
		// the DC in device space (dpr=1 on this rig), so each dot lands on the
		// vertex the API reported rather than on the literal we wrote.
		dc.Push()
		dc.SetTransform(render.Identity())
		dc.SetRGBA(colDotC[0], colDotC[1], colDotC[2], 1)
		for _, g := range got {
			dc.DrawCircle(g.X, g.Y, 5)
			_ = dc.Fill()
		}
		dc.Pop()
	}
}

// Card D: Fill() consumes the path, so the following Stroke() has nothing to
// draw. The D_absent probe reads the card background where a fat orange
// outline would sit — visible absence, not just a counter.
func paintFillConsumes(probe *paintProbe) func(*rendering.PaintContext, rendering.Size) {
	return func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
		cardBG(dc, ox, oy, sz.Width, sz.Height)

		dc.DrawRectangle(ox+60, oy+70, 160, 130)
		dc.SetRGBA(colFillD[0], colFillD[1], colFillD[2], 1)
		_ = dc.Fill()
		probe.checkCurrentPointExists("fill_consumes_d", dc, false)
		dc.SetRGBA(colStrokeA[0], colStrokeA[1], colStrokeA[2], 1)
		dc.SetLineWidth(8)
		_ = dc.Stroke()
		probe.checkCurrentPointExists("empty_stroke_still_empty", dc, false)
	}
}
