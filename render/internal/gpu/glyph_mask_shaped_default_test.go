//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestLayoutTextShapedByDefault pins the A1 fix: the engine's default string
// path must run HarfBuzz shaping (Skia SkShaper posture — shape, then draw),
// not the cmap+hmtx iteration. Discriminator: GPOS kerning on "AV" — the
// shaped run is narrower than the raw cmap advance sum; pre-fix LayoutText
// laid the string out at the cmap width.
func TestLayoutTextShapedByDefault(t *testing.T) {
	face := glyphMaskTestFont(t, 24)
	// DejaVu GSUB has the standard "fi" ligature: shaping yields ONE glyph,
	// the cmap+hmtx iteration yields TWO. If the font/shaper lacks it the
	// test skips rather than lying.
	const s = "fi"
	shaped := text.Shape(s, face)
	cmap := text.LayoutGlyphs(face, s)
	if len(shaped) == 0 || len(shaped) >= len(cmap) {
		t.Skipf("no fi ligature in this font/shaper (shaped=%d cmap=%d)", len(shaped), len(cmap))
	}

	engine := NewGlyphMaskEngine()
	batch, err := engine.LayoutText(face, s, 0, 0, render.RGBA{A: 1}, render.Identity(), 1)
	if err != nil {
		t.Fatalf("LayoutText: %v", err)
	}
	if len(batch.Quads) != len(shaped) {
		t.Fatalf("LayoutText emitted %d glyph quads for \"fi\"; shaped reference has %d (GSUB ligature), cmap-only path yields %d — the default path is not shaping", len(batch.Quads), len(shaped), len(cmap))
	}
}
