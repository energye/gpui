//go:build rust && darwin

package webgpu

import rwgpu "github.com/energye/gpui/gpu/rwgpu"

// createPlatformSurface creates a rendering surface on macOS via CAMetalLayer.
// On macOS, displayHandle is unused (0) and windowHandle is a CAMetalLayer pointer.
func createPlatformSurface(instance *rwgpu.Instance, _, windowHandle uintptr) (*rwgpu.Surface, error) {
	return instance.CreateSurfaceFromMetalLayer(windowHandle)
}
