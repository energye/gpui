//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
)

// SurfaceConfiguration configures surface presentation.
type SurfaceConfiguration = hal.SurfaceConfiguration

// BindGroupDescriptor/Entry live in hal since 片5 (hal 以 webgpu 扁平形为准):
// use hal.BindGroupDescriptor/hal.BindGroupEntry directly.

// TextureRange specifies a range of texture subresources.
type TextureRange = hal.TextureRange
