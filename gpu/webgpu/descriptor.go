//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"github.com/energye/gpui/gpu/hal"
)

// SurfaceConfiguration configures surface presentation.
type SurfaceConfiguration = hal.SurfaceConfiguration

// BindGroupDescriptor/Entry live in hal since 片5:
// use hal.BindGroupDescriptor/hal.BindGroupEntry directly.

// TextureRange specifies a range of texture subresources.
type TextureRange = hal.TextureRange
