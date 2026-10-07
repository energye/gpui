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
	"sort"
)

// NewCubicBez creates a new cubic Bezier curve.
func NewCubicBez(p0, p1, p2, p3 Point) CubicBez {
	return CubicBez{P0: p0, P1: p1, P2: p2, P3: p3}
}

// Eval evaluates the curve at parameter t (0 to 1) using de Casteljau's algorithm.
// Eval evaluates the curve at parameter t (0 to 1) using de Casteljau's algorithm.
func (c CubicBez) Eval(t float64) Point {
	mt := 1.0 - t
	mt2 := mt * mt
	mt3 := mt2 * mt
	t2 := t * t
	t3 := t2 * t

	return Point{
		X: mt3*c.P0.X + 3*mt2*t*c.P1.X + 3*mt*t2*c.P2.X + t3*c.P3.X,
		Y: mt3*c.P0.Y + 3*mt2*t*c.P1.Y + 3*mt*t2*c.P2.Y + t3*c.P3.Y,
	}
}

// Start returns the starting point of the curve.
// Start returns the starting point of the curve.
func (c CubicBez) Start() Point {
	return c.P0
}

// End returns the ending point of the curve.
// End returns the ending point of the curve.
func (c CubicBez) End() Point {
	return c.P3
}

// Subdivide splits the curve at t=0.5 into two halves using de Casteljau.
// Subdivide splits the curve at t=0.5 into two halves using de Casteljau.
func (c CubicBez) Subdivide() (CubicBez, CubicBez) {
	// De Casteljau subdivision at t=0.5
	p01 := c.P0.Lerp(c.P1, 0.5)
	p12 := c.P1.Lerp(c.P2, 0.5)
	p23 := c.P2.Lerp(c.P3, 0.5)
	p012 := p01.Lerp(p12, 0.5)
	p123 := p12.Lerp(p23, 0.5)
	mid := p012.Lerp(p123, 0.5)

	return CubicBez{P0: c.P0, P1: p01, P2: p012, P3: mid},
		CubicBez{P0: mid, P1: p123, P2: p23, P3: c.P3}
}

// Subsegment returns the portion of the curve from t0 to t1.
// Subsegment returns the portion of the curve from t0 to t1.
func (c CubicBez) Subsegment(t0, t1 float64) CubicBez {
	p0 := c.Eval(t0)
	p3 := c.Eval(t1)

	// Calculate control points using derivative at endpoints
	// The derivative at t is: 3[(1-t)^2 + 2(1-t)t +t^2]
	d0 := c.P1.Sub(c.P0)
	d1 := c.P2.Sub(c.P1)
	d2 := c.P3.Sub(c.P2)

	scale := (t1 - t0) / 3.0

	// Derivative at t0
	mt0 := 1.0 - t0
	deriv0 := Point{
		X: 3 * (d0.X*mt0*mt0 + 2*d1.X*mt0*t0 + d2.X*t0*t0),
		Y: 3 * (d0.Y*mt0*mt0 + 2*d1.Y*mt0*t0 + d2.Y*t0*t0),
	}
	p1 := Point{
		X: p0.X + scale*deriv0.X,
		Y: p0.Y + scale*deriv0.Y,
	}

	// Derivative at t1
	mt1 := 1.0 - t1
	deriv1 := Point{
		X: 3 * (d0.X*mt1*mt1 + 2*d1.X*mt1*t1 + d2.X*t1*t1),
		Y: 3 * (d0.Y*mt1*mt1 + 2*d1.Y*mt1*t1 + d2.Y*t1*t1),
	}
	p2 := Point{
		X: p3.X - scale*deriv1.X,
		Y: p3.Y - scale*deriv1.Y,
	}

	return CubicBez{P0: p0, P1: p1, P2: p2, P3: p3}
}

// Extrema returns parameter values where the derivative is zero (extrema points).
// For a cubic Bezier, there can be up to 4 extrema (2 for x, 2 for y).
// Extrema returns parameter values where the derivative is zero (extrema points).
// For a cubic Bezier, there can be up to 4 extrema (2 for x, 2 for y).
func (c CubicBez) Extrema() []float64 {
	// Pre-allocate for max 4 extrema (2 for x, 2 for y)
	result := make([]float64, 0, 4)

	// The derivative is a quadratic: B'(t) = a*t^2 + b*t + c
	// Where the coefficients come from differentiating the Bernstein form
	d0 := c.P1.Sub(c.P0)
	d1 := c.P2.Sub(c.P1)
	d2 := c.P3.Sub(c.P2)

	// X extrema: solve d0.X - 2*d1.X + d2.X = 0
	ax := d0.X - 2*d1.X + d2.X
	bx := 2 * (d1.X - d0.X)
	cx := d0.X

	result = append(result, SolveQuadraticInUnitInterval(ax, bx, cx)...)

	// Y extrema
	ay := d0.Y - 2*d1.Y + d2.Y
	by := 2 * (d1.Y - d0.Y)
	cy := d0.Y

	result = append(result, SolveQuadraticInUnitInterval(ay, by, cy)...)

	sort.Float64s(result)
	return result
}

// BoundingBox returns the tight axis-aligned bounding box of the curve.
// BoundingBox returns the tight axis-aligned bounding box of the curve.
func (c CubicBez) BoundingBox() Rect {
	// Start with endpoints
	bbox := NewRect(c.P0, c.P3)

	// Include extrema points
	for _, t := range c.Extrema() {
		p := c.Eval(t)
		bbox = bbox.Union(NewRect(p, p))
	}

	return bbox
}

// Inflections returns the parameter values of inflection points.
// An inflection point is where the curvature changes sign.
// A cubic can have 0, 1, or 2 inflection points.
// Inflections returns the parameter values of inflection points.
// An inflection point is where the curvature changes sign.
// A cubic can have 0, 1, or 2 inflection points.
func (c CubicBez) Inflections() []float64 {
	// See https://www.caffeineowl.com/graphics/2d/vectorial/cubic-inflexion.html
	a := c.P1.Sub(c.P0)
	b := c.P2.Sub(c.P1).Sub(a)
	cc := c.P3.Sub(c.P0).Sub(c.P2.Sub(c.P1).Mul(3))

	// Cross products for the quadratic equation
	// Solves: crossBC * t^2 + crossAC * t + crossAB = 0
	crossAB := a.Cross(b)
	crossAC := a.Cross(cc)
	crossBC := b.Cross(cc)

	// Note: SolveQuadratic expects a*t^2 + b*t + c = 0
	roots := SolveQuadratic(crossBC, crossAC, crossAB)

	var result []float64
	for _, t := range roots {
		if t >= 0 && t <= 1 {
			result = append(result, t)
		}
	}

	sort.Float64s(result)
	return result
}

// Deriv returns the derivative curve (a quadratic Bezier).
// The derivative gives the tangent direction at any point.
// Deriv returns the derivative curve (a quadratic Bezier).
// The derivative gives the tangent direction at any point.
func (c CubicBez) Deriv() QuadBez {
	return QuadBez{
		P0: Point{X: 3 * (c.P1.X - c.P0.X), Y: 3 * (c.P1.Y - c.P0.Y)},
		P1: Point{X: 3 * (c.P2.X - c.P1.X), Y: 3 * (c.P2.Y - c.P1.Y)},
		P2: Point{X: 3 * (c.P3.X - c.P2.X), Y: 3 * (c.P3.Y - c.P2.Y)},
	}
}

// Tangent returns the tangent vector at parameter t.
// Tangent returns the tangent vector at parameter t.
func (c CubicBez) Tangent(t float64) Vec2 {
	deriv := c.Deriv()
	p := deriv.Eval(t)
	return Vec2(p)
}

// Normal returns the normal vector (perpendicular to tangent) at parameter t.
// Normal returns the normal vector (perpendicular to tangent) at parameter t.
func (c CubicBez) Normal(t float64) Vec2 {
	tan := c.Tangent(t)
	return tan.Perp().Normalize()
}
