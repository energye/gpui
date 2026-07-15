package rwgpu

// s1_skia_subset_enum_test.go — S1 header-lock tests for the Skia 2D WebGPU subset.
//
// Authority: lib/webgpu.h (wgpu-native v29)
// Plan: docs/MAINLINE_PLAN.md S1 + docs/RWGPU_SKIA_SUBSET_CHECKLIST.md
//
// Two classes of enums:
//  1. Identity: gputypes values match header; wire uses uint32(enum) directly.
//  2. Converted: toWGPU* must produce the header value (gputypes numbering differs).
//
// These tests require no GPU; they lock ABI correctness against silent drift.

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// header values are WGPU* constants from lib/webgpu.h (Force32 omitted).

func TestS1IdentityEnumsMatchWebGPUHeader(t *testing.T) {
	t.Run("LoadOp", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.LoadOpUndefined), 0x00000000},
			{"Load", uint32(types.LoadOpLoad), 0x00000001},
			{"Clear", uint32(types.LoadOpClear), 0x00000002},
		})
	})

	t.Run("StoreOp", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.StoreOpUndefined), 0x00000000},
			{"Store", uint32(types.StoreOpStore), 0x00000001},
			{"Discard", uint32(types.StoreOpDiscard), 0x00000002},
		})
	})

	t.Run("BlendFactor", func(t *testing.T) {
		// gputypes covers 0x00–0x0D; Src1* (0x0E–0x11) unused via types API.
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.BlendFactorUndefined), 0x00000000},
			{"Zero", uint32(types.BlendFactorZero), 0x00000001},
			{"One", uint32(types.BlendFactorOne), 0x00000002},
			{"Src", uint32(types.BlendFactorSrc), 0x00000003},
			{"OneMinusSrc", uint32(types.BlendFactorOneMinusSrc), 0x00000004},
			{"SrcAlpha", uint32(types.BlendFactorSrcAlpha), 0x00000005},
			{"OneMinusSrcAlpha", uint32(types.BlendFactorOneMinusSrcAlpha), 0x00000006},
			{"Dst", uint32(types.BlendFactorDst), 0x00000007},
			{"OneMinusDst", uint32(types.BlendFactorOneMinusDst), 0x00000008},
			{"DstAlpha", uint32(types.BlendFactorDstAlpha), 0x00000009},
			{"OneMinusDstAlpha", uint32(types.BlendFactorOneMinusDstAlpha), 0x0000000A},
			{"SrcAlphaSaturated", uint32(types.BlendFactorSrcAlphaSaturated), 0x0000000B},
			{"Constant", uint32(types.BlendFactorConstant), 0x0000000C},
			{"OneMinusConstant", uint32(types.BlendFactorOneMinusConstant), 0x0000000D},
		})
	})

	t.Run("BlendOperation", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.BlendOperationUndefined), 0x00000000},
			{"Add", uint32(types.BlendOperationAdd), 0x00000001},
			{"Subtract", uint32(types.BlendOperationSubtract), 0x00000002},
			{"ReverseSubtract", uint32(types.BlendOperationReverseSubtract), 0x00000003},
			{"Min", uint32(types.BlendOperationMin), 0x00000004},
			{"Max", uint32(types.BlendOperationMax), 0x00000005},
		})
	})

	t.Run("CompareFunction", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.CompareFunctionUndefined), 0x00000000},
			{"Never", uint32(types.CompareFunctionNever), 0x00000001},
			{"Less", uint32(types.CompareFunctionLess), 0x00000002},
			{"Equal", uint32(types.CompareFunctionEqual), 0x00000003},
			{"LessEqual", uint32(types.CompareFunctionLessEqual), 0x00000004},
			{"Greater", uint32(types.CompareFunctionGreater), 0x00000005},
			{"NotEqual", uint32(types.CompareFunctionNotEqual), 0x00000006},
			{"GreaterEqual", uint32(types.CompareFunctionGreaterEqual), 0x00000007},
			{"Always", uint32(types.CompareFunctionAlways), 0x00000008},
		})
	})

	t.Run("StencilOperation", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.StencilOperationUndefined), 0x00000000},
			{"Keep", uint32(types.StencilOperationKeep), 0x00000001},
			{"Zero", uint32(types.StencilOperationZero), 0x00000002},
			{"Replace", uint32(types.StencilOperationReplace), 0x00000003},
			{"Invert", uint32(types.StencilOperationInvert), 0x00000004},
			{"IncrementClamp", uint32(types.StencilOperationIncrementClamp), 0x00000005},
			{"DecrementClamp", uint32(types.StencilOperationDecrementClamp), 0x00000006},
			{"IncrementWrap", uint32(types.StencilOperationIncrementWrap), 0x00000007},
			{"DecrementWrap", uint32(types.StencilOperationDecrementWrap), 0x00000008},
		})
	})

	t.Run("FilterMode", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.FilterModeUndefined), 0x00000000},
			{"Nearest", uint32(types.FilterModeNearest), 0x00000001},
			{"Linear", uint32(types.FilterModeLinear), 0x00000002},
		})
	})

	t.Run("MipmapFilterMode", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.MipmapFilterModeUndefined), 0x00000000},
			{"Nearest", uint32(types.MipmapFilterModeNearest), 0x00000001},
			{"Linear", uint32(types.MipmapFilterModeLinear), 0x00000002},
		})
	})

	t.Run("AddressMode", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.AddressModeUndefined), 0x00000000},
			{"ClampToEdge", uint32(types.AddressModeClampToEdge), 0x00000001},
			{"Repeat", uint32(types.AddressModeRepeat), 0x00000002},
			{"MirrorRepeat", uint32(types.AddressModeMirrorRepeat), 0x00000003},
		})
	})

	t.Run("IndexFormat", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.IndexFormatUndefined), 0x00000000},
			{"Uint16", uint32(types.IndexFormatUint16), 0x00000001},
			{"Uint32", uint32(types.IndexFormatUint32), 0x00000002},
		})
	})

	t.Run("TextureDimension", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.TextureDimensionUndefined), 0x00000000},
			{"1D", uint32(types.TextureDimension1D), 0x00000001},
			{"2D", uint32(types.TextureDimension2D), 0x00000002},
			{"3D", uint32(types.TextureDimension3D), 0x00000003},
		})
	})

	t.Run("TextureViewDimension", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.TextureViewDimensionUndefined), 0x00000000},
			{"1D", uint32(types.TextureViewDimension1D), 0x00000001},
			{"2D", uint32(types.TextureViewDimension2D), 0x00000002},
			{"2DArray", uint32(types.TextureViewDimension2DArray), 0x00000003},
			{"Cube", uint32(types.TextureViewDimensionCube), 0x00000004},
			{"CubeArray", uint32(types.TextureViewDimensionCubeArray), 0x00000005},
			{"3D", uint32(types.TextureViewDimension3D), 0x00000006},
		})
	})

	t.Run("TextureAspect", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", uint32(types.TextureAspectUndefined), 0x00000000},
			{"All", uint32(types.TextureAspectAll), 0x00000001},
			{"StencilOnly", uint32(types.TextureAspectStencilOnly), 0x00000002},
			{"DepthOnly", uint32(types.TextureAspectDepthOnly), 0x00000003},
		})
	})

	t.Run("ColorWriteMask", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"None", uint32(types.ColorWriteMaskNone), 0x00000000},
			{"Red", uint32(types.ColorWriteMaskRed), 0x00000001},
			{"Green", uint32(types.ColorWriteMaskGreen), 0x00000002},
			{"Blue", uint32(types.ColorWriteMaskBlue), 0x00000004},
			{"Alpha", uint32(types.ColorWriteMaskAlpha), 0x00000008},
			{"All", uint32(types.ColorWriteMaskAll), 0x0000000F},
		})
	})
}

// TestS1TextureFormatSkiaSubset locks TextureFormat values used by Skia 2D path
// (RT, mask, depth-stencil, atlas, optional float). Identity cast to wire.
func TestS1TextureFormatSkiaSubset(t *testing.T) {
	runEnumTests(t, []struct {
		name     string
		got      uint32
		expected uint32
	}{
		{"Undefined", uint32(types.TextureFormatUndefined), 0x00000000},
		{"R8Unorm", uint32(types.TextureFormatR8Unorm), 0x00000001},
		{"R8Snorm", uint32(types.TextureFormatR8Snorm), 0x00000002},
		{"R8Uint", uint32(types.TextureFormatR8Uint), 0x00000003},
		{"R8Sint", uint32(types.TextureFormatR8Sint), 0x00000004},
		{"R16Uint", uint32(types.TextureFormatR16Uint), 0x00000007},
		{"R16Sint", uint32(types.TextureFormatR16Sint), 0x00000008},
		{"R16Float", uint32(types.TextureFormatR16Float), 0x00000009},
		{"RG8Unorm", uint32(types.TextureFormatRG8Unorm), 0x0000000A},
		{"R32Float", uint32(types.TextureFormatR32Float), 0x0000000E},
		{"RG16Float", uint32(types.TextureFormatRG16Float), 0x00000015},
		{"RGBA8Unorm", uint32(types.TextureFormatRGBA8Unorm), 0x00000016},
		{"RGBA8UnormSrgb", uint32(types.TextureFormatRGBA8UnormSrgb), 0x00000017},
		{"RGBA8Snorm", uint32(types.TextureFormatRGBA8Snorm), 0x00000018},
		{"RGBA8Uint", uint32(types.TextureFormatRGBA8Uint), 0x00000019},
		{"RGBA8Sint", uint32(types.TextureFormatRGBA8Sint), 0x0000001A},
		{"BGRA8Unorm", uint32(types.TextureFormatBGRA8Unorm), 0x0000001B},
		{"BGRA8UnormSrgb", uint32(types.TextureFormatBGRA8UnormSrgb), 0x0000001C},
		{"RGB10A2Unorm", uint32(types.TextureFormatRGB10A2Unorm), 0x0000001E},
		{"RG11B10Ufloat", uint32(types.TextureFormatRG11B10Ufloat), 0x0000001F},
		{"RGBA16Float", uint32(types.TextureFormatRGBA16Float), 0x00000028},
		{"RGBA32Float", uint32(types.TextureFormatRGBA32Float), 0x00000029},
		{"Stencil8", uint32(types.TextureFormatStencil8), 0x0000002C},
		{"Depth16Unorm", uint32(types.TextureFormatDepth16Unorm), 0x0000002D},
		{"Depth24Plus", uint32(types.TextureFormatDepth24Plus), 0x0000002E},
		{"Depth24PlusStencil8", uint32(types.TextureFormatDepth24PlusStencil8), 0x0000002F},
		{"Depth32Float", uint32(types.TextureFormatDepth32Float), 0x00000030},
		{"Depth32FloatStencil8", uint32(types.TextureFormatDepth32FloatStencil8), 0x00000031},
	})
}

// TestS1ConvertedEnumsMatchWebGPUHeader locks toWGPU* against header values.
// gputypes numbering intentionally differs for these enums (JS-style zeros).
func TestS1ConvertedEnumsMatchWebGPUHeader(t *testing.T) {
	t.Run("PrimitiveTopology", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			// gputypes TriangleList=0 (spec default zero) → header TriangleList=4
			{"TriangleList", toWGPUPrimitiveTopology(types.PrimitiveTopologyTriangleList), 0x00000004},
			{"PointList", toWGPUPrimitiveTopology(types.PrimitiveTopologyPointList), 0x00000001},
			{"LineList", toWGPUPrimitiveTopology(types.PrimitiveTopologyLineList), 0x00000002},
			{"LineStrip", toWGPUPrimitiveTopology(types.PrimitiveTopologyLineStrip), 0x00000003},
			{"TriangleStrip", toWGPUPrimitiveTopology(types.PrimitiveTopologyTriangleStrip), 0x00000005},
			{"Unknown", toWGPUPrimitiveTopology(types.PrimitiveTopology(0xFFFFFFFF)), 0x00000000},
		})
	})

	t.Run("FrontFace", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"CCW", toWGPUFrontFace(types.FrontFaceCCW), 0x00000001},
			{"CW", toWGPUFrontFace(types.FrontFaceCW), 0x00000002},
			{"Unknown", toWGPUFrontFace(types.FrontFace(0xFFFFFFFF)), 0x00000000},
		})
	})

	t.Run("CullMode", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"None", toWGPUCullMode(types.CullModeNone), 0x00000001},
			{"Front", toWGPUCullMode(types.CullModeFront), 0x00000002},
			{"Back", toWGPUCullMode(types.CullModeBack), 0x00000003},
			{"Unknown", toWGPUCullMode(types.CullMode(0xFFFFFFFF)), 0x00000000},
		})
	})

	t.Run("BufferBindingType", func(t *testing.T) {
		// gputypes: Undefined=0,Uniform=1,Storage=2,ReadOnlyStorage=3
		// header: BindingNotUsed=0,Undefined=1,Uniform=2,Storage=3,ReadOnlyStorage=4
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined_as_BindingNotUsed", toWGPUBufferBindingType(types.BufferBindingTypeUndefined), 0x00000000},
			{"Uniform", toWGPUBufferBindingType(types.BufferBindingTypeUniform), 0x00000002},
			{"Storage", toWGPUBufferBindingType(types.BufferBindingTypeStorage), 0x00000003},
			{"ReadOnlyStorage", toWGPUBufferBindingType(types.BufferBindingTypeReadOnlyStorage), 0x00000004},
		})
	})

	t.Run("SamplerBindingType", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined_as_BindingNotUsed", toWGPUSamplerBindingType(types.SamplerBindingTypeUndefined), 0x00000000},
			{"Filtering", toWGPUSamplerBindingType(types.SamplerBindingTypeFiltering), 0x00000002},
			{"NonFiltering", toWGPUSamplerBindingType(types.SamplerBindingTypeNonFiltering), 0x00000003},
			{"Comparison", toWGPUSamplerBindingType(types.SamplerBindingTypeComparison), 0x00000004},
		})
	})

	t.Run("TextureSampleType", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined_as_BindingNotUsed", toWGPUTextureSampleType(types.TextureSampleTypeUndefined), 0x00000000},
			{"Float", toWGPUTextureSampleType(types.TextureSampleTypeFloat), 0x00000002},
			{"UnfilterableFloat", toWGPUTextureSampleType(types.TextureSampleTypeUnfilterableFloat), 0x00000003},
			{"Depth", toWGPUTextureSampleType(types.TextureSampleTypeDepth), 0x00000004},
			{"Sint", toWGPUTextureSampleType(types.TextureSampleTypeSint), 0x00000005},
			{"Uint", toWGPUTextureSampleType(types.TextureSampleTypeUint), 0x00000006},
		})
	})

	t.Run("StorageTextureAccess", func(t *testing.T) {
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined_as_BindingNotUsed", toWGPUStorageTextureAccess(types.StorageTextureAccessUndefined), 0x00000000},
			{"WriteOnly", toWGPUStorageTextureAccess(types.StorageTextureAccessWriteOnly), 0x00000002},
			{"ReadOnly", toWGPUStorageTextureAccess(types.StorageTextureAccessReadOnly), 0x00000003},
			{"ReadWrite", toWGPUStorageTextureAccess(types.StorageTextureAccessReadWrite), 0x00000004},
		})
	})

	t.Run("VertexStepMode", func(t *testing.T) {
		// gputypes: Undefined=0, VertexBufferNotUsed=1, Vertex=2, Instance=3
		// header: Undefined=0, Vertex=1, Instance=2
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", toWGPUVertexStepMode(types.VertexStepModeUndefined), 0x00000000},
			{"VertexBufferNotUsed", toWGPUVertexStepMode(types.VertexStepModeVertexBufferNotUsed), 0x00000000},
			{"Vertex", toWGPUVertexStepMode(types.VertexStepModeVertex), 0x00000001},
			{"Instance", toWGPUVertexStepMode(types.VertexStepModeInstance), 0x00000002},
		})
	})

	t.Run("VertexFormat", func(t *testing.T) {
		// Full mapping table from convert.go comments vs header WGPUVertexFormat.
		runEnumTests(t, []struct {
			name     string
			got      uint32
			expected uint32
		}{
			{"Undefined", toWGPUVertexFormat(types.VertexFormatUndefined), 0},
			{"Uint8x2", toWGPUVertexFormat(types.VertexFormatUint8x2), 2},
			{"Uint8x4", toWGPUVertexFormat(types.VertexFormatUint8x4), 3},
			{"Sint8x2", toWGPUVertexFormat(types.VertexFormatSint8x2), 5},
			{"Sint8x4", toWGPUVertexFormat(types.VertexFormatSint8x4), 6},
			{"Unorm8x2", toWGPUVertexFormat(types.VertexFormatUnorm8x2), 8},
			{"Unorm8x4", toWGPUVertexFormat(types.VertexFormatUnorm8x4), 9},
			{"Snorm8x2", toWGPUVertexFormat(types.VertexFormatSnorm8x2), 11},
			{"Snorm8x4", toWGPUVertexFormat(types.VertexFormatSnorm8x4), 12},
			{"Uint16x2", toWGPUVertexFormat(types.VertexFormatUint16x2), 14},
			{"Uint16x4", toWGPUVertexFormat(types.VertexFormatUint16x4), 15},
			{"Sint16x2", toWGPUVertexFormat(types.VertexFormatSint16x2), 17},
			{"Sint16x4", toWGPUVertexFormat(types.VertexFormatSint16x4), 18},
			{"Unorm16x2", toWGPUVertexFormat(types.VertexFormatUnorm16x2), 20},
			{"Unorm16x4", toWGPUVertexFormat(types.VertexFormatUnorm16x4), 21},
			{"Snorm16x2", toWGPUVertexFormat(types.VertexFormatSnorm16x2), 23},
			{"Snorm16x4", toWGPUVertexFormat(types.VertexFormatSnorm16x4), 24},
			{"Float16x2", toWGPUVertexFormat(types.VertexFormatFloat16x2), 26},
			{"Float16x4", toWGPUVertexFormat(types.VertexFormatFloat16x4), 27},
			{"Float32", toWGPUVertexFormat(types.VertexFormatFloat32), 28},
			{"Float32x2", toWGPUVertexFormat(types.VertexFormatFloat32x2), 29},
			{"Float32x3", toWGPUVertexFormat(types.VertexFormatFloat32x3), 30},
			{"Float32x4", toWGPUVertexFormat(types.VertexFormatFloat32x4), 31},
			{"Uint32", toWGPUVertexFormat(types.VertexFormatUint32), 32},
			{"Uint32x2", toWGPUVertexFormat(types.VertexFormatUint32x2), 33},
			{"Uint32x3", toWGPUVertexFormat(types.VertexFormatUint32x3), 34},
			{"Uint32x4", toWGPUVertexFormat(types.VertexFormatUint32x4), 35},
			{"Sint32", toWGPUVertexFormat(types.VertexFormatSint32), 36},
			{"Sint32x2", toWGPUVertexFormat(types.VertexFormatSint32x2), 37},
			{"Sint32x3", toWGPUVertexFormat(types.VertexFormatSint32x3), 38},
			{"Sint32x4", toWGPUVertexFormat(types.VertexFormatSint32x4), 39},
			{"Unorm1010102", toWGPUVertexFormat(types.VertexFormatUnorm1010102), 40},
		})
	})
}

// TestS1ConvertedEnumsRejectSilentIdentity documents that raw cast is WRONG
// for converted enums — regression guard if someone removes toWGPU*.
func TestS1ConvertedEnumsRejectSilentIdentity(t *testing.T) {
	cases := []struct {
		name      string
		raw       uint32
		converted uint32
		header    uint32
	}{
		{
			name:      "PrimitiveTopologyTriangleList",
			raw:       uint32(types.PrimitiveTopologyTriangleList),
			converted: toWGPUPrimitiveTopology(types.PrimitiveTopologyTriangleList),
			header:    0x00000004,
		},
		{
			name:      "FrontFaceCCW",
			raw:       uint32(types.FrontFaceCCW),
			converted: toWGPUFrontFace(types.FrontFaceCCW),
			header:    0x00000001,
		},
		{
			name:      "CullModeNone",
			raw:       uint32(types.CullModeNone),
			converted: toWGPUCullMode(types.CullModeNone),
			header:    0x00000001,
		},
		{
			name:      "BufferBindingTypeUniform",
			raw:       uint32(types.BufferBindingTypeUniform),
			converted: toWGPUBufferBindingType(types.BufferBindingTypeUniform),
			header:    0x00000002,
		},
		{
			name:      "VertexStepModeVertex",
			raw:       uint32(types.VertexStepModeVertex),
			converted: toWGPUVertexStepMode(types.VertexStepModeVertex),
			header:    0x00000001,
		},
		{
			name:      "VertexFormatFloat32",
			raw:       uint32(types.VertexFormatFloat32),
			converted: toWGPUVertexFormat(types.VertexFormatFloat32),
			header:    28,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.converted != tc.header {
				t.Fatalf("converter = %#x, want header %#x", tc.converted, tc.header)
			}
			if tc.raw == tc.header {
				t.Fatalf("raw cast %#x equals header — converter may be unnecessary; re-audit", tc.raw)
			}
			if tc.raw == tc.converted {
				t.Fatalf("raw cast equals converter for non-identity enum %#x", tc.raw)
			}
		})
	}
}
