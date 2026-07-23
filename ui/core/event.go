package core

// PointerType classifies pointer events (W3C-inspired subset).
type PointerType int

const (
	PointerDown PointerType = iota
	PointerUp
	PointerMove
	PointerCancel
)

// PointerButton identifies a mouse/touch button.
type PointerButton int

const (
	ButtonNone PointerButton = iota
	ButtonLeft
	ButtonMiddle
	ButtonRight
)

// PointerEvent is a tree-coordinate pointer event.
// X/Y are absolute in the tree's root space (logical pixels).
type PointerEvent struct {
	Type      PointerType
	X, Y      float64
	Button    PointerButton
	PointerID int
	// Handled stops bubbling when set by a handler.
	Handled bool
	// Target is the hit node at dispatch start (may be nil).
	Target Node
}

// Pos returns the event location as a Point.
func (e *PointerEvent) Pos() Point { return Point{e.X, e.Y} }

// PointerHandler receives pointer events (optional on Node).
type PointerHandler interface {
	HandlePointer(ev *PointerEvent)
}

// ClickHandler is a convenience for Pressable-style click callbacks.
type ClickHandler interface {
	OnClick(ev *PointerEvent)
}

// KeyType classifies key events.
type KeyType int

const (
	KeyDown KeyType = iota
	KeyUp
)

// KeyEvent is a keyboard event delivered to the focused node.
type KeyEvent struct {
	Type KeyType
	Key  string // physical/logical key name, e.g. "Enter", "a", "ArrowLeft"
	Text string // produced text if any
	// Modifier state (host-filled). Also accept composite Key names like "Ctrl+C".
	Shift   bool
	Ctrl    bool
	Alt     bool
	Meta    bool // Cmd on macOS
	Handled bool
}

// KeyHandler receives key events.
type KeyHandler interface {
	HandleKey(ev *KeyEvent)
}

// EventPhase for future capture/bubble introspection.
type EventPhase int

const (
	PhaseTarget EventPhase = iota
	PhaseBubble
)
