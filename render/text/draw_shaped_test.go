package text

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
)

// inkRight returns the rightmost inked column (any alpha > 0) in dst.
func inkRight(dst *image.RGBA) int {
	right := -1
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			if _, _, _, a := dst.At(x, y).RGBA(); a != 0 {
				if x > right {
					right = x
				}
			}
		}
	}
	return right
}

// TestDrawShapedFfiPins pins roster #2 (A1 completion): text.Draw (the CPU
// bitmap path) must lay out text through the shaper exactly like the GPU
// mask path does. Oracle: a reference composite built IN THIS TEST from
// ShapeForDraw glyphs — same rasterizer (rasterizeHintedGlyph), same shaped
// positions — drawn pixel for pixel. Pre-fix Draw walked the cmap iterator
// (3 glyphs for "ffi"), which differs from the shaped composite (1 ffi
// GSUB ligature glyph).
func TestDrawShapedFfiPins(t *testing.T) {
	source, err := NewFontSourceFromFile(testFontPath(t))
	if err != nil {
		t.Fatalf("load font: %v", err)
	}
	defer func() { _ = source.Close() }()
	sf := source.Face(24).(*sourceFace)
	const origin = 20.0

	shaped := ShapeForDraw("ffi", sf)
	if len(shaped) == 0 {
		t.Skip("shaper unavailable/empty")
	}

	// Reference: composite the shaped glyphs with the same rasterizer.
	ref := image.NewRGBA(image.Rect(0, 0, 300, 60))
	black := image.NewUniform(color.Black)
	rast := NewGlyphMaskRasterizer()
	parsed := sf.source.Parsed()
	for _, g := range shaped {
		if g.GID == 0 {
			continue
		}
		glyphX := origin + g.X
		intX := math.Floor(glyphX)
		res, err := rasterizeHintedGlyph(rast, parsed, g.GID, sf.size, glyphX-intX, 0, sf.config.hinting, sf.Embolden())
		if err != nil || res == nil {
			continue
		}
		mask := &image.Alpha{Pix: res.Mask, Stride: res.Width, Rect: image.Rect(0, 0, res.Width, res.Height)}
		dstX := int(intX) + int(math.Round(float64(res.BearingX)))
		dstY := 40 - int(math.Round(float64(res.BearingY)))
		draw.DrawMask(ref, image.Rect(dstX, dstY, dstX+res.Width, dstY+res.Height), black, image.Point{}, mask, image.Point{}, draw.Over)
	}

	// Actual: the CPU bitmap draw path.
	got := image.NewRGBA(ref.Bounds())
	Draw(got, "ffi", sf, origin, 40, color.Black)

	// Pixel parity (identical rasterizer + positions → bitwise; allow the
	// origin rounding to differ by nothing at all).
	diff := 0
	for y := 0; y < ref.Bounds().Dy(); y++ {
		for x := 0; x < ref.Bounds().Dx(); x++ {
			r1, g1, b1, a1 := ref.At(x, y).RGBA()
			r2, g2, b2, a2 := got.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
			}
		}
	}
	if diff > 0 {
		t.Fatalf("CPU draw differs from the shaped composite in %d pixels — drawGlyphs is not walking the shaped run (cmap-only layout)", diff)
	}
}
