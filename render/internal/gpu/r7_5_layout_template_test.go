//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"golang.org/x/image/font/gofont/goregular"
)

func r75Face(t *testing.T, size float64) text.Face {
	t.Helper()
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	return src.Face(size)
}

func TestR75_LayoutTemplate_ScrollRebase(t *testing.T) {
	face := r75Face(t, 14)
	eng := NewGlyphMaskEngine()
	eng.ResetLayoutTemplateCacheStats()
	color := render.RGBA{R: 0.1, G: 0.1, B: 0.12, A: 1}
	mat := render.Identity()
	const s = "Inbox message subject line"

	b1, err := eng.LayoutText(face, s, 12, 24, color, mat, 1)
	if err != nil {
		t.Fatalf("layout1: %v", err)
	}
	if len(b1.Quads) == 0 {
		t.Fatal("no quads")
	}
	hits, misses, entries := eng.LayoutTemplateCacheStats()
	if hits != 0 || misses != 1 || entries != 1 {
		t.Fatalf("cold stats hits=%d misses=%d entries=%d", hits, misses, entries)
	}

	// Vertical list scroll by integer row height — must hit template + rebase.
	b2, err := eng.LayoutText(face, s, 12, 48, color, mat, 1)
	if err != nil {
		t.Fatalf("layout2: %v", err)
	}
	if len(b2.Quads) != len(b1.Quads) {
		t.Fatalf("quad count %d vs %d", len(b2.Quads), len(b1.Quads))
	}
	hits, misses, _ = eng.LayoutTemplateCacheStats()
	if hits < 1 {
		t.Fatalf("expected layout template hit on scroll, hits=%d misses=%d", hits, misses)
	}

	// Pixel-safe: all quads shifted by uniform dy=24.
	const wantDY float32 = 24
	for i := range b1.Quads {
		d0 := b2.Quads[i].Y0 - b1.Quads[i].Y0
		d1 := b2.Quads[i].Y1 - b1.Quads[i].Y1
		if math.Abs(float64(d0-wantDY)) > 1e-3 || math.Abs(float64(d1-wantDY)) > 1e-3 {
			t.Fatalf("quad %d dy=(%v,%v) want %v", i, d0, d1, wantDY)
		}
		if b2.Quads[i].X0 != b1.Quads[i].X0 || b2.Quads[i].X1 != b1.Quads[i].X1 {
			t.Fatalf("quad %d x changed on vertical scroll", i)
		}
		// UV must be identical (same atlas masks).
		if b2.Quads[i].U0 != b1.Quads[i].U0 || b2.Quads[i].V0 != b1.Quads[i].V0 ||
			b2.Quads[i].U1 != b1.Quads[i].U1 || b2.Quads[i].V1 != b1.Quads[i].V1 {
			t.Fatalf("quad %d UV changed on rebase", i)
		}
	}
}

func TestR75_LayoutTemplate_ColorIndependent(t *testing.T) {
	face := r75Face(t, 13)
	eng := NewGlyphMaskEngine()
	eng.ResetLayoutTemplateCacheStats()
	mat := render.Identity()
	const s = "Color independent label"
	c1 := render.RGBA{R: 1, G: 0, B: 0, A: 1}
	c2 := render.RGBA{R: 0, G: 0, B: 1, A: 1}

	b1, err := eng.LayoutText(face, s, 8, 20, c1, mat, 1)
	if err != nil {
		t.Fatalf("layout1: %v", err)
	}
	b2, err := eng.LayoutText(face, s, 8, 20, c2, mat, 1)
	if err != nil {
		t.Fatalf("layout2: %v", err)
	}
	hits, _, _ := eng.LayoutTemplateCacheStats()
	if hits < 1 {
		t.Fatalf("expected color-only hit, hits=%d", hits)
	}
	if b1.Color == b2.Color {
		t.Fatalf("color should differ: %v vs %v", b1.Color, b2.Color)
	}
	if len(b1.Quads) != len(b2.Quads) {
		t.Fatal("quad count mismatch")
	}
	for i := range b1.Quads {
		if b1.Quads[i] != b2.Quads[i] {
			t.Fatalf("geometry should be identical at same origin, i=%d", i)
		}
	}
}

func TestR75_LayoutTemplate_MatchesFreshLayout(t *testing.T) {
	// Rebased batch must match a cold engine layout at the new origin.
	face := r75Face(t, 14)
	mat := render.Identity()
	color := render.RGBA{R: 0, G: 0, B: 0, A: 1}
	const s = "Rebase equality check"
	const x, y0, y1 = 10.0, 30.0, 30.0 + 22.0

	warm := NewGlyphMaskEngine()
	if _, err := warm.LayoutText(face, s, x, y0, color, mat, 1); err != nil {
		t.Fatal(err)
	}
	got, err := warm.LayoutText(face, s, x, y1, color, mat, 1)
	if err != nil {
		t.Fatal(err)
	}
	hits, _, _ := warm.LayoutTemplateCacheStats()
	if hits < 1 {
		t.Fatalf("expected rebase hit, hits=%d", hits)
	}

	cold := NewGlyphMaskEngine()
	want, err := cold.LayoutText(face, s, x, y1, color, mat, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Quads) != len(want.Quads) {
		t.Fatalf("quads %d vs %d", len(got.Quads), len(want.Quads))
	}
	for i := range want.Quads {
		gq, wq := got.Quads[i], want.Quads[i]
		if math.Abs(float64(gq.X0-wq.X0)) > 1e-3 || math.Abs(float64(gq.Y0-wq.Y0)) > 1e-3 ||
			math.Abs(float64(gq.X1-wq.X1)) > 1e-3 || math.Abs(float64(gq.Y1-wq.Y1)) > 1e-3 {
			t.Fatalf("pos mismatch i=%d got=(%v,%v)-(%v,%v) want=(%v,%v)-(%v,%v)",
				i, gq.X0, gq.Y0, gq.X1, gq.Y1, wq.X0, wq.Y0, wq.X1, wq.Y1)
		}
	}
}

func TestR75_LayoutTextAliased_UsesTemplate(t *testing.T) {
	face := r75Face(t, 14)
	eng := NewGlyphMaskEngine()
	eng.ResetLayoutTemplateCacheStats()
	mat := render.Identity()
	color := render.RGBA{R: 0, G: 0, B: 0, A: 1}
	const s = "Aliased HUD"
	if _, err := eng.LayoutTextAliased(face, s, 4, 16, color, mat, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := eng.LayoutTextAliased(face, s, 4, 40, color, mat, 1); err != nil {
		t.Fatal(err)
	}
	hits, _, _ := eng.LayoutTemplateCacheStats()
	if hits < 1 {
		t.Fatalf("aliased scroll should hit template, hits=%d", hits)
	}
}
