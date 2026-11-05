//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package filter

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// BloomFilter glows the bright areas of a pixmap (P3.7 batch 5, CPU 8-bit leg).
// It is a thin adapter over BloomFloat — the very kernel the render/hdr
// float-domain door runs and the GPU four-pass graph reproduces — plus the
// RGBA8 exit that clamps at 1, which is what makes this leg an explicit LDR
// approximation rather than HDR.
type BloomFilter struct {
	Params render.BloomParams
}

// NewBloomFilter builds a bloom filter from the render layer's parameter
// struct (the single domain: render.BloomParams / render.MaxBloomSigma).
func NewBloomFilter(p render.BloomParams) *BloomFilter {
	return &BloomFilter{Params: p}
}

// Apply runs the bright pass, the separable Gaussian and the additive return
// over bounds. src and dst must not be the same pixmap. Out-of-domain
// parameters leave dst untouched — the node never guesses a parameter into a
// filter, and an identity copies through instead of occupying a pass.
func (f *BloomFilter) Apply(src, dst *render.Pixmap, bounds scene.Rect) {
	if f == nil || src == nil || dst == nil || src == dst {
		return
	}
	p := f.Params
	if err := p.Validate(); err != nil {
		return
	}
	if p.IsIdentity() {
		copyPixmapRegion(src, dst, bounds)
		return
	}

	minX := clampInt(int(bounds.MinX), 0, src.Width())
	maxX := clampInt(int(bounds.MaxX), 0, src.Width())
	minY := clampInt(int(bounds.MinY), 0, src.Height())
	maxY := clampInt(int(bounds.MaxY), 0, src.Height())
	if minX >= maxX || minY >= maxY {
		return
	}
	w, h := maxX-minX, maxY-minY
	sw := src.Width()
	sd, dd := src.Data(), dst.Data()

	// Pixmap storage is premultiplied uint8; the kernel works on 0..1 floats,
	// and the GPU leg samples the same premultiplied texels, so the two legs
	// agree on the same domain before any math happens.
	in := getTempBuffer(w, h)
	defer putTempBuffer(in)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := ((minY+y)*sw + minX + x) * 4
			di := (y*w + x) * 4
			in[di] = float32(sd[si]) / 255
			in[di+1] = float32(sd[si+1]) / 255
			in[di+2] = float32(sd[si+2]) / 255
			in[di+3] = float32(sd[si+3]) / 255
		}
	}

	out := getTempBuffer(w, h)
	defer putTempBuffer(out)
	BloomFloat(out, in, w, h, p.Threshold, p.Sigma, p.Intensity)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := ((minY+y)*sw + minX + x) * 4
			di := (y*w + x) * 4
			// The LDR exit: the summed light above 1 lands on white here,
			// while the float leg keeps it for the tonemap.
			dd[si] = clampUint8(out[di] * 255)
			dd[si+1] = clampUint8(out[di+1] * 255)
			dd[si+2] = clampUint8(out[di+2] * 255)
			dd[si+3] = clampUint8(out[di+3] * 255)
		}
	}
}
