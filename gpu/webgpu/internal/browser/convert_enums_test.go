package browser

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// TestTextureFormatToJS verifies every texture format maps to the correct WebGPU string.
func TestTextureFormatToJS(t *testing.T) {
	tests := []struct {
		format types.TextureFormat
		want   string
	}{
		// 8-bit
		{types.TextureFormatR8Unorm, "r8unorm"},
		{types.TextureFormatR8Snorm, "r8snorm"},
		{types.TextureFormatR8Uint, "r8uint"},
		{types.TextureFormatR8Sint, "r8sint"},
		// 16-bit
		{types.TextureFormatR16Uint, "r16uint"},
		{types.TextureFormatR16Sint, "r16sint"},
		{types.TextureFormatR16Float, "r16float"},
		{types.TextureFormatRG8Unorm, "rg8unorm"},
		{types.TextureFormatRG8Snorm, "rg8snorm"},
		{types.TextureFormatRG8Uint, "rg8uint"},
		{types.TextureFormatRG8Sint, "rg8sint"},
		// 32-bit
		{types.TextureFormatR32Float, "r32float"},
		{types.TextureFormatR32Uint, "r32uint"},
		{types.TextureFormatR32Sint, "r32sint"},
		{types.TextureFormatRG16Uint, "rg16uint"},
		{types.TextureFormatRG16Sint, "rg16sint"},
		{types.TextureFormatRG16Float, "rg16float"},
		{types.TextureFormatRGBA8Unorm, "rgba8unorm"},
		{types.TextureFormatRGBA8UnormSrgb, "rgba8unorm-srgb"},
		{types.TextureFormatRGBA8Snorm, "rgba8snorm"},
		{types.TextureFormatRGBA8Uint, "rgba8uint"},
		{types.TextureFormatRGBA8Sint, "rgba8sint"},
		{types.TextureFormatBGRA8Unorm, "bgra8unorm"},
		{types.TextureFormatBGRA8UnormSrgb, "bgra8unorm-srgb"},
		// Packed 32-bit
		{types.TextureFormatRGB10A2Uint, "rgb10a2uint"},
		{types.TextureFormatRGB10A2Unorm, "rgb10a2unorm"},
		{types.TextureFormatRG11B10Ufloat, "rg11b10ufloat"},
		{types.TextureFormatRGB9E5Ufloat, "rgb9e5ufloat"},
		// 64-bit
		{types.TextureFormatRG32Float, "rg32float"},
		{types.TextureFormatRG32Uint, "rg32uint"},
		{types.TextureFormatRG32Sint, "rg32sint"},
		{types.TextureFormatRGBA16Uint, "rgba16uint"},
		{types.TextureFormatRGBA16Sint, "rgba16sint"},
		{types.TextureFormatRGBA16Float, "rgba16float"},
		// 128-bit
		{types.TextureFormatRGBA32Float, "rgba32float"},
		{types.TextureFormatRGBA32Uint, "rgba32uint"},
		{types.TextureFormatRGBA32Sint, "rgba32sint"},
		// Depth/stencil
		{types.TextureFormatStencil8, "stencil8"},
		{types.TextureFormatDepth16Unorm, "depth16unorm"},
		{types.TextureFormatDepth24Plus, "depth24plus"},
		{types.TextureFormatDepth24PlusStencil8, "depth24plus-stencil8"},
		{types.TextureFormatDepth32Float, "depth32float"},
		{types.TextureFormatDepth32FloatStencil8, "depth32float-stencil8"},
		// BC compressed
		{types.TextureFormatBC1RGBAUnorm, "bc1-rgba-unorm"},
		{types.TextureFormatBC1RGBAUnormSrgb, "bc1-rgba-unorm-srgb"},
		{types.TextureFormatBC2RGBAUnorm, "bc2-rgba-unorm"},
		{types.TextureFormatBC2RGBAUnormSrgb, "bc2-rgba-unorm-srgb"},
		{types.TextureFormatBC3RGBAUnorm, "bc3-rgba-unorm"},
		{types.TextureFormatBC3RGBAUnormSrgb, "bc3-rgba-unorm-srgb"},
		{types.TextureFormatBC4RUnorm, "bc4-r-unorm"},
		{types.TextureFormatBC4RSnorm, "bc4-r-snorm"},
		{types.TextureFormatBC5RGUnorm, "bc5-rg-unorm"},
		{types.TextureFormatBC5RGSnorm, "bc5-rg-snorm"},
		{types.TextureFormatBC6HRGBUfloat, "bc6h-rgb-ufloat"},
		{types.TextureFormatBC6HRGBFloat, "bc6h-rgb-float"},
		{types.TextureFormatBC7RGBAUnorm, "bc7-rgba-unorm"},
		{types.TextureFormatBC7RGBAUnormSrgb, "bc7-rgba-unorm-srgb"},
		// ETC2 compressed
		{types.TextureFormatETC2RGB8Unorm, "etc2-rgb8unorm"},
		{types.TextureFormatETC2RGB8UnormSrgb, "etc2-rgb8unorm-srgb"},
		{types.TextureFormatETC2RGB8A1Unorm, "etc2-rgb8a1unorm"},
		{types.TextureFormatETC2RGB8A1UnormSrgb, "etc2-rgb8a1unorm-srgb"},
		{types.TextureFormatETC2RGBA8Unorm, "etc2-rgba8unorm"},
		{types.TextureFormatETC2RGBA8UnormSrgb, "etc2-rgba8unorm-srgb"},
		{types.TextureFormatEACR11Unorm, "eac-r11unorm"},
		{types.TextureFormatEACR11Snorm, "eac-r11snorm"},
		{types.TextureFormatEACRG11Unorm, "eac-rg11unorm"},
		{types.TextureFormatEACRG11Snorm, "eac-rg11snorm"},
		// ASTC compressed (spot check)
		{types.TextureFormatASTC4x4Unorm, "astc-4x4-unorm"},
		{types.TextureFormatASTC4x4UnormSrgb, "astc-4x4-unorm-srgb"},
		{types.TextureFormatASTC12x12Unorm, "astc-12x12-unorm"},
		{types.TextureFormatASTC12x12UnormSrgb, "astc-12x12-unorm-srgb"},
		// Undefined returns empty
		{types.TextureFormatUndefined, ""},
	}
	for _, tc := range tests {
		got := TextureFormatToJS(tc.format)
		if got != tc.want {
			t.Errorf("TextureFormatToJS(%v) = %q, want %q", tc.format, got, tc.want)
		}
	}
}

// TestTextureDimensionToJS verifies all texture dimension mappings.
func TestTextureDimensionToJS(t *testing.T) {
	tests := []struct {
		dim  types.TextureDimension
		want string
	}{
		{types.TextureDimension1D, "1d"},
		{types.TextureDimension2D, "2d"},
		{types.TextureDimension3D, "3d"},
		{types.TextureDimensionUndefined, ""},
	}
	for _, tc := range tests {
		got := TextureDimensionToJS(tc.dim)
		if got != tc.want {
			t.Errorf("TextureDimensionToJS(%v) = %q, want %q", tc.dim, got, tc.want)
		}
	}
}

// TestTextureViewDimensionToJS verifies all view dimension mappings.
func TestTextureViewDimensionToJS(t *testing.T) {
	tests := []struct {
		dim  types.TextureViewDimension
		want string
	}{
		{types.TextureViewDimension1D, "1d"},
		{types.TextureViewDimension2D, "2d"},
		{types.TextureViewDimension2DArray, "2d-array"},
		{types.TextureViewDimensionCube, "cube"},
		{types.TextureViewDimensionCubeArray, "cube-array"},
		{types.TextureViewDimension3D, "3d"},
		{types.TextureViewDimensionUndefined, ""},
	}
	for _, tc := range tests {
		got := TextureViewDimensionToJS(tc.dim)
		if got != tc.want {
			t.Errorf("TextureViewDimensionToJS(%v) = %q, want %q", tc.dim, got, tc.want)
		}
	}
}

// TestTextureAspectToJS verifies all texture aspect mappings.
func TestTextureAspectToJS(t *testing.T) {
	tests := []struct {
		aspect types.TextureAspect
		want   string
	}{
		{types.TextureAspectAll, "all"},
		{types.TextureAspectStencilOnly, "stencil-only"},
		{types.TextureAspectDepthOnly, "depth-only"},
		{types.TextureAspectUndefined, ""},
	}
	for _, tc := range tests {
		got := TextureAspectToJS(tc.aspect)
		if got != tc.want {
			t.Errorf("TextureAspectToJS(%v) = %q, want %q", tc.aspect, got, tc.want)
		}
	}
}

// TestAddressModeToJS verifies all address mode mappings.
func TestAddressModeToJS(t *testing.T) {
	tests := []struct {
		mode types.AddressMode
		want string
	}{
		{types.AddressModeClampToEdge, "clamp-to-edge"},
		{types.AddressModeRepeat, "repeat"},
		{types.AddressModeMirrorRepeat, "mirror-repeat"},
		{types.AddressModeUndefined, "clamp-to-edge"},
	}
	for _, tc := range tests {
		got := AddressModeToJS(tc.mode)
		if got != tc.want {
			t.Errorf("AddressModeToJS(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestFilterModeToJS verifies all filter mode mappings.
func TestFilterModeToJS(t *testing.T) {
	tests := []struct {
		mode types.FilterMode
		want string
	}{
		{types.FilterModeNearest, "nearest"},
		{types.FilterModeLinear, "linear"},
		{types.FilterModeUndefined, "nearest"},
	}
	for _, tc := range tests {
		got := FilterModeToJS(tc.mode)
		if got != tc.want {
			t.Errorf("FilterModeToJS(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestCompareFunctionToJS verifies all compare function mappings.
func TestCompareFunctionToJS(t *testing.T) {
	tests := []struct {
		fn   types.CompareFunction
		want string
	}{
		{types.CompareFunctionNever, "never"},
		{types.CompareFunctionLess, "less"},
		{types.CompareFunctionEqual, "equal"},
		{types.CompareFunctionLessEqual, "less-equal"},
		{types.CompareFunctionGreater, "greater"},
		{types.CompareFunctionNotEqual, "not-equal"},
		{types.CompareFunctionGreaterEqual, "greater-equal"},
		{types.CompareFunctionAlways, "always"},
		{types.CompareFunctionUndefined, ""},
	}
	for _, tc := range tests {
		got := CompareFunctionToJS(tc.fn)
		if got != tc.want {
			t.Errorf("CompareFunctionToJS(%v) = %q, want %q", tc.fn, got, tc.want)
		}
	}
}

// TestPrimitiveTopologyToJS verifies all primitive topology mappings.
func TestPrimitiveTopologyToJS(t *testing.T) {
	tests := []struct {
		topo types.PrimitiveTopology
		want string
	}{
		{types.PrimitiveTopologyPointList, "point-list"},
		{types.PrimitiveTopologyLineList, "line-list"},
		{types.PrimitiveTopologyLineStrip, "line-strip"},
		{types.PrimitiveTopologyTriangleList, "triangle-list"},
		{types.PrimitiveTopologyTriangleStrip, "triangle-strip"},
	}
	for _, tc := range tests {
		got := PrimitiveTopologyToJS(tc.topo)
		if got != tc.want {
			t.Errorf("PrimitiveTopologyToJS(%v) = %q, want %q", tc.topo, got, tc.want)
		}
	}
}

// TestFrontFaceToJS verifies all front face mappings.
func TestFrontFaceToJS(t *testing.T) {
	tests := []struct {
		face types.FrontFace
		want string
	}{
		{types.FrontFaceCCW, "ccw"},
		{types.FrontFaceCW, "cw"},
	}
	for _, tc := range tests {
		got := FrontFaceToJS(tc.face)
		if got != tc.want {
			t.Errorf("FrontFaceToJS(%v) = %q, want %q", tc.face, got, tc.want)
		}
	}
}

// TestCullModeToJS verifies all cull mode mappings.
func TestCullModeToJS(t *testing.T) {
	tests := []struct {
		mode types.CullMode
		want string
	}{
		{types.CullModeNone, "none"},
		{types.CullModeFront, "front"},
		{types.CullModeBack, "back"},
	}
	for _, tc := range tests {
		got := CullModeToJS(tc.mode)
		if got != tc.want {
			t.Errorf("CullModeToJS(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestIndexFormatToJS verifies all index format mappings.
func TestIndexFormatToJS(t *testing.T) {
	tests := []struct {
		fmt  types.IndexFormat
		want string
	}{
		{types.IndexFormatUint16, "uint16"},
		{types.IndexFormatUint32, "uint32"},
		{types.IndexFormatUndefined, ""},
	}
	for _, tc := range tests {
		got := IndexFormatToJS(tc.fmt)
		if got != tc.want {
			t.Errorf("IndexFormatToJS(%v) = %q, want %q", tc.fmt, got, tc.want)
		}
	}
}

// TestVertexFormatToJS verifies all vertex format mappings.
func TestVertexFormatToJS(t *testing.T) {
	tests := []struct {
		fmt  types.VertexFormat
		want string
	}{
		{types.VertexFormatUint8x2, "uint8x2"},
		{types.VertexFormatUint8x4, "uint8x4"},
		{types.VertexFormatSint8x2, "sint8x2"},
		{types.VertexFormatSint8x4, "sint8x4"},
		{types.VertexFormatUnorm8x2, "unorm8x2"},
		{types.VertexFormatUnorm8x4, "unorm8x4"},
		{types.VertexFormatSnorm8x2, "snorm8x2"},
		{types.VertexFormatSnorm8x4, "snorm8x4"},
		{types.VertexFormatUint16x2, "uint16x2"},
		{types.VertexFormatUint16x4, "uint16x4"},
		{types.VertexFormatSint16x2, "sint16x2"},
		{types.VertexFormatSint16x4, "sint16x4"},
		{types.VertexFormatUnorm16x2, "unorm16x2"},
		{types.VertexFormatUnorm16x4, "unorm16x4"},
		{types.VertexFormatSnorm16x2, "snorm16x2"},
		{types.VertexFormatSnorm16x4, "snorm16x4"},
		{types.VertexFormatFloat16x2, "float16x2"},
		{types.VertexFormatFloat16x4, "float16x4"},
		{types.VertexFormatFloat32, "float32"},
		{types.VertexFormatFloat32x2, "float32x2"},
		{types.VertexFormatFloat32x3, "float32x3"},
		{types.VertexFormatFloat32x4, "float32x4"},
		{types.VertexFormatUint32, "uint32"},
		{types.VertexFormatUint32x2, "uint32x2"},
		{types.VertexFormatUint32x3, "uint32x3"},
		{types.VertexFormatUint32x4, "uint32x4"},
		{types.VertexFormatSint32, "sint32"},
		{types.VertexFormatSint32x2, "sint32x2"},
		{types.VertexFormatSint32x3, "sint32x3"},
		{types.VertexFormatSint32x4, "sint32x4"},
		{types.VertexFormatUnorm1010102, "unorm10-10-10-2"},
		// Undefined returns empty
		{types.VertexFormatUndefined, ""},
	}
	for _, tc := range tests {
		got := VertexFormatToJS(tc.fmt)
		if got != tc.want {
			t.Errorf("VertexFormatToJS(%v) = %q, want %q", tc.fmt, got, tc.want)
		}
	}
}

// TestVertexStepModeToJS verifies all vertex step mode mappings.
func TestVertexStepModeToJS(t *testing.T) {
	tests := []struct {
		mode types.VertexStepMode
		want string
	}{
		{types.VertexStepModeVertex, "vertex"},
		{types.VertexStepModeInstance, "instance"},
		{types.VertexStepModeUndefined, "vertex"},
	}
	for _, tc := range tests {
		got := VertexStepModeToJS(tc.mode)
		if got != tc.want {
			t.Errorf("VertexStepModeToJS(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestBlendFactorToJS verifies all blend factor mappings.
func TestBlendFactorToJS(t *testing.T) {
	tests := []struct {
		factor types.BlendFactor
		want   string
	}{
		{types.BlendFactorZero, "zero"},
		{types.BlendFactorOne, "one"},
		{types.BlendFactorSrc, "src"},
		{types.BlendFactorOneMinusSrc, "one-minus-src"},
		{types.BlendFactorSrcAlpha, "src-alpha"},
		{types.BlendFactorOneMinusSrcAlpha, "one-minus-src-alpha"},
		{types.BlendFactorDst, "dst"},
		{types.BlendFactorOneMinusDst, "one-minus-dst"},
		{types.BlendFactorDstAlpha, "dst-alpha"},
		{types.BlendFactorOneMinusDstAlpha, "one-minus-dst-alpha"},
		{types.BlendFactorSrcAlphaSaturated, "src-alpha-saturated"},
		{types.BlendFactorConstant, "constant"},
		{types.BlendFactorOneMinusConstant, "one-minus-constant"},
	}
	for _, tc := range tests {
		got := BlendFactorToJS(tc.factor)
		if got != tc.want {
			t.Errorf("BlendFactorToJS(%v) = %q, want %q", tc.factor, got, tc.want)
		}
	}
}

// TestBlendOperationToJS verifies all blend operation mappings.
func TestBlendOperationToJS(t *testing.T) {
	tests := []struct {
		op   types.BlendOperation
		want string
	}{
		{types.BlendOperationAdd, "add"},
		{types.BlendOperationSubtract, "subtract"},
		{types.BlendOperationReverseSubtract, "reverse-subtract"},
		{types.BlendOperationMin, "min"},
		{types.BlendOperationMax, "max"},
	}
	for _, tc := range tests {
		got := BlendOperationToJS(tc.op)
		if got != tc.want {
			t.Errorf("BlendOperationToJS(%v) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

// TestStencilOperationToJS verifies all stencil operation mappings.
func TestStencilOperationToJS(t *testing.T) {
	tests := []struct {
		op   types.StencilOperation
		want string
	}{
		{types.StencilOperationKeep, "keep"},
		{types.StencilOperationZero, "zero"},
		{types.StencilOperationReplace, "replace"},
		{types.StencilOperationInvert, "invert"},
		{types.StencilOperationIncrementClamp, "increment-clamp"},
		{types.StencilOperationDecrementClamp, "decrement-clamp"},
		{types.StencilOperationIncrementWrap, "increment-wrap"},
		{types.StencilOperationDecrementWrap, "decrement-wrap"},
	}
	for _, tc := range tests {
		got := StencilOperationToJS(tc.op)
		if got != tc.want {
			t.Errorf("StencilOperationToJS(%v) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

// TestBufferBindingTypeToJS verifies all buffer binding type mappings.
func TestBufferBindingTypeToJS(t *testing.T) {
	tests := []struct {
		typ  types.BufferBindingType
		want string
	}{
		{types.BufferBindingTypeUniform, "uniform"},
		{types.BufferBindingTypeStorage, "storage"},
		{types.BufferBindingTypeReadOnlyStorage, "read-only-storage"},
		{types.BufferBindingTypeUndefined, "uniform"},
	}
	for _, tc := range tests {
		got := BufferBindingTypeToJS(tc.typ)
		if got != tc.want {
			t.Errorf("BufferBindingTypeToJS(%v) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

// TestSamplerBindingTypeToJS verifies all sampler binding type mappings.
func TestSamplerBindingTypeToJS(t *testing.T) {
	tests := []struct {
		typ  types.SamplerBindingType
		want string
	}{
		{types.SamplerBindingTypeFiltering, "filtering"},
		{types.SamplerBindingTypeNonFiltering, "non-filtering"},
		{types.SamplerBindingTypeComparison, "comparison"},
		{types.SamplerBindingTypeUndefined, "filtering"},
	}
	for _, tc := range tests {
		got := SamplerBindingTypeToJS(tc.typ)
		if got != tc.want {
			t.Errorf("SamplerBindingTypeToJS(%v) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

// TestTextureSampleTypeToJS verifies all texture sample type mappings.
func TestTextureSampleTypeToJS(t *testing.T) {
	tests := []struct {
		typ  types.TextureSampleType
		want string
	}{
		{types.TextureSampleTypeFloat, "float"},
		{types.TextureSampleTypeUnfilterableFloat, "unfilterable-float"},
		{types.TextureSampleTypeDepth, "depth"},
		{types.TextureSampleTypeSint, "sint"},
		{types.TextureSampleTypeUint, "uint"},
		{types.TextureSampleTypeUndefined, "float"},
	}
	for _, tc := range tests {
		got := TextureSampleTypeToJS(tc.typ)
		if got != tc.want {
			t.Errorf("TextureSampleTypeToJS(%v) = %q, want %q", tc.typ, got, tc.want)
		}
	}
}

// TestStorageTextureAccessToJS verifies all storage texture access mappings.
func TestStorageTextureAccessToJS(t *testing.T) {
	tests := []struct {
		access types.StorageTextureAccess
		want   string
	}{
		{types.StorageTextureAccessWriteOnly, "write-only"},
		{types.StorageTextureAccessReadOnly, "read-only"},
		{types.StorageTextureAccessReadWrite, "read-write"},
		{types.StorageTextureAccessUndefined, "write-only"},
	}
	for _, tc := range tests {
		got := StorageTextureAccessToJS(tc.access)
		if got != tc.want {
			t.Errorf("StorageTextureAccessToJS(%v) = %q, want %q", tc.access, got, tc.want)
		}
	}
}

// TestTextureFormatMapCompleteness verifies that every non-Undefined format in
// gputypes has a mapping. Missing entries would cause silent empty strings at runtime.
func TestTextureFormatMapCompleteness(t *testing.T) {
	// All non-Undefined, non-Unorm16/Snorm16 formats from gputypes
	allFormats := []types.TextureFormat{
		types.TextureFormatR8Unorm, types.TextureFormatR8Snorm,
		types.TextureFormatR8Uint, types.TextureFormatR8Sint,
		types.TextureFormatR16Uint, types.TextureFormatR16Sint, types.TextureFormatR16Float,
		types.TextureFormatRG8Unorm, types.TextureFormatRG8Snorm,
		types.TextureFormatRG8Uint, types.TextureFormatRG8Sint,
		types.TextureFormatR32Float, types.TextureFormatR32Uint, types.TextureFormatR32Sint,
		types.TextureFormatRG16Uint, types.TextureFormatRG16Sint, types.TextureFormatRG16Float,
		types.TextureFormatRGBA8Unorm, types.TextureFormatRGBA8UnormSrgb,
		types.TextureFormatRGBA8Snorm, types.TextureFormatRGBA8Uint, types.TextureFormatRGBA8Sint,
		types.TextureFormatBGRA8Unorm, types.TextureFormatBGRA8UnormSrgb,
		types.TextureFormatRGB10A2Uint, types.TextureFormatRGB10A2Unorm,
		types.TextureFormatRG11B10Ufloat, types.TextureFormatRGB9E5Ufloat,
		types.TextureFormatRG32Float, types.TextureFormatRG32Uint, types.TextureFormatRG32Sint,
		types.TextureFormatRGBA16Uint, types.TextureFormatRGBA16Sint, types.TextureFormatRGBA16Float,
		types.TextureFormatRGBA32Float, types.TextureFormatRGBA32Uint, types.TextureFormatRGBA32Sint,
		types.TextureFormatStencil8, types.TextureFormatDepth16Unorm,
		types.TextureFormatDepth24Plus, types.TextureFormatDepth24PlusStencil8,
		types.TextureFormatDepth32Float, types.TextureFormatDepth32FloatStencil8,
		types.TextureFormatBC1RGBAUnorm, types.TextureFormatBC1RGBAUnormSrgb,
		types.TextureFormatBC2RGBAUnorm, types.TextureFormatBC2RGBAUnormSrgb,
		types.TextureFormatBC3RGBAUnorm, types.TextureFormatBC3RGBAUnormSrgb,
		types.TextureFormatBC4RUnorm, types.TextureFormatBC4RSnorm,
		types.TextureFormatBC5RGUnorm, types.TextureFormatBC5RGSnorm,
		types.TextureFormatBC6HRGBUfloat, types.TextureFormatBC6HRGBFloat,
		types.TextureFormatBC7RGBAUnorm, types.TextureFormatBC7RGBAUnormSrgb,
		types.TextureFormatETC2RGB8Unorm, types.TextureFormatETC2RGB8UnormSrgb,
		types.TextureFormatETC2RGB8A1Unorm, types.TextureFormatETC2RGB8A1UnormSrgb,
		types.TextureFormatETC2RGBA8Unorm, types.TextureFormatETC2RGBA8UnormSrgb,
		types.TextureFormatEACR11Unorm, types.TextureFormatEACR11Snorm,
		types.TextureFormatEACRG11Unorm, types.TextureFormatEACRG11Snorm,
		types.TextureFormatASTC4x4Unorm, types.TextureFormatASTC4x4UnormSrgb,
		types.TextureFormatASTC5x4Unorm, types.TextureFormatASTC5x4UnormSrgb,
		types.TextureFormatASTC5x5Unorm, types.TextureFormatASTC5x5UnormSrgb,
		types.TextureFormatASTC6x5Unorm, types.TextureFormatASTC6x5UnormSrgb,
		types.TextureFormatASTC6x6Unorm, types.TextureFormatASTC6x6UnormSrgb,
		types.TextureFormatASTC8x5Unorm, types.TextureFormatASTC8x5UnormSrgb,
		types.TextureFormatASTC8x6Unorm, types.TextureFormatASTC8x6UnormSrgb,
		types.TextureFormatASTC8x8Unorm, types.TextureFormatASTC8x8UnormSrgb,
		types.TextureFormatASTC10x5Unorm, types.TextureFormatASTC10x5UnormSrgb,
		types.TextureFormatASTC10x6Unorm, types.TextureFormatASTC10x6UnormSrgb,
		types.TextureFormatASTC10x8Unorm, types.TextureFormatASTC10x8UnormSrgb,
		types.TextureFormatASTC10x10Unorm, types.TextureFormatASTC10x10UnormSrgb,
		types.TextureFormatASTC12x10Unorm, types.TextureFormatASTC12x10UnormSrgb,
		types.TextureFormatASTC12x12Unorm, types.TextureFormatASTC12x12UnormSrgb,
	}
	for _, f := range allFormats {
		s := TextureFormatToJS(f)
		if s == "" {
			t.Errorf("TextureFormatToJS(%v) returned empty string — missing mapping", f)
		}
	}
}

// TestVertexFormatMapCompleteness verifies that every non-Undefined vertex format
// has a mapping.
func TestVertexFormatMapCompleteness(t *testing.T) {
	allFormats := []types.VertexFormat{
		types.VertexFormatUint8x2, types.VertexFormatUint8x4,
		types.VertexFormatSint8x2, types.VertexFormatSint8x4,
		types.VertexFormatUnorm8x2, types.VertexFormatUnorm8x4,
		types.VertexFormatSnorm8x2, types.VertexFormatSnorm8x4,
		types.VertexFormatUint16x2, types.VertexFormatUint16x4,
		types.VertexFormatSint16x2, types.VertexFormatSint16x4,
		types.VertexFormatUnorm16x2, types.VertexFormatUnorm16x4,
		types.VertexFormatSnorm16x2, types.VertexFormatSnorm16x4,
		types.VertexFormatFloat16x2, types.VertexFormatFloat16x4,
		types.VertexFormatFloat32, types.VertexFormatFloat32x2,
		types.VertexFormatFloat32x3, types.VertexFormatFloat32x4,
		types.VertexFormatUint32, types.VertexFormatUint32x2,
		types.VertexFormatUint32x3, types.VertexFormatUint32x4,
		types.VertexFormatSint32, types.VertexFormatSint32x2,
		types.VertexFormatSint32x3, types.VertexFormatSint32x4,
		types.VertexFormatUnorm1010102,
	}
	for _, f := range allFormats {
		s := VertexFormatToJS(f)
		if s == "" {
			t.Errorf("VertexFormatToJS(%v) returned empty string — missing mapping", f)
		}
	}
}
