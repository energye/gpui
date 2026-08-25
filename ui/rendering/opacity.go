package rendering

// RenderOpacity applies a group opacity to its children during paint
// (Flutter Opacity / OpacityLayer subset): SaveLayer(bounds, opacity) around
// the child walk so overlapping children blend as one group instead of each
// child fading against the backdrop. Maps to scene.OpacityLayer when building
// the retained layer tree.
//
// SetOpacity is paint-only (MarkNeedsPaint); HitTest is opacity-blind — a
// faded subtree still receives hits (Flutter semantics).
type RenderOpacity struct {
	Base
	FixedWidth, FixedHeight float64
	Opacity                 float64 // 0..1; out-of-range clamped at use sites
}

// NewRenderOpacity wraps children with the given group opacity.
func NewRenderOpacity(opacity float64, children ...RenderObject) *RenderOpacity {
	t := &RenderOpacity{Opacity: clampUnit(opacity)}
	t.Init(t)
	for _, c := range children {
		t.AddChild(c)
	}
	return t
}

// SetOpacity updates group opacity (paint-only).
func (t *RenderOpacity) SetOpacity(opacity float64) {
	if t == nil {
		return
	}
	op := clampUnit(opacity)
	if op == t.Opacity {
		return
	}
	t.Opacity = op
	t.MarkNeedsPaint()
}

// OpacityParams returns the clamped group opacity for scene.OpacityLayer.
func (t *RenderOpacity) OpacityParams() float64 {
	if t == nil {
		return 1
	}
	return clampUnit(t.Opacity)
}

func clampUnit(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// Layout sizes to fixed or union of children (same model as RenderTransform /
// RenderClipRRect: children get loose constraints, node takes its own pref).
func (t *RenderOpacity) Layout(c Constraints) Size {
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

// Paint isolates children in an offscreen layer composited back at Opacity
// (group semantics). op>=1 paints through with no layer (common case); op<=0
// skips the subtree entirely — nothing is painted, matching scene composite.
func (t *RenderOpacity) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() && !SubtreeNeedsPaint(t) {
		return
	}
	pc.NotePaintVisit()
	op := t.OpacityParams()
	if op <= 0 {
		t.clearPaintDirty()
		return
	}
	sz := t.size
	groupW, groupH := sz.Width, sz.Height
	if groupW <= 0 {
		groupW = t.FixedWidth
	}
	if groupH <= 0 {
		groupH = t.FixedHeight
	}
	childPC := pc.WithOrigin(pc.OriginX, pc.OriginY)
	pushed := false
	if op < 1 && pc.DC != nil && groupW > 0 && groupH > 0 {
		pushed = pc.SaveLayer(groupW, groupH, op)
	}
	for _, ch := range t.children {
		off := ch.Offset()
		ch.Paint(childPC.WithOrigin(childPC.OriginX+off.X, childPC.OriginY+off.Y))
	}
	if pushed {
		pc.RestoreLayer()
	}
	t.clearPaintDirty()
}

// HitTest is opacity-blind: a faded subtree still receives hits (Flutter
// semantics), containment follows the laid-out box.
func (t *RenderOpacity) HitTest(p Point) RenderObject {
	if t == nil {
		return nil
	}
	sz := t.size
	if p.X < 0 || p.Y < 0 || p.X >= sz.Width || p.Y >= sz.Height {
		return nil
	}
	for i := len(t.children) - 1; i >= 0; i-- {
		ch := t.children[i]
		off := ch.Offset()
		if hit := ch.HitTest(Point{X: p.X - off.X, Y: p.Y - off.Y}); hit != nil {
			return hit
		}
	}
	return t
}
