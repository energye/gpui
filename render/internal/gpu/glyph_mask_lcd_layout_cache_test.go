//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestGlyphMaskLCDLayoutKeepsCachedMasks guards the mixed gray+LCD frame bug:
// SetLCDLayout used to clear the whole glyph mask atlas. Masks already queued
// for the current frame (grayscale batches laid out before an LCD draw) were
// erased, then the LCD glyphs re-allocated from (0,0) and overwrote them. The
// grayscale quads sampled LCD texels and rendered as compressed garbage.
//
// The key already carries LCD/BGR flags, so a layout switch must only drop the
// layout template cache — atlas masks stay valid and addressable.
func TestGlyphMaskLCDLayoutKeepsCachedMasks(t *testing.T) {
	src, err := text.NewFontSourceFromFile("../../text/testdata/source-sans/OTF/SourceSans3-Regular.otf")
	if err != nil {
		t.Skipf("test font unavailable: %v", err)
	}
	defer src.Close()
	face := src.Face(20)

	engine := NewGlyphMaskEngine()
	color := render.RGBA{A: 1}
	identity := render.Identity()

	gray, err := engine.LayoutText(face, "AgHij", 12, 50, color, identity, 1)
	if err != nil {
		t.Fatalf("gray LayoutText: %v", err)
	}
	if len(gray.Quads) == 0 || gray.IsLCD {
		t.Fatalf("gray batch quads=%d isLCD=%v, want grayscale quads", len(gray.Quads), gray.IsLCD)
	}
	grayRect := quadAtlasRect(t, gray.Quads[0], int(engine.Atlas().Config().Size))
	grayChecksum := atlasRectChecksum(t, engine, gray.Quads[0].Page, grayRect)

	engine.SetLCDLayout(text.LCDLayoutRGB)
	rgb, err := engine.LayoutText(face, "AgHij", 12, 50, color, identity, 1)
	if err != nil {
		t.Fatalf("RGB LayoutText: %v", err)
	}
	if len(rgb.Quads) == 0 || !rgb.IsLCD {
		t.Fatalf("RGB batch quads=%d isLCD=%v, want LCD quads", len(rgb.Quads), rgb.IsLCD)
	}
	if got := atlasRectChecksum(t, engine, gray.Quads[0].Page, grayRect); got != grayChecksum {
		t.Fatalf("grayscale atlas region was overwritten by the RGB glyphs (checksum %#x -> %#x)", grayChecksum, got)
	}
	rgbRect := quadAtlasRect(t, rgb.Quads[0], int(engine.Atlas().Config().Size))
	if atlasRectsOverlap(grayRect, rgbRect) {
		t.Fatalf("RGB glyph reused the grayscale atlas rect: gray=%v rgb=%v", grayRect, rgbRect)
	}
	rgbChecksum := atlasRectChecksum(t, engine, rgb.Quads[0].Page, rgbRect)

	engine.SetLCDLayout(text.LCDLayoutBGR)
	bgr, err := engine.LayoutText(face, "AgHij", 12, 50, color, identity, 1)
	if err != nil {
		t.Fatalf("BGR LayoutText: %v", err)
	}
	if len(bgr.Quads) == 0 || !bgr.IsLCD {
		t.Fatalf("BGR batch quads=%d isLCD=%v, want LCD quads", len(bgr.Quads), bgr.IsLCD)
	}
	if got := atlasRectChecksum(t, engine, rgb.Quads[0].Page, rgbRect); got != rgbChecksum {
		t.Fatalf("RGB atlas region was overwritten by the BGR glyphs (checksum %#x -> %#x)", rgbChecksum, got)
	}
	if got := atlasRectChecksum(t, engine, gray.Quads[0].Page, grayRect); got != grayChecksum {
		t.Fatalf("grayscale atlas region was overwritten by the BGR glyphs (checksum %#x -> %#x)", grayChecksum, got)
	}

	// Switching back to grayscale must reuse the cached mask, not clear or
	// re-allocate the atlas (same page + UV origin).
	engine.SetLCDLayout(text.LCDLayoutNone)
	again, err := engine.LayoutText(face, "AgHij", 12, 50, color, identity, 1)
	if err != nil {
		t.Fatalf("gray LayoutText after layout round-trip: %v", err)
	}
	if len(again.Quads) == 0 {
		t.Fatal("gray batch empty after layout round-trip")
	}
	if again.Quads[0].Page != gray.Quads[0].Page ||
		math.Abs(float64(again.Quads[0].U0-gray.Quads[0].U0)) > 1e-6 ||
		math.Abs(float64(again.Quads[0].V0-gray.Quads[0].V0)) > 1e-6 {
		t.Fatalf("grayscale mask was not reused: first page=%d uv=(%v,%v), again page=%d uv=(%v,%v)",
			gray.Quads[0].Page, gray.Quads[0].U0, gray.Quads[0].V0,
			again.Quads[0].Page, again.Quads[0].U0, again.Quads[0].V0)
	}
	if got := atlasRectChecksum(t, engine, gray.Quads[0].Page, grayRect); got != grayChecksum {
		t.Fatalf("grayscale atlas region changed after layout round-trip (checksum %#x -> %#x)", grayChecksum, got)
	}
}

type atlasRect struct{ x, y, w, h int }

func quadAtlasRect(t *testing.T, q GlyphMaskQuad, size int) atlasRect {
	t.Helper()
	// store() writes U0 = x/size + 0.5/size and U1 = (x+w)/size - 0.5/size.
	x0 := int(math.Round(float64(q.U0)*float64(size) - 0.5))
	y0 := int(math.Round(float64(q.V0)*float64(size) - 0.5))
	x1 := int(math.Round(float64(q.U1)*float64(size) + 0.5))
	y1 := int(math.Round(float64(q.V1)*float64(size) + 0.5))
	r := atlasRect{x: x0, y: y0, w: x1 - x0, h: y1 - y0}
	if r.w <= 0 || r.h <= 0 || r.x < 0 || r.y < 0 || x1 > size || y1 > size {
		t.Fatalf("invalid atlas rect %+v from quad uv=(%v,%v)-(%v,%v) size=%d", r, q.U0, q.V0, q.U1, q.V1, size)
	}
	return r
}

func atlasRectsOverlap(a, b atlasRect) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

func atlasRectChecksum(t *testing.T, engine *GlyphMaskEngine, page int, r atlasRect) uint64 {
	t.Helper()
	data, size, _ := engine.Atlas().PageR8Data(page)
	if data == nil {
		t.Fatalf("atlas page %d unavailable", page)
	}
	var h uint64 = 1469598103934665603
	for y := r.y; y < r.y+r.h; y++ {
		row := data[y*size+r.x : y*size+r.x+r.w]
		for _, b := range row {
			h ^= uint64(b)
			h *= 1099511628211
		}
	}
	return h
}
