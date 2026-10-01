//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// Ledger handles must be unique across GL object kinds: buffer, texture,
// sampler, program, query and framebuffer ids each live in their own GL
// number line, so raw ids collide (texture 1 vs buffer 1). The swapchain
// FBO folds id+height into one tagged slot (see vramHandleForFBO); every
// other kind gets its own tag band below so the shared hal ledger never
// merges two live objects into one slot (which under-counted and let a 1GB
// card run into driver 0x502 failures during resize storms).
func vramTextureHandle(id uint32) uintptr {
	if id == 0 {
		return 0
	}
	return (uintptr(id) << 1) | 1
}

// vramBufferHandle tags a GL buffer id into a disjoint ledger slot.
func vramBufferHandle(id uint32) uintptr {
	return (uintptr(id) << 1)
}

// glAllocOOM reports whether a GL error from a storage allocation
// (TexImage, RenderbufferStorage) means the driver could not back the
// request. OUT_OF_MEMORY is explicit; INVALID_OPERATION from TexImage /
// RenderbufferStorage with valid descriptors is how an exhausted 1GB
// driver reports pressure (steady 1200x700 succeeds, storm sizes fail
// with 0x502 in pelican logs). Both must flow into the OOM retry chain.
func glAllocOOM(glErr uint32) bool {
	return glErr == gl.OUT_OF_MEMORY || glErr == gl.INVALID_OPERATION
}

// glAllocErr shapes an allocation failure so render.IsGPUOutOfMemory and
// hal VramCheck match it ("out of memory"): the GL layer is the only
// place that sees the raw 0x502/0x505, upper layers only see the string.
func glAllocErr(op string, glErr uint32, detail string) error {
	if glAllocOOM(glErr) {
		return fmt.Errorf("gles: %s failed: GL error 0x%x (%s): out of memory", op, glErr, detail)
	}
	return fmt.Errorf("gles: %s failed: GL error 0x%x (%s)", op, glErr, detail)
}

// vramHandleForFBO maps a GL FBO id into the ledger's handle space.
func vramHandleForFBO(fbo, height uint32) uintptr {
	const tag = uintptr(1) << 63
	return uintptr(fbo)<<32 | uintptr(height) | tag
}

// One tag band per remaining GL kind (top 4 bits); ids ride the low bits.
// FBO lives in its own top-bit band above, texture/buffer keep their
// historical low-bit tags.
const (
	vramTagSampler = uintptr(2) << 60
	vramTagProgram = uintptr(3) << 60
	vramTagShader  = uintptr(4) << 60
)

// vramSamplerHandle tags a GL sampler id into a disjoint ledger slot.
func vramSamplerHandle(id uint32) uintptr {
	return uintptr(id) | vramTagSampler
}

// vramProgramHandle tags a GL program id into a disjoint ledger slot.
func vramProgramHandle(id uint32) uintptr {
	return uintptr(id) | vramTagProgram
}

// vramShaderHandle tags a GL shader id into a disjoint ledger slot.
func vramShaderHandle(id uint32) uintptr {
	return uintptr(id) | vramTagShader
}
