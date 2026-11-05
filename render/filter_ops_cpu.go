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
	"github.com/energye/gpui/gpu/types"
)

func FilterPoolStats() (gets, puts, hits, misses int) {
	return filterPixmapPool.Stats()
}

// ResetFilterPoolStats clears filter pool counters (tests).
// ResetFilterPoolStats clears filter pool counters (tests).
func ResetFilterPoolStats() {
	filterPixmapPool.ResetStats()
}

// RegisterFilterOps wires the CPU image-filter implementations (see
// FilterOpImpls). Called from render/filters init to avoid import cycles with
// internal/filter.
func RegisterFilterOps(impls FilterOpImpls) {
	blurApply = impls.Blur
	blurXYApply = impls.BlurXY
	dropShadowApply = impls.DropShadow
	colorMatrixApply = impls.ColorMatrix
	grayscaleApply = impls.Grayscale
	invertApply = impls.Invert
	erodeApply = impls.Erode
	dilateApply = impls.Dilate
	offsetApply = impls.Offset
	alphaThresholdApply = impls.AlphaThreshold
	lightingApply = impls.Lighting
	tileApply = impls.Tile
	displacementApply = impls.Displacement
	bloomApply = impls.Bloom
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
//
// 通道顺序由那张面自己的格式决定（见 viewChannelOrder，M6 修复点）：RGBA8 直取、
// BGRA8 才翻。旧写法把「离屏呈现目标都是 BGRA8」当普适事实——先试翻转读口、只在
// 报错或长度不足时退回直取，而 SetEffectSurface 那条路发布的正是 RGBA8Unorm，
// 于是被多翻一次，导出面红蓝整体互换（绿与 alpha 不动、白底不变，肉眼看不出来）。
// viewChannelOrder 是一条离屏面的通道顺序真源：问那张纹理自己，不问调用点。
type viewChannelOrder uint8

const (
	viewOrderUnknown viewChannelOrder = iota
	viewOrderRGBA
	viewOrderBGRA
)

// 唯一的「谁负责 swizzle」判定处（M6）：RGBA8/SRGB8 直取，BGRA8 翻转，
// 其余一律 Unknown —— 由调用方 fail-closed，不再靠「先试一个、失败退另一个」碰运气。
func channelOrderOfView(view gpucontext.TextureView) viewChannelOrder {
	if view.IsNil() {
		return viewOrderUnknown
	}
	// gpu/context 的 TextureView 是盒子化的不透明句柄，格式只能经正规出口取回 hal 接口问。
	halv := gpucontext.UnpackTextureView(view)
	if halv == nil {
		return viewOrderUnknown
	}
	tex := halv.Texture()
	if tex == nil {
		return viewOrderUnknown
	}
	switch tex.Format() {
	case types.TextureFormatRGBA8Unorm, types.TextureFormatRGBA8UnormSrgb:
		return viewOrderRGBA
	case types.TextureFormatBGRA8Unorm, types.TextureFormatBGRA8UnormSrgb:
		return viewOrderBGRA
	default:
		return viewOrderUnknown
	}
}

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
	switch channelOrderOfView(c.lastFlushedView) {
	case viewOrderRGBA:
		if sr, ok := raw.(viewReaderStraight); ok && sr != nil {
			rgba, err = sr.ReadbackViewStraightRGBA(c.lastFlushedView, w, h)
		}
	case viewOrderBGRA:
		if br, ok := raw.(viewReaderBGRA); ok && br != nil {
			rgba, err = br.ReadbackViewRGBA(c.lastFlushedView, w, h)
		}
	default:
		// 未知格式：不猜通道顺序（猜就是 M6 的形状），同步失败让调用方保留 GPU 面，
		// 宁可读到上一帧也不读出错色。产出方应声明格式，见 evidence/P3.7/m6-export-byteorder.md。
		return false
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
	// Every layer derives from the PRISTINE input (Skia Merge semantics:
	// all srcs read the same input stream) — a layer must not see earlier
	// layers' over results.
	pristine := filterPixmapPool.GetForOverwrite(base.Width(), base.Height())
	copy(pristine.Data(), base.Data())
	for _, layer := range layers {
		if len(layer) == 0 {
			continue
		}
		layerPix := filterPixmapPool.GetForOverwrite(base.Width(), base.Height())
		copy(layerPix.Data(), pristine.Data())
		// CPU-only layer execution: the GPU graph seeds from the current
		// GPU content (FromView), which would leak earlier layers into
		// this layer — per-layer GPU seeding is a registered follow-up.
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
		mergeOver(layerPix, base)
		filterPixmapPool.Put(layerPix)
	}
	filterPixmapPool.Put(pristine)
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

// ApplyBloom glows the bright areas of the current surface (P3.7 batch 5):
// soft-knee bright pass, separable Gaussian, additive back. The GPU filter
// graph runs it as four passes; without the graph the CPU node applies the
// same render/internal/filter kernel, so both legs agree by construction.
//
// Surface storage is RGBA8, so the summed light clamps at 1 on this leg — an
// explicit LDR approximation, not HDR. The float-domain entry that keeps
// values above 1 as light is render/hdr.FloatTarget.Bloom.
//
// An invalid or identity BloomParams changes nothing (identity never enters
// the graph). Requires render/filters registration (blank-import).
func (c *Context) ApplyBloom(p BloomParams) {
	if c == nil || c.pixmap == nil || bloomApply == nil {
		return
	}
	if err := p.Validate(); err != nil || p.IsIdentity() {
		return
	}
	node := ImageFilterNode{Kind: ImageFilterBloom, Bloom: p}
	if c.tryApplyFilterGraphGPU(node) {
		c.markLayerFullComposite()
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		bloomApply(src, dst, p)
	})
	c.markLayerFullComposite()
}

// SetPaintMaskFilter sets the ambient paint's mask blur radius (P3.4 paint
// level slot): subsequent fills soften their edges by this radius.
func (c *Context) SetPaintMaskFilter(radius float64) {
	if c == nil || c.paint == nil {
		return
	}
	c.paint.MaskFilterRadius = radius
}
