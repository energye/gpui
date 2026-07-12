//go:build !rust && !(js && wasm)

// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

package webgpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

func TestBufferReadUsage(t *testing.T) {
	tests := []struct {
		name  string
		usage types.BufferUsage
		want  types.BufferUsage
	}{
		{
			"Vertex|CopyDst extracts Vertex",
			types.BufferUsageVertex | types.BufferUsageCopyDst,
			types.BufferUsageVertex,
		},
		{
			"Index|CopyDst extracts Index",
			types.BufferUsageIndex | types.BufferUsageCopyDst,
			types.BufferUsageIndex,
		},
		{
			"Uniform|CopyDst extracts Uniform",
			types.BufferUsageUniform | types.BufferUsageCopyDst,
			types.BufferUsageUniform,
		},
		{
			"Indirect|CopyDst extracts Indirect",
			types.BufferUsageIndirect | types.BufferUsageCopyDst,
			types.BufferUsageIndirect,
		},
		{
			"Storage only returns 0",
			types.BufferUsageStorage,
			0,
		},
		{
			"CopyDst only returns 0",
			types.BufferUsageCopyDst,
			0,
		},
		{
			"CopySrc only returns 0",
			types.BufferUsageCopySrc,
			0,
		},
		{
			"Vertex|Index|CopyDst extracts both",
			types.BufferUsageVertex | types.BufferUsageIndex | types.BufferUsageCopyDst,
			types.BufferUsageVertex | types.BufferUsageIndex,
		},
		{
			"MapRead|MapWrite returns 0",
			types.BufferUsageMapRead | types.BufferUsageMapWrite,
			0,
		},
		{
			"None returns 0",
			types.BufferUsageNone,
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bufferReadUsage(tt.usage)
			if got != tt.want {
				t.Errorf("bufferReadUsage(%#x) = %#x, want %#x", tt.usage, got, tt.want)
			}
		})
	}
}

func TestAlignUp(t *testing.T) {
	tests := []struct {
		name      string
		n         uint32
		alignment uint32
		want      uint32
	}{
		{"100 aligned to 256", 100, 256, 256},
		{"256 aligned to 256", 256, 256, 256},
		{"0 aligned to 256", 0, 256, 0},
		{"1 aligned to 1", 1, 1, 1},
		{"257 aligned to 256", 257, 256, 512},
		{"1 aligned to 256", 1, 256, 256},
		{"255 aligned to 256", 255, 256, 256},
		{"512 aligned to 256", 512, 256, 512},
		{"0 aligned to 0 (degenerate)", 0, 0, 0},
		{"100 aligned to 0 (degenerate)", 100, 0, 100},
		{"3 aligned to 4", 3, 4, 4},
		{"4 aligned to 4", 4, 4, 4},
		{"5 aligned to 4", 5, 4, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alignUp(tt.n, tt.alignment)
			if got != tt.want {
				t.Errorf("alignUp(%d, %d) = %d, want %d", tt.n, tt.alignment, got, tt.want)
			}
		})
	}
}
