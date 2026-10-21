package text

import (
	"os"
	"testing"
)

// TestShapeVariableFontWeights pins the HarfBuzz face cache: a variable
// FontSource is shared by every weight, so the shaper must use a face built
// with the requested variation coordinates (advances and FeatureVariations),
// not the default master.
func TestShapeVariableFontWeights(t *testing.T) {
	const path = "testdata/source-sans/VF/SourceSans3VF-Upright.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("variable font fixture missing: %v", err)
	}
	src, err := NewFontSourceFromFile(path)
	if err != nil {
		t.Fatalf("load variable font: %v", err)
	}
	defer src.Close()

	shape := func(weight float32) []ShapedGlyph {
		return Shape("Variable", src.Face(30, WithVariations(NewFontVariation("wght", weight))))
	}
	light := shape(200)
	bold := shape(900)
	if len(light) == 0 || len(light) != len(bold) {
		t.Fatalf("unexpected shaped glyph counts: light=%d bold=%d", len(light), len(bold))
	}
	lightAdvance := light[len(light)-1].X
	boldAdvance := bold[len(bold)-1].X
	if boldAdvance <= lightAdvance*1.05 {
		t.Fatalf("Shape ignored wght: last glyph x light=%.2f bold=%.2f", lightAdvance, boldAdvance)
	}
}
