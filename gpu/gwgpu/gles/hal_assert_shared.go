//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import "github.com/energye/gpui/gpu/hal"

// Shared-type assertions (platform-independent structs).
// RenderBundle/RenderBundleEncoder have no structs (Create returns error); not asserted.
// AccelerationStructure has no struct in GLES (returns error); not asserted.
var (
	_ hal.Buffer             = (*Buffer)(nil)
	_ hal.Texture            = (*Texture)(nil)
	_ hal.TextureView        = (*TextureView)(nil)
	_ hal.Sampler            = (*Sampler)(nil)
	_ hal.ShaderModule       = (*ShaderModule)(nil)
	_ hal.BindGroupLayout    = (*BindGroupLayout)(nil)
	_ hal.BindGroup          = (*BindGroup)(nil)
	_ hal.PipelineLayout     = (*PipelineLayout)(nil)
	_ hal.RenderPipeline     = (*RenderPipeline)(nil)
	_ hal.ComputePipeline    = (*ComputePipeline)(nil)
	_ hal.Fence              = (*Fence)(nil)
	_ hal.QuerySet           = (*QuerySet)(nil)
	_ hal.CommandBuffer      = (*CommandBuffer)(nil)
	_ hal.CommandEncoder     = (*CommandEncoder)(nil)
	_ hal.RenderPassEncoder  = (*RenderPassEncoder)(nil)
	_ hal.ComputePassEncoder = (*ComputePassEncoder)(nil)
)
