package platform

import "errors"

// Cursor identifies a standard cross-platform cursor shape.
type Cursor int

const (
	// CursorDefault is the platform default (arrow).
	CursorDefault Cursor = iota
	// CursorText is the I-beam text cursor.
	CursorText
	// CursorPointer is the hand cursor (links).
	CursorPointer
	// CursorCrosshair is the crosshair cursor.
	CursorCrosshair
	// CursorWait is the busy/wait cursor.
	CursorWait
	// CursorResizeH is the horizontal resize cursor (↔).
	CursorResizeH
	// CursorResizeV is the vertical resize cursor (↕).
	CursorResizeV
	// CursorResizeNE is the top-right / bottom-left diagonal cursor (╱).
	CursorResizeNE
	// CursorResizeNW is the top-left / bottom-right diagonal cursor (╲).
	CursorResizeNW
)

// String implements fmt.Stringer.
func (c Cursor) String() string {
	switch c {
	case CursorText:
		return "text"
	case CursorPointer:
		return "pointer"
	case CursorCrosshair:
		return "crosshair"
	case CursorWait:
		return "wait"
	case CursorResizeH:
		return "resize-h"
	case CursorResizeV:
		return "resize-v"
	case CursorResizeNE:
		return "resize-ne"
	case CursorResizeNW:
		return "resize-nw"
	default:
		return "default"
	}
}

// WindowEdge identifies a window edge/corner for RequestResize.
// Nine values, matching native resize directions on every platform.
type WindowEdge int

const (
	WindowEdgeNone WindowEdge = iota
	WindowEdgeTop
	WindowEdgeBottom
	WindowEdgeLeft
	WindowEdgeRight
	WindowEdgeTopLeft
	WindowEdgeTopRight
	WindowEdgeBottomLeft
	WindowEdgeBottomRight
)

// String implements fmt.Stringer.
func (e WindowEdge) String() string {
	switch e {
	case WindowEdgeTop:
		return "top"
	case WindowEdgeBottom:
		return "bottom"
	case WindowEdgeLeft:
		return "left"
	case WindowEdgeRight:
		return "right"
	case WindowEdgeTopLeft:
		return "top-left"
	case WindowEdgeTopRight:
		return "top-right"
	case WindowEdgeBottomLeft:
		return "bottom-left"
	case WindowEdgeBottomRight:
		return "bottom-right"
	default:
		return "none"
	}
}

// ErrUnsupported is returned by WindowController operations the platform
// cannot perform (e.g. Wayland has no client-side window positioning, no
// programmatic focus). Callers should degrade gracefully.
// All errors are cross-platform tagged; the returned error never carries a
// platform-specific type.
var ErrUnsupported = errors.New("platform: window operation not supported")

// WindowController is the platform window-control SPI. Each backend
// (X11/Wayland/Win32/AppKit) implements it; the upper layer (ui/application
// Window) exposes the same surface uniformly. The interface is deliberately
// platform-neutral (ints/strings/bools only — no HWND/Display/NSView types),
// so the same API drives all four backends.
//
// Not every operation exists on every platform. Operations that are
// fundamentally impossible on a protocol return ErrUnsupported (Wayland:
// SetPosition/Focus/SetAlwaysOnTop/SetDecorations). Queries return
// zero values when the backend cannot answer. See docs/ENGINE_WINDOW_API.md
// (§3 capability matrix) for the per-platform column.
type WindowController interface {
	// Title returns the current window title.
	Title() string
	// SetTitle updates the window title.
	SetTitle(t string)

	// Size returns the client area in logical pixels.
	Size() (w, h int)
	// SetSize requests a new client area (X11: immediate resize; Wayland:
	// a request routed through size hints — the compositor confirms with a
	// configure event, so Size() reflects it after the next EventResize).
	SetSize(w, h int)

	// SetMinSize / SetMaxSize set size constraints (0 = unconstrained).
	SetMinSize(w, h int)
	SetMaxSize(w, h int)

	// Resizable controls whether the user can resize the window.
	// X11: size hints lock (min==max) / unlock; Wayland: same via xdg
	// min/max requests. Query returns the effective state.
	SetResizable(r bool)
	IsResizable() bool

	// SetDecorations toggles system/title-bar chrome at runtime.
	//   - X11: _MOTIF_WM_HINTS decoration switch.
	//   - Wayland: creation-time CSD cannot be rebuilt cheaply → ErrUnsupported.
	// Query returns the effective decoration state (false = frameless).
	SetDecorations(dec bool) error
	IsDecorated() bool

	// SetIgnoreCursorEvents makes the window pass pointer events through
	// (input-transparent; used for overlays / click-through panels).
	//   - Wayland: wl_surface.set_input_region empty region.
	//   - X11: XShape input region empty → ErrUnsupported until landed.
	SetIgnoreCursorEvents(ignore bool) error

	// Position returns the window position in screen coords. ok=false when
	// the backend cannot report it (Wayland).
	Position() (x, y int, ok bool)
	// SetPosition moves the window (X11 only). ErrUnsupported on Wayland.
	SetPosition(x, y int) error

	// Minimize iconifies the window. IsMinimized queries the state
	// (X11: WM state IconicState; Wayland: set_minimized, query via
	// configure; Win32 SW_MINIMIZE/IsIconic; AppKit miniaturize:/
	// isMiniaturized).
	Minimize()
	IsMinimized() bool
	// Maximize / Unmaximize toggle the maximized state.
	Maximize()
	Unmaximize()
	IsMaximized() bool

	// RequestMove starts an interactive window move from the current pointer
	// position (required for frameless windows; X11 _NET_WM_MOVERESIZE,
	// Wayland xdg move).
	RequestMove() error
	// RequestResize starts an interactive resize of the given edge from the
	// current pointer position (frameless windows; X11 _NET_WM_MOVERESIZE,
	// Wayland xdg resize).
	RequestResize(edge WindowEdge) error

	// SetFullscreen enters (true) or exits (false) fullscreen.
	SetFullscreen(fs bool)
	IsFullscreen() bool

	// Show / Hide map / unmap the window (X11; Wayland destroys/recreates the surface stack and emits EventHidden).
	Show() error
	Hide() error
	IsVisible() bool

	// Focus requests keyboard focus (X11). ErrUnsupported on Wayland.
	Focus() error
	IsFocused() bool

	// SetAlwaysOnTop pins the window above others (X11 _NET_WM_STATE_ABOVE).
	// ErrUnsupported on Wayland.
	SetAlwaysOnTop(on bool) error

	// SetCursor changes the pointer cursor for the window.
	SetCursor(c Cursor)
}
