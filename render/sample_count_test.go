package render

import "testing"

// TestMSAASampleCountConstants pins the documented default: engine = 4x,
// opt-out = 1x (docs/RENDER_API_CATALOG.md §5 常量总表).
func TestMSAASampleCountConstants(t *testing.T) {
	if MSAASampleCount1 != 1 {
		t.Fatalf("MSAASampleCount1=%d want 1", MSAASampleCount1)
	}
	if MSAASampleCount4 != 4 {
		t.Fatalf("MSAASampleCount4=%d want 4", MSAASampleCount4)
	}
}

// TestDefaultSampleCount_DefaultUnset verifies the initial state is "auto" (0),
// meaning the engine falls back to env/probe/4x.
func TestDefaultSampleCount_DefaultUnset(t *testing.T) {
	SetDefaultSampleCount(0) // reset
	if got := DefaultSampleCount(); got != 0 {
		t.Fatalf("DefaultSampleCount()=%d want 0 (auto)", got)
	}
}

// TestDefaultSampleCount_SetAndReset covers the code-config contract:
// set 1x/4x, read back, and reset to auto.
func TestDefaultSampleCount_SetAndReset(t *testing.T) {
	SetDefaultSampleCount(0)
	defer SetDefaultSampleCount(0)

	SetDefaultSampleCount(MSAASampleCount1)
	if got := DefaultSampleCount(); got != MSAASampleCount1 {
		t.Fatalf("after Set(1): DefaultSampleCount()=%d want 1", got)
	}

	SetDefaultSampleCount(MSAASampleCount4)
	if got := DefaultSampleCount(); got != MSAASampleCount4 {
		t.Fatalf("after Set(4): DefaultSampleCount()=%d want 4", got)
	}

	SetDefaultSampleCount(0)
	if got := DefaultSampleCount(); got != 0 {
		t.Fatalf("after reset: DefaultSampleCount()=%d want 0", got)
	}
}
