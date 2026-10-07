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

func FilterPoolStats() (gets, puts, hits, misses int) {
	return filterPixmapPool.Stats()
}

// ResetFilterPoolStats clears filter pool counters (tests).
// ResetFilterPoolStats clears filter pool counters (tests).
func ResetFilterPoolStats() {
	filterPixmapPool.ResetStats()
}

// RegisterFilterOps wires image-filter implementations (blur/shadow/color matrix).
// Called from render/filters init to avoid import cycles with internal/filter.
// RegisterFilterOps wires image-filter implementations (blur/shadow/color matrix).
// Called from render/filters init to avoid import cycles with internal/filter.
func RegisterFilterOps(
	blur func(src, dst *Pixmap, radius float64),
	blurXY func(src, dst *Pixmap, radiusX, radiusY float64),
	shadow func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA),
	colorMatrix func(src, dst *Pixmap, matrix [20]float32),
	grayscale func(src, dst *Pixmap),
	invert func(src, dst *Pixmap),
) {
	blurApply = blur
	blurXYApply = blurXY
	dropShadowApply = shadow
	colorMatrixApply = colorMatrix
	grayscaleApply = grayscale
	invertApply = invert
}

func (c *Context) applyFilterInPlace(fn filterApplyFunc) {
	if c == nil || c.pixmap == nil || fn == nil {
		return
	}
	_ = c.FlushGPU()
	// Silent CPU forbidden: if GPU path was available but we are here, count it.
	if c.gpuPathAvailable() && GPUFilterGraphRegistered() {
		c.recordCPUFallbackReason("filter:cpu-fallback")
	}
	src := c.pixmap
	dst := filterPixmapPool.GetForOverwrite(src.Width(), src.Height())
	copy(dst.Data(), src.Data())
	fn(src, dst)
	copy(src.Data(), dst.Data())
	src.NotifyPixelsChanged()
	filterPixmapPool.Put(dst)
	// Layer RT must follow pixmap mutation (L.05 / filter on layer).
	if !c.seedTopLayerGPUFromPixmap() {
		c.noteLayerCPUDraw()
	}
}

// ApplyBlur applies a Gaussian blur to the current surface contents (F.01 / L.04).
// Prefers the GPU multi-RT filter graph when registered (P0-4); otherwise CPU.
// Requires render/filters registration (blank-import).
// ApplyBlur applies a Gaussian blur to the current surface contents (F.01 / L.04).
// Prefers the GPU multi-RT filter graph when registered (P0-4); otherwise CPU.
// Requires render/filters registration (blank-import).
func (c *Context) ApplyBlur(radius float64) {
	if radius <= 0 || blurApply == nil {
		return
	}
	if c.tryApplyFilterGraphGPU(ImageFilterNode{Kind: ImageFilterBlur, Radius: radius}) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		blurApply(src, dst, radius)
	})
}

// ApplyBlurXY applies an anisotropic Gaussian blur (L.04).
// Prefers GPU filter graph when registered (P0-4).
// ApplyBlurXY applies an anisotropic Gaussian blur (L.04).
// Prefers GPU filter graph when registered (P0-4).
func (c *Context) ApplyBlurXY(radiusX, radiusY float64) {
	if (radiusX <= 0 && radiusY <= 0) || blurXYApply == nil {
		return
	}
	if c.tryApplyFilterGraphGPU(ImageFilterNode{Kind: ImageFilterBlurXY, RadiusX: radiusX, RadiusY: radiusY}) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		blurXYApply(src, dst, radiusX, radiusY)
	})
}

// ApplyDropShadow composites a drop shadow under current surface contents (F.02 / L.04).
// Prefers GPU filter graph when registered (P0-4).
// ApplyDropShadow composites a drop shadow under current surface contents (F.02 / L.04).
// Prefers GPU filter graph when registered (P0-4).
func (c *Context) ApplyDropShadow(offsetX, offsetY, blurRadius float64, color RGBA) {
	if dropShadowApply == nil {
		return
	}
	if c.tryApplyFilterGraphGPU(ImageFilterNode{
		Kind: ImageFilterDropShadow, OffsetX: offsetX, OffsetY: offsetY,
		ShadowBlur: blurRadius, ShadowColor: color,
	}) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		dropShadowApply(src, dst, offsetX, offsetY, blurRadius, color)
	})
}

// ApplyColorMatrix applies a 4x5 color transformation matrix (F.04 / L.04).
// Prefers GPU filter graph when registered (P0-4).
// ApplyColorMatrix applies a 4x5 color transformation matrix (F.04 / L.04).
// Prefers GPU filter graph when registered (P0-4).
func (c *Context) ApplyColorMatrix(matrix [20]float32) {
	if colorMatrixApply == nil {
		return
	}
	if c.tryApplyFilterGraphGPU(ImageFilterNode{Kind: ImageFilterColorMatrix, Matrix: matrix}) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		colorMatrixApply(src, dst, matrix)
	})
}

// ApplyGrayscale converts the surface to grayscale via color matrix (F.04 / L.04).
// Prefers GPU filter graph when registered (P0-4).
// ApplyGrayscale converts the surface to grayscale via color matrix (F.04 / L.04).
// Prefers GPU filter graph when registered (P0-4).
func (c *Context) ApplyGrayscale() {
	if grayscaleApply == nil {
		return
	}
	if c.tryApplyFilterGraphGPU(ImageFilterNode{Kind: ImageFilterGrayscale}) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		grayscaleApply(src, dst)
	})
}

// ApplyInvert inverts RGB channels via color matrix (F.04 / L.04).
// Prefers GPU filter graph when registered (P0-4).
// ApplyInvert inverts RGB channels via color matrix (F.04 / L.04).
// Prefers GPU filter graph when registered (P0-4).
func (c *Context) ApplyInvert() {
	if invertApply == nil {
		return
	}
	if c.tryApplyFilterGraphGPU(ImageFilterNode{Kind: ImageFilterInvert}) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		invertApply(src, dst)
	})
}

// syncPublishedFilterBeforeDraw materializes a GPU-published filter result into
// the CPU pixmap before subsequent draws so post-filter geometry participates in
// Image/SavePNG and later GPU flushes (D133 class).
// syncPublishedFilterBeforeDraw materializes a GPU-published filter result into
// the CPU pixmap before subsequent draws so post-filter geometry participates in
// Image/SavePNG and later GPU flushes (D133 class).
func (c *Context) syncPublishedFilterBeforeDraw() {
	if c == nil || !c.pixmapFilterStale {
		return
	}
	if c.materializeFilterGPU() {
		c.clearViewFlushTracking()
	}
}

func (c *Context) clearViewFlushTracking() {
	if c == nil {
		return
	}
	c.viewContentAheadOfPixmap = false
	c.lastFlushedView = gpucontext.TextureView{}
	c.lastFlushedViewW, c.lastFlushedViewH = 0, 0
}

func (c *Context) markViewFlush(view gpucontext.TextureView, w, h int) {
	if c == nil || view.IsNil() || w <= 0 || h <= 0 {
		return
	}
	c.viewContentAheadOfPixmap = true
	c.lastFlushedView = view
	c.lastFlushedViewW, c.lastFlushedViewH = w, h
	// Published filter is no longer the sole surface truth after a view flush.
	// Keep pixmapFilterStale only if we did not just present over it; view flush
	// supersedes for Image() via syncViewFlushIntoPixmap.
}

// syncViewFlushIntoPixmap readbacks the last FlushGPUWithView* target into the
// CPU pixmap so Image/SavePNG match present/offscreen content (D152 class).
// Offscreen present targets are BGRA8; use the swizzling reader first.
// syncViewFlushIntoPixmap readbacks the last FlushGPUWithView* target into the
// CPU pixmap so Image/SavePNG match present/offscreen content (D152 class).
// Offscreen present targets are BGRA8; use the swizzling reader first.
func (c *Context) syncViewFlushIntoPixmap() bool {
	if c == nil || !c.viewContentAheadOfPixmap || c.pixmap == nil {
		return true
	}
	if c.lastFlushedView.IsNil() || c.lastFlushedViewW <= 0 || c.lastFlushedViewH <= 0 {
		c.clearViewFlushTracking()
		return false
	}
	w, h := c.lastFlushedViewW, c.lastFlushedViewH
	if c.pixmap.Width() != w || c.pixmap.Height() != h {
		return false
	}
	need := w * h * 4
	type viewReaderBGRA interface {
		ReadbackViewRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	}
	type viewReaderStraight interface {
		ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	}
	raw := c.GPURenderContext()
	var rgba []byte
	var err error
	if br, ok := raw.(viewReaderBGRA); ok && br != nil {
		rgba, err = br.ReadbackViewRGBA(c.lastFlushedView, w, h)
	}
	if err != nil || len(rgba) < need {
		if sr, ok := raw.(viewReaderStraight); ok && sr != nil {
			rgba, err = sr.ReadbackViewStraightRGBA(c.lastFlushedView, w, h)
		}
	}
	if err != nil || len(rgba) < need {
		return false
	}
	copy(c.pixmap.Data()[:need], rgba[:need])
	c.pixmap.NotifyPixelsChanged()
	c.clearViewFlushTracking()
	c.pixmapFilterStale = false
	return true
}

// tryApplyFilterGraphGPU runs nodes on the registered GPU multi-RT filter graph.
// Returns true when the GPU path fully applied the result (P0-4 / L.04).
//
// Prefer the texture-publish path (no Map/readback) so continuous effect RTs can
// composite via DrawGPUTexture. Pixmap is marked stale and materialised lazily on
// Image/Export/SavePNG. When a layer stack is active, pixels are materialised
// immediately so PopLayer seed/composite stay coherent.
// readbackFilterSrcIntoScratch copies a filter seed RT into filterSrcScratch.
// Used only as R7.2 recovery when FromView fails after FlushGPUWithView.
func (c *Context) readbackFilterSrcIntoScratch(srcView gpucontext.TextureView, w, h, need int) bool {
	if c == nil || srcView.IsNil() || need <= 0 {
		return false
	}
	type viewReader interface {
		ReadbackViewRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	}
	raw := c.GPURenderContext()
	vr, ok := raw.(viewReader)
	if !ok || vr == nil {
		return false
	}
	// filterSrcRT is BGRA offscreen — use swizzling readback.
	rgba, err := vr.ReadbackViewRGBA(srcView, w, h)
	if err != nil || len(rgba) < need {
		return false
	}
	copy(c.filterSrcScratch[:need], rgba[:need])
	return true
}

// FiltersRegistered reports whether image filter ops were registered.
