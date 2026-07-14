//go:build linux && !(js && wasm)

package webgpu

import (
	"os"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// createPlatformSurface creates a rendering surface on Linux.
// Detects Wayland vs X11 based on WAYLAND_DISPLAY environment variable,
// matching the platform detection in gogpu's Linux backend.
func createPlatformSurface(instance *rwgpu.Instance, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return instance.CreateSurfaceFromWaylandSurface(displayHandle, windowHandle)
	}
	// X11 fallback: window handle must be uint64 for Xlib Window (XID).
	return instance.CreateSurfaceFromXlibWindow(displayHandle, uint64(windowHandle))
}
