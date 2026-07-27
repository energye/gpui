package rendering

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// RenderText draws text via render.DrawString / DrawStringWrapped.
// Layout prefers Face.Measure when Face is set; otherwise EstimateTextSize (rune-based).
// When MaxWidth > 0, layout/paint use wrapped multiline (P1).
type RenderText struct {
	Base
	Text       string
	FontSize   float64 // logical; used for estimate and as draw Y baseline offset
	R, G, B, A float64
	// ApproxCharW factor for layout without a Face (latin ~0.55; CJK often ~1).
	ApproxCharW float64
	// Face optional shaped face for true Measure (from render/text).
	Face text.Face
	// MaxWidth > 0 enables word-wrap layout/paint (logical px).
	MaxWidth float64
	// LineSpacing multiplier for wrapped lines (default 1.2).
	LineSpacing float64
	// Align for wrapped text (render.Align*).
	Align render.Align
}

// NewRenderText creates a text node.
func NewRenderText(textStr string) *RenderText {
	t := &RenderText{
		Text: textStr, FontSize: 14,
		R: 0.9, G: 0.9, B: 0.9, A: 1,
		ApproxCharW: 0.55,
		LineSpacing: 1.2,
		Align:       render.AlignLeft,
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

// SetMaxWidth enables (>0) or disables (≤0) wrap; dirties layout+paint.
func (t *RenderText) SetMaxWidth(w float64) {
	if t == nil || t.MaxWidth == w {
		return
	}
	t.MaxWidth = w
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
	ls := t.LineSpacing
	if ls <= 0 {
		ls = 1.2
	}

	if t.MaxWidth > 0 {
		// Wrapped: width capped; height ≈ lines * lineHeight.
		if t.Face != nil && t.Text != "" {
			// Approximate line count via face line height + measure each line would need DC;
			// use Measure on full string width clamp: height from soft estimate.
			mw, _ := text.Measure(t.Text, t.Face)
			if mw > t.MaxWidth {
				// crude line count from char estimate
				avg := aw * fs
				if avg < 1 {
					avg = 1
				}
				charsPerLine := t.MaxWidth / avg
				if charsPerLine < 1 {
					charsPerLine = 1
				}
				// rune count
				n := float64(len([]rune(t.Text)))
				lines := n / charsPerLine
				if lines < 1 {
					lines = 1
				}
				lh := fs * ls * 1.25
				return t.MaxWidth, lines * lh
			}
			_, mh := text.Measure(t.Text, t.Face)
			return t.MaxWidth, mh
		}
		// No face: estimate wrapped height
		n := float64(len([]rune(t.Text)))
		avg := aw * fs
		if avg < 1 {
			avg = 1
		}
		charsPerLine := t.MaxWidth / avg
		if charsPerLine < 1 {
			charsPerLine = 1
		}
		lines := n / charsPerLine
		if lines < 1 {
			lines = 1
		}
		return t.MaxWidth, lines * fs * 1.25 * ls
	}

	if t.Face != nil && t.Text != "" {
		return text.Measure(t.Text, t.Face)
	}
	return EstimateTextSize(t.Text, fs, aw)
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
func (t *RenderText) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() {
		return
	}
	pc.NotePaintVisit()
	if t.Text != "" {
		a := t.A
		if a == 0 && (t.R != 0 || t.G != 0 || t.B != 0) {
			a = 1
		}
		if t.Face != nil && pc.DC != nil {
			pc.DC.SetFont(t.Face)
		}
		if t.MaxWidth > 0 {
			ls := t.LineSpacing
			if ls <= 0 {
				ls = 1.2
			}
			align := t.Align
			drawTextWrapped(pc, t.Text, 0, 0, t.MaxWidth, ls, align, t.R, t.G, t.B, a)
		} else {
			// Y uses FontSize as a simple baseline offset (single-line MVP).
			drawTextColored(pc, t.Text, 0, t.FontSize, t.R, t.G, t.B, a)
		}
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
