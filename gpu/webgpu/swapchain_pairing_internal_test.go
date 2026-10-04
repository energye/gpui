//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"testing"
)

// Swapchain acquire/present pairing (Skia parity: every successful acquire
// ends in exactly one present or discard; failure paths move no counters).
// A phantom acquire (counter moved, no frame tracked) or phantom release
// leaves the swapchain believing a buffer is in flight: next BeginFrame
// fails, the raster loop stalls on acquire timeout, and a recycled buffer
// can show stale content. These tests pin the failure paths headless.
func TestSwapchainPairing_FailurePathsMoveNoCounters(t *testing.T) {
	sc := NewSwapchain(nil, nil, 64, 64)
	baseline := sc.Stats()
	if baseline.Acquires != 0 || baseline.Presents != 0 || baseline.Discards != 0 {
		t.Fatalf("fresh stats = %+v, want zeros", baseline)
	}

	// No surface: BeginFrame errors before acquiring.
	if _, err := sc.BeginFrame(); err == nil {
		t.Fatal("BeginFrame without surface must error")
	}
	// Discard of nil: no-op, no counter.
	sc.DiscardFrame(nil)
	// EndFrame without an open frame: rejected, no counter.
	if err := sc.EndFrame(&Frame{}); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("EndFrame without open frame = %v, want ErrNoFrame", err)
	}
	after := sc.Stats()
	if after != baseline {
		t.Fatalf("failure paths moved counters: before=%+v after=%+v", baseline, after)
	}
}

func TestSwapchainPairing_EmptyDiscardDocumentsSemantics(t *testing.T) {
	sc := NewSwapchain(nil, nil, 64, 64)
	// Discarding an empty (never-acquired) frame is safe and clears the open
	// flag; it records one discard. Callers only discard acquired frames, so
	// production pairing (acquires == presents + discards + open) is exact.
	sc.DiscardFrame(&Frame{})
	st := sc.Stats()
	if st.Discards != 1 || st.Acquires != 0 || st.Presents != 0 {
		t.Fatalf("empty discard stats = %+v, want {0 0 1}", st)
	}
	if err := sc.EndFrame(&Frame{}); !errors.Is(err, ErrNoFrame) {
		t.Fatalf("EndFrame after discard = %v, want ErrNoFrame", err)
	}
}
