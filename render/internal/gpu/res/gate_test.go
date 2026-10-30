package res

import (
	"sync"
	"testing"
)

// gate_test.go pins the device-level submission gate (P2.4d D3): the filter and
// dual-tex caches submit straight to the queue without a Session, so their
// pooled natives are held by a GateHandle until the hal completion token says
// the work finished.

func newGatedCache(budget uint64) (*Registry, *Cache, *fakeNativeTracker, *InflightGate) {
	reg := NewRegistry()
	tracker := &fakeNativeTracker{}
	c := NewCache(reg.DeviceOwner(), tracker.create, CacheOptions{Name: "gated", Budget: budget})
	return reg, c, tracker, NewInflightGate(reg)
}

// A held entry is neither re-lent nor destroyed: this is the exact window where
// a size flip could hand window B a texture window A is still copying.
func TestGate_HoldBlocksRelendAndDestroy(t *testing.T) {
	reg, c, tracker, gate := newGatedCache(0)
	k := CacheKey{W: 8, H: 8, Role: RoleLayerRT}

	ref, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	c.Release(ref) // idle, back in the pool
	if st := c.Stats(); st.Idle != 1 {
		t.Fatalf("idle=%d want 1 after release", st.Idle)
	}

	held, err := c.Acquire(k)
	if err != nil || held.IsNil() {
		t.Fatalf("second acquire: ref=%v err=%v", held, err)
	}
	gateHold := gate.Hold(held)
	if gateHold == nil {
		t.Fatal("hold must take a live ref")
	}
	if gate.Pending() != 1 {
		t.Fatalf("pending=%d want 1", gate.Pending())
	}
	// The caller returns it while the submission is still running.
	c.Release(held)
	if st := c.Stats(); st.Idle != 0 {
		t.Fatalf("a gated entry must not count as idle: idle=%d", st.Idle)
	}
	// A new request must miss rather than re-lend the in-flight native.
	if h, _ := c.HitRate(); h != 1 {
		t.Fatalf("hits=%d want 1 (the first re-pool only)", h)
	}
	ref3, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("third acquire: %v", err)
	}
	if h, m := c.HitRate(); h != 1 || m != 2 {
		t.Fatalf("in-flight entry was re-lent: hits=%d misses=%d want hits=1 misses=2", h, m)
	}
	c.Release(ref3)

	gateHold.Resolve()
	if gate.Pending() != 0 {
		t.Fatalf("pending=%d want 0 after resolve", gate.Pending())
	}
	// Now the first native is poolable again: a fresh same-key take hits.
	if n := reg.Count(); n != 2 {
		t.Fatalf("registry count=%d want 2 (both trios live)", n)
	}
	ref4, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("fourth acquire: %v", err)
	}
	if h, _ := c.HitRate(); h < 2 {
		t.Fatalf("hits=%d want >=2 after resolve", h)
	}
	c.Release(ref4)
	for _, f := range tracker.created {
		if f.released != 0 {
			t.Fatalf("nothing is retired yet, released=%d", f.released)
		}
	}
}

// Retirement while gated defers the native Release to the resolve point.
func TestGate_RetireWaitsForResolve(t *testing.T) {
	reg := NewRegistry()
	gate := NewInflightGate(reg)
	f := &fakeNative{}
	ref := reg.DeviceOwner().Register(f)
	gateHold := gate.Hold(ref)
	reg.Retire(ref)
	reg.Release(ref)
	if f.released != 0 {
		t.Fatalf("retired entry must wait for the gate, released=%d", f.released)
	}
	gateHold.Resolve()
	if f.released != 1 {
		t.Fatalf("resolve must finish the retirement, released=%d", f.released)
	}
}

// Device loss: the fence never arrives, so the hold is dropped without
// decrementing (the abandon flow owns native teardown).
func TestGate_InvalidateClearsWithoutRelease(t *testing.T) {
	reg, c, _, gate := newGatedCache(0)
	ref, err := c.Acquire(CacheKey{W: 4, H: 4, Role: RoleLayerRT})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	h := gate.Hold(ref)
	c.Release(ref)
	reg.Retire(ref)
	h.Invalidate()
	if reg.idleEntry(ref.id) {
		t.Fatal("invalidated entry must not become lendable while retired")
	}
	if n := reg.PendingReleaseCount(); n != 1 {
		t.Fatalf("pending release=%d want 1 (native teardown belongs to abandon)", n)
	}
	if f, ok := reg.NativeOf(ref).(*fakeNative); ok && f.released != 0 {
		t.Fatalf("invalidate must not release the native, released=%d", f.released)
	}
	gate.InvalidateAll()
	if gate.Pending() != 0 {
		t.Fatalf("invalidateAll must clear the gate, pending=%d", gate.Pending())
	}
}

// Idle is what the cache can actually lend or purge. A hold dropped by the
// loss path clears the registry's in-flight flag but never re-queues the entry
// into its key bucket (repool only runs from a real idle transition), so the
// snapshot must not count it — otherwise the probe would read an unusable
// entry as reuse potential.
func TestGate_InvalidateLeavesEntryUnpoolable(t *testing.T) {
	_, c, _, gate := newGatedCache(0)
	k := CacheKey{W: 6, H: 6, Role: RoleLayerRT}

	ref, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	c.Release(ref)
	if st := c.Stats(); st.Idle != 1 {
		t.Fatalf("idle=%d want 1 after release", st.Idle)
	}
	held, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("re-acquire: %v", err)
	}
	h := gate.Hold(held)
	if h == nil {
		t.Fatal("hold must take a live ref")
	}
	c.Release(held)

	h.Invalidate()
	st := c.Stats()
	if st.Idle != 0 {
		t.Fatalf("idle=%d want 0: a loss-cleared entry never sits back in the bucket", st.Idle)
	}
	// The next request must create rather than reuse the unusable entry.
	if _, err := c.Acquire(k); err != nil {
		t.Fatalf("acquire after invalidate: %v", err)
	}
	hits, misses := c.HitRate()
	if hits != 1 || misses != 2 {
		t.Fatalf("hits=%d misses=%d want 1/2 (the cleared entry must not count as a hit)", hits, misses)
	}
	if got, want := c.Stats().Entries, uint64(2); got != want {
		t.Fatalf("entries=%d want %d (the unusable entry stays owned until the device-abandon flow drops it)", got, want)
	}
	if n, _ := c.PurgeIdle(); n != 1 {
		t.Fatalf("purged=%d want 1: the loss-cleared entry must still be reclaimable by the OOM purge", n)
	}
	if got, want := c.Stats().Entries, uint64(1); got != want {
		t.Fatalf("entries=%d want %d after the purge reclaimed the unusable entry", got, want)
	}
}

// Two holds on the same entry resolve independently: resolving one (or
// resolving it twice) must not drop the other's in-flight count.
func TestGate_DoubleHoldResolvesOncePerHandle(t *testing.T) {
	reg := NewRegistry()
	f := &fakeNative{}
	ref := reg.DeviceOwner().Register(f)
	gate := NewInflightGate(reg)
	h1 := gate.Hold(ref)
	h2 := gate.Hold(ref)
	reg.Retire(ref)
	reg.Release(ref)
	h1.Resolve()
	h1.Resolve() // idempotent
	if f.released != 0 {
		t.Fatal("the second hold still references the entry")
	}
	if gate.Pending() != 1 {
		t.Fatalf("pending=%d want 1", gate.Pending())
	}
	h2.Resolve()
	if f.released != 1 {
		t.Fatalf("both holds resolved must release the native, released=%d", f.released)
	}
}

// Nil / foreign refs are skipped instead of corrupting another registry's
// counters; an empty hold is a nil handle whose Resolve is safe.
func TestGate_SkipsNilAndForeignRefs(t *testing.T) {
	regA, regB := NewRegistry(), NewRegistry()
	refB := regB.DeviceOwner().Register(&fakeNative{})
	gate := NewInflightGate(regA)
	if h := gate.Hold(Ref{}, refB); h != nil {
		t.Fatal("foreign and nil refs must not create a hold")
	}
	var nilHandle *GateHandle
	nilHandle.Resolve() // must not panic
	nilHandle.Invalidate()
	if gate.Pending() != 0 {
		t.Fatalf("pending=%d want 0", gate.Pending())
	}
	_ = regA
}

// ResolveAll / InvalidateAll sweep every outstanding handle (teardown and
// device-loss paths after a WaitIdle).
func TestGate_SettleAll(t *testing.T) {
	reg := NewRegistry()
	gate := NewInflightGate(reg)
	refs := make([]Ref, 0, 3)
	handles := make([]*GateHandle, 0, 3)
	for i := 0; i < 3; i++ {
		r := reg.DeviceOwner().Register(&fakeNative{})
		refs = append(refs, r)
		handles = append(handles, gate.Hold(r))
		// The submitter hands its own usage ref back while the fence is
		// still outstanding — only the gate keeps the entry un-lendable.
		reg.Release(r)
	}
	if gate.Pending() != 3 {
		t.Fatalf("pending=%d want 3", gate.Pending())
	}
	gate.ResolveAll()
	if gate.Pending() != 0 {
		t.Fatalf("pending=%d want 0 after ResolveAll", gate.Pending())
	}
	for _, r := range refs {
		if !reg.idleEntry(r.id) {
			t.Fatal("every entry must be idle after ResolveAll")
		}
	}
	for _, h := range handles {
		h.Resolve() // late resolves are no-ops
	}

	refs2 := reg.DeviceOwner().Register(&fakeNative{})
	h := gate.Hold(refs2)
	gate.InvalidateAll()
	if gate.Pending() != 0 {
		t.Fatalf("pending=%d want 0 after InvalidateAll", gate.Pending())
	}
	h.Invalidate() // idempotent
}

// Several window threads gating and completing the same pooled entry at once
// (run with -race in the gate).
func TestGate_ConcurrentHoldResolve(t *testing.T) {
	reg, tracker := NewRegistry(), &fakeNativeTracker{}
	c := NewCache(reg.DeviceOwner(), tracker.create, CacheOptions{Name: "gated", Budget: 0, MaxPerKey: 2})
	gate := NewInflightGate(reg)
	k := CacheKey{W: 4, H: 4, Role: RoleLayerRT}
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				ref, err := c.Acquire(k)
				if err != nil {
					t.Errorf("acquire: %v", err)
					return
				}
				h := gate.Hold(ref)
				c.Release(ref)
				if h != nil {
					h.Resolve()
				}
			}
		}()
	}
	wg.Wait()
	if gate.Pending() != 0 {
		t.Fatalf("pending=%d want 0 after all workers resolved", gate.Pending())
	}
	if n, _ := c.PurgeIdle(); n == 0 {
		t.Fatal("entries must be pooled and purgeable once every hold resolved")
	}
}
