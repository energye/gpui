//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
)

// MarkLost sets sticky device-lost on this facade device (same as native callback).
// Safe on nil. Used by tests and recovery injection.
func (d *Device) MarkLost() {
	if d == nil || d.r == nil {
		return
	}
	d.r.MarkLost()
}

// IsLost reports whether this device handle was marked lost by WGPUDeviceLostCallback.
// Per-device only; recreate via RequestDevice (and swapchain auto-recover) to continue.
// IsLost reports whether this device handle was marked lost by WGPUDeviceLostCallback.
// Per-device only; recreate via RequestDevice (and swapchain auto-recover) to continue.
func (d *Device) IsLost() bool {
	if d == nil || d.r == nil {
		return false
	}
	return d.r.IsLost()
}

// FreeCommandBuffer releases a command buffer after GPU work that used it has
// completed (or after WaitIdle). wgpu-native command buffers hold device
// resources; leaving them unreleased prevents Device.Release from reclaiming
// VRAM and causes subsequent CreateTexture failures under ResetAccelerator.
// FreeCommandBuffer releases a command buffer after GPU work that used it has
// completed (or after WaitIdle). wgpu-native command buffers hold device
// resources; leaving them unreleased prevents Device.Release from reclaiming
// VRAM and causes subsequent CreateTexture failures under ResetAccelerator.
func (d *Device) FreeCommandBuffer(cb hal.CommandBuffer) {
	if cb == nil {
		return
	}
	wcb, ok := cb.(*CommandBuffer)
	if !ok || wcb == nil {
		return
	}
	wcb.Release()
}

// HalDevice returns nil on wgpu-native backend. There is no HAL layer.
// DestroyBuffer implements hal.Device: unwraps hal.Buffer.
func (d *Device) DestroyBuffer(buffer hal.Buffer) {
	if wb, ok := buffer.(*Buffer); ok {
		wb.Release()
	}
}

// DestroyTexture implements hal.Device: unwraps hal.Texture.
// DestroyTexture implements hal.Device: unwraps hal.Texture.
func (d *Device) DestroyTexture(texture hal.Texture) {
	if wt, ok := texture.(*Texture); ok {
		wt.Release()
	}
}

// DestroyTextureView implements hal.Device: unwraps hal.TextureView.
// DestroyTextureView implements hal.Device: unwraps hal.TextureView.
func (d *Device) DestroyTextureView(view hal.TextureView) {
	if wv, ok := view.(*TextureView); ok {
		wv.Release()
	}
}

// DestroySampler implements hal.Device: unwraps hal.Sampler.
// DestroySampler implements hal.Device: unwraps hal.Sampler.
func (d *Device) DestroySampler(sampler hal.Sampler) {
	if ws, ok := sampler.(*Sampler); ok {
		ws.Release()
	}
}

// DestroyBindGroupLayout implements hal.Device: unwraps hal.BindGroupLayout.
// DestroyBindGroupLayout implements hal.Device: unwraps hal.BindGroupLayout.
func (d *Device) DestroyBindGroupLayout(layout hal.BindGroupLayout) {
	if wl, ok := layout.(*BindGroupLayout); ok {
		wl.Release()
	}
}

// DestroyBindGroup implements hal.Device: unwraps hal.BindGroup.
// DestroyBindGroup implements hal.Device: unwraps hal.BindGroup.
func (d *Device) DestroyBindGroup(group hal.BindGroup) {
	if wg, ok := group.(*BindGroup); ok {
		wg.Release()
	}
}

// DestroyPipelineLayout implements hal.Device: unwraps hal.PipelineLayout.
// DestroyPipelineLayout implements hal.Device: unwraps hal.PipelineLayout.
func (d *Device) DestroyPipelineLayout(layout hal.PipelineLayout) {
	if wl, ok := layout.(*PipelineLayout); ok {
		wl.Release()
	}
}

// DestroyShaderModule implements hal.Device: unwraps hal.ShaderModule.
// DestroyShaderModule implements hal.Device: unwraps hal.ShaderModule.
func (d *Device) DestroyShaderModule(module hal.ShaderModule) {
	if wm, ok := module.(*ShaderModule); ok {
		wm.Release()
	}
}

// DestroyRenderPipeline implements hal.Device: unwraps hal.RenderPipeline.
// DestroyRenderPipeline implements hal.Device: unwraps hal.RenderPipeline.
func (d *Device) DestroyRenderPipeline(pipeline hal.RenderPipeline) {
	if wp, ok := pipeline.(*RenderPipeline); ok {
		wp.Release()
	}
}

// DestroyComputePipeline implements hal.Device: unwraps hal.ComputePipeline.
// DestroyComputePipeline implements hal.Device: unwraps hal.ComputePipeline.
func (d *Device) DestroyComputePipeline(pipeline hal.ComputePipeline) {
	if wp, ok := pipeline.(*ComputePipeline); ok {
		wp.Release()
	}
}

// MapBuffer implements hal.Device: blocking map via Buffer.Map.
// UnmapBuffer implements hal.Device: unwraps hal.Buffer.
func (d *Device) UnmapBuffer(buffer hal.Buffer) error {
	wb, ok := buffer.(*Buffer)
	if !ok {
		return hal.ErrInvalidMapRange
	}
	return wb.Unmap()
}

// Release releases the device and all associated resources.
// Release releases the device and all associated resources.
func (d *Device) Release() {
	if d == nil || d.released {
		return
	}
	d.released = true
	// Drop queue ref first (wgpuDeviceGetQueue holds a separate refcount).
	// Without this, DeviceRelease may not free the native device and a later
	// RequestDevice + CreateTexture can OOM ("Not enough memory left").
	if d.queue != nil {
		d.queue.Release()
		d.queue = nil
	}
	if d.r != nil {
		d.r.Release()
		d.r = nil
	}
}

// CreateQuerySet implements hal.Device: timestamp queries are not supported
// on the wgpu-native backend.
// DestroyQuerySet implements hal.Device: no-op (CreateQuerySet never succeeds).
func (d *Device) DestroyQuerySet(_ hal.QuerySet) {}

// CreateRenderBundleEncoder implements hal.Device: render bundles are not
// supported on the wgpu-native backend.
// DestroyRenderBundle implements hal.Device: no-op (CreateRenderBundleEncoder never succeeds).
func (d *Device) DestroyRenderBundle(_ hal.RenderBundle) {}

// CreateAccelerationStructure implements hal.Device: ray tracing is not
// supported on the wgpu-native backend.
// DestroyAccelerationStructure implements hal.Device: no-op (no ray tracing).
func (d *Device) DestroyAccelerationStructure(_ hal.AccelerationStructure) {}

// GetAccelerationStructureBuildSizes implements hal.Device: no ray tracing,
// returns zero sizes.
// GetAccelerationStructureBuildSizes implements hal.Device: no ray tracing,
// returns zero sizes.
func (d *Device) GetAccelerationStructureBuildSizes(_ *hal.GetAccelerationStructureBuildSizesDescriptor) hal.AccelerationStructureBuildSizes {
	return hal.AccelerationStructureBuildSizes{}
}

// GetAccelerationStructureDeviceAddress implements hal.Device: no ray tracing,
// returns 0.
// GetAccelerationStructureDeviceAddress implements hal.Device: no ray tracing,
// returns 0.
func (d *Device) GetAccelerationStructureDeviceAddress(_ hal.AccelerationStructure) uint64 {
	return 0
}

// TlasInstanceToBytes implements hal.Device: no ray tracing, returns nil.
