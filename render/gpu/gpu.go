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
// For tile-based rasterization only (no GPU shapes), use:
//
//	import _ "github.com/energye/gpui/render/raster"
//
// Usage:
//
//	import _ "github.com/energye/gpui/render/gpu" // enable GPU acceleration + tile rasterization
package gpu

import (
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	gpuimpl "github.com/energye/gpui/render/internal/gpu"
)

func init() {
	// Adaptive surface lifecycle: escalate to recreate after real OOM (all GPUs).
	gpuimpl.SetTextureOOMHook(NoteTextureOOM)
	// Skia freeGpuResources when platform surface is unconfigured.
	webgpu.AfterSurfaceUnconfigure = PurgeSurfaceResources
	// Ensure purge+abandon before device recreate (even if host forgot OnDeviceAbandon).
	// Purge surface attachments first; host OnDeviceAbandon then DropGPU+AbandonDevice.
	webgpu.BeforeDeviceRecover = PurgeSurfaceResources
	// GPU accelerator (SDF shapes: circles, rounded rects)
	accel := &gpuimpl.SDFAccelerator{}
	if err := render.RegisterAccelerator(accel); err != nil {
		render.Logger().Warn("GPU accelerator not available", "err", err)
	}

	// Coverage filler: AdaptiveFiller auto-selects between SparseStrips (4x4
	// tiles, SIMD-optimized) and TileCompute (16x16 tiles) based on path
	// complexity and canvas size.
	render.RegisterCoverageFiller(&gpuimpl.AdaptiveFiller{})
}

// SetDeviceProvider configures the GPU accelerator to use a shared GPU device
// from an external provider (e.g., gogpu). This avoids creating a separate
// GPU instance and enables efficient device sharing.
//
// The provider should be a gpucontext.DeviceProvider. The accelerator
// type-asserts provider.Device() to *wgpu.Device for HAL access.
//
// Call this before drawing operations, typically from ggcanvas.New() or
// manually after registering the accelerator.
func SetDeviceProvider(provider gpucontext.DeviceProvider) error {
	return render.SetAcceleratorDeviceProvider(provider)
}

// AbandonDevice drops GPU objects on the current shared device without a
// replacement. Wire to webgpu.Swapchain.OnDeviceAbandon before Destroy/Release.
func AbandonDevice() {
	render.AbandonAcceleratorDevice()
}

// PurgeSurfaceResources frees surface-sized GPU attachments process-wide
// without destroying the logical device (Skia freeGpuResources-style).
func PurgeSurfaceResources() {
	render.PurgeAcceleratorSurfaceResources()
}

// ResetAccelerator closes the current accelerator and registers a fresh SDF
// accelerator. Used by tests that inject a temporary window device via
// SetDeviceProvider so later tests are not left holding a released device.
func ResetAccelerator() error {
	render.CloseAccelerator()
	accel := &gpuimpl.SDFAccelerator{}
	return render.RegisterAccelerator(accel)
}
