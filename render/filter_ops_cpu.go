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
	erode func(src, dst *Pixmap, radius int),
	dilate func(src, dst *Pixmap, radius int),
	offset func(src, dst *Pixmap, dx, dy int),
	alphaThreshold func(src, dst *Pixmap, threshold uint8),
	lighting func(src, dst *Pixmap, lightPos [3]float32, color RGBA, kd, surfaceScale float32),
	tile func(src, dst *Pixmap, srcRect [4]float32),
	displacement func(src, dst *Pixmap, xSel, ySel, scale int, disp *ImageBuf),
) {
	blurApply = blur
	blurXYApply = blurXY
	dropShadowApply = shadow
	colorMatrixApply = colorMatrix
	grayscaleApply = grayscale
	invertApply = invert
	erodeApply = erode
	dilateApply = dilate
	offsetApply = offset
	alphaThresholdApply = alphaThreshold
	lightingApply = lighting
	tileApply = tile
	displacementApply = displacement
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
	// The full surface now lives in the pixmap.
	c.pixmapBaseContent = true
	// Skia crops a saveLayer filter's output to the layer bounds: keep a
	// bounded backdrop layer's outside-bounds pixels equal to the parent's
	// before the RT follows the pixmap (see cropFilterOutputToLayerBounds).
	c.cropFilterOutputToLayerBounds()
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
		c.markLayerFullComposite()
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		blurApply(src, dst, radius)
	})
	c.markLayerFullComposite()
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
		c.markLayerFullComposite()
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		blurXYApply(src, dst, radiusX, radiusY)
	})
	c.markLayerFullComposite()
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
		c.markLayerFullComposite()
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		dropShadowApply(src, dst, offsetX, offsetY, blurRadius, color)
	})
	c.markLayerFullComposite()
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
	c.pixmapBaseContent = true
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

// ApplyErode morphologically shrinks bright regions by the radius (P3.4).
// GPU-first via the filter graph, CPU fallback otherwise.
func (c *Context) ApplyErode(radius int) {
	if radius <= 0 {
		return
	}
	nodes := []ImageFilterNode{{Kind: ImageFilterErode, MorphRadius: radius}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if erodeApply != nil {
			erodeApply(src, dst, radius)
		}
	})
	c.markLayerFullComposite()
}

// ApplyDilate morphologically grows bright regions by the radius (P3.4).
func (c *Context) ApplyDilate(radius int) {
	if radius <= 0 {
		return
	}
	nodes := []ImageFilterNode{{Kind: ImageFilterDilate, MorphRadius: radius}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if dilateApply != nil {
			dilateApply(src, dst, radius)
		}
	})
	c.markLayerFullComposite()
}

// ApplyOffset shifts content by whole pixels (P3.4).
func (c *Context) ApplyOffset(dx, dy int) {
	if dx == 0 && dy == 0 {
		return
	}
	nodes := []ImageFilterNode{{Kind: ImageFilterOffset, IntOffsetX: dx, IntOffsetY: dy}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if offsetApply != nil {
			offsetApply(src, dst, dx, dy)
		}
	})
	c.markLayerFullComposite()
}

// ApplyAlphaThreshold zeroes pixels whose alpha is below the threshold (P3.4).
func (c *Context) ApplyAlphaThreshold(threshold uint8) {
	nodes := []ImageFilterNode{{Kind: ImageFilterAlphaThreshold, AlphaThreshold: threshold}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if alphaThresholdApply != nil {
			alphaThresholdApply(src, dst, threshold)
		}
	})
	c.markLayerFullComposite()
}

// ApplyLighting lights the image with a point diffuse source over the alpha
// height map (P3.4, Skia LightingImageFilter). GPU-first, CPU fallback.
func (c *Context) ApplyLighting(lightPos [3]float32, color RGBA, kd, surfaceScale float32) {
	if kd <= 0 {
		return
	}
	nodes := []ImageFilterNode{{
		Kind: ImageFilterLighting, LightPos: lightPos, LightColor: color,
		LightKD: kd, SurfaceScale: surfaceScale,
	}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if lightingApply != nil {
			lightingApply(src, dst, lightPos, color, kd, surfaceScale)
		}
	})
	c.markLayerFullComposite()
}

// ApplyTile repeats the source tile rect across the output (P3.4, Skia
// ImageFilters::Tile). CPU-first (batch-2 scope).
func (c *Context) ApplyTile(srcRect [4]float32) {
	if srcRect[2] <= srcRect[0] || srcRect[3] <= srcRect[1] {
		return
	}
	nodes := []ImageFilterNode{{Kind: ImageFilterTile, TileSrcRect: srcRect}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if tileApply != nil {
			tileApply(src, dst, srcRect)
		}
	})
	c.markLayerFullComposite()
}

// ApplyMerge composites N independent filter layers source-over onto the
// current content (P3.4, SkImageFilters::Merge semantics): every layer runs
// as its own filter graph from a copy of the base, then the layer outputs
// stack over the base in order. N layers are supported — the composition
// lives at the orchestration level, not in the linear ping-pong graph.
func (c *Context) ApplyMerge(layers ...[]ImageFilterNode) {
	if len(layers) == 0 || c.pixmap == nil {
		return
	}
	_ = c.FlushGPU()
	base := c.pixmap
	for _, layer := range layers {
		if len(layer) == 0 {
			continue
		}
		layerPix := filterPixmapPool.GetForOverwrite(base.Width(), base.Height())
		copy(layerPix.Data(), base.Data())
		if !c.tryApplyFilterGraphGPU(layer...) {
			// CPU fallback for this layer: run every node sequentially.
			work := filterPixmapPool.GetForOverwrite(base.Width(), base.Height())
			copy(work.Data(), layerPix.Data())
			for _, n := range layer {
				if !imageFilterNodeRunnable(n) {
					continue
				}
				next := filterPixmapPool.GetForOverwrite(base.Width(), base.Height())
				copy(next.Data(), work.Data())
				applyImageFilterNode(n, work, next)
				copy(work.Data(), next.Data())
				filterPixmapPool.Put(next)
			}
			copy(layerPix.Data(), work.Data())
			filterPixmapPool.Put(work)
		}
		mergeOver(layerPix, base)
		filterPixmapPool.Put(layerPix)
	}
	c.markLayerFullComposite()
}

// ApplyDisplacementMap resamples the content using a displacement map: for
// each pixel, the map's selected R/G channels (xSel/ySel, R=0) provide a
// signed offset in [-scale/2, scale/2] pixels (Skia
// SkImageFilters::DisplacementMap semantics, clamp-to-edge sampling).
// GPU-first via the aux texture; CPU fallback below.
func (c *Context) ApplyDisplacementMap(xSel, ySel, scale int, displacement *ImageBuf) {
	if displacement == nil || scale == 0 {
		return
	}
	nodes := []ImageFilterNode{{
		Kind:              ImageFilterDisplacement,
		DispXChannel:      xSel,
		DispYChannel:      ySel,
		DispScale:         scale,
		DisplacementImage: displacement,
	}}
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		if displacementApply != nil {
			displacementApply(src, dst, xSel, ySel, scale, displacement)
		}
	})
	c.markLayerFullComposite()
}
