//go:build windows && !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// createPlatformSurface creates a rendering surface on Windows via HWND.
func createPlatformSurface(instance *rwgpu.Instance, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	return createPlatformSurfaceFor(instance, SurfaceBackendWin32, displayHandle, windowHandle)
}

func createPlatformSurfaceFor(instance *rwgpu.Instance, _ SurfaceBackend, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	return instance.CreateSurfaceFromWindowsHWND(displayHandle, windowHandle)
}
