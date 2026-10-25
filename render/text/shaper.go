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
	"sync"
	"sync/atomic"
)

// Shaper converts text to positioned glyphs.
// Implementations provide different levels of text shaping support:
//   - HbShaper: HarfBuzz Go port with full GSUB/GPOS shaping (default)
//   - OwnShaper: Pure Go shaper with GSUB/GPOS support
//   - BuiltinShaper: Simple LTR shaper for Latin, Cyrillic, Greek, CJK (no GSUB/GPOS)
type Shaper interface {
	// Shape converts text into positioned glyphs using the given face.
	// The font size is obtained from face.Size().
	// The returned ShapedGlyph slice is ready for GPU rendering.
	Shape(text string, face Face) []ShapedGlyph
}

// This variable is set before any concurrent access (during init).
var defaultShaper = NewHbShaper()

var (
	shaperMu     sync.RWMutex
	globalShaper Shaper = defaultShaper
)

// SetShaper sets the global shaper used by Shape().
// Pass nil to reset to the default HbShaper (HarfBuzz Go port).
//
// Example usage with a custom shaper:
//
//	text.SetShaper(myCustomShaper)
//	defer text.SetShaper(nil) // Reset to default
func SetShaper(s Shaper) {
	shaperMu.Lock()
	defer shaperMu.Unlock()
	if s == nil {
		s = defaultShaper
	}
	globalShaper = s
}

// GetShaper returns the current global shaper.
func GetShaper() Shaper {
	shaperMu.RLock()
	defer shaperMu.RUnlock()
	return globalShaper
}

// Shape is a convenience function that uses the global shaper.
// It converts text to positioned glyphs using the given face.
// The font size is obtained from face.Size().
//
// Cached slices must not be modified by callers.
// Use ClearShapeResultCache / ShapeResultCacheStats for diagnostics.
func Shape(textStr string, face Face) []ShapedGlyph {
	if textStr == "" || face == nil {
		return nil
	}
	if IsHighChurnLabel(textStr) {
		return GetShaper().Shape(textStr, face)
	}
	key, ok := faceShapeKey(face, textStr, shapeModeOT)
	if !ok {
		return GetShaper().Shape(textStr, face)
	}
	return globalShapeResultCache.getOrCreate(key, func() []ShapedGlyph {
		return GetShaper().Shape(textStr, face)
	})
}

// Useful for tests and one-shot offline work.
func ShapeUncached(textStr string, face Face) []ShapedGlyph {
	return GetShaper().Shape(textStr, face)
}

// shapeForDrawFallbacks counts ShapeForDraw calls that fell back to the
// cmap+hmtx iteration (no HarfBuzz build / shaper returned nothing). The
// fallback is explicit and observable, never silent.
var shapeForDrawFallbacks atomic.Uint64

// ShapeForDrawFallbacks reports how many ShapeForDraw calls used the cmap
// fallback (observability for the A1 fix).
func ShapeForDrawFallbacks() uint64 { return shapeForDrawFallbacks.Load() }

// ShapeForDraw shapes s for direct-draw consumers (the render layer's
// default string path). Skia SkShaper posture: shape first, then draw.
//   - MultiFace inputs split into per-face runs first (Source() is nil, the
//     shaper cannot take a MultiFace wholesale) — the same rule layout uses.
//   - Falls back to the cmap+hmtx iteration (LayoutGlyphs) when shaping
//     yields nothing, and counts it via ShapeForDrawFallbacks.
func ShapeForDraw(s string, face Face) []ShapedGlyph {
	if face == nil || s == "" {
		return nil
	}
	if mf, ok := face.(*MultiFace); ok {
		frs := mf.Runs(s)
		if len(frs) == 0 {
			shapeForDrawFallbacks.Add(1)
			return LayoutGlyphs(face, s)
		}
		var out []ShapedGlyph
		xOff := 0.0
		for _, fr := range frs {
			if fr.Face == nil || fr.Text == "" {
				continue
			}
			gs := Shape(fr.Text, fr.Face)
			if len(gs) == 0 {
				shapeForDrawFallbacks.Add(1)
				gs = LayoutGlyphs(fr.Face, fr.Text)
			}
			if len(gs) == 0 {
				continue
			}
			for i := range gs {
				gs[i].X += xOff
			}
			xOff = gs[len(gs)-1].X + gs[len(gs)-1].XAdvance
			out = append(out, gs...)
		}
		return out
	}
	gs := Shape(s, face)
	if len(gs) == 0 {
		shapeForDrawFallbacks.Add(1)
		return LayoutGlyphs(face, s)
	}
	return gs
}
