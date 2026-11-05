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

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/filter"
)

// Bloom is the render layer's float-domain bloom door. The parameter struct,
// its domain and its caps are defined once in the render package
// (render.BloomParams, render.MaxBloomSigma, render.MaxBloomIntensity) so this
// leg, the CPU 8-bit filter node and the GPU four-pass leg agree by
// construction — render cannot import this package (this one imports its
// filter internals), so the shared source has to sit upstream.

// Bloom returns a fresh target with this target's bright areas glowing:
// bright pass, separable Gaussian, additive back. The receiver is never
// mutated, alpha is carried through unchanged, and the summed light is not
// clamped — Encode's Reinhard rolloff is what decides how it lands in 8 bits.
// A nil receiver or an invalid BloomParams is an error with no output.
func (t *FloatTarget) Bloom(p render.BloomParams) (*FloatTarget, error) {
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
