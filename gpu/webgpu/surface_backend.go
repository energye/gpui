package webgpu

// SurfaceBackend selects which native window-system path CreateSurface uses on Linux.
// Windows/macOS ignore this and always use their platform path.
type SurfaceBackend int

const (
	// SurfaceBackendAuto picks from environment when no explicit platform is known.
	// Prefer CreateSurfaceFor / PresentNativeSurface.Platform instead.
	SurfaceBackendAuto SurfaceBackend = iota
	// SurfaceBackendXlib is X11 Display* + Window (XID), including XWayland.
	SurfaceBackendXlib
	// SurfaceBackendWayland is wl_display* + wl_surface*.
	SurfaceBackendWayland
	// SurfaceBackendWin32 is HWND (Windows).
	SurfaceBackendWin32
	// SurfaceBackendMetal is CAMetalLayer* (macOS).
	SurfaceBackendMetal
)

// String returns a short backend name for logs.
func (b SurfaceBackend) String() string {
	switch b {
	case SurfaceBackendXlib:
		return "xlib"
	case SurfaceBackendWayland:
		return "wayland"
	case SurfaceBackendWin32:
		return "win32"
	case SurfaceBackendMetal:
		return "metal"
	default:
		return "auto"
	}
}
