//go:build !(js && wasm)

package webgpu

import "testing"

func TestSurfaceAcquireEpoch_BumpInvalidatesInFlight(t *testing.T) {
	s := &Surface{}
	epoch := s.acquireEpoch.Load()
	if s.acquireEpochStale(epoch) {
		t.Fatal("fresh acquire epoch must be current")
	}

	s.BumpAcquireEpoch()
	if !s.acquireEpochStale(epoch) {
		t.Fatal("bump must invalidate the in-flight epoch")
	}
	if s.acquireEpochStale(s.acquireEpoch.Load()) {
		t.Fatal("new acquire epoch must be current")
	}

	var nilSurface *Surface
	nilSurface.BumpAcquireEpoch() // nil-safe
	if !nilSurface.acquireEpochStale(0) {
		t.Fatal("nil surface has no current acquire epoch")
	}
}

func TestIsSkipFrameSurfaceErr_AcquireTimeout(t *testing.T) {
	if !isSkipFrameSurfaceErr(ErrAcquireTimeout) {
		t.Fatal("acquire timeout must skip the frame, not reconfigure while the acquire is in flight")
	}
}
