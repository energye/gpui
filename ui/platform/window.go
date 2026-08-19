package platform

import (
	"fmt"
	"sync"
)

// Options configures Open (framework-created windows).
type Options struct {
	// Width/Height are the initial client area in logical pixels.
	Width, Height int
	// Title is the window title.
	Title string
	// Backend selects the display backend. DisplayAuto uses detection.
	Backend DisplayBackend
	// Decorations enables the standard window frame (title bar + borders).
	// false (default) is a frameless plain window; pass true for standard
	// decorations (X11 WM frame / Wayland client-side decorations).
	//   - X11: false sends _MOTIF_WM_HINTS to hide the WM frame.
	//   - Wayland: GNOME provides NO server-side decorations; true draws a
	//     client-side title bar + borders (CSD) via wl_subsurface.
	Decorations bool
	// Min/Max size constraints (logical px; 0 = unconstrained).
	MinWidth, MinHeight int
	MaxWidth, MaxHeight int
	// Position is the desired initial window position (X11 only; Wayland
	// protocol forbids client-side positioning). nil = WM decides.
	Position *Point
	// Fullscreen requests the window start fullscreen (X11 EWMH hot zone /
	// Wayland is a request that the compositor may defer until configure).
	Fullscreen bool
	// Cursor is the initial cursor (default CursorDefault).
	Cursor Cursor
	// Resizable controls whether the window can be resized by the user
	// (X11 size hints: 0 = fixed; Wayland xdg_toplevel has no request —
	// enforced via min==max when false).
	Resizable bool
	// IconName is the application icon / app-id name shown by the window
	// manager (taskbar icon). Wayland: xdg_toplevel.set_app_id (the
	// compositor matches it against installed .desktop files to resolve
	// the icon); X11/Win32/AppKit: not wired yet ("" = backend default).
	IconName string
	// Maximized requests the window start maximized (X11 EWMH initial
	// _NET_WM_STATE_MAXIMIZED; Wayland set_maximized before first commit;
	// Win32 SW_MAXIMIZE; AppKit zoom:).
	Maximized bool
	// Visible controls the initial window visibility.
	// nil = true (default, visible on open).
	//   - X11: map control; Wayland: protocol has no control → ignored;
	//   - Win32 SW_SHOW/SW_HIDE; AppKit orderFront:/orderOut:.
	Visible *bool
}

// Point is a window position in logical pixels (X11 screen coords).
type Point struct {
	X, Y int
}

// Window is the unified L0 platform window: native handles, the event pump
// (Host), and optional IME/Clipboard capabilities discovered at bind time.
//
// Create via Open (framework creates the window) or Adopt (bind an existing
// native surface). The returned Window is already wired: Host.WaitEvents
// pumps native events; PipelineApp / application layers consume them without
// any per-platform code.
type Window struct {
	host    Host
	kind    PlatformKind
	ime     IME
	clip    Clipboard
	ctl     WindowController // window controls SPI; nil = not supported
	mu      sync.Mutex
	closed  bool
	closeFn func()
}

// Controls returns the window-control SPI (title/size/state/cursor ops).
// Returns nil when the backend does not support runtime window control;
// callers must nil-check (optional capability, same pattern as IME).
func (w *Window) Controls() WindowController {
	if w == nil {
		return nil
	}
	return w.ctl
}

// Open creates a native window through the registered backend for the
// requested (or detected) display backend. The window is mapped and its Host
// event pump is ready; the caller must Close it.
//
// On Linux the Auto path uses DetectDisplayBackend (X11 first when DISPLAY is
// set, else native Wayland). Non-Linux stubs return an error until the
// backend is implemented.
func Open(opts Options) (*Window, error) {
	if opts.Width < 1 {
		opts.Width = 640
	}
	if opts.Height < 1 {
		opts.Height = 480
	}
	if opts.Title == "" {
		opts.Title = "gpui"
	}

	want := opts.Backend
	if want == DisplayAuto {
		want = DetectDisplayBackend()
	}
	kind := toPlatformKind(want)
	if kind == PlatformNone {
		return nil, fmt.Errorf("platform: no display backend selected")
	}
	b, err := backendFor(kind)
	if err != nil {
		return nil, err
	}
	return b.Create(opts)
}

// Adopt binds an existing native surface (embedding scenario: the host
// created the window and passes handles in). The backend must recognize the
// handles; otherwise it returns an error. Adopted windows are also Close-able
// (the backend decides whether Close destroys the foreign window or just
// detaches).
func Adopt(ns NativeSurface) (*Window, error) {
	if ns.Kind == PlatformNone {
		return nil, fmt.Errorf("platform: adopt requires a real platform kind")
	}
	b, err := backendFor(ns.Kind)
	if err != nil {
		return nil, err
	}
	return b.Adopt(ns)
}

// newWindow wraps a backend-produced window. Backends call this from Create.
func newWindow(host Host, kind PlatformKind, ime IME, clip Clipboard, ctl WindowController, closeFn func()) *Window {
	if closeFn == nil {
		closeFn = func() {}
	}
	return &Window{host: host, kind: kind, ime: ime, clip: clip, ctl: ctl, closeFn: closeFn}
}

// WrapHost wraps an already-implemented Host into a Window, using the host's
// native surface kind. No capability probing (IME/Clipboard/Controls stay
// nil). Useful for tests and embedding hosts that already implement Host.
func WrapHost(host Host) *Window {
	if host == nil {
		return nil
	}
	return newWindow(host, host.NativeSurface().Kind, nil, nil, nil, nil)
}

// Host returns the window's Host (event pump / size / scale). Never nil for a
// successfully opened/adopted window.
func (w *Window) Host() Host {
	if w == nil {
		return nil
	}
	return w.host
}

// Kind returns the platform kind.
func (w *Window) Kind() PlatformKind {
	if w == nil {
		return PlatformNone
	}
	return w.kind
}

// Backend returns the display backend backing this window (DisplayAuto when
// unknown/None). Mirrors exhost.Window.Backend for window-open reporting.
func (w *Window) Backend() DisplayBackend {
	if w == nil {
		return DisplayAuto
	}
	switch w.kind {
	case PlatformX11:
		return DisplayX11
	case PlatformWayland:
		return DisplayWayland
	case PlatformWin32:
		return DisplayWin32
	default:
		return DisplayAuto
	}
}

// IME returns the input-method capability, or nil when the backend does not
// support IME. Callers must nil-check (optional capability, silent degrade).
func (w *Window) IME() IME {
	if w == nil {
		return nil
	}
	return w.ime
}

// Clipboard returns the clipboard capability, or nil when unsupported.
func (w *Window) Clipboard() Clipboard {
	if w == nil {
		return nil
	}
	return w.clip
}

// Close tears the window down. For created windows it destroys the native
// window; for adopted windows it detaches (backend-dependent). Safe to call
// multiple times. Callers must close the GPU PresentTarget first (same order
// as the existing example discipline).
func (w *Window) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	fn := w.closeFn
	w.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Closed reports whether Close has been called.
func (w *Window) Closed() bool {
	if w == nil {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}
