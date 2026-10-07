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
	"image"
	"math"
)

// NewContextForPixmap creates a Context backed by an existing Pixmap.
// The Context renders directly into the provided pixmap without allocating
// a new one. Used by scene.Renderer for GPU-accelerated scene rendering.
func NewContextForPixmap(pm *Pixmap) *Context {
	if pm == nil {
		return nil
	}
	return NewContext(pm.Width(), pm.Height(), func(o *contextOptions) {
		o.pixmap = pm
	})
}

func NewContext(width, height int, opts ...ContextOption) *Context {
	// Apply options
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	scale := options.deviceScale
	if scale <= 0 {
		scale = 1.0
	}

	// Physical dimensions for the pixmap
	pw := int(float64(width) * scale)
	ph := int(float64(height) * scale)

	// Use provided pixmap or create one at physical resolution
	pixmap := options.pixmap
	if pixmap == nil {
		pixmap = NewPixmap(pw, ph)
	}

	// Use provided renderer or create software renderer at physical resolution
	renderer := options.renderer
	if renderer == nil {
		sr := NewSoftwareRenderer(pw, ph)
		if scale > 1.0 {
			sr.SetDeviceScale(float32(scale))
		}
		renderer = sr
	}

	// Device matrix: maps user coordinates to physical pixels.
	// User matrix starts as Identity — user transforms never include device scale.
	deviceMatrix := Identity()
	if scale != 1.0 {
		deviceMatrix = Scale(scale, scale)
	}

	if scale != 1.0 {
		Logger().Info("NewContext HiDPI",
			"logical_w", width, "logical_h", height,
			"scale", scale,
			"physical_w", pw, "physical_h", ph,
		)
	}

	c := &Context{
		width:                 width,
		height:                height,
		pixmap:                pixmap,
		renderer:              renderer,
		path:                  NewPath(),
		paint:                 NewPaint(),
		matrix:                Identity(),
		deviceMatrix:          deviceMatrix,
		stack:                 make([]Matrix, 0, 8),
		clipStackDepth:        make([]int, 0, 8),
		pipelineMode:          options.pipelineMode,
		damageTrackingEnabled: true,
		antiAlias:             true,
	}
	c.deviceScale.Store(math.Float64bits(scale))
	return c
}

// NewContextForImage creates a context for drawing on an existing image.
// Optional ContextOption arguments can be used for dependency injection.
// The image dimensions are treated as physical pixel dimensions (deviceScale=1.0).
// NewContextForImage creates a context for drawing on an existing image.
// Optional ContextOption arguments can be used for dependency injection.
// The image dimensions are treated as physical pixel dimensions (deviceScale=1.0).
func NewContextForImage(img image.Image, opts ...ContextOption) *Context {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixmap := FromImage(img)

	// Apply options
	options := defaultOptions()
	for _, opt := range opts {
		opt(&options)
	}

	// Use provided renderer or create software renderer
	renderer := options.renderer
	if renderer == nil {
		renderer = NewSoftwareRenderer(width, height)
	}

	c := &Context{
		width:          width,
		height:         height,
		pixmap:         pixmap,
		renderer:       renderer,
		path:           NewPath(),
		paint:          NewPaint(),
		matrix:         Identity(),
		deviceMatrix:   Identity(),
		stack:          make([]Matrix, 0, 8),
		clipStackDepth: make([]int, 0, 8),
		pipelineMode:   options.pipelineMode,
	}
	// Atomic has no literal form: store the documented 1.0 default.
	c.deviceScale.Store(math.Float64bits(1.0))
	return c
}

// NewContextWithScale creates a new drawing context with the given logical
// dimensions and device scale factor. This is a convenience wrapper for:
//
//	render.NewContext(w, h, render.WithDeviceScale(scale))
//
// The internal pixmap is allocated at physical resolution (w*scale x h*scale).
// All drawing operations use logical coordinates (w x h).
//
// Example (macOS Retina 2x):
//
//	dc := render.NewContextWithScale(800, 600, 2.0)
//	dc.Width() // 800 (logical)
//	dc.PixelWidth() // 1600 (physical)
//	dc.DrawCircle(400, 300, 100) // logical coordinates
//
// NewContextWithScale creates a new drawing context with the given logical
// dimensions and device scale factor. This is a convenience wrapper for:
//
//	render.NewContext(w, h, render.WithDeviceScale(scale))
//
// The internal pixmap is allocated at physical resolution (w*scale x h*scale).
// All drawing operations use logical coordinates (w x h).
//
// Example (macOS Retina 2x):
//
//	dc := render.NewContextWithScale(800, 600, 2.0)
//	dc.Width() // 800 (logical)
//	dc.PixelWidth() // 1600 (physical)
//	dc.DrawCircle(400, 300, 100) // logical coordinates
func NewContextWithScale(width, height int, scale float64) *Context {
	return NewContext(width, height, WithDeviceScale(scale))
}

// Close releases resources associated with the Context.
// After Close, the Context should not be used.
// Close is idempotent - multiple calls are safe.
// Implements io.Closer.
//
// Close flushes any pending GPU accelerator operations to ensure all
// queued draw commands are rendered before releasing context state.
// Note: Close does NOT shut down the global GPU accelerator itself,
// since it may be shared by other contexts. To release GPU resources
// at application shutdown, call [CloseAccelerator].
// Close releases resources associated with the Context.
// After Close, the Context should not be used.
// Close is idempotent - multiple calls are safe.
// Implements io.Closer.
//
// Close flushes any pending GPU accelerator operations to ensure all
// queued draw commands are rendered before releasing context state.
// Note: Close does NOT shut down the global GPU accelerator itself,
// since it may be shared by other contexts. To release GPU resources
// at application shutdown, call [CloseAccelerator].
func (c *Context) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true

	// Flush pending GPU operations so queued shapes are not lost.
	c.flushGPUAccelerator()
	c.drainLayerGPUReleases()

	// Drop published filter texture before GPU ctx teardown.
	c.releaseFilterGPUResult()
	c.releaseFilterSrcRT()
	c.pixmapFilterStale = false

	// Close per-context GPU render context if it was created.
	if c.gpuCtx != nil {
		type gpuCtxCloser interface {
			Close()
		}
		if closer, ok := c.gpuCtx.(gpuCtxCloser); ok {
			closer.Close()
		}
		c.gpuCtx = nil
		unregisterGPUContext(c)
	}

	// Clear path to release memory
	c.ClearPath()

	// Clear state stack
	c.stack = nil
	c.clipStackDepth = nil
	c.maskStack = nil
	c.mask = nil
	c.gpuClipPath = nil

	return nil
}

// SetPipelineMode sets the GPU rendering pipeline mode.
// See PipelineMode for available modes.
//
// If the registered accelerator implements PipelineModeAware, the mode is
// propagated so the accelerator can route operations to the correct pipeline
// (render pass vs compute).

// DropGPURenderContext closes the per-context GPU session (device-bound textures
// / pipelines bindings) without closing the Context itself. Call after
// AutoRecover / SetDeviceProvider so the next Present rebuilds a clean session.
// DropGPURenderContext closes the per-context GPU session (device-bound textures
// / pipelines bindings) without closing the Context itself. Call after
// AutoRecover / SetDeviceProvider so the next Present rebuilds a clean session.
func (c *Context) DropGPURenderContext() {
	if c == nil {
		return
	}
	// Device-bound filter publishes / seed RTs pin VRAM across AutoRecover if kept.
	c.releaseFilterGPUResult()
	c.releaseFilterSrcRT()
	c.pixmapFilterStale = false
	if c.gpuCtx != nil {
		type gpuCtxCloser interface {
			Close()
		}
		if closer, ok := c.gpuCtx.(gpuCtxCloser); ok {
			closer.Close()
		}
		c.gpuCtx = nil
	}
	unregisterGPUContext(c)
}
