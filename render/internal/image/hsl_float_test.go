package image

import (
	"math"
	"testing"
)

// Pin (O13 发现第八件): the four non-separable HSL blends must compute the
// whole W3C chain in float — per-stage u8 rounding accumulated ~9/255 of
// drift against the reference. Probes use known inputs with exact-ish
// expectations (±1 quantization).
func TestHSLBlend_FloatDomain(t *testing.T) {
	// src salmon (230,89,153), dst mid-gray-green (128,140,115)
	sr, sg, sb := uint8(230), uint8(89), uint8(153)
	dr, dg, db := uint8(128), uint8(140), uint8(115)

	// Independent W3C reference (float, computed inline).
	lum := func(r, g, b float64) float64 { return 0.3*r + 0.59*g + 0.11*b }
	sat := func(r, g, b float64) float64 {
		return math.Max(r, math.Max(g, b)) - math.Min(r, math.Min(g, b))
	}
	clip := func(r, g, b float64) (float64, float64, float64) {
		l := 0.3*r + 0.59*g + 0.11*b
		n := math.Min(r, math.Min(g, b))
		x := math.Max(r, math.Max(g, b))
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
		return r, g, b
	}
	setLumRef := func(r, g, b, l float64) (uint8, uint8, uint8) {
		d := l - lum(r, g, b)
		r, g, b = clip(r+d, g+d, b+d)
		return uint8(r + 0.5), uint8(g + 0.5), uint8(b + 0.5)
	}
	setSatRef := func(r, g, b, s float64) (float64, float64, float64) {
		vals := []float64{r, g, b}
		idx := []int{0, 1, 2}
		if vals[idx[0]] > vals[idx[1]] {
			idx[0], idx[1] = idx[1], idx[0]
		}
		if vals[idx[1]] > vals[idx[2]] {
			idx[1], idx[2] = idx[2], idx[1]
		}
		if vals[idx[0]] > vals[idx[1]] {
			idx[0], idx[1] = idx[1], idx[0]
		}
		out := []float64{r, g, b}
		if vals[idx[2]] > vals[idx[0]] {
			out[idx[1]] = (vals[idx[1]] - vals[idx[0]]) * s / (vals[idx[2]] - vals[idx[0]])
			out[idx[2]] = s
		} else {
			out[idx[1]] = 0
			out[idx[2]] = 0
		}
		out[idx[0]] = 0
		return out[0], out[1], out[2]
	}

	near := func(name string, got, want uint8, i int) {
		if math.Abs(float64(got)-float64(want)) > 1 {
			t.Errorf("%s[%d]: got=%d want=%d (float-domain drift >1)", name, i, got, want)
		}
	}

	// Color: hue+sat of src, lum of dst.
	cR, cG, cB := blendColor(sr, sg, sb, dr, dg, db)
	wR, wG, wB := setLumRef(float64(sr), float64(sg), float64(sb), lum(float64(dr), float64(dg), float64(db)))
	near("Color.R", cR, wR, 0)
	near("Color.G", cG, wG, 1)
	near("Color.B", cB, wB, 2)

	// Luminosity: lum of src, hue+sat of dst.
	lR, lG, lB := blendLuminosity(sr, sg, sb, dr, dg, db)
	w2R, w2G, w2B := setLumRef(float64(dr), float64(dg), float64(db), lum(float64(sr), float64(sg), float64(sb)))
	near("Lum.R", lR, w2R, 0)
	near("Lum.G", lG, w2G, 1)
	near("Lum.B", lB, w2B, 2)

	// Hue: sat(dst) applied to src hue-set, then lum(dst).
	r1, g1, b1 := setSatRef(float64(sr), float64(sg), float64(sb), sat(float64(dr), float64(dg), float64(db)))
	hR, hG, hB := blendHue(sr, sg, sb, dr, dg, db)
	w3R, w3G, w3B := setLumRef(r1, g1, b1, lum(float64(dr), float64(dg), float64(db)))
	near("Hue.R", hR, w3R, 0)
	near("Hue.G", hG, w3G, 1)
	near("Hue.B", hB, w3B, 2)

	// Saturation: sat(src) applied to dst colors, then lum(dst).
	r2, g2, b2 := setSatRef(float64(dr), float64(dg), float64(db), sat(float64(sr), float64(sg), float64(sb)))
	sR, sG, sB := blendSaturation(sr, sg, sb, dr, dg, db)
	w4R, w4G, w4B := setLumRef(r2, g2, b2, lum(float64(dr), float64(dg), float64(db)))
	near("Sat.R", sR, w4R, 0)
	near("Sat.G", sG, w4G, 1)
	near("Sat.B", sB, w4B, 2)
}
