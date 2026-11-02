//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"math"
	"testing"
)

// Signed-area convention regression pins (P3.5): every verb helper must report
// the SAME Green form ½∫(x·dy − y·dx). Each helper is checked against a dense
// polyline of its own curve, and mixed line+curve paths are checked against
// analytic areas — the case the conventions used to disagree on.

// denseSignedArea integrates ½(x dy − y dx) along the curve by sampling.
type curveEval interface{ Eval(t float64) Point }

func denseSignedArea(c curveEval, n int) float64 {
	sum, prev := 0.0, c.Eval(0)
	for i := 1; i <= n; i++ {
		p := c.Eval(float64(i) / float64(n))
		sum += 0.5 * (prev.X*p.Y - p.X*prev.Y)
		prev = p
	}
	return sum
}

func TestQuadAreaUsesSymmetricGreenForm(t *testing.T) {
	cases := []QuadBez{
		NewQuadBez(Pt(120, 50), Pt(320, 50), Pt(320, 250)),
		NewQuadBez(Pt(0, 0), Pt(50, 40), Pt(100, 0)),
		NewQuadBez(Pt(-30, -12), Pt(7, 91), Pt(58, -44)),
	}
	for i, q := range cases {
		got := quadArea(q.P0, q.P1, q.P2)
		want := denseSignedArea(q, 200000)
		if d := math.Abs(got - want); d > 1e-6 {
			t.Errorf("quad %d: quadArea=%v dense=%v diff=%v (∫x dy leaked into the sum?)", i, got, want, d)
		}
	}
}

func TestLineAndCubicAreaConventionsMatchDense(t *testing.T) {
	l0, l1 := Pt(1, 2), Pt(30, 45)
	if d := math.Abs(lineArea(l0, l1) - denseSignedArea(segCurve{l0, l1}, 100000)); d > 1e-9 {
		t.Errorf("lineArea vs dense diff %v", d)
	}
	c := NewCubicBez(Pt(10, 10), Pt(60, -20), Pt(90, 70), Pt(30, 90))
	if d := math.Abs(cubicArea(c.P0, c.P1, c.P2, c.P3) - denseSignedArea(c, 200000)); d > 1e-6 {
		t.Errorf("cubicArea vs dense diff %v", d)
	}
}

func TestConicAreaMatchesDenseOnItsOwnArc(t *testing.T) {
	con := NewConicBez(Pt(0, 0), Pt(60, -40), Pt(110, 25), 0.6)
	got := conicArea(con.P0, con.P1, con.P2, con.W)
	want := denseSignedArea(con, 200000)
	// ToQuads(1e-3) approximates the arc, so the area carries that tolerance.
	if d := math.Abs(got - want); d > 2e-3*100 {
		t.Errorf("conicArea=%v dense=%v diff=%v beyond degradation tolerance", got, want, d)
	}
	if math.Abs(conicArea(con.P0, con.P1, con.P2, 1.0)-quadArea(con.P0, con.P1, con.P2)) > 1e-12 {
		t.Error("w=1 conic must fall through to the quadratic term")
	}
}

type segCurve struct{ a, b Point }

func (s segCurve) Eval(t float64) Point {
	return Pt(s.a.X+(s.b.X-s.a.X)*t, s.a.Y+(s.b.Y-s.a.Y)*t)
}

// TestMixedSectorAreaExact is the regression itself: centre + two radii + one
// exact quarter arc. With the old mixed conventions the area was inflated by
// ½(x_end·y_end − x_start·y_start) = 37000 for r=200.
func TestMixedSectorAreaExact(t *testing.T) {
	for _, r := range []float64{20, 200, 97.5} {
		cx, cy := 120.0, 250.0
		k := 2 * math.Cos(math.Pi/4) * math.Cos(math.Pi/4)
		p0 := Pt(cx, cy-r)
		p2 := Pt(cx+r, cy)
		ctrl := Pt(cx+((p0.X-cx)+(p2.X-cx))/k, cy+((p0.Y-cy)+(p2.Y-cy))/k)
		sec := NewPath()
		sec.MoveTo(cx, cy)
		sec.LineTo(p0.X, p0.Y)
		sec.ConicTo(ctrl.X, ctrl.Y, p2.X, p2.Y, math.Cos(math.Pi/4))
		sec.Close()
		want := math.Pi * r * r / 4
		// conicArea sums ToQuads(1e-3) pieces, so it carries that approximation
		// budget instead of exact-circle error.
		if rel := math.Abs(sec.Area()-want) / want; rel > 1e-6 {
			t.Errorf("r=%v conic sector Area()=%v want πr²/4=%v rel=%.2e", r, sec.Area(), want, rel)
		}
		// Same sector with the arc degraded to a parabola: its area must equal
		// the dense integral of that parabola (mixed line+curve, second shape).
		quad := NewPath()
		quad.MoveTo(cx, cy)
		quad.LineTo(p0.X, p0.Y)
		quad.QuadraticTo(ctrl.X, ctrl.Y, p2.X, p2.Y)
		quad.Close()
		paraDense := NewPath()
		paraDense.MoveTo(cx, cy)
		paraDense.LineTo(p0.X, p0.Y)
		pq := NewQuadBez(p0, ctrl, p2)
		for i := 1; i <= 20000; i++ {
			e := pq.Eval(float64(i) / 20000)
			paraDense.LineTo(e.X, e.Y)
		}
		paraDense.Close()
		if d := math.Abs(quad.Area() - paraDense.Area()); d > math.Abs(paraDense.Area())*1e-6 {
			t.Errorf("r=%v parabolic sector Area()=%v dense=%v diff=%v", r, quad.Area(), paraDense.Area(), d)
		}
		dense := NewPath()
		dense.MoveTo(cx, cy)
		dense.LineTo(p0.X, p0.Y)
		const n = 20000
		for i := 1; i <= n; i++ {
			a := -math.Pi/2 + (math.Pi/2)*float64(i)/n
			dense.LineTo(cx+r*math.Cos(a), cy+r*math.Sin(a))
		}
		dense.Close()
		if math.Abs(dense.Area()-want) > want*1e-6 {
			t.Errorf("r=%v dense sector Area()=%v want %v", r, dense.Area(), want)
		}
	}
}

// densePolylineOf turns every curve verb into `per` line segments, producing a
// pure-line reference path: its Area() is the shoelace form alone, so a curve
// path agreeing with it proves the curve helpers use the same convention.
func densePolylineOf(p *Path, per int) *Path {
	out := NewPath()
	var cur, start Point
	open := false
	p.Iterate(func(v PathVerb, c []float64) {
		switch v {
		case MoveTo:
			cur = Pt(c[0], c[1])
			start = cur
			if open {
				out.Close()
			}
			out.MoveTo(cur.X, cur.Y)
			open = true
		case LineTo:
			cur = Pt(c[0], c[1])
			out.LineTo(cur.X, cur.Y)
		case QuadTo:
			q := NewQuadBez(cur, Pt(c[0], c[1]), Pt(c[2], c[3]))
			for i := 1; i <= per; i++ {
				e := q.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = q.P2
		case CubicTo:
			cb := NewCubicBez(cur, Pt(c[0], c[1]), Pt(c[2], c[3]), Pt(c[4], c[5]))
			for i := 1; i <= per; i++ {
				e := cb.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = cb.P3
		case ConicTo:
			con := NewConicBez(cur, Pt(c[0], c[1]), Pt(c[2], c[3]), c[4])
			for i := 1; i <= per; i++ {
				e := con.Eval(float64(i) / float64(per))
				out.LineTo(e.X, e.Y)
			}
			cur = con.P2
		case Close:
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

// TestRoundedRectMixedAreaAgainstDenseReference covers the most common mixed
// path in the UI: straight edges plus quadratic corners.
func TestRoundedRectMixedAreaAgainstDenseReference(t *testing.T) {
	w, h, r := 100.0, 60.0, 20.0
	p := NewPath()
	p.RoundedRectangle(0, 0, w, h, r)

	dense := densePolylineOf(p, 4000)
	if rel := math.Abs(p.Area()-dense.Area()) / math.Abs(dense.Area()); rel > 1e-5 {
		t.Errorf("mixed path Area()=%v vs pure-line reference=%v rel=%.2e", p.Area(), dense.Area(), rel)
	}

	// Against the ideal circular corners the quadratic approximation deviates
	// by the classic segment-area difference — informational bound, the dense
	// reference above is the exact gate.
	ideal := w*h - (4.0-math.Pi)*r*r
	if rel := math.Abs(p.Area()-ideal) / math.Abs(ideal); rel > 1e-3 {
		t.Errorf("rounded rect Area()=%v vs ideal circular corners=%v rel=%.2e", p.Area(), ideal, rel)
	}

	// Orientation flip must flip the sign only.
	rev := p.Reversed()
	if math.Abs(math.Abs(rev.Area())-math.Abs(p.Area())) > math.Abs(p.Area())*1e-9 {
		t.Errorf("Reversed() changed |Area()|: %v vs %v", rev.Area(), p.Area())
	}
}

// TestMixedAreaEqualsDenseForManyShapes sweeps mixed paths (radii + arcs + lines
// + conics) against the pure-line reference; each shape fails loudly under the
// old mixed conventions by a per-curve-run ½[x·y] offset.
func TestMixedAreaEqualsDenseForManyShapes(t *testing.T) {
	shapes := []*Path{}
	// Pie sector with a conic arc, offset away from the axes so the telescoping
	// term cannot vanish by luck (the historical blind spot).
	for _, off := range [][2]float64{{120, 250}, {-33, 71}, {512, 512}} {
		cx, cy := off[0], off[1]
		r := 37.5
		k := 2 * math.Cos(math.Pi/4) * math.Cos(math.Pi/4)
		p0 := Pt(cx, cy-r)
		p2 := Pt(cx+r, cy)
		ctrl := Pt(cx+((p0.X-cx)+(p2.X-cx))/k, cy+((p0.Y-cy)+(p2.Y-cy))/k)
		sec := NewPath()
		sec.MoveTo(cx, cy)
		sec.LineTo(p0.X, p0.Y)
		sec.ConicTo(ctrl.X, ctrl.Y, p2.X, p2.Y, math.Cos(math.Pi/4))
		sec.Close()
		shapes = append(shapes, sec)
	}
	// Line + quadratic + cubic + line chain.
	mixed := NewPath()
	mixed.MoveTo(3, 17)
	mixed.LineTo(57, 17)
	mixed.QuadraticTo(80, 40, 57, 63)
	mixed.CubicTo(30, 80, 10, 55, 3, 17)
	mixed.Close()
	shapes = append(shapes, mixed)
	// Circle (all cubics) translated off-axis, plus a rounded rect with conics.
	circ := NewPath()
	circ.Circle(33, 44, 21)
	shapes = append(shapes, circ)
	rr := NewPath()
	rr.RoundedRectangle(11, 9, 90, 40, 13)
	shapes = append(shapes, rr)

	for i, s := range shapes {
		dense := densePolylineOf(s, 6000)
		rel := math.Abs(s.Area()-dense.Area()) / math.Max(1, math.Abs(dense.Area()))
		if rel > 1e-5 {
			t.Errorf("shape %d: Area()=%v dense reference=%v rel=%.2e", i, s.Area(), dense.Area(), rel)
		}
	}
}
