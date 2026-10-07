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


// Area returns the signed area enclosed by the path.
// Positive for clockwise paths, negative for counter-clockwise.
// Uses the shoelace formula extended for curves (Green's theorem).
// Only closed subpaths contribute to the area.
func (p *Path) Area() float64 {
	var area float64
	var current, start Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			start = Pt(coords[0], coords[1])
			current = start
		case LineTo:
			pt := Pt(coords[0], coords[1])
			area += lineArea(current, pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			area += quadArea(current, ctrl, pt)
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			area += cubicArea(current, ctrl1, ctrl2, pt)
			current = pt
		case Close:
			area += lineArea(current, start)
			current = start
		}
	})

	return area
}

// lineArea computes the contribution of a line segment to the signed area.
// Uses the shoelace formula: 0.5 * (x0*y1 - x1*y0)
// lineArea computes the contribution of a line segment to the signed area.
// Uses the shoelace formula: 0.5 * (x0*y1 - x1*y0)
func lineArea(p0, p1 Point) float64 {
	return 0.5 * (p0.X*p1.Y - p1.X*p0.Y)
}

// quadArea computes the contribution of a quadratic Bezier to the signed area.
// Integrates x*dy using the parametric form.
// quadArea computes the contribution of a quadratic Bezier to the signed area.
// Integrates x*dy using the parametric form.
func quadArea(p0, p1, p2 Point) float64 {
	return (p0.X*(2*p1.Y+p2.Y) + p1.X*(-p0.Y+p2.Y) + p2.X*(-2*p1.Y-p0.Y)) / 6.0
}

// cubicArea computes the contribution of a cubic Bezier to the signed area.
// Integrates x*dy using the parametric form and Green's theorem.
// cubicArea computes the contribution of a cubic Bezier to the signed area.
// Integrates x*dy using the parametric form and Green's theorem.
func cubicArea(p0, p1, p2, p3 Point) float64 {
	// Using the formula from the kurbo library:
	return (p0.X*(6*p1.Y+3*p2.Y+p3.Y) +
		3*p1.X*(-2*p0.Y+p2.Y+p3.Y) +
		3*p2.X*(-p0.Y-p1.Y+2*p3.Y) +
		p3.X*(-p0.Y-3*p1.Y-6*p2.Y)) / 20.0
}

// Winding returns the winding number of a point relative to the path.
// 0 = outside, non-zero = inside (for non-zero fill rule).
// Uses ray casting with a horizontal ray to the right.
