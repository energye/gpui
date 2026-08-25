package rendering

// Subtree filter ROs (Flutter ColorFiltered / ImageFiltered subset): paint
// isolates children in a true offscreen layer (PushLayerIsolated via
// SaveLayer) so the filter hits only the subtree; the retained path maps to
// scene.ColorFilterLayer / scene.ImageFilterLayer in BuildLayerTree.
//
// SetMatrix / SetBlurRadius are paint-only (MarkNeedsPaint). HitTest is
// filter-blind — filters change pixels, not geometry or hit targets (Flutter
// semantics).

// RenderColorFilter applies a 4×5 row-major color matrix to descendants.
type RenderColorFilter struct {
	Base
	FixedWidth, FixedHeight float64
	Matrix                  [20]float32
}

// NewRenderColorFilter wraps children with the given color matrix.
func NewRenderColorFilter(matrix [20]float32, children ...RenderObject) *RenderColorFilter {
	t := &RenderColorFilter{Matrix: matrix}
	t.Init(t)
	for _, c := range children {
		t.AddChild(c)
	}
	return t
}

// NewRenderGrayscale wraps children with the standard BT.601 grayscale matrix.
func NewRenderGrayscale(children ...RenderObject) *RenderColorFilter {
	return NewRenderColorFilter([20]float32{
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0, 0, 0, 1, 0,
	}, children...)
}

// SetMatrix updates the subtree color matrix (paint-only).
func (t *RenderColorFilter) SetMatrix(matrix [20]float32) {
	if t == nil {
		return
	}
	if t.Matrix == matrix {
		return
	}
	t.Matrix = matrix
	t.MarkNeedsPaint()
}

// ColorMatrix returns the active matrix for scene.ColorFilterLayer.
func (t *RenderColorFilter) ColorMatrix() [20]float32 {
	if t == nil {
		var id [20]float32
		id[0], id[6], id[12], id[18] = 1, 1, 1, 1
		return id
	}
	return t.Matrix
}

func (t *RenderColorFilter) isIdentity() bool {
	m := t.Matrix
	return m[0] == 1 && m[6] == 1 && m[12] == 1 && m[18] == 1 &&
		m[1] == 0 && m[2] == 0 && m[3] == 0 && m[4] == 0 &&
		m[5] == 0 && m[7] == 0 && m[8] == 0 && m[9] == 0 &&
		m[10] == 0 && m[11] == 0 && m[13] == 0 && m[14] == 0 &&
		m[15] == 0 && m[16] == 0 && m[17] == 0 && m[19] == 0
}

// Layout sizes to fixed or union of children (same model as RenderOpacity /
// RenderTransform: children get loose constraints, node takes its own pref).
func (t *RenderColorFilter) Layout(c Constraints) Size {
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

// Paint isolates children offscreen and applies the matrix on composite.
// Identity matrices paint through with no layer (steady-state trees stay
// unchanged), matching the retained-path layer omission in BuildLayerTree.
func (t *RenderColorFilter) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() && !SubtreeNeedsPaint(t) {
		return
	}
	pc.NotePaintVisit()
	childPC := pc.WithOrigin(pc.OriginX, pc.OriginY)
	pushed := false
	if !t.isIdentity() && pc.DC != nil {
		groupW, groupH := t.size.Width, t.size.Height
		if groupW <= 0 {
			groupW = t.FixedWidth
		}
		if groupH <= 0 {
			groupH = t.FixedHeight
		}
		if groupW > 0 && groupH > 0 {
			pushed = pc.SaveLayer(groupW, groupH, 1)
		}
	}
	for _, ch := range t.children {
		off := ch.Offset()
		ch.Paint(childPC.WithOrigin(childPC.OriginX+off.X, childPC.OriginY+off.Y))
	}
	if pushed {
		pc.DC.ApplyColorMatrix(t.Matrix)
		pc.RestoreLayer()
	}
	t.clearPaintDirty()
}

// HitTest is filter-blind: containment follows the laid-out box.
func (t *RenderColorFilter) HitTest(p Point) RenderObject {
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

// RenderImageFilter applies a uniform Gaussian blur to descendants.
type RenderImageFilter struct {
	Base
	FixedWidth, FixedHeight float64
	BlurRadius              float64
}

// NewRenderImageFilter wraps children with a uniform blur of the given radius.
func NewRenderImageFilter(blurRadius float64, children ...RenderObject) *RenderImageFilter {
	t := &RenderImageFilter{BlurRadius: blurRadius}
	t.Init(t)
	for _, c := range children {
		t.AddChild(c)
	}
	return t
}

// SetBlurRadius updates the subtree blur radius (paint-only).
func (t *RenderImageFilter) SetBlurRadius(r float64) {
	if t == nil {
		return
	}
	if r < 0 {
		r = 0
	}
	if t.BlurRadius == r {
		return
	}
	t.BlurRadius = r
	t.MarkNeedsPaint()
}

// BlurParams returns the clamped blur radius for scene.ImageFilterLayer.
func (t *RenderImageFilter) BlurParams() float64 {
	if t == nil || t.BlurRadius < 0 {
		return 0
	}
	return t.BlurRadius
}

// Layout sizes to fixed or union of children (same model as RenderOpacity).
func (t *RenderImageFilter) Layout(c Constraints) Size {
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

// Paint isolates children offscreen and blurs the group. Radius ≤ 0 paints
// through with no layer, matching the retained-path layer omission.
func (t *RenderImageFilter) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !t.NeedsPaint() && !SubtreeNeedsPaint(t) {
		return
	}
	pc.NotePaintVisit()
	childPC := pc.WithOrigin(pc.OriginX, pc.OriginY)
	pushed := false
	if t.BlurParams() > 0 && pc.DC != nil {
		groupW, groupH := t.size.Width, t.size.Height
		if groupW <= 0 {
			groupW = t.FixedWidth
		}
		if groupH <= 0 {
			groupH = t.FixedHeight
		}
		if groupW > 0 && groupH > 0 {
			pushed = pc.SaveLayer(groupW, groupH, 1)
		}
	}
	for _, ch := range t.children {
		off := ch.Offset()
		ch.Paint(childPC.WithOrigin(childPC.OriginX+off.X, childPC.OriginY+off.Y))
	}
	if pushed {
		pc.DC.ApplyBlur(t.BlurRadius)
		pc.RestoreLayer()
	}
	t.clearPaintDirty()
}

// HitTest is filter-blind: containment follows the laid-out box.
func (t *RenderImageFilter) HitTest(p Point) RenderObject {
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
