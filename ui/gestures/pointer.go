package gestures

import "github.com/energye/gpui/ui/input"

// PointerEvent is the unified pointer/touch sample consumed by the gesture
// arena. It is an alias of input.PointerEvent — the cross-platform normalized
// pointer type — so gestures and the rest of the UI speak one vocabulary:
//
//	ID = 0 → mouse primary cursor
//	ID ≥ 1 → touch slot (multi-touch)
//
// (Plan §5: gestures consume input.PointerEvent; the old platform-derived
// FromPlatform/EffectivePointerID helpers are gone.)
type PointerEvent = input.PointerEvent

// FromInput extracts a PointerEvent from a normalized input event.
// Pointer/Scroll map straight to the pointer sample; Touch maps to its
// PointerEvent-equivalent fields (Kind/ID/X/Y). Non-pointer events (key,
// text, ime, lifecycle) yield ok=false.
func FromInput(in input.Event) (PointerEvent, bool) {
	switch in.Kind {
	case input.KindPointer, input.KindScroll:
		return in.Pointer, true
	case input.KindTouch:
		return PointerEvent{
			Kind: in.Touch.Kind,
			ID:   in.Touch.ID,
			X:    in.Touch.X,
			Y:    in.Touch.Y,
		}, true
	default:
		return PointerEvent{}, false
	}
}
