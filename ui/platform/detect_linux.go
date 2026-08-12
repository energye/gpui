//go:build linux

package platform

import (
	"os"
	"strings"
)

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

// ParseDisplayBackend parses GPUI_DISPLAY / similar values.
// Accepts: auto, x11, xlib, x, wayland, wl, win32, appkit (case-insensitive).
func ParseDisplayBackend(s string) DisplayBackend {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "x11", "xlib", "x":
		return DisplayX11
	case "wayland", "wl":
		return DisplayWayland
	case "win32", "windows":
		return DisplayWin32
	case "appkit", "macos", "cocoa":
		return DisplayAppKit
	default:
		return DisplayAuto
	}
}

// DetectDisplayBackend chooses a Linux window backend.
//
// Order:
//  1. GPUI_DISPLAY env (x11|wayland|auto)
//  2. Auto: prefer **X11** when DISPLAY is set (incl. XWayland) so the
//     compositor/WM draws a normal title bar; else native Wayland
//  3. If neither env is usable, returns DisplayAuto (caller must error)
//
// Why X11 first on dual-stack desktops (GNOME/KDE Wayland + DISPLAY=:0):
// native xdg_toplevel alone has **no** decorations on GNOME (no SSD; apps
// must paint CSD). XWayland windows get the desktop title bar for free.
// Force pure Wayland with GPUI_DISPLAY=wayland (SSD only if compositor
// supports zxdg_decoration_manager_v1).
func DetectDisplayBackend() DisplayBackend {
	if v := os.Getenv("GPUI_DISPLAY"); v != "" {
		b := ParseDisplayBackend(v)
		if b != DisplayAuto {
			return b
		}
	}
	// Also honor GPUI_SURFACE as a weak hint when GPUI_DISPLAY unset.
	if os.Getenv("GPUI_DISPLAY") == "" {
		if v := os.Getenv("GPUI_SURFACE"); v != "" {
			b := ParseDisplayBackend(v)
			if b != DisplayAuto {
				return b
			}
		}
	}
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
