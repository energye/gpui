//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import "math"

// PathMeasure provides arc-length parameterization over a Path: given a
// distance along the path, it returns the position and tangent angle. This
// is the general geometry query behind SVG textPath glyph placement.
//
// The segment→cubic mapping follows resvg's usvg text layout (which maps
// every tiny-skia path segment onto a kurbo CubicBez): LineTo becomes a
// cubic through the 1/3–2/3 elevated control points, QuadTo is raised,
// Close draws the closing line back to the subpath start, and MoveTo only
// resets the pen (consecutive subpaths measure as one continuous run —
// resvg semantics). Arc length per cubic uses the adaptive cubicLength
// helper; the per-curve inverse (distance → parameter) uses a 64-step
// chord table with linear interpolation, well inside the 0.5 px placement
// tolerance the reference implementation targets.

// PathMeasure measures one Path.
type PathMeasure struct {
	curves []CubicBez
	cum    []float64 // cumulative arc length; len(curves)+1 entries
	chords [][]float64 // per-curve 65-entry chord-length table (t = i/64)
	total  float64
}

// NewPathMeasure builds the measure over the path (accuracy feeds the
// adaptive per-curve arc length).
func NewPathMeasure(p *Path, accuracy float64) *PathMeasure {
	if accuracy <= 0 {
		accuracy = 0.5
	}
	m := &PathMeasure{cum: []float64{0}}
	var cur Point
	startPt := Point{}
	appendCubic := func(c CubicBez) {
		m.curves = append(m.curves, c)
		m.total += cubicLength(c.P0, c.P1, c.P2, c.P3, accuracy)
		m.cum = append(m.cum, m.total)
		m.chords = append(m.chords, chordTable(c))
	}
	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			cur = Pt(coords[0], coords[1])
			startPt = cur
		case LineTo:
			pt := Pt(coords[0], coords[1])
			// resvg create_curve_from_line: elevate the line to a cubic
			// through the 1/3 and 2/3 points.
			appendCubic(CubicBez{
				P0: cur,
				P1: Pt(cur.X+(pt.X-cur.X)/3, cur.Y+(pt.Y-cur.Y)/3),
				P2: Pt(cur.X+2*(pt.X-cur.X)/3, cur.Y+2*(pt.Y-cur.Y)/3),
				P3: pt,
			})
			cur = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			appendCubic(raiseQuad(cur, ctrl, pt))
			cur = pt
		case CubicTo:
			c1 := Pt(coords[0], coords[1])
			c2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			appendCubic(CubicBez{P0: cur, P1: c1, P2: c2, P3: pt})
			cur = pt
		case ConicTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			w := coords[4]
			if w == 1 {
				appendCubic(raiseQuad(cur, ctrl, pt))
			} else {
				NewConicBez(cur, ctrl, pt, w).ToQuads(accuracy, func(q QuadBez) {
					appendCubic(raiseQuad(q.P0, q.P1, q.P2))
				})
				cur = pt
			}
		case Close:
			// resvg: the closing edge is a line back to the subpath start.
			if cur != startPt {
				appendCubic(CubicBez{
					P0: cur,
					P1: Pt(cur.X+(startPt.X-cur.X)/3, cur.Y+(startPt.Y-cur.Y)/3),
					P2: Pt(cur.X+2*(startPt.X-cur.X)/3, cur.Y+2*(startPt.Y-cur.Y)/3),
					P3: startPt,
				})
				cur = startPt
			}
		}
	})
	return m
}

// chordTable builds the 65-entry cumulative chord-length table (t = i/64).
func chordTable(c CubicBez) []float64 {
	const steps = 64
	tab := make([]float64, steps+1)
	prev := c.Eval(0)
	for i := 1; i <= steps; i++ {
		pt := c.Eval(float64(i) / steps)
		tab[i] = tab[i-1] + prev.Distance(pt)
		prev = pt
	}
	return tab
}

// raiseQuad lifts a quadratic onto its cubic equivalent (the exact
// representation: P1 = P0 + 2/3·(C-P0), P2 = P3 + 2/3·(C-P3)).
func raiseQuad(p0, c, p2 Point) CubicBez {
	return CubicBez{
		P0: p0,
		P1: Pt(p0.X+2.0/3*(c.X-p0.X), p0.Y+2.0/3*(c.Y-p0.Y)),
		P2: Pt(p2.X+2.0/3*(c.X-p2.X), p2.Y+2.0/3*(c.Y-p2.Y)),
		P3: p2,
	}
}

// Length returns the total arc length (subpath closing edges included,
// resvg collect_normals semantics).
func (m *PathMeasure) Length() float64 { return m.total }

// Sample returns the position and tangent angle (degrees) at distance s
// along the path; ok=false when s lies outside [0, total]. The angle is the
// resvg text-on-path rotation: tangent rotated by −90° (the glyph's vertical
// axis aligns with the path normal pointing up-left of travel).
func (m *PathMeasure) Sample(s float64) (x, y, angleDeg float64, ok bool) {
	if len(m.curves) == 0 || s < 0 || s > m.total {
		return 0, 0, 0, false
	}
	// Locate the curve holding s (cum is sorted; linear scan is fine for the
	// segment counts text paths produce).
	i := 0
	for i < len(m.curves)-1 && s > m.cum[i+1] {
		i++
	}
	segStart := m.cum[i]
	segLen := m.cum[i+1] - segStart
	if segLen <= 0 {
		c := m.curves[i]
		return c.P0.X, c.P0.Y, 0, true
	}
	local := (s - segStart) / segLen // param by arc length, approx
	c := m.curves[i]
	// Refine against the chord table: find the chord interval holding
	// local·chordTotal and interpolate t.
	tab := m.chords[i]
	chordTotal := tab[len(tab)-1]
	target := local * chordTotal
	lo, hi := 0, len(tab)-1
	for lo < hi-1 {
		mid := (lo + hi) / 2
		if tab[mid] < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	var t float64
	if d := tab[hi] - tab[lo]; d > 0 {
		t = (float64(lo) + (target-tab[lo])/d) / float64(len(tab)-1)
	} else {
		t = float64(lo) / float64(len(tab)-1)
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	pos := c.Eval(t)
	dv := c.Deriv().Eval(t)
	angle := math.Atan2(dv.Y, dv.X)*180/math.Pi - 90
	return pos.X, pos.Y, angle, true
}
