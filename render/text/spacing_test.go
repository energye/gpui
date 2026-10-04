package text

import (
	"testing"
)

func spacedGlyphs(n int, adv float64) []ShapedGlyph {
	g := make([]ShapedGlyph, n)
	x := 0.0
	for i := range g {
		g[i] = ShapedGlyph{Cluster: i, X: x, XAdvance: adv}
		x += adv
	}
	return g
}

// TestApplySpacing_GapsNoTrailing locks n-1 semantics: 4 clusters gain 3 gaps.
func TestApplySpacing_GapsNoTrailing(t *testing.T) {
	g := spacedGlyphs(4, 10)
	extra := ApplySpacing(g, "abcd", 5, 0)
	if extra != 15 {
		t.Fatalf("extra=%v want 15 (3 gaps x 5)", extra)
	}
	wantX := []float64{0, 15, 30, 45}
	for i := range g {
		if g[i].X != wantX[i] {
			t.Fatalf("glyph %d X=%v want %v", i, g[i].X, wantX[i])
		}
	}
}

// TestApplySpacing_SingleCluster: one letter measures exactly its advance.
func TestApplySpacing_SingleCluster(t *testing.T) {
	g := spacedGlyphs(1, 10)
	if extra := ApplySpacing(g, "a", 5, 0); extra != 0 {
		t.Fatalf("extra=%v want 0", extra)
	}
	if g[0].X != 0 {
		t.Fatalf("X=%v want 0", g[0].X)
	}
}

// TestApplySpacing_ZeroFastPath: zero spacing touches nothing.
func TestApplySpacing_ZeroFastPath(t *testing.T) {
	g := spacedGlyphs(3, 10)
	before := append([]ShapedGlyph(nil), g...)
	if extra := ApplySpacing(g, "abc", 0, 0); extra != 0 {
		t.Fatalf("extra=%v want 0", extra)
	}
	for i := range g {
		if g[i] != before[i] {
			t.Fatalf("glyph %d changed on zero spacing", i)
		}
	}
}

// TestApplySpacing_WordRuns: one bump per word separator (shaping library
// CSS set: each space grows its own advance; a double space is two bumps,
// tab is not a separator).
func TestApplySpacing_WordRuns(t *testing.T) {
	g := spacedGlyphs(5, 10) // "a  b " runes 0..4, spaces at 1,2,4
	extra := ApplySpacing(g, "a  b ", 0, 7)
	// Separators at 1,2,4: 3 bumps (library per-separator semantics).
	if extra != 21 {
		t.Fatalf("extra=%v want 21 (3 separators x 7)", extra)
	}
	// Half-split distribution: separator content centers in its grown
	// advance, so following glyphs shift by full bumps after each separator.
	wantX := []float64{0, 13.5, 30.5, 44, 57.5}
	for i := range g {
		if g[i].X != wantX[i] {
			t.Fatalf("glyph %d X=%v want %v", i, g[i].X, wantX[i])
		}
	}
}

// TestApplySpacing_LigatureGlue: shared cluster keeps one shift (marks and
// ligature parts never separate).
func TestApplySpacing_LigatureGlue(t *testing.T) {
	g := []ShapedGlyph{
		{Cluster: 0, X: 0, XAdvance: 12},
		{Cluster: 0, X: 0, XAdvance: 0},
		{Cluster: 1, X: 12, XAdvance: 10},
	}
	extra := ApplySpacing(g, "ab", 5, 0)
	if extra != 5 {
		t.Fatalf("extra=%v want 5 (2 clusters, 1 gap)", extra)
	}
	if g[0].X != 0 || g[1].X != 0 || g[2].X != 17 {
		t.Fatalf("clusters split: %+v", g)
	}
}

// TestApplySpacing_ClampForeignClusters: out-of-range clusters clamp,
// never panic.
func TestApplySpacing_ClampForeignClusters(t *testing.T) {
	g := []ShapedGlyph{
		{Cluster: -3, X: 0, XAdvance: 10},
		{Cluster: 500, X: 10, XAdvance: 10},
	}
	extra := ApplySpacing(g, "ab", 4, 0)
	if extra != 4 {
		t.Fatalf("extra=%v want 4", extra)
	}
}

// TestWordSeparators documents the shaping library CSS separator set:
// spaces count, tab does not (tab keeps its tab-stop advance).
func TestWordSeparators(t *testing.T) {
	if got := SpacingExtra(1, "a  b\tc", 0, 7); got != 14 {
		t.Fatalf("word extra=%v want 14 (2 spaces x 7, tab excluded)", got)
	}
	if got := SpacingExtra(1, "abc", 0, 7); got != 0 {
		t.Fatalf("word extra=%v want 0 (no separators)", got)
	}
}

// TestLayoutText_LetterSpacingEndToEnd: layout width = base + gaps.
func TestLayoutText_LetterSpacingEndToEnd(t *testing.T) {
	face := layoutTestFace(t)
	base := LayoutText("PELICAN", face, DefaultLayoutOptions())
	if len(base.Lines) != 1 {
		t.Fatalf("lines=%d want 1", len(base.Lines))
	}
	opts := DefaultLayoutOptions()
	opts.LetterSpacing = 3
	spaced := LayoutText("PELICAN", face, opts)
	if len(spaced.Lines) != 1 {
		t.Fatalf("lines=%d want 1", len(spaced.Lines))
	}
	// 7 ASCII clusters → 6 gaps (1:1 shaping assumed; assert exactness).
	if got, want := spaced.Lines[0].Width-base.Lines[0].Width, 18.0; got != want {
		t.Fatalf("spacing bump=%v want %v", got, want)
	}
}

// TestLayoutText_WordSpacingEndToEnd: one bump per whitespace run.
func TestLayoutText_WordSpacingEndToEnd(t *testing.T) {
	face := layoutTestFace(t)
	base := LayoutText("A B", face, DefaultLayoutOptions())
	opts := DefaultLayoutOptions()
	opts.WordSpacing = 9
	spaced := LayoutText("A B", face, opts)
	if got, want := spaced.Lines[0].Width-base.Lines[0].Width, 9.0; got != want {
		t.Fatalf("spacing bump=%v want %v", got, want)
	}
}

// TestLayoutText_ZeroSpacingIdentical: zero opts match legacy output exactly.
func TestLayoutText_ZeroSpacingIdentical(t *testing.T) {
	face := layoutTestFace(t)
	a := LayoutText("Hello PELICAN rider", face, DefaultLayoutOptions())
	opts := DefaultLayoutOptions()
	opts.LetterSpacing, opts.WordSpacing = 0, 0
	b := LayoutText("Hello PELICAN rider", face, opts)
	if len(a.Lines) != len(b.Lines) || a.Width != b.Width {
		t.Fatalf("zero spacing differs: %+v vs %+v", a.Width, b.Width)
	}
	for i := range a.Lines {
		if len(a.Lines[i].Glyphs) != len(b.Lines[i].Glyphs) {
			t.Fatalf("line %d glyph count differs", i)
		}
		for j := range a.Lines[i].Glyphs {
			if a.Lines[i].Glyphs[j] != b.Lines[i].Glyphs[j] {
				t.Fatalf("line %d glyph %d differs", i, j)
			}
		}
	}
}

// TestWrapTextWithSpacing_BreaksEarlier: spacing consumes break budget.
func TestWrapTextWithSpacing_BreaksEarlier(t *testing.T) {
	face := layoutTestFace(t)
	plain := WrapText("aa bb cc", face, 1000, WrapWordChar)
	if len(plain) != 1 {
		t.Fatalf("plain lines=%d want 1 (wide budget)", len(plain))
	}
	tight := WrapTextWithSpacing("aa bb cc", face, 1000, WrapWordChar, 400, 400)
	if len(tight) <= 1 {
		t.Fatalf("spaced lines=%d want >1 (spacing must consume budget)", len(tight))
	}
	// Zero path delegates identically.
	zero := WrapTextWithSpacing("aa bb cc", face, 1000, WrapWordChar, 0, 0)
	if len(zero) != len(plain) || zero[0].Text != plain[0].Text {
		t.Fatalf("zero spacing differs from WrapText")
	}
}
