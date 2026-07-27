package platform

import (
	"os"
	"strings"
)

// PlatformKind identifies the window system backend.
type PlatformKind int

const (
	PlatformX11     PlatformKind = iota // X11 display server
	PlatformWayland                     // Wayland display server
	PlatformDarwin                      // macOS (AppKit)
	PlatformWindows                     // Windows (Win32)
)

func (k PlatformKind) String() string {
	switch k {
	case PlatformX11:
		return "x11"
	case PlatformWayland:
		return "wayland"
	case PlatformDarwin:
		return "darwin"
	case PlatformWindows:
		return "windows"
	default:
		return "unknown"
	}
}

// DisplayBackend selects or detects the window system backend.
//
//	GPUI_DISPLAY=wayland|x11|auto   (default auto)
type DisplayBackend int

const (
	DisplayAuto    DisplayBackend = iota // auto-detect
	DisplayX11                           // force X11
	DisplayWayland                       // force Wayland
)

func (b DisplayBackend) String() string {
	switch b {
	case DisplayX11:
		return "x11"
	case DisplayWayland:
		return "wayland"
	case DisplayAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// NativeSurface carries the OS-level handles needed by wgpu to create a surface.
type NativeSurface struct {
	Kind    PlatformKind
	Display uintptr // X11 Display* / Wayland wl_display* / etc.
	Window  uintptr // X11 Window / Wayland wl_surface* / etc.
}

// SurfaceProvider is optionally implemented by Hosts that own a real OS surface.
type SurfaceProvider interface {
	NativeSurface() NativeSurface
}

// BackendProvider is optionally implemented by Hosts that report their display backend.
type BackendProvider interface {
	Backend() DisplayBackend
}

// SurfaceOf returns the native surface for h when available.
func SurfaceOf(h Host) (NativeSurface, bool) {
	if h == nil {
		return NativeSurface{}, false
	}
	if sp, ok := h.(SurfaceProvider); ok {
		ns := sp.NativeSurface()
		if ns.Display != 0 && ns.Window != 0 {
			return ns, true
		}
	}
	// Fallback: NativeHandles without Kind (assume X11 on Linux).
	if nh, ok := h.(NativeHandles); ok {
		d, w := nh.Display(), nh.Window()
		if d != 0 && w != 0 {
			return NativeSurface{Kind: PlatformX11, Display: d, Window: w}, true
		}
	}
	return NativeSurface{}, false
}

// HasX11Display reports whether DISPLAY is set (X11 connection available).
func HasX11Display() bool {
	return os.Getenv("DISPLAY") != ""
}

// HasWaylandDisplay reports whether a Wayland session is available.
func HasWaylandDisplay() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland"
}

// ParseDisplayBackend parses GPUI_DISPLAY / free-form backend names.
func ParseDisplayBackend(s string) DisplayBackend {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "x11", "x", "xlib":
		return DisplayX11
	case "wayland", "wl":
		return DisplayWayland
	case "auto", "":
		return DisplayAuto
	default:
		return DisplayAuto
	}
}

// DetectDisplayBackend returns the preferred backend.
//
// Order:
//  1. GPUI_DISPLAY=x11|wayland|auto (explicit)
//  2. Auto: prefer Wayland when WAYLAND_DISPLAY / session is Wayland
//  3. Else X11 when DISPLAY is set
func DetectDisplayBackend() DisplayBackend {
	if v := os.Getenv("GPUI_DISPLAY"); v != "" {
		if b := ParseDisplayBackend(v); b != DisplayAuto {
			return b
		}
		// GPUI_DISPLAY=auto → fall through to env detection
	}
	if HasWaylandDisplay() {
		return DisplayWayland
	}
	if HasX11Display() {
		return DisplayX11
	}
	return DisplayAuto
}
