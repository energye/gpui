package exhost

import (
	"fmt"
	"os"

	"github.com/energye/gpui/ui/platform"
)

// Options configures Open.
type Options struct {
	Width, Height int
	Title         string
	// Backend overrides detection when not DisplayAuto.
	Backend platform.DisplayBackend
}

// Window is a platform window + platform.Host for embedder apps.
type Window struct {
	host    platform.Host
	kind    platform.PlatformKind
	close   func()
	backend platform.DisplayBackend
}

// Host returns the embedder host (never nil after successful Open).
func (w *Window) Host() platform.Host {
	if w == nil {
		return nil
	}
	return w.host
}

// SetScale changes the device scale factor on the live host and injects an
// EventResize carrying the new scale, so the embedder reallocates the present
// target at the new physical resolution (real DPR path). 0/negative is ignored.
// Returns true when the scale actually changed.
func (w *Window) SetScale(scale float64) bool {
	if w == nil || w.host == nil || scale <= 0 {
		return false
	}
	switch h := w.host.(type) {
	case *x11Host:
		before := h.ScaleFactor()
		h.SetScale(scale)
		return h.ScaleFactor() != before
	case *wlHost:
		before := h.ScaleFactor()
		h.SetScale(scale)
		return h.ScaleFactor() != before
	}
	return false
}

// Kind is PlatformX11 or PlatformWayland.
func (w *Window) Kind() platform.PlatformKind {
	if w == nil {
		return platform.PlatformX11
	}
	return w.kind
}

// Backend is the selected display backend.
func (w *Window) Backend() platform.DisplayBackend {
	if w == nil {
		return platform.DisplayAuto
	}
	return w.backend
}

// Close destroys the native window. Call only after GPU PresentTarget is closed.
func (w *Window) Close() {
	if w == nil || w.close == nil {
		return
	}
	w.close()
	w.close = nil
	w.host = nil
}

// Open creates a window using auto backend detection (or opts.Backend).
// On Auto: try Wayland first when available, then X11.
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
	if want == platform.DisplayAuto {
		want = platform.DetectDisplayBackend()
	}

	var errs []error

	// DetectDisplayBackend already prefers X11 when DISPLAY is set (title bar).
	// Order of attempts matches that preference; always allow X11 fallback if Wayland fails.
	tryX11 := platform.HasX11Display() && (want == platform.DisplayX11 || want == platform.DisplayAuto || want == platform.DisplayWayland)
	tryWayland := platform.HasWaylandDisplay() && (want == platform.DisplayWayland || want == platform.DisplayAuto)
	if want == platform.DisplayX11 {
		tryWayland = false
	}

	// Prefer X11 first for normal WM decorations when requested/detected as x11.
	if tryX11 && want != platform.DisplayWayland {
		w, err := openX11(opts.Width, opts.Height, opts.Title)
		if err == nil {
			fmt.Fprintf(os.Stderr, "exhost: backend=x11 %dx%d (WM title bar)\n", opts.Width, opts.Height)
			return w, nil
		}
		errs = append(errs, fmt.Errorf("x11: %w", err))
	}
	if tryWayland {
		w, err := openWayland(opts.Width, opts.Height, opts.Title)
		if err == nil {
			fmt.Fprintf(os.Stderr, "exhost: backend=wayland %dx%d (SSD if compositor supports it)\n", opts.Width, opts.Height)
			return w, nil
		}
		errs = append(errs, fmt.Errorf("wayland: %w", err))
	}
	// Wayland-forced but failed: try X11 if not already.
	if tryX11 && want == platform.DisplayWayland {
		w, err := openX11(opts.Width, opts.Height, opts.Title)
		if err == nil {
			fmt.Fprintf(os.Stderr, "exhost: backend=x11 %dx%d (fallback)\n", opts.Width, opts.Height)
			return w, nil
		}
		errs = append(errs, fmt.Errorf("x11: %w", err))
	}
	if len(errs) == 0 {
		return nil, fmt.Errorf("exhost: no display (set WAYLAND_DISPLAY and/or DISPLAY, or GPUI_DISPLAY=x11|wayland)")
	}
	return nil, fmt.Errorf("exhost: open failed: %v", errs)
}
