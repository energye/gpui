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

// Brush coordinates are user-space, but paths are baked into device space
// before rasterization (Context.deviceSpacePath). Skia keeps a gradient's
// local matrix and composes the canvas CTM at draw time; with baked geometry
// the equivalent is re-expressing the brush in that same device space.
//
// Similarity transforms (translate / rotate / uniform scale / mirror) can be
// baked into gradient geometry exactly. Anything else (non-uniform scale,
// skew) distorts circles and angles, which the gradient types cannot express,
// so those sample through the inverse matrix instead — exact, but on the
// generic ColorAt field path.

// deviceSpaceBrush samples a user-space brush at device-space coordinates by
// mapping each sample back through inv. Fallback for transforms whose effect
// on the gradient cannot be baked into its parameters.
type deviceSpaceBrush struct {
	base Brush
	inv  Matrix
	seed uint64
}

func (deviceSpaceBrush) brushMarker() {}

func (b deviceSpaceBrush) ColorAt(x, y float64) RGBA {
	if b.base == nil {
		return RGBA{}
	}
	p := b.inv.TransformPoint(Pt(x, y))
	return b.base.ColorAt(p.X, p.Y)
}

// BrushFieldSeed returns the cache-key contribution of wrapper brushes whose
// sampled field depends on state the concrete gradient types do not expose:
// group-opacity modulation and device-space inverse sampling. GPU field caches
// key on this value, so a constant seed would let two different wrapped brushes
// at the same geometry share one field. Returns 0 when b needs no extra state.
func BrushFieldSeed(b Brush) uint64 {
	switch v := b.(type) {
	case opacityMulBrush:
		return mixBrushSeed(brushContentSeed(v.base), math.Float64bits(v.mul))
	case *opacityMulBrush:
		if v == nil {
			return 0
		}
		return mixBrushSeed(brushContentSeed(v.base), math.Float64bits(v.mul))
	case deviceSpaceBrush:
		return v.seed
	case *deviceSpaceBrush:
		if v == nil {
			return 0
		}
		return v.seed
	}
	return 0
}

const brushSeedPrime = 1099511628211

func mixBrushSeed(h, v uint64) uint64 { return (h ^ v) * brushSeedPrime }

func mixBrushSeedF(h uint64, f float64) uint64 { return mixBrushSeed(h, math.Float64bits(f)) }

func mixBrushSeedMatrix(h uint64, m Matrix) uint64 {
	h = mixBrushSeedF(h, m.A)
	h = mixBrushSeedF(h, m.B)
	h = mixBrushSeedF(h, m.C)
	h = mixBrushSeedF(h, m.D)
	h = mixBrushSeedF(h, m.E)
	h = mixBrushSeedF(h, m.F)
	return h
}

func mixBrushSeedStops(h uint64, stops []ColorStop, extend ExtendMode) uint64 {
	h = mixBrushSeed(h, uint64(len(stops)))
	h = mixBrushSeed(h, uint64(extend))
	for _, s := range stops {
		h = mixBrushSeedF(h, s.Offset)
		h = mixBrushSeedF(h, s.Color.R)
		h = mixBrushSeedF(h, s.Color.G)
		h = mixBrushSeedF(h, s.Color.B)
		h = mixBrushSeedF(h, s.Color.A)
	}
	return h
}

// brushContentSeed hashes the brush description so wrapped brushes get a field
// cache key that tracks their content, not just their wrapper kind.
func brushContentSeed(b Brush) uint64 {
	h := uint64(0x6120b7a5e0000001)
	switch g := b.(type) {
	case SolidBrush:
		h = mixBrushSeed(h, 1)
		h = mixBrushSeedF(h, g.Color.R)
		h = mixBrushSeedF(h, g.Color.G)
		h = mixBrushSeedF(h, g.Color.B)
		return mixBrushSeedF(h, g.Color.A)
	case *LinearGradientBrush:
		if g == nil {
			return h
		}
		h = mixBrushSeed(h, 2)
		h = mixBrushSeedF(h, g.Start.X)
		h = mixBrushSeedF(h, g.Start.Y)
		h = mixBrushSeedF(h, g.End.X)
		h = mixBrushSeedF(h, g.End.Y)
		return mixBrushSeedStops(h, g.Stops, g.Extend)
	case *RadialGradientBrush:
		if g == nil {
			return h
		}
		h = mixBrushSeed(h, 3)
		h = mixBrushSeedF(h, g.Center.X)
		h = mixBrushSeedF(h, g.Center.Y)
		h = mixBrushSeedF(h, g.Focus.X)
		h = mixBrushSeedF(h, g.Focus.Y)
		h = mixBrushSeedF(h, g.StartRadius)
		h = mixBrushSeedF(h, g.EndRadius)
		return mixBrushSeedStops(h, g.Stops, g.Extend)
	case *SweepGradientBrush:
		if g == nil {
			return h
		}
		h = mixBrushSeed(h, 4)
		h = mixBrushSeedF(h, g.Center.X)
		h = mixBrushSeedF(h, g.Center.Y)
		h = mixBrushSeedF(h, g.StartAngle)
		h = mixBrushSeedF(h, g.EndAngle)
		return mixBrushSeedStops(h, g.Stops, g.Extend)
	case CustomBrush:
		h = mixBrushSeed(h, 5)
		for _, r := range g.Name {
			h = mixBrushSeed(h, uint64(r))
		}
		if g.Func != nil {
			h = mixBrushSeed(h, 1)
		}
		return h
	case opacityMulBrush:
		return mixBrushSeed(brushContentSeed(g.base), math.Float64bits(g.mul))
	case deviceSpaceBrush:
		return g.seed
	case *deviceSpaceBrush:
		if g == nil {
			return h
		}
		return g.seed
	}
	return mixBrushSeed(h, 99)
}

// brushInDeviceSpace re-expresses b in the device space of m. The returned
// bool reports whether the brush changed; solids and the identity matrix are
// left untouched so the common path stays allocation-free.
func brushInDeviceSpace(b Brush, m Matrix) (Brush, bool) {
	if b == nil || m.IsIdentity() {
		return b, false
	}
	if _, ok := b.(SolidBrush); ok {
		return b, false
	}

	scale, similar := m.SimilarityScale()
	det := m.A*m.E - m.B*m.D
	switch g := b.(type) {
	case *LinearGradientBrush:
		if g != nil && similar {
			out := *g
			out.Start = m.TransformPoint(g.Start)
			out.End = m.TransformPoint(g.End)
			return &out, true
		}
	case *RadialGradientBrush:
		if g != nil && similar {
			out := *g
			out.Center = m.TransformPoint(g.Center)
			out.Focus = m.TransformPoint(g.Focus)
			out.StartRadius = g.StartRadius * scale
			out.EndRadius = g.EndRadius * scale
			return &out, true
		}
	case *SweepGradientBrush:
		// A mirror reverses the sweep direction, which start/end angles alone
		// cannot express; keep mirrored sweeps on inverse sampling.
		if g != nil && similar && det > 0 {
			out := *g
			out.Center = m.TransformPoint(g.Center)
			rot := math.Atan2(m.D, m.A)
			out.StartAngle = g.StartAngle + rot
			out.EndAngle = g.EndAngle + rot
			return &out, true
		}
	}

	if math.Abs(det) < 1e-12 {
		return b, false
	}
	inv := m.Invert()
	seed := mixBrushSeedMatrix(brushContentSeed(b), inv)
	return deviceSpaceBrush{base: b, inv: inv, seed: seed}, true
}

// applyDeviceSpaceBrush temporarily re-expresses the current brush in device
// space so brush geometry follows the CTM. Paths are baked into device space
// before rasterization, so a gradient defined in user space would otherwise be
// sampled with device coordinates and stay pinned to the surface while its
// geometry moves. Skia keeps the gradient local and composes the CTM into the
// shader; this is the baked-geometry equivalent.
//
// Returns a restore func (always non-nil; safe to defer). Must run before the
// F1 opacity-group wrapper so that wrapper wraps the device-space brush.
func (c *Context) applyDeviceSpaceBrush() func() {
	if c == nil || c.paint == nil || c.paint.isSolid {
		return func() {}
	}
	m := c.totalMatrix()
	if m.IsIdentity() {
		return func() {}
	}
	savedBrush, savedPattern := c.paint.Brush, c.paint.Pattern

	// ImagePattern stores a device→image inverse of its own; compose ours into
	// it so the native tiled-pattern kernel keeps working.
	if pat, ok := c.paint.Pattern.(*ImagePattern); ok && pat != nil && math.Abs(m.A*m.E-m.B*m.D) > 1e-12 {
		src := pat.Source()
		src.Inverse = src.Inverse.Multiply(m.Invert())
		np := NewImagePatternFromSource(src)
		c.paint.Pattern = np
		c.paint.Brush = CustomBrush{Func: np.ColorAt, Name: "pattern"}
		return func() {
			c.paint.Brush = savedBrush
			c.paint.Pattern = savedPattern
		}
	}

	next, changed := brushInDeviceSpace(c.paint.GetBrush(), m)
	if !changed {
		return func() {}
	}
	c.paint.Brush = next
	c.paint.Pattern = PatternFromBrush(next)
	return func() {
		c.paint.Brush = savedBrush
		c.paint.Pattern = savedPattern
	}
}
