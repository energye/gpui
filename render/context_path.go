//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"math"

)

// Clear resets the entire context to transparent (zero alpha).
// To fill with a specific background color, use [ClearWithColor].
func (c *Context) Clear() {
	c.releaseFilterGPUResult()
	c.pixmapFilterStale = false
	c.clearViewFlushTracking()
	c.midFrameNilFlush = false
	c.pixmap.Clear(Transparent)
}

// ClearWithColor fills the entire context with the specified color.
// This is the recommended way to set a background color before drawing.
// ClearWithColor fills the entire context with the specified color.
// This is the recommended way to set a background color before drawing.
func (c *Context) ClearWithColor(col RGBA) {
	c.releaseFilterGPUResult()
	c.pixmapFilterStale = false
	c.clearViewFlushTracking()
	c.midFrameNilFlush = false
	c.pixmap.Clear(col)
}

// maxDamageRects is the threshold above which individual rects are coalesced
// (touch/overlap merge, then full union only if still over the cap) to avoid
// O(n²) OS damage region submission. Wayland/Android use similar thresholds.
const maxDamageRects = 16

// FrameDamage returns the list of damage rectangles from draw operations
// this frame. Each rect corresponds to one or more Fill/Stroke operations.
// Used by ggcanvas → SetDamageRects → PresentWithDamage for per-rect OS blit.
// Returns nil if no drawing operations occurred.
// FillRectCPU fills a rectangle directly on the CPU pixmap without engaging
// the GPU SDF accelerator. Coordinates are in user space (device scale applied
// automatically). Pending GPU shapes are flushed first for correct z-ordering.
//
// Use for operations where GPU acceleration is counterproductive, such as
// dirty-region background clearing in retained-mode compositors. Without this,
// DrawRectangle+Fill routes through SDF accelerator → blocks non-MSAA blit path.
//
// See ADR-016, TASK-GG-COMPOSITOR-003.
func (c *Context) FillRectCPU(x, y, w, h float64, col RGBA) {
	c.flushGPUAccelerator()

	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(x, y))
	br := ctm.TransformPoint(Pt(x+w, y+h))

	px0 := int(tl.X)
	py0 := int(tl.Y)
	px1 := int(br.X + 0.5)
	py1 := int(br.Y + 0.5)

	pr := uint8(clamp255(col.R * col.A * 255))
	pg := uint8(clamp255(col.G * col.A * 255))
	pb := uint8(clamp255(col.B * col.A * 255))
	pa := uint8(clamp255(col.A * 255))

	c.pixmap.FillRect(image.Rect(px0, py0, px1, py1), pr, pg, pb, pa)
}

// SetColor sets the current drawing color.
//
// New code prefers Solid* + SetFillBrush.
// ClearDash removes the dash pattern, returning to solid lines.
func (c *Context) ClearDash() {
	if c.paint.Stroke != nil {
		c.paint.Stroke.Dash = nil
	}
}

// IsDashed returns true if the current stroke uses a dash pattern.
// MoveTo starts a new subpath at the given point.
func (c *Context) MoveTo(x, y float64) {
	p := c.matrix.TransformPoint(Pt(x, y))
	c.path.MoveTo(p.X, p.Y)
}

// LineTo adds a line to the current path.
// LineTo adds a line to the current path.
func (c *Context) LineTo(x, y float64) {
	p := c.matrix.TransformPoint(Pt(x, y))
	c.path.LineTo(p.X, p.Y)
}

// QuadraticTo adds a quadratic Bezier curve to the current path.
// QuadraticTo adds a quadratic Bezier curve to the current path.
func (c *Context) QuadraticTo(cx, cy, x, y float64) {
	cp := c.matrix.TransformPoint(Pt(cx, cy))
	p := c.matrix.TransformPoint(Pt(x, y))
	c.path.QuadraticTo(cp.X, cp.Y, p.X, p.Y)
}

// CubicTo adds a cubic Bezier curve to the current path.
// CubicTo adds a cubic Bezier curve to the current path.
func (c *Context) CubicTo(c1x, c1y, c2x, c2y, x, y float64) {
	cp1 := c.matrix.TransformPoint(Pt(c1x, c1y))
	cp2 := c.matrix.TransformPoint(Pt(c2x, c2y))
	p := c.matrix.TransformPoint(Pt(x, y))
	c.path.CubicTo(cp1.X, cp1.Y, cp2.X, cp2.Y, p.X, p.Y)
}

// ClosePath closes the current subpath.
// ClosePath closes the current subpath.
func (c *Context) ClosePath() {

	c.path.Close()
}

// ClearPath clears the current path.
// ClearPath clears the current path.
func (c *Context) ClearPath() {
	c.path.Clear()
}

// SetPath replaces the current path with p.
// The path is copied — subsequent modifications to p do not affect the context.
// Use this to render pre-built paths (e.g., from ParseSVGPath):
//
//	path, _ := render.ParseSVGPath
//	dc.SetPath(path)
//	dc.Fill()
// SetPath replaces the current path with p.
// The path is copied — subsequent modifications to p do not affect the context.
// Use this to render pre-built paths (e.g., from ParseSVGPath):
//
//	path, _ := render.ParseSVGPath
//	dc.SetPath(path)
//	dc.Fill()
func (c *Context) SetPath(p *Path) {
	c.path.Clear()
	if p != nil {
		c.path.Append(p)
	}
}

// AppendPath appends the elements of p to the current path without clearing it.
// This allows combining multiple sub-paths before a single Fill or Stroke call.
// Note: path coordinates are copied as-is (not transformed by the current matrix).
// Use DrawPath for transform-aware path rendering.
// AppendPath appends the elements of p to the current path without clearing it.
// This allows combining multiple sub-paths before a single Fill or Stroke call.
// Note: path coordinates are copied as-is (not transformed by the current matrix).
// Use DrawPath for transform-aware path rendering.
func (c *Context) AppendPath(p *Path) {
	if p != nil {
		c.path.Append(p)
	}
}

// DrawPath replays the elements of p through the current transform matrix,
// replacing the current path. Unlike SetPath (which copies raw coordinates),
// DrawPath applies the current matrix (Translate, Scale, Rotate) to all points.
// After DrawPath, call Fill() or Stroke() to render.
//
// This is the correct way to render pre-built paths (e.g., from ParseSVGPath)
// with transforms:
//
//	path, _ := render.ParseSVGPath
//	dc.Push()
//	dc.Translate(x, y)
//	dc.Scale(0.5, 0.5)
//	dc.DrawPath(path)
//	dc.Fill()
//	dc.Pop()
// DrawPath replays the elements of p through the current transform matrix,
// replacing the current path. Unlike SetPath (which copies raw coordinates),
// DrawPath applies the current matrix (Translate, Scale, Rotate) to all points.
// After DrawPath, call Fill() or Stroke() to render.
//
// This is the correct way to render pre-built paths (e.g., from ParseSVGPath)
// with transforms:
//
//	path, _ := render.ParseSVGPath
//	dc.Push()
//	dc.Translate(x, y)
//	dc.Scale(0.5, 0.5)
//	dc.DrawPath(path)
//	dc.Fill()
//	dc.Pop()
func (c *Context) DrawPath(p *Path) {
	c.ClearPath()
	if p == nil {
		return
	}
	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			c.MoveTo(coords[0], coords[1])
		case LineTo:
			c.LineTo(coords[0], coords[1])
		case QuadTo:
			c.QuadraticTo(coords[0], coords[1], coords[2], coords[3])
		case CubicTo:
			c.CubicTo(coords[0], coords[1], coords[2], coords[3], coords[4], coords[5])
		case Close:
			c.ClosePath()
		}
	})
}

// FillPath is a convenience method that replays path p through the current
// transform, fills it, and clears the path. Equivalent to DrawPath(p) + Fill().
// FillPath is a convenience method that replays path p through the current
// transform, fills it, and clears the path. Equivalent to DrawPath(p) + Fill().
func (c *Context) FillPath(p *Path) error {
	c.DrawPath(p)
	return c.Fill()
}

// StrokePath is a convenience method that replays path p through the current
// transform, strokes it, and clears the path. Equivalent to DrawPath(p) + Stroke().
// StrokePath is a convenience method that replays path p through the current
// transform, strokes it, and clears the path. Equivalent to DrawPath(p) + Stroke().
func (c *Context) StrokePath(p *Path) error {
	c.DrawPath(p)
	return c.Stroke()
}

// NewSubPath starts a new subpath without closing the previous one.
// NewSubPath starts a new subpath without closing the previous one.
func (c *Context) NewSubPath() {
	// In most implementations, just starting with MoveTo creates a new subpath
	// This is a no-op but provided for API compatibility
}

// Fill fills the current path and clears it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// The RasterizerMode set via SetRasterizerMode controls algorithm selection.
// Returns an error if the rendering operation fails.
// deviceSpacePath returns the current path transformed to device-space.
// Path coordinates are in user-space (transformed by c.matrix only).
// The renderer operates in device-space, so we apply deviceMatrix here.
// At scale=1.0, returns the original path (zero copy).
func (c *Context) deviceSpacePath() *Path {
	if c.deviceMatrix.IsIdentity() {
		return c.path
	}
	return c.path.Transform(c.deviceMatrix)
}

// SetPixel sets a single pixel.
// DrawPoint draws a single point at the given coordinates.
func (c *Context) DrawPoint(x, y, r float64) {
	c.DrawCircle(x, y, r)
}

// DrawLine draws a line between two points.
// DrawLine draws a line between two points.
func (c *Context) DrawLine(x1, y1, x2, y2 float64) {

	c.MoveTo(x1, y1)
	c.LineTo(x2, y2)
}

// DrawRectangle draws a rectangle.
// DrawRectangle draws a rectangle.
func (c *Context) DrawRectangle(x, y, w, h float64) {

	c.MoveTo(x, y)
	c.LineTo(x+w, y)
	c.LineTo(x+w, y+h)
	c.LineTo(x, y+h)
	c.ClosePath()
}

// DrawRoundedRectangle draws a rectangle with rounded corners.
//
// The corner radius r is clamped to half the smaller dimension.
// All coordinates are transformed through the current matrix,
// ensuring correct rendering on HiDPI/Retina displays.
// DrawRoundedRectangle draws a rectangle with rounded corners.
//
// The corner radius r is clamped to half the smaller dimension.
// All coordinates are transformed through the current matrix,
// ensuring correct rendering on HiDPI/Retina displays.
func (c *Context) DrawRoundedRectangle(x, y, w, h, r float64) {

	c.DrawRoundedRectangleXY(x, y, w, h, r, r)
}

// DrawRoundedRectangleXY draws a rounded rectangle with independent X/Y corner radii
// . Radii are clamped to half width/height.
// DrawRoundedRectangleXY draws a rounded rectangle with independent X/Y corner radii
// . Radii are clamped to half width/height.
func (c *Context) DrawRoundedRectangleXY(x, y, w, h, rx, ry float64) {
	if w <= 0 || h <= 0 {
		return
	}
	if rx < 0 {
		rx = 0
	}
	if ry < 0 {
		ry = 0
	}
	if rx > w/2 {
		rx = w / 2
	}
	if ry > h/2 {
		ry = h / 2
	}
	// Axis-aligned rect fast path.
	if rx <= 0 && ry <= 0 {
		c.DrawRectangle(x, y, w, h)
		return
	}
	// Cubic Bézier approximation for quarter-ellipses (same k as DrawCircle/Ellipse).
	const k = 0.5522847498307936
	kx := k * rx
	ky := k * ry
	// Top edge
	c.MoveTo(x+rx, y)
	c.LineTo(x+w-rx, y)
	// Top-right corner
	c.CubicTo(x+w-rx+kx, y, x+w, y+ry-ky, x+w, y+ry)
	// Right edge
	c.LineTo(x+w, y+h-ry)
	// Bottom-right corner
	c.CubicTo(x+w, y+h-ry+ky, x+w-rx+kx, y+h, x+w-rx, y+h)
	// Bottom edge
	c.LineTo(x+rx, y+h)
	// Bottom-left corner
	c.CubicTo(x+rx-kx, y+h, x, y+h-ry+ky, x, y+h-ry)
	// Left edge
	c.LineTo(x, y+ry)
	// Top-left corner
	c.CubicTo(x, y+ry-ky, x+rx-kx, y, x+rx, y)
	c.ClosePath()
}

// DrawCircle draws a circle.
// DrawCircle draws a circle.
func (c *Context) DrawCircle(x, y, r float64) {

	const k = 0.5522847498307936
	offset := r * k

	c.MoveTo(x+r, y)
	c.CubicTo(x+r, y+offset, x+offset, y+r, x, y+r)
	c.CubicTo(x-offset, y+r, x-r, y+offset, x-r, y)
	c.CubicTo(x-r, y-offset, x-offset, y-r, x, y-r)
	c.CubicTo(x+offset, y-r, x+r, y-offset, x+r, y)
	c.ClosePath()
}

// DrawEllipse draws an ellipse.
// DrawEllipse draws an ellipse.
func (c *Context) DrawEllipse(x, y, rx, ry float64) {
	const k = 0.5522847498307936
	ox := rx * k
	oy := ry * k

	c.MoveTo(x+rx, y)
	c.CubicTo(x+rx, y+oy, x+ox, y+ry, x, y+ry)
	c.CubicTo(x-ox, y+ry, x-rx, y+oy, x-rx, y)
	c.CubicTo(x-rx, y-oy, x-ox, y-ry, x, y-ry)
	c.CubicTo(x+ox, y-ry, x+rx, y-oy, x+rx, y)
	c.ClosePath()
}

// DrawArc draws a circular arc.
// DrawArc draws a circular arc.
func (c *Context) DrawArc(x, y, r, angle1, angle2 float64) {
	const twoPi = 2 * math.Pi
	for angle2 < angle1 {
		angle2 += twoPi
	}

	const maxAngle = math.Pi / 2
	numSegments := int(math.Ceil((angle2 - angle1) / maxAngle))
	angleStep := (angle2 - angle1) / float64(numSegments)

	for i := 0; i < numSegments; i++ {
		a1 := angle1 + float64(i)*angleStep
		a2 := a1 + angleStep
		c.arcSegment(x, y, r, a1, a2)
	}
}

// arcSegment draws a single arc segment.
// Endpoints and control handles go through the CTM (exactly like
// MoveTo/CubicTo and ellipseArcSegment) so rotation/scale apply to the
// arc shape; previously only the center was transformed, so a rotated
// arc (loading spinner, sync icon) painted unrotated and never animated.
// Pure-rotation CTMs only: the radius stays unscaled, matching the
// historical behavior for uniform transforms (T.03 user-space expansion
// owns non-uniform scale/skew for strokes, ellipseArcSegment owns radii).
// arcSegment draws a single arc segment.
// Endpoints and control handles go through the CTM (exactly like
// MoveTo/CubicTo and ellipseArcSegment) so rotation/scale apply to the
// arc shape; previously only the center was transformed, so a rotated
// arc (loading spinner, sync icon) painted unrotated and never animated.
// Pure-rotation CTMs only: the radius stays unscaled, matching the
// historical behavior for uniform transforms (T.03 user-space expansion
// owns non-uniform scale/skew for strokes, ellipseArcSegment owns radii).
func (c *Context) arcSegment(cx, cy, r, a1, a2 float64) {
	alpha := math.Sin(a2-a1) * (math.Sqrt(4+3*math.Tan((a2-a1)/2)*math.Tan((a2-a1)/2)) - 1) / 3

	cos1, sin1 := math.Cos(a1), math.Sin(a1)
	cos2, sin2 := math.Cos(a2), math.Sin(a2)

	p1 := c.matrix.TransformPoint(Pt(cx+r*cos1, cy+r*sin1))
	p2 := c.matrix.TransformPoint(Pt(cx+r*cos2, cy+r*sin2))
	c1 := c.matrix.TransformPoint(Pt(cx+r*cos1-alpha*r*sin1, cy+r*sin1+alpha*r*cos1))
	c2 := c.matrix.TransformPoint(Pt(cx+r*cos2+alpha*r*sin2, cy+r*sin2-alpha*r*cos2))

	if c.path.isEmpty() {
		c.path.MoveTo(p1.X, p1.Y)
	}
	c.path.CubicTo(c1.X, c1.Y, c2.X, c2.Y, p2.X, p2.Y)
}

// DrawEllipticalArc draws an elliptical arc (advanced).
// DrawEllipticalArc draws an elliptical arc (advanced).
func (c *Context) DrawEllipticalArc(x, y, rx, ry, angle1, angle2 float64) {
	// Build the arc from parametric ellipse points p(θ) = (x+rx·cosθ, y+ry·sinθ)
	// with per-axis radii. A Translate+Scale+unit-DrawArc shortcut would lose
	// rx/ry entirely: DrawArc only transforms the arc center through the CTM
	// and uses its radius parameter unscaled in world space (observed: the arc
	// collapsed to a ~1px dot at (x,y)). Points here are transformed per-vertex
	// like MoveTo/CubicTo, so rotation / non-uniform CTMs apply to the ellipse.
	const twoPi = 2 * math.Pi
	for angle2 < angle1 {
		angle2 += twoPi
	}

	const maxAngle = math.Pi / 2
	numSegments := int(math.Ceil((angle2 - angle1) / maxAngle))
	angleStep := (angle2 - angle1) / float64(numSegments)

	for i := 0; i < numSegments; i++ {
		a1 := angle1 + float64(i)*angleStep
		a2 := a1 + angleStep
		c.ellipseArcSegment(x, y, rx, ry, a1, a2)
	}
}

// ellipseArcSegment appends one elliptical arc segment (≤90°) using the same
// cubic approximation as arcSegment but with per-axis radii rx/ry. Endpoints
// and control handles are transformed by the CTM before path insertion so the
// ellipse participates in user transforms exactly like MoveTo/CubicTo.
// ellipseArcSegment appends one elliptical arc segment (≤90°) using the same
// cubic approximation as arcSegment but with per-axis radii rx/ry. Endpoints
// and control handles are transformed by the CTM before path insertion so the
// ellipse participates in user transforms exactly like MoveTo/CubicTo.
func (c *Context) ellipseArcSegment(x, y, rx, ry, a1, a2 float64) {
	alpha := math.Sin(a2-a1) * (math.Sqrt(4+3*math.Tan((a2-a1)/2)*math.Tan((a2-a1)/2)) - 1) / 3

	cos1, sin1 := math.Cos(a1), math.Sin(a1)
	cos2, sin2 := math.Cos(a2), math.Sin(a2)

	p1 := c.matrix.TransformPoint(Pt(x+rx*cos1, y+ry*sin1))
	p2 := c.matrix.TransformPoint(Pt(x+rx*cos2, y+ry*sin2))
	c1 := c.matrix.TransformPoint(Pt(x+rx*cos1-alpha*rx*sin1, y+ry*sin1+alpha*ry*cos1))
	c2 := c.matrix.TransformPoint(Pt(x+rx*cos2+alpha*rx*sin2, y+ry*sin2-alpha*ry*cos2))

	if c.path.isEmpty() {
		c.path.MoveTo(p1.X, p1.Y)
	}
	c.path.CubicTo(c1.X, c1.Y, c2.X, c2.Y, p2.X, p2.Y)
}

// currentColor returns the current drawing color from the paint.
// If the paint is a solid color, returns that color.
// Otherwise returns black as a fallback.
// GetCurrentPoint returns the current point of the path.
// Returns (0, 0, false) if there is no current point.
func (c *Context) GetCurrentPoint() (x, y float64, ok bool) {
	if c.path == nil || !c.path.HasCurrentPoint() {
		return 0, 0, false
	}
	pt := c.path.CurrentPoint()
	return pt.X, pt.Y, true
}

// EncodePNG writes the image as PNG to the given writer.
// This is useful for streaming, network output, or custom storage.
