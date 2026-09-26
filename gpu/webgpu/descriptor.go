//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// Extent3D is a 3D size.
type Extent3D = hal.Extent3D

// Origin3D is a 3D origin point.
type Origin3D = hal.Origin3D

// ImageDataLayout describes the layout of image data in a buffer.
type ImageDataLayout = hal.ImageDataLayout

// BufferDescriptor describes buffer creation parameters.
type BufferDescriptor = hal.BufferDescriptor

// TextureDescriptor describes texture creation parameters.
type TextureDescriptor = hal.TextureDescriptor

// TextureViewDescriptor describes texture view creation parameters.
type TextureViewDescriptor = hal.TextureViewDescriptor

// SamplerDescriptor describes sampler creation parameters.
type SamplerDescriptor = hal.SamplerDescriptor

// ShaderModuleDescriptor describes shader module creation parameters.
type ShaderModuleDescriptor = hal.ShaderModuleDescriptor

// CommandEncoderDescriptor describes command encoder creation.
type CommandEncoderDescriptor = hal.CommandEncoderDescriptor

// ComputePassDescriptor describes compute pass creation.
type ComputePassDescriptor = hal.ComputePassDescriptor

// SurfaceConfiguration configures surface presentation.
type SurfaceConfiguration = hal.SurfaceConfiguration

// StencilOperation describes a stencil operation.
type StencilOperation = types.StencilOperation

// Stencil operation constants. Values match webgpu.h / wgpu-native:
// Undefined=0, Keep=1, Zero=2, ...
const (
	StencilOperationKeep           = types.StencilOperationKeep
	StencilOperationZero           = types.StencilOperationZero
	StencilOperationReplace        = types.StencilOperationReplace
	StencilOperationInvert         = types.StencilOperationInvert
	StencilOperationIncrementClamp = types.StencilOperationIncrementClamp
	StencilOperationDecrementClamp = types.StencilOperationDecrementClamp
	StencilOperationIncrementWrap  = types.StencilOperationIncrementWrap
	StencilOperationDecrementWrap  = types.StencilOperationDecrementWrap
)

// StencilFaceState describes stencil operations for a face.
type StencilFaceState = hal.StencilFaceState

// DepthStencilState describes depth and stencil testing configuration.
type DepthStencilState = hal.DepthStencilState

// RenderPassDescriptor describes a render pass.
// Alias of hal.RenderPassDescriptor (adds TimestampWrites vs old webgpu shape;
// webgpu backend reads Label/ColorAttachments/DepthStencilAttachment only).
type RenderPassDescriptor = hal.RenderPassDescriptor

// RenderPassColorAttachment describes a color attachment for a render pass.
// Alias of hal's (View/ResolveTarget are hal.TextureView interfaces;
// concrete *Texture implements it, keyed literals unchanged).
type RenderPassColorAttachment = hal.RenderPassColorAttachment

// RenderPassDepthStencilAttachment describes a depth/stencil attachment.
// Alias of hal's (View is hal.TextureView interface).
type RenderPassDepthStencilAttachment = hal.RenderPassDepthStencilAttachment

// BindGroupLayoutDescriptor describes a bind group layout.
type BindGroupLayoutDescriptor = hal.BindGroupLayoutDescriptor

// PipelineLayoutDescriptor describes a pipeline layout.
type PipelineLayoutDescriptor struct {
	Label            string
	BindGroupLayouts []*BindGroupLayout
}

// BindGroupDescriptor describes a bind group.
type BindGroupDescriptor struct {
	Label   string
	Layout  *BindGroupLayout
	Entries []BindGroupEntry
}

// BindGroupEntry describes a single resource binding in a bind group.
type BindGroupEntry struct {
	Binding     uint32
	Buffer      *Buffer
	Offset      uint64
	Size        uint64
	Sampler     *Sampler
	TextureView *TextureView
}

// RenderPipelineDescriptor describes a render pipeline.
type RenderPipelineDescriptor struct {
	Label        string
	Layout       *PipelineLayout
	Vertex       VertexState
	Primitive    PrimitiveState
	DepthStencil *DepthStencilState
	Multisample  MultisampleState
	Fragment     *FragmentState
}

// VertexState describes the vertex shader stage.
type VertexState struct {
	Module     *ShaderModule
	EntryPoint string
	Buffers    []VertexBufferLayout
}

// FragmentState describes the fragment shader stage.
type FragmentState struct {
	Module     *ShaderModule
	EntryPoint string
	Targets    []ColorTargetState
}

// ComputePipelineDescriptor describes a compute pipeline.
type ComputePipelineDescriptor struct {
	Label      string
	Layout     *PipelineLayout
	Module     *ShaderModule
	EntryPoint string
}

// ImageCopyTexture specifies a texture subresource for copy operations.
// Texture is hal.Texture interface (concrete *Texture implements it, callers unchanged).
type ImageCopyTexture = hal.ImageCopyTexture

// TextureUsageTransition defines a texture usage state transition.
type TextureUsageTransition = hal.TextureUsageTransition

// TextureRange specifies a range of texture subresources.
type TextureRange = hal.TextureRange

// TextureBarrier defines a texture state transition for synchronization.
type TextureBarrier = hal.TextureBarrier

// TextureCopy describes a texture-to-texture copy region.
type TextureCopy = hal.TextureCopy

// BufferTextureCopy defines a buffer-texture copy region.
type BufferTextureCopy = hal.BufferTextureCopy
