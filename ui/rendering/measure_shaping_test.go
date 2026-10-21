package rendering_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// TestRenderText_MeasureWidthMatchesShapedLayout pins the zero-spacing
// measure path to the shaped layout (Skia/Flutter measure the shaped
// paragraph). Summing isolated rune advances drops kerning/ligatures and
// makes MeasureWidth disagree with TextLayout for strings like "AVATAR".
func TestRenderText_MeasureWidthMatchesShapedLayout(t *testing.T) {
	face := loadTestFace(t, 20)

	layoutWidth := func(s string) (float64, float64) {
		txt := rendering.NewRenderText(s)
		txt.SetFace(face)
		txt.SetFontSize(20)
		_ = txt.Layout(rendering.Loose(1000, 80))
		lay := txt.TextLayout()
		if lay == nil || lay.LineCount() == 0 {
			t.Fatalf("%q: no layout", s)
		}
		_, _, lw, _, ok := lay.Line(0)
		if !ok {
			t.Fatalf("%q: no line 0", s)
		}
		return txt.MeasureWidth(s), lw
	}

	for _, s := range []string{"AVATAR", "AV", "مرحبا", "Hello gpui WWW"} {
		got, want := layoutWidth(s)
		if math.Abs(got-want) > 1e-6 {
			t.Fatalf("%q MeasureWidth=%.6f layout=%.6f, want equal (shaped)", s, got, want)
		}
	}

	// Prove the path is shaped, not an unshaped rune-advance sum: the
	// kerning pair AV must differ from text.Measure by a visible amount.
	unshaped, _ := text.Measure("AVATAR", face)
	shaped, _ := layoutWidth("AVATAR")
	if math.Abs(shaped-unshaped) < 1 {
		t.Fatalf("AVATAR shaped=%.3f unshaped=%.3f; expected a kerning difference", shaped, unshaped)
	}

	empty := rendering.NewRenderText("")
	empty.SetFace(face)
	empty.SetFontSize(20)
	if got := empty.MeasureWidth(""); got != 0 {
		t.Fatalf("empty MeasureWidth=%v, want 0", got)
	}
}
