package render

import (
	"os"
	"testing"

	"github.com/energye/gpui/render/text"
)

func TestTextPathVariableFontWeightsUseDifferentOutlines(t *testing.T) {
	const path = "text/testdata/source-sans/VF/SourceSans3VF-Upright.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("variable font fixture missing: %v", err)
	}
	src, err := text.NewFontSourceFromFile(path)
	if err != nil {
		t.Fatalf("load variable font: %v", err)
	}
	defer src.Close()

	dc := NewContext(600, 200)
	defer dc.Close()

	widthAt := func(weight float32) int {
		dc.SetFont(src.Face(30, text.WithVariations(text.NewFontVariation("wght", weight))))
		p := dc.TextPath("Variable", 10, 100)
		if p == nil {
			t.Fatalf("TextPath at wght %.0f returned nil", weight)
		}
		return p.Bounds().Dx()
	}

	light := widthAt(200)
	bold := widthAt(900)
	if light <= 0 || bold <= 0 {
		t.Fatalf("empty TextPath bounds: light=%d bold=%d", light, bold)
	}
	if bold <= light+light/20 {
		t.Fatalf("wght 900 TextPath must be wider than wght 200 (light=%dpx bold=%dpx); "+
			"the glyph cache ignored the face variations", light, bold)
	}
}
