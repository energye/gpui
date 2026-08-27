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
	b.setSize(out)
	b.RememberConstraints(c)
	b.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
//
// Flutter PaintingContext paintChild walk (FPC-COMPOSITE-ONLY, FRO-REPAINT-B):
//  1. Skip node entirely if neither it nor any descendant needs paint.
//  2. If this node NeedsPaint: draw self and recurse into ALL children that
//     are not clean RepaintBoundaries (a dirty ancestor repaints its
//     non-isolated children; clean boundaries replay/skip).
//  3. If only descendants need paint (self clean): do not draw self; recurse
//     only into dirty paths (paint isolation — sibling subtrees untouched).
//
// The walk never re-paints a clean RepaintBoundary subtree unless the
// boundary itself is dirty, so paint_count/visits stay explainable (R2).
func (b *RenderBox) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !b.NeedsPaint() && !SubtreeNeedsPaint(b) {
		return
	}
	pc.NotePaintVisit()
	if !pc.CompositeOnly || b.NeedsPaint() {
		// Self dirty (or full paint): draw self, then children.
		if b.OnPaint != nil {
			if pc.DC != nil {
				// OnPaint is a leaf drawing callback: isolate its DC state
				// (CTM/clip) so a Transform/Translate inside the callback does
				// not leak into sibling paints drawn afterwards (R5 picture
				// replay regression: Translate leaked to grid + labels).
				pc.DC.Push()
			}
			b.OnPaint(pc, b.size)
			if pc.DC != nil {
				pc.DC.Pop()
			}
		}
		b.paintChildren(pc)
		b.clearPaintDirty()
		return
	}
	// Self clean, descendants dirty: descend only into dirty paths.
	b.paintDirtyDescendants(pc)
}

// paintChildren paints every child; clean RepaintBoundary children are
// skipped when in CompositeOnly mode (their Picture replays elsewhere).
func (b *RenderBox) paintChildren(pc *PaintContext) {
	for _, ch := range b.children {
		off := ch.Offset()
		if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
			continue
		}
		ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
}

// paintDirtyDescendants recurses only into paths that actually need paint.
func (b *RenderBox) paintDirtyDescendants(pc *PaintContext) {
	for _, ch := range b.children {
		if !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
			continue
		}
		off := ch.Offset()
		ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
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

// MoveTo repositions the box and dirties paint — the runtime-safe way to
// move an absolutely-placed child (Base.SetOffset writes the field only;
// without a dirty flag retained/CompositeOnly frames never repaint it).
func (c *RenderColorBox) MoveTo(x, y float64) {
	c.SetOffset(Point{X: x, Y: y})
	c.MarkNeedsPaint()
}

// SetAlpha updates opacity and dirties paint (same rationale as MoveTo).
func (c *RenderColorBox) SetAlpha(a float64) {
	if c.A == a {
		return
	}
	c.A = a
	c.MarkNeedsPaint()
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
