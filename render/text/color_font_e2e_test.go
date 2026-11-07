package text

import (
	"image/color"
	"testing"
)

// The generated color fonts (render/text/testdata/mkcolorfont) exercise the
// full pipeline against real font files: table splice → GlyphType →
// ColorRasterCache flatten → RGBA pixels.
func loadP38ColorFont(t *testing.T, name string) ParsedFont {
	t.Helper()
	src, err := NewFontSourceFromFile("testdata/" + name)
	if err != nil {
		t.Skipf("generated color font missing (%v); run go run ./render/text/testdata/mkcolorfont", err)
	}
	return src.Parsed()
}

func TestP38ColorFonts_E2ERaster(t *testing.T) {
	cache := NewColorRasterCache(16)
	fg := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	t.Run("sbix", func(t *testing.T) {
		font := loadP38ColorFont(t, "p38_color_sbix.ttf")
		cf := font.(ColorFont)
		gid := font.GlyphIndex('A')
		if got := cf.GlyphType(gid); got != GlyphTypeBitmap {
			t.Fatalf("GlyphType = %v, want Bitmap", got)
		}
		img, err := cache.Image(font, gid, 64, 0, fg)
		if err != nil {
			t.Fatalf("raster error = %v", err)
		}
		assertNonEmptyInk(t, img)
		// Red bitmap with a green orientation marker at its top-left.
		sampleNear(t, img, img.Pix.Bounds().Dx()/2, img.Pix.Bounds().Dy()/2, 220, 40, 40)
		sampleNear(t, img, 2, 2, 60, 220, 60)
	})

	t.Run("cbdt", func(t *testing.T) {
		font := loadP38ColorFont(t, "p38_color_cbdt.ttf")
		cf := font.(ColorFont)
		gid := font.GlyphIndex('A')
		if got := cf.GlyphType(gid); got != GlyphTypeBitmap {
			t.Fatalf("GlyphType = %v, want Bitmap", got)
		}
		img, err := cache.Image(font, gid, 64, 0, fg)
		if err != nil {
			t.Fatalf("raster error = %v", err)
		}
		assertNonEmptyInk(t, img)
		sampleNear(t, img, img.Pix.Bounds().Dx()/2, img.Pix.Bounds().Dy()/2, 40, 80, 220)
	})

	t.Run("svg", func(t *testing.T) {
		font := loadP38ColorFont(t, "p38_color_svg.ttf")
		cf := font.(ColorFont)
		gid := font.GlyphIndex('A')
		if got := cf.GlyphType(gid); got != GlyphTypeSVG {
			t.Fatalf("GlyphType = %v, want SVG", got)
		}
		// Without the render-layer renderer wired (text tests don't import
		// render), SVG raster fails closed — the render/svg package covers
		// the wired path (svg color_glyph tests + real window).
		if _, err := cache.Image(font, gid, 64, 0, fg); err == nil {
			t.Error("SVG raster without renderer: error = nil, want fail-closed error")
		}
		g, err := cf.SVGGlyph(gid)
		if err != nil {
			t.Fatalf("SVGGlyph error = %v", err)
		}
		if g.UsesPalette || g.UsesCurrentColor {
			t.Errorf("'A' doc should be self-colored, got currentColor=%v palette=%v", g.UsesCurrentColor, g.UsesPalette)
		}
		gB, err := cf.SVGGlyph(font.GlyphIndex('B'))
		if err != nil {
			t.Fatalf("SVGGlyph(B) error = %v", err)
		}
		if !gB.UsesCurrentColor || !gB.UsesPalette {
			t.Errorf("'B' doc should use currentColor+palette, got %v/%v", gB.UsesCurrentColor, gB.UsesPalette)
		}
		if got := cf.Palette(0); got != nil {
			t.Errorf("SVG font Palette = %v, want nil (no CPAL)", got)
		}
	})

	t.Run("colrv0", func(t *testing.T) {
		font := loadP38ColorFont(t, "p38_color_colrv0.ttf")
		cf := font.(ColorFont)
		gid := font.GlyphIndex('A')
		if got := cf.GlyphType(gid); got != GlyphTypeCOLR {
			t.Fatalf("GlyphType = %v, want COLR", got)
		}
		img, err := cache.Image(font, gid, 64, 0, fg)
		if err != nil {
			t.Fatalf("raster error = %v", err)
		}
		assertNonEmptyInk(t, img)
		if len(cf.Palette(0)) != 4 {
			t.Errorf("Palette(0) = %d entries, want 4", len(cf.Palette(0)))
		}
	})

	t.Run("colrv1", func(t *testing.T) {
		font := loadP38ColorFont(t, "p38_color_colrv1.ttf")
		cf := font.(ColorFont)
		gid := font.GlyphIndex('A')
		if got := cf.GlyphType(gid); got != GlyphTypeCOLR {
			t.Fatalf("GlyphType = %v, want COLR", got)
		}
		// Without the render-layer COLRv1 renderer wired, raster fails
		// closed; the paint graph itself is covered by the emoji parser
		// tests and the render package's end-to-end tests.
		if _, err := cache.Image(font, gid, 64, 0, fg); err == nil {
			t.Error("COLRv1 raster without renderer: error = nil, want fail-closed error")
		}
		cg, err := cf.COLRGlyph(gid, 0)
		if err != nil {
			t.Fatalf("COLRGlyph error = %v", err)
		}
		if cg.Paint == nil {
			t.Fatal("COLRGlyph.Paint = nil, want v1 paint graph")
		}
		if cg.Clip == nil {
			t.Error("COLRGlyph.Clip = nil, want ClipList box")
		}
	})
}

// assertNonEmptyInk checks the raster has non-transparent pixels.
func assertNonEmptyInk(t *testing.T, img *ColorRasterImage) {
	t.Helper()
	b := img.Pix.Bounds()
	for y := 0; y < b.Dy(); y++ {
		row := img.Pix.Pix[y*img.Pix.Stride:]
		for x := 0; x < b.Dx(); x++ {
			if row[x*4+3] > 0 {
				return
			}
		}
	}
	t.Fatal("raster is fully transparent")
}

// sampleNear checks the most-opaque pixel in a small neighborhood around
// (x, y) is inked and matches the expected RGB (tolerance for bitmap
// resampling and alpha-premultiply rounding).
func sampleNear(t *testing.T, img *ColorRasterImage, x, y int, r, g, b byte) {
	t.Helper()
	bounds := img.Pix.Bounds()
	bestA := 0
	var bestRGB [3]byte
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			px, py := x+dx, y+dy
			if px < bounds.Min.X || py < bounds.Min.Y || px >= bounds.Max.X || py >= bounds.Max.Y {
				continue
			}
			off := (py-bounds.Min.Y)*img.Pix.Stride + (px-bounds.Min.X)*4
			a := int(img.Pix.Pix[off+3])
			if a > bestA {
				bestA = a
				bestRGB = [3]byte{img.Pix.Pix[off], img.Pix.Pix[off+1], img.Pix.Pix[off+2]}
			}
		}
	}
	if bestA < 200 {
		t.Fatalf("sample near (%d,%d): max alpha %d, want inked pixel", x, y, bestA)
	}
	tol := 40
	if diff(byte(bestRGB[0]), r) > tol || diff(byte(bestRGB[1]), g) > tol || diff(byte(bestRGB[2]), b) > tol {
		t.Fatalf("sample near (%d,%d): rgb=(%d,%d,%d), want (~%d,%d,%d)", x, y, bestRGB[0], bestRGB[1], bestRGB[2], r, g, b)
	}
}

func diff(a, b byte) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}
