package rendering

import "github.com/energye/gpui/ui/painting"

// RenderBox is a minimal box with optional fixed size and children stacked from top-left.
// Y-down: first child at (pad, pad), subsequent children not auto-positioned (caller SetOffset).
type RenderBox struct {
	Base
	// FixedWidth/Height if >0 force preferred size before constraints.
	FixedWidth, FixedHeight float64
	// Pad is logical padding when laying out a single expanding child.
	Pad float64
	// OnPaint optional custom paint under children.
	OnPaint func(pc *painting.Context, size Size)
}

// NewRenderBox constructs a box.
func NewRenderBox(children ...RenderObject) *RenderBox {
	b := &RenderBox{}
	b.Init(b)
	for _, c := range children {
		b.AddChild(c)
	}
	return b
}

// Layout implements RenderObject.
func (b *RenderBox) Layout(c Constraints) Size {
	if sz, ok := b.LayoutSkipIfClean(c); ok {
		return sz
	}
	pref := Size{Width: b.FixedWidth, Height: b.FixedHeight}
	// Layout children with loosened inner constraints.
	inner := Constraints{
		MinWidth:  0,
		MaxWidth:  c.MaxWidth,
		MinHeight: 0,
		MaxHeight: c.MaxHeight,
	}
	if b.Pad > 0 && c.MaxWidth < Unbounded/2 {
		inner.MaxWidth = c.MaxWidth - 2*b.Pad
		if inner.MaxWidth < 0 {
			inner.MaxWidth = 0
		}
	}
	if b.Pad > 0 && c.MaxHeight < Unbounded/2 {
		inner.MaxHeight = c.MaxHeight - 2*b.Pad
		if inner.MaxHeight < 0 {
			inner.MaxHeight = 0
		}
	}
	var maxW, maxH float64
	for _, ch := range b.children {
		sz := ch.Layout(inner)
		// Default offset: padding top-left (Y-down).
		ch.SetOffset(Point{X: b.Pad, Y: b.Pad})
		if sz.Width > maxW {
			maxW = sz.Width
		}
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}
	if pref.Width <= 0 {
		pref.Width = maxW + 2*b.Pad
	}
	if pref.Height <= 0 {
		pref.Height = maxH + 2*b.Pad
	}
	out := c.Tighten(pref)
	b.setSize(out)
	b.RememberConstraints(c)
	b.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
func (b *RenderBox) Paint(pc *painting.Context) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !b.NeedsPaint() && !SubtreeNeedsPaint(b) {
		return
	}
	pc.NotePaintVisit()
	if b.OnPaint != nil {
		b.OnPaint(pc, b.size)
	}
	for _, ch := range b.children {
		off := ch.Offset()
		// Skip clean non-boundary children under CompositeOnly.
		if pc.CompositeOnly && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
			continue
		}
		ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
	b.clearPaintDirty()
}

// HitTest implements RenderObject (Y-down local coords relative to this box).
func (b *RenderBox) HitTest(p Point) RenderObject {
	// Children front-to-back reverse order.
	kids := b.children
	for i := len(kids) - 1; i >= 0; i-- {
		ch := kids[i]
		off := ch.Offset()
		local := Point{X: p.X - off.X, Y: p.Y - off.Y}
		if hit := ch.HitTest(local); hit != nil {
			return hit
		}
	}
	sz := b.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return b
	}
	return nil
}

// RenderColorBox is a solid color rectangle (logical size).
type RenderColorBox struct {
	Base
	Width, Height float64
	R, G, B, A    float64
}

// NewRenderColorBox creates a colored box; A defaults to 1 if all zero alpha with color.
func NewRenderColorBox(w, h, r, g, b, a float64) *RenderColorBox {
	c := &RenderColorBox{Width: w, Height: h, R: r, G: g, B: b, A: a}
	if c.A == 0 && (r != 0 || g != 0 || b != 0) {
		c.A = 1
	}
	c.Init(c)
	return c
}

// Layout implements RenderObject.
func (c *RenderColorBox) Layout(cons Constraints) Size {
	if sz, ok := c.LayoutSkipIfClean(cons); ok {
		return sz
	}
	out := cons.Tighten(Size{Width: c.Width, Height: c.Height})
	c.setSize(out)
	c.RememberConstraints(cons)
	c.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
func (c *RenderColorBox) Paint(pc *painting.Context) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !c.NeedsPaint() {
		return
	}
	pc.NotePaintVisit()
	sz := c.size
	pc.FillRect(0, 0, sz.Width, sz.Height, c.R, c.G, c.B, c.A)
	c.clearPaintDirty()
}

// SubtreeNeedsPaint reports whether n or any descendant needs paint.
func SubtreeNeedsPaint(n RenderObject) bool {
	if n == nil {
		return false
	}
	if n.NeedsPaint() {
		return true
	}
	for _, c := range n.Children() {
		if SubtreeNeedsPaint(c) {
			return true
		}
	}
	return false
}

// HitTest implements RenderObject.
func (c *RenderColorBox) HitTest(p Point) RenderObject {
	sz := c.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return c
	}
	return nil
}
