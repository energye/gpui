package text

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"strconv"
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

func TestColorAdapter_SyntheticSVGTable(t *testing.T) {
	pf := loadGoRegularFont(t)
	opf, ok := pf.(*ownParsedFont)
	if !ok {
		t.Fatal("test font is not *ownParsedFont")
	}
	gid := pf.GlyphIndex('A')
	if gid == 0 {
		t.Fatal("test font lacks glyph for 'A'")
	}
	tables := make(map[string][]byte, len(opf.tables)+1)
	for k, v := range opf.tables {
		tables[k] = v
	}
	tables["SVG "] = syntheticSVGTable(gid)
	colorFont := &ownParsedFont{rawData: opf.rawData, tables: tables, upem: opf.upem, numGlyphs: opf.numGlyphs}

	cf, ok := any(colorFont).(ColorFont)
	if !ok {
		t.Fatal("ownParsedFont does not implement ColorFont (zero implementers)")
	}
	if !cf.HasColorTables() {
		t.Fatal("HasColorTables() = false with SVG table present, want true")
	}
	if got := cf.GlyphType(gid); got != GlyphTypeSVG {
		t.Errorf("GlyphType(svg gid) = %v, want SVG", got)
	}
	if got := cf.GlyphType(pf.GlyphIndex('B')); got != GlyphTypeOutline {
		t.Errorf("GlyphType(plain gid) = %v, want Outline", got)
	}
	g, err := cf.SVGGlyph(gid)
	if err != nil {
		t.Fatalf("SVGGlyph() error = %v", err)
	}
	if g.GlyphID != gid {
		t.Errorf("SVGGlyph().GlyphID = %d, want %d", g.GlyphID, gid)
	}
	if !bytes.Contains(g.Doc, []byte(`id="glyph`)) {
		t.Errorf("SVGGlyph().Doc missing glyph element: %q", g.Doc)
	}
	if _, err := cf.SVGGlyph(pf.GlyphIndex('B')); err == nil {
		t.Error("SVGGlyph(plain gid) error = nil, want not-found error")
	}
	if got := cf.Palette(0); got != nil {
		t.Errorf("Palette(0) = %v, want nil (no CPAL)", got)
	}

	// Without the render-layer renderer wired, SVG rasterization fails
	// closed (documented not-yet-wired state) instead of guessing.
	cache := NewColorRasterCache(8)
	if _, err := cache.Image(colorFont, gid, 16, 0, color.RGBA{A: 255}); err == nil {
		t.Error("ColorRasterCache.Image(SVG) error = nil without renderer, want error")
	}
}

// syntheticSVGTable builds a minimal "SVG " table with one document record
// covering gid, holding a one-element glyph description.
func syntheticSVGTable(gid uint16) []byte {
	doc := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><g id="glyph` +
		strconv.Itoa(int(gid)) + `"><rect width="500" height="500" fill="red"/></g></svg>`)
	listStart := uint32(10)
	listLen := uint32(2 + 12)
	docOff := listLen // list-relative, non-zero per spec

	buf := make([]byte, int(listStart+listLen)+len(doc))
	binary.BigEndian.PutUint16(buf[0:2], 0) // version 0
	binary.BigEndian.PutUint32(buf[2:6], listStart)
	binary.BigEndian.PutUint16(buf[listStart:listStart+2], 1) // numEntries
	rec := listStart + 2
	binary.BigEndian.PutUint16(buf[rec:rec+2], gid)      // startGlyphID
	binary.BigEndian.PutUint16(buf[rec+2:rec+4], gid)    // endGlyphID
	binary.BigEndian.PutUint32(buf[rec+4:rec+8], docOff) // svgDocOffset
	binary.BigEndian.PutUint32(buf[rec+8:rec+12], uint32(len(doc)))
	copy(buf[listStart+listLen:], doc)
	return buf
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

func TestColorAdapter_SyntheticSbix(t *testing.T) {
	pf := loadGoRegularFont(t)
	opf, ok := pf.(*ownParsedFont)
	if !ok {
		t.Fatal("test font is not *ownParsedFont")
	}
	gid := pf.GlyphIndex('A')
	if gid == 0 {
		t.Fatal("test font lacks glyph for 'A'")
	}
	plainGID := pf.GlyphIndex('B')
	tables := make(map[string][]byte, len(opf.tables)+1)
	for k, v := range opf.tables {
		tables[k] = v
	}
	tables["sbix"] = syntheticSbix(gid, opf.numGlyphs)
	colorFont := &ownParsedFont{rawData: opf.rawData, tables: tables, upem: opf.upem, numGlyphs: opf.numGlyphs}

	cf, ok := any(colorFont).(ColorFont)
	if !ok {
		t.Fatal("ownParsedFont does not implement ColorFont (zero implementers)")
	}
	if !cf.HasColorTables() {
		t.Fatal("HasColorTables() = false with sbix present, want true")
	}
	if got := cf.GlyphType(gid); got != GlyphTypeBitmap {
		t.Errorf("GlyphType(sbix gid) = %v, want Bitmap", got)
	}
	if got := cf.GlyphType(plainGID); got != GlyphTypeOutline {
		t.Errorf("GlyphType(plain gid) = %v, want Outline", got)
	}
	bmp, err := cf.BitmapGlyph(gid, 16)
	if err != nil {
		t.Fatalf("BitmapGlyph() error = %v", err)
	}
	if bmp.GlyphID != gid {
		t.Errorf("BitmapGlyph().GlyphID = %d, want %d", bmp.GlyphID, gid)
	}
	if bmp.PPEM != 16 {
		t.Errorf("BitmapGlyph().PPEM = %d, want 16", bmp.PPEM)
	}
	if bmp.Width != 8 || bmp.Height != 8 {
		t.Errorf("BitmapGlyph() size = %dx%d, want 8x8", bmp.Width, bmp.Height)
	}
	if bmp.OriginX != 2 || bmp.OriginY != 3 {
		t.Errorf("BitmapGlyph() origin = (%v,%v), want (2,3)", bmp.OriginX, bmp.OriginY)
	}
	img, err := bmp.Decode()
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Errorf("decoded bitmap bounds = %v, want 8x8", img.Bounds())
	}
	if _, err := cf.BitmapGlyph(plainGID, 16); err == nil {
		t.Error("BitmapGlyph(plain gid) error = nil, want not-in-bitmap error")
	}
}

// syntheticSbix builds a minimal sbix table: one strike at ppem 16 holding one
// 'png ' glyph entry at gid. Layout per OT spec: version(2) flags(2)
// numStrikes(4) strikeOffsets[numStrikes] → strike: ppem(2) ppi(2)
// glyphOffsetArray[numGlyphs+1].
func syntheticSbix(gid uint16, numGlyphs int) []byte {
	png := syntheticPNG()
	entryLen := 8 + len(png) // originX(2) originY(2) tag(4) data
	header := 8 + 4          // header + one strike offset
	strikeOffsets := uint32(header)
	offsetsEnd := header + 4 + (numGlyphs+1)*4
	entryOff := uint32(offsetsEnd - int(strikeOffsets)) // glyph offset is strike-relative

	buf := make([]byte, offsetsEnd+entryLen)
	binary.BigEndian.PutUint16(buf[0:2], 1) // version
	binary.BigEndian.PutUint32(buf[4:8], 1) // numStrikes
	binary.BigEndian.PutUint32(buf[8:12], strikeOffsets)
	binary.BigEndian.PutUint16(buf[strikeOffsets:strikeOffsets+2], 16) // ppem
	binary.BigEndian.PutUint16(buf[strikeOffsets+2:strikeOffsets+4], 72)
	arr := strikeOffsets + 4
	for i := 0; i <= numGlyphs; i++ {
		var v uint32
		if i == int(gid) {
			v = entryOff
		} else if i == int(gid)+1 {
			v = entryOff + uint32(entryLen)
		}
		binary.BigEndian.PutUint32(buf[int(arr)+i*4:int(arr)+i*4+4], v)
	}
	// Glyph entry: origin offsets + graphic type tag + PNG data.
	pos := offsetsEnd
	binary.BigEndian.PutUint16(buf[pos:pos+2], 2)   // originOffsetX
	binary.BigEndian.PutUint16(buf[pos+2:pos+4], 3) // originOffsetY
	copy(buf[pos+4:pos+8], "png ")
	copy(buf[pos+8:], png)
	return buf
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
