//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import "math"

// EmboldenOutline expands a glyph outline by the standard synthetic-bold
// amount used by FreeType/Skia outline fake bold: size/24 pixels per side.
//
// Unlike bitmap mask dilation this works on the vector outline before
// rasterization, so the result follows the same analytic-AA path as a real
// font cut. Advances are intentionally unchanged: synthetic weight must not
// reflow text, matching Skia's fake-bold contract.
//
// The outline is expanded along each point's local normal, using the contour
// winding to decide outward vs inward (outer contours grow, holes shrink).
// This mirrors FT_Outline_Embolden's point-normal expansion without depending
// on cgo.
func EmboldenOutline(outline *GlyphOutline, sizePx float64) *GlyphOutline {
	if outline == nil || outline.IsEmpty() {
		return outline
	}
	strength := syntheticBoldStrength(sizePx)
	if strength <= 0 {
		return outline.Clone()
	}

	out := outline.Clone()
	var contours [][]outlinePointRef
	var contour []outlinePointRef
	flush := func() {
		if len(contour) >= 3 {
			contours = append(contours, contour)
			contour = nil
		} else {
			contour = contour[:0]
		}
	}
	for si := range out.Segments {
		seg := &out.Segments[si]
		switch seg.Op {
		case OutlineOpMoveTo:
			flush()
			contour = append(contour, outlinePointRef{seg: si, slot: 0})
		case OutlineOpLineTo:
			contour = append(contour, outlinePointRef{seg: si, slot: 0})
		case OutlineOpQuadTo:
			contour = append(contour, outlinePointRef{seg: si, slot: 0})
			contour = append(contour, outlinePointRef{seg: si, slot: 1})
		case OutlineOpCubicTo:
			contour = append(contour, outlinePointRef{seg: si, slot: 0})
			contour = append(contour, outlinePointRef{seg: si, slot: 1})
			contour = append(contour, outlinePointRef{seg: si, slot: 2})
		}
	}
	flush()

	// One orientation for the whole glyph: the largest contour is the outer
	// boundary. Its winding decides which perpendicular points out of the
	// filled region; holes wind the other way, so the same perpendicular
	// shrinks them automatically (standard nonzero fill).
	orientation := 0.0
	maxArea := 0.0
	for _, c := range contours {
		area := outlineContourArea(out, c)
		if math.Abs(area) > maxArea {
			maxArea = math.Abs(area)
			orientation = area
		}
	}
	if orientation == 0 {
		return out
	}
	for _, c := range contours {
		offsetOutlineContour(out, c, strength, orientation)
	}

	out.LSB = outline.LSB - float32(strength)
	out.Bounds = outlineSegmentBounds(out)
	return out
}

// syntheticBoldStrength is the per-side expansion in pixels. FreeType's
// FT_Outline_Embolden strength is FT_MulFix(units_per_EM, y_scale)/24, which
// reduces to ppem/24 for the rendered size (Skia's outline fake bold).
func syntheticBoldStrength(sizePx float64) float64 {
	if sizePx <= 0 || math.IsNaN(sizePx) || math.IsInf(sizePx, 0) {
		return 0
	}
	return sizePx / 24.0
}

type outlinePointRef struct {
	seg  int
	slot int
}

// outlineContourArea returns the shoelace area of one contour.
func outlineContourArea(outline *GlyphOutline, refs []outlinePointRef) float64 {
	var area float64
	for i, ref := range refs {
		p := outline.Segments[ref.seg].Points[ref.slot]
		nextRef := refs[(i+1)%len(refs)]
		q := outline.Segments[nextRef.seg].Points[nextRef.slot]
		area += float64(p.X)*float64(q.Y) - float64(q.X)*float64(p.Y)
	}
	return area
}

// offsetOutlineContour expands one closed contour in place. orientation is
// the winding of the outer contour; positive/negative selects the
// perpendicular that points out of the filled region in Y-down coordinates.
func offsetOutlineContour(outline *GlyphOutline, refs []outlinePointRef, strength, orientation float64) {
	n := len(refs)
	if n < 3 {
		return
	}
	pts := make([]OutlinePoint, n)
	for i, ref := range refs {
		pts[i] = outline.Segments[ref.seg].Points[ref.slot]
	}

	for i, ref := range refs {
		prev := pts[(i-1+n)%n]
		next := pts[(i+1)%n]
		dx := float64(next.X - prev.X)
		dy := float64(next.Y - prev.Y)
		length := math.Hypot(dx, dy)
		if length < 1e-9 {
			continue
		}
		var nx, ny float64
		if orientation > 0 {
			nx, ny = dy/length, -dx/length
		} else {
			nx, ny = -dy/length, dx/length
		}
		seg := &outline.Segments[ref.seg]
		seg.Points[ref.slot].X += float32(nx * strength)
		seg.Points[ref.slot].Y += float32(ny * strength)
	}
}

// outlineSegmentBounds recomputes tight bounds from all segment points.
func outlineSegmentBounds(outline *GlyphOutline) Rect {
	if outline == nil || len(outline.Segments) == 0 {
		return Rect{}
	}
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, seg := range outline.Segments {
		for i := 0; i < outlinePointCount(seg.Op); i++ {
			p := seg.Points[i]
			if p.X < minX {
				minX = p.X
			}
			if p.Y < minY {
				minY = p.Y
			}
			if p.X > maxX {
				maxX = p.X
			}
			if p.Y > maxY {
				maxY = p.Y
			}
		}
	}
	return Rect{MinX: float64(minX), MinY: float64(minY), MaxX: float64(maxX), MaxY: float64(maxY)}
}

func outlinePointCount(op OutlineOp) int {
	switch op {
	case OutlineOpMoveTo, OutlineOpLineTo:
		return 1
	case OutlineOpQuadTo:
		return 2
	case OutlineOpCubicTo:
		return 3
	default:
		return 0
	}
}
