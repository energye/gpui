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

import "math"

// RadialGradientBrush represents a radial color transition.
// Colors radiate from a focal point within a circle defined by center and end radius.
// It implements the Brush interface and supports multiple color stops,
// proper sRGB color interpolation, and configurable extend modes.
//
// RadialGradientBrush follows the vello/peniko gradient model, supporting
// both simple radial gradients (focus at center) and focal gradients
// (focus offset from center for spotlight effects).
//
// Example:
//
//	// Simple radial gradient
//	gradient := render.NewRadialGradientBrush(50, 50, 0, 50).
//	    AddColorStop(0, render.White).
//	    AddColorStop(1, render.Black)
//
//	// Focal gradient (spotlight effect)
//	spotlight := render.NewRadialGradientBrush(50, 50, 0, 50).
//	    SetFocus(30, 30).
//	    AddColorStop(0, render.White).
//	    AddColorStop(1, render.Black)
type RadialGradientBrush struct {
	Center      Point       // Center of the gradient circle
	Focus       Point       // Focal point (can differ from center)
	StartRadius float64     // Inner radius where gradient begins (t=0)
	EndRadius   float64     // Outer radius where gradient ends (t=1)
	Stops       []ColorStop // Color stops defining the gradient
	Extend      ExtendMode  // How gradient extends beyond bounds

	// sortedStops caches stops sorted by offset. Populated lazily on first
	// ColorAt call. Invalidated by AddColorStop (sorted set to false).
	sortedStops []ColorStop
	sorted      bool
}

// NewRadialGradientBrush creates a new radial gradient.
// The gradient transitions from startRadius to endRadius around (cx, cy).
// Focus defaults to center.
func NewRadialGradientBrush(cx, cy, startRadius, endRadius float64) *RadialGradientBrush {
	center := Point{X: cx, Y: cy}
	return &RadialGradientBrush{
		Center:      center,
		Focus:       center, // Default focus at center
		StartRadius: startRadius,
		EndRadius:   endRadius,
		Stops:       nil,
		Extend:      ExtendPad,
	}
}

// SetFocus sets the focal point of the gradient.
// A focal point different from center creates an asymmetric gradient.
// Returns the gradient for method chaining.
func (g *RadialGradientBrush) SetFocus(fx, fy float64) *RadialGradientBrush {
	g.Focus = Point{X: fx, Y: fy}
	return g
}

// AddColorStop adds a color stop at the specified offset.
// Offset should be in the range [0, 1].
// Returns the gradient for method chaining.
func (g *RadialGradientBrush) AddColorStop(offset float64, c RGBA) *RadialGradientBrush {
	g.Stops = append(g.Stops, ColorStop{Offset: offset, Color: c})
	g.sorted = false // invalidate cached sort
	return g
}

// SetExtend sets the extend mode for the gradient.
// Returns the gradient for method chaining.
func (g *RadialGradientBrush) SetExtend(mode ExtendMode) *RadialGradientBrush {
	g.Extend = mode
	return g
}

// brushMarker implements the Brush interface marker.
func (RadialGradientBrush) brushMarker() {}

// ColorAt returns the color at the given point.
// Implements the Pattern and Brush interfaces.
//
// Focal gradients use the two-point conical formulation (Skia
// SkTwoPointRadialGradient / tiny-skia 0.11-0.12 Focal stages): t solves
// |P − (F + t·(C−F))| = t·r for a zero start radius at the focal point.
// Points with no real solution (negative discriminant — the tangent-cone
// complement when the focal lies outside the circle) and degenerate
// t ≤ 0 render transparent, exactly like tiny-skia's
// mask_2pt_conical_degenerates.
func (g *RadialGradientBrush) ColorAt(x, y float64) RGBA {
	// Handle degenerate gradient (zero radius difference)
	radiusDiff := g.EndRadius - g.StartRadius
	if radiusDiff == 0 {
		return firstStopColor(g.ensureSorted())
	}

	if g.Focus.X != g.Center.X || g.Focus.Y != g.Center.Y {
		t, ok := g.computeTFocal(x, y)
		if !ok {
			return Transparent // degenerate: no real t (tiny-skia vector mask)
		}
		return colorAtOffset(g.ensureSorted(), t, g.Extend)
	}

	t := g.computeTSimple(x, y)
	return colorAtOffset(g.ensureSorted(), t, g.Extend)
}

// ensureSorted returns stops sorted by offset, using a cached copy.
// The cache is invalidated by AddColorStop.
func (g *RadialGradientBrush) ensureSorted() []ColorStop {
	return ensureSortedStops(&g.sorted, &g.sortedStops, g.Stops)
}

// computeTSimple calculates t for the simple case where focus equals center.
func (g *RadialGradientBrush) computeTSimple(x, y float64) float64 {
	dx := x - g.Center.X
	dy := y - g.Center.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	radiusDiff := g.EndRadius - g.StartRadius
	if radiusDiff == 0 {
		return 0
	}

	return (distance - g.StartRadius) / radiusDiff
}

// computeTFocal solves the two-point conical equation for the gradient
// parameter t: the t-circle is centered at F + t·(C−F) with radius
// StartRadius + t·(EndRadius−StartRadius); P lies on it. Returns
// ok=false for degenerate points (no real solution or t ≤ 0), which
// render transparent per tiny-skia's degenerate mask.
func (g *RadialGradientBrush) computeTFocal(x, y float64) (float64, bool) {
	wx, wy := x-g.Focus.X, y-g.Focus.Y
	dx, dy := g.Center.X-g.Focus.X, g.Center.Y-g.Focus.Y
	dd := dx*dx + dy*dy // |C−F|²
	wd := wx*dx + wy*dy // w·(C−F)
	ww := wx*wx + wy*wy // |P−F|²
	dr := g.EndRadius - g.StartRadius
	a := dd - dr*dr            // quadratic coefficient (r0=0 model)
	b := wd + g.StartRadius*dr // linear coefficient / −2
	c := ww - g.StartRadius*g.StartRadius

	if math.Abs(a) < 1e-12 {
		// Focal on the circle (|C−F| == r): the quadratic degenerates to
		// linear, t = |w|² / (2·(w·d)) (limit of the general root).
		if wd <= 0 {
			return 0, false
		}
		t := c / (2 * wd)
		if t <= 0 || math.IsNaN(t) {
			return 0, false
		}
		return t, true
	}

	disc := b*b - a*c
	if disc < 0 {
		return 0, false // no real solution: outside the tangent cone
	}
	sqrtD := math.Sqrt(disc)
	// Root selection per tiny-skia's stages: focal inside the circle
	// (a<0) keeps the only non-negative root; focal outside (a>0, the
	// "greater" stage) takes the greater root — both verified against the
	// resvg corpus (focal-point-correction).
	var t float64
	if a < 0 {
		t = (sqrtD - b) / (-a)
	} else {
		t = (-sqrtD - b) / (-a)
	}
	if t <= 0 || math.IsNaN(t) {
		return 0, false
	}
	return t, true
}
