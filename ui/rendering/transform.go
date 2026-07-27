package rendering

// RenderTransform applies a 2D transform around its children during paint
// (translate to center → rotate → scale → translate back).
// Maps to scene.TransformLayer when building the retained layer tree.
//
// Rotation is radians (Y-down canvas, matches render.Context.Rotate).
// Does not claim Matrix4 / perspective or inverse hit-testing.
type RenderTransform struct {
	Base
	FixedWidth, FixedHeight float64
	Rotation                float64
	SX, SY                  float64 // 0 treated as 1
}

// NewRenderTransform wraps children; default scale is 1,1.
func NewRenderTransform(children ...RenderObject) *RenderTransform {
	t := &RenderTransform{SX: 1, SY: 1}
	t.Init(t)
	for _, c := range children {
		t.AddChild(c)
	}
	return t
}

// SetRotation updates rotation (paint-only).
func (t *RenderTransform) SetRotation(rad float64) {
	if t == nil {
		return
	}
	t.Rotation = rad
	t.MarkNeedsPaint()
}

// SetScale updates scale (paint-only).
func (t *RenderTransform) SetScale(sx, sy float64) {
	if t == nil {
		return
	}
	t.SX, t.SY = sx, sy
	t.MarkNeedsPaint()
}

func (t *RenderTransform) effectiveScale() (sx, sy float64) {
	sx, sy = t.SX, t.SY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return sx, sy
}

// Layout sizes to fixed or union of children.
func (t *RenderTransform) Layout(c Constraints) Size {
	if sz, ok := t.LayoutSkipIfClean(c); ok {
		return sz
	}
	inner := Constraints{MinWidth: 0, MaxWidth: c.MaxWidth, MinHeight: 0, MaxHeight: c.MaxHeight}
	var maxW, maxH float64
	for _, ch := range t.children {
		sz := ch.Layout(inner)
		ch.SetOffset(Point{})
		if sz.Width > maxW {
			maxW = sz.Width
		}
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}
	pref := Size{Width: t.FixedWidth, Height: t.FixedHeight}
	if pref.Width <= 0 {
		pref.Width = maxW
	}
	if pref.Height <= 0 {
		pref.Height = maxH
	}
	out := c.Tighten(pref)
	t.setSize(out)
	t.RememberConstraints(c)
	t.clearLayoutDirty()
	return out
}

// Paint applies DC transform then paints children.
func (t *RenderTransform) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() && !SubtreeNeedsPaint(t) {
		return
	}
	pc.NotePaintVisit()
	if !(!pc.CompositeOnly || t.NeedsPaint() || SubtreeNeedsPaint(t)) {
		return
	}
	sx, sy := t.effectiveScale()
	cx := t.size.Width * 0.5
	cy := t.size.Height * 0.5
	if pc.DC != nil {
		pc.DC.Push()
		pc.DC.Translate(pc.OriginX+cx, pc.OriginY+cy)
		if t.Rotation != 0 {
			pc.DC.Rotate(t.Rotation)
		}
		if sx != 1 || sy != 1 {
			pc.DC.Scale(sx, sy)
		}
		pc.DC.Translate(-cx, -cy)
		childPC := &PaintContext{
			DC:          pc.DC,
			OriginX:     0,
			OriginY:     0,
			Scale:       pc.Scale,
			PaintVisits: pc.PaintVisits,
		}
		for _, ch := range t.children {
			if pc.CompositeOnly && ch.IsRepaintBoundary() && !ch.NeedsPaint() && !SubtreeNeedsPaint(ch) {
				continue
			}
			off := ch.Offset()
			ch.Paint(childPC.WithOrigin(off.X, off.Y))
		}
		pc.DC.Pop()
	} else {
		for _, ch := range t.children {
			off := ch.Offset()
			ch.Paint(pc.WithOrigin(pc.OriginX+off.X, pc.OriginY+off.Y))
		}
	}
	t.clearPaintDirty()
}

// HitTest uses the untransformed AABB (inverse CTM later).
func (t *RenderTransform) HitTest(p Point) RenderObject {
	sz := t.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		for i := len(t.children) - 1; i >= 0; i-- {
			ch := t.children[i]
			off := ch.Offset()
			if hit := ch.HitTest(Point{X: p.X - off.X, Y: p.Y - off.Y}); hit != nil {
				return hit
			}
		}
		return t
	}
	return nil
}

// TransformParams for scene.TransformLayer (TX/TY come from RO offset).
func (t *RenderTransform) TransformParams() (rot, sx, sy float64) {
	if t == nil {
		return 0, 1, 1
	}
	sx, sy = t.effectiveScale()
	return t.Rotation, sx, sy
}
