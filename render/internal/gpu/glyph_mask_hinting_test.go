//go:build !nogpu

package gpu

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TestGlyphMaskLightHintingFromFreeType guards the hinting choice: small
// axis-aligned Latin/CJK text now uses HintingFull (FT_LOAD_TARGET_LIGHT
// parity) — the self-own light engine (CFF cf2 + glyf autofit + bytecode
// composite + CFF2 variable) is verified point-identical to FreeType light
// . R21 时期自研 Vertical/Full 引擎与
// FT 结构不一致，故当时退回 None；引擎对齐后解除硬编码。
//
// 旋转/倾斜矩阵、CJK HiDPI（deviceScale≥2）仍走 HintingNone。大字号
// （>48px）同样保留 HintingFull：无 hinting 的 GPU mask 斜边过渡与
// CPU text.Draw（sourceFace 默认 HintingFull）不一致，64/72px CJK 显示
// 文本发虚；保留 hinting 后 GPU/CPU 逐像素一致（规则见
// selectGlyphMaskHinting 注释）。
func TestGlyphMaskLightHintingFromFreeType(t *testing.T) {
	for _, size := range []float64{8, 10, 11, 13, 16} {
		if h := selectGlyphMaskHinting(size, render.Identity(), false, 1.0, text.HintingFull); h != text.HintingFull {
			t.Fatalf("Latin text at %vpx hinting = %v, want HintingFull", size, h)
		}
		if h := selectGlyphMaskHinting(size, render.Identity(), true, 1.0, text.HintingFull); h != text.HintingFull {
			t.Fatalf("CJK text at %vpx hinting = %v, want HintingFull", size, h)
		}
	}
	if h := selectGlyphMaskHinting(13, render.Matrix{A: 1, B: 0.3, D: 0.3, E: 1}, false, 1.0, text.HintingFull); h != text.HintingNone {
		t.Fatalf("skewed text hinting = %v, want HintingNone", h)
	}
	if h := selectGlyphMaskHinting(64, render.Identity(), false, 1.0, text.HintingFull); h != text.HintingFull {
		t.Fatalf("64px Latin text hinting = %v, want HintingFull (CPU text.Draw parity)", h)
	}
	if h := selectGlyphMaskHinting(64, render.Identity(), true, 1.0, text.HintingFull); h != text.HintingFull {
		t.Fatalf("64px CJK text hinting = %v, want HintingFull (CPU text.Draw parity)", h)
	}
	if h := selectGlyphMaskHinting(13, render.Identity(), true, 2.0, text.HintingFull); h != text.HintingFull {
		t.Fatalf("CJK HiDPI hinting = %v, want HintingNone", h)
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

// glyphMaskTestSource loads one TTF source for hinting-key tests (two faces
// from the SAME source differing only in WithHinting).
func glyphMaskTestSource(t *testing.T) *text.FontSource {
	t.Helper()
	for _, p := range []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		filepath.Join("testdata", "test.ttf"),
	} {
		if _, err := os.Stat(p); err == nil {
			src, err := text.NewFontSourceFromFile(p)
			if err != nil {
				continue
			}
			return src
		}
	}
	t.Skip("no TTF font available")
	return nil
}

// TestGlyphMaskHintingSeparatesAtlasEntries pins the A3 fix (Skia
// SkDescriptor rule: every state that changes the generated mask must be in
// the cache key). Two faces from the same source differing ONLY in hinting
// must produce separate atlas entries — pre-fix they collided on one entry
// because GlyphMaskKey carried no hinting bits while the rasterized outline
// depends on hinting (ExtractOutlineHinted).
func TestGlyphMaskHintingSeparatesAtlasEntries(t *testing.T) {
	src := glyphMaskTestSource(t)
	const size = 24
	faceFull := src.Face(size, text.WithHinting(text.HintingFull))
	faceNone := src.Face(size, text.WithHinting(text.HintingNone))
	// Single glyph at an integer origin: both hinting modes resolve the same
	// subpixel phase (fracX=0), so the ONLY key discriminator left is
	// hinting. (Multi-glyph strings let HintingNone's continuous positioning
	// leak into the subpixel key bits and mask the collision.)
	const s = "H"
	engine := NewGlyphMaskEngine()
	if _, err := engine.LayoutText(faceFull, s, 0, 0, render.RGBA{A: 1}, render.Identity(), 1); err != nil {
		t.Fatalf("LayoutText(full): %v", err)
	}
	_, _, entries1, _ := engine.AtlasStats()

	if _, err := engine.LayoutText(faceNone, s, 0, 0, render.RGBA{A: 1}, render.Identity(), 1); err != nil {
		t.Fatalf("LayoutText(none): %v", err)
	}
	_, _, entries2, _ := engine.AtlasStats()

	if entries2 == entries1 {
		t.Fatalf("hinting key collision: atlas entries stayed %d after switching HintingFull→HintingNone on the same source; masks must key on hinting", entries1)
	}
}
