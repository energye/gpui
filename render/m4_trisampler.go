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
)

func newTriSampler(p0, p1, p2 Point, uv [3][2]float64) triSampler {
	den := (p1.Y-p2.Y)*(p0.X-p2.X) + (p2.X-p1.X)*(p0.Y-p2.Y)
	return triSampler{
		x2: p2.X, y2: p2.Y,
		a0: p1.Y - p2.Y, b0: p2.X - p1.X,
		a1: p2.Y - p0.Y, b1: p0.X - p2.X,
		den: den, uvs: uv,
		degen: math.Abs(den) < 1e-12,
	}
}

// uv interpolates UV with barycentric weights; shared diagonal goes to tri1.
// uv interpolates UV with barycentric weights; shared diagonal goes to tri1.
func (t *triSampler) uv(px, py float64) (float64, float64, bool) {
	if t == nil || t.degen {
		return 0, 0, false
	}
	dx := px - t.x2
	dy := py - t.y2
	w0 := (t.a0*dx + t.b0*dy) / t.den
	w1 := (t.a1*dx + t.b1*dy) / t.den
	w2 := 1 - w0 - w1
	const eps = -1e-9
	if w0 < eps || w1 < eps || w2 < eps {
		return 0, 0, false
	}
	return w0*t.uvs[0][0] + w1*t.uvs[1][0] + w2*t.uvs[2][0],
		w0*t.uvs[0][1] + w1*t.uvs[1][1] + w2*t.uvs[2][1], true
}

// PushBackdropLayer creates a layer pre-filled with a snapshot of the current
// parent canvas (L.05 backdrop subset). Subsequent drawing/filters operate over
// that backdrop; PopLayer composites with the given blend/opacity.
