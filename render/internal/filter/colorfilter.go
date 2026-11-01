//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

// P3.4 batch 4: the ColorFilter abstraction (SkColorFilters family). A
// color filter maps a straight-alpha RGBA color to another; composition is
// function composition. The premul storage invariant is preserved by the
// pixmap wrapper in Context.ApplyColorFilter (unpremul → filter → premul).

package filter

import (
	intBlend "github.com/energye/gpui/render/internal/blend"

	"github.com/energye/gpui/render"
)

// BlendColorFilter blends the color toward a fixed color in the given blend
// mode (SkColorFilters::Blend). The blend math runs on straight colors with
// the fixed color as source and the pixel as destination.
type BlendColorFilter struct {
	color [4]float32
	mode  intBlend.BlendMode
}

// NewBlendColorFilter builds SkColorFilters::Blend.
func NewBlendColorFilter(color render.RGBA, mode intBlend.BlendMode) *BlendColorFilter {
	return &BlendColorFilter{color: [4]float32{float32(color.R), float32(color.G), float32(color.B), float32(color.A)}, mode: mode}
}

// Filter applies the blend.
func (f *BlendColorFilter) Filter(r, g, b, a float32) (float32, float32, float32, float32) {
	return blendStraight(f.color, [4]float32{r, g, b, a}, f.mode)
}

// blendStraight evaluates premul-style blend factors on straight colors and
// re-derives straight output (src over dst per the mode's factor pair from
// the internal blend tables).
func blendStraight(src, dst [4]float32, mode intBlend.BlendMode) (float32, float32, float32, float32) {
	// Delegate to the shared blend core via premul u8 round trip: the mode
	// table lives in internal/blend and is the single source of truth.
	sp := [4]uint8{u8c(src[0] * src[3]), u8c(src[1] * src[3]), u8c(src[2] * src[3]), u8c(src[3])}
	dp := [4]uint8{u8c(dst[0] * dst[3]), u8c(dst[1] * dst[3]), u8c(dst[2] * dst[3]), u8c(dst[3])}
	fn := blendFunc(mode)
	rr, rg, rb, ra := fn(sp[0], sp[1], sp[2], sp[3], dp[0], dp[1], dp[2], dp[3])
	out := [4]float32{float32(rr) / 255, float32(rg) / 255, float32(rb) / 255, float32(ra) / 255}
	if out[3] <= 0 {
		return 0, 0, 0, 0
	}
	return out[0] / out[3], out[1] / out[3], out[2] / out[3], out[3]
}

// LightingColorFilter multiplies each channel by a multiplier and adds an
// addend (SkColorFilters::Lighting): out = clamp(c*mul + add).
type LightingColorFilter struct {
	mul colorMul
	add [4]float32
}

type colorMul struct{ r, g, b float32 }

// NewLightingColorFilter builds SkColorFilters::Lighting (multiplier and
// addend as straight colors; alpha passes through).
func NewLightingColorFilter(multiplier, addend render.RGBA) *LightingColorFilter {
	return &LightingColorFilter{
		mul: colorMul{float32(multiplier.R), float32(multiplier.G), float32(multiplier.B)},
		add: [4]float32{float32(addend.R), float32(addend.G), float32(addend.B), float32(addend.A)},
	}
}

// Filter applies per-channel multiply-add (alpha unchanged).
func (f *LightingColorFilter) Filter(r, g, b, a float32) (float32, float32, float32, float32) {
	clamp01 := func(v float32) float32 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	return clamp01(r*f.mul.r + f.add[0]), clamp01(g*f.mul.g + f.add[1]), clamp01(b*f.mul.b + f.add[2]), a
}

// TableColorFilter maps each channel through a 256-entry LUT
// (SkColorFilters::Table). A nil or short table leaves that channel
// unchanged.
type TableColorFilter struct {
	rTab, gTab, bTab, aTab []uint8
}

// NewTableColorFilter builds SkColorFilters::Table (tables indexed by the
// straight 0-255 channel value).
func NewTableColorFilter(rTab, gTab, bTab, aTab []uint8) *TableColorFilter {
	return &TableColorFilter{rTab: rTab, gTab: gTab, bTab: bTab, aTab: aTab}
}

// Filter looks each channel up in its LUT.
func (f *TableColorFilter) Filter(r, g, b, a float32) (float32, float32, float32, float32) {
	lut := func(tab []uint8, v float32) float32 {
		if len(tab) != 256 {
			return v
		}
		return float32(tab[int(v*255+0.5)]) / 255.0
	}
	return lut(f.rTab, r), lut(f.gTab, g), lut(f.bTab, b), lut(f.aTab, a)
}

// composeColorFilter chains two filters: outer(inner(color)).
type composeColorFilter struct {
	outer, inner render.ColorFilter
}

// ComposeColorFilter builds SkColorFilters::Compose (outer ∘ inner).
func ComposeColorFilter(outer, inner render.ColorFilter) render.ColorFilter {
	return &composeColorFilter{outer: outer, inner: inner}
}

// Filter evaluates inner first, then outer.
func (f *composeColorFilter) Filter(r, g, b, a float32) (float32, float32, float32, float32) {
	r2, g2, b2, a2 := f.inner.Filter(r, g, b, a)
	return f.outer.Filter(r2, g2, b2, a2)
}

// blendFunc resolves the shared premul blend core for the mode (the single
// source of truth lives in internal/blend).
func blendFunc(mode intBlend.BlendMode) func(sr, sg, sb, sa, dr, dg, db, da byte) (byte, byte, byte, byte) {
	return intBlend.GetBlendFunc(mode)
}
