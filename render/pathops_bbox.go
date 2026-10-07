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

// BoundingBox returns the tight axis-aligned bounding box of the path.
// Uses curve extrema for accuracy.
func (p *Path) BoundingBox() Rect {
	if len(p.verbs) == 0 {
		return Rect{}
	}

	// Initialize with extreme values
	bbox := Rect{
		Min: Point{X: math.MaxFloat64, Y: math.MaxFloat64},
		Max: Point{X: -math.MaxFloat64, Y: -math.MaxFloat64},
	}

	var current Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			pt := Pt(coords[0], coords[1])
			bbox = expandBBox(bbox, pt)
			current = pt
		case LineTo:
			pt := Pt(coords[0], coords[1])
			bbox = expandBBox(bbox, pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			bbox = bbox.Union(quadBBox(current, ctrl, pt))
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			bbox = bbox.Union(cubicBBox(current, ctrl1, ctrl2, pt))
			current = pt
		case Close:
			// Close doesn't add new points
		}
	})

	// Handle empty path case
	if bbox.Min.X == math.MaxFloat64 {
		return Rect{}
	}

	return bbox
}

// expandBBox expands the bounding box to include the point.
// expandBBox expands the bounding box to include the point.
func expandBBox(bbox Rect, pt Point) Rect {
	return Rect{
		Min: Point{X: math.Min(bbox.Min.X, pt.X), Y: math.Min(bbox.Min.Y, pt.Y)},
		Max: Point{X: math.Max(bbox.Max.X, pt.X), Y: math.Max(bbox.Max.Y, pt.Y)},
	}
}

// quadBBox returns the tight bounding box of a quadratic Bezier.
// quadBBox returns the tight bounding box of a quadratic Bezier.
func quadBBox(p0, p1, p2 Point) Rect {
	q := NewQuadBez(p0, p1, p2)
	return q.BoundingBox()
}

// cubicBBox returns the tight bounding box of a cubic Bezier.
// cubicBBox returns the tight bounding box of a cubic Bezier.
func cubicBBox(p0, p1, p2, p3 Point) Rect {
	c := NewCubicBez(p0, p1, p2, p3)
	return c.BoundingBox()
}

// Flatten converts all curves to line segments with given tolerance.
// tolerance is the maximum distance from the curve.
// Flatten converts all curves to line segments with given tolerance.
// tolerance is the maximum distance from the curve.
func (p *Path) Flatten(tolerance float64) []Point {
	if len(p.verbs) == 0 {
		return nil
	}

	points := make([]Point, 0, len(p.verbs)*4)
	p.FlattenCallback(tolerance, func(pt Point) {
		points = append(points, pt)
	})
	return points
}

// FlattenCallback calls fn for each point in the flattened path.
// More efficient than Flatten() as it avoids allocation.
// FlattenCallback calls fn for each point in the flattened path.
// More efficient than Flatten() as it avoids allocation.
func (p *Path) FlattenCallback(tolerance float64, fn func(pt Point)) {
	if tolerance <= 0 {
		tolerance = 0.1 // Default tolerance
	}

	var current, start Point
	var started bool

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			if started {
				fn(current) // Emit last point of previous subpath
			}
			pt := Pt(coords[0], coords[1])
			fn(pt)
			start = pt
			current = pt
			started = true
		case LineTo:
			pt := Pt(coords[0], coords[1])
			fn(pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			flattenQuad(current, ctrl, pt, tolerance, fn)
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			flattenCubic(current, ctrl1, ctrl2, pt, tolerance, fn)
			current = pt
		case Close:
			if current != start {
				fn(start)
			}
			current = start
		}
	})
}

// flattenQuad flattens a quadratic Bezier curve.
// flattenQuad flattens a quadratic Bezier curve.
func flattenQuad(p0, p1, p2 Point, tolerance float64, fn func(pt Point)) {
	q := NewQuadBez(p0, p1, p2)
	flattenQuadRecursive(q, tolerance*tolerance, fn, 0)
}

// flattenQuadRecursive recursively subdivides the quadratic.
// flattenQuadRecursive recursively subdivides the quadratic.
func flattenQuadRecursive(q QuadBez, toleranceSq float64, fn func(pt Point), depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		fn(q.P2)
		return
	}

	// Flatness test: distance from control point to chord midpoint
	mid := q.P0.Lerp(q.P2, 0.5)
	dist := q.P1.Sub(mid)
	if dist.LengthSquared() <= toleranceSq {
		fn(q.P2)
		return
	}

	// Subdivide
	q1, q2 := q.Subdivide()
	flattenQuadRecursive(q1, toleranceSq, fn, depth+1)
	flattenQuadRecursive(q2, toleranceSq, fn, depth+1)
}

// flattenCubic flattens a cubic Bezier curve.
// flattenCubic flattens a cubic Bezier curve.
func flattenCubic(p0, p1, p2, p3 Point, tolerance float64, fn func(pt Point)) {
	c := NewCubicBez(p0, p1, p2, p3)
	flattenCubicRecursive(c, tolerance*tolerance, fn, 0)
}

// flattenCubicRecursive recursively subdivides the cubic.
