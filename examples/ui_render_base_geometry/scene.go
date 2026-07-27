// Package main — Geometry axis window smoke (ENGINE_UI_RENDER_BASE §2 FC-DRAW-* / §24).
// Example-local only; not a ui/ library package.
package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

type geoScene struct {
	Root *rendering.AbsoluteBox

	// animated panels (RepaintBoundary) — only these should drive dirty locality
	StrokePulse *rendering.RenderBox
	CirclePulse *rendering.RenderBox
	ClipPulse   *rendering.RenderBox

	phase float64
}

func label(s string, size float64, r, g, b float64) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.FontSize = size
	t.R, t.G, t.B, t.A = r, g, b, 1
	return t
}

func panel(w, h float64, paint func(pc *rendering.PaintContext, size rendering.Size)) *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = w, h
	b.SetRepaintBoundary(true)
	b.OnPaint = paint
	return b
}

func buildGeometryScene(winW, winH float64) *geoScene {
	s := &geoScene{}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	root.Place(label("Geometry axis — ENGINE_UI_RENDER_BASE §2 FC-DRAW-* (PaintContext + render)", 13, 0.75, 0.78, 0.85), 12, 8)
	root.Place(label("A: FillRect/RRect/Gradient · Stroke* · Circle · ClipRoundRect | B/D Path/Arc/Oval ui 未暴露", 11, 0.55, 0.58, 0.62), 12, 28)

	const (
		pw, ph = 200.0, 120.0
		gap    = 12.0
		top    = 52.0
	)

	// --- Row 1: FillRect · FillRoundRect · LinearGradient ---
	root.Place(label("FC-DRAW-RECT fill", 11, 0.6, 0.85, 1), 12, top)
	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.16, 0.18, 0.22, 1)
		fillRect(pc, 16, 20, sz.Width-32, sz.Height-40, 0.25, 0.55, 0.90, 1)
		drawText(pc, "FillRect", 24, 58, 0.95, 0.96, 0.98, 1)
	}), 12, top+18)

	root.Place(label("FC-DRAW-RRECT fill", 11, 0.6, 0.85, 1), 12+pw+gap, top)
	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.16, 0.18, 0.22, 1)
		fillRoundRect(pc, 16, 20, sz.Width-32, sz.Height-40, 16, 0.35, 0.75, 0.45, 1)
		drawText(pc, "FillRoundRect", 22, 58, 0.1, 0.12, 0.1, 1)
	}), 12+pw+gap, top+18)

	root.Place(label("FS-LINEAR gradient", 11, 0.6, 0.85, 1), 12+2*(pw+gap), top)
	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.16, 0.18, 0.22, 1)
		fillLinear2(pc, 12, 16, sz.Width-24, sz.Height-32,
			12, 16, sz.Width-12, 16,
			0.15, 0.40, 0.95, 1,
			0.95, 0.30, 0.25, 1,
		)
		drawText(pc, "FillLinearGradient", 18, 58, 0.98, 0.98, 1, 1)
	}), 12+2*(pw+gap), top+18)

	// --- Row 2: StrokeRect · StrokeRoundRect · StrokeLine ---
	row2 := top + 18 + ph + 28
	root.Place(label("FC-DRAW-RECT stroke", 11, 0.6, 0.85, 1), 12, row2)
	s.StrokePulse = panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		// pulse width 1..4
		lw := 1.0 + 3.0*s.phase
		strokeRect(pc, 20, 24, sz.Width-40, sz.Height-48, lw, 0.95, 0.55, 0.20, 1)
		drawText(pc, "StrokeRect", 28, 62, 0.9, 0.9, 0.92, 1)
	})
	root.Place(s.StrokePulse, 12, row2+18)

	root.Place(label("FC-DRAW-RRECT stroke", 11, 0.6, 0.85, 1), 12+pw+gap, row2)
	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		strokeRoundRect(pc, 18, 22, sz.Width-36, sz.Height-44, 14, 2.5, 0.40, 0.85, 0.95, 1)
		drawText(pc, "StrokeRoundRect", 20, 62, 0.9, 0.92, 0.95, 1)
	}), 12+pw+gap, row2+18)

	root.Place(label("FC-DRAW-LINE", 11, 0.6, 0.85, 1), 12+2*(pw+gap), row2)
	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		strokeLine(pc, 16, 24, sz.Width-16, 24, 1, 0.7, 0.7, 0.75, 1)
		strokeLine(pc, 16, sz.Height/2, sz.Width-16, sz.Height/2, 3, 0.95, 0.80, 0.25, 1)
		strokeLine(pc, 16, sz.Height-28, sz.Width-16, 40, 2, 0.55, 0.75, 0.95, 1)
		drawText(pc, "StrokeLine ×3", 24, 70, 0.9, 0.9, 0.92, 1)
	}), 12+2*(pw+gap), row2+18)

	// --- Row 3: FillCircle · StrokeCircle · ClipRoundRect ---
	row3 := row2 + 18 + ph + 28
	root.Place(label("FC-DRAW-CIRCLE fill", 11, 0.6, 0.85, 1), 12, row3)
	s.CirclePulse = panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		cx, cy := sz.Width/2, sz.Height/2-4
		rad := 18 + 10*s.phase
		fillCircle(pc, cx, cy, rad, 0.90, 0.35, 0.55, 1)
		drawText(pc, "FillCircle", 28, 96, 0.9, 0.9, 0.92, 1)
	})
	root.Place(s.CirclePulse, 12, row3+18)

	root.Place(label("FC-DRAW-CIRCLE stroke", 11, 0.6, 0.85, 1), 12+pw+gap, row3)
	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		cx, cy := sz.Width/2, sz.Height/2-4
		strokeCircle(pc, cx, cy, 28, 2.5, 0.45, 0.90, 0.70, 1)
		strokeCircle(pc, cx, cy, 16, 1.5, 0.95, 0.95, 0.5, 1)
		drawText(pc, "StrokeCircle", 24, 96, 0.9, 0.9, 0.92, 1)
	}), 12+pw+gap, row3+18)

	root.Place(label("FC-CLIP-RRECT", 11, 0.6, 0.85, 1), 12+2*(pw+gap), row3)
	s.ClipPulse = panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		// outer guide
		strokeRoundRect(pc, 12, 12, sz.Width-24, sz.Height-24, 20, 1, 0.4, 0.45, 0.5, 1)
		pushClipRound(pc, 12, 12, sz.Width-24, sz.Height-24, 20)
		// content that must be clipped to rrect
		off := s.phase * 30
		fillRect(pc, -10+off, 20, sz.Width, 36, 0.95, 0.55, 0.2, 1)
		fillRect(pc, 20, 50+off*0.3, 80, 40, 0.3, 0.6, 0.95, 1)
		popClip(pc)
		drawText(pc, "PushClipRRect", 16, 100, 0.9, 0.9, 0.92, 1)
	})
	root.Place(s.ClipPulse, 12+2*(pw+gap), row3+18)

	// --- Row 4 (P1): Radial · Path triangle · Oval/Arc ---
	row4 := row3 + 18 + ph + 28
	root.Place(label("FS-RADIAL / PATH / OVAL (P1)", 11, 0.95, 0.75, 0.4), 12, row4)

	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		fillRadial2(pc, 8, 8, sz.Width-16, sz.Height-16,
			sz.Width/2, sz.Height/2, 4, 55,
			1, 0.85, 0.3, 1,
			0.15, 0.2, 0.45, 1,
		)
		drawText(pc, "FillRadialGradient", 16, 100, 0.95, 0.95, 0.98, 1)
	}), 12, row4+18)

	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		tri := render.NewPath()
		tri.MoveTo(sz.Width*0.5, 18)
		tri.LineTo(sz.Width-24, sz.Height-22)
		tri.LineTo(24, sz.Height-22)
		tri.Close()
		fillPath(pc, tri, 0.35, 0.75, 0.95, 1)
		strokePath(pc, tri, 2, 0.95, 0.95, 1, 1)
		drawText(pc, "FillPath/StrokePath", 18, 100, 0.9, 0.9, 0.92, 1)
	}), 12+pw+gap, row4+18)

	root.Place(panel(pw, ph, func(pc *rendering.PaintContext, sz rendering.Size) {
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.15, 0.18, 1)
		fillOval(pc, 20, 28, sz.Width-40, 40, 0.55, 0.35, 0.85, 1)
		strokeArc(pc, sz.Width/2, 70, 28, -1.2, 1.2, 3, 0.95, 0.6, 0.2, 1)
		drawText(pc, "FillOval+StrokeArc", 16, 100, 0.9, 0.9, 0.92, 1)
	}), 12+2*(pw+gap), row4+18)

	// Footnote
	note := label(fmt.Sprintf(
		"dirty locality: only StrokePulse/CirclePulse/ClipPulse animate | P1: radial/path/oval/arc | not P6 present",
	), 11, 0.5, 0.52, 0.55)
	root.Place(note, 12, winH-24)

	return s
}

func (s *geoScene) onTick(dt float64, schedule func()) {
	if s == nil {
		return
	}
	s.phase += dt * 0.7
	if s.phase > 1 {
		s.phase -= 1
	}
	if s.StrokePulse != nil {
		s.StrokePulse.MarkNeedsPaint()
	}
	if s.CirclePulse != nil {
		s.CirclePulse.MarkNeedsPaint()
	}
	if s.ClipPulse != nil {
		s.ClipPulse.MarkNeedsPaint()
	}
	if schedule != nil {
		schedule()
	}
}
