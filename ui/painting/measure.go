package painting

import (
	"unicode/utf8"
)

// EstimateTextSize returns a layout estimate without a font face.
// Uses rune count (not bytes) so CJK is closer than len(string).
// fontSize and approxCharW are logical; height ≈ fontSize * 1.25.
func EstimateTextSize(s string, fontSize, approxCharW float64) (w, h float64) {
	if fontSize <= 0 {
		fontSize = 14
	}
	if approxCharW <= 0 {
		approxCharW = 0.55
	}
	n := float64(utf8.RuneCountInString(s))
	return n * fontSize * approxCharW, fontSize * 1.25
}

// MeasureText returns the size of s using the DC current font when available.
// If DC is nil or has no face, falls back to EstimateTextSize(fontSize, approxCharW).
func (c *Context) MeasureText(s string, fontSize, approxCharW float64) (w, h float64) {
	if c != nil && c.DC != nil && c.DC.Font() != nil && s != "" {
		return c.DC.MeasureString(s)
	}
	return EstimateTextSize(s, fontSize, approxCharW)
}

// MeasureTextMultiline measures wrapped/multiline string when a font is set;
// otherwise estimates as single-line height * line count heuristic.
func (c *Context) MeasureTextMultiline(s string, lineSpacing, fontSize, approxCharW float64) (w, h float64) {
	if c != nil && c.DC != nil && c.DC.Font() != nil && s != "" {
		if lineSpacing <= 0 {
			lineSpacing = 1.2
		}
		return c.DC.MeasureMultilineString(s, lineSpacing)
	}
	// Fallback: treat as one line estimate (callers needing wrap should set a font).
	return EstimateTextSize(s, fontSize, approxCharW)
}
