//go:build darwin && !(js && wasm)

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// createPlatformSurface creates a rendering surface on macOS via CAMetalLayer.
// On macOS, displayHandle is unused (0) and windowHandle is a CAMetalLayer pointer.
func createPlatformSurface(instance *rwgpu.Instance, displayHandle, windowHandle uintptr) (*rwgpu.Surface, error) {
	return createPlatformSurfaceFor(instance, SurfaceBackendMetal, displayHandle, windowHandle)
}

func createPlatformSurfaceFor(instance *rwgpu.Instance, _ SurfaceBackend, _, windowHandle uintptr) (*rwgpu.Surface, error) {
	return instance.CreateSurfaceFromMetalLayer(windowHandle)
}
