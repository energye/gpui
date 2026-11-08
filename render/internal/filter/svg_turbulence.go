//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package filter

import (
	"math"

	"github.com/energye/gpui/render"
)

// SVGTurbulence implements SVG feTurbulence (P3.8 4b-3), a line-by-line
// translation of resvg `src/filter/turbulence.rs` (SVG 1.1 spec appendix
// algorithm): the LCG-seeded Perlin lattice init, the two modes
// (fractalNoise sums the noise, turbulence sums |noise|), the numOctaves
// zoom loop, and the stitchTiles frequency adjustment + lattice wrap
// bookkeeping (resvg keeps the wrap state per octave — translated verbatim,
// including its per-pixel tile_x quirk).
//
// resvg's apply() writes UNPREMULTIPLIED pixels; mod.rs apply_turbulence
// then premultiplies (multiply_alpha) and marks the Image with the filter's
// color-interpolation-filters space, converting back to sRGB once at canvas
// apply. For the single-primitive chains the conformance corpus exercises,
// the equivalent here is: generate unpremul → premultiply → linearRGB ?
// convert to sRGB storage (the svgConvertColorSpace(false) curve round trip
// used by the lighting pair).
//
// Coordinates: resvg samples the noise at (pixel + region_offset) / canvas
// scale to get user space. This engine's filter buffers are user-space 1:1
// (the region origin is subtracted at evaluator setup), so the svg layer
// passes offset = region origin and the scale division folds away — the
// corpus transforms keep resvg's get_scale at (1,1) too (skew only enters
// the matrix off-diagonal, which get_scale ignores).

// Turbulence lattice constants (resvg turbulence.rs verbatim).
const (
	svgRandM   = 2147483647 // 2**31 - 1
	svgRandA   = 16807      // 7**5; primitive root of m
	svgRandQ   = 127773     // m / a
	svgRandR   = 2836       // m % a
	svgBSize   = 0x100
	svgBSize32 = 0x100
	svgBLen    = svgBSize + svgBSize + 2
	svgBM      = 0xff
	svgPerlinN = 0x1000
)

// svgStitchInfo carries the stitchTiles wrap state (resvg StitchInfo).
type svgStitchInfo struct {
	width, height int // how much to subtract to wrap for stitching
	wrapX, wrapY  int // minimum value to wrap
}

// SVGTurbulenceFilter generates the fractal noise over the full surface.
type SVGTurbulenceFilter struct {
	p render.SVGTurbulenceParams
}

// NewSVGTurbulence builds the filter.
func NewSVGTurbulence(p render.SVGTurbulenceParams) *SVGTurbulenceFilter {
	return &SVGTurbulenceFilter{p: p}
}

// svgTurbRandom is resvg's LCG step (Schrage's method; no int32 overflow).
func svgTurbRandom(seed int32) int32 {
	result := svgRandA*(seed%svgRandQ) - svgRandR*(seed/svgRandQ)
	if result <= 0 {
		result += svgRandM
	}
	return result
}

// svgTurbInit builds the lattice selector + unit-gradient tables
// (resvg init() verbatim; the random walk consumes the seed identically, so
// the lattice is bit-identical for equal seeds).
func svgTurbInit(seed int32) ([svgBLen]int, [4][svgBLen][2]float64) {
	s := seed
	if s <= 0 {
		s = -s%(svgRandM-1) + 1
	}
	if s > svgRandM-1 {
		s = svgRandM - 1
	}
	var ls [svgBLen]int
	var gradient [4][svgBLen][2]float64
	for k := 0; k < 4; k++ {
		for i := 0; i < svgBSize; i++ {
			ls[i] = i
			for j := 0; j < 2; j++ {
				s = svgTurbRandom(s)
				gradient[k][i][j] = float64(s%(svgBSize32+svgBSize32)-svgBSize32) / float64(svgBSize32)
			}
			ss := math.Sqrt(gradient[k][i][0]*gradient[k][i][0] + gradient[k][i][1]*gradient[k][i][1])
			gradient[k][i][0] /= ss
			gradient[k][i][1] /= ss
		}
	}
	for i := svgBSize - 1; i > 0; i-- {
		k := ls[i]
		s = svgTurbRandom(s)
		j := int(s % svgBSize32)
		ls[i] = ls[j]
		ls[j] = k
	}
	for i := 0; i < svgBSize+2; i++ {
		ls[svgBSize+i] = ls[i]
		for g := 0; g < 4; g++ {
			gradient[g][svgBSize+i][0] = gradient[g][i][0]
			gradient[g][svgBSize+i][1] = gradient[g][i][1]
		}
	}
	return ls, gradient
}

// svgTurbNoise2 is resvg noise2() verbatim: the 2×2 lattice cell around
// (x, y), stitch wrap adjustment before the mask, s-curve bilinear blend of
// the four gradient dot products.
func svgTurbNoise2(channel int, x, y float64, ls *[svgBLen]int, gradient *[4][svgBLen][2]float64, info *svgStitchInfo) float64 {
	t := x + float64(svgPerlinN)
	bx0 := int(t)
	bx1 := bx0 + 1
	rx0 := t - float64(int64(t))
	rx1 := rx0 - 1.0
	t = y + float64(svgPerlinN)
	by0 := int(t)
	by1 := by0 + 1
	ry0 := t - float64(int64(t))
	ry1 := ry0 - 1.0

	if info != nil {
		if bx0 >= info.wrapX {
			bx0 -= info.width
		}
		if bx1 >= info.wrapX {
			bx1 -= info.width
		}
		if by0 >= info.wrapY {
			by0 -= info.height
		}
		if by1 >= info.wrapY {
			by1 -= info.height
		}
	}

	bx0 &= svgBM
	bx1 &= svgBM
	by0 &= svgBM
	by1 &= svgBM
	i := ls[bx0]
	j := ls[bx1]
	b00 := ls[i+by0]
	b10 := ls[j+by0]
	b01 := ls[i+by1]
	b11 := ls[j+by1]
	sx := svgTurbSCurve(rx0)
	sy := svgTurbSCurve(ry0)
	q := &gradient[channel][b00]
	u := rx0*q[0] + ry0*q[1]
	q = &gradient[channel][b10]
	v := rx1*q[0] + ry0*q[1]
	a := svgTurbLerp(sx, u, v)
	q = &gradient[channel][b01]
	u = rx0*q[0] + ry1*q[1]
	q = &gradient[channel][b11]
	v = rx1*q[0] + ry1*q[1]
	b := svgTurbLerp(sx, u, v)
	return svgTurbLerp(sy, a, b)
}

// svgTurbulence is resvg turbulence() verbatim: the stitchTiles frequency
// adjustment (lo/hi freq nearest to a whole lattice cell), the octave zoom
// loop with the stitch wrap update, and the fractal (signed) vs turbulence
// (abs) summation.
func svgTurbulenceOctaves(channel int, x, y, tileX, tileY, tileW, tileH, baseFreqX, baseFreqY float64, numOctaves int, fractalSum, doStitching bool, ls *[svgBLen]int, gradient *[4][svgBLen][2]float64) float64 {
	var stitch *svgStitchInfo
	if doStitching {
		// When stitching tiled turbulence, the frequencies must be adjusted
		// so that the tile borders will be continuous.
		if !svgF32ApproxZero(baseFreqX) {
			loFreq := math.Floor(tileW*baseFreqX) / tileW
			hiFreq := math.Ceil(tileW*baseFreqX) / tileW
			if baseFreqX/loFreq < hiFreq/baseFreqX {
				baseFreqX = loFreq
			} else {
				baseFreqX = hiFreq
			}
		}
		if !svgF32ApproxZero(baseFreqY) {
			loFreq := math.Floor(tileH*baseFreqY) / tileH
			hiFreq := math.Ceil(tileH*baseFreqY) / tileH
			if baseFreqY/loFreq < hiFreq/baseFreqY {
				baseFreqY = loFreq
			} else {
				baseFreqY = hiFreq
			}
		}
		// Set up initial stitch values.
		width := int(tileW*baseFreqX + 0.5)
		height := int(tileH*baseFreqY + 0.5)
		wrapX := int(tileX*baseFreqX + float64(svgPerlinN) + float64(width))
		wrapY := int(tileY*baseFreqY + float64(svgPerlinN) + float64(height))
		stitch = &svgStitchInfo{width: width, height: height, wrapX: wrapX, wrapY: wrapY}
	}

	sum := 0.0
	x *= baseFreqX
	y *= baseFreqY
	ratio := 1.0
	for i := 0; i < numOctaves; i++ {
		if fractalSum {
			sum += svgTurbNoise2(channel, x, y, ls, gradient, stitch) / ratio
		} else {
			sum += math.Abs(svgTurbNoise2(channel, x, y, ls, gradient, stitch)) / ratio
		}
		x *= 2.0
		y *= 2.0
		ratio *= 2.0

		if stitch != nil {
			// Update stitch values. Subtracting PerlinN before the
			// multiplication and adding it afterward simplifies to
			// subtracting it once.
			stitch.width *= 2
			stitch.wrapX = 2*stitch.wrapX - svgPerlinN
			stitch.height *= 2
			stitch.wrapY = 2*stitch.wrapY - svgPerlinN
		}
	}
	return sum
}

// Apply generates the noise over the full surface (the source content is
// ignored — resvg apply_turbulence has no input image). Output: unpremul
// noise → premultiply → linearRGB ? back to sRGB storage.
func (f *SVGTurbulenceFilter) Apply(src, dst *render.Pixmap) {
	_ = src
	w, h := dst.Width(), dst.Height()
	data := dst.Data()
	if w == 0 || h == 0 {
		return
	}
	p := f.p
	ls, gradient := svgTurbInit(p.Seed)
	tileW, tileH := float64(w), float64(h)
	bfx := float64(p.BaseFrequencyX)
	bfy := float64(p.BaseFrequencyY)

	x, y := 0, 0
	for i := 0; i+3 < len(data); i += 4 {
		// resvg: (pixel + offset) / canvas scale — user-space buffer folds
		// the scale away (see file comment).
		tx := float64(x) + float64(p.OffsetX)
		ty := float64(y) + float64(p.OffsetY)
		for ch := 0; ch < 4; ch++ {
			n := svgTurbulenceOctaves(ch, tx, ty, float64(x), float64(y), tileW, tileH,
				bfx, bfy, p.NumOctaves, p.FractalNoise, p.StitchTiles, &ls, &gradient)
			var v float64
			if p.FractalNoise {
				v = (n*255.0 + 255.0) / 2.0
			} else {
				v = n * 255.0
			}
			// resvg: (f32_bound(0.0, n as f32, 255.0) + 0.5) as u8 — the
			// f32 narrowing happens BEFORE the clamp.
			vf := float32(v)
			if vf < 0 {
				vf = 0
			}
			if vf > 255 {
				vf = 255
			}
			data[i+ch] = uint8(vf + 0.5)
		}
		x++
		if x == w {
			x = 0
			y++
		}
	}

	// resvg mod.rs apply_turbulence: multiply_alpha right after generation;
	// the linearRGB marking converts back once at canvas apply — folded here
	// for the single-primitive chain (see file comment).
	svgMultiplyAlpha(data)
	if p.LinearRGB {
		svgConvertColorSpace(data, false)
	}
}

func svgTurbSCurve(t float64) float64 {
	return t * t * (3.0 - 2.0*t)
}

func svgTurbLerp(t, a, b float64) float64 {
	return a + t*(b-a)
}

// svgF32ApproxZero mirrors usvg ApproxZeroUlps(4) on f32: only exact ±0 and
// the first four denormal steps count as zero (one f32 ULP at zero is
// 2^-149 ≈ 1.4e-45). Values arrive as f32 widened to f64 (usvg stores f32).
func svgF32ApproxZero(v float64) bool {
	return math.Abs(v) <= 5.605193966189168e-45 // 4 × f32 denormal ULP
}
