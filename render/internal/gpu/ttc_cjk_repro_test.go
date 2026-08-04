//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

func ttcCJKFace(t *testing.T) text.Face {
	t.Helper()
	candidates := []string{
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/opentype/noto/NotoSerifCJK-Regular.ttc",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			src, err := text.NewFontSourceFromFile(p)
			if err != nil {
				t.Fatalf("NewFontSourceFromFile(%s): %v", p, err)
			}
			f := src.Face(14)
			if f == nil {
				t.Fatalf("Face(14) nil for %s", p)
			}
			return f
		}
	}
	t.Skip("no CJK TTC font on this machine")
	return nil
}

func TestProbeTTC_CJKGlyphMaskEngine(t *testing.T) {
	face := ttcCJKFace(t)
	s := "合成"
	for _, r := range []rune(s) {
		if !face.HasGlyph(r) {
			t.Fatalf("face missing glyph %q", r)
		}
	}

	engine := NewGlyphMaskEngine()
	batch, err := engine.LayoutText(face, s, 10, 24, render.RGBA{A: 1}, render.Identity(), 1.0)
	if err != nil {
		t.Fatalf("LayoutText TTC CJK: %v", err)
	}
	t.Logf("quads=%d", len(batch.Quads))
	for i, q := range batch.Quads {
		t.Logf("quad[%d] x0=%v y0=%v x1=%v y1=%v", i, q.X0, q.Y0, q.X1, q.Y1)
	}
	if len(batch.Quads) == 0 {
		t.Fatal("TTC CJK LayoutText produced ZERO quads")
	}

	pg, _, _ := engine.Atlas().PageR8Data(0)
	if pg == nil {
		t.Fatal("no atlas page 0")
	}
	nz := 0
	for _, p := range pg {
		if p != 0 {
			nz++
		}
	}
	t.Logf("atlas nonzero coverage bytes = %d", nz)
	if nz == 0 {
		t.Fatal("atlas has NO CJK coverage — glyphs not rasterized")
	}

	pgw, pgh := 0, 0
	if pgw == 0 {
		pgw = 256
		pgh = 256
	}
	t.Logf("page size %dx%d", pgw, pgh)
	rows := map[int]int{}
	for i, p := range pg {
		if p != 0 {
			rows[i/pgw]++
		}
	}
	for _, y := range sortedKeys(rows) {
		t.Logf("mask row y=%d nz=%d", y, rows[y])
	}
}

func sortedKeys(m map[int]int) []int {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	for i := 0; i < len(ks); i++ {
		for j := i + 1; j < len(ks); j++ {
			if ks[j] < ks[i] {
				ks[i], ks[j] = ks[j], ks[i]
			}
		}
	}
	return ks
}
