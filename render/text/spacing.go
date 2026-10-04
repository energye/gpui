//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import (
	"math"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// Letter/word spacing semantics (Flutter TextStyle + CSS text-spacing).
//
// Spacing math delegates to the vendored shaping library
// (third_party/go-text/typesetting/shaping: Output.AddWordSpacing,
// Output.AddLetterSpacing, AddSpacing) — the single source for cluster
// grouping, half-split distribution and run-boundary rules. This file only
// bridges between ShapedGlyph pen positions and shaping.Output advances.
//
//   - LetterSpacing adds space between glyph clusters (n-1 gaps over a
//     single run: no trailing phantom, so single characters measure exactly
//     their advance and centered text stays centered). Combining marks share
//     their base cluster and never separate from it.
//   - WordSpacing adds space per word separator (the shaping library's CSS
//     word-separator set: space, no-break space, Ethiopic/Aegean/Ugaritic/
//     Phoenician separators). Tab is not a separator: it keeps its
//     tab-stop advance with no word gap.
//
// Zero spacing is a fast path: glyphs untouched, extra 0.

// pxToFixed quantizes logical pixels to 26.6 fixed (1/64px units).
func pxToFixed(px float64) fixed.Int26_6 {
	return fixed.Int26_6(math.Round(px * 64))
}

// fixedToPx converts 26.6 fixed back to logical pixels.
func fixedToPx(v fixed.Int26_6) float64 {
	return float64(v) / 64
}

// toLibraryDirection maps the text direction to the shaping library
// direction (only the vertical axis matters for spacing math).
func toLibraryDirection(d Direction) di.Direction {
	switch d {
	case DirectionRTL:
		return di.DirectionRTL
	case DirectionTTB:
		return di.DirectionTTB
	case DirectionBTT:
		return di.DirectionBTT
	default:
		return di.DirectionLTR
	}
}

// isWordSeparator reports the CSS word separators honored by the shaping
// library's AddWordSpacing (mirrors its separator set so wrap measurement
// and layout widths count identically).
func isWordSeparator(r rune) bool {
	switch r {
	case '\u0020', // space
		'\u00A0',                   // no-break space
		'\u1361',                   // Ethiopic word space
		'\U00010100', '\U00010101', // Aegean word separators
		'\U0001039F', // Ugaritic word divider
		'\U0001091F': // Phoenician word separator
		return true
	default:
		return false
	}
}

// countWordSeparators counts word-separator runes in text.
func countWordSeparators(text string) int {
	n := 0
	for _, r := range text {
		if isWordSeparator(r) {
			n++
		}
	}
	return n
}

// ApplySpacing shifts shaped glyphs for letter/word spacing in place and
// returns the total width addition (for run Advance bumps). Glyph X positions
// must already include their base offsets; spacing deltas add on top.
// Horizontal text: shifts X. Vertical text is not shifted here (layout calls
// ApplySpacingDir); this entry preserves the horizontal caret/paint stream.
func ApplySpacing(glyphs []ShapedGlyph, text string, letterSpacing, wordSpacing float64) float64 {
	return applySpacingWithDir(glyphs, []rune(text), letterSpacing, wordSpacing, false)
}

// ApplySpacingDir is ApplySpacing with an explicit vertical axis: vertical
// text shifts Y/YAdvance, horizontal shifts X/XAdvance. Layout passes the
// segment direction; single-line UI callers use ApplySpacing (horizontal).
func ApplySpacingDir(glyphs []ShapedGlyph, text string, letterSpacing, wordSpacing float64, dir Direction) float64 {
	return applySpacingWithDir(glyphs, []rune(text), letterSpacing, wordSpacing, dir.IsVertical())
}

// applySpacingWithDir bridges ShapedGlyph pen positions to the shaping
// library: glyphs become one shaping run, AddWordSpacing/AddLetterSpacing do
// the cluster math, and pen+offset positions flow back. Per-glyph content
// offsets (marks, kerning) are preserved: only spacing deltas move glyphs.
func applySpacingWithDir(glyphs []ShapedGlyph, runes []rune, letterSpacing, wordSpacing float64, vertical bool) float64 {
	if len(glyphs) == 0 || (letterSpacing == 0 && wordSpacing == 0) {
		return 0
	}
	n := len(runes)
	lib := make([]shaping.Glyph, len(glyphs))
	// Group consecutive equal clusters for GlyphCount (ligature/mark glue).
	counts := make([]int, len(glyphs))
	for i := range glyphs {
		c := glyphs[i].Cluster
		if c < 0 {
			c = 0
		}
		if n > 0 && c >= n {
			c = n - 1
		}
		if i == 0 || c != libClamp(glyphs[i-1].Cluster, n) {
			j := i
			for j < len(glyphs) && libClamp(glyphs[j].Cluster, n) == c {
				j++
			}
			for k := i; k < j; k++ {
				counts[k] = j - i
			}
		}
	}
	var libDir di.Direction
	if vertical {
		libDir = di.DirectionTTB
	} else {
		libDir = di.DirectionLTR
	}
	oldAdv := make([]fixed.Int26_6, len(glyphs))
	oldOff := make([]fixed.Int26_6, len(glyphs))
	pen := 0.0
	for i := range glyphs {
		c := libClamp(glyphs[i].Cluster, n)
		var adv, pos float64
		if vertical {
			adv = glyphs[i].YAdvance
			pos = glyphs[i].Y
		} else {
			adv = glyphs[i].XAdvance
			pos = glyphs[i].X
		}
		off := pos - pen
		pen += adv
		oldAdv[i] = pxToFixed(adv)
		oldOff[i] = pxToFixed(off)
		g := shaping.Glyph{
			Advance:      oldAdv[i],
			ClusterIndex: c,
			RuneCount:    1,
			GlyphCount:   counts[i],
		}
		if vertical {
			g.YAdvance = oldAdv[i]
			g.YOffset = oldOff[i]
		} else {
			g.XAdvance = oldAdv[i]
			g.XOffset = oldOff[i]
		}
		lib[i] = g
	}
	out := shaping.Output{Glyphs: lib, Direction: libDir}
	if wordSpacing != 0 && n > 0 {
		out.AddWordSpacing(runes, pxToFixed(wordSpacing))
	}
	if letterSpacing != 0 {
		out.AddLetterSpacing(pxToFixed(letterSpacing), true, true)
	}
	// Write back deltas only: base positions stay float-exact, so spaced
	// widths differ from unspaced by exactly the spacing bumps (no 1/64
	// requantization of shaped advances). Extra is the summed delta, exact
	// for exact-valued spacings.
	var dPen, extra float64
	for i := range glyphs {
		g := &out.Glyphs[i]
		var dAdv, dOff float64
		if vertical {
			dAdv = fixedToPx(g.YAdvance - oldAdv[i])
			dOff = fixedToPx(g.YOffset - oldOff[i])
			glyphs[i].YAdvance += dAdv
			glyphs[i].Y += dPen + dOff
		} else {
			dAdv = fixedToPx(g.XAdvance - oldAdv[i])
			dOff = fixedToPx(g.XOffset - oldOff[i])
			glyphs[i].XAdvance += dAdv
			glyphs[i].X += dPen + dOff
		}
		dPen += dAdv
		extra += dAdv
	}
	return extra
}

// libClamp clamps a cluster index into rune range (never panics on foreign
// shaper output, e.g. byte-based clusters on multibyte text).
func libClamp(c, n int) int {
	if c < 0 {
		return 0
	}
	if n > 0 && c >= n {
		return n - 1
	}
	return c
}

// SpacingExtra reports the width addition spacing produces for a shaped run
// with distinctClusters distinct clusters over text: letter gaps (clusters-1)
// plus one word gap per word separator (shaping library CSS set).
func SpacingExtra(distinctClusters int, text string, letterSpacing, wordSpacing float64) float64 {
	if letterSpacing == 0 && wordSpacing == 0 {
		return 0
	}
	return letterSpacing*float64(max(distinctClusters-1, 0)) +
		wordSpacing*float64(countWordSeparators(text))
}
