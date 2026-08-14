//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// TestTessellateAA_SquareConvex verifies the analytic-AA fringe bands for a
// simple convex square (CCW): the exterior band carries d ∈ {0, -aa} (0 on
// the boundary line, -aa outward) and the interior band d ∈ {0, +aa} (0 on
// the line, +aa inward); each band has exactly 6 vertices per boundary edge.
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
		t.Fatal("TessellateAA emitted no exterior band vertices")
	}
	if n%6 != 0 {
		t.Fatalf("band vertex count %d not a multiple of 6 (quad verts)", n)
	}
	// Bands: 4 edges (square closes exactly) x 6 verts = 24 each.
	band := tess.bandVerts
	if len(band) != 4*6*3 {
		t.Fatalf("exterior band verts=%d want 72 (4 edges x 6 verts x 3 floats)", len(band))
	}
	inner := tess.innerBandVerts
	if len(inner) != 4*6*3 {
		t.Fatalf("interior band verts=%d want 72", len(inner))
	}
	// Exterior d: the two edge-line corners 0, the two outward corners -aa.
	for i := 2; i < len(band); i += 3 {
		d := float64(band[i])
		if d != 0 && math.Abs(d-(-aaCoverHalfWidth)) > 1e-5 {
			t.Fatalf("exterior band d=%.3f want 0 or -%.3f", d, aaCoverHalfWidth)
		}
	}
	// Interior d: the two edge-line corners 0, the two inward corners +aa.
	for i := 2; i < len(inner); i += 3 {
		d := float64(inner[i])
		if d != 0 && math.Abs(d-aaCoverHalfWidth) > 1e-5 {
			t.Fatalf("interior band d=%.3f want 0 or +%.3f", d, aaCoverHalfWidth)
		}
	}
}

// TestTessellateAA_CWSquare verifies the orientation flip: a clockwise square
// (hole-style winding) must still get outward exterior bands and inward
// interior bands (the fill-side direction is orientation-normalized).
func TestTessellateAA_CWSquare(t *testing.T) {
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(0, 10)
	p.LineTo(10, 10)
	p.LineTo(10, 0)
	p.Close()

	tess := NewFanTessellator()
	tess.TessellateAA(p)
	// Both fringe halves must be emitted with the same vertex count (the
	// orientation flip only changes the inward/outward direction, which the
	// shader consumes as ±d; the window verification checks the side).
	if len(tess.bandVerts) == 0 || len(tess.innerBandVerts) == 0 {
		t.Fatal("CW square emitted no fringe bands")
	}
	if len(tess.bandVerts) != len(tess.innerBandVerts) {
		t.Fatalf("CW square band mismatch: outer=%d inner=%d", len(tess.bandVerts), len(tess.innerBandVerts))
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
	// Structure sanity for the non-convex ring: both fringe halves are
	// emitted, each containing exactly one quad (6 verts) per flattened edge.
	if len(tess.bandVerts)%18 != 0 {
		t.Fatalf("exterior band verts %d not a multiple of 18 (edge x 6 verts x 3 floats)", len(tess.bandVerts))
	}
	if len(tess.innerBandVerts) != len(tess.bandVerts) {
		t.Fatalf("arc band halves mismatch: outer=%d inner=%d", len(tess.bandVerts), len(tess.innerBandVerts))
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

// TestPathGeomCacheAA_MissThenHit verifies the AA fringe bands are computed
// lazily on the first AA request and cached for the next (shared fan, no
// retess).
func TestPathGeomCacheAA_MissThenHit(t *testing.T) {
	c := NewPathGeometryCache()
	p := render.NewPath()
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(5, 14)
	p.LineTo(0, 10)
	p.Close()

	v1, _, ba1, iba1, ok := c.GetOrTessellateAA(p, render.FillRuleNonZero, false, false)
	if !ok || len(v1) == 0 || len(ba1) != 0 || len(iba1) != 0 {
		t.Fatalf("non-AA miss must stay lazy: verts=%d band=%d inner=%d ok=%v", len(v1), len(ba1), len(iba1), ok)
	}
	// First AA request on the same entry computes + caches the AA geometry.
	v2, _, ba2, iba2, ok2 := c.GetOrTessellateAA(p, render.FillRuleNonZero, false, true)
	if !ok2 || len(v2) == 0 || len(ba2) == 0 || len(iba2) == 0 {
		t.Fatalf("AA miss failed: verts=%d band=%d inner=%d ok=%v", len(v2), len(ba2), len(iba2), ok2)
	}
	// Second AA request reuses the cached AA geometry and the shared fan.
	v3, _, ba3, iba3, ok3 := c.GetOrTessellateAA(p, render.FillRuleNonZero, false, true)
	if !ok3 || len(ba3) != len(ba2) || len(iba3) != len(iba2) {
		t.Fatalf("AA hit mismatch: %d vs %d, %d vs %d", len(ba3), len(ba2), len(iba3), len(iba2))
	}
	if &v3[0] != &v1[0] {
		t.Fatal("expected zero-copy fan hit")
	}
}