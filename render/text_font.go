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
	"image"
	"math"
	"strings"

	"github.com/energye/gpui/render/text"
)

// trackTextDamage registers the ink bounds of a GPU-queued text draw into the
// current frame + layer damage. Fill/Stroke record c.path.Bounds() inside
// Context.Fill/Stroke, so rectangle/vector draws always contribute to damage;
// GPU glyph-mask/MSDF text paths bypass Context.Fill and therefore must record
// their own bounds. Without this, an isolation layer's damage (which becomes
// the layer-RT scissor in FlushGPUWithViewDamage) only covers shape draws and
// silently clips CJK runs that extend past the last FillRect edge — the
// "合成残影" bug in ui_wr_r18_savelayer.
func (c *Context) trackTextDamage(s string, x, y float64) {
	if c.face == nil || s == "" {
		return
	}
	m := c.face.Metrics()
	ascent := m.Ascent
	if ascent < 0 {
		ascent = -ascent
	}
	descent := m.Descent
	if descent < 0 {
		descent = -descent
	}
	width := c.face.Advance(s)
	if width <= 0 {
		width = m.XHeight * float64(len(s)) //nolint:mnd // conservative fallback
	}
	// x is the text baseline origin in user space (Y-down baseline, ascent above).
	bounds := image.Rect(
		int(math.Floor(x))-1,
		int(math.Floor(y-ascent))-1,
		int(math.Ceil(x+width))+1,
		int(math.Ceil(y+descent))+1,
	)
	if bounds.Empty() {
		return
	}
	c.trackDamage(bounds)
}

// tryGPUGlyphMaskTextAliased attempts to render aliased text via the GPU glyph
// mask pipeline. Same Tier 6 pipeline but with binary (0/255) rasterization.
// Returns true if text was successfully queued for aliased glyph mask rendering.
// selectTextStrategy returns the effective text rendering strategy.
//
// When TextModeAuto, the strategy is derived from the face + CTM alone:
//   - rotated/sheared/non-uniform transforms → glyph outlines (kPath);
//   - axis-aligned text within the glyph-mask size bound → glyph bitmaps;
//   - otherwise → the default MSDF→CPU fallback.
//
// Explicit modes (MSDF, Vector, Bitmap, GlyphMask) are returned as-is.
func (c *Context) selectTextStrategy() TextMode {
	if m, ok := forceTextMode(); ok {
		return m
	}
	if c.textMode != TextModeAuto {
		return c.textMode
	}
	// GOGPU_RENDER_MODE=cpu: route text straight to the CPU bitmap pipeline —
	// the window present (pixmap upload) must carry glyphs without the GPU
	// session's glyph-mask/MSDF renderers.
	if CPUOnlyMode() {
		return TextModeBitmap
	}
	// Transform-quality routing: rotated/sheared/non-uniform transforms render
	// glyph outlines as paths (GPU stencil+cover / CPU Tier 2). the
	// kTransformedMask (rotated bitmap quads) is not enabled: the glyph-mask
	// pipeline renders rotated quads incorrectly under multi-draw accumulation
	// (observed black flooding in the 9-cell render_text_transform example) —
	// routing into it would render falsely. Rotated text stays on outlines.
	if c.needsOutlineTransform() {
		return TextModeVector
	}
	if c.shouldUseGlyphMask() {
		return TextModeGlyphMask
	}
	return TextModeAuto
}

// shouldUseGlyphMask returns true when auto-selection should prefer glyph
// mask rendering (Tier 6). Conditions: GPU with glyph mask support, font size
// in device pixels <= glyphMaskMaxSize. Rotated/sheared/non-uniform matrices
// were already routed to outlines by selectTextStrategy, so no axis check is
// needed here.
// shouldUseGlyphMask returns true when auto-selection should prefer glyph
// mask rendering (Tier 6). Conditions: GPU with glyph mask support, font size
// in device pixels <= glyphMaskMaxSize. Rotated/sheared/non-uniform matrices
// were already routed to outlines by selectTextStrategy, so no axis check is
// needed here.
func (c *Context) shouldUseGlyphMask() bool {
	a := Accelerator()
	if a == nil {
		return false
	}
	if _, ok := a.(GPUGlyphMaskAccelerator); !ok {
		return false
	}

	if c.face == nil {
		return false
	}

	return c.glyphMaskDeviceSize() <= glyphMaskMaxSize
}

// glyphMaskDeviceSize returns the effective font size in device pixels,
// accounting for deviceScale and the Y scale component of the matrix.
// glyphMaskDeviceSize returns the effective font size in device pixels,
// accounting for deviceScale and the Y scale component of the matrix.
func (c *Context) glyphMaskDeviceSize() float64 {
	deviceSize := c.face.Size() * c.DeviceScale()
	absScale := c.matrix.E
	if absScale < 0 {
		absScale = -absScale
	}
	if absScale != 0 {
		deviceSize *= absScale
	}
	return deviceSize
}

// DrawStringAnchored draws text with an anchor point.
// The anchor point is specified by ax and ay, which are in the range [0, 1].
//
//	(0, 0) = top-left
//	(0.5, 0.5) = center
//	(1, 1) = bottom-right
//
// The text is positioned so that the anchor point is at (x, y).
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
func (c *Context) LoadFontFace(path string, points float64) error {

	source, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return err
	}
	c.face = source.Face(points)
	return nil
}

// LoadFontFaceWithVariations loads a variable font and applies axis values (X.09).
// Example: dc.LoadFontFaceWithVariations("Cantarell.ttf", 16, text.NewFontVariation("wght", 700))
// LoadFontFaceWithVariations loads a variable font and applies axis values (X.09).
// Example: dc.LoadFontFaceWithVariations("Cantarell.ttf", 16, text.NewFontVariation("wght", 700))
func (c *Context) LoadFontFaceWithVariations(path string, points float64, vars ...text.FontVariation) error {
	source, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return err
	}
	if len(vars) > 0 {
		c.face = source.Face(points, text.WithVariations(vars...))
	} else {
		c.face = source.Face(points)
	}
	return nil
}

// FontVariationAxes returns variation axes for the current face's font source (X.09).
// FontVariationAxes returns variation axes for the current face's font source (X.09).
func (c *Context) FontVariationAxes() []text.VariationAxis {
	if c == nil || c.face == nil {
		return nil
	}
	src := c.face.Source()
	if src == nil {
		return nil
	}
	return src.VariationAxes()
}

// WordWrap wraps text to fit within the given width using word boundaries.
// Returns a slice of strings, one per wrapped line.
// If no font face is set, returns the input string as a single-element slice.
//
// This method is compatible with fogleman/gg's WordWrap.
// splitLines splits text by line breaks, normalizing \r\n and \r to \n.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}
