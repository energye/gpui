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
	"math"
)

// Bloom domain caps, defined once here because every leg consumes them:
// the CPU 8-bit filter node, the GPU four-pass graph, and the float-domain
// entry in render/hdr (which imports this package, never the other way).
// Sigma is capped where the separable CPU passes stop being reasonable
// (σ=16 is already 49 taps per direction); wider light belongs to a GPU blur.
// Intensity is capped looser than any engine-side gameplay ceiling — a caller
// may clamp tighter, never wider.
const (
	MaxBloomSigma     = 16
	MaxBloomIntensity = 8
)

// BloomParams is one bloom request. The zero value is legal and paints
// nothing (Intensity 0 is the identity), so a caller can freeze one set of
// parameters and fade the glow in.
type BloomParams struct {
	// Threshold is where the soft knee starts in [0,1] luma: at or below it
	// nothing glows, and the ramp reaches full at white. 1 masks everything.
	Threshold float64
	// Sigma is the separable Gaussian standard deviation, in texels. It is a
	// spread, not a mip level count — this leg never downsamples.
	Sigma float64
	// Intensity is the additive gain on the blurred bright pass.
	Intensity float64
}

// Validate reports a named error for a non-finite or out-of-range field.
func (p BloomParams) Validate() error {
	const op = "render.BloomParams.Validate"
	switch {
	case !bloomFinite(p.Threshold) || p.Threshold < 0 || p.Threshold > 1:
		return errors.New(op + ": threshold out of range")
	case !bloomFinite(p.Sigma) || p.Sigma < 0 || p.Sigma > MaxBloomSigma:
		return errors.New(op + ": sigma out of range")
	case !bloomFinite(p.Intensity) || p.Intensity < 0 || p.Intensity > MaxBloomIntensity:
		return errors.New(op + ": intensity out of range")
	}
	return nil
}

// IsIdentity reports whether a bloom would add nothing at all.
func (p BloomParams) IsIdentity() bool { return p.Intensity == 0 }

// bloomNodeRunnable reports whether a node carries work: identity nodes must
// never occupy a pass, and out-of-domain parameters must never be guessed
// into one.
func bloomNodeRunnable(p BloomParams) bool {
	return p.Validate() == nil && !p.IsIdentity()
}

func bloomFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
