package res

import "testing"

// fakeNative is a deterministic native-resource stand-in: it counts Release
// calls so tests assert timing and count rather than "didn't crash".
type fakeNative struct {
	tag      int
	released int
}

func (f *fakeNative) Release() { f.released++ }

// TestReleaseSequenceTiming pins the exact release gate: native Release may
// only happen when refs==0 && retire && inflight==0, in that order.
func TestReleaseSequenceTiming(t *testing.T) {
	reg := NewRegistry()
	f := &fakeNative{tag: 1}
	ref := reg.Register(f)

	// 1) retire while referenced → no release
	reg.Retire(ref)
	if f.released != 0 {
		t.Fatalf("released while referenced (retire only): %d", f.released)
	}
	// 2) drop the last ref, no submission in flight → release exactly once
	reg.Release(ref)
	if f.released != 1 {
		t.Fatalf("expected 1 release after refs==0&&retire&&!inflight, got %d", f.released)
	}
}

func TestRegistry_InflightBlocksReleaseUntilSubmitDone(t *testing.T) {
	reg := NewRegistry()
	sub := NewSubmission(reg)
	f := &fakeNative{tag: 1}
	ref := reg.Register(f)

	reg.Retire(ref)            // mark for destruction
	sub.Track(ref)             // referenced by a submitted CB (refs still >=1)
	reg.Release(ref)           // refs==0, but inflight==1 → must NOT release
	if f.released != 0 {
		t.Fatalf("released while submission in flight: %d", f.released)
	}
	sub.SubmitDone()           // fence arrived → inflight==0 → release
	if f.released != 1 {
		t.Fatalf("expected release after SubmitDone, got %d", f.released)
	}
}

func TestRegistry_ResolveRebindReturnsNewInstance(t *testing.T) {
	reg := NewRegistry()
	key := SourceKey{Kind: KindTextureView, Role: RoleSessionResolve}

	a := &fakeNative{tag: 1}
	refA := reg.Register(a)
	reg.Bind(key, refA)

	got, ok := reg.Resolve(key)
	if !ok || reg.NativeOf(got) != a {
		t.Fatalf("Resolve before rebuild must return instance A")
	}
	reg.Release(got)

	// Rebuild: new instance B replaces A; A is retired.
	b := &fakeNative{tag: 2}
	refB := reg.Register(b)
	reg.Bind(key, refB)

	got2, ok := reg.Resolve(key)
	if !ok {
		t.Fatalf("Resolve after rebuild must succeed")
	}
	if reg.NativeOf(got2) != b {
		t.Fatalf("Resolve after rebuild must return the NEW instance, got old")
	}
	reg.Release(got2)

	// A must be released only once its own owner ref drops.
	if a.released != 0 {
		t.Fatalf("A released prematurely: %d", a.released)
	}
	reg.Release(refA)
	if a.released != 1 {
		t.Fatalf("A must release exactly once after owner ref drop, got %d", a.released)
	}
	// B is the current active bound instance: dropping its owner ref must NOT
	// release it (it stays alive as the resolvable instance).
	reg.Release(refB)
	if b.released != 0 {
		t.Fatalf("active bound instance must not be released by owner Release: %d", b.released)
	}
	// Replacing B with C retires B; with no refs left, B is released.
	c3 := &fakeNative{tag: 3}
	refC := reg.Register(c3)
	reg.Bind(key, refC)
	if b.released != 1 {
		t.Fatalf("B must release after being replaced and unreferenced, got %d", b.released)
	}
	reg.Release(refC)
}

func TestRegistry_MonotonicIDsNeverReused(t *testing.T) {
	reg := NewRegistry()
	ids := make(map[uint32]bool)
	for i := 0; i < 100; i++ {
		f := &fakeNative{tag: i}
		ref := reg.Register(f)
		if ids[ref.id] {
			t.Fatalf("registry id %d reused", ref.id)
		}
		ids[ref.id] = true
		reg.Release(ref)
	}
}

func TestRegistry_InvalidateAllSkipsNativeRelease(t *testing.T) {
	reg := NewRegistry()
	var natives []*fakeNative
	for i := 0; i < 3; i++ {
		f := &fakeNative{tag: i}
		reg.Register(f)
		natives = append(natives, f)
	}
	reg.Retire(Ref{reg: reg, id: 0}) // no-op, must be safe

	reg.InvalidateAll()
	for i, f := range natives {
		if f.released != 0 {
			t.Fatalf("InvalidateAll must not call native Release (entry %d): %d", i, f.released)
		}
	}
	if reg.Count() != 0 {
		t.Fatalf("expected empty registry after InvalidateAll, got %d", reg.Count())
	}
}

func TestRegistry_BindRetiresPredecessor(t *testing.T) {
	reg := NewRegistry()
	key := SourceKey{Kind: KindTextureView, Role: RoleFrameScratch}

	a := &fakeNative{tag: 1}
	refA := reg.Register(a)
	reg.Bind(key, refA)

	b := &fakeNative{tag: 2}
	refB := reg.Register(b)
	reg.Bind(key, refB)

	// A is retired by the rebind; B is now active.
	if _, ok := reg.Resolve(key); !ok {
		t.Fatalf("new instance must be resolvable")
	}
	reg.Release(refB) // keep B bound release ordering: refB dropped then refA
	reg.Release(refA)
	// A releases exactly once (its predecessor role was retired on rebind).
	if a.released != 1 {
		t.Fatalf("predecessor A must release exactly once, got %d", a.released)
	}
	_ = b
}