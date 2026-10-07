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
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// Device represents a logical GPU device.
// On the wgpu-native backend, this wraps rwgpu Device.
type Device struct {
	r        *rwgpu.Device
	instance *Instance // stored for PopErrorScope which needs Instance handle
	queue    *Queue
	features Features
	limits   Limits
	released bool
}
