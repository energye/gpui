package painting

// FillRoundRect fills a rounded rectangle in local logical coordinates (Y-down).
// radius is the corner radius in logical px (clamped by half min(w,h)).
func (c *Context) FillRoundRect(x, y, w, h, radius, r, g, b, a float64) {
	if c == nil || c.DC == nil || w <= 0 || h <= 0 {
		return
	}
	if radius < 0 {
		radius = 0
	}
	maxR := w
	if h < maxR {
		maxR = h
	}
	maxR *= 0.5
	if radius > maxR {
		radius = maxR
	}
	ax := c.OriginX + x
	ay := c.OriginY + y
	c.DC.SetRGBA(r, g, b, a)
	if radius <= 0 {
		c.DC.DrawRectangle(ax, ay, w, h)
	} else {
		c.DC.DrawRoundedRectangle(ax, ay, w, h, radius)
	}
	_ = c.DC.Fill()
}

// FillCircle fills a circle centered at local logical (cx, cy).
func (c *Context) FillCircle(cx, cy, radius, r, g, b, a float64) {
	if c == nil || c.DC == nil || radius <= 0 {
		return
	}
	c.DC.SetRGBA(r, g, b, a)
	c.DC.DrawCircle(c.OriginX+cx, c.OriginY+cy, radius)
	_ = c.DC.Fill()
}

// StrokeCircle strokes a circle centered at local logical (cx, cy).
func (c *Context) StrokeCircle(cx, cy, radius, lineWidth, r, g, b, a float64) {
	if c == nil || c.DC == nil || radius <= 0 {
		return
	}
	c.applyStroke(lineWidth, r, g, b, a)
	c.DC.DrawCircle(c.OriginX+cx, c.OriginY+cy, radius)
	_ = c.DC.Stroke()
}
