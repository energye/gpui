// Package main scene builder for ui_l1_render_matrix (example-local only — not a ui/ library).
package main

import (
	"fmt"

	"github.com/energye/gpui/ui/painting"
	"github.com/energye/gpui/ui/rendering"
)

// matrixScene holds example widgets driven by tickers in main.
type matrixScene struct {
	Root *rendering.AbsoluteBox

	Spin  *rendering.RenderSpinner
	HotA  *rendering.RenderColorBox
	HotB  *rendering.RenderColorBox
	Latin *rendering.RenderText
	CJK   *rendering.RenderText
	List  *rendering.VirtualList
	VP    *rendering.RenderViewport
	Grad  *rendering.RenderBox

	itemCount  int
	itemExtent float64
	scrollY    float64
	phase      float64
}

func buildMatrixScene(winW, winH float64) *matrixScene {
	s := &matrixScene{
		itemCount:  1000,
		itemExtent: 28,
	}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.09, G: 0.10, B: 0.12, A: 1}
	s.Root = root

	// Title chrome (static after first paint)
	title := rendering.NewRenderText("gpui render matrix — PARTIAL Flutter L1 contracts (not P6 dirty-rect present)")
	title.FontSize = 13
	title.R, title.G, title.B, title.A = 0.75, 0.78, 0.85, 1
	root.Place(title, 12, 8)

	// --- M1 spinner panel ---
	m1Label := rendering.NewRenderText("M1 spinner + static")
	m1Label.FontSize = 12
	m1Label.R, m1Label.G, m1Label.B = 0.6, 0.85, 1
	root.Place(m1Label, 12, 36)

	s.Spin = rendering.NewRenderSpinner(36)
	root.Place(s.Spin, 40, 64)
	for i := 0; i < 12; i++ {
		c := rendering.NewRenderColorBox(10, 10, 0.22, 0.25, 0.30, 1)
		c.SetRepaintBoundary(true)
		root.Place(c, 100+float64(i%4)*14, 60+float64(i/4)*14)
	}

	// --- M2 multi-boundary ---
	m2Label := rendering.NewRenderText("M2 multi-boundary")
	m2Label.FontSize = 12
	m2Label.R, m2Label.G, m2Label.B = 0.6, 0.85, 1
	root.Place(m2Label, 220, 36)

	s.HotA = rendering.NewRenderColorBox(36, 36, 0.85, 0.35, 0.25, 1)
	s.HotA.SetRepaintBoundary(true)
	s.HotB = rendering.NewRenderColorBox(36, 36, 0.25, 0.75, 0.55, 1)
	s.HotB.SetRepaintBoundary(true)
	root.Place(s.HotA, 220, 60)
	root.Place(s.HotB, 268, 60)
	for i := 0; i < 6; i++ {
		c := rendering.NewRenderColorBox(18, 18, 0.28, 0.30, 0.34, 1)
		c.SetRepaintBoundary(true)
		root.Place(c, 220+float64(i%3)*22, 108+float64(i/3)*22)
	}

	// --- M4 text ---
	m4Label := rendering.NewRenderText("M4 text Latin/CJK")
	m4Label.FontSize = 12
	m4Label.R, m4Label.G, m4Label.B = 0.6, 0.85, 1
	root.Place(m4Label, 400, 36)

	s.Latin = rendering.NewRenderText("Hello matrix")
	s.Latin.FontSize = 16
	s.Latin.SetRepaintBoundary(true)
	s.CJK = rendering.NewRenderText("你好世界 · 渲染矩阵")
	s.CJK.FontSize = 16
	s.CJK.R, s.CJK.G, s.CJK.B = 0.95, 0.85, 0.45
	s.CJK.SetRepaintBoundary(true)
	root.Place(s.Latin, 400, 70)
	root.Place(s.CJK, 400, 100)

	// --- M3 mini virtual list ---
	m3Label := rendering.NewRenderText("M3 VirtualList 1k (auto-scroll)")
	m3Label.FontSize = 12
	m3Label.R, m3Label.G, m3Label.B = 0.6, 0.85, 1
	root.Place(m3Label, 12, 170)

	s.List = rendering.NewVirtualList(s.itemCount, s.itemExtent, func(i int) rendering.RenderObject {
		r, g, b := 0.30, 0.34, 0.40
		if i%2 == 0 {
			r, g, b = 0.22, 0.48, 0.72
		}
		row := rendering.NewRenderColorBox(0, s.itemExtent-2, r, g, b, 1)
		return row
	})
	s.List.CacheExtent = s.itemExtent * 2
	s.VP = rendering.NewRenderViewport(s.List)
	// Viewport needs a fixed size via wrapping AbsoluteBox placement + Fixed on a box.
	// RenderViewport lays out to constraints from parent AbsoluteBox inner max.
	// Give it an explicit size by placing inside a sized box.
	listHost := rendering.NewRenderBox(s.VP)
	listHost.FixedWidth, listHost.FixedHeight = 360, 160
	listHost.SetRelayoutBoundary(true)
	root.Place(listHost, 12, 192)

	// --- M5 gradient / rrect ---
	m5Label := rendering.NewRenderText("M5 gradient + round rect")
	m5Label.FontSize = 12
	m5Label.R, m5Label.G, m5Label.B = 0.6, 0.85, 1
	root.Place(m5Label, 400, 170)

	s.Grad = rendering.NewRenderBox()
	s.Grad.FixedWidth, s.Grad.FixedHeight = 280, 120
	s.Grad.SetRepaintBoundary(true)
	s.Grad.OnPaint = func(pc *painting.Context, size rendering.Size) {
		// phase-tinted gradient endpoints for slow visual change when MarkNeedsPaint.
		pc.FillLinearGradient2(0, 0, size.Width, size.Height,
			0, 0, size.Width, size.Height,
			0.12, 0.35, 0.85, 1,
			0.90, 0.25+0.2*s.phase, 0.35, 1,
		)
		pc.FillRoundRect(16, 24, size.Width-32, size.Height-48, 12,
			0.08, 0.09, 0.11, 0.75)
		pc.DrawTextColored("FillLinearGradient + FillRoundRect", 28, 64, 0.92, 0.94, 0.98, 1)
	}
	root.Place(s.Grad, 400, 192)

	// Footnote
	note := rendering.NewRenderText(fmt.Sprintf(
		"metrics: layer dirty locality ≠ GPU partial present | list rows=%d extent=%.0f",
		s.itemCount, s.itemExtent,
	))
	note.FontSize = 11
	note.R, note.G, note.B = 0.55, 0.58, 0.62
	root.Place(note, 12, winH-28)

	return s
}

func (s *matrixScene) onTick(dt float64, schedule func()) {
	if s == nil {
		return
	}
	s.phase += dt
	if s.phase > 1 {
		s.phase -= 1
	}
	// M1
	if s.Spin != nil {
		s.Spin.SetPhase(s.phase)
	}
	// M2 — only two hot boundaries pulse
	if s.HotA != nil {
		s.HotA.R = 0.55 + 0.4*s.phase
		s.HotA.MarkNeedsPaint()
	}
	if s.HotB != nil {
		s.HotB.G = 0.45 + 0.4*(1-s.phase)
		s.HotB.MarkNeedsPaint()
	}
	// M4 — color only on latin (no layout storm)
	if s.Latin != nil {
		s.Latin.SetColor(0.7+0.25*s.phase, 0.85, 1-0.3*s.phase, 1)
	}
	// M3 scroll
	if s.VP != nil && s.List != nil {
		s.scrollY += 60 * dt
		max := float64(s.itemCount)*s.itemExtent - 160
		if max < 0 {
			max = 0
		}
		if s.scrollY > max {
			s.scrollY = 0
		}
		s.VP.SetScrollOffset(0, s.scrollY)
	}
	// M5 — gradient band repaint (boundary-local)
	if s.Grad != nil {
		s.Grad.MarkNeedsPaint()
	}
	if schedule != nil {
		schedule()
	}
}
