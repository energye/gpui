//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package svg

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

// OpenType SVG-table color glyph rasterization: this package implements the
// text package's SVGTableRenderer hook (the renderer needs Parse +
// RenderTableGlyph, which live here; the render main package must not
// import this package back).

// InstallSVGTableRenderer wires the SVG-table color glyph raster into the
// text package: table documents rasterize through Parse + RenderTableGlyph
// into premultiplied RGBA. Explicit and idempotent, same wiring discipline
// as SetFontResolver — call once where SVG rendering is available.
func InstallSVGTableRenderer() {
	text.SetSVGTableRenderer(renderTableGlyphRGBA)
}

// The rasterization window around the baseline origin, in em units
// (design-space, y down: the glyph lives above the baseline, i.e. at
// negative y). Real color glyphs stay well inside; ink outside this window
// is not part of the raster (documented bound — the table contract forbids
// clipping but glyph art exceeding ±2em is pathological).
const (
	svgWindowLeftEm   = -1.0
	svgWindowTopEm    = -2.0
	svgWindowRightEm  = 2.0
	svgWindowBottomEm = 1.0
)

// renderTableGlyphRGBA is the text.SVGTableRenderer implementation.
func renderTableGlyphRGBA(glyphID uint16, doc []byte, upem, ppem float64, fg color.RGBA, palette []emoji.Color) (*image.RGBA, float32, float32, error) {
	if upem <= 0 || ppem <= 0 {
		return nil, 0, 0, fmt.Errorf("svg: invalid table glyph metrics upem=%v ppem=%v", upem, ppem)
	}
	d, err := Parse(doc)
	if err != nil {
		return nil, 0, 0, err
	}
	var pal []color.Color
	if len(palette) > 0 {
		pal = make([]color.Color, len(palette))
		for i, c := range palette {
			pal[i] = c.ToRGBA()
		}
	}

	// Canvas: the window scaled to pixels. design (left,top) maps to (0,0).
	s := ppem / upem
	cw := int(math.Ceil((svgWindowRightEm - svgWindowLeftEm) * ppem))
	ch := int(math.Ceil((svgWindowBottomEm - svgWindowTopEm) * ppem))
	dc := render.NewContext(cw, ch, render.WithCPUOnly())
	dc.Push()
	dc.Translate(-svgWindowLeftEm*ppem, -svgWindowTopEm*ppem)
	dc.Scale(s, s)
	ok := d.RenderTableGlyph(dc, fmt.Sprintf("glyph%d", glyphID), TableGlyphOptions{
		CurrentColor: fg,
		Palette:      pal,
		ViewportW:    upem,
		ViewportH:    upem,
	})
	dc.Pop()
	if !ok {
		return nil, 0, 0, fmt.Errorf("svg: glyph %d has no glyph<N> element", glyphID)
	}
	frame, isRGBA := dc.Image().(*image.RGBA)
	if !isRGBA {
		return nil, 0, 0, fmt.Errorf("svg: glyph %d readback is not RGBA", glyphID)
	}

	// Crop to ink; origin = baseline pen position inside the cropped image
	// (shared with the COLRv1 rasterizer).
	penX := -svgWindowLeftEm * ppem
	penY := -svgWindowTopEm * ppem
	return render.CropImageToInk(frame, penX, penY)
}


