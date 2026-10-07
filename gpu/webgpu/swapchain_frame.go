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
	"image"
	"os"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// Stats returns cumulative present-path counters.
func (sc *Swapchain) Stats() SwapchainStats {
	if sc == nil {
		return SwapchainStats{}
	}
	st := SwapchainStats{
		Acquires:       sc.acquires,
		Presents:       sc.presents,
		Discards:       sc.discards,
		Reconfigures:   sc.reconfigures,
		Suboptimal:     sc.suboptimal,
		AcquireRetries: sc.acquireRetries,
		LastAcquireNs:  sc.lastAcquireNs,
		LastPresentNs:  sc.lastPresentNs,
	}
	st.LastAcquireMs = float64(sc.lastAcquireNs) / 1e6
	st.LastPresentMs = float64(sc.lastPresentNs) / 1e6
	return st
}

// ResetStats clears counters (configuration retained).
// ResetStats clears counters (configuration retained).
func (sc *Swapchain) ResetStats() {
	if sc == nil {
		return
	}
	sc.acquires = 0
	sc.presents = 0
	sc.discards = 0
	sc.reconfigures = 0
	sc.suboptimal = 0
	sc.acquireRetries = 0
	sc.lastAcquireNs = 0
	sc.lastPresentNs = 0
}

// SetPreferVSync selects Fifo when available (default production UI path).
// SetSupportedPresentModesForTest overrides the cached surface-capability
// list (test-only; ConfigureFromCapabilities normally populates it from the
// adapter). Used to unit-test PresentModeForVsync without a real surface.
func (sc *Swapchain) SetSupportedPresentModesForTest(modes []types.PresentMode) {
	if sc == nil {
		return
	}
	sc.supportedPresentModes = append([]types.PresentMode(nil), modes...)
}

// SetPresentModeForce reconfigures the surface with an explicit present mode
// even when the extent is unchanged (Resize no-ops same-size calls). Used by
// the runtime vsync switch (Fifo steady ↔ Mailbox/Immediate during resize).
// A reconfigure invalidates the swapchain buffers — the caller owes full
// frames until each buffer is fully written.
func (sc *Swapchain) BeginFrame() (*Frame, error) {
	if sc == nil {
		return nil, fmt.Errorf("wgpu: swapchain is nil")
	}
	sc.frameMu.Lock()
	defer sc.frameMu.Unlock()
	if sc.Surface == nil {
		return nil, ErrInvalidHandle
	}
	if sc.Surface.released {
		return nil, ErrReleased
	}
	if sc.frameOpen {
		return nil, ErrFrameInFlight
	}
	if sc.Width == 0 || sc.Height == 0 {
		return nil, fmt.Errorf("wgpu: swapchain extent must be non-zero")
	}

	// Ensure device is healthy (DeviceLostCallback → recover or hal.ErrDeviceLost).
	if err := sc.ensureDeviceLocked(); err != nil {
		return nil, err
	}
	if err := sc.takeRecoverGrace(); err != nil {
		return nil, err
	}

	// Hung-acquire gate: while latched, skip the cgo acquire and route into
	// the throttled DiscardTexture+Configure recreate (Skia swapchain
	// re-create on acquire failure). This keeps the frame loop alive on
	// software Vulkan where acquire can block forever.
	if err := sc.acquireHungLocked(); err != nil {
		return nil, err
	}

	if sc.pendingReconfigure || !sc.configured {
		if err := sc.reconfigureThrottled(); err != nil {
			return nil, err
		}
	}

	// Pump pending DeviceLost callbacks before acquire.
	if sc.Device != nil {
		sc.Device.FlushCallbacks()
		if err := sc.ensureDeviceLocked(); err != nil {
			return nil, err
		}
		if err := sc.takeRecoverGrace(); err != nil {
			return nil, err
		}
	}

	t0 := time.Now()
	st, suboptimal, err := sc.acquireSurfaceTexture()
	if err != nil {
		if isDeviceLostErr(err) || sc.deviceKnownLostLocked() {
			// Recover here, but never Present in the same BeginFrame — session
			// depth alloc after mid-call recover was the minimize OOM path.
			if rerr := sc.ensureDeviceLocked(); rerr != nil {
				return nil, rerr
			}
			if err := sc.takeRecoverGrace(); err != nil {
				return nil, err
			}
			// Recover without grace (grace already 0): still skip this frame.
			return nil, ErrRecovered
		} else if isSkipFrameSurfaceErr(err) {
			return nil, err
		} else {
			// Outdated / surface error / acquire timeout: one reconfigure then retry.
			if os.Getenv("WR_RESIZE_DBG") == "1" {
				fmt.Fprintf(os.Stderr, "DBG bf acquire-fail=%dms err=%v\n", time.Since(t0).Milliseconds(), err)
			}
			if sc.Surface != nil {
				sc.Surface.DiscardTexture(nil)
			}
			// The window may have out-paced the last applied size (interactive
			// resize drag): reconfigure at the CURRENT live window extent so
			// the retry acquire matches the surface instead of looping on a
			// stale configured size (which leaves every frame "outdated" and
			// drops it — the window then keeps the pre-drag content).
			if w, h, ok := sc.Surface.WindowSize(); ok && w > 0 && h > 0 {
				sc.Width, sc.Height = w, h
			}
			if cfgErr := sc.Configure(); cfgErr != nil {
				if isDeviceLostErr(cfgErr) || sc.deviceKnownLostLocked() {
					if rerr := sc.ensureDeviceLocked(); rerr != nil {
						return nil, rerr
					}
					if err := sc.takeRecoverGrace(); err != nil {
						return nil, err
					}
					return nil, ErrRecovered
				} else {
					return nil, fmt.Errorf("%w (reconfigure: %v)", err, cfgErr)
				}
			} else {
				sc.lastReconfig = time.Now()
				sc.acquireRetries++
				if os.Getenv("WR_RESIZE_DBG") == "1" {
					fmt.Fprintf(os.Stderr, "DBG bf retry-cfg=%dms (new %dx%d)\n", time.Since(t0).Milliseconds(), sc.Width, sc.Height)
				}
				// A fresh swapchain after the recreate: clear the hung latch so
				// the retry acquire actually attempts (and future frames are
				// not blocked by a stale latch).
				sc.acquireHung.Store(false)
				st, suboptimal, err = sc.acquireSurfaceTexture()
				if os.Getenv("WR_RESIZE_DBG") == "1" && err != nil {
					fmt.Fprintf(os.Stderr, "DBG bf retry-acq-fail=%dms err=%v\n", time.Since(t0).Milliseconds(), err)
				}
				if err != nil {
					if isDeviceLostErr(err) || sc.deviceKnownLostLocked() {
						if rerr := sc.ensureDeviceLocked(); rerr != nil {
							return nil, rerr
						}
						if err := sc.takeRecoverGrace(); err != nil {
							return nil, err
						}
						return nil, ErrRecovered
					} else {
						return nil, err
					}
				}
			}
		}
	}
	sc.lastAcquireNs = time.Since(t0).Nanoseconds()
	sc.acquires++

	view, err := st.CreateView(nil)
	if err != nil {
		// Drop acquired surface texture or next Configure panics native:
		// "SurfaceOutput must be dropped before a new Surface is made".
		if st != nil {
			st.Release()
		}
		return nil, fmt.Errorf("wgpu: surface texture CreateView: %w", err)
	}
	// Internal unpack (片7d): Frame carries the concrete view; hal iface stays at API boundary.
	cv, _ := view.(*TextureView)
	if suboptimal {
		sc.suboptimal++
		// Act once per extent. Continuous reconfigure of the same size causes
		// black flashes and burns CPU without improving the surface.
		if sc.suboptHandledW != sc.Width || sc.suboptHandledH != sc.Height {
			sc.pendingReconfigure = true
		}
	}
	sc.frameOpen = true
	return &Frame{
		SurfaceTexture: st,
		View:           cv,
		Handle:         TextureViewToHandle(cv),
		Suboptimal:     suboptimal,
		Width:          sc.Width,
		Height:         sc.Height,
	}, nil
}

// reconfigureThrottled runs Configure at most once per 500ms to avoid native
// Surface.Configure thrash under long multi-module stress.
// reconfigureThrottled runs Configure at most once per 500ms to avoid native
// Surface.Configure thrash under long multi-module stress.
func (sc *Swapchain) reconfigureThrottled() error {
	const minInterval = 500 * time.Millisecond
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if err := sc.ensureDeviceLocked(); err != nil {
		return err
	}
	if !sc.lastReconfig.IsZero() && time.Since(sc.lastReconfig) < minInterval {
		sc.pendingReconfigure = true
		return fmt.Errorf("wgpu: surface reconfigure rate-limited")
	}
	// Best-effort drop of any dangling surface output before reconfigure.
	if sc.Surface != nil {
		sc.Surface.DiscardTexture(nil)
	}
	if err := sc.Configure(); err != nil {
		return err
	}
	sc.lastReconfig = time.Now()
	// A fresh swapchain has new images: a hung acquire may recover here.
	sc.acquireHung.Store(false)
	return nil
}

// acquireSurfaceTexture calls Surface.GetCurrentTexture with a bounded wait.
// On timeout the acquire is latched hung (see acquireHung) and the error
// routes BeginFrame into the DiscardTexture+Configure recreate path — the
// Skia/Flutter swapchain-recreate-on-failure model. The stuck cgo goroutine
// is abandoned once; the latch prevents per-frame goroutine leaks.
// acquireSurfaceTexture calls Surface.GetCurrentTexture with a bounded wait.
// On timeout the acquire is latched hung (see acquireHung) and the error
// routes BeginFrame into the DiscardTexture+Configure recreate path — the
// Skia/Flutter swapchain-recreate-on-failure model. The stuck cgo goroutine
// is abandoned once; the latch prevents per-frame goroutine leaks.
func (sc *Swapchain) acquireSurfaceTexture() (*SurfaceTexture, bool, error) {
	if sc == nil || sc.Surface == nil {
		return nil, false, ErrInvalidHandle
	}
	if sc.acquireHung.Load() {
		return nil, false, ErrAcquireTimeout
	}
	type acqRes struct {
		st  *SurfaceTexture
		sub bool
		err error
	}
	ch := make(chan acqRes, 1)
	go func() {
		st, sub, err := sc.Surface.GetCurrentTexture()
		ch <- acqRes{st: st, sub: sub, err: err}
	}()
	select {
	case r := <-ch:
		return r.st, r.sub, r.err
	case <-time.After(acquireTimeout):
		sc.acquireHung.Store(true)
		sc.hungMarkedAt = time.Now()
		sc.acquireTimeouts++
		return nil, false, ErrAcquireTimeout
	}
}

// acquireHungLocked is the BeginFrame gate: while the acquire is latched
// hung, skip the cgo call entirely and route to the throttled recreate so
// the frame loop never blocks on a dead surface again (Chrome
// SyntheticBeginFrameSource-style fallback for the swapchain). Caller holds
// frameMu.
// acquireHungLocked is the BeginFrame gate: while the acquire is latched
// hung, skip the cgo call entirely and route to the throttled recreate so
// the frame loop never blocks on a dead surface again (Chrome
// SyntheticBeginFrameSource-style fallback for the swapchain). Caller holds
// frameMu.
func (sc *Swapchain) acquireHungLocked() error {
	if sc == nil || !sc.acquireHung.Load() {
		return nil
	}
	// Throttled recreate attempt: a new swapchain can unstick the acquire.
	if sc.hungMarkedAt.IsZero() || time.Since(sc.hungMarkedAt) >= 500*time.Millisecond {
		sc.hungMarkedAt = time.Now()
		if err := sc.reconfigureThrottled(); err == nil {
			sc.pendingReconfigure = true
			sc.acquireHung.Store(false)
			return nil
		}
	}
	return ErrAcquireTimeout
}

// EndFrame presents the frame to the platform surface.
// EndFrame presents the frame to the platform surface.
func (sc *Swapchain) EndFrame(frame *Frame) error {
	return sc.endFrame(frame, nil)
}

// EndFrameWithDamage presents the frame, forwarding damage rects when the
// backend supports partial present.
// EndFrameWithDamage presents the frame, forwarding damage rects when the
// backend supports partial present.
func (sc *Swapchain) EndFrameWithDamage(frame *Frame, rects []image.Rectangle) error {
	return sc.endFrame(frame, rects)
}

func (sc *Swapchain) endFrame(frame *Frame, rects []image.Rectangle) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if frame == nil {
		return fmt.Errorf("wgpu: frame is nil")
	}
	sc.frameMu.Lock()
	defer sc.frameMu.Unlock()
	if !sc.frameOpen {
		return ErrNoFrame
	}
	if frame.View != nil {
		frame.View.Release()
		frame.View = nil
	}
	if frame.Suboptimal {
		if sc.suboptHandledW != sc.Width || sc.suboptHandledH != sc.Height {
			sc.pendingReconfigure = true
		}
	}
	t0 := time.Now()
	var err error
	if sc.Surface == nil {
		err = ErrInvalidHandle
	} else if len(rects) > 0 {
		err = sc.Surface.PresentWithDamage(frame.SurfaceTexture, rects)
	} else if len(frame.DamageRects) > 0 {
		err = sc.Surface.PresentWithDamage(frame.SurfaceTexture, frame.DamageRects)
	} else {
		err = sc.Surface.Present(frame.SurfaceTexture)
	}
	sc.lastPresentNs = time.Since(t0).Nanoseconds()
	sc.presents++
	// After Present, drop ReturnedWithOwnership surface texture.
	// Must happen after Present so surface no longer holds "current" image.
	if frame.SurfaceTexture != nil {
		frame.SurfaceTexture.Release()
		frame.SurfaceTexture = nil
	}
	sc.frameOpen = false
	if sc.Device != nil {
		sc.Device.FlushCallbacks()
		if err == nil && sc.Device.IsLost() {
			err = hal.ErrDeviceLost
		}
	}
	if isDeviceLostErr(err) {
		return hal.ErrDeviceLost
	}
	return err
}

// DiscardFrame drops an acquired frame without presenting.
// Releases the surface texture immediately (ReturnedWithOwnership).
// DiscardFrame drops an acquired frame without presenting.
// Releases the surface texture immediately (ReturnedWithOwnership).
func (sc *Swapchain) DiscardFrame(frame *Frame) {
	if frame == nil {
		return
	}
	if frame.View != nil {
		frame.View.Release()
		frame.View = nil
	}
	if frame.SurfaceTexture != nil {
		frame.SurfaceTexture.Release()
		frame.SurfaceTexture = nil
	}
	if sc != nil {
		sc.frameMu.Lock()
		sc.discards++
		sc.frameOpen = false
		sc.frameMu.Unlock()
	}
}

// PresentModeName returns a short label for the active present mode.
// Release unconfigures the surface; does not release Surface/Device ownership.
func (sc *Swapchain) Release() {
	if sc == nil || sc.Surface == nil {
		return
	}
	sc.Surface.Unconfigure(nil)
	sc.configured = false
}
