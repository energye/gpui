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

import "math"

// flattenCubicRecursive recursively subdivides the cubic.
func flattenCubicRecursive(c CubicBez, toleranceSq float64, fn func(pt Point), depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		fn(c.P3)
		return
	}

	// Flatness test using the standard cubic flatness metric
	flatness := cubicFlatness(c)

	if flatness <= toleranceSq*16 { // Adjust for the metric scale
		fn(c.P3)
		return
	}

	// Subdivide
	c1, c2 := c.Subdivide()
	flattenCubicRecursive(c1, toleranceSq, fn, depth+1)
	flattenCubicRecursive(c2, toleranceSq, fn, depth+1)
}

// Reversed returns a new path with reversed direction.
// Each subpath is reversed independently.
// Reversed returns a new path with reversed direction.
// Each subpath is reversed independently.
func (p *Path) Reversed() *Path {
	if len(p.verbs) == 0 {
		return NewPath()
	}

	// Collect subpaths
	subpaths := p.collectSubpaths()

	// Reverse each subpath and build new path
	result := NewPath()
	for _, sp := range subpaths {
		reverseSubpath(sp, result)
	}

	return result
}

// soaVerbEntry tracks a verb and its coordinate offset for reverse iteration.
type soaVerbEntry struct {
	verb PathVerb
	off  int // offset into coords slice
}

// subpathSOA represents a single subpath using SOA (verbs + coords).
type subpathSOA struct {
	verbs  []PathVerb
	coords []float64
	closed bool
}

// collectSubpaths splits the path into separate subpaths using SOA iteration.
// collectSubpaths splits the path into separate subpaths using SOA iteration.
func (p *Path) collectSubpaths() []subpathSOA {
	var subpaths []subpathSOA
	var current subpathSOA

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			if len(current.verbs) > 0 {
				subpaths = append(subpaths, current)
			}
			current = subpathSOA{
				verbs:  []PathVerb{MoveTo},
				coords: []float64{coords[0], coords[1]},
			}
		case Close:
			current.closed = true
			subpaths = append(subpaths, current)
			current = subpathSOA{}
		default:
			current.verbs = append(current.verbs, verb)
			current.coords = append(current.coords, coords...)
		}
	})

	if len(current.verbs) > 0 {
		subpaths = append(subpaths, current)
	}

	return subpaths
}

// reverseSubpath reverses a single subpath and appends to result.
// reverseSubpath reverses a single subpath and appends to result.
func reverseSubpath(sp subpathSOA, result *Path) {
	if len(sp.verbs) == 0 {
		return
	}

	// Get the endpoint of the subpath.
	endPoint := getSubpathEndpointSOA(sp)
	result.MoveTo(endPoint.X, endPoint.Y)

	// Build an index of coord offsets per verb for reverse iteration.
	entries := make([]soaVerbEntry, len(sp.verbs))
	ci := 0
	for i, v := range sp.verbs {
		entries[i] = soaVerbEntry{verb: v, off: ci}
		ci += verbCoordCount(v)
	}

	// Reverse elements (skip index 0 which is MoveTo).
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		c := sp.coords[e.off:]
		prevPoint := getSOAElementStartPoint(sp, entries, i)

		switch e.verb {
		case MoveTo:
			continue
		case LineTo:
			result.LineTo(prevPoint.X, prevPoint.Y)
		case QuadTo:
			// Reverse quadratic: keep control, swap endpoint
			result.QuadraticTo(c[0], c[1], prevPoint.X, prevPoint.Y)
		case CubicTo:
			// Reverse cubic: swap control points and endpoint
			result.CubicTo(c[2], c[3], c[0], c[1], prevPoint.X, prevPoint.Y)
		}
	}

	if sp.closed {
		result.Close()
	}
}

// getSubpathEndpointSOA returns the endpoint of a subpath.
// getSubpathEndpointSOA returns the endpoint of a subpath.
func getSubpathEndpointSOA(sp subpathSOA) Point {
	if len(sp.verbs) == 0 {
		return Point{}
	}

	// Walk to find the last verb's endpoint.
	ci := 0
	var lastPt Point
	for _, v := range sp.verbs {
		n := verbCoordCount(v)
		if n >= 2 {
			lastPt = Pt(sp.coords[ci+n-2], sp.coords[ci+n-1])
		}
		ci += n
	}
	return lastPt
}

// getSOAElementStartPoint returns the start point of element at index i.
// getSOAElementStartPoint returns the start point of element at index i.
func getSOAElementStartPoint(sp subpathSOA, entries []soaVerbEntry, i int) Point {
	if i == 0 {
		if sp.verbs[0] == MoveTo {
			return Pt(sp.coords[0], sp.coords[1])
		}
		return Point{}
	}

	// Get endpoint of previous element.
	prev := entries[i-1]
	n := verbCoordCount(prev.verb)
	if n >= 2 {
		return Pt(sp.coords[prev.off+n-2], sp.coords[prev.off+n-1])
	}
	return Point{}
}

// Length returns the total arc length of the path.
// accuracy controls the precision of the approximation (smaller = more accurate).
// Length returns the total arc length of the path.
// accuracy controls the precision of the approximation (smaller = more accurate).
func (p *Path) Length(accuracy float64) float64 {
	if accuracy <= 0 {
		accuracy = 0.001 // Default accuracy
	}

	var length float64
	var current Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			current = Pt(coords[0], coords[1])
		case LineTo:
			pt := Pt(coords[0], coords[1])
			length += current.Distance(pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			length += quadLength(current, ctrl, pt, accuracy)
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			length += cubicLength(current, ctrl1, ctrl2, pt, accuracy)
			current = pt
		case Close:
			// Historical Length() does not add the closing edge (callers that need
			// closed perimeter use TotalLength / ComputeMetrics).
		}
	})

	return length
}

// quadLength computes the arc length of a quadratic Bezier.
// Uses adaptive subdivision.
// quadLength computes the arc length of a quadratic Bezier.
// Uses adaptive subdivision.
func quadLength(p0, p1, p2 Point, accuracy float64) float64 {
	q := NewQuadBez(p0, p1, p2)
	return quadLengthRecursive(q, accuracy*accuracy, 0)
}

// quadLengthRecursive recursively computes quadratic arc length.
// quadLengthRecursive recursively computes quadratic arc length.
func quadLengthRecursive(q QuadBez, accuracySq float64, depth int) float64 {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 16 {
		return q.P0.Distance(q.P2)
	}

	// Compute chord length and control polygon length
	chord := q.P0.Distance(q.P2)
	polygon := q.P0.Distance(q.P1) + q.P1.Distance(q.P2)

	// If they're close enough, use the average
	diff := polygon - chord
	if diff*diff <= accuracySq {
		return (chord + polygon) / 2
	}

	// Subdivide
	q1, q2 := q.Subdivide()
	return quadLengthRecursive(q1, accuracySq, depth+1) + quadLengthRecursive(q2, accuracySq, depth+1)
}

// cubicLength computes the arc length of a cubic Bezier.
// Uses adaptive subdivision.
// cubicLength computes the arc length of a cubic Bezier.
// Uses adaptive subdivision.
func cubicLength(p0, p1, p2, p3 Point, accuracy float64) float64 {
	c := NewCubicBez(p0, p1, p2, p3)
	return cubicLengthRecursive(c, accuracy*accuracy, 0)
}

// cubicLengthRecursive recursively computes cubic arc length.
// cubicLengthRecursive recursively computes cubic arc length.
func cubicLengthRecursive(c CubicBez, accuracySq float64, depth int) float64 {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 16 {
		return c.P0.Distance(c.P3)
	}

	// Compute chord length and control polygon length
	chord := c.P0.Distance(c.P3)
	polygon := c.P0.Distance(c.P1) + c.P1.Distance(c.P2) + c.P2.Distance(c.P3)

	// If they're close enough, use the average
	diff := polygon - chord
	if diff*diff <= accuracySq {
		return (chord + polygon) / 2
	}

	// Subdivide
	c1, c2 := c.Subdivide()
	return cubicLengthRecursive(c1, accuracySq, depth+1) + cubicLengthRecursive(c2, accuracySq, depth+1)
}

// Trim returns a new open path containing the portion of this path between
// normalized arc-length parameters t0 and t1 in [0,1].
// Curves are approximated via Flatten. Multi-subpath paths are concatenated in order.
// If t1<=t0 or the path has no measurable length, returns an empty path.
// Trim returns a new open path containing the portion of this path between
// normalized arc-length parameters t0 and t1 in [0,1].
// Curves are approximated via Flatten. Multi-subpath paths are concatenated in order.
// If t1<=t0 or the path has no measurable length, returns an empty path.
func (p *Path) Trim(t0, t1 float64) *Path {
	out := NewPath()
	if p == nil || len(p.verbs) == 0 {
		return out
	}
	if t0 < 0 {
		t0 = 0
	}
	if t1 > 1 {
		t1 = 1
	}
	if t1 <= t0 {
		return out
	}
	pts := p.Flatten(0.25)
	if len(pts) < 2 {
		return out
	}
	// Build cumulative lengths
	segLen := make([]float64, len(pts)-1)
	var total float64
	for i := 0; i < len(pts)-1; i++ {
		d := pts[i].Distance(pts[i+1])
		segLen[i] = d
		total += d
	}
	if total <= 1e-9 {
		return out
	}
	startDist := t0 * total
	endDist := t1 * total

	pointAt := func(dist float64) Point {
		if dist <= 0 {
			return pts[0]
		}
		if dist >= total {
			return pts[len(pts)-1]
		}
		acc := 0.0
		for i := 0; i < len(segLen); i++ {
			if acc+segLen[i] >= dist {
				// interpolate on segment i
				t := 0.0
				if segLen[i] > 1e-12 {
					t = (dist - acc) / segLen[i]
				}
				return Pt(
					pts[i].X+(pts[i+1].X-pts[i].X)*t,
					pts[i].Y+(pts[i+1].Y-pts[i].Y)*t,
				)
			}
			acc += segLen[i]
		}
		return pts[len(pts)-1]
	}

	startPt := pointAt(startDist)
	out.MoveTo(startPt.X, startPt.Y)
	acc := 0.0
	// Emit vertices strictly inside (startDist, endDist] and final end point.
	for i := 0; i < len(segLen); i++ {
		segStart := acc
		segEnd := acc + segLen[i]
		acc = segEnd
		if segEnd <= startDist {
			continue
		}
		if segStart >= endDist {
			break
		}
		// endpoint of this segment if inside range
		if segEnd <= endDist+1e-9 {
			if segEnd > startDist {
				out.LineTo(pts[i+1].X, pts[i+1].Y)
			}
		} else {
			// end falls inside this segment
			endPt := pointAt(endDist)
			out.LineTo(endPt.X, endPt.Y)
			break
		}
	}
	return out
}

// WithCorners returns a new path where sharp polyline corners are rounded with
// the given radius (E.02 CornerPathEffect subset). Curves are first flattened.
// radius <= 0 returns a clone of the flattened polyline path.
// WithCorners returns a new path where sharp polyline corners are rounded with
// the given radius (E.02 CornerPathEffect subset). Curves are first flattened.
// radius <= 0 returns a clone of the flattened polyline path.
func (p *Path) WithCorners(radius float64) *Path {
	out := NewPath()
	if p == nil || len(p.verbs) == 0 {
		return out
	}
	pts := p.Flatten(0.35)
	if len(pts) < 2 {
		return out
	}
	// Detect closed: last point equals first
	closed := len(pts) >= 3 && pts[0].Distance(pts[len(pts)-1]) < 1e-6
	if closed && len(pts) > 1 {
		pts = pts[:len(pts)-1] // drop duplicate close point
	}
	n := len(pts)
	if n < 2 {
		return out
	}
	if radius <= 0 || n < 3 {
		out.MoveTo(pts[0].X, pts[0].Y)
		for i := 1; i < n; i++ {
			out.LineTo(pts[i].X, pts[i].Y)
		}
		if closed {
			out.Close()
		}
		return out
	}

	// Helper: unit vector from a->b
	unit := func(a, b Point) (Point, float64) {
		dx, dy := b.X-a.X, b.Y-a.Y
		l := math.Hypot(dx, dy)
		if l < 1e-12 {
			return Point{}, 0
		}
		return Pt(dx/l, dy/l), l
	}

	// Process each vertex; for open path, first/last stay sharp.
	type cornerCut struct {
		enter, leave Point // points on adjacent edges where curve starts/ends
		has          bool
	}
	cuts := make([]cornerCut, n)
	for i := 0; i < n; i++ {
		if !closed && (i == 0 || i == n-1) {
			continue
		}
		prev := pts[(i-1+n)%n]
		cur := pts[i]
		next := pts[(i+1)%n]
		uIn, lenIn := unit(prev, cur)
		uOut, lenOut := unit(cur, next)
		if lenIn == 0 || lenOut == 0 {
			continue
		}
		// max cut limited by half adjacent segment lengths
		r := radius
		maxCut := math.Min(lenIn, lenOut) * 0.5
		if r > maxCut {
			r = maxCut
		}
		if r <= 1e-6 {
			continue
		}
		cuts[i] = cornerCut{
			enter: Pt(cur.X-uIn.X*r, cur.Y-uIn.Y*r),
			leave: Pt(cur.X+uOut.X*r, cur.Y+uOut.Y*r),
			has:   true,
		}
	}

	// Build path
	start := pts[0]
	if cuts[0].has {
		start = cuts[0].leave
	}
	out.MoveTo(start.X, start.Y)
	for i := 1; i < n; i++ {
		if cuts[i].has {
			out.LineTo(cuts[i].enter.X, cuts[i].enter.Y)
			// quadratic corner through original vertex
			out.QuadraticTo(pts[i].X, pts[i].Y, cuts[i].leave.X, cuts[i].leave.Y)
		} else {
			out.LineTo(pts[i].X, pts[i].Y)
		}
	}
	if closed {
		// close with first corner
		if cuts[0].has {
			out.LineTo(cuts[0].enter.X, cuts[0].enter.Y)
			out.QuadraticTo(pts[0].X, pts[0].Y, cuts[0].leave.X, cuts[0].leave.Y)
		}
		out.Close()
	}
	return out
}

// Discrete returns a path of short line segments sampled along p at approximately
// segLen spacing, with optional perpendicular deviation (E.02 DiscretePathEffect / 1D–2D).
// deviation == 0 yields a dashed polyline of segment length ~segLen/2 drawn then gap.
// When deviation != 0, sample points are offset perpendicularly by ±deviation (deterministic).
// Discrete returns a path of short line segments sampled along p at approximately
// segLen spacing, with optional perpendicular deviation (E.02 DiscretePathEffect / 1D–2D).
// deviation == 0 yields a dashed polyline of segment length ~segLen/2 drawn then gap.
// When deviation != 0, sample points are offset perpendicularly by ±deviation (deterministic).
func (p *Path) Discrete(segLen, deviation float64) *Path {
	out := NewPath()
	if p == nil || len(p.verbs) == 0 {
		return out
	}
	if segLen <= 1e-6 {
		segLen = 1
	}
	pts := p.Flatten(0.35)
	if len(pts) < 2 {
		return out
	}
	// Walk cumulative length and emit short segments
	var acc float64
	var emitOn bool
	var last Point
	started := false
	step := 0
	for i := 0; i < len(pts)-1; i++ {
		a, b := pts[i], pts[i+1]
		dx, dy := b.X-a.X, b.Y-a.Y
		seg := math.Hypot(dx, dy)
		if seg < 1e-12 {
			continue
		}
		ux, uy := dx/seg, dy/seg
		// perpendicular
		px, py := -uy, ux
		pos := 0.0
		for pos < seg {
			remain := segLen - acc
			if remain <= 1e-9 {
				acc = 0
				emitOn = !emitOn
				remain = segLen
			}
			take := remain
			if take > seg-pos {
				take = seg - pos
			}
			x0 := a.X + ux*pos
			y0 := a.Y + uy*pos
			x1 := a.X + ux*(pos+take)
			y1 := a.Y + uy*(pos+take)
			if deviation != 0 {
				// deterministic alternate lateral offset
				s := 1.0
				if step%2 == 1 {
					s = -1
				}
				x0 += px * deviation * s
				y0 += py * deviation * s
				x1 += px * deviation * s
				y1 += py * deviation * s
			}
			if emitOn {
				if !started {
					out.MoveTo(x0, y0)
					started = true
					last = Pt(x0, y0)
				} else if last.Distance(Pt(x0, y0)) > 1e-4 {
					out.MoveTo(x0, y0)
				}
				out.LineTo(x1, y1)
				last = Pt(x1, y1)
			} else {
				started = false
			}
			acc += take
			pos += take
			if acc >= segLen-1e-9 {
				acc = 0
				emitOn = !emitOn
				step++
			}
		}
	}
	return out
}
