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
	"image"
	"image/color"
	"sync/atomic"

	"github.com/energye/gpui/render/text/emoji"
)

// SVGTableRenderer renders one OpenType SVG-table color glyph document into
// premultiplied RGBA pixels.
//
//   - glyphID: the glyph being rendered; the document element with
//     id "glyph<N>" is the glyph description (use-semantics, OT 1.9.1).
//   - doc: decompressed UTF-8 SVG document (may describe several glyphs).
//   - upem: font design units per em; ppem: requested pixel size of the em.
//     The table's coordinate contract applies: document user units are font
//     design units, y is down, the origin is the baseline pen position, and
//     the initial (em-square) viewport must NOT be clipped.
//   - fg: foreground color for currentColor (typically the text color).
//   - palette: CPAL palette colors for var(--colorN) references (may be nil).
//
// Returns the RGBA image plus the baseline-relative origin (the pixel
// position of the baseline pen point inside the image), matching the
// ColorRasterImage origin conventions.
type SVGTableRenderer func(glyphID uint16, doc []byte, upem, ppem float64, fg color.RGBA, palette []emoji.Color) (img *image.RGBA, originX, originY float32, err error)

var svgTableRenderer atomic.Pointer[SVGTableRenderer]

// SetSVGTableRenderer installs the pixel renderer for SVG-table color
// glyphs. The text package owns the data plane (table parsing, glyph type,
// caching); pixel production lives behind this hook in the render layer
// (render.Context + render/svg) because render/svg depends on text, not the
// other way around. Unset: SVG-table glyphs rasterize fail-closed (raster
// error → outline/none), the documented not-yet-wired state.
func SetSVGTableRenderer(fn SVGTableRenderer) {
	if fn == nil {
		svgTableRenderer.Store(nil)
		return
	}
	svgTableRenderer.Store(&fn)
}

// currentSVGTableRenderer returns the installed renderer, or nil.
func currentSVGTableRenderer() SVGTableRenderer {
	if p := svgTableRenderer.Load(); p != nil {
		return *p
	}
	return nil
}

// COLRv1Renderer renders a COLRv1 paint graph (static instance) into
// premultiplied RGBA pixels.
//
//   - font: outline source for PaintGlyph fills (ExtractOutline at upem
//     yields design units).
//   - glyph: the COLRv1 glyph (Paint root set; Clip optional).
//   - upem/ppem: font design units per em and requested pixel size.
//   - paletteIndex: CPAL palette to resolve color references against.
//   - fg: foreground color for palette index 0xFFFF.
//
// Returns the RGBA image plus the baseline-relative origin (same
// conventions as ColorRasterImage), or an error (fail-closed: the caller
// falls back to the outline glyph).
type COLRv1Renderer func(font ParsedFont, glyph *emoji.COLRGlyph, upem, ppem float64, paletteIndex int, fg color.RGBA) (img *image.RGBA, originX, originY float32, err error)

var colrV1Renderer atomic.Pointer[COLRv1Renderer]

// SetCOLRv1Renderer installs the COLRv1 paint-graph pixel renderer. The
// implementation lives in the render layer (render.Context gradients,
// clip, composite); text keeps the data plane. Unset: v1 glyphs rasterize
// fail-closed, the documented not-yet-wired state.
func SetCOLRv1Renderer(fn COLRv1Renderer) {
	if fn == nil {
		colrV1Renderer.Store(nil)
		return
	}
	colrV1Renderer.Store(&fn)
}

// currentCOLRv1Renderer returns the installed renderer, or nil.
func currentCOLRv1Renderer() COLRv1Renderer {
	if p := colrV1Renderer.Load(); p != nil {
		return *p
	}
	return nil
}
