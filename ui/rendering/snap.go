package rendering

import "math"

// SnapCoord rounds a logical coordinate onto the device pixel grid (R19 1px
// alignment). Skia/Flutter-style: logical v maps to physical v*scale; rounding
// to the nearest physical pixel and mapping back gives a coordinate that lands
// exactly on a device pixel boundary, so hairlines stay crisp on HiDPI.
//
//	scale <= 0 falls back to 1 (identity DPR).
func SnapCoord(v, scale float64) float64 {
	if scale <= 0 {
		scale = 1
	}
	return math.Round(v*scale) / scale
}

// SnapLine returns the device-aligned origin (x for vertical lines, y for
// horizontal lines) of a 1px stroke so the stroke covers one physical pixel
// column/row centered on the grid (crisp, not blurred between two pixels).
// Flutter/Skia hairline practice: a 1px (logical) stroke of width 1/scale must
// start at a half-pixel offset from a snapped coordinate.
func SnapLine(v, scale float64) float64 {
	if scale <= 0 {
		scale = 1
	}
	half := 1 / scale / 2
	return (math.Floor(v*scale)+0.5)/scale - half
}

// SnapRect snaps a rectangle's origin and size onto the device grid, keeping
// the right/bottom edges on grid too (used for 1px borders that must not
// flicker between widths at fractional DPR).
func SnapRect(x, y, w, h, scale float64) (sx, sy, sw, sh float64) {
	if scale <= 0 {
		scale = 1
	}
	x0 := math.Round(x * scale)
	y0 := math.Round(y * scale)
	x1 := math.Round((x + w) * scale)
	y1 := math.Round((y + h) * scale)
	return x0 / scale, y0 / scale, (x1 - x0) / scale, (y1 - y0) / scale
}

// SnapX / SnapY are PaintContext-aware convenience wrappers over SnapCoord.
func (pc *PaintContext) SnapX(v float64) float64 {
	s := float64(1)
	if pc != nil && pc.Scale > 0 {
		s = pc.Scale
	}
	return SnapCoord(v, s)
}

// SnapY mirrors SnapX for the Y axis.
func (pc *PaintContext) SnapY(v float64) float64 {
	return pc.SnapX(v)
}

// SnapLineX aligns a vertical 1px line's x to the device grid (crisp hairline).
func (pc *PaintContext) SnapLineX(v float64) float64 {
	s := float64(1)
	if pc != nil && pc.Scale > 0 {
		s = pc.Scale
	}
	return SnapLine(v, s)
}

// SnapLineY aligns a horizontal 1px line's y to the device grid.
func (pc *PaintContext) SnapLineY(v float64) float64 {
	return pc.SnapLineX(v)
}

// SnapRect aligns a rect to the device grid (1px border crispness).
func (pc *PaintContext) SnapRect(x, y, w, h float64) (float64, float64, float64, float64) {
	s := float64(1)
	if pc != nil && pc.Scale > 0 {
		s = pc.Scale
	}
	return SnapRect(x, y, w, h, s)
}
