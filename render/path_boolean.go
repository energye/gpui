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
// library) of the regions covered by a and b: curves are flattened with a
// 0.1px tolerance, region membership follows the non-zero winding rule, and
// the returned contours are wound so the path is valid under both
// FillRuleNonZero and FillRuleEvenOdd. Output geometry is preserved to
// 1/1024px and is not capped by region size.
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

	subject := pathToClipperPaths(a)
	if len(subject) == 0 {
		// a holds no closed region (e.g. a bare line): behaves as empty.
		if op == PathOpUnion || op == PathOpXor {
			return b.Clone()
		}
		return NewPath()
	}
	clip := pathToClipperPaths(b)
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
	return clipperPathsToPath(solution)
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

// addClipperPoint appends (x, y) to path unless it repeats the previous point.
func addClipperPoint(path *clipper.Path, x, y float64) {
	pt := clipper.IntPoint{X: pathBooleanCoord(x), Y: pathBooleanCoord(y)}
	if n := len(*path); n > 0 && (*path)[n-1].X == pt.X && (*path)[n-1].Y == pt.Y {
		return
	}
	*path = append(*path, &pt)
}

// pathToClipperPaths flattens p into closed polygonal contours for Clipper.
// Contours with fewer than three distinct points carry no region and are dropped.
func pathToClipperPaths(p *Path) clipper.Paths {
	var out clipper.Paths
	var current, start Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			out = append(out, clipper.NewPath())
			current = Pt(coords[0], coords[1])
			start = current
			addClipperPoint(&out[len(out)-1], coords[0], coords[1])
		case LineTo:
			if len(out) == 0 {
				return
			}
			current = Pt(coords[0], coords[1])
			addClipperPoint(&out[len(out)-1], coords[0], coords[1])
		case QuadTo:
			if len(out) == 0 {
				return
			}
			end := Pt(coords[2], coords[3])
			seg := &out[len(out)-1]
			flattenQuad(current, Pt(coords[0], coords[1]), end, pathBooleanFlattenTolerance, func(pt Point) {
				addClipperPoint(seg, pt.X, pt.Y)
			})
			current = end
		case CubicTo:
			if len(out) == 0 {
				return
			}
			end := Pt(coords[4], coords[5])
			seg := &out[len(out)-1]
			flattenCubic(current, Pt(coords[0], coords[1]), Pt(coords[2], coords[3]), end, pathBooleanFlattenTolerance, func(pt Point) {
				addClipperPoint(seg, pt.X, pt.Y)
			})
			current = end
		case Close:
			if len(out) == 0 {
				return
			}
			// Mirror Path.FlattenCallback: Close draws a line back to the
			// subpath start. Clipper would close the contour implicitly, but
			// emitting the point keeps the polygon identical to the one the
			// renderer rasterizes; addClipperPoint drops it when already there.
			if current != start {
				addClipperPoint(&out[len(out)-1], start.X, start.Y)
			}
			current = start
		}
	})

	kept := out[:0]
	for _, c := range out {
		if len(c) >= 3 {
			kept = append(kept, c)
		}
	}
	return kept
}

// clipperPathsToPath converts Clipper solution contours back into a Path.
// Contour orientation is preserved: Clipper winds outer contours and holes
// oppositely, which keeps the result valid under both fill rules.
func clipperPathsToPath(paths clipper.Paths) *Path {
	out := NewPath()
	for _, c := range paths {
		if len(c) < 3 {
			continue
		}
		out.MoveTo(float64(c[0].X)*pathBooleanInvScale, float64(c[0].Y)*pathBooleanInvScale)
		for _, pt := range c[1:] {
			out.LineTo(float64(pt.X)*pathBooleanInvScale, float64(pt.Y)*pathBooleanInvScale)
		}
		out.Close()
	}
	return out
}
