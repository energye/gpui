package platform

// EventType classifies host events delivered to the UI layer.
type EventType int

const (
	EventPointer EventType = iota
	EventKey
	EventText
	EventResize
	EventFocus
	EventClose
	EventRedraw
)

// PointerKind mirrors core pointer types without importing core (SPI boundary).
type PointerKind int

const (
	PointerDown PointerKind = iota
	PointerUp
	PointerMove
	PointerCancel
)

// PointerBtn identifies a button.
type PointerBtn int

const (
	BtnNone PointerBtn = iota
	BtnLeft
	BtnMiddle
	BtnRight
)

// Event is a platform-neutral input/window event.
type Event struct {
	Type EventType

	// Pointer fields (logical pixels in window client area).
	Pointer   PointerKind
	X, Y      float64
	Button    PointerBtn
	PointerID int

	// Key fields.
	Key  string
	Text string
	Down bool // true=KeyDown, false=KeyUp

	// Window fields.
	Width, Height int
	Focused       bool
}

// Host is the SPI surface apps and core frame loops consume.
// Implementations must not be imported by core/primitive.
type Host interface {
	// Caps returns available capabilities.
	Caps() Caps
	// Size returns client area in logical pixels.
	Size() (width, height int)
	// ScaleFactor returns DPI scale (1.0 = standard).
	ScaleFactor() float64
	// PumpEvents drains the OS/queue and returns events since last pump.
	PumpEvents() []Event
	// RequestRedraw asks the host to schedule another frame.
	RequestRedraw()
	// Close releases host resources.
	Close() error
}

// NativeHandles is optional; Linux X11 adapters may expose display/window.
type NativeHandles interface {
	// Display returns an OS display pointer (e.g. *Display on X11) as uintptr.
	Display() uintptr
	// Window returns an OS window handle as uintptr.
	Window() uintptr
}
