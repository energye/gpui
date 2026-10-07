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
	"math"

	"github.com/energye/gpui/render/internal/clip"
)

// Clip sets the current path as the clipping region and clears the path.
// Subsequent drawing operations will be clipped to this region.
// The clip region is intersected with any existing clip regions.
func (c *Context) Clip() {
	if c.clipStack == nil {
		c.initClipStack()
	}

	// Path elements are in user-space; clip stack operates in device-space.
	// Transform through deviceMatrix to get device coordinates.
	devicePath := c.deviceSpacePath()
	clipVerbs, clipCoords := convertPathToClipVerbs(devicePath)

	// Push the path as a clip region
	_ = c.clipStack.PushPath(clipVerbs, clipCoords, c.antiAlias) // C.05: honor SetAntiAlias

	// Store the device-space path for GPU depth clipping (GPU-CLIP-003a).
	// The GPU DepthClipPipeline fan-tessellates this path at draw time.
	c.gpuClipPath = devicePath.Clone()
	c.bumpClipMaskGPUGen()

	// Clear the path
	c.path.Clear()
}

// ClipPreserve sets the current path as the clipping region but keeps the path.
// This is like Clip() but doesn't clear the path, allowing you to both clip
// and then fill/stroke the same path.
func (c *Context) ClipPreserve() {
	if c.clipStack == nil {
		c.initClipStack()
	}

	// Path elements are in user-space; clip stack operates in device-space.
	devicePath := c.deviceSpacePath()
	clipVerbs, clipCoords := convertPathToClipVerbs(devicePath)

	// Push the path as a clip region
	_ = c.clipStack.PushPath(clipVerbs, clipCoords, c.antiAlias) // C.05: honor SetAntiAlias

	// Store the device-space path for GPU depth clipping (GPU-CLIP-003a).
	c.gpuClipPath = devicePath.Clone()
	c.bumpClipMaskGPUGen()
	// Path is preserved
}

// ClipRect sets a rectangular clipping region.
// This is a faster alternative to creating a rectangular path and calling Clip().
// The clip region is intersected with any existing clip regions.
func (c *Context) ClipRect(x, y, w, h float64) {
	// Single shared implementation.
	c.ClipRectOp(x, y, w, h, ClipOpIntersect)
}

// ClipRoundRect sets a rounded rectangle clipping region.
// The rectangle is defined by (x, y, w, h) in user coordinates and the
// corners are rounded with the given radius. The radius is clamped to
// min(w, h)/2. If radius is zero, this is equivalent to ClipRect.
//
// On GPU, this uses a two-level clip strategy:
//   - Scissor rect (hardware, free) for the bounding box
//   - Analytic SDF in the fragment shader for the rounded corners
//
// On CPU, the SDF is evaluated per-pixel during coverage computation.
func (c *Context) ClipRoundRect(x, y, w, h, radius float64) {

	if radius <= 0 {
		c.ClipRect(x, y, w, h)
		return
	}

	if c.clipStack == nil {
		c.initClipStack()
	}

	// Transform the rectangle corners to device coordinates.
	tm := c.totalMatrix()
	p1 := tm.TransformPoint(Pt(x, y))
	p2 := tm.TransformPoint(Pt(x+w, y+h))

	// Create clip rectangle in device coordinates.
	devX := math.Min(p1.X, p2.X)
	devY := math.Min(p1.Y, p2.Y)
	devW := math.Abs(p2.X - p1.X)
	devH := math.Abs(p2.Y - p1.Y)

	// Scale radius by the total transform scale factor.
	scaledRadius := radius * tm.ScaleFactor()

	// Clamp to half the smaller dimension.
	maxRadius := math.Min(devW, devH) / 2
	if scaledRadius > maxRadius {
		scaledRadius = maxRadius
	}

	rect := clip.NewRect(devX, devY, devW, devH)
	c.clipStack.PushRRect(rect, scaledRadius)
	c.bumpClipMaskGPUGen()
}

// ResetClip removes all clipping regions, restoring the full canvas as drawable.
func (c *Context) ResetClip() {

	if c.clipStack == nil {
		return
	}

	// Reset to physical pixel bounds (clip stack operates in device-space).
	bounds := clip.NewRect(0, 0, float64(c.pixmap.Width()), float64(c.pixmap.Height()))
	c.clipStack.Reset(bounds)
	c.gpuClipPath = nil
	c.bumpClipMaskGPUGen()
}

// initClipStack initializes the clip stack with canvas bounds in device-space.
func (c *Context) initClipStack() {
	bounds := clip.NewRect(0, 0, float64(c.pixmap.Width()), float64(c.pixmap.Height()))
	c.clipStack = clip.NewClipStack(bounds)
}

// convertPathToClipVerbs converts a render.Path to clip.PathVerb + coords slices.
// Both PathVerb types have identical byte values, so this is a simple cast.
func convertPathToClipVerbs(p *Path) ([]clip.PathVerb, []float64) {
	verbs := p.Verbs()
	result := make([]clip.PathVerb, len(verbs))
	for i, v := range verbs {
		result[i] = clip.PathVerb(v)
	}
	return result, p.Coords()
}

// resetGPUClipForPass suspends the context's active canvas clip for the
// duration of an offscreen sub-pass: offscreenPassDepth goes true so doFill's
// setGPUClipRect / applyClipToPaint stay inert, and the canvas clip path is
// parked aside. The clipStack itself is untouched — the caller's clip must
// survive verbatim. GPU clip state and the scissor timeline stay owned by
// Begin/EndOffscreenPass (suspend-and-restore): recording segments here would
// attribute later main-stream draws to this clip, so this func records none.
// Returns nil when no clip is active.
func (c *Context) resetGPUClipForPass() func() {
	if !c.isClipActive() {
		return nil
	}
	c.offscreenPassDepth++
	savedPath := c.gpuClipPath
	c.gpuClipPath = nil
	return func() {
		c.offscreenPassDepth--
		if c.offscreenPassDepth == 0 {
			c.gpuClipPath = savedPath
		}
	}
}

// offscreenPassSuspended reports whether an offscreen recording sub-pass is
// active and the surface clip must not leak into it.
// applyClipToPaint sets the ClipCoverage function on the paint when a clip
// stack is active and has entries. This allows the renderer to apply per-pixel
// clip masks during compositing.
func (c *Context) applyClipToPaint() {
	// Offscreen sub-pass: record the FULL picture — clip coverage applies at
	// composite time, not at record time.
	if c.offscreenPassSuspended() {
		return
	}
	if c.clipStack == nil || c.clipStack.Depth() == 0 {
		return
	}
	cs := c.clipStack
	c.paint.ClipCoverage = func(x, y float64) byte {
		return cs.Coverage(x, y)
	}
}

// applyMaskToPaint sets the MaskCoverage function on the paint when an alpha
// mask is active. This allows the renderer to apply per-pixel mask modulation
// during compositing. Mask and clip compose multiplicatively.
// isClipActive reports whether a clip region is currently active.
func (c *Context) isClipActive() bool {
	return c.clipStack != nil && c.clipStack.Depth() > 0
}

// setGPUClipRect sets the GPU scissor rect, RRect clip, or depth clip path if a
// clip region is active. Returns a cleanup function that must be deferred to
// clear the clip state. Handles three cases:
//
//  1. Rect-only clip → hardware scissor rect (free, zero per-pixel cost)
//  2. RRect clip → scissor rect (bounding box) + SDF in fragment shader
//  3. Path clip → scissor rect (bounding box) + depth buffer (GPU-CLIP-003a)
//
// If no clip is active or the accelerator doesn't support ClipAware, the
// returned function is a no-op.
// setGPUClipRect sets the GPU scissor rect, RRect clip, or depth clip path if a
// clip region is active. Returns a cleanup function that must be deferred to
// clear the clip state. Handles three cases:
//
//  1. Rect-only clip → hardware scissor rect (free, zero per-pixel cost)
//  2. RRect clip → scissor rect (bounding box) + SDF in fragment shader
//  3. Path clip → scissor rect (bounding box) + depth buffer (GPU-CLIP-003a)
//
// If no clip is active or the accelerator doesn't support ClipAware, the
// returned function is a no-op.
func (c *Context) setGPUClipRect() func() {
	// Offscreen sub-pass: the surface clip must not leak into the pass stream
	// (its scissor coords are meaningless in the small offscreen viewport).
	if c.offscreenPassSuspended() {
		return func() {}
	}
	if !c.isClipActive() {
		return func() {}
	}
	// F1: scissor segments belong to the active GPU target stream. doFill /
	// DrawString historically called setGPUClipRect() before Queue*→prepareTarget.
	// When the first draw inside a PushLayer ran under an active ClipRect, SetClip
	// was recorded on the parent stream and then present-stashed without Clear —
	// later unclipped HUD/FPS text inherited the layer clip and vanished.
	if rc := c.gpuCtxOps(); rc != nil {
		_ = rc.PrepareTarget(c.gpuRenderTarget())
	}
	rectOnly := c.clipStack.IsRectOnly()
	rrectOnly := c.clipStack.IsRRectOnly()

	// Arbitrary path clip → GPU depth clipping when a device path is stored.
	// Mask/difference clips (HasMaskClip, no gpuClipPath) fall through to
	// bounds scissor; fine coverage is applied via GPU R8 mask.
	if !rectOnly && !rrectOnly {
		if c.gpuClipPath != nil {
			return c.setGPUClipPath()
		}
		// Fall through: coarse scissor only.
	}

	bounds := c.clipStack.Bounds()
	x0 := uint32(math.Floor(bounds.X))
	y0 := uint32(math.Floor(bounds.Y))
	x1 := uint32(math.Ceil(bounds.X + bounds.W))
	y1 := uint32(math.Ceil(bounds.Y + bounds.H))
	if x1 <= x0 || y1 <= y0 {
		// Skia isClipEmpty: an empty clip covers no pixels, so following
		// draws cannot be visible. Record an explicit empty segment so the
		// draws join an empty scissor group that dispatch skips — without
		// it they would join the unclipped group and paint with a stale or
		// full scissor on full-surface presents.
		if rc := c.gpuCtxOps(); rc != nil {
			rc.SetClipRect(x0, y0, 0, 0)
			return func() { rc.ClearClipRect() }
		}
		return func() {}
	}

	// Per-context path (GPURenderContext available)
	if rc := c.gpuCtxOps(); rc != nil {
		rc.SetClipRect(x0, y0, x1-x0, y1-y0)
		if !rectOnly {
			rrBounds, radius, hasRRect := c.clipStack.RRectBounds()
			if hasRRect {
				rc.SetClipRRect(
					float32(rrBounds.X), float32(rrBounds.Y),
					float32(rrBounds.W), float32(rrBounds.H),
					float32(radius),
				)
				return func() {
					rc.ClearClipRect()
					rc.ClearClipRRect()
				}
			}
		}
		return func() { rc.ClearClipRect() }
	}

	// Fallback: global accelerator (backward compat for mock accelerators)
	a := Accelerator()
	if a == nil {
		return func() {}
	}
	ca, ok := a.(ClipAware)
	if !ok {
		return func() {}
	}
	ca.SetClipRect(x0, y0, x1-x0, y1-y0)
	if !rectOnly {
		if rca, ok2 := a.(RRectClipAware); ok2 {
			rrBounds, radius, hasRRect := c.clipStack.RRectBounds()
			if hasRRect {
				rca.SetClipRRect(
					float32(rrBounds.X), float32(rrBounds.Y),
					float32(rrBounds.W), float32(rrBounds.H),
					float32(radius),
				)
				return func() {
					ca.ClearClipRect()
					rca.ClearClipRRect()
				}
			}
		}
	}
	return func() { ca.ClearClipRect() }
}

// setGPUClipPath handles arbitrary path clips by sending the device-space clip
// path to the GPU depth clip pipeline (GPU-CLIP-003a). Returns a cleanup
// function that clears the depth clip state after drawing completes.
// If no GPU context is available or no clip path is stored, returns a no-op.
// setGPUClipPath handles arbitrary path clips by sending the device-space clip
// path to the GPU depth clip pipeline (GPU-CLIP-003a). Returns a cleanup
// function that clears the depth clip state after drawing completes.
// If no GPU context is available or no clip path is stored, returns a no-op.
func (c *Context) setGPUClipPath() func() {
	if c.gpuClipPath == nil {
		return func() {}
	}
	rc := c.gpuCtxOps()
	if rc == nil {
		return func() {}
	}
	// Set scissor rect to clip bounding box (coarse clip, free).
	bounds := c.clipStack.Bounds()
	x0 := uint32(math.Floor(bounds.X))
	y0 := uint32(math.Floor(bounds.Y))
	x1 := uint32(math.Ceil(bounds.X + bounds.W))
	y1 := uint32(math.Ceil(bounds.Y + bounds.H))
	if x1 > x0 && y1 > y0 {
		rc.SetClipRect(x0, y0, x1-x0, y1-y0)
	}
	// Set depth clip path for fine per-pixel clipping.
	rc.SetClipPath(c.gpuClipPath)
	return func() {
		rc.ClearClipPath()
		rc.ClearClipRect()
	}
}

// tryGPUFillWithMode attempts GPU fill based on the rasterizer mode.
// Returns (true, _) if GPU handled the fill, or (false, cpuMode) with the
// fallback CPU mode to use.
