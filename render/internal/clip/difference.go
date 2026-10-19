//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package clip

import (
	"math"

	"github.com/energye/gpui/render/internal/image"
)

// NewRectDifferenceMask builds a mask covering bounds where pixels inside
// hole are 0 (clipped out) and outside hole (but in bounds) are 255.
// Used for ClipOpDifference of a rectangle. Pixels straddling a hole edge
// receive analytic partial coverage, so the difference clip is anti-aliased
// at fractional hole coordinates (Skia clip AA semantics).
func NewRectDifferenceMask(bounds, hole Rect) (*MaskClipper, error) {
	if bounds.IsEmpty() {
		return nil, image.ErrInvalidDimensions
	}
	width := int(bounds.W + 0.5)
	height := int(bounds.H + 0.5)
	if width <= 0 || height <= 0 {
		return nil, image.ErrInvalidDimensions
	}
	mask, err := image.NewImageBuf(width, height, image.FormatGray8)
	if err != nil {
		return nil, err
	}

	// Hole edges in device coordinates.
	hx0, hy0 := hole.X, hole.Y
	hx1, hy1 := hole.X+hole.W, hole.Y+hole.H

	// No effective hole: full coverage everywhere.
	if hole.IsEmpty() || hx1 <= hx0 || hy1 <= hy0 {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				_ = mask.SetRGBA(x, y, 255, 255, 255, 255)
			}
		}
		return &MaskClipper{mask: mask, bounds: bounds}, nil
	}

	for y := 0; y < height; y++ {
		py0 := bounds.Y + float64(y)
		oy := overlap1D(py0, py0+1, hy0, hy1)
		if oy <= 0 {
			// Row fully outside the hole: full coverage.
			for x := 0; x < width; x++ {
				_ = mask.SetRGBA(x, y, 255, 255, 255, 255)
			}
			continue
		}
		for x := 0; x < width; x++ {
			px0 := bounds.X + float64(x)
			ox := overlap1D(px0, px0+1, hx0, hx1)
			if ox <= 0 {
				_ = mask.SetRGBA(x, y, 255, 255, 255, 255)
				continue
			}
			// Coverage = 1 - (fraction of the pixel covered by the hole).
			cov := (1 - ox*oy) * 255
			v := uint8(math.Round(cov)) //nolint:gosec // cov is in [0,255]
			_ = mask.SetRGBA(x, y, v, v, v, 255)
		}
	}
	return &MaskClipper{mask: mask, bounds: bounds}, nil
}

// overlap1D returns the length of the overlap of the 1D intervals [a0,a1]
// and [b0,b1], or zero when they do not overlap.
func overlap1D(a0, a1, b0, b1 float64) float64 {
	lo := math.Max(a0, b0)
	hi := math.Min(a1, b1)
	if hi <= lo {
		return 0
	}
	return hi - lo
}
