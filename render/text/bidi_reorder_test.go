package text

import "testing"

func TestReorderRTLShapedGlyphs_ReversesAndResetsX(t *testing.T) {
	in := []ShapedGlyph{
		{GID: 1, Cluster: 0, X: 0, XAdvance: 10},
		{GID: 2, Cluster: 1, X: 10, XAdvance: 12},
		{GID: 3, Cluster: 2, X: 22, XAdvance: 8},
	}
	out := ReorderRTLShapedGlyphs(in)
	if len(out) != 3 {
		t.Fatalf("len=%d", len(out))
	}
	// visual order: 3, 2, 1
	if out[0].GID != 3 || out[1].GID != 2 || out[2].GID != 1 {
		t.Fatalf("order gids=%v want 3,2,1", []GlyphID{out[0].GID, out[1].GID, out[2].GID})
	}
	// clusters preserved on glyphs
	if out[0].Cluster != 2 || out[2].Cluster != 0 {
		t.Fatalf("clusters=%d,%d,%d", out[0].Cluster, out[1].Cluster, out[2].Cluster)
	}
	// X rebased LTR for painting
	if out[0].X != 0 {
		t.Fatalf("first X=%v want 0", out[0].X)
	}
	if out[1].X != 8 {
		t.Fatalf("second X=%v want 8", out[1].X)
	}
	if out[2].X != 20 {
		t.Fatalf("third X=%v want 20", out[2].X)
	}
	// total width unchanged
	totalIn := in[len(in)-1].X + in[len(in)-1].XAdvance
	totalOut := out[len(out)-1].X + out[len(out)-1].XAdvance
	if totalIn != totalOut {
		t.Fatalf("width in=%v out=%v", totalIn, totalOut)
	}
}

func TestOwnShaper_RTLFace_Reorders(t *testing.T) {
	src, err := NewFontSource(requireTestFont(t), WithParser("own"))
	if err != nil {
		t.Fatal(err)
	}
	// Use Latin "abc" with RTL face — pure reordering of glyph run.
	ltr := src.Face(16, WithDirection(DirectionLTR))
	rtl := src.Face(16, WithDirection(DirectionRTL))
	a := ShapeUncached("abc", ltr)
	b := ShapeUncached("abc", rtl)
	if len(a) < 2 || len(b) != len(a) {
		t.Fatalf("len ltr=%d rtl=%d", len(a), len(b))
	}
	// RTL should reverse GIDs relative to LTR for the same cmap string
	if a[0].GID == b[0].GID && a[len(a)-1].GID == b[len(b)-1].GID {
		// if same order, fail — expect reverse
		same := true
		for i := range a {
			if a[i].GID != b[i].GID {
				same = false
				break
			}
		}
		if same {
			t.Fatalf("RTL face did not reorder glyphs")
		}
	}
	// First RTL glyph should match last LTR glyph
	if a[0].GID != b[len(b)-1].GID || a[len(a)-1].GID != b[0].GID {
		t.Fatalf("expected reverse: ltr first/last=%d/%d rtl first/last=%d/%d",
			a[0].GID, a[len(a)-1].GID, b[0].GID, b[len(b)-1].GID)
	}
}

func TestLayout_RTLSegment_VisualOrder(t *testing.T) {
	src, err := NewFontSource(requireTestFont(t), WithParser("own"))
	if err != nil {
		t.Fatal(err)
	}
	face := src.Face(16)
	// Hebrew sample — segmenter should mark RTL; layout reorders
	text := "שלום"
	layout := LayoutText(text, face, LayoutOptions{Direction: DirectionRTL})
	if layout == nil || len(layout.Lines) == 0 || len(layout.Lines[0].Glyphs) == 0 {
		// font may lack Hebrew — fallback to Latin RTL force via shape path only
		t.Skip("no glyphs for Hebrew in test font")
	}
	glyphs := layout.Lines[0].Glyphs
	// X should be non-decreasing for paint order
	for i := 1; i < len(glyphs); i++ {
		if glyphs[i].X+1e-9 < glyphs[i-1].X {
			t.Fatalf("paint X not monotonic: [%d]=%v [%d]=%v", i-1, glyphs[i-1].X, i, glyphs[i].X)
		}
	}
}
