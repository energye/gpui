package render

import (
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
	intImage "github.com/energye/gpui/render/internal/image"
)

// Layer represents a drawing layer with blend mode and opacity.
// Layers allow isolating drawing operations and compositing them with
// different blend modes and opacity values, similar to layers in Photoshop
// or SVG group opacity.
type Layer struct {
	pixmap    *Pixmap
	blendMode BlendMode
	opacity   float64
	mask      *Mask // optional alpha mask, applied on PopLayer (nil = no mask)
	// damage is the union of draw bounds on this layer in pixmap pixel space.
	// PopLayer composites only this rect (plus AA pad) instead of the full
	// surface — full 800x600 CPU blend was ~50ms/frame on Intel iGPU.
	// Empty means "unknown / full surface" (safe fallback).
	damage image.Rectangle
	// fullComposite forces a full-surface blend (backdrop snapshots, masks).
	fullComposite bool

	// GPU layer RT (P0-1 / L.01–L.02): draw into an offscreen texture when
	// available, then composite with DrawGPUTexture* on Pop — no GPU→CPU
	// readback of the layer surface.
	gpuView    gpucontext.TextureView
	gpuRelease func()
	gpuW, gpuH int
	// cpuDrew is true when any CPU path wrote into layer.pixmap while the
	// layer was active (fallback, SetPixel, advanced-blend CPU layers, etc.).
	cpuDrew bool
}

// layerStack manages the layer hierarchy for the context.
type layerStack struct {
	layers []*Layer
	// pool reuses full-surface layer Pixmaps (S6.4). intImage.Pool is ImageBuf-only.
	pool *pixmapPool
}

// newLayerStack creates a new layer stack with a pool for memory reuse.
func newLayerStack() *layerStack {
	return &layerStack{
		layers: make([]*Layer, 0, 4),
		pool:   newPixmapPool(8),
	}
}

// LayerPoolStats returns S6.4 layer pixmap pool counters for the context.
func (c *Context) LayerPoolStats() (gets, puts, hits, misses int) {
	if c == nil || c.layerStack == nil || c.layerStack.pool == nil {
		return 0, 0, 0, 0
	}
	return c.layerStack.pool.Stats()
}

// ResetLayerPoolStats clears layer pool counters (tests).
func (c *Context) ResetLayerPoolStats() {
	if c == nil || c.layerStack == nil || c.layerStack.pool == nil {
		return
	}
	c.layerStack.pool.ResetStats()
}

// PushLayer creates a new layer and makes it the active drawing target.
// All subsequent drawing operations will render to this layer until PopLayer is called.
//
// The layer will be composited onto the parent layer/canvas when PopLayer is called,
// using the specified blend mode and opacity.
//
// Parameters:
//   - blendMode: How to composite this layer onto the parent (e.g., BlendMultiply, BlendScreen)
//   - opacity: Layer opacity in range [0.0, 1.0] where 0 is fully transparent and 1 is fully opaque
//
// Example:
//
//	dc.PushLayer(gg.BlendMultiply, 0.5)
//	dc.SetRGB(1, 0, 0)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.PopLayer() // Composite circle onto canvas with multiply blend at 50% opacity
func (c *Context) PushLayer(blendMode BlendMode, opacity float64) {
	c.pushLayerSurface(blendMode, opacity, true)
}

// pushLayerSurface is the shared PushLayer implementation.
// clear=true for normal layers; false when the caller will fully overwrite (backdrop).
func (c *Context) pushLayerSurface(blendMode BlendMode, opacity float64, clear bool) {
	// Clamp opacity to valid range
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}

	// Initialize layer stack if needed
	if c.layerStack == nil {
		c.layerStack = newLayerStack()
	}

	// Save base pixmap on first push
	if len(c.layerStack.layers) == 0 && c.basePixmap == nil {
		c.basePixmap = c.pixmap
	}

	// Acquire layer surface from pool (S6.4: avoid per-push NewPixmap).
	var layerPixmap *Pixmap
	if clear {
		layerPixmap = c.layerStack.pool.Get(c.width, c.height)
	} else {
		layerPixmap = c.layerStack.pool.GetForOverwrite(c.width, c.height)
	}

	// Create layer
	layer := &Layer{
		pixmap:    layerPixmap,
		blendMode: blendMode,
		opacity:   opacity,
	}

	// P0-1/P0-3: layers without a mask get a GPU offscreen RT when GPU is
	// available. Content draws SourceOver into the RT; Pop uses texture blit
	// (Normal/Copy) or dual-tex advanced blend (Multiply/Screen/…).
	// Mask layers also take a GPU RT (R1); Pop uses masked composite.
	pw, ph := layerPixmap.Width(), layerPixmap.Height()
	if pw > 0 && ph > 0 {
		view, release := c.CreateOffscreenTexture(pw, ph)
		if !view.IsNil() && release != nil {
			layer.gpuView = view
			layer.gpuRelease = release
			layer.gpuW = pw
			layer.gpuH = ph
		}
	}

	// Save current pixmap and switch to layer pixmap
	c.layerStack.layers = append(c.layerStack.layers, layer)
	c.pixmap = layerPixmap
}

// PopLayer composites the current layer onto the parent layer/canvas.
// Uses the blend mode and opacity specified in the corresponding PushLayer call.
//
// The layer is composited using the specified blend mode and opacity.
// After compositing, the layer's memory is returned to the pool for reuse.
//
// If there are no layers to pop, this function does nothing.
//
// Example:
//
//	dc.PushLayer(gg.BlendScreen, 1.0)
//	// ... draw operations ...
//	dc.PopLayer() // Composite layer onto parent
func (c *Context) PopLayer() {
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}

	// Pop the current layer (keep fields for composite/release).
	layers := c.layerStack.layers
	layer := layers[len(layers)-1]
	c.layerStack.layers = layers[:len(layers)-1]

	// Finish any pending draws that targeted this layer's GPU RT first.
	// Must happen before restoring the parent target so gpuRenderTarget still
	// points at the layer view if FlushGPU is used as a fallback.
	if !layer.gpuView.IsNil() {
		if rc := c.gpuCtxOps(); rc != nil && rc.PendingCount() > 0 {
			_ = c.FlushGPUWithView(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH)) //nolint:gosec // bounded
		}
	} else if rc := c.gpuCtxOps(); rc != nil && rc.PendingCount() > 0 {
		// CPU layer: materialize any stray GPU ops into the layer pixmap.
		_ = c.FlushGPU()
	}

	// Get parent pixmap (either previous layer or base)
	var parentPixmap *Pixmap
	if len(c.layerStack.layers) > 0 {
		parentPixmap = c.layerStack.layers[len(c.layerStack.layers)-1].pixmap
	} else {
		// Restore base pixmap
		parentPixmap = c.basePixmap
		c.basePixmap = nil
	}

	// Restore parent as the active drawing target BEFORE compositing so
	// DrawGPUTexture* / composite write into the parent, not the child.
	c.pixmap = parentPixmap

	// Apply mask to layer content before compositing (PushMaskLayer).
	// Prefer GPU R8 modulate composite when layer content is on a GPU RT.
	maskedGPUDone := false
	if layer.mask != nil {
		layer.fullComposite = true
		if !layer.gpuView.IsNil() && !layer.cpuDrew && parentPixmap != nil {
			if c.compositeLayerMaskedGPU(layer, parentPixmap) {
				maskedGPUDone = true
				if layer.gpuRelease != nil {
					layer.gpuRelease()
					layer.gpuRelease = nil
				}
				layer.gpuView = gpucontext.TextureView{}
			}
		}
		if !maskedGPUDone {
			// Materialize GPU content into pixmap, then CPU DestinationIn mask.
			if !layer.gpuView.IsNil() && !layer.cpuDrew {
				_ = c.materializeLayerGPUToPixmap(layer)
			}
			c.applyMaskToPixmap(layer.pixmap, layer.mask)
		}
	}

	// GPU composite path: no mask, no CPU writes into the layer pixmap.
	// Content lives on layer.gpuView.
	canGPUTexture := !layer.gpuView.IsNil() && !layer.cpuDrew && layer.mask == nil && !maskedGPUDone
	canGPUComposite := canGPUTexture &&
		(layer.blendMode == BlendNormal || layer.blendMode == BlendCopy)
	canGPUAdvanced := canGPUTexture && IsAdvancedBlendMode(layer.blendMode)

	if canGPUAdvanced {
		// P0-3: dual-tex layer Pop (dest = parent pixmap, src = layer RT).
		if c.compositeLayerAdvancedGPU(layer, parentPixmap) {
			if layer.gpuRelease != nil {
				// Texture already read; release immediately.
				layer.gpuRelease()
				layer.gpuRelease = nil
			}
			layer.gpuView = gpucontext.TextureView{}
		} else {
			// Fallback: release GPU RT and composite empty/CPU pixmap.
			if layer.gpuRelease != nil {
				layer.gpuRelease()
				layer.gpuRelease = nil
			}
			layer.gpuView = gpucontext.TextureView{}
			c.compositeLayer(layer, parentPixmap)
		}
	} else if canGPUComposite {
		// Layer RT is already in device/pixel space. Temporarily clear CTM so
		// the full-surface blit is not re-transformed by the user matrix.
		savedMat := c.matrix
		savedDev := c.deviceMatrix
		c.matrix = Identity()
		c.deviceMatrix = Identity()
		op := float32(layer.opacity)
		if op < 0 {
			op = 0
		}
		if op > 1 {
			op = 1
		}
		c.DrawGPUTextureWithOpacity(layer.gpuView, 0, 0, layer.gpuW, layer.gpuH, op)
		c.matrix = savedMat
		c.deviceMatrix = savedDev
		// Keep the texture alive until the composite command is flushed.
		if layer.gpuRelease != nil {
			c.layerGPUReleases = append(c.layerGPUReleases, layer.gpuRelease)
			layer.gpuRelease = nil
		}
		layer.gpuView = gpucontext.TextureView{}

		// P1-3 / F.03: do NOT mid-frame FlushGPU when compositing onto the base
		// (or CPU parent). The GPU texture blit stays queued and materializes on
		// the next FlushGPU / Image / PresentFrame — single submit per frame.
		// Callers that sample pixmap.GetPixel without Flush must FlushGPU first
		// (Image() already flushes). Nested GPU parents already batch.
	} else if !maskedGPUDone {
		// CPU composite (advanced blend, mask, CPU-drawn content, or no GPU).
		if layer.gpuRelease != nil {
			layer.gpuRelease()
			layer.gpuRelease = nil
		}
		layer.gpuView = gpucontext.TextureView{}
		c.compositeLayer(layer, parentPixmap)
	} else {
		// Masked GPU composite already wrote parent; just drop any residual RT.
		if layer.gpuRelease != nil {
			layer.gpuRelease()
			layer.gpuRelease = nil
		}
		layer.gpuView = gpucontext.TextureView{}
	}

	// Return layer surface to pool (S6.4).
	if c.layerStack.pool != nil {
		c.layerStack.pool.Put(layer.pixmap)
	}
	layer.pixmap = nil
	layer.mask = nil
}

// PushMaskLayer creates an isolated layer with an associated alpha mask.
// All subsequent drawing operations render to this layer normally (without masking).
// When PopLayer is called, the ENTIRE layer is masked by the mask and then
// composited onto the parent using source-over blending with full opacity.
//
// This produces different results from SetMask: PushMaskLayer masks the
// composited group, while SetMask masks each shape individually.
//
// Matches Vello push_mask_layer() semantics (research §4).
//
// Example:
//
//	mask := gg.NewMaskFromAlpha(maskImage)
//	dc.PushMaskLayer(mask)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.DrawRect(80, 80, 40, 40)
//	dc.Fill()
//	dc.PopLayer() // entire layer content masked, then composited
func (c *Context) PushMaskLayer(mask *Mask) {
	// Clamp: nil mask means no masking (equivalent to PushLayer).
	if mask == nil {
		c.PushLayer(BlendNormal, 1.0)
		return
	}

	// Initialize layer stack if needed.
	if c.layerStack == nil {
		c.layerStack = newLayerStack()
	}

	// Save base pixmap on first push.
	if len(c.layerStack.layers) == 0 && c.basePixmap == nil {
		c.basePixmap = c.pixmap
	}

	// Acquire layer surface from pool (S6.4).
	layerPixmap := c.layerStack.pool.Get(c.width, c.height)

	// Create layer with mask.
	layer := &Layer{
		pixmap:    layerPixmap,
		blendMode: BlendNormal,
		opacity:   1.0,
		mask:      mask,
	}

	// R1 residual fix: mask layers also get a GPU RT so in-layer Fill/Stroke
	// stay on GPU. Pop uses CompositeMaskedLayer (R8 modulate) or CPU mask.
	pw, ph := layerPixmap.Width(), layerPixmap.Height()
	if pw > 0 && ph > 0 {
		view, release := c.CreateOffscreenTexture(pw, ph)
		if !view.IsNil() && release != nil {
			layer.gpuView = view
			layer.gpuRelease = release
			layer.gpuW = pw
			layer.gpuH = ph
		}
	}

	// Switch to layer pixmap.
	c.layerStack.layers = append(c.layerStack.layers, layer)
	c.pixmap = layerPixmap
}

// applyMaskToPixmap applies a DestinationIn mask to a pixmap's pixel data.
// For each pixel: all channels are scaled by mask.At(x,y) / 255.
func (c *Context) applyMaskToPixmap(pm *Pixmap, mask *Mask) {
	applyMaskToPixmapData(pm, mask)
}

// SetBlendMode sets the blend mode for subsequent fill and stroke operations.
//
// GPU fixed-function modes (B.02): BlendNormal (SourceOver), BlendCopy, BlendClear,
// BlendPlus, DstOut/SrcAtop/Xor, DstOver/SrcIn/SrcOut/DstIn/DstAtop.
// Advanced modes (Multiply/Screen/…) use resolve+CPU composite+GPU blit or CPU.
//
// Example:
//
//	dc.SetBlendMode(gg.BlendMultiply)
//	dc.Fill() // Future: will use multiply blend mode
func (c *Context) SetBlendMode(mode BlendMode) {
	c.paint.BlendMode = mode
}

// compositeLayer composites a layer onto a parent pixmap using the layer's
// blend mode and opacity.
//
// When layer.damage is known and fullComposite is false, only the damaged
// rectangle is blended (AA pad). This is required for continuous UI layers at
// 60fps — full-surface SourceOver of 800x600 was ~50ms on Intel HD.
func (c *Context) compositeLayer(layer *Layer, parent *Pixmap) {
	// Convert pixmaps to ImageBuf for blending
	srcImg := c.pixmapToImageBuf(layer.pixmap)
	dstImg := c.pixmapToImageBuf(parent)

	srcW, srcH := srcImg.Bounds()
	r := image.Rect(0, 0, srcW, srcH)
	if !layer.fullComposite && !layer.damage.Empty() {
		// AA / filter soft edge pad (filters that expand bounds should mark fullComposite).
		const pad = 2
		d := layer.damage.Inset(-pad)
		d = d.Intersect(r)
		if d.Empty() {
			return
		}
		r = d
	}

	srcRect := intImage.Rect{X: r.Min.X, Y: r.Min.Y, Width: r.Dx(), Height: r.Dy()}
	params := intImage.DrawParams{
		SrcRect: &srcRect,
		DstRect: intImage.Rect{
			X:      r.Min.X,
			Y:      r.Min.Y,
			Width:  r.Dx(),
			Height: r.Dy(),
		},
		Interp:    intImage.InterpNearest, // No scaling, so nearest is fine
		Opacity:   layer.opacity,
		BlendMode: layer.blendMode,
	}

	intImage.DrawImage(dstImg, srcImg, params)
}

// noteLayerDamage unions a pixmap-space rectangle into the current top layer.
// bounds should already be in the same pixel space as the layer pixmap
// (physical when deviceScale!=1 after trackDamage scaling).
func (c *Context) noteLayerDamage(bounds image.Rectangle) {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 || bounds.Empty() {
		return
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top == nil || top.fullComposite {
		return
	}
	if top.damage.Empty() {
		top.damage = bounds
		return
	}
	top.damage = top.damage.Union(bounds)
}

// markLayerFullComposite forces the current top layer to full-surface Pop blend.
func (c *Context) markLayerFullComposite() {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	c.layerStack.layers[len(c.layerStack.layers)-1].fullComposite = true
}

// compositeLayerAdvancedGPU dual-tex composites a GPU layer RT onto parent.
// Returns false if GPU dual-tex is unavailable (caller falls back to CPU).
func (c *Context) compositeLayerAdvancedGPU(layer *Layer, parent *Pixmap) bool {
	if c == nil || layer == nil || parent == nil || layer.gpuView.IsNil() {
		return false
	}
	type advancedLayerCompositor interface {
		CompositeAdvancedLayer(parentData []byte, parentW, parentH int,
			srcView gpucontext.TextureView, srcW, srcH int,
			damage image.Rectangle, mode BlendMode, opacity float64) error
	}
	rc := c.gpuCtxOps()
	ac, ok := rc.(advancedLayerCompositor)
	if !ok || ac == nil {
		// Try concrete any from GPURenderContext()
		if raw := c.GPURenderContext(); raw != nil {
			ac, ok = raw.(advancedLayerCompositor)
		}
	}
	if !ok || ac == nil {
		return false
	}
	damage := layer.damage
	if layer.fullComposite {
		damage = image.Rectangle{}
	}
	err := ac.CompositeAdvancedLayer(
		parent.Data(), parent.Width(), parent.Height(),
		layer.gpuView, layer.gpuW, layer.gpuH,
		damage, layer.blendMode, layer.opacity,
	)
	return err == nil
}

// layerForceCPUDraw reports whether the current top layer must receive CPU
// draws into its pixmap. False when a GPU layer RT is active (P0-1).
func (c *Context) layerForceCPUDraw() bool {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return false
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top == nil {
		return true
	}
	if !top.gpuView.IsNil() {
		return false
	}
	return true
}

// noteLayerCPUDraw marks that the current top layer received CPU pixmap writes.
// PopLayer then uses CPU composite instead of GPU texture blit.
func (c *Context) noteLayerCPUDraw() {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top != nil {
		top.cpuDrew = true
	}
}

// materializeLayerGPUToPixmap readbacks a layer GPU RT into its pixmap (R1).
func (c *Context) materializeLayerGPUToPixmap(layer *Layer) bool {
	if c == nil || layer == nil || layer.gpuView.IsNil() || layer.pixmap == nil {
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
	rgba, err := vr.ReadbackViewRGBA(layer.gpuView, layer.gpuW, layer.gpuH)
	if err != nil || len(rgba) < layer.gpuW*layer.gpuH*4 {
		return false
	}
	dst := layer.pixmap.Data()
	n := layer.gpuW * layer.gpuH * 4
	if len(dst) < n {
		n = len(dst)
	}
	copy(dst[:n], rgba[:n])
	layer.pixmap.NotifyPixelsChanged()
	return true
}

// seedTopLayerGPUFromPixmap uploads the top layer pixmap into its GPU RT (L.05).
// Keeps GPU RT coherent after CPU snapshot / filter writes.
func (c *Context) seedTopLayerGPUFromPixmap() bool {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return false
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top == nil || top.gpuView.IsNil() || top.pixmap == nil {
		return false
	}
	type viewUploader interface {
		UploadRGBAToView(view gpucontext.TextureView, data []byte, w, h int) error
	}
	raw := c.GPURenderContext()
	vu, ok := raw.(viewUploader)
	if !ok || vu == nil {
		return false
	}
	if err := vu.UploadRGBAToView(top.gpuView, top.pixmap.Data(), top.gpuW, top.gpuH); err != nil {
		return false
	}
	// GPU RT matches pixmap; allow GPU Pop composite again.
	top.cpuDrew = false
	return true
}

// compositeLayerMaskedGPU dual-path: GPU layer RT × R8 mask → parent (R1 L.02 mask).
func (c *Context) compositeLayerMaskedGPU(layer *Layer, parent *Pixmap) bool {
	if c == nil || layer == nil || parent == nil || layer.mask == nil || layer.gpuView.IsNil() {
		return false
	}
	type maskedCompositor interface {
		CompositeMaskedLayer(parentData []byte, parentW, parentH int,
			srcView gpucontext.TextureView, srcW, srcH int,
			mask *Mask, opacity float64) error
	}
	raw := c.GPURenderContext()
	mc, ok := raw.(maskedCompositor)
	if !ok || mc == nil {
		return false
	}
	err := mc.CompositeMaskedLayer(
		parent.Data(), parent.Width(), parent.Height(),
		layer.gpuView, layer.gpuW, layer.gpuH,
		layer.mask, layer.opacity,
	)
	if err != nil {
		return false
	}
	c.recordGPUOp()
	return true
}

// drainLayerGPUReleases releases GPU layer textures whose composite draws have
// already been flushed. Safe to call repeatedly.
func (c *Context) drainLayerGPUReleases() {
	if c == nil || len(c.layerGPUReleases) == 0 {
		return
	}
	for _, rel := range c.layerGPUReleases {
		if rel != nil {
			rel()
		}
	}
	c.layerGPUReleases = c.layerGPUReleases[:0]
}
