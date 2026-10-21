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

// strokeDeviceScale returns the maximum CTM axis scale multiplied by the
// device scale. It is the same scale used by the GPU stroke path when it
// bakes paint.TransformScale, and by the CPU path when it maps user-space
// stroke outlines through the CTM and deviceMatrix.
func (c *Context) strokeDeviceScale() float64 {
	scale := 1.0
	if c != nil && !c.matrix.IsIdentity() {
		if sx := math.Hypot(c.matrix.A, c.matrix.D); sx > scale {
			scale = sx
		}
		if sy := math.Hypot(c.matrix.B, c.matrix.E); sy > scale {
			scale = sy
		}
	}
	if c != nil {
		if ds := c.DeviceScale(); ds > 0 {
			scale *= ds
		}
	}
	return scale
}

// setThinStrokeWidth updates both the legacy width and the active Stroke
// struct so EffectiveLineWidth sees the new value.
func setThinStrokeWidth(p *Paint, w float64) {
	p.LineWidth = w
	if p.Stroke != nil {
		p.Stroke.Width = w
	}
}

// thinStrokePaint approximates a positive sub-pixel solid stroke as exactly
// one device pixel with alpha multiplied by the original device width.
//
// Skia rasterizes a 0.5px band as 50% coverage; for axis-aligned strokes that
// is pixel-identical to a 1px band at 50% alpha (integer anchor: 25% on two
// rows; half-pixel anchor: 50% on one row). The approximation keeps the same
// total ink while avoiding backend AA-band quantisation (the GPU band
// tessellator under/over-covers thin bands depending on sub-pixel position,
// and the 4x software edge builder drops widths below 0.25px).
//
// Returns nil for hairlines (width <= 0), normal strokes (device width >= 1),
// and non-solid paints (gradients/patterns keep the exact-width path).
func thinStrokePaint(p *Paint, scale float64) *Paint {
	if p == nil || scale <= 0 {
		return nil
	}
	devW := p.EffectiveLineWidth() * scale
	if devW <= 0 || devW >= 1 {
		return nil
	}
	c, ok := p.SolidColor()
	if !ok || c.A <= 0 {
		return nil
	}
	cp := p.Clone()
	cp.SetBrush(SolidRGBA(c.R, c.G, c.B, c.A*devW))
	setThinStrokeWidth(cp, 1/scale)
	return cp
}
