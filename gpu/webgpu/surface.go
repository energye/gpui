//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"fmt"
	"image"
	"strings"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

// isDeviceLostErr reports whether err indicates a permanently lost GPU device.
func isDeviceLostErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, hal.ErrDeviceLost) {
		return true
	}
	if errors.Is(err, rwgpu.ErrDeviceLost) || errors.Is(err, rwgpu.ErrSurfaceDeviceLost) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "device lost") || strings.Contains(msg, "Device lost") ||
		strings.Contains(msg, "Parent device is lost")
}

// mapSurfaceAcquireErr maps rwgpu surface acquire failures to public webgpu
// sentinels so callers can errors.Is against hal.ErrDeviceLost / hal.ErrTimeout / etc.
func mapSurfaceAcquireErr(err error) error {
	if err == nil {
		return nil
	}
	if isDeviceLostErr(err) {
		return hal.ErrDeviceLost
	}
	switch {
	case errors.Is(err, rwgpu.ErrSurfaceOccluded):
		return ErrSurfaceOccluded
	case errors.Is(err, rwgpu.ErrSurfaceTimeout):
		return hal.ErrTimeout
	case errors.Is(err, rwgpu.ErrSurfaceNeedsReconfigure):
		return hal.ErrSurfaceOutdated
	case errors.Is(err, rwgpu.ErrSurfaceLost):
		return hal.ErrSurfaceLost
	case errors.Is(err, rwgpu.ErrSurfaceOutOfMemory):
		return ErrOutOfMemory
	}
	// Message fallback for wrapped/opaque paths.
	msg := err.Error()
	switch {
	case strings.Contains(msg, "occluded"):
		return ErrSurfaceOccluded
	case strings.Contains(msg, "timeout"):
		return hal.ErrTimeout
	case strings.Contains(msg, "needs reconfigure") || strings.Contains(msg, "outdated"):
		return hal.ErrSurfaceOutdated
	case strings.Contains(msg, "surface lost"):
		return hal.ErrSurfaceLost
	}
	return fmt.Errorf("wgpu: %w", err)
}

// isSkipFrameSurfaceErr is true when the surface is temporarily unavailable
// and the caller should skip the frame without reconfiguring.
func isSkipFrameSurfaceErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrSurfaceOccluded) || errors.Is(err, hal.ErrTimeout) ||
		errors.Is(err, rwgpu.ErrSurfaceOccluded) || errors.Is(err, rwgpu.ErrSurfaceTimeout)
}

// isOutdatedSurfaceErr is true when the surface needs reconfiguration.
func isOutdatedSurfaceErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, hal.ErrSurfaceOutdated) || errors.Is(err, hal.ErrSurfaceLost) ||
		errors.Is(err, rwgpu.ErrSurfaceNeedsReconfigure) || errors.Is(err, rwgpu.ErrSurfaceLost)
}

// Surface represents a platform rendering surface.
// On the wgpu-native backend, this wraps rwgpu Surface.
type Surface struct {
	r        *rwgpu.Surface
	device   *Device
	released bool

	// backend is the window-system the surface was created for (X11 vs
	// Wayland vs ...). X11 surfaces can probe the live window extent
	// (WindowSize) for stale-size reconfiguration during resize drags.
	backend SurfaceBackend

	// Platform handles for recreate after device-lost (AutoRecover).
	// Force-Unconfigure after lost SIGSEGVs reconfigure on this .so; drop+recreate instead.
	instance      *Instance
	displayHandle uintptr
	windowHandle  uintptr

	// current is the last acquired surface texture not yet Released/Presented.
	// Used by DiscardTexture so Configure never runs with a live SurfaceOutput
	// (native: "SurfaceOutput must be dropped before a new Surface is made").
	current *SurfaceTexture

	// Cached configuration for texture creation.
	configFormat TextureFormat
	configWidth  uint32
	configHeight uint32
}

// CreateSurface creates a rendering surface from a typed platform target.
// Implements hal.Instance (takes hal.SurfaceTarget, returns hal.Surface).
// Kind selects the window-system path; handles stay caller-owned.
//
// displayHandle and windowHandle are platform-specific:
//   - Windows: displayHandle=HINSTANCE (can be 0), windowHandle=HWND
//   - macOS: displayHandle=0, windowHandle=CAMetalLayer*
//   - Linux/X11: displayHandle=Display*, windowHandle=Window
//   - Linux/Wayland: displayHandle=wl_display*, windowHandle=wl_surface*
func (i *Instance) CreateSurface(target hal.SurfaceTarget) (hal.Surface, error) {
	return i.CreateSurfaceFor(surfaceBackendForTarget(target.Kind), target.DisplayHandle, target.WindowHandle)
}

// surfaceBackendForTarget maps a hal.SurfaceTargetKind to the wgpu-native
// SurfaceBackend path. Headless/invalid fall back to Auto (env heuristic).
func surfaceBackendForTarget(kind hal.SurfaceTargetKind) SurfaceBackend {
	switch kind {
	case hal.SurfaceTargetXlibWindow:
		return SurfaceBackendXlib
	case hal.SurfaceTargetWaylandSurface:
		return SurfaceBackendWayland
	case hal.SurfaceTargetWindowsHWND:
		return SurfaceBackendWin32
	case hal.SurfaceTargetMetalLayer:
		return SurfaceBackendMetal
	default:
		return SurfaceBackendAuto
	}
}

// CreateSurfaceFromHandles creates a rendering surface from platform-specific handles.
// On Linux this uses SurfaceBackendAuto (DISPLAY/WAYLAND_DISPLAY heuristic).
// Prefer CreateSurfaceFor when the window system is already known (X11 vs
// Wayland handles must match).
//
// This is the pre-hal entry point kept for existing callers (tests, swapchain
// recovery); new code should pass a hal.SurfaceTarget to CreateSurface.
func (i *Instance) CreateSurfaceFromHandles(displayHandle, windowHandle uintptr) (*Surface, error) {
	return i.CreateSurfaceFor(SurfaceBackendAuto, displayHandle, windowHandle)
}

// CreateSurfaceFor creates a surface using an explicit window-system backend.
// On Linux, backend must match the handle types (Xlib vs Wayland); mismatch
// causes native SIGSEGV in Configure/GetCapabilities.
func (i *Instance) CreateSurfaceFor(backend SurfaceBackend, displayHandle, windowHandle uintptr) (*Surface, error) {
	if i == nil {
		return nil, fmt.Errorf("wgpu: instance is nil")
	}
	if i.released {
		return nil, ErrReleased
	}
	// Native wgpu aborts on null platform pointers (Vulkan "Display pointer is not set").
	if windowHandle == 0 {
		return nil, fmt.Errorf("wgpu: CreateSurface requires a non-zero window handle")
	}

	rs, err := createPlatformSurfaceFor(i.r, backend, displayHandle, windowHandle)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create surface: %w", err)
	}

	return &Surface{
		r:             rs,
		backend:       backend,
		instance:      i,
		displayHandle: displayHandle,
		windowHandle:  windowHandle,
	}, nil
}

// WindowSize returns the current native window extent when the platform can
// probe it (X11: XGetGeometry, fresh at call time). ok=false falls back to
// the applied/configured swapchain size (Wayland/Windows/macOS are told the
// size by the window system instead of probing).
func (s *Surface) WindowSize() (width, height uint32, ok bool) {
	if s == nil || s.released {
		return 0, 0, false
	}
	if s.backend != SurfaceBackendXlib {
		return 0, 0, false
	}
	if w, h, ok := x11WindowSize(s.displayHandle, s.windowHandle); ok {
		return uint32(w), uint32(h), true
	}
	return 0, 0, false
}

// Configure configures the surface for presentation.
// Implements hal.Surface (takes hal.Device interface, internal unpack).
func (s *Surface) Configure(device hal.Device, config *SurfaceConfiguration) error {
	// Defense order: nil surface → released → invalid handle → config/device → lost → native.
	if s == nil {
		return fmt.Errorf("wgpu: surface is nil")
	}
	if s.released {
		return ErrReleased
	}
	if s.r == nil {
		return ErrInvalidHandle
	}
	if config == nil {
		return fmt.Errorf("wgpu: surface configuration is nil")
	}
	if device == nil {
		return fmt.Errorf("wgpu: device is nil")
	}
	wd, ok := device.(*Device)
	if !ok || wd == nil {
		return fmt.Errorf("wgpu: Configure requires *Device")
	}
	if wd.IsLost() {
		return hal.ErrDeviceLost
	}
	if config.Width == 0 || config.Height == 0 {
		return fmt.Errorf("wgpu: surface extent must be non-zero (got %dx%d)", config.Width, config.Height)
	}
	// Drop any in-flight surface output before reconfigure (native contract).
	s.DiscardTexture(nil)

	rConfig := &rwgpu.SurfaceConfiguration{
		Format:      config.Format,
		Usage:       config.Usage,
		Width:       config.Width,
		Height:      config.Height,
		AlphaMode:   config.AlphaMode,
		PresentMode: config.PresentMode,
	}

	if err := s.r.Configure(wd.r, rConfig); err != nil {
		if isDeviceLostErr(err) {
			return hal.ErrDeviceLost
		}
		return fmt.Errorf("wgpu: failed to configure surface: %w", err)
	}

	s.device = wd
	s.configFormat = config.Format
	s.configWidth = config.Width
	s.configHeight = config.Height
	return nil
}

// Unconfigure removes the surface configuration.
// Implements hal.Surface (takes hal.Device, ignored; device tracked internally).
// Nil-safe and released-safe. Skips native when device is lost (rwgpu lost-safe path).
func (s *Surface) Unconfigure(_ hal.Device) {
	if s == nil || s.released {
		return
	}
	s.DiscardTexture(nil)
	if s.r != nil {
		s.r.Unconfigure()
	}
	s.device = nil
	// Engine purge of surface-bound session textures (registered by render/gpu).
	if AfterSurfaceUnconfigure != nil {
		AfterSurfaceUnconfigure()
	}
}

// GetCurrentTexture acquires the next texture for rendering.
// Returns the surface texture and whether the surface is suboptimal.
func (s *Surface) GetCurrentTexture() (*SurfaceTexture, bool, error) {
	if err := prepareSurfaceCall(s); err != nil {
		return nil, false, err
	}
	if s.device == nil {
		return nil, false, fmt.Errorf("wgpu: surface not configured")
	}
	// Skia: abandon sticky → refuse; FlushCallbacks folds pending lost signals.
	if s.device.IsLost() {
		return nil, false, hal.ErrDeviceLost
	}
	s.device.FlushCallbacks()
	if s.device.IsLost() {
		return nil, false, hal.ErrDeviceLost
	}
	// One in-flight surface texture at a time (lost-safe Release on textures).
	// rwgpu.GetCurrentTexture also absorbs sticky uncaptured "Parent device is lost".
	s.DiscardTexture(nil)
	// Re-check after DiscardTexture / flush — never enter native when lost.
	if s.device.IsLost() {
		return nil, false, hal.ErrDeviceLost
	}

	rst, suboptimal, err := s.r.GetCurrentTexture()
	if err != nil {
		return nil, false, mapSurfaceAcquireErr(err)
	}

	st := &SurfaceTexture{
		r: rst,
		texture: &Texture{
			r:      rst.Texture,
			device: s.device,
			format: s.configFormat,
		},
		surface: s,
	}
	s.current = st
	return st, suboptimal, nil
}

// Present presents a surface texture to the screen.
func (s *Surface) Present(texture *SurfaceTexture) error {
	if err := prepareSurfaceCall(s); err != nil {
		return err
	}
	if texture == nil {
		return fmt.Errorf("wgpu: surface texture is nil")
	}
	if s.device != nil && s.device.IsLost() {
		return hal.ErrDeviceLost
	}
	// rwgpu Present takes variadic *SurfaceTexture.
	if err := s.r.Present(texture.r); err != nil {
		if isDeviceLostErr(err) {
			return hal.ErrDeviceLost
		}
		return err
	}
	// Present does not release ownership; caller still must Release after Present.
	// Clear current tracking only if this was the tracked texture — Release owns free.
	if s.current == texture {
		s.current = nil
	}
	return nil
}

// PresentWithDamage presents a surface texture, optionally with damage rects.
// On the wgpu-native backend, damage rects are ignored. wgpu-native does not support them.
func (s *Surface) PresentWithDamage(st *SurfaceTexture, _ []image.Rectangle) error {
	return s.Present(st)
}

// ActualExtent returns the configured surface dimensions.
func (s *Surface) ActualExtent() (width, height uint32) {
	if s.released {
		return 0, 0
	}
	return s.configWidth, s.configHeight
}

// SetPrepareFrame registers a platform hook called before each GetCurrentTexture.
// This is a no-op: wgpu-native handles HiDPI internally.
// The function signature uses any to avoid importing core in the native build path.
func (s *Surface) SetPrepareFrame(_ any) {}

// DiscardTexture drops the last acquired surface texture without presenting it.
// Implements hal.Surface (takes hal.SurfaceTexture, ignored when nil; drops current).
// Safe to call when no texture is held. Must run before Configure if a previous
// GetCurrentTexture succeeded and the texture was not Released.
func (s *Surface) DiscardTexture(_ hal.SurfaceTexture) {
	if s == nil || s.current == nil {
		return
	}
	st := s.current
	s.current = nil
	st.Release()
}

// AcquireTexture acquires the next surface texture for rendering.
// Implements hal.Surface.
func (s *Surface) AcquireTexture(_ hal.Fence) (*hal.AcquiredSurfaceTexture, error) {
	st, suboptimal, err := s.GetCurrentTexture()
	if err != nil {
		return nil, err
	}
	return &hal.AcquiredSurfaceTexture{Texture: st, Suboptimal: suboptimal}, nil
}

// Release releases the surface.
// Nil-safe and idempotent.
// Kept as門面 for existing callers; hal.Surface.Destroy forwards here.
func (s *Surface) Release() {
	if s == nil || s.released {
		return
	}
	s.DiscardTexture(nil)
	s.released = true
	if s.r != nil {
		s.r.Release()
		s.r = nil
	}
}

// Destroy implements hal.Surface (via hal.Resource): same as Release.
func (s *Surface) Destroy() { s.Release() }

// SurfaceTexture is a texture acquired from a surface for rendering.
type SurfaceTexture struct {
	r       *rwgpu.SurfaceTexture
	texture *Texture
	surface *Surface
}

// AsTexture returns the underlying Texture for direct WriteTexture access.
func (st *SurfaceTexture) AsTexture() hal.Texture { return st.texture }

// CreateView creates a texture view of this surface texture.
func (st *SurfaceTexture) CreateView(desc *hal.TextureViewDescriptor) (hal.TextureView, error) {
	if st.texture == nil || st.texture.r == nil {
		return nil, ErrReleased
	}

	var rDesc *rwgpu.TextureViewDescriptor
	if desc != nil {
		rDesc = &rwgpu.TextureViewDescriptor{
			Label:           desc.Label,
			Format:          desc.Format,
			Dimension:       desc.Dimension,
			Aspect:          rwgpu.TextureAspect(desc.Aspect),
			BaseMipLevel:    desc.BaseMipLevel,
			MipLevelCount:   desc.MipLevelCount,
			BaseArrayLayer:  desc.BaseArrayLayer,
			ArrayLayerCount: desc.ArrayLayerCount,
		}
	}

	rv, err := st.texture.r.CreateView(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create surface texture view: %w", err)
	}

	return &TextureView{r: rv, device: st.surface.device, texture: st.texture}, nil
}

// Texture returns the underlying Texture.
func (st *SurfaceTexture) Texture() hal.Texture {
	return st.texture
}

// Release drops ownership of the surface texture returned by GetCurrentTexture.
// webgpu.h marks WGPUSurfaceTexture.texture as ReturnedWithOwnership — callers
// must release exactly once after Present or discard.
func (st *SurfaceTexture) Release() {
	if st == nil {
		return
	}
	if st.surface != nil && st.surface.current == st {
		st.surface.current = nil
	}
	if st.texture != nil {
		st.texture.Release()
		st.texture = nil
	}
	st.r = nil
}

// Destroy implements hal.SurfaceTexture: same as Release.
func (st *SurfaceTexture) Destroy() { st.Release() }

func (st *SurfaceTexture) NativeHandle() uintptr { return 0 }

// Format implements hal.Texture: delegates to underlying texture.
func (st *SurfaceTexture) Format() TextureFormat {
	if st.texture != nil {
		return st.texture.Format()
	}
	return types.TextureFormatUndefined
}

func (st *SurfaceTexture) CurrentUsage() TextureUsage { return 0 }

func (st *SurfaceTexture) AddPendingRef() {}

func (st *SurfaceTexture) DecPendingRef() {}

var _ hal.SurfaceTexture = (*SurfaceTexture)(nil)

var _ hal.Surface = (*Surface)(nil)
