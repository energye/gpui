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

func TestHitTestCluster_LTR(t *testing.T) {
	// Visual = logical: clusters 0,1,2 advances 10,12,8
	gs := []ShapedGlyph{
		{GID: 1, Cluster: 0, X: 0, XAdvance: 10},
		{GID: 2, Cluster: 1, X: 10, XAdvance: 12},
		{GID: 3, Cluster: 2, X: 22, XAdvance: 8},
	}
	if HitTestCluster(gs, -1) != 0 {
		t.Fatal("before start")
	}
	if HitTestCluster(gs, 0) != 0 {
		t.Fatal("at 0")
	}
	if HitTestCluster(gs, 9.9) != 0 {
		t.Fatal("end of first")
	}
	if HitTestCluster(gs, 10) != 1 {
		t.Fatal("start of second")
	}
	if HitTestCluster(gs, 21) != 1 {
		t.Fatal("inside second")
	}
	if HitTestCluster(gs, 22) != 2 {
		t.Fatal("start of third")
	}
	c, trailing := HitTestClusterEdge(gs, 100)
	if c != 2 || !trailing {
		t.Fatalf("past end: c=%d trailing=%v", c, trailing)
	}
	// mid of second glyph → trailing
	_, tr := HitTestClusterEdge(gs, 10+6) // mid of 12
	if !tr {
		t.Fatal("mid should trailing")
	}
}

func TestHitTestCluster_RTLVisual(t *testing.T) {
	// After ReorderRTL: visual order clusters 2,1,0
	in := []ShapedGlyph{
		{GID: 1, Cluster: 0, X: 0, XAdvance: 10},
		{GID: 2, Cluster: 1, X: 10, XAdvance: 12},
		{GID: 3, Cluster: 2, X: 22, XAdvance: 8},
	}
	gs := ReorderRTLShapedGlyphs(in)
	// Visual: cluster2 @0..8, cluster1 @8..20, cluster0 @20..30
	if HitTestCluster(gs, 0) != 2 {
		t.Fatalf("left hit want cluster 2 got %d", HitTestCluster(gs, 0))
	}
	if HitTestCluster(gs, 8) != 1 {
		t.Fatalf("mid run want 1 got %d", HitTestCluster(gs, 8))
	}
	if HitTestCluster(gs, 20) != 0 {
		t.Fatalf("right want 0 got %d", HitTestCluster(gs, 20))
	}
	// Round-trip: caret X for cluster should hit-test back (approx)
	for _, cl := range []int{0, 1, 2} {
		x := CaretXForCluster(gs, cl)
		got := HitTestCluster(gs, x)
		if got != cl {
			// At exact boundary between glyphs, HitTest uses [X, X+adv); caret at
			// leading edge of cluster may be the left edge of that visual glyph.
			if got != cl {
				t.Fatalf("cluster %d caretX=%v hit=%d", cl, x, got)
			}
		}
	}
}

func TestCaretXForCluster_LTR(t *testing.T) {
	gs := []ShapedGlyph{
		{GID: 1, Cluster: 0, X: 0, XAdvance: 10},
		{GID: 2, Cluster: 1, X: 10, XAdvance: 12},
		{GID: 3, Cluster: 2, X: 22, XAdvance: 8},
	}
	if CaretXForCluster(gs, 0) != 0 {
		t.Fatal()
	}
	if CaretXForCluster(gs, 1) != 10 {
		t.Fatal()
	}
	if CaretXForCluster(gs, 2) != 22 {
		t.Fatal()
	}
	if CaretXForCluster(gs, 99) != 30 {
		t.Fatalf("after last want 30 got %v", CaretXForCluster(gs, 99))
	}
	if RunAdvance(gs) != 30 {
		t.Fatalf("advance %v", RunAdvance(gs))
	}
}
