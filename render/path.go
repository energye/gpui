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

// PathVerb represents a path construction command.
type PathVerb byte

const (
	// MoveTo moves the current point without drawing. Consumes 2 coords (x, y).
	MoveTo PathVerb = iota
	// LineTo draws a line to the specified point. Consumes 2 coords (x, y).
	LineTo
	// QuadTo draws a quadratic Bezier curve. Consumes 4 coords (cx, cy, x, y).
	QuadTo
	// CubicTo draws a cubic Bezier curve. Consumes 6 coords (c1x, c1y, c2x, c2y, x, y).
	CubicTo
	// Close closes the current subpath. Consumes 0 coords.
	Close
	// ConicTo draws a rational quadratic (conic) curve. Consumes 5 coords
	// (cx, cy, x, y, w) where w is the conic weight: w <= 0 degenerates to a
	// straight line at construction time, w == 1 equals a quadratic Bezier,
	// and w == sqrt(2)/2 represents an exact quarter circular arc.
	// ConicTo is appended after Close to keep the serialized values of the
	// original five verbs stable.
	ConicTo
)
