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
	"bytes"
	"encoding/binary"

	"github.com/energye/gpui/render/text/emoji"
)

type colorBackends struct {
	cbdt *emoji.CBDTExtractor
	sbix *emoji.SBIXParser
	colr *emoji.COLRParser
	svg  *emoji.SVGTableParser
	pal  [][]emoji.Color // CPAL palettes (standalone: SVG var() source)
}

func (f *ownParsedFont) loadColor() *colorBackends {
	if f == nil {
		return nil
	}
	backends, _ := f.color.load(func() (*colorBackends, error) {
		out := &colorBackends{}
		if cbdt, cblc := f.tables["CBDT"], f.tables["CBLC"]; len(cbdt) > 0 && len(cblc) > 0 {
			if ext, err := emoji.NewCBDTExtractor(cbdt, cblc); err == nil {
				out.cbdt = ext
			}
		}
		if sbix := f.tables["sbix"]; len(sbix) > 0 && f.numGlyphs > 0 {
			if parser, err := emoji.NewSBIXParser(sbix, uint16(f.numGlyphs)); err == nil {
				out.sbix = parser
			}
		}
		if colr, cpal := f.tables["COLR"], f.tables["CPAL"]; len(colr) > 0 && len(cpal) > 0 {
			if parser, err := emoji.NewCOLRParser(colr, cpal); err == nil {
				out.colr = parser
			}
		}
		if svg := f.tables["SVG "]; len(svg) > 0 {
			if parser, err := emoji.NewSVGTableParser(svg); err == nil {
				out.svg = parser
			}
		}
		if cpal := f.tables["CPAL"]; len(cpal) > 0 {
			if palettes, err := emoji.ParseCPALTable(cpal); err == nil {
				out.pal = palettes
			}
		}
		if out.cbdt == nil && out.sbix == nil && out.colr == nil && out.svg == nil {
			return nil, nil
		}
		return out, nil
	})
	return backends
}

func (f *ownParsedFont) HasColorTables() bool {
	return f.loadColor() != nil
}

// hasSbixGlyph reports whether any sbix strike carries a bitmap for glyphID.
func (b *colorBackends) hasSbixGlyph(glyphID uint16) bool {
	if b.sbix == nil {
		return false
	}
	for s := range b.sbix.NumStrikes() {
		if b.sbix.HasGlyph(int(glyphID), s) {
			return true
		}
	}
	return false
}

// hasCBDTGlyph reports whether the CBLC index covers glyphID.
func (b *colorBackends) hasCBDTGlyph(glyphID uint16) bool {
	return b.cbdt != nil && b.cbdt.HasGlyph(glyphID)
}

func (f *ownParsedFont) GlyphType(glyphID uint16) GlyphType {
	backends := f.loadColor()
	if backends == nil {
		return GlyphTypeOutline
	}
	if backends.hasCBDTGlyph(glyphID) || backends.hasSbixGlyph(glyphID) {
		return GlyphTypeBitmap
	}
	if backends.colr != nil && backends.colr.HasGlyph(glyphID) {
		return GlyphTypeCOLR
	}
	if backends.svg != nil && backends.svg.HasGlyph(glyphID) {
		return GlyphTypeSVG
	}
	return GlyphTypeOutline
}

// BitmapGlyph extracts an embedded bitmap glyph (CBDT first, then sbix).
func (f *ownParsedFont) BitmapGlyph(glyphID uint16, ppem uint16) (*emoji.BitmapGlyph, error) {
	backends := f.loadColor()
	if backends == nil {
		return nil, emoji.ErrGlyphNotInBitmap
	}
	if backends.hasCBDTGlyph(glyphID) {
		return backends.cbdt.GetGlyph(glyphID, ppem)
	}
	if backends.sbix != nil {
		strike := backends.sbix.BestStrikeForPPEM(ppem)
		if strike >= 0 && backends.sbix.HasGlyph(int(glyphID), strike) {
			return backends.sbix.GetGlyph(int(glyphID), strike)
		}
	}
	return nil, emoji.ErrGlyphNotInBitmap
}

// COLRGlyph returns the COLR glyph data for layered rendering.
func (f *ownParsedFont) COLRGlyph(glyphID uint16, paletteIndex int) (*emoji.COLRGlyph, error) {
	backends := f.loadColor()
	if backends == nil || backends.colr == nil {
		return nil, emoji.ErrNoCOLRTable
	}
	return backends.colr.GetGlyph(glyphID, paletteIndex)
}

// COLRGlyphVar is COLRGlyph at a variation instance (P4.4): the COLR table's
// ItemVariationStore supplies deltas for every Var* paint field, resolved with
// the face's normalized axis coordinates (fvar + avar).
func (f *ownParsedFont) COLRGlyphVar(glyphID uint16, paletteIndex int, variations []FontVariation) (*emoji.COLRGlyph, error) {
	backends := f.loadColor()
	if backends == nil || backends.colr == nil {
		return nil, emoji.ErrNoCOLRTable
	}
	if len(variations) == 0 {
		return backends.colr.GetGlyph(glyphID, paletteIndex)
	}
	colr := f.tables["COLR"]
	if len(colr) < 34 || binary.BigEndian.Uint16(colr[0:2]) != 1 {
		return backends.colr.GetGlyph(glyphID, paletteIndex)
	}
	ivsOff := binary.BigEndian.Uint32(colr[30:34])
	if ivsOff == 0 || int(ivsOff) >= len(colr) {
		return backends.colr.GetGlyph(glyphID, paletteIndex)
	}
	ivs, err := parseItemVariationStore(colr[ivsOff:])
	if err != nil || ivs == nil {
		return backends.colr.GetGlyph(glyphID, paletteIndex)
	}
	coords := normalizeCoords(f.loadFvar(), variations)
	if av := f.loadAvar(); av != nil {
		av.apply(coords)
	}
	vars := colrVarDeltas{ivs: ivs, coords: coords}
	root, err := backends.colr.PaintRootVar(glyphID, vars)
	if err != nil || root == nil {
		return backends.colr.GetGlyph(glyphID, paletteIndex)
	}
	g := &emoji.COLRGlyph{GlyphID: glyphID, Version: 1, Paint: root}
	if cb, ok := backends.colr.ClipBoxFor(glyphID); ok {
		g.Clip = &cb
	}
	return g, nil
}

// colrVarDeltas resolves a COLR varIndex (outer<<16|inner, HarfBuzz form) into
// an integer delta at the face's normalized coordinates.
type colrVarDeltas struct {
	ivs    *itemVariationStore
	coords []int16
}

func (d colrVarDeltas) Delta(varIndex uint32) int32 {
	if d.ivs == nil {
		return 0
	}
	return d.ivs.computeDelta(uint16(varIndex>>16), uint16(varIndex), d.coords)
}

// SVGGlyph returns the decoded SVG document for an SVG-table glyph.
func (f *ownParsedFont) SVGGlyph(glyphID uint16) (*emoji.SVGGlyph, error) {
	backends := f.loadColor()
	if backends == nil || backends.svg == nil {
		return nil, emoji.ErrNoSVGTable
	}
	doc, err := backends.svg.GlyphDocument(glyphID)
	if err != nil {
		return nil, err
	}
	g := &emoji.SVGGlyph{GlyphID: glyphID, Doc: doc}
	g.UsesCurrentColor = bytes.Contains(doc, []byte("currentColor"))
	g.UsesPalette = bytes.Contains(doc, []byte("var(--color"))
	return g, nil
}

// Palette returns CPAL palette paletteIndex (palette 0 is the static default).
func (f *ownParsedFont) Palette(paletteIndex int) []emoji.Color {
	backends := f.loadColor()
	if backends == nil || paletteIndex < 0 || paletteIndex >= len(backends.pal) {
		return nil
	}
	return backends.pal[paletteIndex]
}
