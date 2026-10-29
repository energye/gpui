//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"sync"

	"github.com/energye/gpui/gpu/hal"
)

// Device creation and adapter selection live in the render package's single
// funnel (render/adapter_policy.go policy + render/device_open.go
// OpenStandaloneDevice + render/backend.go createNativeInstance). This file
// keeps only the minimal device holder below — the P2.4a mirror helpers
// (renderDeviceDescriptor/requestDeviceWithRetry) and the never-called
// GPUInfo/createDevice/getDeviceQueue/releaseAdapter cluster were removed;
// consumers go through render.OpenStandaloneDevice / render.DeviceDescriptor.

// Backend is a minimal device holder for legacy command/texture helpers.
// Device creation and scene rendering are owned by VelloAccelerator; this type
// only carries a device/queue pair so CoreCommandEncoder and GPUTexture keep
// working.
type Backend struct {
	mu sync.RWMutex

	device hal.Device
	queue  hal.Queue

	initialized bool
}

// IsInitialized reports whether the backend holds a usable device.
func (b *Backend) IsInitialized() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.initialized
}

// Device returns the logical device.
func (b *Backend) Device() hal.Device {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.device
}

// Queue returns the device queue.
func (b *Backend) Queue() hal.Queue {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queue
}

// Close releases the device. Surviving paths never attach an adapter or
// instance, so only the device needs releasing.
func (b *Backend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.initialized {
		return
	}

	if b.device != nil {
		b.device.Release()
		b.device = nil
	}

	b.queue = nil
	b.initialized = false
}
