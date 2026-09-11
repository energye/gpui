package rendering

import (
	"testing"
)

// Soft-wrap seams are shared points (prev.End == next.Start): downstream
// affinity must resolve to the next line's leading edge (Flutter), or the
// painted caret sticks at the previous line's trailing edge and vertical
// stepping re-derives the old line forever.
func TestWrapSeamDownstreamGoesNextLine(t *testing.T) {
	lay := BuildTextLayout("aaaaaaaaaaaaaaaaaaaaaaaa", nil, 14, 60, 1.2)
	if lay.LineCount() < 2 {
		t.Skipf("no wrap produced (lines=%d)", lay.LineCount())
	}
	s0, e0, _, _, _ := lay.Line(0)
	s1, _, _, _, _ := lay.Line(1)
	if e0 != s1 {
		t.Skipf("no shared seam (line0=[%d,%d) line1 starts %d)", s0, e0, s1)
	}
	seam := s1
	xd, yd, _, ok := lay.GetOffsetForCaret(seam, AffinityDownstream, 1.5)
	if !ok {
		t.Fatal("downstream caret missing")
	}
	if yd != lay.LineTop(1) {
		t.Fatalf("downstream seam y=%.2f want line1 top %.2f (caret paints on wrong line)", yd, lay.LineTop(1))
	}
	xu, yu, _, ok := lay.GetOffsetForCaret(seam, AffinityUpstream, 1.5)
	if !ok {
		t.Fatal("upstream caret missing")
	}
	if yu != lay.LineTop(0) {
		t.Fatalf("upstream seam y=%.2f want line0 top %.2f", yu, lay.LineTop(0))
	}
	_ = xd
	_ = xu
	li, _, ok := lay.CaretForOffset(seam)
	if !ok {
		t.Fatal("CaretForOffset missing")
	}
	if li != 1 {
		t.Fatalf("CaretForOffset(seam)=line %d want 1 (vertical stepping traps on line 0)", li)
	}
}
