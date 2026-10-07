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
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego"
)

// deviceCallbackHandler is the Go function called by native code via purego.NewCallback.
// Signature: void(status uint32, device uintptr, message StringView, userdata1 uintptr, userdata2 uintptr)
func deviceCallbackHandler(status uintptr, device uintptr, messageData uintptr, messageLength uintptr, userdata1, userdata2 uintptr) uintptr {
	msg := callbackStringView(messageData, messageLength)

	// Find and complete the request
	deviceRequestsMu.Lock()
	req, ok := deviceRequests[userdata1]
	if ok {
		delete(deviceRequests, userdata1)
	}
	deviceRequestsMu.Unlock()

	if ok && req != nil {
		req.status = RequestDeviceStatus(status)
		if device != 0 {
			trackResource(device, "Device")
			req.device = &Device{handle: device}
		}
		req.message = msg
		close(req.done)
	}
	return 0
}

// initDeviceCallback creates the C callback function pointer using purego.
// initDeviceCallback creates the C callback function pointer using purego.
func initDeviceCallback() {
	deviceCallbackPtr = purego.NewCallback(deviceCallbackHandler)
}

// Non-panicking device lifecycle callbacks. wgpu-native's default uncaptured
// handler aborts the process on Validation/OOM ("Not enough memory left"),
// which turns transient VRAM pressure into hard SIGABRT. We record the last
// error so CreateTexture/etc. can return a Go error (null handle path).
//
// Callers must check IsLost() and skip native ops.

var (
	uncapturedErrorCallbackPtr  uintptr
	uncapturedErrorCallbackOnce sync.Once
	deviceLostCallbackPtr       uintptr
	deviceLostCallbackOnce      sync.Once

	lastUncapturedMu     sync.Mutex
	lastUncapturedMsg    string
	lastUncapturedTyp    ErrorType
	lastUncapturedDevice uintptr // native handle of the device that reported it (0=unknown)

	// liveDevices maps native WGPUDevice handle → *Device (secondary route).
	liveDevices sync.Map // map[uintptr]*Device

	// deviceSlots maps callback Userdata1 slot id → *Device.
	// Multi-window: each Device has its own slot; lost marks only that device.
	deviceSlots   sync.Map // map[uintptr]*Device
	deviceSlotSeq atomic.Uint64

	// lostDeviceHandles: sticky lost native handles after Device.Release.
	lostDeviceHandles sync.Map // map[uintptr]struct{}
)

// LastUncapturedError returns and clears the most recent uncaptured device error.
// allocDeviceSlot reserves a userdata slot before RequestDevice so DeviceLost /
// Uncaptured callbacks can identify this logical device even before the native
// handle exists. Call bindDeviceSlot after RequestDevice succeeds.
func allocDeviceSlot() uintptr {
	id := deviceSlotSeq.Add(1)
	if id == 0 {
		id = deviceSlotSeq.Add(1)
	}
	slot := uintptr(id)
	// Placeholder until bindDeviceSlot; prevents free-slot reuse races.
	deviceSlots.Store(slot, (*Device)(nil))
	return slot
}

// bindDeviceSlot associates a userdata slot with the created *Device.
// bindDeviceSlot associates a userdata slot with the created *Device.
func bindDeviceSlot(slot uintptr, d *Device) {
	if slot == 0 || d == nil {
		return
	}
	d.callbackUserdata = slot
	deviceSlots.Store(slot, d)
}

// freeDeviceSlot drops userdata routing for a released or failed device.
// freeDeviceSlot drops userdata routing for a released or failed device.
func freeDeviceSlot(slot uintptr) {
	if slot == 0 {
		return
	}
	deviceSlots.Delete(slot)
}

// deviceFromUserdata resolves the *Device registered for a callback Userdata1.
// deviceFromUserdata resolves the *Device registered for a callback Userdata1.
func deviceFromUserdata(userdata1 uintptr) *Device {
	if userdata1 == 0 {
		return nil
	}
	if v, ok := deviceSlots.Load(userdata1); ok {
		if d, ok := v.(*Device); ok {
			return d // may be nil while RequestDevice is in flight
		}
	}
	return nil
}

// registerLiveDevice records handle → *Device for handle-based lost routing.
// A newly registered device is healthy: any stale sticky lost mark for this
// handle is dropped (tests reuse fake handles; native handles are unique).
// registerLiveDevice records handle → *Device for handle-based lost routing.
// A newly registered device is healthy: any stale sticky lost mark for this
// handle is dropped (tests reuse fake handles; native handles are unique).
func registerLiveDevice(d *Device) {
	if d == nil || d.handle == 0 {
		return
	}
	lostDeviceHandles.Delete(d.handle)
	d.lost.Store(false)
	liveDevices.Store(d.handle, d)
	if d.callbackUserdata != 0 {
		deviceSlots.Store(d.callbackUserdata, d)
	}
}

// unregisterLiveDevice removes a device from callback routing (on Release).
// unregisterLiveDevice removes a device from callback routing (on Release).
func unregisterLiveDevice(d *Device) {
	if d == nil {
		return
	}
	if d.handle != 0 {
		liveDevices.Delete(d.handle)
	}
	if d.callbackUserdata != 0 {
		freeDeviceSlot(d.callbackUserdata)
		d.callbackUserdata = 0
	}
}

// markDeviceLost sets Device.lost for the registered handle only (per-device).
