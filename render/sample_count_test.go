package render

import "testing"

// TestMSAASampleCountConstants pins the documented default: engine = 1x,
// 4x is opt-in (docs/RENDER_API_CATALOG.md §9 常量总表).
func TestMSAASampleCountConstants(t *testing.T) {
	if MSAASampleCount1 != 1 {
		t.Fatalf("MSAASampleCount1=%d want 1", MSAASampleCount1)
	}
	if MSAASampleCount4 != 4 {
		t.Fatalf("MSAASampleCount4=%d want 4", MSAASampleCount4)
	}
}

// TestMSAASampleCount_DefaultUnset verifies the unset state returns the
// engine default of 1x — callers use the result directly, no extra defaults.
func TestMSAASampleCount_DefaultUnset(t *testing.T) {
	SetMSAASampleCount(0) // reset
	if got := MSAASampleCount(); got != MSAASampleCount1 {
		t.Fatalf("MSAASampleCount()=%d want %d (engine default 1x)", got, MSAASampleCount1)
	}
}

// TestMSAASampleCount_SetAndReset covers the code-config contract:
// set 1x/4x, read back, and reset to the engine default (1x).
func TestMSAASampleCount_SetAndReset(t *testing.T) {
	SetMSAASampleCount(0)
	defer SetMSAASampleCount(0)

	SetMSAASampleCount(MSAASampleCount1)
	if got := MSAASampleCount(); got != MSAASampleCount1 {
		t.Fatalf("after Set(1): MSAASampleCount()=%d want 1", got)
	}

	SetMSAASampleCount(MSAASampleCount4)
	if got := MSAASampleCount(); got != MSAASampleCount4 {
		t.Fatalf("after Set(4): MSAASampleCount()=%d want 4", got)
	}

	SetMSAASampleCount(0)
	if got := MSAASampleCount(); got != MSAASampleCount1 {
		t.Fatalf("after reset: MSAASampleCount()=%d want %d (engine default 1x)", got, MSAASampleCount1)
	}
}
