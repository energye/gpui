package webgpu

import "github.com/energye/gpui/gpu/types"

// MinBindGroups is the minimum guaranteed number of bind groups per WebGPU spec.
// All compliant devices support at least 4 bind groups. Portable code should
// use no more than MinBindGroups.
const MinBindGroups = 4

// MaxBindGroups is the HAL hard cap on bind groups (wgpu-hal MAX_BIND_GROUPS = 8).
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

// Backends masks
const (
	BackendsAll     = types.BackendsAll
	BackendsPrimary = types.BackendsPrimary
	BackendsVulkan  = types.BackendsVulkan
	BackendsMetal   = types.BackendsMetal
	BackendsDX12    = types.BackendsDX12
	BackendsGL      = types.BackendsGL
)

// Feature and limit types
type Features = types.Features
type Limits = types.Limits

// Buffer usage
type BufferUsage = types.BufferUsage

const (
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

// Texture types
type TextureUsage = types.TextureUsage

const (
	TextureUsageCopySrc          = types.TextureUsageCopySrc
	TextureUsageCopyDst          = types.TextureUsageCopyDst
	TextureUsageTextureBinding   = types.TextureUsageTextureBinding
	TextureUsageStorageBinding   = types.TextureUsageStorageBinding
	TextureUsageRenderAttachment = types.TextureUsageRenderAttachment
)

type TextureFormat = types.TextureFormat
type TextureDimension = types.TextureDimension
type TextureViewDimension = types.TextureViewDimension
type TextureAspect = types.TextureAspect

// Texture dimension constants
const (
	TextureDimension1D = types.TextureDimension1D
	TextureDimension2D = types.TextureDimension2D
	TextureDimension3D = types.TextureDimension3D
)

// Commonly used texture format constants
const (
	TextureFormatRGBA8Unorm     = types.TextureFormatRGBA8Unorm
	TextureFormatRGBA8UnormSrgb = types.TextureFormatRGBA8UnormSrgb
	TextureFormatBGRA8Unorm     = types.TextureFormatBGRA8Unorm
	TextureFormatBGRA8UnormSrgb = types.TextureFormatBGRA8UnormSrgb
	TextureFormatDepth24Plus    = types.TextureFormatDepth24Plus
	TextureFormatDepth32Float   = types.TextureFormatDepth32Float
)

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
type PowerPreference = types.PowerPreference

// RequestAdapterOptions controls adapter selection.
//
// Following the WebGPU spec, CompatibleSurface is a typed *Surface pointer
// (not a raw handle). Backends that require a surface for adapter enumeration
// (e.g., GLES/OpenGL which needs a GL context) use this to perform deferred
// enumeration when RequestAdapter is called.
type RequestAdapterOptions struct {
	// PowerPreference indicates power consumption preference.
	PowerPreference PowerPreference
	// ForceFallbackAdapter forces the use of a fallback (software) adapter.
	ForceFallbackAdapter bool
	// CompatibleSurface, if non-nil, indicates that the adapter must support
	// rendering to this surface. For GLES backends, this triggers deferred
	// adapter enumeration using the surface's GL context.
	CompatibleSurface *Surface
}

const (
	PowerPreferenceNone            = types.PowerPreferenceNone
	PowerPreferenceLowPower        = types.PowerPreferenceLowPower
	PowerPreferenceHighPerformance = types.PowerPreferenceHighPerformance
)

// Default functions (re-exported for convenience)
var (
	DefaultLimits             = types.DefaultLimits
	DefaultInstanceDescriptor = types.DefaultInstanceDescriptor
)
