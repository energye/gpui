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
	"fmt"
	"hash/fnv"
	"os"

	"github.com/energye/gpui/render/text"
)

// forceTextMode lets a developer override text rendering globally via the
// GOGPU_TEXT_MODE env var (vector|glyphmask|msdf|bitmap|aliased) to A/B the
// rendering paths without a code change. Empty = no override.
func forceTextMode() (TextMode, bool) {
	switch os.Getenv("GOGPU_TEXT_MODE") {
	case "vector":
		return TextModeVector, true
	case "glyphmask":
		return TextModeGlyphMask, true
	case "msdf":
		return TextModeMSDF, true
	case "bitmap":
		return TextModeBitmap, true
	case "aliased":
		return TextModeAliased, true
	default:
		return TextModeAuto, false
	}
}

// This is a type alias for text.Alignment, provided for fogleman/gg compatibility.
type Align = text.Alignment

// Alignment constants re-exported from the text package for convenience.
const (
	AlignLeft   = text.AlignLeft
	AlignCenter = text.AlignCenter
	AlignRight  = text.AlignRight
)

// SetFont sets the current font face for text drawing.
// The face should be created from a FontSource.
//
// Example:
//
//	source, _ := text.NewFontSourceFromFile("font.ttf")
//	face := source.Face(12.0)
//	ctx.SetFont(face)
//
// needsOutlineTransform reports whether the current CTM contains rotation,
// shear, or non-uniform scale — transforms under which fixed-resolution
// bitmap/SDF text pipelines visibly degrade.
func (c *Context) needsOutlineTransform() bool {
	m := c.matrix
	// Rotation or shear: off-axis columns in the affine matrix.
	if m.B != 0 || m.D != 0 {
		return true
	}
	// Non-uniform scale (including flip): X and Y scale magnitudes differ.
	a := m.A
	if a < 0 {
		a = -a
	}
	e := m.E
	if e < 0 {
		e = -e
	}
	return a != e
}

// nestedFaceMaxDepth caps MultiFace-in-MultiFace recursion in drawFaceRuns.
// Composite font chains resolve one level per nesting; deeper chains fall
// through to the resolved-face path instead of recursing unboundedly.
const nestedFaceMaxDepth = 4

// drawStringMultiFace renders fallback font runs (X.06). Each contiguous run
// uses a single FontSource so the GPU glyph-mask path can operate correctly.
// Runs may resolve to a nested MultiFace (composite chains); those recurse
// with a depth cap instead of reaching dispatch with a sourceless face.
// TextPath returns a user-space Path containing the vector outlines of text s
// positioned at (x, y) where y is the baseline. The returned path can be filled,
// stroked, or used for hit-testing with the caller's own pipeline.
//
// Returns nil if no font is set or the text produces no outlines.
//
// Enterprise pattern: matches HTML5 Canvas addText() (proposed), Cairo text_path(),
// Skia SkTextBlob → SkPath (via getPath).
func (c *Context) TextPath(s string, x, y float64) *Path {
	if c.face == nil {
		return nil
	}
	return c.textOutlinePath(s, x, y)
}

// textOutlinePath builds a user-space Path containing glyph outlines for text s
// at user-space position (x, y). Uses glyph cache for efficiency.
// Returns nil if the font has no FontSource (e.g. MultiFace) or the text
// produces no outlines.
// textOutlinePath builds a user-space Path containing glyph outlines for text s
// at user-space position (x, y). Uses glyph cache for efficiency.
// Returns nil if the font has no FontSource (e.g. MultiFace) or the text
// produces no outlines.
func (c *Context) textOutlinePath(s string, x, y float64) *Path {
	source := c.face.Source()
	if source == nil {
		return nil
	}
	// The context keeps a glyph cache for the hot path; extraction semantics
	// are identical to the pure function below.
	return textOutlinePathFrom(source, c.face, s, x, y, c.ensureOutlineExtractor(), c.ensureGlyphCache())
}

// TextOutlinePath builds a user-space Path containing glyph outlines for text
// s at user-space position (x, baseline). Pure function: it needs only the
// face (single FontSource — MultiFace has none) and a per-call outline
// extractor, owns no GPU or window state, and is safe on any thread. This is
// what makes TextPath-style content RECORDABLE: a display-list walk computes
// the path on the UI thread and records FillPath/StrokePath commands; the
// Context method above is its cache-backed twin.
//
// Shaping uses text.Shape (GSUB/GPOS — ligatures and kerning apply), matching
// the GPU mask path since A1.
func TextOutlinePath(face text.Face, s string, x, y float64) *Path {
	if face == nil || s == "" {
		return nil
	}
	source := face.Source()
	if source == nil {
		return nil
	}
	return textOutlinePathFrom(source, face, s, x, y, text.NewOutlineExtractor(), nil)
}

// textOutlinePathFrom is the shared outline-path builder. cache may be nil
// (per-call extraction).
func textOutlinePathFrom(source *text.FontSource, face text.Face, s string, x, y float64, extractor *text.OutlineExtractor, cache *text.GlyphCache) *Path {
	parsed := source.Parsed()
	fontSize := face.Size()
	variations := face.Variations()

	fontID := textFontIDWithVariations(computeTextFontID(source), variations)
	sizeKey := text.FixedSize(fontSize)

	path := NewPath()
	hasContour := false

	shaped := text.Shape(s, face)
	for _, sg := range shaped {
		var outline *text.GlyphOutline
		if cache != nil {
			cacheKey := text.OutlineCacheKey{
				FontID:  fontID,
				GID:     sg.GID,
				Size:    sizeKey,
				Hinting: text.HintingNone,
			}
			outline = cache.GetOrCreate(cacheKey, func() *text.GlyphOutline {
				o, err := extractor.ExtractOutlineHintedVar(parsed, sg.GID, fontSize, text.HintingNone, variations)
				if err != nil || o == nil || o.IsEmpty() {
					return nil
				}
				return o
			})
		} else {
			o, err := extractor.ExtractOutlineHintedVar(parsed, sg.GID, fontSize, text.HintingNone, variations)
			if err != nil || o == nil || o.IsEmpty() {
				continue
			}
			outline = o
		}
		if outline == nil {
			continue
		}

		gx := x + sg.X

		for _, seg := range outline.Segments {
			// sfnt.LoadGlyph returns Y-down coordinates (screen convention):
			// Y=0 at baseline, Y<0 above baseline, Y>0 below baseline.
			// So we ADD outlineY to baseline (no flip needed).
			switch seg.Op {
			case text.OutlineOpMoveTo:
				if hasContour {
					path.Close()
				}
				path.MoveTo(gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y))
				hasContour = true
			case text.OutlineOpLineTo:
				path.LineTo(gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y))
			case text.OutlineOpQuadTo:
				path.QuadraticTo(
					gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y),
					gx+float64(seg.Points[1].X), y+float64(seg.Points[1].Y))
			case text.OutlineOpCubicTo:
				path.CubicTo(
					gx+float64(seg.Points[0].X), y+float64(seg.Points[0].Y),
					gx+float64(seg.Points[1].X), y+float64(seg.Points[1].Y),
					gx+float64(seg.Points[2].X), y+float64(seg.Points[2].Y))
			}
		}
	}
	if hasContour {
		path.Close()
	}
	if path.isEmpty() {
		return nil
	}
	return path
}

// drawStringAsOutlines renders text by converting glyph vector outlines to a Path
// and filling through the normal multi-tier pipeline (GPU → CoverageFiller → Analytic).
// Strategy B (Vello pattern): handles rotation, non-uniform scale, shear, mirroring,
// and extreme scales that exceed the bitmap threshold.
//
// Design: all glyphs are composed into ONE path for a single efficient fill call.
// Outlines are built in user space, then path.Transform(CTM) converts to device space.
// The device-space path is routed through doFill() so that GPU accelerator can render
// it to the surface (stencil+cover) when SurfaceTarget is active, or CPU renders
// to pixmap in standalone mode.
// ensureOutlineExtractor lazily initializes the outline extractor.
func (c *Context) ensureOutlineExtractor() *text.OutlineExtractor {
	if c.outlineExtractor == nil {
		c.outlineExtractor = text.NewOutlineExtractor()
	}
	return c.outlineExtractor
}

// ensureGlyphCache lazily initializes the glyph cache reference.
// Uses the global shared cache to benefit from cross-Context reuse.
// ensureGlyphCache lazily initializes the glyph cache reference.
// Uses the global shared cache to benefit from cross-Context reuse.
func (c *Context) ensureGlyphCache() *text.GlyphCache {
	if c.glyphCache == nil {
		c.glyphCache = text.GetGlobalGlyphCache()
	}
	return c.glyphCache
}

// computeTextFontID generates a stable hash identifier for a font source.
// Uses FNV-1a hash of font name and glyph count as a lightweight fingerprint.
// Same algorithm as internal/gpu/gpu_text.go:computeFontID.
// computeTextFontID generates a stable hash identifier for a font source.
// Uses FNV-1a hash of font name and glyph count as a lightweight fingerprint.
// Same algorithm as internal/gpu/gpu_text.go:computeFontID.
func computeTextFontID(source *text.FontSource) uint64 {
	if source == nil {
		return 0
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s:%d", source.Name(), source.Parsed().NumGlyphs())
	return h.Sum64()
}

// textFontIDWithVariations separates outline-cache entries for two faces made
// from the same variable FontSource. The base identity is font name + glyph
// count; without the variation hash wght 200 and wght 900 share one cached
// outline (usually the first weight that happened to be rendered).
func textFontIDWithVariations(base uint64, variations []text.FontVariation) uint64 {
	if len(variations) == 0 {
		return base
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d:%d", base, text.VariationHash(variations))
	return h.Sum64()
}

// fontHeight returns the font's natural line height (ascent + descent + line gap).
// fontHeight returns the font's natural line height (ascent + descent + line gap).
func (c *Context) fontHeight() float64 {
	if c.face == nil {
		return 0
	}
	return c.face.Metrics().LineHeight()
}

// splitLines splits text by line breaks, normalizing \r\n and \r to \n.
