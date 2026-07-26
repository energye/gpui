package rendering

// Unbounded is a large constraint max meaning "no limit" for practical UI trees.
const Unbounded = 1e9

// Constraints are Flutter-style box constraints in logical pixels.
type Constraints struct {
	MinWidth, MaxWidth   float64
	MinHeight, MaxHeight float64
}

// Tight returns constraints that force exactly w×h.
func Tight(w, h float64) Constraints {
	return Constraints{MinWidth: w, MaxWidth: w, MinHeight: h, MaxHeight: h}
}

// Loose returns 0..w, 0..h.
func Loose(w, h float64) Constraints {
	return Constraints{MaxWidth: w, MaxHeight: h}
}

// Expand returns min=0 max=Unbounded (or capped by parent if needed later).
func Expand() Constraints {
	return Constraints{MaxWidth: Unbounded, MaxHeight: Unbounded}
}

// Tighten clamps size into the constraint range.
func (c Constraints) Tighten(sz Size) Size {
	w := sz.Width
	h := sz.Height
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

// Equal reports whether constraints are identical.
func (c Constraints) Equal(o Constraints) bool {
	return c.MinWidth == o.MinWidth && c.MaxWidth == o.MaxWidth &&
		c.MinHeight == o.MinHeight && c.MaxHeight == o.MaxHeight
}
