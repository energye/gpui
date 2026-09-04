package text

import (
	"image/color"
	"os"
	"testing"
)

func TestColorRaster_CBDT_Synthetic(t *testing.T) {
	pf := loadGoRegularFont(t)
	opf := pf.(*ownParsedFont)
	gid := pf.GlyphIndex('A')
	tables := make(map[string][]byte, len(opf.tables)+2)
	for k, v := range opf.tables {
		tables[k] = v
	}
	tables["CBDT"] = syntheticCBDT()
	tables["CBLC"] = syntheticCBLC(gid)
	font := &ownParsedFont{rawData: opf.rawData, tables: tables, upem: opf.upem, numGlyphs: opf.numGlyphs}

	cache := NewColorRasterCache(64)
	img, err := cache.Image(font, gid, 16, 0, color.RGBA{255, 255, 255, 255})
	if err != nil {
		t.Fatalf("Image(bitmap gid) error = %v", err)
	}
	if img.Pix.Bounds().Dx() != 8 || img.Pix.Bounds().Dy() != 8 {
		t.Errorf("bounds = %v, want 8x8", img.Pix.Bounds())
	}
}

func TestColorRaster_COLR_Synthetic(t *testing.T) {
	pf := loadGoRegularFont(t)
	opf := pf.(*ownParsedFont)
	gid := pf.GlyphIndex('A')
	tables := make(map[string][]byte, len(opf.tables)+2)
	for k, v := range opf.tables {
		tables[k] = v
	}
	tables["COLR"] = syntheticCOLR(gid)
	tables["CPAL"] = syntheticCPAL()
	font := &ownParsedFont{rawData: opf.rawData, tables: tables, upem: opf.upem, numGlyphs: opf.numGlyphs}

	cache := NewColorRasterCache(64)
	img, err := cache.Image(font, gid, 16, 0, color.RGBA{255, 255, 255, 255})
	if err != nil {
		t.Fatalf("Image(COLR gid) error = %v", err)
	}
	if colorRasterNonBlank(img) == 0 {
		t.Error("COLR composite is fully transparent, want ink from layer outline")
	}
}

func TestColorRaster_Outline_Error(t *testing.T) {
	pf := loadGoRegularFont(t)
	cache := NewColorRasterCache(64)
	if _, err := cache.Image(pf, pf.GlyphIndex('B'), 16, 0, color.RGBA{255, 255, 255, 255}); err == nil {
		t.Error("Image(outline gid) = nil error, want refusal")
	}
}

func TestColorRaster_CBDT_RealEmoji(t *testing.T) {
	for _, p := range []string{
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/TTF/NotoColorEmoji.ttf",
	} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		src, err := NewFontSource(data, WithParser("own"))
		if err != nil {
			t.Fatalf("NewFontSource: %v", err)
		}
		defer src.Close()
		gid := src.Parsed().GlyphIndex(0x1F600)
		if src.Parsed().(ColorFont).GlyphType(gid) != GlyphTypeBitmap {
			t.Fatalf("U+1F600 type = %v, want Bitmap", src.Parsed().(ColorFont).GlyphType(gid))
		}
		cache := NewColorRasterCache(64)
		first, err := cache.Image(src.Parsed(), gid, 16, 0, color.RGBA{255, 255, 255, 255})
		if err != nil {
			t.Fatalf("Image(emoji) error = %v", err)
		}
		if colorRasterNonBlank(first) == 0 {
			t.Error("emoji raster is fully transparent, want ink")
		}
		second, err := cache.Image(src.Parsed(), gid, 16, 0, color.RGBA{255, 255, 255, 255})
		if err != nil {
			t.Fatalf("Image(emoji) second call error = %v", err)
		}
		if second != first {
			t.Error("second call missed cache, want same pointer")
		}
		return
	}
	t.Skip("NotoColorEmoji not available")
}

func colorRasterNonBlank(img *ColorRasterImage) int {
	n := 0
	for i := 3; i < len(img.Pix.Pix); i += 4 {
		if img.Pix.Pix[i] != 0 {
			n++
		}
	}
	return n
}
