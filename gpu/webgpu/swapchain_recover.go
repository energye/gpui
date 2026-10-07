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
	"os"
	"time"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
	"github.com/energye/gpui/gpu/types"
)

// EnableAutoRecover recreates the device after DeviceLostCallback (optional).
// onRecreated rebinds app GPU state; adapter must outlive the swapchain.
//
// Optional: set sc.OnDeviceAbandon before EnableAutoRecover so the host can
// drop GPUShared/session resources while the old device is still addressable.
// tryRecover then Destroy/Release's the old device *before* RequestDevice
// Adapter/callback device params use hal interfaces (7f Device→hal.Device).
func (sc *Swapchain) EnableAutoRecover(adapter hal.Adapter, deviceLabel string, onRecreated func(hal.Device)) {
	if sc == nil {
		return
	}
	sc.RecoveryAdapter = adapter
	sc.DeviceLabel = deviceLabel
	sc.OnDeviceRecreated = onRecreated
	if sc.recoverCooldown <= 0 {
		sc.recoverCooldown = time.Second
	}
}

// Recoveries returns how many successful device recreations have completed.
// Recoveries returns how many successful device recreations have completed.
func (sc *Swapchain) Recoveries() uint64 {
	if sc == nil {
		return 0
	}
	return sc.recoverAttempts
}

// ClearRecoverCooldown allows the next tryRecoverDeviceLocked to run immediately
// (e.g. when the window becomes visible again after occlusion device-lost).
// ClearRecoverCooldown allows the next tryRecoverDeviceLocked to run immediately
// (e.g. when the window becomes visible again after occlusion device-lost).
func (sc *Swapchain) ClearRecoverCooldown() {
	if sc == nil {
		return
	}
	sc.frameMu.Lock()
	sc.lastRecoverAt = time.Time{}
	sc.frameMu.Unlock()
}

// Stats returns cumulative present-path counters.
// PresentModeForVsync picks the present mode for a runtime vsync switch from
// the modes cached at ConfigureFromCapabilities. on=true → Fifo (steady UI,
// no tearing); on=false → Mailbox (latest-frame-wins, no tearing), falling
// back to FifoRelaxed/Immediate per availability — the low-latency path used
// while an interactive resize storm is active so content tracks the window
// instead of waiting one Fifo vblank (~16.5ms) per frame.
func (sc *Swapchain) PresentModeForVsync(on bool) PresentMode {
	if on {
		return PresentModeFifo
	}
	for _, m := range []PresentMode{PresentModeMailbox, PresentModeFifoRelaxed, PresentModeImmediate} {
		for _, s := range sc.supportedPresentModes {
			if s == m {
				return m
			}
		}
	}
	return PresentModeFifo
}

// SetSupportedPresentModesForTest overrides the cached surface-capability
// list (test-only; ConfigureFromCapabilities normally populates it from the
// adapter). Used to unit-test PresentModeForVsync without a real surface.
func (sc *Swapchain) deviceKnownLostLocked() bool {
	return sc != nil && sc.Device != nil && sc.Device.IsLost()
}

// requestDeviceWithVRAMProbe requests a device and verifies a 1x1 texture
// allocation succeeds. Retries when the previous device heap is slow to reclaim
// after Unconfigure + heavy GPU work (portable across vendors/sizes).
// Returns hal.Device (7f Device→hal.Device).
// requestDeviceWithVRAMProbe requests a device and verifies a 1x1 texture
// allocation succeeds. Retries when the previous device heap is slow to reclaim
// after Unconfigure + heavy GPU work (portable across vendors/sizes).
// Returns hal.Device (7f Device→hal.Device).
func (sc *Swapchain) requestDeviceWithVRAMProbe(label string, lim *types.Limits) (hal.Device, error) {
	if sc == nil || sc.RecoveryAdapter == nil {
		return nil, fmt.Errorf("wgpu: requestDeviceWithVRAMProbe: nil swapchain/adapter")
	}
	var inst *Instance
	if sc.Surface != nil {
		inst = sc.Surface.instance
	}
	// Prefer instance from recovery path via adapter's stored instance if needed.
	var last error
	for attempt := 0; attempt < 5; attempt++ {
		devDesc := &hal.DeviceDescriptor{Label: label, RequiredLimits: types.DefaultLimits()}
		if lim != nil {
			devDesc.RequiredLimits = *lim
		} else {
			l := devDesc.RequiredLimits
			l.MaxBufferSize = 64 * 1024 * 1024
			l.MaxStorageBufferBindingSize = 32 * 1024 * 1024
			if l.MaxStorageBuffersPerShaderStage < 9 {
				l.MaxStorageBuffersPerShaderStage = 9
			}
			devDesc.RequiredLimits = l
		}
		dev, err := sc.RecoveryAdapter.RequestDevice(devDesc)
		if err != nil {
			last = err
		} else {
			tex, err2 := dev.CreateTexture(&hal.TextureDescriptor{
				Label:         "recover_vram_probe",
				Size:          hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
				MipLevelCount: 1,
				SampleCount:   1,
				Dimension:     types.TextureDimension2D,
				Format:        types.TextureFormatBGRA8Unorm,
				Usage:         types.TextureUsageRenderAttachment,
			})
			if err2 == nil {
				tex.Destroy()
				return dev, nil
			}
			last = err2
			dev.Release()
		}
		if inst != nil {
			inst.ProcessEvents()
		}
		time.Sleep(time.Duration(25*(attempt+1)) * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("requestDeviceWithVRAMProbe: exhausted retries")
	}
	return nil, last
}

// ForceRecoverHealthy abandons the current device while it is still healthy
// (native Release reclaims VRAM), then RequestDevice + recreate surface + Configure.
// Used by hosts that can tear down before sticky-lost (force-lost proof, scheduled
// context reset). Prefer over MarkLost alone when the .so leaves VRAM pinned after
// Release-on-lost of a fully-loaded UI device.
// ForceRecoverHealthy abandons the current device while it is still healthy
// (native Release reclaims VRAM), then RequestDevice + recreate surface + Configure.
// Used by hosts that can tear down before sticky-lost (force-lost proof, scheduled
// context reset). Prefer over MarkLost alone when the .so leaves VRAM pinned after
// Release-on-lost of a fully-loaded UI device.
func (sc *Swapchain) ForceRecoverHealthy() error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if sc.RecoveryAdapter == nil {
		return fmt.Errorf("wgpu: ForceRecoverHealthy requires RecoveryAdapter")
	}
	sc.frameMu.Lock()
	defer sc.frameMu.Unlock()
	sc.configured = false
	sc.frameOpen = false
	sc.pendingReconfigure = true

	oldDev := sc.Device
	oldSurf := sc.Surface
	var disp, win uintptr
	var inst *Instance
	if oldSurf != nil {
		disp, win = oldSurf.displayHandle, oldSurf.windowHandle
		inst = oldSurf.instance
	}
	canRecreateSurf := inst != nil && win != 0

	// Engine + host abandon.
	if BeforeDeviceRecover != nil {
		BeforeDeviceRecover()
	}
	if oldDev != nil && sc.OnDeviceAbandon != nil {
		sc.OnDeviceAbandon(oldDev)
	}
	if oldDev != nil && !oldDev.IsLost() {
		_ = oldDev.WaitIdle()
		oldDev.FlushCallbacks()
	}
	if oldSurf != nil {
		oldSurf.DiscardTexture(nil)
		if oldDev == nil || !oldDev.IsLost() {
			// Healthy Unconfigure frees swapchain images cleanly.
			oldSurf.Unconfigure(nil)
		}
		if canRecreateSurf {
			oldSurf.Release()
		}
	}
	if oldDev != nil {
		// Healthy Release (lost=false) — measured path that reclaims VRAM.
		if !oldDev.IsLost() {
			oldDev.Release()
		} else {
			rwgpu.WithForceNativeReleaseOnLost(func() { oldDev.Release() })
		}
	}
	sc.Device = nil
	if canRecreateSurf {
		sc.Surface = nil
	}
	if os.Getenv("GPUI_DEBUG_GPU") == "1" {
		fmt.Printf("ForceRecoverHealthy: cmdBufLive=%d\n", rwgpu.CmdBufLive())
		if leak := rwgpu.ReportLeaks(); leak != nil && leak.Count > 0 {
			fmt.Printf("ForceRecoverHealthy: LEAK after abandon+Release: %s\n", leak.String())
		} else {
			fmt.Printf("ForceRecoverHealthy: no tracked leaks after abandon+Release\n")
		}
	}
	if inst != nil {
		inst.ProcessEvents()
	}
	// Allow driver to reclaim heaps after last ref drop (portable across GPUs).
	for i := 0; i < 3; i++ {
		if inst != nil {
			inst.ProcessEvents()
		}
		time.Sleep(20 * time.Millisecond)
	}

	label := sc.DeviceLabel
	if label == "" {
		label = "gpui-recovered-device"
	}
	dev, err := sc.requestDeviceWithVRAMProbe(label, nil)
	if err != nil {
		return fmt.Errorf("ForceRecoverHealthy RequestDevice: %w", err)
	}
	sc.Device = dev
	if canRecreateSurf {
		ns, err := inst.CreateSurfaceFromHandles(disp, win)
		if err != nil {
			return fmt.Errorf("ForceRecoverHealthy CreateSurface: %w", err)
		}
		sc.Surface = ns
	} else if sc.Surface == nil {
		return fmt.Errorf("ForceRecoverHealthy: surface nil")
	}
	if sc.OnDeviceRecreated != nil {
		sc.OnDeviceRecreated(dev)
	}
	if err := sc.ConfigureFromCapabilities(sc.RecoveryAdapter); err != nil {
		if err2 := sc.Configure(); err2 != nil {
			return fmt.Errorf("ForceRecoverHealthy configure: %v (caps: %v)", err2, err)
		}
	}
	sc.recoverAttempts++
	sc.recoverGrace = 12
	sc.lastRecoverAt = time.Now()
	return nil
}

// tryRecoverDeviceLocked creates a new device and reconfigures the surface.
// Caller holds frameMu. Requires RecoveryAdapter; rate-limited by recoverCooldown.
//
// Measured on libwgpu_native (940MX):
//   - wgpuDeviceDestroy → permanent CreateTexture/RequestDevice OOM
//   - Unconfigure-after-lost + Configure same surface → SIGSEGV
//   - Force-Release surface+device children, recreate Surface, Release-only old device → OK
//
// Skia/Flutter: abandon all resources, drop context, recreate surface+device.
// tryRecoverDeviceLocked creates a new device and reconfigures the surface.
// Caller holds frameMu. Requires RecoveryAdapter; rate-limited by recoverCooldown.
//
// Measured on libwgpu_native (940MX):
//   - wgpuDeviceDestroy → permanent CreateTexture/RequestDevice OOM
//   - Unconfigure-after-lost + Configure same surface → SIGSEGV
//   - Force-Release surface+device children, recreate Surface, Release-only old device → OK
//
// Skia/Flutter: abandon all resources, drop context, recreate surface+device.
func (sc *Swapchain) tryRecoverDeviceLocked() error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if sc.RecoveryAdapter == nil {
		return hal.ErrDeviceLost
	}
	if sc.recoverCooldown <= 0 {
		sc.recoverCooldown = time.Second
	}
	if !sc.lastRecoverAt.IsZero() && time.Since(sc.lastRecoverAt) < sc.recoverCooldown {
		return fmt.Errorf("%w: recovery rate-limited", hal.ErrDeviceLost)
	}
	sc.lastRecoverAt = time.Now()

	sc.configured = false
	sc.frameOpen = false
	sc.pendingReconfigure = true

	oldDev := sc.Device
	oldSurf := sc.Surface
	var disp, win uintptr
	var inst *Instance
	if oldSurf != nil {
		disp, win = oldSurf.displayHandle, oldSurf.windowHandle
		inst = oldSurf.instance
	}
	// Always drop+recreate Surface when platform handles are known.
	// Measured on this libwgpu_native:
	//   - Unconfigure-after-lost + Configure same surface → SIGSEGV
	//   - Unconfigure-while-healthy then MarkLost + Configure same surface →
	//     SIGSEGV / CreateTexture OOM on next device (UI force-lost path)
	//   - Release surface + CreateSurface + Configure new device → OK
	canRecreateSurf := inst != nil && win != 0

	rwgpu.WithForceNativeReleaseOnLost(func() {
		if oldDev != nil {
			if sc.OnDeviceAbandon != nil {
				sc.OnDeviceAbandon(oldDev) // sessions + GPUShared under force
			}
			if !oldDev.IsLost() {
				_ = oldDev.WaitIdle()
			}
			oldDev.FlushCallbacks()
		}
		if oldSurf != nil {
			oldSurf.DiscardTexture(nil)
			// Never Unconfigure-when-lost (SIGSEGV). Always Release when we can recreate.
			if canRecreateSurf {
				oldSurf.Release()
			}
		}
		if oldDev != nil {
			// Release only — no wgpuDeviceDestroy (measured permanent OOM).
			oldDev.Release()
		}
	})
	sc.Device = nil
	if canRecreateSurf {
		sc.Surface = nil
	}
	if leak := rwgpu.ReportLeaks(); leak != nil && leak.Count > 0 {
		fmt.Printf("ForceRecoverHealthy: LEAK after abandon+Release: %s\n", leak.String())
	} else {
		fmt.Printf("ForceRecoverHealthy: no tracked leaks after abandon+Release\n")
	}
	if inst != nil {
		inst.ProcessEvents()
	}
	time.Sleep(50 * time.Millisecond)

	// Let the driver reclaim heaps before the next RequestDevice. Without this,
	// dual-device peak on 1GB cards OOMs CreateTexture (session_depth_stencil).
	if inst != nil {
		inst.ProcessEvents()
	}
	// Allow driver to reclaim heaps after last ref drop (portable across GPUs).
	for i := 0; i < 3; i++ {
		if inst != nil {
			inst.ProcessEvents()
		}
		time.Sleep(20 * time.Millisecond)
	}

	label := sc.DeviceLabel
	if label == "" {
		label = "gpui-recovered-device"
	}
	dev, err := sc.requestDeviceWithVRAMProbe(label, nil)
	if err != nil {
		return fmt.Errorf("%w: RequestDevice: %v", hal.ErrDeviceLost, err)
	}
	sc.Device = dev

	if canRecreateSurf {
		ns, err := inst.CreateSurfaceFromHandles(disp, win)
		if err != nil {
			return fmt.Errorf("%w: recreate surface: %v", hal.ErrDeviceLost, err)
		}
		sc.Surface = ns
	} else if sc.Surface == nil {
		return fmt.Errorf("%w: surface is nil after recover", hal.ErrDeviceLost)
	}

	if sc.OnDeviceRecreated != nil {
		sc.OnDeviceRecreated(dev)
	}

	if err := sc.ConfigureFromCapabilities(sc.RecoveryAdapter); err != nil {
		if err2 := sc.Configure(); err2 != nil {
			return fmt.Errorf("%w: reconfigure: %v (caps: %v)", hal.ErrDeviceLost, err2, err)
		}
	}
	sc.recoverAttempts++
	sc.recoverGrace = 12
	return nil
}

// ensureDeviceLocked recovers a lost device when EnableAutoRecover is armed.
// No-op when the device is healthy. Caller must hold frameMu.
// ensureDeviceLocked recovers a lost device when EnableAutoRecover is armed.
// No-op when the device is healthy. Caller must hold frameMu.
func (sc *Swapchain) ensureDeviceLocked() error {
	if sc == nil || !sc.deviceKnownLostLocked() {
		return nil
	}
	if sc.RecoveryAdapter == nil {
		return hal.ErrDeviceLost
	}
	return sc.tryRecoverDeviceLocked()
}

// takeRecoverGrace returns ErrRecovered while post-recover settle frames remain.
// Must run after every ensureDeviceLocked that may have recovered — including the
// GetCurrentTexture lost-retry path (user OOM: recover then same-frame Present).
// takeRecoverGrace returns ErrRecovered while post-recover settle frames remain.
// Must run after every ensureDeviceLocked that may have recovered — including the
// GetCurrentTexture lost-retry path (user OOM: recover then same-frame Present).
func (sc *Swapchain) takeRecoverGrace() error {
	if sc != nil && sc.recoverGrace > 0 {
		sc.recoverGrace--
		return ErrRecovered
	}
	return nil
}

// PresentModeName returns a short label for the active present mode.
func (sc *Swapchain) PresentModeName() string {
	if sc == nil {
		return "nil"
	}
	switch sc.PresentMode {
	case PresentModeFifo:
		return "fifo"
	case PresentModeFifoRelaxed:
		return "fifo-relaxed"
	case PresentModeMailbox:
		return "mailbox"
	case PresentModeImmediate:
		return "immediate"
	default:
		return fmt.Sprintf("mode(%d)", int(sc.PresentMode))
	}
}

// Release unconfigures the surface; does not release Surface/Device ownership.
