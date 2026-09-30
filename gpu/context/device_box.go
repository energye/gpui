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

package context

import (
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
)

// Device/Queue/Adapter handles gain a boxed hal-interface encoding so any
// backend can travel through the opaque
// gpucontext handles without concrete type asserts.
//
// Legacy producers pack raw concrete pointers; Unpack
// fails those closed (magic mismatch) and callers fall back to the legacy
// concrete assert. New producers pack with PackDevice/PackQueue/
// PackAdapter here. Same precedent as texture_view_box.go (H3 black-screen
// fix): never put a foreign concrete pointer directly into a handle.

const (
	deviceBoxMagic  uint64 = 0x6770756465766964 // "gpudevid" in hex
	queueBoxMagic   uint64 = 0x6770757175657565 // "gpuqueue" in hex
	adapterBoxMagic uint64 = 0x6770756164617074 // "gpuadapt" in hex
)

type deviceBox struct {
	magic uint64
	v     hal.Device
}

type queueBox struct {
	magic uint64
	v     hal.Queue
}

type adapterBox struct {
	magic uint64
	v     hal.Adapter
}

// PackDevice boxes a hal.Device into an opaque handle.
// A nil device yields the zero handle.
func PackDevice(v hal.Device) Device {
	if v == nil {
		return Device{}
	}
	b := &deviceBox{magic: deviceBoxMagic, v: v}
	return NewDevice(unsafe.Pointer(b)) //nolint:gosec // opaque handle pattern
}

// UnpackDevice recovers the hal.Device from an opaque handle.
// Returns nil for nil handles and for legacy raw-pointer handles
// (fail closed, never garbage).
func UnpackDevice(h Device) hal.Device {
	if h.IsNil() {
		return nil
	}
	b := (*deviceBox)(h.Pointer())
	if b == nil || b.magic != deviceBoxMagic {
		return nil
	}
	return b.v
}

// PackQueue boxes a hal.Queue into an opaque handle.
// A nil queue yields the zero handle.
func PackQueue(v hal.Queue) Queue {
	if v == nil {
		return Queue{}
	}
	b := &queueBox{magic: queueBoxMagic, v: v}
	return NewQueue(unsafe.Pointer(b)) //nolint:gosec // opaque handle pattern
}

// UnpackQueue recovers the hal.Queue from an opaque handle.
// Returns nil for nil handles and for legacy raw-pointer handles.
func UnpackQueue(h Queue) hal.Queue {
	if h.IsNil() {
		return nil
	}
	b := (*queueBox)(h.Pointer())
	if b == nil || b.magic != queueBoxMagic {
		return nil
	}
	return b.v
}

// PackAdapter boxes a hal.Adapter into an opaque handle.
// A nil adapter yields the zero handle.
func PackAdapter(v hal.Adapter) Adapter {
	if v == nil {
		return Adapter{}
	}
	b := &adapterBox{magic: adapterBoxMagic, v: v}
	return NewAdapter(unsafe.Pointer(b)) //nolint:gosec // opaque handle pattern
}

// UnpackAdapter recovers the hal.Adapter from an opaque handle.
// Returns nil for nil handles and for legacy raw-pointer handles.
func UnpackAdapter(h Adapter) hal.Adapter {
	if h.IsNil() {
		return nil
	}
	b := (*adapterBox)(h.Pointer())
	if b == nil || b.magic != adapterBoxMagic {
		return nil
	}
	return b.v
}
