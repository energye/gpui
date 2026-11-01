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
)

// ConicBez is a rational quadratic Bezier curve (a conic section):
//
//	B(t) = ((1-t)² P0 + 2(1-t)t·W·P1 + t² P2) / ((1-t)² + 2(1-t)t·W + t²)
//
// W < 0 is invalid; W == 0 degenerates to the line P0P2; W == 1 is exactly a
// quadratic Bezier (parabola); W == sqrt(2)/2 traces a quarter circular arc
// when P0/P1/P2 form the standard quarter-circle control polygon. An affine
// image of a conic is a conic with the same weight.
//
// Subdivision notes: splitting via homogeneous de Casteljau preserves the
// point set of the arc (it may reparameterize, which rendering does not care
// about). Sub-weights converge to 1 geometrically, which is what makes
// ToQuads converge.
type ConicBez struct {
	P0, P1, P2 Point
	W          float64
}

// NewConicBez creates a conic from endpoints, control point and weight.
func NewConicBez(p0, p1, p2 Point, w float64) ConicBez {
	return ConicBez{P0: p0, P1: p1, P2: p2, W: w}
}

// Eval evaluates the curve at parameter t (0 to 1).
func (c ConicBez) Eval(t float64) Point {
	mt := 1.0 - t
	nx := mt*mt*c.P0.X + 2*mt*t*c.W*c.P1.X + t*t*c.P2.X
	ny := mt*mt*c.P0.Y + 2*mt*t*c.W*c.P1.Y + t*t*c.P2.Y
	d := mt*mt + 2*mt*t*c.W + t*t
	if d == 0 {
		return c.P2
	}
	return Point{X: nx / d, Y: ny / d}
}

// Start returns the starting point of the curve.
func (c ConicBez) Start() Point {
	return c.P0
}

// End returns the ending point of the curve.
func (c ConicBez) End() Point {
	return c.P2
}

// Chop splits the curve at t=0.5 into two conics covering the left and right
// halves of the arc. Both halves carry the new weight sqrt((1+W)/2) — for a
// circular arc this is exactly cos(θ/4) when W = cos(θ/2), so the split is
// the geometric half as well.
func (c ConicBez) Chop() (ConicBez, ConicBez) {
	den := 1.0 + c.W
	wp := c.W * c.P1.X
	wq := c.W * c.P1.Y
	p01 := Point{X: (c.P0.X + wp) / den, Y: (c.P0.Y + wq) / den}
	p12 := Point{X: (wp + c.P2.X) / den, Y: (wq + c.P2.Y) / den}
	// B(1/2) = (P0 + 2W·P1 + P2) / (2 + 2W)
	mid := Point{
		X: (c.P0.X + 2*wp + c.P2.X) / (2 * den),
		Y: (c.P0.Y + 2*wq + c.P2.Y) / (2 * den),
	}
	w1 := math.Sqrt((1.0 + c.W) / 2.0)
	left := ConicBez{P0: c.P0, P1: p01, P2: mid, W: w1}
	right := ConicBez{P0: mid, P1: p12, P2: c.P2, W: w1}
	return left, right
}

// Subsegment returns the portion of the curve from t0 to t1 as a conic in
// standard form, using the exact rational-quadratic subdivision identities
// (consistent with Chop at t=1/2):
//
//	left piece  [0,t]:  ctrl = ((1-t)·P0 + t·W·P1) / ((1-t)+t·W)
//	                    weight = ((1-t)+t·W) / sqrt(D(t))
//	right piece [t,1]:  ctrl = (t·P2 + (1-t)·W·P1) / (t+(1-t)·W)
//	                    weight = (t+(1-t)·W) / sqrt(D(t))
//
// with D(t) the rational denominator. Interior ranges compose a left piece of
// [0,t1] with its own right piece at t0/t1.
func (c ConicBez) Subsegment(t0, t1 float64) ConicBez {
	if t0 <= 0 && t1 >= 1 {
		return c
	}
	if t0 >= t1 {
		mid := c.Eval(t0)
		return ConicBez{P0: mid, P1: mid, P2: mid, W: 1}
	}
	if t1 >= 1 {
		return conicRightPiece(c, t0)
	}
	if t0 <= 0 {
		return conicLeftPiece(c, t1)
	}
	// Interior range: take the [0,t1] left piece (its standard-form parameter
	// is a Möbius transform s = λr/(1-r+λr) of the scaled original parameter,
	// λ = 1/sqrt(D(t1))), then cut its right piece at the transformed split.
	seg := conicLeftPiece(c, t1)
	lam := 1 / math.Sqrt(c.conicDenom(t1))
	s0 := t0 / t1
	r0 := s0 / (lam*(1-s0) + s0)
	return conicRightPiece(seg, r0)
}

// conicDenom is the rational denominator D(t) = (1-t)² + 2(1-t)t·W + t².
func (c ConicBez) conicDenom(t float64) float64 {
	mt := 1.0 - t
	return mt*mt + 2*mt*t*c.W + t*t
}

// conicLeftPiece returns the [0, t] portion in standard form.
func conicLeftPiece(c ConicBez, t float64) ConicBez {
	mt := 1.0 - t
	h := mt + t*c.W
	if h == 0 {
		h = 1
	}
	return ConicBez{
		P0: c.P0,
		P1: Point{
			X: (mt*c.P0.X + t*c.W*c.P1.X) / h,
			Y: (mt*c.P0.Y + t*c.W*c.P1.Y) / h,
		},
		P2: c.Eval(t),
		W:  h / math.Sqrt(c.conicDenom(t)),
	}
}

// conicRightPiece returns the [t, 1] portion in standard form.
func conicRightPiece(c ConicBez, t float64) ConicBez {
	mt := 1.0 - t
	h := t + mt*c.W
	if h == 0 {
		h = 1
	}
	return ConicBez{
		P0: c.Eval(t),
		P1: Point{
			X: (t*c.P2.X + mt*c.W*c.P1.X) / h,
			Y: (t*c.P2.Y + mt*c.W*c.P1.Y) / h,
		},
		P2: c.P2,
		W:  h / math.Sqrt(c.conicDenom(t)),
	}
}

// Tangent returns the curve tangent direction at t (not normalized).
func (c ConicBez) Tangent(t float64) Point {
	mt := 1.0 - t
	// B'(t) = (N'(t)·D(t) − N(t)·D'(t)) / D(t)²; the direction is enough.
	nxd := 2 * (mt*(c.W*c.P1.X-c.P0.X) + t*(c.P2.X-c.W*c.P1.X))
	nyd := 2 * (mt*(c.W*c.P1.Y-c.P0.Y) + t*(c.P2.Y-c.W*c.P1.Y))
	nx := mt*mt*c.P0.X + 2*mt*t*c.W*c.P1.X + t*t*c.P2.X
	ny := mt*mt*c.P0.Y + 2*mt*t*c.W*c.P1.Y + t*t*c.P2.Y
	d := mt*mt + 2*mt*t*c.W + t*t
	dd := 2 * (mt*(c.W-1) + t*(1-c.W))
	return Point{
		X: nxd*d - nx*dd,
		Y: nyd*d - ny*dd,
	}
}

// Extrema returns parameter values in (0, 1) where the x or y derivative is
// zero, sorted ascending. Used for tight bounding boxes. The derivative of a
// rational quadratic is a cubic; its roots in (0, 1) are located by scanning
// for sign changes and bisecting (robust, and bbox callers are not hot paths).
func (c ConicBez) Extrema() []float64 {
	var result []float64
	for _, xAxis := range []bool{true, false} {
		f := func(t float64) float64 { return c.derivNumerator(t, xAxis) }
		result = append(result, cubicRootsInUnit(f)...)
	}
	sortFloat64s(result)
	return result
}

// derivNumerator is the x (xAxis) or y derivative numerator of the rational
// quadratic: N'(t)·D(t) − N(t)·D'(t), a cubic polynomial in t.
func (c ConicBez) derivNumerator(t float64, xAxis bool) float64 {
	mt := 1.0 - t
	var p0, p1, p2 float64
	if xAxis {
		p0, p1, p2 = c.P0.X, c.W*c.P1.X, c.P2.X
	} else {
		p0, p1, p2 = c.P0.Y, c.W*c.P1.Y, c.P2.Y
	}
	nd := 2 * (mt*(p1-p0) + t*(p2-p1))
	n := mt*mt*p0 + 2*mt*t*p1 + t*t*p2
	d := mt*mt + 2*mt*t*c.W + t*t
	dd := 2 * (mt*(c.W-1) + t*(1-c.W))
	return nd*d - n*dd
}

// cubicRootsInUnit finds all roots of f in (0, 1) by scanning for sign
// changes and bisecting each bracket. Cubics over [0, 1] arising from conic
// derivatives have few roots; a 64-sample scan resolves every sign change at
// the resolution bounding boxes need.
func cubicRootsInUnit(f func(float64) float64) []float64 {
	var roots []float64
	const n = 64
	prevT := 0.0
	prevV := f(0)
	for i := 1; i <= n; i++ {
		t := float64(i) / n
		v := f(t)
		if prevV == 0 {
			roots = append(roots, prevT)
		} else if (prevV < 0 && v > 0) || (prevV > 0 && v < 0) {
			lo, hi := prevT, t
			flo := prevV
			for hi-lo > 1e-10 {
				mid := (lo + hi) / 2
				fm := f(mid)
				if fm == 0 {
					lo, hi = mid, mid
					break
				}
				if (flo < 0 && fm < 0) || (flo > 0 && fm > 0) {
					lo, flo = mid, fm
				} else {
					hi = mid
				}
			}
			roots = append(roots, (lo+hi)/2)
		}
		prevT, prevV = t, v
	}
	return roots
}

func sortFloat64s(a []float64) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// BoundingBox returns the tight axis-aligned bounding box of the curve.
func (c ConicBez) BoundingBox() Rect {
	bbox := NewRect(c.P0, c.P2)
	for _, t := range c.Extrema() {
		p := c.Eval(t)
		bbox = bbox.Union(NewRect(p, p))
	}
	return bbox
}

// ToQuads approximates the conic with quadratic Beziers within tol distance
// (in path units), emitting each quad via fn. Each emitted quad interpolates
// the conic at its own endpoints and midpoint. Recursion splits the conic at
// t=0.5; sub-weights converge to 1, so the approximation error quarters with
// each level of depth.
func (c ConicBez) ToQuads(tol float64, fn func(QuadBez)) {
	const maxDepth = 18
	var rec func(ConicBez, int)
	rec = func(cur ConicBez, depth int) {
		q := cur.quadApprox()
		if depth >= maxDepth || cur.quadError(q) <= tol {
			fn(q)
			return
		}
		l, r := cur.Chop()
		rec(l, depth+1)
		rec(r, depth+1)
	}
	rec(c, 0)
}

// quadApprox returns the quadratic Bezier that passes through this conic's
// endpoints and its midpoint at t=0.5: control = 2·B(1/2) − (P0+P2)/2.
func (c ConicBez) quadApprox() QuadBez {
	mid := c.Eval(0.5)
	ctrl := Point{
		X: 2*mid.X - (c.P0.X+c.P2.X)/2,
		Y: 2*mid.Y - (c.P0.Y+c.P2.Y)/2,
	}
	return QuadBez{P0: c.P0, P1: ctrl, P2: c.P2}
}

// quadError is the max deviation between the conic and its quad approximation
// sampled at the eighth points (both curves agree at 0, 1/2 and 1 by
// construction; the remaining samples bound the error for this family).
func (c ConicBez) quadError(q QuadBez) float64 {
	worst := 0.0
	for _, t := range []float64{0.125, 0.25, 0.375, 0.625, 0.75, 0.875} {
		cc := c.Eval(t)
		qq := q.Eval(t)
		dx := cc.X - qq.X
		dy := cc.Y - qq.Y
		if d := math.Sqrt(dx*dx + dy*dy); d > worst {
			worst = d
		}
	}
	return worst
}

// HasConics reports whether the path contains any conic (rational quadratic)
// segments.
func (p *Path) HasConics() bool {
	for _, v := range p.verbs {
		if v == ConicTo {
			return true
		}
	}
	return false
}

// ExpandConicsToQuads returns a path with every conic replaced by quadratic
// Beziers interpolating it within tol. Paths without conics are returned
// unchanged (same pointer, zero cost). Subsystem boundaries that cannot carry
// the ConicTo verb (raster/stroke/clip/scene formats) call this so a conic
// never reaches a switch that would silently skip it.
func (p *Path) ExpandConicsToQuads(tol float64) *Path {
	if p == nil || !p.HasConics() {
		return p
	}
	if tol <= 0 {
		tol = 0.1
	}
	out := NewPath()
	var cur Point
	p.Iterate(func(verb PathVerb, coords []float64) {
		switch verb {
		case MoveTo:
			cur = Pt(coords[0], coords[1])
			out.MoveTo(cur.X, cur.Y)
		case LineTo:
			cur = Pt(coords[0], coords[1])
			out.LineTo(cur.X, cur.Y)
		case QuadTo:
			cur = Pt(coords[2], coords[3])
			out.QuadraticTo(coords[0], coords[1], cur.X, cur.Y)
		case CubicTo:
			cur = Pt(coords[4], coords[5])
			out.CubicTo(coords[0], coords[1], coords[2], coords[3], cur.X, cur.Y)
		case ConicTo:
			end := Pt(coords[2], coords[3])
			NewConicBez(cur, Pt(coords[0], coords[1]), end, coords[4]).ToQuads(tol, func(q QuadBez) {
				out.QuadraticTo(q.P1.X, q.P1.Y, q.P2.X, q.P2.Y)
			})
			cur = end
		case Close:
			out.Close()
			cur = out.start
		}
	})
	return out
}
