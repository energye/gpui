//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"github.com/energye/gpui/render/text"
)

// SetFont sets the current font face for text drawing.
// The face should be created from a FontSource.
//
// Example:
//
//	source, _ := text.NewFontSourceFromFile("font.ttf")
//	face := source.Face(12.0)
//	ctx.SetFont(face)
func (c *Context) SetFont(face text.Face) {
	c.face = face
}

// Font returns the current font face.
// Returns nil if no font has been set.
// Font returns the current font face.
// Returns nil if no font has been set.
func (c *Context) Font() text.Face {
	return c.face
}

// DrawString draws text at position (x, y) where y is the baseline.
// If no font has been set with SetFont, this function does nothing.
//
// If a GPU accelerator is registered and supports text rendering (implements
// GPUTextAccelerator), the text is rendered via the GPU MSDF pipeline.
// The CTM (Current Transform Matrix) is passed to the GPU so that Scale,
// Rotate, and Skew transforms affect text rendering, not just position.
// Otherwise, the CPU text pipeline is used with transform-aware rendering:
//   - Translation-only: bitmap fast path (zero quality loss)
//   - Uniform scale ≤256px: bitmap at device size
//   - Everything else: glyph outlines as vector paths (Strategy B, Vello pattern)
//
// The baseline is the line on which most letters sit. Characters with
// descenders (like 'g', 'j', 'p', 'q', 'y') extend below the baseline.
// DrawString draws text at position (x, y) where y is the baseline.
// If no font has been set with SetFont, this function does nothing.
//
// If a GPU accelerator is registered and supports text rendering (implements
// GPUTextAccelerator), the text is rendered via the GPU MSDF pipeline.
// The CTM (Current Transform Matrix) is passed to the GPU so that Scale,
// Rotate, and Skew transforms affect text rendering, not just position.
// Otherwise, the CPU text pipeline is used with transform-aware rendering:
//   - Translation-only: bitmap fast path (zero quality loss)
//   - Uniform scale ≤256px: bitmap at device size
//   - Everything else: glyph outlines as vector paths (Strategy B, Vello pattern)
//
// The baseline is the line on which most letters sit. Characters with
// descenders (like 'g', 'j', 'p', 'q', 'y') extend below the baseline.
func (c *Context) DrawString(s string, x, y float64) {

	if c.face == nil {
		return
	}
	c.syncPublishedFilterBeforeDraw()

	// X.06: MultiFace → per-face runs so each FontSource can use GPU glyph masks.
	if mf, ok := c.face.(*text.MultiFace); ok {
		c.drawStringMultiFace(mf, s, x, y)
		return
	}

	// Set GPU scissor rect for rectangular clips.
	defer c.setGPUClipRect()()
	defer c.applyTextDecorations(s, x, y)

	c.dispatchText(s, x, y)
}

// SplitColorGlyphs partitions shaped glyphs into color vs outline subsets,
// preserving order. Color glyphs (CBDT bitmaps, COLR layers) cannot go
// through the R8 mask atlas; outline glyphs stay on the mask path.
// Pure-outline runs return the input slice untouched with zero allocation.
// DrawStringAnchored draws text with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
//
//	(0, 0) = top-left
//	(0.5, 0.5) = center
//	(1, 1) = bottom-right
//
// The text is positioned so that the anchor point is at (x, y).
func (c *Context) DrawStringAnchored(s string, x, y, ax, ay float64) {
	if c.face == nil {
		return
	}

	// Measure the text and calculate offset based on anchor.
	// The anchor maps linearly within the text bounding box:
	//   ay=0 → y is the top of the text (baseline = y + ascent)
	//   ay=0.5 → y is the vertical center (baseline = y + ascent - h/2)
	//   ay=1 → y is the bottom (baseline = y + ascent - h)
	// Formula: baseline = y + ascent - ay * h
	// where h = ascent + descent (visual bounding box, no lineGap).
	w, _ := text.Measure(s, c.face)
	metrics := c.face.Metrics()
	h := metrics.Ascent + metrics.Descent
	x -= w * ax
	y = y + metrics.Ascent - ay*h

	// Delegate to DrawString which handles TextMode routing.
	c.DrawString(s, x, y)
}

// MeasureString returns the dimensions of text in pixels.
// Returns (width, height) where:
//   - width is the horizontal advance of the text
//   - height is the line height (ascent + descent + line gap)
//
// If no font has been set, returns (0, 0).
// MeasureString returns the dimensions of text in pixels.
// Returns (width, height) where:
//   - width is the horizontal advance of the text
//   - height is the line height (ascent + descent + line gap)
//
// If no font has been set, returns (0, 0).
func (c *Context) MeasureString(s string) (w, h float64) {
	if c.face == nil {
		return 0, 0
	}
	return text.Measure(s, c.face)
}

// LoadFontFace loads a font from a file and sets it as the current font.
// The size is specified in points.
//
// Deprecated: Use text.NewFontSourceFromFile and SetFont instead.
// This method is provided for convenience and backward compatibility.
//
// Example (new way):
//
//	source, err := text.NewFontSourceFromFile("font.ttf")
//	if err != nil {
//	    return err
//	}
//	face := source.Face(12.0)
//	ctx.SetFont(face)
//
// WordWrap wraps text to fit within the given width using word boundaries.
// Returns a slice of strings, one per wrapped line.
// If no font face is set, returns the input string as a single-element slice.
//
// This method is compatible with fogleman/gg's WordWrap.
func (c *Context) WordWrap(s string, w float64) []string {
	if c.face == nil {
		return []string{s}
	}
	results := text.WrapText(s, c.face, w, text.WrapWord)
	lines := make([]string, len(results))
	for i, r := range results {
		lines[i] = r.Text
	}
	return lines
}

// MeasureMultilineString measures text that may contain newlines.
// The lineSpacing parameter is a multiplier for the font's natural line height
// (1.0 = normal spacing, 1.5 = 50% extra space between lines).
// Returns (width, height) where width is the maximum line width and height
// is the total height of all lines with the given line spacing.
// If no font face is set, returns (0, 0).
//
// This method is compatible with fogleman/gg's MeasureMultilineString.
// MeasureMultilineString measures text that may contain newlines.
// The lineSpacing parameter is a multiplier for the font's natural line height
// (1.0 = normal spacing, 1.5 = 50% extra space between lines).
// Returns (width, height) where width is the maximum line width and height
// is the total height of all lines with the given line spacing.
// If no font face is set, returns (0, 0).
//
// This method is compatible with fogleman/gg's MeasureMultilineString.
func (c *Context) MeasureMultilineString(s string, lineSpacing float64) (width, height float64) {
	if c.face == nil {
		return 0, 0
	}
	lines := splitLines(s)
	metrics := c.face.Metrics()
	fh := metrics.LineHeight()
	for _, line := range lines {
		lw, _ := text.Measure(line, c.face)
		if lw > width {
			width = lw
		}
	}
	// Visual height: ascent above first baseline + (n-1) inter-line gaps + descent below last baseline.
	n := float64(len(lines))
	height = (n-1)*fh*lineSpacing + metrics.Ascent + metrics.Descent
	return
}

// DrawStringWrapped wraps text to the given width and draws it with alignment.
// The text is positioned relative to (x, y) using the anchor (ax, ay):
//
//	(0, 0) = top-left of the text block is at (x, y)
//	(0.5, 0.5) = center of the text block is at (x, y)
//	(1, 1) = bottom-right of the text block is at (x, y)
//
// The lineSpacing parameter multiplies the font's natural line height
// (1.0 = normal, 1.5 = 50% extra space between lines).
// The align parameter controls horizontal alignment within the wrapped width.
// If no font face is set, this method does nothing.
//
// This method is compatible with fogleman/gg's DrawStringWrapped.
// DrawStringWrapped wraps text to the given width and draws it with alignment.
// The text is positioned relative to (x, y) using the anchor (ax, ay):
//
//	(0, 0) = top-left of the text block is at (x, y)
//	(0.5, 0.5) = center of the text block is at (x, y)
//	(1, 1) = bottom-right of the text block is at (x, y)
//
// The lineSpacing parameter multiplies the font's natural line height
// (1.0 = normal, 1.5 = 50% extra space between lines).
// The align parameter controls horizontal alignment within the wrapped width.
// If no font face is set, this method does nothing.
//
// This method is compatible with fogleman/gg's DrawStringWrapped.
func (c *Context) DrawStringWrapped(s string, x, y, ax, ay, width, lineSpacing float64, align Align) {
	if c.face == nil {
		return
	}
	lines := c.WordWrap(s, width)
	if len(lines) == 0 {
		return
	}

	metrics := c.face.Metrics()
	fh := metrics.LineHeight()

	// Visual height of the text block:
	// - (n-1) inter-line gaps of fh*lineSpacing
	// - ascent above first baseline + descent below last baseline
	n := float64(len(lines))
	h := (n-1)*fh*lineSpacing + metrics.Ascent + metrics.Descent

	// Adjust starting position by anchor (bounding-box model):
	//   ay=0 → y is the top of the block (first baseline = y + ascent)
	//   ay=0.5 → y is the vertical center
	//   ay=1 → y is the bottom of the block
	// Formula: first_baseline = y + ascent - ay * h
	x -= ax * width
	y = y + metrics.Ascent - ay*h

	// Adjust x base for alignment
	switch align {
	case text.AlignCenter:
		x += width / 2
	case text.AlignRight:
		x += width
	}

	for _, line := range lines {
		drawX := x
		switch align {
		case text.AlignCenter:
			lw, _ := c.MeasureString(line)
			drawX = x - lw/2
		case text.AlignRight:
			lw, _ := c.MeasureString(line)
			drawX = x - lw
		}
		c.DrawString(line, drawX, y)
		y += fh * lineSpacing
	}
}

// drawStringCPU selects the optimal CPU text rendering strategy based on the CTM.
// Three-tier decision tree modeled after Skia (QR decomposition, 256px threshold)
// and Cairo (three-matrix model):
//
//   - Tier 0: Translation-only → bitmap fast path (no quality loss)
//   - Tier 1: Uniform positive scale ≤256px → bitmap at device size (Strategy A)
//   - Tier 2: Everything else → glyph outlines as vector paths (Strategy B)
//
// StrokeString strokes text outlines at position (x, y) where y is the baseline.
// The stroke width, cap, join, and dash come from the current paint state.
//
// For thick strokes (lineWidth > 2), use [Context.SetLineJoin] with [LineJoinRound]
// to avoid miter spikes at glyph segment junctions. Glyph outlines contain many
// short curve segments, and the default [LineJoinMiter] produces sharp spikes at
// each junction. All enterprise text renderers recommend or
// default to round joins for stroked text.
//
// Unlike DrawString, StrokeString always uses vector outlines regardless of the
// current TextMode — MSDF and glyph mask pipelines cannot produce stroked text.
// If no font has been set with SetFont, this function does nothing.
//
// Enterprise pattern: matches HTML5 Canvas strokeText(), Cairo show_text() + stroke(),
// Skia SkPaint::kStroke_Style + drawTextBlob.
func (c *Context) StrokeString(s string, x, y float64) {
	if c.face == nil {
		return
	}
	path := c.textOutlinePath(s, x, y)
	if path == nil {
		return
	}

	// User matrix only — doStroke() applies deviceMatrix via deviceSpacePath().
	transformedPath := path.Transform(c.matrix)
	c.trackDamage(transformedPath.Bounds())

	// Set GPU scissor rect for rectangular clips.
	defer c.setGPUClipRect()()

	// Save and restore context path — doStroke uses c.path.
	savedPath := c.path
	c.path = transformedPath
	_ = c.doStroke()
	c.path = savedPath
}

// StrokeStringAnchored strokes text outlines with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
//
//	(0, 0) = top-left
//	(0.5, 0.5) = center
//	(1, 1) = bottom-right
//
// The text is positioned so that the anchor point is at (x, y).
// The stroke width, cap, join, and dash come from the current paint state.
// Always uses vector outlines regardless of TextMode.
// StrokeStringAnchored strokes text outlines with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
//
//	(0, 0) = top-left
//	(0.5, 0.5) = center
//	(1, 1) = bottom-right
//
// The text is positioned so that the anchor point is at (x, y).
// The stroke width, cap, join, and dash come from the current paint state.
// Always uses vector outlines regardless of TextMode.
func (c *Context) StrokeStringAnchored(s string, x, y, ax, ay float64) {
	if c.face == nil {
		return
	}

	w, _ := text.Measure(s, c.face)
	metrics := c.face.Metrics()
	h := metrics.Ascent + metrics.Descent
	x -= w * ax
	y = y + metrics.Ascent - ay*h

	c.StrokeString(s, x, y)
}

// TextPath returns a user-space Path containing the vector outlines of text s
// positioned at (x, y) where y is the baseline. The returned path can be filled,
// stroked, or used for hit-testing with the caller's own pipeline.
//
// Returns nil if no font is set or the text produces no outlines.
//
// Enterprise pattern: matches HTML5 Canvas addText() (proposed), Cairo text_path(),
// Skia SkTextBlob → SkPath (via getPath).
