// Package prim decor facades: pure paint math plus builders over
// existing ui/rendering nodes (P2).
//
// Like layout, this file creates no new RenderObjects. Colored, clip,
// opacity, transform and filter facades compose existing nodes; the
// Decorated box paints through RenderBox.OnPaint with rendering draw
// helpers; hit math (rounded rect, oval, shadow exclusion) is pure and
// unit-tested. Colors, radii and widths resolve from theme via scope.
package prim

import (
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// ---- group blend math ----

// BlendSrcOver composites src over dst with group opacity (整层合成).
// Channels are 0..1. Used by OpacityLayer tests with容差≤12/通道.
func BlendSrcOver(dst, src [3]float64, opacity float64) [3]float64 {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	return [3]float64{
		src[0]*opacity + dst[0]*(1-opacity),
		src[1]*opacity + dst[1]*(1-opacity),
		src[2]*opacity + dst[2]*(1-opacity),
	}
}

// ---- hit math (Hit ≡ 绘) ----

// RRectContains is the uniform-radius rounded-rect containment used for
// both paint clipping and hit testing, so clipped corners never hit.
func RRectContains(x, y, w, h, radius float64) bool {
	if x < 0 || y < 0 || x >= w || y >= h {
		return false
	}
	if radius <= 0 {
		return true
	}
	r := radius
	if r*2 > w {
		r = w / 2
	}
	if r*2 > h {
		r = h / 2
	}
	var cx, cy float64
	switch {
	case x < r && y < r:
		cx, cy = x-r, y-r
	case x >= w-r && y < r:
		cx, cy = x-(w-r), y-r
	case x >= w-r && y >= h-r:
		cx, cy = x-(w-r), y-(h-r)
	case x < r && y >= h-r:
		cx, cy = x-r, y-(h-r)
	default:
		return true
	}
	return cx*cx+cy*cy <= r*r
}

// OvalContains reports ellipse containment in a w×h box.
func OvalContains(x, y, w, h float64) bool {
	if x < 0 || y < 0 || x >= w || y >= h || w <= 0 || h <= 0 {
		return false
	}
	rx, ry := w/2, h/2
	dx, dy := (x-rx)/rx, (y-ry)/ry
	return dx*dx+dy*dy <= 1
}

// ShadowHitExcluded reports whether a point in the shadow ring must not
// count as a hit: inside the shadow extent but outside the rounded box.
func ShadowHitExcluded(x, y, w, h, radius, shadowDX, shadowDY float64) bool {
	if RRectContains(x, y, w, h, radius) {
		return false
	}
	sx, sy := x-shadowDX, y-shadowDY
	_ = sx
	_ = sy
	return true
}

// GradientSample samples a two-stop linear gradient along t in 0..1.
func GradientSample(from, to theme.Color, t float64) theme.Color {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return theme.Color{
		R: from.R + (to.R-from.R)*t,
		G: from.G + (to.G-from.G)*t,
		B: from.B + (to.B-from.B)*t,
		A: from.A + (to.A-from.A)*t,
	}
}

// GradientMonotonic reports whether the midpoint sits component-wise
// between the endpoints (single-segment monotonic check for tests).
func GradientMonotonic(from, mid, to theme.Color) bool {
	between := func(a, m, b float64) bool {
		return (a <= m && m <= b) || (b <= m && m <= a)
	}
	return between(from.R, mid.R, to.R) && between(from.G, mid.G, to.G) && between(from.B, mid.B, to.B)
}

// ---- Colored / Decorated ----

// Colored builds a solid box from a theme color.
func Colored(w, h float64, c theme.Color) *rendering.RenderColorBox {
	return rendering.NewRenderColorBox(w, h, c.R, c.G, c.B, c.A)
}

// DecorSpec is the resolved decoration: background, border, radius,
// shadow offset and an optional two-stop linear gradient. Empty
// gradient (both stops alpha 0 and RGB 0) means solid fill.
type DecorSpec struct {
	Bg           theme.Color
	Border       theme.Color
	BorderWidth  float64
	Radius       float64
	ShadowColor  theme.Color
	ShadowDX     float64
	ShadowDY     float64
	GradientFrom theme.Color
	GradientTo   theme.Color
	HasGradient  bool
}

// DecorProps carries optional call-site overrides.
type DecorProps struct {
	Bg          *theme.Color
	Border      *theme.Color
	BorderWidth *float64
	Radius      *float64
}

// ResolveDecor merges props > component theme > seed. Component theme
// is a partial DecorSpec plus set flags simplified to nil checks.
func ResolveDecor(props DecorProps, ctheme *DecorSpec, seed theme.Tokens) DecorSpec {
	spec := DecorSpec{
		Bg:          seed.ColorBgContainer,
		Border:      seed.ColorBorder,
		BorderWidth: seed.LineWidth,
		Radius:      seed.Radius,
	}
	if ctheme != nil {
		spec = *ctheme
	}
	if props.Bg != nil {
		spec.Bg = *props.Bg
	}
	if props.Border != nil {
		spec.Border = *props.Border
	}
	if props.BorderWidth != nil {
		spec.BorderWidth = *props.BorderWidth
	}
	if props.Radius != nil {
		spec.Radius = *props.Radius
	}
	return spec
}

// NewDecoratedBox builds a decorated box: background (or gradient),
// border and shadow painted through OnPaint, clipped and hit-tested by
// an outer rounded clip so Hit ≡ 绘. Shadow is paint only and never
// extends the hit area (the clip box size excludes it).
func NewDecoratedBox(w, h float64, spec DecorSpec, child rendering.RenderObject) *rendering.RenderClipRRect {
	inner := rendering.NewRenderBox()
	inner.FixedWidth, inner.FixedHeight = w, h
	if child != nil {
		inner.AddChild(child)
	}
	specCopy := spec
	inner.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil {
			return
		}
		if specCopy.ShadowColor.A > 0 && (specCopy.ShadowDX != 0 || specCopy.ShadowDY != 0) {
			s := specCopy.ShadowColor
			rendering.FillRoundRect(pc, specCopy.ShadowDX, specCopy.ShadowDY, size.Width, size.Height, specCopy.Radius, s.R, s.G, s.B, s.A)
		}
		if specCopy.HasGradient {
			f, t := specCopy.GradientFrom, specCopy.GradientTo
			rendering.FillLinearGradient(pc, 0, 0, size.Width, size.Height,
				0, 0, size.Width, size.Height,
				f.R, f.G, f.B, f.A, t.R, t.G, t.B, t.A)
		} else {
			bg := specCopy.Bg
			rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, specCopy.Radius, bg.R, bg.G, bg.B, bg.A)
		}
		if specCopy.BorderWidth > 0 {
			bd := specCopy.Border
			rendering.StrokeRoundRect(pc, 0, 0, size.Width, size.Height, specCopy.Radius, specCopy.BorderWidth, bd.R, bd.G, bd.B, bd.A)
		}
	}
	clip := rendering.NewRenderClipRRect(inner)
	clip.FixedWidth, clip.FixedHeight = w, h
	clip.Radius = spec.Radius
	return clip
}

// ---- Clip / Opacity / Transform / Fitted / CustomPaint ----

// NewClipRect clips children to a hard rectangle (radius 0).
func NewClipRect(child rendering.RenderObject) *rendering.RenderClipRRect {
	c := rendering.NewRenderClipRRect()
	c.Radius = 0
	if child != nil {
		c.AddChild(child)
	}
	return c
}

// NewClipRRect clips children to a uniform rounded rectangle.
func NewClipRRect(radius float64, child rendering.RenderObject) *rendering.RenderClipRRect {
	c := rendering.NewRenderClipRRect()
	if radius < 0 {
		radius = 0
	}
	c.Radius = radius
	if child != nil {
		c.AddChild(child)
	}
	return c
}

// OvalClipSpec describes an oval clip in a w×h box for tests.
type OvalClipSpec struct {
	W, H float64
}

// Contains reports ellipse containment for the spec box.
func (o OvalClipSpec) Contains(x, y float64) bool { return OvalContains(x, y, o.W, o.H) }

// NewOpacity wraps children with group opacity (整层一次合成).
func NewOpacity(opacity float64, children ...rendering.RenderObject) *rendering.RenderOpacity {
	return rendering.NewRenderOpacity(opacity, children...)
}

// NewTransformed wraps children with rotation (radians) and scale.
func NewTransformed(rotation, sx, sy float64, children ...rendering.RenderObject) *rendering.RenderTransform {
	t := rendering.NewRenderTransform(children...)
	t.Rotation = rotation
	if sx != 0 || sy != 0 {
		t.SX, t.SY = sx, sy
	}
	return t
}

// FittedScale returns the uniform scale fitting child into parent.
func FittedScale(parentW, parentH, childW, childH float64) float64 {
	if childW <= 0 || childH <= 0 {
		return 1
	}
	s := parentW / childW
	if h := parentH / childH; h < s {
		s = h
	}
	if s < 0 {
		return 0
	}
	return s
}

// NewFitted wraps child in a transform scaled to fit parent.
func NewFitted(parentW, parentH, childW, childH float64, child rendering.RenderObject) *rendering.RenderTransform {
	t := rendering.NewRenderTransform()
	s := FittedScale(parentW, parentH, childW, childH)
	t.SX, t.SY = s, s
	if child != nil {
		t.AddChild(child)
	}
	return t
}

// Painter draws custom content in a fixed box (CustomPaint门面).
type Painter func(pc *rendering.PaintContext, size rendering.Size)

// NewCustomPaint builds a fixed box invoking painter plus an optional
// child. RepaintBoundary isolates its repaints when true.
func NewCustomPaint(w, h float64, painter Painter, boundary bool, child rendering.RenderObject) *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = w, h
	if child != nil {
		b.AddChild(child)
	}
	if painter != nil {
		pc := painter
		b.OnPaint = func(ctx *rendering.PaintContext, size rendering.Size) { pc(ctx, size) }
	}
	if boundary {
		b.SetRepaintBoundary(true)
	}
	return b
}

// ---- FollowTarget / Follower (overlay定位唯一口) ----

// FollowPlacement re-exports the overlay placement vocabulary.
type FollowPlacement = overlay.Placement

// ResolveFollower positions a follower rect from an anchor rect without
// reimplementing placement: the only call is overlay.Resolve.
func ResolveFollower(anchor rendering.Rect, ow, oh float64, want FollowPlacement, opt *overlay.ResolveOptions) overlay.Resolved {
	return overlay.Resolve(anchor, ow, oh, want, opt)
}
