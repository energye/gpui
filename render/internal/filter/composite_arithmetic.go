//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package filter

import (
	"github.com/energye/gpui/render"
)

// CompositeArithmetic implements SVG feComposite operator="arithmetic"
// (P3.8 4b-2), a line-by-line translation of resvg `composite.rs`:
//
//	result = k1·i1·i2 + k2·i1 + k3·i2 + k4
//
// computed per channel in the premul domain (inputs /255 → result clamped
// to [0,1] for alpha, [0,alpha] for RGB — the premul invariant), with
// alpha≈0 pixels fully transparent. src (current content) is i1; the aux
// ImageBuf is i2; both are region-sized.
type CompositeArithmetic struct {
	k1, k2, k3, k4 float32
	in2            *render.ImageBuf
	linearRGB      bool
}

// NewCompositeArithmetic builds the filter. linearRGB mirrors resvg's
// color-interpolation-filters default: inputs are converted into LinearRGB
// (demultiply → curve → premultiply), the arithmetic runs there, and the
// result is converted back to sRGB storage (the chain-wide equivalent of
// resvg's Image color_space bookkeeping for a single primitive).
func NewCompositeArithmetic(k1, k2, k3, k4 float32, in2 *render.ImageBuf, linearRGB bool) *CompositeArithmetic {
	return &CompositeArithmetic{k1: k1, k2: k2, k3: k3, k4: k4, in2: in2, linearRGB: linearRGB}
}

// Apply runs the arithmetic composition over the full surface.
func (f *CompositeArithmetic) Apply(src, dst *render.Pixmap) {
	w, h := src.Width(), src.Height()
	s1 := src.Data()
	d := dst.Data()
	var s2 []byte
	var dw, dh int
	if f.in2 != nil {
		s2 = f.in2.PremultipliedData()
		dw, dh = f.in2.Width(), f.in2.Height()
	}
	if f.linearRGB {
		// Convert both inputs into LinearRGB on private copies (resvg
		// into_color_space: demultiply → curve → premultiply).
		s1 = append([]byte(nil), s1...)
		svgConvertColorSpace(s1, true)
		if s2 != nil {
			s2 = append([]byte(nil), s2...)
			svgConvertColorSpace(s2, true)
		}
	}
	calc := func(i1, i2 uint8, max float32) float32 {
		a := float32(i1) / 255
		b := float32(i2) / 255
		r := f.k1*a*b + f.k2*a + f.k3*b + f.k4
		if r < 0 {
			r = 0
		}
		if r > max {
			r = max
		}
		return r
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			var c2 [4]uint8
			if x < dw && y < dh && s2 != nil {
				j := (y*dw + x) * 4
				copy(c2[:], s2[j:j+4])
			}
			a := calc(s1[i+3], c2[3], 1.0)
			if a <= 0 {
				// resvg: alpha approx-zero → leave transparent (dst cleared
				// by the pool overwrite copy; set explicitly for safety).
				d[i], d[i+1], d[i+2], d[i+3] = 0, 0, 0, 0
				continue
			}
			d[i] = uint8(calc(s1[i], c2[0], a) * 255)
			d[i+1] = uint8(calc(s1[i+1], c2[1], a) * 255)
			d[i+2] = uint8(calc(s1[i+2], c2[2], a) * 255)
			d[i+3] = uint8(a * 255)
		}
	}
	if f.linearRGB {
		svgConvertColorSpace(d, false)
	}
}
