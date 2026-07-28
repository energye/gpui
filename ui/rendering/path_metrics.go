package rendering

import "github.com/energye/gpui/render"

// PathMetric is the UI-facing alias of render.PathMetric (FPath-METRICS).
type PathMetric = render.PathMetric

// ComputePathMetrics builds per-contour path metrics for arc-length queries
// (Flutter Path.computeMetrics subset). tolerance ≤0 uses render default.
//
// Use with NewPath / FillPath / StrokePath on the UI CustomPaint path.
func ComputePathMetrics(p *render.Path, tolerance float64) []PathMetric {
	if p == nil {
		return nil
	}
	return p.ComputeMetrics(tolerance)
}

// PathTotalLength returns the sum of contour lengths including Close edges.
func PathTotalLength(p *render.Path, tolerance float64) float64 {
	if p == nil {
		return 0
	}
	return p.TotalLength(tolerance)
}

// PathPositionAt samples the first non-empty contour at arc-length distance.
func PathPositionAt(p *render.Path, distance, tolerance float64) (x, y float64, ok bool) {
	if p == nil {
		return 0, 0, false
	}
	pt, ok := p.PositionAt(distance, tolerance)
	return pt.X, pt.Y, ok
}

// PathTangentAt samples position and unit tangent on the first non-empty contour.
func PathTangentAt(p *render.Path, distance, tolerance float64) (x, y, tx, ty float64, ok bool) {
	if p == nil {
		return 0, 0, 0, 0, false
	}
	pos, tan, ok := p.TangentAt(distance, tolerance)
	return pos.X, pos.Y, tan.X, tan.Y, ok
}
