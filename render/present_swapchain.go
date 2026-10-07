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
	"errors"
	"fmt"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"os"
	"time"
)

// newWebgpuHalSwapchain builds the default hal.Swapchain.
func newWebgpuHalSwapchain(surf hal.Surface, dev hal.Device, w, h uint32) hal.Swapchain {
	return webgpu.NewSwapchain(surf, dev, w, h).NewHalSwapchain()
}

// newPresentSwapchain picks the backend once (creation sentence) and
// returns its hal.Swapchain; render talks hal afterwards. Callers own
// cleanup on error (their Surface/Device lifetimes differ).
// It resolves the same way as the target constructor, so env-wins keeps
// both consistent without threading state between them.
func newPresentSwapchain(surf hal.Surface, dev hal.Device, w, h uint32) (hal.Swapchain, error) {
	want, err := ResolveBackend()
	if err != nil {
		return nil, err
	}
	if want == BackendGo {
		return newGLHalSwapchain(surf, dev, w, h)
	}
	return newWebgpuHalSwapchain(surf, dev, w, h), nil
}

// presentLevels orders downgrade attempts from the resolved policy downward.
// The first level is the 1.2 behavior; lower levels only run after an
// OOM-class failure, so resource-sufficient opens follow the old path.
func presentLevels(start AdapterPolicy) []presentLevel {
	levels := []presentLevel{{name: start.String(), policy: start}}
	if start != PolicyLow {
		levels = append(levels, presentLevel{name: PolicyLow.String(), policy: PolicyLow})
	}
	return append(levels, presentLevel{name: "software", software: true})
}

// NewPresentTarget creates a GPU present path from native window handles.
// logicalW/H are layout pixels; scale is device pixel ratio (≥1).
// The GPU implementation follows GPUI_BACKEND (see ResolveBackend).
// peekShared snapshots the share state (borrowable = device published).
func peekShared() (inst hal.Instance, adapter hal.Adapter, device hal.Device, borrowable bool, refs int) {
	shareMu.Lock()
	defer shareMu.Unlock()
	return shareInst, shareAdapter, shareDevice, shareDevice != nil, shareRefs
}

// buildSharedSurface opens a window on the borrowed share: own surface +
// swapchain on the shared device. Non-OOM errors return directly; OOM-class
// errors return for the downgrade chain to try a lower level.
// buildSharedSurface opens a window on the borrowed share: own surface +
// swapchain on the shared device. Non-OOM errors return directly; OOM-class
// errors return for the downgrade chain to try a lower level.
func buildSharedSurface(ns PresentNativeSurface, logicalW, logicalH int, scale float64) (*PresentTarget, error) {
	// R0-3: pin a share ref across the slow ops below. Snapshotting the
	// pointers without a ref lets the owner Close free them mid-build, so
	// the borrow continues on a dead device and then resurrects the count.
	shareMu.Lock()
	inst, device := shareInst, shareDevice
	if device == nil || inst == nil {
		shareMu.Unlock()
		return nil, errors.New("render: shared present device not open")
	}
	adapter := shareAdapter
	shareRefs++
	shareMu.Unlock()
	// dropPin releases the pin taken above; success paths keep it as the
	// borrower's own ref.
	dropPin := func() {
		shareMu.Lock()
		shareRefs--
		shareMu.Unlock()
	}
	backend := surfaceBackendFor(ns.Platform)
	surf, err := inst.CreateSurface(surfaceTargetFor(ns))
	if err != nil {
		dropPin()
		return nil, fmt.Errorf("render: CreateSurface(%s): %w", backend, err)
	}
	physW, physH := physicalSize(logicalW, logicalH, scale)
	sc, err := newPresentSwapchain(surf, device, physW, physH)
	if err != nil {
		surf.Destroy()
		dropPin()
		return nil, err
	}
	sc.SetUsage(types.TextureUsageRenderAttachment | types.TextureUsageCopyDst)
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		surf.Destroy()
		dropPin()
		return nil, fmt.Errorf("render: ConfigureFromCapabilities: %w", err)
	}
	// Defensive: the share must still be the chain we pinned. With the pin
	// held the owner cannot free it, so any mismatch is a logic error —
	// fail closed instead of counting a dead device.
	shareMu.Lock()
	if shareDevice != device || shareInst != inst {
		shareRefs--
		shareMu.Unlock()
		sc.Release()
		surf.Destroy()
		return nil, fmt.Errorf("render: shared present device changed during open: %w", errSharedNotOpen)
	}
	shareMu.Unlock()
	// Same device as the share: the pin above is this borrower's refcount.
	// Do NOT rebind the
	// global accelerator provider here: the rebind invalidates every
	// live GPU session (abandonAllContextGPU) and a concurrent flush on
	// another window's raster thread rebuilds its session against the
	// same device anyway (deviceGen unchanged → no rebuild at all).
	// Rebinding with the identical device is a no-op for correctness
	// and a crash vector for concurrency (two sessions racing
	// ensureTexturesForView on the post-abandon rebuild → nil pass).
	dc := NewContext(logicalW, logicalH, WithDeviceScale(scale))
	t := &PresentTarget{
		ns:                ns,
		logicW:            logicalW,
		logicH:            logicalH,
		scale:             scale,
		inst:              inst,
		adapter:           adapter,
		device:            device,
		shared:            true,
		surf:              surf,
		sc:                sc,
		dc:                dc,
		resizeStormWindow: 300 * time.Millisecond,
		vsyncOn:           true,
	}
	// R0-6: track every live target so a shared recovery can reconfigure
	// all swapchains exactly once (the pin above is this borrower's ref).
	shareMu.Lock()
	registerSharedTarget(t)
	shareMu.Unlock()
	return t, nil
}

func surfaceBackendFor(p PresentPlatform) string {
	switch p {
	case PresentPlatformWayland:
		return "wayland"
	case PresentPlatformWin32:
		return "win32"
	case PresentPlatformAppKit:
		return "metal"
	default:
		return "xlib"
	}
}

// surfaceTargetFor maps a present platform + raw handles to a hal.SurfaceTarget.
// Kind selects the window-system path; handles stay caller-owned.
// surfaceTargetFor maps a present platform + raw handles to a hal.SurfaceTarget.
// Kind selects the window-system path; handles stay caller-owned.
func surfaceTargetFor(ns PresentNativeSurface) hal.SurfaceTarget {
	var kind hal.SurfaceTargetKind
	switch ns.Platform {
	case PresentPlatformWayland:
		kind = hal.SurfaceTargetWaylandSurface
	case PresentPlatformWin32:
		kind = hal.SurfaceTargetWindowsHWND
	case PresentPlatformAppKit:
		kind = hal.SurfaceTargetMetalLayer
	default:
		kind = hal.SurfaceTargetXlibWindow
	}
	return hal.SurfaceTarget{Kind: kind, DisplayHandle: ns.Display, WindowHandle: ns.Window}
}

func physicalSize(logicalW, logicalH int, scale float64) (uint32, uint32) {
	pw := int(float64(logicalW)*scale + 0.5)
	ph := int(float64(logicalH)*scale + 0.5)
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	return uint32(pw), uint32(ph)
}

// Context returns the drawing context (logical size, device scale applied).
// Context returns the drawing context (logical size, device scale applied).
func (t *PresentTarget) Context() *Context {
	if t == nil {
		return nil
	}
	return t.dc
}

// LogicalSize returns layout size in logical pixels.
// LogicalSize returns layout size in logical pixels.
func (t *PresentTarget) LogicalSize() (w, h int) {
	if t == nil {
		return 0, 0
	}
	return t.logicW, t.logicH
}

// Scale returns device pixel ratio.
// Scale returns device pixel ratio.
func (t *PresentTarget) Scale() float64 {
	if t == nil || t.scale <= 0 {
		return 1
	}
	return t.scale
}

// SetResizeStormWindow configures the storm window used by InFullRecovery /
// the full-path forcing (see resizeStormWindow). <=0 disables storm awareness
// (fixed postResizeFull budget only). Tests shrink the window to exercise the
// storm boundary without waiting.
// SetResizeStormWindow configures the storm window used by InFullRecovery /
// the full-path forcing (see resizeStormWindow). <=0 disables storm awareness
// (fixed postResizeFull budget only). Tests shrink the window to exercise the
// storm boundary without waiting.
func (t *PresentTarget) SetResizeStormWindow(d time.Duration) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.resizeStormWindow = d
}

// Resize records a new logical size / scale. Record-only (T3): the UI thread
// only stores the size and arms swapchainPending; the drawing Context sync
// (dc.Resize/SetDeviceScale) and the wgpu swapchain reconfigure both run on
// the raster thread at the present boundary (applyPendingSwapchainLocked),
// serialized with BeginFrame/EndFrame via mu. Same-size calls are no-ops.
// This keeps render.Context raster-exclusive: the UI thread never blocks on
// the surface and never touches dc off raster (T3 render-owns-raster).
// Resize records a new logical size / scale. Record-only (T3): the UI thread
// only stores the size and arms swapchainPending; the drawing Context sync
// (dc.Resize/SetDeviceScale) and the wgpu swapchain reconfigure both run on
// the raster thread at the present boundary (applyPendingSwapchainLocked),
// serialized with BeginFrame/EndFrame via mu. Same-size calls are no-ops.
// This keeps render.Context raster-exclusive: the UI thread never blocks on
// the surface and never touches dc off raster (T3 render-owns-raster).
func (t *PresentTarget) Resize(logicalW, logicalH int, scale float64) error {
	if t == nil {
		return errors.New("render: nil PresentTarget")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("render: PresentTarget closed")
	}
	if logicalW < 1 {
		logicalW = 1
	}
	if logicalH < 1 {
		logicalH = 1
	}
	if scale <= 0 {
		scale = 1
	}
	if t.logicW == logicalW && t.logicH == logicalH && t.scale == scale {
		return nil
	}
	t.logicW, t.logicH, t.scale = logicalW, logicalH, scale
	t.swapchainPending = true
	return nil
}

// SetOnSwapchainResized registers a callback fired on the raster thread
// (inside the present critical section) immediately after the swapchain has
// been reconfigured to a new logical size, before the first buffer of that
// size is committed. The Wayland host uses it to declare xdg window geometry
// in the same wire batch as the new-size buffer. A nil callback is a no-op.
// SetOnSwapchainResized registers a callback fired on the raster thread
// (inside the present critical section) immediately after the swapchain has
// been reconfigured to a new logical size, before the first buffer of that
// size is committed. The Wayland host uses it to declare xdg window geometry
// in the same wire batch as the new-size buffer. A nil callback is a no-op.
func (t *PresentTarget) SetOnSwapchainResized(fn func(logicalW, logicalH int)) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onSwapchainResized = fn
}

// SetVsync switches the swapchain present mode at the next present boundary
// (raster thread, serialized with BeginFrame/EndFrame): true = Fifo (vsync,
// steady UI), false = Mailbox/Immediate (low-latency — used while an
// interactive resize storm is active so content tracks the window instead of
// waiting one Fifo vblank per frame). Record-only, like Resize: the UI
// thread never blocks on the surface. The swapchain buffers are undefined
// after a mode switch, so the switch arms the post-resize full-write budget.
// SetVsync switches the swapchain present mode at the next present boundary
// (raster thread, serialized with BeginFrame/EndFrame): true = Fifo (vsync,
// steady UI), false = Mailbox/Immediate (low-latency — used while an
// interactive resize storm is active so content tracks the window instead of
// waiting one Fifo vblank per frame). Record-only, like Resize: the UI
// thread never blocks on the surface. The swapchain buffers are undefined
// after a mode switch, so the switch arms the post-resize full-write budget.
func (t *PresentTarget) SetVsync(vsync bool) {
	if t == nil || t.sc == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	// fixedNoVsync platforms (Wayland): present is permanently non-blocking
	// (compositor vblank swap; ENGINE_FRAME_PRESENT_STANDARD.md 块3), so the
	// Fifo↔low-latency switch is a no-op — the storm path already submits
	// without waiting.
	if t.fixedNoVsync {
		return
	}
	if t.vsyncOn == vsync {
		return
	}
	t.vsyncOn = vsync
	t.vsyncPending = true
}

// applyPendingSwapchain reconfigures the swapchain to the recorded logical
// size, then arms the post-resize full-write budget. Runs on the raster
// thread at the present boundary (serialized with BeginFrame/EndFrame via
// mu). Caller must hold mu.
//
// X11/Xwayland: the reconfigure is deferred to the present boundary (the
// extent check below, then BeginFrame's outdated-retry) instead of a
// synchronous Configure here — on X11/Xwayland each Configure costs
// 26–383ms (scaling with window size), so configuring synchronously on
// every resize step — and again in BeginFrame when the window moved during
// the Configure — doubled the per-step stall (the visible "content lags the
// window" gap); measured +70% drag presents.
//
// Other platforms (Wayland/Windows/macOS/browser): wgpu does not report
// "outdated" when the surface size changes, so the swapchain MUST be
// reconfigured here or it stays at the stale extent (presented content
// clipped/stretched); natively these Configure calls are ~1–5ms.
// applyPendingSwapchain reconfigures the swapchain to the recorded logical
// size, then arms the post-resize full-write budget. Runs on the raster
// thread at the present boundary (serialized with BeginFrame/EndFrame via
// mu). Caller must hold mu.
//
// X11/Xwayland: the reconfigure is deferred to the present boundary (the
// extent check below, then BeginFrame's outdated-retry) instead of a
// synchronous Configure here — on X11/Xwayland each Configure costs
// 26–383ms (scaling with window size), so configuring synchronously on
// every resize step — and again in BeginFrame when the window moved during
// the Configure — doubled the per-step stall (the visible "content lags the
// window" gap); measured +70% drag presents.
//
// Other platforms (Wayland/Windows/macOS/browser): wgpu does not report
// "outdated" when the surface size changes, so the swapchain MUST be
// reconfigured here or it stays at the stale extent (presented content
// clipped/stretched); natively these Configure calls are ~1–5ms.
func (t *PresentTarget) applyPendingSwapchainLocked() error {
	if t == nil || t.sc == nil {
		return nil
	}
	if t.swapchainPending {
		t.swapchainPending = false
		// T3 raster-exclusive Context sync: the UI thread only recorded the
		// size above; the dc pixmap realloc + scale switch run here on the
		// raster thread, serialized with the draw below via mu — the packet
		// built at the new size is drawn at the new size, no UI/raster race.
		if t.dc != nil {
			_ = t.dc.Resize(t.logicW, t.logicH)
			t.dc.SetDeviceScale(t.scale)
		}
		if t.ns.Platform == PresentPlatformX11 {
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				pw, ph := physicalSize(t.logicW, t.logicH, t.scale)
				fmt.Fprintf(os.Stderr, "DBG swapchain apply %dx%d (logic %dx%d, deferred)\n", pw, ph, t.logicW, t.logicH)
			}
			// The swapchain extent is reconfigured by BeginFrame's outdated-retry
			// at the live window size; nothing to configure synchronously here.
		} else {
			pw, ph := physicalSize(t.logicW, t.logicH, t.scale)
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG swapchain apply %dx%d (logic %dx%d)\n", pw, ph, t.logicW, t.logicH)
			}
			if err := t.sc.Resize(pw, ph); err != nil {
				// Keep the flag set: the next present retries the reconfigure.
				t.swapchainPending = true
				return err
			}
			// The swapchain now serves buffers at the new size; the next commit
			// (this same present critical section) attaches one of them. Notify
			// the host now — before that commit — so a platform that declares
			// window geometry (Wayland xdg_surface.set_window_geometry) has it
			// hit the compositor in the same batch as the new-size buffer.
			if t.onSwapchainResized != nil {
				t.onSwapchainResized(t.logicW, t.logicH)
			}
		}
		// Swapchain reconfigured → every buffer is undefined. Owe full frames
		// so each buffer is fully written before retained LoadOpLoad frames
		// read it again (otherwise resize storms leave black regions). Also
		// arm the storm window so every present while the resize storm is
		// active stays full even if the 3-frame budget is spent between steps.
		t.postResizeFull = 3
		t.lastResizeAt = time.Now()
	}
	// Runtime present-mode switch (SetVsync), applied at the same boundary so
	// the surface mode never changes mid-frame. A mode change is also a
	// surface reconfigure: buffers are undefined → owe full frames.
	if t.vsyncPending {
		t.vsyncPending = false
		mode := t.sc.PresentModeForVsync(t.vsyncOn)
		if mode != t.sc.GetPresentMode() {
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG vsync %v -> %v\n", t.sc.GetPresentMode(), mode)
			}
			if err := t.sc.SetPresentModeForce(mode); err != nil {
				t.vsyncPending = true
				return err
			}
			t.postResizeFull = 3
			t.lastResizeAt = time.Now()
		}
	}
	return nil
}

// PresentClear clears to RGBA (0–1) and presents one full frame (P0 path).
// inResizeStormLocked reports whether a resize storm is active: the most
// recent physical resize is fresher than resizeStormWindow. Caller holds mu.
func (t *PresentTarget) inResizeStormLocked() bool {
	if t.resizeStormWindow <= 0 {
		return false
	}
	return !t.lastResizeAt.IsZero() && time.Since(t.lastResizeAt) < t.resizeStormWindow
}

// PresentWith begins a frame, runs draw (logical coords on dc), and presents
// via PresentFrameFull (explicit full-surface path). Prefer PresentWithAuto for
// steady retained UI frames that accumulate FrameDamage during draw.
// Safe to call from the raster thread only.
