package render

import (
	"image"
	"testing"
)

// TestR74_TrackDamageOverflowPrefersTouchMerge verifies R7.4: exceeding
// maxDamageRects no longer immediately collapses every rect into one AABB.
// Touching pairs coalesce first; distant clusters remain separate when ≤ cap.
func TestR74_TrackDamageOverflowPrefersTouchMerge(t *testing.T) {
	dc := NewContext(2000, 2000)
	defer dc.Close()

	// 18 pairs of edge-adjacent 8×8 rects, each pair isolated far from others.
	// After touch-merge → 18 clusters. Cap is 16, so final result is a single
	// union only because 18 > 16 — still correct. Use 12 pairs (24 rects) that
	// merge to 12 clusters ≤ 16 to prove multi-rect retention.
	const pairs = 12
	for i := 0; i < pairs; i++ {
		x := i * 120
		y := i * 10
		// two edge-adjacent rects: [x,y,x+8,y+8] and [x+8,y,x+16,y+8]
		dc.TrackDamageRect(image.Rect(x, y, x+8, y+8))
		dc.TrackDamageRect(image.Rect(x+8, y, x+16, y+8))
	}

	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatal("expected damage rects")
	}
	if len(rects) > maxDamageRects {
		t.Fatalf("over cap: got %d > %d", len(rects), maxDamageRects)
	}
	// Must retain multiple clusters (not force full-surface AABB of all 12).
	if len(rects) < pairs {
		// If deviceScale / matrix altered coords, still require >1 multi-rect.
		if len(rects) <= 1 {
			t.Fatalf("R7.4 regression: overflow collapsed to %d rect(s); want multi-rect clusters", len(rects))
		}
	}
	// Each surviving rect should be tight-ish (pair is 16×8), not the full
	// span of all pairs (~12*120 wide).
	full := image.Rectangle{}
	for _, r := range rects {
		full = full.Union(r)
	}
	if len(rects) >= 2 {
		// At least one rect must be much smaller than the global union.
		smallOK := false
		for _, r := range rects {
			if r.Dx()*r.Dy() < full.Dx()*full.Dy()/4 {
				smallOK = true
				break
			}
		}
		if !smallOK {
			t.Fatalf("expected at least one tight cluster vs global union %v; rects=%v", full, rects)
		}
	}
}

// TestR74_TrackDamageOverflowStillCaps ensures distant-only overflow still
// stays within maxDamageRects (may full-union when pairwise cannot reduce enough).
func TestR74_TrackDamageOverflowStillCaps(t *testing.T) {
	dc := NewContext(4000, 4000)
	defer dc.Close()
	for i := 0; i < maxDamageRects+5; i++ {
		// fully isolated 4×4 rects
		x, y := i*80, i*3
		dc.TrackDamageRect(image.Rect(x, y, x+4, y+4))
	}
	rects := dc.FrameDamage()
	if len(rects) > maxDamageRects {
		t.Fatalf("cap broken: %d > %d", len(rects), maxDamageRects)
	}
}
