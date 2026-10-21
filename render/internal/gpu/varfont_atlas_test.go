//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// loadVariableFontForAtlasTest loads the tracked Source Sans 3 variable font.
// The file is checked in under render/text/testdata; this package's test
// working directory is render/internal/gpu, hence the relative path.
func loadVariableFontForAtlasTest(t *testing.T) *text.FontSource {
	t.Helper()
	const path = "../../text/testdata/source-sans/VF/SourceSans3VF-Upright.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("variable font fixture missing: %v", err)
	}
	src, err := text.NewFontSourceFromFile(path)
	if err != nil {
		t.Fatalf("load variable font: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })
	return src
}

func variableFace(src *text.FontSource, weight float32) text.Face {
	return src.Face(30, text.WithVariations(text.NewFontVariation("wght", weight)))
}

func glyphMaskBatchWidth(b GlyphMaskBatch) float32 {
	if len(b.Quads) == 0 {
		return 0
	}
	minX, maxX := b.Quads[0].X0, b.Quads[0].X1
	for _, q := range b.Quads[1:] {
		if q.X0 < minX {
			minX = q.X0
		}
		if q.X1 > maxX {
			maxX = q.X1
		}
	}
	return maxX - minX
}

func textBatchWidth(b TextBatch) float32 {
	if len(b.Quads) == 0 {
		return 0
	}
	minX, maxX := b.Quads[0].X0, b.Quads[0].X1
	for _, q := range b.Quads[1:] {
		if q.X0 < minX {
			minX = q.X0
		}
		if q.X1 > maxX {
			maxX = q.X1
		}
	}
	return maxX - minX
}

// TestGlyphMaskVariableFontWeightsUseDifferentOutlines pins the atlas-key and
// rasterization fix: a variable FontSource is shared by every weight, so both
// the GlyphMaskKey fontID and the extracted outline must carry the face's
// variation coordinates. Before the fix all four weights produced identical
// quads/masks (the default master won).
func TestGlyphMaskVariableFontWeightsUseDifferentOutlines(t *testing.T) {
	src := loadVariableFontForAtlasTest(t)
	eng := NewGlyphMaskEngine()
	color := render.RGBA{R: 1, G: 1, B: 1, A: 1}

	light, err := eng.LayoutText(variableFace(src, 200), "Variable", 10, 50, color, render.Identity(), 1)
	if err != nil {
		t.Fatalf("layout light: %v", err)
	}
	lw := glyphMaskBatchWidth(light)
	// LayoutText returns engine-owned quad scratch; copy before the next call.
	lightQuads := append([]GlyphMaskQuad(nil), light.Quads...)
	bold, err := eng.LayoutText(variableFace(src, 900), "Variable", 10, 50, color, render.Identity(), 1)
	if err != nil {
		t.Fatalf("layout bold: %v", err)
	}
	bw := glyphMaskBatchWidth(bold)
	if lw <= 0 || bw <= 0 {
		t.Fatalf("empty glyph mask quads: light=%d bold=%d", len(light.Quads), len(bold.Quads))
	}
	if bw <= lw*1.05 {
		t.Fatalf("wght 900 must emit a wider run than wght 200 (light=%.2fpx bold=%.2fpx); "+
			"variations were ignored by the glyph-mask atlas", lw, bw)
	}
	if len(lightQuads) != len(bold.Quads) {
		// Different advances mutate positions but not the glyph count.
		t.Fatalf("glyph count changed across weights: light=%d bold=%d", len(lightQuads), len(bold.Quads))
	}
	identical := true
	for i := range lightQuads {
		if lightQuads[i] != bold.Quads[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Fatal("wght 200 and wght 900 produced identical quads")
	}
}

// TestMSDFVariableFontWeightsUseDifferentOutlines covers the MSDF atlas path,
// which has its own glyph key and default-master outline extraction.
func TestMSDFVariableFontWeightsUseDifferentOutlines(t *testing.T) {
	src := loadVariableFontForAtlasTest(t)
	eng := NewGPUTextEngine()
	color := render.RGBA{R: 1, G: 1, B: 1, A: 1}

	light, err := eng.LayoutText(variableFace(src, 200), "Variable", 10, 50, color, render.Identity(), 1)
	if err != nil {
		t.Fatalf("layout light: %v", err)
	}
	lw := textBatchWidth(light)
	bold, err := eng.LayoutText(variableFace(src, 900), "Variable", 10, 50, color, render.Identity(), 1)
	if err != nil {
		t.Fatalf("layout bold: %v", err)
	}
	bw := textBatchWidth(bold)
	if lw <= 0 || bw <= 0 {
		t.Fatalf("empty MSDF quads: light=%d bold=%d", len(light.Quads), len(bold.Quads))
	}
	if bw <= lw*1.05 {
		t.Fatalf("wght 900 must emit a wider MSDF run than wght 200 (light=%.2fpx bold=%.2fpx); "+
			"variations were ignored by the MSDF atlas", lw, bw)
	}
}

func TestVariationFontIDDistinct(t *testing.T) {
	base := uint64(0x1234)
	light := variationFontID(base, []text.FontVariation{{Tag: [4]byte{'w', 'g', 'h', 't'}, Value: 200}})
	bold := variationFontID(base, []text.FontVariation{{Tag: [4]byte{'w', 'g', 'h', 't'}, Value: 900}})
	if light == base || bold == base {
		t.Fatalf("variationFontID must fold non-default variations into the font ID: base=%x light=%x bold=%x", base, light, bold)
	}
	if light == bold {
		t.Fatalf("different weights must not collide: light=%x bold=%x", light, bold)
	}
	if got := variationFontID(base, nil); got != base {
		t.Fatalf("empty variations must keep the base font ID: got=%x want=%x", got, base)
	}
}
