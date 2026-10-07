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

// String returns a human-readable name for the verb.
func (v PathVerb) String() string {
	switch v {
	case MoveTo:
		return "MoveTo"
	case LineTo:
		return "LineTo"
	case QuadTo:
		return "QuadTo"
	case CubicTo:
		return "CubicTo"
	case Close:
		return "Close"
	default:
		return "Unknown"
	}
}

// verbCoordCount returns the number of float64 coordinates consumed by a verb.
// verbCoordCount returns the number of float64 coordinates consumed by a verb.
func verbCoordCount(v PathVerb) int {
	switch v {
	case MoveTo, LineTo:
		return 2
	case QuadTo:
		return 4
	case CubicTo:
		return 6
	case Close:
		return 0
	default:
		return 0
	}
}

// Path represents a vector path using SOA (Structure of Arrays) layout.
//
// Internally, the path stores verbs and coordinates in separate contiguous slices
// for cache efficiency and zero per-verb heap allocations.
type Path struct {
	verbs   []PathVerb
	coords  []float64
	start   Point // Starting point of current subpath
	current Point // Current point

	// Incremental bounding box.
	// O(1) per path operation, zero extra cost vs computing at Fill() time.
	boundsMinX, boundsMinY float64
	boundsMaxX, boundsMaxY float64
	boundsValid            bool

	// Embedded stack buffers for small paths (≤32 verbs, ≤10 cubics).
	// Avoids heap allocation for typical shapes (rect=5, circle=7, icon≤20 verbs).
	// Slices point to these buffers initially; grow to heap only if exceeded.
	verbsBuf  [32]PathVerb
	coordsBuf [96]float64 // 96 = 16 cubics × 6 coords, or 48 lines × 2 coords
}

// NewPath creates a new empty path with stack-backed buffers.
// NewPath creates a new empty path with stack-backed buffers.
func NewPath() *Path {
	p := &Path{}
	p.verbs = p.verbsBuf[:0]
	p.coords = p.coordsBuf[:0]
	return p
}

// expandBounds updates the incremental bounding box to include (x, y).
// Iterate calls fn for each verb in the path with the corresponding coordinate slice.
// This is the primary zero-allocation iteration API.
//
// The coords slice passed to fn is a sub-slice of the path's coordinate buffer:
//   - MoveTo: coords has 2 elements (x, y)
//   - LineTo: coords has 2 elements (x, y)
//   - QuadTo: coords has 4 elements (cx, cy, x, y)
//   - CubicTo: coords has 6 elements (c1x, c1y, c2x, c2y, x, y)
//   - Close: coords has 0 elements (nil)
func (p *Path) Iterate(fn func(verb PathVerb, coords []float64)) {
	ci := 0
	for _, v := range p.verbs {
		n := verbCoordCount(v)
		if n > 0 {
			fn(v, p.coords[ci:ci+n])
		} else {
			fn(v, nil)
		}
		ci += n
	}
}

// Verbs returns the verb stream. The returned slice must not be modified.
// Verbs returns the verb stream. The returned slice must not be modified.
func (p *Path) Verbs() []PathVerb {
	return p.verbs
}

// Coords returns the coordinate stream. The returned slice must not be modified.
// Coords returns the coordinate stream. The returned slice must not be modified.
func (p *Path) Coords() []float64 {
	return p.coords
}

// NumVerbs returns the number of verbs in the path.
// NumVerbs returns the number of verbs in the path.
func (p *Path) NumVerbs() int {
	return len(p.verbs)
}

// HasCurves reports whether the path contains any quadratic or cubic curves.
// HasCurves reports whether the path contains any quadratic or cubic curves.
func (p *Path) HasCurves() bool {
	for _, v := range p.verbs {
		if v == QuadTo || v == CubicTo {
			return true
		}
	}
	return false
}

// CurrentPoint returns the current point.
