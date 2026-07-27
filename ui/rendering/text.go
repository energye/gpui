package rendering

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/painting"
)

// RenderText draws a single-line (or raw) string via render.DrawString.
// Layout prefers Face.Measure when Face is set; otherwise EstimateTextSize (rune-based).
type RenderText struct {
	Base
	Text       string
	FontSize   float64 // logical; used for estimate and as draw Y baseline offset
	R, G, B, A float64
	// ApproxCharW factor for layout without a Face (latin ~0.55; CJK often ~1).
	ApproxCharW float64
	// Face optional shaped face for true Measure (from render/text).
	Face text.Face
}

// NewRenderText creates a text node.
func NewRenderText(textStr string) *RenderText {
	t := &RenderText{
		Text: textStr, FontSize: 14,
		R: 0.9, G: 0.9, B: 0.9, A: 1,
		ApproxCharW: 0.55,
	}
	t.Init(t)
	return t
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

// SetFace sets an optional font face for measure/draw alignment with render text.
func (t *RenderText) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

func (t *RenderText) measureSize() (w, h float64) {
	fs := t.FontSize
	if fs <= 0 {
		fs = 14
	}
	aw := t.ApproxCharW
	if aw <= 0 {
		aw = 0.55
	}
	if t.Face != nil && t.Text != "" {
		return text.Measure(t.Text, t.Face)
	}
	return painting.EstimateTextSize(t.Text, fs, aw)
}

// Layout implements RenderObject.
func (t *RenderText) Layout(c Constraints) Size {
	if sz, ok := t.LayoutSkipIfClean(c); ok {
		return sz
	}
	w, h := t.measureSize()
	out := c.Tighten(Size{Width: w, Height: h})
	t.setSize(out)
	t.RememberConstraints(c)
	t.clearLayoutDirty()
	return out
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
		if t.Face != nil && pc.DC != nil {
			pc.DC.SetFont(t.Face)
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
