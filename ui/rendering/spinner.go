package rendering

import (
	"math"
)

// RenderSpinner is a test/demo spinner (not a product kit control).
// It is a RepaintBoundary by default so phase updates only dirty this subtree.
type RenderSpinner struct {
	Base
	// Edge is logical square edge length; default 24. (not named Size — conflicts with Size() method)
	Edge float64
	// Phase is rotation phase in [0,1).
	Phase float64
	// Color
	R, G, B, A float64
}

// NewRenderSpinner creates a spinner with repaint boundary enabled.
func NewRenderSpinner(edge float64) *RenderSpinner {
	if edge <= 0 {
		edge = 24
	}
	s := &RenderSpinner{
		Edge: edge,
		R:    0.2, G: 0.55, B: 0.95, A: 1,
	}
	s.Init(s)
	s.SetRepaintBoundary(true)
	return s
}

// SetPhase updates phase and marks paint only (no layout).
func (s *RenderSpinner) SetPhase(p float64) {
	if s == nil {
		return
	}
	// Normalize
	p = p - math.Floor(p)
	if p == s.Phase {
		return
	}
	s.Phase = p
	s.MarkNeedsPaint()
}

// Layout implements RenderObject — fixed size, never depends on phase.
func (s *RenderSpinner) Layout(c Constraints) Size {
	if sz, ok := s.LayoutSkipIfClean(c); ok {
		return sz
	}
	out := c.Tighten(Size{Width: s.Edge, Height: s.Edge})
	s.setSize(out)
	s.RememberConstraints(c)
	s.clearLayoutDirty()
	return out
}

// Paint draws 4 dots around a circle; phase rotates which is brightest.
func (s *RenderSpinner) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	// Spinner is a RepaintBoundary: parents skip us when clean. If Paint is called, draw.
	pc.NotePaintVisit()
	sz := s.size
	if sz.Width <= 0 {
		sz.Width = s.Edge
	}
	if sz.Height <= 0 {
		sz.Height = s.Edge
	}
	cx := sz.Width / 2
	cy := sz.Height / 2
	radius := math.Min(sz.Width, sz.Height)*0.5 - 3
	if radius < 2 {
		radius = 2
	}
	dot := math.Max(2, radius*0.28)
	base := s.Phase * 2 * math.Pi
	for i := 0; i < 4; i++ {
		ang := base + float64(i)*math.Pi/2
		dx := math.Cos(ang) * radius
		dy := math.Sin(ang) * radius
		// Y-down: +dy is downward (math angle still works for relative layout).
		ph := s.Phase + float64(i)*0.25
		ph = ph - math.Floor(ph)
		op := 0.3 + 0.7*math.Abs(0.5-ph)*2
		if op > 1 {
			op = 1
		}
		fillRect(pc, cx+dx-dot/2, cy+dy-dot/2, dot, dot, s.R, s.G, s.B, s.A*op)
	}
	s.clearPaintDirty()
}

// HitTest implements RenderObject.
func (s *RenderSpinner) HitTest(p Point) RenderObject {
	sz := s.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return s
	}
	return nil
}
