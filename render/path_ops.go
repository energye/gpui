package render

import "math"

// Path operations for area calculation, winding number, containment testing,
// bounding box computation, flattening, and arc length measurement.

// Area returns the signed area enclosed by the path.
// Positive for clockwise paths, negative for counter-clockwise.
// Uses the shoelace formula extended for curves (Green's theorem).
// Only closed subpaths contribute to the area.
func (p *Path) Area() float64 {
	var area float64
	var current, start Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			start = Pt(coords[0], coords[1])
			current = start
		case LineTo:
			pt := Pt(coords[0], coords[1])
			area += lineArea(current, pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			area += quadArea(current, ctrl, pt)
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			area += cubicArea(current, ctrl1, ctrl2, pt)
			current = pt
		case Close:
			area += lineArea(current, start)
			current = start
		}
	})

	return area
}

// lineArea computes the contribution of a line segment to the signed area.
// Uses the shoelace formula: 0.5 * (x0*y1 - x1*y0)
func lineArea(p0, p1 Point) float64 {
	return 0.5 * (p0.X*p1.Y - p1.X*p0.Y)
}

// quadArea computes the contribution of a quadratic Bezier to the signed area.
// Integrates x*dy using the parametric form.
func quadArea(p0, p1, p2 Point) float64 {
	// For a quadratic Bezier B(t) = (1-t)^2*P0 + 2*(1-t)*t*P1 + t^2*P2
	// Area contribution = integral of x*dy from t=0 to t=1
	return (p0.X*(2*p1.Y+p2.Y) + p1.X*(-p0.Y+p2.Y) + p2.X*(-2*p1.Y-p0.Y)) / 6.0
}

// cubicArea computes the contribution of a cubic Bezier to the signed area.
// Integrates x*dy using the parametric form and Green's theorem.
func cubicArea(p0, p1, p2, p3 Point) float64 {
	// Using the formula from the kurbo library:
	return (p0.X*(6*p1.Y+3*p2.Y+p3.Y) +
		3*p1.X*(-2*p0.Y+p2.Y+p3.Y) +
		3*p2.X*(-p0.Y-p1.Y+2*p3.Y) +
		p3.X*(-p0.Y-3*p1.Y-6*p2.Y)) / 20.0
}

// Winding returns the winding number of a point relative to the path.
// 0 = outside, non-zero = inside (for non-zero fill rule).
// Uses ray casting with a horizontal ray to the right.
func (p *Path) Winding(pt Point) int {
	var winding int
	var current, start Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			start = Pt(coords[0], coords[1])
			current = start
		case LineTo:
			ep := Pt(coords[0], coords[1])
			winding += lineWinding(current, ep, pt)
			current = ep
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			ep := Pt(coords[2], coords[3])
			winding += quadWinding(current, ctrl, ep, pt)
			current = ep
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			ep := Pt(coords[4], coords[5])
			winding += cubicWinding(current, ctrl1, ctrl2, ep, pt)
			current = ep
		case Close:
			winding += lineWinding(current, start, pt)
			current = start
		}
	})

	return winding
}

// lineWinding computes the winding contribution of a line segment.
func lineWinding(p0, p1, pt Point) int {
	if p0.Y <= pt.Y && p1.Y > pt.Y {
		// Upward crossing
		if isLeft(p0, p1, pt) > 0 {
			return 1
		}
	} else if p0.Y > pt.Y && p1.Y <= pt.Y {
		// Downward crossing
		if isLeft(p0, p1, pt) < 0 {
			return -1
		}
	}
	return 0
}

// isLeft returns positive if pt is left of line p0-p1, negative if right, 0 if on.
func isLeft(p0, p1, pt Point) float64 {
	return (p1.X-p0.X)*(pt.Y-p0.Y) - (pt.X-p0.X)*(p1.Y-p0.Y)
}

// quadWinding computes the winding contribution of a quadratic Bezier.
func quadWinding(p0, p1, p2, pt Point) int {
	// Early exit if point is outside the vertical range
	minY := math.Min(math.Min(p0.Y, p1.Y), p2.Y)
	maxY := math.Max(math.Max(p0.Y, p1.Y), p2.Y)
	if pt.Y < minY || pt.Y > maxY {
		return 0
	}

	// Early exit if point is to the right of the curve
	maxX := math.Max(math.Max(p0.X, p1.X), p2.X)
	if pt.X > maxX {
		return 0
	}

	// Flatten the curve and sum line winding contributions
	return flattenQuadWinding(p0, p1, p2, pt)
}

// flattenQuadWinding computes winding by adaptively flattening the quadratic.
func flattenQuadWinding(p0, p1, p2, pt Point) int {
	q := NewQuadBez(p0, p1, p2)

	// Use adaptive subdivision based on flatness
	const tolerance = 0.1
	var winding int
	flattenQuadWindingRecursive(q, pt, tolerance, &winding, 0)
	return winding
}

// flattenQuadWindingRecursive recursively subdivides and accumulates winding.
func flattenQuadWindingRecursive(q QuadBez, pt Point, tolerance float64, winding *int, depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		*winding += lineWinding(q.P0, q.P2, pt)
		return
	}

	// Flatness test: distance from control point to chord
	mid := q.P0.Lerp(q.P2, 0.5)
	dist := q.P1.Sub(mid).Length()

	if dist <= tolerance {
		// Flat enough - use line approximation
		*winding += lineWinding(q.P0, q.P2, pt)
		return
	}

	// Subdivide and recurse
	q1, q2 := q.Subdivide()
	flattenQuadWindingRecursive(q1, pt, tolerance, winding, depth+1)
	flattenQuadWindingRecursive(q2, pt, tolerance, winding, depth+1)
}

// cubicWinding computes the winding contribution of a cubic Bezier.
func cubicWinding(p0, p1, p2, p3, pt Point) int {
	// Early exit if point is outside the vertical range
	minY := math.Min(math.Min(p0.Y, p1.Y), math.Min(p2.Y, p3.Y))
	maxY := math.Max(math.Max(p0.Y, p1.Y), math.Max(p2.Y, p3.Y))
	if pt.Y < minY || pt.Y > maxY {
		return 0
	}

	// Early exit if point is to the right of the curve
	maxX := math.Max(math.Max(p0.X, p1.X), math.Max(p2.X, p3.X))
	if pt.X > maxX {
		return 0
	}

	// Flatten the curve and sum line winding contributions
	return flattenCubicWinding(p0, p1, p2, p3, pt)
}

// flattenCubicWinding computes winding by adaptively flattening the cubic.
func flattenCubicWinding(p0, p1, p2, p3, pt Point) int {
	c := NewCubicBez(p0, p1, p2, p3)

	const tolerance = 0.1
	var winding int
	flattenCubicWindingRecursive(c, pt, tolerance, &winding, 0)
	return winding
}

// flattenCubicWindingRecursive recursively subdivides and accumulates winding.
func flattenCubicWindingRecursive(c CubicBez, pt Point, tolerance float64, winding *int, depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		*winding += lineWinding(c.P0, c.P3, pt)
		return
	}

	// Flatness test: max distance from control points to chord
	flatness := cubicFlatness(c)

	if flatness <= tolerance {
		// Flat enough - use line approximation
		*winding += lineWinding(c.P0, c.P3, pt)
		return
	}

	// Subdivide and recurse
	c1, c2 := c.Subdivide()
	flattenCubicWindingRecursive(c1, pt, tolerance, winding, depth+1)
	flattenCubicWindingRecursive(c2, pt, tolerance, winding, depth+1)
}

// cubicFlatness returns the maximum distance from control points to the chord.
func cubicFlatness(c CubicBez) float64 {
	// Distance from P1 and P2 to the line P0-P3
	ux := 3.0*c.P1.X - 2.0*c.P0.X - c.P3.X
	uy := 3.0*c.P1.Y - 2.0*c.P0.Y - c.P3.Y
	vx := 3.0*c.P2.X - c.P0.X - 2.0*c.P3.X
	vy := 3.0*c.P2.Y - c.P0.Y - 2.0*c.P3.Y

	return math.Max(ux*ux+uy*uy, vx*vx+vy*vy)
}

// Contains tests if a point is inside the path using the non-zero fill rule.
func (p *Path) Contains(pt Point) bool {
	return p.Winding(pt) != 0
}

// BoundingBox returns the tight axis-aligned bounding box of the path.
// Uses curve extrema for accuracy.
func (p *Path) BoundingBox() Rect {
	if len(p.verbs) == 0 {
		return Rect{}
	}

	// Initialize with extreme values
	bbox := Rect{
		Min: Point{X: math.MaxFloat64, Y: math.MaxFloat64},
		Max: Point{X: -math.MaxFloat64, Y: -math.MaxFloat64},
	}

	var current Point

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			pt := Pt(coords[0], coords[1])
			bbox = expandBBox(bbox, pt)
			current = pt
		case LineTo:
			pt := Pt(coords[0], coords[1])
			bbox = expandBBox(bbox, pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			bbox = bbox.Union(quadBBox(current, ctrl, pt))
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			bbox = bbox.Union(cubicBBox(current, ctrl1, ctrl2, pt))
			current = pt
		case Close:
			// Close doesn't add new points
		}
	})

	// Handle empty path case
	if bbox.Min.X == math.MaxFloat64 {
		return Rect{}
	}

	return bbox
}

// expandBBox expands the bounding box to include the point.
func expandBBox(bbox Rect, pt Point) Rect {
	return Rect{
		Min: Point{X: math.Min(bbox.Min.X, pt.X), Y: math.Min(bbox.Min.Y, pt.Y)},
		Max: Point{X: math.Max(bbox.Max.X, pt.X), Y: math.Max(bbox.Max.Y, pt.Y)},
	}
}

// quadBBox returns the tight bounding box of a quadratic Bezier.
func quadBBox(p0, p1, p2 Point) Rect {
	q := NewQuadBez(p0, p1, p2)
	return q.BoundingBox()
}

// cubicBBox returns the tight bounding box of a cubic Bezier.
func cubicBBox(p0, p1, p2, p3 Point) Rect {
	c := NewCubicBez(p0, p1, p2, p3)
	return c.BoundingBox()
}

// Flatten converts all curves to line segments with given tolerance.
// tolerance is the maximum distance from the curve.
func (p *Path) Flatten(tolerance float64) []Point {
	if len(p.verbs) == 0 {
		return nil
	}

	points := make([]Point, 0, len(p.verbs)*4)
	p.FlattenCallback(tolerance, func(pt Point) {
		points = append(points, pt)
	})
	return points
}

// FlattenCallback calls fn for each point in the flattened path.
// More efficient than Flatten() as it avoids allocation.
func (p *Path) FlattenCallback(tolerance float64, fn func(pt Point)) {
	if tolerance <= 0 {
		tolerance = 0.1 // Default tolerance
	}

	var current, start Point
	var started bool

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			if started {
				fn(current) // Emit last point of previous subpath
			}
			pt := Pt(coords[0], coords[1])
			fn(pt)
			start = pt
			current = pt
			started = true
		case LineTo:
			pt := Pt(coords[0], coords[1])
			fn(pt)
			current = pt
		case QuadTo:
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			flattenQuad(current, ctrl, pt, tolerance, fn)
			current = pt
		case CubicTo:
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			flattenCubic(current, ctrl1, ctrl2, pt, tolerance, fn)
			current = pt
		case Close:
			if current != start {
				fn(start)
			}
			current = start
		}
	})
}

// flattenQuad flattens a quadratic Bezier curve.
func flattenQuad(p0, p1, p2 Point, tolerance float64, fn func(pt Point)) {
	q := NewQuadBez(p0, p1, p2)
	flattenQuadRecursive(q, tolerance*tolerance, fn, 0)
}

// flattenQuadRecursive recursively subdivides the quadratic.
func flattenQuadRecursive(q QuadBez, toleranceSq float64, fn func(pt Point), depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		fn(q.P2)
		return
	}

	// Flatness test: distance from control point to chord midpoint
	mid := q.P0.Lerp(q.P2, 0.5)
	dist := q.P1.Sub(mid)
	if dist.LengthSquared() <= toleranceSq {
		fn(q.P2)
		return
	}

	// Subdivide
	q1, q2 := q.Subdivide()
	flattenQuadRecursive(q1, toleranceSq, fn, depth+1)
	flattenQuadRecursive(q2, toleranceSq, fn, depth+1)
}

// flattenCubic flattens a cubic Bezier curve.
func flattenCubic(p0, p1, p2, p3 Point, tolerance float64, fn func(pt Point)) {
	c := NewCubicBez(p0, p1, p2, p3)
	flattenCubicRecursive(c, tolerance*tolerance, fn, 0)
}

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
			// Close doesn't add length (already computed if there's a closing line)
		}
	})

	return length
}

// quadLength computes the arc length of a quadratic Bezier.
// Uses adaptive subdivision.
func quadLength(p0, p1, p2 Point, accuracy float64) float64 {
	q := NewQuadBez(p0, p1, p2)
	return quadLengthRecursive(q, accuracy*accuracy, 0)
}

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
func cubicLength(p0, p1, p2, p3 Point, accuracy float64) float64 {
	c := NewCubicBez(p0, p1, p2, p3)
	return cubicLengthRecursive(c, accuracy*accuracy, 0)
}

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
// normalized arc-length parameters t0 and t1 in [0,1] (E.03 / Skia trim path).
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
