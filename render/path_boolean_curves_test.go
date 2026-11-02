//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

const pathBoolProbeTol = 0.1

// TestBooleanResultAreaMatchesDenseReference pins the mixed line+curve result
// areas absolutely (not just through the identities): each boolean output's
// Area() must equal the same verbs sampled into a pure-line path. A convention
// mismatch between edge types showed up here as a per-curve-run ½[x·y] offset.
func TestBooleanResultAreaMatchesDenseReference(t *testing.T) {
	circle := render.NewPath()
	circle.Circle(0, 0, 20)
	blob := render.NewPath()
	blob.MoveTo(10, 10)
	blob.CubicTo(60, -20, 90, 70, 30, 90)
	blob.QuadraticTo(-10, 80, 5, 40)
	blob.Close()
	tilted := boolRect(5, -12, 44, 30)
	ops := []render.PathBooleanOp{render.PathOpUnion, render.PathOpIntersect,
		render.PathOpDifference, render.PathOpXor}
	for _, a := range []*render.Path{circle, blob} {
		for _, op := range ops {
			res := a.Op(tilted, op)
			if res.NumVerbs() == 0 {
				continue
			}
			ref := denseAreaReference(res, 4000)
			if ref == nil {
				t.Fatal("no dense reference")
			}
			scale := math.Max(1, math.Abs(ref.Area()))
			if rel := math.Abs(res.Area()-ref.Area()) / scale; rel > 1e-3 {
				t.Errorf("op=%d |A|=%v: Area()=%v dense=%v rel=%.2e", op, a.Area(), res.Area(), ref.Area(), rel)
			}
		}
	}
}

// denseAreaReference turns curve verbs into line segments of the same geometry.
func denseAreaReference(p *render.Path, per int) *render.Path {
	out := render.NewPath()
	var cur, start render.Point
	open := false
	p.Iterate(func(v render.PathVerb, c []float64) {
		switch v {
		case render.MoveTo:
			cur = render.Pt(c[0], c[1])
			start = cur
			if open {
				out.Close()
			}
			out.MoveTo(cur.X, cur.Y)
			open = true
		case render.LineTo:
			cur = render.Pt(c[0], c[1])
			out.LineTo(cur.X, cur.Y)
		case render.QuadTo:
			q := render.NewQuadBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]))
			for i := 1; i <= per; i++ {
				e := q.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = q.P2
		case render.CubicTo:
			cb := render.NewCubicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), render.Pt(c[4], c[5]))
			for i := 1; i <= per; i++ {
				e := cb.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = cb.P3
		case render.ConicTo:
			con := render.NewConicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), c[4])
			for i := 1; i <= per; i++ {
				e := con.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = con.P2
		case render.Close:
			if cur != start {
				out.LineTo(start.X, start.Y)
			}
			out.Close()
			cur = start
			open = false
		}
	})
	if open {
		out.Close()
	}
	return out
}

// TestBooleanLargeRadiusArcChainsStayOnCurve: a big circle flattens to many
// samples, some of them collinear on Clipper's integer grid (Clipper drops those
// from the solution). A broken chain degrades to a chord — never to a wrong
// curve — so the arcs that do come back must still sit on the authored circle.
func TestBooleanLargeRadiusArcChainsStayOnCurve(t *testing.T) {
	big := render.NewPath()
	big.Circle(0, 0, 300)
	src := cubicRuns(big)
	res := big.Op(boolRect(-400, -50, 800, 100), render.PathOpIntersect)
	if !res.HasCurves() {
		t.Fatal("large circle lost its arcs")
	}
	pts, kinds := curveSamples(res)
	nArc := 0
	worst := 0.0
	for i, p := range pts {
		if kinds[i] != render.CubicTo {
			continue
		}
		nArc++
		best := math.Inf(1)
		for _, c := range src {
			if v := distToCubic(c, p); v < best {
				best = v
			}
		}
		if best > worst {
			worst = best
		}
	}
	if nArc < 16 {
		t.Fatalf("expected arc samples, got %d", nArc)
	}
	if worst > 1e-3 {
		t.Fatalf("large-radius arcs stray from the authored circle by %v", worst)
	}
	// Area identity must hold at this scale too: |A∩B| + |A∪B| = |A| + |B|.
	band := boolRect(-400, -50, 800, 100)
	union := big.Op(band, render.PathOpUnion).Area()
	inter := res.Area()
	if e := math.Abs(union+inter-big.Area()-band.Area()) / big.Area(); e > 2e-3 {
		t.Fatalf("large-radius area identity off by %v (union=%v intersect=%v |A|=%v |B|=%v)",
			e, union, inter, big.Area(), band.Area())
	}
}

// cubicRuns collects the cubic segments a path is made of (start points carried
// through verbs, as the renderer sees them).
func cubicRuns(p *render.Path) []render.CubicBez {
	var out []render.CubicBez
	var cur render.Point
	p.Iterate(func(verb render.PathVerb, c []float64) {
		switch verb {
		case render.MoveTo:
			cur = render.Pt(c[0], c[1])
		case render.LineTo:
			cur = render.Pt(c[0], c[1])
		case render.CubicTo:
			next := render.Pt(c[4], c[5])
			out = append(out, render.NewCubicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), next))
			cur = next
		case render.QuadTo:
			cur = render.Pt(c[2], c[3])
		case render.ConicTo:
			cur = render.Pt(c[2], c[3])
		case render.Close:
		}
	})
	return out
}

// distToCubic is the distance from p to the cubic, by ternary search over
// sixteen brackets (no derivative, robust enough for an exactness pin).
func distToCubic(c render.CubicBez, p render.Point) float64 {
	worst := math.Inf(1)
	for i := 0; i < 16; i++ {
		lo := float64(i) / 16
		hi := float64(i+1) / 16
		for k := 0; k < 60; k++ {
			m1 := lo + (hi-lo)/3
			m2 := hi - (hi-lo)/3
			if d(c.Eval(m1), p) < d(c.Eval(m2), p) {
				hi = m2
			} else {
				lo = m1
			}
		}
		if v := d(c.Eval((lo+hi)/2), p); v < worst {
			worst = v
		}
	}
	return worst
}

func d(a, b render.Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// distToConic mirrors distToCubic for rational quadratics.
func distToConic(c render.ConicBez, p render.Point) float64 {
	worst := math.Inf(1)
	for i := 0; i < 16; i++ {
		lo := float64(i) / 16
		hi := float64(i+1) / 16
		for k := 0; k < 60; k++ {
			m1 := lo + (hi-lo)/3
			m2 := hi - (hi-lo)/3
			if d(c.Eval(m1), p) < d(c.Eval(m2), p) {
				hi = m2
			} else {
				lo = m1
			}
		}
		if v := d(c.Eval((lo+hi)/2), p); v < worst {
			worst = v
		}
	}
	return worst
}

// sampleCurvePoints walks the output and returns 17 samples of every curve verb
// (quad/cubic/conic), reporting the verb kind it came from.
func curveSamples(p *render.Path) (pts []render.Point, kinds []render.PathVerb) {
	var cur render.Point
	p.Iterate(func(verb render.PathVerb, c []float64) {
		switch verb {
		case render.MoveTo, render.LineTo:
			cur = render.Pt(c[0], c[1])
		case render.QuadTo:
			q := render.NewQuadBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]))
			for i := 0; i <= 16; i++ {
				pts = append(pts, q.Eval(float64(i)/16))
				kinds = append(kinds, render.QuadTo)
			}
			cur = q.P2
		case render.CubicTo:
			cb := render.NewCubicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), render.Pt(c[4], c[5]))
			for i := 0; i <= 16; i++ {
				pts = append(pts, cb.Eval(float64(i)/16))
				kinds = append(kinds, render.CubicTo)
			}
			cur = cb.P3
		case render.ConicTo:
			con := render.NewConicBez(cur, render.Pt(c[0], c[1]), render.Pt(c[2], c[3]), c[4])
			for i := 0; i <= 16; i++ {
				pts = append(pts, con.Eval(float64(i)/16))
				kinds = append(kinds, render.ConicTo)
			}
			cur = con.P2
		case render.Close:
		}
	})
	return pts, kinds
}

// TestBooleanStraightInputsStayPolygonal: no curve in, no curve out — the
// pre-batch-5 geometry path is untouched (same verbs, no curve verbs invented).
func TestBooleanStraightInputsStayPolygonal(t *testing.T) {
	outer := boolRect(16, 16, 480, 480)
	inner := boolRect(96, 96, 320, 320)
	res := outer.Op(inner, render.PathOpDifference)
	if res.HasCurves() {
		t.Fatal("polygonal inputs must not gain curve verbs")
	}
	if got := res.NumVerbs(); got != 10 {
		t.Fatalf("frame difference verbs = %d, want 10", got)
	}
	u := boolRect(0, 0, 100, 100).Op(boolRect(50, 0, 100, 100), render.PathOpUnion)
	if u.HasCurves() || u.NumVerbs() != 5 {
		t.Fatalf("overlapping squares union: verbs=%d curves=%v", u.NumVerbs(), u.HasCurves())
	}
}

// TestBooleanPreservesCubicRuns: a circle clipped by a rectangle comes back with
// cubic arcs, and every arc sample lies on the authored circle — not on a 0.1px
// chord chain.
func TestBooleanPreservesCubicRuns(t *testing.T) {
	circle := render.NewPath()
	circle.Circle(0, 0, 20)
	src := cubicRuns(circle)
	if len(src) != 4 {
		t.Fatalf("circle should be 4 cubics, got %d", len(src))
	}
	clip := boolRect(0, -25, 25, 50) // keeps the right half plus a rect tab

	res := circle.Op(clip, render.PathOpUnion)
	if !res.HasCurves() {
		t.Fatal("circle∪rect lost its arcs")
	}
	pts, kinds := curveSamples(res)
	curves := 0
	for _, k := range kinds {
		if k == render.CubicTo {
			curves++
		}
	}
	if curves < 32 {
		t.Fatalf("expected a run of cubic samples back, got %d", curves)
	}
	worst := 0.0
	for _, p := range pts {
		// Circle-authored points must sit on one of the original four cubics.
		best := math.Inf(1)
		for _, c := range src {
			if v := distToCubic(c, p); v < best {
				best = v
			}
		}
		// Rect-side output is straight, so only curve samples are examined; a
		// sample on the rect tab edge stays within its own authored curve.
		if best > 1e-6 && math.Hypot(p.X, p.Y) < 20.5 && math.Abs(math.Hypot(p.X, p.Y)-20) < 0.3 {
			if best > worst {
				worst = best
			}
		}
	}
	if worst > 1e-6 {
		t.Fatalf("refit arcs stray from the authored circle by %v", worst)
	}
	// Curve segments replaced chord chains: fewer verbs than the same result
	// sampled as a polygon (the "vs 全 flatten 段数" assertion of the assessment).
	if flat := len(res.Flatten(pathBoolProbeTol)); res.NumVerbs() >= flat {
		t.Fatalf("no chords collapsed: verbs=%d flatten=%d", res.NumVerbs(), flat)
	}
	if res.NumVerbs() > 16 {
		t.Fatalf("curve-preserving result should be compact, got %d verbs", res.NumVerbs())
	}
}

// TestBooleanUncutContourReproducedExactly: a contour that the operation never
// cuts comes back verb-for-verb as authored — the strongest statement the hybrid
// refit can make.
func TestBooleanUncutContourReproducedExactly(t *testing.T) {
	circle := render.NewPath()
	circle.Circle(0, 0, 20)
	far := boolRect(200, 200, 50, 50)

	u := circle.Op(far, render.PathOpUnion)
	if len(boolContours(u)) != 2 {
		t.Fatalf("disjoint union should keep both contours, got %d", len(boolContours(u)))
	}
	got := cubicRuns(u)
	want := cubicRuns(circle)
	if len(got) != len(want) {
		t.Fatalf("circle arcs reproduced as %d cubics, want %d", len(got), len(want))
	}
	for i := range want {
		if d(got[i].P0, want[i].P0) > 1e-9 || d(got[i].P1, want[i].P1) > 1e-9 ||
			d(got[i].P2, want[i].P2) > 1e-9 || d(got[i].P3, want[i].P3) > 1e-9 {
			t.Fatalf("cubic %d not reproduced exactly: %+v vs %+v", i, got[i], want[i])
		}
	}
	// The straight square stays straight and exact.
	sq := boolVertexSet(u)
	if !boolHasPoint(sq, 200, 200, 1e-9) || !boolHasPoint(sq, 250, 250, 1e-9) {
		t.Fatalf("square corners lost: %v", sq)
	}
}

// TestBooleanHoleKeepsExactArcsReversed: a difference that leaves the circle as a
// hole walks the authored runs backwards — the refit must flip each subsegment
// instead of emitting it forward from the wrong end.
func TestBooleanHoleKeepsExactArcsReversed(t *testing.T) {
	square := boolRect(-40, -40, 80, 80)
	circle := render.NewPath()
	circle.Circle(0, 0, 20)

	res := square.Op(circle, render.PathOpDifference)
	contours := boolContours(res)
	if len(contours) != 2 {
		t.Fatalf("square-circle hole should give 2 contours, got %d", len(contours))
	}
	if !res.HasCurves() {
		t.Fatal("hole lost its arcs")
	}
	src := cubicRuns(circle)
	pts, kinds := curveSamples(res)
	nCubic := 0
	for _, k := range kinds {
		if k == render.CubicTo {
			nCubic++
		}
	}
	if nCubic < 32 {
		t.Fatalf("hole should carry cubic samples, got %d", nCubic)
	}
	worst := 0.0
	for _, p := range pts {
		best := math.Inf(1)
		for _, c := range src {
			if v := distToCubic(c, p); v < best {
				best = v
			}
		}
		if best > worst {
			worst = best
		}
	}
	// Curve endpoints join Clipper's grid vertices, which carry the 1/1024px
	// quantization the polygon already had — three orders tighter than the 0.1px
	// chords the refit replaced.
	if worst > 1e-3 {
		t.Fatalf("reversed hole arcs off the authored circle by %v", worst)
	}
	// Hole orientation must oppose the outer contour (non-zero winding).
	if !boolInside(res, -35, 0) || boolInside(res, 0, 0) {
		t.Fatal("hole is not a hole")
	}
}

// TestBooleanAreaReconciliation is the assessment's identity check:
// |A∪B| = |A| + |B| − |A∩B|, and |A−B| = |A| − |A∩B|, on curve-accurate areas.
func TestBooleanAreaReconciliation(t *testing.T) {
	circle := render.NewPath()
	circle.Circle(0, 0, 20)
	blob := render.NewPath()
	blob.MoveTo(10, 10)
	blob.CubicTo(60, -20, 90, 70, 30, 90)
	blob.QuadraticTo(-10, 80, 5, 40)
	blob.Close()
	tilted := boolRect(5, -12, 44, 30)

	cases := []struct {
		name string
		a, b *render.Path
	}{
		{"circle/rect", circle, tilted},
		{"circle/blob", circle, blob},
		{"blob/rect", blob, tilted},
		{"circle/disjoint-rect", circle, boolRect(500, 500, 40, 40)},
		{"square/circle-hole", boolRect(-40, -40, 80, 80), circle},
	}
	for _, tc := range cases {
		a, b := tc.a.Area(), tc.b.Area()
		u := tc.a.Op(tc.b, render.PathOpUnion).Area()
		i := tc.a.Op(tc.b, render.PathOpIntersect).Area()
		dif := tc.a.Op(tc.b, render.PathOpDifference).Area()
		xor := tc.a.Op(tc.b, render.PathOpXor).Area()
		scale := math.Max(a, b)
		if scale <= 0 {
			scale = 1
		}
		if e := math.Abs(u+i-a-b) / scale; e > 5e-4 {
			t.Errorf("%s: union+intersect identity off by %v (a=%v b=%v u=%v i=%v)", tc.name, e, a, b, u, i)
		}
		if e := math.Abs(dif-(a-i)) / scale; e > 5e-4 {
			t.Errorf("%s: difference identity off by %v (a=%v i=%v d=%v)", tc.name, e, a, i, dif)
		}
		if e := math.Abs(xor-(a+b-2*i)) / scale; e > 5e-4 {
			t.Errorf("%s: xor identity off by %v (a=%v b=%v i=%v x=%v)", tc.name, e, a, b, i, xor)
		}
	}
}

// TestBooleanConicInputKeepsConicVerb: conic arcs come back as conics, with the
// authored weight and points on the authored curve.
func TestBooleanConicInputKeepsConicVerb(t *testing.T) {
	con := render.NewPath()
	con.MoveTo(0, 0)
	con.ConicTo(60, -50, 120, 0, 0.6)
	con.LineTo(120, 40)
	con.LineTo(0, 40)
	con.Close()
	clip := boolRect(20, -60, 60, 200) // cuts both sides of the conic arc

	res := con.Op(clip, render.PathOpIntersect)
	if !res.HasConics() {
		t.Fatal("conic input lost its conic verbs")
	}
	var weights []float64
	res.Iterate(func(verb render.PathVerb, c []float64) {
		if verb == render.ConicTo {
			weights = append(weights, c[4])
		}
	})
	// A rational sub-arc has its own weight (SkConic::Subpart likewise), so the
	// pin is that the refit stays a genuine conic of the same family, not that
	// the number survives.
	for _, w := range weights {
		if !(w > 0) || math.IsNaN(w) || math.IsInf(w, 0) {
			t.Fatalf("refit emitted a conic with a bad weight: %v", w)
		}
	}
	// Every emitted conic sample must sit on the authored conic.
	src := render.NewConicBez(render.Pt(0, 0), render.Pt(60, -50), render.Pt(120, 0), 0.6)
	pts, kinds := curveSamples(res)
	worst := 0.0
	for i, p := range pts {
		if kinds[i] != render.ConicTo {
			continue
		}
		best := distToConic(src, p)
		if best > worst {
			worst = best
		}
	}
	if worst > 1e-3 {
		t.Fatalf("refit conic samples stray from the authored conic by %v", worst)
	}
	if len(weights) == 0 {
		t.Fatal("no conic verbs collected")
	}
}

// TestBooleanCurveResultStaysInsideOriginalBoundary: the arcs that come back must
// not wander outside the region the flattened boolean produced — inside/outside
// classification is unchanged by the refit.
func TestBooleanCurveResultStaysInsideOriginalBoundary(t *testing.T) {
	circle := render.NewPath()
	circle.Circle(0, 0, 20)
	clip := boolRect(0, -25, 25, 50)
	res := circle.Op(clip, render.PathOpUnion)

	for _, in := range [][2]float64{{-15, 0}, {18, 18}, {2, 19}, {24, 0}, {19.9, 0.5}, {1, 20.6}} {
		if !boolInside(res, in[0], in[1]) {
			t.Errorf("(%v,%v) should be inside circle∪rect", in[0], in[1])
		}
	}
	for _, out := range [][2]float64{{-21, 0}, {-15, 15}, {-24, -24}, {26, 0}, {-0.5, 26}, {0, 26}} {
		if boolInside(res, out[0], out[1]) {
			t.Errorf("(%v,%v) should be outside circle∪rect", out[0], out[1])
		}
	}
}
