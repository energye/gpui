package overlay

import "github.com/energye/gpui/ui/rendering"

// Entry is one portal/sheet/tooltip slot above the main band.
//
// Bounds are in logical window coordinates (Y-down). If W/H are 0, Layout
// fills them from Child size after layout.
type Entry struct {
	id uint64

	// Child is the content root (may be nil for pure barrier).
	Child rendering.RenderObject

	// X,Y,W,H logical bounds for hit testing and offset when building layers.
	X, Y, W, H float64

	// Barrier when true: any hit inside bounds is consumed even if Child misses.
	Barrier bool

	// OnRemove is invoked once when the entry is removed from State.
	OnRemove func()

	// Mark when content needs re-raster (tests / callers set after visual change).
	NeedsPaint bool

	removed bool
}

// ID returns the stable entry id (0 if not yet inserted).
func (e *Entry) ID() uint64 {
	if e == nil {
		return 0
	}
	return e.id
}

// Bounds returns the axis-aligned hit rect.
func (e *Entry) Bounds() rendering.Rect {
	if e == nil {
		return rendering.Rect{}
	}
	return rendering.NewRect(e.X, e.Y, e.W, e.H)
}

// Contains reports whether p (window space) is inside bounds.
func (e *Entry) Contains(p rendering.Point) bool {
	if e == nil || e.W <= 0 || e.H <= 0 {
		return false
	}
	return e.Bounds().Contains(p)
}

// NewEntry builds a content entry at (x,y) with optional fixed size.
func NewEntry(child rendering.RenderObject, x, y, w, h float64) *Entry {
	return &Entry{Child: child, X: x, Y: y, W: w, H: h, NeedsPaint: true}
}

// NewBarrierEntry builds a full-area (or rect) pointer barrier, optional child.
func NewBarrierEntry(x, y, w, h float64, child rendering.RenderObject) *Entry {
	return &Entry{
		Child:      child,
		X:          x,
		Y:          y,
		W:          w,
		H:          h,
		Barrier:    true,
		NeedsPaint: true,
	}
}
