package render

import (
	"errors"
	"fmt"
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// ErrNilSurfaceView is returned when PresentFrame receives a nil texture view.
var ErrNilSurfaceView = errors.New("render: nil surface texture view")

// PresentFrame flushes the current GPU scene into a surface texture view and
// then invokes present (typically Swapchain.EndFrame / Surface.Present).
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
	if c == nil {
		return errors.New("render: nil context")
	}
	if view.IsNil() {
		return ErrNilSurfaceView
	}
	if width == 0 || height == 0 {
		return fmt.Errorf("render: present extent must be non-zero (got %dx%d)", width, height)
	}
	if err := c.FlushGPUWithView(view, width, height); err != nil {
		return fmt.Errorf("render: FlushGPUWithView: %w", err)
	}
	if present != nil {
		if err := present(); err != nil {
			return fmt.Errorf("render: present: %w", err)
		}
	}
	return nil
}

// PresentFrameDamage is PresentFrame with a single damage rect for LoadOpLoad paths.
func (c *Context) PresentFrameDamage(view gpucontext.TextureView, width, height uint32, damage image.Rectangle, present func() error) error {
	if c == nil {
		return errors.New("render: nil context")
	}
	if view.IsNil() {
		return ErrNilSurfaceView
	}
	if width == 0 || height == 0 {
		return fmt.Errorf("render: present extent must be non-zero (got %dx%d)", width, height)
	}
	if err := c.FlushGPUWithViewDamage(view, width, height, damage); err != nil {
		return fmt.Errorf("render: FlushGPUWithViewDamage: %w", err)
	}
	if present != nil {
		if err := present(); err != nil {
			return fmt.Errorf("render: present: %w", err)
		}
	}
	return nil
}

// PresentFrameDamageRects is PresentFrame with multiple damage rects (ADR-028).
// Distant dirty regions can keep independent scissors instead of one union box.
func (c *Context) PresentFrameDamageRects(view gpucontext.TextureView, width, height uint32, rects []image.Rectangle, present func() error) error {
	if c == nil {
		return errors.New("render: nil context")
	}
	if view.IsNil() {
		return ErrNilSurfaceView
	}
	if width == 0 || height == 0 {
		return fmt.Errorf("render: present extent must be non-zero (got %dx%d)", width, height)
	}
	if err := c.FlushGPUWithViewDamageRects(view, width, height, rects); err != nil {
		return fmt.Errorf("render: FlushGPUWithViewDamageRects: %w", err)
	}
	if present != nil {
		if err := present(); err != nil {
			return fmt.Errorf("render: present: %w", err)
		}
	}
	return nil
}
