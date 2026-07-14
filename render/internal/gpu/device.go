//go:build !nogpu

package gpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

const minRenderStorageBuffersPerShaderStage = 9

func renderDeviceDescriptor(label string) *webgpu.DeviceDescriptor {
	limits := webgpu.DefaultLimits()
	if limits.MaxStorageBuffersPerShaderStage < minRenderStorageBuffersPerShaderStage {
		limits.MaxStorageBuffersPerShaderStage = minRenderStorageBuffersPerShaderStage
	}
	return &webgpu.DeviceDescriptor{
		Label:          label,
		RequiredLimits: limits,
	}
}

// GPUInfo contains information about the selected GPU.
type GPUInfo struct {
	// Name is the GPU name (e.g., "NVIDIA GeForce RTX 3080").
	Name string
	// Vendor is the GPU vendor.
	Vendor string
	// DeviceType is the type of GPU (discrete, integrated, etc.).
	DeviceType types.DeviceType
	// Backend is the graphics API in use (Vulkan, Metal, DX12).
	Backend types.Backend
	// Driver is the driver version string.
	Driver string
}

// String returns a human-readable description of the GPU.
func (g *GPUInfo) String() string {
	return fmt.Sprintf("%s (%s, %s)", g.Name, g.DeviceType, g.Backend)
}

// getGPUInfo retrieves information about the GPU adapter.
func getGPUInfo(adapter *webgpu.Adapter) (*GPUInfo, error) {
	if adapter == nil {
		return nil, fmt.Errorf("adapter is nil")
	}
	info := adapter.Info()

	return &GPUInfo{
		Name:       info.Name,
		Vendor:     info.Vendor,
		DeviceType: info.DeviceType,
		Backend:    info.Backend,
		Driver:     info.Driver,
	}, nil
}

// logGPUInfo logs information about the selected GPU.
func logGPUInfo(adapter *webgpu.Adapter) {
	info, err := getGPUInfo(adapter)
	if err != nil {
		slogger().Warn("failed to get GPU info", "err", err)
		return
	}

	slogger().Info("GPU selected", "gpu", info.String(), "driver", info.Driver)
}

// createDevice creates a logical device from an adapter.
// This is a helper function that encapsulates device creation logic.
func createDevice(adapter *webgpu.Adapter, label string) (*webgpu.Device, error) {
	if adapter == nil {
		return nil, fmt.Errorf("adapter is nil")
	}

	device, err := adapter.RequestDevice(renderDeviceDescriptor(label))
	if err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	return device, nil
}

// getDeviceQueue retrieves the queue associated with a device.
func getDeviceQueue(device *webgpu.Device) (*webgpu.Queue, error) {
	if device == nil {
		return nil, fmt.Errorf("device is nil")
	}
	queue := device.Queue()
	if queue == nil {
		return nil, fmt.Errorf("device returned nil queue")
	}
	return queue, nil
}

// releaseDevice releases a device and its associated resources.
func releaseDevice(device *webgpu.Device) error {
	if device == nil {
		return nil
	}
	device.Release()
	return nil
}

// releaseAdapter releases an adapter.
func releaseAdapter(adapter *webgpu.Adapter) error {
	if adapter == nil {
		return nil
	}
	adapter.Release()
	return nil
}

// CheckDeviceLimits verifies that the device meets minimum requirements.
// This can be used to validate GPU capabilities before rendering.
func CheckDeviceLimits(device *webgpu.Device) error {
	if device == nil {
		return fmt.Errorf("device is nil")
	}
	limits := device.Limits()

	// Log some basic limits for diagnostics.
	slogger().Debug("device limits",
		"maxTexture2D", limits.MaxTextureDimension2D,
		"maxBuffer", limits.MaxBufferSize)

	return nil
}
