//go:build linux

package platform

import "os"

// DisplayBackend is the preferred Linux window system for hosts/examples.
type DisplayBackend int

const (
	// DisplayAuto selects Wayland when available, else X11.
	DisplayAuto DisplayBackend = iota
	// DisplayX11 forces Xlib (including XWayland via DISPLAY).
	DisplayX11
	// DisplayWayland forces wl_display / wl_surface.
	DisplayWayland
	// DisplayWin32 forces the Win32 backend (placeholder until S5; only
	// succeeds on Windows builds).
	DisplayWin32
	// DisplayAppKit forces the macOS AppKit backend (placeholder until S5;
	// only succeeds on darwin builds).
	DisplayAppKit
)

// String implements fmt.Stringer.
func (b DisplayBackend) String() string {
	switch b {
	case DisplayX11:
		return "x11"
	case DisplayWayland:
		return "wayland"
	case DisplayWin32:
		return "win32"
	case DisplayAppKit:
		return "appkit"
	default:
		return "auto"
	}
}

// DetectDisplayBackend chooses a Linux window backend for the Auto path.
// Prefer **X11** when DISPLAY is set (incl. XWayland) so the compositor/WM
// draws a normal title bar; else native Wayland.
//
// Why X11 first on dual-stack desktops (GNOME/KDE Wayland + DISPLAY=:0):
// native xdg_toplevel alone has **no** decorations on GNOME (no SSD; apps
// must paint CSD). XWayland windows get the desktop title bar for free.
// Force a backend in code via platform.Options.Backend (pure Wayland gets
// SSD only if the compositor supports zxdg_decoration_manager_v1).
func DetectDisplayBackend() DisplayBackend {
	// Prefer X11 (title bar via WM / XWayland) when available.
	if os.Getenv("DISPLAY") != "" {
		return DisplayX11
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return DisplayWayland
	}
	return DisplayAuto
}

// HasX11Display reports whether an X11 display string is available.
func HasX11Display() bool { return os.Getenv("DISPLAY") != "" }

// HasWaylandDisplay reports whether a Wayland display name is available.
func HasWaylandDisplay() bool { return os.Getenv("WAYLAND_DISPLAY") != "" }
