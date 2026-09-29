//go:build linux && !(js && wasm)

package egl

import "testing"

// No extension entry points means no devices, not an error: callers
// keep the platform display path. Forces the proc-missing branch via
// the resolver seam so the test needs no hardware.
func TestQueryDevicesNoExtensionFailsOpen(t *testing.T) {
	orig := deviceProcResolver
	deviceProcResolver = func(string) uintptr { return 0 }
	defer func() { deviceProcResolver = orig }()

	if got := QueryDevices(); got != nil {
		t.Fatalf("QueryDevices without entry points = %v, want nil", got)
	}
	if got := FindDiscreteDevice(); got != 0 {
		t.Fatalf("FindDiscreteDevice without entry points = %#x, want 0", got)
	}
	if got := PlatformDisplayForDevice(0); got != NoDisplay {
		t.Fatalf("PlatformDisplayForDevice(0) = %v, want NoDisplay", got)
	}
}
