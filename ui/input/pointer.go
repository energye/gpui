package input

// PointerKind classifies a pointer sample (mouse or touch, unified).
type PointerKind int

const (
	PointerMove PointerKind = iota
	PointerDown
	PointerUp
	PointerCancel
	PointerScroll // wheel / trackpad; deltas in ScrollX/ScrollY
	PointerEnter  // pointer entered the window (position in X/Y when known)
	PointerLeave  // pointer left the window (X/Y may be zero)
)

func (k PointerKind) String() string {
	switch k {
	case PointerMove:
		return "move"
	case PointerDown:
		return "down"
	case PointerUp:
		return "up"
	case PointerCancel:
		return "cancel"
	case PointerScroll:
		return "scroll"
	case PointerEnter:
		return "enter"
	case PointerLeave:
		return "leave"
	default:
		return "unknown"
	}
}

// PrimaryPointerID is the mouse cursor pointer id. Touch slots start above it.
const PrimaryPointerID = 0

// PointerEvent is a single pointer/touch sample in logical coordinates
// (Y-down). Mouse = ID 0; touch slots are platform-assigned ≥ 1.
type PointerEvent struct {
	Kind    PointerKind
	ID      int     // 0 = mouse primary; ≥ 1 = touch slot
	X, Y    float64 // logical px
	Button  int     // 0 = none/move, 1/2/3 = left/middle/right (mouse)
	Buttons int     // held-button mask for Move, W3C order (1=primary, 2=secondary, 4=auxiliary, 8=back, 16=forward); 0 = none/unknown
	ScrollX float64 // Kind=PointerScroll
	ScrollY float64
	Phase   Phase // Kind=PointerScroll gesture phase (Unknown = no info)
}

// TouchEvent is a multi-touch sample (ID ≥ 1). Semantically identical to a
// pointer sample; separated for clarity at the touch layer.
type TouchEvent struct {
	Kind     PointerKind // Move/Down/Up/Cancel (no Scroll)
	ID       int         // touch slot
	X, Y     float64     // logical px
	Pressure float64     // 0–1 (0 = unknown/unsupported)
}
