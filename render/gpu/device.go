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
