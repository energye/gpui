package rendering

// Point is a 2D point in logical pixels (Y-down, origin top-left).
type Point struct {
	X, Y float64
}

// Size is width/height in logical pixels.
type Size struct {
	Width, Height float64
}

// Rect is axis-aligned in logical pixels (Y-down).
type Rect struct {
	Min Point
	// Max is exclusive bottom-right in layout space (Min + Size).
	Max Point
}

// Contains reports whether p is inside r (half-open on Max).
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.Y >= r.Min.Y && p.X < r.Max.X && p.Y < r.Max.Y
}

// Size returns width/height of r.
func (r Rect) Size() Size {
	return Size{Width: r.Max.X - r.Min.X, Height: r.Max.Y - r.Min.Y}
}

// NewRect builds a rect from origin and size.
func NewRect(x, y, w, h float64) Rect {
	return Rect{Min: Point{X: x, Y: y}, Max: Point{X: x + w, Y: y + h}}
}
