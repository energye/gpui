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

import "sync"

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
