package rendering

// RenderClipRRect clips its children to a rounded rect (uniform corner radius).
// Paint path uses PaintContext.PushClipRRect; retained path emits scene.ClipRRectLayer
// via BuildLayerTree (Flutter ClipRRectLayer / pushClipRRect layer role).
//
// Radius <= 0 falls back to a hard rect clip on the paint path (same as PushClipRRect).
// Does not claim per-corner radii, ClipOp.difference, or ClipPath.
type RenderClipRRect struct {
	Base
	FixedWidth, FixedHeight float64
	Radius                  float64
}

// NewRenderClipRRect wraps children under a rounded-rect clip.
func NewRenderClipRRect(children ...RenderObject) *RenderClipRRect {
	c := &RenderClipRRect{}
	c.Init(c)
	for _, ch := range children {
		c.AddChild(ch)
	}
	return c
}

// SetRadius updates the uniform corner radius (paint + layer params).
func (c *RenderClipRRect) SetRadius(radius float64) {
	if c == nil {
		return
	}
	c.Radius = radius
	c.MarkNeedsPaint()
}

// ClipRRectParams returns local clip bounds (0,0,w,h) and radius for scene.ClipRRectLayer.
// Width/height come from the laid-out size (or fixed prefs before layout).
func (c *RenderClipRRect) ClipRRectParams() (w, h, radius float64) {
	if c == nil {
		return 0, 0, 0
	}
	w, h = c.size.Width, c.size.Height
	if w <= 0 {
		w = c.FixedWidth
	}
	if h <= 0 {
		h = c.FixedHeight
	}
	return w, h, c.Radius
}

// Layout sizes to fixed or union of children (same model as RenderTransform).
func (c *RenderClipRRect) Layout(cons Constraints) Size {
	if sz, ok := c.LayoutSkipIfClean(cons); ok {
		return sz
	}
	inner := Constraints{MinWidth: 0, MaxWidth: cons.MaxWidth, MinHeight: 0, MaxHeight: cons.MaxHeight}
	var maxW, maxH float64
	for _, ch := range c.children {
		sz := ch.Layout(inner)
		ch.SetOffset(Point{})
		if sz.Width > maxW {
			maxW = sz.Width
		}
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}
	pref := Size{Width: c.FixedWidth, Height: c.FixedHeight}
	if pref.Width <= 0 {
		pref.Width = maxW
	}
	if pref.Height <= 0 {
		pref.Height = maxH
	}
	out := cons.Tighten(pref)
	c.setSize(out)
	c.RememberConstraints(cons)
	c.clearLayoutDirty()
	return out
}

// Paint applies rounded clip then paints children.
func (c *RenderClipRRect) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !c.NeedsPaint() && !SubtreeNeedsPaint(c) {
		return
	}
	pc.NotePaintVisit()
	if !(!pc.CompositeOnly || c.NeedsPaint() || SubtreeNeedsPaint(c)) {
		return
	}
	sz := c.size
	pushed := pc.DC != nil && sz.Width > 0 && sz.Height > 0
	if pushed {
		pc.PushClipRRect(0, 0, sz.Width, sz.Height, c.Radius)
	}
	for _, ch := range c.children {
		if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
			continue
		}
		off := ch.Offset()
		ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
	if pushed {
		pc.PopClip()
	}
	c.clearPaintDirty()
}

// HitTest uses the un-rounded AABB (corner holes still hit for now).
func (c *RenderClipRRect) HitTest(p Point) RenderObject {
	sz := c.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		for i := len(c.children) - 1; i >= 0; i-- {
			ch := c.children[i]
			off := ch.Offset()
			if hit := ch.HitTest(Point{X: p.X - off.X, Y: p.Y - off.Y}); hit != nil {
				return hit
			}
		}
		return c
	}
	return nil
}
