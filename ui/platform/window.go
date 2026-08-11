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
	mu      sync.Mutex
	closed  bool
	closeFn func()
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
func newWindow(host Host, kind PlatformKind, ime IME, clip Clipboard, closeFn func()) *Window {
	if closeFn == nil {
		closeFn = func() {}
	}
	return &Window{host: host, kind: kind, ime: ime, clip: clip, closeFn: closeFn}
}

// WrapHost wraps an already-implemented Host into a Window, using the host's
// native surface kind. No capability probing (IME/Clipboard stay nil).
// Useful for tests and embedding hosts that already implement Host.
func WrapHost(host Host) *Window {
	if host == nil {
		return nil
	}
	return newWindow(host, host.NativeSurface().Kind, nil, nil, nil)
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
