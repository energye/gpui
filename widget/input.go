package widget

import "github.com/energye/gpui/render"

// Input is a labeled single-line field (paint-only).
type Input struct {
	Bounds      Rect
	Label       string
	Value       string
	Placeholder string
	Error       string
	Focused     bool
	Disabled    bool
}

// FieldRect returns the editable box (below optional label).
func (i Input) FieldRect(th Theme) Rect {
	r := i.Bounds
	if i.Label == "" {
		if r.H <= 0 {
			r.H = th.ControlHeight
		}
		return r
	}
	// label ~18px + gap 4 + field
	fieldH := th.ControlHeight
	if fieldH <= 0 {
		fieldH = 32
	}
	return Rect{X: r.X, Y: r.Y + 22, W: r.W, H: fieldH}
}

// HitTest hits the field box (not the outer label area alone).
// HitTest with theme is HitTestTheme; see control.go for Hitter.
func (i Input) hitField(th Theme, x, y float64) bool {
	return i.FieldRect(th).Contains(x, y)
}

// Draw paints label, field, value/placeholder, and error text.
func (i Input) Draw(dc *render.Context, th Theme) {
	if th.ControlHeight <= 0 {
		th = DefaultTheme()
	}
	if i.Label != "" {
		setRGBA(dc, th.Text)
		dc.DrawString(i.Label, i.Bounds.X, i.Bounds.Y+14)
	}
	fr := i.FieldRect(th)
	bg := th.Surface
	border := th.Border
	if i.Focused && !i.Disabled {
		border = th.Primary
	}
	if i.Error != "" {
		border = th.Error
	}
	if i.Disabled {
		bg = th.SurfaceAlt
	}
	fillRound(dc, fr, th.RadiusSM, bg)
	strokeRound(dc, fr, th.RadiusSM, 1, border)
	if i.Focused && !i.Disabled {
		strokeRound(dc, fr.Inset(-2), th.RadiusSM+2, 2, th.FocusRing)
	}

	text := i.Value
	col := th.Text
	if text == "" {
		text = i.Placeholder
		col = th.Placeholder
	}
	if i.Disabled {
		col = th.TextDisabled
	}
	if text != "" {
		setRGBA(dc, col)
		dc.DrawString(text, fr.X+th.PadX, textBaselineY(fr, th.FontSize))
	}
	if i.Error != "" {
		setRGBA(dc, th.Error)
		dc.DrawString(i.Error, fr.X, fr.Y+fr.H+16)
	}
}
