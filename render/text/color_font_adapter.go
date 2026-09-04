package text

import (
	"github.com/energye/gpui/render/text/emoji"
)

type colorBackends struct {
	cbdt *emoji.CBDTExtractor
	colr *emoji.COLRParser
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
		if colr, cpal := f.tables["COLR"], f.tables["CPAL"]; len(colr) > 0 && len(cpal) > 0 {
			if parser, err := emoji.NewCOLRParser(colr, cpal); err == nil {
				out.colr = parser
			}
		}
		if out.cbdt == nil && out.colr == nil {
			return nil, nil
		}
		return out, nil
	})
	return backends
}

func (f *ownParsedFont) HasColorTables() bool {
	return f.loadColor() != nil
}

func (f *ownParsedFont) GlyphType(glyphID uint16) GlyphType {
	backends := f.loadColor()
	if backends == nil {
		return GlyphTypeOutline
	}
	if backends.cbdt != nil && backends.cbdt.HasGlyph(glyphID) {
		return GlyphTypeBitmap
	}
	if backends.colr != nil && backends.colr.HasGlyph(glyphID) {
		return GlyphTypeCOLR
	}
	return GlyphTypeOutline
}

func (f *ownParsedFont) BitmapGlyph(glyphID uint16, ppem uint16) (*emoji.BitmapGlyph, error) {
	backends := f.loadColor()
	if backends == nil || backends.cbdt == nil {
		return nil, emoji.ErrNoCBDTTable
	}
	return backends.cbdt.GetGlyph(glyphID, ppem)
}

func (f *ownParsedFont) COLRGlyph(glyphID uint16, paletteIndex int) (*emoji.COLRGlyph, error) {
	backends := f.loadColor()
	if backends == nil || backends.colr == nil {
		return nil, emoji.ErrNoCOLRTable
	}
	return backends.colr.GetGlyph(glyphID, paletteIndex)
}
