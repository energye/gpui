//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

// P3.4 batch 1 CPU filters: morphological erode/dilate (Skia
// SkImageFilters::Erode/Dilate), Offset, and AlphaThreshold. All run on
// premul RGBA8 storage (the established convective-filter domain).

package filter

import (
	"image"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// MorphologyFilter applies erode (min) or dilate (max) over a (2r+1)²
// neighborhood. O(w·h·r²) straightforward implementation; Skia's separable
// O(n) optimization is a registered follow-up.
type MorphologyFilter struct {
	erode  bool
	radius int
}

// NewErodeFilter keeps the darkest (minimum) color in each neighborhood.
func NewErodeFilter(radiusX, radiusY int) *MorphologyFilter {
	return &MorphologyFilter{erode: true, radius: radiusX}
}

// NewDilateFilter keeps the brightest (maximum) color in each neighborhood.
func NewDilateFilter(radiusX, radiusY int) *MorphologyFilter {
	return &MorphologyFilter{erode: false, radius: radiusX}
}

// ExpandBounds grows by the neighborhood radius.
func (m *MorphologyFilter) ExpandBounds(bounds image.Rectangle) image.Rectangle {
	return bounds.Inset(-m.radius)
}

// Apply runs the morphology op over the bounds (premul domain: alpha and RGB
// take the neighborhood min/max together, which keeps premul consistency
// because min/max preserve the premul invariant only if applied per-channel
// on the same neighborhood — Skia's raster morphology does exactly this).
func (m *MorphologyFilter) Apply(src, dst *render.Pixmap, bounds scene.Rect) {
	w, h := src.Width(), src.Height()
	r := m.radius
	if r <= 0 {
		copyPixmapRegion(src, dst, bounds)
		return
	}
	srcData, dstData := src.Data(), dst.Data()
	for y := int(bounds.MinY); y < int(bounds.MaxY); y++ {
		for x := int(bounds.MinX); x < int(bounds.MaxX); x++ {
			var best [4]uint8
			first := true
			for dy := -r; dy <= r; dy++ {
				sy := clampInt(y+dy, 0, h-1)
				for dx := -r; dx <= r; dx++ {
					sx := clampInt(x+dx, 0, w-1)
					i := (sy*w + sx) * 4
					px := [4]uint8{srcData[i], srcData[i+1], srcData[i+2], srcData[i+3]}
					if first {
						best = px
						first = false
						continue
					}
					if m.erode {
						for c := 0; c < 4; c++ {
							if px[c] < best[c] {
								best[c] = px[c]
							}
						}
					} else {
						for c := 0; c < 4; c++ {
							if px[c] > best[c] {
								best[c] = px[c]
							}
						}
					}
				}
			}
			oi := (y*w + x) * 4
			dstData[oi], dstData[oi+1], dstData[oi+2], dstData[oi+3] = best[0], best[1], best[2], best[3]
		}
	}
}

// OffsetFilter translates the source by (dx, dy) pixels; uncovered area
// stays transparent.
type OffsetFilter struct {
	dx, dy int
}

// NewOffsetFilter builds an offset filter.
func NewOffsetFilter(dx, dy int) *OffsetFilter { return &OffsetFilter{dx: dx, dy: dy} }

// ExpandBounds grows by the absolute offset.
func (f *OffsetFilter) ExpandBounds(bounds image.Rectangle) image.Rectangle {
	return bounds.Inset(-absI(f.dx)).Inset(-absI(f.dy))
}

// Apply copies src shifted by (dx, dy).
func (f *OffsetFilter) Apply(src, dst *render.Pixmap, bounds scene.Rect) {
	w, h := src.Width(), src.Height()
	srcData, dstData := src.Data(), dst.Data()
	for y := int(bounds.MinY); y < int(bounds.MaxY); y++ {
		for x := int(bounds.MinX); x < int(bounds.MaxX); x++ {
			sx, sy := x-f.dx, y-f.dy
			oi := (y*w + x) * 4
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				dstData[oi], dstData[oi+1], dstData[oi+2], dstData[oi+3] = 0, 0, 0, 0
				continue
			}
			si := (sy*w + sx) * 4
			dstData[oi], dstData[oi+1], dstData[oi+2], dstData[oi+3] =
				srcData[si], srcData[si+1], srcData[si+2], srcData[si+3]
		}
	}
}

// AlphaThresholdFilter zeroes pixels whose alpha is below the threshold
// (Skia's alpha-threshold mask-image semantics: hard binary cutoff).
type AlphaThresholdFilter struct {
	threshold uint8
}

// NewAlphaThresholdFilter builds the filter (0..255).
func NewAlphaThresholdFilter(threshold uint8) *AlphaThresholdFilter {
	return &AlphaThresholdFilter{threshold: threshold}
}

// ExpandBounds is identity.
func (f *AlphaThresholdFilter) ExpandBounds(bounds image.Rectangle) image.Rectangle {
	return bounds
}

// Apply zeroes premul pixels below the alpha threshold.
func (f *AlphaThresholdFilter) Apply(src, dst *render.Pixmap, bounds scene.Rect) {
	w := src.Width()
	srcData, dstData := src.Data(), dst.Data()
	for y := int(bounds.MinY); y < int(bounds.MaxY); y++ {
		for x := int(bounds.MinX); x < int(bounds.MaxX); x++ {
			i := (y*w + x) * 4
			if srcData[i+3] < f.threshold {
				dstData[i], dstData[i+1], dstData[i+2], dstData[i+3] = 0, 0, 0, 0
				continue
			}
			dstData[i], dstData[i+1], dstData[i+2], dstData[i+3] =
				srcData[i], srcData[i+1], srcData[i+2], srcData[i+3]
		}
	}
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
