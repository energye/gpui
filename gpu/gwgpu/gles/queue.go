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
	"fmt"
	"image"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/gwgpu/gles/wgl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Queue implements hal.Queue for OpenGL.
// Holds a shared *AdapterContext (owned by Instance).
type Queue struct {
	ctx             *AdapterContext
	submissionIndex uint64
	fence           *Fence // signaled at each submit for GPU completion tracking
	batch           writeBatch
}

// Submit submits command buffers to the GPU.
// One Lock for the whole submit: staged uploads flush first, then all GL
// commands run under a per-submit state cache that drops repeated setup.
func (q *Queue) Submit(commandBuffers ...hal.CommandBuffer) (uint64, error) {
	glCtx := q.ctx.Lock()
	defer q.ctx.Unlock()
	st := newGLExecState()
	if list := q.batch.drain(); len(list) > 0 {
		flushWriteBatch(glCtx, st, list)
	}

	for _, cb := range commandBuffers {
		cmdBuf, ok := cb.(*CommandBuffer)
		if !ok {
			return 0, fmt.Errorf("gles: invalid command buffer type")
		}

		for i, cmd := range cmdBuf.commands {
			cmd.Execute(glCtx, st)
			if glErr := glCtx.GetError(); glErr != 0 {
				hal.Logger().Warn("gles: GL error after command", "error", fmt.Sprintf("0x%x", glErr), "index", i, "command", fmt.Sprintf("%T", cmd))
			}
		}
	}

	q.submissionIndex++

	// FenceSync must be inserted BEFORE Flush so the sync object tracks the
	// commands being flushed. Flushing first would leave the fence un-flushed.
	if q.fence != nil {
		q.fence.Maintain()
		if err := q.fence.Signal(q.submissionIndex); err != nil {
			return 0, err
		}
	}

	glCtx.Flush()

	return q.submissionIndex, nil
}

// Poll returns the highest submission index known to be completed.
// Polls pending GL sync objects via glGetSynciv (non-blocking, no flush).
// Safe because Submit() always flushes after inserting the fence — the fence
// is guaranteed to be in the GPU command queue by the time we poll it.
// Maintenance (cleanup of completed sync objects) happens in Submit(), not here
// Backend divergence: webgpu Queue.Poll always returns 0,
// gles returns Fence.GetLatest (or submissionIndex when queueless),
// metal returns the GPU-callback-driven completedIndex.
func (q *Queue) Poll() uint64 {
	if q.fence != nil {
		// Fence poll issues GL queries: bind first so they run on a
		// current context (see Fence contract in resource.go). On bind
		// failure fall back to the cached value — same as the
		// no-fence-sync path, no GL touched.
		if _, err := q.ctx.TryLock(); err != nil {
			return q.fence.lastCompleted.Load()
		}
		defer q.ctx.Unlock()
		return q.fence.GetLatest()
	}
	return q.submissionIndex
}

// LastSubmissionIndex returns the most recent submission index.
func (q *Queue) LastSubmissionIndex() uint64 {
	return q.submissionIndex
}

// WriteBuffer stages data for upload. No GL runs here (no MakeCurrent):
// the payload flushes with the next Submit/Present under that Lock.
func (q *Queue) WriteBuffer(buffer hal.Buffer, offset uint64, data []byte) error {
	buf, ok := buffer.(*Buffer)
	if !ok {
		return fmt.Errorf("gles: WriteBuffer: invalid buffer type")
	}
	if len(data) == 0 {
		return nil
	}
	if q.batch.stage(buf, offset, data) {
		// Flush now: the batch just crossed the size threshold. Bind
		// failure keeps the payload staged (no drain) for the next
		// Submit/Present instead of dropping it.
		glCtx, err := q.ctx.TryLock()
		if err != nil {
			return fmt.Errorf("gles: WriteBuffer flush bind failed: %w", err)
		}
		defer q.ctx.Unlock()
		flushWriteBatch(glCtx, newGLExecState(), q.batch.drain())
	}
	return nil
}

// WriteTexture writes data to a texture immediately.
func (q *Queue) WriteTexture(dst *hal.ImageCopyTexture, data []byte, layout *hal.ImageDataLayout, size *hal.Extent3D) error {
	tex, ok := dst.Texture.(*Texture)
	if !ok {
		return fmt.Errorf("gles: invalid texture type for WriteTexture")
	}

	glCtx := q.ctx.Lock()
	defer q.ctx.Unlock()

	// Staged buffer uploads predate this call: flush first to keep GL order.
	if list := q.batch.drain(); len(list) > 0 {
		flushWriteBatch(glCtx, newGLExecState(), list)
	}

	_, format, dataType := textureFormatToGL(tex.format)

	glCtx.BindTexture(tex.target, tex.id)

	if tex.target == gl.TEXTURE_2D {
		if tex.format == gputypes.TextureFormatR8Unorm {
			glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		}
		// Honor Origin: dirty-rect uploads (glyph atlas pages) arrive with a
		// non-zero origin — writing at (0,0) misplaces ink into wrong cells.
		// Windows 真机待复验.
		glCtx.TexSubImage2D(tex.target, int32(dst.MipLevel),
			int32(dst.Origin.X), int32(dst.Origin.Y),
			int32(size.Width), int32(size.Height), format, dataType,
			unsafe.Pointer(&data[0]))
		if tex.format == gputypes.TextureFormatR8Unorm {
			glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
		}
	}

	glCtx.BindTexture(tex.target, 0)

	hal.Logger().Debug("gles: texture written",
		"format", tex.format,
		"width", size.Width,
		"height", size.Height,
	)

	return nil
}

// Present presents a surface texture to the screen.
//
// Makes the GL context current on the user window's DC (via LockForDC),
// blits the swapchain FBO to the default framebuffer with Y-flip, then
// SwapBuffers.
//
// damageRects is accepted but ignored on Windows WGL — WGL has no
// damage-aware swap API.
func (q *Queue) Present(surface hal.Surface, tex hal.SurfaceTexture, _ []image.Rectangle) error {
	surf, ok := surface.(*Surface)
	if !ok {
		return fmt.Errorf("gles: invalid surface type")
	}

	hdc := wgl.GetDC(surf.hwnd)
	if hdc == 0 {
		return fmt.Errorf("gles: GetDC failed for hwnd 0x%x", surf.hwnd)
	}
	defer wgl.ReleaseDC(surf.hwnd, hdc)

	// MakeCurrent to user window DC for presentation.
	glCtx := q.ctx.LockForDC(hdc)
	defer q.ctx.Unlock()

	if list := q.batch.drain(); len(list) > 0 {
		flushWriteBatch(glCtx, newGLExecState(), list)
	}
	surf.blitSwapchainToDefaultWith(glCtx)

	if err := wgl.SwapBuffers(hdc); err != nil {
		return err
	}
	surf.DiscardTexture(tex)

	return nil
}

// GetTimestampPeriod returns the timestamp period in nanoseconds.
func (q *Queue) GetTimestampPeriod() float32 {
	// OpenGL doesn't have a standard way to query this
	// Return 1.0 to indicate nanoseconds
	return 1.0
}

// SupportsCommandBufferCopies returns false for GLES.
// GLES uses direct GL calls (glBufferSubData, glTexSubImage2D) for writes,
// not command buffer copy operations.
func (q *Queue) SupportsCommandBufferCopies() bool {
	return false
}

// SetSwapchainSuppressed is a no-op on GLES.
// GLES uses eglSwapBuffers for presentation, which is not affected by command
// submission ordering.
func (q *Queue) SetSwapchainSuppressed(_ bool) {}
