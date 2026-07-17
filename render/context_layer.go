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

	// opacityGroup (F1): Normal/Copy layers without mask skip isolation RT and
	// multiply paint alpha into subsequent draws instead. Semantically equal for
	// single SourceOver fills; multi-draw isolation still uses a real layer RT
	// when blend is advanced or a mask is set.
	opacityGroup bool

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

	// Create layer shell.
	layer := &Layer{
		blendMode: blendMode,
		opacity:   opacity,
	}

	// F1 Normal/Copy opacity-group: skip isolation RT and multiply paint alpha
	// (layerOpacityMul). Matches CSS-style group opacity for non-overlapping
	// SourceOver UI cards (PKS MULTI_LAYER). Overlapping multi-draw groups that
	// need Skia SaveLayer isolation should use an advanced blend mode or a future
	// PushLayerIsolated API — full-window RT×N was ~30fps on this path.
	//
	// Backdrop layers (clear=false) always need an isolation surface so the
	// parent snapshot can be copied; opacity-group would skip the pool and break
	// PushBackdropLayer.
	if clear && (blendMode == BlendNormal || blendMode == BlendCopy) {
		layer.opacityGroup = true
		c.layerStack.layers = append(c.layerStack.layers, layer)
		return
	}

	// Advanced blend / mask path: isolation RT (GPU preferred).
	pw, ph := c.width, c.height
	if pw > 0 && ph > 0 {
		view, release := c.CreateOffscreenTexture(pw, ph)
		if !view.IsNil() && release != nil {
			layer.gpuView = view
			layer.gpuRelease = release
			layer.gpuW = pw
			layer.gpuH = ph
		}
	}

	// Acquire layer surface from pool (S6.4: avoid per-push NewPixmap).
	var layerPixmap *Pixmap
	if layer.gpuView.IsNil() {
		if clear {
			layerPixmap = c.layerStack.pool.Get(c.width, c.height)
		} else {
			layerPixmap = c.layerStack.pool.GetForOverwrite(c.width, c.height)
		}
	} else {
		layerPixmap = c.layerStack.pool.GetForOverwrite(c.width, c.height)
	}
	layer.pixmap = layerPixmap

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

	// F1 opacity-group: nothing to composite — draws already hit the parent
	// with multiplied alpha.
	if layer.opacityGroup {
		if len(c.layerStack.layers) == 0 {
			c.basePixmap = nil
		}
		return
	}

	// Finish any pending draws that targeted this layer's GPU RT first.
	// Must happen before restoring the parent target so gpuRenderTarget still
	// points at the layer view if FlushGPU is used as a fallback.
	if !layer.gpuView.IsNil() {
		if rc := c.gpuCtxOps(); rc != nil && rc.PendingCount() > 0 {
			// F1: damage-aware flush when dirty rect is a real subset of the layer.
			// Near-full damage (e.g. particle mesh covering the stage) falls back to
			// full flush — damage path has setup cost without reducing encode work.
			if !layer.fullComposite && !layer.damage.Empty() && layer.gpuW > 0 && layer.gpuH > 0 {
				const pad = 2
				full := image.Rect(0, 0, layer.gpuW, layer.gpuH)
				d := layer.damage.Inset(-pad).Intersect(full)
				useDamage := !d.Empty()
				if useDamage {
					// area ratio; int math avoids float on hot path
					fullA := layer.gpuW * layer.gpuH
					if fullA > 0 && d.Dx()*d.Dy()*10 >= fullA*7 { // ≥70%
						useDamage = false
					}
				}
				if useDamage {
					_ = c.FlushGPUWithViewDamage(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH), d) //nolint:gosec
				} else {
					_ = c.FlushGPUWithView(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH)) //nolint:gosec
				}
			} else {
				_ = c.FlushGPUWithView(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH)) //nolint:gosec // bounded
			}
		}
	}
	// F1: CPU Normal layers already wrote into layer.pixmap via forceCPULayer.
	// Do NOT FlushGPU here — parent/stage pending ops would be resolved into the
	// layer pixmap and force full-surface submits every Pop (MULTI_LAYER ~10fps).

	// Get parent pixmap (previous isolation layer or base). Opacity-group
	// ancestors keep pixmap==nil — walk past them to the real surface.
	var parentPixmap *Pixmap
	if len(c.layerStack.layers) > 0 {
		for i := len(c.layerStack.layers) - 1; i >= 0; i-- {
			L := c.layerStack.layers[i]
			if L == nil || L.opacityGroup {
				continue
			}
			if L.pixmap != nil {
				parentPixmap = L.pixmap
				break
			}
		}
		if parentPixmap == nil {
			parentPixmap = c.basePixmap
			if parentPixmap == nil {
				parentPixmap = c.pixmap
			}
		}
	} else {
		// Restore base pixmap
		parentPixmap = c.basePixmap
		c.basePixmap = nil
	}

	// Restore parent as the active drawing target BEFORE compositing so
	// DrawGPUTexture* / composite write into the parent, not the child.
	if parentPixmap != nil {
		c.pixmap = parentPixmap
	}

	// Apply mask to layer content before compositing (PushMaskLayer).
	// Prefer GPU R8 modulate composite when layer content is on a GPU RT.
	maskedGPUDone := false
	if layer.mask != nil {
		layer.fullComposite = true
		// Parent draws may still be present-stashed (queued before PushMaskLayer).
		// CompositeMaskedLayer / applyMask write into parent.Data — materialize the
		// stashed base first so final Image/Flush does not paint base over the mask.
		if parentPixmap != nil {
			_ = c.FlushGPU()
		}
		if !layer.gpuView.IsNil() && !layer.cpuDrew && parentPixmap != nil {
			if c.compositeLayerMaskedGPU(layer, parentPixmap) {
				maskedGPUDone = true
				// CompositeMaskedLayer writes parent.Data (CPU pixmap), not a parent
				// GPU RT. Nested PushMaskLayer parents still hold an empty gpuView —
				// mark them cpuDrew so the next Pop uses pixmap + mask path.
				if c.layerStack != nil {
					for i := len(c.layerStack.layers) - 1; i >= 0; i-- {
						L := c.layerStack.layers[i]
						if L == nil || L.opacityGroup {
							continue
						}
						if L.pixmap == parentPixmap {
							L.cpuDrew = true
							break
						}
					}
				}
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
		// P0-3: defer dual-tex to Present Flush (keep layer RT until resolve).
		if c.queueLayerAdvancedGPU(layer, parentPixmap) {
			layer.gpuRelease = nil
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
		// F1: damage-tight GPU composite — only blit the dirty UV rect instead
		// of the full layer RT (800x600 full-surface was the MULTI_LAYER tax).
		if !layer.fullComposite && !layer.damage.Empty() && layer.gpuW > 0 && layer.gpuH > 0 {
			const pad = 2
			d := layer.damage.Inset(-pad)
			d = d.Intersect(image.Rect(0, 0, layer.gpuW, layer.gpuH))
			if !d.Empty() {
				c.DrawGPUTextureWithOpacityUV(layer.gpuView,
					float64(d.Min.X), float64(d.Min.Y), d.Dx(), d.Dy(), op,
					float32(d.Min.X)/float32(layer.gpuW),
					float32(d.Min.Y)/float32(layer.gpuH),
					float32(d.Max.X)/float32(layer.gpuW),
					float32(d.Max.Y)/float32(layer.gpuH),
				)
			}
		} else {
			c.DrawGPUTextureWithOpacity(layer.gpuView, 0, 0, layer.gpuW, layer.gpuH, op)
		}
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

// queueLayerAdvancedGPU defers advanced blend to Flush when a present View exists.
// Offscreen unit-test / Image() path has View-nil targets; deferred dual-tex
// resolve seeds from CPU and historically wiped outside-clip base (D05 black).
// For pure-CPU parents, materialize the layer RT and composite immediately.
func (c *Context) queueLayerAdvancedGPU(layer *Layer, parent *Pixmap) bool {
	if c == nil || layer == nil || parent == nil || layer.gpuView.IsNil() {
		return false
	}
	// Prefer immediate CPU advanced composite when the active target stream is
	// pixmap-backed (no live present View). Correct for FlushGPU/Image.
	if c.preferImmediateAdvancedLayerComposite() {
		// Parent base draws may still be stashed/pending on GPU — compositeLayer
		// blends against parent.Data, so materialize the parent stream first.
		_ = c.FlushGPU()
		if !layer.cpuDrew {
			_ = c.materializeLayerGPUToPixmap(layer)
		}
		// Fall through to PopLayer CPU compositeLayer path.
		return false
	}
	type advancedLayerQueuer interface {
		QueueAdvancedLayerComposite(srcView gpucontext.TextureView, srcW, srcH int,
			damage image.Rectangle, mode BlendMode, opacity float64, release func())
	}
	var q advancedLayerQueuer
	if raw := c.GPURenderContext(); raw != nil {
		q, _ = raw.(advancedLayerQueuer)
	}
	if q == nil {
		return false
	}
	damage := layer.damage
	if layer.fullComposite {
		damage = image.Rectangle{}
	}
	release := layer.gpuRelease
	q.QueueAdvancedLayerComposite(layer.gpuView, layer.gpuW, layer.gpuH,
		damage, layer.blendMode, layer.opacity, release)
	return true
}

// preferImmediateAdvancedLayerComposite: keep deferred dual-tex queue for
// present FlushGPUWithView (F1). View-nil Image/FlushGPU advanced resolve is
// handled in GPURenderContext.resolvePendingAdvancedLayersEnc (CPU blend into
// pixmap Data when the original target has no View).
func (c *Context) preferImmediateAdvancedLayerComposite() bool {
	return false
}

// compositeLayerAdvancedGPU is kept for tests; routes to queueLayerAdvancedGPU.
func (c *Context) compositeLayerAdvancedGPU(layer *Layer, parent *Pixmap) bool {
	return c.queueLayerAdvancedGPU(layer, parent)
}

// layerForceCPUDraw reports whether the current top layer must receive CPU
// pixmap draws (no GPU RT). Opacity-group layers intentionally keep the parent
// GPU target — they must NOT force CPU (F1). Callers still mark
// noteLayerCPUDraw on real CPU writes to isolation layers.
func (c *Context) layerForceCPUDraw() bool {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return false
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top == nil {
		return true
	}
	// F1: Normal/Copy opacity-group draws into the parent surface with alpha mul.
	if top.opacityGroup {
		return false
	}
	if !top.gpuView.IsNil() {
		return false
	}
	return true
}

// opacityMulBrush multiplies ColorAt alpha by a constant (F1 opacity-group).
type opacityMulBrush struct {
	base Brush
	mul  float64
}

func (opacityMulBrush) brushMarker() {}

func (b opacityMulBrush) ColorAt(x, y float64) RGBA {
	if b.base == nil {
		return RGBA{}
	}
	c := b.base.ColorAt(x, y)
	c.A *= b.mul
	return c
}

// applyLayerOpacityMul temporarily multiplies paint alpha by open opacity-group
// layers. Returns a restore func (always non-nil; safe to defer).
func (c *Context) applyLayerOpacityMul() func() {
	if c == nil || c.paint == nil {
		return func() {}
	}
	mul := c.layerOpacityMul()
	if mul == 1 {
		return func() {}
	}
	if c.paint.isSolid {
		savedA := c.paint.solidColor.A
		c.paint.solidColor.A = savedA * mul
		return func() { c.paint.solidColor.A = savedA }
	}
	// Non-solid: wrap the effective brush so ColorAt / GPU field paths see mul.
	savedBrush := c.paint.Brush
	savedPattern := c.paint.Pattern
	savedIsSolid := c.paint.isSolid
	savedSolid := c.paint.solidColor
	base := c.paint.GetBrush()
	c.paint.Brush = opacityMulBrush{base: base, mul: mul}
	c.paint.Pattern = PatternFromBrush(c.paint.Brush)
	c.paint.isSolid = false
	return func() {
		c.paint.Brush = savedBrush
		c.paint.Pattern = savedPattern
		c.paint.isSolid = savedIsSolid
		c.paint.solidColor = savedSolid
	}
}

// layerOpacityMul returns the product of opacityGroup layer opacities currently
// on the stack (F1). Real isolation layers do not contribute here — their
// opacity is applied at Pop composite time.
func (c *Context) layerOpacityMul() float64 {
	if c == nil || c.layerStack == nil {
		return 1
	}
	m := 1.0
	for _, L := range c.layerStack.layers {
		if L != nil && L.opacityGroup {
			m *= L.opacity
		}
	}
	if m < 0 {
		return 0
	}
	if m > 1 {
		return 1
	}
	return m
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
