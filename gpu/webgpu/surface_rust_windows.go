//go:build rust && windows

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// createPlatformSurface creates a rendering surface on Windows via HWND.
func createPlatformSurface(instance *rwgpu.Instance, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	return instance.CreateSurfaceFromWindowsHWND(displayHandle, windowHandle)
}
