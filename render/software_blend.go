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

func compositeAdvanced(pixmap *Pixmap, x, y int, color RGBA, coverage uint8, mode BlendMode) {
	srcA := color.A * float64(coverage) / 255.0
	if srcA <= 0 && mode != BlendClear {
		return
	}
	dst := pixmap.GetPixel(x, y)
	cov := float64(coverage) / 255.0

	// Porter-Duff fixed modes (B.02): premul ops with coverage as source alpha scale.
	switch mode {
	case BlendClear:
		inv := 1.0 - cov
		dr, dg, db, da := pixmap.getPremul(x, y)
		pixmap.setPremul(x, y, dr*inv, dg*inv, db*inv, da*inv)
		return
	case BlendCopy:
		sa := color.A * cov
		inv := 1.0 - cov
		dr, dg, db, da := pixmap.getPremul(x, y)
		pixmap.setPremul(x, y,
			color.R*sa+dr*inv,
			color.G*sa+dg*inv,
			color.B*sa+db*inv,
			sa+da*inv,
		)
		return
	case BlendPlus:
		sa := color.A * cov
		dr, dg, db, da := pixmap.getPremul(x, y)
		add := func(a, b float64) float64 {
			v := a + b
			if v > 1 {
				return 1
			}
			if v < 0 {
				return 0
			}
			return v
		}
		pixmap.setPremul(x, y,
			add(color.R*sa, dr),
			add(color.G*sa, dg),
			add(color.B*sa, db),
			add(sa, da),
		)
		return
	case BlendModulate:
		// Premul component product: out = (src*cov) * dst
		sa := color.A * cov
		dr, dg, db, da := pixmap.getPremul(x, y)
		pixmap.setPremul(x, y,
			color.R*sa*dr,
			color.G*sa*dg,
			color.B*sa*db,
			sa*da,
		)
		return
	case BlendDestinationOut:
		sa := color.A * cov
		inv := 1.0 - sa
		dr, dg, db, da := pixmap.getPremul(x, y)
		pixmap.setPremul(x, y, dr*inv, dg*inv, db*inv, da*inv)
		return
	case BlendSourceAtop:
		sa := color.A * cov
		dr, dg, db, da := pixmap.getPremul(x, y)
		invs := 1.0 - sa
		pixmap.setPremul(x, y,
			color.R*sa*da+dr*invs,
			color.G*sa*da+dg*invs,
			color.B*sa*da+db*invs,
			sa*da+da*invs,
		)
		return
	case BlendXor:
		sa := color.A * cov
		dr, dg, db, da := pixmap.getPremul(x, y)
		pixmap.setPremul(x, y,
			color.R*sa*(1-da)+dr*(1-sa),
			color.G*sa*(1-da)+dg*(1-sa),
			color.B*sa*(1-da)+db*(1-sa),
			sa*(1-da)+da*(1-sa),
		)
		return
	}

	// Work in 0..255 straight channels for advanced/separable blend formulas.
	sr := uint8(clamp255(color.R * 255))
	sg := uint8(clamp255(color.G * 255))
	sb := uint8(clamp255(color.B * 255))
	sa := uint8(clamp255(srcA * 255))
	dr := uint8(clamp255(dst.R * 255))
	dg := uint8(clamp255(dst.G * 255))
	db := uint8(clamp255(dst.B * 255))
	da := uint8(clamp255(dst.A * 255))

	br, bg, bb := sr, sg, sb
	switch mode {
	case BlendMultiply:
		br = uint8((int(sr) * int(dr)) / 255)
		bg = uint8((int(sg) * int(dg)) / 255)
		bb = uint8((int(sb) * int(db)) / 255)
	case BlendScreen:
		br = uint8(255 - (255-int(sr))*(255-int(dr))/255)
		bg = uint8(255 - (255-int(sg))*(255-int(dg))/255)
		bb = uint8(255 - (255-int(sb))*(255-int(db))/255)
	case BlendOverlay:
		br = overlayChannelU8(sr, dr)
		bg = overlayChannelU8(sg, dg)
		bb = overlayChannelU8(sb, db)
	case BlendDarken:
		br, bg, bb = min3u8(sr, dr), min3u8(sg, dg), min3u8(sb, db)
	case BlendLighten:
		br, bg, bb = max3u8(sr, dr), max3u8(sg, dg), max3u8(sb, db)
	case BlendDifference:
		br, bg, bb = absDiff3u8(sr, dr), absDiff3u8(sg, dg), absDiff3u8(sb, db)
	case BlendExclusion:
		br, bg, bb = exclusionU8(sr, dr), exclusionU8(sg, dg), exclusionU8(sb, db)
	case BlendHardLight:
		br, bg, bb = hardLightU8(sr, dr), hardLightU8(sg, dg), hardLightU8(sb, db)
	case BlendSoftLight:
		br, bg, bb = softLightU8(sr, dr), softLightU8(sg, dg), softLightU8(sb, db)
	case BlendColorDodge:
		br, bg, bb = colorDodgeU8(sr, dr), colorDodgeU8(sg, dg), colorDodgeU8(sb, db)
	case BlendColorBurn:
		br, bg, bb = colorBurnU8(sr, dr), colorBurnU8(sg, dg), colorBurnU8(sb, db)
	case BlendHue, BlendSaturation, BlendColor, BlendLuminosity:
		br, bg, bb = advancedBlendRGB(mode, sr, sg, sb, dr, dg, db)
	}

	// Source-over of blended straight color with src alpha onto dst.
	srcAlpha := float64(sa) / 255.0
	dstAlpha := float64(da) / 255.0
	outA := srcAlpha + dstAlpha*(1-srcAlpha)
	if outA <= 0 {
		pixmap.SetPixel(x, y, Transparent)
		return
	}
	outR := (float64(br)/255.0*srcAlpha + float64(dr)/255.0*dstAlpha*(1-srcAlpha)) / outA
	outG := (float64(bg)/255.0*srcAlpha + float64(dg)/255.0*dstAlpha*(1-srcAlpha)) / outA
	outB := (float64(bb)/255.0*srcAlpha + float64(db)/255.0*dstAlpha*(1-srcAlpha)) / outA
	pixmap.SetPixel(x, y, RGBA{R: outR, G: outG, B: outB, A: outA})
}

func overlayChannelU8(src, dst uint8) uint8 {
	if dst < 128 {
		return uint8((2 * int(src) * int(dst)) / 255)
	}
	return uint8(255 - (2*(255-int(src))*(255-int(dst)))/255)
}

// blendAlphaRunsFromCoreRunsPaint is like blendAlphaRunsFromCoreRuns but samples
// the paint color at each pixel instead of using a single constant color.
// When clipFn is non-nil, each pixel's alpha is multiplied by the clip coverage.
// When maskFn is non-nil, each pixel's alpha is multiplied by the mask coverage.
// blendAlphaRunsFromCoreRunsPaint is like blendAlphaRunsFromCoreRuns but samples
// the paint color at each pixel instead of using a single constant color.
// When clipFn is non-nil, each pixel's alpha is multiplied by the clip coverage.
// When maskFn is non-nil, each pixel's alpha is multiplied by the mask coverage.
func min3u8(a, b uint8) uint8 { return min(a, b) }
func max3u8(a, b uint8) uint8 { return max(a, b) }
func absDiff3u8(a, b uint8) uint8 {
	if a >= b {
		return a - b
	}
	return b - a
}
func exclusionU8(s, d uint8) uint8 {
	return uint8(int(s) + int(d) - (2*int(s)*int(d))/255)
}
func hardLightU8(s, d uint8) uint8 {
	if s <= 127 {
		return uint8((2 * int(s) * int(d)) / 255)
	}
	return uint8(255 - (2*(255-int(s))*(255-int(d)))/255)
}
func softLightU8(s, d uint8) uint8 {
	sf := float64(s) / 255.0
	df := float64(d) / 255.0
	var r float64
	if sf <= 0.5 {
		r = df - (1.0-2.0*sf)*df*(1.0-df)
	} else {
		var d2 float64
		if df <= 0.25 {
			d2 = ((16.0*df-12.0)*df + 4.0) * df
		} else {
			d2 = math.Sqrt(df)
		}
		r = df + (2.0*sf-1.0)*(d2-df)
	}
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	return uint8(r * 255.0)
}
func colorDodgeU8(s, d uint8) uint8 {
	if s >= 255 {
		return 255
	}
	v := (int(d) * 255) / (255 - int(s))
	if v > 255 {
		return 255
	}
	return uint8(v)
}
func colorBurnU8(s, d uint8) uint8 {
	if s == 0 {
		return 0
	}
	v := 255 - ((255-int(d))*255)/int(s)
	if v < 0 {
		return 0
	}
	return uint8(v)
}

func advancedBlendRGB(mode BlendMode, sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	// Delegate to image package blend helpers via DrawParams blend path.
	// Reconstruct using same formulas as internal/image (keep in sync).
	switch mode {
	case BlendHue:
		return hslBlendHue(sr, sg, sb, dr, dg, db)
	case BlendSaturation:
		return hslBlendSat(sr, sg, sb, dr, dg, db)
	case BlendColor:
		return hslBlendColor(sr, sg, sb, dr, dg, db)
	case BlendLuminosity:
		return hslBlendLum(sr, sg, sb, dr, dg, db)
	default:
		return sr, sg, sb
	}
}

func hslLum(r, g, b uint8) float64 { return 0.3*float64(r) + 0.59*float64(g) + 0.11*float64(b) }
func hslSat(r, g, b uint8) float64 {
	maxv := float64(max3(r, g, b))
	minv := float64(min3(r, g, b))
	return maxv - minv
}
func max3(a, b, c uint8) uint8 { return max(a, b, c) }
func min3(a, b, c uint8) uint8 { return min(a, b, c) }
func hslClip(r, g, b float64) (uint8, uint8, uint8) {
	l := 0.3*r + 0.59*g + 0.11*b
	n := r
	if g < n {
		n = g
	}
	if b < n {
		n = b
	}
	x := r
	if g > x {
		x = g
	}
	if b > x {
		x = b
	}
	if n < 0 {
		r = l + (r-l)*l/(l-n)
		g = l + (g-l)*l/(l-n)
		b = l + (b-l)*l/(l-n)
	}
	if x > 255 {
		r = l + (r-l)*(255-l)/(x-l)
		g = l + (g-l)*(255-l)/(x-l)
		b = l + (b-l)*(255-l)/(x-l)
	}
	cl := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v + 0.5)
	}
	return cl(r), cl(g), cl(b)
}
func hslSetLum(r, g, b, l float64) (uint8, uint8, uint8) {
	d := l - (0.3*r + 0.59*g + 0.11*b)
	return hslClip(r+d, g+d, b+d)
}
func hslSetSat(r, g, b, s float64) (float64, float64, float64) {
	type ch struct {
		v float64
		i int
	}
	cs := [3]ch{{r, 0}, {g, 1}, {b, 2}}
	if cs[0].v > cs[1].v {
		cs[0], cs[1] = cs[1], cs[0]
	}
	if cs[1].v > cs[2].v {
		cs[1], cs[2] = cs[2], cs[1]
	}
	if cs[0].v > cs[1].v {
		cs[0], cs[1] = cs[1], cs[0]
	}
	minc, midc, maxc := cs[0], cs[1], cs[2]
	if maxc.v > minc.v {
		midc.v = ((midc.v - minc.v) * s) / (maxc.v - minc.v)
		maxc.v = s
	} else {
		midc.v, maxc.v = 0, 0
	}
	minc.v = 0
	out := [3]float64{}
	out[minc.i], out[midc.i], out[maxc.i] = minc.v, midc.v, maxc.v
	return out[0], out[1], out[2]
}
func hslBlendHue(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	r, g, b := hslSetSat(float64(sr), float64(sg), float64(sb), hslSat(dr, dg, db))
	return hslSetLum(r, g, b, hslLum(dr, dg, db))
}
func hslBlendSat(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	r, g, b := hslSetSat(float64(dr), float64(dg), float64(db), hslSat(sr, sg, sb))
	return hslSetLum(r, g, b, hslLum(dr, dg, db))
}
func hslBlendColor(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return hslSetLum(float64(sr), float64(sg), float64(sb), hslLum(dr, dg, db))
}
func hslBlendLum(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return hslSetLum(float64(dr), float64(dg), float64(db), hslLum(sr, sg, sb))
}
