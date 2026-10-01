//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// allocSwapchainColorTex allocates the null-data color texture backing the
// swapchain FBO. Storage failures are shaped as OOM (see glAllocErr) so the
// caller can Finish+retry transient pressure, then borrow the old frame.
// The null-upload spelling differs per platform and lives in the twins
// (swapchain_tex_linux/windows.go); this shared body holds one copy of the
// bind/param/error flow.
func allocSwapchainColorTex(glCtx *gl.Context, internalFormat, format, dataType uint32, width, height int32) (uint32, error) {
	tex := glCtx.GenTextures(1)
	if tex == 0 {
		return 0, fmt.Errorf("gles: glGenTextures returned 0")
	}
	glCtx.BindTexture(gl.TEXTURE_2D, tex)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	glCtx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	texImageSwapchainNull(glCtx, internalFormat, format, dataType, width, height)
	if glErr := glCtx.GetError(); glErr != 0 {
		glCtx.BindTexture(gl.TEXTURE_2D, 0)
		glCtx.DeleteTextures(tex)
		return 0, glAllocErr("TexImage2D(swapchain)", glErr,
			fmt.Sprintf("format=0x%x %dx%d", internalFormat, width, height))
	}
	glCtx.BindTexture(gl.TEXTURE_2D, 0)
	return tex, nil
}

// allocateSwapchainFBO creates a persistent swapchain framebuffer backed by
// a color texture (Flutter fl_framebuffer parity: glTexImage2D null-data
// color texture + glFramebufferTexture2D, not a renderbuffer). A texture
// lets the driver page the surface instead of demanding one contiguous
// renderbuffer block, which is what failed synchronously (108–299ms
// Configure stalls) on every drag-storm step with a renderbuffer.
// Must be called with the GL context current (caller holds AdapterContext lock).
// OOM shaping comes from allocSwapchainColorTex above: the caller can
// Finish+retry transient pressure, then borrow the old frame.
func allocateSwapchainFBO(glCtx *gl.Context, format gputypes.TextureFormat, width, height uint32) (fbo, colorTex uint32, err error) {
	if glCtx == nil {
		return 0, 0, fmt.Errorf("gles: allocateSwapchainFBO: nil gl context")
	}
	if width == 0 || height == 0 {
		return 0, 0, hal.ErrZeroArea
	}

	internalFormat, dataFormat, dataType := textureFormatToGL(format)

	colorTex, err = allocSwapchainColorTex(glCtx, internalFormat, dataFormat, dataType, int32(width), int32(height))
	if err != nil {
		return 0, 0, err
	}
	fbo = glCtx.GenFramebuffers(1)
	if fbo == 0 {
		glCtx.DeleteTextures(colorTex)
		return 0, 0, fmt.Errorf("gles: glGenFramebuffers returned 0")
	}
	glCtx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	glCtx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, colorTex, 0)

	status := glCtx.CheckFramebufferStatus(gl.FRAMEBUFFER)

	glCtx.BindFramebuffer(gl.FRAMEBUFFER, 0)
	glCtx.BindTexture(gl.TEXTURE_2D, 0)

	if status != gl.FRAMEBUFFER_COMPLETE {
		glCtx.DeleteFramebuffers(fbo)
		glCtx.DeleteTextures(colorTex)
		return 0, 0, fmt.Errorf("gles: swapchain framebuffer incomplete (status 0x%x)", status)
	}

	hal.Logger().Debug("gles: allocated swapchain FBO",
		"fbo", fbo,
		"colorTex", colorTex,
		"width", width,
		"height", height,
		"internalFormat", fmt.Sprintf("0x%x", internalFormat),
	)
	return fbo, colorTex, nil
}

// destroySwapchainFBO releases the swapchain framebuffer and its color
// texture. Safe to call with zero handles or nil context.
func destroySwapchainFBO(glCtx *gl.Context, fbo, colorTex uint32) {
	if glCtx == nil {
		return
	}
	if fbo != 0 {
		glCtx.DeleteFramebuffers(fbo)
	}
	if colorTex != 0 {
		glCtx.DeleteTextures(colorTex)
	}
}

// blitSwapchainToDefaultWith performs the present-time Y-flipping blit from
// the Surface's swapchain FBO to the default framebuffer (FBO 0).
// Must be called with GL context current on the user window DC.
func (s *Surface) blitSwapchainToDefaultWith(glCtx *gl.Context) {
	if glCtx == nil || s.swapchainFBO == 0 {
		return
	}
	if s.fboWidth == 0 || s.fboHeight == 0 {
		return
	}

	glCtx.Disable(gl.SCISSOR_TEST)

	glCtx.BindFramebuffer(gl.READ_FRAMEBUFFER, s.swapchainFBO)
	glCtx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)

	// Default framebuffer only accepts BACK as draw buffer. Render passes
	// leave COLOR_ATTACHMENT0 in glDrawBuffers state, which silently drops
	// the blit below (black window, no GL error).
	glCtx.DrawBuffers([]uint32{gl.BACK})

	w := int32(s.fboWidth)
	h := int32(s.fboHeight)
	// Borrowed old FBO (fbo size != window size): stretch the last good
	// image to the new window. Steady state stays 1:1.
	dw, dh := w, h
	if s.config != nil && s.config.Width != 0 && s.config.Height != 0 &&
		(s.config.Width != s.fboWidth || s.config.Height != s.fboHeight) {
		dw, dh = int32(s.config.Width), s.presentHeight()
	}

	glCtx.BlitFramebuffer(
		0, h, w, 0, // source Y-flipped
		0, 0, dw, dh, // dest normal (stretched only while borrowing)
		gl.COLOR_BUFFER_BIT, gl.NEAREST,
	)

	glCtx.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	glCtx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)
}

// sameExtentConfigured reports whether the live swapchain FBO already backs
// the requested extent+format (Flutter OnScreenSurfaceResize parity: size ==
// GetSize() returns without rebuilding). Present-mode flips during a drag
// storm otherwise destroyed and reallocated the FBO on every vsync switch
// (~490 rebuilds in an 8s storm), stalling the raster thread and flashing
// the window. Only skips when a live FBO backs the size; a previous failed
// alloc (FBO 0) must retry. Shared by both platform Configure twins.
func (s *Surface) sameExtentConfigured(config *hal.SurfaceConfiguration) bool {
	if config == nil || !s.hasLiveSwapchain() {
		return false
	}
	return s.configured && s.config != nil &&
		s.config.Width == config.Width && s.config.Height == config.Height &&
		s.config.Format == config.Format &&
		s.fboWidth == config.Width && s.fboHeight == config.Height
}

// hasLiveSwapchain reports whether a usable swapchain FBO is backing the
// current size. A previous failed alloc leaves FBO 0 and must retry.
func (s *Surface) hasLiveSwapchain() bool {
	return s != nil && s.swapchainFBO != 0 && s.fboWidth != 0 && s.fboHeight != 0
}

// borrowedSize reports whether the live FBO still serves an older window
// size (a storm step borrowed the old frame to survive transient driver
// pressure). Swapchain.BeginFrame retries the rebuild until it converges.
func (s *Surface) borrowedSize(w, h uint32) bool {
	return s.hasLiveSwapchain() && (s.fboWidth != w || s.fboHeight != h)
}

// borrowOldFrameOrErr keeps serving the old FBO when a rebuild fails under
// pressure (nil), or returns the error when no old frame exists to borrow
// (very first allocation). Reason names the Warn log (ledger vs rebuild).
func (s *Surface) borrowOldFrameOrErr(w, h uint32, err error, reason string) error {
	if s.hasLiveSwapchain() {
		hal.Logger().Warn("gles: "+reason+", borrowing old frame",
			"want", fmt.Sprintf("%dx%d", w, h),
			"have", fmt.Sprintf("%dx%d", s.fboWidth, s.fboHeight),
			"err", err)
		return nil
	}
	return err
}

// reconfigureSwapchainFBOWith allocates the new swapchain FBO first and
// only then retires the old one (Flutter OnScreenSurfaceResize parity:
// the old EGL surface is destroyed after the new one is created). The
// previous destroy-first order left FBO 0 behind when the new alloc
// failed under drag-storm pressure, so Present blitted nothing and the
// window flashed transparent for that frame.
//
// A failed rebuild no longer fails the frame when a live FBO exists: after
// one Finish+retry (lets in-flight work release driver memory), the old FBO
// keeps serving and the new size converges on later presents (see
// Swapchain.BeginFrame). Only the very first allocation (no old frame to
// borrow) returns the error. Caller must hold the AdapterContext lock.
func (s *Surface) reconfigureSwapchainFBOWith(glCtx *gl.Context, format gputypes.TextureFormat, width, height uint32) error {
	need := hal.VramSurfaceBytes(width, height, format)
	if err := hal.VramCheck("GLES.Surface.Configure", need); err != nil {
		return s.borrowOldFrameOrErr(width, height, err, "gles: swapchain ledger over budget")
	}
	fbo, colorTex, err := allocateSwapchainFBO(glCtx, format, width, height)
	if err != nil {
		// Transient driver pressure (the 108–299ms storm stalls): one Finish
		// lets in-flight work release memory, then retry once. Steady-state
		// successes never pay this.
		glCtx.Finish()
		fbo, colorTex, err = allocateSwapchainFBO(glCtx, format, width, height)
	}
	if err != nil {
		return s.borrowOldFrameOrErr(width, height, err, "gles: swapchain rebuild failed")
	}
	destroySwapchainFBO(glCtx, s.swapchainFBO, s.colorTexture)
	if s.vramHandle != 0 {
		hal.VramForget(s.vramHandle)
		s.vramHandle = 0
	}
	s.swapchainFBO = fbo
	s.colorTexture = colorTex
	s.fboWidth = width
	s.fboHeight = height
	s.vramHandle = vramHandleForFBO(fbo, height)
	hal.VramAdd(s.vramHandle, need)
	return nil
}

// presentHeight is the window height damage rects and blit dest use.
// Normally the FBO height; while borrowing an old FBO (fbo size !=
// window size) it is the config height so the frame still fills the
// new window instead of leaving garbage margins.
func (s *Surface) presentHeight() int32 {
	if s == nil {
		return 0
	}
	if s.config != nil && s.config.Height != 0 {
		return int32(s.config.Height)
	}
	return int32(s.fboHeight)
}
