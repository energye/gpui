//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rwgpu

import (
	"log"
	"strings"

	"github.com/ebitengine/purego"
)

// markDeviceLost sets Device.lost for the registered handle only (per-device).
func markDeviceLost(handle uintptr) {
	if handle == 0 {
		return
	}
	lostDeviceHandles.Store(handle, struct{}{})
	if v, ok := liveDevices.Load(handle); ok {
		if d, ok := v.(*Device); ok && d != nil {
			d.lost.Store(true)
		}
	}
}

// markDeviceObjectLost marks one *Device sticky-lost (userdata primary path).
// markDeviceObjectLost marks one *Device sticky-lost (userdata primary path).
func markDeviceObjectLost(d *Device) {
	if d == nil {
		return
	}
	d.lost.Store(true)
	if d.handle != 0 {
		lostDeviceHandles.Store(d.handle, struct{}{})
	}
}

// markDeviceLostFromCallback routes a native device-lost signal onto exactly
// one Device when possible:
//  1. userdata1 slot (preferred — multi-window / multi-device)
//  2. native device handle from callback arg
//
// Never marks unrelated live devices.
// markDeviceLostFromCallback routes a native device-lost signal onto exactly
// one Device when possible:
//  1. userdata1 slot (preferred — multi-window / multi-device)
//  2. native device handle from callback arg
//
// Never marks unrelated live devices.
func markDeviceLostFromCallback(devicePtr, userdata1 uintptr) {
	if d := deviceFromUserdata(userdata1); d != nil {
		markDeviceObjectLost(d)
		return
	}
	h := deviceHandleFromCallbackArg(devicePtr)
	if h != 0 {
		markDeviceLost(h)
		return
	}
	// purego may pass WGPUDevice by value.
	if devicePtr != 0 {
		if _, ok := liveDevices.Load(devicePtr); ok {
			markDeviceLost(devicePtr)
		}
	}
	// Unresolved: do NOT mark all devices (would poison other windows).
}

// noteLostMessage marks sticky lost for one known device handle only.
// If deviceHandle is 0, no device is marked (caller must pass the owner).
// noteLostMessage marks sticky lost for one known device handle only.
// If deviceHandle is 0, no device is marked (caller must pass the owner).
func noteLostMessage(deviceHandle uintptr, msg string) {
	if !looksLikeDeviceLost(msg) || deviceHandle == 0 {
		return
	}
	markDeviceLost(deviceHandle)
}

// IsDeviceHandleLost reports sticky lost state for a native device handle.
// Survives Device.Release (liveDevices unregister) so child Release/Destroy
// still skip native after the parent Device object is gone.
// IsDeviceHandleLost reports sticky lost state for a native device handle.
// Survives Device.Release (liveDevices unregister) so child Release/Destroy
// still skip native after the parent Device object is gone.
func IsDeviceHandleLost(handle uintptr) bool {
	if handle == 0 {
		return false
	}
	if _, ok := lostDeviceHandles.Load(handle); ok {
		return true
	}
	if v, ok := liveDevices.Load(handle); ok {
		if d, ok := v.(*Device); ok && d != nil {
			return d.lost.Load()
		}
	}
	return false
}

// IsLost reports DeviceLostCallback state for this device. Safe on nil.
// IsLost reports DeviceLostCallback state for this device. Safe on nil.
func (d *Device) IsLost() bool {
	return d != nil && d.lost.Load()
}

// MarkLost sets sticky device-lost state (same effect as DeviceLostCallback /
// uncaptured "Parent device is lost"). Safe on nil. Used by facade tests and
// recovery paths that inject lost without a native callback.
// MarkLost sets sticky device-lost state (same effect as DeviceLostCallback /
// uncaptured "Parent device is lost"). Safe on nil. Used by facade tests and
// recovery paths that inject lost without a native callback.
func (d *Device) MarkLost() {
	if d == nil {
		return
	}
	d.lost.Store(true)
	if d.handle != 0 {
		lostDeviceHandles.Store(d.handle, struct{}{})
	}
}

// uncapturedErrorHandler records validation/OOM/etc. When the message looks
// like device-lost (e.g. "Parent device is lost"), also marks the owning
// device sticky-lost so subsequent public calls refuse with ErrDeviceLost
// instead of treating the device as healthy.
//
// We must mark sticky on the FIRST uncaptured lost message so the next
// GetCurrentTexture refuses before purego Call.
// uncapturedErrorHandler records validation/OOM/etc. When the message looks
// like device-lost (e.g. "Parent device is lost"), also marks the owning
// device sticky-lost so subsequent public calls refuse with ErrDeviceLost
// instead of treating the device as healthy.
//
// We must mark sticky on the FIRST uncaptured lost message so the next
// GetCurrentTexture refuses before purego Call.
func uncapturedErrorHandler(devicePtr, errType, messageData, messageLength, userdata1, _ uintptr) uintptr {
	msg := callbackStringView(messageData, messageLength)
	// Resolve owning device for sticky storage + multi-device isolation.
	ownerHandle := uintptr(0)
	if d := deviceFromUserdata(userdata1); d != nil && d.handle != 0 {
		ownerHandle = d.handle
	} else {
		ownerHandle = deviceHandleFromCallbackArg(devicePtr)
	}
	lastUncapturedMu.Lock()
	lastUncapturedTyp = ErrorType(errType)
	lastUncapturedMsg = msg
	lastUncapturedDevice = ownerHandle
	lastUncapturedMu.Unlock()
	if looksLikeDeviceLost(msg) {
		log.Printf("rwgpu: Uncaptured looksLikeDeviceLost type=%d msg=%q owner=%#x userdata1=%d",
			errType, msg, ownerHandle, userdata1)
		// Userdata first — which window/device owns this error.
		markDeviceLostFromCallback(devicePtr, userdata1)
	}
	return 0
}

func initUncapturedErrorCallback() {
	uncapturedErrorCallbackPtr = purego.NewCallback(uncapturedErrorHandler)
}

// deviceLostEnterHook, when non-nil, is invoked at the start of deviceLostHandler.
// Tests use it to count native → Go DeviceLost deliveries. Production leaves nil.
var deviceLostEnterHook func()

// deviceLostHandler is WGPUDeviceLostCallback — primary writer of Device.lost.
// userdata1 is the per-device slot allocated at RequestDevice (multi-window safe).
// Always logs ENTER so soaks can see whether native delivered into Go before GCT abort.
// deviceLostHandler is WGPUDeviceLostCallback — primary writer of Device.lost.
// userdata1 is the per-device slot allocated at RequestDevice (multi-window safe).
// Always logs ENTER so soaks can see whether native delivered into Go before GCT abort.
func deviceLostHandler(devicePtr, reason, messageData, messageLength, userdata1, userdata2 uintptr) uintptr {
	_ = userdata2
	if deviceLostEnterHook != nil {
		deviceLostEnterHook()
	}
	msg := callbackStringView(messageData, messageLength)
	h := deviceHandleFromCallbackArg(devicePtr)
	log.Printf("rwgpu: DeviceLostCallback ENTER reason=%d msg=%q devicePtr=%#x handle=%#x userdata1=%d",
		reason, msg, devicePtr, h, userdata1)
	markDeviceLostFromCallback(devicePtr, userdata1)
	d := deviceFromUserdata(userdata1)
	lost := d != nil && d.IsLost()
	if !lost && h != 0 {
		lost = IsDeviceHandleLost(h)
	}
	log.Printf("rwgpu: DeviceLostCallback DONE sticky_lost=%v routed_device=%v", lost, d != nil)
	return 0
}

// deviceHandleFromCallbackArg extracts a WGPUDevice handle from the C callback's
// first argument. webgpu.h passes WGPUDevice const* (pointer-to-handle); some
// purego/ABI paths may pass the handle value directly.
// Returns 0 when the pointed-to device is null (e.g. FailedCreation) — does not
// mark any other device lost.
// deviceHandleFromCallbackArg extracts a WGPUDevice handle from the C callback's
// first argument. webgpu.h passes WGPUDevice const* (pointer-to-handle); some
// purego/ABI paths may pass the handle value directly.
// Returns 0 when the pointed-to device is null (e.g. FailedCreation) — does not
// mark any other device lost.
func deviceHandleFromCallbackArg(devicePtr uintptr) uintptr {
	if devicePtr == 0 {
		return 0
	}
	h := *(*uintptr)(ptrFromUintptr(devicePtr))
	if h != 0 {
		return h
	}
	// ABI fallback: treat arg as the handle itself when non-null.
	return devicePtr
}

func looksLikeDeviceLost(msg string) bool {
	if msg == "" {
		return false
	}
	// Match common wgpu / Vulkan / DX12 / Metal phrasings (log + uncaptured).
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "device lost") ||
		strings.Contains(lower, "device_lost") ||
		strings.Contains(lower, "parent device is lost") ||
		strings.Contains(lower, "device is lost") ||
		strings.Contains(lower, "lost the device") ||
		strings.Contains(lower, "dxgienum::device_removed") ||
		strings.Contains(lower, "device_removed") ||
		strings.Contains(lower, "vk_error_device_lost")
}

func initDeviceLostCallback() {
	deviceLostCallbackPtr = purego.NewCallback(deviceLostHandler)
}

// RequestDevice requests a GPU device from the adapter.
// This is a synchronous wrapper that blocks until the device is available.
// absorbLostUncapturedLocked consumes a pending uncaptured lost message for this
// device and marks sticky lost. Returns true if the device is now lost.
// Caller should hold gpuMu when folding with surface acquire.
func (d *Device) absorbLostUncapturedLocked() bool {
	if d == nil {
		return false
	}
	if d.IsLost() {
		return true
	}
	_, msg, owner := PeekLastUncapturedError()
	if !looksLikeDeviceLost(msg) {
		return d.IsLost()
	}
	if owner != 0 && d.handle != 0 && owner != d.handle {
		return false
	}
	_, _ = LastUncapturedError()
	markDeviceObjectLost(d)
	return true
}

// SyncLostState pumps instance events and folds pending Uncaptured/DeviceLost
// into sticky IsLost. Does not call WriteBuffer canary —
// soft native returns errors from GetCurrentTexture/Submit instead of SIGABRT.
// Safe and idempotent on nil / already-lost devices.
// SyncLostState pumps instance events and folds pending Uncaptured/DeviceLost
// into sticky IsLost. Does not call WriteBuffer canary —
// soft native returns errors from GetCurrentTexture/Submit instead of SIGABRT.
// Safe and idempotent on nil / already-lost devices.
func (d *Device) SyncLostState() {
	if d == nil || d.IsLost() {
		return
	}
	if checkInit() != nil {
		return
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	pumpInstanceEvents(d)
	_ = d.absorbLostUncapturedLocked()
}

// syncLostStateLocked is for call sites that already hold gpuMu.
// syncLostStateLocked is for call sites that already hold gpuMu.
func (d *Device) syncLostStateLocked() {
	if d == nil || d.IsLost() {
		return
	}
	pumpInstanceEvents(d)
	_ = d.absorbLostUncapturedLocked()
}

// Destroy forces native device teardown and sticky-lost state (Skia/Flutter
// abandon-context). Sticky is force-marked so public APIs refuse with
// ErrDeviceLost even if DeviceLostCallback is not delivered. Idempotent and nil-safe.
