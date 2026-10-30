//go:build !nogpu

package gpu

import (
	"testing"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// pool_purge_lock_test.go pins the P2.4d purge / OOM chain for the device-level
// shared pools. Two things must hold: GPUShared's purge frees both pooled
// domains through the render.PurgeEvictable registry, and it never takes s.mu —
// the texture-OOM retry runs inside create calls that already hold the shared
// lock, so a purge that grabbed it would deadlock on itself.

// newPooledShared returns a GPUShared on the noop device with the dual-tex and
// filter-graph domains bound (registry owner + submission gate installed).
func newPooledShared(t *testing.T) (hal.Device, *GPUShared) {
	t.Helper()
	device := newNoopTestDevice(t)
	s := NewGPUShared()
	s.mu.Lock()
	s.device = device
	s.queue = device.Queue()
	s.deviceReady = true
	// The test owns the noop device (its cleanup releases it), so Close must
	// treat it as an external device instead of releasing it twice.
	s.externalDevice = true
	s.dualTexBlendLocked()
	s.filterGPULocked()
	s.mu.Unlock()
	return device, s
}

func dualTexOf(s *GPUShared) *dualTexBlendCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dualTexBlendLocked()
}

func filterOf(s *GPUShared) *filterGPUCache {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.filterGPULocked()
}

// poolSnap takes a dual-tex snap of the quantized bucket of (w,h).
func poolSnap(t *testing.T, s *GPUShared, device hal.Device, w, h int) dualTexViewBlendOut {
	t.Helper()
	tex, view, ref, err := dualTexOf(s).getOutBGRA(device, device.Queue(), w, h)
	if err != nil {
		t.Fatalf("dual-tex getOutBGRA(%d,%d): %v", w, h, err)
	}
	if tex == nil || ref.IsNil() {
		t.Fatalf("dual-tex getOutBGRA(%d,%d): pooled path not taken", w, h)
	}
	return dualTexViewBlendOut{tex: tex, view: view, ref: ref}
}

func recycleSnap(t *testing.T, s *GPUShared, o dualTexViewBlendOut) {
	t.Helper()
	dualTexOf(s).recycleOut(o, nil)
}

// poolRT takes one filter-graph render target.
func poolRT(t *testing.T, s *GPUShared, device hal.Device, w, h int) res.Ref {
	t.Helper()
	_, _, ref, err := filterOf(s).acquireRT(device, w, h)
	if err != nil {
		t.Fatalf("filter acquireRT(%d,%d): %v", w, h, err)
	}
	if ref.IsNil() {
		t.Fatalf("filter acquireRT(%d,%d): pooled path not taken", w, h)
	}
	return ref
}

func recycleRT(t *testing.T, s *GPUShared, ref res.Ref) {
	t.Helper()
	filterOf(s).recycleRT(ref, nil)
}

func dualTexDomain(t *testing.T, s *GPUShared) *res.Cache {
	t.Helper()
	c := dualTexOf(s)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.outCache == nil {
		t.Fatal("dual-tex snap cache was never built")
	}
	return c.outCache
}

func filterDomain(t *testing.T, s *GPUShared) *res.Cache {
	t.Helper()
	c := filterOf(s)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rtCache == nil {
		t.Fatal("filter RT cache was never built")
	}
	return c.rtCache
}

// purgeUnderSharedLock runs the device purge while the caller holds s.mu, the
// exact state of a texture-OOM retry. A purge that reached for s.mu would block
// forever (Go mutexes are not reentrant), so the timeout is the assertion.
func purgeUnderSharedLock(t *testing.T, s *GPUShared) int64 {
	t.Helper()
	done := make(chan int64, 1)
	go func() { done <- s.PurgeEvictable() }()
	select {
	case freed := <-done:
		return freed
	case <-time.After(5 * time.Second):
		t.Fatal("PurgeEvictable blocked while GPUShared.mu was held: the purge path must take only the pool locks")
		return 0
	}
}

// TestGPUShared_PurgeEvictable_NeverTakesSharedLock is the lock-order pin plus
// the two-domain outlet: one pooled entry per domain is freed, one checked-out
// entry per domain survives.
func TestGPUShared_PurgeEvictable_NeverTakesSharedLock(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	snapHold := poolSnap(t, s, device, 256, 256)
	snapPool := poolSnap(t, s, device, 256, 256)
	rtHold := poolRT(t, s, device, 128, 128)
	rtPool := poolRT(t, s, device, 128, 128)
	recycleSnap(t, s, snapPool)
	recycleRT(t, s, rtPool)
	if snapHold.view == nil || snapHold.tex == nil || rtHold.IsNil() {
		t.Fatal("the checked-out snap / RT must stay valid alongside the pooled ones")
	}

	dt, ft := dualTexDomain(t, s), filterDomain(t, s)
	if st := dt.Stats(); st.Entries != 2 || st.Idle != 1 {
		t.Fatalf("dual-tex before purge: entries=%d idle=%d, want 2/1", st.Entries, st.Idle)
	}
	if st := ft.Stats(); st.Entries != 2 || st.Idle != 1 {
		t.Fatalf("filter before purge: entries=%d idle=%d, want 2/1", st.Entries, st.Idle)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if freed := purgeUnderSharedLock(t, s); freed != 2 {
		t.Fatalf("purge freed %d entries, want 2 (one per pooled domain)", freed)
	}
	// Read the domains through their caches directly: the helpers above would
	// take s.mu, which this frame still holds.
	if st := dt.Stats(); st.Entries != 1 || st.Idle != 0 {
		t.Fatalf("dual-tex after purge: entries=%d idle=%d, want the checked-out snap only", st.Entries, st.Idle)
	}
	if st := ft.Stats(); st.Entries != 1 || st.Idle != 0 {
		t.Fatalf("filter after purge: entries=%d idle=%d, want the checked-out RT only", st.Entries, st.Idle)
	}
	if st := dt.Stats(); st.Purged != 1 {
		t.Fatalf("dual-tex purged=%d, want 1", st.Purged)
	}
	if st := ft.Stats(); st.Purged != 1 {
		t.Fatalf("filter purged=%d, want 1", st.Purged)
	}
}

// TestGPUShared_PoolsAreOnTheOOMChain proves the wiring end-to-end through the
// registry the create-retry actually calls, not just the method on GPUShared.
func TestGPUShared_PoolsAreOnTheOOMChain(t *testing.T) {
	device, s := newPooledShared(t)
	t.Cleanup(s.Close)

	// Drain whatever an earlier test in this binary pooled, so the count below
	// is this holder's two entries only (the registry aggregates by name).
	render.PurgeEvictables()
	recycleSnap(t, s, poolSnap(t, s, device, 64, 64))
	recycleRT(t, s, poolRT(t, s, device, 64, 64))

	_, byName := render.PurgeEvictables()
	if got := byName["gpu-shared-pools"]; got != 2 {
		t.Fatalf("OOM chain freed %d entries under gpu-shared-pools, want 2 (snap + filter RT): %+v", got, byName)
	}
	if st := dualTexDomain(t, s).Stats(); st.Entries != 0 {
		t.Fatalf("dual-tex entries=%d after chain purge, want 0", st.Entries)
	}
	if st := filterDomain(t, s).Stats(); st.Entries != 0 {
		t.Fatalf("filter entries=%d after chain purge, want 0", st.Entries)
	}
}
