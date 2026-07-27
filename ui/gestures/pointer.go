package gestures

import "github.com/energye/gpui/ui/platform"

// PointerEvent is a pointer sample in logical coordinates (Y-down).
type PointerEvent struct {
	Kind      platform.PointerKind
	X, Y      float64
	ScrollX   float64
	ScrollY   float64
	Button    int
	PointerID int // 0 → PrimaryPointerID
}

// FromPlatform converts a platform event. Non-pointer events yield Kind=-1
// style zero with invalid Kind; callers should check ev.Type first.
func FromPlatform(ev platform.Event) PointerEvent {
	id := PrimaryPointerID
	return PointerEvent{
		Kind:      ev.Pointer,
		X:         ev.X,
		Y:         ev.Y,
		ScrollX:   ev.ScrollX,
		ScrollY:   ev.ScrollY,
		Button:    ev.Button,
		PointerID: id,
	}
}

// EffectivePointerID returns PointerID or PrimaryPointerID.
func (e PointerEvent) EffectivePointerID() int {
	if e.PointerID == 0 {
		return PrimaryPointerID
	}
	return e.PointerID
}
