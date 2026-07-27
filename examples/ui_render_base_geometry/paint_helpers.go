package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// local draw helpers: UI controls how to paint; render executes.
func fillRect(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.OriginX+x, pc.OriginY+y
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

func fillRoundRect(pc *rendering.PaintContext, x, y, w, h, radius, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	ax, ay := pc.OriginX+x, pc.OriginY+y
	pc.DC.SetRGBA(r, g, b, a)
	if radius <= 0 {
		pc.DC.DrawRectangle(ax, ay, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, w, h, radius)
	}
	_ = pc.DC.Fill()
}

func strokeRect(pc *rendering.PaintContext, x, y, w, h, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	ax, ay := pc.OriginX+x, pc.OriginY+y
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Stroke()
}

func strokeRoundRect(pc *rendering.PaintContext, x, y, w, h, radius, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	ax, ay := pc.OriginX+x, pc.OriginY+y
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	if radius <= 0 {
		pc.DC.DrawRectangle(ax, ay, w, h)
	} else {
		pc.DC.DrawRoundedRectangle(ax, ay, w, h, radius)
	}
	_ = pc.DC.Stroke()
}

func strokeLine(pc *rendering.PaintContext, x1, y1, x2, y2, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	pc.DC.DrawLine(pc.OriginX+x1, pc.OriginY+y1, pc.OriginX+x2, pc.OriginY+y2)
	_ = pc.DC.Stroke()
}

func fillCircle(pc *rendering.PaintContext, cx, cy, radius, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawCircle(pc.OriginX+cx, pc.OriginY+cy, radius)
	_ = pc.DC.Fill()
}

func strokeCircle(pc *rendering.PaintContext, cx, cy, radius, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	pc.DC.DrawCircle(pc.OriginX+cx, pc.OriginY+cy, radius)
	_ = pc.DC.Stroke()
}

func fillLinear2(pc *rendering.PaintContext, x, y, w, h, x0, y0, x1, y1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.OriginX+x, pc.OriginY+y
	br := render.NewLinearGradientBrush(pc.OriginX+x0, pc.OriginY+y0, pc.OriginX+x1, pc.OriginY+y1).
		AddColorStop(0, render.RGBA{R: r0, G: g0, B: b0, A: a0}).
		AddColorStop(1, render.RGBA{R: r1, G: g1, B: b1, A: a1})
	pc.DC.SetFillBrush(br)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

func fillRadial2(pc *rendering.PaintContext, x, y, w, h, cx, cy, rad0, rad1, r0, g0, b0, a0, r1, g1, b1, a1 float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := pc.OriginX+x, pc.OriginY+y
	br := render.NewRadialGradientBrush(pc.OriginX+cx, pc.OriginY+cy, rad0, rad1).
		AddColorStop(0, render.RGBA{R: r0, G: g0, B: b0, A: a0}).
		AddColorStop(1, render.RGBA{R: r1, G: g1, B: b1, A: a1})
	pc.DC.SetFillBrush(br)
	pc.DC.DrawRectangle(ax, ay, w, h)
	_ = pc.DC.Fill()
}

func fillOval(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawEllipse(pc.OriginX+x+w/2, pc.OriginY+y+h/2, w/2, h/2)
	_ = pc.DC.Fill()
}

func strokeArc(pc *rendering.PaintContext, cx, cy, radius, a1, a2, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || radius <= 0 {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	pc.DC.DrawArc(pc.OriginX+cx, pc.OriginY+cy, radius, a1, a2)
	_ = pc.DC.Stroke()
}

// pushClipRound / popClip delegate to library UI helpers (not ad-hoc DC.ClipRoundRect).
func pushClipRound(pc *rendering.PaintContext, x, y, w, h, radius float64) {
	if pc == nil {
		return
	}
	pc.PushClipRRect(x, y, w, h, radius)
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
	if pc == nil || pc.DC == nil || p == nil {
		return
	}
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	pc.DC.SetRGBA(r, g, b, a)
	_ = pc.DC.FillPath(p)
	pc.DC.Pop()
}

func strokePath(pc *rendering.PaintContext, p *render.Path, lw, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || p == nil {
		return
	}
	if lw <= 0 {
		lw = 1
	}
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.SetLineWidth(lw)
	_ = pc.DC.StrokePath(p)
	pc.DC.Pop()
}
