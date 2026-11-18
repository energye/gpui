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
	"fmt"
	"os"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// surfacePersistentRegistrationEnabled gates P6 P1. Default ON for backends
// whose surface keeps its back buffer (GL FBO); wgpu reports the capability
// false and stays on the full-composite path. GPUI_SURFACE_PERSISTENT=0 is the
// kill switch for a regression window.
//
// The registration only carries COLOR across frames. Stencil is transient:
// loading a cross-frame stencil feeds stale winding into stencil-then-cover and
// fills vector paths to their bounding box (solid icon blocks), so the surface
// pass clears stencil on the first pass of every frame (see
// GPURenderSession.isSurfaceCarry).
func surfacePersistentRegistrationEnabled() bool {
	return os.Getenv("GPUI_SURFACE_PERSISTENT") != "0"
}

// setSurfacePersistent records whether the present surface keeps its back
// buffer across frames (hal.SurfacePersistentBackBuffer). Set once when the
// PresentTarget is built; wgpu-native reports false and keeps the old
// full-composite behavior.
func (c *Context) setSurfacePersistent(persistent bool) {
	if c == nil {
		return
	}
	c.surfacePersistent = persistent
}

// setSurfaceContentValid records whether the surface back buffer currently
// holds a complete frame. PresentTarget clears it for forced/resized/recovered
// frames and sets it after a successful present; markSurfaceContentForPresent
// only registers the frame view as content-bearing when this is true.
func (c *Context) setSurfaceContentValid(valid bool) {
	if c == nil {
		return
	}
	c.surfaceContentValid = valid
}

// compositeSurfaceEnabled reports whether the P6 P3 composite-surface flow is
// active for this context: a NON-persistent surface (wgpu-native) with the
// per-context GPU ops able to own a composite surface and the kill switch
// unset (GPUI_COMPOSITE_SURFACE=0 restores the full-composite path).
func (c *Context) compositeSurfaceEnabled() bool {
	if c == nil || c.surfacePersistent || os.Getenv("GPUI_COMPOSITE_SURFACE") == "0" {
		return false
	}
	if rc := c.gpuCtxOps(); rc != nil {
		_, ok := rc.(interface {
			EnsureCompositeSurface(w, h int) (gpucontext.TextureView, bool, error)
			BlitCompositeSurfaceTo(dst gpucontext.TextureView, w, h uint32) error
		})
		if ok {
			return true
		}
	}
	return false
}

// markSurfaceContentForPresent registers the just-acquired surface view as
// holding the previous complete composite (GL persistent back buffer), so the
// composite loads color and damage-scissors instead of treating the fresh
// view handle as a full-repaint target. Called ONLY from
// PresentTarget.present, before the flush. The shared flush core must not
// mark: layer/effect flushes (recordLocal → FlushGPUWithView) reach it with
// pooled offscreen views whose stale pixels would be loaded under the new
// recording (hover re-record filled icon holes with the previous tenant's
// pixels — the P1 default-off regression).
//
// P6 P3: on the composite-surface flow the marked view is the render-owned
// composite surface, and the mark fires EVERY frame regardless of validity —
// the carry mark is what keeps the session's stencil per-pass Clear (a
// stable view without it would load cross-frame stencil winding and fill
// vector paths to their bounding box).
func (c *Context) markSurfaceContentForPresent(view gpucontext.TextureView) {
	if c == nil || view.IsNil() {
		return
	}
	if !c.surfacePersistent && !c.compositeSurfaceEnabled() {
		return
	}
	if c.surfacePersistent && !c.surfaceContentValid {
		// GL: invalid back buffer (force/resize/recovery) → no carry mark,
		// the frame composites full.
		return
	}
	if rc := c.gpuCtxOps(); rc != nil {
		if m, ok := rc.(interface {
			MarkSurfaceContent(gpucontext.TextureView)
		}); ok {
			m.MarkSurfaceContent(view)
		}
	}
}

// surfacePersistentEnabled reports the registered capability (tests).
func (c *Context) surfacePersistentEnabled() bool {
	return c != nil && c.surfacePersistent
}

// surfaceContentValidFlag reports whether the back buffer is known complete
// (tests).
func (c *Context) surfaceContentValidFlag() bool {
	return c != nil && c.surfaceContentValid
}

// setCompositeSurfaceValid records whether the render-owned composite surface
// (P6 P3) currently holds a complete frame. Cleared when the surface is
// (re)created/resized and on device recovery; set after each successful
// composite flush into it.
func (c *Context) setCompositeSurfaceValid(valid bool) {
	if c == nil {
		return
	}
	c.compositeSurfaceValid = valid
}

// ensureCompositeSurface forwards to the per-context GPU layer: allocate or
// resize the composite surface. recreated=true → content undefined, the
// caller clears the validity gate.
func (c *Context) ensureCompositeSurface(w, h int) (gpucontext.TextureView, bool, error) {
	if rc := c.gpuCtxOps(); rc != nil {
		if p, ok := rc.(interface {
			EnsureCompositeSurface(w, h int) (gpucontext.TextureView, bool, error)
		}); ok {
			return p.EnsureCompositeSurface(w, h)
		}
	}
	return gpucontext.TextureView{}, false, fmt.Errorf("render: composite surface unavailable")
}

// blitCompositeSurfaceTo forwards to the per-context GPU layer: copy the
// composite surface onto the acquired swapchain view (full-screen quad).
func (c *Context) blitCompositeSurfaceTo(dst gpucontext.TextureView, w, h uint32) error {
	if rc := c.gpuCtxOps(); rc != nil {
		if p, ok := rc.(interface {
			BlitCompositeSurfaceTo(dst gpucontext.TextureView, w, h uint32) error
		}); ok {
			return p.BlitCompositeSurfaceTo(dst, w, h)
		}
	}
	return fmt.Errorf("render: composite surface blit unavailable")
}

// CompositeClipSafe reports whether a scene composite may clip this frame to
// its damage and skip non-intersecting subtrees (P6 P2): only when pixels
// outside the damage provably hold the previous complete frame. Two carriers:
// the P1 persistent back buffer (GL, validity-gated) and the P3 render-owned
// composite surface (wgpu, validity-gated). First frame, forced/resize/
// recovery frames, kill switches off, and CPU contexts return false — the
// caller must composite full, because pixels outside the damage have no
// defined previous content to keep.
func (c *Context) CompositeClipSafe() bool {
	if c == nil {
		return false
	}
	if c.surfacePersistent {
		return c.surfaceContentValid
	}
	return c.compositeSurfaceEnabled() && c.compositeSurfaceValid
}

// readCompositeSurfacePixels forwards to the per-context GPU layer: read the
// P6 P3 composite surface back as top-down RGBA (wgpu live-surface capture;
// GL uses the FBO path and never reaches here).
func (c *Context) readCompositeSurfacePixels() ([]byte, int, int, bool) {
	if rc := c.gpuCtxOps(); rc != nil {
		if p, ok := rc.(interface {
			ReadCompositeSurfacePixels() ([]byte, int, int, bool)
		}); ok {
			return p.ReadCompositeSurfacePixels()
		}
	}
	return nil, 0, 0, false
}
