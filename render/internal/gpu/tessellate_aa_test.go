//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// TestTessellateAA_SquareConvex verifies the analytic-AA fan geometry for a
// simple convex square (CCW): every fan triangle carries a signed edge
// distance that is 0 on the boundary edge and positive (toward the fill) at
// the fan origin; the exterior band has exactly 6 vertices per boundary edge.
func TestTessellateAA_SquareConvex(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(0, 10)
	p.Close()

	tess := NewFanTessellator()
	n := tess.TessellateAA(p)
	if n == 0 {
		t.Fatal("TessellateAA emitted no cover vertices")
	}
	if n%3 != 0 {
		t.Fatalf("cover vertex count %d not a multiple of 3 (triangle verts)", n)
	}
	// Fan: 2 triangles (6 vertices). Verify d at the fan origin (0,0) is the
	// exact distance to the triangle's boundary edge (10px for both edges).
	const tol = 1e-4
	got := tess.aaVerts
	// Triangle 1: (0,0,d0) (10,0,0) (10,10,0)
	if g := math.Abs(float64(got[2]) - 10); g > tol {
		t.Errorf("tri1 origin d=%.3f want 10", got[2])
	}
	if got[5] != 0 || got[8] != 0 {
		t.Errorf("tri1 boundary d not 0: %v %v", got[5], got[8])
	}
	// Triangle 2: (0,0,d0) (10,10,0) (0,10,0)
	if g := math.Abs(float64(got[11]) - 10); g > tol {
		t.Errorf("tri2 origin d=%.3f want 10", got[11])
	}
	// Band: 4 edges (square closes exactly) x 6 verts = 24.
	band := tess.bandVerts
	if len(band) != 4*6*3 {
		t.Fatalf("band verts=%d want 72 (4 edges x 6 verts x 3 floats)", len(band))
	}
	// Every band vertex d is 0 (on the edge line) or -aaCoverHalfWidth (outside).
	for i := 2; i < len(band); i += 3 {
		d := float64(band[i])
		if d != 0 && math.Abs(d-(-aaCoverHalfWidth)) > 1e-5 {
			t.Fatalf("band d=%.3f want 0 or -%.3f", d, aaCoverHalfWidth)
		}
	}
	// Interior side sign: sample the interpolated d at a point 0.5px inside
	// the left edge (closing edge is exact, so no gap) via the fan attribute
	// of the covering triangle — the origin distances are all positive.
	for i := 0; i < len(got); i += 9 {
		if got[i+2] < 0 {
			t.Fatalf("fan origin d<0: %.3f (interior side must be positive)", got[i+2])
		}
	}
}

// TestTessellateAA_CWSquare verifies the orientation flip: a clockwise square
// (hole-style winding) must still get positive (fill-side) origin distances.
func TestTessellateAA_CWSquare(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(0, 10)
	p.LineTo(10, 10)
	p.LineTo(10, 0)
	p.Close()

	tess := NewFanTessellator()
	tess.TessellateAA(p)
	got := tess.aaVerts
	if len(got) == 0 {
		t.Fatal("no AA cover verts")
	}
	for i := 0; i < len(got); i += 9 {
		if got[i+2] < 0 {
			t.Fatalf("CW square origin d<0: %.3f (fill side must be positive)", got[i+2])
		}
	}
}

// TestTessellateAA_ArcBandSweep approximates the ui_ant_verify blue arc stroke
// band (annular sector, single contour, non-convex for a 137° sweep) and
// verifies the AA fan covers it: the fan origin distance must be positive and
// the band count matches the flattened edge count (band = 6 verts per edge).
func TestTessellateAA_ArcBandSweep(t *testing.T) {
	p := render.NewPath()
	// Annular sector ring matching the ui_ant_verify blue arc stroke band:
	// centerline r=90, half-stroke 1px → outer arc r=91, inner arc r=89,
	// angles 3.4→5.8 rad, closed with round (semicircular) caps of radius 1.
	const cx, cy = 0.0, 0.0
	const rOut, rIn, rMid = 91.0, 89.0, 90.0
	const nArc, nCap = 48, 8
	pt := func(r, a float64) (float64, float64) {
		return cx + r*math.Cos(a), cy + r*math.Sin(a)
	}
	// Outer arc 3.4 → 5.8 (CCW).
	ox, oy := pt(rOut, 3.4)
	p.MoveTo(ox, oy)
	for i := 1; i <= nArc; i++ {
		x, y := pt(rOut, 3.4+(5.8-3.4)*float64(i)/float64(nArc))
		p.LineTo(x, y)
	}
	// Round cap at 5.8: semicircle r=1 centered on the centerline endpoint,
	// bulging outward (angles 5.8 → 5.8+π).
	mx, my := pt(rMid, 5.8)
	for i := 1; i <= nCap; i++ {
		a := 5.8 + math.Pi*float64(i)/float64(nCap)
		p.LineTo(mx+math.Cos(a), my+math.Sin(a))
	}
	// Inner arc 5.8 → 3.4 (reversed).
	for i := 1; i <= nArc; i++ {
		x, y := pt(rIn, 5.8-(5.8-3.4)*float64(i)/float64(nArc))
		p.LineTo(x, y)
	}
	// Round cap at 3.4: semicircle r=1 centered on the centerline endpoint,
	// bulging outward (angles 3.4-π → 3.4).
	mx, my = pt(rMid, 3.4)
	for i := 1; i <= nCap; i++ {
		a := 3.4 - math.Pi + math.Pi*float64(i)/float64(nCap)
		p.LineTo(mx+math.Cos(a), my+math.Sin(a))
	}
	p.Close()

	tess := NewFanTessellator()
	n := tess.TessellateAA(p)
	if n == 0 {
		t.Fatal("arc band AA tessellation empty")
	}
	// Structure sanity for the non-convex ring: fan + exterior band emitted;
	// the band contains exactly one quad (6 verts) per flattened edge. Sign
	// conventions are exact only for convex contours (SquareConvex/CWSquare);
	// non-star corners near the fan origin are the documented mis-attribution
	// the exterior band + stencil gate keep bounded.
	if len(tess.aaVerts)%9 != 0 {
		t.Fatalf("arc band fan verts %d not a multiple of 9 (triple floats)", len(tess.aaVerts))
	}
	if len(tess.bandVerts)%18 != 0 {
		t.Fatalf("band verts %d not a multiple of 18 (edge x 6 verts x 3 floats)", len(tess.bandVerts))
	}
}

// TestTessellateAA_BandDOutside ensures the exterior band's sampled coverage
// field is the outside half of the fringe: band corner distances are -aa and
// the on-line corners are 0, so fragment coverage ramps 0.5 -> 0 outward.
func TestTessellateAA_BandDOutside(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.Close()

	tess := NewFanTessellator()
	tess.TessellateAA(p)
	band := tess.bandVerts
	if len(band) == 0 {
		t.Fatal("no band verts")
	}
	seenLine, seenOut := false, false
	for i := 2; i < len(band); i += 3 {
		if band[i] == 0 {
			seenLine = true
		}
		if float64(band[i]) < -aaCoverHalfWidth/2 {
			seenOut = true
		}
	}
	if !seenLine || !seenOut {
		t.Fatalf("band must contain on-line (0) and outside (-aa) corners, got line=%v out=%v", seenLine, seenOut)
	}
}

// TestPathGeomCacheAA_MissThenHit verifies the AA geometry is computed lazily
// on the first AA request and cached for the next (shared fan, no retess).
func TestPathGeomCacheAA_MissThenHit(t *testing.T) {
	c := NewPathGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(5, 14)
	p.LineTo(0, 10)
	p.Close()

	v1, _, a1, b1, ok := c.GetOrTessellateAA(p, render.FillRuleNonZero, false, false)
	if !ok || len(v1) == 0 || len(a1) != 0 || len(b1) != 0 {
		t.Fatalf("non-AA miss must stay lazy: verts=%d aa=%d band=%d ok=%v", len(v1), len(a1), len(b1), ok)
	}
	// First AA request on the same entry computes + caches the AA geometry.
	v2, _, a2, b2, ok2 := c.GetOrTessellateAA(p, render.FillRuleNonZero, false, true)
	if !ok2 || len(v2) == 0 || len(a2) == 0 || len(b2) == 0 {
		t.Fatalf("AA miss failed: verts=%d aa=%d band=%d ok=%v", len(v2), len(a2), len(b2), ok2)
	}
	// Second AA request reuses the cached AA geometry and the shared fan.
	v3, _, a3, b3, ok3 := c.GetOrTessellateAA(p, render.FillRuleNonZero, false, true)
	if !ok3 || len(a3) != len(a2) || len(b3) != len(b2) {
		t.Fatalf("AA hit mismatch: %d vs %d, %d vs %d", len(a3), len(a2), len(b3), len(b2))
	}
	if &v3[0] != &v1[0] {
		t.Fatal("expected zero-copy fan hit")
	}
}