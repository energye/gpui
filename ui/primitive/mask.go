package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Mask is a dimmed full-area layer that absorbs clicks (C-Overlay).
// Typically placed under a panel in an overlay stack.
type Mask struct {
	core.NodeBase

	// Color of the mask; default semi-transparent black.
	Color render.RGBA
	// OnDismiss is called when the mask is clicked.
	OnDismiss func()
	// Width/Height should match viewport; 0 → expand in layout.
	Width, Height float64
}

// NewMask creates a dismissible mask.
func NewMask() *Mask {
	m := &Mask{
		Color: render.RGBA{R: 0, G: 0, B: 0, A: 0.45},
	}
	m.Init(m)
	m.Hit = core.HitTarget
	return m
}

// TypeID implements core.Node.
func (m *Mask) TypeID() string { return TypeMask }

// Layout implements core.Node.
func (m *Mask) Layout(c core.Constraints) core.Size {
	w, h := m.Width, m.Height
	if w <= 0 {
		w = c.MaxWidth
		if w >= core.Unbounded/2 {
			w = c.MinWidth
		}
	}
	if h <= 0 {
		h = c.MaxHeight
		if h >= core.Unbounded/2 {
			h = c.MinHeight
		}
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	m.SetSize(out)
	return out
}

// Paint implements core.Node.
func (m *Mask) Paint(pc *core.PaintContext) {
	col := m.Color
	if pc != nil && pc.Theme != nil {
		if c := pc.Theme.Color(core.TokenColorBgMask); c.A > 0 && m.Color.A == 0.45 {
			col = c
		}
	}
	if pc != nil && col.A > 0 {
		sz := m.Size()
		pc.FillLocalRect(0, 0, sz.Width, sz.Height, col)
	}
}

// HitTest implements core.Node.
func (m *Mask) HitTest(p core.Point) core.Node {
	if m.LocalBounds().Contains(p) {
		return m
	}
	return nil
}

// HandlePointer implements core.PointerHandler.
func (m *Mask) HandlePointer(ev *core.PointerEvent) {
	if ev != nil && ev.Type == core.PointerDown {
		ev.Handled = true
	}
}

// OnClick implements core.ClickHandler.
func (m *Mask) OnClick(ev *core.PointerEvent) {
	if m.OnDismiss != nil {
		m.OnDismiss()
	}
	if ev != nil {
		ev.Handled = true
	}
}
