//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestGlyphMaskUnhintedFromFreeType guards the hinting choice: small axis-aligned
// Latin/CJK text uses NO hinting so glyphs rasterize pixel-identical to
// FreeType's FT_LOAD_NO_HINTING (verified per-glyph at 8–16px). The self-own
// Vertical/Full engines diverged structurally from FreeType light hinting,
// which is why the window previously rendered "completely different" glyphs
// from a browser. LCD subpixel rendering is orthogonal and unchanged.
func TestGlyphMaskUnhintedFromFreeType(t *testing.T) {
	for _, size := range []float64{8, 10, 11, 13, 16} {
		if h := selectGlyphMaskHinting(size, render.Identity(), false, 1.0); h != text.HintingNone {
			t.Fatalf("Latin text at %vpx hinting = %v, want HintingNone", size, h)
		}
		if h := selectGlyphMaskHinting(size, render.Identity(), true, 1.0); h != text.HintingNone {
			t.Fatalf("CJK text at %vpx hinting = %v, want HintingNone", size, h)
		}
	}
	if h := selectGlyphMaskHinting(13, render.Matrix{A: 1, B: 0.3, D: 0.3, E: 1}, false, 1.0); h != text.HintingNone {
		t.Fatalf("skewed text hinting = %v, want HintingNone", h)
	}
}

// TestGlyphMaskEvenSpacing guards against the two failure modes seen while
// fixing the faded glyph-mask text:
//
//  1. Fully-hinted glyphs must land on integer device pixels (else the grid-fit
//     stems are displaced and render faded). So every quad's left edge is an
//     integer. Since R21 switched the mask pipeline to unhinted rendering
//     (FreeType-identical), sub-pixel placement is now legal AA — this
//     assertion only applies while a hinting mode is selected.
//  2. Spacing must stay even: rounding each glyph's ABSOLUTE position
//     independently makes adjacent advances jitter by ±1px and opens visible
//     gaps inside words ("anyway" -> "an yway"). Using rounded ADVANCES instead
//     makes every like-advance identical. The test lays out a word and asserts
//     each glyph-to-glyph advance equals the rounded shaped advance (no jitter).
func TestGlyphMaskEvenSpacing(t *testing.T) {
	face := reproFont(t)
	var glyphs []text.ShapedGlyph
	for g := range face.Glyphs("anyway") {
		glyphs = append(glyphs, text.ShapedGlyph{GID: g.GID, X: g.X, Y: g.Y})
	}
	if len(glyphs) < 3 {
		t.Skip("font produced too few glyphs")
	}

	eng := NewGlyphMaskEngine()
	advances := func(baseX float64) []float64 {
		b, err := eng.LayoutShapedGlyphs(face, glyphs, baseX, 20, render.RGBA{A: 1}, render.Identity(), 1.0, false)
		if err != nil {
			t.Fatalf("LayoutShapedGlyphs: %v", err)
		}
		if len(b.Quads) != len(glyphs) {
			t.Skipf("got %d quads for %d glyphs (some empty)", len(b.Quads), len(glyphs))
		}
		out := make([]float64, 0, len(b.Quads))
		for i := range b.Quads {
			if i+1 < len(b.Quads) {
				out = append(out, float64(b.Quads[i+1].X0-b.Quads[i].X0))
			}
		}
		return out
	}

	// Even spacing means the internal advances depend only on the glyphs, not
	// on the word's sub-pixel start position. Rounding absolute positions (the
	// bug) makes them jitter with the base fraction and opens gaps in words;
	// rounding advances makes them identical regardless of base.
	a := advances(100.0)
	for _, base := range []float64{100.25, 100.5, 100.75} {
		b := advances(base)
		for i := range a {
			if math.Abs(a[i]-b[i]) > 0.01 {
				t.Fatalf("advance[%d] changed with sub-pixel base: %.2f at x=100.00 vs %.2f at x=%.2f — spacing jitters with position (opens gaps in words)",
					i, a[i], b[i], base)
			}
		}
	}
}
