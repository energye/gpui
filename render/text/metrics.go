package text

// Metrics holds font metrics at a specific size.
// These metrics are derived from the font file and scaled to the face size.
// Metrics represents the overall font metrics.
type Metrics struct {
	// Ascent is the distance from the baseline to the top of the font (positive).
	// This is the maximum height a glyph can reach above the baseline.
	Ascent float64

	// Descent is the distance from the baseline to the bottom of the font (positive, below baseline).
	// This is the maximum depth a glyph can reach below the baseline.
	// Note: Unlike FontMetrics.Descent, this is stored as a positive value.
	Descent float64

	// LineGap is the recommended gap between lines.
	LineGap float64

	// XHeight is the height of lowercase letters (like 'x').
	XHeight float64

	// CapHeight is the height of uppercase letters.
	CapHeight float64

	// VerticalAscent is the vertical typographic ascender (from vhea).
	// Positive when present; 0 when the font has no vhea table.
	VerticalAscent float64

	// VerticalDescent is the vertical typographic descender (from vhea),
	// stored positive (below the vertical baseline); 0 when no vhea table.
	VerticalDescent float64

	// VerticalLineGap is the vertical typographic gap (from vhea);
	// 0 when the font has no vhea table.
	VerticalLineGap float64
}

// LineHeight returns the total line height (ascent + descent + line gap).
// This is the recommended vertical distance between baselines of consecutive lines.
func (m Metrics) LineHeight() float64 {
	return m.Ascent + m.Descent + m.LineGap
}
