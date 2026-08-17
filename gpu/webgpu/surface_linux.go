//go:build linux && !(js && wasm)

package webgpu

import (
	"fmt"
	"os"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// createPlatformSurface is the Auto path (legacy).
func createPlatformSurface(instance *rwgpu.Instance, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	return createPlatformSurfaceFor(instance, SurfaceBackendAuto, displayHandle, windowHandle)
}

func createPlatformSurfaceFor(instance *rwgpu.Instance, backend SurfaceBackend, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	return createLinuxSurface(instance, resolveLinuxBackend(backend), displayHandle, windowHandle)
}

func createLinuxSurface(instance *rwgpu.Instance, backend SurfaceBackend, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	if displayHandle == 0 || windowHandle == 0 {
		return nil, fmt.Errorf("wgpu: Linux CreateSurface requires non-zero display and window handles")
	}
	switch backend {
	case SurfaceBackendWayland:
		return instance.CreateSurfaceFromWaylandSurface(displayHandle, windowHandle)
	case SurfaceBackendXlib, SurfaceBackendAuto:
		// Auto should already be resolved; treat remaining Auto as Xlib.
		return instance.CreateSurfaceFromXlibWindow(displayHandle, uint64(windowHandle))
	default:
		return nil, fmt.Errorf("wgpu: unsupported Linux surface backend %v", backend)
	}
}

// resolveLinuxBackend maps Auto to a concrete backend.
// Explicit Xlib/Wayland are returned unchanged.
//
// Auto: Wayland only if WAYLAND_DISPLAY set and DISPLAY empty (pure Wayland
// client). If both are set (common XWayland), default Xlib — callers with
// real wl_* handles must pass SurfaceBackendWayland explicitly.
func resolveLinuxBackend(backend SurfaceBackend) SurfaceBackend {
	if backend == SurfaceBackendXlib || backend == SurfaceBackendWayland {
		return backend
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") == "" {
		return SurfaceBackendWayland
	}
	return SurfaceBackendXlib
}
