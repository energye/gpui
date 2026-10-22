//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

import (
	"math"

	"github.com/energye/gpui/render"
)

// Matrix is render.Matrix, not a parallel type. A Recorder must satisfy
// render.Canvas with the same signatures a live *render.Context has, and a
// playback backend hands a recorded CTM straight to a context; a second
// matrix algebra would force conversions on both sides without adding
// anything. The constructors below are thin forwarders kept so recording-side
// call sites read consistently.
type Matrix = render.Matrix

// Identity returns the identity transformation matrix.
func Identity() Matrix { return render.Identity() }

// Translate creates a translation matrix.
func Translate(x, y float64) Matrix { return render.Translate(x, y) }

// Scale creates a scaling matrix.
func Scale(sx, sy float64) Matrix { return render.Scale(sx, sy) }

// Rotate creates a rotation matrix (angle in radians).
func Rotate(angle float64) Matrix { return render.Rotate(angle) }

// Shear creates a shear matrix.
func Shear(x, y float64) Matrix { return render.Shear(x, y) }

// TransformPoint applies m to (x, y) and returns the transformed coordinates.
// render.Matrix works in render.Point; this keeps the recorder's float-pair
// call sites (and render.Context.TransformPoint's signature) readable.
func TransformPoint(m Matrix, x, y float64) (float64, float64) {
	p := m.TransformPoint(render.Pt(x, y))
	return p.X, p.Y
}

// Rect represents an axis-aligned rectangle.
// Min is the top-left corner (minimum coordinates).
// Max is the bottom-right corner (maximum coordinates).
type Rect struct {
	MinX, MinY float64
	MaxX, MaxY float64
}

// NewRect creates a rectangle from position and size.
func NewRect(x, y, width, height float64) Rect {
	return Rect{
		MinX: x,
		MinY: y,
		MaxX: x + width,
		MaxY: y + height,
	}
}

// NewRectFromPoints creates a rectangle from two corner points.
// The points are normalized so Min <= Max.
func NewRectFromPoints(x1, y1, x2, y2 float64) Rect {
	return Rect{
		MinX: math.Min(x1, x2),
		MinY: math.Min(y1, y2),
		MaxX: math.Max(x1, x2),
		MaxY: math.Max(y1, y2),
	}
}

// X returns the left edge of the rectangle.
func (r Rect) X() float64 {
	return r.MinX
}

// Y returns the top edge of the rectangle.
func (r Rect) Y() float64 {
	return r.MinY
}

// Width returns the width of the rectangle.
func (r Rect) Width() float64 {
	return r.MaxX - r.MinX
}

// Height returns the height of the rectangle.
func (r Rect) Height() float64 {
	return r.MaxY - r.MinY
}

// IsEmpty returns true if the rectangle has zero or negative area.
func (r Rect) IsEmpty() bool {
	return r.MaxX <= r.MinX || r.MaxY <= r.MinY
}

// Contains returns true if the point is inside the rectangle.
func (r Rect) Contains(x, y float64) bool {
	return x >= r.MinX && x <= r.MaxX && y >= r.MinY && y <= r.MaxY
}

// Union returns the smallest rectangle containing both r and other.
func (r Rect) Union(other Rect) Rect {
	return Rect{
		MinX: math.Min(r.MinX, other.MinX),
		MinY: math.Min(r.MinY, other.MinY),
		MaxX: math.Max(r.MaxX, other.MaxX),
		MaxY: math.Max(r.MaxY, other.MaxY),
	}
}

// Intersect returns the intersection of r and other.
// Returns an empty rectangle if they don't intersect.
func (r Rect) Intersect(other Rect) Rect {
	result := Rect{
		MinX: math.Max(r.MinX, other.MinX),
		MinY: math.Max(r.MinY, other.MinY),
		MaxX: math.Min(r.MaxX, other.MaxX),
		MaxY: math.Min(r.MaxY, other.MaxY),
	}
	if result.IsEmpty() {
		return Rect{}
	}
	return result
}

// Inset returns a new rectangle inset by the given amounts.
// Positive values shrink the rectangle, negative values expand it.
func (r Rect) Inset(dx, dy float64) Rect {
	return Rect{
		MinX: r.MinX + dx,
		MinY: r.MinY + dy,
		MaxX: r.MaxX - dx,
		MaxY: r.MaxY - dy,
	}
}

// Offset returns a new rectangle offset by the given amounts.
func (r Rect) Offset(dx, dy float64) Rect {
	return Rect{
		MinX: r.MinX + dx,
		MinY: r.MinY + dy,
		MaxX: r.MaxX + dx,
		MaxY: r.MaxY + dy,
	}
}
