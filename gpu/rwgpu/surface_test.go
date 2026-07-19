package rwgpu

import (
	"testing"
)

// TestSurfaceGetCapabilities_NilSurface tests nil safety for surface.
// Does not require native library — nil/zero guards run before checkInit.
func TestSurfaceGetCapabilities_NilSurface(t *testing.T) {
	var surface *Surface
	_, err := surface.GetCapabilities(&Adapter{handle: 1})
	if err == nil {
		t.Error("Expected error for nil surface, got nil")
	}
	surface = &Surface{handle: 0}
	_, err = surface.GetCapabilities(&Adapter{handle: 1})
	if err == nil {
		t.Error("Expected error for zero-handle surface, got nil")
	}
}

// TestSurfaceGetCapabilities_NilAdapter tests nil safety for adapter.
func TestSurfaceGetCapabilities_NilAdapter(t *testing.T) {
	// We cannot create a real surface without a window, so we just test nil adapter
	surface := &Surface{handle: 1} // fake handle
	_, err := surface.GetCapabilities(nil)
	if err == nil {
		t.Error("Expected error for nil adapter, got nil")
	}
}

// Note: Full integration testing of GetCapabilities requires a real window surface,
// which is tested in the examples (e.g., examples/triangle).
