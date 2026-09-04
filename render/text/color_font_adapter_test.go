package text

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestColorAdapter_ImplementsInterface(t *testing.T) {
	pf := loadGoRegularFont(t)
	if _, ok := any(pf).(ColorFont); !ok {
		t.Fatal("ownParsedFont does not implement ColorFont (zero implementers)")
	}
}

func TestColorAdapter_NoColorTables(t *testing.T) {
	pf := loadGoRegularFont(t)
	cf, ok := any(pf).(ColorFont)
	if !ok {
		t.Fatal("ownParsedFont does not implement ColorFont (zero implementers)")
	}
	if cf.HasColorTables() {
		t.Error("HasColorTables() = true for outline-only font, want false")
	}
	if got := DetectGlyphType(pf, pf.GlyphIndex('A')); got != GlyphTypeOutline {
		t.Errorf("DetectGlyphType() = %v, want Outline", got)
	}
}

func TestColorAdapter_SyntheticCOLR(t *testing.T) {
	pf := loadGoRegularFont(t)
	opf, ok := pf.(*ownParsedFont)
	if !ok {
		t.Fatal("test font is not *ownParsedFont")
	}
	gid := pf.GlyphIndex('A')
	if gid == 0 {
		t.Fatal("test font lacks glyph for 'A'")
	}
	tables := make(map[string][]byte, len(opf.tables)+2)
	for k, v := range opf.tables {
		tables[k] = v
	}
	tables["COLR"] = syntheticCOLR(gid)
	tables["CPAL"] = syntheticCPAL()
	colorFont := &ownParsedFont{rawData: opf.rawData, tables: tables, upem: opf.upem, numGlyphs: opf.numGlyphs}

	cf, ok := any(colorFont).(ColorFont)
	if !ok {
		t.Fatal("ownParsedFont does not implement ColorFont (zero implementers)")
	}
	if !cf.HasColorTables() {
		t.Fatal("HasColorTables() = false with COLR/CPAL present, want true")
	}
	if got := cf.GlyphType(gid); got != GlyphTypeCOLR {
		t.Errorf("GlyphType(color gid) = %v, want COLR", got)
	}
	if got := cf.GlyphType(pf.GlyphIndex('B')); got != GlyphTypeOutline {
		t.Errorf("GlyphType(plain gid) = %v, want Outline", got)
	}
	if got := DetectGlyphType(colorFont, gid); got != GlyphTypeCOLR {
		t.Errorf("DetectGlyphType(color gid) = %v, want COLR", got)
	}
}

func TestColorAdapter_SyntheticCBDT(t *testing.T) {
	pf := loadGoRegularFont(t)
	opf, ok := pf.(*ownParsedFont)
	if !ok {
		t.Fatal("test font is not *ownParsedFont")
	}
	gid := pf.GlyphIndex('A')
	if gid == 0 {
		t.Fatal("test font lacks glyph for 'A'")
	}
	tables := make(map[string][]byte, len(opf.tables)+2)
	for k, v := range opf.tables {
		tables[k] = v
	}
	tables["CBDT"] = syntheticCBDT()
	tables["CBLC"] = syntheticCBLC(gid)
	colorFont := &ownParsedFont{rawData: opf.rawData, tables: tables, upem: opf.upem, numGlyphs: opf.numGlyphs}

	cf, ok := any(colorFont).(ColorFont)
	if !ok {
		t.Fatal("ownParsedFont does not implement ColorFont (zero implementers)")
	}
	if !cf.HasColorTables() {
		t.Fatal("HasColorTables() = false with CBDT/CBLC present, want true")
	}
	if got := cf.GlyphType(gid); got != GlyphTypeBitmap {
		t.Errorf("GlyphType(bitmap gid) = %v, want Bitmap", got)
	}
	if got := cf.GlyphType(pf.GlyphIndex('B')); got != GlyphTypeOutline {
		t.Errorf("GlyphType(plain gid) = %v, want Outline", got)
	}
	bmp, err := cf.BitmapGlyph(gid, 16)
	if err != nil {
		t.Fatalf("BitmapGlyph() error = %v", err)
	}
	if bmp.GlyphID != gid {
		t.Errorf("BitmapGlyph().GlyphID = %d, want %d", bmp.GlyphID, gid)
	}
}

func syntheticCOLR(gid uint16) []byte {
	buf := make([]byte, 24)
	binary.BigEndian.PutUint16(buf[0:2], 0)
	binary.BigEndian.PutUint16(buf[2:4], 1)
	binary.BigEndian.PutUint32(buf[4:8], 14)
	binary.BigEndian.PutUint32(buf[8:12], 20)
	binary.BigEndian.PutUint16(buf[12:14], 1)
	binary.BigEndian.PutUint16(buf[14:16], gid)
	binary.BigEndian.PutUint16(buf[16:18], 0)
	binary.BigEndian.PutUint16(buf[18:20], 1)
	binary.BigEndian.PutUint16(buf[20:22], gid)
	binary.BigEndian.PutUint16(buf[22:24], 0)
	return buf
}

func syntheticCPAL() []byte {
	buf := make([]byte, 18)
	binary.BigEndian.PutUint16(buf[0:2], 0)
	binary.BigEndian.PutUint16(buf[2:4], 1)
	binary.BigEndian.PutUint16(buf[4:6], 1)
	binary.BigEndian.PutUint32(buf[8:12], 14)
	binary.BigEndian.PutUint16(buf[12:14], 0)
	buf[14], buf[15], buf[16], buf[17] = 0, 0, 255, 255
	return buf
}

func syntheticCBLC(gid uint16) []byte {
	buf := make([]byte, 84)
	binary.BigEndian.PutUint16(buf[0:2], 3)
	binary.BigEndian.PutUint16(buf[2:4], 0)
	binary.BigEndian.PutUint32(buf[4:8], 1)
	binary.BigEndian.PutUint32(buf[8:12], 56)
	binary.BigEndian.PutUint32(buf[12:16], 28)
	binary.BigEndian.PutUint32(buf[16:20], 1)
	binary.BigEndian.PutUint16(buf[48:50], gid)
	binary.BigEndian.PutUint16(buf[50:52], gid)
	buf[52], buf[53], buf[54] = 16, 16, 32
	binary.BigEndian.PutUint16(buf[56:58], gid)
	binary.BigEndian.PutUint16(buf[58:60], gid)
	binary.BigEndian.PutUint32(buf[60:64], 8)
	binary.BigEndian.PutUint16(buf[64:66], 2)
	binary.BigEndian.PutUint16(buf[66:68], 19)
	binary.BigEndian.PutUint32(buf[68:72], 0)
	binary.BigEndian.PutUint32(buf[72:76], uint32(4+len(syntheticPNG())))
	buf[76], buf[77] = 8, 8
	return buf
}

func syntheticPNG() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, G: uint8(x * 32), B: uint8(y * 32), A: 255})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		panic(err)
	}
	return pngBuf.Bytes()
}

func syntheticCBDT() []byte {
	data := syntheticPNG()
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(data)))
	copy(buf[4:], data)
	return buf
}
