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

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Surface implements hal.Surface for OpenGL on Linux.
// When Instance has a pre-created AdapterContext (X11/headless), ownsContext=false —
// Surface shares Instance's context.
// When Instance has no context (Wayland), ownsContext=true — Surface owns its own
// AdapterContext (intentional Wayland divergence).
type Surface struct {
	displayHandle uintptr
	windowHandle  uintptr
	ctx           *AdapterContext
	eglDisplay    egl.EGLDisplay
	eglSurface    egl.EGLSurface
	ownsContext   bool // true = Surface owns AdapterContext, false = shared from Instance
	version       string
	renderer      string
	configured    bool
	config        *hal.SurfaceConfiguration

	// current tracks the single in-flight acquired frame (webgpu parity:
	// one frame at a time — Acquire discards the previous one, Discard or
	// a successful Present drops it). SurfaceTexture carries no GL
	// resources, so this is API strictness only, no GPU behavior change.
	current *SurfaceTexture

	// Wayland-specific: wl_egl_window handle. On Wayland, EGL cannot use
	// the raw wl_surface* directly — it needs a wl_egl_window wrapper.
	// Created in Configure via libwayland-egl.so, destroyed in Destroy.
	// Zero on X11 (where windowHandle is used directly with eglCreateWindowSurface).
	eglWindow uintptr // wl_egl_window* (0 on X11 or before Configure)
	isWayland bool    // true if the Context was created for Wayland

	// Swapchain offscreen framebuffer. User render passes that target this
	// Surface render into swapchainFBO (backed by colorRenderbuffer), not FBO 0.
	// Queue.Present blits this FBO to the default framebuffer with an explicit
	// Y-flip before SwapBuffers.
	swapchainFBO        uint32
	colorRenderbuffer   uint32
	fboWidth, fboHeight uint32
	// vramHandle is the ledger slot for the swapchain surface bytes
	// (tagged FBO id; 0 = not charged). Refunded on re-configure.
	vramHandle uintptr
}

// GetAdapterInfo returns adapter information from this surface's GL context.
// Probes GL version, extensions, features, limits, and MSAA support to build
// an accurate ExposedAdapter.
func (s *Surface) GetAdapterInfo() hal.ExposedAdapter {
	if s.ctx == nil {
		return placeholderExposedAdapter("OpenGL 3.3+ / ES 3.0+ (no AdapterContext)")
	}

	glCtx := s.ctx.Lock()
	defer s.ctx.Unlock()

	caps := queryAdapterCapabilities(glCtx)

	driverInfo := "OpenGL 3.3+"
	if caps.IsES {
		driverInfo = fmt.Sprintf("OpenGL ES %d.%d", caps.GLMajor, caps.GLMinor)
	} else if caps.GLMajor > 0 {
		driverInfo = fmt.Sprintf("OpenGL %d.%d", caps.GLMajor, caps.GLMinor)
	}

	return hal.ExposedAdapter{
		Adapter: &Adapter{
			ctx:           s.ctx,
			displayHandle: s.displayHandle,
			windowHandle:  s.windowHandle,
			version:       s.version,
			renderer:      s.renderer,
			caps:          caps,
		},
		Info: gputypes.AdapterInfo{
			Name:       caps.Renderer,
			Vendor:     caps.Vendor,
			VendorID:   caps.VendorID,
			DeviceID:   0,
			DeviceType: caps.DeviceType,
			Driver:     caps.Version,
			DriverInfo: driverInfo,
			Backend:    gputypes.BackendGL,
		},
		Features: caps.Features,
		Capabilities: hal.Capabilities{
			Limits: caps.Limits,
			AlignmentsMask: hal.Alignments{
				BufferCopyOffset: 4,
				BufferCopyPitch:  4,
			},
			DownlevelCapabilities: gputypes.DownlevelCapabilities{
				ShaderModel: gputypes.ShaderModelSm5,
				Limits:      gputypes.DownlevelExtraLimits{},
				Flags:       caps.DownlevelFlags,
			},
		},
	}
}

// Configure configures the surface for presentation.
//
// On Wayland, this creates a wl_egl_window via libwayland-egl.so and then
// an EGL window surface. On X11, eglCreateWindowSurface takes the raw X11
// Window handle directly.
//
// Returns hal.ErrZeroArea if width or height is zero.
// This commonly happens when the window is minimized or not yet fully visible.
// Wait until the window has valid dimensions before calling Configure again.
func (s *Surface) Configure(_ hal.Device, config *hal.SurfaceConfiguration) error {
	// Validate dimensions first (before any side effects).
	if config.Width == 0 || config.Height == 0 {
		return hal.ErrZeroArea
	}

	if s.ctx == nil || s.ctx.EGL() == nil {
		return fmt.Errorf("gles: surface has no AdapterContext")
	}

	// Create EGL window surface if not yet created (first Configure call).
	// On Wayland: need wl_egl_window → eglCreateWindowSurface.
	// On X11: eglCreateWindowSurface with raw X11 Window.
	if s.eglSurface == 0 && s.windowHandle != 0 {
		if err := s.createEGLWindowSurface(config.Width, config.Height); err != nil {
			return fmt.Errorf("gles: failed to create EGL window surface: %w", err)
		}
	}

	// On Wayland resize: update wl_egl_window dimensions.
	if s.isWayland && s.eglWindow != 0 && s.config != nil {
		if s.config.Width != config.Width || s.config.Height != config.Height {
			egl.WlEGLWindowResize(s.eglWindow, int32(config.Width), int32(config.Height), 0, 0)
		}
	}

	// Swapchain FBO allocation needs a current context (Lock). A bind
	// failure used to fall through into 0-handle allocs; stop instead —
	// the caller surfaces the EGL error instead of a bare Gen failure.
	glCtx, lockErr := s.ctx.TryLock()
	if lockErr != nil {
		return fmt.Errorf("gles: failed to bind GL context for swapchain framebuffer: %w", lockErr)
	}
	defer s.ctx.Unlock()

	if err := s.reconfigureSwapchainFBOWith(glCtx, config.Format, config.Width, config.Height); err != nil {
		return fmt.Errorf("gles: failed to configure swapchain framebuffer: %w", err)
	}

	s.configured = true
	s.config = config
	return nil
}

// createEGLWindowSurface creates the EGL window surface. On Wayland this
// requires creating a wl_egl_window first via libwayland-egl.so.
func (s *Surface) createEGLWindowSurface(width, height uint32) error {
	eglCtx := s.ctx.EGL()
	s.eglDisplay = eglCtx.Display()
	s.isWayland = eglCtx.WindowKind() == egl.WindowKindWayland

	if s.isWayland {
		return s.createWaylandEGLSurface(width, height)
	}
	return s.createX11EGLSurface()
}

// createWaylandEGLSurface creates a wl_egl_window then an EGL window surface.
// Prefers EGL 1.5 eglCreatePlatformWindowSurface (spec-correct void* native window)
// with fallback to EGL 1.4 eglCreateWindowSurface.
func (s *Surface) createWaylandEGLSurface(width, height uint32) error {
	if !egl.InitWaylandEGL() {
		return fmt.Errorf("libwayland-egl.so not available — cannot create Wayland EGL surface")
	}

	eglWin := egl.WlEGLWindowCreate(s.windowHandle, int32(width), int32(height))
	if eglWin == 0 {
		return fmt.Errorf("wl_egl_window_create failed for wl_surface 0x%x", s.windowHandle)
	}
	s.eglWindow = eglWin

	// EGL 1.5 path: eglCreatePlatformWindowSurface takes void* — spec-correct for Wayland.
	// Falls back to eglCreateWindowSurface internally if EGL 1.5 unavailable.
	attribs := []egl.EGLAttrib{egl.EGLAttrib(egl.None)}
	eglSurface := egl.CreatePlatformWindowSurface(s.eglDisplay, s.ctx.EGL().Config(), eglWin, &attribs[0])
	if eglSurface == egl.NoSurface {
		egl.WlEGLWindowDestroy(eglWin)
		s.eglWindow = 0
		return fmt.Errorf("eglCreatePlatformWindowSurface failed for wl_egl_window: error 0x%x", egl.GetError())
	}
	s.eglSurface = eglSurface

	eglPath := "eglCreateWindowSurface (EGL 1.4)"
	if egl.HasPlatformWindowSurface() {
		eglPath = "eglCreatePlatformWindowSurface (EGL 1.5)"
	}
	hal.Logger().Info("gles: Wayland EGL window surface created",
		"path", eglPath,
		"eglWindow", fmt.Sprintf("0x%x", eglWin),
		"eglSurface", fmt.Sprintf("0x%x", eglSurface),
		"width", width, "height", height,
	)
	return nil
}

// createX11EGLSurface creates an EGL window surface directly from an X11 Window.
func (s *Surface) createX11EGLSurface() error {
	attribs := []egl.EGLInt{egl.None}
	eglSurface := egl.CreateWindowSurface(s.eglDisplay, s.ctx.EGL().Config(), egl.EGLNativeWindowType(s.windowHandle), &attribs[0])
	if eglSurface == egl.NoSurface {
		return fmt.Errorf("eglCreateWindowSurface failed for X11 window 0x%x: error 0x%x", s.windowHandle, egl.GetError())
	}
	s.eglSurface = eglSurface

	hal.Logger().Info("gles: X11 EGL window surface created",
		"eglSurface", fmt.Sprintf("0x%x", eglSurface),
		"window", fmt.Sprintf("0x%x", s.windowHandle),
	)
	return nil
}

// Unconfigure marks the surface as unconfigured and releases the EGL window
// surface and wl_egl_window (on Wayland).
func (s *Surface) Unconfigure(_ hal.Device) {
	if s.ctx != nil {
		glCtx := s.ctx.Lock()
		destroySwapchainFBO(glCtx, s.swapchainFBO, s.colorRenderbuffer)
		s.ctx.Unlock()
	}
	if s.vramHandle != 0 {
		hal.VramForget(s.vramHandle)
		s.vramHandle = 0
	}
	s.swapchainFBO = 0
	s.colorRenderbuffer = 0
	s.fboWidth = 0
	s.fboHeight = 0
	s.current = nil

	// Destroy EGL surface before wl_egl_window (order matters).
	if s.eglSurface != 0 && s.eglDisplay != 0 {
		egl.DestroySurface(s.eglDisplay, s.eglSurface)
		s.eglSurface = 0
	}
	if s.eglWindow != 0 {
		egl.WlEGLWindowDestroy(s.eglWindow)
		s.eglWindow = 0
	}

	s.configured = false
	s.config = nil
}

// AcquireTexture returns the next surface texture for rendering.
//
// Strict codes: nil receiver, unconfigured surface
// (!configured/config==nil) or a lost context (ctx==nil) all report
// hal.ErrSurfaceLost (errors.Is-compatible); a previous in-flight frame is
// discarded first.
// Timeout/NotReady do not apply: GL acquire is synchronous and always
// succeeds once configured.
func (s *Surface) AcquireTexture(_ hal.Fence) (*hal.AcquiredSurfaceTexture, error) {
	if s == nil {
		return nil, fmt.Errorf("gles: surface is nil: %w", hal.ErrSurfaceLost)
	}
	if !s.configured || s.config == nil {
		return nil, fmt.Errorf("gles: surface not configured: %w", hal.ErrSurfaceLost)
	}
	if s.ctx == nil {
		return nil, fmt.Errorf("gles: surface context lost: %w", hal.ErrSurfaceLost)
	}
	// One in-flight frame at a time: drop the previous one first.
	s.current = nil
	st := &SurfaceTexture{
		surface: s,
	}
	s.current = st
	return &hal.AcquiredSurfaceTexture{
		Texture:    st,
		Suboptimal: false,
	}, nil
}

// DiscardTexture discards a previously acquired texture without presenting.
// Nil-safe; drops the tracked in-flight frame when tex is nil or matches it,
// ignores foreign textures.
func (s *Surface) DiscardTexture(tex hal.SurfaceTexture) {
	if s == nil || s.current == nil {
		return
	}
	if tex == nil {
		s.current = nil
		return
	}
	if st, ok := tex.(*SurfaceTexture); ok && st == s.current {
		s.current = nil
	}
}

// ActualExtent returns the configured surface dimensions.
// GLES does not clamp the extent, so these always match the requested values.
// Returns (0, 0) if the surface is not configured.
func (s *Surface) ActualExtent() (width, height uint32) {
	if s.config == nil {
		return 0, 0
	}
	return s.config.Width, s.config.Height
}

// Destroy releases the surface resources.
// Order: GL resources → EGL surface → wl_egl_window → AdapterContext (if owned).
func (s *Surface) Destroy() {
	// Release swapchain FBO before tearing down the GL context.
	if s.ctx != nil {
		glCtx := s.ctx.Lock()
		destroySwapchainFBO(glCtx, s.swapchainFBO, s.colorRenderbuffer)
		s.ctx.Unlock()
	}
	s.swapchainFBO = 0
	s.colorRenderbuffer = 0
	s.fboWidth = 0
	s.fboHeight = 0
	s.current = nil

	// Destroy EGL surface before wl_egl_window (order matters per Wayland spec).
	if s.eglSurface != 0 && s.eglDisplay != 0 {
		egl.DestroySurface(s.eglDisplay, s.eglSurface)
		s.eglSurface = 0
	}
	if s.eglWindow != 0 {
		egl.WlEGLWindowDestroy(s.eglWindow)
		s.eglWindow = 0
	}

	// Only destroy AdapterContext if Surface owns it (Wayland path).
	// When shared from Instance (X11/headless), Instance.Destroy handles cleanup.
	if s.ownsContext && s.ctx != nil {
		s.ctx.Destroy()
	}
	s.ctx = nil
}

// SurfaceTexture implements hal.SurfaceTexture for OpenGL.
// It represents the default framebuffer.
type SurfaceTexture struct {
	surface *Surface
}

// CurrentUsage returns 0 — GLES surface textures have no state tracking.
func (t *SurfaceTexture) CurrentUsage() gputypes.TextureUsage { return 0 }
func (t *SurfaceTexture) AddPendingRef()                      {}
func (t *SurfaceTexture) DecPendingRef()                      {}

// Format returns the surface configuration format, or Undefined if unconfigured.
func (t *SurfaceTexture) Format() gputypes.TextureFormat {
	if t.surface != nil && t.surface.config != nil {
		return t.surface.config.Format
	}
	return gputypes.TextureFormatUndefined
}

// Destroy is a no-op for surface textures.
func (t *SurfaceTexture) Destroy() {}

// NativeHandle returns 0 (OpenGL default framebuffer has no handle).
func (t *SurfaceTexture) NativeHandle() uintptr { return 0 }
