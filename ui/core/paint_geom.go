package core

import (
	"math"

	"github.com/energye/gpui/render"
)

// ── Shared geometry / stroke quality (kit and primitive compose on these) ──
//
// All UI chrome (Decorated borders, Radio rings, Checkbox marks, Icons) should
// go through these helpers so AA, line caps, and border-box math stay consistent.
// Do not call raw DC.Stroke() from kit without prepareStroke.

// deviceScale returns a positive DPR (defaults to 1).
func (pc *PaintContext) deviceScale() float64 {
	if pc == nil || pc.Scale <= 0 {
		return 1
	}
	return pc.Scale
}

// SnapToDevice snaps a logical coordinate onto a device-pixel boundary.
func (pc *PaintContext) SnapToDevice(v float64) float64 {
	s := pc.deviceScale()
	return math.Round(v*s) / s
}

// SnapHairlineCenter places a 1-device-px stroke on a half-pixel center for
// crisper analytic AA (Skia-style hairline alignment).
func (pc *PaintContext) SnapHairlineCenter(v float64) float64 {
	s := pc.deviceScale()
	return (math.Floor(v*s) + 0.5) / s
}

// prepareStroke applies the standard UI stroke quality stack:
// anti-alias on, round caps/joins, width, color. Call before every Stroke path.
//
// Uses SetStroke so EffectiveLineCap/Join/Width always see Round — SetLineCap
// alone is ignored when paint.Stroke was populated by SetDash (dashed buttons).
func (pc *PaintContext) prepareStroke(lineWidth float64, col render.RGBA) {
	if pc == nil || pc.DC == nil {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	pc.DC.SetAntiAlias(true)
	// Preserve dash if present; replace geometry style only.
	prev := pc.DC.GetStroke()
	st := render.DefaultStroke().
		WithWidth(lineWidth).
		WithCap(render.LineCapRound).
		WithJoin(render.LineJoinRound)
	if prev.Dash != nil {
		st = st.WithDash(prev.Dash)
	}
	pc.DC.SetStroke(st)
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
}

// prepareFill enables AA and sets fill color.
func (pc *PaintContext) prepareFill(col render.RGBA) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.SetAntiAlias(true)
	pc.DC.SetRGBA(col.R, col.G, col.B, col.A)
}

// StrokeLocalPolyline strokes a polyline in coordinates relative to Origin.
// Points are local (x,y). Uses round caps/joins for smooth corners (checks, icons).
// pts is flat: x0,y0, x1,y1, ...
func (pc *PaintContext) StrokeLocalPolyline(pts []float64, lineWidth float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || len(pts) < 4 || col.A <= 0 {
		return
	}
	pc.prepareStroke(lineWidth, col)
	ox, oy := pc.Origin.X, pc.Origin.Y
	pc.DC.MoveTo(ox+pts[0], oy+pts[1])
	for i := 2; i+1 < len(pts); i += 2 {
		pc.DC.LineTo(ox+pts[i], oy+pts[i+1])
	}
	_ = pc.DC.Stroke()
}

// StrokeLocalLine is a 2-point convenience for icons (plus, minus, close).
func (pc *PaintContext) StrokeLocalLine(x0, y0, x1, y1, lineWidth float64, col render.RGBA) {
	pc.StrokeLocalPolyline([]float64{x0, y0, x1, y1}, lineWidth, col)
}

// FillLocalRoundRect fills a rounded rect relative to Origin.
// Radius is clamped to min(w,h)/2 so corners stay circular and match StrokeLocalRoundRect.
func (pc *PaintContext) FillLocalRoundRect(x, y, w, h, radius float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 || col.A <= 0 {
		return
	}
	ax := pc.Origin.X + x
	ay := pc.Origin.Y + y
	r := clampCornerRadius(w, h, radius)
	pc.prepareFill(col)
	if r <= 0 {
		pc.DC.DrawRectangle(ax, ay, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, w, h, r)
	}
	_ = pc.DC.Fill()
}

// StrokeLocalRoundRect strokes a rounded rect relative to Origin using a
// border-box model: the stroke is fully inside [x,y,w,h].
//
// The path is inset by lineWidth/2 and the path radius is reduced by the same
// amount, so the outer edge of the stroke coincides with the outer edge of a
// matching FillLocalRoundRect at the same (x,y,w,h,radius). This keeps fill
// and stroke radii aligned and avoids a "double border" from painting the
// stroke centered on the outer path.
//
// Uses prepareStroke (AA + round join/cap) so button/input corners stay rounded.
func (pc *PaintContext) StrokeLocalRoundRect(x, y, w, h, radius, lineWidth float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 || col.A <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	// Degenerate: stroke thicker than the box — fill the whole box instead of
	// producing an inverted/empty path (common for very small indicators).
	if lineWidth >= w || lineWidth >= h {
		pc.FillLocalRoundRect(x, y, w, h, radius, col)
		return
	}

	outerR := clampCornerRadius(w, h, radius)
	inset := lineWidth / 2
	iw := w - lineWidth
	ih := h - lineWidth
	// Outer stroke radius should equal outerR:
	//   pathRadius + inset = outerR  →  pathRadius = outerR - inset
	pathR := outerR - inset
	if pathR < 0 {
		pathR = 0
	}
	// Keep path radius within the inset rect (defense in depth).
	pathR = clampCornerRadius(iw, ih, pathR)

	ax := pc.Origin.X + x + inset
	ay := pc.Origin.Y + y + inset
	pc.prepareStroke(lineWidth, col)

	if pathR <= 0 {
		pc.DC.DrawRectangle(ax, ay, iw, ih)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, iw, ih, pathR)
	}
	_ = pc.DC.Stroke()
}

// FillLocalCircle fills a circle centered at local (cx, cy) with radius r.
// Prefer this over FillLocalRoundRect for true circles (radio disc, switch thumb).
func (pc *PaintContext) FillLocalCircle(cx, cy, r float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || r <= 0 || col.A <= 0 {
		return
	}
	ax := pc.Origin.X + cx
	ay := pc.Origin.Y + cy
	pc.prepareFill(col)
	pc.DC.DrawCircle(ax, ay, r)
	_ = pc.DC.Fill()
}

// StrokeLocalCircle strokes a circle with border-box semantics: the outer edge of
// the stroke sits at radius r (stroke fully inside the disc of radius r).
//
// For thin UI rings (≤1.5px, Ant radio/checkbox), paints as an EvenOdd filled
// annulus so both edges get proper fill-AA instead of a fragile 1px stroke expand.
func (pc *PaintContext) StrokeLocalCircle(cx, cy, r, lineWidth float64, col render.RGBA) {
	if pc == nil || pc.DC == nil || r <= 0 || col.A <= 0 {
		return
	}
	if lineWidth <= 0 {
		lineWidth = 1
	}
	if lineWidth >= r*2 {
		pc.FillLocalCircle(cx, cy, r, col)
		return
	}
	ax := pc.Origin.X + cx
	ay := pc.Origin.Y + cy

	// Thin ring → dual-disk EvenOdd fill (stable AA at 1x UI sizes).
	if lineWidth <= 1.5 {
		inner := r - lineWidth
		if inner < 0 {
			inner = 0
		}
		pc.prepareFill(col)
		pc.DC.SetFillRule(render.FillRuleEvenOdd)
		pc.DC.DrawCircle(ax, ay, r)
		if inner > 0.01 {
			pc.DC.DrawCircle(ax, ay, inner)
		}
		_ = pc.DC.Fill()
		pc.DC.SetFillRule(render.FillRuleNonZero) // restore default
		return
	}

	// Path radius so outer edge of stroke = r.
	pathR := r - lineWidth/2
	if pathR < 0 {
		pathR = 0
	}
	pc.prepareStroke(lineWidth, col)
	pc.DC.DrawCircle(ax, ay, pathR)
	_ = pc.DC.Stroke()
}

// PaintLocalCheck draws a standard check mark inside a local [0,w]×[0,h] box
// (Ant checkbox proportions). Used by Checkbox and IconCheck.
func (pc *PaintContext) PaintLocalCheck(w, h, lineWidth float64, col render.RGBA) {
	if w <= 0 {
		w = 16
	}
	if h <= 0 {
		h = 16
	}
	if lineWidth <= 0 {
		lineWidth = w * 0.12
		if lineWidth < 1.5 {
			lineWidth = 1.5
		}
		if lineWidth > 2.2 {
			lineWidth = 2.2
		}
	}
	// Relative proportions of a 16px Ant-style check.
	pc.StrokeLocalPolyline([]float64{
		w * 0.22, h * 0.50,
		w * 0.42, h * 0.70,
		w * 0.78, h * 0.30,
	}, lineWidth, col)
}

// PaintLocalClose draws an X mark inside local [0,w]×[0,h].
func (pc *PaintContext) PaintLocalClose(w, h, pad, lineWidth float64, col render.RGBA) {
	if w <= 0 {
		w = 16
	}
	if h <= 0 {
		h = 16
	}
	if pad < 0 {
		pad = w * 0.2
	}
	if lineWidth <= 0 {
		lineWidth = 1.5
	}
	pc.StrokeLocalLine(pad, pad, w-pad, h-pad, lineWidth, col)
	pc.StrokeLocalLine(w-pad, pad, pad, h-pad, lineWidth, col)
}
