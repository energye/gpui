// Package platform is the L0 SPI for the L1 UI engine: native window handles,
// input events, and optional vsync. Platform backends live in *_linux.go etc.
package platform

import "time"

// PlatformKind identifies the window system used for NativeSurface handles.
type PlatformKind int

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
	EventClose
	EventResize
	EventExpose
	EventPointer
	EventKey
	EventWake // WakeUp from another goroutine
)

// PointerKind classifies pointer events.
type PointerKind int

const (
	PointerMove PointerKind = iota
	PointerDown
	PointerUp
	PointerScroll
)

// Event is a platform input or lifecycle event.
// Pointer coordinates are in logical pixels, origin top-left (Y-down).
type Event struct {
	Type EventType

	// Resize / Size (logical pixels).
	Width  int
	Height int
	Scale  float64 // device pixel ratio; 0 means unchanged / unknown

	// Pointer (logical).
	Pointer PointerKind
	X, Y    float64
	Button  int
	ScrollX float64
	ScrollY float64

	// Key
	KeyCode int
	Rune    rune
	Pressed bool
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
