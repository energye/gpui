// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build windows && !(js && wasm)

package dx12

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu/hal"
	"github.com/energye/gpui/gpu/webgpu/hal/dx12/d3d12"
	"github.com/energye/gpui/gpu/webgpu/hal/dx12/dxgi"
)

func TestTextureFormatToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		format types.TextureFormat
		expect d3d12.DXGI_FORMAT
	}{
		// 8-bit formats
		{"R8Unorm", types.TextureFormatR8Unorm, d3d12.DXGI_FORMAT_R8_UNORM},
		{"R8Snorm", types.TextureFormatR8Snorm, d3d12.DXGI_FORMAT_R8_SNORM},
		{"R8Uint", types.TextureFormatR8Uint, d3d12.DXGI_FORMAT_R8_UINT},
		{"R8Sint", types.TextureFormatR8Sint, d3d12.DXGI_FORMAT_R8_SINT},

		// 16-bit formats
		{"R16Uint", types.TextureFormatR16Uint, d3d12.DXGI_FORMAT_R16_UINT},
		{"R16Sint", types.TextureFormatR16Sint, d3d12.DXGI_FORMAT_R16_SINT},
		{"R16Float", types.TextureFormatR16Float, d3d12.DXGI_FORMAT_R16_FLOAT},
		{"RG8Unorm", types.TextureFormatRG8Unorm, d3d12.DXGI_FORMAT_R8G8_UNORM},
		{"RG8Snorm", types.TextureFormatRG8Snorm, d3d12.DXGI_FORMAT_R8G8_SNORM},
		{"RG8Uint", types.TextureFormatRG8Uint, d3d12.DXGI_FORMAT_R8G8_UINT},
		{"RG8Sint", types.TextureFormatRG8Sint, d3d12.DXGI_FORMAT_R8G8_SINT},

		// 32-bit formats
		{"R32Uint", types.TextureFormatR32Uint, d3d12.DXGI_FORMAT_R32_UINT},
		{"R32Sint", types.TextureFormatR32Sint, d3d12.DXGI_FORMAT_R32_SINT},
		{"R32Float", types.TextureFormatR32Float, d3d12.DXGI_FORMAT_R32_FLOAT},
		{"RG16Uint", types.TextureFormatRG16Uint, d3d12.DXGI_FORMAT_R16G16_UINT},
		{"RG16Sint", types.TextureFormatRG16Sint, d3d12.DXGI_FORMAT_R16G16_SINT},
		{"RG16Float", types.TextureFormatRG16Float, d3d12.DXGI_FORMAT_R16G16_FLOAT},
		{"RGBA8Unorm", types.TextureFormatRGBA8Unorm, d3d12.DXGI_FORMAT_R8G8B8A8_UNORM},
		{"RGBA8UnormSrgb", types.TextureFormatRGBA8UnormSrgb, d3d12.DXGI_FORMAT_R8G8B8A8_UNORM_SRGB},
		{"RGBA8Snorm", types.TextureFormatRGBA8Snorm, d3d12.DXGI_FORMAT_R8G8B8A8_SNORM},
		{"RGBA8Uint", types.TextureFormatRGBA8Uint, d3d12.DXGI_FORMAT_R8G8B8A8_UINT},
		{"RGBA8Sint", types.TextureFormatRGBA8Sint, d3d12.DXGI_FORMAT_R8G8B8A8_SINT},
		{"BGRA8Unorm", types.TextureFormatBGRA8Unorm, d3d12.DXGI_FORMAT_B8G8R8A8_UNORM},
		{"BGRA8UnormSrgb", types.TextureFormatBGRA8UnormSrgb, d3d12.DXGI_FORMAT_B8G8R8A8_UNORM_SRGB},

		// Packed formats
		{"RGB10A2Uint", types.TextureFormatRGB10A2Uint, d3d12.DXGI_FORMAT_R10G10B10A2_UINT},
		{"RGB10A2Unorm", types.TextureFormatRGB10A2Unorm, d3d12.DXGI_FORMAT_R10G10B10A2_UNORM},
		{"RG11B10Ufloat", types.TextureFormatRG11B10Ufloat, d3d12.DXGI_FORMAT_R11G11B10_FLOAT},

		// 64-bit formats
		{"RG32Uint", types.TextureFormatRG32Uint, d3d12.DXGI_FORMAT_R32G32_UINT},
		{"RG32Sint", types.TextureFormatRG32Sint, d3d12.DXGI_FORMAT_R32G32_SINT},
		{"RG32Float", types.TextureFormatRG32Float, d3d12.DXGI_FORMAT_R32G32_FLOAT},
		{"RGBA16Uint", types.TextureFormatRGBA16Uint, d3d12.DXGI_FORMAT_R16G16B16A16_UINT},
		{"RGBA16Sint", types.TextureFormatRGBA16Sint, d3d12.DXGI_FORMAT_R16G16B16A16_SINT},
		{"RGBA16Float", types.TextureFormatRGBA16Float, d3d12.DXGI_FORMAT_R16G16B16A16_FLOAT},

		// 128-bit formats
		{"RGBA32Uint", types.TextureFormatRGBA32Uint, d3d12.DXGI_FORMAT_R32G32B32A32_UINT},
		{"RGBA32Sint", types.TextureFormatRGBA32Sint, d3d12.DXGI_FORMAT_R32G32B32A32_SINT},
		{"RGBA32Float", types.TextureFormatRGBA32Float, d3d12.DXGI_FORMAT_R32G32B32A32_FLOAT},

		// Depth/stencil formats
		{"Depth16Unorm", types.TextureFormatDepth16Unorm, d3d12.DXGI_FORMAT_D16_UNORM},
		{"Depth24Plus", types.TextureFormatDepth24Plus, d3d12.DXGI_FORMAT_D24_UNORM_S8_UINT},
		{"Depth24PlusStencil8", types.TextureFormatDepth24PlusStencil8, d3d12.DXGI_FORMAT_D24_UNORM_S8_UINT},
		{"Depth32Float", types.TextureFormatDepth32Float, d3d12.DXGI_FORMAT_D32_FLOAT},
		{"Depth32FloatStencil8", types.TextureFormatDepth32FloatStencil8, d3d12.DXGI_FORMAT_D32_FLOAT_S8X24_UINT},
		{"Stencil8", types.TextureFormatStencil8, d3d12.DXGI_FORMAT_D24_UNORM_S8_UINT},

		// BC compressed formats
		{"BC1RGBAUnorm", types.TextureFormatBC1RGBAUnorm, d3d12.DXGI_FORMAT_BC1_UNORM},
		{"BC1RGBAUnormSrgb", types.TextureFormatBC1RGBAUnormSrgb, d3d12.DXGI_FORMAT_BC1_UNORM_SRGB},
		{"BC2RGBAUnorm", types.TextureFormatBC2RGBAUnorm, d3d12.DXGI_FORMAT_BC2_UNORM},
		{"BC2RGBAUnormSrgb", types.TextureFormatBC2RGBAUnormSrgb, d3d12.DXGI_FORMAT_BC2_UNORM_SRGB},
		{"BC3RGBAUnorm", types.TextureFormatBC3RGBAUnorm, d3d12.DXGI_FORMAT_BC3_UNORM},
		{"BC3RGBAUnormSrgb", types.TextureFormatBC3RGBAUnormSrgb, d3d12.DXGI_FORMAT_BC3_UNORM_SRGB},
		{"BC4RUnorm", types.TextureFormatBC4RUnorm, d3d12.DXGI_FORMAT_BC4_UNORM},
		{"BC4RSnorm", types.TextureFormatBC4RSnorm, d3d12.DXGI_FORMAT_BC4_SNORM},
		{"BC5RGUnorm", types.TextureFormatBC5RGUnorm, d3d12.DXGI_FORMAT_BC5_UNORM},
		{"BC5RGSnorm", types.TextureFormatBC5RGSnorm, d3d12.DXGI_FORMAT_BC5_SNORM},
		{"BC6HRGBUfloat", types.TextureFormatBC6HRGBUfloat, d3d12.DXGI_FORMAT_BC6H_UF16},
		{"BC6HRGBFloat", types.TextureFormatBC6HRGBFloat, d3d12.DXGI_FORMAT_BC6H_SF16},
		{"BC7RGBAUnorm", types.TextureFormatBC7RGBAUnorm, d3d12.DXGI_FORMAT_BC7_UNORM},
		{"BC7RGBAUnormSrgb", types.TextureFormatBC7RGBAUnormSrgb, d3d12.DXGI_FORMAT_BC7_UNORM_SRGB},

		// Unknown format
		{"Unknown", types.TextureFormat(65535), d3d12.DXGI_FORMAT_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureFormatToD3D12(tt.format)
			if got != tt.expect {
				t.Errorf("textureFormatToD3D12(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

func TestTextureDimensionToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		dim    types.TextureDimension
		expect d3d12.D3D12_RESOURCE_DIMENSION
	}{
		{"1D", types.TextureDimension1D, d3d12.D3D12_RESOURCE_DIMENSION_TEXTURE1D},
		{"2D", types.TextureDimension2D, d3d12.D3D12_RESOURCE_DIMENSION_TEXTURE2D},
		{"3D", types.TextureDimension3D, d3d12.D3D12_RESOURCE_DIMENSION_TEXTURE3D},
		{"Unknown defaults to 2D", types.TextureDimension(99), d3d12.D3D12_RESOURCE_DIMENSION_TEXTURE2D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureDimensionToD3D12(tt.dim)
			if got != tt.expect {
				t.Errorf("textureDimensionToD3D12(%v) = %v, want %v", tt.dim, got, tt.expect)
			}
		})
	}
}

func TestTextureViewDimensionToSRV(t *testing.T) {
	tests := []struct {
		name   string
		dim    types.TextureViewDimension
		expect d3d12.D3D12_SRV_DIMENSION
	}{
		{"1D", types.TextureViewDimension1D, d3d12.D3D12_SRV_DIMENSION_TEXTURE1D},
		{"2D", types.TextureViewDimension2D, d3d12.D3D12_SRV_DIMENSION_TEXTURE2D},
		{"2DArray", types.TextureViewDimension2DArray, d3d12.D3D12_SRV_DIMENSION_TEXTURE2DARRAY},
		{"Cube", types.TextureViewDimensionCube, d3d12.D3D12_SRV_DIMENSION_TEXTURECUBE},
		{"CubeArray", types.TextureViewDimensionCubeArray, d3d12.D3D12_SRV_DIMENSION_TEXTURECUBEARRAY},
		{"3D", types.TextureViewDimension3D, d3d12.D3D12_SRV_DIMENSION_TEXTURE3D},
		{"Unknown defaults to 2D", types.TextureViewDimension(99), d3d12.D3D12_SRV_DIMENSION_TEXTURE2D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureViewDimensionToSRV(tt.dim)
			if got != tt.expect {
				t.Errorf("textureViewDimensionToSRV(%v) = %v, want %v", tt.dim, got, tt.expect)
			}
		})
	}
}

func TestTextureViewDimensionToRTV(t *testing.T) {
	tests := []struct {
		name   string
		dim    types.TextureViewDimension
		expect d3d12.D3D12_RTV_DIMENSION
	}{
		{"1D", types.TextureViewDimension1D, d3d12.D3D12_RTV_DIMENSION_TEXTURE1D},
		{"2D", types.TextureViewDimension2D, d3d12.D3D12_RTV_DIMENSION_TEXTURE2D},
		{"2DArray", types.TextureViewDimension2DArray, d3d12.D3D12_RTV_DIMENSION_TEXTURE2DARRAY},
		{"3D", types.TextureViewDimension3D, d3d12.D3D12_RTV_DIMENSION_TEXTURE3D},
		{"Unknown defaults to 2D", types.TextureViewDimension(99), d3d12.D3D12_RTV_DIMENSION_TEXTURE2D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureViewDimensionToRTV(tt.dim)
			if got != tt.expect {
				t.Errorf("textureViewDimensionToRTV(%v) = %v, want %v", tt.dim, got, tt.expect)
			}
		})
	}
}

func TestTextureViewDimensionToDSV(t *testing.T) {
	tests := []struct {
		name   string
		dim    types.TextureViewDimension
		expect d3d12.D3D12_DSV_DIMENSION
	}{
		{"1D", types.TextureViewDimension1D, d3d12.D3D12_DSV_DIMENSION_TEXTURE1D},
		{"2D", types.TextureViewDimension2D, d3d12.D3D12_DSV_DIMENSION_TEXTURE2D},
		{"2DArray", types.TextureViewDimension2DArray, d3d12.D3D12_DSV_DIMENSION_TEXTURE2DARRAY},
		{"Unknown defaults to 2D", types.TextureViewDimension(99), d3d12.D3D12_DSV_DIMENSION_TEXTURE2D},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureViewDimensionToDSV(tt.dim)
			if got != tt.expect {
				t.Errorf("textureViewDimensionToDSV(%v) = %v, want %v", tt.dim, got, tt.expect)
			}
		})
	}
}

func TestIsDepthFormat(t *testing.T) {
	tests := []struct {
		name   string
		format types.TextureFormat
		expect bool
	}{
		{"Depth16Unorm", types.TextureFormatDepth16Unorm, true},
		{"Depth24Plus", types.TextureFormatDepth24Plus, true},
		{"Depth24PlusStencil8", types.TextureFormatDepth24PlusStencil8, true},
		{"Depth32Float", types.TextureFormatDepth32Float, true},
		{"Depth32FloatStencil8", types.TextureFormatDepth32FloatStencil8, true},
		{"Stencil8", types.TextureFormatStencil8, true},
		{"RGBA8Unorm", types.TextureFormatRGBA8Unorm, false},
		{"R32Float", types.TextureFormatR32Float, false},
		{"BGRA8Unorm", types.TextureFormatBGRA8Unorm, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDepthFormat(tt.format)
			if got != tt.expect {
				t.Errorf("isDepthFormat(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

func TestDepthFormatToTypeless(t *testing.T) {
	tests := []struct {
		name   string
		format types.TextureFormat
		expect d3d12.DXGI_FORMAT
	}{
		{"Depth16Unorm", types.TextureFormatDepth16Unorm, d3d12.DXGI_FORMAT_R16_TYPELESS},
		{"Depth24Plus", types.TextureFormatDepth24Plus, d3d12.DXGI_FORMAT_R24G8_TYPELESS},
		{"Depth24PlusStencil8", types.TextureFormatDepth24PlusStencil8, d3d12.DXGI_FORMAT_R24G8_TYPELESS},
		{"Depth32Float", types.TextureFormatDepth32Float, d3d12.DXGI_FORMAT_R32_TYPELESS},
		{"Depth32FloatStencil8", types.TextureFormatDepth32FloatStencil8, d3d12.DXGI_FORMAT_R32G8X24_TYPELESS},
		{"Non-depth returns UNKNOWN", types.TextureFormatRGBA8Unorm, d3d12.DXGI_FORMAT_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := depthFormatToTypeless(tt.format)
			if got != tt.expect {
				t.Errorf("depthFormatToTypeless(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

func TestDepthFormatToSRV(t *testing.T) {
	tests := []struct {
		name   string
		format types.TextureFormat
		expect d3d12.DXGI_FORMAT
	}{
		{"Depth16Unorm", types.TextureFormatDepth16Unorm, d3d12.DXGI_FORMAT_R16_UNORM},
		{"Depth24Plus", types.TextureFormatDepth24Plus, d3d12.DXGI_FORMAT_R24_UNORM_X8_TYPELESS},
		{"Depth24PlusStencil8", types.TextureFormatDepth24PlusStencil8, d3d12.DXGI_FORMAT_R24_UNORM_X8_TYPELESS},
		{"Depth32Float", types.TextureFormatDepth32Float, d3d12.DXGI_FORMAT_R32_FLOAT},
		{"Depth32FloatStencil8", types.TextureFormatDepth32FloatStencil8, d3d12.DXGI_FORMAT_R32_FLOAT_X8X24_TYPELESS},
		{"Non-depth returns UNKNOWN", types.TextureFormatRGBA8Unorm, d3d12.DXGI_FORMAT_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := depthFormatToSRV(tt.format)
			if got != tt.expect {
				t.Errorf("depthFormatToSRV(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

func TestAddressModeToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.AddressMode
		expect d3d12.D3D12_TEXTURE_ADDRESS_MODE
	}{
		{"Repeat", types.AddressModeRepeat, d3d12.D3D12_TEXTURE_ADDRESS_MODE_WRAP},
		{"MirrorRepeat", types.AddressModeMirrorRepeat, d3d12.D3D12_TEXTURE_ADDRESS_MODE_MIRROR},
		{"ClampToEdge", types.AddressModeClampToEdge, d3d12.D3D12_TEXTURE_ADDRESS_MODE_CLAMP},
		{"Unknown defaults to Clamp", types.AddressMode(99), d3d12.D3D12_TEXTURE_ADDRESS_MODE_CLAMP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addressModeToD3D12(tt.mode)
			if got != tt.expect {
				t.Errorf("addressModeToD3D12(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

func TestFilterModeToD3D12(t *testing.T) {
	tests := []struct {
		name    string
		min     types.FilterMode
		mag     types.FilterMode
		mipmap  types.FilterMode
		compare types.CompareFunction
		expect  d3d12.D3D12_FILTER
	}{
		{"AllNearest", types.FilterModeNearest, types.FilterModeNearest, types.FilterModeNearest, types.CompareFunctionUndefined, d3d12.D3D12_FILTER(0x00)},
		{"MipLinear", types.FilterModeNearest, types.FilterModeNearest, types.FilterModeLinear, types.CompareFunctionUndefined, d3d12.D3D12_FILTER(0x01)},
		{"MagLinear", types.FilterModeNearest, types.FilterModeLinear, types.FilterModeNearest, types.CompareFunctionUndefined, d3d12.D3D12_FILTER(0x04)},
		{"MinLinear", types.FilterModeLinear, types.FilterModeNearest, types.FilterModeNearest, types.CompareFunctionUndefined, d3d12.D3D12_FILTER(0x10)},
		{"AllLinear", types.FilterModeLinear, types.FilterModeLinear, types.FilterModeLinear, types.CompareFunctionUndefined, d3d12.D3D12_FILTER(0x15)},
		{"ComparisonAllNearest", types.FilterModeNearest, types.FilterModeNearest, types.FilterModeNearest, types.CompareFunctionLess, d3d12.D3D12_FILTER(0x80)},
		{"ComparisonAllLinear", types.FilterModeLinear, types.FilterModeLinear, types.FilterModeLinear, types.CompareFunctionLess, d3d12.D3D12_FILTER(0x95)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterModeToD3D12(tt.min, tt.mag, tt.mipmap, tt.compare)
			if got != tt.expect {
				t.Errorf("filterModeToD3D12() = %#x, want %#x", got, tt.expect)
			}
		})
	}
}

func TestCompareFunctionToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		fn     types.CompareFunction
		expect d3d12.D3D12_COMPARISON_FUNC
	}{
		{"Never", types.CompareFunctionNever, d3d12.D3D12_COMPARISON_FUNC_NEVER},
		{"Less", types.CompareFunctionLess, d3d12.D3D12_COMPARISON_FUNC_LESS},
		{"Equal", types.CompareFunctionEqual, d3d12.D3D12_COMPARISON_FUNC_EQUAL},
		{"LessEqual", types.CompareFunctionLessEqual, d3d12.D3D12_COMPARISON_FUNC_LESS_EQUAL},
		{"Greater", types.CompareFunctionGreater, d3d12.D3D12_COMPARISON_FUNC_GREATER},
		{"NotEqual", types.CompareFunctionNotEqual, d3d12.D3D12_COMPARISON_FUNC_NOT_EQUAL},
		{"GreaterEqual", types.CompareFunctionGreaterEqual, d3d12.D3D12_COMPARISON_FUNC_GREATER_EQUAL},
		{"Always", types.CompareFunctionAlways, d3d12.D3D12_COMPARISON_FUNC_ALWAYS},
		{"Unknown defaults to Never", types.CompareFunction(99), d3d12.D3D12_COMPARISON_FUNC_NEVER},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareFunctionToD3D12(tt.fn)
			if got != tt.expect {
				t.Errorf("compareFunctionToD3D12(%v) = %v, want %v", tt.fn, got, tt.expect)
			}
		})
	}
}

func TestAlignTo256(t *testing.T) {
	tests := []struct {
		name   string
		input  uint64
		expect uint64
	}{
		{"zero", 0, 0},
		{"1", 1, 256},
		{"255", 255, 256},
		{"256", 256, 256},
		{"257", 257, 512},
		{"512", 512, 512},
		{"1000", 1000, 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignTo256(tt.input)
			if got != tt.expect {
				t.Errorf("alignTo256(%d) = %d, want %d", tt.input, got, tt.expect)
			}
		})
	}
}

func TestBlendFactorToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		factor types.BlendFactor
		expect d3d12.D3D12_BLEND
	}{
		{"Zero", types.BlendFactorZero, d3d12.D3D12_BLEND_ZERO},
		{"One", types.BlendFactorOne, d3d12.D3D12_BLEND_ONE},
		{"Src", types.BlendFactorSrc, d3d12.D3D12_BLEND_SRC_COLOR},
		{"OneMinusSrc", types.BlendFactorOneMinusSrc, d3d12.D3D12_BLEND_INV_SRC_COLOR},
		{"SrcAlpha", types.BlendFactorSrcAlpha, d3d12.D3D12_BLEND_SRC_ALPHA},
		{"OneMinusSrcAlpha", types.BlendFactorOneMinusSrcAlpha, d3d12.D3D12_BLEND_INV_SRC_ALPHA},
		{"Dst", types.BlendFactorDst, d3d12.D3D12_BLEND_DEST_COLOR},
		{"OneMinusDst", types.BlendFactorOneMinusDst, d3d12.D3D12_BLEND_INV_DEST_COLOR},
		{"DstAlpha", types.BlendFactorDstAlpha, d3d12.D3D12_BLEND_DEST_ALPHA},
		{"OneMinusDstAlpha", types.BlendFactorOneMinusDstAlpha, d3d12.D3D12_BLEND_INV_DEST_ALPHA},
		{"SrcAlphaSaturated", types.BlendFactorSrcAlphaSaturated, d3d12.D3D12_BLEND_SRC_ALPHA_SAT},
		{"Constant", types.BlendFactorConstant, d3d12.D3D12_BLEND_BLEND_FACTOR},
		{"OneMinusConstant", types.BlendFactorOneMinusConstant, d3d12.D3D12_BLEND_INV_BLEND_FACTOR},
		{"Unknown defaults to One", types.BlendFactor(99), d3d12.D3D12_BLEND_ONE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blendFactorToD3D12(tt.factor)
			if got != tt.expect {
				t.Errorf("blendFactorToD3D12(%v) = %v, want %v", tt.factor, got, tt.expect)
			}
		})
	}
}

func TestBlendOperationToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		op     types.BlendOperation
		expect d3d12.D3D12_BLEND_OP
	}{
		{"Add", types.BlendOperationAdd, d3d12.D3D12_BLEND_OP_ADD},
		{"Subtract", types.BlendOperationSubtract, d3d12.D3D12_BLEND_OP_SUBTRACT},
		{"ReverseSubtract", types.BlendOperationReverseSubtract, d3d12.D3D12_BLEND_OP_REV_SUBTRACT},
		{"Min", types.BlendOperationMin, d3d12.D3D12_BLEND_OP_MIN},
		{"Max", types.BlendOperationMax, d3d12.D3D12_BLEND_OP_MAX},
		{"Unknown defaults to Add", types.BlendOperation(99), d3d12.D3D12_BLEND_OP_ADD},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blendOperationToD3D12(tt.op)
			if got != tt.expect {
				t.Errorf("blendOperationToD3D12(%v) = %v, want %v", tt.op, got, tt.expect)
			}
		})
	}
}

func TestCullModeToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.CullMode
		expect d3d12.D3D12_CULL_MODE
	}{
		{"None", types.CullModeNone, d3d12.D3D12_CULL_MODE_NONE},
		{"Front", types.CullModeFront, d3d12.D3D12_CULL_MODE_FRONT},
		{"Back", types.CullModeBack, d3d12.D3D12_CULL_MODE_BACK},
		{"Unknown defaults to None", types.CullMode(99), d3d12.D3D12_CULL_MODE_NONE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cullModeToD3D12(tt.mode)
			if got != tt.expect {
				t.Errorf("cullModeToD3D12(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

func TestFrontFaceToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		face   types.FrontFace
		expect int32
	}{
		{"CCW", types.FrontFaceCCW, 1},
		{"CW", types.FrontFaceCW, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := frontFaceToD3D12(tt.face)
			if got != tt.expect {
				t.Errorf("frontFaceToD3D12(%v) = %v, want %v", tt.face, got, tt.expect)
			}
		})
	}
}

func TestPrimitiveTopologyTypeToD3D12(t *testing.T) {
	tests := []struct {
		name     string
		topology types.PrimitiveTopology
		expect   d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE
	}{
		{"PointList", types.PrimitiveTopologyPointList, d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE_POINT},
		{"LineList", types.PrimitiveTopologyLineList, d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE_LINE},
		{"LineStrip", types.PrimitiveTopologyLineStrip, d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE_LINE},
		{"TriangleList", types.PrimitiveTopologyTriangleList, d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE_TRIANGLE},
		{"TriangleStrip", types.PrimitiveTopologyTriangleStrip, d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE_TRIANGLE},
		{"Unknown defaults to Triangle", types.PrimitiveTopology(99), d3d12.D3D12_PRIMITIVE_TOPOLOGY_TYPE_TRIANGLE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primitiveTopologyTypeToD3D12(tt.topology)
			if got != tt.expect {
				t.Errorf("primitiveTopologyTypeToD3D12(%v) = %v, want %v", tt.topology, got, tt.expect)
			}
		})
	}
}

func TestPrimitiveTopologyToD3D12(t *testing.T) {
	tests := []struct {
		name     string
		topology types.PrimitiveTopology
		expect   d3d12.D3D_PRIMITIVE_TOPOLOGY
	}{
		{"PointList", types.PrimitiveTopologyPointList, d3d12.D3D_PRIMITIVE_TOPOLOGY_POINTLIST},
		{"LineList", types.PrimitiveTopologyLineList, d3d12.D3D_PRIMITIVE_TOPOLOGY_LINELIST},
		{"LineStrip", types.PrimitiveTopologyLineStrip, d3d12.D3D_PRIMITIVE_TOPOLOGY_LINESTRIP},
		{"TriangleList", types.PrimitiveTopologyTriangleList, d3d12.D3D_PRIMITIVE_TOPOLOGY_TRIANGLELIST},
		{"TriangleStrip", types.PrimitiveTopologyTriangleStrip, d3d12.D3D_PRIMITIVE_TOPOLOGY_TRIANGLESTRIP},
		{"Unknown defaults to TriangleList", types.PrimitiveTopology(99), d3d12.D3D_PRIMITIVE_TOPOLOGY_TRIANGLELIST},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primitiveTopologyToD3D12(tt.topology)
			if got != tt.expect {
				t.Errorf("primitiveTopologyToD3D12(%v) = %v, want %v", tt.topology, got, tt.expect)
			}
		})
	}
}

func TestStencilOpToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		op     hal.StencilOperation
		expect d3d12.D3D12_STENCIL_OP
	}{
		{"Keep", hal.StencilOperationKeep, d3d12.D3D12_STENCIL_OP_KEEP},
		{"Zero", hal.StencilOperationZero, d3d12.D3D12_STENCIL_OP_ZERO},
		{"Replace", hal.StencilOperationReplace, d3d12.D3D12_STENCIL_OP_REPLACE},
		{"Invert", hal.StencilOperationInvert, d3d12.D3D12_STENCIL_OP_INVERT},
		{"IncrementClamp", hal.StencilOperationIncrementClamp, d3d12.D3D12_STENCIL_OP_INCR_SAT},
		{"DecrementClamp", hal.StencilOperationDecrementClamp, d3d12.D3D12_STENCIL_OP_DECR_SAT},
		{"IncrementWrap", hal.StencilOperationIncrementWrap, d3d12.D3D12_STENCIL_OP_INCR},
		{"DecrementWrap", hal.StencilOperationDecrementWrap, d3d12.D3D12_STENCIL_OP_DECR},
		{"Unknown defaults to Keep", hal.StencilOperation(99), d3d12.D3D12_STENCIL_OP_KEEP},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stencilOpToD3D12(tt.op)
			if got != tt.expect {
				t.Errorf("stencilOpToD3D12(%v) = %v, want %v", tt.op, got, tt.expect)
			}
		})
	}
}

func TestInputStepModeToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		mode   types.VertexStepMode
		expect d3d12.D3D12_INPUT_CLASSIFICATION
	}{
		{"Vertex", types.VertexStepModeVertex, d3d12.D3D12_INPUT_CLASSIFICATION_PER_VERTEX_DATA},
		{"Instance", types.VertexStepModeInstance, d3d12.D3D12_INPUT_CLASSIFICATION_PER_INSTANCE_DATA},
		{"Unknown defaults to Vertex", types.VertexStepMode(99), d3d12.D3D12_INPUT_CLASSIFICATION_PER_VERTEX_DATA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inputStepModeToD3D12(tt.mode)
			if got != tt.expect {
				t.Errorf("inputStepModeToD3D12(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}

func TestVertexFormatToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		format types.VertexFormat
		expect d3d12.DXGI_FORMAT
	}{
		// 8-bit formats
		{"Uint8x2", types.VertexFormatUint8x2, d3d12.DXGI_FORMAT_R8G8_UINT},
		{"Uint8x4", types.VertexFormatUint8x4, d3d12.DXGI_FORMAT_R8G8B8A8_UINT},
		{"Sint8x2", types.VertexFormatSint8x2, d3d12.DXGI_FORMAT_R8G8_SINT},
		{"Sint8x4", types.VertexFormatSint8x4, d3d12.DXGI_FORMAT_R8G8B8A8_SINT},
		{"Unorm8x2", types.VertexFormatUnorm8x2, d3d12.DXGI_FORMAT_R8G8_UNORM},
		{"Unorm8x4", types.VertexFormatUnorm8x4, d3d12.DXGI_FORMAT_R8G8B8A8_UNORM},
		{"Snorm8x2", types.VertexFormatSnorm8x2, d3d12.DXGI_FORMAT_R8G8_SNORM},
		{"Snorm8x4", types.VertexFormatSnorm8x4, d3d12.DXGI_FORMAT_R8G8B8A8_SNORM},

		// 16-bit formats
		{"Uint16x2", types.VertexFormatUint16x2, d3d12.DXGI_FORMAT_R16G16_UINT},
		{"Uint16x4", types.VertexFormatUint16x4, d3d12.DXGI_FORMAT_R16G16B16A16_UINT},
		{"Sint16x2", types.VertexFormatSint16x2, d3d12.DXGI_FORMAT_R16G16_SINT},
		{"Sint16x4", types.VertexFormatSint16x4, d3d12.DXGI_FORMAT_R16G16B16A16_SINT},
		{"Unorm16x2", types.VertexFormatUnorm16x2, d3d12.DXGI_FORMAT_R16G16_UNORM},
		{"Unorm16x4", types.VertexFormatUnorm16x4, d3d12.DXGI_FORMAT_R16G16B16A16_UNORM},
		{"Snorm16x2", types.VertexFormatSnorm16x2, d3d12.DXGI_FORMAT_R16G16_SNORM},
		{"Snorm16x4", types.VertexFormatSnorm16x4, d3d12.DXGI_FORMAT_R16G16B16A16_SNORM},
		{"Float16x2", types.VertexFormatFloat16x2, d3d12.DXGI_FORMAT_R16G16_FLOAT},
		{"Float16x4", types.VertexFormatFloat16x4, d3d12.DXGI_FORMAT_R16G16B16A16_FLOAT},

		// 32-bit formats
		{"Float32", types.VertexFormatFloat32, d3d12.DXGI_FORMAT_R32_FLOAT},
		{"Float32x2", types.VertexFormatFloat32x2, d3d12.DXGI_FORMAT_R32G32_FLOAT},
		{"Float32x3", types.VertexFormatFloat32x3, d3d12.DXGI_FORMAT_R32G32B32_FLOAT},
		{"Float32x4", types.VertexFormatFloat32x4, d3d12.DXGI_FORMAT_R32G32B32A32_FLOAT},
		{"Uint32", types.VertexFormatUint32, d3d12.DXGI_FORMAT_R32_UINT},
		{"Uint32x2", types.VertexFormatUint32x2, d3d12.DXGI_FORMAT_R32G32_UINT},
		{"Uint32x3", types.VertexFormatUint32x3, d3d12.DXGI_FORMAT_R32G32B32_UINT},
		{"Uint32x4", types.VertexFormatUint32x4, d3d12.DXGI_FORMAT_R32G32B32A32_UINT},
		{"Sint32", types.VertexFormatSint32, d3d12.DXGI_FORMAT_R32_SINT},
		{"Sint32x2", types.VertexFormatSint32x2, d3d12.DXGI_FORMAT_R32G32_SINT},
		{"Sint32x3", types.VertexFormatSint32x3, d3d12.DXGI_FORMAT_R32G32B32_SINT},
		{"Sint32x4", types.VertexFormatSint32x4, d3d12.DXGI_FORMAT_R32G32B32A32_SINT},

		// Packed
		{"Unorm1010102", types.VertexFormatUnorm1010102, d3d12.DXGI_FORMAT_R10G10B10A2_UNORM},

		// Unknown
		{"Unknown", types.VertexFormat(255), d3d12.DXGI_FORMAT_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vertexFormatToD3D12(tt.format)
			if got != tt.expect {
				t.Errorf("vertexFormatToD3D12(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

func TestColorWriteMaskToD3D12(t *testing.T) {
	tests := []struct {
		name   string
		mask   types.ColorWriteMask
		expect uint8
	}{
		{"Red", types.ColorWriteMaskRed, uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_RED)},
		{"Green", types.ColorWriteMaskGreen, uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_GREEN)},
		{"Blue", types.ColorWriteMaskBlue, uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_BLUE)},
		{"Alpha", types.ColorWriteMaskAlpha, uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_ALPHA)},
		{
			"All",
			types.ColorWriteMaskRed | types.ColorWriteMaskGreen | types.ColorWriteMaskBlue | types.ColorWriteMaskAlpha,
			uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_RED) | uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_GREEN) | uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_BLUE) | uint8(d3d12.D3D12_COLOR_WRITE_ENABLE_ALPHA),
		},
		{"None", types.ColorWriteMask(0), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorWriteMaskToD3D12(tt.mask)
			if got != tt.expect {
				t.Errorf("colorWriteMaskToD3D12(%v) = %v, want %v", tt.mask, got, tt.expect)
			}
		})
	}
}

func TestShaderStagesToD3D12Visibility(t *testing.T) {
	tests := []struct {
		name   string
		stages types.ShaderStages
		expect d3d12.D3D12_SHADER_VISIBILITY
	}{
		{"Vertex only", types.ShaderStageVertex, d3d12.D3D12_SHADER_VISIBILITY_VERTEX},
		{"Fragment only", types.ShaderStageFragment, d3d12.D3D12_SHADER_VISIBILITY_PIXEL},
		{"All stages", types.ShaderStageVertex | types.ShaderStageFragment | types.ShaderStageCompute, d3d12.D3D12_SHADER_VISIBILITY_ALL},
		{"Vertex+Fragment", types.ShaderStageVertex | types.ShaderStageFragment, d3d12.D3D12_SHADER_VISIBILITY_ALL},
		{"Compute only", types.ShaderStageCompute, d3d12.D3D12_SHADER_VISIBILITY_ALL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shaderStagesToD3D12Visibility(tt.stages)
			if got != tt.expect {
				t.Errorf("shaderStagesToD3D12Visibility(%v) = %v, want %v", tt.stages, got, tt.expect)
			}
		})
	}
}

// TestTextureFormatToDXGI tests the DXGI surface format conversion.
func TestTextureFormatToDXGI(t *testing.T) {
	tests := []struct {
		name   string
		format types.TextureFormat
		expect dxgi.DXGI_FORMAT
	}{
		// Common surface formats
		{"RGBA8Unorm", types.TextureFormatRGBA8Unorm, dxgi.DXGI_FORMAT_R8G8B8A8_UNORM},
		{"RGBA8UnormSrgb", types.TextureFormatRGBA8UnormSrgb, dxgi.DXGI_FORMAT_R8G8B8A8_UNORM_SRGB},
		{"BGRA8Unorm", types.TextureFormatBGRA8Unorm, dxgi.DXGI_FORMAT_B8G8R8A8_UNORM},
		{"BGRA8UnormSrgb", types.TextureFormatBGRA8UnormSrgb, dxgi.DXGI_FORMAT_B8G8R8A8_UNORM_SRGB},
		{"RGBA16Float", types.TextureFormatRGBA16Float, dxgi.DXGI_FORMAT_R16G16B16A16_FLOAT},
		{"RGB10A2Unorm", types.TextureFormatRGB10A2Unorm, dxgi.DXGI_FORMAT_R10G10B10A2_UNORM},

		// Depth formats
		{"Depth16Unorm", types.TextureFormatDepth16Unorm, dxgi.DXGI_FORMAT_D16_UNORM},
		{"Depth24Plus", types.TextureFormatDepth24Plus, dxgi.DXGI_FORMAT_D24_UNORM_S8_UINT},
		{"Depth24PlusStencil8", types.TextureFormatDepth24PlusStencil8, dxgi.DXGI_FORMAT_D24_UNORM_S8_UINT},
		{"Depth32Float", types.TextureFormatDepth32Float, dxgi.DXGI_FORMAT_D32_FLOAT},
		{"Depth32FloatStencil8", types.TextureFormatDepth32FloatStencil8, dxgi.DXGI_FORMAT_D32_FLOAT_S8X24_UINT},

		// BC compressed formats
		{"BC1RGBAUnorm", types.TextureFormatBC1RGBAUnorm, dxgi.DXGI_FORMAT_BC1_UNORM},
		{"BC7RGBAUnorm", types.TextureFormatBC7RGBAUnorm, dxgi.DXGI_FORMAT_BC7_UNORM},

		// Unknown format
		{"Unknown", types.TextureFormat(65535), dxgi.DXGI_FORMAT_UNKNOWN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := textureFormatToDXGI(tt.format)
			if got != tt.expect {
				t.Errorf("textureFormatToDXGI(%v) = %v, want %v", tt.format, got, tt.expect)
			}
		})
	}
}

// TestCompositeAlphaModeToDXGI tests composite alpha mode conversions.
func TestCompositeAlphaModeToDXGI(t *testing.T) {
	tests := []struct {
		name   string
		mode   hal.CompositeAlphaMode
		expect dxgi.DXGI_ALPHA_MODE
	}{
		{"Premultiplied", hal.CompositeAlphaModePremultiplied, dxgi.DXGI_ALPHA_MODE_PREMULTIPLIED},
		{"Unpremultiplied", hal.CompositeAlphaModeUnpremultiplied, dxgi.DXGI_ALPHA_MODE_STRAIGHT},
		{"Inherit", hal.CompositeAlphaModeInherit, dxgi.DXGI_ALPHA_MODE_UNSPECIFIED},
		{"Opaque", hal.CompositeAlphaModeOpaque, dxgi.DXGI_ALPHA_MODE_IGNORE},
		{"Unknown defaults to Ignore", hal.CompositeAlphaMode(99), dxgi.DXGI_ALPHA_MODE_IGNORE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compositeAlphaModeToDXGI(tt.mode)
			if got != tt.expect {
				t.Errorf("compositeAlphaModeToDXGI(%v) = %v, want %v", tt.mode, got, tt.expect)
			}
		})
	}
}
