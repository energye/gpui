package main

import (
	"math"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

type clipScene struct {
	Root *rendering.AbsoluteBox

	Hot        *rendering.RenderBox
	OpacityBox *rendering.RenderBox
	ClipInner  *rendering.RenderBox
	ClipRRect  *rendering.RenderClipRRect
	Xform      *rendering.RenderTransform
	phase      float64
	clipOff    float64
}

func lbl(s string, size, r, g, b float64, face text.Face) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.FontSize = size
	t.R, t.G, t.B, t.A = r, g, b, 1
	if face != nil {
		t.SetFace(face)
	}
	return t
}

func fill(pc *rendering.PaintContext, x, y, w, h, r, g, b, a float64) {
	if pc == nil || pc.DC == nil || w <= 0 || h <= 0 {
		return
	}
	pc.DC.SetRGBA(r, g, b, a)
	pc.DC.DrawRectangle(pc.OriginX+x, pc.OriginY+y, w, h)
	_ = pc.DC.Fill()
}

func stroke(pc *rendering.PaintContext, x, y, w, h, r, g, b float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.SetRGBA(r, g, b, 1)
	pc.DC.SetLineWidth(1)
	pc.DC.DrawRectangle(pc.OriginX+x, pc.OriginY+y, w, h)
	_ = pc.DC.Stroke()
}

func panel(w, h float64, paint func(*rendering.PaintContext, rendering.Size)) *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = w, h
	b.SetRepaintBoundary(true)
	b.OnPaint = paint
	return b
}

func buildClipLayerScene(winW, winH float64, face text.Face) *clipScene {
	s := &clipScene{}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.09, G: 0.10, B: 0.12, A: 1}
	s.Root = root

	root.Place(lbl("ClipLayer axis — Boundary · Clip · ClipRRect · Opacity · Transform", 13, 0.75, 0.78, 0.85, face), 12, 8)
	root.Place(lbl("static boundaries skip · hot anim · ClipRRectLayer + TransformLayer in scene tree", 10, 0.5, 0.55, 0.6, face), 12, 28)

	// Static RepaintBoundary cards
	for i := 0; i < 6; i++ {
		ii := i
		card := panel(100, 64, func(pc *rendering.PaintContext, sz rendering.Size) {
			fill(pc, 0, 0, sz.Width, sz.Height, 0.14, 0.16, 0.20, 1)
			stroke(pc, 1, 1, sz.Width-2, sz.Height-2, 0.3, 0.35, 0.4)
			fill(pc, 12, 16, sz.Width-24, sz.Height-32, 0.2+float64(ii)*0.08, 0.35, 0.55, 1)
		})
		root.Place(card, 12+float64(i%3)*112, 52+float64(i/3)*76)
	}

	s.Hot = panel(220, 100, func(pc *rendering.PaintContext, sz rendering.Size) {
		fill(pc, 0, 0, sz.Width, sz.Height, 0.12, 0.14, 0.18, 1)
		fill(pc, 16, 20, sz.Width-32, sz.Height-40, 0.3+0.5*s.phase, 0.55, 1-0.4*s.phase, 1)
	})
	root.Place(s.Hot, 360, 52)
	root.Place(lbl("hot RepaintBoundary", 11, 0.95, 0.75, 0.4, face), 360, 158)

	s.ClipInner = panel(300, 120, func(pc *rendering.PaintContext, sz rendering.Size) {
		fill(pc, 0, 0, sz.Width, sz.Height, 0.11, 0.13, 0.16, 1)
		if pc.DC != nil {
			pc.DC.Push()
			pc.DC.ClipRect(pc.OriginX+8, pc.OriginY+8, sz.Width-16, sz.Height-16)
			y0 := 8 - s.clipOff
			for i := 0; i < 8; i++ {
				yy := y0 + float64(i)*28
				fill(pc, 16, yy, sz.Width-32, 24, 0.25, 0.45+float64(i)*0.05, 0.7, 1)
			}
			pc.DC.Pop()
		}
		stroke(pc, 8, 8, sz.Width-16, sz.Height-16, 0.9, 0.5, 0.3)
	})
	root.Place(s.ClipInner, 12, 220)
	root.Place(lbl("ClipRect overflow (paint scroll)", 11, 0.95, 0.75, 0.4, face), 12, 346)

	// RenderClipRRect → scene.ClipRRectLayer via BuildLayerTree (序7 main path).
	rrectChild := rendering.NewRenderColorBox(120, 72, 0.85, 0.35, 0.55, 1)
	s.ClipRRect = rendering.NewRenderClipRRect(rrectChild)
	s.ClipRRect.FixedWidth, s.ClipRRect.FixedHeight = 120, 72
	s.ClipRRect.SetRadius(18)
	s.ClipRRect.SetRepaintBoundary(true)
	root.Place(s.ClipRRect, 12, 370)
	root.Place(lbl("ClipRRectLayer RO→scene", 11, 0.95, 0.75, 0.4, face), 12, 448)

	s.OpacityBox = panel(160, 120, func(pc *rendering.PaintContext, sz rendering.Size) {
		fill(pc, 0, 0, sz.Width, sz.Height, 0.12, 0.14, 0.18, 1)
		a := 0.25 + 0.7*s.phase
		fill(pc, 20, 20, sz.Width-40, sz.Height-40, 0.9, 0.4, 0.55, a)
	})
	root.Place(s.OpacityBox, 330, 220)
	root.Place(lbl("Opacity pulse", 11, 0.95, 0.75, 0.4, face), 330, 346)

	// Transform target area
	back := panel(100, 100, func(pc *rendering.PaintContext, sz rendering.Size) {
		fill(pc, 0, 0, sz.Width, sz.Height, 0.12, 0.14, 0.18, 1)
		stroke(pc, 1, 1, sz.Width-2, sz.Height-2, 0.35, 0.4, 0.45)
	})
	root.Place(back, 520, 220)

	child := rendering.NewRenderColorBox(64, 64, 0.3, 0.75, 0.95, 1)
	s.Xform = rendering.NewRenderTransform(child)
	s.Xform.FixedWidth, s.Xform.FixedHeight = 64, 64
	s.Xform.SetRepaintBoundary(true)
	s.Xform.SetScale(1, 1)
	root.Place(s.Xform, 538, 238)
	root.Place(lbl("TransformLayer rot", 11, 0.95, 0.75, 0.4, face), 520, 346)

	root.Place(lbl("not P6 · transform hit≈AABB · opacity via paint alpha", 10, 0.5, 0.52, 0.55, face), 12, winH-24)
	return s
}

func (s *clipScene) onTick(dt float64, schedule func()) {
	if s == nil {
		return
	}
	s.phase += dt * 0.8
	if s.phase > 1 {
		s.phase -= 1
	}
	s.clipOff += dt * 40
	if s.clipOff > 120 {
		s.clipOff = 0
	}
	if s.Hot != nil {
		s.Hot.MarkNeedsPaint()
	}
	if s.OpacityBox != nil {
		s.OpacityBox.MarkNeedsPaint()
	}
	if s.ClipInner != nil {
		s.ClipInner.MarkNeedsPaint()
	}
	if s.Xform != nil {
		s.Xform.SetRotation(s.phase * 2 * math.Pi)
	}
	// Pulse radius so ClipRRect RO stays on the hot paint path (layer params update).
	if s.ClipRRect != nil {
		s.ClipRRect.SetRadius(10 + 12*s.phase)
	}
	if schedule != nil {
		schedule()
	}
}
