package core

import (
	"math"

	"github.com/energye/gpui/render"
)

// Vec2 is a 2D vector in game units. It mirrors render.Vec2/render.Point
// field for field but stays a separate type so game math never depends on
// render internals. Convert once at the render boundary.
type Vec2 struct {
	X, Y float64
}

// V2 builds a Vec2.
func V2(x, y float64) Vec2 { return Vec2{X: x, Y: y} }

// Add returns v+w.
func (v Vec2) Add(w Vec2) Vec2 { return Vec2{v.X + w.X, v.Y + w.Y} }

// Sub returns v-w.
func (v Vec2) Sub(w Vec2) Vec2 { return Vec2{v.X - w.X, v.Y - w.Y} }

// Mul scales v by s.
func (v Vec2) Mul(s float64) Vec2 { return Vec2{v.X * s, v.Y * s} }

// Div scales v by 1/s. Zero s follows IEEE-754 (no panic).
func (v Vec2) Div(s float64) Vec2 { return Vec2{v.X / s, v.Y / s} }

// Neg returns -v.
func (v Vec2) Neg() Vec2 { return Vec2{-v.X, -v.Y} }

// Dot returns the dot product.
func (v Vec2) Dot(w Vec2) float64 { return v.X*w.X + v.Y*w.Y }

// Cross returns the 2D cross product (scalar).
func (v Vec2) Cross(w Vec2) float64 { return v.X*w.Y - v.Y*w.X }

// Length returns the magnitude.
func (v Vec2) Length() float64 { return math.Sqrt(v.X*v.X + v.Y*v.Y) }

// LengthSq returns the squared magnitude (cheaper for comparisons).
func (v Vec2) LengthSq() float64 { return v.X*v.X + v.Y*v.Y }

// Normalize returns the unit vector. Zero vector stays zero (no NaN).
func (v Vec2) Normalize() Vec2 {
	l := v.Length()
	if l == 0 {
		return Vec2{}
	}
	return Vec2{v.X / l, v.Y / l}
}

// Lerp interpolates: t=0 returns v, t=1 returns w.
func (v Vec2) Lerp(w Vec2, t float64) Vec2 {
	return Vec2{v.X + (w.X-v.X)*t, v.Y + (w.Y-v.Y)*t}
}

// Rotate returns v rotated by rad radians counter-clockwise.
func (v Vec2) Rotate(rad float64) Vec2 {
	c, s := math.Cos(rad), math.Sin(rad)
	return Vec2{v.X*c - v.Y*s, v.X*s + v.Y*c}
}

// Perp returns v rotated 90 degrees counter-clockwise.
func (v Vec2) Perp() Vec2 { return Vec2{-v.Y, v.X} }

// Atan2 returns the angle of v in radians.
func (v Vec2) Atan2() float64 { return math.Atan2(v.Y, v.X) }

// Angle returns the signed angle from v to w in radians.
func (v Vec2) Angle(w Vec2) float64 { return math.Atan2(v.Cross(w), v.Dot(w)) }

// IsZero reports whether v is exactly the zero vector.
func (v Vec2) IsZero() bool { return v.X == 0 && v.Y == 0 }

// ApproxEqual reports whether v and w differ by less than eps per component.
func (v Vec2) ApproxEqual(w Vec2, eps float64) bool {
	return math.Abs(v.X-w.X) < eps && math.Abs(v.Y-w.Y) < eps
}

// ToRenderPoint converts to render.Point at the render boundary.
func (v Vec2) ToRenderPoint() render.Point { return render.Point{X: v.X, Y: v.Y} }

// Vec2FromRenderPoint converts a render.Point back to game units.
func Vec2FromRenderPoint(p render.Point) Vec2 { return Vec2{X: p.X, Y: p.Y} }

// Rect is an axis-aligned box: origin (X,Y) plus size (W,H).
type Rect struct {
	X, Y, W, H float64
}

// NewRect builds a Rect.
func NewRect(x, y, w, h float64) Rect { return Rect{X: x, Y: y, W: w, H: h} }

// IsEmpty reports whether the rect has no area.
func (r Rect) IsEmpty() bool { return r.W <= 0 || r.H <= 0 }

// Area returns W*H, or 0 for an empty rect.
func (r Rect) Area() float64 {
	if r.IsEmpty() {
		return 0
	}
	return r.W * r.H
}

// Center returns the center point.
func (r Rect) Center() Vec2 { return Vec2{r.X + r.W/2, r.Y + r.H/2} }

// Contains reports whether p is inside: min edge inclusive, max exclusive.
// An empty rect contains nothing.
func (r Rect) Contains(p Vec2) bool {
	if r.IsEmpty() {
		return false
	}
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

// ContainsRect reports whether o lies fully inside r.
func (r Rect) ContainsRect(o Rect) bool {
	if r.IsEmpty() || o.IsEmpty() {
		return false
	}
	return o.X >= r.X && o.Y >= r.Y && o.X+o.W <= r.X+r.W && o.Y+o.H <= r.Y+r.H
}

// Intersects reports whether r and o overlap with positive area.
func (r Rect) Intersects(o Rect) bool {
	_, ok := r.Intersection(o)
	return ok
}

// Intersection returns the overlap box, or false when there is none.
func (r Rect) Intersection(o Rect) (Rect, bool) {
	x1 := math.Max(r.X, o.X)
	y1 := math.Max(r.Y, o.Y)
	x2 := math.Min(r.X+r.W, o.X+o.W)
	y2 := math.Min(r.Y+r.H, o.Y+o.H)
	if x2 <= x1 || y2 <= y1 {
		return Rect{}, false
	}
	return Rect{x1, y1, x2 - x1, y2 - y1}, true
}

// Union returns the smallest box holding both. Empty side is ignored.
func (r Rect) Union(o Rect) Rect {
	if r.IsEmpty() {
		return o
	}
	if o.IsEmpty() {
		return r
	}
	x1 := math.Min(r.X, o.X)
	y1 := math.Min(r.Y, o.Y)
	x2 := math.Max(r.X+r.W, o.X+o.W)
	y2 := math.Max(r.Y+r.H, o.Y+o.H)
	return Rect{x1, y1, x2 - x1, y2 - y1}
}

// Offset moves the rect by d.
func (r Rect) Offset(d Vec2) Rect { return Rect{r.X + d.X, r.Y + d.Y, r.W, r.H} }

// Inset shrinks (positive dx/dy) or grows (negative) the rect.
func (r Rect) Inset(dx, dy float64) Rect {
	return Rect{r.X + dx, r.Y + dy, r.W - 2*dx, r.H - 2*dy}
}

// Mat2D is a 2D affine matrix in row-major 2x3 form:
//
//	x' = A*x + B*y + C
//	y' = D*x + E*y + F
//
// Same layout as render.Matrix but an independent type: game code composes
// camera/parallax math here, then converts once at the render boundary.
// Perspective stays out (see game/camera/project.go, capability 1.1).
type Mat2D struct {
	A, B, C float64
	D, E, F float64
}

// Identity2D returns the identity matrix.
func Identity2D() Mat2D { return Mat2D{A: 1, E: 1} }

// Translate2D builds a translation matrix.
func Translate2D(x, y float64) Mat2D { return Mat2D{A: 1, C: x, E: 1, F: y} }

// Scale2D builds a scaling matrix.
func Scale2D(x, y float64) Mat2D { return Mat2D{A: x, E: y} }

// Rotate2D builds a rotation matrix (rad radians, counter-clockwise).
func Rotate2D(rad float64) Mat2D {
	c, s := math.Cos(rad), math.Sin(rad)
	return Mat2D{A: c, B: -s, D: s, E: c}
}

// Mul returns m*n (n applies first).
func (m Mat2D) Mul(n Mat2D) Mat2D {
	return Mat2D{
		A: m.A*n.A + m.B*n.D,
		B: m.A*n.B + m.B*n.E,
		C: m.A*n.C + m.B*n.F + m.C,
		D: m.D*n.A + m.E*n.D,
		E: m.D*n.B + m.E*n.E,
		F: m.D*n.C + m.E*n.F + m.F,
	}
}

// TransformPoint applies m to p.
func (m Mat2D) TransformPoint(p Vec2) Vec2 {
	return Vec2{
		X: m.A*p.X + m.B*p.Y + m.C,
		Y: m.D*p.X + m.E*p.Y + m.F,
	}
}

// Invert returns the inverse matrix, or false for a singular matrix.
// Singular stays an error here; render.Matrix.Invert falls back to identity.
// Exact det==0 keeps replay stable across machines.
func (m Mat2D) Invert() (Mat2D, bool) {
	det := m.A*m.E - m.B*m.D
	if det == 0 {
		return Mat2D{}, false
	}
	inv := 1 / det
	return Mat2D{
		A: m.E * inv,
		B: -m.B * inv,
		C: (m.B*m.F - m.C*m.E) * inv,
		D: -m.D * inv,
		E: m.A * inv,
		F: (m.C*m.D - m.A*m.F) * inv,
	}, true
}

// IsIdentity reports whether m is exactly identity.
func (m Mat2D) IsIdentity() bool {
	return m.A == 1 && m.B == 0 && m.C == 0 &&
		m.D == 0 && m.E == 1 && m.F == 0
}

// ApproxEqual reports whether all six components differ by less than eps.
func (m Mat2D) ApproxEqual(n Mat2D, eps float64) bool {
	return math.Abs(m.A-n.A) < eps && math.Abs(m.B-n.B) < eps &&
		math.Abs(m.C-n.C) < eps && math.Abs(m.D-n.D) < eps &&
		math.Abs(m.E-n.E) < eps && math.Abs(m.F-n.F) < eps
}

// ToRenderMatrix converts to render.Matrix at the render boundary.
func (m Mat2D) ToRenderMatrix() render.Matrix {
	return render.Matrix{A: m.A, B: m.B, C: m.C, D: m.D, E: m.E, F: m.F}
}

// Mat2DFromRenderMatrix converts a render.Matrix back to game units.
func Mat2DFromRenderMatrix(m render.Matrix) Mat2D {
	return Mat2D{A: m.A, B: m.B, C: m.C, D: m.D, E: m.E, F: m.F}
}
