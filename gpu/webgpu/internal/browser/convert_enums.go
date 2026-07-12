package browser

import "github.com/energye/gpui/gpu/types"

// TextureFormatToJS converts a gputypes.TextureFormat to the WebGPU JS string.
// Returns "" for TextureFormatUndefined.
func TextureFormatToJS(f types.TextureFormat) string {
	s, ok := textureFormatMap[f]
	if ok {
		return s
	}
	return ""
}

// textureFormatMap maps gputypes texture format constants to WebGPU JS string names.
// Covers all formats defined in the WebGPU specification.
var textureFormatMap = map[types.TextureFormat]string{
	// 8-bit
	types.TextureFormatR8Unorm: "r8unorm",
	types.TextureFormatR8Snorm: "r8snorm",
	types.TextureFormatR8Uint:  "r8uint",
	types.TextureFormatR8Sint:  "r8sint",

	// 16-bit
	types.TextureFormatR16Uint:  "r16uint",
	types.TextureFormatR16Sint:  "r16sint",
	types.TextureFormatR16Float: "r16float",
	types.TextureFormatRG8Unorm: "rg8unorm",
	types.TextureFormatRG8Snorm: "rg8snorm",
	types.TextureFormatRG8Uint:  "rg8uint",
	types.TextureFormatRG8Sint:  "rg8sint",

	// 32-bit
	types.TextureFormatR32Float:       "r32float",
	types.TextureFormatR32Uint:        "r32uint",
	types.TextureFormatR32Sint:        "r32sint",
	types.TextureFormatRG16Uint:       "rg16uint",
	types.TextureFormatRG16Sint:       "rg16sint",
	types.TextureFormatRG16Float:      "rg16float",
	types.TextureFormatRGBA8Unorm:     "rgba8unorm",
	types.TextureFormatRGBA8UnormSrgb: "rgba8unorm-srgb",
	types.TextureFormatRGBA8Snorm:     "rgba8snorm",
	types.TextureFormatRGBA8Uint:      "rgba8uint",
	types.TextureFormatRGBA8Sint:      "rgba8sint",
	types.TextureFormatBGRA8Unorm:     "bgra8unorm",
	types.TextureFormatBGRA8UnormSrgb: "bgra8unorm-srgb",

	// Packed 32-bit
	types.TextureFormatRGB10A2Uint:   "rgb10a2uint",
	types.TextureFormatRGB10A2Unorm:  "rgb10a2unorm",
	types.TextureFormatRG11B10Ufloat: "rg11b10ufloat",
	types.TextureFormatRGB9E5Ufloat:  "rgb9e5ufloat",

	// 64-bit
	types.TextureFormatRG32Float:   "rg32float",
	types.TextureFormatRG32Uint:    "rg32uint",
	types.TextureFormatRG32Sint:    "rg32sint",
	types.TextureFormatRGBA16Uint:  "rgba16uint",
	types.TextureFormatRGBA16Sint:  "rgba16sint",
	types.TextureFormatRGBA16Float: "rgba16float",

	// 128-bit
	types.TextureFormatRGBA32Float: "rgba32float",
	types.TextureFormatRGBA32Uint:  "rgba32uint",
	types.TextureFormatRGBA32Sint:  "rgba32sint",

	// Depth/stencil
	types.TextureFormatStencil8:             "stencil8",
	types.TextureFormatDepth16Unorm:         "depth16unorm",
	types.TextureFormatDepth24Plus:          "depth24plus",
	types.TextureFormatDepth24PlusStencil8:  "depth24plus-stencil8",
	types.TextureFormatDepth32Float:         "depth32float",
	types.TextureFormatDepth32FloatStencil8: "depth32float-stencil8",

	// BC compressed
	types.TextureFormatBC1RGBAUnorm:     "bc1-rgba-unorm",
	types.TextureFormatBC1RGBAUnormSrgb: "bc1-rgba-unorm-srgb",
	types.TextureFormatBC2RGBAUnorm:     "bc2-rgba-unorm",
	types.TextureFormatBC2RGBAUnormSrgb: "bc2-rgba-unorm-srgb",
	types.TextureFormatBC3RGBAUnorm:     "bc3-rgba-unorm",
	types.TextureFormatBC3RGBAUnormSrgb: "bc3-rgba-unorm-srgb",
	types.TextureFormatBC4RUnorm:        "bc4-r-unorm",
	types.TextureFormatBC4RSnorm:        "bc4-r-snorm",
	types.TextureFormatBC5RGUnorm:       "bc5-rg-unorm",
	types.TextureFormatBC5RGSnorm:       "bc5-rg-snorm",
	types.TextureFormatBC6HRGBUfloat:    "bc6h-rgb-ufloat",
	types.TextureFormatBC6HRGBFloat:     "bc6h-rgb-float",
	types.TextureFormatBC7RGBAUnorm:     "bc7-rgba-unorm",
	types.TextureFormatBC7RGBAUnormSrgb: "bc7-rgba-unorm-srgb",

	// ETC2 compressed
	types.TextureFormatETC2RGB8Unorm:       "etc2-rgb8unorm",
	types.TextureFormatETC2RGB8UnormSrgb:   "etc2-rgb8unorm-srgb",
	types.TextureFormatETC2RGB8A1Unorm:     "etc2-rgb8a1unorm",
	types.TextureFormatETC2RGB8A1UnormSrgb: "etc2-rgb8a1unorm-srgb",
	types.TextureFormatETC2RGBA8Unorm:      "etc2-rgba8unorm",
	types.TextureFormatETC2RGBA8UnormSrgb:  "etc2-rgba8unorm-srgb",
	types.TextureFormatEACR11Unorm:         "eac-r11unorm",
	types.TextureFormatEACR11Snorm:         "eac-r11snorm",
	types.TextureFormatEACRG11Unorm:        "eac-rg11unorm",
	types.TextureFormatEACRG11Snorm:        "eac-rg11snorm",

	// ASTC compressed
	types.TextureFormatASTC4x4Unorm:       "astc-4x4-unorm",
	types.TextureFormatASTC4x4UnormSrgb:   "astc-4x4-unorm-srgb",
	types.TextureFormatASTC5x4Unorm:       "astc-5x4-unorm",
	types.TextureFormatASTC5x4UnormSrgb:   "astc-5x4-unorm-srgb",
	types.TextureFormatASTC5x5Unorm:       "astc-5x5-unorm",
	types.TextureFormatASTC5x5UnormSrgb:   "astc-5x5-unorm-srgb",
	types.TextureFormatASTC6x5Unorm:       "astc-6x5-unorm",
	types.TextureFormatASTC6x5UnormSrgb:   "astc-6x5-unorm-srgb",
	types.TextureFormatASTC6x6Unorm:       "astc-6x6-unorm",
	types.TextureFormatASTC6x6UnormSrgb:   "astc-6x6-unorm-srgb",
	types.TextureFormatASTC8x5Unorm:       "astc-8x5-unorm",
	types.TextureFormatASTC8x5UnormSrgb:   "astc-8x5-unorm-srgb",
	types.TextureFormatASTC8x6Unorm:       "astc-8x6-unorm",
	types.TextureFormatASTC8x6UnormSrgb:   "astc-8x6-unorm-srgb",
	types.TextureFormatASTC8x8Unorm:       "astc-8x8-unorm",
	types.TextureFormatASTC8x8UnormSrgb:   "astc-8x8-unorm-srgb",
	types.TextureFormatASTC10x5Unorm:      "astc-10x5-unorm",
	types.TextureFormatASTC10x5UnormSrgb:  "astc-10x5-unorm-srgb",
	types.TextureFormatASTC10x6Unorm:      "astc-10x6-unorm",
	types.TextureFormatASTC10x6UnormSrgb:  "astc-10x6-unorm-srgb",
	types.TextureFormatASTC10x8Unorm:      "astc-10x8-unorm",
	types.TextureFormatASTC10x8UnormSrgb:  "astc-10x8-unorm-srgb",
	types.TextureFormatASTC10x10Unorm:     "astc-10x10-unorm",
	types.TextureFormatASTC10x10UnormSrgb: "astc-10x10-unorm-srgb",
	types.TextureFormatASTC12x10Unorm:     "astc-12x10-unorm",
	types.TextureFormatASTC12x10UnormSrgb: "astc-12x10-unorm-srgb",
	types.TextureFormatASTC12x12Unorm:     "astc-12x12-unorm",
	types.TextureFormatASTC12x12UnormSrgb: "astc-12x12-unorm-srgb",
}

// TextureDimensionToJS converts a gputypes.TextureDimension to the WebGPU JS string.
func TextureDimensionToJS(d types.TextureDimension) string {
	switch d {
	case types.TextureDimension1D:
		return "1d"
	case types.TextureDimension2D:
		return "2d"
	case types.TextureDimension3D:
		return "3d"
	default:
		return ""
	}
}

// TextureViewDimensionToJS converts a gputypes.TextureViewDimension to JS string.
func TextureViewDimensionToJS(d types.TextureViewDimension) string {
	switch d {
	case types.TextureViewDimension1D:
		return "1d"
	case types.TextureViewDimension2D:
		return "2d"
	case types.TextureViewDimension2DArray:
		return "2d-array"
	case types.TextureViewDimensionCube:
		return "cube"
	case types.TextureViewDimensionCubeArray:
		return "cube-array"
	case types.TextureViewDimension3D:
		return "3d"
	default:
		return ""
	}
}

// TextureAspectToJS converts a gputypes.TextureAspect to JS string.
func TextureAspectToJS(a types.TextureAspect) string {
	switch a {
	case types.TextureAspectAll:
		return "all"
	case types.TextureAspectStencilOnly:
		return "stencil-only"
	case types.TextureAspectDepthOnly:
		return "depth-only"
	default:
		return ""
	}
}

// AddressModeToJS converts a gputypes.AddressMode to JS string.
func AddressModeToJS(m types.AddressMode) string {
	switch m {
	case types.AddressModeClampToEdge:
		return "clamp-to-edge"
	case types.AddressModeRepeat:
		return "repeat"
	case types.AddressModeMirrorRepeat:
		return "mirror-repeat"
	default:
		return "clamp-to-edge"
	}
}

// FilterModeToJS converts a gputypes.FilterMode to JS string.
func FilterModeToJS(m types.FilterMode) string {
	switch m {
	case types.FilterModeNearest:
		return "nearest"
	case types.FilterModeLinear:
		return "linear"
	default:
		return "nearest"
	}
}

// CompareFunctionToJS converts a gputypes.CompareFunction to JS string.
func CompareFunctionToJS(f types.CompareFunction) string {
	switch f {
	case types.CompareFunctionNever:
		return "never"
	case types.CompareFunctionLess:
		return "less"
	case types.CompareFunctionEqual:
		return "equal"
	case types.CompareFunctionLessEqual:
		return "less-equal"
	case types.CompareFunctionGreater:
		return "greater"
	case types.CompareFunctionNotEqual:
		return "not-equal"
	case types.CompareFunctionGreaterEqual:
		return "greater-equal"
	case types.CompareFunctionAlways:
		return "always"
	default:
		return ""
	}
}

// PrimitiveTopologyToJS converts a gputypes.PrimitiveTopology to JS string.
func PrimitiveTopologyToJS(t types.PrimitiveTopology) string {
	switch t {
	case types.PrimitiveTopologyPointList:
		return "point-list"
	case types.PrimitiveTopologyLineList:
		return "line-list"
	case types.PrimitiveTopologyLineStrip:
		return "line-strip"
	case types.PrimitiveTopologyTriangleList:
		return "triangle-list"
	case types.PrimitiveTopologyTriangleStrip:
		return "triangle-strip"
	default:
		return "triangle-list"
	}
}

// FrontFaceToJS converts a gputypes.FrontFace to JS string.
func FrontFaceToJS(f types.FrontFace) string {
	switch f {
	case types.FrontFaceCW:
		return "cw"
	default:
		return "ccw"
	}
}

// CullModeToJS converts a gputypes.CullMode to JS string.
func CullModeToJS(m types.CullMode) string {
	switch m {
	case types.CullModeFront:
		return "front"
	case types.CullModeBack:
		return "back"
	default:
		return "none"
	}
}

// IndexFormatToJS converts a gputypes.IndexFormat to JS string.
func IndexFormatToJS(f types.IndexFormat) string {
	switch f {
	case types.IndexFormatUint16:
		return "uint16"
	case types.IndexFormatUint32:
		return "uint32"
	default:
		return ""
	}
}

// VertexFormatToJS converts a gputypes.VertexFormat to JS string.
func VertexFormatToJS(f types.VertexFormat) string {
	s, ok := vertexFormatMap[f]
	if ok {
		return s
	}
	return ""
}

var vertexFormatMap = map[types.VertexFormat]string{
	types.VertexFormatUint8x2:      "uint8x2",
	types.VertexFormatUint8x4:      "uint8x4",
	types.VertexFormatSint8x2:      "sint8x2",
	types.VertexFormatSint8x4:      "sint8x4",
	types.VertexFormatUnorm8x2:     "unorm8x2",
	types.VertexFormatUnorm8x4:     "unorm8x4",
	types.VertexFormatSnorm8x2:     "snorm8x2",
	types.VertexFormatSnorm8x4:     "snorm8x4",
	types.VertexFormatUint16x2:     "uint16x2",
	types.VertexFormatUint16x4:     "uint16x4",
	types.VertexFormatSint16x2:     "sint16x2",
	types.VertexFormatSint16x4:     "sint16x4",
	types.VertexFormatUnorm16x2:    "unorm16x2",
	types.VertexFormatUnorm16x4:    "unorm16x4",
	types.VertexFormatSnorm16x2:    "snorm16x2",
	types.VertexFormatSnorm16x4:    "snorm16x4",
	types.VertexFormatFloat16x2:    "float16x2",
	types.VertexFormatFloat16x4:    "float16x4",
	types.VertexFormatFloat32:      "float32",
	types.VertexFormatFloat32x2:    "float32x2",
	types.VertexFormatFloat32x3:    "float32x3",
	types.VertexFormatFloat32x4:    "float32x4",
	types.VertexFormatUint32:       "uint32",
	types.VertexFormatUint32x2:     "uint32x2",
	types.VertexFormatUint32x3:     "uint32x3",
	types.VertexFormatUint32x4:     "uint32x4",
	types.VertexFormatSint32:       "sint32",
	types.VertexFormatSint32x2:     "sint32x2",
	types.VertexFormatSint32x3:     "sint32x3",
	types.VertexFormatSint32x4:     "sint32x4",
	types.VertexFormatUnorm1010102: "unorm10-10-10-2",
}

// VertexStepModeToJS converts a gputypes.VertexStepMode to JS string.
func VertexStepModeToJS(m types.VertexStepMode) string {
	switch m {
	case types.VertexStepModeInstance:
		return "instance"
	default:
		return "vertex"
	}
}

// BlendFactorToJS converts a gputypes.BlendFactor to JS string.
func BlendFactorToJS(f types.BlendFactor) string {
	switch f {
	case types.BlendFactorZero:
		return "zero"
	case types.BlendFactorOne:
		return "one"
	case types.BlendFactorSrc:
		return "src"
	case types.BlendFactorOneMinusSrc:
		return "one-minus-src"
	case types.BlendFactorSrcAlpha:
		return "src-alpha"
	case types.BlendFactorOneMinusSrcAlpha:
		return "one-minus-src-alpha"
	case types.BlendFactorDst:
		return "dst"
	case types.BlendFactorOneMinusDst:
		return "one-minus-dst"
	case types.BlendFactorDstAlpha:
		return "dst-alpha"
	case types.BlendFactorOneMinusDstAlpha:
		return "one-minus-dst-alpha"
	case types.BlendFactorSrcAlphaSaturated:
		return "src-alpha-saturated"
	case types.BlendFactorConstant:
		return "constant"
	case types.BlendFactorOneMinusConstant:
		return "one-minus-constant"
	default:
		return "one"
	}
}

// BlendOperationToJS converts a gputypes.BlendOperation to JS string.
func BlendOperationToJS(op types.BlendOperation) string {
	switch op {
	case types.BlendOperationAdd:
		return "add"
	case types.BlendOperationSubtract:
		return "subtract"
	case types.BlendOperationReverseSubtract:
		return "reverse-subtract"
	case types.BlendOperationMin:
		return "min"
	case types.BlendOperationMax:
		return "max"
	default:
		return "add"
	}
}

// StencilOperationToJS converts a gputypes.StencilOperation to JS string.
func StencilOperationToJS(op types.StencilOperation) string {
	switch op {
	case types.StencilOperationKeep:
		return "keep"
	case types.StencilOperationZero:
		return "zero"
	case types.StencilOperationReplace:
		return "replace"
	case types.StencilOperationInvert:
		return "invert"
	case types.StencilOperationIncrementClamp:
		return "increment-clamp"
	case types.StencilOperationDecrementClamp:
		return "decrement-clamp"
	case types.StencilOperationIncrementWrap:
		return "increment-wrap"
	case types.StencilOperationDecrementWrap:
		return "decrement-wrap"
	default:
		return "keep"
	}
}

// BufferBindingTypeToJS converts a gputypes.BufferBindingType to JS string.
func BufferBindingTypeToJS(t types.BufferBindingType) string {
	switch t {
	case types.BufferBindingTypeUniform:
		return "uniform"
	case types.BufferBindingTypeStorage:
		return "storage"
	case types.BufferBindingTypeReadOnlyStorage:
		return "read-only-storage"
	default:
		return "uniform"
	}
}

// SamplerBindingTypeToJS converts a gputypes.SamplerBindingType to JS string.
func SamplerBindingTypeToJS(t types.SamplerBindingType) string {
	switch t {
	case types.SamplerBindingTypeFiltering:
		return "filtering"
	case types.SamplerBindingTypeNonFiltering:
		return "non-filtering"
	case types.SamplerBindingTypeComparison:
		return "comparison"
	default:
		return "filtering"
	}
}

// TextureSampleTypeToJS converts a gputypes.TextureSampleType to JS string.
func TextureSampleTypeToJS(t types.TextureSampleType) string {
	switch t {
	case types.TextureSampleTypeFloat:
		return "float"
	case types.TextureSampleTypeUnfilterableFloat:
		return "unfilterable-float"
	case types.TextureSampleTypeDepth:
		return "depth"
	case types.TextureSampleTypeSint:
		return "sint"
	case types.TextureSampleTypeUint:
		return "uint"
	default:
		return "float"
	}
}

// StorageTextureAccessToJS converts a gputypes.StorageTextureAccess to JS string.
func StorageTextureAccessToJS(a types.StorageTextureAccess) string {
	switch a {
	case types.StorageTextureAccessWriteOnly:
		return "write-only"
	case types.StorageTextureAccessReadOnly:
		return "read-only"
	case types.StorageTextureAccessReadWrite:
		return "read-write"
	default:
		return "write-only"
	}
}

// LoadOpToJS converts a gputypes.LoadOp to the WebGPU JS string.
// Returns "load" for LoadOpLoad, "clear" for LoadOpClear, and "load" as default.
func LoadOpToJS(op types.LoadOp) string {
	switch op {
	case types.LoadOpClear:
		return "clear"
	case types.LoadOpLoad:
		return "load"
	default:
		return "load"
	}
}

// StoreOpToJS converts a gputypes.StoreOp to the WebGPU JS string.
// Returns "store" for StoreOpStore, "discard" for StoreOpDiscard, and "store" as default.
func StoreOpToJS(op types.StoreOp) string {
	switch op {
	case types.StoreOpDiscard:
		return "discard"
	case types.StoreOpStore:
		return "store"
	default:
		return "store"
	}
}
