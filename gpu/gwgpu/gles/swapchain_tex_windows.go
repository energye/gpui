//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build windows && !(js && wasm)

package gles

import (
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// texImageSwapchainNull uploads empty swapchain color storage. Only the
// pixels spelling differs per platform (nil here vs uintptr 0 on Linux);
// the surrounding bind/param/error flow lives once in surface.go.
func texImageSwapchainNull(glCtx *gl.Context, internalFormat, format, dataType uint32, width, height int32) {
	glCtx.TexImage2D(gl.TEXTURE_2D, 0, int32(internalFormat), width, height, 0, format, dataType, nil)
}
