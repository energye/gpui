//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Queue implements hal.Queue for OpenGL on Linux.
// Holds a shared *AdapterContext (owned by Instance or Surface).
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

		// Execute recorded commands with GL error checking.
		// Per-command GetError is a full FFI round trip each: only pay it
		// in debug (hal debug logger on); otherwise one check per command
		// buffer below. Error detection is preserved, per-command index is
		// debug-only detail.
		cmdDbg := hal.Logger().Enabled(context.Background(), slog.LevelDebug)
		for i, cmd := range cmdBuf.commands {
			cmd.Execute(glCtx, st)
			if cmdDbg {
				if glErr := glCtx.GetError(); glErr != 0 {
					detail := fmt.Sprintf("%T", cmd)
					if vaoCmd, ok := cmd.(*BindVAOCommand); ok {
						detail = fmt.Sprintf("%T{vao=%d}", cmd, vaoCmd.vao)
					}
					hal.Logger().Warn("gles: GL error after command", "error", fmt.Sprintf("0x%x", glErr), "index", i, "command", detail)
				}
			}
			// Return hot-path commands to their pools now that Execute has
			// consumed them; detached from the buffer so Destroy can't
			// double-release. Cold-path types are dropped for the GC.
			releasePooledCommand(cmd)
			cmdBuf.commands[i] = nil
		}
		// Recycle the command slice backing (see commandListPool).
		recycleCommandList(cmdBuf.commands)
		cmdBuf.commands = nil
		if !cmdDbg {
			if glErr := glCtx.GetError(); glErr != 0 {
				hal.Logger().Warn("gles: GL error in submit", "error", fmt.Sprintf("0x%x", glErr))
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
		// Padded sources arrive with BytesPerRow > tight width; tell GL
		// the row length in pixels (UNPACK_ROW_LENGTH) so TexSubImage2D
		// reads the right row starts.
		if layout != nil && layout.BytesPerRow > 0 {
			pixelBytes := uint32(4)
			switch tex.format {
			case gputypes.TextureFormatR8Unorm:
				pixelBytes = 1
			}
			if rowLen := layout.BytesPerRow / pixelBytes; rowLen > size.Width {
				glCtx.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(rowLen))
				defer glCtx.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
			}
		}
		// Set alignment to 1 for single-channel formats whose row stride
		// may not be a multiple of the default 4-byte GL_UNPACK_ALIGNMENT.
		if tex.format == gputypes.TextureFormatR8Unorm {
			glCtx.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
		}
		// Use TexSubImage2D to update existing texture data.
		// TexImage2D reallocates storage on every call; TexSubImage2D updates in-place.
		// Honor Origin: dirty-rect uploads (glyph atlas pages) arrive with a
		// non-zero origin — writing at (0,0) misplaces ink into wrong cells.
		glCtx.TexSubImage2D(tex.target, int32(dst.MipLevel),
			int32(dst.Origin.X), int32(dst.Origin.Y),
			int32(size.Width), int32(size.Height), format, dataType,
			unsafe.Pointer(&data[0]))
		// Restore default alignment after upload.
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
// Makes the GL context current on the window EGLSurface (via LockForSurface),
// blits the Surface's swapchain offscreen FBO to the default framebuffer with
// an explicit Y-flip, then SwapBuffers.
//
// damageRects is an optional list of rectangles (physical pixels, top-left
// origin) indicating which surface regions changed this frame. When non-empty
// and EGL_KHR_swap_buffers_with_damage is available, the rects are passed to
// eglSwapBuffersWithDamageKHR as compositor hints. EGL uses bottom-left
// origin, so Y coordinates are flipped here. When the extension is unavailable
// or no rects are provided, the standard eglSwapBuffers path is used.
func (q *Queue) Present(surface hal.Surface, tex hal.SurfaceTexture, damageRects []image.Rectangle) error {
	surf, ok := surface.(*Surface)
	if !ok {
		return fmt.Errorf("gles: invalid surface type")
	}

	// Bind-or-stop: a failed MakeCurrent(surface) must not fall through
	// into blit + SwapBuffers with nothing current (the old LockForSurface
	// returned gl anyway, drawing 17 dead frames into the void after a
	// maximize BadAlloc 0x3003). On failure rebuild the dead EGL surface
	// once (Flutter OnScreenSurfaceResize parity) and retry the bind once;
	// a still-dead surface skips the frame with an error so the caller
	// keeps the last good image instead of presenting transparent.
	glCtx, err := q.ctx.TryLockForSurface(surf.eglSurface)
	if err != nil {
		if rerr := surf.recreateEGLWindowSurface(); rerr == nil {
			glCtx, err = q.ctx.TryLockForSurface(surf.eglSurface)
		} else {
			hal.Logger().Warn("gles: Present surface dead, recreate failed", "err", rerr)
		}
		if err != nil {
			return fmt.Errorf("gles: Present bind surface failed (skip frame): %w", err)
		}
	}
	defer q.ctx.Unlock()

	if list := q.batch.drain(); len(list) > 0 {
		flushWriteBatch(glCtx, newGLExecState(), list)
	}
	surf.blitSwapchainToDefaultWith(glCtx)

	// Use damage-aware swap when the extension is available and rects provided.
	if len(damageRects) > 0 && egl.HasSwapBuffersWithDamage() {
		// Convert image.Rectangle (top-left origin) to EGL packed int32 array
		// (bottom-left origin). Each rect is {x, y, width, height}.
		// Stack-allocate for up to 8 rects (8 * 4 = 32 ints).
		var stackInts [32]int32
		ints := stackInts[:0]
		// Damage rects arrive in surface (config-size) coordinates; flip
		// against the config height, which is the window size. Normally it
		// equals the FBO height; while borrowing an old FBO they differ.
		surfaceHeight := surf.presentHeight()
		for _, r := range damageRects {
			// Y-flip: EGL uses bottom-left origin.
			// egl_y = surface_height - rect.Max.Y
			ints = append(ints,
				int32(r.Min.X),
				surfaceHeight-int32(r.Max.Y),
				int32(r.Dx()),
				int32(r.Dy()),
			)
		}
		result := egl.SwapBuffersWithDamage(surf.eglDisplay, surf.eglSurface, &ints[0], int32(len(damageRects)))
		if result == egl.False {
			return fmt.Errorf("gles: eglSwapBuffersWithDamageKHR failed: error 0x%x", egl.GetError())
		}
		surf.DiscardTexture(tex)
		return nil
	}

	// Standard full-surface swap.
	result := egl.SwapBuffers(surf.eglDisplay, surf.eglSurface)
	if result == egl.False {
		return fmt.Errorf("gles: eglSwapBuffers failed: error 0x%x", egl.GetError())
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

// SupportsCommandBufferCopies returns false for GLES on Linux.
// GLES uses direct GL calls for writes, not command buffer copy operations.
func (q *Queue) SupportsCommandBufferCopies() bool {
	return false
}

// SetSwapchainSuppressed is a no-op on GLES.
// GLES uses eglSwapBuffers for presentation, which is not affected by command
// submission ordering.
func (q *Queue) SetSwapchainSuppressed(_ bool) {}
