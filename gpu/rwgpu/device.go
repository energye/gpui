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
)

// RequestDeviceCallbackInfo holds callback configuration for RequestDevice.
type RequestDeviceCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Mode        CallbackMode
	Callback    uintptr // Function pointer
	Userdata1   uintptr
	Userdata2   uintptr
}

// deviceRequest holds state for an async device request.
type deviceRequest struct {
	done    chan struct{}
	device  *Device
	status  RequestDeviceStatus
	message string
}

var (
	// deviceRequests is the global registry for pending device requests.
	// Protected by deviceRequestsMu for concurrent access.
	deviceRequests   = make(map[uintptr]*deviceRequest)
	deviceRequestsMu sync.Mutex
	deviceRequestID  uintptr

	// deviceCallbackPtr is the callback function pointer (created once).
	// Protected by deviceCallbackOnce for concurrent initialization.
	deviceCallbackPtr  uintptr
	deviceCallbackOnce sync.Once
)
