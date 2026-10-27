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

	gpucontext "github.com/energye/gpui/gpu/context"
	intImage "github.com/energye/gpui/render/internal/image"
)

// SetBlendMode sets the blend mode for subsequent fill and stroke operations.
//
// GPU fixed-function modes (B.02): BlendNormal (SourceOver), BlendCopy, BlendClear,
// BlendPlus, DstOut/SrcAtop/Xor, DstOver/SrcIn/SrcOut/DstIn/DstAtop.
// Advanced modes (Multiply/Screen/…) use resolve+CPU composite+GPU blit or CPU.
//
// Example:
//
//	dc.SetBlendMode(render.BlendMultiply)
//	dc.Fill()
func (c *Context) SetBlendMode(mode BlendMode) {

	c.paint.BlendMode = mode
}

// BlendMode reports the blend mode used by subsequent fills and strokes.
func (c *Context) BlendMode() BlendMode {
	if c == nil || c.paint == nil {
		return BlendNormal
	}
	return c.paint.BlendMode
}

// compositeLayer composites a layer onto a parent pixmap using the layer's
// blend mode and opacity.
//
// When layer.damage is known and fullComposite is false, only the damaged
// rectangle is blended (AA pad). This is required for continuous UI layers at
// 60fps — full-surface SourceOver of 800x600 was ~50ms on Intel HD.
// compositeLayer composites a layer onto a parent pixmap using the layer's
// blend mode and opacity.
//
// When layer.damage is known and fullComposite is false, only the damaged
// rectangle is blended (AA pad). This is required for continuous UI layers at
// 60fps — full-surface SourceOver of 800x600 was ~50ms on Intel HD.
func (c *Context) compositeLayer(layer *Layer, parent *Pixmap) {
	r := image.Rect(0, 0, layer.pixmap.Width(), layer.pixmap.Height())
	if layer.backdrop && layer.backdropBounded {
		// Skia saveLayer bounds: the backdrop effect lands only inside the clip
		// captured at push; outside keeps the parent's materialized content.
		r = r.Intersect(layer.backdropBounds)
		if r.Empty() {
			return
		}
	}
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

	// Normal layers SourceOver straight on the premultiplied pixmap bytes. The
	// generic intImage path reads its source as straight colours, which
	// multiplies partial-alpha layer content by alpha a second time: masked
	// layers carrying soft content used to read squared on the CPU path while
	// the GPU composite was already correct.
	if layer.blendMode == BlendNormal {
		compositePremulSourceOver(layer.pixmap, parent, r, layer.opacity)
		return
	}

	// Convert pixmaps to ImageBuf for blending
	srcImg := c.pixmapToImageBuf(layer.pixmap)
	dstImg := c.pixmapToImageBuf(parent)

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

// compositePremulSourceOver does SourceOver in premultiplied space over the
// given rectangle (physical pixels):
//
//	out = src*opacity + dst*(1 - srcA*opacity)
//
// Both pixmaps store premultiplied RGBA8 bytes, so no unpremultiply round trip
// is needed and partial-alpha content keeps its true coverage.
func compositePremulSourceOver(src, dst *Pixmap, r image.Rectangle, opacity float64) {
	if src == nil || dst == nil {
		return
	}
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	sw, sh := src.Width(), src.Height()
	dw, dh := dst.Width(), dst.Height()
	r = r.Intersect(image.Rect(0, 0, sw, sh)).Intersect(image.Rect(0, 0, dw, dh))
	if r.Empty() {
		return
	}
	sd, dd := src.Data(), dst.Data()
	op := int(opacity*255 + 0.5)
	for y := r.Min.Y; y < r.Max.Y; y++ {
		srow := y * sw * 4
		drow := y * dw * 4
		for x := r.Min.X; x < r.Max.X; x++ {
			si := srow + x*4
			di := drow + x*4
			sa := int(sd[si+3])
			if op < 255 {
				sa = sa * op / 255
			}
			if sa == 0 {
				continue
			}
			sr := int(sd[si])
			sg := int(sd[si+1])
			sb := int(sd[si+2])
			if op < 255 {
				sr = sr * op / 255
				sg = sg * op / 255
				sb = sb * op / 255
			}
			inv := 255 - sa
			dd[di+0] = uint8(sr + int(dd[di+0])*inv/255)
			dd[di+1] = uint8(sg + int(dd[di+1])*inv/255)
			dd[di+2] = uint8(sb + int(dd[di+2])*inv/255)
			dd[di+3] = uint8(sa + int(dd[di+3])*inv/255)
		}
	}
}

// noteLayerDamage unions a pixmap-space rectangle into the current top layer.
// bounds should already be in the same pixel space as the layer pixmap
// (physical when deviceScale!=1 after trackDamage scaling).
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
// markLayerFullComposite forces the current top layer to full-surface Pop blend.
func (c *Context) markLayerFullComposite() {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	c.layerStack.layers[len(c.layerStack.layers)-1].fullComposite = true
}

// noteParentCompositeDamage unions a popped child's composite rect into the
// enclosing isolation layer's damage. Without it a damage-limited parent
// composite drops nested layer/texture content (nested backdrop inside a layer
// that also drew vector art was clipped to the vector bounds).
func (c *Context) noteParentCompositeDamage(rect image.Rectangle) {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 || rect.Empty() {
		return
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top == nil || top.opacityGroup || top.fullComposite {
		return
	}
	top.damage = top.damage.Union(rect)
}

// markLayerBackdrop marks the current top layer as a backdrop snapshot layer.
// bounds/bounded come from the clip active at push (Skia saveLayer bounds): the
// filtered snapshot may only land inside them; the Pop blit itself stays
// full-surface so the parent content outside the bounds is not blacked out.
func (c *Context) markLayerBackdrop(bounds image.Rectangle, bounded bool) {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	top.backdrop = true
	top.backdropBounds = bounds
	top.backdropBounded = bounded
}

// cropFilterOutputToLayerBounds restores the pixels outside a bounded backdrop
// layer's clip after a filter ran over the whole layer content.
//
// Skia crops a saveLayer filter's output to the layer bounds; Flutter's
// BackdropFilterLayer documents the same ("applied to all the area within its
// parent or ancestor widget's clip"). The layer's outside-bounds pixels must
// stay identical to the parent's, because the Pop composites the whole layer
// texture (full-layer blit is what re-materializes the surface on windowed
// present) — a blurred outside would bleed the effect over the parent.
//
// Call after the filter result landed in the layer pixmap and before the layer
// RT is seeded from it.
func (c *Context) cropFilterOutputToLayerBounds() {
	if c == nil || c.pixmap == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	layer := c.layerStack.layers[len(c.layerStack.layers)-1]
	if layer == nil || !layer.backdrop || !layer.backdropBounded || layer.pixmap == nil {
		return
	}
	if layer.pixmap != c.pixmap {
		return // filter result must live in the active layer pixmap
	}
	parent := c.enclosingNonOpacityPixmap()
	if parent == nil || parent == layer.pixmap {
		return
	}
	w, h := layer.pixmap.Width(), layer.pixmap.Height()
	if parent.Width() != w || parent.Height() != h {
		return
	}
	b := layer.backdropBounds.Intersect(image.Rect(0, 0, w, h))
	if b.Empty() {
		return
	}
	lp := layer.pixmap.Data()
	pp := parent.Data()
	if len(lp) < w*h*4 || len(pp) < w*h*4 {
		return
	}
	row := w * 4
	// Full-width bands above and below the bounds.
	copyRows := func(y0, y1 int) {
		if y0 < 0 {
			y0 = 0
		}
		if y1 > h {
			y1 = h
		}
		for y := y0; y < y1; y++ {
			o := y * row
			copy(lp[o:o+row], pp[o:o+row])
		}
	}
	copyRows(0, b.Min.Y)
	copyRows(b.Max.Y, h)
	// Left and right strips inside the bounds rows.
	left := b.Min.X * 4
	right := b.Max.X * 4
	for y := b.Min.Y; y < b.Max.Y; y++ {
		o := y * row
		if left > 0 {
			copy(lp[o:o+left], pp[o:o+left])
		}
		if right < row {
			copy(lp[o+right:o+row], pp[o+right:o+row])
		}
	}
	layer.pixmap.NotifyPixelsChanged()
}

// enclosingNonOpacityPixmap walks past the top layer to the parent surface
// pixmap (enclosing isolation layer, else the base pixmap recorded at push).
func (c *Context) enclosingNonOpacityPixmap() *Pixmap {
	if c == nil || c.layerStack == nil {
		return nil
	}
	for i := len(c.layerStack.layers) - 2; i >= 0; i-- {
		L := c.layerStack.layers[i]
		if L == nil || L.opacityGroup {
			continue
		}
		if L.pixmap != nil {
			return L.pixmap
		}
	}
	return c.basePixmap
}

// queueLayerAdvancedGPU defers advanced blend to Flush when a present View exists.
// Offscreen unit-test / Image() path has View-nil targets; deferred dual-tex
// queueLayerAdvancedGPU defers advanced blend to dual-tex resolve on the next
// GPU flush (F1). View-nil Image/FlushGPU advanced resolve is handled in
// GPURenderContext.resolvePendingAdvancedLayersEnc (CPU blend into pixmap Data
// when the original target has no View).
// queueLayerAdvancedGPU defers advanced blend to Flush when a present View exists.
// Offscreen unit-test / Image() path has View-nil targets; deferred dual-tex
// queueLayerAdvancedGPU defers advanced blend to dual-tex resolve on the next
// GPU flush (F1). View-nil Image/FlushGPU advanced resolve is handled in
// GPURenderContext.resolvePendingAdvancedLayersEnc (CPU blend into pixmap Data
// when the original target has no View).
func (c *Context) queueLayerAdvancedGPU(layer *Layer, parent *Pixmap) bool {
	if c == nil || layer == nil || parent == nil || layer.gpuView.IsNil() {
		return false
	}
	type advancedLayerQueuer interface {
		QueueAdvancedLayerComposite(srcView gpucontext.TextureView, srcW, srcH int,
			damage image.Rectangle, mode BlendMode, opacity float64,
			dstView gpucontext.TextureView, release func())
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
	// Nested saveLayer: the enclosing isolation RT is the destination. Zero
	// means base/present surface. This lets FlushOffscreen skip pops that
	// belong to the parent instead of blending them into the wrong layer.
	var dstView gpucontext.TextureView
	if c.layerStack != nil {
		for i := len(c.layerStack.layers) - 1; i >= 0; i-- {
			L := c.layerStack.layers[i]
			if L == nil || L.opacityGroup {
				continue
			}
			if !L.gpuView.IsNil() {
				dstView = L.gpuView
			}
			break
		}
	}
	q.QueueAdvancedLayerComposite(layer.gpuView, layer.gpuW, layer.gpuH,
		damage, layer.blendMode, layer.opacity, dstView, release)
	return true
}

// layerForceCPUDraw reports whether the current top layer must receive CPU
// pixmap draws (no GPU RT). Opacity-group layers intentionally keep the parent
// GPU target — they must NOT force CPU (F1). Callers still mark
// noteLayerCPUDraw on real CPU writes to isolation layers.
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
// applyLayerOpacityMul temporarily multiplies paint alpha by open opacity-group
// layers. Returns a restore func (always non-nil; safe to defer).
func (c *Context) applyLayerOpacityMul() func() {
	if c == nil || c.paint == nil {
		return func() {}
	}
	// Destination-dependent operators take their source alpha literally (DstIn
	// is a mask), so a group opacity must not scale it here. Normal/Copy place
	// src over the group and keep the alpha multiply.
	if c.paint.BlendMode != BlendNormal && c.paint.BlendMode != BlendCopy {
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

// materializeLayerGPUToPixmap readbacks a layer GPU RT into its pixmap.
// materializeLayerGPUToPixmap readbacks a layer GPU RT into its pixmap.
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

// materializeLayerForCPU folds pending GPU content of the top isolation layer
// into its CPU pixmap before a software fallback draws into that pixmap.
// Without it PopLayer would CPU-composite a surface that never saw the GPU
// draws (e.g. a gradient DstIn mask that fell back), losing that content.
func (c *Context) materializeLayerForCPU() {
	if c == nil || c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	layer := c.layerStack.layers[len(c.layerStack.layers)-1]
	if layer == nil || layer.gpuView.IsNil() || layer.cpuDrew {
		return
	}
	_ = c.flushLayerViewGPU(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH), nil) //nolint:gosec // bounded
	_ = c.materializeLayerGPUToPixmap(layer)
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
	// The upload lands outside a render pass; tell the session the view already
	// has color content so its first pass loads instead of clearing the seed
	// (Skia saveLayer-with-backdrop keeps the backdrop image).
	if mk, ok := raw.(interface {
		MarkViewSeeded(view gpucontext.TextureView)
	}); ok && mk != nil {
		mk.MarkViewSeeded(top.gpuView)
	}
	// GPU RT matches pixmap; allow GPU Pop composite again.
	top.cpuDrew = false
	return true
}

// compositeLayerMaskedGPU dual-path: GPU layer RT × R8 mask → parent.
// compositeLayerMaskedGPU dual-path: GPU layer RT × R8 mask → parent.
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

// queueMaskedLayerGPU modulates the layer RT by its PushMaskLayer mask on the
// GPU and queues a replayable composite draw onto the parent target (Skia
// saveLayer-with-mask semantics: the mask is coverage in the composite draw,
// and the frame stays fully replayable for PresentFrame). Returns false when
// the GPU session cannot take it — the caller then falls back to the CPU
// readback composite path.
func (c *Context) queueMaskedLayerGPU(layer *Layer) bool {
	if c == nil || layer == nil || layer.mask == nil || layer.gpuView.IsNil() {
		return false
	}
	type maskedViewCompositor interface {
		ModulateMaskedLayerGPU(srcView gpucontext.TextureView, srcW, srcH int, mask *Mask) (gpucontext.TextureView, int, int, func(), error)
	}
	raw := c.GPURenderContext()
	mc, ok := raw.(maskedViewCompositor)
	if !ok || mc == nil {
		return false
	}
	view, w, h, release, err := mc.ModulateMaskedLayerGPU(layer.gpuView, layer.gpuW, layer.gpuH, layer.mask)
	if err != nil || view.IsNil() || release == nil || w <= 0 || h <= 0 {
		return false
	}
	// Layer RTs / modulated textures are device-space 1:1 — composite without CTM.
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
	rc := c.gpuCtxOps()
	before := 0
	if rc != nil {
		before = rc.PendingCount()
	}
	c.DrawGPUTextureWithOpacity(view, 0, 0, w, h, op)
	c.matrix = savedMat
	c.deviceMatrix = savedDev
	// The queued composite still samples the modulated texture; release it only
	// once the GPU has no outstanding work (same contract as layer RTs).
	c.layerGPUReleases = append(c.layerGPUReleases, release)
	if rc == nil || rc.PendingCount() <= before {
		// Draw did not queue (session refused it) — let the caller fall back.
		// The registered release rides to the next drain; the queue path simply
		// never claimed the composite.
		return false
	}
	return true
}

// drainLayerGPUReleases releases GPU layer textures whose composite draws have
// already been flushed. Safe to call repeatedly.
//
// C5 fix (use-after-free): a queued-but-unencoded composite quad — pending OR
// present-stashed — still references its layer texture. A later layer Pop's
// mid-frame flush used to drain releases unconditionally, freeing a texture
// whose composite quad was still stashed (earlier card vanished; R6 single
// card never triggered it). Only release when the GPU side truly holds no
// outstanding work; otherwise the releases ride to the next frame boundary
// (BeginFrame fallback) — Skia GrSurfaceProxy keeps refs until submit, same
// semantics.
// drainLayerGPUReleases releases GPU layer textures whose composite draws have
// already been flushed. Safe to call repeatedly.
//
// C5 fix (use-after-free): a queued-but-unencoded composite quad — pending OR
// present-stashed — still references its layer texture. A later layer Pop's
// mid-frame flush used to drain releases unconditionally, freeing a texture
// whose composite quad was still stashed (earlier card vanished; R6 single
// card never triggered it). Only release when the GPU side truly holds no
// outstanding work; otherwise the releases ride to the next frame boundary
// (BeginFrame fallback) — Skia GrSurfaceProxy keeps refs until submit, same
// semantics.
func (c *Context) drainLayerGPUReleases() {
	if c == nil || len(c.layerGPUReleases) == 0 {
		return
	}
	if rc := c.gpuCtxOps(); rc != nil && (rc.PendingCount() > 0 || rc.HasPendingStash()) {
		return
	}
	for _, rel := range c.layerGPUReleases {
		if rel != nil {
			rel()
		}
	}
	c.layerGPUReleases = c.layerGPUReleases[:0]
}

// acquireEffectPublishView allocates a pooled TextureBinding RT for F14 effect
// FlushGPU publish. Caller must pass release to attachFilterGPUResult (or call it).
