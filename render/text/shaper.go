package text

import "sync"

// Shaper converts text to positioned glyphs.
// Implementations provide different levels of text shaping support:
//   - OwnShaper: Pure Go shaper with GSUB/GPOS support (default, ADR-048)
//   - BuiltinShaper: Simple LTR shaper for Latin, Cyrillic, Greek, CJK (no GSUB/GPOS)
//   - BuiltinShaper: Simple LTR shaper for scripts without GSUB/GPOS (legacy)
type Shaper interface {
	// Shape converts text into positioned glyphs using the given face.
	// The font size is obtained from face.Size().
	// The returned ShapedGlyph slice is ready for GPU rendering.
	Shape(text string, face Face) []ShapedGlyph
}

// defaultShaper is initialized to OwnShaper in shaper_own.go init().
// This variable is set before any concurrent access (during init).
var defaultOwnShaper = NewOwnShaper()

var (
	shaperMu     sync.RWMutex
	globalShaper Shaper = defaultOwnShaper
)

// SetShaper sets the global shaper used by Shape().
// Pass nil to reset to the default OwnShaper (Pure Go GSUB/GPOS).
//
// Example usage with a custom shaper:
//
//	text.SetShaper(myCustomShaper)
//	defer text.SetShaper(nil) // Reset to default
func SetShaper(s Shaper) {
	shaperMu.Lock()
	defer shaperMu.Unlock()
	if s == nil {
		s = defaultOwnShaper
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
// S6.5: results are cached in the process-wide shape result cache when the face
// has a FontSource. Cached slices must not be modified by callers.
// Use ClearShapeResultCache / ShapeResultCacheStats for diagnostics.
func Shape(textStr string, face Face) []ShapedGlyph {
	if textStr == "" || face == nil {
		return nil
	}
	key, ok := faceShapeKey(face, textStr, shapeModeOT)
	if !ok {
		return GetShaper().Shape(textStr, face)
	}
	return globalShapeResultCache.getOrCreate(key, func() []ShapedGlyph {
		return GetShaper().Shape(textStr, face)
	})
}

// ShapeUncached always runs the global shaper without consulting the S6.5
// result cache. Useful for tests and one-shot offline work.
func ShapeUncached(textStr string, face Face) []ShapedGlyph {
	return GetShaper().Shape(textStr, face)
}
