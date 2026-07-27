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
