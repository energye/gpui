//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"github.com/energye/gpui/gpu/hal"
)

// CreateShaderModule creates a shader module from WGSL source.
func CreateShaderModule(device hal.Device, label string, wgslSource string) (hal.ShaderModule, error) {
	return device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: label,
		WGSL:  wgslSource,
	})
}

// GPUResources holds the device, shader module and pipeline layout common to
// GPU rasterizers; released via DestroyGPUResources.
type GPUResources struct {
	Device         hal.Device
	ShaderModule   hal.ShaderModule
	PipelineLayout hal.PipelineLayout
	BindLayouts    []hal.BindGroupLayout
	Pipelines      []hal.ComputePipeline
}

// Destroy cleans up all GPU resources in the correct order.
func (r *GPUResources) Destroy() {
	if r.Device == nil {
		return
	}

	// Destroy pipelines first
	for _, p := range r.Pipelines {
		if p != nil {
			p.Destroy()
		}
	}

	// Destroy pipeline layout
	if r.PipelineLayout != nil {
		r.PipelineLayout.Destroy()
	}

	// Destroy bind group layouts
	for _, l := range r.BindLayouts {
		if l != nil {
			l.Destroy()
		}
	}

	// Destroy shader module
	if r.ShaderModule != nil {
		r.ShaderModule.Destroy()
	}
}
