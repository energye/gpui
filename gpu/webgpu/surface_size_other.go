//go:build !linux

package webgpu

// x11WindowSize is unavailable off Linux; callers fall back to the applied
// swapchain size (the configured surface stays authoritative on Wayland /
// Windows / macOS, where the app is told the size instead of probing it).
func x11WindowSize(display, window uintptr) (w, h int, ok bool) {
	return 0, 0, false
}