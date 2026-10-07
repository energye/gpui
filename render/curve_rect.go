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

// NewRect creates a rectangle from two points.
// The points are normalized so Min <= Max.
func NewRect(p1, p2 Point) Rect {
	return Rect{
		Min: Point{X: math.Min(p1.X, p2.X), Y: math.Min(p1.Y, p2.Y)},
		Max: Point{X: math.Max(p1.X, p2.X), Y: math.Max(p1.Y, p2.Y)},
	}
}

// Width returns the width of the rectangle.
// Width returns the width of the rectangle.
func (r Rect) Width() float64 {
	return r.Max.X - r.Min.X
}

// Height returns the height of the rectangle.
// Height returns the height of the rectangle.
func (r Rect) Height() float64 {
	return r.Max.Y - r.Min.Y
}

// Union returns the smallest rectangle containing both r and other.
// Union returns the smallest rectangle containing both r and other.
func (r Rect) Union(other Rect) Rect {
	return Rect{
		Min: Point{X: math.Min(r.Min.X, other.Min.X), Y: math.Min(r.Min.Y, other.Min.Y)},
		Max: Point{X: math.Max(r.Max.X, other.Max.X), Y: math.Max(r.Max.Y, other.Max.Y)},
	}
}

// Contains returns true if the point is inside the rectangle.
// Contains returns true if the point is inside the rectangle.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

// -------------------------------------------------------------------
// Line
// -------------------------------------------------------------------

type Line struct {
	P0, P1 Point
}

// NewLine creates a new line segment.
