package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// Text paints a single-line string (C-Measure baseline; ellipsis later).
//
// FontSize is authoritative when > 0: if Face was created at a different size
// (e.g. Face(14) while FontSize=8), the face is re-derived via FontSource.
type Text struct {
	core.NodeBase

	Value    string
	Color    render.RGBA
	FontSize float64 // points; 0 → 14
	// Face is optional; when set, used for measure and draw (re-sized to FontSize).
	Face text.Face
	// MaxWidth when > 0 constrains layout width (no wrap in M0; may clip).
	MaxWidth float64
}

// NewText constructs a Text node.
func NewText(value string) *Text {
	t := &Text{
		Value:    value,
		Color:    render.RGBA{R: 0, G: 0, B: 0, A: 0.88},
		FontSize: 14,
	}
	t.Init(t)
	t.Hit = core.HitDefer
	return t
}

// TypeID implements core.Node.
func (t *Text) TypeID() string { return TypeText }

// SetValue updates text and dirties layout.
func (t *Text) SetValue(v string) {
	if t.Value == v {
		return
	}
	t.Value = v
	t.MarkNeedsLayout()
}

// SetFontSize updates point size and dirties layout (re-derives Face when needed).
func (t *Text) SetFontSize(px float64) {
	if t.FontSize == px {
		return
	}
	t.FontSize = px
	t.MarkNeedsLayout()
	t.MarkNeedsPaint()
}

// effectiveFace returns Face sized to FontSize.
func (t *Text) effectiveFace() text.Face {
	size := t.FontSize
	if size <= 0 {
		size = 14
	}
	return faceAtSize(t.Face, size)
}

// Layout implements core.Node.
func (t *Text) Layout(c core.Constraints) core.Size {
	w, h := t.measure()
	if t.MaxWidth > 0 && w > t.MaxWidth {
		w = t.MaxWidth
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	t.SetSize(out)
	return out
}

func (t *Text) measure() (w, h float64) {
	size := t.FontSize
	if size <= 0 {
		size = 14
	}
	face := t.effectiveFace()
	if face != nil {
		w = face.Advance(t.Value)
		m := face.Metrics()
		h = m.Ascent + m.Descent
		if h <= 0 {
			h = size * 1.2
		}
		return w, h
	}
	// Approximate when no face: ~0.5em average advance (Latin-ish).
	w = float64(len([]rune(t.Value))) * size * 0.5
	h = size * 1.2
	return w, h
}

// Paint implements core.Node.
func (t *Text) Paint(pc *core.PaintContext) {
	if pc == nil || pc.DC == nil || t.Value == "" {
		return
	}
	dc := pc.DC
	face := t.effectiveFace()
	if face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(t.Color.R, t.Color.G, t.Color.B, t.Color.A)
	// Baseline ≈ Origin.Y + ascent
	ascent := t.FontSize
	if t.FontSize <= 0 {
		ascent = 14
	}
	if face != nil {
		ascent = face.Metrics().Ascent
	} else {
		ascent = ascent * 0.8
	}
	dc.DrawString(t.Value, pc.Origin.X, pc.Origin.Y+ascent)
}

// HitTest implements core.Node.
func (t *Text) HitTest(p core.Point) core.Node { return t.DefaultHitTest(p) }
