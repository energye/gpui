//go:build !(js && wasm) && !linux

package webgpu

// SetLinuxSurfaceBackend is a no-op outside Linux.
func SetLinuxSurfaceBackend(backend string) {}

// CreateSurfaceX11 falls back to CreateSurface on non-Linux.
func (i *Instance) CreateSurfaceX11(display, window uintptr) (*Surface, error) {
	return i.CreateSurface(display, window)
}

// CreateSurfaceWayland falls back to CreateSurface on non-Linux.
func (i *Instance) CreateSurfaceWayland(display, surface uintptr) (*Surface, error) {
	return i.CreateSurface(display, surface)
}

func (s *Surface) pinLinuxKindFromDefault() {}
