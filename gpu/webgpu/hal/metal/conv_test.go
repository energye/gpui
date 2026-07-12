// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build darwin && !(js && wasm)

package metal

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// TestTextureFormatToMTL tests texture format conversions to Metal pixel formats.
func TestTextureFormatToMTL(t *testing.T) {
	tests := []struct {
		name   string
		format types.TextureFormat
		expect MTLPixelFormat
	}{
		// 8-bit formats
		{"R8Unorm", types.TextureFormatR8Unorm, MTLPixelFormatR8Unorm},
		{"R8Snorm", types.TextureFormatR8Snorm, MTLPixelFormatR8Snorm},
		{"R8Uint", types.TextureFormatR8Uint, MTLPixelFormatR8Uint},
		{"R8Sint", types.TextureFormatR8Sint, MTLPixelFormatR8Sint},

		// 16-bit formats
		{"R16Uint", types.TextureFormatR16Uint, MTLPixelFormatR16Uint},
		{"R16Sint", types.TextureFormatR16Sint, MTLPixelFormatR16Sint},
		{"R16Float", types.TextureFormatR16Float, MTLPixelFormatR16Float},
		{"RG8Unorm", types.TextureFormatRG8Unorm, MTLPixelFormatRG8Unorm},
		{"RG8Snorm", types.TextureFormatRG8Snorm, MTLPixelFormatRG8Snorm},
		{"RG8Uint", types.TextureFormatRG8Uint, MTLPixelFormatRG8Uint},
		{"RG8Sint", types.TextureFormatRG8Sint, MTLPixelFormatRG8Sint},

		// 32-bit formats
		{"R32Uint", types.TextureFormatR32Uint, MTLPixelFormatR32Uint},
		{"R32Sint", types.TextureFormatR32Sint, MTLPixelFormatR32Sint},
		{"R32Float", types.TextureFormatR32Float, MTLPixelFormatR32Float},
		{"RG16Uint", types.TextureFormatRG16Uint, MTLPixelFormatRG16Uint},
		{"RG16Sint", types.TextureFormatRG16Sint, MTLPixelFormatRG16Sint},
		{"RG16Float", types.TextureFormatRG16Float, MTLPixelFormatRG16Float},
		{"RGBA8Unorm", types.TextureFormatRGBA8Unorm, MTLPixelFormatRGBA8Unorm},
		{"RGBA8UnormSrgb", types.TextureFormatRGBA8UnormSrgb, MTLPixelFormatRGBA8UnormSRGB},
		{"RGBA8Snorm", types.TextureFormatRGBA8Snorm, MTLPixelFormatRGBA8Snorm},
		{"RGBA8Uint", types.TextureFormatRGBA8Uint, MTLPixelFormatRGBA8Uint},
		{"RGBA8Sint", types.TextureFormatRGBA8Sint, MTLPixelFormatRGBA8Sint},
		{"BGRA8Unorm", types.TextureFormatBGRA8Unorm, MTLPixelFormatBGRA8Unorm},
		{"BGRA8UnormSrgb", types.TextureFormatBGRA8UnormSrgb, MTLPixelFormatBGRA8UnormSRGB},

		// Packed formats
		{"RGB10A2Unorm", types.TextureFormatRGB10A2Unorm, MTLPixelFormatRGB10A2Unorm},
		{"RG11B10Ufloat", types.TextureFormatRG11B10Ufloat, MTLPixelFormatRG11B10Float},
		{"RGB9E5Ufloat", types.TextureFormatRGB9E5Ufloat, MTLPixelFormatRGB9E5Float},

		// 64-bit formats
		{"RG32Uint", types.TextureFormatRG32Uint, MTLPixelFormatRG32Uint},
		{"RG32Sint", types.TextureFormatRG32Sint, MTLPixelFormatRG32Sint},
		{"RG32Float", types.TextureFormatRG32Float, MTLPixelFormatRG32Float},
		{"RGBA16Uint", types.TextureFormatRGBA16Uint, MTLPixelFormatRGBA16Uint},
		{"RGBA16Sint", types.TextureFormatRGBA16Sint, MTLPixelFormatRGBA16Sint},
		{"RGBA16Float", types.TextureFormatRGBA16Float, MTLPixelFormatRGBA16Float},

		// 128-bit formats
		{"RGBA32Uint", types.TextureFormatRGBA32Uint, MTLPixelFormatRGBA32Uint},
		{"RGBA32Sint", types.TextureFormatRGBA32Sint, MTLPixelFormatRGBA32Sint},
		{"RGBA32Float", types.TextureFormatRGBA32Float, MTLPixelFormatRGBA32Float},

		// Depth/stencil formats
		{"Depth16Unorm", types.TextureFormatDepth16Unorm, MTLPixelFormatDepth16Unorm},
		{"Depth32Float", types.TextureFormatDepth32Float, MTLPixelFormatDepth32Float},
		{"Depth24Plus", types.TextureFormatDepth24Plus, MTLPixelFormatDepth32Float},
		{"Depth24PlusStencil8", types.TextureFormatDepth24PlusStencil8, MTLPixelFormatDepth32FloatStencil8},
		{"Depth32FloatStencil8", types.TextureFormatDepth32FloatStencil8, MTLPixelFormatDepth32FloatStencil8},
		{"Stencil8", types.TextureFormatStencil8, MTLPixelFormatStencil8},

		// Unknown format
		{"Unknown", types.TextureFormat(65535), MTLPixelFormatInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureFormatToMTL(tt.format)
			if got != tt.expect {
				t.Errorf("textureFormatToMTL(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

// TestTextureUsageToMTL tests texture usage flag conversions.
func TestTextureUsageToMTL(t *testing.T) {
	tests := []struct {
		name   string
		usage  types.TextureUsage
		expect MTLTextureUsage
	}{
		{"CopySrc", types.TextureUsageCopySrc, MTLTextureUsageShaderRead},
		{"CopyDst", types.TextureUsageCopyDst, MTLTextureUsageShaderRead},
		{"TextureBinding", types.TextureUsageTextureBinding, MTLTextureUsageShaderRead},
		{"StorageBinding", types.TextureUsageStorageBinding, MTLTextureUsageShaderRead | MTLTextureUsageShaderWrite},
		{"RenderAttachment", types.TextureUsageRenderAttachment, MTLTextureUsageRenderTarget},
		{
			"CopySrc and TextureBinding",
			types.TextureUsageCopySrc | types.TextureUsageTextureBinding,
			MTLTextureUsageShaderRead,
		},
		{
			"All",
			types.TextureUsageCopySrc | types.TextureUsageCopyDst | types.TextureUsageTextureBinding | types.TextureUsageStorageBinding | types.TextureUsageRenderAttachment,
			MTLTextureUsageShaderRead | MTLTextureUsageShaderWrite | MTLTextureUsageRenderTarget,
		},
		{"None defaults to Unknown", types.TextureUsage(0), MTLTextureUsageUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureUsageToMTL(tt.usage)
			if got != tt.expect {
				t.Errorf("textureUsageToMTL(%v) = %v, want %v", tt.usage, got, tt.expect)
			}
		})
	}
}

// TestTextureTypeFromDimension tests texture type from dimension conversions.
func TestTextureTypeFromDimension(t *testing.T) {
	tests := []struct {
		name        string
		dimension   types.TextureDimension
		sampleCount uint32
		depth       uint32
		expect      MTLTextureType
	}{
		{"1D", types.TextureDimension1D, 1, 1, MTLTextureType1D},
		{"1DArray", types.TextureDimension1D, 1, 3, MTLTextureType1DArray},
		{"2D", types.TextureDimension2D, 1, 1, MTLTextureType2D},
		{"2DArray", types.TextureDimension2D, 1, 3, MTLTextureType2DArray},
		{"2DMultisample", types.TextureDimension2D, 4, 1, MTLTextureType2DMultisample},
		{"3D", types.TextureDimension3D, 1, 1, MTLTextureType3D},
		{"Unknown defaults to 2D", types.TextureDimension(99), 1, 1, MTLTextureType2D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureTypeFromDimension(tt.dimension, tt.sampleCount, tt.depth)
			if got != tt.expect {
				t.Errorf("textureTypeFromDimension(%v, %d, %d) = %v, want %v",
					tt.dimension, tt.sampleCount, tt.depth, got, tt.expect)
			}
		})
	}
}

// TestTextureViewDimensionToMTL tests texture view dimension conversions.
func TestTextureViewDimensionToMTL(t *testing.T) {
	tests := []struct {
		name      string
		dimension types.TextureViewDimension
		expect    MTLTextureType
	}{
		{"1D", types.TextureViewDimension1D, MTLTextureType1D},
		{"2D", types.TextureViewDimension2D, MTLTextureType2D},
		{"2DArray", types.TextureViewDimension2DArray, MTLTextureType2DArray},
		{"Cube", types.TextureViewDimensionCube, MTLTextureTypeCube},
		{"CubeArray", types.TextureViewDimensionCubeArray, MTLTextureTypeCubeArray},
		{"3D", types.TextureViewDimension3D, MTLTextureType3D},
		{"Unknown defaults to 2D", types.TextureViewDimension(99), MTLTextureType2D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureViewDimensionToMTL(tt.dimension)
			if got != tt.expect {
				t.Errorf("textureViewDimensionToMTL(%v) = %v, want %v", tt.dimension, got, tt.expect)
			}
		})
	}
}

// TestFilterModeToMTL tests filter mode conversions.
func TestFilterModeToMTL(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.FilterMode
		expect MTLSamplerMinMagFilter
	}{
		{"Nearest", types.FilterModeNearest, MTLSamplerMinMagFilterNearest},
		{"Linear", types.FilterModeLinear, MTLSamplerMinMagFilterLinear},
		{"Unknown defaults to Nearest", types.FilterMode(99), MTLSamplerMinMagFilterNearest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterModeToMTL(tt.mode)
			if got != tt.expect {
				t.Errorf("filterModeToMTL(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

// TestMipmapFilterModeToMTL tests mipmap filter mode conversions.
func TestMipmapFilterModeToMTL(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.FilterMode
		expect MTLSamplerMipFilter
	}{
		{"Nearest", types.FilterModeNearest, MTLSamplerMipFilterNearest},
		{"Linear", types.FilterModeLinear, MTLSamplerMipFilterLinear},
		{"Unknown defaults to NotMipmapped", types.FilterMode(99), MTLSamplerMipFilterNotMipmapped},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mipmapFilterModeToMTL(tt.mode)
			if got != tt.expect {
				t.Errorf("mipmapFilterModeToMTL(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

// TestAddressModeToMTL tests address mode conversions.
func TestAddressModeToMTL(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.AddressMode
		expect MTLSamplerAddressMode
	}{
		{"ClampToEdge", types.AddressModeClampToEdge, MTLSamplerAddressModeClampToEdge},
		{"Repeat", types.AddressModeRepeat, MTLSamplerAddressModeRepeat},
		{"MirrorRepeat", types.AddressModeMirrorRepeat, MTLSamplerAddressModeMirrorRepeat},
		{"Unknown defaults to ClampToEdge", types.AddressMode(99), MTLSamplerAddressModeClampToEdge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addressModeToMTL(tt.mode)
			if got != tt.expect {
				t.Errorf("addressModeToMTL(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

// TestCompareFunctionToMTL tests compare function conversions.
func TestCompareFunctionToMTL(t *testing.T) {
	tests := []struct {
		name   string
		fn     types.CompareFunction
		expect MTLCompareFunction
	}{
		{"Never", types.CompareFunctionNever, MTLCompareFunctionNever},
		{"Less", types.CompareFunctionLess, MTLCompareFunctionLess},
		{"Equal", types.CompareFunctionEqual, MTLCompareFunctionEqual},
		{"LessEqual", types.CompareFunctionLessEqual, MTLCompareFunctionLessEqual},
		{"Greater", types.CompareFunctionGreater, MTLCompareFunctionGreater},
		{"NotEqual", types.CompareFunctionNotEqual, MTLCompareFunctionNotEqual},
		{"GreaterEqual", types.CompareFunctionGreaterEqual, MTLCompareFunctionGreaterEqual},
		{"Always", types.CompareFunctionAlways, MTLCompareFunctionAlways},
		{"Unknown defaults to Always", types.CompareFunction(99), MTLCompareFunctionAlways},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareFunctionToMTL(tt.fn)
			if got != tt.expect {
				t.Errorf("compareFunctionToMTL(%v) = %v, want %v", tt.fn, got, tt.expect)
			}
		})
	}
}

// TestStencilOperationToMTL guards the WebGPU→Metal stencil-op mapping. The two
// enums use different numeric values (WebGPU Keep=1, Metal Keep=0), so this must
// be an explicit table — a missing/incorrect entry silently disables stencil ops
// (the macOS rounded-UI-renders-as-squares regression).
func TestStencilOperationToMTL(t *testing.T) {
	tests := []struct {
		name   string
		op     types.StencilOperation
		expect MTLStencilOperation
	}{
		{"Keep", types.StencilOperationKeep, MTLStencilOperationKeep},
		{"Zero", types.StencilOperationZero, MTLStencilOperationZero},
		{"Replace", types.StencilOperationReplace, MTLStencilOperationReplace},
		{"Invert", types.StencilOperationInvert, MTLStencilOperationInvert},
		{"IncrementClamp", types.StencilOperationIncrementClamp, MTLStencilOperationIncrementClamp},
		{"DecrementClamp", types.StencilOperationDecrementClamp, MTLStencilOperationDecrementClamp},
		{"IncrementWrap", types.StencilOperationIncrementWrap, MTLStencilOperationIncrementWrap},
		{"DecrementWrap", types.StencilOperationDecrementWrap, MTLStencilOperationDecrementWrap},
		{"Unknown defaults to Keep", types.StencilOperation(99), MTLStencilOperationKeep},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stencilOperationToMTL(tt.op)
			if got != tt.expect {
				t.Errorf("stencilOperationToMTL(%v) = %v, want %v", tt.op, got, tt.expect)
			}
		})
	}
}

// TestPrimitiveTopologyToMTL tests primitive topology conversions.
func TestPrimitiveTopologyToMTL(t *testing.T) {
	tests := []struct {
		name     string
		topology types.PrimitiveTopology
		expect   MTLPrimitiveType
	}{
		{"PointList", types.PrimitiveTopologyPointList, MTLPrimitiveTypePoint},
		{"LineList", types.PrimitiveTopologyLineList, MTLPrimitiveTypeLine},
		{"LineStrip", types.PrimitiveTopologyLineStrip, MTLPrimitiveTypeLineStrip},
		{"TriangleList", types.PrimitiveTopologyTriangleList, MTLPrimitiveTypeTriangle},
		{"TriangleStrip", types.PrimitiveTopologyTriangleStrip, MTLPrimitiveTypeTriangleStrip},
		{"Unknown defaults to Triangle", types.PrimitiveTopology(99), MTLPrimitiveTypeTriangle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primitiveTopologyToMTL(tt.topology)
			if got != tt.expect {
				t.Errorf("primitiveTopologyToMTL(%v) = %v, want %v", tt.topology, got, tt.expect)
			}
		})
	}
}

// TestBlendFactorToMTL tests blend factor conversions.
func TestBlendFactorToMTL(t *testing.T) {
	tests := []struct {
		name   string
		factor types.BlendFactor
		expect MTLBlendFactor
	}{
		{"Zero", types.BlendFactorZero, MTLBlendFactorZero},
		{"One", types.BlendFactorOne, MTLBlendFactorOne},
		{"Src", types.BlendFactorSrc, MTLBlendFactorSourceColor},
		{"OneMinusSrc", types.BlendFactorOneMinusSrc, MTLBlendFactorOneMinusSourceColor},
		{"SrcAlpha", types.BlendFactorSrcAlpha, MTLBlendFactorSourceAlpha},
		{"OneMinusSrcAlpha", types.BlendFactorOneMinusSrcAlpha, MTLBlendFactorOneMinusSourceAlpha},
		{"Dst", types.BlendFactorDst, MTLBlendFactorDestinationColor},
		{"OneMinusDst", types.BlendFactorOneMinusDst, MTLBlendFactorOneMinusDestinationColor},
		{"DstAlpha", types.BlendFactorDstAlpha, MTLBlendFactorDestinationAlpha},
		{"OneMinusDstAlpha", types.BlendFactorOneMinusDstAlpha, MTLBlendFactorOneMinusDestinationAlpha},
		{"SrcAlphaSaturated", types.BlendFactorSrcAlphaSaturated, MTLBlendFactorSourceAlphaSaturated},
		{"Constant", types.BlendFactorConstant, MTLBlendFactorBlendColor},
		{"OneMinusConstant", types.BlendFactorOneMinusConstant, MTLBlendFactorOneMinusBlendColor},
		{"Unknown defaults to One", types.BlendFactor(99), MTLBlendFactorOne},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blendFactorToMTL(tt.factor)
			if got != tt.expect {
				t.Errorf("blendFactorToMTL(%v) = %v, want %v", tt.factor, got, tt.expect)
			}
		})
	}
}

// TestBlendOperationToMTL tests blend operation conversions.
func TestBlendOperationToMTL(t *testing.T) {
	tests := []struct {
		name   string
		op     types.BlendOperation
		expect MTLBlendOperation
	}{
		{"Add", types.BlendOperationAdd, MTLBlendOperationAdd},
		{"Subtract", types.BlendOperationSubtract, MTLBlendOperationSubtract},
		{"ReverseSubtract", types.BlendOperationReverseSubtract, MTLBlendOperationReverseSubtract},
		{"Min", types.BlendOperationMin, MTLBlendOperationMin},
		{"Max", types.BlendOperationMax, MTLBlendOperationMax},
		{"Unknown defaults to Add", types.BlendOperation(99), MTLBlendOperationAdd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blendOperationToMTL(tt.op)
			if got != tt.expect {
				t.Errorf("blendOperationToMTL(%v) = %v, want %v", tt.op, got, tt.expect)
			}
		})
	}
}

// TestLoadOpToMTL tests load operation conversions.
func TestLoadOpToMTL(t *testing.T) {
	tests := []struct {
		name   string
		op     types.LoadOp
		expect MTLLoadAction
	}{
		{"Clear", types.LoadOpClear, MTLLoadActionClear},
		{"Load", types.LoadOpLoad, MTLLoadActionLoad},
		{"Unknown defaults to DontCare", types.LoadOp(99), MTLLoadActionDontCare},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadOpToMTL(tt.op)
			if got != tt.expect {
				t.Errorf("loadOpToMTL(%v) = %v, want %v", tt.op, got, tt.expect)
			}
		})
	}
}

// TestStoreOpToMTL tests store operation conversions.
func TestStoreOpToMTL(t *testing.T) {
	tests := []struct {
		name   string
		op     types.StoreOp
		expect MTLStoreAction
	}{
		{"Store", types.StoreOpStore, MTLStoreActionStore},
		{"Discard", types.StoreOpDiscard, MTLStoreActionDontCare},
		{"Unknown defaults to Store", types.StoreOp(99), MTLStoreActionStore},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storeOpToMTL(tt.op)
			if got != tt.expect {
				t.Errorf("storeOpToMTL(%v) = %v, want %v", tt.op, got, tt.expect)
			}
		})
	}
}

// TestCullModeToMTL tests cull mode conversions.
func TestCullModeToMTL(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.CullMode
		expect MTLCullMode
	}{
		{"None", types.CullModeNone, MTLCullModeNone},
		{"Front", types.CullModeFront, MTLCullModeFront},
		{"Back", types.CullModeBack, MTLCullModeBack},
		{"Unknown defaults to None", types.CullMode(99), MTLCullModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cullModeToMTL(tt.mode)
			if got != tt.expect {
				t.Errorf("cullModeToMTL(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

// TestFrontFaceToMTL tests front face conversions.
func TestFrontFaceToMTL(t *testing.T) {
	tests := []struct {
		name   string
		face   types.FrontFace
		expect MTLWinding
	}{
		{"CCW", types.FrontFaceCCW, MTLWindingCounterClockwise},
		{"CW", types.FrontFaceCW, MTLWindingClockwise},
		{"Unknown defaults to CCW", types.FrontFace(99), MTLWindingCounterClockwise},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := frontFaceToMTL(tt.face)
			if got != tt.expect {
				t.Errorf("frontFaceToMTL(%v) = %v, want %v", tt.face, got, tt.expect)
			}
		})
	}
}

// TestIndexFormatToMTL tests index format conversions.
func TestIndexFormatToMTL(t *testing.T) {
	tests := []struct {
		name   string
		format types.IndexFormat
		expect MTLIndexType
	}{
		{"Uint16", types.IndexFormatUint16, MTLIndexTypeUInt16},
		{"Uint32", types.IndexFormatUint32, MTLIndexTypeUInt32},
		{"Unknown defaults to Uint32", types.IndexFormat(99), MTLIndexTypeUInt32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indexFormatToMTL(tt.format)
			if got != tt.expect {
				t.Errorf("indexFormatToMTL(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

// TestVertexFormatToMTL tests vertex format conversions.
func TestVertexFormatToMTL(t *testing.T) {
	tests := []struct {
		name   string
		format types.VertexFormat
		expect MTLVertexFormat
	}{
		// 8-bit formats
		{"Uint8x2", types.VertexFormatUint8x2, MTLVertexFormatUChar2},
		{"Uint8x4", types.VertexFormatUint8x4, MTLVertexFormatUChar4},
		{"Sint8x2", types.VertexFormatSint8x2, MTLVertexFormatChar2},
		{"Sint8x4", types.VertexFormatSint8x4, MTLVertexFormatChar4},
		{"Unorm8x2", types.VertexFormatUnorm8x2, MTLVertexFormatUChar2Normalized},
		{"Unorm8x4", types.VertexFormatUnorm8x4, MTLVertexFormatUChar4Normalized},
		{"Snorm8x2", types.VertexFormatSnorm8x2, MTLVertexFormatChar2Normalized},
		{"Snorm8x4", types.VertexFormatSnorm8x4, MTLVertexFormatChar4Normalized},

		// 16-bit formats
		{"Uint16x2", types.VertexFormatUint16x2, MTLVertexFormatUShort2},
		{"Uint16x4", types.VertexFormatUint16x4, MTLVertexFormatUShort4},
		{"Sint16x2", types.VertexFormatSint16x2, MTLVertexFormatShort2},
		{"Sint16x4", types.VertexFormatSint16x4, MTLVertexFormatShort4},
		{"Unorm16x2", types.VertexFormatUnorm16x2, MTLVertexFormatUShort2Normalized},
		{"Unorm16x4", types.VertexFormatUnorm16x4, MTLVertexFormatUShort4Normalized},
		{"Snorm16x2", types.VertexFormatSnorm16x2, MTLVertexFormatShort2Normalized},
		{"Snorm16x4", types.VertexFormatSnorm16x4, MTLVertexFormatShort4Normalized},
		{"Float16x2", types.VertexFormatFloat16x2, MTLVertexFormatHalf2},
		{"Float16x4", types.VertexFormatFloat16x4, MTLVertexFormatHalf4},

		// 32-bit formats
		{"Float32", types.VertexFormatFloat32, MTLVertexFormatFloat},
		{"Float32x2", types.VertexFormatFloat32x2, MTLVertexFormatFloat2},
		{"Float32x3", types.VertexFormatFloat32x3, MTLVertexFormatFloat3},
		{"Float32x4", types.VertexFormatFloat32x4, MTLVertexFormatFloat4},
		{"Uint32", types.VertexFormatUint32, MTLVertexFormatUInt},
		{"Uint32x2", types.VertexFormatUint32x2, MTLVertexFormatUInt2},
		{"Uint32x3", types.VertexFormatUint32x3, MTLVertexFormatUInt3},
		{"Uint32x4", types.VertexFormatUint32x4, MTLVertexFormatUInt4},
		{"Sint32", types.VertexFormatSint32, MTLVertexFormatInt},
		{"Sint32x2", types.VertexFormatSint32x2, MTLVertexFormatInt2},
		{"Sint32x3", types.VertexFormatSint32x3, MTLVertexFormatInt3},
		{"Sint32x4", types.VertexFormatSint32x4, MTLVertexFormatInt4},

		// Packed formats
		{"Unorm1010102", types.VertexFormatUnorm1010102, MTLVertexFormatUInt1010102Normalized},

		// Unknown format
		{"Unknown", types.VertexFormat(255), MTLVertexFormatInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vertexFormatToMTL(tt.format)
			if got != tt.expect {
				t.Errorf("vertexFormatToMTL(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

// TestVertexStepModeToMTL tests vertex step mode conversions.
func TestVertexStepModeToMTL(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.VertexStepMode
		expect MTLVertexStepFunction
	}{
		{"Vertex", types.VertexStepModeVertex, MTLVertexStepFunctionPerVertex},
		{"Instance", types.VertexStepModeInstance, MTLVertexStepFunctionPerInstance},
		{"Unknown defaults to PerVertex", types.VertexStepMode(99), MTLVertexStepFunctionPerVertex},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vertexStepModeToMTL(tt.mode)
			if got != tt.expect {
				t.Errorf("vertexStepModeToMTL(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}
