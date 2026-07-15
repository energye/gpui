package widget

import "image"

// Rect is a logical-pixel axis-aligned box (origin top-left).
type Rect struct {
	X, Y, W, H float64
}

// Contains reports whether (px,py) lies inside the rect (inclusive min, exclusive max on edges).
func (r Rect) Contains(px, py float64) bool {
	return px >= r.X && py >= r.Y && px < r.X+r.W && py < r.Y+r.H
}

// Inset returns a rect shrunk by d on all sides (may become empty).
func (r Rect) Inset(d float64) Rect {
	return Rect{X: r.X + d, Y: r.Y + d, W: r.W - 2*d, H: r.H - 2*d}
}

// ImageRect converts to integer image.Rectangle (ceil-ish expand for damage).
func (r Rect) ImageRect() image.Rectangle {
	x0 := int(r.X)
	y0 := int(r.Y)
	x1 := int(r.X + r.W + 0.999)
	y1 := int(r.Y + r.H + 0.999)
	if x1 < x0 {
		x1 = x0
	}
	if y1 < y0 {
		y1 = y0
	}
	return image.Rect(x0, y0, x1, y1)
}

// Align is horizontal text alignment inside a cell/control.
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)
