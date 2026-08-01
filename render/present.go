package render

import (
	"errors"
	"fmt"
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// ErrNilSurfaceView is returned when PresentFrame receives a nil texture view.
var ErrNilSurfaceView = errors.New("render: nil surface texture view")

// surfaceCacheOps is the optional per-context surface-cache capability
// (A2 retained compositing) implemented by gpu.GPURenderContext.
type surfaceCacheOps interface {
	SurfaceCacheView() gpucontext.TextureView
	BlitCacheToView(view gpucontext.TextureView, w, h uint32) error
}

// resolvePresentView returns the actual flush target for a present.
// In surface-cache mode the persistent cache texture substitutes for the
// swapchain view so steady frames render with LoadOpLoad + damage scissor;
// the cache is blitted to the swapchain after the flush. Returns the cache
// blitter when cache mode is active (nil otherwise).
func (c *Context) resolvePresentView(view gpucontext.TextureView) (gpucontext.TextureView, surfaceCacheOps) {
	if c == nil {
		return view, nil
	}
	rc := c.gpuCtxOps()
	if rc == nil {
		return view, nil
	}
	if sc, ok := rc.(surfaceCacheOps); ok {
		if cv := sc.SurfaceCacheView(); !cv.IsNil() {
			return cv, sc
		}
	}
	return view, nil
}

// presentAfterFlush runs the present callback (or marks the view for later readback)
// after a successful GPU flush into view.
func (c *Context) presentAfterFlush(view gpucontext.TextureView, width, height uint32, present func() error) error {
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
	return nil
}

// flushPresentView flushes the scene into the (possibly cache-substituted)
// render target, then composites the cache onto the swapchain view when cache
// mode is active. Runs before the present callback.
func (c *Context) flushPresentView(view gpucontext.TextureView, width, height uint32, damage []image.Rectangle) error {
	renderView, sc := c.resolvePresentView(view)
	if len(damage) == 0 {
		if err := c.FlushGPUWithView(renderView, width, height); err != nil {
			return err
		}
	} else {
		if err := c.FlushGPUWithViewDamageRects(renderView, width, height, damage); err != nil {
			return err
		}
	}
	if sc != nil {
		if err := sc.BlitCacheToView(view, width, height); err != nil {
			return fmt.Errorf("render: blit surface cache: %w", err)
		}
	}
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
	if err := c.flushPresentView(view, width, height, nil); err != nil {
		return fmt.Errorf("render: FlushGPUWithView: %w", err)
	}
	return c.presentAfterFlush(view, width, height, present)
}

// PresentFrameDamage is PresentFrame with a single damage rect for LoadOpLoad paths.
func (c *Context) PresentFrameDamage(view gpucontext.TextureView, width, height uint32, damage image.Rectangle, present func() error) error {
	if err := preparePresent(c, view, width, height); err != nil {
		return err
	}
	var rects []image.Rectangle
	if !damage.Empty() {
		rects = []image.Rectangle{damage}
	}
	if err := c.flushPresentView(view, width, height, rects); err != nil {
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
	if err := c.flushPresentView(view, width, height, rects); err != nil {
		return fmt.Errorf("render: FlushGPUWithViewDamageRects: %w", err)
	}
	return c.presentAfterFlush(view, width, height, present)
}
