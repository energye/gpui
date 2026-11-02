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

	"github.com/ctessum/go.clipper"
)

// PathBooleanOp selects a path boolean operation (H.04 / SkPath::Op).
type PathBooleanOp int

const (
	PathOpUnion PathBooleanOp = iota
	PathOpIntersect
	PathOpDifference
	PathOpXor
)

// Boolean path operations run on the Clipper library (Vatti scan-line polygon
// clipping). Path coordinates are mapped onto Clipper's integer grid at
// pathBooleanScale units per pixel, so output geometry is exact to within
// 1/(2*pathBooleanScale) px instead of being quantized to the pixel grid.
const (
	// pathBooleanScale is the number of Clipper integer units per path unit.
	pathBooleanScale = 1024.0
	// pathBooleanInvScale converts Clipper integer units back to path units.
	pathBooleanInvScale = 1.0 / pathBooleanScale
	// pathBooleanFlattenTolerance flattens input curves to within this many
	// pixels (matches Path.Flatten's default tolerance).
	pathBooleanFlattenTolerance = 0.1
	// pathBooleanMaxCoord caps input coordinates. Beyond it Clipper leaves its
	// fast 64-bit integer range; 1e6 px is far above any UI scale.
	pathBooleanMaxCoord = 1.0e6
)

// BooleanPath computes a path representing a boolean combination of a and b.
//
// The result is the exact geometric boolean (Vatti scan-line, via the Clipper
// library) of the regions covered by a and b: the inputs are flattened with a
// 0.1px tolerance to place the intersections, region membership follows the
// non-zero winding rule, and the returned contours are wound so the path is
// valid under both FillRuleNonZero and FillRuleEvenOdd. Output geometry is
// preserved to 1/1024px and is not capped by region size.
//
// Curve segments survive the operation (P3.5 批 5, Skia SkPath::Op semantics):
// the flattened samples carry their source segment id and parameter, and every
// run of consecutive samples in the solution comes back as that segment's exact
// subsegment (QuadTo/CubicTo/ConicTo). Straight edges and the vertices Clipper
// created at intersections stay lines.
func BooleanPath(a, b *Path, op PathBooleanOp) *Path {
	if a == nil || a.isEmpty() {
		if op == PathOpUnion || op == PathOpXor {
			if b == nil {
				return NewPath()
			}
			return b.Clone()
		}
		return NewPath()
	}
	if b == nil || b.isEmpty() {
		if op == PathOpUnion || op == PathOpDifference || op == PathOpXor {
			return a.Clone()
		}
		return NewPath()
	}

	subject, traceA := pathToClipperPathsTraced(a)
	if len(subject) == 0 {
		// a holds no closed region (e.g. a bare line): behaves as empty.
		if op == PathOpUnion || op == PathOpXor {
			return b.Clone()
		}
		return NewPath()
	}
	clip, traceB := pathToClipperPathsTraced(b)
	if len(clip) == 0 {
		if op == PathOpUnion || op == PathOpDifference || op == PathOpXor {
			return a.Clone()
		}
		return NewPath()
	}

	c := clipper.NewClipper(clipper.IoNone)
	c.AddPaths(subject, clipper.PtSubject, true)
	c.AddPaths(clip, clipper.PtClip, true)
	solution, ok := c.Execute1(clipTypeFor(op), clipper.PftNonZero, clipper.PftNonZero)
	if !ok {
		return NewPath()
	}
	return refitClipperPaths(solution, traceA, traceB)
}

// Op returns BooleanPath(p, other, op).
func (p *Path) Op(other *Path, op PathBooleanOp) *Path {
	return BooleanPath(p, other, op)
}

// clipTypeFor maps a PathBooleanOp onto Clipper's ClipType.
func clipTypeFor(op PathBooleanOp) clipper.ClipType {
	switch op {
	case PathOpIntersect:
		return clipper.CtIntersection
	case PathOpDifference:
		return clipper.CtDifference
	case PathOpXor:
		return clipper.CtXor
	default:
		return clipper.CtUnion
	}
}

// pathBooleanCoord maps one coordinate onto Clipper's integer grid.
// Non-finite values collapse to 0 and out-of-range values are clamped so the
// Clipper input never leaves its supported coordinate range.
func pathBooleanCoord(v float64) clipper.CInt {
	if math.IsNaN(v) {
		v = 0
	}
	if v > pathBooleanMaxCoord {
		v = pathBooleanMaxCoord
	} else if v < -pathBooleanMaxCoord {
		v = -pathBooleanMaxCoord
	}
	return clipper.CInt(math.Round(v * pathBooleanScale))
}
