// Copyright 2026 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build darwin && !(js && wasm)

package metal

import "github.com/energye/gpui/gpu/hal"

// Shared-type assertions (Buffer/Texture/View/Sampler/Shader/Bind/Resource/Pipe/Fence).
// QuerySet/RenderBundle/RenderBundleEncoder have no structs (Create returns error); not asserted.
var (
	_ hal.Buffer                = (*Buffer)(nil)
	_ hal.Texture               = (*Texture)(nil)
	_ hal.TextureView           = (*TextureView)(nil)
	_ hal.Sampler               = (*Sampler)(nil)
	_ hal.ShaderModule          = (*ShaderModule)(nil)
	_ hal.BindGroupLayout       = (*BindGroupLayout)(nil)
	_ hal.BindGroup             = (*BindGroup)(nil)
	_ hal.PipelineLayout        = (*PipelineLayout)(nil)
	_ hal.RenderPipeline        = (*RenderPipeline)(nil)
	_ hal.ComputePipeline       = (*ComputePipeline)(nil)
	_ hal.Fence                 = (*Fence)(nil)
	_ hal.CommandBuffer         = (*CommandBuffer)(nil)
	_ hal.CommandEncoder        = (*CommandEncoder)(nil)
	_ hal.RenderPassEncoder     = (*RenderPassEncoder)(nil)
	_ hal.ComputePassEncoder    = (*ComputePassEncoder)(nil)
	_ hal.Surface               = (*Surface)(nil)
	_ hal.SurfaceTexture        = (*SurfaceTexture)(nil)
	_ hal.Backend               = Backend{}
	_ hal.Instance              = (*Instance)(nil)
	_ hal.Adapter               = (*Adapter)(nil)
	_ hal.Device                = (*Device)(nil)
	_ hal.Queue                 = (*Queue)(nil)
	_ hal.AccelerationStructure = (*AccelerationStructure)(nil)
)
