package rendering

// RenderAlignBox is a Flutter-style Align: it fills the incoming constraints
// and positions its child by fractional alignment of the leftover space
// (Flutter FractionalOffset semantics). Positioning is layout-driven — on
// parent resize the child offset is recomputed by Layout, so animated blocks
// follow the parent size without imperative coordinates or clamping.
type RenderAlignBox struct {
	Base
	// AlignX/AlignY are fractional alignment in 0..1 (0 = left/top,
	// 0.5 = center, 1 = right/bottom).
	AlignX, AlignY float64
	// FixedWidth/Height 0 = fill parent max constraints.
	FixedWidth, FixedHeight float64

	child RenderObject
}

// NewRenderAlignBox builds an align box with one child. The child is laid out
// with loose constraints and offset by the fractional alignment.
func NewRenderAlignBox(child RenderObject, ax, ay float64) *RenderAlignBox {
	a := &RenderAlignBox{AlignX: ax, AlignY: ay, child: child}
	a.Init(a)
	if child != nil {
		a.AddChild(child)
	}
	return a
}

// SetAlignment changes the fractional alignment and dirties layout so the
// child offset is recomputed on the next layout pass. The child is also
// marked for repaint (its pixels move).
func (a *RenderAlignBox) SetAlignment(ax, ay float64) {
	if a == nil {
		return
	}
	if a.AlignX == ax && a.AlignY == ay {
		return
	}
	a.AlignX, a.AlignY = ax, ay
	a.MarkNeedsLayout()
	if a.child != nil {
		a.child.MarkNeedsPaint()
	}
}

// Layout implements RenderObject: size from constraints, child laid out loose,
// child offset = leftover space * alignment.
func (a *RenderAlignBox) Layout(c Constraints) Size {
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
	if a.child != nil {
		inner := Constraints{
			MinWidth: 0, MaxWidth: out.Width,
			MinHeight: 0, MaxHeight: out.Height,
		}
		sz := a.child.Layout(inner)
		a.child.SetOffset(Point{
			X: (out.Width - sz.Width) * a.AlignX,
			Y: (out.Height - sz.Height) * a.AlignY,
		})
	}
	a.RememberConstraints(c)
	a.clearLayoutDirty()
	return out
}

// Paint implements RenderObject: only the child is drawn (no own content).
func (a *RenderAlignBox) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !a.NeedsPaint() && !SubtreeNeedsPaint(a) {
		return
	}
	pc.NotePaintVisit()
	if a.child != nil {
		off := a.child.Offset()
		a.child.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
	}
	a.clearPaintDirty()
}

// HitTest implements RenderObject (child offset relative to this box).
func (a *RenderAlignBox) HitTest(p Point) RenderObject {
	if a.child != nil {
		off := a.child.Offset()
		if hit := a.child.HitTest(Point{X: p.X - off.X, Y: p.Y - off.Y}); hit != nil {
			return hit
		}
	}
	return nil
}
