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

import ()

// NewLine creates a new line segment.
func NewLine(p0, p1 Point) Line {
	return Line{P0: p0, P1: p1}
}

// Eval evaluates the line at parameter t (0 to 1).
// Eval evaluates the line at parameter t (0 to 1).
func (l Line) Eval(t float64) Point {
	return l.P0.Lerp(l.P1, t)
}

// Start returns the starting point of the line.
// Start returns the starting point of the line.
func (l Line) Start() Point {
	return l.P0
}

// End returns the ending point of the line.
// End returns the ending point of the line.
func (l Line) End() Point {
	return l.P1
}

// Subdivide splits the line at t=0.5 into two halves.
// Subdivide splits the line at t=0.5 into two halves.
func (l Line) Subdivide() (Line, Line) {
	mid := l.Eval(0.5)
	return Line{P0: l.P0, P1: mid}, Line{P0: mid, P1: l.P1}
}

// Subsegment returns the portion of the line from t0 to t1.
// Subsegment returns the portion of the line from t0 to t1.
func (l Line) Subsegment(t0, t1 float64) Line {
	return Line{
		P0: l.Eval(t0),
		P1: l.Eval(t1),
	}
}

// BoundingBox returns the axis-aligned bounding box of the line.
// BoundingBox returns the axis-aligned bounding box of the line.
func (l Line) BoundingBox() Rect {
	return NewRect(l.P0, l.P1)
}

// Length returns the length of the line segment.
// Length returns the length of the line segment.
func (l Line) Length() float64 {
	return l.P0.Distance(l.P1)
}

// Midpoint returns the midpoint of the line segment.
// Midpoint returns the midpoint of the line segment.
func (l Line) Midpoint() Point {
	return l.Eval(0.5)
}

// Reversed returns a copy of the line with endpoints swapped.
// Reversed returns a copy of the line with endpoints swapped.
func (l Line) Reversed() Line {
	return Line{P0: l.P1, P1: l.P0}
}

// -------------------------------------------------------------------
// QuadBez - Quadratic Bezier Curve
// -------------------------------------------------------------------

type QuadBez struct {
	P0, P1, P2 Point
}

// NewQuadBez creates a new quadratic Bezier curve.
