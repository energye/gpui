package render

import (
	"errors"
	"fmt"
	"image"
	"time"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// ErrNilSurfaceView is returned when PresentFrame receives a nil texture view.
var ErrNilSurfaceView = errors.New("render: nil surface texture view")

// FrameFlushMs / FramePresentWaitMs / FrameAcquireWaitMs carry the last
// F期 split timings from render to the embedder metrics. Flush = GPU干活
// (重录+画画); Acquire = BeginFrame 等空闲缓冲(Fifo 背压睡这儿);
// PresentWait = EndFrame 等显示器. Last-write-wins, raster-thread only.
var (
	FrameFlushMs       float64
	FramePresentWaitMs float64
	FrameAcquireWaitMs float64
)

// notePresentWaitMs records present-wait duration (F期尺子：等显示器单记)。
func notePresentWaitMs(tWait time.Time) {
	FramePresentWaitMs = time.Since(tWait).Seconds() * 1000
}

// presentAfterFlush runs the present callback (or marks the view for later readback)
// after a successful GPU flush into view.
func (c *Context) presentAfterFlush(view gpucontext.TextureView, width, height uint32, present func() error) error {
	tWait := time.Now()
	if present != nil {
		// Swapchain EndFrame releases the surface view — do not keep it for
		// SavePNG/Image readback (would sample a freed view → black pixmap).
		if err := present(); err != nil {
			return fmt.Errorf("render: present: %w", err)
		}
		c.clearViewFlushTracking()
	} else {
		// Offscreen / no-op present: view remains readable for Image/SavePNG.
		c.markViewFlush(view, int(width), int(height)) //nolint:gosec
	}
	// F期尺子：等显示器的耗时（Fifo 被动等）。Flush(干活)由调用方单记。
	notePresentWaitMs(tWait)
	return nil
}

// preparePresent validates present arguments shared by all PresentFrame* entry points.
func preparePresent(c *Context, view gpucontext.TextureView, width, height uint32) error {
	if c == nil {
		return errors.New("render: nil context")
	}
	if view.IsNil() {
		return ErrNilSurfaceView
	}
	if width == 0 || height == 0 {
		return fmt.Errorf("render: present extent must be non-zero (got %dx%d)", width, height)
	}
	return nil
}

// PresentFrame flushes the current GPU scene into a surface texture view and
// then invokes present (typically Swapchain.EndFrame / Surface.Present).
//
// For retained UI steady frames prefer PresentFrameAuto (S6.1); use PresentFrame
// / PresentFrameFull for bootstrap and deliberate full redraw only.
//
// This is the S.03 window present entry point for application code:
//
//	frame, err := swapchain.BeginFrame()
//	// draw into dc...
//	err = dc.PresentFrame(frame.Handle, frame.Width, frame.Height, func() error {
//	    return swapchain.EndFrame(frame)
//	})
//
// Offscreen paths may pass a no-op present after FlushGPUWithView semantics.
func (c *Context) PresentFrame(view gpucontext.TextureView, width, height uint32, present func() error) error {
	if err := preparePresent(c, view, width, height); err != nil {
		return err
	}
	if err := c.FlushGPUWithView(view, width, height); err != nil {
		return fmt.Errorf("render: FlushGPUWithView: %w", err)
	}
	return c.presentAfterFlush(view, width, height, present)
}

// PresentFrameDamage is PresentFrame with a single damage rect for LoadOpLoad paths.
func (c *Context) PresentFrameDamage(view gpucontext.TextureView, width, height uint32, damage image.Rectangle, present func() error) error {
	if err := preparePresent(c, view, width, height); err != nil {
		return err
	}
	if err := c.FlushGPUWithViewDamage(view, width, height, damage); err != nil {
		return fmt.Errorf("render: FlushGPUWithViewDamage: %w", err)
	}
	return c.presentAfterFlush(view, width, height, present)
}

// PresentFrameDamageRects is PresentFrame with multiple damage rects (ADR-028).
// Distant dirty regions can keep independent scissors instead of one union box.
func (c *Context) PresentFrameDamageRects(view gpucontext.TextureView, width, height uint32, rects []image.Rectangle, present func() error) error {
	if err := preparePresent(c, view, width, height); err != nil {
		return err
	}
	if err := c.FlushGPUWithViewDamageRects(view, width, height, rects); err != nil {
		return fmt.Errorf("render: FlushGPUWithViewDamageRects: %w", err)
	}
	return c.presentAfterFlush(view, width, height, present)
}
