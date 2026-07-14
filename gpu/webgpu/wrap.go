//go:build !(js && wasm)

package webgpu

import "fmt"

// NewDeviceFromHAL is not supported with the wgpu-native backend.
// Devices are created through wgpu-native, not through Go HAL.
func NewDeviceFromHAL(
	_ any,
	_ any,
	_ Features,
	limits Limits,
	_ string,
) (*Device, error) {
	_ = limits
	return nil, fmt.Errorf("wgpu: NewDeviceFromHAL is not available with wgpu-native; use CreateInstance/RequestAdapter/RequestDevice")
}

// NewSurfaceFromHAL is not supported with the wgpu-native backend.
func NewSurfaceFromHAL(_ any, _ string) *Surface {
	return nil
}

// NewTextureFromHAL is not supported with the wgpu-native backend.
func NewTextureFromHAL(_ any, _ *Device, _ TextureFormat) *Texture {
	return nil
}

// NewTextureViewFromHAL is not supported with the wgpu-native backend.
func NewTextureViewFromHAL(_ any, _ *Device) *TextureView {
	return nil
}

// NewSamplerFromHAL is not supported with the wgpu-native backend.
func NewSamplerFromHAL(_ any, _ *Device) *Sampler {
	return nil
}
