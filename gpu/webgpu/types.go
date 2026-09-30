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

import "github.com/energye/gpui/gpu/types"

// MinBindGroups is the minimum guaranteed number of bind groups per WebGPU spec.
// All compliant devices support at least 4 bind groups. Portable code should
// use no more than MinBindGroups.
const MinBindGroups = 4

// MaxBindGroups is the HAL hard cap on bind groups.
// Devices may support up to 8, but only MinBindGroups (4) is guaranteed.
const MaxBindGroups = 8

// Backend types
type Backend = types.Backend
type Backends = types.Backends

// Backend constants
const (
	BackendVulkan = types.BackendVulkan
	BackendMetal  = types.BackendMetal
	BackendDX12   = types.BackendDX12
	BackendGL     = types.BackendGL
)

// Feature and limit types
type Features = types.Features
type Limits = types.Limits

// Buffer usage
type BufferUsage = types.BufferUsage

// Texture types
type TextureUsage = types.TextureUsage

type TextureFormat = types.TextureFormat
type TextureDimension = types.TextureDimension
type TextureViewDimension = types.TextureViewDimension
type TextureAspect = types.TextureAspect

// Shader types
type ShaderStages = types.ShaderStages

const (
	ShaderStageVertex   = types.ShaderStageVertex
	ShaderStageFragment = types.ShaderStageFragment
	ShaderStageCompute  = types.ShaderStageCompute
)

// Primitive types
type PrimitiveTopology = types.PrimitiveTopology
type IndexFormat = types.IndexFormat
type FrontFace = types.FrontFace
type CullMode = types.CullMode

type PrimitiveState = types.PrimitiveState
type MultisampleState = types.MultisampleState

// Render types
type LoadOp = types.LoadOp
type StoreOp = types.StoreOp
type Color = types.Color

// Bind group types
type BindGroupLayoutEntry = types.BindGroupLayoutEntry
type VertexBufferLayout = types.VertexBufferLayout
type ColorTargetState = types.ColorTargetState

// Sampler types
type AddressMode = types.AddressMode
type FilterMode = types.FilterMode
type MipmapFilterMode = types.MipmapFilterMode
type CompareFunction = types.CompareFunction

const (
	MipmapFilterModeNearest = types.MipmapFilterModeNearest
	MipmapFilterModeLinear  = types.MipmapFilterModeLinear
)

// Surface/presentation types
type PresentMode = types.PresentMode
type CompositeAlphaMode = types.CompositeAlphaMode

const (
	PresentModeImmediate   = types.PresentModeImmediate
	PresentModeMailbox     = types.PresentModeMailbox
	PresentModeFifo        = types.PresentModeFifo
	PresentModeFifoRelaxed = types.PresentModeFifoRelaxed
)

// Adapter types
type AdapterInfo = types.AdapterInfo
type DeviceType = types.DeviceType

// Default functions (re-exported for convenience)
var (
	DefaultInstanceDescriptor = types.DefaultInstanceDescriptor
)
