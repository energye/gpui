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

// NewQuadBez creates a new quadratic Bezier curve.
func NewQuadBez(p0, p1, p2 Point) QuadBez {
	return QuadBez{P0: p0, P1: p1, P2: p2}
}

// Eval evaluates the curve at parameter t (0 to 1) using de Casteljau's algorithm.
// Eval evaluates the curve at parameter t (0 to 1) using de Casteljau's algorithm.
func (q QuadBez) Eval(t float64) Point {
	mt := 1.0 - t
	return Point{
		X: mt*mt*q.P0.X + 2*mt*t*q.P1.X + t*t*q.P2.X,
		Y: mt*mt*q.P0.Y + 2*mt*t*q.P1.Y + t*t*q.P2.Y,
	}
}

// Start returns the starting point of the curve.
// Start returns the starting point of the curve.
func (q QuadBez) Start() Point {
	return q.P0
}

// End returns the ending point of the curve.
// End returns the ending point of the curve.
func (q QuadBez) End() Point {
	return q.P2
}

// Subdivide splits the curve at t=0.5 into two halves using de Casteljau.
// Subdivide splits the curve at t=0.5 into two halves using de Casteljau.
func (q QuadBez) Subdivide() (QuadBez, QuadBez) {
	mid := q.Eval(0.5)
	return QuadBez{
			P0: q.P0,
			P1: q.P0.Lerp(q.P1, 0.5),
			P2: mid,
		}, QuadBez{
			P0: mid,
			P1: q.P1.Lerp(q.P2, 0.5),
			P2: q.P2,
		}
}

// Subsegment returns the portion of the curve from t0 to t1.
// Subsegment returns the portion of the curve from t0 to t1.
func (q QuadBez) Subsegment(t0, t1 float64) QuadBez {
	p0 := q.Eval(t0)
	p2 := q.Eval(t1)

	// Calculate the control point for the subsegment
	// Using the property that the tangent at any point is the lerp of
	// the control polygon edges
	d0 := q.P1.Sub(q.P0)
	d1 := q.P2.Sub(q.P1)
	dt := (t1 - t0)

	// Tangent direction at t0, scaled for the new segment
	tanDir := Point{
		X: d0.X + t0*(d1.X-d0.X),
		Y: d0.Y + t0*(d1.Y-d0.Y),
	}
	p1 := Point{
		X: p0.X + dt*tanDir.X,
		Y: p0.Y + dt*tanDir.Y,
	}

	return QuadBez{P0: p0, P1: p1, P2: p2}
}

// Extrema returns parameter values where the derivative is zero (extrema points).
// Used for computing tight bounding boxes.
// Extrema returns parameter values where the derivative is zero (extrema points).
// Used for computing tight bounding boxes.
func (q QuadBez) Extrema() []float64 {
	var result []float64

	// For a quadratic Bezier, the derivative is linear:
	// B'(t) = 2[ + t]
	// Setting to zero: t = /

	d0 := q.P1.Sub(q.P0)
	d1 := q.P2.Sub(q.P1)
	dd := Point{X: d1.X - d0.X, Y: d1.Y - d0.Y}

	// X extrema
	if dd.X != 0 {
		t := -d0.X / dd.X
		if t > 0 && t < 1 {
			result = append(result, t)
		}
	}

	// Y extrema
	if dd.Y != 0 {
		t := -d0.Y / dd.Y
		if t > 0 && t < 1 {
			result = append(result, t)
		}
	}

	sort.Float64s(result)
	return result
}

// BoundingBox returns the tight axis-aligned bounding box of the curve.
// BoundingBox returns the tight axis-aligned bounding box of the curve.
func (q QuadBez) BoundingBox() Rect {
	// Start with endpoints
	bbox := NewRect(q.P0, q.P2)

	// Include extrema points
	for _, t := range q.Extrema() {
		p := q.Eval(t)
		bbox = bbox.Union(NewRect(p, p))
	}

	return bbox
}

// Raise elevates the quadratic to a cubic Bezier curve.
// Returns an exact cubic representation of this quadratic.
// Raise elevates the quadratic to a cubic Bezier curve.
// Returns an exact cubic representation of this quadratic.
func (q QuadBez) Raise() CubicBez {
	return CubicBez{
		P0: q.P0,
		P1: Point{
			X: q.P0.X + (2.0/3.0)*(q.P1.X-q.P0.X),
			Y: q.P0.Y + (2.0/3.0)*(q.P1.Y-q.P0.Y),
		},
		P2: Point{
			X: q.P2.X + (2.0/3.0)*(q.P1.X-q.P2.X),
			Y: q.P2.Y + (2.0/3.0)*(q.P1.Y-q.P2.Y),
		},
		P3: q.P2,
	}
}

// -------------------------------------------------------------------
// CubicBez - Cubic Bezier Curve
// -------------------------------------------------------------------

type CubicBez struct {
	P0, P1, P2, P3 Point
}

// NewCubicBez creates a new cubic Bezier curve.
