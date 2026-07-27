package platform

import (
	"fmt"
	"runtime"
)

// HostOptions is a cross-platform window request.
type HostOptions struct {
	Width, Height int
	Title         string
	Scale         float64
	// PreferHeadless forces Headless even on Linux (CI).
	PreferHeadless bool
	// Backend selects Linux display backend (DisplayAuto honors GPUI_DISPLAY).
	// Ignored on non-Linux.
	Backend DisplayBackend
}

// NewHost picks the best host for GOOS.
// Linux → Wayland (when available) or X11 thin adapter;
// Windows/Darwin → API-complete stubs;
// PreferHeadless or unknown OS → Headless.
func NewHost(opts HostOptions) (Host, error) {
	if opts.PreferHeadless {
		return NewHeadless(opts.Width, opts.Height), nil
	}
	switch runtime.GOOS {
	case "linux":
		// NewLinuxHost auto-detects Wayland vs X11 (GPUI_DISPLAY / opts.Backend).
		return NewLinuxHost(LinuxOptions{
			Width: opts.Width, Height: opts.Height,
			Title: opts.Title, Scale: opts.Scale,
			Backend: opts.Backend,
		})
	case "windows":
		return NewWindowsHost(WindowsOptions{
			Width: opts.Width, Height: opts.Height,
			Title: opts.Title, Scale: opts.Scale,
		})
	case "darwin":
		return NewDarwinHost(DarwinOptions{
			Width: opts.Width, Height: opts.Height,
			Title: opts.Title, Scale: opts.Scale,
		})
	default:
		return nil, fmt.Errorf("platform: unsupported GOOS %q (use Headless)", runtime.GOOS)
	}
}

// GPUPresentReady reports whether this host can drive PresentFrame* today.
// Linux X11 and Wayland hosts are ready; Win/mac stubs are API-shaped only.
func GPUPresentReady(h Host) bool {
	if h == nil {
		return false
	}
	switch h.(type) {
	case *LinuxHost, *WaylandHost:
		return true
	}
	return false
}
