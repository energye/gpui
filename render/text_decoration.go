package render

// TextDecoration is a bitset of text decorations (X.08).
type TextDecoration uint8

const (
	TextDecorationNone        TextDecoration = 0
	TextDecorationUnderline   TextDecoration = 1 << 0
	TextDecorationLineThrough TextDecoration = 1 << 1
	TextDecorationOverline    TextDecoration = 1 << 2
)

// SetTextDecoration sets decorations applied after DrawString (underline etc.).
func (c *Context) SetTextDecoration(d TextDecoration) {
	if c == nil {
		return
	}
	c.textDecoration = d
}

// TextDecoration returns the current decoration bitset.
func (c *Context) TextDecoration() TextDecoration {
	if c == nil {
		return TextDecorationNone
	}
	return c.textDecoration
}

func (c *Context) applyTextDecorations(s string, x, y float64) {
	if c == nil || c.textDecoration == TextDecorationNone || c.face == nil || s == "" {
		return
	}
	w := c.face.Advance(s)
	if w <= 0 {
		return
	}
	m := c.face.Metrics()
	// Thickness ~ max(1, size/16)
	size := c.face.Size()
	thickness := size / 16
	if thickness < 1 {
		thickness = 1
	}
	// Underline slightly below baseline
	if c.textDecoration&TextDecorationUnderline != 0 {
		uy := y + m.Descent*0.5
		if uy < y+1 {
			uy = y + thickness
		}
		c.drawDecorationLine(x, uy, w, thickness)
	}
	if c.textDecoration&TextDecorationLineThrough != 0 {
		// Midway through x-height / ascent
		my := y - m.Ascent*0.35
		c.drawDecorationLine(x, my, w, thickness)
	}
	if c.textDecoration&TextDecorationOverline != 0 {
		oy := y - m.Ascent
		c.drawDecorationLine(x, oy, w, thickness)
	}
}

func (c *Context) drawDecorationLine(x, y, w, thickness float64) {
	// Use current fill color as a thin filled rect (GPU or CPU via path).
	c.DrawRectangle(x, y, w, thickness)
	_ = c.Fill()
}
