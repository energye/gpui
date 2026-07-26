// Package exhost is a minimal Linux window host for L1 examples.
//
// It auto-selects Wayland or X11 so GPU surfaces match the native handles:
//
//	GPUI_DISPLAY=wayland|x11|auto   (default auto)
//	Auto: prefer Wayland when WAYLAND_DISPLAY is set; fall back to X11 (DISPLAY)
//
// FFI: purego only (libwayland-client / libX11). NO cgo — see docs/ENGINE_CODING_RULES.md.
//
// Close order for callers: stop GPU present (app.Close) before Window.Close.
package exhost
