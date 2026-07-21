package text

// ShapedGlyph represents a positioned glyph ready for GPU rendering.
// Unlike Glyph which contains CPU rasterization data (Mask), ShapedGlyph
// is minimal and designed for efficient GPU text rendering pipelines.
type ShapedGlyph struct {
	// GID is the glyph index in the font.
	GID GlyphID

	// Cluster is the source character index in the original text.
	// Used for hit testing and cursor positioning.
	Cluster int

	// IsCJK indicates that this glyph belongs to a CJK script (Han, Hiragana,
	// Katakana, Hangul). Used for script-aware rendering: CJK glyphs use
	// reduced hinting and bypass bucket quantization (ADR-027).
	IsCJK bool

	// X is the horizontal position relative to the text origin.
	X float64

	// Y is the vertical position relative to the baseline.
	Y float64

	// XAdvance is the horizontal advance to the next glyph.
	XAdvance float64

	// YAdvance is the vertical advance (for vertical text).
	YAdvance float64
}

// ReorderRTLShapedGlyphs reverses a logical-order shaped run into visual order
// for RTL scripts, recomputing X so the pen still advances left-to-right for
// drawing (origin at the left of the run). Cluster indices are preserved on
// each glyph (logical source indices).
//
// ENGINE_GAPS G1.c: pairs with BuiltinSegmenter bidi levels — layout places
// RTL segments after Shape, then reorders for paint/hit-test consistency.
func ReorderRTLShapedGlyphs(glyphs []ShapedGlyph) []ShapedGlyph {
	n := len(glyphs)
	if n <= 1 {
		return glyphs
	}
	out := make([]ShapedGlyph, n)
	x := 0.0
	for i := n - 1; i >= 0; i-- {
		g := glyphs[i]
		g.X = x
		out[n-1-i] = g
		x += g.XAdvance
	}
	return out
}

// shouldReorderRTL reports whether shaped glyphs should be reversed for
// visual RTL paint order. Only an explicit RTL direction reorders here;
// mixed paragraphs rely on BuiltinSegmenter + layout shapeSegments which
// reorders per RTL segment (avoids double-reverse with ShapeUncached).
func shouldReorderRTL(dir Direction, _ []rune) bool {
	return dir == DirectionRTL
}
