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
