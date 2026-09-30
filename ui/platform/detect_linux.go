//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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
	DisplayWin32
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
// Prefer the session's native backend: a GNOME/KDE Wayland session exports
// WAYLAND_DISPLAY (and DISPLAY too, via Xwayland). Native Wayland is the
// actual compositor there — X11 means Xwayland, which carries interactive
// resize/present limitations (swapchain reconfigure stalls; no sync-request
// resize) — so Wayland wins whenever it is available. X11 is the fallback
// for X11-only sessions.
func DetectDisplayBackend() DisplayBackend {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return DisplayWayland
	}
	if os.Getenv("DISPLAY") != "" {
		return DisplayX11
	}
	return DisplayAuto
}

// HasX11Display reports whether an X11 display string is available.
func HasX11Display() bool { return os.Getenv("DISPLAY") != "" }

// HasWaylandDisplay reports whether a Wayland display name is available.
func HasWaylandDisplay() bool { return os.Getenv("WAYLAND_DISPLAY") != "" }
