package rwgpu

import (
	"errors"
	"testing"
	"unsafe"
)

// testDevice builds a registered Device shell for package tests (not a public API).
func testDevice(handle uintptr) *Device {
	d := &Device{handle: handle}
	registerLiveDevice(d)
	return d
}

func TestDeviceLostOnObject(t *testing.T) {
	const h uintptr = 0xdeadbeef
	const otherH uintptr = 0xcafebabe
	d := testDevice(h)
	other := testDevice(otherH)
	t.Cleanup(func() {
		unregisterLiveDevice(d)
		unregisterLiveDevice(other)
	})

	if d.IsLost() || other.IsLost() {
		t.Fatal("devices must not be lost initially")
	}

	markDeviceLost(h)
	if !d.IsLost() {
		t.Fatal("callback target device must be lost")
	}
	if other.IsLost() {
		t.Fatal("other device must remain healthy")
	}
	if err := refuseIfLost("test", h); err == nil {
		t.Fatal("refuseIfLost must refuse lost device")
	}
	if err := refuseIfLost("test", otherH); err != nil {
		t.Fatalf("healthy device must not refuse: %v", err)
	}
}

func TestNewDeviceNotPoisonedByOldLost(t *testing.T) {
	oldD := testDevice(0x1111)
	newD := testDevice(0x2222)
	t.Cleanup(func() {
		unregisterLiveDevice(oldD)
		unregisterLiveDevice(newD)
	})

	markDeviceLost(0x1111)
	if !oldD.IsLost() || newD.IsLost() {
		t.Fatal("lost must be isolated to the callback device")
	}
}

func TestDeviceLostHandler_MarksDeviceObject(t *testing.T) {
	const h uintptr = 0x5555
	d := testDevice(h)
	other := testDevice(0x6666)
	t.Cleanup(func() {
		unregisterLiveDevice(d)
		unregisterLiveDevice(other)
	})

	devSlot := h
	msg := "gpu reset"
	b := append([]byte(msg), 0)
	deviceLostHandler(uintptr(unsafe.Pointer(&devSlot)), 1,
		uintptr(unsafe.Pointer(&b[0])), ^uintptr(0), 0, 0)

	if !d.IsLost() {
		t.Fatal("DeviceLostCallback must set Device.lost")
	}
	if other.IsLost() {
		t.Fatal("other device must remain healthy")
	}
}

func TestUncapturedDoesNotMarkLost(t *testing.T) {
	const h uintptr = 0x7777
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })

	devSlot := h
	msg := "Parent device is lost"
	b := append([]byte(msg), 0)
	uncapturedErrorHandler(uintptr(unsafe.Pointer(&devSlot)), uintptr(ErrorTypeValidation),
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(msg)), 0, 0)
	if d.IsLost() {
		t.Fatal("uncaptured must not mark lost")
	}
}

func TestCallbackStringView_STRLEN(t *testing.T) {
	msg := "Parent device is lost"
	b := append([]byte(msg), 0)
	got := callbackStringView(uintptr(unsafe.Pointer(&b[0])), ^uintptr(0))
	if got != msg {
		t.Fatalf("STRLEN decode=%q want %q", got, msg)
	}
}

func TestRefuseIfLost_RequiresRegisteredDevice(t *testing.T) {
	if err := refuseIfLost("op", 0); err != nil {
		t.Fatalf("handle 0: %v", err)
	}
	if err := refuseIfLost("op", 0x9999); err != nil {
		t.Fatalf("unregistered handle: %v", err)
	}
	d := testDevice(0x8888)
	t.Cleanup(func() { unregisterLiveDevice(d) })
	markDeviceLost(0x8888)
	if !errors.Is(refuseIfLost("op", 0x8888), ErrDeviceLost) {
		t.Fatal("marked device must refuse")
	}
}
