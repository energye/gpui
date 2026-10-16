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
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
)

// Target coordinate adjustment.
//
// The GLSL backend resolves the coordinate-space flip from a uniform block
// instead of a baked -1.0 (glsl.WriterFlagTargetCoordinateAdjust), so one
// compiled shader set serves both kinds of target:
//
//	render target (texture or surface FBO)  vec4(-1,  1, 0, 0)
//	window default framebuffer              vec4( 1, -1, H, 0)
//
// A render target is drawn with the Y-negative vertex flip, which puts the
// logical top row at the framebuffer's bottom row and makes the fragment
// @builtin(position) read the logical Y that fragment math (clip coverage,
// mask UVs) expects. A window default framebuffer has GL's bottom-left origin
// at the bottom of the display, so there the geometry is drawn mirrored (Y sign
// +1) and the fragment Y read is mirrored back (scale -1, offset H), which
// yields the same logical Y as the render-target case.
//
// The block is bound once per GL context to the highest uniform buffer slot.
// Application bindings are handed out sequentially from 0 per class
// (computeBindingMap), so the top slot is out of their range by construction
// and stays reserved for this block. It is assigned after linking instead of
// with a layout(binding = N) qualifier: the GLSL 3.30 path has no explicit
// block bindings, and the slot is a context property rather than a shader one.
const targetAdjustBlockName = "GpuiTargetAdjust"

// targetAdjustSize is the std140 size of the block: one vec4.
const targetAdjustSize = 16

// targetAdjustRenderTarget is the adjustment for a render target whose bottom
// row is the logical top row — every offscreen texture and surface FBO. The Y
// sign is -1.0, so the vertex epilogue stays bit-identical to the literal
// negation it replaced, and the fragment read is the identity on gl_FragCoord.
var targetAdjustRenderTarget = [4]float32{-1, 1, 0, 0}

// targetAdjustWindowValue returns the adjustment for a window default
// framebuffer of the given height.
func targetAdjustWindowValue(height float32) [4]float32 {
	return [4]float32{1, -1, height, 0}
}

// targetAdjustState is the per-GL-context target adjustment block. Every field
// is touched with the AdapterContext lock held (AdapterContext.mu), like the
// rest of this package's GL state; it is created on the pinned render thread.
type targetAdjustState struct {
	ubo      uint32
	slot     uint32
	uploaded [4]float32
	ready    bool // uploaded is the value currently in ubo
}

// ensureTargetAdjust creates the target adjustment buffer on first use and
// returns the uniform slot reserved for it (0 when the driver refuses the
// buffer). The caller holds the context lock.
func (c *AdapterContext) ensureTargetAdjust() uint32 {
	if c == nil || c.gl == nil {
		return 0
	}
	if c.targetAdjust.ubo != 0 {
		return c.targetAdjust.slot
	}
	glCtx := c.gl
	var maxBindings int32
	glCtx.GetIntegerv(gl.MAX_UNIFORM_BUFFER_BINDINGS, &maxBindings)
	if maxBindings < 2 {
		hal.Logger().Error("gles: no uniform buffer slot for the target adjustment block",
			"maxBindings", maxBindings)
		return 0
	}
	ubo := glCtx.GenBuffers(1)
	if ubo == 0 {
		hal.Logger().Error("gles: target adjustment buffer creation failed")
		return 0
	}
	c.targetAdjust = targetAdjustState{ubo: ubo, slot: uint32(maxBindings - 1)}
	allocTargetAdjustStore(glCtx, ubo)
	c.setTargetAdjust(glCtx, targetAdjustRenderTarget)
	return c.targetAdjust.slot
}

// setTargetAdjust binds the target adjustment buffer to its reserved slot and
// uploads value when it differs from the current contents. The caller holds the
// context lock. The length-16 buffer belongs to the GL context, so it is not
// released separately (twin of every other per-context object here).
func (c *AdapterContext) setTargetAdjust(glCtx *gl.Context, value [4]float32) {
	if c == nil || glCtx == nil {
		return
	}
	st := &c.targetAdjust
	if st.ubo == 0 {
		return
	}
	// Bind the slot before the value: an unbound slot reads as zero, which
	// collapses the vertex Y sign instead of falling back to the last value.
	glCtx.BindBufferBase(gl.UNIFORM_BUFFER, st.slot, st.ubo)
	if st.ready && st.uploaded == value {
		return
	}
	glCtx.BindBuffer(gl.UNIFORM_BUFFER, st.ubo)
	glCtx.BufferSubData(gl.UNIFORM_BUFFER, 0, targetAdjustSize, unsafe.Pointer(&value[0]))
	st.uploaded = value
	st.ready = true
}

// assignTargetAdjustBinding points the target adjustment block at its reserved
// slot for a linked program. Programs without the block (compute pipelines, or
// shader sets compiled before the flag existed) are left untouched.
func assignTargetAdjustBinding(glCtx *gl.Context, program, slot uint32) {
	if slot == 0 || glCtx == nil {
		return
	}
	index := glCtx.GetUniformBlockIndex(program, targetAdjustBlockName)
	if index == 0xFFFFFFFF { // GL_INVALID_INDEX: block optimised out or absent
		return
	}
	glCtx.UniformBlockBinding(program, index, slot)
}
