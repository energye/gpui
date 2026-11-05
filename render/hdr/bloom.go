//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package hdr

import (
	"errors"
	"math"

	"github.com/energye/gpui/render/internal/filter"
)

// Bloom domain caps. Sigma is capped where the CPU cost of the separable
// passes stops being reasonable (σ=16 is already 49 taps per direction);
// wider light belongs to a GPU blur, not this path. Intensity is capped
// looser than any engine-side gameplay ceiling: a caller may clamp tighter,
// never wider.
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
	// Sigma is the separable Gaussian standard deviation, in texels.
	Sigma float64
	// Intensity is the additive gain on the blurred bright pass.
	Intensity float64
}

// Validate reports a named error for a non-finite or out-of-range field.
func (p BloomParams) Validate() error {
	const op = "hdr.BloomParams.Validate"
	switch {
	case !finiteFloat(p.Threshold) || p.Threshold < 0 || p.Threshold > 1:
		return errors.New(op + ": threshold out of range")
	case !finiteFloat(p.Sigma) || p.Sigma < 0 || p.Sigma > MaxBloomSigma:
		return errors.New(op + ": sigma out of range")
	case !finiteFloat(p.Intensity) || p.Intensity < 0 || p.Intensity > MaxBloomIntensity:
		return errors.New(op + ": intensity out of range")
	}
	return nil
}

// IsIdentity reports whether Bloom would add nothing at all.
func (p BloomParams) IsIdentity() bool { return p.Intensity == 0 }

// Bloom returns a fresh target with this target's bright areas glowing:
// bright pass, separable Gaussian, additive back. The receiver is never
// mutated, alpha is carried through unchanged, and the summed light is not
// clamped — Encode's Reinhard rolloff is what decides how it lands in 8 bits.
// A nil receiver or an invalid BloomParams is an error with no output.
func (t *FloatTarget) Bloom(p BloomParams) (*FloatTarget, error) {
	const op = "hdr.FloatTarget.Bloom"
	if t == nil {
		return nil, errors.New(op + ": nil source")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	out, err := NewFloatTarget(t.w, t.h)
	if err != nil {
		return nil, err
	}
	if p.IsIdentity() {
		copy(out.pix, t.pix)
		return out, nil
	}
	filter.BloomFloat(out.pix, t.pix, t.w, t.h, p.Threshold, p.Sigma, p.Intensity)
	return out, nil
}

func finiteFloat(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
