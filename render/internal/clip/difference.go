package clip

import "github.com/energye/gpui/render/internal/image"

// NewRectDifferenceMask builds a mask covering bounds where pixels inside
// hole are 0 (clipped out) and outside hole (but in bounds) are 255.
// Used for ClipOpDifference of a rectangle.
func NewRectDifferenceMask(bounds, hole Rect) (*MaskClipper, error) {
	if bounds.IsEmpty() {
		return nil, image.ErrInvalidDimensions
	}
	width := int(bounds.W + 0.5)
	height := int(bounds.H + 0.5)
	if width <= 0 || height <= 0 {
		return nil, image.ErrInvalidDimensions
	}
	mask, err := image.NewImageBuf(width, height, image.FormatGray8)
	if err != nil {
		return nil, err
	}
	// Fill entire mask with full coverage.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			_ = mask.SetRGBA(x, y, 255, 255, 255, 255)
		}
	}
	// Punch hole: zero coverage inside hole (in mask coordinates).
	hx0 := int(hole.X - bounds.X + 0.5)
	hy0 := int(hole.Y - bounds.Y + 0.5)
	hx1 := int(hole.X + hole.W - bounds.X + 0.5)
	hy1 := int(hole.Y + hole.H - bounds.Y + 0.5)
	if hx0 < 0 {
		hx0 = 0
	}
	if hy0 < 0 {
		hy0 = 0
	}
	if hx1 > width {
		hx1 = width
	}
	if hy1 > height {
		hy1 = height
	}
	for y := hy0; y < hy1; y++ {
		for x := hx0; x < hx1; x++ {
			_ = mask.SetRGBA(x, y, 0, 0, 0, 255)
		}
	}
	return &MaskClipper{mask: mask, bounds: bounds}, nil
}
