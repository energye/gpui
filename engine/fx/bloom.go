//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package fx

import (
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/render/hdr"
)

// MaxImagePixels caps one grading intermediate (16M pixels, 64MB at RGBA8).
// Bigger pictures report core OutOfMemory instead of allocating.
const MaxImagePixels = 16 << 20

// MaxBloomSigma is the render layer's own CPU cap (σ=16 is already 49 taps
// per direction); the engine delegates the domain instead of re-inventing it.
const MaxBloomSigma = hdr.MaxBloomSigma

// MaxBloomIntensity caps the additive glow strength. The render layer allows
// more light than a game should ever ask for, so this stays the tighter cap.
const MaxBloomIntensity = 4

// Image is one isolated grading intermediate: W*H straight-alpha
// core.Colors in row-major order. It never aliases another Image:
// constructors copy, Apply returns fresh images, Clone copies again.
type Image struct {
	W, H int
	Pix  []core.Color
}

// NewImage builds a W-by-H transparent-black image. Non-positive sizes are
// InvalidArg; W*H beyond MaxImagePixels is OutOfMemory.
func NewImage(w, h int) (*Image, error) {
	const op = "fx.NewImage"
	if w <= 0 || h <= 0 {
		return nil, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return nil, core.OutOfMemory(op, "pixels")
	}
	return &Image{W: w, H: h, Pix: make([]core.Color, w*h)}, nil
}

// NewImageFromColors builds an image copying pix (row-major, len W*H).
// Bad sizes are InvalidArg, a length mismatch is BadData, over-budget is
// OutOfMemory. A NaN/Inf component is BadData, never a silent pixel.
func NewImageFromColors(w, h int, pix []core.Color) (*Image, error) {
	const op = "fx.NewImageFromColors"
	if w <= 0 || h <= 0 {
		return nil, core.InvalidArg(op, "size")
	}
	if int64(w)*int64(h) > int64(MaxImagePixels) {
		return nil, core.OutOfMemory(op, "pixels")
	}
	if len(pix) != w*h {
		return nil, core.BadData(op, "length")
	}
	for i, c := range pix {
		if !finiteColor(c) {
			return nil, core.BadData(op, "pixel")
		}
		_ = i
	}
	cp := make([]core.Color, len(pix))
	copy(cp, pix)
	return &Image{W: w, H: h, Pix: cp}, nil
}

// valid reports whether img is drawable (non-nil, positive, exact pixels).
func (img *Image) valid() bool {
	return img != nil && img.W > 0 && img.H > 0 && len(img.Pix) == img.W*img.H
}

// check validates img for Apply: nil is InvalidArg, corrupt is BadData.
func (img *Image) check(op string) error {
	if img == nil {
		return core.InvalidArg(op, "image")
	}
	if !img.valid() {
		return core.BadData(op, "image")
	}
	return nil
}

// At returns the pixel at (x, y), or false when out of range.
func (img *Image) At(x, y int) (core.Color, bool) {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return core.Color{}, false
	}
	return img.Pix[y*img.W+x], true
}

// Set stores c at (x, y). False on bad coordinates or a corrupt image;
// a non-finite c is rejected (false) and stores nothing.
func (img *Image) Set(x, y int, c core.Color) bool {
	if !img.valid() || x < 0 || y < 0 || x >= img.W || y >= img.H {
		return false
	}
	if !finiteColor(c) {
		return false
	}
	img.Pix[y*img.W+x] = c
	return true
}

// Clone returns a fresh copy, or nil for a nil/corrupt image (never panics).
func (img *Image) Clone() *Image {
	if !img.valid() {
		return nil
	}
	cp := make([]core.Color, len(img.Pix))
	copy(cp, img.Pix)
	return &Image{W: img.W, H: img.H, Pix: cp}
}

// ApproxEqual reports whether o has the same size and every component
// differs by less than eps. Non-positive eps means exact equality.
func (img *Image) ApproxEqual(o *Image, eps float64) bool {
	if !img.valid() || !o.valid() || img.W != o.W || img.H != o.H {
		return false
	}
	if eps <= 0 {
		return equalExact(img.Pix, o.Pix)
	}
	for i := range img.Pix {
		if !img.Pix[i].ApproxEqual(o.Pix[i], eps) {
			return false
		}
	}
	return true
}

func equalExact(a, b []core.Color) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// finiteFloat lives in warp.go: shared NaN/Inf guard.
func finiteColor(c core.Color) bool {
	return finiteFloat(c.R) && finiteFloat(c.G) && finiteFloat(c.B) && finiteFloat(c.A)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// Bloom glows bright areas: pixels above Threshold are masked with a soft
// knee, blurred by the render layer's separable Gaussian (shared kernel, so
// this is the same math the GPU leg runs), and added back scaled by
// Intensity. Sigma 0 leaves the bright pass unblurred (a hard halo on the
// masked pixels only); Intensity 0 is the identity, so callers can freeze one
// Grade and fade the glow with Intensity.
type Bloom struct {
	Threshold float64
	Sigma     float64
	Intensity float64
}

// NewBloom builds a Bloom. Threshold must be in [0,1], Sigma in
// [0,MaxBloomSigma], Intensity in [0,MaxBloomIntensity]; all floats must
// be finite. Anything else is InvalidArg and builds nothing.
func NewBloom(threshold, sigma, intensity float64) (Bloom, error) {
	b := Bloom{Threshold: threshold, Sigma: sigma, Intensity: intensity}
	if err := b.Validate(); err != nil {
		return Bloom{}, err
	}
	return b, nil
}

// Validate reports InvalidArg for non-finite or out-of-range fields.
func (b Bloom) Validate() error {
	const op = "fx.Bloom.Validate"
	if !finiteFloat(b.Threshold) || b.Threshold < 0 || b.Threshold > 1 {
		return core.InvalidArg(op, "threshold")
	}
	if !finiteFloat(b.Sigma) || b.Sigma < 0 || b.Sigma > MaxBloomSigma {
		return core.InvalidArg(op, "sigma")
	}
	if !finiteFloat(b.Intensity) || b.Intensity < 0 || b.Intensity > MaxBloomIntensity {
		return core.InvalidArg(op, "intensity")
	}
	return nil
}

// IsIdentity reports whether Apply adds (almost) nothing: zero intensity
// always; a zero Sigma only blurs nothing, the mask can still light up, so it
// is not an identity.
func (b Bloom) IsIdentity() bool { return b.Intensity == 0 }

// Apply returns src plus the blurred bright-pass, as a fresh image. The
// pixel math is the render layer's float-domain bloom kernel (P3.7 batch 5:
// one implementation for both legs), so an over-bright glow keeps its light
// through the passes; this stage still hands back LDR colors, clamping its
// output to [0,1] the way every other fx stage does. Colors cross the
// render layer's float32 store (1e-7 relative), alpha never enters the
// kernel and comes back exactly as it went in.
//
// src is never mutated. A nil src is InvalidArg, a corrupt src is BadData,
// an invalid Bloom is InvalidArg, an image wider than the render layer's
// float target cap is InvalidArg; all leave nothing allocated to the caller.
func (b Bloom) Apply(src *Image) (*Image, error) {
	const op = "fx.Bloom.Apply"
	if err := src.check(op); err != nil {
		return nil, err
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	n := len(src.Pix)
	out := make([]core.Color, n)
	if b.IsIdentity() {
		copy(out, src.Pix)
		return &Image{W: src.W, H: src.H, Pix: out}, nil
	}
	if src.W > hdr.MaxFloatDimension || src.H > hdr.MaxFloatDimension {
		return nil, core.InvalidArg(op, "image side over float target cap")
	}
	target, err := hdr.NewFloatTarget(src.W, src.H)
	if err != nil {
		return nil, core.OutOfMemory(op, "bloom target")
	}
	texels := target.FloatPixels()
	for i, c := range src.Pix {
		j := i * 4
		texels[j] = float32(c.R)
		texels[j+1] = float32(c.G)
		texels[j+2] = float32(c.B)
		texels[j+3] = float32(c.A)
	}
	glowed, err := target.Bloom(hdr.BloomParams{
		Threshold: b.Threshold, Sigma: b.Sigma, Intensity: b.Intensity,
	})
	if err != nil {
		return nil, core.OutOfMemory(op, "bloom target")
	}
	lit := glowed.FloatPixels()
	for i, c := range src.Pix {
		j := i * 4
		out[i] = core.RGBA(clamp01(float64(lit[j])), clamp01(float64(lit[j+1])),
			clamp01(float64(lit[j+2])), c.A)
	}
	return &Image{W: src.W, H: src.H, Pix: out}, nil
}
