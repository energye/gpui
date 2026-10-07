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
	"math"
)

// MoveTo moves to a point without drawing.
func (p *Path) MoveTo(x, y float64) {
	p.verbs = append(p.verbs, MoveTo)
	p.coords = append(p.coords, x, y)
	p.expandBounds(x, y)
	p.start = Pt(x, y)
	p.current = p.start
}

// LineTo draws a line to a point.
// LineTo draws a line to a point.
func (p *Path) LineTo(x, y float64) {
	p.verbs = append(p.verbs, LineTo)
	p.coords = append(p.coords, x, y)
	p.expandBounds(x, y)
	p.current = Pt(x, y)
}

// QuadraticTo draws a quadratic Bezier curve.
// QuadraticTo draws a quadratic Bezier curve.
func (p *Path) QuadraticTo(cx, cy, x, y float64) {
	p.verbs = append(p.verbs, QuadTo)
	p.coords = append(p.coords, cx, cy, x, y)
	p.expandBounds(cx, cy)
	p.expandBounds(x, y)
	p.current = Pt(x, y)
}

// CubicTo draws a cubic Bezier curve.
// CubicTo draws a cubic Bezier curve.
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float64) {
	p.verbs = append(p.verbs, CubicTo)
	p.coords = append(p.coords, c1x, c1y, c2x, c2y, x, y)
	p.expandBounds(c1x, c1y)
	p.expandBounds(c2x, c2y)
	p.expandBounds(x, y)
	p.current = Pt(x, y)
}

// Close closes the current subpath by drawing a line to the start point.
// Close closes the current subpath by drawing a line to the start point.
func (p *Path) Close() {
	p.verbs = append(p.verbs, Close)
	p.current = p.start
}

// Clear removes all elements from the path, releasing the underlying storage.
// Clear removes all elements from the path, releasing the underlying storage.
func (p *Path) Clear() {
	p.verbs = p.verbs[:0]
	p.coords = p.coords[:0]
	p.start = Point{}
	p.current = Point{}
	p.boundsValid = false
}

// Reset clears the path for reuse, keeping allocated capacity.
// This is identical to Clear in the current implementation.
// Reset clears the path for reuse, keeping allocated capacity.
// This is identical to Clear in the current implementation.
func (p *Path) Reset() {
	p.verbs = p.verbs[:0]
	p.coords = p.coords[:0]
	p.start = Point{}
	p.current = Point{}
}

// Append adds all elements from other to this path.
// The current point and subpath start are updated to match other's state.
// Append adds all elements from other to this path.
// The current point and subpath start are updated to match other's state.
func (p *Path) Append(other *Path) {
	if other == nil || len(other.verbs) == 0 {
		return
	}
	p.verbs = append(p.verbs, other.verbs...)
	p.coords = append(p.coords, other.coords...)
	p.current = other.current
	p.start = other.start
}

// Iterate calls fn for each verb in the path with the corresponding coordinate slice.
// This is the primary zero-allocation iteration API.
//
// The coords slice passed to fn is a sub-slice of the path's coordinate buffer:
//   - MoveTo: coords has 2 elements (x, y)
//   - LineTo: coords has 2 elements (x, y)
//   - QuadTo: coords has 4 elements (cx, cy, x, y)
//   - CubicTo: coords has 6 elements (c1x, c1y, c2x, c2y, x, y)
//   - Close: coords has 0 elements (nil)
//
// Transform applies a transformation matrix to all points in the path.
func (p *Path) Transform(m Matrix) *Path {
	result := NewPath()
	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			pt := m.TransformPoint(Pt(coords[0], coords[1]))
			result.MoveTo(pt.X, pt.Y)
		case LineTo:
			pt := m.TransformPoint(Pt(coords[0], coords[1]))
			result.LineTo(pt.X, pt.Y)
		case QuadTo:
			ctrl := m.TransformPoint(Pt(coords[0], coords[1]))
			pt := m.TransformPoint(Pt(coords[2], coords[3]))
			result.QuadraticTo(ctrl.X, ctrl.Y, pt.X, pt.Y)
		case CubicTo:
			ctrl1 := m.TransformPoint(Pt(coords[0], coords[1]))
			ctrl2 := m.TransformPoint(Pt(coords[2], coords[3]))
			pt := m.TransformPoint(Pt(coords[4], coords[5]))
			result.CubicTo(ctrl1.X, ctrl1.Y, ctrl2.X, ctrl2.Y, pt.X, pt.Y)
		case Close:
			result.Close()
		}
	})
	return result
}

// Rectangle adds a rectangle to the path.
// Rectangle adds a rectangle to the path.
func (p *Path) Rectangle(x, y, w, h float64) {
	p.MoveTo(x, y)
	p.LineTo(x+w, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()
}

// Circle adds a circle to the path using cubic Bezier curves.
// Circle adds a circle to the path using cubic Bezier curves.
func (p *Path) Circle(cx, cy, r float64) {
	// Magic constant for circle approximation with cubic Beziers
	const k = 0.5522847498307936 // 4/3 * (sqrt(2) - 1)
	offset := r * k

	p.MoveTo(cx+r, cy)
	p.CubicTo(cx+r, cy+offset, cx+offset, cy+r, cx, cy+r)
	p.CubicTo(cx-offset, cy+r, cx-r, cy+offset, cx-r, cy)
	p.CubicTo(cx-r, cy-offset, cx-offset, cy-r, cx, cy-r)
	p.CubicTo(cx+offset, cy-r, cx+r, cy-offset, cx+r, cy)
	p.Close()
}

// Ellipse adds an ellipse to the path.
// Ellipse adds an ellipse to the path.
func (p *Path) Ellipse(cx, cy, rx, ry float64) {
	const k = 0.5522847498307936
	ox := rx * k
	oy := ry * k

	p.MoveTo(cx+rx, cy)
	p.CubicTo(cx+rx, cy+oy, cx+ox, cy+ry, cx, cy+ry)
	p.CubicTo(cx-ox, cy+ry, cx-rx, cy+oy, cx-rx, cy)
	p.CubicTo(cx-rx, cy-oy, cx-ox, cy-ry, cx, cy-ry)
	p.CubicTo(cx+ox, cy-ry, cx+rx, cy-oy, cx+rx, cy)
	p.Close()
}

// Arc adds a circular arc to the path.
// The arc is drawn from angle1 to angle2 (in radians) around center (cx, cy).
// Arc adds a circular arc to the path.
// The arc is drawn from angle1 to angle2 (in radians) around center (cx, cy).
func (p *Path) Arc(cx, cy, r, angle1, angle2 float64) {
	// Normalize angles
	const twoPi = 2 * math.Pi
	for angle2 < angle1 {
		angle2 += twoPi
	}

	// Split into multiple cubic Bezier curves
	// Maximum 90 degrees per segment
	const maxAngle = math.Pi / 2
	numSegments := int(math.Ceil((angle2 - angle1) / maxAngle))
	angleStep := (angle2 - angle1) / float64(numSegments)

	for i := 0; i < numSegments; i++ {
		a1 := angle1 + float64(i)*angleStep
		a2 := a1 + angleStep
		p.arcSegment(cx, cy, r, a1, a2)
	}
}

// arcSegment adds a single arc segment (<=90 degrees).
// arcSegment adds a single arc segment (<=90 degrees).
func (p *Path) arcSegment(cx, cy, r, a1, a2 float64) {
	// Calculate control points for cubic Bezier approximation
	alpha := math.Sin(a2-a1) * (math.Sqrt(4+3*math.Tan((a2-a1)/2)*math.Tan((a2-a1)/2)) - 1) / 3

	cos1, sin1 := math.Cos(a1), math.Sin(a1)
	cos2, sin2 := math.Cos(a2), math.Sin(a2)

	x1 := cx + r*cos1
	y1 := cy + r*sin1
	x2 := cx + r*cos2
	y2 := cy + r*sin2

	c1x := x1 - alpha*r*sin1
	c1y := y1 + alpha*r*cos1
	c2x := x2 + alpha*r*sin2
	c2y := y2 - alpha*r*cos2

	if len(p.verbs) == 0 {
		p.MoveTo(x1, y1)
	}
	p.CubicTo(c1x, c1y, c2x, c2y, x2, y2)
}

// RoundedRectangle adds a rectangle with rounded corners.
// RoundedRectangle adds a rectangle with rounded corners.
func (p *Path) RoundedRectangle(x, y, w, h, r float64) {
	// Clamp radius to half of the smaller dimension
	maxR := math.Min(w, h) / 2
	if r > maxR {
		r = maxR
	}

	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.Arc(x+w-r, y+r, r, -math.Pi/2, 0)
	p.LineTo(x+w, y+h-r)
	p.Arc(x+w-r, y+h-r, r, 0, math.Pi/2)
	p.LineTo(x+r, y+h)
	p.Arc(x+r, y+h-r, r, math.Pi/2, math.Pi)
	p.LineTo(x, y+r)
	p.Arc(x+r, y+r, r, math.Pi, 3*math.Pi/2)
	p.Close()
}

// Clone creates a deep copy of the path.
// Clone creates a deep copy of the path.
func (p *Path) Clone() *Path {
	result := &Path{
		verbs:       make([]PathVerb, len(p.verbs)),
		coords:      make([]float64, len(p.coords)),
		start:       p.start,
		current:     p.current,
		boundsMinX:  p.boundsMinX,
		boundsMinY:  p.boundsMinY,
		boundsMaxX:  p.boundsMaxX,
		boundsMaxY:  p.boundsMaxY,
		boundsValid: p.boundsValid,
	}
	copy(result.verbs, p.verbs)
	copy(result.coords, p.coords)
	return result
}
