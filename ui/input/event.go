package input

// Kind classifies the cross-platform input event.
type Kind int

const (
	// KindNone is the zero value; not a real event.
	KindNone Kind = iota
	// KindClose is the window close request.
	KindClose
	// KindResize carries new logical size / dpr.
	KindResize
	// KindKey is a key press/release (logical key code).
	KindKey
	// KindPointer is a mouse pointer sample (ID 0 = primary mouse cursor).
	KindPointer
	// KindTouch is a multi-touch sample (ID = touch slot, ≥ 1).
	KindTouch
	// KindScroll is a wheel / trackpad scroll delta.
	KindScroll
	// KindText is committed text (keyboard chars, paste, or IME commit).
	KindText
	// KindIME is an in-progress IME session event (compose/caret).
	KindIME
	// KindWake is a cross-thread wake-up (no input payload).
	KindWake
)

func (k Kind) String() string {
	switch k {
	case KindClose:
		return "close"
	case KindResize:
		return "resize"
	case KindKey:
		return "key"
	case KindPointer:
		return "pointer"
	case KindTouch:
		return "touch"
	case KindScroll:
		return "scroll"
	case KindText:
		return "text"
	case KindIME:
		return "ime"
	case KindWake:
		return "wake"
	default:
		return "none"
	}
}

// Modifiers is the cross-platform modifier state at event time.
// Every backend fills the same four booleans; upper layers never map keysyms.
type Modifiers struct {
	Shift   bool
	Control bool
	Alt     bool
	Meta    bool
}

// IsEmpty reports no modifier is held.
func (m Modifiers) IsEmpty() bool {
	return !m.Shift && !m.Control && !m.Alt && !m.Meta
}

// Event is the normalized cross-platform input event. Exactly one payload
// field is meaningful per Kind; the rest are zero.
//
// All coordinates are logical pixels, Y-down (same convention as the rest of
// the UI engine). WindowID is assigned by the application layer for routing
// in multi-window setups.
type Event struct {
	Kind      Kind
	WindowID  int
	Modifiers Modifiers

	// KindResize: new client-area size in logical pixels (Width/Height ≥ 1).
	Width  int
	Height int
	Scale  float64 // device pixel ratio; 0 = unchanged/unknown

	// KindKey: logical key (Keys) + printable rune when one exists.
	Key KeyEvent

	// KindPointer / KindScroll: mouse sample.
	Pointer PointerEvent

	// KindTouch: multi-touch sample.
	Touch TouchEvent

	// KindText: committed character(s).
	Text TextEvent

	// KindIME: in-progress IME session event.
	IME IMEEvent
}