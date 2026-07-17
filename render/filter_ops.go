package render

import (
	gpucontext "github.com/energye/gpui/gpu/context"
)

// filterApplyFunc applies a filter from src into dst over the full pixmap.
type filterApplyFunc func(src, dst *Pixmap)

var (
	blurApply        func(src, dst *Pixmap, radius float64)
	blurXYApply      func(src, dst *Pixmap, radiusX, radiusY float64)
	dropShadowApply  func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA)
	colorMatrixApply func(src, dst *Pixmap, matrix [20]float32)
	grayscaleApply   func(src, dst *Pixmap)
	invertApply      func(src, dst *Pixmap)
	// gpuFilterGraphApply runs F.03 multi-RT GPU ping-pong (optional).
	// src is tight RGBA8 w*h*4; returns same layout result.
	gpuFilterGraphApply func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)
	// gpuFilterGraphApplyTexture publishes a GPU texture without CPU Map/readback.
	// Preferred for continuous effect RTs (glow/blur present path).
	gpuFilterGraphApplyTexture func(src []byte, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)
	// gpuFilterGraphApplyFromView seeds from a GPU texture (no CPU upload/readback).
	gpuFilterGraphApplyFromView func(srcView gpucontext.TextureView, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)

	// S6.4: shared intermediate pool for CPU filter ping-pong (blur/shadow/graph).
	filterPixmapPool = newPixmapPool(8)
)

// FilterPoolStats returns S6.4 filter intermediate pool counters.
func FilterPoolStats() (gets, puts, hits, misses int) {
	return filterPixmapPool.Stats()
}

// ResetFilterPoolStats clears filter pool counters (tests).
func ResetFilterPoolStats() {
	filterPixmapPool.ResetStats()
}

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
	// S6.4: reuse intermediate RT; full overwrite path (copy src→dst first).
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

	finish := func(view gpucontext.TextureView, release func()) bool {
		if view.IsNil() {
			if release != nil {
				release()
			}
			return false
		}
		c.attachFilterGPUResult(view, w, h, release)
		c.pixmapFilterStale = true
		hasLayer := c.layerStack != nil && len(c.layerStack.layers) > 0
		if hasLayer {
			if c.materializeFilterGPU() {
				if !c.seedTopLayerGPUFromPixmap() {
					c.noteLayerCPUDraw()
				}
			} else {
				c.noteLayerCPUDraw()
			}
		}
		c.recordGPUOp()
		return true
	}

	need := w * h * 4
	if cap(c.filterSrcScratch) < need {
		c.filterSrcScratch = make([]byte, need)
	}
	c.filterSrcScratch = c.filterSrcScratch[:need]

	// 1) Zero-readback (R7.2): flush pending draws into retained filterSrcRT, then
	// GPU→GPU filter. Glow/effect RTs hit this every recompute (pending > 0).
	// If we already flushed to filterSrcRT, never seed path 2/3 from the stale
	// CPU pixmap (draws never reached pixmap) — recover via src RT readback only.
	pending := 0
	if rc := c.gpuCtxOps(); rc != nil {
		type pcounter interface{ PendingCount() int }
		if pc, ok := rc.(pcounter); ok {
			pending = pc.PendingCount()
		}
	}
	flushedToSrc := false
	if pending > 0 && gpuFilterGraphApplyFromView != nil {
		if srcView, ok := c.ensureFilterSrcRT(w, h); ok {
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

	// 2) Pixmap-coherent path: only when we did NOT already flush draws solely
	// to filterSrcRT. FlushGPU may readback pending GPU draws into the pixmap.
	if !flushedToSrc {
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
func FiltersRegistered() bool {
	return blurApply != nil && dropShadowApply != nil && grayscaleApply != nil
}

// RegisterGPUFilterGraph wires an optional GPU multi-RT image filter graph (F.03).
// When set, ApplyImageFilterGraph prefers the GPU path for supported node sets.
func RegisterGPUFilterGraph(fn func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)) {
	gpuFilterGraphApply = fn
}

// RegisterGPUFilterGraphTexture wires a zero-readback GPU filter publisher.
// Result textures are composited with DrawGPUTexture; CPU pixels materialise lazily.
func RegisterGPUFilterGraphTexture(fn func(src []byte, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)) {
	gpuFilterGraphApplyTexture = fn
}

// RegisterGPUFilterGraphFromView wires a GPU→GPU filter path seeded from a texture view.
func RegisterGPUFilterGraphFromView(fn func(srcView gpucontext.TextureView, w, h int, nodes []ImageFilterNode) (gpucontext.TextureView, func(), error)) {
	gpuFilterGraphApplyFromView = fn
}

// GPUFilterGraphRegistered reports whether a GPU multi-RT filter graph is wired.
func GPUFilterGraphRegistered() bool {
	return gpuFilterGraphApply != nil || gpuFilterGraphApplyTexture != nil || gpuFilterGraphApplyFromView != nil
}

// SwapGPUFilterGraph replaces the pixel-readback GPU filter graph and returns the previous one (tests).
// Note: alone this does NOT disable texture/from-view publishers — use
// DisableGPUFilterGraphForTest to force the pure CPU filter path.
func SwapGPUFilterGraph(fn func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error)) func(src []byte, w, h int, nodes []ImageFilterNode) ([]byte, error) {
	prev := gpuFilterGraphApply
	gpuFilterGraphApply = fn
	return prev
}

// DisableGPUFilterGraphForTest clears pixel/texture/from-view GPU filter hooks so
// ApplyBlur/ApplyImageFilterGraph exercise the CPU intermediate pool (S6.4).
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
}

// ApplyImageFilterGraph runs a multi-node image filter graph with intermediate
// surface ping-pong (F.03 / Skia-style ImageFilter DAG chain).
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
	// S6.4: drop no-ops / merge consecutive color matrices before GPU or CPU path.
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

	// CPU fallback: pooled pixmap ping-pong intermediate surfaces (S6.4).
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

// coalesceImageFilterNodes drops no-ops and merges consecutive color-matrix nodes (S6.4).
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
			ImageFilterColorMatrix, ImageFilterDropShadow:
			any = true
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
	default:
		return false
	}
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
	}
}

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
func (c *Context) GPUFilterTexture() (view gpucontext.TextureView, w, h int, ok bool) {
	if c == nil || c.filterGPUView.IsNil() || c.filterGPUW <= 0 || c.filterGPUH <= 0 {
		return gpucontext.TextureView{}, 0, 0, false
	}
	return c.filterGPUView, c.filterGPUW, c.filterGPUH, true
}

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
// (required) and optional secondary buffers in one Map (R7.2 Export path).
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
	type viewReader interface {
		ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	}
	raw := c.GPURenderContext()
	vr, ok := raw.(viewReader)
	if !ok || vr == nil {
		return false
	}
	rgba, err := vr.ReadbackViewStraightRGBA(c.filterGPUView, c.filterGPUW, c.filterGPUH)
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

func (c *Context) ensureFilterSrcRT(w, h int) (gpucontext.TextureView, bool) {
	if c == nil || w <= 0 || h <= 0 {
		return gpucontext.TextureView{}, false
	}
	if !c.filterSrcView.IsNil() && c.filterSrcW == w && c.filterSrcH == h {
		return c.filterSrcView, true
	}
	c.releaseFilterSrcRT()
	type offscreenCreator interface {
		CreateOffscreenTexture(w, h int) (gpucontext.TextureView, func())
	}
	rc := c.gpuCtxOps()
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
	return view, true
}
