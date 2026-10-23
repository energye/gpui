package main

import (
	"fmt"
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

// Palette. Pixel-probe expectations read these same constants, so a palette
// edit can never desync the drawn pixels from the asserted ones.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colBase   = [3]float64{0.30, 0.34, 0.42} // faint "original path" ink
	colTrim   = [3]float64{0.95, 0.55, 0.20} // orange
	colDot    = [3]float64{0.98, 0.85, 0.25} // amber
	colSharp  = [3]float64{0.35, 0.30, 0.45} // sharp shape under the rounded one
	colRound  = [3]float64{0.20, 0.72, 0.68} // teal
	colDash   = [3]float64{0.45, 0.85, 0.40} // green
	colJit    = [3]float64{0.85, 0.35, 0.75} // magenta
	colTitle  = [3]float64{0.80, 0.86, 0.94}
)

// tolColor is the flat-fill tolerance (8/255).
const tolColor = 8.0 / 255

// probeSpec is one end-of-run pixel probe: card index, card-local sample point
// and the expected flat color.
type probeSpec struct {
	card      int
	pt        render.Point
	key, desc string
	want      [3]float64
	tol       float64
}

// ---- geometry (card-local) + probe anchors, built once at init ----

var (
	diamondVerts = [][2]float64{{140, 57}, {225, 132}, {140, 207}, {55, 132}}
	cornerVerts  = [][2]float64{{60, 70}, {220, 70}, {220, 130}, {140, 130}, {140, 190}, {60, 190}}

	trimBase, trimCut           *render.Path
	cornerSharp, cornerRound    *render.Path
	cornerOpen                  *render.Path
	waveBase, waveDash, waveJit *render.Path

	trimLenBase, trimLenCut       float64
	pTrimOn, pTrimOff, pTrimEndPt render.Point

	pCornerCut, pCornerRoundPt, pCornerEdgePt render.Point

	pDashOn, pDashGap               render.Point
	pJitPlus, pJitMinus, pJitMirror render.Point

	probeSpecs []probeSpec
)

func init() {
	// A. Trim: closed diamond, kept arc [0.25, 0.6]; anchors derived from the
	// base arc length (0.425 = middle of the kept range, 0.10 = removed head).
	trimBase = buildClosed(diamondVerts)
	trimLenBase = polyLen(trimBase)
	trimCut = trimBase.Trim(0.25, 0.6)
	trimLenCut = polyLen(trimCut)
	pTrimOn, _ = pointAtArc(trimBase, 0.425*trimLenBase)
	pTrimOff, _ = pointAtArc(trimBase, 0.10*trimLenBase)
	pTrimEndPt = pathEndPoint(trimCut)

	// B. WithCorners: closed L hexagon, r=20. Corner (220,130) has incoming
	// direction (0,1) and outgoing (-1,0); the inward bisector is (-1,-1)/√2.
	cornerSharp = buildClosed(cornerVerts)
	cornerRound = cornerSharp.WithCorners(20)
	cornerOpen = buildOpen(cornerVerts)
	// 3px inside the corner is sliver (sharp only), 12px inside is rounded.
	bx, by := -1/math.Sqrt2, -1/math.Sqrt2
	pCornerCut = render.Pt(220+3*bx, 130+3*by)
	pCornerRoundPt = render.Pt(220+12*bx, 130+12*by)
	pCornerEdgePt = render.Pt(140, 100)

	// C/D. Discrete on a cubic wave. Contract: period = segLen (22), so the
	// drawn pieces cover [0,11), [22,33), ... and the gaps [11,22), [33,44) ...
	waveBase = render.NewPath()
	waveBase.MoveTo(60, 180)
	waveBase.CubicTo(140, 60, 260, 200, 340, 90)
	waveDash = waveBase.Discrete(22, 0)
	waveJit = waveBase.Discrete(22, 7)
	pDashOn, _ = pointAtArc(waveBase, 5.5)   // middle of the first drawn piece
	pDashGap, _ = pointAtArc(waveBase, 16.5) // middle of the first gap
	on1, perp1 := pointAtArc(waveBase, 5.5)
	pJitPlus = render.Pt(on1.X+perp1.X*7, on1.Y+perp1.Y*7)
	pJitMirror = render.Pt(on1.X-perp1.X*7, on1.Y-perp1.Y*7)
	on2, perp2 := pointAtArc(waveBase, 27.5) // second drawn piece sits on the other side
	pJitMinus = render.Pt(on2.X-perp2.X*7, on2.Y-perp2.Y*7)

	probeSpecs = []probeSpec{
		{0, pTrimOn, "A_trim_on", "A kept arc = trim orange", colTrim, tolColor},
		{0, pTrimOff, "A_trim_off", "A removed head keeps base ink only", colBase, tolColor},
		{0, pTrimEndPt, "A_trim_end_dot", "A trim cut end dot = amber", colDot, tolColor},
		{1, pCornerCut, "B_corner_cut", "B corner sliver shows sharp shape", colSharp, tolColor},
		{1, pCornerRoundPt, "B_corner_round", "B 12px inward = rounded fill", colRound, tolColor},
		{1, pCornerEdgePt, "B_edge", "B straight edge = rounded fill", colRound, tolColor},
		{2, pDashOn, "C_dash_on", "C drawn half-period = dash ink", colDash, tolColor},
		{2, pDashGap, "C_dash_gap", "C gap half-period keeps base ink", colBase, tolColor},
		{3, pJitPlus, "D_jit_plus", "D first piece on +7 side = jitter ink", colJit, tolColor},
		{3, pJitMinus, "D_jit_minus", "D next piece on -7 side = jitter ink", colJit, tolColor},
		{3, pJitMirror, "D_jit_mirror", "D mirrored side at same arc = card bg", colCardBG, tolColor},
	}
}

// ---- small geometry helpers (independent of the render implementation) ----

func buildClosed(verts [][2]float64) *render.Path {
	p := render.NewPath()
	if len(verts) == 0 {
		return p
	}
	p.MoveTo(verts[0][0], verts[0][1])
	for _, v := range verts[1:] {
		p.LineTo(v[0], v[1])
	}
	p.Close()
	return p
}

func buildOpen(verts [][2]float64) *render.Path {
	p := render.NewPath()
	if len(verts) == 0 {
		return p
	}
	p.MoveTo(verts[0][0], verts[0][1])
	for _, v := range verts[1:] {
		p.LineTo(v[0], v[1])
	}
	return p
}

func polyLen(p *render.Path) float64 {
	pts := p.Flatten(0.1)
	var l float64
	for i := 1; i < len(pts); i++ {
		l += pts[i-1].Distance(pts[i])
	}
	return l
}

// pointAtArc walks the flattened polyline by arc length and returns the point
// plus the unit perpendicular (used for the ±deviation anchors).
func pointAtArc(p *render.Path, arc float64) (render.Point, render.Point) {
	pts := p.Flatten(0.1)
	rem := arc
	for i := 1; i < len(pts); i++ {
		a, b := pts[i-1], pts[i]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l < 1e-12 {
			continue
		}
		if rem <= l {
			t := rem / l
			return render.Pt(a.X+dx*t, a.Y+dy*t), render.Pt(-dy/l, dx/l)
		}
		rem -= l
	}
	last := pts[len(pts)-1]
	return last, render.Pt(0, 0)
}

func pathEndPoint(p *render.Path) render.Point {
	cs := p.Coords()
	if len(cs) < 2 {
		return render.Pt(0, 0)
	}
	return render.Pt(cs[len(cs)-2], cs[len(cs)-1])
}

func countVerbs(p *render.Path, want render.PathVerb) int {
	n := 0
	for _, v := range p.Verbs() {
		if v == want {
			n++
		}
	}
	return n
}

func sameVertices(p *render.Path, verts [][2]float64) bool {
	cs := p.Coords()
	if len(cs) != len(verts)*2 {
		return false
	}
	for i, v := range verts {
		if cs[2*i] != v[0] || cs[2*i+1] != v[1] {
			return false
		}
	}
	return true
}

func boundsWithin(p *render.Path, minX, minY, maxX, maxY float64) bool {
	cs := p.Coords()
	for i := 0; i+1 < len(cs); i += 2 {
		if cs[i] < minX || cs[i] > maxX || cs[i+1] < minY || cs[i+1] > maxY {
			return false
		}
	}
	return true
}

// edgeSeg is one drawn straight segment of a path.
type edgeSeg struct{ x0, y0, x1, y1 float64 }

func pathEdges(p *render.Path) []edgeSeg {
	var out []edgeSeg
	cs := p.Coords()
	ci := 0
	var cx, cy float64
	has := false
	for _, v := range p.Verbs() {
		switch v {
		case render.MoveTo:
			cx, cy = cs[ci], cs[ci+1]
			ci += 2
			has = true
		case render.LineTo:
			x, y := cs[ci], cs[ci+1]
			ci += 2
			if has {
				out = append(out, edgeSeg{cx, cy, x, y})
			}
			cx, cy = x, y
			has = true
		case render.QuadTo:
			ci += 4
		case render.CubicTo:
			ci += 6
		}
	}
	return out
}

func edgeLen(s edgeSeg) float64 { return math.Hypot(s.x1-s.x0, s.y1-s.y0) }

// runLogicProbes checks the three APIs against their documented contract with
// value-level assertions (same fixture set as render/pathops_subpath_test.go).
// They are pure geometry checks and run once before the window opens.
func runLogicProbes() (probes, failed []string) {
	check := func(key string, pass bool) {
		probes = append(probes, key)
		if !pass {
			failed = append(failed, key)
		}
	}

	// A. Trim
	check("trim_len", trimLenBase > 0 && math.Abs(trimLenCut-0.35*trimLenBase) <= 0.01*trimLenBase)
	{
		edges := pathEdges(trimCut)
		ok := len(edges) > 0
		if ok {
			first, last := edges[0], edges[len(edges)-1]
			ok = math.Hypot(first.x0-225, first.y0-132) < 0.05 && math.Hypot(last.x1-106, last.y1-177) < 0.05
		}
		check("trim_ends", ok)
	}
	check("trim_open", countVerbs(trimCut, render.Close) == 0 && countVerbs(trimCut, render.QuadTo) == 0 && countVerbs(trimCut, render.CubicTo) == 0)
	check("trim_empty_inverted", len(trimBase.Trim(0.7, 0.3).Verbs()) == 0)

	// B. WithCorners
	check("corner_quads", countVerbs(cornerRound, render.QuadTo) == 6 && countVerbs(cornerOpen.WithCorners(20), render.QuadTo) == 4)
	{
		ro := cornerOpen.WithCorners(20)
		cs := ro.Coords()
		check("corner_open_ends", len(cs) >= 4 && cs[0] == 60 && cs[1] == 70 && cs[len(cs)-2] == 60 && cs[len(cs)-1] == 190)
	}
	check("corner_clone_r0", sameVertices(cornerSharp.WithCorners(0), cornerVerts))
	sq := buildClosed([][2]float64{{0, 0}, {20, 0}, {20, 20}, {0, 20}})
	check("corner_clamp", boundsWithin(sq.WithCorners(1000), -1e-6, -1e-6, 20+1e-6, 20+1e-6))

	// C. Discrete: period = segLen (11 drawn + 11 gap), deterministic.
	line := render.NewPath()
	line.MoveTo(0, 0)
	line.LineTo(300, 0)
	dash := line.Discrete(22, 0)
	dEdges := pathEdges(dash)
	dashOK := len(dEdges) >= 12
	if dashOK {
		for i, e := range dEdges {
			if math.Abs(edgeLen(e)-11) > 0.01 || e.y0 != 0 || e.y1 != 0 {
				dashOK = false
				break
			}
			if i > 0 && math.Abs(e.x0-dEdges[i-1].x1-11) > 0.01 {
				dashOK = false
				break
			}
		}
	}
	check("dash_period", dashOK)
	check("dash_deterministic", fmt.Sprint(dash.Coords()) == fmt.Sprint(line.Discrete(22, 0).Coords()))

	// D. Discrete jitter: every piece is a straight ±deviation offset and the
	// sides alternate (the 2026-10-19 fix; before it every piece was -side).
	jit := line.Discrete(22, 7)
	jEdges := pathEdges(jit)
	jitOK := len(jEdges) > 0 && len(jEdges) == len(dEdges)
	pos, neg := 0, 0
	prevPos := false
	for i, e := range jEdges {
		if math.Abs(math.Abs(e.y0)-7) > 1e-9 || math.Abs(e.y0-e.y1) > 1e-9 {
			jitOK = false
			break
		}
		if e.y0 > 0 {
			pos++
		} else {
			neg++
		}
		if i > 0 && (e.y0 > 0) == prevPos {
			jitOK = false
			break
		}
		prevPos = e.y0 > 0
	}
	if pos == 0 || neg == 0 || math.Abs(float64(pos-neg)) > 1 {
		jitOK = false
	}
	check("jit_two_sided", jitOK)
	check("jit_deterministic", fmt.Sprint(jit.Coords()) == fmt.Sprint(line.Discrete(22, 7).Coords()))

	// C/D on the card's own curve: drawn duty stays ~50% and the jitter really
	// uses both sides (this is what the pixel probes sample).
	if baseLen := polyLen(waveBase); baseLen > 0 {
		var drawn float64
		for _, e := range pathEdges(waveDash) {
			drawn += edgeLen(e)
		}
		check("wave_dash_duty", drawn/baseLen > 0.42 && drawn/baseLen < 0.58)
	}
	{
		base := waveBase.Flatten(0.1)
		pos, neg := 0, 0
		for _, e := range pathEdges(waveJit) {
			switch d := signedOffsetOfPiece(e, base); {
			case d > 2:
				pos++
			case d < -2:
				neg++
			}
		}
		check("wave_jit_two_sided", pos >= 4 && neg >= 4)
	}
	return probes, failed
}

// signedOffsetOfPiece returns the signed perpendicular distance of a piece's
// midpoint from the base polyline (sign = which side of the base direction).
func signedOffsetOfPiece(piece edgeSeg, base []render.Point) float64 {
	mx, my := (piece.x0+piece.x1)/2, (piece.y0+piece.y1)/2
	best, bestD := math.Inf(1), 0.0
	for i := 1; i < len(base); i++ {
		a, b := base[i-1], base[i]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l < 1e-12 {
			continue
		}
		t := ((mx-a.X)*dx + (my-a.Y)*dy) / (l * l)
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
		px, py := a.X+dx*t, a.Y+dy*t
		if d := math.Hypot(mx-px, my-py); d < best {
			best = d
			bestD = (dx*(my-a.Y) - dy*(mx-a.X)) / l
		}
	}
	return bestD
}

// ---- scene ----

// buildPathShapeScene fills the shell body with the four demo cards. The card
// background is painted by the card's own OnPaint (first op), so the
// background-reading probes see exactly colCardBG.
func buildPathShapeScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  Trim(0.25,0.6) — arc-length sub-segment", paintTrim)
	card(1, "B  WithCorners(20) — rounded, sharp slivers left", paintCorners)
	card(2, "C  Discrete(22,0) — 11 drawn / 11 gap", paintDiscreteDash)
	card(3, "D  Discrete(22,7) — ±7 jitter, sides alternate", paintDiscreteJitter)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

// paintTrim: the full diamond stays faint, the kept arc [0.25,0.6] is drawn
// bright on top, and the two cut points get amber dots.
func paintTrim(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	// Card-local coords: same convention as ui/rendering (draw.go), so the
	// precomputed paths and the probe anchors share one coordinate system.
	dc.Push()
	dc.Translate(ox, oy)
	dc.SetRGBA(colBase[0], colBase[1], colBase[2], 1)
	dc.SetLineWidth(3)
	dc.DrawPath(trimBase)
	_ = dc.Stroke()

	dc.SetRGBA(colTrim[0], colTrim[1], colTrim[2], 1)
	dc.SetLineWidth(8)
	dc.DrawPath(trimCut)
	_ = dc.Stroke()

	dc.SetRGBA(colDot[0], colDot[1], colDot[2], 1)
	if edges := pathEdges(trimCut); len(edges) > 0 {
		dc.DrawCircle(edges[0].x0, edges[0].y0, 5)
		_ = dc.Fill()
	}
	dc.DrawCircle(pTrimEndPt.X, pTrimEndPt.Y, 5)
	_ = dc.Fill()
	dc.Pop()
}

// paintCorners: the sharp L shape sits underneath, the rounded one is filled on
// top — at every corner only a thin sliver of the sharp shape stays visible.
func paintCorners(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	dc.SetRGBA(colSharp[0], colSharp[1], colSharp[2], 1)
	dc.DrawPath(cornerSharp)
	_ = dc.Fill()

	dc.SetRGBA(colRound[0], colRound[1], colRound[2], 1)
	dc.DrawPath(cornerRound)
	_ = dc.Fill()
	dc.Pop()
}

// paintDiscreteDash: faint base wave plus Discrete(22,0) dashes on the path.
func paintDiscreteDash(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	dc.SetRGBA(colBase[0], colBase[1], colBase[2], 1)
	dc.SetLineWidth(3)
	dc.DrawPath(waveBase)
	_ = dc.Stroke()

	dc.SetRGBA(colDash[0], colDash[1], colDash[2], 1)
	dc.SetLineWidth(5)
	dc.DrawPath(waveDash)
	_ = dc.Stroke()
	dc.Pop()
}

// paintDiscreteJitter: same base wave plus Discrete(22,7) — the pieces zigzag
// to +7 and -7 alternately instead of shifting to one side.
func paintDiscreteJitter(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.Push()
	dc.Translate(ox, oy)
	dc.SetRGBA(colBase[0], colBase[1], colBase[2], 1)
	dc.SetLineWidth(3)
	dc.DrawPath(waveBase)
	_ = dc.Stroke()

	dc.SetRGBA(colJit[0], colJit[1], colJit[2], 1)
	dc.SetLineWidth(5)
	dc.DrawPath(waveJit)
	_ = dc.Stroke()
	dc.Pop()
}
