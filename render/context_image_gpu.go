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
)

// tryGPUDrawImage attempts to render the image via GPU Tier 3 (textured quad).
// Returns true if the image was queued for GPU rendering, false if the caller
// should fall back to the CPU SetFillPattern→Fill() path.
//
// The destination is always a full CTM-transformed parallelogram (TL/TR/BR/BL).
// Rotation and skew are handled by emitting non-axis-aligned quad vertices;
// the previous axis-aligned-only gate incorrectly fell back to ImagePattern
// Fill, which GPU solid-path rendering cannot texture.
func (c *Context) tryGPUDrawImage(img *ImageBuf, opts DrawImageOptions, srcX, srcY, srcW, srcH int, dstWidth, dstHeight float64) bool {
	rc := c.gpuCtxOps()
	if rc == nil {
		if c.gpuPathAvailable() {
			c.recordCPUFallbackReason("image:tryGPUDrawImage")
		}
		return false
	}
	// No-device gate: only claim the draw when the GPU session can execute
	// it. Without a device the queued quad never lands in the pixmap while
	// the CPU fallback is skipped.
	// Optional interface keeps third-party rc impls on old behavior.
	if dr, ok := rc.(interface{ IsDeviceReady() bool }); ok && !dr.IsDeviceReady() {
		c.recordCPUFallbackReason("image:no-device")
		return false
	}

	defer c.setGPUClipRect()()

	// Transform all four user-space corners through the full CTM so rotation,
	// scale, shear, and translation are preserved in device pixels.
	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(opts.X, opts.Y))
	tr := ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y))
	br := ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y+dstHeight))
	bl := ctm.TransformPoint(Pt(opts.X, opts.Y+dstHeight))

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
	vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32

	// Compute source UV rectangle (normalized 0..1 within the image).
	imgW, imgH := img.Bounds()
	u0 := float32(srcX) / float32(imgW)
	v0 := float32(srcY) / float32(imgH)
	u1 := float32(srcX+srcW) / float32(imgW)
	v1 := float32(srcY+srcH) / float32(imgH)

	// Get premultiplied pixel data for GPU upload.
	pixelData := img.PremultipliedData()
	if len(pixelData) == 0 {
		c.recordCPUFallbackReason("image:tryGPUDrawImage")
		return false
	}

	nearest := opts.Interpolation == InterpNearest
	bicubic := opts.Interpolation == InterpBicubic
	contentDirty := img.TakeGPUDirty()
	rc.QueueImageDraw(target, pixelData, img.GenerationID(), imgW, imgH, img.Stride(),
		float32(tl.X), float32(tl.Y),
		float32(tr.X), float32(tr.Y),
		float32(br.X), float32(br.Y),
		float32(bl.X), float32(bl.Y),
		float32(opts.Opacity), vpW, vpH, u0, v0, u1, v1, nearest, contentDirty, bicubic)
	c.recordGPUOp()
	return true
}

// CreateImagePattern creates an image pattern from a rectangular region of an image.
// The pattern can be used with SetFillPattern or SetStrokePattern.
//
// Example:
//
//	img, _ := render.LoadImage("texture.png")
//	pattern := dc.CreateImagePattern(img, 0, 0, 100, 100)
//	dc.SetFillPattern(pattern)
//	dc.DrawRectangle(0, 0, 400, 300)
//	dc.Fill()
//
// prepareGPUTextureDraw shares sync/validate/CTM/viewport setup for DrawGPUTexture*.
// Returns ok=false when GPU is unavailable or view is nil.
func (c *Context) prepareGPUTextureDraw(view gpucontext.TextureView, x, y float64, width, height int) (g gpuTextureDrawGeom, ok bool) {
	c.syncPublishedFilterBeforeDraw()
	rc := c.gpuCtxOps()
	if rc == nil || view.IsNil() {
		return g, false
	}
	// Full CTM quad (same as tryGPUDrawImage): rotation/scale/shear ride
	// the four corners, Dst rect stays the AABB for damage/scissor.
	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(x, y))
	tr := ctm.TransformPoint(Pt(x+float64(width), y))
	br := ctm.TransformPoint(Pt(x+float64(width), y+float64(height)))
	bl := ctm.TransformPoint(Pt(x, y+float64(height)))
	minX := min4(tl.X, tr.X, br.X, bl.X)
	maxX := max4(tl.X, tr.X, br.X, bl.X)
	minY := min4(tl.Y, tr.Y, br.Y, bl.Y)
	maxY := max4(tl.Y, tr.Y, br.Y, bl.Y)
	target := c.gpuRenderTarget()
	return gpuTextureDrawGeom{
		rc:     rc,
		target: target,
		dstX:   float32(minX),
		dstY:   float32(minY),
		dstW:   float32(maxX - minX),
		dstH:   float32(maxY - minY),
		tlX:    float32(tl.X),
		tlY:    float32(tl.Y),
		trX:    float32(tr.X),
		trY:    float32(tr.Y),
		brX:    float32(br.X),
		brY:    float32(br.Y),
		blX:    float32(bl.X),
		blY:    float32(bl.Y),
		vpW:    uint32(target.Width),  //nolint:gosec // viewport fits uint32
		vpH:    uint32(target.Height), //nolint:gosec // viewport fits uint32
	}, true
}

// DrawGPUTexture composites an existing GPU texture view as a textured quad
// at (x, y) with the given dimensions. No CPU readback or upload — pure
// GPU-to-GPU compositing. The view must be from the same device (e.g.,
// FlushGPUWithView output). CTM and scissor clip are applied.
//
// This is the Skia GrSurfaceProxyView direct-bind pattern for cached
// offscreen rendering (RepaintBoundary, layer compositing).
// DrawGPUTexture composites an existing GPU texture view as a textured quad
// at (x, y) with the given dimensions. No CPU readback or upload — pure
// GPU-to-GPU compositing. The view must be from the same device (e.g.,
// FlushGPUWithView output). CTM and scissor clip are applied.
//
// This is the Skia GrSurfaceProxyView direct-bind pattern for cached
// offscreen rendering (RepaintBoundary, layer compositing).
func (c *Context) DrawGPUTexture(view gpucontext.TextureView, x, y float64, width, height int) {
	c.DrawGPUTextureWithOpacity(view, x, y, width, height, 1.0)
}

// DrawGPUTextureWithOpacity composites a GPU texture view as an overlay with
// the specified opacity (0.0 = fully transparent, 1.0 = fully opaque).
// Same as DrawGPUTexture but with alpha blending for fade transitions
// and OpacityLayer compositing (Flutter pattern).
//
// R6 group-opacity fix: a texture blit inside an open opacity-group layer
// (F1 PushLayer(Normal, α) fast path) must inherit the group alpha exactly
// like vector fills do via applyLayerOpacityMul. Without this, an animating
// RenderOpacity whose child rides a retained texture cache blits at full
// strength every frame — the card stops breathing (vector snapshot paths
// looked correct, masking the split). Skia saveLayer multiplies group alpha
// into every draw record; same semantics here.
// DrawGPUTextureWithOpacity composites a GPU texture view as an overlay with
// the specified opacity (0.0 = fully transparent, 1.0 = fully opaque).
// Same as DrawGPUTexture but with alpha blending for fade transitions
// and OpacityLayer compositing (Flutter pattern).
//
// R6 group-opacity fix: a texture blit inside an open opacity-group layer
// (F1 PushLayer(Normal, α) fast path) must inherit the group alpha exactly
// like vector fills do via applyLayerOpacityMul. Without this, an animating
// RenderOpacity whose child rides a retained texture cache blits at full
// strength every frame — the card stops breathing (vector snapshot paths
// looked correct, masking the split). Skia saveLayer multiplies group alpha
// into every draw record; same semantics here.
func (c *Context) DrawGPUTextureWithOpacity(view gpucontext.TextureView, x, y float64, width, height int, opacity float32) {
	g, ok := c.prepareGPUTextureDraw(view, x, y, width, height)
	if !ok {
		return
	}
	defer c.setGPUClipRect()()
	if mul := c.layerOpacityMul(); mul < 1 {
		opacity *= float32(mul)
	}
	g.rc.QueueGPUTextureDrawQuad(g.target, view, g.dstX, g.dstY, g.dstW, g.dstH,
		g.tlX, g.tlY, g.trX, g.trY, g.brX, g.brY, g.blX, g.blY, opacity, g.vpW, g.vpH)
	c.recordGPUOp()
}

// DrawVideoPlanes composites one uploaded NV12 plane slot with GPU-side
// YUV→RGB conversion (P3-A layer 4: uniform + Y + UV + sampler through
// the YUV convert pipeline, both backends from one WGSL source). Same
// fail-closed contract as DrawVideoSlot: false draws nothing and the
// caller keeps the CPU fallback (bridge converts into its owned buffer).
// DrawVideoPlanes composites one uploaded NV12 plane slot with GPU-side
// YUV→RGB conversion (P3-A layer 4: uniform + Y + UV + sampler through
// the YUV convert pipeline, both backends from one WGSL source). Same
// fail-closed contract as DrawVideoSlot: false draws nothing and the
// caller keeps the CPU fallback (bridge converts into its owned buffer).
func (c *Context) DrawVideoPlanes(slot *VideoPlaneSlot, opts VideoDrawOptions) bool {
	if c == nil || slot == nil || slot.YView == nil || slot.UVView == nil {
		return false
	}
	if slot.W <= 0 || slot.H <= 0 {
		return false
	}
	if opts.Opacity < 0 {
		return false
	}
	yView, uvView := slot.packedPlaneViews()
	if yView.IsNil() || uvView.IsNil() {
		return false
	}
	c.syncPublishedFilterBeforeDraw()
	rc := c.gpuCtxOps()
	if rc == nil {
		return false
	}
	// No-device gate (same as DrawVideoSlot): only claim the draw when
	// the GPU session can execute it.
	if dr, ok := rc.(interface{ IsDeviceReady() bool }); ok && !dr.IsDeviceReady() {
		return false
	}
	// Optional queue port: sessions without the YUV entry keep the CPU
	// fallback instead of breaking older implementers.
	qp, ok := rc.(interface {
		QueueGPUVideoPlanesDraw(target GPURenderTarget, yView, uvView gpucontext.TextureView, videoW, videoH int, dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32) bool
	})
	if !ok {
		return false
	}
	dw, dh := opts.DstWidth, opts.DstHeight
	if dw <= 0 {
		dw = float64(slot.W)
	}
	if dh <= 0 {
		dh = float64(slot.H)
	}
	opacity := float32(opts.Opacity)
	if opacity == 0 {
		opacity = 1.0
	}
	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(opts.X, opts.Y))
	br := ctm.TransformPoint(Pt(opts.X+dw, opts.Y+dh))
	target := c.gpuRenderTarget()
	defer c.setGPUClipRect()()
	if mul := c.layerOpacityMul(); mul < 1 {
		opacity *= float32(mul)
	}
	ok = qp.QueueGPUVideoPlanesDraw(target, yView, uvView, slot.W, slot.H,
		float32(tl.X), float32(tl.Y),
		float32(br.X-tl.X), float32(br.Y-tl.Y),
		opacity, uint32(target.Width), uint32(target.Height)) //nolint:gosec // viewport fits uint32
	if !ok {
		return false
	}
	c.recordGPUOp()
	c.TrackDamageRect(image.Rect(int(opts.X), int(opts.Y), int(opts.X+dw), int(opts.Y+dh)))
	return true
}

// DrawVideoSlot composites one uploaded video slot as a textured quad.
// Fast path for P2 direct upload: the slot texture was rewritten in place
// by VideoTexturePool.Upload, so this call queues zero-upload GPU-to-GPU
// compositing and returns true. It reuses the DrawGPUTexture queue path
// (CTM + scissor clip + group-opacity inherit) and adds no work to the
// generic DrawImage path: existing callers never enter here.
//
// Fail-closed: no GPU session, nil slot/view, or bad size returns false;
// the caller then falls back to DrawVideoFrame (generic DrawImage) and
// counts one CPU fallback. Sampling uses the GPU default (bilinear-grade);
// per-mode interpolation selection stays on the generic path.
// DrawVideoSlot composites one uploaded video slot as a textured quad.
// Fast path for P2 direct upload: the slot texture was rewritten in place
// by VideoTexturePool.Upload, so this call queues zero-upload GPU-to-GPU
// compositing and returns true. It reuses the DrawGPUTexture queue path
// (CTM + scissor clip + group-opacity inherit) and adds no work to the
// generic DrawImage path: existing callers never enter here.
//
// Fail-closed: no GPU session, nil slot/view, or bad size returns false;
// the caller then falls back to DrawVideoFrame (generic DrawImage) and
// counts one CPU fallback. Sampling uses the GPU default (bilinear-grade);
// per-mode interpolation selection stays on the generic path.
func (c *Context) DrawVideoSlot(slot *VideoSlot, opts VideoDrawOptions) bool {
	if c == nil || slot == nil || slot.View == nil || slot.Texture == nil {
		return false
	}
	if slot.W <= 0 || slot.H <= 0 {
		return false
	}
	if opts.Opacity < 0 {
		return false
	}
	view := slot.packedView()
	if view.IsNil() {
		return false
	}
	c.syncPublishedFilterBeforeDraw()
	rc := c.gpuCtxOps()
	if rc == nil {
		return false
	}
	// No-device gate (same as tryGPUDrawImage): only claim the draw when
	// the GPU session can execute it. Without a device the queued quad
	// never lands in the pixmap while the CPU fallback is skipped.
	if dr, ok := rc.(interface{ IsDeviceReady() bool }); ok && !dr.IsDeviceReady() {
		return false
	}
	dw, dh := opts.DstWidth, opts.DstHeight
	if dw <= 0 {
		dw = float64(slot.W)
	}
	if dh <= 0 {
		dh = float64(slot.H)
	}
	opacity := float32(opts.Opacity)
	if opacity == 0 {
		opacity = 1.0
	}
	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(opts.X, opts.Y))
	br := ctm.TransformPoint(Pt(opts.X+dw, opts.Y+dh))
	target := c.gpuRenderTarget()
	defer c.setGPUClipRect()()
	if mul := c.layerOpacityMul(); mul < 1 {
		opacity *= float32(mul)
	}
	rc.QueueGPUTextureDraw(target, view,
		float32(tl.X), float32(tl.Y),
		float32(br.X-tl.X), float32(br.Y-tl.Y),
		opacity, uint32(target.Width), uint32(target.Height)) //nolint:gosec // viewport fits uint32
	c.recordGPUOp()
	// Damage follows the logical dest rect so PresentFrameDamage skips
	// clean frames: no-new-frame callers skip Show entirely, so no damage
	// is recorded and the last texture stays on screen untouched.
	c.TrackDamageRect(image.Rect(int(opts.X), int(opts.Y), int(opts.X+dw), int(opts.Y+dh)))
	return true
}

// DrawGPUTextureWithOpacityUV composites a sub-rectangle of a GPU texture with
// opacity. u0..v1 are normalized source UVs (F1 damage-tight layer composite).
// Inherits open opacity-group alpha, same as DrawGPUTextureWithOpacity.
// DrawGPUTextureWithOpacityUV composites a sub-rectangle of a GPU texture with
// opacity. u0..v1 are normalized source UVs (F1 damage-tight layer composite).
// Inherits open opacity-group alpha, same as DrawGPUTextureWithOpacity.
func (c *Context) DrawGPUTextureWithOpacityUV(view gpucontext.TextureView, x, y float64, width, height int, opacity float32, u0, v0, u1, v1 float32) {
	g, ok := c.prepareGPUTextureDraw(view, x, y, width, height)
	if !ok {
		return
	}
	defer c.setGPUClipRect()()
	if mul := c.layerOpacityMul(); mul < 1 {
		opacity *= float32(mul)
	}

	type uvDrawer interface {
		QueueGPUTextureDrawQuadUV(target GPURenderTarget, view gpucontext.TextureView,
			dstX, dstY, dstW, dstH, tlX, tlY, trX, trY, brX, brY, blX, blY, opacity float32, vpW, vpH uint32,
			u0, v0, u1, v1 float32)
	}
	if ud, ok := g.rc.(uvDrawer); ok {
		ud.QueueGPUTextureDrawQuadUV(g.target, view, g.dstX, g.dstY, g.dstW, g.dstH,
			g.tlX, g.tlY, g.trX, g.trY, g.brX, g.brY, g.blX, g.blY, opacity, g.vpW, g.vpH, u0, v0, u1, v1)
	} else {
		g.rc.QueueGPUTextureDraw(g.target, view, g.dstX, g.dstY, g.dstW, g.dstH, opacity, g.vpW, g.vpH)
	}
	c.recordGPUOp()
}

// DrawGPUTextureBase composites a GPU texture view as the compositor base layer.
// The base layer is drawn BEFORE all GPU tiers (SDF, convex, stencil, text) in
// the render pass, making it the background for zero-readback rendering.
//
// Use this to composite a CPU pixmap texture as the background, with GPU shapes
// rendered on top in the same render pass. Last call per frame wins.
//
// See ADR-015 (Compositor Base Layer), Flutter OffsetLayer pattern.
// DrawGPUTextureBase composites a GPU texture view as the compositor base layer.
// The base layer is drawn BEFORE all GPU tiers (SDF, convex, stencil, text) in
// the render pass, making it the background for zero-readback rendering.
//
// Use this to composite a CPU pixmap texture as the background, with GPU shapes
// rendered on top in the same render pass. Last call per frame wins.
//
// See ADR-015 (Compositor Base Layer), Flutter OffsetLayer pattern.
func (c *Context) DrawGPUTextureBase(view gpucontext.TextureView, x, y float64, width, height int) {
	g, ok := c.prepareGPUTextureDraw(view, x, y, width, height)
	if !ok {
		return
	}
	// Base layer intentionally does not set scissor clip (matches prior behavior).
	g.rc.QueueBaseLayer(g.target, view, g.dstX, g.dstY, g.dstW, g.dstH, 1.0, g.vpW, g.vpH)
	c.recordGPUOp()
}

// TextureView is an opaque GPU texture handle re-exported
// for ui-layer retained compositing (picture/layer texture caches). ui/ must
// not import gpu/ directly; this alias is the ui → render boundary for
// CreateOffscreenTexture / DrawGPUTexture* handles.
type TextureView = gpucontext.TextureView

// CreateOffscreenTexture allocates a GPU texture for offscreen rendering.
// The texture can be rendered to via FlushGPUWithView and composited via
// DrawGPUTexture. Returns (nil, nil) if GPU is not available.
//
// The returned TextureView is valid until release() is called.
// Call release() to return the texture resources to the GPU.
//
// Usage flags: RenderAttachment | CopySrc | TextureBinding.
// CreateOffscreenTexture allocates a GPU texture for offscreen rendering.
// The texture can be rendered to via FlushGPUWithView and composited via
// DrawGPUTexture. Returns (nil, nil) if GPU is not available.
//
// The returned TextureView is valid until release() is called.
// Call release() to return the texture resources to the GPU.
//
// Usage flags: RenderAttachment | CopySrc | TextureBinding.
func (c *Context) CreateOffscreenTexture(width, height int) (gpucontext.TextureView, func()) {
	rc := c.gpuCtxOps()
	if rc == nil {
		return gpucontext.TextureView{}, nil
	}
	type offscreenCreator interface {
		CreateOffscreenTexture(w, h int) (gpucontext.TextureView, func())
	}
	if oc, ok := rc.(offscreenCreator); ok {
		return oc.CreateOffscreenTexture(width, height)
	}
	return gpucontext.TextureView{}, nil
}
