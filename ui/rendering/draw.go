package rendering

import (
	"github.com/energye/gpui/render"
)

// Geometry / Paint draw helpers on PaintContext (Flutter Canvas draw* subset).
// Coordinates are logical, origin-relative (Y-down). render executes; this package only orients.

// --- Rect / RRect ---

// FillRect fills an axis-aligned rectangle (FC-DRAW-RECT fill).
func FillRect(pc *PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

// StrokeRect strokes an axis-aligned rectangle (FC-DRAW-RECT stroke / FP-STYLE).
func StrokeRect(pc *PaintContext, x, y, w, h, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Stroke()
}

// FillRoundRect fills a rounded rect with uniform corner radius (FC-DRAW-RRECT fill).
func FillRoundRect(pc *PaintContext, x, y, w, h, radius, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	if radius <= 0 {
		pc.DC.DrawRectangle(ax, ay, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, w, h, radius)
	}
	_ = pc.DC.Fill()
}

// StrokeRoundRect strokes a rounded rect with uniform corner radius (FC-DRAW-RRECT stroke).
func StrokeRoundRect(pc *PaintContext, x, y, w, h, radius, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	if radius < 0 {
		radius = 0
	}
	ax, ay := pc.Abs(x, y)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	if radius <= 0 {
		pc.DC.DrawRectangle(ax, ay, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, w, h, radius)
	}
	_ = pc.DC.Stroke()
}

// --- Line / Circle / Oval / Arc ---

// StrokeLine draws a segment (FC-DRAW-LINE).
func StrokeLine(pc *PaintContext, x1, y1, x2, y2, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	ax1, ay1 := pc.Abs(x1, y1)
	ax2, ay2 := pc.Abs(x2, y2)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	pc.DC.DrawLine(ax1, ay1, ax2, ay2)
	_ = pc.DC.Stroke()
}

// FillCircle fills a circle centered at (cx,cy) (FC-DRAW-CIRCLE).
func FillCircle(pc *PaintContext, cx, cy, radius, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	ax, ay := pc.Abs(cx, cy)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawCircle(ax, ay, radius)
	_ = pc.DC.Fill()
}

// StrokeCircle strokes a circle (FC-DRAW-CIRCLE stroke).
func StrokeCircle(pc *PaintContext, cx, cy, radius, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	ax, ay := pc.Abs(cx, cy)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	pc.DC.DrawCircle(ax, ay, radius)
	_ = pc.DC.Stroke()
}

// FillOval fills an ellipse in the bounding box (x,y,w,h) (FC-DRAW-OVAL).
func FillOval(pc *PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x+w/2, y+h/2)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawEllipse(ax, ay, w/2, h/2)
	_ = pc.DC.Fill()
}

// StrokeOval strokes an ellipse in the bounding box (FC-DRAW-OVAL stroke).
func StrokeOval(pc *PaintContext, x, y, w, h, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	ax, ay := pc.Abs(x+w/2, y+h/2)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	pc.DC.DrawEllipse(ax, ay, w/2, h/2)
	_ = pc.DC.Stroke()
}

// FillArc fills a circular arc sector from angle1 to angle2 (radians, FC-DRAW-ARC).
func FillArc(pc *PaintContext, cx, cy, radius, angle1, angle2, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	ax, ay := pc.Abs(cx, cy)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawArc(ax, ay, radius, angle1, angle2)
	_ = pc.DC.Fill()
}

// StrokeArc strokes a circular arc (FC-DRAW-ARC).
func StrokeArc(pc *PaintContext, cx, cy, radius, angle1, angle2, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	ax, ay := pc.Abs(cx, cy)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	pc.DC.DrawArc(ax, ay, radius, angle1, angle2)
	_ = pc.DC.Stroke()
}

// --- Path ---

// NewPath creates a render.Path for UI CustomPaint-style drawing (FC-DRAW-PATH).
func NewPath() *render.Path { return render.NewPath() }

// PathAddRect appends an axis-aligned rect to p in local coordinates (Path.addRect).
func PathAddRect(p *render.Path, x, y, w, h float64) {
	if p == nil || w <= 0 || h <= 0 {
		return
	}
	p.Rectangle(x, y, w, h)
}

// PathAddRRect appends a uniform rounded rect to p (Path.addRRect subset).
func PathAddRRect(p *render.Path, x, y, w, h, radius float64) {
	if p == nil || w <= 0 || h <= 0 {
		return
	}
	if radius <= 0 {
		p.Rectangle(x, y, w, h)
		return
	}
	p.RoundedRectangle(x, y, w, h, radius)
}

// PathAddOval appends an ellipse in bounding box (x,y,w,h) (Path.addOval).
func PathAddOval(p *render.Path, x, y, w, h float64) {
	if p == nil || w <= 0 || h <= 0 {
		return
	}
	p.Ellipse(x+w/2, y+h/2, w/2, h/2)
}

// SetFillRule sets non-zero / even-odd fill for subsequent FillPath (Path.fillType UI).
func SetFillRule(pc *PaintContext, rule render.FillRule) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.SetFillRule(rule)
}

// FillPath fills a path in local coordinates (origin applied via CTM translate).
func FillPath(pc *PaintContext, p *render.Path, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || p == nil {
		return
	}
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	pc.DC.SetRGBA(r, g, b, a)
	_ = pc.DC.FillPath(p)
	pc.DC.Pop()
}

// StrokePath strokes a path in local coordinates.
func StrokePath(pc *PaintContext, p *render.Path, lineWidth, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || p == nil {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lineWidth)
	_ = pc.DC.StrokePath(p)
	pc.DC.Pop()
}

// --- Points / Vertices / DRRect ---

// DrawPoints draws filled circular points at local (x,y) pairs (FC-DRAW-POINTS).
// pts is [x0,y0, x1,y1, ...]; odd trailing coordinate is ignored. radius is logical px.
func DrawPoints(pc *PaintContext, pts []float64, radius, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || len(pts) < 2 || radius <= 0 {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	for i := 0; i+1 < len(pts); i += 2 {
		ax, ay := pc.Abs(pts[i], pts[i+1])
		pc.DC.DrawPoint(ax, ay, radius)
		_ = pc.DC.Fill()
	}
}

// VertexMode re-exports render triangle modes for DrawVertices.
type VertexMode = render.VertexMode

const (
	VertexModeTriangles   = render.VertexModeTriangles
	VertexModeTriangleFan = render.VertexModeTriangleFan
)

// ColorRGBA is a 0..1 float color (alias of render.RGBA) for per-vertex colors.
type ColorRGBA = render.RGBA

// DrawVertices draws a triangle mesh in local coordinates (FC-DRAW-VERTICES).
// positions use package Point (logical); when len(colors)==len(positions), per-vertex
// colors are used (CPU averages triangle color). mode is Triangles or TriangleFan.
func DrawVertices(pc *PaintContext, positions []Point, colors []ColorRGBA, mode VertexMode) {
	if pc == nil || pc.DC == nil || len(positions) < 3 {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	pos := make([]render.Point, len(positions))
	for i, p := range positions {
		pos[i] = render.Point{X: p.X + ox, Y: p.Y + oy}
	}
	pc.DC.DrawVertices(pos, colors, mode)
}

// FillDRRect fills the ring between an outer and inner rounded rect (FC-DRAW-DRRECT).
// Uses even-odd fill on outer∪inner paths. Inner must lie inside outer; degenerate
// sizes are no-ops. Uniform corner radii only (not per-corner).
func FillDRRect(pc *PaintContext, ox, oy, ow, oh, oRad, ix, iy, iw, ih, iRad, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || ow <= 0 || oh <= 0 || iw <= 0 || ih <= 0 {
		return
	}
	p := NewPath()
	PathAddRRect(p, ox, oy, ow, oh, oRad)
	PathAddRRect(p, ix, iy, iw, ih, iRad)
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	// Even-odd so the inner RRect punches a hole; restore non-zero after.
	pc.DC.SetFillRule(render.FillRuleEvenOdd)
	pc.DC.SetRGBA(r, g, b, a)
	_ = pc.DC.FillPath(p)
	pc.DC.SetFillRule(render.FillRuleNonZero)
	pc.DC.Pop()
}

// --- Stroke style (FP-STROKE-*) ---

// SetStrokeStyle sets line width, cap, and join on the current DC (FP-STROKE-W/CAP/JOIN).
// Pass negative cap/join sentinels to leave unchanged: use render.LineCap / LineJoin constants.
func SetStrokeStyle(pc *PaintContext, lineWidth float64, cap render.LineCap, join render.LineJoin) {
	if pc == nil || pc.DC == nil {
		return
	}
	if lineWidth > 0 {
		pc.DC.SetLineWidth(lineWidth)
	}
	pc.DC.SetLineCap(cap)
	pc.DC.SetLineJoin(join)
}

// --- Gradients (FS-*) ---

// FillLinearGradient fills a rect with a two-stop linear gradient (FS-LINEAR).
// (x0,y0)→(x1,y1) are gradient endpoints in local coordinates.
func FillLinearGradient(pc *PaintContext, x, y, w, h, x0, y0, x1, y1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	ax0, ay0 := pc.Abs(x0, y0)
	ax1, ay1 := pc.Abs(x1, y1)
	br := render.NewLinearGradientBrush(ax0, ay0, ax1, ay1).
		AddColorStop(0, render.RGBA{R: r0, G: g0, B: b0, A: a0}).
		AddColorStop(1, render.RGBA{R: r1, G: g1, B: b1, A: a1})
	pc.DC.SetFillBrush(br)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

// FillRadialGradient fills a rect with a two-stop radial gradient (FS-RADIAL).
// (cx,cy) is center in local coordinates; rad0/rad1 are start/end radii.
func FillRadialGradient(pc *PaintContext, x, y, w, h, cx, cy, rad0, rad1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	acx, acy := pc.Abs(cx, cy)
	br := render.NewRadialGradientBrush(acx, acy, rad0, rad1).
		AddColorStop(0, render.RGBA{R: r0, G: g0, B: b0, A: a0}).
		AddColorStop(1, render.RGBA{R: r1, G: g1, B: b1, A: a1})
	pc.DC.SetFillBrush(br)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

// FillSweepGradient fills a rect with a two-stop sweep (conic) gradient (FS-SWEEP).
// startAngle is radians; center (cx,cy) is local.
func FillSweepGradient(pc *PaintContext, x, y, w, h, cx, cy, startAngle, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.Abs(x, y)
	acx, acy := pc.Abs(cx, cy)
	br := render.NewSweepGradientBrush(acx, acy, startAngle).
		AddColorStop(0, render.RGBA{R: r0, G: g0, B: b0, A: a0}).
		AddColorStop(1, render.RGBA{R: r1, G: g1, B: b1, A: a1})
	pc.DC.SetFillBrush(br)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}
