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

	"github.com/energye/gpui/render/internal/stroke"
)

// Stroke implements Renderer.Stroke with anti-aliasing support.
// Strokes are expanded to fill paths and rendered with the Fill method,
// which provides analytic anti-aliased results.
func (r *SoftwareRenderer) Stroke(pixmap *Pixmap, p *Path, paint *Paint) error {
	// Get effective line width
	width := paint.EffectiveLineWidth()

	// Get transform scale for dash pattern scaling
	transformScale := paint.TransformScale
	if transformScale <= 0 {
		transformScale = 1.0
	}

	// Apply dash pattern if set
	// Scale dash pattern by transform scale
	pathToDraw := p
	if paint.IsDashed() {
		dash := paint.EffectiveDash()
		if transformScale > 1.0 {
			dash = dash.Scale(transformScale)
		}
		pathToDraw = dashPath(p, dash)
	}

	// Convert render.PathVerb to stroke.PathVerb (same layout, just cast)
	strokeVerbs := convertVerbsToStroke(pathToDraw.Verbs())

	// Create stroke style from paint
	// Scale line width by transform scale (path coordinates are already transformed)
	effectiveWidth := width * transformScale
	if effectiveWidth < 1.0 {
		effectiveWidth = 1.0 // Minimum 1px stroke for visibility
	}
	strokeStyle := stroke.Stroke{
		Width:      effectiveWidth,
		Cap:        convertLineCap(paint.EffectiveLineCap()),
		Join:       convertLineJoin(paint.EffectiveLineJoin()),
		MiterLimit: paint.EffectiveMiterLimit(),
	}
	if strokeStyle.MiterLimit <= 0 {
		strokeStyle.MiterLimit = 4.0 // Default
	}

	// Create stroke expander with tight tolerance for smooth curves.
	// 0.1 px base tolerance; on HiDPI, divide by deviceScale for finer curves.
	expander := stroke.NewStrokeExpander(strokeStyle)
	strokeTol := float64(0.1)
	if ds := math.Float32frombits(r.deviceScale.Load()); ds > 1.0 {
		strokeTol = 0.1 / float64(ds)
	}
	expander.SetTolerance(strokeTol)

	// Expand stroke to fill path (SOA: verb+coords in, verb+coords out)
	outVerbs, outCoords := expander.Expand(strokeVerbs, pathToDraw.Coords())

	// Convert back to render.Path (reuse scratch to avoid per-stroke allocation).
	if r.scratchStrokePath == nil {
		r.scratchStrokePath = NewPath()
	}
	strokeResultToPath(r.scratchStrokePath, outVerbs, outCoords)

	// Route stroke fills through AnalyticFiller.
	// Stroke-expanded multi-contour outlines (e.g., closed path → 4 contours)
	// require per-scanline winding tracking that the tile-based SparseStripsFiller
	// does not support (Vello's strip pipeline uses per-strip fill_gap flags).
	// not tile rasterizers. Single-contour strokes work with either filler after
	// the expander.go fix (#347), but multi-contour needs scanline.
	prevMode := r.rasterizerMode
	r.rasterizerMode = RasterizerAnalytic
	err := r.Fill(pixmap, r.scratchStrokePath, paint)
	r.rasterizerMode = prevMode
	return err
}

// convertVerbsToStroke converts render.PathVerb slice to stroke.PathVerb slice.
// Both types have identical byte values, so this is a simple cast.
// convertVerbsToStroke converts render.PathVerb slice to stroke.PathVerb slice.
// Both types have identical byte values, so this is a simple cast.
func convertVerbsToStroke(verbs []PathVerb) []stroke.PathVerb {
	result := make([]stroke.PathVerb, len(verbs))
	for i, v := range verbs {
		result[i] = stroke.PathVerb(v)
	}
	return result
}

// strokeResultToPath converts stroke output (verbs+coords) into dst Path.
// Reuses dst to avoid per-stroke allocation (Skia fOuter.reset() pattern).
// strokeResultToPath converts stroke output (verbs+coords) into dst Path.
// Reuses dst to avoid per-stroke allocation (Skia fOuter.reset() pattern).
func strokeResultToPath(dst *Path, verbs []stroke.PathVerb, coords []float64) {
	dst.Reset()
	ci := 0
	for _, v := range verbs {
		switch v {
		case stroke.VerbMoveTo:
			dst.MoveTo(coords[ci], coords[ci+1])
			ci += 2
		case stroke.VerbLineTo:
			dst.LineTo(coords[ci], coords[ci+1])
			ci += 2
		case stroke.VerbQuadTo:
			dst.QuadraticTo(coords[ci], coords[ci+1], coords[ci+2], coords[ci+3])
			ci += 4
		case stroke.VerbCubicTo:
			dst.CubicTo(coords[ci], coords[ci+1], coords[ci+2], coords[ci+3], coords[ci+4], coords[ci+5])
			ci += 6
		case stroke.VerbClose:
			dst.Close()
		}
	}
}

// convertLineCap converts render.LineCap to stroke.LineCap.
// convertLineCap converts render.LineCap to stroke.LineCap.
func convertLineCap(c LineCap) stroke.LineCap {
	switch c {
	case LineCapButt:
		return stroke.LineCapButt
	case LineCapRound:
		return stroke.LineCapRound
	case LineCapSquare:
		return stroke.LineCapSquare
	default:
		return stroke.LineCapButt
	}
}

// convertLineJoin converts render.LineJoin to stroke.LineJoin.
// convertLineJoin converts render.LineJoin to stroke.LineJoin.
func convertLineJoin(join LineJoin) stroke.LineJoin {
	switch join {
	case LineJoinMiter:
		return stroke.LineJoinMiter
	case LineJoinRound:
		return stroke.LineJoinRound
	case LineJoinBevel:
		return stroke.LineJoinBevel
	default:
		return stroke.LineJoinMiter
	}
}

// dashPath converts a path to a dashed path using the given dash pattern.
// This walks along the path and outputs only the "dash" portions, skipping gaps.
// dashPath converts a path to a dashed path using the given dash pattern.
// This walks along the path and outputs only the "dash" portions, skipping gaps.
func dashPath(p *Path, dash *Dash) *Path {
	if dash == nil || !dash.IsDashed() {
		return p
	}

	pattern := dash.effectiveArray()
	if len(pattern) == 0 {
		return p
	}

	result := NewPath()

	// State for walking along the path
	var (
		currentX, currentY float64 // current position
		startX, startY     float64 // subpath start
		patternIdx         int     // current index in pattern
		patternPos         float64 // position within current pattern element
		inDash             bool    // true if currently drawing (vs gap)
	)

	// Initialize with offset
	offset := dash.NormalizedOffset()
	patternIdx, patternPos, inDash = dashStateAtOffset(pattern, offset)

	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			currentX, currentY = coords[0], coords[1]
			startX, startY = currentX, currentY
			// Reset pattern state for new subpath
			patternIdx, patternPos, inDash = dashStateAtOffset(pattern, offset)
			if inDash {
				result.MoveTo(currentX, currentY)
			}

		case LineTo:
			dashLine(result, &currentX, &currentY, coords[0], coords[1],
				pattern, &patternIdx, &patternPos, &inDash)

		case QuadTo:
			// Flatten quadratic to lines for dashing
			ctrl := Pt(coords[0], coords[1])
			pt := Pt(coords[2], coords[3])
			dashQuad(result, &currentX, &currentY, ctrl, pt,
				pattern, &patternIdx, &patternPos, &inDash)

		case CubicTo:
			// Flatten cubic to lines for dashing
			ctrl1 := Pt(coords[0], coords[1])
			ctrl2 := Pt(coords[2], coords[3])
			pt := Pt(coords[4], coords[5])
			dashCubic(result, &currentX, &currentY, ctrl1, ctrl2, pt,
				pattern, &patternIdx, &patternPos, &inDash)

		case Close:
			// Close by dashing line back to start
			if currentX != startX || currentY != startY {
				dashLine(result, &currentX, &currentY, startX, startY,
					pattern, &patternIdx, &patternPos, &inDash)
			}
		}
	})

	return result
}

// dashStateAtOffset calculates the pattern state at a given offset.
// dashStateAtOffset calculates the pattern state at a given offset.
func dashStateAtOffset(pattern []float64, offset float64) (idx int, pos float64, inDash bool) {
	patternLen := 0.0
	for _, l := range pattern {
		patternLen += l
	}
	if patternLen <= 0 {
		return 0, 0, true
	}

	// Normalize offset
	offset = math.Mod(offset, patternLen)
	if offset < 0 {
		offset += patternLen
	}

	// Walk through pattern to find position
	accumulated := 0.0
	for i, l := range pattern {
		if offset < accumulated+l {
			return i, offset - accumulated, i%2 == 0
		}
		accumulated += l
	}

	return 0, 0, true
}

// dashLine dashes a line segment from (currentX, currentY) to (x, y).
// dashLine dashes a line segment from (currentX, currentY) to (x, y).
func dashLine(result *Path, currentX, currentY *float64, x, y float64,
	pattern []float64, patternIdx *int, patternPos *float64, inDash *bool) {
	dx := x - *currentX
	dy := y - *currentY
	segmentLen := math.Sqrt(dx*dx + dy*dy)

	if segmentLen < 1e-10 {
		return
	}

	// Unit direction
	ux, uy := dx/segmentLen, dy/segmentLen

	remaining := segmentLen
	startX, startY := *currentX, *currentY

	for remaining > 1e-10 {
		patternVal := pattern[*patternIdx]
		available := patternVal - *patternPos

		if available <= 0 {
			// Move to next pattern element
			*patternIdx = (*patternIdx + 1) % len(pattern)
			*patternPos = 0
			*inDash = (*patternIdx % 2) == 0
			continue
		}

		consume := math.Min(available, remaining)
		endX := startX + ux*consume
		endY := startY + uy*consume

		if *inDash {
			// We're in a dash - draw the line
			if result.isEmpty() || !pathEndAt(result, startX, startY) {
				result.MoveTo(startX, startY)
			}
			result.LineTo(endX, endY)
		}
		// If in gap, we just skip

		startX, startY = endX, endY
		remaining -= consume
		*patternPos += consume

		// Check if we've finished current pattern element
		if *patternPos >= patternVal-1e-10 {
			*patternIdx = (*patternIdx + 1) % len(pattern)
			*patternPos = 0
			*inDash = (*patternIdx % 2) == 0
		}
	}

	*currentX, *currentY = x, y
}

// dashQuad dashes a quadratic bezier curve by flattening it.
// dashQuad dashes a quadratic bezier curve by flattening it.
func dashQuad(result *Path, currentX, currentY *float64, control, end Point,
	pattern []float64, patternIdx *int, patternPos *float64, inDash *bool) {
	// Flatten quadratic to line segments (shared path_ops flattener).
	tolerance := 0.5 // reasonable tolerance for dashing
	start := Pt(*currentX, *currentY)
	var points []float64
	points = append(points, start.X, start.Y)
	flattenQuad(start, control, end, tolerance, func(pt Point) {
		points = append(points, pt.X, pt.Y)
	})

	for i := 2; i < len(points); i += 2 {
		dashLine(result, currentX, currentY, points[i], points[i+1],
			pattern, patternIdx, patternPos, inDash)
	}
}

// dashCubic dashes a cubic bezier curve by flattening it.
// dashCubic dashes a cubic bezier curve by flattening it.
func dashCubic(result *Path, currentX, currentY *float64, c1, c2, end Point,
	pattern []float64, patternIdx *int, patternPos *float64, inDash *bool) {
	// Keep dash-specific cubic flatten: path_ops cubicFlatness metric differs
	// slightly from control-point distance used historically for dashing.
	tolerance := 0.5 // reasonable tolerance for dashing
	points := flattenCubicForDash(*currentX, *currentY,
		c1.X, c1.Y, c2.X, c2.Y, end.X, end.Y, tolerance)

	for i := 2; i < len(points); i += 2 {
		dashLine(result, currentX, currentY, points[i], points[i+1],
			pattern, patternIdx, patternPos, inDash)
	}
}

// pathEndAt checks if the path ends at the given point.
// pathEndAt checks if the path ends at the given point.
func pathEndAt(p *Path, x, y float64) bool {
	if p.isEmpty() {
		return false
	}
	cp := p.CurrentPoint()
	return math.Abs(cp.X-x) < 1e-10 && math.Abs(cp.Y-y) < 1e-10
}

// flattenCubicForDash flattens a cubic bezier to line points.
// flattenCubicForDash flattens a cubic bezier to line points.
func flattenCubicForDash(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance float64) []float64 {
	points := []float64{x0, y0}
	flattenCubicRecForDash(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance, &points, 0)
	return points
}

func flattenCubicRecForDash(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tolerance float64, points *[]float64, depth int) {
	// Max recursion depth to prevent stack overflow (e.g. NaN coordinates)
	if depth > 10 {
		*points = append(*points, x1, y1)
		return
	}

	// Check if curve is flat enough
	// Use distance of control points from the line
	d1 := pointLineDistance(c1x, c1y, x0, y0, x1, y1)
	d2 := pointLineDistance(c2x, c2y, x0, y0, x1, y1)
	dist := math.Max(d1, d2)

	if dist < tolerance {
		*points = append(*points, x1, y1)
		return
	}

	// Subdivide using de Casteljau
	x01 := (x0 + c1x) / 2
	y01 := (y0 + c1y) / 2
	x12 := (c1x + c2x) / 2
	y12 := (c1y + c2y) / 2
	x23 := (c2x + x1) / 2
	y23 := (c2y + y1) / 2
	x012 := (x01 + x12) / 2
	y012 := (y01 + y12) / 2
	x123 := (x12 + x23) / 2
	y123 := (y12 + y23) / 2
	x0123 := (x012 + x123) / 2
	y0123 := (y012 + y123) / 2

	flattenCubicRecForDash(x0, y0, x01, y01, x012, y012, x0123, y0123, tolerance, points, depth+1)
	flattenCubicRecForDash(x0123, y0123, x123, y123, x23, y23, x1, y1, tolerance, points, depth+1)
}

// pointLineDistance calculates perpendicular distance from point to line.
// pointLineDistance calculates perpendicular distance from point to line.
func pointLineDistance(px, py, x0, y0, x1, y1 float64) float64 {
	dx := x1 - x0
	dy := y1 - y0
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 1e-10 {
		// Line is a point, return distance to that point
		return math.Sqrt((px-x0)*(px-x0) + (py-y0)*(py-y0))
	}
	// Cross product gives area of parallelogram, divide by base for height
	return math.Abs((py-y0)*dx-(px-x0)*dy) / length
}
