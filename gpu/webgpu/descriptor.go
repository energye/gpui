//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
)

// SurfaceConfiguration configures surface presentation.
type SurfaceConfiguration = hal.SurfaceConfiguration

// BindGroupDescriptor describes a bind group.
type BindGroupDescriptor struct {
	Label   string
	Layout  *BindGroupLayout
	Entries []BindGroupEntry
}

// BindGroupEntry describes a single resource binding in a bind group.
type BindGroupEntry struct {
	Binding     uint32
	Buffer      *Buffer
	Offset      uint64
	Size        uint64
	Sampler     *Sampler
	TextureView *TextureView
}

// TextureRange specifies a range of texture subresources.
type TextureRange = hal.TextureRange
