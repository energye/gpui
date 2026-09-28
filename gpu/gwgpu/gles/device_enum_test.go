//go:build linux && !nogpu

package gles

import (
	"testing"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
)

// Enumeration must never kill the process: unsupported drivers answer
// empty (fallback to the platform display), and every reported device
// carries a handle. Regression for two kills found while adding it:
// GetSymbol on libEGL.so.1 (no such symbol, stale pointer) and the
// 3-arg ffi tail call (addr 0x1000000000) — both SIGSEGV'd the test
// binary before the contract was pinned to empty-on-unsupported.
func TestEGLDeviceEnumeration(t *testing.T) {
	if err := egl.Init(); err != nil {
		t.Skipf("egl init: %v", err)
	}
	if !egl.DeviceEnumerationSupported() {
		t.Skip("no EGL_EXT_device_enumeration")
	}
	for i, d := range egl.QueryDevices() {
		t.Logf("egl device[%d] nvidia=%v mesa=%v name=%q", i, d.IsNVIDIA, d.IsMesa, d.Name)
		if d.Device == 0 {
			t.Fatalf("device[%d] has nil handle", i)
		}
	}
}
