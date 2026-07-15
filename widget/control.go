package widget

// Hitter is implemented by paint controls that support pointer hit-testing.
type Hitter interface {
	HitTest(x, y float64) bool
}

// BoundsOf is implemented by controls with a primary axis-aligned box.
type BoundsOf interface {
	PrimaryBounds() Rect
}

// PrimaryBounds helpers for first-batch types.
func (b Button) PrimaryBounds() Rect    { return b.Bounds }
func (r ListRow) PrimaryBounds() Rect   { return r.Bounds }
func (c TableCell) PrimaryBounds() Rect { return c.Bounds }

// InputPrimaryBounds returns the interactive field box (label excluded).
func (i Input) PrimaryBounds(th Theme) Rect { return i.FieldRect(th) }

// HitTest implements Hitter for Input using DefaultTheme field geometry.
// Prefer Input.HitTest(x,y,th) when a custom theme is used.
func (i Input) HitTest(x, y float64) bool {
	return i.HitTestTheme(x, y, DefaultTheme())
}

// HitTestTheme is the theme-aware field hit test.
func (i Input) HitTestTheme(x, y float64, th Theme) bool {
	return i.FieldRect(th).Contains(x, y)
}

// HitFirst returns the first control in order that contains (x,y), or -1.
func HitFirst(x, y float64, items ...Hitter) int {
	for i, h := range items {
		if h != nil && h.HitTest(x, y) {
			return i
		}
	}
	return -1
}
