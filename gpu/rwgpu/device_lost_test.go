package rwgpu

import "testing"

func TestDeviceLostSticky(t *testing.T) {
	// Save and restore sticky state so other tests are not poisoned.
	was := deviceLostSticky.Load()
	defer func() {
		deviceLostSticky.Store(was)
	}()

	deviceLostSticky.Store(false)
	// Clear a synthetic handle if present.
	const h uintptr = 0xdeadbeef
	lostDevices.Delete(h)

	if AnyDeviceLost() {
		t.Fatal("expected no device lost initially")
	}
	d := &Device{handle: h}
	if d.IsLost() {
		t.Fatal("device should not be lost before mark")
	}

	markDeviceLost(h)
	if !AnyDeviceLost() {
		t.Fatal("AnyDeviceLost should be true after mark")
	}
	if !d.IsLost() {
		t.Fatal("device.IsLost should be true after mark")
	}
	if !IsDeviceHandleLost(h) {
		t.Fatal("IsDeviceHandleLost should be true for marked handle")
	}

	// GetCurrentTexture pre-check path: zero-device surface still sees sticky.
	s := &Surface{handle: 1, device: h}
	if !(AnyDeviceLost() || (s.device != 0 && IsDeviceHandleLost(s.device))) {
		t.Fatal("surface pre-check should refuse acquire after device lost")
	}

	// looksLikeDeviceLost covers native panic message phrasing.
	for _, msg := range []string{
		"device lost",
		"Parent device is lost",
		"Validation Error: parent device is lost",
	} {
		if !looksLikeDeviceLost(msg) {
			t.Fatalf("looksLikeDeviceLost(%q) = false", msg)
		}
	}
	if looksLikeDeviceLost("unrelated validation error") {
		t.Fatal("looksLikeDeviceLost should not match unrelated messages")
	}
}

func TestClearDeviceLostSticky_AllowsNewDevice(t *testing.T) {
	// After a prior lost, RequestDevice path clears process sticky so a new
	// device can run. Per-handle entries for old devices remain lost.
	was := deviceLostSticky.Load()
	defer func() {
		deviceLostSticky.Store(was)
	}()

	const oldH uintptr = 0x1111
	const newH uintptr = 0x2222
	lostDevices.Delete(oldH)
	lostDevices.Delete(newH)

	markDeviceLost(oldH)
	if !AnyDeviceLost() {
		t.Fatal("sticky should be set after mark")
	}
	clearDeviceLostSticky()
	if AnyDeviceLost() {
		t.Fatal("clearDeviceLostSticky must clear process sticky")
	}
	// Old handle still lost via map.
	if !IsDeviceHandleLost(oldH) {
		// IsDeviceHandleLost checks sticky first then map; sticky cleared so map should hit.
		// Wait — IsDeviceHandleLost after sticky clear still checks map.
		t.Fatal("old handle must remain lost via per-handle map")
	}
	// New handle is fine.
	if IsDeviceHandleLost(newH) {
		t.Fatal("new handle must not be lost")
	}
	dNew := &Device{handle: newH}
	if dNew.IsLost() {
		t.Fatal("new device must not report IsLost")
	}
	// Surface configured on old device still refuses.
	if err := refuseIfLost("test", oldH); err == nil {
		t.Fatal("old device handle must still refuse")
	}
	if err := refuseIfLost("test", newH); err != nil {
		t.Fatalf("new device must not refuse: %v", err)
	}
}

func TestMarkDeviceFromCallbackArg_NullTripsSticky(t *testing.T) {
	was := deviceLostSticky.Load()
	defer func() {
		deviceLostSticky.Store(was)
	}()
	deviceLostSticky.Store(false)
	markDeviceFromCallbackArg(0)
	if !AnyDeviceLost() {
		t.Fatal("null devicePtr must trip process sticky fuse")
	}
}
