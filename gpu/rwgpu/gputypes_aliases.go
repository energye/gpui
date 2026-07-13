package wgpu

import "github.com/energye/gpui/gpu/types"

// Type aliases from gputypes for single-import ergonomics.
// Importing "github.com/energye/gpui/gpu/rwgpu" is sufficient — no separate
// gputypes import required when using these aliases.

// Extent3D is a 3D extent (width/height/depth or array layers).
type Extent3D = types.Extent3D

// Origin3D is a 3D origin (x/y/z or array layer offset).
type Origin3D = types.Origin3D

// Color is an RGBA color with double precision.
// Note: wgpu package also defines Color struct for render pass clear values.
// This alias shadows it — use wgpu.Color directly for the render pass type.

// MapMode specifies buffer mapping mode.
// Note: MapMode is already defined as a native type in buffer.go (uint64).

// Texture types.
// TextureAspect is defined as a native enum in enums.go.
type TextureFormat = types.TextureFormat
type TextureDimension = types.TextureDimension
type TextureViewDimension = types.TextureViewDimension

// Buffer types.
type BufferUsage = types.BufferUsage

// Texture usage type.
type TextureUsage = types.TextureUsage

// Shader stage type.
// ShaderStage is the uint32 bitflag for individual stages (vertex/fragment/compute).
// ShaderStages is an alias for the same type for use in pipeline descriptors.
type ShaderStage = types.ShaderStage
type ShaderStages = types.ShaderStages

// Primitive assembly types.
type PrimitiveTopology = types.PrimitiveTopology
type FrontFace = types.FrontFace
type CullMode = types.CullMode
type IndexFormat = types.IndexFormat

// Blend types.
type BlendFactor = types.BlendFactor
type BlendOperation = types.BlendOperation
type ColorWriteMask = types.ColorWriteMask

// Depth/stencil types.
type CompareFunction = types.CompareFunction
type StencilOperation = types.StencilOperation

// Vertex types.
type VertexFormat = types.VertexFormat
type VertexStepMode = types.VertexStepMode

// Sampler types.
type FilterMode = types.FilterMode
type MipmapFilterMode = types.MipmapFilterMode
type AddressMode = types.AddressMode

// Surface/presentation types.
type PresentMode = types.PresentMode
type CompositeAlphaMode = types.CompositeAlphaMode

// Adapter types.
type PowerPreference = types.PowerPreference

// Render pass types.
type LoadOp = types.LoadOp
type StoreOp = types.StoreOp

// Features is a bitmask of enabled GPU features.
// Note: Limits is defined as a native FFI struct in adapter.go (matches wgpu-native ABI).
type Features = types.Features

// --- BufferUsage constants ---

const (
	BufferUsageNone         = types.BufferUsageNone
	BufferUsageMapRead      = types.BufferUsageMapRead
	BufferUsageMapWrite     = types.BufferUsageMapWrite
	BufferUsageCopySrc      = types.BufferUsageCopySrc
	BufferUsageCopyDst      = types.BufferUsageCopyDst
	BufferUsageIndex        = types.BufferUsageIndex
	BufferUsageVertex       = types.BufferUsageVertex
	BufferUsageUniform      = types.BufferUsageUniform
	BufferUsageStorage      = types.BufferUsageStorage
	BufferUsageIndirect     = types.BufferUsageIndirect
	BufferUsageQueryResolve = types.BufferUsageQueryResolve
)

// --- TextureUsage constants ---

const (
	TextureUsageNone             = types.TextureUsageNone
	TextureUsageCopySrc          = types.TextureUsageCopySrc
	TextureUsageCopyDst          = types.TextureUsageCopyDst
	TextureUsageTextureBinding   = types.TextureUsageTextureBinding
	TextureUsageStorageBinding   = types.TextureUsageStorageBinding
	TextureUsageRenderAttachment = types.TextureUsageRenderAttachment
)

// --- TextureFormat constants ---

const (
	TextureFormatUndefined           = types.TextureFormatUndefined
	TextureFormatR8Unorm             = types.TextureFormatR8Unorm
	TextureFormatR8Snorm             = types.TextureFormatR8Snorm
	TextureFormatR8Uint              = types.TextureFormatR8Uint
	TextureFormatR8Sint              = types.TextureFormatR8Sint
	TextureFormatR16Uint             = types.TextureFormatR16Uint
	TextureFormatR16Sint             = types.TextureFormatR16Sint
	TextureFormatR16Float            = types.TextureFormatR16Float
	TextureFormatRG8Unorm            = types.TextureFormatRG8Unorm
	TextureFormatRG8Snorm            = types.TextureFormatRG8Snorm
	TextureFormatRG8Uint             = types.TextureFormatRG8Uint
	TextureFormatRG8Sint             = types.TextureFormatRG8Sint
	TextureFormatR32Float            = types.TextureFormatR32Float
	TextureFormatR32Uint             = types.TextureFormatR32Uint
	TextureFormatR32Sint             = types.TextureFormatR32Sint
	TextureFormatRG16Uint            = types.TextureFormatRG16Uint
	TextureFormatRG16Sint            = types.TextureFormatRG16Sint
	TextureFormatRG16Float           = types.TextureFormatRG16Float
	TextureFormatRGBA8Unorm          = types.TextureFormatRGBA8Unorm
	TextureFormatRGBA8UnormSrgb      = types.TextureFormatRGBA8UnormSrgb
	TextureFormatRGBA8Snorm          = types.TextureFormatRGBA8Snorm
	TextureFormatRGBA8Uint           = types.TextureFormatRGBA8Uint
	TextureFormatRGBA8Sint           = types.TextureFormatRGBA8Sint
	TextureFormatBGRA8Unorm          = types.TextureFormatBGRA8Unorm
	TextureFormatBGRA8UnormSrgb      = types.TextureFormatBGRA8UnormSrgb
	TextureFormatRGB10A2Uint         = types.TextureFormatRGB10A2Uint
	TextureFormatRGB10A2Unorm        = types.TextureFormatRGB10A2Unorm
	TextureFormatRG11B10Ufloat       = types.TextureFormatRG11B10Ufloat
	TextureFormatRG32Float           = types.TextureFormatRG32Float
	TextureFormatRG32Uint            = types.TextureFormatRG32Uint
	TextureFormatRG32Sint            = types.TextureFormatRG32Sint
	TextureFormatRGBA16Uint          = types.TextureFormatRGBA16Uint
	TextureFormatRGBA16Sint          = types.TextureFormatRGBA16Sint
	TextureFormatRGBA16Float         = types.TextureFormatRGBA16Float
	TextureFormatRGBA32Float         = types.TextureFormatRGBA32Float
	TextureFormatRGBA32Uint          = types.TextureFormatRGBA32Uint
	TextureFormatRGBA32Sint          = types.TextureFormatRGBA32Sint
	TextureFormatDepth32Float        = types.TextureFormatDepth32Float
	TextureFormatDepth24Plus         = types.TextureFormatDepth24Plus
	TextureFormatDepth24PlusStencil8 = types.TextureFormatDepth24PlusStencil8
	TextureFormatDepth16Unorm        = types.TextureFormatDepth16Unorm
)

// --- TextureDimension constants ---

const (
	TextureDimension1D = types.TextureDimension1D
	TextureDimension2D = types.TextureDimension2D
	TextureDimension3D = types.TextureDimension3D
)

// --- ShaderStage constants ---

const (
	ShaderStageNone     = types.ShaderStageNone
	ShaderStageVertex   = types.ShaderStageVertex
	ShaderStageFragment = types.ShaderStageFragment
	ShaderStageCompute  = types.ShaderStageCompute
)

// --- PrimitiveTopology constants ---

const (
	PrimitiveTopologyPointList     = types.PrimitiveTopologyPointList
	PrimitiveTopologyLineList      = types.PrimitiveTopologyLineList
	PrimitiveTopologyLineStrip     = types.PrimitiveTopologyLineStrip
	PrimitiveTopologyTriangleList  = types.PrimitiveTopologyTriangleList
	PrimitiveTopologyTriangleStrip = types.PrimitiveTopologyTriangleStrip
)

// --- FrontFace constants ---

const (
	FrontFaceCCW = types.FrontFaceCCW
	FrontFaceCW  = types.FrontFaceCW
)

// --- CullMode constants ---

const (
	CullModeNone  = types.CullModeNone
	CullModeFront = types.CullModeFront
	CullModeBack  = types.CullModeBack
)

// --- IndexFormat constants ---

const (
	IndexFormatUint16 = types.IndexFormatUint16
	IndexFormatUint32 = types.IndexFormatUint32
)

// --- LoadOp constants ---

const (
	LoadOpLoad  = types.LoadOpLoad
	LoadOpClear = types.LoadOpClear
)

// --- StoreOp constants ---

const (
	StoreOpStore   = types.StoreOpStore
	StoreOpDiscard = types.StoreOpDiscard
)

// --- FilterMode constants ---

const (
	FilterModeNearest = types.FilterModeNearest
	FilterModeLinear  = types.FilterModeLinear
)

// --- AddressMode constants ---

const (
	AddressModeRepeat       = types.AddressModeRepeat
	AddressModeMirrorRepeat = types.AddressModeMirrorRepeat
	AddressModeClampToEdge  = types.AddressModeClampToEdge
)

// --- CompareFunction constants ---

const (
	CompareFunctionUndefined    = types.CompareFunctionUndefined
	CompareFunctionNever        = types.CompareFunctionNever
	CompareFunctionLess         = types.CompareFunctionLess
	CompareFunctionEqual        = types.CompareFunctionEqual
	CompareFunctionLessEqual    = types.CompareFunctionLessEqual
	CompareFunctionGreater      = types.CompareFunctionGreater
	CompareFunctionNotEqual     = types.CompareFunctionNotEqual
	CompareFunctionGreaterEqual = types.CompareFunctionGreaterEqual
	CompareFunctionAlways       = types.CompareFunctionAlways
)

// --- PresentMode constants ---

const (
	PresentModeImmediate   = types.PresentModeImmediate
	PresentModeMailbox     = types.PresentModeMailbox
	PresentModeFifo        = types.PresentModeFifo
	PresentModeFifoRelaxed = types.PresentModeFifoRelaxed
)

// --- CompositeAlphaMode constants ---

const (
	CompositeAlphaModeAuto            = types.CompositeAlphaModeAuto
	CompositeAlphaModeOpaque          = types.CompositeAlphaModeOpaque
	CompositeAlphaModePremultiplied   = types.CompositeAlphaModePremultiplied
	CompositeAlphaModeUnpremultiplied = types.CompositeAlphaModeUnpremultiplied
	CompositeAlphaModeInherit         = types.CompositeAlphaModeInherit
)

// --- PowerPreference constants ---

const (
	PowerPreferenceNone            = types.PowerPreferenceNone
	PowerPreferenceLowPower        = types.PowerPreferenceLowPower
	PowerPreferenceHighPerformance = types.PowerPreferenceHighPerformance
)

// --- ColorWriteMask constants ---

const (
	ColorWriteMaskNone  = types.ColorWriteMaskNone
	ColorWriteMaskRed   = types.ColorWriteMaskRed
	ColorWriteMaskGreen = types.ColorWriteMaskGreen
	ColorWriteMaskBlue  = types.ColorWriteMaskBlue
	ColorWriteMaskAlpha = types.ColorWriteMaskAlpha
	ColorWriteMaskAll   = types.ColorWriteMaskAll
)

// --- VertexFormat constants ---

const (
	VertexFormatUint8x2   = types.VertexFormatUint8x2
	VertexFormatUint8x4   = types.VertexFormatUint8x4
	VertexFormatSint8x2   = types.VertexFormatSint8x2
	VertexFormatSint8x4   = types.VertexFormatSint8x4
	VertexFormatFloat32   = types.VertexFormatFloat32
	VertexFormatFloat32x2 = types.VertexFormatFloat32x2
	VertexFormatFloat32x3 = types.VertexFormatFloat32x3
	VertexFormatFloat32x4 = types.VertexFormatFloat32x4
	VertexFormatUint32    = types.VertexFormatUint32
	VertexFormatUint32x2  = types.VertexFormatUint32x2
	VertexFormatUint32x3  = types.VertexFormatUint32x3
	VertexFormatUint32x4  = types.VertexFormatUint32x4
	VertexFormatSint32    = types.VertexFormatSint32
	VertexFormatSint32x2  = types.VertexFormatSint32x2
	VertexFormatSint32x3  = types.VertexFormatSint32x3
	VertexFormatSint32x4  = types.VertexFormatSint32x4
)

// --- VertexStepMode constants ---

const (
	VertexStepModeVertex   = types.VertexStepModeVertex
	VertexStepModeInstance = types.VertexStepModeInstance
)

// Binding layout types.
type BufferBindingType = types.BufferBindingType
type SamplerBindingType = types.SamplerBindingType
type TextureSampleType = types.TextureSampleType

// --- BufferBindingType constants ---

const (
	BufferBindingTypeUndefined       = types.BufferBindingTypeUndefined
	BufferBindingTypeUniform         = types.BufferBindingTypeUniform
	BufferBindingTypeStorage         = types.BufferBindingTypeStorage
	BufferBindingTypeReadOnlyStorage = types.BufferBindingTypeReadOnlyStorage
)

// --- SamplerBindingType constants ---

const (
	SamplerBindingTypeUndefined    = types.SamplerBindingTypeUndefined
	SamplerBindingTypeFiltering    = types.SamplerBindingTypeFiltering
	SamplerBindingTypeNonFiltering = types.SamplerBindingTypeNonFiltering
	SamplerBindingTypeComparison   = types.SamplerBindingTypeComparison
)

// --- TextureSampleType constants ---

const (
	TextureSampleTypeUndefined         = types.TextureSampleTypeUndefined
	TextureSampleTypeFloat             = types.TextureSampleTypeFloat
	TextureSampleTypeUnfilterableFloat = types.TextureSampleTypeUnfilterableFloat
	TextureSampleTypeDepth             = types.TextureSampleTypeDepth
	TextureSampleTypeSint              = types.TextureSampleTypeSint
	TextureSampleTypeUint              = types.TextureSampleTypeUint
)

// --- TextureViewDimension constants ---

const (
	TextureViewDimensionUndefined = types.TextureViewDimensionUndefined
	TextureViewDimension1D        = types.TextureViewDimension1D
	TextureViewDimension2D        = types.TextureViewDimension2D
	TextureViewDimension2DArray   = types.TextureViewDimension2DArray
	TextureViewDimensionCube      = types.TextureViewDimensionCube
	TextureViewDimensionCubeArray = types.TextureViewDimensionCubeArray
	TextureViewDimension3D        = types.TextureViewDimension3D
)

// --- StencilOperation constants ---

const (
	StencilOperationUndefined      = types.StencilOperationUndefined
	StencilOperationKeep           = types.StencilOperationKeep
	StencilOperationZero           = types.StencilOperationZero
	StencilOperationReplace        = types.StencilOperationReplace
	StencilOperationInvert         = types.StencilOperationInvert
	StencilOperationIncrementClamp = types.StencilOperationIncrementClamp
	StencilOperationDecrementClamp = types.StencilOperationDecrementClamp
	StencilOperationIncrementWrap  = types.StencilOperationIncrementWrap
	StencilOperationDecrementWrap  = types.StencilOperationDecrementWrap
)
