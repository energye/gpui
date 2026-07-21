package text

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func loadCFFTestFont(t *testing.T) *ownParsedFont {
	t.Helper()
	path := filepath.Join("testdata", "CFFTest.otf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read CFFTest.otf: %v", err)
	}
	parsed, err := (&ownParser{}).Parse(data)
	if err != nil {
		t.Fatalf("parse CFFTest.otf: %v", err)
	}
	own, ok := parsed.(*ownParsedFont)
	if !ok {
		t.Fatalf("expected *ownParsedFont, got %T", parsed)
	}
	if !own.hasCFFTable() {
		t.Fatal("CFFTest.otf should expose CFF table without glyf")
	}
	return own
}

func TestCFF_OutlineAndBounds_CFFTest(t *testing.T) {
	own := loadCFFTestFont(t)
	const ppem = 32.0

	// CFFTest maps '0' and U+4E2D ('中'); see x/image font testdata README.
	for _, r := range []rune{'0', '中'} {
		gid := own.GlyphIndex(r)
		if gid == 0 && r != 0 {
			// gid 0 is .notdef; CFFTest uses gid 0 for unmapped Latin — skip if missing.
			if r != '0' {
				t.Fatalf("glyph index for %q is 0", r)
			}
		}
		if r == '0' && gid == 0 {
			// probe earlier showed '0' -> gid 1
			t.Fatalf("expected non-zero gid for '0', got 0")
		}

		bounds := own.GlyphBounds(gid, ppem)
		if bounds == (Rect{}) {
			t.Fatalf("GlyphBounds(%q gid=%d) is zero — CFF bounds not wired", r, gid)
		}
		if bounds.MaxX <= bounds.MinX || bounds.MaxY <= bounds.MinY {
			t.Fatalf("GlyphBounds(%q)=%+v invalid", r, bounds)
		}

		ext := NewOutlineExtractor()
		outline, err := ext.ExtractOutline(own, GlyphID(gid), ppem)
		if err != nil {
			t.Fatalf("ExtractOutline(%q): %v", r, err)
		}
		if outline == nil || len(outline.Segments) == 0 {
			t.Fatalf("ExtractOutline(%q): empty outline — would not paint", r)
		}
		if outline.Advance <= 0 {
			t.Fatalf("ExtractOutline(%q): advance=%v", r, outline.Advance)
		}

		// Rasterize: at least one opaque pixel.
		rast := NewGlyphMaskRasterizer()
		mask, err := rast.Rasterize(own, GlyphID(gid), ppem, 0, 0)
		if err != nil {
			t.Fatalf("Rasterize(%q): %v", r, err)
		}
		if mask == nil || len(mask.Mask) == 0 {
			t.Fatalf("Rasterize(%q): nil/empty mask", r)
		}
		if !maskBytesHaveInk(mask.Mask) {
			t.Fatalf("Rasterize(%q): mask has no ink", r)
		}
		t.Logf("%q gid=%d segs=%d bounds=%+v advance=%.2f mask=%dx%d",
			r, gid, len(outline.Segments), outline.Bounds, outline.Advance,
			mask.Width, mask.Height)
	}
}

func TestCFF_SystemOTF_NimbusSans(t *testing.T) {
	path := "/usr/share/fonts/opentype/urw-base35/NimbusSans-Regular.otf"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("system CFF font not available: %v", err)
	}
	src, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("NewFontSource: %v", err)
	}
	own, ok := src.Parsed().(*ownParsedFont)
	if !ok {
		t.Fatalf("expected *ownParsedFont")
	}
	if !own.hasCFFTable() {
		t.Fatal("NimbusSans-Regular.otf should be CFF")
	}

	const ppem = 48.0
	gid := own.GlyphIndex('A')
	if gid == 0 {
		t.Fatal("glyph 'A' missing")
	}
	bounds := own.GlyphBounds(gid, ppem)
	if bounds == (Rect{}) {
		t.Fatal("CFF GlyphBounds still zero")
	}
	outline, err := NewOutlineExtractor().ExtractOutline(own, GlyphID(gid), ppem)
	if err != nil {
		t.Fatal(err)
	}
	if outline == nil || len(outline.Segments) == 0 {
		t.Fatal("empty CFF outline for 'A'")
	}
	rast := NewGlyphMaskRasterizer()
	mask, err := rast.Rasterize(own, GlyphID(gid), ppem, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if mask == nil || !maskBytesHaveInk(mask.Mask) {
		t.Fatal("Nimbus 'A' mask has no ink")
	}
}

func TestCFF_NotoSansCJK_TTC(t *testing.T) {
	path := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("NotoSansCJK not available: %v", err)
	}
	src, err := NewFontSource(data, WithCollectionIndex(0))
	if err != nil {
		t.Fatalf("NewFontSource TTC: %v", err)
	}
	own, ok := src.Parsed().(*ownParsedFont)
	if !ok {
		t.Fatalf("expected *ownParsedFont")
	}
	if !own.hasCFFTable() {
		t.Fatal("NotoSansCJK face 0 should be CFF OTTO")
	}

	const ppem = 48.0
	gid := own.GlyphIndex('中')
	if gid == 0 {
		t.Fatal("glyph 中 missing in NotoSansCJK")
	}
	bounds := own.GlyphBounds(gid, ppem)
	if bounds == (Rect{}) {
		t.Fatal("CFF GlyphBounds for 中 is zero")
	}
	outline, err := NewOutlineExtractor().ExtractOutline(own, GlyphID(gid), ppem)
	if err != nil {
		t.Fatalf("ExtractOutline 中: %v", err)
	}
	if outline == nil || len(outline.Segments) == 0 {
		t.Fatal("empty outline for 中 — Noto CJK still blank")
	}
	rast := NewGlyphMaskRasterizer()
	mask, err := rast.Rasterize(own, GlyphID(gid), ppem, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if mask == nil || !maskBytesHaveInk(mask.Mask) {
		t.Fatal("Noto 中 mask has no ink")
	}
	t.Logf("Noto 中 gid=%d segs=%d bounds=%+v", gid, len(outline.Segments), outline.Bounds)
}

func maskBytesHaveInk(mask []byte) bool {
	for _, a := range mask {
		if a > 0 {
			return true
		}
	}
	return false
}

func TestCFF2_DetectedAndRejected(t *testing.T) {
	data := buildMinimalCFF2SFNT()
	parsed, err := (&ownParser{}).Parse(data)
	if err != nil {
		t.Fatalf("parse synthetic CFF2: %v", err)
	}
	own, ok := parsed.(*ownParsedFont)
	if !ok {
		t.Fatalf("type %T", parsed)
	}
	if !own.hasCFF2Table() {
		t.Fatal("expected hasCFF2Table")
	}
	if own.hasCFFTable() {
		t.Fatal("must not claim CFF 1")
	}
	_, err = NewOutlineExtractor().ExtractOutline(own, 0, 16)
	if err == nil {
		t.Fatal("expected CFF2 error for truncated/synthetic table")
	}
	if !errors.Is(err, ErrCFF2Unsupported) && !containsCFF2(err.Error()) {
		t.Fatalf("want ErrCFF2Unsupported, got %v", err)
	}
}

// TestCFF2_OutlineAndRaster_NotoVF loads a real CFF2 VF (default instance) and
// verifies outlines + mask ink — ENGINE_GAPS G1.b CFF2 出字.
func TestCFF2_OutlineAndRaster_NotoVF(t *testing.T) {
	path := filepath.Join("testdata", "NotoSansCJK-VF.abc.otf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	parsed, err := (&ownParser{}).Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	own, ok := parsed.(*ownParsedFont)
	if !ok {
		t.Fatalf("type %T", parsed)
	}
	if !own.hasCFF2Table() || own.hasCFFTable() {
		t.Fatalf("expected CFF2-only font (cff2=%v cff1=%v)", own.hasCFF2Table(), own.hasCFFTable())
	}
	const ppem = 48.0
	// Font is a tiny subset; probe first few gids for non-empty outlines.
	var found bool
	for gid := uint16(0); gid < 8 && int(gid) < own.NumGlyphs(); gid++ {
		outline, err := NewOutlineExtractor().ExtractOutline(own, GlyphID(gid), ppem)
		if err != nil {
			t.Fatalf("ExtractOutline gid=%d: %v", gid, err)
		}
		if outline == nil || len(outline.Segments) == 0 {
			continue
		}
		found = true
		if outline.Bounds == (Rect{}) {
			t.Fatalf("gid %d: empty bounds with segments", gid)
		}
		if outline.Bounds.MaxX <= outline.Bounds.MinX {
			t.Fatalf("gid %d: invalid bounds %+v", gid, outline.Bounds)
		}
		mask, err := NewGlyphMaskRasterizer().Rasterize(own, GlyphID(gid), ppem, 0, 0)
		if err != nil {
			t.Fatalf("Rasterize gid=%d: %v", gid, err)
		}
		if mask == nil || !maskBytesHaveInk(mask.Mask) {
			t.Fatalf("gid %d: no ink in mask", gid)
		}
		// CFF2 Y-down: bounds should span negative Y for typical Latin (baseline-relative).
		t.Logf("gid=%d segs=%d bounds=%+v mask=%dx%d advance=%.2f",
			gid, len(outline.Segments), outline.Bounds, mask.Width, mask.Height, outline.Advance)
		break
	}
	if !found {
		t.Fatal("no non-empty CFF2 glyph outline in first 8 gids")
	}

	// Also via glyph index for 'a' if cmap maps it.
	if gid := own.GlyphIndex('a'); gid != 0 {
		outline, err := NewOutlineExtractor().ExtractOutline(own, GlyphID(gid), ppem)
		if err != nil {
			t.Fatalf("ExtractOutline('a'): %v", err)
		}
		if outline == nil || len(outline.Segments) == 0 {
			t.Fatal("'a' empty outline")
		}
	}
}

func containsCFF2(s string) bool {
	return errors.Is(errors.New(s), ErrCFF2Unsupported) || // never
		(len(s) > 0 && (indexStr(s, "CFF2") >= 0 || indexStr(s, "not yet supported") >= 0))
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// buildMinimalCFF2SFNT creates a tiny OTTO font with head, maxp, CFF2 tables.
func buildMinimalCFF2SFNT() []byte {
	// head table (54 bytes minimum fields we parse: unitsPerEm at 18)
	head := make([]byte, 54)
	// unitsPerEm = 1000 at offset 18
	head[18], head[19] = 0x03, 0xe8
	// indexToLocFormat at 50
	// maxp: version + numGlyphs
	maxp := make([]byte, 6)
	maxp[0], maxp[1], maxp[2], maxp[3] = 0x00, 0x00, 0x50, 0x00 // version 0.5 for CFF
	maxp[4], maxp[5] = 0, 1                                     // 1 glyph
	cff2 := []byte{0x00, 0x02, 0x00, 0x00}                      // dummy CFF2 header major=2

	tables := []struct {
		tag  string
		data []byte
	}{
		{"CFF2", cff2},
		{"head", head},
		{"maxp", maxp},
	}
	// sort tags alphabetically for sfnt? own parser doesn't require order
	// Build directory: OTTO + numTables=3
	numTables := len(tables)
	// offsets: header 12 + 16*numTables, then table data 4-byte aligned
	headerSize := 12 + 16*numTables
	// collect data with padding
	type ent struct {
		tag    string
		data   []byte
		offset int
	}
	ents := make([]ent, numTables)
	off := headerSize
	for i, tb := range tables {
		// align
		for off%4 != 0 {
			off++
		}
		ents[i] = ent{tb.tag, tb.data, off}
		off += len(tb.data)
	}
	out := make([]byte, off)
	// scaler type OTTO
	copy(out[0:4], []byte("OTTO"))
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))
	// searchRange etc. can be 0
	for i, e := range ents {
		rec := 12 + i*16
		copy(out[rec:rec+4], e.tag)
		// pad tag to 4
		if len(e.tag) < 4 {
			// CFF2 is 4
		}
		binary.BigEndian.PutUint32(out[rec+8:rec+12], uint32(e.offset))
		binary.BigEndian.PutUint32(out[rec+12:rec+16], uint32(len(e.data)))
		copy(out[e.offset:], e.data)
	}
	return out
}
