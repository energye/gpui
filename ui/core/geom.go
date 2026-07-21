package core

import "math"

// Point is a 2D position in logical pixels (DIP).
type Point struct {
	X, Y float64
}

// Add returns p + o.
func (p Point) Add(o Point) Point { return Point{p.X + o.X, p.Y + o.Y} }

// Sub returns p - o.
func (p Point) Sub(o Point) Point { return Point{p.X - o.X, p.Y - o.Y} }

// In reports whether p is inside r (half-open: min inclusive, max exclusive).
func (p Point) In(r Rect) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Size is width/height in logical pixels.
type Size struct {
	Width, Height float64
}

// IsZero reports whether both dimensions are zero.
func (s Size) IsZero() bool { return s.Width == 0 && s.Height == 0 }

// Max returns the component-wise max of a and b.
func MaxSize(a, b Size) Size {
	return Size{math.Max(a.Width, b.Width), math.Max(a.Height, b.Height)}
}

// Rect is an axis-aligned rectangle in logical pixels.
type Rect struct {
	Min, Max Point
}

// NewRect returns a rect from origin and size.
func NewRect(x, y, w, h float64) Rect {
	if w < 0 {
		x, w = x+w, -w
	}
	if h < 0 {
		y, h = y+h, -h
	}
	return Rect{Min: Point{x, y}, Max: Point{x + w, y + h}}
}

// RectFromSize returns a rect at origin with the given size.
func RectFromSize(s Size) Rect { return NewRect(0, 0, s.Width, s.Height) }

// Width returns Max.X - Min.X.
func (r Rect) Width() float64 { return r.Max.X - r.Min.X }

// Height returns Max.Y - Min.Y.
func (r Rect) Height() float64 { return r.Max.Y - r.Min.Y }

// Size returns the dimensions of r.
func (r Rect) Size() Size { return Size{r.Width(), r.Height()} }

// Offset returns a rect shifted by p.
func (r Rect) Offset(p Point) Rect {
	return Rect{Min: r.Min.Add(p), Max: r.Max.Add(p)}
}

// Contains reports whether p is inside r.
func (r Rect) Contains(p Point) bool { return p.In(r) }

// Intersect returns the intersection of r and o (empty if none).
func (r Rect) Intersect(o Rect) Rect {
	minX := math.Max(r.Min.X, o.Min.X)
	minY := math.Max(r.Min.Y, o.Min.Y)
	maxX := math.Min(r.Max.X, o.Max.X)
	maxY := math.Min(r.Max.Y, o.Max.Y)
	if minX >= maxX || minY >= maxY {
		return Rect{}
	}
	return Rect{Min: Point{minX, minY}, Max: Point{maxX, maxY}}
}

// Empty reports whether r has non-positive area.
func (r Rect) Empty() bool { return r.Width() <= 0 || r.Height() <= 0 }

// Inset shrinks r by the given edge insets (clamped to non-negative size).
func (r Rect) Inset(l, t, right, b float64) Rect {
	out := Rect{
		Min: Point{r.Min.X + l, r.Min.Y + t},
		Max: Point{r.Max.X - right, r.Max.Y - b},
	}
	if out.Max.X < out.Min.X {
		out.Max.X = out.Min.X
	}
	if out.Max.Y < out.Min.Y {
		out.Max.Y = out.Min.Y
	}
	return out
}
