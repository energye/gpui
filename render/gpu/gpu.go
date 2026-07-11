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
//	import _ "github.com/energye/gpui/render/gpu" // enable GPU acceleration
package gpu

import (
	"github.com/energye/gpui/render"
	gpuimpl "github.com/energye/gpui/render/internal/gpu"
	"github.com/gogpu/gpucontext"
)

func init() {
	accel := &gpuimpl.SDFAccelerator{}
	if err := render.RegisterAccelerator(accel); err != nil {
		render.Logger().Warn("GPU accelerator not available", "err", err)
	}

	render.RegisterCoverageFiller(&gpuimpl.AdaptiveFiller{})
}

// SetDeviceProvider configures the GPU accelerator to use a shared GPU device.
func SetDeviceProvider(provider gpucontext.DeviceProvider) error {
	return render.SetAcceleratorDeviceProvider(provider)
}
