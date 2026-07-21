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

// ---------------------------------------------------------------------------
// Hit-test helpers (ENGINE_GAPS G1.c UAX#9 / cluster mapping)
//
// Glyphs must be in *visual paint order* with monotonic non-decreasing X
// (post ReorderRTLShapedGlyphs or LTR shape). Cluster on each glyph is the
// *logical* source index and is preserved across visual reordering.
// ---------------------------------------------------------------------------

// HitTestCluster returns the logical source cluster (character index) for a
// horizontal hit at x (same coordinate space as glyph.X / XAdvance).
//
// Rules (single visual run):
//   - x before the first glyph → first glyph's Cluster
//   - x in [g.X, g.X+g.XAdvance) → that glyph's Cluster
//   - x past the last glyph → last glyph's Cluster (clamped; callers that need
//     "after last" caret can use HitTestClusterEdge)
//
// Empty run returns 0.
func HitTestCluster(glyphs []ShapedGlyph, x float64) int {
	c, _ := HitTestClusterEdge(glyphs, x)
	return c
}

// HitTestClusterEdge is like HitTestCluster but also reports trailing edge:
// trailing is true when the hit is in the right half of the glyph's advance
// (caret should sit after this cluster for LTR, or callers may flip for RTL
// editing policies). For empty run: cluster=0, trailing=false.
func HitTestClusterEdge(glyphs []ShapedGlyph, x float64) (cluster int, trailing bool) {
	if len(glyphs) == 0 {
		return 0, false
	}
	if x < glyphs[0].X {
		return glyphs[0].Cluster, false
	}
	last := glyphs[len(glyphs)-1]
	end := last.X + last.XAdvance
	if x >= end {
		return last.Cluster, true
	}
	for i := range glyphs {
		g := &glyphs[i]
		right := g.X + g.XAdvance
		if x < right || i == len(glyphs)-1 {
			// Prefer this glyph when x is inside; mid-point → trailing.
			mid := g.X + g.XAdvance*0.5
			return g.Cluster, x >= mid
		}
	}
	return last.Cluster, true
}

// CaretXForCluster returns the visual X of the caret for a logical cluster
// index in a shaped visual run.
//
// For a glyph with matching Cluster, returns that glyph's X (leading edge in
// paint order). If multiple glyphs share a cluster (ligature), returns the
// minimum X among them. If no glyph has the cluster:
//   - cluster below min → first glyph X
//   - cluster above max → end of last glyph (X+XAdvance)
//   - otherwise nearest lower cluster's trailing edge
//
// Empty run returns 0.
func CaretXForCluster(glyphs []ShapedGlyph, cluster int) float64 {
	if len(glyphs) == 0 {
		return 0
	}
	minC, maxC := glyphs[0].Cluster, glyphs[0].Cluster
	var matchMinX float64
	var matchFound bool

	for i := range glyphs {
		g := &glyphs[i]
		if g.Cluster < minC {
			minC = g.Cluster
		}
		if g.Cluster > maxC {
			maxC = g.Cluster
		}
		if g.Cluster == cluster {
			if !matchFound || g.X < matchMinX {
				matchMinX = g.X
				matchFound = true
			}
		}
	}
	if matchFound {
		return matchMinX
	}
	if cluster <= minC {
		return glyphs[0].X
	}
	if cluster > maxC {
		last := glyphs[len(glyphs)-1]
		return last.X + last.XAdvance
	}
	// Nearest: last visual glyph whose Cluster < cluster → trailing edge;
	// if none, first glyph with Cluster > cluster → leading edge.
	for i := len(glyphs) - 1; i >= 0; i-- {
		if glyphs[i].Cluster < cluster {
			return glyphs[i].X + glyphs[i].XAdvance
		}
	}
	for i := range glyphs {
		if glyphs[i].Cluster > cluster {
			return glyphs[i].X
		}
	}
	last := glyphs[len(glyphs)-1]
	return last.X + last.XAdvance
}

// RunAdvance returns total visual width of a shaped run (paint coordinates).
func RunAdvance(glyphs []ShapedGlyph) float64 {
	if len(glyphs) == 0 {
		return 0
	}
	last := glyphs[len(glyphs)-1]
	return last.X + last.XAdvance - glyphs[0].X
}
