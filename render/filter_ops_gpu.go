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

// gpuSeededFilterFlusher is implemented by the GPU context's combined
// mesh-seed + filter encoder when it can upload CPU base pixels into the seed
// RT before replaying pending draws (see FlushAndFilterFromView).
type gpuSeededFilterFlusher interface {
	FlushAndFilterFromView(srcView gpucontext.TextureView, seed []byte, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)
}

// tryApplyFilterGraphGPU runs nodes on the registered GPU multi-RT filter graph.
// Returns true when the GPU path fully applied the result (P0-4 / L.04).
//
// Prefer the texture-publish path (no Map/readback) so continuous effect RTs can
// composite via DrawGPUTexture. Pixmap is marked stale and materialised lazily on
// Image/Export/SavePNG. When a layer stack is active, pixels are materialised
// immediately so PopLayer seed/composite stay coherent.
func (c *Context) tryApplyFilterGraphGPU(nodes ...ImageFilterNode) bool {
	if c == nil || c.pixmap == nil || len(nodes) == 0 {
		return false
	}
	// WithCPUOnly 契约：整条链只走软件光栅。与 doFill/doStroke 的 c.cpuOnly
	// 门同类——CPUOnly 离屏（SVG 烘焙的 tile/mask/filter 腿）不能把滤镜图
	// 交给 GPU：pixmap 从未上传，GPU 结果回贴后滤镜等于没应用。
	if CPUOnlyMode() || c.cpuOnly {
		return false
	}
	if gpuFilterGraphApplyTexture == nil && gpuFilterGraphApply == nil {
		return false
	}
	nodes = coalesceImageFilterNodes(nodes)
	if len(nodes) == 0 || !imageFilterGraphGPUSupported(nodes) {
		return false
	}
	// Ensure at least one runnable node with registered CPU ops (GPU graph
	// still requires filters package for support checks / fallback policy).
	runnable := 0
	for i := range nodes {
		if imageFilterNodeRunnable(nodes[i]) {
			runnable++
		}
	}
	if runnable == 0 {
		return false
	}

	src := c.pixmap
	w, h := src.Width(), src.Height()
	if w <= 0 || h <= 0 {
		return false
	}
	hasLayer := c.layerStack != nil && len(c.layerStack.layers) > 0

	// P4.3 stage-1 timing (env-gated, off by default).
	tOps := p43Now()
	tLayer := p43Since(tOps)
	defer func() {
		if p43TimingOn {
			p43Log("ops w=%d h=%d hasLayer=%v layer=%.3fms total=%.3fms",
				w, h, hasLayer, p43Ms(tLayer), p43Ms(p43Since(tOps)))
		}
	}()

	finish := func(view gpucontext.TextureView, release func()) bool {
		if view.IsNil() {
			if release != nil {
				release()
			}
			return false
		}
		c.attachFilterGPUResult(view, w, h, release)
		c.pixmapFilterStale = true
		if hasLayer {
			tL0 := p43Now()
			if c.materializeFilterGPU() {
				// Skia crops a saveLayer filter's output to the layer bounds:
				// restore the parent's pixels outside a bounded backdrop
				// layer's clip before the RT follows the pixmap.
				c.cropFilterOutputToLayerBounds()
				if !c.seedTopLayerGPUFromPixmap() {
					c.noteLayerCPUDraw()
				}
			} else {
				c.noteLayerCPUDraw()
			}
			tLayer = p43Since(tL0)
		}
		c.recordGPUOp()
		return true
	}

	need := w * h * 4
	if cap(c.filterSrcScratch) < need {
		c.filterSrcScratch = make([]byte, need)
	}
	c.filterSrcScratch = c.filterSrcScratch[:need]

	// 1) Zero-readback: flush pending draws into retained filterSrcRT, then
	// GPU→GPU filter. Glow/effect RTs hit this every recompute (pending > 0).
	// If we already flushed to filterSrcRT, never seed path 2/3 from the stale
	// CPU pixmap (draws never reached pixmap) — recover via src RT readback only.
	//
	// Before seeding, materialize any prior published filter and sync view-flush
	// content into the pixmap so intermediate FlushGPU/Present content is not
	// dropped when only new ops remain pending (D105/D140 kitchen-sink pattern).
	_ = c.syncViewFlushIntoPixmap()
	_ = c.materializeFilterGPU()

	pending := 0
	if rc := c.gpuCtxOps(); rc != nil {
		type pcounter interface{ PendingCount() int }
		if pc, ok := rc.(pcounter); ok {
			pending = pc.PendingCount()
		}
	}
	// Mid-frame surface materialization (nil-view FlushGPU *or* advanced-blend
	// internal Flush into pixmap Data) leaves prior content only in the pixmap
	// while new ops remain pending. Fold pending into pixmap and take path-2 so
	// we never filter a Clear / badge-only / post-Multiply fragment surface
	// (D105/D140). Pure pending frames (typical glow) keep zero-copy path-1
	// without a full-surface WriteTexture seed.
	gpuMid := false
	if raw := c.GPURenderContext(); raw != nil {
		type mid interface {
			MidFrameDataFlush() bool
			ClearMidFrameDataFlush()
		}
		if m, ok := raw.(mid); ok && m != nil && m.MidFrameDataFlush() {
			gpuMid = true
			m.ClearMidFrameDataFlush()
		}
	}
	if c.midFrameNilFlush || gpuMid {
		if !hasLayer && pending > 0 {
			// A prior readback made the pixmap authoritative; fold the new
			// pending draws there too. FlushGPU is not used here because an
			// effect surface would instead publish a base-less RT.
			// With an active layer RT there is no base pixmap to fold into:
			// folding returned false and the filter fell back to CPU while the
			// pending commands stayed on the layer. Keep the layer-coherent GPU
			// path instead (the seed is flushed by the layer session).
			if !c.foldPendingIntoPixmap() {
				return false
			}
			pending = 0
		}
		c.midFrameNilFlush = false
	}
	// CPU base content is part of the surface an image filter sees (Skia
	// filter-layer semantics). The zero-readback path must upload it into the
	// seed RT before replaying pending draws; otherwise its first pass clears
	// the base (e.g. transparent corners over an opaque ClearWithColor).
	seedBase := c.pixmapBaseContent && !hasLayer
	if seedBase && pending > 0 {
		if raw := c.GPURenderContext(); raw != nil {
			if _, ok := raw.(gpuSeededFilterFlusher); !ok {
				// No seeded combined flush: fold pending into the pixmap and
				// use the pixmap-coherent upload path below.
				if !c.foldPendingIntoPixmap() {
					return false
				}
				pending = 0
				seedBase = false
			}
		}
	}
	var seed []byte
	if seedBase {
		seed = src.Data()[:need]
	}
	flushedToSrc := false
	if pending > 0 && gpuFilterGraphApplyFromView != nil {
		if srcView, ok := c.ensureFilterSrcRT(w, h); ok {
			// seed (nil on pure-GPU frames) uploads CPU base pixels into the RT
			// before pending draws replay; a full-surface upload only happens when
			// the pixmap actually holds base content (not the glow hot path).
			// single Queue.Submit for mesh seed + filter (when GPU context
			// supports FlushAndFilterFromView). Pure submit coalesce; pixels same.
			if raw := c.GPURenderContext(); raw != nil {
				if ff, ok := raw.(gpuSeededFilterFlusher); ok {
					view, release, err := ff.FlushAndFilterFromView(srcView, seed, w, h, nodes)
					if err == nil && finish(view, release) {
						// Mesh seed flush is internal to FlushAndFilterFromView (bypasses
						// FlushGPUWithView); count it for FrameFlushes observability.
						c.recordFrameFlush()
						return true
					}
					if release != nil {
						release()
					}
					// Pending consumed by combined path: do not re-Flush empty queues.
					// Mesh was submitted on recovery inside FlushAndFilterFromView when
					// possible; try FromView/readback only.
					pendingLeft := 0
					if rc2 := c.gpuCtxOps(); rc2 != nil {
						type pcounter interface{ PendingCount() int }
						if pc, ok := rc2.(pcounter); ok {
							pendingLeft = pc.PendingCount()
						}
					}
					if pendingLeft == 0 {
						flushedToSrc = true
						view2, rel2, err2 := gpuFilterGraphApplyFromView(srcView, w, h, nodes)
						if err2 == nil && finish(view2, rel2) {
							return true
						}
						if rel2 != nil {
							rel2()
						}
						if c.readbackFilterSrcIntoScratch(srcView, w, h, need) {
							if gpuFilterGraphApplyTexture != nil {
								view3, rel3, err3 := gpuFilterGraphApplyTexture(c.filterSrcScratch, w, h, nodes)
								if err3 == nil && finish(view3, rel3) {
									return true
								}
								if rel3 != nil {
									rel3()
								}
							}
						}
						return false
					}
					// pending still present — fall through to classic two-submit path
				}
			}
			// Classic two-submit path. When the pixmap holds base content this
			// flush would clear the seed RT and drop it, so skip straight to the
			// pixmap-coherent path instead.
			if !seedBase {
				if err := c.FlushGPUWithView(srcView, uint32(w), uint32(h)); err == nil { //nolint:gosec
					flushedToSrc = true
					view, release, err := gpuFilterGraphApplyFromView(srcView, w, h, nodes)
					if err == nil && finish(view, release) {
						return true
					}
					if release != nil {
						release()
					}
					// FromView failed after a successful seed flush: seed texture/pixel
					// paths from filterSrcRT (correct content), not from stale pixmap.
					if c.readbackFilterSrcIntoScratch(srcView, w, h, need) {
						if gpuFilterGraphApplyTexture != nil {
							view, release, err := gpuFilterGraphApplyTexture(c.filterSrcScratch, w, h, nodes)
							if err == nil && finish(view, release) {
								return true
							}
							if release != nil {
								release()
							}
						}
						if gpuFilterGraphApply != nil {
							out, err := gpuFilterGraphApply(c.filterSrcScratch, w, h, nodes)
							if err == nil && len(out) >= need {
								copy(src.Data(), out[:need])
								src.NotifyPixelsChanged()
								c.pixmapFilterStale = false
								c.releaseFilterGPUResult()
								if !c.seedTopLayerGPUFromPixmap() {
									c.noteLayerCPUDraw()
								}
								c.recordGPUOp()
								return true
							}
						}
					}
					// Recovery failed — do not fall through to pixmap seed.
					return false
				}
			}
		}
	}

	// 2) Pixmap-coherent path: only when we did NOT already flush draws solely
	// to filterSrcRT. FlushGPU may readback pending GPU draws into the pixmap.
	if !flushedToSrc {
		if pending > 0 {
			// Fold remaining draws into the pixmap before uploading, including
			// effect surfaces (whose FlushGPU publishes instead of reading back).
			if !c.foldPendingIntoPixmap() {
				return false
			}
			pending = 0
		}
		_ = c.FlushGPU()
		copy(c.filterSrcScratch, src.Data()[:need])

		if gpuFilterGraphApplyTexture != nil {
			view, release, err := gpuFilterGraphApplyTexture(c.filterSrcScratch, w, h, nodes)
			if err == nil && finish(view, release) {
				return true
			}
			if release != nil {
				release()
			}
		}

		// 3) Pixel readback path (tests / environments without texture publish).
		if gpuFilterGraphApply == nil {
			return false
		}
		out, err := gpuFilterGraphApply(c.filterSrcScratch, w, h, nodes)
		if err != nil || len(out) < need {
			return false
		}
		copy(src.Data(), out[:need])
		src.NotifyPixelsChanged()
		c.pixmapFilterStale = false
		c.releaseFilterGPUResult()
		if !c.seedTopLayerGPUFromPixmap() {
			c.noteLayerCPUDraw()
		}
		c.recordGPUOp()
		return true
	}
	return false
}

// foldPendingIntoPixmap resolves pending GPU draws into the CPU pixmap with a
// nil-view flush, bypassing the effect-surface publish shortcut. The pixmap is
// then the complete surface (CPU base + GPU draws), which the upload filter
// path needs. Reports false when pending work could not be resolved.
func (c *Context) foldPendingIntoPixmap() bool {
	if c == nil || c.pixmap == nil {
		return false
	}
	if rc := c.gpuCtxOps(); rc != nil {
		t := c.gpuRenderTarget()
		if !t.View.IsNil() {
			// Active layer RT: the layer-coherent path owns the flush target.
			return false
		}
		c.recordFrameFlush()
		if err := rc.Flush(t); err != nil {
			return false
		}
		if pc, ok := rc.(interface{ PendingCount() int }); ok && pc.PendingCount() != 0 {
			return false
		}
		c.applyDitherIfEnabled()
		c.clearViewFlushTracking()
		c.midFrameNilFlush = true
		c.pixmapBaseContent = true
		if c.effectSurface {
			c.pixmapFilterStale = false
		}
		return true
	}
	if a := Accelerator(); a != nil {
		t := c.gpuRenderTarget()
		if !t.View.IsNil() {
			return false
		}
		c.recordFrameFlush()
		c.warnGPUFallback("foldPendingIntoPixmap")
		if err := a.Flush(t); err != nil {
			return false
		}
		c.applyDitherIfEnabled()
		c.midFrameNilFlush = true
		c.pixmapBaseContent = true
		return true
	}
	return false
}

// readbackFilterSrcIntoScratch copies a filter seed RT into filterSrcScratch.
// Used only as R7.2 recovery when FromView fails after FlushGPUWithView.
// FiltersRegistered reports whether image filter ops were registered.
func FiltersRegistered() bool {
	return blurApply != nil && dropShadowApply != nil && grayscaleApply != nil
}

// RegisterGPUFilterGraph wires an optional GPU multi-RT image filter graph (F.03).
// When set, ApplyImageFilterGraph prefers the GPU path for supported node sets.
// RegisterGPUFilterGraph wires an optional GPU multi-RT image filter graph (F.03).
// When set, ApplyImageFilterGraph prefers the GPU path for supported node sets.
func RegisterGPUFilterGraph(fn func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)) {
	gpuFilterGraphApply = fn
}

// RegisterGPUFilterGraphTexture wires a zero-readback GPU filter publisher.
// Result textures are composited with DrawGPUTexture; CPU pixels materialise lazily.
// RegisterGPUFilterGraphTexture wires a zero-readback GPU filter publisher.
// Result textures are composited with DrawGPUTexture; CPU pixels materialise lazily.
func RegisterGPUFilterGraphTexture(fn func(src []byte, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)) {
	gpuFilterGraphApplyTexture = fn
}

// RegisterGPUFilterGraphFromView wires a GPU→GPU filter path seeded from a texture view.
// RegisterGPUFilterGraphFromView wires a GPU→GPU filter path seeded from a texture view.
func RegisterGPUFilterGraphFromView(fn func(srcView gpucontext.TextureView, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)) {
	gpuFilterGraphApplyFromView = fn
}

// UnregisterGPUFilterGraphs clears the pixel/texture/from-view GPU filter hooks.
// The registering GPUShared calls it on Close (same discipline as the
// Unregister{PurgeEvictable,GPUPoolStatsCollector,ShaderEffectEvictor} trio):
// the hook closures capture that shared, and leaving them registered kept a
// closed shared alive globally — the next filter op resurrected it, installed
// a fresh device on it, and the published view then crossed devices at
// readback (H20). After unregistering, GPUFilterGraphRegistered reports false
// and filter ops fall back to CPU until a live shared re-registers.
func UnregisterGPUFilterGraphs() {
	gpuFilterGraphApply = nil
	gpuFilterGraphApplyTexture = nil
	gpuFilterGraphApplyFromView = nil
}

// GPUFilterGraphRegistered reports whether a GPU multi-RT filter graph is wired.
// GPUFilterGraphRegistered reports whether a GPU multi-RT filter graph is wired.
func GPUFilterGraphRegistered() bool {
	return gpuFilterGraphApply != nil || gpuFilterGraphApplyTexture != nil || gpuFilterGraphApplyFromView != nil
}

// SwapGPUFilterGraph replaces the pixel-readback GPU filter graph and returns the previous one (tests).
// Note: alone this does NOT disable texture/from-view publishers — use
// DisableGPUFilterGraphForTest to force the pure CPU filter path.
// SwapGPUFilterGraph replaces the pixel-readback GPU filter graph and returns the previous one (tests).
// Note: alone this does NOT disable texture/from-view publishers — use
// DisableGPUFilterGraphForTest to force the pure CPU filter path.
func SwapGPUFilterGraph(fn func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)) func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error) {
	prev := gpuFilterGraphApply
	gpuFilterGraphApply = fn
	return prev
}

// DisableGPUFilterGraphForTest clears pixel/texture/from-view GPU filter hooks so
// ApplyBlur/ApplyImageFilterGraph exercise the CPU intermediate pool.
// Returns a restore function that reinstalls the previous hooks.
// DisableGPUFilterGraphForTest clears pixel/texture/from-view GPU filter hooks so
// ApplyBlur/ApplyImageFilterGraph exercise the CPU intermediate pool.
// Returns a restore function that reinstalls the previous hooks.
func DisableGPUFilterGraphForTest() (restore func()) {
	prevApply := gpuFilterGraphApply
	prevTex := gpuFilterGraphApplyTexture
	prevView := gpuFilterGraphApplyFromView
	gpuFilterGraphApply = nil
	gpuFilterGraphApplyTexture = nil
	gpuFilterGraphApplyFromView = nil
	return func() {
		gpuFilterGraphApply = prevApply
		gpuFilterGraphApplyTexture = prevTex
		gpuFilterGraphApplyFromView = prevView
	}
}

// ImageFilterKind identifies a node in an image-filter graph (F.03).
type ImageFilterKind int

const (
	// ImageFilterBlur is isotropic Gaussian blur.
	ImageFilterBlur ImageFilterKind = iota
	// ImageFilterBlurXY is anisotropic Gaussian blur.
	ImageFilterBlurXY
	// ImageFilterDropShadow composites a drop shadow under content.
	ImageFilterDropShadow
	// ImageFilterColorMatrix applies a 4x5 color matrix.
	ImageFilterColorMatrix
	// ImageFilterGrayscale converts to grayscale.
	ImageFilterGrayscale
	// ImageFilterInvert inverts RGB channels.
	ImageFilterInvert
	// ImageFilterErode is morphological minimum (P3.4).
	ImageFilterErode
	// ImageFilterDilate is morphological maximum (P3.4).
	ImageFilterDilate
	// ImageFilterOffset translates content by whole pixels (P3.4).
	ImageFilterOffset
	// ImageFilterAlphaThreshold zeroes alpha below a threshold (P3.4).
	ImageFilterAlphaThreshold
	// ImageFilterLighting is point-source diffuse lighting over the source
	// alpha height map (P3.4, Skia LightingImageFilter).
	ImageFilterLighting
	// ImageFilterTile repeats the source tile rect across the output (P3.4,
	// Skia ImageFilters::Tile).
	ImageFilterTile
	// ImageFilterBloom glows the bright areas: bright pass, separable
	// Gaussian, additive composite (P3.7 batch 5, four passes on the GPU leg;
	// RGBA8 storage means the additive result clamps — LDR approximation).
	ImageFilterBloom
	// ImageFilterDisplacement resamples the source by a displacement map's
	// selected channels (P3.4, Skia SkImageFilters::DisplacementMap). It
	// needs the aux texture slot for the map, so it rides the GPU graph
	// only when a displacement image is supplied.
	ImageFilterDisplacement
)

// ImageFilterNode is one node in a multi-pass image filter graph (F.03).
// Unused fields for a given Kind are ignored.
type ImageFilterNode struct {
	Kind ImageFilterKind

	// Blur / BlurXY
	Radius  float64
	RadiusX float64
	RadiusY float64

	// DropShadow
	OffsetX, OffsetY float64
	ShadowBlur       float64
	ShadowColor      RGBA

	// ColorMatrix
	Matrix [20]float32

	// Erode/Dilate/Offset/AlphaThreshold (P3.4): MorphRadius/MorphRadiusY for
	// morphology (Y falls back to MorphRadius when 0 — SVG feMorphology takes
	// separate radii), IntOffsetX/Y for offset, AlphaThreshold for the cutoff.
	// The integer offset avoids float rounding at whole-pixel granularity.
	MorphRadius    int
	MorphRadiusY   int
	IntOffsetX     int
	IntOffsetY     int
	AlphaThreshold uint8

	// Lighting (P3.4): point light position (z = height above the plane),
	// light color, diffuse constant, surface scale for the alpha height map.
	LightPos     [3]float32
	LightColor   RGBA
	LightKD      float32
	SurfaceScale float32
	// Tile (P3.4): source rect (x0, y0, x1, y1) repeated across the output.
	TileSrcRect [4]float32

	// Displacement (P3.4): channel selectors (0=R, 1=G, 2=B, 3=A), the
	// offset scale in pixels, and the map image.
	// Bloom (P3.7 batch 5): the one parameter struct all three legs read.
	Bloom BloomParams

	DispXChannel      int
	DispYChannel      int
	DispScale         int
	DisplacementImage *ImageBuf
}

// ApplyImageFilterGraph runs a multi-node image filter graph with intermediate
// surface ping-pong.
//
// Pipeline:
//  1. FlushGPU so the graph operates on real GPU-produced pixels when content
//     was drawn on the native path.
//  2. Apply nodes in order using two full-size intermediate pixmaps (ping-pong),
//     matching multi-RT filter graphs without in-place clobber.
//  3. Write the final result back to the context surface.
//
// Requires render/filters registration (blank-import).
// ApplyImageFilterGraph runs a multi-node image filter graph with intermediate
// surface ping-pong.
//
// Pipeline:
//  1. FlushGPU so the graph operates on real GPU-produced pixels when content
//     was drawn on the native path.
//  2. Apply nodes in order using two full-size intermediate pixmaps (ping-pong),
//     matching multi-RT filter graphs without in-place clobber.
//  3. Write the final result back to the context surface.
//
// Requires render/filters registration (blank-import).
func (c *Context) ApplyImageFilterGraph(nodes ...ImageFilterNode) {
	if c == nil || c.pixmap == nil || len(nodes) == 0 {
		return
	}
	runnable := 0
	for i := range nodes {
		if imageFilterNodeRunnable(nodes[i]) {
			runnable++
		}
	}
	if runnable == 0 {
		return
	}
	nodes = coalesceImageFilterNodes(nodes)

	// F.03 / P0-4: GPU multi-RT ping-pong when all nodes are GPU-supported.
	if c.tryApplyFilterGraphGPU(nodes...) {
		return
	}

	src := c.pixmap
	w, h := src.Width(), src.Height()
	if w <= 0 || h <= 0 {
		return
	}

	// CPU fallback: pooled pixmap ping-pong intermediate surfaces.
	if c.gpuPathAvailable() && GPUFilterGraphRegistered() {
		c.recordCPUFallbackReason("filter:graph-cpu")
	}
	_ = c.FlushGPU()
	bufA := filterPixmapPool.GetForOverwrite(w, h)
	bufB := filterPixmapPool.GetForOverwrite(w, h)
	copy(bufA.Data(), src.Data())
	bufA.NotifyPixelsChanged()

	cur, nxt := bufA, bufB
	for i := range nodes {
		n := nodes[i]
		if !imageFilterNodeRunnable(n) {
			continue
		}
		// Seed next buffer with current so partial writes keep prior content.
		copy(nxt.Data(), cur.Data())
		nxt.NotifyPixelsChanged()
		applyImageFilterNode(n, cur, nxt)
		nxt.NotifyPixelsChanged()
		cur, nxt = nxt, cur
	}

	copy(src.Data(), cur.Data())
	src.NotifyPixelsChanged()
	filterPixmapPool.Put(bufA)
	filterPixmapPool.Put(bufB)
	if !c.seedTopLayerGPUFromPixmap() {
		c.noteLayerCPUDraw()
	}
}

// coalesceImageFilterNodes drops no-ops and merges consecutive color-matrix nodes.
// Blur radii are NOT merged (Gaussian convolution is not a simple max/sum of radii).
// coalesceImageFilterNodes drops no-ops and merges consecutive color-matrix nodes.
// Blur radii are NOT merged (Gaussian convolution is not a simple max/sum of radii).
func coalesceImageFilterNodes(nodes []ImageFilterNode) []ImageFilterNode {
	if len(nodes) <= 1 {
		return nodes
	}
	out := make([]ImageFilterNode, 0, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if !imageFilterNodeRunnable(n) {
			continue
		}
		if len(out) > 0 && out[len(out)-1].Kind == ImageFilterColorMatrix && n.Kind == ImageFilterColorMatrix {
			out[len(out)-1].Matrix = composeColorMatrix4x5(out[len(out)-1].Matrix, n.Matrix)
			continue
		}
		out = append(out, n)
	}
	return out
}

// composeColorMatrix4x5 multiplies two 4x5 row-major color matrices: result applies a then b.
// Layout: rows [R,G,B,A] × columns [R,G,B,A,bias].
// composeColorMatrix4x5 multiplies two 4x5 row-major color matrices: result applies a then b.
// Layout: rows [R,G,B,A] × columns [R,G,B,A,bias].
func composeColorMatrix4x5(a, b [20]float32) [20]float32 {
	var r [20]float32
	for i := 0; i < 4; i++ {
		bi := i * 5
		r[bi+0] = b[bi+0]*a[0] + b[bi+1]*a[5] + b[bi+2]*a[10] + b[bi+3]*a[15]
		r[bi+1] = b[bi+0]*a[1] + b[bi+1]*a[6] + b[bi+2]*a[11] + b[bi+3]*a[16]
		r[bi+2] = b[bi+0]*a[2] + b[bi+1]*a[7] + b[bi+2]*a[12] + b[bi+3]*a[17]
		r[bi+3] = b[bi+0]*a[3] + b[bi+1]*a[8] + b[bi+2]*a[13] + b[bi+3]*a[18]
		r[bi+4] = b[bi+0]*a[4] + b[bi+1]*a[9] + b[bi+2]*a[14] + b[bi+3]*a[19] + b[bi+4]
	}
	return r
}

func imageFilterGraphGPUSupported(nodes []ImageFilterNode) bool {
	any := false
	for i := range nodes {
		n := nodes[i]
		if !imageFilterNodeRunnable(n) {
			continue
		}
		switch n.Kind {
		case ImageFilterBlur, ImageFilterBlurXY, ImageFilterGrayscale, ImageFilterInvert,
			ImageFilterColorMatrix, ImageFilterDropShadow,
			ImageFilterErode, ImageFilterDilate, ImageFilterOffset, ImageFilterAlphaThreshold,
			ImageFilterLighting, ImageFilterTile, ImageFilterBloom:
			any = true
		case ImageFilterDisplacement:
			// GPU-capable only when the node carries its map; the aux slot
			// upload happens in the graph runner.
			if n.DisplacementImage != nil {
				any = true
			}
		default:
			return false
		}
	}
	return any
}

func imageFilterNodeRunnable(n ImageFilterNode) bool {
	switch n.Kind {
	case ImageFilterBlur:
		return n.Radius > 0 && blurApply != nil
	case ImageFilterBlurXY:
		return (n.RadiusX > 0 || n.RadiusY > 0) && blurXYApply != nil
	case ImageFilterDropShadow:
		return dropShadowApply != nil
	case ImageFilterColorMatrix:
		return colorMatrixApply != nil
	case ImageFilterGrayscale:
		return grayscaleApply != nil
	case ImageFilterInvert:
		return invertApply != nil
	case ImageFilterErode:
		return n.MorphRadius > 0 && erodeApply != nil
	case ImageFilterDilate:
		return n.MorphRadius > 0 && dilateApply != nil
	case ImageFilterOffset:
		return (n.IntOffsetX != 0 || n.IntOffsetY != 0) && offsetApply != nil
	case ImageFilterAlphaThreshold:
		return alphaThresholdApply != nil
	case ImageFilterLighting:
		return lightingApply != nil && n.LightKD > 0
	case ImageFilterTile:
		return tileApply != nil && n.TileSrcRect[2] > n.TileSrcRect[0] && n.TileSrcRect[3] > n.TileSrcRect[1]
	case ImageFilterDisplacement:
		return displacementApply != nil && n.DisplacementImage != nil
	case ImageFilterBloom:
		// Identity and out-of-domain nodes are not runnable: an identity must
		// never occupy a pass, and a bad parameter must never be clamped into
		// one by guesswork.
		return bloomApply != nil && bloomNodeRunnable(n.Bloom)
	default:
		return false
	}
}

// morphRadiusYOf resolves the anisotropic morphology Y radius: nodes built
// before the field existed (or by isotropic callers) leave it 0 — fall back
// to the X radius.
func morphRadiusYOf(n ImageFilterNode) int {
	if n.MorphRadiusY > 0 {
		return n.MorphRadiusY
	}
	return n.MorphRadius
}

func applyImageFilterNode(n ImageFilterNode, src, dst *Pixmap) {
	switch n.Kind {
	case ImageFilterBlur:
		blurApply(src, dst, n.Radius)
	case ImageFilterBlurXY:
		blurXYApply(src, dst, n.RadiusX, n.RadiusY)
	case ImageFilterDropShadow:
		dropShadowApply(src, dst, n.OffsetX, n.OffsetY, n.ShadowBlur, n.ShadowColor)
	case ImageFilterColorMatrix:
		colorMatrixApply(src, dst, n.Matrix)
	case ImageFilterGrayscale:
		grayscaleApply(src, dst)
	case ImageFilterInvert:
		invertApply(src, dst)
	case ImageFilterErode:
		erodeApply(src, dst, n.MorphRadius, morphRadiusYOf(n))
	case ImageFilterDilate:
		dilateApply(src, dst, n.MorphRadius, morphRadiusYOf(n))
	case ImageFilterOffset:
		offsetApply(src, dst, n.IntOffsetX, n.IntOffsetY)
	case ImageFilterAlphaThreshold:
		alphaThresholdApply(src, dst, n.AlphaThreshold)
	case ImageFilterLighting:
		lightingApply(src, dst, n.LightPos, n.LightColor, n.LightKD, n.SurfaceScale)
	case ImageFilterTile:
		tileApply(src, dst, n.TileSrcRect)
	case ImageFilterDisplacement:
		displacementApply(src, dst, n.DispXChannel, n.DispYChannel, n.DispScale, n.DisplacementImage)
	case ImageFilterBloom:
		bloomApply(src, dst, n.Bloom)
	}
}

// attachFilterGPUResult stores a published filter texture for GPU compositing.
// Double-buffered: the previous publish is released only when replaced again, so
// the present path can still sample last frame's view during the current encode.
// attachFilterGPUResult stores a published filter texture for GPU compositing.
// Double-buffered: the previous publish is released only when replaced again, so
// the present path can still sample last frame's view during the current encode.
func (c *Context) attachFilterGPUResult(view gpucontext.TextureView, w, h int, release func()) {
	if c == nil {
		if release != nil {
			release()
		}
		return
	}
	// Drop the slot older than "previous" (two-frame latency free).
	if c.filterGPUReleasePrev != nil {
		c.filterGPUReleasePrev()
		c.filterGPUReleasePrev = nil
	}
	// Demote current → previous (still sampleable until next attach).
	c.filterGPUReleasePrev = c.filterGPURelease
	c.filterGPUView = view
	c.filterGPUW = w
	c.filterGPUH = h
	c.filterGPURelease = release
}

// releaseFilterGPUResult drops any published filter texture (and the prev slot).
// releaseFilterGPUResult drops any published filter texture (and the prev slot).
func (c *Context) releaseFilterGPUResult() {
	if c == nil {
		return
	}
	if c.filterGPUReleasePrev != nil {
		c.filterGPUReleasePrev()
		c.filterGPUReleasePrev = nil
	}
	if c.filterGPURelease != nil {
		c.filterGPURelease()
	}
	c.filterGPURelease = nil
	c.filterGPUView = gpucontext.TextureView{}
	c.filterGPUW, c.filterGPUH = 0, 0
}

// GPUFilterTexture returns the latest GPU filter result for zero-copy present
// (DrawGPUTexture). ok=false when no published filter texture is available.
// GPUFilterTexture returns the latest GPU filter result for zero-copy present
// (DrawGPUTexture). ok=false when no published filter texture is available.
func (c *Context) GPUFilterTexture() (view gpucontext.TextureView, w, h int, ok bool) {
	if c == nil || c.filterGPUView.IsNil() || c.filterGPUW <= 0 || c.filterGPUH <= 0 {
		return gpucontext.TextureView{}, 0, 0, false
	}
	return c.filterGPUView, c.filterGPUW, c.filterGPUH, true
}

// materializeFilterGPU readbacks a published filter texture into the pixmap when
// pixmapFilterStale. Used by Image/Export/SavePNG and layer-coherent filter paths.
// materializeFilterGPU readbacks a published filter texture into the pixmap when
// pixmapFilterStale. Used by Image/Export/SavePNG and layer-coherent filter paths.
func (c *Context) materializeFilterGPU() bool {
	if c == nil || c.pixmap == nil {
		return false
	}
	if !c.pixmapFilterStale {
		return true
	}
	return c.materializeFilterGPUTo(c.pixmap.Data(), nil)
}

// materializeFilterGPUTo readbacks the published filter texture into primary
// (required) and optional secondary buffers in one Map.
// secondary may be the pixmap when primary is an ImageBuf destination.
// materializeFilterGPUTo readbacks the published filter texture into primary
// (required) and optional secondary buffers in one Map.
// secondary may be the pixmap when primary is an ImageBuf destination.
func (c *Context) materializeFilterGPUTo(primary, secondary []byte) bool {
	if c == nil {
		return false
	}
	if !c.pixmapFilterStale {
		return true
	}
	if c.filterGPUView.IsNil() || c.filterGPUW <= 0 || c.filterGPUH <= 0 {
		c.pixmapFilterStale = false
		return false
	}
	// 通道顺序由这张效果面自己的格式决定（M6 的真正病灶）：这里曾把「filter 发布纹理是
	// RGBA8Unorm」当普适事实，无条件走不翻通道的那个读口；而 SetEffectSurface 那条路发布
	// 的实测是 BGRA8Unorm（types 0x1B），于是导出面整片红蓝互换 —— 绿与 alpha 不动、
	// 白底不变，所以肉眼看不出来。格式未知一律 fail-closed，不再靠「先试一个、失败退另一个」猜。
	type viewStraightReader interface {
		ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	}
	type viewSwizzleReader interface {
		ReadbackViewRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	}
	raw := c.GPURenderContext()
	var rgba []byte
	var err error
	switch channelOrderOfView(c.filterGPUView) {
	case viewOrderRGBA:
		vr, ok := raw.(viewStraightReader)
		if !ok || vr == nil {
			return false
		}
		rgba, err = vr.ReadbackViewStraightRGBA(c.filterGPUView, c.filterGPUW, c.filterGPUH)
	case viewOrderBGRA:
		vr, ok := raw.(viewSwizzleReader)
		if !ok || vr == nil {
			return false
		}
		rgba, err = vr.ReadbackViewRGBA(c.filterGPUView, c.filterGPUW, c.filterGPUH)
	default:
		return false
	}
	if err != nil || len(rgba) < c.filterGPUW*c.filterGPUH*4 {
		return false
	}
	n := c.filterGPUW * c.filterGPUH * 4
	if primary != nil {
		pn := n
		if len(primary) < pn {
			pn = len(primary)
		}
		copy(primary[:pn], rgba[:pn])
	}
	if secondary != nil {
		sn := n
		if len(secondary) < sn {
			sn = len(secondary)
		}
		copy(secondary[:sn], rgba[:sn])
	}
	if c.pixmap != nil {
		// Keep gen stable when possible; Notify for consumers keyed on pixmap gen.
		c.pixmap.NotifyPixelsChanged()
		// Both callers pass the pixmap as primary/secondary, so the pixmap now
		// holds the resolved surface.
		c.pixmapBaseContent = true
	}
	c.pixmapFilterStale = false
	return true
}

func (c *Context) releaseFilterSrcRT() {
	if c == nil {
		return
	}
	if c.filterSrcRelease != nil {
		c.filterSrcRelease()
	}
	c.filterSrcRelease = nil
	c.filterSrcView = gpucontext.TextureView{}
	c.filterSrcW, c.filterSrcH = 0, 0
}

// forgetFilterSourceContent marks the filter-source scratch as content-free
// for the next pass. The scratch is a persistent Context-level RT reused by
// every filter application in a frame; without this the session keeps it in
// renderedViews/colorContentViews and picks LoadOpLoad, so the previous
// filter's silhouette is carried into the next filter and PopLayer
// re-composites it at its own position (frame-local stacking). The CPU leg
// always starts from the clean layer pixmap; Skia isolates filter sources too.
func forgetFilterSourceContent(rc gpuContextOps, view gpucontext.TextureView) {
	if view.IsNil() {
		return
	}
	type viewContentForgetter interface {
		ForgetViewContent(view gpucontext.TextureView)
	}
	if f, ok := rc.(viewContentForgetter); ok && f != nil {
		f.ForgetViewContent(view)
	}
}

func (c *Context) ensureFilterSrcRT(w, h int) (gpucontext.TextureView, bool) {
	if c == nil || w <= 0 || h <= 0 {
		return gpucontext.TextureView{}, false
	}
	type offscreenCreator interface {
		CreateOffscreenTexture(w, h int) (gpucontext.TextureView, func())
	}
	rc := c.gpuCtxOps()
	if !c.filterSrcView.IsNil() && c.filterSrcW == w && c.filterSrcH == h {
		forgetFilterSourceContent(rc, c.filterSrcView)
		return c.filterSrcView, true
	}
	c.releaseFilterSrcRT()
	oc, ok := rc.(offscreenCreator)
	if !ok || oc == nil {
		return gpucontext.TextureView{}, false
	}
	view, rel := oc.CreateOffscreenTexture(w, h)
	if view.IsNil() {
		return gpucontext.TextureView{}, false
	}
	c.filterSrcView = view
	c.filterSrcW, c.filterSrcH = w, h
	c.filterSrcRelease = rel
	forgetFilterSourceContent(rc, view)
	return view, true
}

// mergeOver composites src over dst (premul source-over) in place on dst.
func mergeOver(src, dst *Pixmap) {
	if src == nil || dst == nil || len(src.Data()) != len(dst.Data()) {
		return
	}
	s, d := src.Data(), dst.Data()
	for i := 0; i+3 < len(d); i += 4 {
		sa := uint32(s[i+3])
		if sa == 0 {
			continue
		}
		if sa == 255 {
			d[i], d[i+1], d[i+2], d[i+3] = s[i], s[i+1], s[i+2], 255
			continue
		}
		ia := 255 - sa
		d[i] = uint8((uint32(s[i])*sa + uint32(d[i])*ia) / 255)
		d[i+1] = uint8((uint32(s[i+1])*sa + uint32(d[i+1])*ia) / 255)
		d[i+2] = uint8((uint32(s[i+2])*sa + uint32(d[i+2])*ia) / 255)
		a := uint32(s[i+3]) + uint32(d[i+3])*ia/255
		d[i+3] = uint8(a)
	}
}
