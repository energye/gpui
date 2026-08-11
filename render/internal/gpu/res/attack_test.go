package res

import "testing"

// Boundary/attack tests: reference-count contract, invalidation-then-reuse,
// repeated submission tracking. These exercise the states the frame path can
// hit under resize storms, device loss, and re-recorded frames.

func TestRegistry_ResolveTransientRefMustBeReleased(t *testing.T) {
	reg := NewRegistry()
	key := SourceKey{Kind: KindTextureView, Role: RoleSessionResolve}
	ref := reg.Register(&fakeNative{tag: 1})
	reg.Bind(key, ref)

	// Resolve hands out a transient ref; the caller must Release it.
	g1, ok := reg.Resolve(key)
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
	ref := reg.Register(&fakeNative{tag: 1})

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
	ref := reg.Register(&fakeNative{tag: 1})
	reg.Bind(key, ref)

	reg.InvalidateAll() // device loss
	if reg.Count() != 0 {
		t.Fatalf("expected empty registry, got %d", reg.Count())
	}
	if _, ok := reg.Resolve(key); ok {
		t.Fatal("resolve after invalidation must fail")
	}
	// Re-register on the recovered device.
	ref2 := reg.Register(&fakeNative{tag: 2})
	reg.Bind(key, ref2)
	if _, ok := reg.Resolve(key); !ok {
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
	ref := reg.Register(f)
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
	c := NewCache(reg, func(k CacheKey) (Native, error) { return &fakeNative{tag: 1}, nil }, 0)
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

func TestCache_ReleaseUnknownRefNoPanic(t *testing.T) {
	reg := NewRegistry()
	c := NewCache(reg, func(k CacheKey) (Native, error) { return &fakeNative{}, nil }, 0)
	// A ref not tracked by this cache: Release must not panic (falls through
	// to the registry).
	ref := reg.Register(&fakeNative{tag: 9})
	c.Release(ref)
	reg.Release(ref)
}