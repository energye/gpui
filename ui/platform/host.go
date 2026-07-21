package platform

import "time"

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
	EventScroll
	EventIME
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

	// Scroll deltas (EventScroll).
	ScrollDX, ScrollDY float64

	// IME composition (EventIME).
	IMEText   string
	IMECursor int
	IMEEnd    bool

	// Window fields.
	Width, Height int
	Focused       bool
}

// Host is the SPI surface apps and core frame loops consume.
// Implementations must not be imported by core/primitive.
//
// Demand-driven frame model (gogpu-aligned):
//   - WaitEvents blocks when idle (timeout < 0) or until timeout (animation tick).
//   - PumpEvents is non-blocking drain (equivalent to WaitEvents(0)).
//   - RequestRedraw / WakeUp unblocks a waiting WaitEvents.
type Host interface {
	// Caps returns available capabilities.
	Caps() Caps
	// Size returns client area in logical pixels.
	Size() (width, height int)
	// ScaleFactor returns DPI scale (1.0 = standard).
	ScaleFactor() float64
	// PumpEvents drains the OS/queue and returns events since last pump (non-blocking).
	PumpEvents() []Event
	// WaitEvents waits for input then drains like PumpEvents.
	// timeout < 0: block until events or WakeUp; 0: non-blocking; >0: max wait.
	WaitEvents(timeout time.Duration) []Event
	// WakeUp unblocks a WaitEvents waiter (any goroutine). No-op if not waiting.
	WakeUp()
	// RequestRedraw asks the host to schedule another frame (enqueue EventRedraw + WakeUp).
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
