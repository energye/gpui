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
	"context"
	"fmt"
	"runtime"
	"time"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

// Queue returns the device's command queue (hal.Queue conformance).
func (d *Device) Queue() hal.Queue {
	return d.queue
}

// Features returns the device's enabled features.
// Features returns the device's enabled features.
func (d *Device) Features() Features {
	return d.features
}

// Limits returns the device's resource limits.
// Limits returns the device's resource limits.
func (d *Device) Limits() Limits {
	return d.limits
}

// CreateBuffer creates a GPU buffer.
// CreateBuffer creates a GPU buffer.
func (d *Device) CreateBuffer(desc *hal.BufferDescriptor) (hal.Buffer, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: buffer descriptor is nil")
	}
	rb, err := d.r.CreateBuffer(&rwgpu.BufferDescriptor{
		Label:            desc.Label,
		Size:             desc.Size,
		Usage:            desc.Usage,
		MappedAtCreation: desc.MappedAtCreation,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create buffer: %w", err)
	}

	return &Buffer{r: rb, device: d}, nil
}

// CreateTexture creates a GPU texture.
// CreateTexture creates a GPU texture.
func (d *Device) CreateTexture(desc *hal.TextureDescriptor) (hal.Texture, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: texture descriptor is nil")
	}
	rt, err := d.r.CreateTexture(&rwgpu.TextureDescriptor{
		Label:         desc.Label,
		Usage:         desc.Usage,
		Dimension:     desc.Dimension,
		Size:          rwgpu.Extent3D{Width: desc.Size.Width, Height: desc.Size.Height, DepthOrArrayLayers: desc.Size.DepthOrArrayLayers},
		Format:        desc.Format,
		MipLevelCount: desc.MipLevelCount,
		SampleCount:   desc.SampleCount,
		ViewFormats:   desc.ViewFormats,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create texture: %w", err)
	}

	return &Texture{r: rt, device: d, format: desc.Format}, nil
}

// CreateTextureView creates a view into a texture.
// In rwgpu, CreateView is a method on Texture, not Device.
// Takes hal.Texture (片7d); internal unpack to concrete handle.
// CreateTextureView creates a view into a texture.
// In rwgpu, CreateView is a method on Texture, not Device.
// Takes hal.Texture (片7d); internal unpack to concrete handle.
func (d *Device) CreateTextureView(texture hal.Texture, desc *hal.TextureViewDescriptor) (hal.TextureView, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	wt, ok := texture.(*Texture)
	if !ok || wt == nil || wt.r == nil {
		return nil, fmt.Errorf("wgpu: texture is nil")
	}

	var rDesc *rwgpu.TextureViewDescriptor
	if desc != nil {
		// webgpu.h: omitted counts use UINT32_MAX (WGPU_*_COUNT_UNDEFINED).
		const countUndefined = ^uint32(0)
		mipLevelCount := desc.MipLevelCount
		if mipLevelCount == 0 {
			mipLevelCount = countUndefined
		}
		arrayLayerCount := desc.ArrayLayerCount
		if arrayLayerCount == 0 {
			arrayLayerCount = countUndefined
		}
		rDesc = &rwgpu.TextureViewDescriptor{
			Label:           desc.Label,
			Format:          desc.Format,
			Dimension:       desc.Dimension,
			Aspect:          rwgpu.TextureAspect(desc.Aspect),
			BaseMipLevel:    desc.BaseMipLevel,
			MipLevelCount:   mipLevelCount,
			BaseArrayLayer:  desc.BaseArrayLayer,
			ArrayLayerCount: arrayLayerCount,
		}
	}

	rv, err := wt.r.CreateView(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create texture view: %w", err)
	}

	return &TextureView{r: rv, device: d, texture: wt}, nil
}

// CreateSampler creates a texture sampler.
// CreateSampler creates a texture sampler.
func (d *Device) CreateSampler(desc *hal.SamplerDescriptor) (hal.Sampler, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	var rDesc *rwgpu.SamplerDescriptor
	if desc != nil {
		rDesc = &rwgpu.SamplerDescriptor{
			Label:        desc.Label,
			AddressModeU: desc.AddressModeU,
			AddressModeV: desc.AddressModeV,
			AddressModeW: desc.AddressModeW,
			MagFilter:    desc.MagFilter,
			MinFilter:    desc.MinFilter,
			MipmapFilter: desc.MipmapFilter,
			LodMinClamp:  desc.LodMinClamp,
			LodMaxClamp:  desc.LodMaxClamp,
			Compare:      desc.Compare,
			Anisotropy:   desc.Anisotropy,
		}
	}

	rs, err := d.r.CreateSampler(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create sampler: %w", err)
	}

	return &Sampler{r: rs, device: d}, nil
}

// CreateShaderModule creates a shader module.
// CreateShaderModule creates a shader module.
func (d *Device) CreateShaderModule(desc *hal.ShaderModuleDescriptor) (hal.ShaderModule, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: shader module descriptor is nil")
	}
	var rm *rwgpu.ShaderModule
	var err error

	switch {
	case desc.WGSL != "":
		rm, err = d.r.CreateShaderModuleWGSL(desc.WGSL)
	case len(desc.SPIRV) > 0:
		rm, err = d.r.CreateShaderModuleSPIRV(desc.Label, desc.SPIRV)
	default:
		return nil, fmt.Errorf("wgpu: shader module descriptor has no source")
	}

	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create shader module: %w", err)
	}

	return &ShaderModule{r: rm, device: d}, nil
}

// CreateBindGroupLayout creates a bind group layout.
// CreateBindGroupLayout creates a bind group layout.
func (d *Device) CreateBindGroupLayout(desc *hal.BindGroupLayoutDescriptor) (hal.BindGroupLayout, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: bind group layout descriptor is nil")
	}
	n := len(desc.Entries)
	var stack [8]rwgpu.BindGroupLayoutEntry
	var rEntries []rwgpu.BindGroupLayoutEntry
	if n <= len(stack) {
		rEntries = stack[:n]
	} else {
		rEntries = make([]rwgpu.BindGroupLayoutEntry, n)
	}
	for i, e := range desc.Entries {
		rEntries[i] = convertBindGroupLayoutEntry(e)
	}

	rl, err := d.r.CreateBindGroupLayout(&rwgpu.BindGroupLayoutDescriptor{
		Label:   desc.Label,
		Entries: rEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create bind group layout: %w", err)
	}

	return &BindGroupLayout{r: rl, device: d}, nil
}

// CreatePipelineLayout creates a pipeline layout.
// CreatePipelineLayout creates a pipeline layout.
func (d *Device) CreatePipelineLayout(desc *hal.PipelineLayoutDescriptor) (hal.PipelineLayout, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: pipeline layout descriptor is nil")
	}
	n := len(desc.BindGroupLayouts)
	var stack [8]*rwgpu.BindGroupLayout
	var rLayouts []*rwgpu.BindGroupLayout
	if n <= len(stack) {
		rLayouts = stack[:n]
	} else {
		rLayouts = make([]*rwgpu.BindGroupLayout, n)
	}
	for i, l := range desc.BindGroupLayouts {
		if l == nil {
			continue
		}
		if wl, ok := l.(*BindGroupLayout); ok && wl != nil {
			rLayouts[i] = wl.r
		}
	}

	rl, err := d.r.CreatePipelineLayout(&rwgpu.PipelineLayoutDescriptor{
		Label:            desc.Label,
		BindGroupLayouts: rLayouts,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create pipeline layout: %w", err)
	}

	return &PipelineLayout{r: rl, device: d}, nil
}

// CreateBindGroup creates a bind group.
// Takes hal.BindGroupDescriptor;
// internal unpack of hal interfaces to concrete handles.
// CreateBindGroup creates a bind group.
// Takes hal.BindGroupDescriptor;
// internal unpack of hal interfaces to concrete handles.
func (d *Device) CreateBindGroup(desc *hal.BindGroupDescriptor) (hal.BindGroup, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: bind group descriptor is nil")
	}
	var rLayout *rwgpu.BindGroupLayout
	if desc.Layout != nil {
		if wl, ok := desc.Layout.(*BindGroupLayout); ok && wl != nil {
			rLayout = wl.r
		}
	}

	n := len(desc.Entries)
	var stack [8]rwgpu.BindGroupEntry
	var rEntries []rwgpu.BindGroupEntry
	if n <= len(stack) {
		rEntries = stack[:n]
	} else {
		rEntries = make([]rwgpu.BindGroupEntry, n)
	}
	for i, e := range desc.Entries {
		rEntries[i] = convertBindGroupEntry(e)
	}

	rg, err := d.r.CreateBindGroup(&rwgpu.BindGroupDescriptor{
		Label:   desc.Label,
		Layout:  rLayout,
		Entries: rEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create bind group: %w", err)
	}

	return &BindGroup{r: rg, device: d}, nil
}

// CreateRenderPipeline creates a render pipeline.
// CreateRenderPipeline creates a render pipeline.
func (d *Device) CreateRenderPipeline(desc *hal.RenderPipelineDescriptor) (hal.RenderPipeline, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: render pipeline descriptor is nil")
	}
	// convert via pooled scratch (common ≤4 VB / ≤16 attrs / ≤4 targets).
	sc := acquireRPLConvertScratch()
	rDesc, keepAlive := convertRenderPipelineDescInto(sc, desc)
	rp, err := d.r.CreateRenderPipeline(rDesc)
	runtime.KeepAlive(keepAlive)
	runtime.KeepAlive(sc)
	releaseRPLConvertScratch(sc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create render pipeline: %w", err)
	}

	return &RenderPipeline{r: rp, device: d}, nil
}

// CreateComputePipeline creates a compute pipeline.
// CreateComputePipeline creates a compute pipeline.
func (d *Device) CreateComputePipeline(desc *hal.ComputePipelineDescriptor) (hal.ComputePipeline, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: compute pipeline descriptor is nil")
	}
	var rLayout *rwgpu.PipelineLayout
	if desc.Layout != nil {
		if wl, ok := desc.Layout.(*PipelineLayout); ok && wl != nil {
			rLayout = wl.r
		}
	}
	var rModule *rwgpu.ShaderModule
	if desc.Module != nil {
		if wm, ok := desc.Module.(*ShaderModule); ok && wm != nil {
			rModule = wm.r
		}
	}

	rp, err := d.r.CreateComputePipeline(&rwgpu.ComputePipelineDescriptor{
		Label:      desc.Label,
		Layout:     rLayout,
		Module:     rModule,
		EntryPoint: desc.EntryPoint,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create compute pipeline: %w", err)
	}

	return &ComputePipeline{r: rp, device: d}, nil
}

// CreateCommandEncoder creates a command encoder for recording GPU commands.
// Implements hal.Device (returns hal.CommandEncoder interface).
// CreateCommandEncoder creates a command encoder for recording GPU commands.
// Implements hal.Device (returns hal.CommandEncoder interface).
func (d *Device) CreateCommandEncoder(desc *hal.CommandEncoderDescriptor) (hal.CommandEncoder, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	var rDesc *rwgpu.CommandEncoderDescriptor
	if desc != nil {
		rDesc = &rwgpu.CommandEncoderDescriptor{
			Label: desc.Label,
		}
	}

	re, err := d.r.CreateCommandEncoder(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create command encoder: %w", err)
	}

	return &CommandEncoder{r: re, device: d}, nil
}

// CreateFence creates a GPU synchronization fence.
// On the wgpu-native backend, fences are not exposed by wgpu-native.
// Returns a no-op fence for API compatibility.
// Implements hal.Device (returns hal.Fence interface).
// HalDevice returns nil on wgpu-native backend. There is no HAL layer.
func (d *Device) HalDevice() any { return nil }

// DestroyBuffer implements hal.Device: unwraps hal.Buffer.
// MapBuffer implements hal.Device: blocking map via Buffer.Map.
func (d *Device) MapBuffer(buffer hal.Buffer, offset, size uint64) (hal.BufferMapping, error) {
	wb, ok := buffer.(*Buffer)
	if !ok {
		return hal.BufferMapping{}, hal.ErrInvalidMapRange
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := wb.Map(ctx, types.MapModeRead, offset, size); err != nil {
		return hal.BufferMapping{}, err
	}
	mr, err := wb.MappedRange(offset, size)
	if err != nil {
		return hal.BufferMapping{}, err
	}
	data := mr.BytesMut()
	if len(data) == 0 {
		return hal.BufferMapping{}, hal.ErrInvalidMapRange
	}
	return hal.BufferMapping{Ptr: unsafe.Pointer(&data[0]), IsCoherent: true}, nil //nolint:gosec // ADR-018 opaque handle
}

// UnmapBuffer implements hal.Device: unwraps hal.Buffer.
// CreateQuerySet implements hal.Device: timestamp queries are not supported
// on the wgpu-native backend.
func (d *Device) CreateQuerySet(_ *hal.QuerySetDescriptor) (hal.QuerySet, error) {
	return nil, hal.ErrTimestampsNotSupported
}

// DestroyQuerySet implements hal.Device: no-op (CreateQuerySet never succeeds).
// CreateRenderBundleEncoder implements hal.Device: render bundles are not
// supported on the wgpu-native backend.
func (d *Device) CreateRenderBundleEncoder(_ *hal.RenderBundleEncoderDescriptor) (hal.RenderBundleEncoder, error) {
	return nil, fmt.Errorf("webgpu: render bundles not supported")
}

// DestroyRenderBundle implements hal.Device: no-op (CreateRenderBundleEncoder never succeeds).
// CreateAccelerationStructure implements hal.Device: ray tracing is not
// supported on the wgpu-native backend.
func (d *Device) CreateAccelerationStructure(_ *hal.AccelerationStructureDescriptor) (hal.AccelerationStructure, error) {
	return nil, fmt.Errorf("webgpu: ray tracing not supported")
}

// DestroyAccelerationStructure implements hal.Device: no-op (no ray tracing).
