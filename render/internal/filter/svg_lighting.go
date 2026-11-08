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

// SVGLighting implements the SVG feDiffuseLighting/feSpecularLighting pair
// (P3.8 4b-2). This is a line-by-line translation of resvg
// `src/filter/lighting.rs` — the normal-map kernels (9 boundary cases with
// per-case factors), the three light-source vectors (Distant/Point/Spot,
// Spot's cone color clamp), and the two mode formulas:
//
//	diffuse:  factor = kd · (N·L/|N|)   (flat pixels: kd · L.z), alpha = 255
//	specular: H = L + (0,0,1); factor = ks · ((N·H)/(|N|·|H|))^exp, alpha = max(R,G,B)
//
// The height map is the SOURCE ALPHA channel (premul storage is fine: only
// A is read). Both outputs are legal premul RGBA8 (diffuse A=255; specular
// RGB ≤ A by construction). Surfaces smaller than 3×3 produce a fully
// transparent result (resvg returns without writing). CPU-only by design
// for now — the GPU variant is a registered follow-up batch (same stance as
// turbulence/convolve: deterministic offline comparison first).

// (Parameters ride render.SVGLightingParams — the public SVG attribute set;
// this file holds the resvg lighting.rs translation itself.)

type svgVec3 struct{ x, y, z float32 }

func (v svgVec3) dot(o svgVec3) float32 { return v.x*o.x + v.y*o.y + v.z*o.z }
func (v svgVec3) length() float32       { return float32(math.Sqrt(float64(v.x*v.x + v.y*v.y + v.z*v.z))) }

func (v svgVec3) sub(o svgVec3) svgVec3 {
	return svgVec3{v.x - o.x, v.y - o.y, v.z - o.z}
}

func (v svgVec3) add(o svgVec3) svgVec3 {
	return svgVec3{v.x + o.x, v.y + o.y, v.z + o.z}
}

func (v svgVec3) normalized() svgVec3 {
	l := v.length()
	if l == 0 {
		return v // resvg: normalized().unwrap_or(v) — degenerate falls back to itself
	}
	return svgVec3{v.x / l, v.y / l, v.z / l}
}

// svgNormal carries the raw integer normal estimate and the per-case scaling
// factors (resvg Normal).
type svgNormal struct {
	fx, fy float32
	nx, ny int16
}

// SVGLightingFilter writes the lit image over the full surface.
type SVGLightingFilter struct {
	p render.SVGLightingParams
}

// NewSVGLighting builds the filter.
func NewSVGLighting(p render.SVGLightingParams) *SVGLightingFilter { return &SVGLightingFilter{p: p} }

func svgAlphaAt(data []uint8, w, h, x, y int) uint8 {
	if x < 0 || x >= w || y < 0 || y >= h {
		return 0
	}
	return data[(y*w+x)*4+3]
}

func svgBound(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// Apply runs the lighting over the whole surface (src alpha = height map).
func (f *SVGLightingFilter) Apply(src, dst *render.Pixmap) {
	w, h := src.Width(), src.Height()
	srcData, dstData := src.Data(), dst.Data()
	// resvg: dest starts transparent; surfaces smaller than 3×3 stay empty.
	for i := range dstData {
		dstData[i] = 0
	}
	if w < 3 || h < 3 {
		return
	}

	p := f.p
	// Distant light has a fixed vector; the others recompute per pixel.
	fixed := svgVec3{1, 1, 1}
	if p.LightType == 0 {
		az := float64(p.Azimuth) * math.Pi / 180
		el := float64(p.Elevation) * math.Pi / 180
		fixed = svgVec3{
			float32(math.Cos(az) * math.Cos(el)),
			float32(math.Sin(az) * math.Cos(el)),
			float32(math.Sin(el)),
		}
	}
	lc := [3]uint8{
		uint8(p.LightColor.R*255 + 0.5),
		uint8(p.LightColor.G*255 + 0.5),
		uint8(p.LightColor.B*255 + 0.5),
	}

	calc := func(x, y int, n svgNormal) {
		var lv svgVec3
		switch p.LightType {
		case 1, 2:
			nz := float32(svgAlphaAt(srcData, w, h, x, y)) / 255 * p.SurfaceScale
			origin := svgVec3{p.LightX, p.LightY, p.LightZ}
			lv = origin.sub(svgVec3{float32(x), float32(y), nz}).normalized()
		default:
			lv = fixed
		}

		lightColor := lc
		if p.LightType == 2 {
			// resvg light_color: cone clamp against the spot direction.
			origin := svgVec3{p.LightX, p.LightY, p.LightZ}
			s := svgVec3{p.PointsAtX, p.PointsAtY, p.PointsAtZ}.sub(origin).normalized()
			mls := -(lv.dot(s))
			if mls <= 0 {
				lightColor = [3]uint8{0, 0, 0}
			} else if p.HasLimitingCone && mls < float32(math.Cos(float64(p.LimitingConeAngle)*math.Pi/180)) {
				lightColor = [3]uint8{0, 0, 0}
			} else {
				fac := float32(math.Pow(float64(mls), float64(p.SpotSpecularExponent)))
				comp := func(c uint8) uint8 {
					return uint8(svgBound(float32(c)*fac) + 0.5)
				}
				lightColor = [3]uint8{comp(lc[0]), comp(lc[1]), comp(lc[2])}
			}
		}

		// light_factor (resvg diffuse_lighting / specular_lighting): the
		// flat-normal branches differ per mode — diffuse uses L.z directly,
		// specular uses (H.z/|H|)^exp with H = L + (0,0,1).
		var factor float32
		specPow := func(v float32) float32 {
			if p.SpecularExponent == 1 {
				return v
			}
			return float32(math.Pow(float64(v), float64(p.SpecularExponent)))
		}
		if n.nx == 0 && n.ny == 0 {
			if !p.Specular {
				factor = lv.z
			} else {
				hv := lv.add(svgVec3{0, 0, 1})
				hl := hv.length()
				if hl == 0 {
					factor = 0
				} else {
					factor = specPow(hv.z / hl)
				}
			}
		} else {
			nx := float32(n.nx) * (p.SurfaceScale / 255)
			ny := float32(n.ny) * (p.SurfaceScale / 255)
			nx *= n.fx
			ny *= n.fy
			N := svgVec3{nx, ny, 1}
			if !p.Specular {
				factor = N.dot(lv) / N.length()
			} else {
				hv := lv.add(svgVec3{0, 0, 1})
				hl := hv.length()
				if hl == 0 {
					factor = 0
				} else {
					factor = specPow(N.dot(hv) / N.length() / hl)
				}
			}
		}
		if !p.Specular {
			factor *= p.DiffuseConstant
		} else {
			factor *= p.SpecularConstant
		}

		comp := func(c uint8) uint8 {
			return uint8(svgBound(float32(c)*factor) + 0.5)
		}
		r, g, b := comp(lightColor[0]), comp(lightColor[1]), comp(lightColor[2])
		a := uint8(255)
		if p.Specular {
			a = maxU8(maxU8(r, g), b)
		}
		i := (y*w + x) * 4
		dstData[i], dstData[i+1], dstData[i+2], dstData[i+3] = r, g, b, a
	}
	// resvg: the result Image carries the filter's color-interpolation-filters
	// space and is converted back to sRGB on apply_to_canvas — linear mode
	// therefore brightens the lit bytes (lighting_color stays sRGB; only the
	// output passes the curve).
	if p.LinearRGB {
		svgConvertColorSpace(dstData, false)
	}

	// The 9 boundary kernels, resvg verbatim (alpha-only inputs; factors
	// per case: corners 2/3, top/bottom rows 1/3·1/2, side columns
	// 1/2·1/3, interior 1/4).
	calc(0, 0, svgNormal{
		2.0 / 3, 2.0 / 3,
		int16(-2*int(svgAlphaAt(srcData, w, h, 0, 0)) + 2*int(svgAlphaAt(srcData, w, h, 1, 0)) - int(svgAlphaAt(srcData, w, h, 0, 1)) + int(svgAlphaAt(srcData, w, h, 1, 1))),
		int16(-2*int(svgAlphaAt(srcData, w, h, 0, 0)) - int(svgAlphaAt(srcData, w, h, 1, 0)) + 2*int(svgAlphaAt(srcData, w, h, 0, 1)) + int(svgAlphaAt(srcData, w, h, 1, 1))),
	})
	calc(w-1, 0, svgNormal{
		2.0 / 3, 2.0 / 3,
		int16(-2*int(svgAlphaAt(srcData, w, h, w-2, 0)) + 2*int(svgAlphaAt(srcData, w, h, w-1, 0)) - int(svgAlphaAt(srcData, w, h, w-2, 1)) + int(svgAlphaAt(srcData, w, h, w-1, 1))),
		int16(-int(svgAlphaAt(srcData, w, h, w-2, 0)) - 2*int(svgAlphaAt(srcData, w, h, w-1, 0)) + int(svgAlphaAt(srcData, w, h, w-2, 1)) + 2*int(svgAlphaAt(srcData, w, h, w-1, 1))),
	})
	calc(0, h-1, svgNormal{
		2.0 / 3, 2.0 / 3,
		int16(-int(svgAlphaAt(srcData, w, h, 0, h-2)) + int(svgAlphaAt(srcData, w, h, 1, h-2)) - 2*int(svgAlphaAt(srcData, w, h, 0, h-1)) + 2*int(svgAlphaAt(srcData, w, h, 1, h-1))),
		int16(-2*int(svgAlphaAt(srcData, w, h, 0, h-2)) - int(svgAlphaAt(srcData, w, h, 1, h-2)) + 2*int(svgAlphaAt(srcData, w, h, 0, h-1)) + int(svgAlphaAt(srcData, w, h, 1, h-1))),
	})
	calc(w-1, h-1, svgNormal{
		2.0 / 3, 2.0 / 3,
		int16(-int(svgAlphaAt(srcData, w, h, w-2, h-2)) + int(svgAlphaAt(srcData, w, h, w-1, h-2)) - 2*int(svgAlphaAt(srcData, w, h, w-2, h-1)) + 2*int(svgAlphaAt(srcData, w, h, w-1, h-1))),
		int16(-int(svgAlphaAt(srcData, w, h, w-2, h-2)) - 2*int(svgAlphaAt(srcData, w, h, w-1, h-2)) + int(svgAlphaAt(srcData, w, h, w-2, h-1)) + 2*int(svgAlphaAt(srcData, w, h, w-1, h-1))),
	})
	for x := 1; x < w-1; x++ {
		a := func(dx, dy int) int { return int(svgAlphaAt(srcData, w, h, x+dx, dy)) }
		calc(x, 0, svgNormal{
			1.0 / 3, 1.0 / 2,
			int16(-2*a(-1, 0) + 2*a(1, 0) - a(-1, 1) + a(1, 1)),
			int16(-a(-1, 0) - 2*a(0, 0) - a(1, 0) + a(-1, 1) + 2*a(0, 1) + a(1, 1)),
		})
		tl, t, tr := a(-1, h-2), a(0, h-2), a(1, h-2)
		l, c, r := a(-1, h-1), a(0, h-1), a(1, h-1)
		calc(x, h-1, svgNormal{
			1.0 / 3, 1.0 / 2,
			int16(-tl + tr - 2*l + 2*r),
			int16(-tl - 2*t - tr + l + 2*c + r),
		})
	}
	for y := 1; y < h-1; y++ {
		lc := func(dy, dx int) int { return int(svgAlphaAt(srcData, w, h, dx, y+dy)) }
		// left_column_normal: top=(0,y-1) topRight=(1,y-1) center=(0,y)
		// right=(1,y) bottom=(0,y+1) bottomRight=(1,y+1), factors (1/2, 1/3).
		calc(0, y, svgNormal{
			1.0 / 2, 1.0 / 3,
			int16(-lc(-1, 0) + lc(-1, 1) - 2*lc(0, 0) + 2*lc(0, 1) - lc(1, 0) + lc(1, 1)),
			int16(-2*lc(-1, 0) - lc(-1, 1) + 2*lc(1, 0) + lc(1, 1)),
		})
		// right_column_normal: topLeft=(w-2,y-1) top=(w-1,y-1)
		// left=(w-2,y) center=(w-1,y) bottomLeft=(w-2,y+1) bottom=(w-1,y+1).
		rc := func(dy, dx int) int { return int(svgAlphaAt(srcData, w, h, w-1+dx, y+dy)) }
		tl, t := rc(-1, -1), rc(-1, 0)
		l, c := rc(0, -1), rc(0, 0)
		bl, b := rc(1, -1), rc(1, 0)
		calc(w-1, y, svgNormal{
			1.0 / 2, 1.0 / 3,
			int16(-tl + t - 2*l + 2*c - bl + b),
			int16(-tl - 2*t + bl + 2*b),
		})
	}
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			g := func(dx, dy int) int { return int(svgAlphaAt(srcData, w, h, x+dx, y+dy)) }
			tl, t, tr := g(-1, -1), g(0, -1), g(1, -1)
			l, r := g(-1, 0), g(1, 0)
			bl, b, br := g(-1, 1), g(0, 1), g(1, 1)
			calc(x, y, svgNormal{
				1.0 / 4, 1.0 / 4,
				int16(-tl + tr - 2*l + 2*r - bl + br),
				int16(-tl - 2*t - tr + bl + 2*b + br),
			})
		}
	}
}

// svgSRGBToLinear / svgLinearToSRGB are resvg's precomputed curve tables
// (filter/mod.rs verbatim, 256 entries each).

// svgSRGBToLinear converts one sRGB byte to LinearRGB (resvg filter/mod.rs table verbatim).
var svgSRGBToLinear = [256]uint8{
  0,   0,   0,   0,   0,   0,   0,   1,   1,   1,   1,   1,   1,   1,   1,   1,
  1,   1,   2,   2,   2,   2,   2,   2,   2,   2,   3,   3,   3,   3,   3,   3,
  4,   4,   4,   4,   4,   5,   5,   5,   5,   6,   6,   6,   6,   7,   7,   7,
  8,   8,   8,   8,   9,   9,   9,  10,  10,  10,  11,  11,  12,  12,  12,  13,
 13,  13,  14,  14,  15,  15,  16,  16,  17,  17,  17,  18,  18,  19,  19,  20,
 20,  21,  22,  22,  23,  23,  24,  24,  25,  25,  26,  27,  27,  28,  29,  29,
 30,  30,  31,  32,  32,  33,  34,  35,  35,  36,  37,  37,  38,  39,  40,  41,
 41,  42,  43,  44,  45,  45,  46,  47,  48,  49,  50,  51,  51,  52,  53,  54,
 55,  56,  57,  58,  59,  60,  61,  62,  63,  64,  65,  66,  67,  68,  69,  70,
 71,  72,  73,  74,  76,  77,  78,  79,  80,  81,  82,  84,  85,  86,  87,  88,
 90,  91,  92,  93,  95,  96,  97,  99, 100, 101, 103, 104, 105, 107, 108, 109,
111, 112, 114, 115, 116, 118, 119, 121, 122, 124, 125, 127, 128, 130, 131, 133,
134, 136, 138, 139, 141, 142, 144, 146, 147, 149, 151, 152, 154, 156, 157, 159,
161, 163, 164, 166, 168, 170, 171, 173, 175, 177, 179, 181, 183, 184, 186, 188,
190, 192, 194, 196, 198, 200, 202, 204, 206, 208, 210, 212, 214, 216, 218, 220,
222, 224, 226, 229, 231, 233, 235, 237, 239, 242, 244, 246, 248, 250, 253, 255,
}

// svgLinearToSRGB converts one LinearRGB byte back to sRGB (resvg verbatim).
var svgLinearToSRGB = [256]uint8{
  0,  13,  22,  28,  34,  38,  42,  46,  50,  53,  56,  59,  61,  64,  66,  69,
 71,  73,  75,  77,  79,  81,  83,  85,  86,  88,  90,  92,  93,  95,  96,  98,
 99, 101, 102, 104, 105, 106, 108, 109, 110, 112, 113, 114, 115, 117, 118, 119,
120, 121, 122, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136,
137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 148, 149, 150, 151,
152, 153, 154, 155, 155, 156, 157, 158, 159, 159, 160, 161, 162, 163, 163, 164,
165, 166, 167, 167, 168, 169, 170, 170, 171, 172, 173, 173, 174, 175, 175, 176,
177, 178, 178, 179, 180, 180, 181, 182, 182, 183, 184, 185, 185, 186, 187, 187,
188, 189, 189, 190, 190, 191, 192, 192, 193, 194, 194, 195, 196, 196, 197, 197,
198, 199, 199, 200, 200, 201, 202, 202, 203, 203, 204, 205, 205, 206, 206, 207,
208, 208, 209, 209, 210, 210, 211, 212, 212, 213, 213, 214, 214, 215, 215, 216,
216, 217, 218, 218, 219, 219, 220, 220, 221, 221, 222, 222, 223, 223, 224, 224,
225, 226, 226, 227, 227, 228, 228, 229, 229, 230, 230, 231, 231, 232, 232, 233,
233, 234, 234, 235, 235, 236, 236, 237, 237, 238, 238, 238, 239, 239, 240, 240,
241, 241, 242, 242, 243, 243, 244, 244, 245, 245, 246, 246, 246, 247, 247, 248,
248, 249, 249, 250, 250, 251, 251, 251, 252, 252, 253, 253, 254, 254, 255, 255,
}

// svgConvertColorSpace converts premul RGBA bytes between sRGB and LinearRGB
// (resvg into_srgb/into_linear_rgb: demultiply → curve LUT → premultiply,
// each step rounded like the Rust `as u8` truncation).
func svgConvertColorSpace(data []byte, toLinear bool) {
	lut := svgSRGBToLinear[:]
	if !toLinear {
		lut = svgLinearToSRGB[:]
	}
	n := len(data) / 4
	for i := 0; i < n; i++ {
		o := i * 4
		a := float32(data[o+3]) / 255
		if a == 0 {
			continue
		}
		// demultiply: (p/a + 0.5) as u8
		r := uint8(float32(data[o])/a + 0.5)
		g := uint8(float32(data[o+1])/a + 0.5)
		b := uint8(float32(data[o+2])/a + 0.5)
		// curve
		r, g, b = lut[r], lut[g], lut[b]
		// premultiply: (p*a + 0.5) as u8
		data[o] = uint8(float32(r)*a + 0.5)
		data[o+1] = uint8(float32(g)*a + 0.5)
		data[o+2] = uint8(float32(b)*a + 0.5)
	}
}

func maxU8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
