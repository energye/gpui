package painting

import "github.com/energye/gpui/render"

// StrokeStyle is a thin UI-facing stroke configuration (logical px).
// Maps to render line width / cap / join for subsequent Stroke* calls.
type StrokeStyle struct {
	Width float64
	// Cap: 0=butt, 1=round, 2=square (matches render.LineCap iota).
	Cap render.LineCap
	// Join: 0=miter, 1=round, 2=bevel (matches render.LineJoin iota).
	Join render.LineJoin
}

// DefaultStrokeStyle returns a 1px butt/miter stroke.
func DefaultStrokeStyle() StrokeStyle {
	return StrokeStyle{Width: 1, Cap: render.LineCapButt, Join: render.LineJoinMiter}
}

// SetStrokeStyle applies width/cap/join on the underlying DC (no-op if nil).
func (c *Context) SetStrokeStyle(s StrokeStyle) {
	if c == nil || c.DC == nil {
		return
	}
	w := s.Width
	if w <= 0 {
		w = 1
	}
	c.DC.SetLineWidth(w)
	c.DC.SetLineCap(s.Cap)
	c.DC.SetLineJoin(s.Join)
}

// applyStrokeColor sets stroke color then style if width>0.
func (c *Context) applyStroke(width, r, g, b, a float64) {
	if width <= 0 {
		width = 1
	}
	c.DC.SetRGBA(r, g, b, a)
	c.DC.SetLineWidth(width)
}

// StrokeRect strokes an axis-aligned rectangle in local logical coordinates.
func (c *Context) StrokeRect(x, y, w, h, lineWidth, r, g, b, a float64) {
	if c == nil || c.DC == nil || w <= 0 || h <= 0 {
		return
	}
	ax, ay := c.OriginX+x, c.OriginY+y
	c.applyStroke(lineWidth, r, g, b, a)
	c.DC.DrawRectangle(ax, ay, w, h)
	_ = c.DC.Stroke()
}

// StrokeRoundRect strokes a rounded rectangle in local logical coordinates.
func (c *Context) StrokeRoundRect(x, y, w, h, radius, lineWidth, r, g, b, a float64) {
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
	ax, ay := c.OriginX+x, c.OriginY+y
	c.applyStroke(lineWidth, r, g, b, a)
	if radius <= 0 {
		c.DC.DrawRectangle(ax, ay, w, h)
	} else {
		c.DC.DrawRoundedRectangle(ax, ay, w, h, radius)
	}
	_ = c.DC.Stroke()
}

// StrokeLine strokes a line segment in local logical coordinates.
func (c *Context) StrokeLine(x1, y1, x2, y2, lineWidth, r, g, b, a float64) {
	if c == nil || c.DC == nil {
		return
	}
	c.applyStroke(lineWidth, r, g, b, a)
	c.DC.DrawLine(c.OriginX+x1, c.OriginY+y1, c.OriginX+x2, c.OriginY+y2)
	_ = c.DC.Stroke()
}
