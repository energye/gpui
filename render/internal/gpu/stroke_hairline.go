package gpu

import "math"

import "github.com/energye/gpui/render"

// snapHairlineStrokePath snaps axis-aligned stroke geometry to pixel centers so a
// 1 device-pixel stroke covers the intended row/column (Skia-style hairline align).
//
// Without this, a vertical line at x=N with width 1 covers [N-0.5, N+0.5] and
// center-sampled rasterization paints column N-1 only — caret/hairline tests miss.
func snapHairlineStrokePath(path *render.Path) *render.Path {
	if path == nil || path.NumVerbs() == 0 {
		return path
	}
	type pt struct{ x, y float64 }
	var pts []pt
	var verbs []render.PathVerb
	var segStarts []int // index into pts for each MoveTo

	path.Iterate(func(verb render.PathVerb, coords []float64) {
		verbs = append(verbs, verb)
		switch verb {
		case render.MoveTo:
			segStarts = append(segStarts, len(pts))
			pts = append(pts, pt{coords[0], coords[1]})
		case render.LineTo:
			pts = append(pts, pt{coords[0], coords[1]})
		case render.QuadTo:
			pts = append(pts, pt{coords[0], coords[1]}, pt{coords[2], coords[3]})
		case render.CubicTo:
			pts = append(pts, pt{coords[0], coords[1]}, pt{coords[2], coords[3]}, pt{coords[4], coords[5]})
		case render.Close:
			// no points
		}
	})
	if len(pts) == 0 {
		return path
	}

	// Walk consecutive LineTo pairs within each subpath and snap axis-aligned edges.
	// Point index mirror of Iterate order.
	pi := 0
	for _, verb := range verbs {
		switch verb {
		case render.MoveTo:
			pi++
		case render.LineTo:
			if pi > 0 {
				a := &pts[pi-1]
				b := &pts[pi]
				const eps = 1e-6
				if math.Abs(a.x-b.x) <= eps {
					// vertical
					x := math.Floor(a.x) + 0.5
					a.x, b.x = x, x
				} else if math.Abs(a.y-b.y) <= eps {
					// horizontal
					y := math.Floor(a.y) + 0.5
					a.y, b.y = y, y
				}
			}
			pi++
		case render.QuadTo:
			pi += 2
		case render.CubicTo:
			pi += 3
		case render.Close:
		}
	}

	out := render.NewPath()
	pi = 0
	for _, verb := range verbs {
		switch verb {
		case render.MoveTo:
			out.MoveTo(pts[pi].x, pts[pi].y)
			pi++
		case render.LineTo:
			out.LineTo(pts[pi].x, pts[pi].y)
			pi++
		case render.QuadTo:
			out.QuadraticTo(pts[pi].x, pts[pi].y, pts[pi+1].x, pts[pi+1].y)
			pi += 2
		case render.CubicTo:
			out.CubicTo(pts[pi].x, pts[pi].y, pts[pi+1].x, pts[pi+1].y, pts[pi+2].x, pts[pi+2].y)
			pi += 3
		case render.Close:
			out.Close()
		}
	}
	return out
}

// shouldSnapHairline reports whether effective device stroke width is a hairline
// (≤1 device px) that benefits from pixel-center alignment.
func shouldSnapHairline(paint *render.Paint) bool {
	if paint == nil {
		return false
	}
	w := effectiveStrokeWidth(paint)
	return w > 0 && w <= 1.0+1e-6
}
