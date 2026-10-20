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
	"time"

	gpucontext "github.com/energye/gpui/gpu/context"
)

func (c *Context) recordGPUOp() {
	c.pathStats.GPUOps++
}

func (c *Context) recordFrameFlush() {
	c.pathStats.FrameFlushes++
}

// recordCPUFallbackReason increments the CPU fallback counter and records a short reason.
// Reason is best-effort diagnostic for soak/leak tools (last reason wins).
// A fallback during a scratch swap lands pixels in scratch — mark it dirty
// so Commit uploads (and Begin pre-clears) instead of skipping.
func (c *Context) recordCPUFallbackReason(reason string) {
	c.pathStats.CPUFallbackOps++
	if reason != "" {
		c.pathStats.LastCPUFallbackReason = reason
	}
	// The CPU now owns surface pixels the pending GPU queue does not encode;
	// zero-readback GPU consumers must seed from the pixmap (see
	// pixmapBaseContent).
	if c != nil {
		c.pixmapBaseContent = true
	}
	if c != nil && c.passMain != nil {
		c.passScratchDirty = true
	}
}

// PixmapForTest exposes the live pixmap (scratch while a pass swap is
// active). Tests only.
// BeginGPUFrame resets per-context GPU frame state so the next render pass
// uses LoadOpClear. Call this on persistent contexts before re-rendering
// to the same view — without it, frameRendered=true from the previous frame
// causes LoadOpLoad, preserving stale content.
//
// Not needed for one-shot contexts (NewContext + Close per frame).
// Not needed when the view changes between frames (auto-reset on view change).
func (c *Context) BeginGPUFrame() {
	c.ensureGPUCtx()
	if c.baseGpuCtx != nil {
		c.baseGpuCtx.BeginFrame()
	}
	if c.layerGpuCtx != nil {
		// Layer textures are pooled and reused across frames; reset the layer
		// session's LoadOp bookkeeping too, otherwise a recycled layer view can
		// be treated as already rendered and skip its clear.
		c.layerGpuCtx.BeginFrame()
	}
}

// MemDigCmdBufs returns retained GPU command buffer count after last submit.
// prev=-1 means no GPU session yet. usedSurface indicates surface present path.
// MemDigCmdBufs returns retained GPU command buffer count after last submit.
// prev=-1 means no GPU session yet. usedSurface indicates surface present path.
func (c *Context) MemDigCmdBufs() (prev int, usedSurface bool) {
	if c == nil {
		return -1, false
	}
	type dig interface {
		DigCmdBufStats() (int, bool)
	}
	if rc := c.gpuCtxOps(); rc != nil {
		if d, ok := rc.(dig); ok {
			return d.DigCmdBufStats()
		}
	}
	return -1, false
}

// gpuFlushOffscreenTarget flushes pending GPU ops whose target View is an
// offscreen isolation RT (layer / effect publish view) rather than the window
// surface. Accelerators implementing the offscreen extension clear the target
// transparent (Skia saveLayer base semantics); an opaque clear would paint a
// fresh layer RT opaque black under its first composite blit (nested
// isolation). Accelerators without the extension keep the plain Flush.
func gpuFlushOffscreenTarget(rc gpuContextOps, t GPURenderTarget) error {
	type offscreenFlusher interface {
		FlushOffscreen(GPURenderTarget) error
	}
	if of, ok := rc.(offscreenFlusher); ok {
		return of.FlushOffscreen(t)
	}
	return rc.Flush(t)
}

// gpuFlushOffscreenBaseTarget is gpuFlushOffscreenTarget for an effect-publish
// RT that stands in for the base surface. Base-destined advanced layers must
// resolve into this RT, not remain queued for the (never flushed) base view.
func gpuFlushOffscreenBaseTarget(rc gpuContextOps, t GPURenderTarget) error {
	type offscreenBaseFlusher interface {
		FlushOffscreenBase(GPURenderTarget) error
	}
	if bf, ok := rc.(offscreenBaseFlusher); ok {
		return bf.FlushOffscreenBase(t)
	}
	return gpuFlushOffscreenTarget(rc, t)
}

func (c *Context) FlushGPU() error {
	c.recordFrameFlush()
	rc := c.gpuCtxOps()
	pending := 0
	pendingAdvanced := false
	if rc != nil {
		type pcounter interface{ PendingCount() int }
		if pc, ok := rc.(pcounter); ok {
			pending = pc.PendingCount()
		}
		if pa, ok := rc.(interface{ HasPendingAdvanced() bool }); ok {
			pendingAdvanced = pa.HasPendingAdvanced()
		}
	}
	t := c.gpuRenderTarget()
	// A non-nil View here is the top isolated layer's RT (gpuRenderTarget picks
	// it): that flush is offscreen, so the compositor must clear transparent.
	offscreen := !t.View.IsNil()
	var err error

	// F14: effect offscreen — flush into pooled TextureBinding RT and publish for
	// zero-readback DrawGPUTexture. Skip when a layer already owns a GPU view
	// (P0-1) so we do not steal the layer destination.
	if (pending > 0 || pendingAdvanced) && c.effectSurface && t.View.IsNil() {
		if view, w, h, rel, ok := c.acquireEffectPublishView(); ok {
			t.View = view
			t.ViewWidth = uint32(w)  //nolint:gosec // pixmap dims bounded
			t.ViewHeight = uint32(h) //nolint:gosec
			// CPU-painted base (ClearWithColor / fallback draws) is part of the
			// effect surface: seed the publish RT before replaying pending draws
			// instead of clearing the base away.
			t.SeedFromData = c.pixmapBaseContent
			if rc != nil {
				err = gpuFlushOffscreenBaseTarget(rc, t)
			} else if a := Accelerator(); a != nil {
				c.warnGPUFallback("FlushGPU")
				err = a.Flush(t)
			}
			if err == nil {
				c.attachFilterGPUResult(view, w, h, rel)
				c.pixmapFilterStale = true
				c.clearViewFlushTracking()
				// Content lives on GPU, not pixmap (unlike nil-view Map path).
				c.midFrameNilFlush = false
				c.drainLayerGPUReleases()
				return nil
			}
			// Fall through to CPU-readback Flush on failure.
			rel()
			t.View = gpucontext.TextureView{}
			t.ViewWidth, t.ViewHeight = 0, 0
			offscreen = !t.View.IsNil()
		}
	}

	if rc := c.gpuCtxOps(); rc != nil {
		if offscreen {
			err = gpuFlushOffscreenTarget(rc, t)
		} else {
			err = rc.Flush(t)
		}
	} else if a := Accelerator(); a != nil {
		c.warnGPUFallback("FlushGPU")
		err = a.Flush(t)
	}
	if err == nil {
		c.applyDitherIfEnabled()
		// Nil-view flush with work readbacks into pixmap Data — pixmap is
		// authoritative for that content. Empty flushes must NOT drop a prior
		// view-present coherence flag (Image after PresentFrame*).
		if pending > 0 {
			c.clearViewFlushTracking()
			c.midFrameNilFlush = true
			if t.View.IsNil() {
				// GPU pixels were composited into the pixmap: it now holds base
				// content that a zero-readback consumer must seed from.
				c.pixmapBaseContent = true
			}
			// CPU path overwrote published GPU content.
			if c.effectSurface {
				c.pixmapFilterStale = false
			}
		}
	}
	// Layer GPU RTs used as DrawGPUTexture sources can be released now.
	c.drainLayerGPUReleases()
	return err
}

// flushGPUWithViewCore is the shared body for FlushGPUWithView* variants.
// requireAccel mirrors historical FlushGPUWithView behavior (ErrFallbackToCPU
// when neither per-context ops nor a global accelerator is available); damage
// variants leave err nil in that case.
// flushGPUWithViewCore is the shared body for FlushGPUWithView* variants.
// requireAccel mirrors historical FlushGPUWithView behavior (ErrFallbackToCPU
// when neither per-context ops nor a global accelerator is available); damage
// variants leave err nil in that case.
func (c *Context) flushGPUWithViewCore(view gpucontext.TextureView, width, height uint32, damage []image.Rectangle, opName string, requireAccel bool) error {
	return c.flushGPUWithViewCoreImpl(view, width, height, damage, opName, requireAccel, false)
}

// flushLayerViewGPU flushes pending ops into a layer isolation RT (offscreen).
// PopLayer self-flushes its RT after the layer already left the stack, so the
// target no longer matches gpuRenderTarget's top layer; the offscreen intent
// is explicit here so the compositor clears the RT transparent (saveLayer
// base) instead of the surface's opaque black.
func (c *Context) flushLayerViewGPU(view gpucontext.TextureView, width, height uint32, damage []image.Rectangle) error {
	tFlush := time.Now()
	err := c.flushGPUWithViewCoreImpl(view, width, height, damage, "FlushLayerViewGPU", false, true)
	noteFlushMs(tFlush)
	return err
}

func (c *Context) flushGPUWithViewCoreImpl(view gpucontext.TextureView, width, height uint32, damage []image.Rectangle, opName string, requireAccel, forceOffscreen bool) error {
	c.recordFrameFlush()
	t := c.gpuRenderTarget()
	// Flushing into the currently active isolated layer's own RT is an
	// offscreen flush (layer self-flush / effect RT), not a surface present:
	// the compositor must clear it transparent (see gpuFlushOffscreenTarget).
	offscreen := forceOffscreen || (!view.IsNil() && view == t.View)
	if !view.IsNil() {
		t.View = view
		t.ViewWidth = width
		t.ViewHeight = height
	}
	if len(damage) > 0 {
		t.DamageRects = damage
	}
	var err error
	if rc := c.gpuCtxOps(); rc != nil {
		if offscreen {
			err = gpuFlushOffscreenTarget(rc, t)
		} else {
			err = rc.Flush(t)
		}
	} else if a := Accelerator(); a != nil {
		c.warnGPUFallback(opName)
		err = a.Flush(t)
	} else if requireAccel {
		err = ErrFallbackToCPU
	}
	if err == nil {
		c.drainLayerGPUReleases()
	}
	return err
}

// FlushGPUWithView flushes pending GPU operations, resolving directly to the
// given texture view instead of reading back to CPU. The view is passed
// through GPURenderTarget.View so the render session uses it as the per-pass
// resolve target, enabling multiple Contexts to render to different views
// without cross-contamination.
//
// This is the per-pass render target path for ggcanvas.RenderDirect.
// When view is nil/zero, behaves identically to FlushGPU (CPU readback).
// flushWithTiming走一遍上屏干活并单记耗时：三个Flush入口共用，不含 present 等。
// FlushGPUWithView flushes pending GPU operations, resolving directly to the
// given texture view instead of reading back to CPU. The view is passed
// through GPURenderTarget.View so the render session uses it as the per-pass
// resolve target, enabling multiple Contexts to render to different views
// without cross-contamination.
//
// This is the per-pass render target path for ggcanvas.RenderDirect.
// When view is nil/zero, behaves identically to FlushGPU (CPU readback).
// flushWithTiming走一遍上屏干活并单记耗时：三个Flush入口共用，不含 present 等。
func (c *Context) flushWithTiming(view gpucontext.TextureView, width, height uint32, damage []image.Rectangle, op string, full bool) error {
	tFlush := time.Now()
	err := c.flushGPUWithViewCore(view, width, height, damage, op, full)
	noteFlushMs(tFlush)
	return err
}

func (c *Context) FlushGPUWithView(view gpucontext.TextureView, width, height uint32) error {
	return c.flushWithTiming(view, width, height, nil, "FlushGPUWithView", true)
}

// BeginOffscreenPass isolates an offscreen recording sub-pass from the main
// frame stream: while active, all queued commands / clip timeline / LoadOp
// tracking belong exclusively to the sub-pass target; the main pass state is
// suspended and restored by the returned func. Callers MUST invoke the
// restore func (defer) after flushing the sub-pass into its own view.
//
// This is what makes retained texture re-records safe mid-frame (Skia
// GrRecordingContext / Flutter EntityPass pass-ownership semantics). With no
// GPU ops interface it returns a no-op func.
// FlushGPUWithViewDamage flushes pending GPU operations with damage-aware
// optimization. When damageRect is non-empty, the compositor uses LoadOpLoad
// (preserves previous frame) and scissor-clips to the dirty region — only
// the damaged pixels are re-composited. When damageRect is empty, behaves
// identically to FlushGPUWithView (full compositor pass).
//
// IMPORTANT: damage-aware rendering (LoadOpLoad + scissor) works only on the
// blit-only compositor path (frames with only DrawGPUTextureBase/DrawGPUTexture
// calls, no vector shapes). When the frame contains Fill/Stroke operations,
// the MSAA render path is used which always does LoadOpClear — damageRect is
// ignored and a warning is logged. This matches enterprise practice: Chrome,
//
// This enables sub-region compositing: a 48×48 spinner updates only 9KB
// instead of the full surface (8MB at 1080p). See ADR-016 Phase 2.
func (c *Context) FlushGPUWithViewDamage(view gpucontext.TextureView, width, height uint32, damageRect image.Rectangle) error {
	var damage []image.Rectangle
	if !damageRect.Empty() {
		damage = []image.Rectangle{damageRect}
	}
	return c.flushWithTiming(view, width, height, damage, "FlushGPUWithViewDamage", false)
}

// FlushGPUWithViewDamageRects renders to a surface view with multiple damage rects
// (ADR-028). Each overlay gets its own scissor from the closest damage rect,
// enabling per-draw dynamic scissor for distant dirty regions.
// GPURenderContext returns the per-context GPU render context, lazily created.
// Returns nil if no GPU accelerator is registered or it does not support
// per-context rendering. The returned value should be type-asserted to
// *gpu.GPURenderContext in internal/gpu consumers.
func (c *Context) GPURenderContext() any {
	c.ensureGPUCtx()
	return c.gpuCtx
}

// ensureGPUCtx lazily creates the per-context GPU render context.
// ensureGPUCtx lazily creates the per-context GPU render context.
func (c *Context) ensureGPUCtx() {
	if c.baseGpuCtx != nil {
		if c.gpuCtx == nil {
			c.gpuCtx = c.baseGpuCtx
		}
		return
	}
	if c.gpuCtx != nil {
		// Defensive: active pointer survived a partial teardown, but the base
		// owner is gone. Rebuild the base instead of treating a layer context as
		// the window target.
		c.gpuCtx = nil
	}
	// ADR-020: GOGPU_RENDER_MODE=cpu forces every draw through the CPU
	// rasterizer. Without this gate, offscreen layers, shaped glyphs and
	// image/filter paths still create a GPU context and defer pixels until
	// Image/Flush, so CPU contracts see an untouched pixmap.
	if CPUOnlyMode() {
		return
	}
	a := Accelerator()
	if a == nil {
		return
	}
	p, ok := a.(GPURenderContextProvider)
	if !ok {
		return
	}
	rc := p.NewGPURenderContext()
	if rc == nil {
		return
	}
	ops, ok := rc.(gpuContextOps)
	if !ok {
		return
	}
	c.baseGpuCtx = ops
	c.gpuCtx = ops
	registerGPUContext(c)
}

// ensureLayerGPUCtx lazily creates the offscreen-layer GPU context at 1x.
//
// Offscreen layer RTs are 1x textures (see CreateOffscreenTexture). Drawing
// them through the base 4x session would rasterize into a 4x MSAA attachment and
// resolve over the 1x texture, which drops any CPU-seeded content (backdrop /
// filter seeds) on the first post-filter draw. A dedicated 1x session keeps
// the seed and the following tint/child draws on the same 1x target. It also
// means layer Push/Pop never destroys the base session or changes its sample
// count.
func (c *Context) ensureLayerGPUCtx() {
	if c.layerGpuCtx != nil {
		return
	}
	c.ensureGPUCtx()
	if c.baseGpuCtx == nil {
		return
	}
	a := Accelerator()
	if a == nil {
		return
	}
	p, ok := a.(GPURenderContextProvider)
	if !ok {
		return
	}
	rc := p.NewGPURenderContext()
	if rc == nil {
		return
	}
	ops, ok := rc.(gpuContextOps)
	if !ok {
		return
	}
	ops.SetPreferSampleCount1(true)
	c.layerGpuCtx = ops
}

// gpuCtxOps returns the active per-context GPU ops interface (base or layer),
// or nil if unavailable.
func (c *Context) gpuCtxOps() gpuContextOps {
	c.ensureGPUCtx()
	if c.gpuCtx == nil {
		c.gpuCtx = c.baseGpuCtx
	}
	return c.gpuCtx
}

// gpuRenderTarget returns the current context's pixel buffer as a GPU render target.
// When the top PushLayer has a GPU offscreen RT (P0-1), View is set so queued
// fills/strokes resolve into the layer texture instead of the CPU pixmap.
// gpuRenderTarget returns the current context's pixel buffer as a GPU render target.
// When the top PushLayer has a GPU offscreen RT (P0-1), View is set so queued
// fills/strokes resolve into the layer texture instead of the CPU pixmap.
func (c *Context) gpuRenderTarget() GPURenderTarget {
	t := GPURenderTarget{
		Data:   c.pixmap.Data(),
		Width:  c.pixmap.Width(),
		Height: c.pixmap.Height(),
		Stride: c.pixmap.Width() * 4,
	}
	if c.layerStack != nil && len(c.layerStack.layers) > 0 {
		top := c.layerStack.layers[len(c.layerStack.layers)-1]
		if top != nil && !top.gpuView.IsNil() {
			t.View = top.gpuView
			t.ViewWidth = uint32(top.gpuW)  //nolint:gosec // layer dims bounded
			t.ViewHeight = uint32(top.gpuH) //nolint:gosec // layer dims bounded
		}
	}
	return t
}

// warnGPUFallback logs a one-time warning when a GPU operation falls back to
// the global accelerator instead of the per-context GPURenderContext. This
// indicates shape leaking risk in multi-context scenarios (RepaintBoundary).
// warnGPUFallback logs a one-time warning when a GPU operation falls back to
// the global accelerator instead of the per-context GPURenderContext. This
// indicates shape leaking risk in multi-context scenarios (RepaintBoundary).
func (c *Context) warnGPUFallback(op string) {
	if !c.gpuFallbackWarned {
		c.gpuFallbackWarned = true
		Logger().Warn("GPU operation using global accelerator instead of per-context — shapes may leak in multi-context",
			"op", op, "w", c.width, "h", c.height)
	}
}

// flushGPUAccelerator flushes pending GPU shapes before a CPU fallback operation.
// flushGPUAccelerator flushes pending GPU shapes before a CPU fallback operation.
func (c *Context) flushGPUAccelerator() {
	// Callers flush before rasterizing on the CPU (FillRectCPU, bitmap text,
	// vertices/mesh fallbacks). The pixmap becomes base content the pending GPU
	// queue no longer describes.
	c.pixmapBaseContent = true
	if rc := c.gpuCtxOps(); rc != nil {
		t := c.gpuRenderTarget()
		if !t.View.IsNil() {
			// Top isolated layer RT target: offscreen flush, transparent base.
			_ = gpuFlushOffscreenTarget(rc, t)
			return
		}
		_ = rc.Flush(t)
		return
	}
	if a := Accelerator(); a != nil {
		c.warnGPUFallback("flushGPUAccelerator")
		_ = a.Flush(c.gpuRenderTarget())
	}
}

// tryGPUFill attempts to fill the current path using the GPU accelerator.
// When a mask is active and the accelerator implements MaskAware, the mask
// is uploaded as a GPU texture. Otherwise, falls back to CPU.
// tryGPUFill attempts to fill the current path using the GPU accelerator.
// When a mask is active and the accelerator implements MaskAware, the mask
// is uploaded as a GPU texture. Otherwise, falls back to CPU.
func (c *Context) tryGPUFill() error {
	cleanup, err := c.setupGPUMask()
	if err != nil {
		return err
	}
	defer cleanup()
	if rc := c.gpuCtxOps(); rc != nil {
		return c.tryGPUOpRC(rc.FillShape, rc.FillPath, c.totalMatrix())
	}
	a := Accelerator()
	if a == nil {
		return ErrFallbackToCPU
	}
	c.warnGPUFallback("tryGPUFill")
	return c.tryGPUOp(a, a.FillShape, a.FillPath, AccelFill)
}

// tryGPUStroke attempts to stroke the current path using the GPU accelerator.
// When a mask is active and the accelerator implements MaskAware, the mask
// is uploaded as a GPU texture. Otherwise, falls back to CPU.
// tryGPUStroke attempts to stroke the current path using the GPU accelerator.
// When a mask is active and the accelerator implements MaskAware, the mask
// is uploaded as a GPU texture. Otherwise, falls back to CPU.
func (c *Context) tryGPUStroke() error {
	cleanup, err := c.setupGPUMask()
	if err != nil {
		return err
	}
	defer cleanup()
	if rc := c.gpuCtxOps(); rc != nil {
		return c.tryGPUOpRC(rc.StrokeShape, rc.StrokePath, c.totalMatrix())
	}
	a := Accelerator()
	if a == nil {
		return ErrFallbackToCPU
	}
	c.warnGPUFallback("tryGPUStroke")
	return c.tryGPUOp(a, a.StrokeShape, a.StrokePath, AccelStroke)
}

// setupGPUMask uploads the active alpha mask to the GPU accelerator.
// The mask remains bound until ClearMask/SetMask(nil) or a replacement
// SetMaskTexture — it must survive until FlushGPU so cover-inline R8 sampling
// (L.06) can bind the texture during RecordDraws.
//
// Returns a no-op cleanup for call-site defer compatibility.
// tryGPUOpRC routes GPU operations through the per-context GPURenderContext.
func (c *Context) tryGPUOpRC(
	shapeFn func(GPURenderTarget, DetectedShape, *Paint) error,
	pathFn func(GPURenderTarget, *Path, *Paint, Matrix) error,
	totalM Matrix,
) error {
	target := c.gpuRenderTarget()

	shape := DetectShape(c.path)
	if accel := sdfAccelForShape(shape.Kind); accel != 0 {
		if err := shapeFn(target, shape, c.paint); err == nil {
			return nil
		}
	}

	return pathFn(target, c.path, c.paint, totalM)
}

// tryGPUOp attempts GPU rendering using shape-specific SDF first, then general path.
//
// When PipelineModeCompute is active and the accelerator supports compute,
// all operations are routed directly to the path function (which accumulates
// for the compute pipeline). Shape detection is skipped because the compute
// pipeline handles all shapes uniformly.
//
// When PipelineModeRenderPass is active (or Auto selects RenderPass), the
// existing tier-based approach is used: shape SDF first, then general path.
// tryGPUOp attempts GPU rendering using shape-specific SDF first, then general path.
//
// When PipelineModeCompute is active and the accelerator supports compute,
// all operations are routed directly to the path function (which accumulates
// for the compute pipeline). Shape detection is skipped because the compute
// pipeline handles all shapes uniformly.
//
// When PipelineModeRenderPass is active (or Auto selects RenderPass), the
// existing tier-based approach is used: shape SDF first, then general path.
func (c *Context) tryGPUOp(
	a GPUAccelerator,
	shapeFn func(GPURenderTarget, DetectedShape, *Paint) error,
	pathFn func(GPURenderTarget, *Path, *Paint) error,
	pathAccel AcceleratedOp,
) error {
	target := c.gpuRenderTarget()

	// When explicitly in Compute mode, skip shape detection and route
	// all operations directly to the path function. The accelerator's
	// FillPath/StrokePath accumulates into the compute scene.
	if c.pipelineMode == PipelineModeCompute {
		if cpa, ok := a.(ComputePipelineAware); ok && cpa.CanCompute() {
			if a.CanAccelerate(pathAccel) {
				return pathFn(target, c.path, c.paint)
			}
		}
		// Compute requested but not available — fall through to render pass.
	}

	// Try shape-specific SDF first for higher quality output.
	shape := DetectShape(c.path)
	if accel := sdfAccelForShape(shape.Kind); accel != 0 && a.CanAccelerate(accel) {
		if err := shapeFn(target, shape, c.paint); err == nil {
			return nil
		}
	}

	// Try general GPU path operation.
	if a.CanAccelerate(pathAccel) {
		return pathFn(target, c.path, c.paint)
	}

	return ErrFallbackToCPU
}

// sdfAccelForShape maps a shape kind to its SDF acceleration capability.
// tryGPUFillWithMode attempts GPU fill based on the rasterizer mode.
// Returns (true, _) if GPU handled the fill, or (false, cpuMode) with the
// fallback CPU mode to use.
func (c *Context) tryGPUFillWithMode(mode RasterizerMode) (bool, RasterizerMode) {
	if mode == RasterizerSDF {
		c.setForceSDF(true)
		err := c.tryGPUFill()
		c.setForceSDF(false)
		if err == nil {
			c.recordGPUOp()
			c.takeBrushBootstrapIfAny()
			return true, mode
		}
		mode = RasterizerAuto // Non-SDF shape → auto CPU fallback.
	}
	if mode == RasterizerAuto {
		if err := c.tryGPUFill(); err == nil {
			c.recordGPUOp()
			c.takeBrushBootstrapIfAny()
			return true, mode
		} else if c.gpuPathAvailable() {
			reason := "fill"
			if err != nil {
				reason = "fill:" + err.Error()
			}
			c.recordCPUFallbackReason(reason)
		}
	}
	return false, mode
}

// tryGPUStrokeWithMode attempts GPU stroke based on the rasterizer mode.
// Returns (true, _) if GPU handled the stroke, or (false, cpuMode) with the
// fallback CPU mode to use.
// tryGPUStrokeWithMode attempts GPU stroke based on the rasterizer mode.
// Returns (true, _) if GPU handled the stroke, or (false, cpuMode) with the
// fallback CPU mode to use.
func (c *Context) tryGPUStrokeWithMode(mode RasterizerMode) (bool, RasterizerMode) {
	if mode == RasterizerSDF {
		c.setForceSDF(true)
		err := c.tryGPUStroke()
		c.setForceSDF(false)
		if err == nil {
			c.recordGPUOp()
			c.takeBrushBootstrapIfAny()
			return true, mode
		}
		mode = RasterizerAuto
	}
	if mode == RasterizerAuto {
		if err := c.tryGPUStroke(); err == nil {
			c.recordGPUOp()
			c.takeBrushBootstrapIfAny()
			return true, mode
		} else if c.gpuPathAvailable() {
			reason := "stroke"
			if err != nil {
				reason = "stroke:" + err.Error()
			}
			c.recordCPUFallbackReason(reason)
		}
	}
	return false, mode
}

// setForceSDF enables/disables forced SDF on the registered accelerator.
