//go:build !(js && wasm)

package webgpu

import (
	"errors"
	"testing"
	"time"
)

// acquireSurfaceTexture / acquireHungLocked are pure bookkeeping paths that
// must never touch the native surface while the acquire is latched hung —
// these tests pin that without a GPU (the stuck-cgo case is the point).

func TestAcquireSurfaceTexture_HungLatchReturnsImmediately(t *testing.T) {
	sc := &Swapchain{Surface: &Surface{}}
	sc.acquireHung.Store(true)

	t0 := time.Now()
	_, _, err := sc.acquireSurfaceTexture()
	if !errors.Is(err, ErrAcquireTimeout) {
		t.Fatalf("hung latch must return ErrAcquireTimeout, got %v", err)
	}
	if time.Since(t0) > 50*time.Millisecond {
		t.Fatalf("hung latch must not wait (no cgo), took %v", time.Since(t0))
	}
	// The latch stays set: subsequent calls fail fast too.
	if _, _, err := sc.acquireSurfaceTexture(); !errors.Is(err, ErrAcquireTimeout) {
		t.Fatalf("latch must persist, got %v", err)
	}
	if sc.acquireTimeouts != 0 {
		t.Fatalf("hung-latch fast path must not count a timeout, got %d", sc.acquireTimeouts)
	}
}

func TestAcquireSurfaceTexture_NilSurfaceGuard(t *testing.T) {
	sc := &Swapchain{}
	_, _, err := sc.acquireSurfaceTexture()
	if !errors.Is(err, ErrInvalidHandle) {
		t.Fatalf("nil surface must yield ErrInvalidHandle, got %v", err)
	}
}

func TestAcquireHungLocked_ThrottledRecreateWhileHung(t *testing.T) {
	sc := &Swapchain{Surface: &Surface{}}
	sc.acquireHung.Store(true)
	sc.hungMarkedAt = time.Now() // within the recreate throttle window

	if err := sc.acquireHungLocked(); !errors.Is(err, ErrAcquireTimeout) {
		t.Fatalf("hung gate must return ErrAcquireTimeout in throttle window, got %v", err)
	}
}

func TestAcquireHungLocked_NoGateWhenHealthy(t *testing.T) {
	sc := &Swapchain{Surface: &Surface{}}
	// acquireHung false → the gate is open (nil).
	if err := sc.acquireHungLocked(); err != nil {
		t.Fatalf("healthy swapchain must not be gated, got %v", err)
	}
}

func TestAcquireHungLocked_ReconfigureFailureKeepsLatch(t *testing.T) {
	sc := &Swapchain{Surface: &Surface{}}
	sc.acquireHung.Store(true)
	// Past the recreate throttle window → a recreate attempt runs; without a
	// real device Configure fails, so the gate stays closed and the latch
	// persists (the frame loop keeps failing fast instead of re-blocking).
	sc.hungMarkedAt = time.Now().Add(-time.Second)

	if err := sc.acquireHungLocked(); !errors.Is(err, ErrAcquireTimeout) {
		t.Fatalf("failed recreate must keep the gate closed, got %v", err)
	}
	if !sc.acquireHung.Load() {
		t.Fatal("latch must persist when reconfigure fails")
	}
}
