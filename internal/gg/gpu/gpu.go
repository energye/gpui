//go:build !nogpu

// Package gpu registers the GPU accelerator and coverage filler for
// hardware-accelerated rendering.
//
// Import this package to enable both GPU-based shape rendering (SDF for circles,
// rounded rectangles) and adaptive tile-based rasterization for complex paths.
// The GPU accelerator uses wgpu/hal compute shaders for parallel evaluation.
//
// If GPU initialization fails (no Vulkan/Metal/DX12 available), the
// registration is silently skipped and rendering falls back to CPU.
//
// Usage:
//
//	import _ "github.com/energye/gpui/internal/gg/gpu" // enable GPU acceleration
package gpu

import (
	"github.com/energye/gpui/internal/gg"
	gpuimpl "github.com/energye/gpui/internal/gg/internal/gpu"
	"github.com/gogpu/gpucontext"
)

func init() {
	accel := &gpuimpl.SDFAccelerator{}
	if err := gg.RegisterAccelerator(accel); err != nil {
		gg.Logger().Warn("GPU accelerator not available", "err", err)
	}

	gg.RegisterCoverageFiller(&gpuimpl.AdaptiveFiller{})
}

// SetDeviceProvider configures the GPU accelerator to use a shared GPU device.
func SetDeviceProvider(provider gpucontext.DeviceProvider) error {
	return gg.SetAcceleratorDeviceProvider(provider)
}