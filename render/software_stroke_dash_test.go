package render

import (
	"math"
	"testing"
)

// TestDashPhaseContinuesAcrossSubpaths pins the B2 fix (WHATWG canvas /
// Skia SkDashPath semantics): the dash pattern continues across subpaths —
// each subpath resumes where the previous one left off. Pre-fix every
// MoveTo reset the pattern to the original offset, so multi-contour dashed
// strokes misplaced their dashes.
//
// Oracle (independent, computed by hand): subpath1 (0,0)→(50,0) with
// pattern [20,20], offset 0 draws on [0,20), off [20,40), on [40,50) and
// ends at pattern position 50 — mid-way through the second ON block
// (40..60). Subpath2 (60,0)→(110,0) therefore continues ON for the
// remaining 10 units of that block: first dash spans [60,70); then off
// [70,90); then on [90,110). A per-subpath reset would instead draw the
// first dash as [60,80).
func TestDashPhaseContinuesAcrossSubpaths(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.LineTo(50, 0)
	p.MoveTo(60, 0)
	p.LineTo(110, 0)

	out := dashPath(p, NewDash(20, 20))

	// Collect the drawn dash segments on the second subpath (y==0, x>=60).
	type seg struct{ x0, x1 float64 }
	var segs []seg
	var cx, cy float64
	var inDash bool
	var sx float64
	out.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			cx, cy = coords[0], coords[1]
			inDash = true // a MoveTo from dashPath starts a drawn dash
			sx = cx
		case LineTo:
			nx, ny := coords[0], coords[1]
			if inDash && cy == 0 && ny == 0 && cx >= 60 {
				segs = append(segs, seg{sx, nx})
			}
			cx, cy = nx, ny
		case Close:
			inDash = false
		}
	})
	if len(segs) == 0 {
		t.Fatalf("no dash segments found on the second subpath")
	}
	first := segs[0]
	if math.Abs(first.x0-60) > 1e-6 || math.Abs(first.x1-70) > 1e-6 {
		t.Fatalf("first dash on second subpath = [%.1f,%.1f], want [60,70] (pattern position 50 carried over: 10 units left in the ON block); a [60,80] result means the phase reset per subpath", first.x0, first.x1)
	}
	// And the remainder: off [70,90), on [90,110).
	if len(segs) < 2 || math.Abs(segs[1].x0-90) > 1e-6 || math.Abs(segs[1].x1-110) > 1e-6 {
		t.Fatalf("second dash on second subpath = %+v, want [90,110]", segs)
	}
}
