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
	"sort"
)

// ExtendMode defines how gradients extend beyond their defined bounds.
type ExtendMode int

const (
	// ExtendPad extends edge colors beyond bounds (default behavior).
	ExtendPad ExtendMode = iota
	// ExtendRepeat repeats the gradient pattern.
	ExtendRepeat
	// ExtendReflect mirrors the gradient pattern.
	ExtendReflect
)

// ColorStop represents a color at a specific position in a gradient.
type ColorStop struct {
	Offset float64 // Position in gradient, 0.0 to 1.0
	Color  RGBA    // Color at this position
}

// sortStops sorts color stops by offset and removes duplicates.
func sortStops(stops []ColorStop) []ColorStop {
	if len(stops) == 0 {
		return stops
	}

	// Create a copy to avoid modifying the original
	sorted := make([]ColorStop, len(stops))
	copy(sorted, stops)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Offset < sorted[j].Offset
	})

	return sorted
}

// ensureSortedStops returns a cached sorted copy of stops.
// Shared by Linear/Radial/Sweep gradient brushes to avoid three identical
// ensureSorted methods. *sorted / *sortedStops are the brush's cache fields.
func ensureSortedStops(sorted *bool, sortedStops *[]ColorStop, stops []ColorStop) []ColorStop {
	if *sorted {
		return *sortedStops
	}
	*sortedStops = sortStops(stops)
	*sorted = true
	return *sortedStops
}

// firstStopColor returns the first stop's color or Transparent if empty.
// The stops slice MUST be pre-sorted by offset.
func firstStopColor(stops []ColorStop) RGBA {
	if len(stops) == 0 {
		return Transparent
	}
	return stops[0].Color
}

// applyExtendMode applies the extend mode to normalize t to [0, 1].
func applyExtendMode(t float64, mode ExtendMode) float64 {
	switch mode {
	case ExtendRepeat:
		t -= math.Floor(t)
		if t < 0 {
			t++
		}
	case ExtendReflect:
		t = math.Abs(t)
		period := math.Floor(t)
		t -= period
		if int(period)%2 == 1 {
			t = 1 - t
		}
	default: // ExtendPad
		t = clamp01(t)
	}
	return t
}

// clamp01 clamps a value to [0, 1] range.
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// interpolateColorSRGB interpolates two stop colors in premultiplied
// sRGB — the SVG default interpolation space (color-interpolation=sRGB)
// and Skia/tiny-skia's stop lerp space (PremultipliedColorU8). The
// previous linear-space lerp shifted every multi-stop ramp measurably
// (grey t=0.2 read 0.48 sRGB instead of 0.20).
func interpolateColorSRGB(c1, c2 RGBA, t float64) RGBA {
	// Premultiply, lerp, back to straight.
	r1, g1, b1 := c1.R*c1.A, c1.G*c1.A, c1.B*c1.A
	r2, g2, b2 := c2.R*c2.A, c2.G*c2.A, c2.B*c2.A
	return RGBA{
		R: r1 + t*(r2-r1),
		G: g1 + t*(g2-g1),
		B: b1 + t*(b2-b1),
		A: c1.A + t*(c2.A-c1.A),
	}.Unpremultiply()
}

// colorAtOffset returns the interpolated color at a given offset.
// Handles edge cases: empty stops, single stop, out-of-bounds t.
// The stops slice MUST be pre-sorted by offset. Callers are responsible
// for sorting stops once (at AddColorStop time or lazily before first use).
func colorAtOffset(stops []ColorStop, t float64, mode ExtendMode) RGBA {
	// Edge case: no stops
	if len(stops) == 0 {
		return Transparent
	}

	// Edge case: single stop
	if len(stops) == 1 {
		return stops[0].Color
	}

	// Apply extend mode to normalize t
	t = applyExtendMode(t, mode)

	// Find the two stops to interpolate between
	// Binary search for efficiency (no allocation)
	idx := sort.Search(len(stops), func(i int) bool {
		return stops[i].Offset >= t
	})

	// Handle edge cases after extend mode
	if idx == 0 {
		return stops[0].Color
	}
	if idx >= len(stops) {
		return stops[len(stops)-1].Color
	}

	// Interpolate between stops[idx-1] and stops[idx]
	stop1 := stops[idx-1]
	stop2 := stops[idx]

	// Avoid division by zero for coincident stops
	if stop2.Offset == stop1.Offset {
		return stop1.Color
	}

	// Calculate interpolation factor
	localT := (t - stop1.Offset) / (stop2.Offset - stop1.Offset)

	return interpolateColorSRGB(stop1.Color, stop2.Color, localT)
}
