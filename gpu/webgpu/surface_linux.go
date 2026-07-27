//go:build linux && !(js && wasm)

package webgpu

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// linuxSurfaceKind selects the native surface source for CreateSurface on Linux.
// 0 = auto (env), 1 = X11, 2 = Wayland.
var linuxSurfaceKind atomic.Int32

const (
	linuxSurfAuto    int32 = 0
	linuxSurfX11     int32 = 1
	linuxSurfWayland int32 = 2
)

// SetLinuxSurfaceBackend pins the Linux CreateSurface path for subsequent
// CreateSurface / device-lost recreate calls. Pass "wayland", "x11", or "auto".
// Prefer CreateSurfaceX11 / CreateSurfaceWayland for explicit one-shot creation.
func SetLinuxSurfaceBackend(backend string) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "wayland", "wl":
		linuxSurfaceKind.Store(linuxSurfWayland)
	case "x11", "x", "xlib":
		linuxSurfaceKind.Store(linuxSurfX11)
	default:
		linuxSurfaceKind.Store(linuxSurfAuto)
	}
}

// CreateSurfaceX11 creates a surface from an Xlib Display* + Window.
func (i *Instance) CreateSurfaceX11(display, window uintptr) (*Surface, error) {
	return i.createSurfaceLinux(display, window, false)
}

// CreateSurfaceWayland creates a surface from wl_display* + wl_surface*.
func (i *Instance) CreateSurfaceWayland(display, surface uintptr) (*Surface, error) {
	return i.createSurfaceLinux(display, surface, true)
}

func (i *Instance) createSurfaceLinux(displayHandle, windowHandle uintptr, wayland bool) (*Surface, error) {
	if i == nil {
		return nil, fmt.Errorf("wgpu: instance is nil")
	}
	if i.released {
		return nil, ErrReleased
	}
	if displayHandle == 0 || windowHandle == 0 {
		return nil, fmt.Errorf("wgpu: Linux CreateSurface requires non-zero display and window handles")
	}
	var rs *rwgpu.Surface
	var err error
	if wayland {
		rs, err = i.r.CreateSurfaceFromWaylandSurface(displayHandle, windowHandle)
	} else {
		rs, err = i.r.CreateSurfaceFromXlibWindow(displayHandle, uint64(windowHandle))
	}
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create surface: %w", err)
	}
	s := &Surface{
		r:             rs,
		instance:      i,
		displayHandle: displayHandle,
		windowHandle:  windowHandle,
	}
	if wayland {
		s.linuxWayland = true
		s.linuxKindSet = true
	} else {
		s.linuxWayland = false
		s.linuxKindSet = true
	}
	// Keep process default aligned with last explicit create (helps ForceRecover).
	if wayland {
		linuxSurfaceKind.Store(linuxSurfWayland)
	} else {
		linuxSurfaceKind.Store(linuxSurfX11)
	}
	return s, nil
}

// createPlatformSurface creates a rendering surface on Linux.
// Preference: Surface-pinned kind (recreate) → process override → env detection.
func createPlatformSurface(instance *rwgpu.Instance, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	if displayHandle == 0 || windowHandle == 0 {
		return nil, fmt.Errorf("wgpu: Linux CreateSurface requires non-zero display and window handles")
	}
	if resolveLinuxWayland() {
		return instance.CreateSurfaceFromWaylandSurface(displayHandle, windowHandle)
	}
	return instance.CreateSurfaceFromXlibWindow(displayHandle, uint64(windowHandle))
}

func resolveLinuxWayland() bool {
	switch linuxSurfaceKind.Load() {
	case linuxSurfWayland:
		return true
	case linuxSurfX11:
		return false
	}
	// auto: GPUI_DISPLAY first, then WAYLAND_DISPLAY.
	if v := os.Getenv("GPUI_DISPLAY"); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "wayland", "wl":
			return true
		case "x11", "x", "xlib":
			return false
		}
	}
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// pinLinuxKindFromDefault records the resolved backend on the Surface so
// ForceRecover / AutoRecover recreate the same native source type.
func (s *Surface) pinLinuxKindFromDefault() {
	if s == nil {
		return
	}
	s.linuxKindSet = true
	s.linuxWayland = resolveLinuxWayland()
}
