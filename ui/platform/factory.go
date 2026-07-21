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
}

// NewHost picks the best host for GOOS.
// Linux → real X11 thin adapter; Windows/Darwin → API-complete stubs;
// PreferHeadless or unknown OS → Headless.
func NewHost(opts HostOptions) (Host, error) {
	if opts.PreferHeadless {
		return NewHeadless(opts.Width, opts.Height), nil
	}
	switch runtime.GOOS {
	case "linux":
		return NewLinuxHost(LinuxOptions{
			Width: opts.Width, Height: opts.Height,
			Title: opts.Title, Scale: opts.Scale,
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
// Only Linux X11 host is ready; Win/mac stubs are API-shaped only.
func GPUPresentReady(h Host) bool {
	if h == nil {
		return false
	}
	_, ok := h.(*LinuxHost)
	return ok
}
