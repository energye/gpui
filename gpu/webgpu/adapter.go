//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// Adapter represents a physical GPU.
// On the wgpu-native backend, this wraps rwgpu Adapter.
type Adapter struct {
	r        *rwgpu.Adapter
	info     AdapterInfo
	features Features
	limits   Limits
	instance *Instance
	released bool
}

// Info returns adapter metadata.
func (a *Adapter) Info() AdapterInfo { return a.info }

// Features returns supported features.
func (a *Adapter) Features() Features { return a.features }

// Limits returns the adapter's resource limits.
func (a *Adapter) Limits() Limits { return a.limits }

// RequestDevice creates a logical device from this adapter.
// If desc is nil, default features and limits are used.
// Takes hal.DeviceDescriptor (canonical, 片7a以 hal 为准；DeviceDescriptor 是 hal 别名，同形).
// Implements hal.Adapter (returns hal.Device interface).
func (a *Adapter) RequestDevice(desc *hal.DeviceDescriptor) (hal.Device, error) {
	if a.released {
		return nil, ErrReleased
	}

	var rDesc *rwgpu.DeviceDescriptor
	if desc != nil {
		rDesc = &rwgpu.DeviceDescriptor{
			Label: desc.Label,
		}
		// Limits conversion: if user specified limits, convert them.
		if desc.RequiredLimits != (types.Limits{}) {
			rl := convertToLimits(desc.RequiredLimits)
			rDesc.RequiredLimits = &rl
		}
	}

	rd, err := a.r.RequestDevice(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to request device: %w", err)
	}

	deviceFeatures := convertFeatures(rd.Features())
	deviceLimits := convertLimits(rd.Limits())

	rq := rd.Queue()

	dev := &Device{
		r:        rd,
		instance: a.instance,
		features: deviceFeatures,
		limits:   deviceLimits,
	}
	dev.queue = &Queue{r: rq, device: dev}
	return dev, nil
}

// GetSurfaceCapabilities returns the capabilities of a surface for this adapter.
// Implements hal.Adapter (takes hal.Surface interface, internal unpack).
func (a *Adapter) GetSurfaceCapabilities(surface hal.Surface) *hal.SurfaceCapabilities {
	ws, ok := surface.(*Surface)
	if !ok || ws == nil {
		return nil
	}
	if a.released || ws.r == nil {
		return nil
	}

	caps, err := ws.r.GetCapabilities(a.r)
	if err != nil {
		return nil
	}

	return &hal.SurfaceCapabilities{
		Formats:      caps.Formats,
		PresentModes: caps.PresentModes,
		AlphaModes:   caps.AlphaModes,
	}
}

// Open implements hal.Adapter: not supported on wgpu-native (devices come from RequestDevice).
func (a *Adapter) Open(_ types.Features, _ types.Limits) (hal.OpenDevice, error) {
	return hal.OpenDevice{}, fmt.Errorf("webgpu: Adapter.Open not supported, use RequestDevice")
}

// TextureFormatCapabilities implements hal.Adapter: returns a conservative default.
func (a *Adapter) TextureFormatCapabilities(_ types.TextureFormat) hal.TextureFormatCapabilities {
	return hal.TextureFormatCapabilities{}
}

// Release releases the adapter.
func (a *Adapter) Release() {
	if a.released {
		return
	}
	a.released = true
	if a.r != nil {
		a.r.Release()
	}
}

// convertToLimits converts gputypes.Limits to rwgpu Limits.
//
//nolint:dupl // Symmetric field-by-field mapping (rwgpu→gputypes vs gputypes→rwgpu), not real duplication.
func convertToLimits(gl types.Limits) rwgpu.Limits {
	return rwgpu.Limits{
		MaxTextureDimension1D:                     gl.MaxTextureDimension1D,
		MaxTextureDimension2D:                     gl.MaxTextureDimension2D,
		MaxTextureDimension3D:                     gl.MaxTextureDimension3D,
		MaxTextureArrayLayers:                     gl.MaxTextureArrayLayers,
		MaxBindGroups:                             gl.MaxBindGroups,
		MaxBindGroupsPlusVertexBuffers:            gl.MaxBindGroupsPlusVertexBuffers,
		MaxBindingsPerBindGroup:                   gl.MaxBindingsPerBindGroup,
		MaxDynamicUniformBuffersPerPipelineLayout: gl.MaxDynamicUniformBuffersPerPipelineLayout,
		MaxDynamicStorageBuffersPerPipelineLayout: gl.MaxDynamicStorageBuffersPerPipelineLayout,
		MaxSampledTexturesPerShaderStage:          gl.MaxSampledTexturesPerShaderStage,
		MaxSamplersPerShaderStage:                 gl.MaxSamplersPerShaderStage,
		MaxStorageBuffersPerShaderStage:           gl.MaxStorageBuffersPerShaderStage,
		MaxStorageTexturesPerShaderStage:          gl.MaxStorageTexturesPerShaderStage,
		MaxUniformBuffersPerShaderStage:           gl.MaxUniformBuffersPerShaderStage,
		MaxUniformBufferBindingSize:               gl.MaxUniformBufferBindingSize,
		MaxStorageBufferBindingSize:               gl.MaxStorageBufferBindingSize,
		MinUniformBufferOffsetAlignment:           gl.MinUniformBufferOffsetAlignment,
		MinStorageBufferOffsetAlignment:           gl.MinStorageBufferOffsetAlignment,
		MaxVertexBuffers:                          gl.MaxVertexBuffers,
		MaxBufferSize:                             gl.MaxBufferSize,
		MaxVertexAttributes:                       gl.MaxVertexAttributes,
		MaxVertexBufferArrayStride:                gl.MaxVertexBufferArrayStride,
		MaxInterStageShaderVariables:              gl.MaxInterStageShaderVariables,
		MaxColorAttachments:                       gl.MaxColorAttachments,
		MaxColorAttachmentBytesPerSample:          gl.MaxColorAttachmentBytesPerSample,
		MaxComputeWorkgroupStorageSize:            gl.MaxComputeWorkgroupStorageSize,
		MaxComputeWorkgroupSizeX:                  gl.MaxComputeWorkgroupSizeX,
		MaxComputeWorkgroupSizeY:                  gl.MaxComputeWorkgroupSizeY,
		MaxComputeWorkgroupSizeZ:                  gl.MaxComputeWorkgroupSizeZ,
		MaxComputeWorkgroupsPerDimension:          gl.MaxComputeWorkgroupsPerDimension,
		MaxComputeInvocationsPerWorkgroup:         gl.MaxComputeInvocationsPerWorkgroup,
	}
}

var _ hal.Adapter = (*Adapter)(nil)
