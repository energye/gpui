package gestures

import "github.com/energye/gpui/ui/platform"

// ScrollDelta is a wheel/trackpad scroll sample (logical px convention of platform.Event).
type ScrollDelta struct {
	ScrollX, ScrollY float64
	X, Y             float64 // pointer position if known
}

// ApplyScrollWheel invokes onScroll for PointerScroll events without entering the arena.
// Returns true if the event was a scroll and onScroll was called.
func ApplyScrollWheel(e PointerEvent, onScroll func(ScrollDelta)) bool {
	if e.Kind != platform.PointerScroll || onScroll == nil {
		return false
	}
	onScroll(ScrollDelta{ScrollX: e.ScrollX, ScrollY: e.ScrollY, X: e.X, Y: e.Y})
	return true
}
