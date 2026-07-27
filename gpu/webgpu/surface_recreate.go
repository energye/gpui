//go:build !(js && wasm)

package webgpu

// recreateSurfaceForRecover rebuilds a surface after device-lost using the
// same Linux backend (X11 vs Wayland) as the original surface when known.
func recreateSurfaceForRecover(inst *Instance, disp, win uintptr, linuxKindSet, linuxWayland bool) (*Surface, error) {
	if inst == nil {
		return nil, ErrInvalidHandle
	}
	if linuxKindSet {
		if linuxWayland {
			return inst.CreateSurfaceWayland(disp, win)
		}
		return inst.CreateSurfaceX11(disp, win)
	}
	return inst.CreateSurface(disp, win)
}
