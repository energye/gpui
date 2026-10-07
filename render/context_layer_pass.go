//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	gpucontext "github.com/energye/gpui/gpu/context"
)

// acquireEffectPublishView allocates a pooled TextureBinding RT for F14 effect
// FlushGPU publish. Caller must pass release to attachFilterGPUResult (or call it).
func (c *Context) acquireEffectPublishView() (view gpucontext.TextureView, w, h int, release func(), ok bool) {
	if c == nil || c.pixmap == nil {
		return gpucontext.TextureView{}, 0, 0, nil, false
	}
	w, h = c.pixmap.Width(), c.pixmap.Height()
	if w <= 0 || h <= 0 {
		return gpucontext.TextureView{}, 0, 0, nil, false
	}
	c.ensureGPUCtx()
	v, rel := c.CreateOffscreenTexture(w, h)
	if v.IsNil() || rel == nil {
		return gpucontext.TextureView{}, 0, 0, nil, false
	}
	return v, w, h, rel, true
}

// Push saves the current state (transform, paint, clip, and mask).
// Push saves the current state (transform, paint, clip, and mask).
func (c *Context) Push() {

	c.stack = append(c.stack, c.matrix)

	// Save current clip stack depth
	depth := 0
	if c.clipStack != nil {
		depth = c.clipStack.Depth()
	}
	c.clipStackDepth = append(c.clipStackDepth, depth)

	// Save current mask (clone if exists)
	var maskCopy *Mask
	if c.mask != nil {
		maskCopy = c.mask.Clone()
	}
	c.maskStack = append(c.maskStack, maskCopy)

	// Save current anti-aliasing state
	c.antiAliasStack = append(c.antiAliasStack, c.antiAlias)
}

// Pop restores the last saved state.
// Pop restores the last saved state.
// Pop restores the last saved state.
// Pop restores the last saved state.
func (c *Context) Pop() {

	if len(c.stack) == 0 {
		return
	}

	// Restore transform matrix
	c.matrix = c.stack[len(c.stack)-1]
	c.stack = c.stack[:len(c.stack)-1]

	// Restore clip stack depth
	if len(c.clipStackDepth) > 0 {
		targetDepth := c.clipStackDepth[len(c.clipStackDepth)-1]
		c.clipStackDepth = c.clipStackDepth[:len(c.clipStackDepth)-1]

		// Pop clip stack entries until we reach the target depth
		if c.clipStack != nil {
			changed := false
			for c.clipStack.Depth() > targetDepth {
				c.clipStack.Pop()
				changed = true
			}
			// Clear GPU clip path if all path clips were popped.
			if c.gpuClipPath != nil && c.clipStack.IsRRectOnly() {
				c.gpuClipPath = nil
			}
			if changed {
				c.bumpClipMaskGPUGen()
			}
		}
	}

	// Restore mask
	if len(c.maskStack) > 0 {
		c.mask = c.maskStack[len(c.maskStack)-1]
		c.maskStack = c.maskStack[:len(c.maskStack)-1]
	}

	// Restore anti-aliasing state
	if len(c.antiAliasStack) > 0 {
		c.antiAlias = c.antiAliasStack[len(c.antiAliasStack)-1]
		c.antiAliasStack = c.antiAliasStack[:len(c.antiAliasStack)-1]
	}
}

// Identity resets the user transformation matrix to the identity matrix.
// Device scale is applied separately at rendering boundaries (not in the CTM),
// so Identity() always resets to a pure identity matrix regardless of scale.
// SetSharedEncoder sets a shared command encoder for single-command-buffer
// frames (ADR-017, Flutter Impeller pattern). When set, FlushGPU/FlushGPUWithView
// record render passes into this encoder instead of creating their own and
// submitting. The caller is responsible for encoder.Finish() + queue.Submit().
//
// Pass a zero-value CommandEncoder (IsNil() == true) to restore normal
// per-context submit behavior.
// Identity resets the user transformation matrix to the identity matrix.
// Device scale is applied separately at rendering boundaries (not in the CTM),
// so Identity() always resets to a pure identity matrix regardless of scale.
// SetSharedEncoder sets a shared command encoder for single-command-buffer
// frames (ADR-017, Flutter Impeller pattern). When set, FlushGPU/FlushGPUWithView
// record render passes into this encoder instead of creating their own and
// submitting. The caller is responsible for encoder.Finish() + queue.Submit().
//
// Pass a zero-value CommandEncoder (IsNil() == true) to restore normal
// per-context submit behavior.
func (c *Context) SetSharedEncoder(encoder gpucontext.CommandEncoder) {
	if rc := c.gpuCtxOps(); rc != nil {
		type encoderSetter interface {
			SetSharedEncoder(encoder gpucontext.CommandEncoder)
		}
		if es, ok := rc.(encoderSetter); ok {
			es.SetSharedEncoder(encoder)
		}
	}
}

// CreateSharedEncoder creates a command encoder for single-command-buffer
// frames (ADR-017). Multiple render.Contexts record render passes into this
// encoder via SetSharedEncoder. Call SubmitSharedEncoder after all contexts
// have flushed to submit in one GPU call.
// Returns a zero-value CommandEncoder (IsNil() == true) if GPU is not available.
// CreateSharedEncoder creates a command encoder for single-command-buffer
// frames (ADR-017). Multiple render.Contexts record render passes into this
// encoder via SetSharedEncoder. Call SubmitSharedEncoder after all contexts
// have flushed to submit in one GPU call.
// Returns a zero-value CommandEncoder (IsNil() == true) if GPU is not available.
// CreateSharedEncoder creates a command encoder for single-command-buffer
// frames (ADR-017). Multiple render.Contexts record render passes into this
// encoder via SetSharedEncoder. Call SubmitSharedEncoder after all contexts
// have flushed to submit in one GPU call.
// Returns a zero-value CommandEncoder (IsNil() == true) if GPU is not available.
// CreateSharedEncoder creates a command encoder for single-command-buffer
// frames (ADR-017). Multiple render.Contexts record render passes into this
// encoder via SetSharedEncoder. Call SubmitSharedEncoder after all contexts
// have flushed to submit in one GPU call.
// Returns a zero-value CommandEncoder (IsNil() == true) if GPU is not available.
func (c *Context) CreateSharedEncoder() gpucontext.CommandEncoder {
	rc := c.gpuCtxOps()
	if rc == nil {
		return gpucontext.CommandEncoder{}
	}
	type encoderCreator interface {
		CreateEncoder() gpucontext.CommandEncoder
	}
	if ec, ok := rc.(encoderCreator); ok {
		return ec.CreateEncoder()
	}
	return gpucontext.CommandEncoder{}
}

// SubmitSharedEncoder finishes the shared encoder and submits the resulting
// command buffer to the GPU. Call after all contexts have flushed their
// render passes into the encoder. Returns the command buffer submission
// index for fence tracking, or error.
// SubmitSharedEncoder finishes the shared encoder and submits the resulting
// command buffer to the GPU. Call after all contexts have flushed their
// render passes into the encoder. Returns the command buffer submission
// index for fence tracking, or error.
// SubmitSharedEncoder finishes the shared encoder and submits the resulting
// command buffer to the GPU. Call after all contexts have flushed their
// render passes into the encoder. Returns the command buffer submission
// index for fence tracking, or error.
// SubmitSharedEncoder finishes the shared encoder and submits the resulting
// command buffer to the GPU. Call after all contexts have flushed their
// render passes into the encoder. Returns the command buffer submission
// index for fence tracking, or error.
func (c *Context) SubmitSharedEncoder(encoder gpucontext.CommandEncoder) error {
	rc := c.gpuCtxOps()
	if rc == nil {
		return nil
	}
	type encoderSubmitter interface {
		SubmitEncoder(encoder gpucontext.CommandEncoder) error
	}
	if es, ok := rc.(encoderSubmitter); ok {
		return es.SubmitEncoder(encoder)
	}
	return nil
}

// BeginGPUFrame resets per-context GPU frame state so the next render pass
// uses LoadOpClear. Call this on persistent contexts before re-rendering
// to the same view — without it, frameRendered=true from the previous frame
// causes LoadOpLoad, preserving stale content.
//
// Not needed for one-shot contexts (NewContext + Close per frame).
// Not needed when the view changes between frames (auto-reset on view change).
// BeginOffscreenPass isolates an offscreen recording sub-pass from the main
// frame stream: while active, all queued commands / clip timeline / LoadOp
// tracking belong exclusively to the sub-pass target; the main pass state is
// suspended and restored by the returned func. Callers MUST invoke the
// restore func (defer) after flushing the sub-pass into its own view.
//
// This is what makes retained texture re-records safe mid-frame (Skia
// GrRecordingContext / Flutter EntityPass pass-ownership semantics). With no
// GPU ops interface it returns a no-op func.
// BeginGPUFrame resets per-context GPU frame state so the next render pass
// uses LoadOpClear. Call this on persistent contexts before re-rendering
// to the same view — without it, frameRendered=true from the previous frame
// causes LoadOpLoad, preserving stale content.
//
// Not needed for one-shot contexts (NewContext + Close per frame).
// Not needed when the view changes between frames (auto-reset on view change).
// BeginOffscreenPass isolates an offscreen recording sub-pass from the main
// frame stream: while active, all queued commands / clip timeline / LoadOp
// tracking belong exclusively to the sub-pass target; the main pass state is
// suspended and restored by the returned func. Callers MUST invoke the
// restore func (defer) after flushing the sub-pass into its own view.
//
// This is what makes retained texture re-records safe mid-frame (Skia
// GrRecordingContext / Flutter EntityPass pass-ownership semantics). With no
// GPU ops interface it returns a no-op func.
func (c *Context) BeginOffscreenPass() func() {
	rc := c.gpuCtxOps()
	bo, ok := rc.(interface {
		BeginOffscreenPass(GPURenderTarget) func()
	})
	if !ok {
		return func() {}
	}
	// The sub-pass records the FULL picture into its own view; the composite
	// applies clip geometry when blitting. The caller's canvas clip must not
	// leak into the pass in ANY form:
	//   - rc scissor/RRect segments reference surface pixels far outside the
	//     small offscreen viewport (out-of-bounds scissor kills the pass);
	//   - doFill's setGPUClipRect re-fires on every fill while the Context
	//     clipStack is active, re-queueing those segments onto the sub-pass
	//     stream mid-record.
	// Suspend the canvas clip for the whole pass and restore it after. The
	// clipStack itself is untouched (the caller's clip must survive);
	// offscreenPassDepth gates setGPUClipRect / applyClipToPaint instead.
	restoreClip := c.resetGPUClipForPass()
	restore := bo.BeginOffscreenPass(c.gpuRenderTarget())
	return func() {
		restore()
		if restoreClip != nil {
			restoreClip()
		}
	}
}

// resetGPUClipForPass suspends the context's active canvas clip for the
// duration of an offscreen sub-pass: offscreenPassDepth goes true so doFill's
// setGPUClipRect / applyClipToPaint stay inert, and the canvas clip path is
// parked aside. The clipStack itself is untouched — the caller's clip must
// survive verbatim. GPU clip state and the scissor timeline stay owned by
// Begin/EndOffscreenPass (suspend-and-restore): recording segments here would
// attribute later main-stream draws to this clip, so this func records none.
// Returns nil when no clip is active.
// offscreenPassSuspended reports whether an offscreen recording sub-pass is
// active and the surface clip must not leak into it.
// resetGPUClipForPass suspends the context's active canvas clip for the
// duration of an offscreen sub-pass: offscreenPassDepth goes true so doFill's
// setGPUClipRect / applyClipToPaint stay inert, and the canvas clip path is
// parked aside. The clipStack itself is untouched — the caller's clip must
// survive verbatim. GPU clip state and the scissor timeline stay owned by
// Begin/EndOffscreenPass (suspend-and-restore): recording segments here would
// attribute later main-stream draws to this clip, so this func records none.
// Returns nil when no clip is active.
// offscreenPassSuspended reports whether an offscreen recording sub-pass is
// active and the surface clip must not leak into it.
func (c *Context) offscreenPassSuspended() bool {
	return c.offscreenPassDepth > 0
}

// FlushGPUWithViewDamage flushes pending GPU operations with damage-aware
// optimization. When damageRect is non-empty, the compositor uses LoadOpLoad
// (preserves previous frame) and scissor-clips to the dirty region — only
// the damaged pixels are re-composited. When damageRect is empty, behaves
// identically to FlushGPUWithView (full compositor pass).
//
// IMPORTANT: damage-aware rendering (LoadOpLoad + scissor) works only on the
// blit-only compositor path (frames with only DrawGPUTextureBase/DrawGPUTexture
// calls, no vector shapes). When the frame contains Fill/Stroke operations,
// the MSAA render path is used which always does LoadOpClear — damageRect is
// ignored and a warning is logged. This matches enterprise practice: Chrome,
//
// This enables sub-region compositing: a 48×48 spinner updates only 9KB
// instead of the full surface (8MB at 1080p). See ADR-016 Phase 2.
