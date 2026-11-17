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

// markSurfaceContentForPresent registers the just-acquired surface view as
// holding the previous complete composite (GL persistent back buffer), so the
// composite loads color and damage-scissors instead of treating the fresh
// view handle as a full-repaint target. Called ONLY from
// PresentTarget.present, before the flush. The shared flush core must not
// mark: layer/effect flushes (recordLocal → FlushGPUWithView) reach it with
// pooled offscreen views whose stale pixels would be loaded under the new
// recording (hover re-record filled icon holes with the previous tenant's
// pixels — the P1 default-off regression).
func (c *Context) markSurfaceContentForPresent(view gpucontext.TextureView) {
	if c == nil || view.IsNil() || !c.surfacePersistent || !c.surfaceContentValid {
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

// CompositeClipSafe reports whether a scene composite may clip this frame to
// its damage and skip non-intersecting subtrees (P6 P2): only when the back
// buffer provably holds the previous complete frame (P6 P1 persistent surface
// + validity gate). First frame, forced/resize/recovery frames, P1 disabled
// via GPUI_SURFACE_PERSISTENT=0, and non-persistent surfaces (wgpu reports
// the capability false) all return false — the caller must composite full,
// because pixels outside the damage have no defined previous content to keep.
func (c *Context) CompositeClipSafe() bool {
	return c != nil && c.surfacePersistent && c.surfaceContentValid
}
