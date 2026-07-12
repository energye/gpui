// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu/hal/gles/gl"
)

func TestMapFilterMode(t *testing.T) {
	tests := []struct {
		name string
		mode types.FilterMode
		want int32
	}{
		{"Nearest", types.FilterModeNearest, gl.NEAREST},
		{"Linear", types.FilterModeLinear, gl.LINEAR},
		{"Undefined defaults to Nearest", types.FilterModeUndefined, gl.NEAREST},
		{"Unknown defaults to Nearest", types.FilterMode(99), gl.NEAREST},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapFilterMode(tt.mode)
			if got != tt.want {
				t.Errorf("mapFilterMode(%v) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestMapMinFilter(t *testing.T) {
	tests := []struct {
		name         string
		minFilter    types.FilterMode
		mipmapFilter types.FilterMode
		want         int32
	}{
		{"Nearest+Nearest", types.FilterModeNearest, types.FilterModeNearest, gl.NEAREST_MIPMAP_NEAREST},
		{"Nearest+Linear", types.FilterModeNearest, types.FilterModeLinear, gl.NEAREST_MIPMAP_LINEAR},
		{"Linear+Nearest", types.FilterModeLinear, types.FilterModeNearest, gl.LINEAR_MIPMAP_NEAREST},
		{"Linear+Linear", types.FilterModeLinear, types.FilterModeLinear, gl.LINEAR_MIPMAP_LINEAR},
		{"Nearest+Undefined", types.FilterModeNearest, types.FilterModeUndefined, gl.NEAREST_MIPMAP_NEAREST},
		{"Linear+Undefined", types.FilterModeLinear, types.FilterModeUndefined, gl.LINEAR_MIPMAP_NEAREST},
		{"Default (both zero)", types.FilterMode(0), types.FilterMode(0), gl.NEAREST_MIPMAP_NEAREST},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapMinFilter(tt.minFilter, tt.mipmapFilter)
			if got != tt.want {
				t.Errorf("mapMinFilter(%v, %v) = %v, want %v", tt.minFilter, tt.mipmapFilter, got, tt.want)
			}
		})
	}
}

func TestMapAddressMode(t *testing.T) {
	tests := []struct {
		name string
		mode types.AddressMode
		want int32
	}{
		{"Repeat", types.AddressModeRepeat, gl.REPEAT},
		{"MirrorRepeat", types.AddressModeMirrorRepeat, gl.MIRRORED_REPEAT},
		{"ClampToEdge", types.AddressModeClampToEdge, gl.CLAMP_TO_EDGE},
		{"Undefined defaults to ClampToEdge", types.AddressModeUndefined, gl.CLAMP_TO_EDGE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapAddressMode(tt.mode)
			if got != tt.want {
				t.Errorf("mapAddressMode(%v) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestMapCompareFunction(t *testing.T) {
	tests := []struct {
		name string
		fn   types.CompareFunction
		want int32
	}{
		{"Never", types.CompareFunctionNever, gl.NEVER},
		{"Less", types.CompareFunctionLess, gl.LESS},
		{"Equal", types.CompareFunctionEqual, gl.EQUAL},
		{"LessEqual", types.CompareFunctionLessEqual, gl.LEQUAL},
		{"Greater", types.CompareFunctionGreater, gl.GREATER},
		{"NotEqual", types.CompareFunctionNotEqual, gl.NOTEQUAL},
		{"GreaterEqual", types.CompareFunctionGreaterEqual, gl.GEQUAL},
		{"Always", types.CompareFunctionAlways, gl.ALWAYS},
		{"Undefined defaults to Always", types.CompareFunctionUndefined, gl.ALWAYS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCompareFunction(tt.fn)
			if got != tt.want {
				t.Errorf("mapCompareFunction(%v) = %v, want %v", tt.fn, got, tt.want)
			}
		})
	}
}

// TestSamplerBindMap verifies the SamplerBindMap lookup logic used in SetBindGroupCommand.
// Given samplerBindMap[texUnit] = samplerGLBinding, searching for glBinding should return texUnit.
func TestSamplerBindMap(t *testing.T) {
	var bindMap [maxTextureSlots]int8
	for i := range bindMap {
		bindMap[i] = -1 // no sampler (default)
	}
	// Texture unit 1 paired with sampler glBinding 2.
	bindMap[1] = 2
	// Texture unit 5 paired with sampler glBinding 3.
	bindMap[5] = 3

	tests := []struct {
		name        string
		glBinding   int8
		wantTexUnit int // -1 if not found
	}{
		{"sampler 2 maps to texUnit 1", 2, 1},
		{"sampler 3 maps to texUnit 5", 3, 5},
		{"sampler 0 not mapped", 0, -1},
		{"sampler 7 not mapped", 7, -1},
		{"sampler 1 not mapped (only 2 and 3 are)", 1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Replicate the lookup from SetBindGroupCommand.Execute (command.go:952).
			found := -1
			for texUnit := range bindMap {
				if bindMap[texUnit] == tt.glBinding {
					found = texUnit
					break
				}
			}
			if found != tt.wantTexUnit {
				t.Errorf("lookup glBinding=%d: got texUnit=%d, want %d", tt.glBinding, found, tt.wantTexUnit)
			}
		})
	}
}

func TestIsNonFilterableFormat(t *testing.T) {
	tests := []struct {
		name string
		fmt  types.TextureFormat
		want bool
	}{
		// Integer formats are non-filterable.
		{"R8Uint", types.TextureFormatR8Uint, true},
		{"RGBA8Sint", types.TextureFormatRGBA8Sint, true},
		{"R32Uint", types.TextureFormatR32Uint, true},
		// 32-bit float formats are non-filterable.
		{"R32Float", types.TextureFormatR32Float, true},
		{"RGBA32Float", types.TextureFormatRGBA32Float, true},
		// Depth32Float and Stencil8 are non-filterable.
		{"Depth32Float", types.TextureFormatDepth32Float, true},
		{"Stencil8", types.TextureFormatStencil8, true},
		// Standard filterable formats.
		{"RGBA8Unorm", types.TextureFormatRGBA8Unorm, false},
		{"R16Float", types.TextureFormatR16Float, false},
		{"RGBA16Float", types.TextureFormatRGBA16Float, false},
		{"Depth24Plus", types.TextureFormatDepth24Plus, false},
		{"BGRA8Unorm", types.TextureFormatBGRA8Unorm, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNonFilterableFormat(tt.fmt)
			if got != tt.want {
				t.Errorf("isNonFilterableFormat(%v) = %v, want %v", tt.fmt, got, tt.want)
			}
		})
	}
}
