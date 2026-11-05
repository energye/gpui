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

// Rec.709 luma weights — the one brightness definition this CPU pass uses and
// the WGSL leg of P3.7 batch 5 step 2 has to agree with.
const (
	bloomLumaR = 0.2126
	bloomLumaG = 0.7152
	bloomLumaB = 0.0722
)

// BloomFloat runs the bloom kernel over a linear float buffer of RGBA
// float32 texels (row-major, len == w*h*4): bright pass with the soft knee
// ramp, separable Gaussian with the shared CachedGaussianKernel (edge
// extension, same convention as BlurFilter), then the additive return. dst
// receives src's alpha unchanged; the glow only touches RGB.
//
// Nothing is clamped on output: values above 1 stay light until whatever
// tonemaps at the exit decides. src and dst must not alias.
func BloomFloat(dst, src []float32, w, h int, threshold, sigma, intensity float64) {
	n := w * h

	// Pass 1: bright extract into the pooled glow buffer. Luma comes from the
	// [0,1]-clamped color so the threshold domain stays meaningful; the ramp
	// reaches 1 at full white, and threshold 1 masks everything out. The ramp
	// itself runs in float64, which is the exact math the WGSL leg must match.
	glow := getTempBuffer(w, h)
	defer putTempBuffer(glow)
	denom := 1 - threshold
	for i := 0; i < n; i++ {
		j := i * 4
		r := float32(clampFloat01(float64(src[j])))
		g := float32(clampFloat01(float64(src[j+1])))
		b := float32(clampFloat01(float64(src[j+2])))
		lum := bloomLumaR*float64(r) + bloomLumaG*float64(g) + bloomLumaB*float64(b)
		m := float64(0)
		if lum > threshold && denom > 0 {
			m = clampFloat01((lum - threshold) / denom)
		}
		glow[j] = r * float32(m)
		glow[j+1] = g * float32(m)
		glow[j+2] = b * float32(m)
	}

	// Pass 2: separable Gaussian, horizontal into a second pooled buffer then
	// vertical back into glow. A sigma of 0 degenerates the kernel to one tap.
	kernel := CachedGaussianKernel(sigma)
	ks := len(kernel)
	half := ks / 2
	if half > 0 {
		tmp := getTempBuffer(w, h)
		defer putTempBuffer(tmp)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var r, g, b float32
				for k := 0; k < ks; k++ {
					sx := clampInt(x+k-half, 0, w-1)
					j := (y*w + sx) * 4
					wt := kernel[k]
					r += glow[j] * wt
					g += glow[j+1] * wt
					b += glow[j+2] * wt
				}
				j := (y*w + x) * 4
				tmp[j] = r
				tmp[j+1] = g
				tmp[j+2] = b
			}
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				var r, g, b float32
				for k := 0; k < ks; k++ {
					sy := clampInt(y+k-half, 0, h-1)
					j := (sy*w + x) * 4
					wt := kernel[k]
					r += tmp[j] * wt
					g += tmp[j+1] * wt
					b += tmp[j+2] * wt
				}
				j := (y*w + x) * 4
				glow[j] = r
				glow[j+1] = g
				glow[j+2] = b
			}
		}
	}

	// Pass 3: additive. The source keeps its own light (unclamped), alpha is
	// carried straight through.
	gain := float32(intensity)
	for i := 0; i < n; i++ {
		j := i * 4
		dst[j] = src[j] + glow[j]*gain
		dst[j+1] = src[j+1] + glow[j+1]*gain
		dst[j+2] = src[j+2] + glow[j+2]*gain
		dst[j+3] = src[j+3]
	}
}

func clampFloat01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
