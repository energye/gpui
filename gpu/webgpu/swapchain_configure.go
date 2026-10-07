//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package webgpu

import (
	"fmt"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// NewHalSwapchain builds the hal.Swapchain view of this swapchain so render
// talks hal only.
func (sc *Swapchain) NewHalSwapchain() hal.Swapchain {
	return &halSwapchainAdapter{sc: sc}
}

// NewSwapchain builds a swapchain for an existing surface + device.
// Call Configure before BeginFrame.
// Implements hal-facing flow (device stored as hal.Device interface).
// NewSwapchain builds a swapchain for an existing surface + device.
// Call Configure before BeginFrame.
// Implements hal-facing flow (device stored as hal.Device interface).
func NewSwapchain(surface hal.Surface, device hal.Device, width, height uint32) *Swapchain {
	ws, _ := surface.(*Surface)
	return &Swapchain{
		Surface:         ws,
		Device:          device,
		Width:           width,
		Height:          height,
		Format:          types.TextureFormatBGRA8Unorm,
		Usage:           types.TextureUsageRenderAttachment,
		PresentMode:     PresentModeFifo,
		AlphaMode:       types.CompositeAlphaModeOpaque,
		recoverCooldown: time.Second,
	}
}

// EnableAutoRecover recreates the device after DeviceLostCallback (optional).
// onRecreated rebinds app GPU state; adapter must outlive the swapchain.
//
// Optional: set sc.OnDeviceAbandon before EnableAutoRecover so the host can
// drop GPUShared/session resources while the old device is still addressable.
// tryRecover then Destroy/Release's the old device *before* RequestDevice
// Adapter/callback device params use hal interfaces (7f Device→hal.Device).
// SetPreferVSync selects Fifo when available (default production UI path).
func (sc *Swapchain) SetPreferVSync() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []PresentMode{PresentModeFifo, PresentModeFifoRelaxed, PresentModeMailbox, PresentModeImmediate}
}

// SetPreferFifoRelaxed selects the non-blocking but vsync-preferring mode
// (ENGINE_FRAME_PRESENT_STANDARD.md 块3): FifoRelaxed waits for the vblank
// when the frame is ready in time and submits immediately when late — the
// steady Wayland mode, where the compositor's vblank swap guarantees no
// tearing and a client-side Fifo wait would only block. Mailbox/Immediate
// are fallbacks per availability.
// SetPreferFifoRelaxed selects the non-blocking but vsync-preferring mode
// (ENGINE_FRAME_PRESENT_STANDARD.md 块3): FifoRelaxed waits for the vblank
// when the frame is ready in time and submits immediately when late — the
// steady Wayland mode, where the compositor's vblank swap guarantees no
// tearing and a client-side Fifo wait would only block. Mailbox/Immediate
// are fallbacks per availability.
func (sc *Swapchain) SetPreferFifoRelaxed() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []PresentMode{PresentModeFifoRelaxed, PresentModeMailbox, PresentModeImmediate}
}

// SetPreferLowLatency prefers Mailbox/Immediate when supported (gamesing; may tear).
// SetPreferLowLatency prefers Mailbox/Immediate when supported (gamesing; may tear).
func (sc *Swapchain) SetPreferLowLatency() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []PresentMode{PresentModeMailbox, PresentModeFifoRelaxed, PresentModeFifo, PresentModeImmediate}
}

// Configure applies SurfaceConfiguration to the underlying surface.
// Prefer choosing Format/PresentMode from Adapter.GetSurfaceCapabilities when available.
// Configure applies SurfaceConfiguration to the underlying surface.
// Prefer choosing Format/PresentMode from Adapter.GetSurfaceCapabilities when available.
func (sc *Swapchain) Configure() error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if sc.Surface == nil {
		return fmt.Errorf("wgpu: swapchain surface is nil")
	}
	if sc.Device == nil {
		return fmt.Errorf("wgpu: swapchain device is nil")
	}
	if sc.Width == 0 || sc.Height == 0 {
		return fmt.Errorf("wgpu: swapchain extent must be non-zero")
	}
	if sc.Usage == 0 {
		sc.Usage = types.TextureUsageRenderAttachment
	}
	if sc.Format == 0 {
		sc.Format = types.TextureFormatBGRA8Unorm
	}
	if sc.PresentMode == 0 {
		sc.PresentMode = PresentModeFifo
	}
	if sc.AlphaMode == 0 {
		sc.AlphaMode = types.CompositeAlphaModeOpaque
	}
	cfg := &SurfaceConfiguration{
		Width:       sc.Width,
		Height:      sc.Height,
		Format:      sc.Format,
		Usage:       sc.Usage,
		PresentMode: sc.PresentMode,
		AlphaMode:   sc.AlphaMode,
	}
	if err := sc.Surface.Configure(sc.Device, cfg); err != nil {
		return err
	}
	sc.configured = true
	sc.pendingReconfigure = false
	sc.suboptHandledW = sc.Width
	sc.suboptHandledH = sc.Height
	sc.reconfigures++
	return nil
}

// ConfigureFromCapabilities picks a supported format/present/alpha mode then configures.
// ConfigureFromCapabilities picks a supported format/present/alpha mode then configures.
func (sc *Swapchain) ConfigureFromCapabilities(adapter hal.Adapter) error {
	if sc == nil || adapter == nil {
		return fmt.Errorf("wgpu: swapchain/adapter nil")
	}
	wa, ok := adapter.(*Adapter)
	if !ok || wa == nil {
		return fmt.Errorf("wgpu: ConfigureFromCapabilities requires *Adapter")
	}
	caps := wa.GetSurfaceCapabilities(sc.Surface)
	if caps != nil {
		if len(caps.Formats) > 0 {
			sc.Format = caps.Formats[0]
			// Prefer BGRA8Unorm when listed (common on Windows/Linux).
			for _, f := range caps.Formats {
				if f == types.TextureFormatBGRA8Unorm || f == types.TextureFormatRGBA8Unorm {
					sc.Format = f
					break
				}
			}
		}
		sc.PresentMode = pickPresentMode(caps.PresentModes, sc.PreferPresentModes)
		sc.supportedPresentModes = append([]PresentMode(nil), caps.PresentModes...)
		if len(caps.AlphaModes) > 0 {
			sc.AlphaMode = caps.AlphaModes[0]
			for _, am := range caps.AlphaModes {
				if am == types.CompositeAlphaModeOpaque {
					sc.AlphaMode = am
					break
				}
			}
		}
	}
	return sc.Configure()
}

func pickPresentMode(available []PresentMode, prefer []PresentMode) PresentMode {
	if len(available) == 0 {
		return PresentModeFifo
	}
	has := func(m PresentMode) bool {
		for _, a := range available {
			if a == m {
				return true
			}
		}
		return false
	}
	// Explicit preference list.
	for _, p := range prefer {
		if has(p) {
			return p
		}
	}
	// Default: Fifo (vsync) for steady UI.
	if has(PresentModeFifo) {
		return PresentModeFifo
	}
	return available[0]
}

// Resize updates extent and reconfigures when the size actually changes.
// Same-size calls are no-ops to avoid Surface.Configure thrash (black flash).
//
// Callers should draw + Present a full frame immediately after a successful
// size-changing Resize (Skia/Flutter: first frame after surface recreate must
// be complete before the compositor samples it).
// Resize updates extent and reconfigures when the size actually changes.
// Same-size calls are no-ops to avoid Surface.Configure thrash (black flash).
//
// Callers should draw + Present a full frame immediately after a successful
// size-changing Resize (Skia/Flutter: first frame after surface recreate must
// be complete before the compositor samples it).
func (sc *Swapchain) Resize(width, height uint32) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if width == 0 || height == 0 {
		return fmt.Errorf("wgpu: swapchain extent must be non-zero")
	}
	if sc.Width == width && sc.Height == height && sc.configured && !sc.pendingReconfigure {
		return nil
	}
	sc.Width = width
	sc.Height = height
	// Configure() marks suboptHandled for this extent so the first suboptimal
	// present after resize does not immediately reconfigure again (double flash).
	return sc.Configure()
}

// PresentModeForVsync picks the present mode for a runtime vsync switch from
// the modes cached at ConfigureFromCapabilities. on=true → Fifo (steady UI,
// no tearing); on=false → Mailbox (latest-frame-wins, no tearing), falling
// back to FifoRelaxed/Immediate per availability — the low-latency path used
// while an interactive resize storm is active so content tracks the window
// instead of waiting one Fifo vblank (~16.5ms) per frame.
// SetPresentModeForce reconfigures the surface with an explicit present mode
// even when the extent is unchanged (Resize no-ops same-size calls). Used by
// the runtime vsync switch (Fifo steady ↔ Mailbox/Immediate during resize).
// A reconfigure invalidates the swapchain buffers — the caller owes full
// frames until each buffer is fully written.
func (sc *Swapchain) SetPresentModeForce(mode PresentMode) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if mode == 0 {
		mode = PresentModeFifo
	}
	if sc.PresentMode == mode && sc.configured && !sc.pendingReconfigure {
		return nil
	}
	sc.PresentMode = mode
	sc.pendingReconfigure = true
	return sc.Configure()
}

// MarkNeedsReconfigure schedules a reconfigure on the next BeginFrame.
// Call after window resize events or when the compositor reports outdated.
// MarkNeedsReconfigure schedules a reconfigure on the next BeginFrame.
// Call after window resize events or when the compositor reports outdated.
func (sc *Swapchain) MarkNeedsReconfigure() {
	if sc != nil {
		sc.pendingReconfigure = true
	}
}

// BeginFrame acquires the next surface texture and creates a render view.
// Caller must EndFrame (or DiscardFrame) exactly once per successful BeginFrame.
//
// Error policy:
//   - DeviceLost + EnableAutoRecover → recreate device and retry
//   - DeviceLost without recovery → hal.ErrDeviceLost (no native abort)
//   - Occluded / Timeout → skip frame
//   - Outdated / other → reconfigure once and retry
// Window visibility policy is host-side, not here.
