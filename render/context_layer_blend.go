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

// seedTopLayerGPUFromPixmap uploads the top layer pixmap into its GPU RT (L.05).
// Keeps GPU RT coherent after CPU snapshot / filter writes.
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
