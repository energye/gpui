package rendering

import "github.com/energye/gpui/ui/painting"

// RenderText draws a single-line (or raw) string via render.DrawString.
// Layout uses approximate size if measure is unavailable.
type RenderText struct {
	Base
	Text       string
	FontSize   float64 // logical; hint only for approximate metrics
	R, G, B, A float64
	// Approx char width factor for layout without shaping (P4 MVP).
	ApproxCharW float64
}

// NewRenderText creates a text node.
func NewRenderText(text string) *RenderText {
	t := &RenderText{
		Text: text, FontSize: 14,
		R: 0.9, G: 0.9, B: 0.9, A: 1,
		ApproxCharW: 0.55,
	}
	t.Init(t)
	return t
}

// Layout implements RenderObject.
func (t *RenderText) Layout(c Constraints) Size {
	if sz, ok := t.LayoutSkipIfClean(c); ok {
		return sz
	}
	fs := t.FontSize
	if fs <= 0 {
		fs = 14
	}
	aw := t.ApproxCharW
	if aw <= 0 {
		aw = 0.55
	}
	w := float64(len(t.Text)) * fs * aw
	h := fs * 1.25
	out := c.Tighten(Size{Width: w, Height: h})
	t.setSize(out)
	t.RememberConstraints(c)
	t.clearLayoutDirty()
	return out
}

// SetText updates the string and dirties layout+paint when changed.
func (t *RenderText) SetText(s string) {
	if t == nil || t.Text == s {
		return
	}
	t.Text = s
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// SetColor updates RGBA and dirties paint only (no layout).
func (t *RenderText) SetColor(r, g, b, a float64) {
	if t == nil {
		return
	}
	t.R, t.G, t.B, t.A = r, g, b, a
	t.MarkNeedsPaint()
}

// Paint implements RenderObject.
func (t *RenderText) Paint(pc *painting.Context) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() {
		return
	}
	pc.NotePaintVisit()
	if t.Text != "" {
		// Y uses FontSize as a simple baseline offset (engine MVP, not full TextPainter).
		a := t.A
		if a == 0 && (t.R != 0 || t.G != 0 || t.B != 0) {
			a = 1
		}
		pc.DrawTextColored(t.Text, 0, t.FontSize, t.R, t.G, t.B, a)
	}
	t.clearPaintDirty()
}

// HitTest implements RenderObject.
func (t *RenderText) HitTest(p Point) RenderObject {
	sz := t.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return t
	}
	return nil
}
