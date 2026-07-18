package text

import (
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func testFaceForShapeCache(t *testing.T, size float64) Face {
	t.Helper()
	src, err := NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	return src.Face(size)
}

func TestS65_LayoutGlyphs_CacheHit(t *testing.T) {
	ClearShapeResultCache()
	face := testFaceForShapeCache(t, 14)
	s := "scroll-row-label Hello"

	g1 := LayoutGlyphs(face, s)
	if len(g1) == 0 {
		t.Fatal("expected glyphs")
	}
	st1 := ShapeResultCacheStats()
	if st1.Misses < 1 {
		t.Fatalf("expected miss on first layout, stats=%+v", st1)
	}

	g2 := LayoutGlyphs(face, s)
	if len(g2) != len(g1) {
		t.Fatalf("glyph count %d vs %d", len(g2), len(g1))
	}
	// Same underlying slice (zero-copy hit).
	if &g1[0] != &g2[0] {
		t.Fatal("expected shared cached slice on hit")
	}
	st2 := ShapeResultCacheStats()
	if st2.Hits < 1 {
		t.Fatalf("expected hit, stats=%+v", st2)
	}
	if g1[0].GID != g2[0].GID || g1[len(g1)-1].X != g2[len(g2)-1].X {
		t.Fatalf("glyph content mismatch")
	}
}

func TestS65_Shape_CacheHit(t *testing.T) {
	ClearShapeResultCache()
	face := testFaceForShapeCache(t, 16)
	s := "office draft"

	a := Shape(s, face)
	b := Shape(s, face)
	if len(a) == 0 {
		t.Fatal("empty shape")
	}
	if len(a) != len(b) {
		t.Fatalf("len %d vs %d", len(a), len(b))
	}
	st := ShapeResultCacheStats()
	if st.Hits < 1 || st.Misses < 1 {
		t.Fatalf("want hits and misses, got %+v", st)
	}
	c := ShapeUncached(s, face)
	if len(c) != len(a) {
		t.Fatalf("uncached len %d vs %d", len(c), len(a))
	}
	for i := range a {
		if a[i].GID != c[i].GID {
			t.Fatalf("gid mismatch at %d: %v vs %v", i, a[i].GID, c[i].GID)
		}
	}
}

func TestS65_LayoutAndShape_ModeIsolation(t *testing.T) {
	ClearShapeResultCache()
	face := testFaceForShapeCache(t, 16)
	s := "fi"
	_ = LayoutGlyphs(face, s)
	_ = Shape(s, face)
	st := ShapeResultCacheStats()
	if st.Misses < 2 {
		t.Fatalf("expected separate cache entries for layout vs OT, stats=%+v", st)
	}
	_ = LayoutGlyphs(face, s)
	_ = Shape(s, face)
	st2 := ShapeResultCacheStats()
	if st2.Hits < 2 {
		t.Fatalf("expected hits after warm, stats=%+v", st2)
	}
}

func TestS65_MultiFaceRuns_CacheAndMerge(t *testing.T) {
	ClearMultiFaceRunsCache()
	face := testFaceForShapeCache(t, 14)
	mf, err := NewMultiFace(face, face)
	if err != nil {
		t.Fatal(err)
	}
	s := "HelloABC"
	r1 := mf.Runs(s)
	if len(r1) != 1 {
		t.Fatalf("same-face text should merge to 1 run, got %d", len(r1))
	}
	r2 := mf.Runs(s)
	if len(r2) != 1 || r2[0].Text != s {
		t.Fatalf("cached runs mismatch: %+v", r2)
	}
	if r1[0].Face != r2[0].Face {
		t.Fatal("face pointer changed across cache hit")
	}
}

func TestIsHighChurnLabel(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"short", false},
		{"scroll-row-label Hello", false},
		{"FPS 57.4  CPU 10%  RSS 200000KB  frame 1234  (本进程)", true},
		{"FPS 60.0  CPU 10%  RSS 200000KB  frame 0  (static)", true},
		{"ABCDEFGHIJKL", false}, // long, no digits
		{"12", false},           // short
	}
	for _, tc := range cases {
		if got := IsHighChurnLabel(tc.s); got != tc.want {
			t.Errorf("IsHighChurnLabel(%q)=%v want %v", tc.s, got, tc.want)
		}
	}
}

func TestLayoutGlyphs_SkipsHighChurnCache(t *testing.T) {
	ClearShapeResultCache()
	face := testFaceForShapeCache(t, 14)
	s := "FPS 57.4  CPU 10%  RSS 200000KB  frame 99"
	_ = LayoutGlyphs(face, s)
	_ = LayoutGlyphs(face, s)
	st := ShapeResultCacheStats()
	if st.Entries != 0 {
		t.Fatalf("high-churn label should not populate cache, stats=%+v", st)
	}
	// stable label still caches
	stable := "scroll-row-label Hello"
	_ = LayoutGlyphs(face, stable)
	_ = LayoutGlyphs(face, stable)
	st2 := ShapeResultCacheStats()
	if st2.Hits < 1 || st2.Entries < 1 {
		t.Fatalf("stable label should cache, stats=%+v", st2)
	}
}
