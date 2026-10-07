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
	"image"
	"os"
	"time"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"

	// Register the pure-Go GL backend for SelectBackend (BackendGo).
	_ "github.com/energye/gpui/gpu/gwgpu/gles"
)

// PresentClear clears to RGBA (0–1) and presents one full frame (P0 path).
func (t *PresentTarget) PresentClear(r, g, b, a float64) error {
	return t.PresentWith(func(dc *Context) {
		dc.SetRGBA(r, g, b, a)
		dc.DrawRectangle(0, 0, float64(t.logicW), float64(t.logicH))
		_ = dc.Fill()
	})
}

// InFullRecovery reports whether the swapchain was reconfigured and full
// frames are still owed to its buffers — either by the fixed post-resize
// budget (postResizeFull) or by an active resize storm (lastResizeAt within
// the storm window). Callers that would skip work for retained steady frames
// (e.g. compositeOnly) must disable that path while this is true so every new
// buffer gets fully written.
// InFullRecovery reports whether the swapchain was reconfigured and full
// frames are still owed to its buffers — either by the fixed post-resize
// budget (postResizeFull) or by an active resize storm (lastResizeAt within
// the storm window). Callers that would skip work for retained steady frames
// (e.g. compositeOnly) must disable that path while this is true so every new
// buffer gets fully written.
func (t *PresentTarget) InFullRecovery() bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.swapchainPending || t.postResizeFull > 0 || t.inResizeStormLocked()
}

// inResizeStormLocked reports whether a resize storm is active: the most
// recent physical resize is fresher than resizeStormWindow. Caller holds mu.
// PresentWith begins a frame, runs draw (logical coords on dc), and presents
// via PresentFrameFull (explicit full-surface path). Prefer PresentWithAuto for
// steady retained UI frames that accumulate FrameDamage during draw.
// Safe to call from the raster thread only.
func (t *PresentTarget) PresentWith(draw func(dc *Context)) error {
	_, err := t.present(draw, true)
	return err
}

// PresentWithAuto begins a frame, runs draw, then presents via PresentFrameAuto
// so idle/damage/full modes follow FrameDamage from the draw callback.
// Bootstrap/resize callers that must force a full path should use PresentWith.
// Returns the PresentOutcome for metrics (damage mode, rect count).
// PresentWithAuto begins a frame, runs draw, then presents via PresentFrameAuto
// so idle/damage/full modes follow FrameDamage from the draw callback.
// Bootstrap/resize callers that must force a full path should use PresentWith.
// Returns the PresentOutcome for metrics (damage mode, rect count).
func (t *PresentTarget) PresentWithAuto(draw func(dc *Context)) (PresentOutcome, error) {
	return t.present(draw, false)
}

// LastPresentOutcome returns the outcome of the most recent present call.
// LastPresentOutcome returns the outcome of the most recent present call.
func (t *PresentTarget) LastPresentOutcome() PresentOutcome {
	if t == nil {
		return PresentOutcome{}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastOutcome
}

// Fallbacks reports how many downgrade levels were retried before this
// target opened (0 = first level succeeded). Metrics JSON uses it.
// Fallbacks reports how many downgrade levels were retried before this
// target opened (0 = first level succeeded). Metrics JSON uses it.
func (t *PresentTarget) Fallbacks() int {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.fallbacks
}

// GPUBackend reports the actual adapter category behind this target:
// discrete, integrated, or software (CPU fallback). Metrics JSON uses it.
// GPUBackend reports the actual adapter category behind this target:
// discrete, integrated, or software (CPU fallback). Metrics JSON uses it.
func (t *PresentTarget) GPUBackend() string {
	if t == nil {
		return "unknown"
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.adapter == nil {
		return "unknown"
	}
	switch t.adapter.Info().DeviceType {
	case types.DeviceTypeDiscreteGPU:
		return "discrete"
	case types.DeviceTypeIntegratedGPU:
		return "integrated"
	default:
		return "software"
	}
}

// LastDamageAreaPx returns physical-pixel area of the last frame's damage union
// (0 when idle / unknown). Used for M-DAMAGE-AREA style metrics.
// LastDamageAreaPx returns physical-pixel area of the last frame's damage union
// (0 when idle / unknown). Used for M-DAMAGE-AREA style metrics.
func (t *PresentTarget) LastDamageAreaPx() int64 {
	if t == nil {
		return 0
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastDamageArea
}

func (t *PresentTarget) present(draw func(dc *Context), forceFull bool) (PresentOutcome, error) {
	out := PresentOutcome{}
	if t == nil {
		return out, errors.New("render: nil PresentTarget")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return out, errors.New("render: PresentTarget closed")
	}
	if t.dc == nil || t.sc == nil || t.device == nil {
		return out, errors.New("render: PresentTarget not initialized")
	}

	// Reconfigured swapchain buffers are undefined until each is fully
	// written; owed full frames force the full path.
	// During an active resize storm every present stays full too, so a buffer
	// at an intermediate size is never LoadOpLoad'd half-written.
	// Apply a swapchain resize recorded on the UI thread here — on the raster
	// thread, serialized with BeginFrame/EndFrame — so the surface tracks the
	// window without stalling the UI thread and without Configure/present
	// races dropping frames during interactive resize drags.
	var pApply, pBegin, pEnd time.Time
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		pApply = time.Now()
	}
	if err := t.applyPendingSwapchainLocked(); err != nil {
		return out, err
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		if d := time.Since(pApply); d > 20*time.Millisecond {
			fmt.Fprintf(os.Stderr, "DBG phase apply=%dms\n", d.Milliseconds())
		}
		pBegin = time.Now()
	}
	if t.postResizeFull > 0 || t.inResizeStormLocked() {
		forceFull = true
	}

	if t.device != nil {
		t.device.FlushCallbacks()
	}

	t.dc.BeginFrame()
	if draw != nil {
		draw(t.dc)
	}

	// Snapshot damage for metrics before PresentFrameAuto consumes the plan.
	// Clipped to the physical surface: damage fully outside it (e.g. spinner
	// rects below the viewport fold) presents nothing — acquiring a buffer
	// for them only to discard repeats the swapchain poison below every
	// frame, stalling the raster loop until acquire times out.
	union := t.dc.FrameDamageUnion()
	pw, ph := physicalSize(t.logicW, t.logicH, t.scale)
	clipped := union.Intersect(image.Rect(0, 0, int(pw), int(ph)))
	t.lastDamageArea = int64(clipped.Dx()) * int64(clipped.Dy())
	if t.lastDamageArea < 0 {
		t.lastDamageArea = 0
	}
	// Idle frames skip the swapchain entirely: acquiring a buffer and then
	// discarding it without present leaves the image unrecycled in the
	// driver, draining the swapchain until acquire times out (~250ms) and
	// forces a ~1s reconfigure loop on static windows.
	if !forceFull && clipped.Empty() && t.postResizeFull <= 0 && !t.inResizeStormLocked() {
		out = PresentOutcome{Mode: PresentModeIdle, Idle: true}
		t.lastOutcome = out
		return out, nil
	}

	tAcquire := time.Now()
	frame, err := t.sc.BeginFrame()
	if err != nil {
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG present BeginFrame err=%v (logic %dx%d)\n", err, t.logicW, t.logicH)
		}
		// R0-6: a lost shared device recovers once for ALL windows
		// (abandon once + recreate once + reconfigure every swapchain)
		// instead of each window racing its own recovery on a device it
		// does not own. Drop t.mu across the slow recovery, then retry
		// the acquire exactly once.
		if t.shouldRecoverSharedLocked(err) {
			t.mu.Unlock()
			rerr := RecoverSharedDevice()
			t.mu.Lock()
			if rerr != nil {
				return out, fmt.Errorf("render: shared recovery: %v (present: %w)", rerr, err)
			}
			if t.closed || t.sc == nil || t.device == nil {
				return out, errors.New("render: PresentTarget closed during recovery")
			}
			frame, err = t.sc.BeginFrame()
			if err != nil {
				return out, fmt.Errorf("render: BeginFrame after recovery: %w", err)
			}
		} else {
			return out, fmt.Errorf("render: BeginFrame: %w", err)
		}
	}
	// Acquire 等空闲缓冲(Fifo 背压睡这儿)单记，不掺进干活。
	// 恢复重试也在内：慢恢复属于等，不属于干活。失败路径直接返回，
	// 不记 (调用方 err != nil 时不采样)。
	noteAcquireWaitMs(tAcquire)
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		if d := time.Since(pBegin); d > 20*time.Millisecond {
			fmt.Fprintf(os.Stderr, "DBG phase begin=%dms (frame %dx%d)\n", d.Milliseconds(), frame.Width, frame.Height)
		}
		pEnd = time.Now()
	}
	if os.Getenv("WR_RESIZE_DBG") == "1" {
		fmt.Fprintf(os.Stderr, "DBG sc.frame %dx%d (logic %dx%d)\n", frame.Width, frame.Height, t.logicW, t.logicH)
	}
	// Extent check (Impeller KHRSwapchainVK::AcquireNextDrawable parity: a
	// successful acquire is still reconfigured when
	// `!out_of_date && size_ == GetSize()` fails). X11 does not report
	// "outdated" after programmatic resizes, so a matching acquire is not
	// proof the swapchain tracks the window — without this check the chain
	// stays at the stale extent forever and every present clips to it.
	// Like Impeller there is no storm deferral here: a mismatch reconfigures
	// at the recorded size and re-acquires once, every present if needed
	// (bounded: one forced reconfigure per present, convergence across
	// presents). Deferring inside the storm window left drag-resizes painting
	// clipped frames with blank regions until the storm settled; the
	// synchronous Configure costs ~1–5ms on real GPUs, and correctness
	// (content tracks the window) outranks saving it on software raster.
	if pw, ph := physicalSize(t.logicW, t.logicH, t.scale); frame.Width != pw || frame.Height != ph {
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG sc.frame stale %dx%d want %dx%d (force reconfig)\n", frame.Width, frame.Height, pw, ph)
		}
		t.sc.DiscardFrame(frame)
		var rApply time.Time
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			rApply = time.Now()
		}
		if rerr := t.sc.Resize(pw, ph); rerr != nil {
			t.swapchainPending = true
			return out, fmt.Errorf("render: stale extent reconfigure: %w", rerr)
		}
		t.postResizeFull = 3
		t.lastResizeAt = time.Now()
		frame, err = t.sc.BeginFrame()
		if err != nil {
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG present BeginFrame err=%v (logic %dx%d)\n", err, t.logicW, t.logicH)
			}
			return out, fmt.Errorf("render: BeginFrame: %w", err)
		}
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG sc.frame %dx%d (logic %dx%d, forced reconfig=%dms)\n", frame.Width, frame.Height, t.logicW, t.logicH, time.Since(rApply).Milliseconds())
		}
	}
	// Failed/timeout BeginFrames above return early and do NOT consume the
	// post-resize full budget: the next frame still owes a full write.
	presentFn := func() error {
		if err := t.sc.EndFrame(frame); err != nil {
			return err
		}
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			if d := time.Since(pEnd); d > 20*time.Millisecond {
				fmt.Fprintf(os.Stderr, "DBG phase endframe=%dms (frame %dx%d)\n", d.Milliseconds(), frame.Width, frame.Height)
			}
		}
		if t.postResizeFull > 0 {
			t.postResizeFull--
		}
		return nil
	}
	// hal frame carries the view as a word; rebuild the typed handle here
	// (render boundary owns the gpucontext type, hal does not).
	view := gpucontext.NewTextureView(unsafe.Pointer(frame.ViewHandle))
	if forceFull {
		if err := t.dc.PresentFrameFull(view, frame.Width, frame.Height, presentFn); err != nil {
			t.sc.DiscardFrame(frame)
			return out, fmt.Errorf("render: PresentFrameFull: %w", err)
		}
		out = PresentOutcome{Mode: PresentModeFull, Rects: 1}
		t.lastOutcome = out
		return out, nil
	}
	out, err = t.dc.PresentFrameAuto(view, frame.Width, frame.Height, presentFn)
	if err != nil {
		t.sc.DiscardFrame(frame)
		return out, fmt.Errorf("render: PresentFrameAuto: %w", err)
	}
	if out.Idle {
		// Idle drew nothing and PresentFrameAuto does not call the present
		// callback, so the acquired swapchain frame would stay in-flight and
		// poison the next BeginFrame ("frame already in flight" → black
		// screen after resize storms / full-static retained frames). Release
		// the acquire explicitly to keep BeginFrame/Present paired.
		t.sc.DiscardFrame(frame)
	}
	t.lastOutcome = out
	return out, nil
}

// WaitIdle blocks until the device has finished all submitted GPU work.
// Nil-safe and closed-safe: reports nil when there is no device to wait on.
// Close paths call it before destroying view-bound caches (layer textures)
// so deferred releases never execute while submissions still reference them.
// Both backends implement it via hal.Device (native = wgpu wait, go = noop).
// WaitIdle blocks until the device has finished all submitted GPU work.
// Nil-safe and closed-safe: reports nil when there is no device to wait on.
// Close paths call it before destroying view-bound caches (layer textures)
// so deferred releases never execute while submissions still reference them.
// Both backends implement it via hal.Device (native = wgpu wait, go = noop).
func (t *PresentTarget) WaitIdle() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	dev := t.device
	t.mu.Unlock()
	if dev == nil {
		_, _, dev, _, _ = peekShared()
	}
	if dev == nil {
		return nil
	}
	return dev.WaitIdle()
}

// Close releases GPU resources. Safe to call multiple times.
// Windows on the borrowed share release only their own surface/swapchain/
// context; the shared instance/adapter/device go when the last PresentTarget
// (first or borrowed) closes.
// Close releases GPU resources. Safe to call multiple times.
// Windows on the borrowed share release only their own surface/swapchain/
// context; the shared instance/adapter/device go when the last PresentTarget
// (first or borrowed) closes.
func (t *PresentTarget) Close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	t.closed = true
	// R0-6: drop the recovery-registry entry on every close path below.
	defer unregisterSharedTarget(t)
	// R6 measurement: WR_MEMDIG=1 dumps the process VRAM ledger (live,
	// peak, count, budget) at window close for post-fix comparison.
	defer func() {
		if os.Getenv("WR_MEMDIG") == "1" {
			fmt.Fprintf(os.Stderr, "MEMDIG live=%.2fMiB peak=%.2fMiB count=%d budget=%dMiB\n",
				float64(VramLiveBytes())/(1024*1024),
				float64(VramPeakBytes())/(1024*1024),
				VramLiveCount(), VramBudgetMB())
		}
	}()
	if t.dc != nil {
		_ = t.dc.Close()
		t.dc = nil
	}
	if t.sc != nil {
		t.sc.Release()
		t.sc = nil
	}
	if t.surf != nil {
		t.surf.Destroy()
		t.surf = nil
	}
	if t.shared {
		t.device = nil
		t.adapter = nil
		t.inst = nil
		releaseShared()
		return nil
	}
	// R0-1: the owner window's device is the published share. Never Release
	// it directly while borrowers may hold refs — that leaves the share
	// pointer (and every borrower's handle) wild while releaseShared keeps
	// the non-nil pointer alive. Clear our aliases and let releaseShared
	// own the lifetime (last close releases).
	shareMu.Lock()
	isShare := t.device != nil && t.device == shareDevice
	shareMu.Unlock()
	if isShare {
		t.device = nil
		t.adapter = nil
		t.inst = nil
		releaseShared()
		return nil
	}
	// Orphan device (not the published share): release directly and do NOT
	// touch the share refcount, which belongs to another chain.
	if t.device != nil {
		t.device.Release()
		t.device = nil
	}
	if t.adapter != nil {
		t.adapter.Release()
		t.adapter = nil
	}
	if t.inst != nil {
		t.inst.Release()
		t.inst = nil
	}
	return nil
}
