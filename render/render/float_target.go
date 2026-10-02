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
	"errors"
	"image"
	"math"

	"github.com/energye/gpui/gpu/types"
)

// Float budgets. Over-area targets report an error at construction;
// the 8-bit PixmapTarget path never sees this file.
const (
	// MaxFloatDimension caps one side of a float target.
	MaxFloatDimension = 8192
	// MaxFloatPixels caps the total texel count.
	MaxFloatPixels = 8192 * 8192
)

// FloatTarget is a CPU-backed linear HDR render target (S66, 9.1 thaw).
// Each texel is 4 float32s (R,G,B,A); values above 1.0 are legal light,
// not clamped storage. It implements RenderTarget with the RGBA16Float
// format tag, but Pixels returns nil on purpose: the 8-bit software
// renderer refuses it, so the old main road cannot wander in by mistake.
// Only new S66 code paths touch FloatPixels and Encode.
type FloatTarget struct {
	w, h int
	pix  []float32
}

// NewFloatTarget builds a W-by-H float target cleared to transparent
// black. Non-positive sizes or over-budget areas are errors, never a
// half-built target.
func NewFloatTarget(w, h int) (*FloatTarget, error) {
	const op = "render.NewFloatTarget"
	if w <= 0 || h <= 0 || w > MaxFloatDimension || h > MaxFloatDimension {
		return nil, errors.New(op + ": bad size")
	}
	if int64(w)*int64(h) > int64(MaxFloatPixels) {
		return nil, errors.New(op + ": over budget")
	}
	return &FloatTarget{w: w, h: h, pix: make([]float32, w*h*4)}, nil
}

// Width returns the target width in texels.
func (t *FloatTarget) Width() int { return t.w }

// Height returns the target height in texels.
func (t *FloatTarget) Height() int { return t.h }

// Format tags the target RGBA16Float: new-branch only, 8-bit code
// keys off RGBA8Unorm/BGRA8Unorm and never matches this.
func (t *FloatTarget) Format() types.TextureFormat {
	return types.TextureFormatRGBA16Float
}

// TextureView returns nil: CPU-side staging only in this thaw.
func (t *FloatTarget) TextureView() TextureView { return nil }

// Pixels returns nil: no 8-bit backing, the software renderer declines.
func (t *FloatTarget) Pixels() []byte { return nil }

// Stride returns the bytes per row (width times 16).
func (t *FloatTarget) Stride() int { return t.w * 16 }

// FloatPixels exposes the linear HDR texels in RGBA order for S66 paths.
func (t *FloatTarget) FloatPixels() []float32 { return t.pix }

// At reads one texel. Out of range returns zeros, never a panic.
func (t *FloatTarget) At(x, y int) [4]float32 {
	if x < 0 || y < 0 || x >= t.w || y >= t.h {
		return [4]float32{}
	}
	i := (y*t.w + x) * 4
	return [4]float32{t.pix[i], t.pix[i+1], t.pix[i+2], t.pix[i+3]}
}

// Set writes one texel. Non-finite components are rejected (false) so
// NaN can never sneak into the HDR store; out of range is also false.
func (t *FloatTarget) Set(x, y int, c [4]float32) bool {
	if x < 0 || y < 0 || x >= t.w || y >= t.h {
		return false
	}
	for _, v := range c {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	i := (y*t.w + x) * 4
	copy(t.pix[i:i+4], c[:])
	return true
}

// ClearFloat fills the target with linear color c (alpha as given).
// Non-finite components leave the target untouched (false).
func (t *FloatTarget) ClearFloat(c [4]float32) bool {
	for _, v := range c {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return false
		}
	}
	for i := 0; i < len(t.pix); i += 4 {
		copy(t.pix[i:i+4], c[:])
	}
	return true
}

// Encode writes the target out as 8-bit sRGB: exposure first, then the
// frozen Reinhard rolloff x/(1+x), then the sRGB curve. Exposure must be
// in (0,MaxEncodeExposure]; anything else is an error, never a guess.
// This is the screenshot-vs-design compare path: HDR explains itself,
// 8-bit stays the comparison language.
func (t *FloatTarget) Encode(exposure float64) (*image.RGBA, error) {
	const op = "render.FloatTarget.Encode"
	if math.IsNaN(exposure) || math.IsInf(exposure, 0) || exposure <= 0 || exposure > MaxEncodeExposure {
		return nil, errors.New(op + ": bad exposure")
	}
	out := image.NewRGBA(image.Rect(0, 0, t.w, t.h))
	for y := 0; y < t.h; y++ {
		for x := 0; x < t.w; x++ {
			i := (y*t.w + x) * 4
			r := srgbEncode(reinhard(float64(t.pix[i]) * exposure))
			g := srgbEncode(reinhard(float64(t.pix[i+1]) * exposure))
			b := srgbEncode(reinhard(float64(t.pix[i+2]) * exposure))
			a := uint8(clamp01f(t.pix[i+3])*255 + 0.5)
			oi := y*out.Stride + x*4
			out.Pix[oi], out.Pix[oi+1], out.Pix[oi+2], out.Pix[oi+3] = r, g, b, a
		}
	}
	return out, nil
}

// MaxEncodeExposure caps Encode exposure (16x matches fx.MaxExposure).
const MaxEncodeExposure = 16

// reinhard is the frozen S66 rolloff x/(1+x), same math as fx Reinhard.
func reinhard(x float64) float64 {
	if x < 0 {
		return 0
	}
	return x / (1 + x)
}

// srgbEncode maps linear 0..1 to sRGB 8-bit steps.
func srgbEncode(x float64) uint8 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 255
	}
	var s float64
	if x <= 0.0031308 {
		s = 12.92 * x
	} else {
		s = 1.055*math.Pow(x, 1/2.4) - 0.055
	}
	return uint8(s*255 + 0.5)
}

func clamp01f(x float32) float32 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
