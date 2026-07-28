package rendering

import "math"

// RenderTransform applies a 2D transform around its children during paint
// (translate to center → rotate → scale → translate back).
// Maps to scene.TransformLayer when building the retained layer tree.
//
// Rotation is radians (Y-down canvas, matches render.Context.Rotate).
// HitTest applies the inverse of the same center-based rotate+scale (not pure AABB).
// Does not claim Matrix4 / perspective or nested arbitrary CTM stacks.
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

// HitTest maps p through the inverse of the paint CTM, then hits children in
// local space. Paint order is T(c)·R(θ)·S·T(-c); inverse is T(c)·S⁻¹·R(-θ)·T(-c).
// A parent-local point inside the untransformed AABB can miss after rotation/scale,
// and a point outside that AABB can still hit the painted geometry.
func (t *RenderTransform) HitTest(p Point) RenderObject {
	if t == nil {
		return nil
	}
	local := t.inverseMapPoint(p)
	sz := t.size
	// After inverse, only the local content box (and children) are hittable.
	if local.X < 0 || local.Y < 0 || local.X >= sz.Width || local.Y >= sz.Height {
		return nil
	}
	for i := len(t.children) - 1; i >= 0; i-- {
		ch := t.children[i]
		off := ch.Offset()
		if hit := ch.HitTest(Point{X: local.X - off.X, Y: local.Y - off.Y}); hit != nil {
			return hit
		}
	}
	return t
}

// inverseMapPoint applies the inverse of the paint transform to a parent-local point.
// Identity (rot=0, scale=1) returns p unchanged.
func (t *RenderTransform) inverseMapPoint(p Point) Point {
	sx, sy := t.effectiveScale()
	cx := t.size.Width * 0.5
	cy := t.size.Height * 0.5
	// 1) T(-c)
	x := p.X - cx
	y := p.Y - cy
	// 2) R(-θ)
	if t.Rotation != 0 {
		c := math.Cos(t.Rotation)
		s := math.Sin(t.Rotation)
		// R(-θ): [c s; -s c] applied to (x,y)  (same as transpose of R(θ))
		x, y = x*c+y*s, -x*s+y*c
	}
	// 3) S⁻¹
	if sx != 1 {
		if sx == 0 {
			return Point{X: math.Inf(1), Y: math.Inf(1)} // degenerate → miss
		}
		x /= sx
	}
	if sy != 1 {
		if sy == 0 {
			return Point{X: math.Inf(1), Y: math.Inf(1)}
		}
		y /= sy
	}
	// 4) T(c)
	return Point{X: x + cx, Y: y + cy}
}

// TransformParams for scene.TransformLayer (TX/TY come from RO offset).
func (t *RenderTransform) TransformParams() (rot, sx, sy float64) {
	if t == nil {
		return 0, 1, 1
	}
	sx, sy = t.effectiveScale()
	return t.Rotation, sx, sy
}
