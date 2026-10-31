package res

import "testing"

// Boundary/attack tests: reference-count contract, invalidation-then-reuse,
// repeated submission tracking. These exercise the states the frame path can
// hit under resize storms, device loss, and re-recorded frames.

func TestRegistry_ResolveTransientRefMustBeReleased(t *testing.T) {
	reg := NewRegistry()
	key := SourceKey{Kind: KindTextureView, Role: RoleSessionResolve}
	ref := reg.DeviceOwner().Register(&fakeNative{tag: 1})
	reg.DeviceOwner().Bind(key, ref)

	// Resolve hands out a transient ref; the caller must Release it.
	g1, ok := reg.DeviceOwner().Resolve(key)
	if !ok {
		t.Fatal("resolve must succeed")
	}
	if reg.slots[ref.id].refs != 2 { // 1 owner + 1 transient
		t.Fatalf("expected refs=2 after resolve, got %d", reg.slots[ref.id].refs)
	}
	// Leak the transient ref → refs stays 2 after owner release.
	reg.Release(ref)
	if reg.slots[ref.id].refs != 1 {
		t.Fatalf("leaked transient ref must keep refs=1, got %d", reg.slots[ref.id].refs)
	}
	// Correct usage: release the transient.
	reg.Release(g1)
	if reg.slots[ref.id].refs != 0 {
		t.Fatalf("expected refs=0 after releasing both, got %d", reg.slots[ref.id].refs)
	}
}

func TestRegistry_ReleaseAndReacquireIndependentRefs(t *testing.T) {
	reg := NewRegistry()
	ref := reg.DeviceOwner().Register(&fakeNative{tag: 1})

	extra := reg.Acquire(ref.id)
	if reg.slots[ref.id].refs != 2 {
		t.Fatalf("expected refs=2 after acquire, got %d", reg.slots[ref.id].refs)
	}
	reg.Release(ref)
	reg.Release(extra)
	if reg.slots[ref.id].refs != 0 {
		t.Fatalf("expected refs=0, got %d", reg.slots[ref.id].refs)
	}
	// Over-release is a no-op.
	reg.Release(ref)
	reg.Release(extra)
	if reg.slots[ref.id].refs != 0 {
		t.Fatalf("over-release must not go negative, got %d", reg.slots[ref.id].refs)
	}
}

func TestRegistry_InvalidateThenReuse(t *testing.T) {
	reg := NewRegistry()
	key := SourceKey{Kind: KindTextureView, Role: RoleFrameScratch}
	ref := reg.DeviceOwner().Register(&fakeNative{tag: 1})
	reg.DeviceOwner().Bind(key, ref)

	reg.InvalidateAll() // device loss
	if reg.Count() != 0 {
		t.Fatalf("expected empty registry, got %d", reg.Count())
	}
	if _, ok := reg.DeviceOwner().Resolve(key); ok {
		t.Fatal("resolve after invalidation must fail")
	}
	// Re-register on the recovered device.
	ref2 := reg.DeviceOwner().Register(&fakeNative{tag: 2})
	reg.DeviceOwner().Bind(key, ref2)
	if _, ok := reg.DeviceOwner().Resolve(key); !ok {
		t.Fatal("resolve after re-register must succeed")
	}
	reg.Release(ref2)
}

func TestSubmission_DoubleTrackSingleSubmitDone(t *testing.T) {
	// Semantics: one SubmitDone resolves the ENTIRE submission, so every
	// Track (however many references the submit holds) is released together.
	// The in-flight count only exists to prevent lending/releasing a resource
	// referenced by an unfinished submit.
	reg := NewRegistry()
	sub := NewSubmission(reg)
	f := &fakeNative{tag: 1}
	ref := reg.DeviceOwner().Register(f)
	reg.Retire(ref)

	sub.Track(ref)
	sub.Track(ref) // two CBs in one submit reference the same resource
	reg.Release(ref)
	if f.released != 0 {
		t.Fatalf("must not release while in-flight, got %d", f.released)
	}
	if n := len(sub.InFlight()); n != 2 {
		t.Fatalf("expected 2 tracked jobs, got %d", n)
	}
	sub.SubmitDone() // the whole submit completed → released
	if f.released != 1 {
		t.Fatalf("SubmitDone must release the retired resource, got %d", f.released)
	}
	// Extra SubmitDone is a no-op.
	sub.SubmitDone()
}

func TestCache_InvalidateThenAcquire(t *testing.T) {
	reg := NewRegistry()
	c := NewCache(reg.DeviceOwner(), func(k CacheKey) (Native, error) { return &fakeNative{tag: 1}, nil }, CacheOptions{})
	k := CacheKey{W: 4, H: 4, Role: RoleLayerRT}
	r1, err := c.Acquire(k)
	if err != nil {
		t.Fatal(err)
	}
	c.Release(r1)

	c.InvalidateAll()
	reg.InvalidateAll() // abandon flow: both
	if reg.Count() != 0 {
		t.Fatalf("expected empty after invalidation, got %d", reg.Count())
	}
	// Re-acquire on the recovered device must not panic or reuse stale ids.
	r2, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("re-acquire after invalidation: %v", err)
	}
	if r2.IsNil() {
		t.Fatal("re-acquired ref must be valid")
	}
	c.Release(r2)
}

func TestRegistry_ResolveStatsCountsHitsAndMisses(t *testing.T) {
	reg := NewRegistry()
	h, m := reg.ResolveStats()
	if h != 0 || m != 0 {
		t.Fatalf("fresh registry must report 0/0, got %d/%d", h, m)
	}

	key := SourceKey{Kind: KindTextureView, Role: RoleSessionResolve}
	ref := reg.DeviceOwner().Register(&fakeNative{tag: 1})
	reg.DeviceOwner().Bind(key, ref)

	r1, ok := reg.DeviceOwner().Resolve(key)
	if !ok {
		t.Fatal("resolve must succeed after bind")
	}
	reg.Release(r1)
	h, m = reg.ResolveStats()
	if h != 1 || m != 0 {
		t.Fatalf("expected 1 hit 0 miss, got %d/%d", h, m)
	}

	// Miss: unresolvable key → miss counter increments.
	if _, ok := reg.DeviceOwner().Resolve(SourceKey{Kind: KindTextureView, Role: RoleAtlasPage, Index: 9}); ok {
		t.Fatal("unbound role must not resolve")
	}
	h, m = reg.ResolveStats()
	if h != 1 || m != 1 {
		t.Fatalf("expected 1 hit 1 miss, got %d/%d", h, m)
	}

	// Miss: retired bound instance (rebuild replaced it mid-flush).
	ref2 := reg.DeviceOwner().Register(&fakeNative{tag: 2})
	reg.DeviceOwner().Bind(key, ref2) // ref retired
	if _, ok := reg.DeviceOwner().Resolve(key); !ok {
		t.Fatal("resolving the new bound instance must succeed")
	}
	h, m = reg.ResolveStats()
	if h != 2 {
		t.Fatalf("expected 2 hits, got %d", h)
	}
	reg.Release(ref)
	reg.Release(ref2)
}

func TestCache_ReleaseUnknownRefNoPanic(t *testing.T) {
	reg := NewRegistry()
	c := NewCache(reg.DeviceOwner(), func(k CacheKey) (Native, error) { return &fakeNative{}, nil }, CacheOptions{})
	// A ref not tracked by this cache: Release must not panic (falls through
	// to the registry).
	ref := reg.DeviceOwner().Register(&fakeNative{tag: 9})
	c.Release(ref)
	reg.Release(ref)
}

// TestRegistry_ReleaseIdleCallbackIsEdgeTriggered pins J1 (P2.4e): the idle
// callback fires exactly once on the transition into idle — a repeated Release
// of an already-released Ref must not re-fire it.
func TestRegistry_ReleaseIdleCallbackIsEdgeTriggered(t *testing.T) {
	reg := NewRegistry()
	ref := reg.DeviceOwner().Register(&fakeNative{tag: 1})
	fires := 0
	reg.SetIdleCallback(ref.id, func() { fires++ })
	reg.Release(ref)
	reg.Release(ref) // contract violation: over-release
	if fires != 1 {
		t.Fatalf("idle callback must fire exactly once, got %d", fires)
	}
}

// TestCache_OverReleaseWithIdleCallbackKeepsPoolConsistent pins J1 (P2.4e) at
// the cache level: a double Release of a cache-owned Ref must not duplicate
// the pooled entry nor force a re-create on the next acquire.
func TestCache_OverReleaseWithIdleCallbackKeepsPoolConsistent(t *testing.T) {
	reg := NewRegistry()
	created := 0
	c := NewCache(reg.DeviceOwner(), func(k CacheKey) (Native, error) {
		created++
		return &fakeNative{tag: created}, nil
	}, CacheOptions{})
	k := CacheKey{W: 4, H: 4, Role: RoleLayerRT}
	r1, err := c.Acquire(k)
	if err != nil {
		t.Fatal(err)
	}
	c.Release(r1)
	c.Release(r1) // contract violation: over-release

	c.mu.Lock()
	n := len(c.pool[k])
	c.mu.Unlock()
	if n != 1 {
		t.Fatalf("pool bucket must hold exactly 1 entry after over-release, got %d", n)
	}

	// The pooled entry must remain lendable and must not be poisoned: no
	// re-create, and releasing the re-lent ref must not destroy anything.
	r2, err := c.Acquire(k)
	if err != nil {
		t.Fatal(err)
	}
	if r2.IsNil() {
		t.Fatal("pooled entry must remain lendable")
	}
	if created != 1 {
		t.Fatalf("over-release must not force a re-create, created=%d", created)
	}
	c.Release(r2)
}
