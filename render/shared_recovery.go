package render

import (
	"errors"
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// Shared-device recovery (R0-6).
//
// Every L1 window borrows one process device but owns its own surface and
// swapchain. The per-swapchain auto-recovery (EnableAutoRecover /
// ForceRecoverHealthy) is therefore never armed on L1 swapchains: one
// window abandoning + recreating "its" device would leave every sibling
// window pointing at a released device. Recovery must abandon once,
// recreate once, and reconfigure ALL registered swapchains — that is what
// RecoverSharedDevice does. No production caller armed the old per-window
// path (only tests), so nothing is migrated.

// shareRecoverMu single-flights shared recoveries: concurrent device-lost
// presents from N windows collapse into one abandon + one RequestDevice.
var shareRecoverMu sync.Mutex

// RecoverSharedDevice abandons the shared device once, recreates it once,
// and reconfigures every live registered swapchain onto the new device.
// Returns errSharedNotOpen when no share is published. Safe to call
// concurrently; concurrent callers collapse into one recovery.
//
// Failure semantics: if the new device cannot be created, the share is
// detached (shareDevice=nil, generation bumped) so the next open publishes
// fresh and no pointer aliases released memory. Live targets keep failing
// loudly until reopened — the same as before recovery existed, minus the
// wild pointers.
func RecoverSharedDevice() error {
	shareRecoverMu.Lock()
	defer shareRecoverMu.Unlock()

	shareMu.Lock()
	oldDev := shareDevice
	adapter := shareAdapter
	inst := shareInst
	targets := make([]*PresentTarget, 0, len(shareTargets))
	for t := range shareTargets {
		targets = append(targets, t)
	}
	shareMu.Unlock()

	if oldDev == nil || adapter == nil {
		return errSharedNotOpen
	}

	// Phase A: every window drops GPU objects while the old device is
	// still addressable (Skia abandonContext order). Capture the surface
	// format for the accelerator rebind below.
	var format types.TextureFormat
	formatSet := false
	for _, t := range targets {
		t.mu.Lock()
		sc := t.sc
		if sc != nil {
			if !formatSet {
				format = sc.Format
				formatSet = true
			}
			if sc.OnDeviceAbandon != nil {
				sc.OnDeviceAbandon(oldDev)
			}
		}
		t.mu.Unlock()
	}
	if !oldDev.IsLost() {
		_ = oldDev.WaitIdle()
	}
	oldDev.FlushCallbacks()
	oldDev.Release()

	// Phase B: recreate exactly once.
	dev, err := requestPresentDeviceWithRetry(adapter, DeviceDescriptorForAdapter("ui-l1-present", adapter), "ui-l1-shared-recover")
	if err != nil {
		detachDeadShare(oldDev)
		return fmt.Errorf("render: shared recovery RequestDevice: %w", err)
	}
	if err := waitDeviceReady(inst, dev, "ui-l1-shared-recover"); err != nil {
		dev.Release()
		detachDeadShare(oldDev)
		return err
	}

	shareMu.Lock()
	if shareDevice != oldDev {
		// Membership changed under us (a concurrent open/close published
		// or detached). Do not adopt: release the spare and report the
		// current state — the share is healthy, just not ours.
		shareMu.Unlock()
		dev.Release()
		if shareDevice == nil {
			return errSharedNotOpen
		}
		return nil
	}
	shareDevice = dev
	shareGen++
	shareMu.Unlock()

	// Sessions rebuild off the accelerator provider switch (deviceGen path
	// in render/internal/gpu); rebind so every window picks up the device.
	_ = SetAcceleratorDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: dev, Adpt: adapter, Format: format,
	})

	// Phase C: reconfigure every live target onto the new device.
	// Buffers from the old swapchain are undefined — arm the same 3-frame
	// full-write budget a resize uses (Skia recreate semantics).
	var firstErr error
	for _, t := range targets {
		t.mu.Lock()
		if t.closed || t.sc == nil {
			t.mu.Unlock()
			continue
		}
		t.device = dev
		t.sc.Device = dev
		if cerr := t.sc.ConfigureFromCapabilities(adapter); cerr != nil {
			if cerr2 := t.sc.Configure(); cerr2 != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("render: shared recovery reconfigure: %v (caps: %v)", cerr2, cerr)
				}
				t.mu.Unlock()
				continue
			}
		}
		if t.sc.OnDeviceRecreated != nil {
			t.sc.OnDeviceRecreated(dev)
		}
		t.postResizeFull = 3
		t.swapchainPending = false
		t.mu.Unlock()
	}
	return firstErr
}

// detachDeadShare nils a share whose device was released but never
// replaced, so no pointer aliases freed memory. Callers hold no locks.
func detachDeadShare(oldDev *webgpu.Device) {
	shareMu.Lock()
	defer shareMu.Unlock()
	if shareDevice == oldDev {
		shareDevice = nil
		shareAdapter = nil
		shareInst = nil
		shareGen++
	}
}

// shouldRecoverSharedLocked reports whether a present error should drive a
// shared recovery. Caller holds t.mu; takes shareMu briefly (t.mu → shareMu
// matches Close's established order).
func (t *PresentTarget) shouldRecoverSharedLocked(err error) bool {
	if err == nil || t == nil || t.device == nil {
		return false
	}
	if !errors.Is(err, webgpu.ErrDeviceLost) {
		return false
	}
	shareMu.Lock()
	defer shareMu.Unlock()
	return shareDevice != nil && t.device == shareDevice
}
