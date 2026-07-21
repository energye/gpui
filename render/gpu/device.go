//go:build !nogpu

package gpu

import "github.com/energye/gpui/gpu/webgpu"

// minStorageBuffers matches render/internal/gpu (Vello coarse pass needs 9).
const minStorageBuffersPerShaderStage = 9

// DeviceDescriptor returns a DeviceDescriptor with limits suitable for GPUI
// render (including Vello compute). Use this when RequestDevice for a
// window/swapchain device that will be injected via SetDeviceProvider.
//
// Important: do NOT pass adapter.Limits() wholesale as RequiredLimits — some
// backends advertise enormous MaxStorageBuffersPerShaderStage (e.g. 524288)
// and requesting those as required can OOM device creation or tiny allocations.
// This helper starts from WebGPU DefaultLimits and only raises storage buffers.
func DeviceDescriptor(label string) *webgpu.DeviceDescriptor {
	limits := webgpu.DefaultLimits()
	if limits.MaxStorageBuffersPerShaderStage < minStorageBuffersPerShaderStage {
		limits.MaxStorageBuffersPerShaderStage = minStorageBuffersPerShaderStage
	}
	return &webgpu.DeviceDescriptor{
		Label:          label,
		RequiredLimits: limits,
	}
}

// DeviceDescriptorLowVRAM returns limits aimed at 1–2GB GPUs (e.g. 940MX).
// RequiredLimits are still *minimums the device must support* (not pre-alloc),
// but we avoid advertising 256MiB buffer / 128MiB storage binding floors that
// encourage large Vulkan heap reservations on some wgpu-native paths.
//
// Suitable for solid/UI present + modest meshes. Heavy compute/Vello paths
// may need the default DeviceDescriptor.
func DeviceDescriptorLowVRAM(label string) *webgpu.DeviceDescriptor {
	limits := webgpu.DefaultLimits()
	if limits.MaxStorageBuffersPerShaderStage < minStorageBuffersPerShaderStage {
		limits.MaxStorageBuffersPerShaderStage = minStorageBuffersPerShaderStage
	}
	// Tighten large "capability floors" that UI compositing never needs.
	limits.MaxBufferSize = 64 * 1024 * 1024               // 64 MiB (was 256)
	limits.MaxStorageBufferBindingSize = 32 * 1024 * 1024 // 32 MiB (was 128)
	limits.MaxTextureDimension2D = 4096                   // 4k enough for UI windows
	limits.MaxTextureDimension1D = 4096
	limits.MaxTextureArrayLayers = 64
	limits.MaxBindingsPerBindGroup = 128
	limits.MaxNonSamplerBindings = 10000
	return &webgpu.DeviceDescriptor{
		Label:          label,
		RequiredLimits: limits,
	}
}
