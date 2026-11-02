//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"math"
	"testing"
)

const conicQuarterWeight = math.Sqrt2 / 2

// quarterCircleConic returns the standard quarter circular arc control
// polygon: (r,0) → ctrl (r,r) → (0,r), the exact conic representation.
func quarterCircleConic(r float64) ConicBez {
	return ConicBez{
		P0: Pt(r, 0),
		P1: Pt(r, r),
		P2: Pt(0, r),
		W:  conicQuarterWeight,
	}
}

func assertNear(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %v, want %v (±%v)", name, got, want, tol)
	}
}

func TestConicQuarterCircleMatchesAnalytic(t *testing.T) {
	r := 100.0
	c := quarterCircleConic(r)
	for i := 0; i <= 64; i++ {
		tt := float64(i) / 64
		p := c.Eval(tt)
		d := math.Hypot(p.X, p.Y) // distance from arc center (0,0)
		assertNear(t, "quarter circle radius at t", d, r, 1e-6)
	}
}

func TestConicWeightOneEqualsQuad(t *testing.T) {
	c := ConicBez{P0: Pt(1, 2), P1: Pt(5, 7), P2: Pt(9, 3), W: 1}
	q := QuadBez{P0: Pt(1, 2), P1: Pt(5, 7), P2: Pt(9, 3)}
	for i := 0; i <= 16; i++ {
		tt := float64(i) / 16
		cc := c.Eval(tt)
		qq := q.Eval(tt)
		assertNear(t, "w=1 conic x", cc.X, qq.X, 1e-12)
		assertNear(t, "w=1 conic y", cc.Y, qq.Y, 1e-12)
	}
}

func TestConicChopHalvesCircleArc(t *testing.T) {
	r := 50.0
	c := quarterCircleConic(r)
	l, rt := c.Chop()
	// Halving a quarter circle (θ=90°, w=cos45°) yields two eighth arcs
	// (θ=45°, w=cos22.5°).
	assertNear(t, "chop weight", l.W, math.Cos(math.Pi/8), 1e-12)
	assertNear(t, "chop weight right", rt.W, math.Cos(math.Pi/8), 1e-12)
	for _, half := range []ConicBez{l, rt} {
		for i := 0; i <= 32; i++ {
			tt := float64(i) / 32
			p := half.Eval(tt)
			d := math.Hypot(p.X, p.Y)
			assertNear(t, "eighth arc radius", d, r, 1e-9)
		}
	}
	// Chopped pieces cover the original arc endpoints.
	assertNear(t, "left start", l.P0.X, c.P0.X, 1e-12)
	assertNear(t, "right end", rt.P2.X, c.P2.X, 1e-12)
	assertNear(t, "join point", l.P2.X, rt.P0.X, 1e-12)
}

func TestConicToQuadsTolerance(t *testing.T) {
	r := 200.0
	tol := 0.05
	var quads []QuadBez
	quarterCircleConic(r).ToQuads(tol, func(q QuadBez) {
		quads = append(quads, q)
	})
	if len(quads) < 2 {
		t.Fatalf("expected multiple quads for tol=%v, got %d", tol, len(quads))
	}
	// Every emitted quad must stay within tol of the analytic arc.
	worst := 0.0
	for _, q := range quads {
		for i := 0; i <= 16; i++ {
			tt := float64(i) / 16
			p := q.Eval(tt)
			d := math.Abs(math.Hypot(p.X, p.Y) - r)
			if d > worst {
				worst = d
			}
		}
	}
	if worst > tol {
		t.Errorf("quad approximation deviates %v > tol %v", worst, tol)
	}
	// Quads chain: each starts where the previous ended.
	for i := 1; i < len(quads); i++ {
		if quads[i].P0 != quads[i-1].P2 {
			t.Errorf("quad %d does not chain from quad %d", i, i-1)
		}
	}
	if quads[0].P0 != quarterCircleConic(r).P0 || quads[len(quads)-1].P2 != quarterCircleConic(r).P2 {
		t.Errorf("quad chain does not span the conic endpoints")
	}
}

func TestConicSubsegmentPointSet(t *testing.T) {
	r := 80.0
	c := quarterCircleConic(r)
	t0, t1 := 0.3, 0.75
	sub := c.Subsegment(t0, t1)
	// Endpoints match the original curve.
	p0 := c.Eval(t0)
	assertNear(t, "sub start x", sub.P0.X, p0.X, 1e-9)
	assertNear(t, "sub start y", sub.P0.Y, p0.Y, 1e-9)
	p1 := c.Eval(t1)
	assertNear(t, "sub end x", sub.P2.X, p1.X, 1e-9)
	assertNear(t, "sub end y", sub.P2.Y, p1.Y, 1e-9)
	// Every sampled point of the sub-conic lies on the analytic arc, and
	// spans only the angular range [t0, t1] (angles run 0..π/2 here).
	for i := 0; i <= 32; i++ {
		tt := float64(i) / 32
		p := sub.Eval(tt)
		d := math.Hypot(p.X, p.Y)
		assertNear(t, "sub arc radius", d, r, 1e-8)
		ang := math.Atan2(p.Y, p.X)
		if ang < -1e-9 || ang > math.Pi/2+1e-9 {
			t.Errorf("sub arc angle %v outside [0, π/2]", ang)
		}
	}
	// The original midpoint of the sub-range lies on the sub-conic (sampling
	// resolution ~0.03px at 2048 samples; a wrong re-fit deviates ~0.5px).
	mid := c.Eval((t0 + t1) / 2)
	best := math.Inf(1)
	for i := 0; i <= 2048; i++ {
		tt := float64(i) / 2048
		p := sub.Eval(tt)
		if d := math.Hypot(p.X-mid.X, p.Y-mid.Y); d < best {
			best = d
		}
	}
	if best > 0.1 {
		t.Errorf("original midpoint %v away from sub-conic", best)
	}
}

func TestConicBoundingBoxTight(t *testing.T) {
	r := 30.0
	// Quarter arc from (r,0) to (0,r): tight box is [0,r]×[0,r].
	b := quarterCircleConic(r).BoundingBox()
	assertNear(t, "bbox min x", b.Min.X, 0, 1e-9)
	assertNear(t, "bbox min y", b.Min.Y, 0, 1e-9)
	assertNear(t, "bbox max x", b.Max.X, r, 1e-9)
	assertNear(t, "bbox max y", b.Max.Y, r, 1e-9)

	// A 120° arc: θ=120°, w=cos(60°)=0.5, control at the endpoint tangent
	// intersection (tangent at (r,0) is the line x=r; tangent at the 120°
	// point meets it at y=r√3).
	c120 := ConicBez{
		P0: Pt(r, 0),
		P1: Pt(r, r*math.Sqrt(3)),
		P2: Pt(-r/2, r*math.Sqrt(3)/2),
		W:  0.5,
	}
	b2 := c120.BoundingBox()
	// Max y of the 120° arc (from 0° to 120°) is at 90°: y = r.
	assertNear(t, "120° arc bbox max y", b2.Max.Y, r, 1e-6)
	assertNear(t, "120° arc bbox max x", b2.Max.X, r, 1e-6)
	for i := 0; i <= 64; i++ {
		tt := float64(i) / 64
		p := c120.Eval(tt)
		d := math.Hypot(p.X, p.Y)
		assertNear(t, "120° arc radius", d, r, 1e-6)
	}
}

func TestPathConicToStorageRoundtrip(t *testing.T) {
	p := NewPath()
	p.MoveTo(10, 20)
	p.ConicTo(30, 40, 50, 60, conicQuarterWeight)
	p.Close()
	if p.NumVerbs() != 3 {
		t.Fatalf("verb count = %d, want 3", p.NumVerbs())
	}
	if !p.HasCurves() {
		t.Error("HasCurves should be true for conic path")
	}
	var verbs []PathVerb
	var conicCoords []float64
	p.Iterate(func(verb PathVerb, coords []float64) {
		verbs = append(verbs, verb)
		if verb == ConicTo {
			conicCoords = append(conicCoords, coords...)
		}
	})
	if len(verbs) != 3 || verbs[1] != ConicTo {
		t.Fatalf("verbs = %v, want [MoveTo ConicTo Close]", verbs)
	}
	want := []float64{30, 40, 50, 60, conicQuarterWeight}
	if len(conicCoords) != 5 {
		t.Fatalf("conic coords = %v, want 5 elements", conicCoords)
	}
	for i, w := range want {
		assertNear(t, "conic coord", conicCoords[i], w, 0)
	}
}

func TestPathConicZeroWeightDegradesToLine(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.ConicTo(10, 10, 20, 0, 0)
	verbs := []PathVerb{}
	p.Iterate(func(verb PathVerb, coords []float64) {
		verbs = append(verbs, verb)
	})
	if len(verbs) != 2 || verbs[1] != LineTo {
		t.Fatalf("w=0 should emit LineTo, got %v", verbs)
	}
}

func TestPathConicTransformKeepsWeight(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.ConicTo(100, 0, 100, 100, conicQuarterWeight)
	m := Scale(2, 3)
	q := p.Transform(m)
	q.Iterate(func(verb PathVerb, coords []float64) {
		if verb != ConicTo {
			return
		}
		assertNear(t, "transformed ctrl x", coords[0], 200, 1e-9)
		assertNear(t, "transformed end x", coords[2], 200, 1e-9)
		assertNear(t, "transformed end y", coords[3], 300, 1e-9)
		assertNear(t, "weight preserved", coords[4], conicQuarterWeight, 0)
	})
}

func TestPathIterateUnknownVerbPanics(t *testing.T) {
	p := NewPath()
	p.MoveTo(0, 0)
	p.LineTo(1, 1)
	// Corrupt the verb stream the way a missed consumer upgrade or bad
	// deserialize would: inject a value outside the known set.
	p.verbs[1] = PathVerb(200)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Iterate should panic on unknown verb")
		}
	}()
	p.Iterate(func(verb PathVerb, coords []float64) {})
}

func TestQuadAreaExactAgainstNumericIntegral(t *testing.T) {
	// Regression pin: quadArea must equal the numeric ½∫(x·dy − y·dx) Green
	// contribution — the same convention lineArea and cubicArea use. History:
	// 批 1 pinned the closed form against ∫x·dy (the old formula had also
	// dropped the −3y0/+3y2 endpoint terms), which agreed only because both
	// sides then used ∫x·dy. The conic-window sector check (P3.5 真窗 1) exposed
	// that mixing ∫x·dy curve edges with shoelace line edges inflates a mixed
	// path by ½[x·y] per curve run, so the pin moved to the symmetric form.
	cases := [][3]Point{
		{Pt(0, 0), Pt(1, 0.5), Pt(0, 1)},
		{Pt(0, 0), Pt(2, 1), Pt(0, 3)},
		{Pt(64, 0), Pt(64, 128), Pt(0, 64)},
		{Pt(5, -2), Pt(-3, 4), Pt(7, 9)},
	}
	for i, c := range cases {
		q := NewQuadBez(c[0], c[1], c[2])
		var num float64
		const n = 20000
		prev := q.Eval(0)
		for j := 1; j <= n; j++ {
			cur := q.Eval(float64(j) / n)
			num += 0.5 * (prev.X*cur.Y - cur.X*prev.Y)
			prev = cur
		}
		if math.Abs(num-quadArea(c[0], c[1], c[2])) > math.Abs(num)*1e-3+1e-6 {
			t.Errorf("quad case %d: formula %v vs numeric %v", i, quadArea(c[0], c[1], c[2]), num)
		}
	}
}

func TestConicArcQuarterCirclePixelFill(t *testing.T) {
	// A conic quarter arc path filled by the CPU rasterizer must match the
	// analytic quarter disk area.
	r := 64.0
	p := NewPath()
	p.MoveTo(r, 0)
	p.ConicTo(r, r, 0, r, conicQuarterWeight)
	p.LineTo(0, 0)
	p.Close()

	q := NewPath()
	q.MoveTo(r, 0)
	q.Arc(0, 0, r, 0, math.Pi/2)
	q.LineTo(0, 0)
	q.Close()

	areaP := p.Area()
	areaQ := q.Area()
	analytic := math.Pi * r * r / 4
	assertNear(t, "conic quarter disk area", areaP, analytic, analytic*0.002)
	assertNear(t, "cubic-arc quarter disk area", areaQ, analytic, analytic*0.002)
}
