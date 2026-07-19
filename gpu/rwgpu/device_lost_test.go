package rwgpu

import (
	"errors"
	"testing"
	"unsafe"
)

// testDevice builds a registered Device shell for package tests (not a public API).
// Each test device gets its own callback userdata slot (multi-window isolation).
func testDevice(handle uintptr) *Device {
	d := &Device{handle: handle}
	slot := allocDeviceSlot()
	bindDeviceSlot(slot, d)
	registerLiveDevice(d)
	return d
}

// testDeviceCleanup unregisters handle + userdata slot.
func testDeviceCleanup(d *Device) {
	unregisterLiveDevice(d)
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
	// Prefer userdata slot (multi-window); handle is secondary.
	deviceLostHandler(uintptr(unsafe.Pointer(&devSlot)), 1,
		uintptr(unsafe.Pointer(&b[0])), ^uintptr(0), d.callbackUserdata, 0)

	if !d.IsLost() {
		t.Fatal("DeviceLostCallback must set Device.lost")
	}
	if other.IsLost() {
		t.Fatal("other device must remain healthy")
	}
}

func TestDeviceLostHandler_UserdataIsolatesDevices(t *testing.T) {
	d1 := testDevice(0xa001)
	d2 := testDevice(0xa002)
	t.Cleanup(func() {
		unregisterLiveDevice(d1)
		unregisterLiveDevice(d2)
	})

	// Lost for d1 via userdata only (devicePtr unresolvable → 0).
	deviceLostHandler(0, 1, 0, 0, d1.callbackUserdata, 0)
	if !d1.IsLost() {
		t.Fatal("userdata must mark d1 lost")
	}
	if d2.IsLost() {
		t.Fatal("userdata must not mark d2 lost")
	}
}

func TestUncapturedMarksLostWhenMessageMatches(t *testing.T) {
	const h uintptr = 0x7777
	d := testDevice(h)
	other := testDevice(0x7778)
	t.Cleanup(func() {
		unregisterLiveDevice(d)
		unregisterLiveDevice(other)
	})

	devSlot := h
	msg := "Parent device is lost"
	b := append([]byte(msg), 0)
	uncapturedErrorHandler(uintptr(unsafe.Pointer(&devSlot)), uintptr(ErrorTypeValidation),
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(msg)), d.callbackUserdata, 0)
	if !d.IsLost() {
		t.Fatal("uncaptured device-lost message must mark owning device lost")
	}
	if other.IsLost() {
		t.Fatal("uncaptured must not mark unrelated devices lost")
	}
	if err := refuseIfLost("CreateTexture", h); !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("after uncaptured lost refuseIfLost=%v want ErrDeviceLost", err)
	}
}

func TestUncapturedNonLostMessageDoesNotMarkLost(t *testing.T) {
	const h uintptr = 0x7779
	d := testDevice(h)
	t.Cleanup(func() { unregisterLiveDevice(d) })

	devSlot := h
	msg := "Validation error: invalid bind group"
	b := append([]byte(msg), 0)
	uncapturedErrorHandler(uintptr(unsafe.Pointer(&devSlot)), uintptr(ErrorTypeValidation),
		uintptr(unsafe.Pointer(&b[0])), uintptr(len(msg)), 0, 0)
	if d.IsLost() {
		t.Fatal("ordinary validation uncaptured must not mark lost")
	}
}

func TestMarkDeviceLostFromCallback_DoesNotPoisonOtherDevices(t *testing.T) {
	// Unresolved callback must NOT mark all live devices (multi-window rule).
	d1 := testDevice(0xc001)
	d2 := testDevice(0xc002)
	t.Cleanup(func() {
		unregisterLiveDevice(d1)
		unregisterLiveDevice(d2)
	})
	markDeviceLostFromCallback(0, 0)
	if d1.IsLost() || d2.IsLost() {
		t.Fatal("unresolved lost must not poison unrelated devices")
	}
	// Userdata routes only one device.
	markDeviceLostFromCallback(0, d1.callbackUserdata)
	if !d1.IsLost() || d2.IsLost() {
		t.Fatal("userdata must mark only the owning device")
	}
}

func TestSurfaceParentLost_AbsorbsUncapturedMessage(t *testing.T) {
	const h uintptr = 0xc010
	d := testDevice(h)
	other := testDevice(0xc011)
	t.Cleanup(func() {
		unregisterLiveDevice(d)
		unregisterLiveDevice(other)
	})

	// Inject sticky uncaptured attributed to this surface's device.
	lastUncapturedMu.Lock()
	lastUncapturedTyp = ErrorTypeValidation
	lastUncapturedMsg = "Parent device is lost"
	lastUncapturedDevice = h
	lastUncapturedMu.Unlock()

	s := &Surface{handle: 1, device: h, deviceRef: d}
	if !s.surfaceParentLost() {
		t.Fatal("surfaceParentLost must absorb uncaptured device-lost message")
	}
	if !d.IsLost() {
		t.Fatal("device must be sticky-lost after absorb")
	}
	if other.IsLost() {
		t.Fatal("other window device must stay healthy")
	}
	// GetCurrentTexture must refuse without native.
	_, _, err := s.GetCurrentTexture()
	if !errors.Is(err, ErrSurfaceDeviceLost) && !errors.Is(err, ErrDeviceLost) {
		t.Fatalf("GetCurrentTexture after uncaptured lost: %v", err)
	}
}

func TestSurfaceParentLost_IgnoresOtherDeviceUncaptured(t *testing.T) {
	const hA, hB uintptr = 0xc020, 0xc021
	dA := testDevice(hA)
	dB := testDevice(hB)
	t.Cleanup(func() {
		unregisterLiveDevice(dA)
		unregisterLiveDevice(dB)
	})

	// Uncaptured lost belongs to device A; surface B must ignore it.
	lastUncapturedMu.Lock()
	lastUncapturedTyp = ErrorTypeValidation
	lastUncapturedMsg = "Parent device is lost"
	lastUncapturedDevice = hA
	lastUncapturedMu.Unlock()

	sB := &Surface{handle: 2, device: hB, deviceRef: dB}
	if sB.surfaceParentLost() {
		t.Fatal("surface B must not absorb device A uncaptured lost")
	}
	if dB.IsLost() {
		t.Fatal("device B must not be marked lost")
	}
}

func TestLooksLikeDeviceLost_Messages(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"", false},
		{"Parent device is lost", true},
		{"device lost", true},
		{"DEVICE_REMOVED", true},
		{"vk_error_device_lost", true},
		{"validation: bad shader", false},
	}
	for _, tc := range cases {
		if got := looksLikeDeviceLost(tc.msg); got != tc.want {
			t.Errorf("looksLikeDeviceLost(%q)=%v want %v", tc.msg, got, tc.want)
		}
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
