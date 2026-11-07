package svg

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render/text/emoji"
)

// tableGlyphDoc is a two-glyph SVG-table document (font units, y down:
// glyph art sits at negative y above the baseline).
const tableGlyphDoc = `<svg xmlns="http://www.w3.org/2000/svg">` +
	`<g id="glyph1"><rect x="0" y="-500" width="500" height="500" fill="#ff0000"/></g>` +
	`<g id="glyph2"><rect x="0" y="-500" width="500" height="500" fill="#0000ff"/></g>` +
	`</svg>`

// rasterAt samples the premultiplied RGBA pixel at (x, y) of the image.
func rasterAt(t *testing.T, img interface{ At(x, y int) color.Color }, x, y int) color.Color {
	t.Helper()
	return img.At(x, y)
}

func TestSVGTableGlyph_SelectsGlyphElement(t *testing.T) {
	// upem=1000, ppem=100 → scale 0.1; pen at pixel (100, 200) in the 300²
	// window. Ink of glyph1: design (0,-500)-(500,0) → pixel (100,150)-(150,200).
	img, ox, oy, err := renderTableGlyphRGBA(1, []byte(tableGlyphDoc), 1000, 100, color.RGBA{R: 0, G: 0, B: 0, A: 255}, nil)
	if err != nil {
		t.Fatalf("renderTableGlyphRGBA() error = %v", err)
	}
	if ox != 0 || oy != 50 {
		t.Errorf("origin = (%v, %v), want (0, 50)", ox, oy)
	}
	c := rasterAt(t, img, 25, 25) // cropped center
	r, g, b, a := c.RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("glyph1 center = (%d,%d,%d,%d), want opaque red", r>>8, g>>8, b>>8, a>>8)
	}
	if img.Bounds().Dx() != 50 || img.Bounds().Dy() != 50 {
		t.Errorf("cropped size = %v, want 50x50", img.Bounds())
	}

	img2, _, _, err := renderTableGlyphRGBA(2, []byte(tableGlyphDoc), 1000, 100, color.RGBA{R: 0, G: 0, B: 0, A: 255}, nil)
	if err != nil {
		t.Fatalf("renderTableGlyphRGBA(2) error = %v", err)
	}
	var r2, g2, b2, _ uint32
	r2, g2, b2, _ = rasterAt(t, img2, 25, 25).RGBA()
	if r2>>8 != 0 || g2>>8 != 0 || b2>>8 != 255 {
		t.Errorf("glyph2 center = (%d,%d,%d), want opaque blue", r2>>8, g2>>8, b2>>8)
	}
}

func TestSVGTableGlyph_CurrentColor(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg">` +
		`<g id="glyph1"><rect x="0" y="-500" width="500" height="500" fill="currentColor"/></g>` +
		`</svg>`
	img, _, _, err := renderTableGlyphRGBA(1, []byte(doc), 1000, 100, color.RGBA{R: 0, G: 128, B: 0, A: 255}, nil)
	if err != nil {
		t.Fatalf("renderTableGlyphRGBA() error = %v", err)
	}
	r, g, b, _ := rasterAt(t, img, 25, 25).RGBA()
	if r>>8 != 0 || g>>8 != 128 || b>>8 != 0 {
		t.Errorf("currentColor glyph = (%d,%d,%d), want green (foreground)", r>>8, g>>8, b>>8)
	}
}

func TestSVGTableGlyph_PaletteVars(t *testing.T) {
	doc := `<svg xmlns="http://www.w3.org/2000/svg">` +
		`<g id="glyph1"><rect x="0" y="-500" width="500" height="500" fill="var(--color0, black)"/></g>` +
		`<g id="glyph2"><rect x="0" y="-500" width="500" height="500" fill="var(--color7, orange)"/></g>` +
		`</svg>`
	pal := []emoji.Color{{R: 1, G: 2, B: 3, A: 255}} // --color0 only

	img, _, _, err := renderTableGlyphRGBA(1, []byte(doc), 1000, 100, color.RGBA{}, pal)
	if err != nil {
		t.Fatalf("renderTableGlyphRGBA() error = %v", err)
	}
	r, g, b, _ := rasterAt(t, img, 25, 25).RGBA()
	if r>>8 != 1 || g>>8 != 2 || b>>8 != 3 {
		t.Errorf("var(--color0) glyph = (%d,%d,%d), want palette color (1,2,3)", r>>8, g>>8, b>>8)
	}

	// --color7 missing from the palette → the fallback applies.
	img2, _, _, err := renderTableGlyphRGBA(2, []byte(doc), 1000, 100, color.RGBA{}, pal)
	if err != nil {
		t.Fatalf("renderTableGlyphRGBA(2) error = %v", err)
	}
	r, g, b, _ = rasterAt(t, img2, 25, 25).RGBA()
	if r>>8 != 255 || g>>8 != 165 || b>>8 != 0 {
		t.Errorf("var fallback glyph = (%d,%d,%d), want orange", r>>8, g>>8, b>>8)
	}
}

func TestSVGTableGlyph_MissingElement(t *testing.T) {
	if _, _, _, err := renderTableGlyphRGBA(9, []byte(tableGlyphDoc), 1000, 100, color.RGBA{}, nil); err == nil {
		t.Error("missing glyph9 element: error = nil, want error")
	}
}
