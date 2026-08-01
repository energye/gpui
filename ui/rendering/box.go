package rendering

// RenderBox is a minimal box with optional fixed size and children stacked from top-left.
// Y-down: first child at (pad, pad), subsequent children not auto-positioned (caller SetOffset).
type RenderBox struct {
	Base
	// FixedWidth/Height if >0 force preferred size before constraints.
	FixedWidth, FixedHeight float64
	// Pad is logical padding when laying out a single expanding child.
	Pad float64
	// OnPaint optional custom paint under children.
	OnPaint func(pc *PaintContext, size Size)
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
	if out != b.size {
		// Size changed → own content (OnPaint bg/border) must repaint even under
		// CompositeOnly (bubbled-only dirt would otherwise skip it).
		b.MarkNeedsPaint()
	}
	b.setSize(out)
	b.RememberConstraints(c)
	b.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
//
// CompositeOnly rules (Flutter-like):
//  1. Skip node if neither it nor any descendant needs paint.
//  2. If this node NeedsPaint: draw self and all children except clean RepaintBoundaries
//     (scroll/offset dirties parent → children must redraw).
//  3. If only descendants need paint: do not draw self; recurse only into dirty paths.
func (b *RenderBox) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !b.NeedsPaint() && !SubtreeNeedsPaint(b) {
		return
	}
	pc.NotePaintVisit()
	// Own-content dirt only: bubbled descendant dirt must not re-run OnPaint
	// (same R4 static-loss guard as AbsoluteBox.Paint).
	paintSelf := !pc.CompositeOnly || b.NeedsPaintSelf()
	if paintSelf {
		if b.OnPaint != nil {
			b.OnPaint(pc, b.size)
		}
		for _, ch := range b.children {
			off := ch.Offset()
			if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
				continue
			}
			ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
		}
		b.clearPaintDirty()
		return
	}
	// Descend only into dirty subtrees / dirty boundaries.
	for _, ch := range b.children {
		if !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
			continue
		}
		off := ch.Offset()
		ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
	// Walked every dirty path; clear the bubbled flag so steady frames early-out.
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
// If Paint is invoked, always draw — CompositeOnly skipping is the caller's job
// (parent omits clean RepaintBoundary children). Leaves must redraw when a
// scrolling ancestor repaints.
//
// W1: with UseBoundaryCache, a clean RepaintBoundary ColorBox Replays its
// Picture instead of re-recording (boundary_skip); dirty path re-records.
func (c *RenderColorBox) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.BoundaryCache != nil && pc.BoundaryCache.tryReplay(pc, c) {
		return // clean Replay is not a "repaint" for R12b overlay
	}
	pc.NotePaintVisit()
	sz := c.size
	w, h := sz.Width, sz.Height
	if w <= 0 {
		w = c.Width
	}
	if h <= 0 {
		h = c.Height
	}
	fillRect(pc, 0, 0, w, h, c.R, c.G, c.B, c.A)
	// R12b: mark live re-paints (dirty path / first record), not cache hits.
	pc.NoteDebugRepaint(w, h)
	c.clearPaintDirty()
	if pc.BoundaryCache != nil && c.IsRepaintBoundary() {
		pc.BoundaryCache.storeColorBox(pc, c)
	}
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
