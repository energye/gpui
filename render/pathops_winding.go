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

import "math"

// Winding returns the winding number of a point relative to the path.
// 0 = outside, non-zero = inside (for non-zero fill rule).
// Uses ray casting with a horizontal ray to the right.
func (p *Path) Winding(pt Point) int {
	var winding int
	var current, start Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			start = Pt(coords[0], coords[1])
			current = start
		case LineTo:
			ep := Pt(coords[0], coords[1])
			winding += lineWinding(current, ep, pt)
			current = ep
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			ep := Pt(coords[2], coords[3])
			winding += quadWinding(current, ctrl, ep, pt)
			current = ep
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			ep := Pt(coords[4], coords[5])
			winding += cubicWinding(current, ctrl1, ctrl2, ep, pt)
			current = ep
		case Close:
			winding += lineWinding(current, start, pt)
			current = start
		}
	})

	return winding
}

// lineWinding computes the winding contribution of a line segment.
// lineWinding computes the winding contribution of a line segment.
func lineWinding(p0, p1, pt Point) int {
	if p0.Y <= pt.Y && p1.Y > pt.Y {
		// Upward crossing
		if isLeft(p0, p1, pt) > 0 {
			return 1
		}
	} else if p0.Y > pt.Y && p1.Y <= pt.Y {
		// Downward crossing
		if isLeft(p0, p1, pt) < 0 {
			return -1
		}
	}
	return 0
}

// isLeft returns positive if pt is left of line p0-p1, negative if right, 0 if on.
// isLeft returns positive if pt is left of line p0-p1, negative if right, 0 if on.
func isLeft(p0, p1, pt Point) float64 {
	return (p1.X-p0.X)*(pt.Y-p0.Y) - (pt.X-p0.X)*(p1.Y-p0.Y)
}

// quadWinding computes the winding contribution of a quadratic Bezier.
// quadWinding computes the winding contribution of a quadratic Bezier.
func quadWinding(p0, p1, p2, pt Point) int {
	// Early exit if point is outside the vertical range
	minY := math.Min(math.Min(p0.Y, p1.Y), p2.Y)
	maxY := math.Max(math.Max(p0.Y, p1.Y), p2.Y)
	if pt.Y < minY || pt.Y > maxY {
		return 0
	}

	// Early exit if point is to the right of the curve
	maxX := math.Max(math.Max(p0.X, p1.X), p2.X)
	if pt.X > maxX {
		return 0
	}

	// Flatten the curve and sum line winding contributions
	return flattenQuadWinding(p0, p1, p2, pt)
}

// flattenQuadWinding computes winding by adaptively flattening the quadratic.
// flattenQuadWinding computes winding by adaptively flattening the quadratic.
func flattenQuadWinding(p0, p1, p2, pt Point) int {
	q := NewQuadBez(p0, p1, p2)

	// Use adaptive subdivision based on flatness
	const tolerance = 0.1
	var winding int
	flattenQuadWindingRecursive(q, pt, tolerance, &winding, 0)
	return winding
}

// flattenQuadWindingRecursive recursively subdivides and accumulates winding.
// flattenQuadWindingRecursive recursively subdivides and accumulates winding.
func flattenQuadWindingRecursive(q QuadBez, pt Point, tolerance float64, winding *int, depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		*winding += lineWinding(q.P0, q.P2, pt)
		return
	}

	// Flatness test: distance from control point to chord
	mid := q.P0.Lerp(q.P2, 0.5)
	dist := q.P1.Sub(mid).Length()

	if dist <= tolerance {
		// Flat enough - use line approximation
		*winding += lineWinding(q.P0, q.P2, pt)
		return
	}

	// Subdivide and recurse
	q1, q2 := q.Subdivide()
	flattenQuadWindingRecursive(q1, pt, tolerance, winding, depth+1)
	flattenQuadWindingRecursive(q2, pt, tolerance, winding, depth+1)
}

// cubicWinding computes the winding contribution of a cubic Bezier.
// cubicWinding computes the winding contribution of a cubic Bezier.
func cubicWinding(p0, p1, p2, p3, pt Point) int {
	// Early exit if point is outside the vertical range
	minY := math.Min(math.Min(p0.Y, p1.Y), math.Min(p2.Y, p3.Y))
	maxY := math.Max(math.Max(p0.Y, p1.Y), math.Max(p2.Y, p3.Y))
	if pt.Y < minY || pt.Y > maxY {
		return 0
	}

	// Early exit if point is to the right of the curve
	maxX := math.Max(math.Max(p0.X, p1.X), math.Max(p2.X, p3.X))
	if pt.X > maxX {
		return 0
	}

	// Flatten the curve and sum line winding contributions
	return flattenCubicWinding(p0, p1, p2, p3, pt)
}

// flattenCubicWinding computes winding by adaptively flattening the cubic.
// flattenCubicWinding computes winding by adaptively flattening the cubic.
func flattenCubicWinding(p0, p1, p2, p3, pt Point) int {
	c := NewCubicBez(p0, p1, p2, p3)

	const tolerance = 0.1
	var winding int
	flattenCubicWindingRecursive(c, pt, tolerance, &winding, 0)
	return winding
}

// flattenCubicWindingRecursive recursively subdivides and accumulates winding.
// flattenCubicWindingRecursive recursively subdivides and accumulates winding.
func flattenCubicWindingRecursive(c CubicBez, pt Point, tolerance float64, winding *int, depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		*winding += lineWinding(c.P0, c.P3, pt)
		return
	}

	// Flatness test: max distance from control points to chord
	flatness := cubicFlatness(c)

	if flatness <= tolerance {
		// Flat enough - use line approximation
		*winding += lineWinding(c.P0, c.P3, pt)
		return
	}

	// Subdivide and recurse
	c1, c2 := c.Subdivide()
	flattenCubicWindingRecursive(c1, pt, tolerance, winding, depth+1)
	flattenCubicWindingRecursive(c2, pt, tolerance, winding, depth+1)
}

// cubicFlatness returns the maximum distance from control points to the chord.
// cubicFlatness returns the maximum distance from control points to the chord.
func cubicFlatness(c CubicBez) float64 {
	ux := 3.0*c.P1.X - 2.0*c.P0.X - c.P3.X
	uy := 3.0*c.P1.Y - 2.0*c.P0.Y - c.P3.Y
	vx := 3.0*c.P2.X - c.P0.X - 2.0*c.P3.X
	vy := 3.0*c.P2.Y - c.P0.Y - 2.0*c.P3.Y

	return math.Max(ux*ux+uy*uy, vx*vx+vy*vy)
}

// Contains tests if a point is inside the path using the non-zero fill rule.
// Contains tests if a point is inside the path using the non-zero fill rule.
func (p *Path) Contains(pt Point) bool {
	return p.Winding(pt) != 0
}

// BoundingBox returns the tight axis-aligned bounding box of the path.
// Uses curve extrema for accuracy.
