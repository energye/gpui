package main

import (
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	contentX0    = 12.0
	contentY0    = 36.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Pixel probes read these same constants, so a palette edit cannot
// desync the drawn pixels from the asserted ones.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colFillA  = [3]float64{0.18, 0.78, 0.70} // teal, difference
	colFillB  = [3]float64{0.30, 0.55, 0.95} // blue, union
	colFillC  = [3]float64{0.80, 0.35, 0.85} // purple, intersect
	colFillD  = [3]float64{0.55, 0.85, 0.30} // green, xor
	colGhost  = [3]float64{0.72, 0.77, 0.86}
)

// ---- card geometry (content-local coordinates) ----

const (
	// A: two copies of the same difference frame + fractional hole. The left
	// copy uses NonZero, the right copy uses EvenOdd; both must show the hole.
	aOuterX, aOuterY, aOuterW, aOuterH = 4.0, 4.0, 180.0, 140.0
	aHoleX, aHoleY, aHoleW, aHoleH     = 39.25, 39.25, 110.0, 70.0
	aRightX                            = 222.0

	// B: two overlapping circles; alpha fill exposes a double-fill seam.
	bAX, bBX, bY, bR = 126.0, 240.0, 96.0, 62.0
	unionAlpha       = 0.72

	// C: diamond ∩ circle.
	cX, cY   = 203.0, 96.0
	cHalfW   = 95.0
	cHalfH   = 58.0
	cCircleR = 68.0

	// D: XOR at far coordinates, then translated into the card for display.
	dAX, dBX, dY, dR = 3050.0, 3160.0, 96.0, 62.0
	dDisplayX        = 203.0
	dTranslateX      = dDisplayX - (dAX+dBX)/2 // -2902
)

var (
	pathsOnce sync.Once

	diffLeft, diffRight *render.Path
	unionAB             *render.Path
	unionSourceA        *render.Path
	unionSourceB        *render.Path
	intersect           *render.Path
	intersectDiamond    *render.Path
	intersectCircle     *render.Path
	xorFar              *render.Path
	xorLocal            *render.Path
	xorSourceLeft       *render.Path
	xorSourceRight      *render.Path
)

func makeRect(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.Rectangle(x, y, w, h)
	return p
}

func makeCircle(cx, cy, r float64) *render.Path {
	p := render.NewPath()
	p.Circle(cx, cy, r)
	return p
}

func makeDiamond(cx, cy, halfW, halfH float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(cx, cy-halfH)
	p.LineTo(cx+halfW, cy)
	p.LineTo(cx, cy+halfH)
	p.LineTo(cx-halfW, cy)
	p.Close()
	return p
}

// contentPath lifts a content-local path into card-local coordinates.
// rendering.FillPath adds the card origin, so the content inset must be part
// of the path itself (same convention as the other ui_render_base_* windows).
func contentPath(p *render.Path) *render.Path {
	if p == nil {
		return nil
	}
	return p.Transform(render.Translate(contentX0, contentY0))
}

func ensurePaths() {
	pathsOnce.Do(func() {
		// A: same local shape, two copies; only the fill rule changes.
		diffLeft = contentPath(makeRect(aOuterX, aOuterY, aOuterW, aOuterH).
			Op(makeRect(aHoleX, aHoleY, aHoleW, aHoleH), render.PathOpDifference))
		rightHoleX := aRightX + (aHoleX - aOuterX)
		diffRight = contentPath(makeRect(aRightX, aOuterY, aOuterW, aOuterH).
			Op(makeRect(rightHoleX, aHoleY, aHoleW, aHoleH), render.PathOpDifference))

		// B: union of two overlapping circles.
		unionSourceA = makeCircle(bAX, bY, bR)
		unionSourceB = makeCircle(bBX, bY, bR)
		unionAB = contentPath(unionSourceA.Op(unionSourceB, render.PathOpUnion))
		unionSourceA = contentPath(unionSourceA)
		unionSourceB = contentPath(unionSourceB)

		// C: diamond ∩ circle.
		intersectDiamond = makeDiamond(cX, cY, cHalfW, cHalfH)
		intersectCircle = makeCircle(cX, cY, cCircleR)
		intersect = contentPath(intersectDiamond.Op(intersectCircle, render.PathOpIntersect))
		intersectDiamond = contentPath(intersectDiamond)
		intersectCircle = contentPath(intersectCircle)

		// D: build at x≈3000 so the operation itself is beyond 2048, then
		// translate the result into the card for display.
		xorSourceLeft = makeCircle(dAX, dY, dR)
		xorSourceRight = makeCircle(dBX, dY, dR)
		xorFar = xorSourceLeft.Op(xorSourceRight, render.PathOpXor)
		tr := render.Translate(dTranslateX+contentX0, contentY0)
		xorLocal = xorFar.Transform(tr)
		xorSourceLeft = xorSourceLeft.Transform(tr)
		xorSourceRight = xorSourceRight.Transform(tr)
	})
}

// buildPathBoolScene fills the shell body with the four boolean-op cards.
func buildPathBoolScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(true)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A  Difference — NonZero (left) / EvenOdd (right)", paintDiffCard)
	card(1, "B  Union — overlap filled once (α=0.72)", paintUnionCard)
	card(2, "C  Intersect — diamond ∩ circle (exact vertices)", paintIntersectCard)
	card(3, "D  Xor at x≈3000 — no 2048 truncation", paintXorCard)
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func drawGhost(pc *rendering.PaintContext, p *render.Path) {
	if p == nil {
		return
	}
	rendering.StrokePath(pc, p, 1.0, colGhost[0], colGhost[1], colGhost[2], 0.85)
}

func paintDiffCard(pc *rendering.PaintContext, sz rendering.Size) {
	ensurePaths()
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	dc.SetFillRule(render.FillRuleNonZero)
	rendering.FillPath(pc, diffLeft, colFillA[0], colFillA[1], colFillA[2], 1)
	dc.SetFillRule(render.FillRuleEvenOdd)
	rendering.FillPath(pc, diffRight, colFillA[0], colFillA[1], colFillA[2], 1)
	dc.SetFillRule(render.FillRuleNonZero)
}

func paintUnionCard(pc *rendering.PaintContext, sz rendering.Size) {
	ensurePaths()
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	rendering.FillPath(pc, unionAB, colFillB[0], colFillB[1], colFillB[2], unionAlpha)
	drawGhost(pc, unionSourceA)
	drawGhost(pc, unionSourceB)
}

func paintIntersectCard(pc *rendering.PaintContext, sz rendering.Size) {
	ensurePaths()
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	rendering.FillPath(pc, intersect, colFillC[0], colFillC[1], colFillC[2], 1)
	drawGhost(pc, intersectDiamond)
	drawGhost(pc, intersectCircle)
}

func paintXorCard(pc *rendering.PaintContext, sz rendering.Size) {
	ensurePaths()
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	rendering.FillPath(pc, xorLocal, colFillD[0], colFillD[1], colFillD[2], 1)
	drawGhost(pc, xorSourceLeft)
	drawGhost(pc, xorSourceRight)
}
