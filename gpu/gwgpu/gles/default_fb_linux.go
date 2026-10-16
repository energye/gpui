//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"fmt"
	"os"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/hal"
)

// Window default framebuffer as the surface render target.
//
// A surface pass on such a Surface writes straight into the window default
// framebuffer (FBO 0) instead of the Surface's offscreen swapchain FBO, which
// removes the Y-flipping glBlitFramebuffer Queue.Present used to run before
// every SwapBuffers (spin_min 2026-10-16: 40ms per 10s window, ~67us/frame).
// The scene is drawn mirrored into FBO 0 and the fragment @builtin(position)
// read is mirrored back, both resolved from the GpuiTargetAdjust block (see
// target_adjust.go), so the window shows the pixels the offscreen route
// produced — pixel-verified by the tolerance-0 window goldens.
//
// FBO 0 cannot have images attached (GL forbids attaching a texture to it), so
// the pass runs on the window's own depth/stencil buffers. Those exist because
// chooseEGLConfig requires DepthSize 24 + StencilSize 8 (egl/context.go), and
// EGL treats both as minimums: the selected config carries at least that many
// bits. Impeller's GLES backend takes the same route (Android asks for 24/8 and
// binds FBO 0 without attaching anything); the alternative used by Flutter's
// Windows and GTK embedders — wrapping the window in an FBO with a
// GL_DEPTH24_STENCIL8 renderbuffer — still needs the present blit, so it buys
// nothing here.
//
// The query below is the guard: a config that cannot back a depth/stencil-
// writing pass leaves directDefaultFB false and the old offscreen route stays,
// so no pass ever renders without the depth/stencil it asked for.

// directDefaultFBDisabled forces the offscreen swapchain route for bisecting
// regressions against the direct route (GPUI_GLES_NO_DIRECT_FBO0=1).
var directDefaultFBDisabled = os.Getenv("GPUI_GLES_NO_DIRECT_FBO0") != ""

// eglConfigHasDepthStencil reports whether the window config carries the
// depth/stencil bits a surface pass needs, plus the queried sizes for logs.
// DepthSize and StencilSize are minimums in eglChooseConfig, so this normally
// reports the requested 24/8. A failed query reports false: an uninspectable
// config keeps the offscreen route rather than rendering into missing buffers.
func eglConfigHasDepthStencil(eglCtx *egl.Context) (depth, stencil int32, ok bool) {
	if eglCtx == nil {
		return 0, 0, false
	}
	display, cfg := eglCtx.Display(), eglCtx.Config()
	if display == 0 || cfg == 0 {
		return 0, 0, false
	}
	var d, s egl.EGLInt
	if egl.GetConfigAttrib(display, cfg, egl.DepthSize, &d) == egl.False {
		return 0, 0, false
	}
	if egl.GetConfigAttrib(display, cfg, egl.StencilSize, &s) == egl.False {
		return 0, 0, false
	}
	return int32(d), int32(s), int32(d) >= 24 && int32(s) >= 8
}

// markDefaultFBStale records that the window default framebuffer's depth and
// stencil buffers lose their content at the swap. EGL leaves a back buffer's
// depth/stencil undefined after SwapBuffers, so what a pass leaves behind is
// not there next frame: the first surface pass of the next frame clears
// whatever its own load ops leave out (ClearDefaultDepthStencilCommand).
// Color needs no such flag: the same LoadOpClear / LoadOpLoad contract the
// offscreen route has (and the damage-scissored present already relies on the
// back buffer keeping untouched pixels) applies unchanged.
func (s *Surface) markDefaultFBStale() {
	if s.directDefaultFB {
		s.defaultFBStale = true
	}
}

// directWindowSurface returns the window whose default framebuffer the given
// command buffers render into, or nil when they only touch FBOs.
//
// FBO 0 is whatever surface the EGL context is bound to, so a buffer that
// draws into it must be run with that window bound (Queue.Submit), and two
// windows in one submit would have the second one's passes land in the first
// one's back buffer: rejected here instead.
func directWindowSurface(commandBuffers []hal.CommandBuffer) (*Surface, error) {
	var surf *Surface
	for _, cb := range commandBuffers {
		cmdBuf, ok := cb.(*CommandBuffer)
		if !ok || cmdBuf.windowSurface == nil {
			continue
		}
		if surf != nil && surf != cmdBuf.windowSurface {
			return nil, fmt.Errorf("gles: Submit mixes two windows' default framebuffers")
		}
		surf = cmdBuf.windowSurface
	}
	return surf, nil
}

// enableDirectDefaultFB resolves whether this Surface renders into FBO 0 and
// logs the decision. Called once per EGL window surface (create and recreate).
func (s *Surface) enableDirectDefaultFB(eglCtx *egl.Context) {
	depth, stencil, ok := eglConfigHasDepthStencil(eglCtx)
	s.directDefaultFB = ok && !directDefaultFBDisabled
	s.defaultFBStale = false
	switch {
	case s.directDefaultFB:
		hal.Logger().Info("gles: surface renders into the window default framebuffer",
			"depth", depth, "stencil", stencil)
	case directDefaultFBDisabled:
		hal.Logger().Info("gles: surface keeps the offscreen swapchain route",
			"reason", "GPUI_GLES_NO_DIRECT_FBO0", "depth", depth, "stencil", stencil)
	default:
		hal.Logger().Warn("gles: surface keeps the offscreen swapchain route",
			"reason", "window config lacks 24/8 depth/stencil", "depth", depth, "stencil", stencil)
	}
}
