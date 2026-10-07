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

// expandBounds updates the incremental bounding box to include (x, y).
func (p *Path) expandBounds(x, y float64) {
	if !p.boundsValid {
		p.boundsMinX, p.boundsMinY = x, y
		p.boundsMaxX, p.boundsMaxY = x, y
		p.boundsValid = true
		return
	}
	if x < p.boundsMinX {
		p.boundsMinX = x
	}
	if y < p.boundsMinY {
		p.boundsMinY = y
	}
	if x > p.boundsMaxX {
		p.boundsMaxX = x
	}
	if y > p.boundsMaxY {
		p.boundsMaxY = y
	}
}

// Bounds returns the axis-aligned bounding box of the path as image.Rectangle.
// Computed incrementally during path construction (O(1) per operation).
// Returns empty rectangle for empty paths.
// Bounds returns the axis-aligned bounding box of the path as image.Rectangle.
// Computed incrementally during path construction (O(1) per operation).
// Returns empty rectangle for empty paths.
func (p *Path) Bounds() image.Rectangle {
	if !p.boundsValid {
		return image.Rectangle{}
	}
	return image.Rect(
		int(math.Floor(p.boundsMinX)),
		int(math.Floor(p.boundsMinY)),
		int(math.Ceil(p.boundsMaxX)),
		int(math.Ceil(p.boundsMaxY)),
	)
}

// MoveTo moves to a point without drawing.
// CurrentPoint returns the current point.
func (p *Path) CurrentPoint() Point {
	return p.current
}

// HasCurrentPoint returns true if the path has a current point.
// A path has a current point after MoveTo, LineTo, or any curve operation.
// HasCurrentPoint returns true if the path has a current point.
// A path has a current point after MoveTo, LineTo, or any curve operation.
func (p *Path) HasCurrentPoint() bool {
	return len(p.verbs) > 0
}

// isEmpty returns true if the path has no elements.
// isEmpty returns true if the path has no elements.
func (p *Path) isEmpty() bool {
	return len(p.verbs) == 0
}

// Transform applies a transformation matrix to all points in the path.
