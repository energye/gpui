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
	"math"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// FrameDamage returns the list of damage rectangles from draw operations
// this frame. Each rect corresponds to one or more Fill/Stroke operations.
// Used by ggcanvas → SetDamageRects → PresentWithDamage for per-rect OS blit.
// Returns nil if no drawing operations occurred.
func (c *Context) FrameDamage() []image.Rectangle {
	if len(c.frameDamageRects) == 0 {
		return nil
	}
	return c.frameDamageRects
}

// FrameDamageUnion returns the bounding box of all damage rects this frame.
// Convenience method for debug display or single-rect consumers.
// FrameDamageUnion returns the bounding box of all damage rects this frame.
// Convenience method for debug display or single-rect consumers.
func (c *Context) FrameDamageUnion() image.Rectangle {
	var r image.Rectangle
	for _, dr := range c.frameDamageRects {
		r = r.Union(dr)
	}
	return r
}

// ResetFrameDamage clears the per-frame damage accumulator.
// Call at the start of each frame before drawing operations.
// ResetFrameDamage clears the per-frame damage accumulator.
// Call at the start of each frame before drawing operations.
func (c *Context) ResetFrameDamage() {
	c.frameDamageRects = c.frameDamageRects[:0]
}

// SetDamageTracking enables or disables per-operation damage recording.
// When disabled, Fill/Stroke do not append to FrameDamage.
// Used by retained-mode compositors to suppress damage during replay
// of cached (clean) scene content (ADR-021 false positive fix).
// SetDamageTracking enables or disables per-operation damage recording.
// When disabled, Fill/Stroke do not append to FrameDamage.
// Used by retained-mode compositors to suppress damage during replay
// of cached (clean) scene content (ADR-021 false positive fix).
func (c *Context) SetDamageTracking(enabled bool) {
	c.damageTrackingEnabled = enabled
}

// TrackDamageRect registers an external damage rectangle on the surface.
// Use this for compositor operations that modify the surface but don't use
// Fill/Stroke (e.g., DrawGPUTexture for dirty RepaintBoundary overlays).
// No-op when damage tracking is disabled or rect is empty.
//
// Callers with retained-mode knowledge (e.g., ui widget tree) should call
// this for each dirty boundary after compositing, so that FrameDamage()
// accurately reflects which surface regions changed this frame.
//
// Bounds are in logical (user-space) coordinates. The context automatically
// scales them to physical pixels via deviceScale for the OS compositor.
// TrackDamageRect registers an external damage rectangle on the surface.
// Use this for compositor operations that modify the surface but don't use
// Fill/Stroke (e.g., DrawGPUTexture for dirty RepaintBoundary overlays).
// No-op when damage tracking is disabled or rect is empty.
//
// Callers with retained-mode knowledge (e.g., ui widget tree) should call
// this for each dirty boundary after compositing, so that FrameDamage()
// accurately reflects which surface regions changed this frame.
//
// Bounds are in logical (user-space) coordinates. The context automatically
// scales them to physical pixels via deviceScale for the OS compositor.
func (c *Context) TrackDamageRect(bounds image.Rectangle) {
	c.trackDamage(bounds)
}

// trackDamageDevicePoints unions a device/pixmap-space AABB of points into
// frame + layer damage. Points are already in physical pixels (post-CTM);
// do not apply deviceScale again.
//
// Fast-path: skip O(n) AABB scan when no isolation layer needs Pop damage
// (base canvas / opacity-group / fullComposite). Large DrawMesh batches
// (particle kitchen-sink) would otherwise pay a full vertex scan every frame.
// trackDamageDevicePoints unions a device/pixmap-space AABB of points into
// frame + layer damage. Points are already in physical pixels (post-CTM);
// do not apply deviceScale again.
//
// Fast-path: skip O(n) AABB scan when no isolation layer needs Pop damage
// (base canvas / opacity-group / fullComposite). Large DrawMesh batches
// (particle kitchen-sink) would otherwise pay a full vertex scan every frame.
func (c *Context) trackDamageDevicePoints(pts []Point) {
	if c == nil || !c.damageTrackingEnabled || len(pts) == 0 {
		return
	}
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}
	top := c.layerStack.layers[len(c.layerStack.layers)-1]
	if top == nil || top.opacityGroup || top.fullComposite {
		return
	}
	minX, minY := pts[0].X, pts[0].Y
	maxX, maxY := minX, minY
	for i := 1; i < len(pts); i++ {
		p := pts[i]
		if p.X < minX {
			minX = p.X
		} else if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		} else if p.Y > maxY {
			maxY = p.Y
		}
	}
	// AA pad
	bounds := image.Rect(
		int(math.Floor(minX))-1,
		int(math.Floor(minY))-1,
		int(math.Ceil(maxX))+1,
		int(math.Ceil(maxY))+1,
	)
	if bounds.Empty() {
		return
	}
	c.frameDamageRects = append(c.frameDamageRects, bounds)
	// touch-merge first (CoalesceDamageRects); only full-union if still over cap.
	if len(c.frameDamageRects) > maxDamageRects {
		c.frameDamageRects = CoalesceDamageRects(c.frameDamageRects, maxDamageRects)
	}
	c.noteLayerDamage(bounds)
}

// trackDamage adds a damage rectangle for the current draw operation.
// No-op when damage tracking is disabled (cached scene replay).
// If rect count exceeds maxDamageRects, coalesces via CoalesceDamageRects
// (touch/overlap merge, then full-union only if still over the cap).
// trackDamage adds a damage rectangle for the current draw operation.
// No-op when damage tracking is disabled (cached scene replay).
// If rect count exceeds maxDamageRects, coalesces via CoalesceDamageRects
// (touch/overlap merge, then full-union only if still over the cap).
func (c *Context) trackDamage(bounds image.Rectangle) {
	if !c.damageTrackingEnabled || bounds.Empty() {
		return
	}

	// Scale logical damage rect to physical pixels for OS compositor APIs
	// (Vulkan VK_KHR_incremental_present, DX12 Present1, EGL, Wayland damage_buffer).
	// Floor/Ceil ensures conservative rounding with no pixel gaps.
	if !c.deviceMatrix.IsIdentity() {
		s := math.Float64frombits(c.deviceScale.Load())
		bounds = image.Rect(
			int(math.Floor(float64(bounds.Min.X)*s)),
			int(math.Floor(float64(bounds.Min.Y)*s)),
			int(math.Ceil(float64(bounds.Max.X)*s)),
			int(math.Ceil(float64(bounds.Max.Y)*s)),
		)
	}

	c.frameDamageRects = append(c.frameDamageRects, bounds)
	// prefer pairwise touch-merge over immediate full AABB collapse.
	if len(c.frameDamageRects) > maxDamageRects {
		c.frameDamageRects = CoalesceDamageRects(c.frameDamageRects, maxDamageRects)
	}

	// Layer Pop composites only this union.
	c.noteLayerDamage(bounds)
}

// FillRectCPU fills a rectangle directly on the CPU pixmap without engaging
// the GPU SDF accelerator. Coordinates are in user space (device scale applied
// automatically). Pending GPU shapes are flushed first for correct z-ordering.
//
// Use for operations where GPU acceleration is counterproductive, such as
// dirty-region background clearing in retained-mode compositors. Without this,
// DrawRectangle+Fill routes through SDF accelerator → blocks non-MSAA blit path.
//
// See ADR-016, TASK-GG-COMPOSITOR-003.
// FlushGPUWithViewDamageRects renders to a surface view with multiple damage rects
// (ADR-028). Each overlay gets its own scissor from the closest damage rect,
// enabling per-draw dynamic scissor for distant dirty regions.
func (c *Context) FlushGPUWithViewDamageRects(view gpucontext.TextureView, width, height uint32, rects []image.Rectangle) error {
	var damage []image.Rectangle
	if len(rects) > 0 {
		damage = rects
	}
	return c.flushWithTiming(view, width, height, damage, "FlushGPUWithViewDamageRects", false)
}

// gpuContextOps is the per-context GPU rendering interface.
// GPURenderContext (internal/gpu) implements this, allowing context.go
// to route draw calls through the per-context queue without importing internal/gpu.
type gpuContextOps interface {
	FillShape(target GPURenderTarget, shape DetectedShape, paint *Paint) error
	StrokeShape(target GPURenderTarget, shape DetectedShape, paint *Paint) error
	// F2: FillPath/StrokePath take the total user→device matrix; path stays
	// device-space (baked). The GPU backend unbakes to user space for
	// transform-independent caching when the matrix is a similarity.
	FillPath(target GPURenderTarget, path *Path, paint *Paint, matrix Matrix) error
	StrokePath(target GPURenderTarget, path *Path, paint *Paint, matrix Matrix) error
	DrawText(target GPURenderTarget, face any, s string, x, y float64, color RGBA, matrix Matrix, deviceScale float64) error
	DrawGlyphMaskText(target GPURenderTarget, face any, s string, x, y float64, color RGBA, matrix Matrix, deviceScale float64) error
	DrawGlyphMaskTextAliased(target GPURenderTarget, face any, s string, x, y float64, color RGBA, matrix Matrix, deviceScale float64) error
	QueueImageDraw(target GPURenderTarget, pixelData []byte, genID uint64, imgWidth, imgHeight, imgStride int,
		tlX, tlY, trX, trY, brX, brY, blX, blY, opacity float32, viewportW, viewportH uint32,
		u0, v0, u1, v1 float32, nearest bool, contentDirty bool, bicubic ...bool)
	// QueueImageDrawTint is QueueImageDraw plus per-quad straight tint
	// . Implemented by
	// internal/gpu; callers that cannot assert it must keep CPU fallback.
	QueueImageDrawTint(target GPURenderTarget, pixelData []byte, genID uint64, imgWidth, imgHeight, imgStride int,
		tlX, tlY, trX, trY, brX, brY, blX, blY, opacity float32, viewportW, viewportH uint32,
		u0, v0, u1, v1 float32, tintR, tintG, tintB, tintA float32, nearest bool, contentDirty bool, bicubic ...bool)
	// QueueColoredMesh draws triangle list/fan with optional per-vertex colors (V.01).
	QueueColoredMesh(target GPURenderTarget, positions []Point, colors []RGBA, triangleList bool)
	// QueueColoredMeshIndexed draws unique verts + uint16 indices.
	QueueColoredMeshIndexed(target GPURenderTarget, positions []Point, colors []RGBA, indices []uint16)
	QueueGPUTextureDraw(target GPURenderTarget, view gpucontext.TextureView,
		dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32)
	// QueueGPUTextureDrawQuad is QueueGPUTextureDraw with explicit CTM quad
	// corners (TL/TR/BR/BL in device pixels). Same as QueueImageDraw corner
	// handling: axis-aligned is the special case, rotation/scale/shear ride
	// the corners. Dst rect stays the AABB for damage/scissor.
	QueueGPUTextureDrawQuad(target GPURenderTarget, view gpucontext.TextureView,
		dstX, dstY, dstW, dstH, tlX, tlY, trX, trY, brX, brY, blX, blY, opacity float32, vpW, vpH uint32)
	// QueueGPUTextureDrawQuadUV is the Quad variant with explicit source UVs.
	QueueGPUTextureDrawQuadUV(target GPURenderTarget, view gpucontext.TextureView,
		dstX, dstY, dstW, dstH, tlX, tlY, trX, trY, brX, brY, blX, blY, opacity float32, vpW, vpH uint32,
		u0, v0, u1, v1 float32)
	QueueBaseLayer(target GPURenderTarget, view gpucontext.TextureView,
		dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32)
	Flush(target GPURenderTarget) error
	// PrepareTarget switches the GPU command stream to target (F1 present-stash).
	// Must run before SetClipRect so scissor segments are not recorded on the
	// wrong stream and later stashed unpaired into the parent.
	PrepareTarget(target GPURenderTarget) error
	// BeginOffscreenPass suspends the main command stream for an isolated
	// sub-pass (retained texture record → its own offscreen view) and returns
	// a restore func. EndOffscreenPass must be called (via the restore func)
	// after the sub-pass flushed; see gpu.GPURenderContext.
	SetClipRect(x, y, w, h uint32)
	ClearClipRect()
	SetClipRRect(x, y, w, h, radius float32)
	ClearClipRRect()
	SetClipPath(path *Path)
	ClearClipPath()
	BeginFrame()
	SetPipelineMode(mode PipelineMode)
	SetAntiAlias(enabled bool)
	SetPreferSampleCount1(enabled bool)
	PendingCount() int
	// HasPendingStash reports whether the F1 present-stash holds parent draws.
	// Stashed composite quads may reference held layer textures — while active,
	// those textures must stay alive (C5 use-after-free fix).
	HasPendingStash() bool
	// TakeBrushBootstrapReason returns G.04 ColorAt→GPU blit diagnostic, if any.
	TakeBrushBootstrapReason() string
	Close()
}

// GPURenderContext returns the per-context GPU render context, lazily created.
// Returns nil if no GPU accelerator is registered or it does not support
// per-context rendering. The returned value should be type-asserted to
// *gpu.GPURenderContext in internal/gpu consumers.
