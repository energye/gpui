package wrkit

import (
	"github.com/energye/gpui/ui/rendering"
)

// Panel is a nested AbsoluteBox region with background — use for shell layout
// so children Place coords are **relative to the panel**, not the window.
// This avoids "coordinate hole" UX where root-level Place looks unowned.
type Panel struct {
	Box *rendering.AbsoluteBox
	// X,Y are the panel origin on the parent (window or outer panel).
	X, Y float64
	// W,H logical size.
	W, H float64
}

// NewPanel builds a fixed-size AbsoluteBox with solid background.
func NewPanel(w, h, r, g, b, a float64) *Panel {
	box := rendering.NewAbsoluteBox(w, h)
	box.Background = &rendering.Color{R: r, G: g, B: b, A: a}
	return &Panel{Box: box, W: w, H: h}
}

// PlaceOn puts this panel onto parent at (x,y).
func (p *Panel) PlaceOn(parent *rendering.AbsoluteBox, x, y float64) {
	if p == nil || p.Box == nil || parent == nil {
		return
	}
	p.X, p.Y = x, y
	parent.Place(p.Box, x, y)
}

// Place child at panel-local (x,y).
func (p *Panel) Place(child rendering.RenderObject, x, y float64) {
	if p == nil || p.Box == nil {
		return
	}
	p.Box.Place(child, x, y)
}

// LabelAt places a faced label at panel-local coords.
func (p *Panel) LabelAt(text string, size, x, y, r, g, b float64) *rendering.RenderText {
	t := Label(text, size, r, g, b)
	p.Place(t, x, y)
	return t
}

// Align places child inside this panel with Flutter Align semantics: the
// wrapper fills the panel and offsets child by fractional alignment of the
// leftover space (0=left/top, 0.5=center, 1=right/bottom). Positioning is
// layout-driven — on panel resize Layout recomputes the offset automatically,
// so the child follows the size without imperative coordinates or clamping.
func (p *Panel) Align(child rendering.RenderObject, ax, ay float64) *rendering.RenderAlignBox {
	if p == nil || p.Box == nil {
		return nil
	}
	a := rendering.NewRenderAlignBox(child, ax, ay)
	p.Box.Place(a, 0, 0)
	return a
}

// ColorAt places a solid color box at panel-local coords (optional boundary).
func (p *Panel) ColorAt(w, h, x, y, r, g, b, a float64, boundary bool) *rendering.RenderColorBox {
	c := rendering.NewRenderColorBox(w, h, r, g, b, a)
	if boundary {
		c.SetRepaintBoundary(true)
	}
	p.Place(c, x, y)
	return c
}
