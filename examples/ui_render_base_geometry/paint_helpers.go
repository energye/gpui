package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Thin wrappers → library draw API (序5). Example must not re-implement geometry.

func fillRect(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	rendering.FillRect(pc, x, y, w, h, r, g, b, a)
}

func fillRoundRect(pc *rendering.PaintContext, x, y, w, h, radius, r, g, b, a float64) {
	rendering.FillRoundRect(pc, x, y, w, h, radius, r, g, b, a)
}

func strokeRect(pc *rendering.PaintContext, x, y, w, h, lw, r, g, b, a float64) {
	rendering.StrokeRect(pc, x, y, w, h, lw, r, g, b, a)
}

func strokeRoundRect(pc *rendering.PaintContext, x, y, w, h, radius, lw, r, g, b, a float64) {
	rendering.StrokeRoundRect(pc, x, y, w, h, radius, lw, r, g, b, a)
}

func strokeLine(pc *rendering.PaintContext, x1, y1, x2, y2, lw, r, g, b, a float64) {
	rendering.StrokeLine(pc, x1, y1, x2, y2, lw, r, g, b, a)
}

func fillCircle(pc *rendering.PaintContext, cx, cy, radius, r, g, b, a float64) {
	rendering.FillCircle(pc, cx, cy, radius, r, g, b, a)
}

func strokeCircle(pc *rendering.PaintContext, cx, cy, radius, lw, r, g, b, a float64) {
	rendering.StrokeCircle(pc, cx, cy, radius, lw, r, g, b, a)
}

func fillLinear2(pc *rendering.PaintContext, x, y, w, h, x0, y0, x1, y1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	rendering.FillLinearGradient(pc, x, y, w, h, x0, y0, x1, y1, r0, g0, b0, a0, r1, g1, b1, a1)
}

func fillRadial2(pc *rendering.PaintContext, x, y, w, h, cx, cy, rad0, rad1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	rendering.FillRadialGradient(pc, x, y, w, h, cx, cy, rad0, rad1, r0, g0, b0, a0, r1, g1, b1, a1)
}

func fillOval(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	rendering.FillOval(pc, x, y, w, h, r, g, b, a)
}

func strokeArc(pc *rendering.PaintContext, cx, cy, radius, a1, a2, lw, r, g, b, a float64) {
	rendering.StrokeArc(pc, cx, cy, radius, a1, a2, lw, r, g, b, a)
}

func pushClipRound(pc *rendering.PaintContext, x, y, w, h, radius float64) {
	if pc != nil {
		pc.PushClipRRect(x, y, w, h, radius)
	}
}

func popClip(pc *rendering.PaintContext) {
	if pc != nil {
		pc.PopClip()
	}
}

func drawText(pc *rendering.PaintContext, s string, x, y, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || s == "" {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawString(s, pc.OriginX+x, pc.OriginY+y)
}

func fillPath(pc *rendering.PaintContext, p *render.Path, r, g, b, a float64) {
	rendering.FillPath(pc, p, r, g, b, a)
}

func strokePath(pc *rendering.PaintContext, p *render.Path, lw, r, g, b, a float64) {
	rendering.StrokePath(pc, p, lw, r, g, b, a)
}
