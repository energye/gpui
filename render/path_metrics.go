package render

import "math"

// PathMetric describes one contour of a path for arc-length queries
// (Flutter Path.computeMetrics / PathMetric subset).
//
// Contours are split on MoveTo. Close adds the closing edge into the metric.
// Curves are approximated by Flatten-style subdivision (tolerance).
type PathMetric struct {
	// pts is the polyline approximating this contour (at least 1 point).
	pts []Point
	// cum[i] = arc length from pts[0] to pts[i]; cum[0]=0; len(cum)==len(pts).
	cum []float64
}

// Length returns the total arc length of this contour.
func (m PathMetric) Length() float64 {
	if len(m.cum) == 0 {
		return 0
	}
	return m.cum[len(m.cum)-1]
}

// IsEmpty reports whether the contour has no measurable segments.
func (m PathMetric) IsEmpty() bool {
	return m.Length() <= 0 || len(m.pts) < 2
}

// PositionAt returns the point at arc-length distance along this contour.
// distance is clamped to [0, Length]. ok is false only when the contour is empty.
func (m PathMetric) PositionAt(distance float64) (Point, bool) {
	pos, _, ok := m.sampleAt(distance)
	return pos, ok
}

// TangentAt returns position and unit tangent at arc-length distance.
// Tangent is the direction of the segment containing the sample (zero if degenerate).
func (m PathMetric) TangentAt(distance float64) (pos, tangent Point, ok bool) {
	return m.sampleAt(distance)
}

func (m PathMetric) sampleAt(distance float64) (pos, tangent Point, ok bool) {
	if len(m.pts) == 0 {
		return Point{}, Point{}, false
	}
	if len(m.pts) == 1 || m.Length() <= 0 {
		return m.pts[0], Point{}, true
	}
	total := m.Length()
	if distance < 0 {
		distance = 0
	}
	if distance > total {
		distance = total
	}
	// Find segment i where cum[i] <= distance <= cum[i+1].
	i := 0
	for i+1 < len(m.cum) && m.cum[i+1] < distance {
		i++
	}
	if i+1 >= len(m.pts) {
		return m.pts[len(m.pts)-1], Point{}, true
	}
	segLen := m.cum[i+1] - m.cum[i]
	var t float64
	if segLen > 1e-12 {
		t = (distance - m.cum[i]) / segLen
	}
	a, b := m.pts[i], m.pts[i+1]
	pos = a.Lerp(b, t)
	d := b.Sub(a)
	if ls := d.LengthSquared(); ls > 1e-24 {
		inv := 1 / math.Sqrt(ls)
		tangent = Pt(d.X*inv, d.Y*inv)
	}
	return pos, tangent, true
}

// ComputeMetrics builds per-contour PathMetrics (Flutter Path.computeMetrics subset).
// tolerance controls curve flattening (≤0 → 0.25). Multi-contour paths yield one
// metric per MoveTo-started subpath. Empty paths return nil.
func (p *Path) ComputeMetrics(tolerance float64) []PathMetric {
	if p == nil || len(p.verbs) == 0 {
		return nil
	}
	if tolerance <= 0 {
		tolerance = 0.25
	}

	var metrics []PathMetric
	var pts []Point
	var current, start Point
	var started bool

	flush := func() {
		if !started || len(pts) == 0 {
			return
		}
		metrics = append(metrics, buildMetric(pts))
		pts = nil
		started = false
	}

	appendPt := func(pt Point) {
		if len(pts) > 0 {
			last := pts[len(pts)-1]
			if last.X == pt.X && last.Y == pt.Y {
				return
			}
		}
		pts = append(pts, pt)
	}

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			flush()
			pt := Pt(coords[0], coords[1])
			pts = []Point{pt}
			start = pt
			current = pt
			started = true
		case LineTo:
			if !started {
				return
			}
			pt := Pt(coords[0], coords[1])
			appendPt(pt)
			current = pt
		case QuadTo:
			if !started {
				return
			}
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			flattenQuad(current, ctrl, pt, tolerance, func(fp Point) {
				appendPt(fp)
			})
			current = pt
		case CubicTo:
			if !started {
				return
			}
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			flattenCubic(current, ctrl1, ctrl2, pt, tolerance, func(fp Point) {
				appendPt(fp)
			})
			current = pt
		case Close:
			if !started {
				return
			}
			// Closing edge from current back to subpath start (Flutter/Skia).
			if current.X != start.X || current.Y != start.Y {
				appendPt(start)
			}
			current = start
		}
	})
	flush()
	return metrics
}

func buildMetric(pts []Point) PathMetric {
	cum := make([]float64, len(pts))
	for i := 1; i < len(pts); i++ {
		cum[i] = cum[i-1] + pts[i-1].Distance(pts[i])
	}
	return PathMetric{pts: pts, cum: cum}
}

// TotalLength is the sum of all contour lengths from ComputeMetrics(tolerance).
// Prefer this (or ComputeMetrics) when closed paths matter: unlike Length(),
// closing edges from Close are included.
func (p *Path) TotalLength(tolerance float64) float64 {
	var sum float64
	for _, m := range p.ComputeMetrics(tolerance) {
		sum += m.Length()
	}
	return sum
}

// PositionAt samples the first non-empty contour at arc-length distance.
// For multi-contour paths, only contour 0 is used (Flutter often iterates metrics).
// ok is false when the path has no measurable contour.
func (p *Path) PositionAt(distance, tolerance float64) (Point, bool) {
	for _, m := range p.ComputeMetrics(tolerance) {
		if m.IsEmpty() {
			continue
		}
		return m.PositionAt(distance)
	}
	return Point{}, false
}

// TangentAt samples position and unit tangent on the first non-empty contour.
func (p *Path) TangentAt(distance, tolerance float64) (pos, tangent Point, ok bool) {
	for _, m := range p.ComputeMetrics(tolerance) {
		if m.IsEmpty() {
			continue
		}
		return m.TangentAt(distance)
	}
	return Point{}, Point{}, false
}
