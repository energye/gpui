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

// Image returns the context's image.
// Pending GPU commands are flushed first so readback matches SavePNG semantics
// (CPU pixmap must include GPU-rendered content).
func (c *Context) Image() image.Image {
	_ = c.FlushGPU() // also applies dither when enabled
	_ = c.syncViewFlushIntoPixmap()
	_ = c.materializeFilterGPU()
	return c.pixmap.ToImage()
}

// SavePNG saves the context to a PNG file.
// Fill fills the current path and clears it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// The RasterizerMode set via SetRasterizerMode controls algorithm selection.
// Returns an error if the rendering operation fails.
func (c *Context) Fill() error {

	c.trackDamage(c.path.Bounds())
	err := c.doFill()
	c.path.Clear()
	return err
}

// Stroke strokes the current path and clears it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// The RasterizerMode set via SetRasterizerMode controls algorithm selection.
// Returns an error if the rendering operation fails.
// Stroke strokes the current path and clears it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// The RasterizerMode set via SetRasterizerMode controls algorithm selection.
// Returns an error if the rendering operation fails.
func (c *Context) Stroke() error {

	c.trackDamage(c.path.Bounds())
	err := c.doStroke()
	c.path.Clear()
	return err
}

// FillPreserve fills the current path without clearing it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// Returns an error if the rendering operation fails.
// FillPreserve fills the current path without clearing it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// Returns an error if the rendering operation fails.
func (c *Context) FillPreserve() error {
	return c.doFill()
}

// StrokePreserve strokes the current path without clearing it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// Returns an error if the rendering operation fails.
// StrokePreserve strokes the current path without clearing it.
// If a GPU accelerator is registered and supports the path, it is used first.
// Otherwise, the software renderer handles the operation.
// Returns an error if the rendering operation fails.
func (c *Context) StrokePreserve() error {
	return c.doStroke()
}

// Push saves the current state (transform, paint, clip, and mask).
// SetPixel sets a single pixel.
func (c *Context) SetPixel(x, y int, col RGBA) {
	c.pixmap.SetPixel(x, y, col)
	c.noteLayerCPUDraw()
}

// WritePixels writes a tight premul RGBA8 rectangle into the surface (S.07).
// Coordinates are in surface pixel space (device pixels), not transformed by CTM.
//
// Pipeline:
//  1. CPU pixmap mirror is updated (so Image/readback stay consistent).
//  2. When a GPU accelerator is available, pixels are uploaded via the
//     textured-quad path (queue.WriteTexture under the hood) and count as
//     a GPU op — not a silent CPU-only store.
//
// pixels must be row-major premul RGBA8 with length >= width*height*4.
// Out-of-bounds regions are clipped; the clipped sub-rectangle is uploaded.
// WritePixels writes a tight premul RGBA8 rectangle into the surface (S.07).
// Coordinates are in surface pixel space (device pixels), not transformed by CTM.
//
// Pipeline:
//  1. CPU pixmap mirror is updated (so Image/readback stay consistent).
//  2. When a GPU accelerator is available, pixels are uploaded via the
//     textured-quad path (queue.WriteTexture under the hood) and count as
//     a GPU op — not a silent CPU-only store.
//
// pixels must be row-major premul RGBA8 with length >= width*height*4.
// Out-of-bounds regions are clipped; the clipped sub-rectangle is uploaded.
func (c *Context) WritePixels(x, y, width, height int, pixels []byte) {
	if c == nil || c.pixmap == nil || width <= 0 || height <= 0 {
		return
	}
	if len(pixels) < width*height*4 {
		return
	}

	origW := width
	srcOX, srcOY := 0, 0
	dstW, dstH := c.pixmap.Width(), c.pixmap.Height()

	if x < 0 {
		srcOX = -x
		width += x
		x = 0
	}
	if y < 0 {
		srcOY = -y
		height += y
		y = 0
	}
	if x >= dstW || y >= dstH || width <= 0 || height <= 0 {
		return
	}
	if x+width > dstW {
		width = dstW - x
	}
	if y+height > dstH {
		height = dstH - y
	}
	if width <= 0 || height <= 0 {
		return
	}

	// CPU mirror: copy clipped rows into pixmap.
	dst := c.pixmap.Data()
	for row := 0; row < height; row++ {
		srcOff := ((srcOY+row)*origW + srcOX) * 4
		dstOff := ((y+row)*dstW + x) * 4
		copy(dst[dstOff:dstOff+width*4], pixels[srcOff:srcOff+width*4])
	}
	c.pixmap.NotifyPixelsChanged()
	c.trackDamage(image.Rect(x, y, x+width, y+height))

	// GPU path: textured blit of the same clipped pixels.
	if c.tryGPUWritePixels(x, y, width, height, pixels, origW, srcOX, srcOY) {
		c.recordGPUOp()
	}
}

// tryGPUWritePixels uploads a clipped premul RGBA8 block via GPU image draw.
// origW/srcOX/srcOY describe the source packing of the full WritePixels buffer.
// tryGPUWritePixels uploads a clipped premul RGBA8 block via GPU image draw.
// origW/srcOX/srcOY describe the source packing of the full WritePixels buffer.
func (c *Context) tryGPUWritePixels(dstX, dstY, width, height int, pixels []byte, origW, srcOX, srcOY int) bool {
	if !c.gpuPathAvailable() {
		return false
	}
	rc := c.gpuCtxOps()
	if rc == nil {
		return false
	}

	// Pack tight buffer for upload (GPU path accepts tight rows via stride=w*4).
	tight := make([]byte, width*height*4)
	for row := 0; row < height; row++ {
		srcOff := ((srcOY+row)*origW + srcOX) * 4
		copy(tight[row*width*4:(row+1)*width*4], pixels[srcOff:srcOff+width*4])
	}

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec
	vpH := uint32(target.Height) //nolint:gosec
	fx, fy := float32(dstX), float32(dstY)
	fw, fh := float32(width), float32(height)
	rc.QueueImageDraw(target, tight, c.pixmap.GenerationID(), width, height, width*4,
		fx, fy,
		fx+fw, fy,
		fx+fw, fy+fh,
		fx, fy+fh,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		true /* nearest — pixel-exact write */, false)
	return true
}

// DrawPoint draws a single point at the given coordinates.
// doFill performs the fill operation respecting the current RasterizerMode.
func (c *Context) doFill() error {
	c.syncPublishedFilterBeforeDraw()
	mode := c.rasterizerMode

	// Set GPU scissor rect for rectangular clips.
	defer c.setGPUClipRect()()

	// Set clip/mask coverage BEFORE GPU attempt so that CPU SDF fallback
	// (SDFAccelerator.FillShape) can apply per-pixel clip+mask coverage.
	// The GPU render-pass path ignores paint.ClipCoverage (uses shader-based
	// clipping), so setting it early is harmless for the GPU path.
	c.applyClipToPaint()
	defer func() { c.paint.ClipCoverage = nil }()

	c.applyMaskToPaint()
	defer func() { c.paint.MaskCoverage = nil }()

	// F1 opacity-group: multiply paint alpha by open Normal layer opacities.
	defer c.applyLayerOpacityMul()()

	// Transform path to device-space for rendering.
	// At scale=1.0 this is a zero-copy no-op.
	devicePath := c.deviceSpacePath()

	// Propagate anti-aliasing state to GPU render context.
	if rc := c.gpuCtxOps(); rc != nil {
		rc.SetAntiAlias(c.antiAlias)
	}

	// Mask/difference clips without gpuClipPath: prefer GPU R8 mask
	// over full-surface CPU; only fall back if MaskAware install fails.
	forceCPUClip := c.clipStack != nil && c.clipStack.HasMaskClip() && c.gpuClipPath == nil
	clipMaskCleanup := func() {}
	if forceCPUClip {
		if cleanup, okInstall := c.installGPUClipMask(); okInstall {
			clipMaskCleanup = cleanup
			forceCPUClip = false
		}
	}
	defer clipMaskCleanup()
	// P0-1: layers with a GPU RT draw on GPU into the offscreen view. Layers
	// without a GPU RT (advanced blend, mask, no GPU) stay on the CPU pixmap
	// path — Pop uses damage-bounded CPU composite (no full-surface readback).
	forceCPULayer := c.layerForceCPUDraw()

	// Temporarily swap c.path to device-space for GPU tryGPUOp
	// (which reads c.path for shape detection and path rendering).
	origPath := c.path
	c.path = devicePath
	var ok bool
	var cpuMode RasterizerMode
	if forceCPUClip {
		ok, cpuMode = false, mode
		if c.gpuPathAvailable() {
			c.recordCPUFallbackReason("clip-mask")
		}
	} else if forceCPULayer {
		// Intentional CPU layer path (no GPU RT on this layer).
		ok, cpuMode = false, mode
	} else if _, isSW := c.renderer.(*SoftwareRenderer); !isSW {
		// Custom Renderer DI: never intercept with the global GPU accelerator.
		ok, cpuMode = false, mode
	} else if CPUOnlyMode() {
		// GOGPU_RENDER_MODE=cpu: route shapes to the CPU rasterizer so the
		// window present can upload the pixmap without the GPU session
		// (window presents depend on the GPU session, which may fail on
		// CPU-only setups — black window).
		ok, cpuMode = false, mode
	} else {
		ok, cpuMode = c.tryGPUFillWithMode(mode)
	}
	c.path = origPath
	if ok {
		return nil
	}

	// CPU path: flush pending GPU, apply mode and AA state to software renderer.
	// When drawing into a PushLayer surface, do NOT flush GPU here — pending GPU
	// ops may target the layer's offscreen view (or parent). Flushing would force
	// a costly full-surface resolve and break 60fps UI.
	if !(c.layerStack != nil && len(c.layerStack.layers) > 0) {
		c.flushGPUAccelerator()
	}
	if sr, ok := c.renderer.(*SoftwareRenderer); ok {
		sr.rasterizerMode = cpuMode
		sr.antiAlias = c.antiAlias
		defer func() {
			sr.rasterizerMode = RasterizerAuto
			sr.antiAlias = true
		}()
	}

	// CPU wrote into the layer pixmap — Pop must not GPU-blit an empty RT.
	c.noteLayerCPUDraw()
	return c.renderer.Fill(c.pixmap, devicePath, c.paint)
}

// doStroke performs the stroke operation respecting the current RasterizerMode.
//
// GPU strokes batch until PresentFrame /
// Image / SavePNG — matching doFill. Per-op flush used to force a GPU submit on
// every Stroke() and locked multi-stroke UI scenes to
// ~20fps.
//
// Fallback (T.03): expand stroke in pure user space, transform the outline by
// the CTM, then fill (NonZero). NonZero matches Skia stroke outlines and keeps
// sharp open joins solid (EvenOdd punched holes at inner-pivot V-joins).
// Closed stroke rings stay hollow via reverse inner contour winding.
// HiDPI via deviceSpacePath in doFill; Image()/SavePNG() FlushGPU.
// doStroke performs the stroke operation respecting the current RasterizerMode.
//
// GPU strokes batch until PresentFrame /
// Image / SavePNG — matching doFill. Per-op flush used to force a GPU submit on
// every Stroke() and locked multi-stroke UI scenes to
// ~20fps.
//
// Fallback (T.03): expand stroke in pure user space, transform the outline by
// the CTM, then fill (NonZero). NonZero matches Skia stroke outlines and keeps
// sharp open joins solid (EvenOdd punched holes at inner-pivot V-joins).
// Closed stroke rings stay hollow via reverse inner contour winding.
// HiDPI via deviceSpacePath in doFill; Image()/SavePNG() FlushGPU.
func (c *Context) doStroke() error {
	c.syncPublishedFilterBeforeDraw()
	mode := c.rasterizerMode

	// Match doFill GPU setup: scissor, clip/mask coverage, AA.
	defer c.setGPUClipRect()()
	c.applyClipToPaint()
	defer func() { c.paint.ClipCoverage = nil }()
	c.applyMaskToPaint()
	defer func() { c.paint.MaskCoverage = nil }()
	// F1 opacity-group: match doFill alpha multiply for GPU stroke path.
	defer c.applyLayerOpacityMul()()

	if rc := c.gpuCtxOps(); rc != nil {
		rc.SetAntiAlias(c.antiAlias)
	}

	// Mask/difference clips: GPU R8 mask when possible, else CPU reason.
	forceCPUClip := c.clipStack != nil && c.clipStack.HasMaskClip() && c.gpuClipPath == nil
	clipMaskCleanup := func() {}
	if forceCPUClip {
		if cleanup, okInstall := c.installGPUClipMask(); okInstall {
			clipMaskCleanup = cleanup
			forceCPUClip = false
		}
	}
	defer clipMaskCleanup()
	forceCPULayer := c.layerForceCPUDraw()
	_, isSoftwareRenderer := c.renderer.(*SoftwareRenderer)
	// T.03: non-uniform / skewed CTM needs user-space stroke expand then transform
	// . Direct GPU StrokePath expands with a uniform device width.
	userSpaceStroke := c.matrixRequiresUserSpaceStroke()
	if forceCPULayer {
		// Intentional CPU layer path (no GPU RT on this layer).
	} else if !isSoftwareRenderer {
		// Custom Renderer DI: never intercept with the global GPU accelerator.
	} else if !forceCPUClip && !userSpaceStroke && !CPUOnlyMode() {
		devicePath := c.deviceSpacePath()
		origPath := c.path
		origScale := c.paint.TransformScale
		// Propagate the total baked scale (user CTM x HiDPI deviceScale)
		// so stroke width and dash periods match device-space path
		// coordinates (paint.TransformScale contract, paint.go). Only
		// HiDPI was propagated before: uniform user scale was lost and
		// every scaled stroke rendered 1/scale too thin
		// (TestStrokeExpansion_ScaledDashedRect).
		if origScale <= 0 {
			baked := 1.0
			if !c.matrix.IsIdentity() {
				if sx := math.Hypot(c.matrix.A, c.matrix.D); sx > baked {
					baked = sx
				}
				if sy := math.Hypot(c.matrix.B, c.matrix.E); sy > baked {
					baked = sy
				}
			}
			if ds := math.Float64frombits(c.deviceScale.Load()); ds > 0 {
				baked *= ds
			}
			c.paint.TransformScale = baked
		}
		c.path = devicePath
		ok, _ := c.tryGPUStrokeWithMode(mode)
		c.path = origPath
		c.paint.TransformScale = origScale
		if ok {
			// Batched with fills until PresentFrame / Image / SavePNG.
			return nil
		}
	} else if c.gpuPathAvailable() && (forceCPUClip || userSpaceStroke) {
		reason := "clip-mask"
		if userSpaceStroke && !forceCPUClip {
			reason = "stroke-user-space"
		}
		c.recordCPUFallbackReason(reason)
	}

	outline := c.expandStrokeToPathSpace()
	if outline == nil || outline.NumVerbs() == 0 {
		return nil
	}

	origPath := c.path
	origRule := c.paint.FillRule
	origScale := c.paint.TransformScale
	c.path = outline
	c.paint.FillRule = FillRuleNonZero
	// Outline already includes user CTM; device scale applied in doFill.
	c.paint.TransformScale = 1
	err := c.doFill()
	c.path = origPath
	c.paint.FillRule = origRule
	c.paint.TransformScale = origScale
	// Do not flush here. Batching is required for Skia-class multi-stroke frames.
	// Immediate materialization remains available via FlushGPU / Image / Present.
	return err
}

// applyClipToPaint sets the ClipCoverage function on the paint when a clip
// stack is active and has entries. This allows the renderer to apply per-pixel
// clip masks during compositing.
// applyMaskToPaint sets the MaskCoverage function on the paint when an alpha
// mask is active. This allows the renderer to apply per-pixel mask modulation
// during compositing. Mask and clip compose multiplicatively.
func (c *Context) applyMaskToPaint() {
	if c.mask == nil {
		return
	}
	m := c.mask
	c.paint.MaskCoverage = func(x, y int) uint8 {
		return m.At(x, y)
	}
}

// isClipActive reports whether a clip region is currently active.
