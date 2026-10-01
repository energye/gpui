//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build windows && !(js && wasm)

package gles

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/gles/wgl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Surface implements hal.Surface for OpenGL on Windows.
// Lightweight — does NOT own the GL context. The context lives on Instance's
// hidden window via AdapterContext. Surface stores only the user HWND and a
// reference to the shared AdapterContext.
type Surface struct {
	hwnd       wgl.HWND
	ctx        *AdapterContext // shared, NOT owned
	configured bool
	config     *hal.SurfaceConfiguration

	// Swapchain offscreen framebuffer. User render passes that target this
	// Surface render into swapchainFBO (backed by a color texture, not
	// FBO 0). Queue.Present blits this FBO to the default framebuffer
	// with an explicit Y-flip before SwapBuffers.
	swapchainFBO uint32
	colorTexture uint32
	fboWidth     uint32
	fboHeight    uint32
	// vramHandle is the ledger slot for the swapchain surface bytes
	// (tagged FBO id; 0 = not charged). Refunded on re-configure.
	vramHandle uintptr

	// current tracks the single in-flight acquired frame (webgpu parity:
	// one frame at a time — Acquire discards the previous one, Discard or
	// a successful Present drops it). SurfaceTexture carries no GL
	// resources, so this is API strictness only, no GPU behavior change.
	current *SurfaceTexture
}

// GetAdapterInfo returns adapter information from this surface's GL context.
// Not used in the new architecture (Instance.EnumerateAdapters queries directly),
// but kept for interface compatibility.
func (s *Surface) GetAdapterInfo() hal.ExposedAdapter {
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
			ctx:     s.ctx,
			version: fmt.Sprintf("%d.%d", caps.GLMajor, caps.GLMinor),
			caps:    caps,
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
// Uses two separate lock scopes to avoid stomping MakeCurrent state:
// 1. LockForDC(userDC) — set swap interval (requires context on user DC)
// 2. Lock() — allocate swapchain FBO (requires context on hidden DC)
func (s *Surface) Configure(_ hal.Device, config *hal.SurfaceConfiguration) error {
	if config.Width == 0 || config.Height == 0 {
		return hal.ErrZeroArea
	}

	// Scope 1: Set swap interval on user window DC.
	// SetSwapInterval requires a current context on the target DC.
	hdc := wgl.GetDC(s.hwnd)
	if hdc != 0 {
		s.ctx.LockForDC(hdc)
		wgl.LoadExtensions(hdc)
		if wgl.HasSwapControl() {
			var interval int
			switch config.PresentMode {
			case hal.PresentModeFifo, hal.PresentModeFifoRelaxed:
				interval = 1
			case hal.PresentModeImmediate, hal.PresentModeMailbox:
				interval = 0
			default:
				interval = 1
			}
			_ = wgl.SetSwapInterval(interval)
		}
		s.ctx.Unlock()
		wgl.ReleaseDC(s.hwnd, hdc)
	}

	// Same-extent/format fast path (shared helper in surface.go; Linux twin:
	// resource_linux.go): a present-mode flip must not realloc the FBO when
	// the size did not change. Skip only with a live FBO backing the size;
	// FBO 0 retries.
	if s.sameExtentConfigured(config) {
		s.config = config
		return nil
	}

	// Scope 2: Allocate swapchain FBO on hidden DC. Stop on bind failure
	// (Linux twin: resource_linux.go); a 0 handle would only fail later
	// with a bare Gen error and hide the real cause.
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

// Unconfigure marks the surface as unconfigured.
func (s *Surface) Unconfigure(_ hal.Device) {
	glCtx := s.ctx.Lock()
	defer s.ctx.Unlock()

	destroySwapchainFBO(glCtx, s.swapchainFBO, s.colorTexture)
	if s.vramHandle != 0 {
		hal.VramForget(s.vramHandle)
		s.vramHandle = 0
	}
	s.swapchainFBO = 0
	s.colorTexture = 0
	s.fboWidth = 0
	s.fboHeight = 0
	s.current = nil
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
func (s *Surface) ActualExtent() (width, height uint32) {
	if s.config == nil {
		return 0, 0
	}
	return s.config.Width, s.config.Height
}

// Destroy releases the surface resources.
// Does NOT destroy the GL context — that's owned by Instance.
// Bind failure (driver gone) still clears bookkeeping: handles die
// with the driver.
func (s *Surface) Destroy() {
	if s.ctx != nil {
		if glCtx, err := s.ctx.TryLock(); err == nil {
			defer s.ctx.Unlock()
			destroySwapchainFBO(glCtx, s.swapchainFBO, s.colorTexture)
		}
	}
	s.swapchainFBO = 0
	s.colorTexture = 0
	s.current = nil
}

// SurfaceTexture implements hal.SurfaceTexture for OpenGL on Windows.
type SurfaceTexture struct {
	surface *Surface
}

func (t *SurfaceTexture) CurrentUsage() gputypes.TextureUsage { return 0 }
func (t *SurfaceTexture) AddPendingRef()                      {}
func (t *SurfaceTexture) DecPendingRef()                      {}
func (t *SurfaceTexture) Destroy()                            {}
func (t *SurfaceTexture) NativeHandle() uintptr               { return 0 }

// Format returns the surface configuration format, or Undefined if unconfigured.
func (t *SurfaceTexture) Format() gputypes.TextureFormat {
	if t.surface != nil && t.surface.config != nil {
		return t.surface.config.Format
	}
	return gputypes.TextureFormatUndefined
}
