package core

import "math"

// Constraints is the layout input: min/max width and height.
// A single layout pass uses tight or loose constraints (Flutter-style).
type Constraints struct {
	MinWidth, MaxWidth   float64
	MinHeight, MaxHeight float64
}

// Unbounded is a sentinel for unconstrained max dimensions.
const Unbounded = math.MaxFloat64

// Loose returns constraints with zero minimum and the given maximums.
func Loose(maxW, maxH float64) Constraints {
	return Constraints{MaxWidth: maxW, MaxHeight: maxH}
}

// Tight returns constraints where min == max for both axes.
func Tight(w, h float64) Constraints {
	return Constraints{MinWidth: w, MaxWidth: w, MinHeight: h, MaxHeight: h}
}

// TightWidth locks width and leaves height loose up to maxH.
func TightWidth(w, maxH float64) Constraints {
	return Constraints{MinWidth: w, MaxWidth: w, MaxHeight: maxH}
}

// TightHeight locks height and leaves width loose up to maxW.
func TightWidthHeight(maxW, h float64) Constraints {
	return Constraints{MaxWidth: maxW, MinHeight: h, MaxHeight: h}
}

// Expand loosens minimums to zero while keeping max.
func (c Constraints) Expand() Constraints {
	return Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
}

// Tighten clamps a preferred size into the constraint range.
func (c Constraints) Tighten(preferred Size) Size {
	w := preferred.Width
	h := preferred.Height
	if w < c.MinWidth {
		w = c.MinWidth
	}
	if w > c.MaxWidth {
		w = c.MaxWidth
	}
	if h < c.MinHeight {
		h = c.MinHeight
	}
	if h > c.MaxHeight {
		h = c.MaxHeight
	}
	return Size{Width: w, Height: h}
}

// ConstrainSize is an alias for Tighten.
func (c Constraints) ConstrainSize(s Size) Size { return c.Tighten(s) }

// HasBoundedWidth reports whether max width is finite.
func (c Constraints) HasBoundedWidth() bool { return c.MaxWidth < Unbounded }

// HasBoundedHeight reports whether max height is finite.
func (c Constraints) HasBoundedHeight() bool { return c.MaxHeight < Unbounded }

// IsTight reports whether both axes are fixed.
func (c Constraints) IsTight() bool {
	return c.MinWidth == c.MaxWidth && c.MinHeight == c.MaxHeight
}

// Deflate subtracts padding from the available space.
func (c Constraints) Deflate(l, t, r, b float64) Constraints {
	maxW := c.MaxWidth - l - r
	maxH := c.MaxHeight - t - b
	minW := c.MinWidth - l - r
	minH := c.MinHeight - t - b
	if minW < 0 {
		minW = 0
	}
	if minH < 0 {
		minH = 0
	}
	if maxW < minW {
		maxW = minW
	}
	if maxH < minH {
		maxH = minH
	}
	return Constraints{MinWidth: minW, MaxWidth: maxW, MinHeight: minH, MaxHeight: maxH}
}

// WithMaxWidth returns a copy with a capped max width.
func (c Constraints) WithMaxWidth(w float64) Constraints {
	if w < c.MaxWidth {
		c.MaxWidth = w
	}
	if c.MinWidth > c.MaxWidth {
		c.MinWidth = c.MaxWidth
	}
	return c
}

// WithMaxHeight returns a copy with a capped max height.
func (c Constraints) WithMaxHeight(h float64) Constraints {
	if h < c.MaxHeight {
		c.MaxHeight = h
	}
	if c.MinHeight > c.MaxHeight {
		c.MinHeight = c.MaxHeight
	}
	return c
}

// EnforceMax clamps max to at least min.
func (c Constraints) EnforceMax() Constraints {
	if c.MaxWidth < c.MinWidth {
		c.MaxWidth = c.MinWidth
	}
	if c.MaxHeight < c.MinHeight {
		c.MaxHeight = c.MinHeight
	}
	return c
}
