package text

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func squareOutline() *GlyphOutline {
	return &GlyphOutline{
		Segments: []OutlineSegment{
			{Op: OutlineOpMoveTo, Points: [3]OutlinePoint{{X: 0, Y: 0}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 100, Y: 0}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 100, Y: 100}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 0, Y: 100}}},
		},
		Bounds:  Rect{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100},
		Advance: 120,
		LSB:     0,
		GID:     7,
		Type:    GlyphTypeOutline,
	}
}

func TestEmboldenOutline_GrowsSquareKeepsAdvance(t *testing.T) {
	before := squareOutline()
	after := EmboldenOutline(before, 14)
	if after == before {
		t.Fatal("bold outline must be a copy, not the original")
	}
	if after.Advance != before.Advance {
		t.Fatalf("Advance=%v want %v (synthetic bold never reflows)", after.Advance, before.Advance)
	}
	grownX := after.Bounds.Width() - before.Bounds.Width()
	grownY := after.Bounds.Height() - before.Bounds.Height()
	if grownX <= 0 || grownY <= 0 {
		t.Fatalf("bounds did not grow: %.3fx%.3f", grownX, grownY)
	}
	// Standard outline fake bold at 14px is 14/24 per side. A square's
	// corner normals add only ~1.414*strength per axis, always < 2px.
	// The old 1px mask dilation added exactly 2px per axis and was the
	// visible 14/15px blur this test guards against.
	if grownX >= 2 || grownY >= 2 {
		t.Fatalf("14px outline growth %.3fx%.3f >= 2px (mask-dilation regression)", grownX, grownY)
	}
	if after.Bounds.MinX >= before.Bounds.MinX || after.Bounds.MaxX <= before.Bounds.MaxX {
		t.Fatalf("outer square did not expand: %+v", after.Bounds)
	}
	if after.LSB >= before.LSB {
		t.Fatalf("LSB=%v want smaller than %v", after.LSB, before.LSB)
	}
}

func TestEmboldenOutline_HoleShrinks(t *testing.T) {
	outline := &GlyphOutline{
		Segments: []OutlineSegment{
			// Outer contour (positive area in Y-down).
			{Op: OutlineOpMoveTo, Points: [3]OutlinePoint{{X: 0, Y: 0}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 100, Y: 0}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 100, Y: 100}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 0, Y: 100}}},
			// Hole contour (opposite winding).
			{Op: OutlineOpMoveTo, Points: [3]OutlinePoint{{X: 25, Y: 25}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 25, Y: 75}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 75, Y: 75}}},
			{Op: OutlineOpLineTo, Points: [3]OutlinePoint{{X: 75, Y: 25}}},
		},
		Bounds:  Rect{MinX: 0, MinY: 0, MaxX: 100, MaxY: 100},
		Advance: 120,
	}
	after := EmboldenOutline(outline, 14)
	hole := after.Segments[4:8]
	if hole[0].Points[0].X <= 25 || hole[0].Points[0].Y <= 25 {
		t.Fatalf("hole top-left moved outward: %+v", hole[0].Points[0])
	}
	if hole[2].Points[0].X >= 75 || hole[2].Points[0].Y >= 75 {
		t.Fatalf("hole bottom-right moved outward: %+v", hole[2].Points[0])
	}
	if after.Bounds.MinX >= 0 || after.Bounds.MaxX <= 100 {
		t.Fatalf("outer bounds did not expand: %+v", after.Bounds)
	}
}

func TestEmboldenOutline_Real14pxGlyphUsesOutlineStrength(t *testing.T) {
	path := filepath.Join("testdata", "source-sans", "TTF", "SourceSans3-Regular.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Source Sans regular unavailable: %v", err)
	}
	src, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("NewFontSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	parsed := src.Parsed()
	gid := GlyphID(parsed.GlyphIndex('B'))
	plain, err := NewOutlineExtractor().ExtractOutline(parsed, gid, 14)
	if err != nil {
		t.Fatalf("ExtractOutline: %v", err)
	}
	if plain == nil || plain.IsEmpty() {
		t.Fatal("empty B outline")
	}
	bold := EmboldenOutline(plain, 14)
	grownX := bold.Bounds.Width() - plain.Bounds.Width()
	grownY := bold.Bounds.Height() - plain.Bounds.Height()
	limit := 2 * syntheticBoldStrength(14) * 1.05
	if grownX <= 0 || grownY <= 0 || grownX > limit || grownY > limit {
		t.Fatalf("14px B growth=(%.3f,%.3f) want (0,%.3f]", grownX, grownY, limit)
	}
	if bold.Advance != plain.Advance {
		t.Fatalf("advance changed: %v -> %v", plain.Advance, bold.Advance)
	}
}

func TestEmboldenOutline_14And15pxBelowLegacyDilation(t *testing.T) {
	for _, size := range []float64{14, 15} {
		strength := syntheticBoldStrength(size)
		if math.Abs(strength-size/24) > 1e-9 || strength >= 1 {
			t.Fatalf("%gpx strength=%v want size/24 < 1", size, strength)
		}
		before := squareOutline()
		after := EmboldenOutline(before, size)
		if grew := after.Bounds.Width() - before.Bounds.Width(); grew >= 2 {
			t.Fatalf("%gpx grew %.3fpx, legacy 1px mask dilation grew 2px", size, grew)
		}
	}
}

func TestEmboldenOutline_CFFGlyphGrows(t *testing.T) {
	for _, path := range []string{
		filepath.Join("testdata", "CFFTest.otf"),
		filepath.Join("testdata", "source-sans", "OTF", "SourceSans3-Semibold.otf"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("CFF test font %s unavailable: %v", path, err)
		}
		src, err := NewFontSource(data)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		t.Cleanup(func() { _ = src.Close() })
		parsed := src.Parsed()
		gid := GlyphID(parsed.GlyphIndex('H'))
		if gid == 0 {
			gid = GlyphID(parsed.GlyphIndex('A'))
		}
		extractor := NewOutlineExtractor()
		plain, err := extractor.ExtractOutline(parsed, gid, 14)
		if gid == 0 || err != nil || plain == nil || plain.IsEmpty() {
			// Tiny CFF fixtures may only carry a .notdef glyph; scan for
			// any drawable outline instead of assuming Latin coverage.
			plain = nil
			for candidate := GlyphID(1); int(candidate) < parsed.NumGlyphs(); candidate++ {
				outline, oErr := extractor.ExtractOutline(parsed, candidate, 14)
				if oErr == nil && outline != nil && !outline.IsEmpty() {
					gid, plain = candidate, outline
					break
				}
			}
		}
		if plain == nil || plain.IsEmpty() {
			t.Fatalf("%s: no drawable CFF outline", path)
		}
		bold := EmboldenOutline(plain, 14)
		if bold.Bounds.Width() <= plain.Bounds.Width() || bold.Bounds.Height() <= plain.Bounds.Height() {
			t.Fatalf("%s: CFF outline did not grow: plain=%+v bold=%+v", path, plain.Bounds, bold.Bounds)
		}
		if bold.Advance != plain.Advance {
			t.Fatalf("%s: advance changed", path)
		}
	}
}

func inkBytes(mask []byte) int {
	n := 0
	for _, v := range mask {
		if v != 0 {
			n++
		}
	}
	return n
}

func TestRasterizeBoldVariants_GrowWithoutAdvanceChange(t *testing.T) {
	path := filepath.Join("testdata", "source-sans", "TTF", "SourceSans3-Regular.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Source Sans regular unavailable: %v", err)
	}
	src, err := NewFontSource(data)
	if err != nil {
		t.Fatalf("NewFontSource: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	parsed := src.Parsed()
	gid := GlyphID(parsed.GlyphIndex('B'))
	rast := NewGlyphMaskRasterizer()

	plainAA, err := rast.RasterizeHinted(parsed, gid, 14, 0, 0, HintingNone)
	if err != nil {
		t.Fatalf("plain AA: %v", err)
	}
	boldAA, err := rast.RasterizeHintedBold(parsed, gid, 14, 0, 0, HintingNone, true)
	if err != nil {
		t.Fatalf("bold AA: %v", err)
	}
	if plainAA == nil || boldAA == nil {
		t.Fatal("AA variants returned nil")
	}
	if boldAA.Advance != plainAA.Advance {
		t.Fatalf("AA advance %v -> %v", plainAA.Advance, boldAA.Advance)
	}
	if inkBytes(boldAA.Mask) <= inkBytes(plainAA.Mask) {
		t.Fatalf("AA bold ink %d <= plain %d", inkBytes(boldAA.Mask), inkBytes(plainAA.Mask))
	}

	plainAliased, err := rast.RasterizeAliased(parsed, gid, 14, 0, 0, HintingNone)
	if err != nil {
		t.Fatalf("plain aliased: %v", err)
	}
	boldAliased, err := rast.RasterizeAliasedBold(parsed, gid, 14, 0, 0, HintingNone, true)
	if err != nil {
		t.Fatalf("bold aliased: %v", err)
	}
	if plainAliased == nil || boldAliased == nil {
		t.Fatal("aliased variants returned nil")
	}
	if boldAliased.Advance != plainAliased.Advance {
		t.Fatalf("aliased advance %v -> %v", plainAliased.Advance, boldAliased.Advance)
	}
	if inkBytes(boldAliased.Mask) <= inkBytes(plainAliased.Mask) {
		t.Fatalf("aliased bold ink %d <= plain %d", inkBytes(boldAliased.Mask), inkBytes(plainAliased.Mask))
	}

	plainLCD, err := rast.RasterizeLCD(parsed, gid, 14, 0, 0, HintingNone, DefaultLCDFilter(), LCDLayoutRGB)
	if err != nil {
		t.Fatalf("plain LCD: %v", err)
	}
	boldLCD, err := rast.RasterizeLCDBold(parsed, gid, 14, 0, 0, HintingNone, DefaultLCDFilter(), LCDLayoutRGB, true)
	if err != nil {
		t.Fatalf("bold LCD: %v", err)
	}
	if plainLCD == nil || boldLCD == nil {
		t.Fatal("LCD variants returned nil")
	}
	if inkBytes(boldLCD.Mask) <= inkBytes(plainLCD.Mask) {
		t.Fatalf("LCD bold ink %d <= plain %d", inkBytes(boldLCD.Mask), inkBytes(plainLCD.Mask))
	}
}

func TestEmboldenOutline_NilAndEmpty(t *testing.T) {
	if got := EmboldenOutline(nil, 14); got != nil {
		t.Fatal("nil outline must pass through")
	}
	empty := &GlyphOutline{Advance: 10}
	if got := EmboldenOutline(empty, 14); got != empty {
		t.Fatal("empty outline must pass through unchanged")
	}
}
