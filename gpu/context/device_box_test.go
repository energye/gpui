// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build !(js && wasm)

package context

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	gputypes "github.com/energye/gpui/gpu/types"
)

func openNoopDevice(t *testing.T) hal.Device {
	t.Helper()
	be := noop.NewBackend()
	inst, err := be.CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	t.Cleanup(inst.Release)
	adapters := inst.EnumerateAdapters(nil)
	if len(adapters) == 0 {
		t.Fatal("noop: no adapters")
	}
	opened, err := adapters[0].Adapter.Open(0, gputypes.Limits{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { opened.Device.Release() })
	return opened.Device
}

func TestPackUnpackDeviceRoundtrip(t *testing.T) {
	dev := openNoopDevice(t)
	h := PackDevice(dev)
	if h.IsNil() {
		t.Fatal("PackDevice returned nil handle for non-nil device")
	}
	if got := UnpackDevice(h); got != hal.Device(dev) {
		t.Fatalf("UnpackDevice mismatch")
	}
}

func TestPackUnpackQueueRoundtrip(t *testing.T) {
	dev := openNoopDevice(t)
	q := dev.Queue()
	if q == nil {
		t.Skip("noop device has nil queue")
	}
	h := PackQueue(q)
	if h.IsNil() {
		t.Fatal("PackQueue returned nil handle for non-nil queue")
	}
	if got := UnpackQueue(h); got != hal.Queue(q) {
		t.Fatalf("UnpackQueue mismatch")
	}
}

func TestPackUnpackAdapterRoundtrip(t *testing.T) {
	be := noop.NewBackend()
	inst, err := be.CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	t.Cleanup(inst.Release)
	adapters := inst.EnumerateAdapters(nil)
	if len(adapters) == 0 {
		t.Fatal("noop: no adapters")
	}
	ad := adapters[0].Adapter
	h := PackAdapter(ad)
	if h.IsNil() {
		t.Fatal("PackAdapter returned nil handle for non-nil adapter")
	}
	if got := UnpackAdapter(h); got != hal.Adapter(ad) {
		t.Fatalf("UnpackAdapter mismatch")
	}
}

func TestPackNilYieldsNilHandle(t *testing.T) {
	if h := PackDevice(nil); !h.IsNil() {
		t.Fatal("PackDevice(nil) = non-nil handle")
	}
	if h := PackQueue(nil); !h.IsNil() {
		t.Fatal("PackQueue(nil) = non-nil handle")
	}
	if h := PackAdapter(nil); !h.IsNil() {
		t.Fatal("PackAdapter(nil) = non-nil handle")
	}
	if got := UnpackDevice(Device{}); got != nil {
		t.Fatal("UnpackDevice(zero) = non-nil")
	}
	if got := UnpackQueue(Queue{}); got != nil {
		t.Fatal("UnpackQueue(zero) = non-nil")
	}
	if got := UnpackAdapter(Adapter{}); got != nil {
		t.Fatal("UnpackAdapter(zero) = non-nil")
	}
}

func TestUnpackForeignFailsClosed(t *testing.T) {
	var foreign int
	fdev := NewDevice(unsafe.Pointer(&foreign))
	if got := UnpackDevice(fdev); got != nil {
		t.Fatal("UnpackDevice(foreign) = non-nil, want nil")
	}
	fq := NewQueue(unsafe.Pointer(&foreign))
	if got := UnpackQueue(fq); got != nil {
		t.Fatal("UnpackQueue(foreign) = non-nil, want nil")
	}
	fa := NewAdapter(unsafe.Pointer(&foreign))
	if got := UnpackAdapter(fa); got != nil {
		t.Fatal("UnpackAdapter(foreign) = non-nil, want nil")
	}
}
