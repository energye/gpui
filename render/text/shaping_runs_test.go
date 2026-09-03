package text

import (
	"strings"
	"testing"
)

func testShapingFace(t *testing.T) Face {
	t.Helper()
	face, _, err := LoadMultiFace(16)
	if err != nil || face == nil {
		t.Skipf("LoadMultiFace unavailable: %v", err)
	}
	return face
}

// M0 item 8 regression lock (scoped): the shaping chain works run by run —
// every fallback run of a mixed 5000-char text shapes non-empty with its own
// face. Whole-face Shape(MultiFace) intentionally stays 0 (Source()==nil
// gate untouched: flipping it would change per-rune measure values inside
// WrapText for every MultiFace caller — stopped by the M0 熔断).
func TestShapeNonEmpty_MultiFace(t *testing.T) {
	face := testShapingFace(t)
	mf, ok := face.(*MultiFace)
	if !ok {
		t.Skipf("not a MultiFace: %T", face)
	}
	var b strings.Builder
	for b.Len() < 12000 {
		b.WriteString("a世界bHello你好مرحبا ")
	}
	runs := mf.Runs(b.String())
	if len(runs) == 0 {
		t.Fatalf("0 runs for mixed text")
	}
	for i, r := range runs {
		g := Shape(r.Text, r.Face)
		if len(g) == 0 {
			t.Fatalf("run %d %q shapes to 0 glyphs", i, r.Text)
		}
	}
}

// M0 item 8: font-fallback splitting still cuts mixed text per face.
func TestItemizeRuns_MixedLatinCJK(t *testing.T) {
	face := testShapingFace(t)
	mf, ok := face.(*MultiFace)
	if !ok {
		t.Skipf("not a MultiFace: %T", face)
	}
	runs := mf.Runs("Hello世界abc你好")
	// Run count depends on face order (a CJK face with Latin coverage
	// merges neighbors); lock lossless splitting, not an exact count.
	if len(runs) < 2 {
		t.Fatalf("runs=%d want ≥2", len(runs))
	}
	var joined strings.Builder
	for _, r := range runs {
		if r.Face == nil || r.Text == "" {
			t.Fatalf("degenerate run %+v", r)
		}
		joined.WriteString(r.Text)
	}
	if joined.String() != "Hello世界abc你好" {
		t.Fatalf("runs join %q", joined.String())
	}
}
