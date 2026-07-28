package rendering

// AbsoluteBox is a fixed-size container that **preserves** each child's Offset
// during Layout (unlike RenderBox, which resets children to Pad).
//
// Use for HUD / demo shells where widgets are placed at absolute logical
// coordinates inside the window.
type AbsoluteBox struct {
	Base
	FixedWidth, FixedHeight float64
	// Background optional solid fill under children (nil = transparent).
	Background *Color
}

// Color is a simple RGBA for AbsoluteBox background.
type Color struct {
	R, G, B, A float64
}

// NewAbsoluteBox creates an absolute container of the given size.
func NewAbsoluteBox(w, h float64) *AbsoluteBox {
	a := &AbsoluteBox{FixedWidth: w, FixedHeight: h}
	a.Init(a)
	return a
}

// Place adds child (if not already a child) and sets its offset.
func (a *AbsoluteBox) Place(child RenderObject, x, y float64) {
	if a == nil || child == nil {
		return
	}
	found := false
	for _, c := range a.children {
		if c == child {
			found = true
			break
		}
	}
	if !found {
		a.AddChild(child)
	}
	child.SetOffset(Point{X: x, Y: y})
}

// Layout implements RenderObject: size from Fixed*; layout each child loosely
// without clobbering Offset.
func (a *AbsoluteBox) Layout(c Constraints) Size {
	if sz, ok := a.LayoutSkipIfClean(c); ok {
		return sz
	}
	pref := Size{Width: a.FixedWidth, Height: a.FixedHeight}
	if pref.Width <= 0 {
		pref.Width = c.MaxWidth
	}
	if pref.Height <= 0 {
		pref.Height = c.MaxHeight
	}
	out := c.Tighten(pref)
	a.setSize(out)
	inner := Constraints{
		MinWidth: 0, MaxWidth: out.Width,
		MinHeight: 0, MaxHeight: out.Height,
	}
	for _, ch := range a.children {
		off := ch.Offset()
		_ = ch.Layout(inner)
		ch.SetOffset(off) // critical: keep absolute placement
	}
	a.RememberConstraints(c)
	a.clearLayoutDirty()
	return out
}

// Paint implements RenderObject.
func (a *AbsoluteBox) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	// W1: fully-clean AbsoluteBox repaint boundary may Replay a recorded Picture.
	if pc.BoundaryCache != nil && a.IsRepaintBoundary() && pc.BoundaryCache.tryReplay(pc, a) {
		return
	}
	if pc.CompositeOnly && !a.NeedsPaint() && !SubtreeNeedsPaint(a) {
		return
	}
	pc.NotePaintVisit()
	selfDirty := a.NeedsPaint()
	paintSelf := !pc.CompositeOnly || selfDirty
	if paintSelf {
		if a.Background != nil {
			bg := a.Background
			fillRect(pc, 0, 0, a.size.Width, a.size.Height, bg.R, bg.G, bg.B, bg.A)
		}
		for _, ch := range a.children {
			off := ch.Offset()
			if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
				continue
			}
			ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
		}
		a.clearPaintDirty()
		// Re-store outer Picture only when this boundary itself was dirty or had no entry.
		// Inner-only dirty must not bump outer boundary_rerecord (R3 nested rule).
		if pc.BoundaryCache != nil && a.IsRepaintBoundary() && (selfDirty || !pc.BoundaryCache.HasValid(a)) {
			pc.BoundaryCache.storeAbsoluteColorChildren(pc, a)
		}
		return
	}
	// Only descendants dirty: walk dirty paths; do not re-store outer cache.
	// CompositeOnly: skip fully-clean children (including clean boundaries).
	// FullPaint never takes this branch (paintSelf is always true when !CompositeOnly).
	for _, ch := range a.children {
		if !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
			continue
		}
		off := ch.Offset()
		ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
}

// HitTest implements RenderObject (children front-to-back reverse).
func (a *AbsoluteBox) HitTest(p Point) RenderObject {
	kids := a.children
	for i := len(kids) - 1; i >= 0; i-- {
		ch := kids[i]
		off := ch.Offset()
		local := Point{X: p.X - off.X, Y: p.Y - off.Y}
		if hit := ch.HitTest(local); hit != nil {
			return hit
		}
	}
	sz := a.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return a
	}
	return nil
}
