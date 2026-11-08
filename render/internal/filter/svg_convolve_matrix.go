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

// SVGConvolveMatrix implements SVG feConvolveMatrix (P3.8 4b-3), a
// line-by-line translation of resvg `src/filter/convolve_matrix.rs`:
// the kernel flipped against the sample window (get(columns-ox-1,
// rows-oy-1)), the three edge modes (none skips, duplicate clamps, wrap
// modulos), the divisor/bias output curve, and the preserveAlpha contract —
// resvg mod.rs apply_convolve_matrix runs the kernel on straight-alpha
// input when preserveAlpha (demultiply first, alpha passes through) and on
// premultiplied input otherwise.
//
// The input arrives premultiplied sRGB; for linearRGB it is converted into
// LinearRGB first (resvg into_color_space), convolved there, and the output
// converted back to sRGB storage (the chain-wide Image color_space
// bookkeeping for a single primitive — same stance as the arithmetic
// composite). Output is legal premul RGBA8 by construction: non-preserve
// RGB clamps to the new alpha; preserve RGB clamps to [0,1] then multiplies
// by it.

// SVGConvolveMatrixFilter convolves the content with the kernel matrix.
type SVGConvolveMatrixFilter struct {
	p render.SVGConvolveMatrixParams
}

// NewSVGConvolveMatrix builds the filter.
func NewSVGConvolveMatrix(p render.SVGConvolveMatrixParams) *SVGConvolveMatrixFilter {
	return &SVGConvolveMatrixFilter{p: p}
}

// Apply runs the convolution over the full surface. src is the premul
// back buffer (read-only); dst receives the result.
func (f *SVGConvolveMatrixFilter) Apply(src, dst *render.Pixmap) {
	w, h := src.Width(), src.Height()
	sData := append([]byte(nil), src.Data()...) // resvg allocates a back-buffer copy
	if f.p.LinearRGB {
		svgConvertColorSpace(sData, true)
	}
	if f.p.PreserveAlpha {
		svgDemultiply(sData)
	}
	dData := dst.Data()
	if w == 0 || h == 0 {
		return
	}
	cols, rows := f.p.Columns, f.p.Rows
	kernel := f.p.Kernel
	divisor := f.p.Divisor
	bias := f.p.Bias
	widthMax := w - 1
	heightMax := h - 1
	preserve := f.p.PreserveAlpha
	bound := func(min, val, max int) int {
		if val < min {
			return min
		}
		if val > max {
			return max
		}
		return val
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var newR, newG, newB, newA float32
			for oy := 0; oy < rows; oy++ {
				for ox := 0; ox < cols; ox++ {
					tx := x - f.p.TargetX + ox
					ty := y - f.p.TargetY + oy

					switch f.p.EdgeMode {
					case render.SVGEdgeModeNone:
						if tx < 0 || tx > widthMax || ty < 0 || ty > heightMax {
							continue
						}
					case render.SVGEdgeModeDuplicate:
						tx = bound(0, tx, widthMax)
						ty = bound(0, ty, heightMax)
					case render.SVGEdgeModeWrap:
						for tx < 0 {
							tx += w
						}
						tx %= w
						for ty < 0 {
							ty += h
						}
						ty %= h
					}

					// Kernel flip: get(columns-ox-1, rows-oy-1), row-major.
					k := kernel[(rows-oy-1)*cols+(cols-ox-1)]
					pOff := (ty*w + tx) * 4
					newR += float32(sData[pOff]) / 255.0 * k
					newG += float32(sData[pOff+1]) / 255.0 * k
					newB += float32(sData[pOff+2]) / 255.0 * k
					if !preserve {
						newA += float32(sData[pOff+3]) / 255.0 * k
					}
				}
			}

			inOff := (y*w + x) * 4
			if preserve {
				newA = float32(sData[inOff+3]) / 255.0
			} else {
				newA = newA/divisor + bias
			}
			boundedA := svgF32Bound(0.0, newA, 1.0)
			calc := func(v float32) float32 {
				v = v/divisor + bias*newA
				if preserve {
					return svgF32Bound(0.0, v, 1.0) * boundedA
				}
				return svgF32Bound(0.0, v, boundedA)
			}
			dData[inOff] = uint8(calc(newR)*255.0 + 0.5)
			dData[inOff+1] = uint8(calc(newG)*255.0 + 0.5)
			dData[inOff+2] = uint8(calc(newB)*255.0 + 0.5)
			dData[inOff+3] = uint8(boundedA*255.0 + 0.5)
		}
	}

	if f.p.LinearRGB {
		svgConvertColorSpace(dData, false)
	}
}

// svgF32Bound is resvg f32_bound (clamp).
func svgF32Bound(min, val, max float32) float32 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// svgDemultiply is resvg demultiply_alpha verbatim ((p/a + 0.5) as u8; a==0
// gives 0/0 = NaN → Rust's saturating cast maps NaN to 0, and premul data
// has c==0 there too).
func svgDemultiply(data []byte) {
	for i := 0; i+3 < len(data); i += 4 {
		a := float32(data[i+3]) / 255
		if a == 0 {
			data[i], data[i+1], data[i+2] = 0, 0, 0
			continue
		}
		data[i] = uint8(float32(data[i])/a + 0.5)
		data[i+1] = uint8(float32(data[i+1])/a + 0.5)
		data[i+2] = uint8(float32(data[i+2])/a + 0.5)
	}
}

// svgMultiplyAlpha is resvg multiply_alpha verbatim ((p*a + 0.5) as u8,
// premul storage from the unpremul turbulence output).
func svgMultiplyAlpha(data []byte) {
	for i := 0; i+3 < len(data); i += 4 {
		a := float32(data[i+3]) / 255
		data[i+2] = uint8(float32(data[i+2])*a + 0.5)
		data[i+1] = uint8(float32(data[i+1])*a + 0.5)
		data[i] = uint8(float32(data[i])*a + 0.5)
	}
}
