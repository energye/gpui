// Package platform is the L0 SPI for the L1 UI engine: native window handles,
// input events, and optional vsync. Platform backends live in *_linux.go etc.
package platform

import "time"

// PlatformKind identifies the window system used for NativeSurface handles.
type PlatformKind int

// PlatformNone is the unset PlatformKind (not a real window system).
const PlatformNone PlatformKind = -1

const (
	// PlatformX11 is Linux Xlib (Display* + Window).
	PlatformX11 PlatformKind = iota
	// PlatformWayland is Linux Wayland (wl_display* + wl_surface*).
	PlatformWayland
	// PlatformWin32 is Windows (HWND in Window; Display may be 0 or HINSTANCE).
	PlatformWin32
	// PlatformAppKit is macOS (CAMetalLayer* / NSView* in Window per gpu binding).
	PlatformAppKit
)

// String implements fmt.Stringer.
func (k PlatformKind) String() string {
	switch k {
	case PlatformNone:
		return "none"
	case PlatformX11:
		return "x11"
	case PlatformWayland:
		return "wayland"
	case PlatformWin32:
		return "win32"
	case PlatformAppKit:
		return "appkit"
	default:
		return "unknown"
	}
}

// NativeSurface holds OS handles required to create a GPU present surface.
// The host creates the window; the engine receives handles only (no window toolkit lock-in).
type NativeSurface struct {
	Kind    PlatformKind
	Display uintptr // X11 Display* / Wayland wl_display*; often 0 on Win/mac
	Window  uintptr // X11 Window / HWND / layer or view pointer
}

// EventType classifies platform events.
type EventType int

const (
	EventNone EventType = iota
	// EventCloseRequested asks the app whether the window may close (title-bar
	// ✕, WM_DELETE_WINDOW, xdg close). The window stays alive unless the app
	// calls Window.Close; only after that (or native destruction) does
	// EventClose fire.
	EventCloseRequested
	// EventClose reports that the window is already destroyed/unmapped.
	// No further events will arrive for it.
	EventClose
	EventResize
	EventExpose
	EventMove     // window moved on screen (X11; Wayland has no event — see X/Y doc)
	EventScale    // device pixel ratio changed (multi-monitor drag / OS scaling)
	EventOccluded // window fully occluded or minimized (see Event.Occluded; stop rendering to save power)
	EventPointer
	EventKey
	EventIME   // input-method session event (compose/commit/caret)
	EventFocus // keyboard focus changed (see Event.Focused)
	EventWake  // WakeUp from another goroutine
)

// PointerKind classifies pointer events.
type PointerKind int

const (
	PointerMove PointerKind = iota
	PointerDown
	PointerUp
	PointerScroll
	PointerEnter // pointer (or window-level grab) entered the window
	PointerLeave // pointer left the window
)

// Event is a platform input or lifecycle event.
// Pointer coordinates are in logical pixels, origin top-left (Y-down).
type Event struct {
	Type EventType

	// Resize / Size (logical pixels).
	Width  int
	Height int
	Scale  float64 // device pixel ratio; 0 means unchanged / unknown

	// EventOccluded: true = fully obscured / minimized (stop rendering),
	// false = visible again. (X11 VisibilityNotify; Wayland suspended state;
	// Win32 WM_SHOWWINDOW; AppKit occlusionState.)
	Occluded bool

	// Pointer (logical).
	Pointer PointerKind
	X, Y    float64
	Button  int
	ScrollX float64
	ScrollY float64

	// EventMove: window top-left screen position in logical pixels (X11
	// reports it; Wayland has no event and silently omits moves).
	MoveX, MoveY int

	// Key
	KeyCode int
	Rune    rune
	Pressed bool

	// Focus / visibility (EventFocus)
	Focused bool

	// IME (EventIME): pre-edit / commit / caret events from the input method.
	IMEKind  int    // 0 = compose (pre-edit), 1 = commit, 2 = caret move
	IMEText  string // compose pre-edit / commit text
	IMEStart int    // affected range start (bytes); -1 = whole buffer
	IMEEnd   int    // affected range end (bytes); -1 = whole buffer
}

// Host is the cross-platform window/input SPI.
// Implementations must not call into ui embedder re-entrantly from WaitEvents
// without care (prefer queue + WakeUp).
type Host interface {
	// NativeSurface returns current GPU surface handles (may be zero before map).
	NativeSurface() NativeSurface
	// Size returns client area in logical pixels.
	Size() (w, h int)
	// ScaleFactor returns device pixel ratio (≥ 1 when known; default 1).
	ScaleFactor() float64
	// WaitEvents blocks until events or timeout. timeout < 0 means infinite
	// (IDLE). timeout == 0 means non-blocking poll.
	WaitEvents(timeout time.Duration) []Event
	// WakeUp unblocks WaitEvents (from any goroutine).
	WakeUp()
}

// VSyncWaiter is optional; when provided by a Host that implements it,
// the scheduler prefers true display timing over 16.67ms fallback.
type VSyncWaiter interface {
	// WaitVSync blocks until the next vertical blank (or returns error).
	WaitVSync() error
}
