package res

import (
	"sync"
	"testing"
	"time"
)

// One device registry serves N window sessions. Owner scoping is what makes a
// window close drop exactly its entries: roles, release and invalidation must
// never leak across owners.
func TestRegistry_OwnerIsolation(t *testing.T) {
	reg := NewRegistry()
	a := reg.NewOwner()
	b := reg.NewOwner()
	if a.ID() == b.ID() {
		t.Fatal("owners must get distinct ids")
	}

	fa := &fakeNative{tag: 1}
	fb := &fakeNative{tag: 2}
	refA := a.Register(fa)
	refB := b.Register(fb)

	// Same SourceKey, different owners: each resolves its own instance.
	key := SourceKey{Kind: KindTextureView, Role: RoleSessionResolve}
	a.Bind(key, refA)
	b.Bind(key, refB)
	gotA, ok := a.Resolve(key)
	if !ok || reg.NativeOf(gotA) != fa {
		t.Fatal("owner A must resolve its own instance")
	}
	gotB, ok := b.Resolve(key)
	if !ok || reg.NativeOf(gotB) != fb {
		t.Fatal("owner B must resolve its own instance")
	}

	a.ReleaseOwned()
	if fa.released != 1 {
		t.Fatalf("owner A native must release exactly once, got %d", fa.released)
	}
	if fb.released != 0 {
		t.Fatalf("owner B native must survive owner A teardown, got %d", fb.released)
	}
	if n := reg.Count(); n != 1 {
		t.Fatalf("registry must keep owner B's entry, count=%d", n)
	}
	if _, ok := b.Resolve(key); !ok {
		t.Fatal("owner B role must survive owner A teardown")
	}
	if _, ok := a.Resolve(key); ok {
		t.Fatal("owner A role must be gone after ReleaseOwned")
	}
	b.ReleaseOwned()
	if fb.released != 1 {
		t.Fatalf("owner B native must release on its own teardown, got %d", fb.released)
	}
	if reg.Count() != 0 {
		t.Fatalf("registry not empty after both owners teardown: %d", reg.Count())
	}
}

// Device loss drops bookkeeping without native Release (the abandon flow owns
// native teardown); scope it to the owner like ReleaseOwned.
func TestRegistry_InvalidateOwnedSkipsRelease(t *testing.T) {
	reg := NewRegistry()
	a := reg.NewOwner()
	b := reg.NewOwner()
	fa := &fakeNative{tag: 1}
	fb := &fakeNative{tag: 2}
	a.Register(fa)
	fbRef := b.Register(fb)

	a.InvalidateOwned()
	if fa.released != 0 {
		t.Fatalf("InvalidateOwned must not release natives: %d", fa.released)
	}
	if !reg.hasEntry(fbRef.id) {
		t.Fatal("InvalidateOwned must not touch another owner's entries")
	}
	if a.Count() != 0 {
		t.Fatalf("owner A entries not dropped: %d", a.Count())
	}
}

// Idle callbacks run outside the registry lock: caches re-pool from the
// callback, which re-enters the registry. An in-lock callback would deadlock.
func TestRegistry_IdleCallbackMayReenter(t *testing.T) {
	reg := NewRegistry()
	owner := reg.NewOwner()
	ref := owner.Register(&fakeNative{tag: 1})

	done := make(chan struct{})
	reg.SetIdleCallback(reg.ID(ref), func() {
		_ = reg.Count() // re-enter the registry
		close(done)
	})
	reg.Release(ref)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("idle callback deadlocked re-entering the registry")
	}
}

// Concurrent windows share one registry: register/release from parallel
// goroutines must not corrupt counters (run with -race in the gate).
func TestRegistry_ConcurrentOwners(t *testing.T) {
	reg := NewRegistry()
	const owners, perOwner = 4, 50
	var wg sync.WaitGroup
	for i := 0; i < owners; i++ {
		o := reg.NewOwner()
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perOwner; j++ {
				f := &fakeNative{}
				ref := o.Register(f)
				o.Bind(SourceKey{Kind: KindTextureView, Role: RoleLayerRT, Index: uint32(j)}, ref)
				reg.Release(ref)
				o.ReleaseOwned()
				if f.released > 1 {
					t.Errorf("native released %d times", f.released)
					return
				}
			}
		}()
	}
	wg.Wait()
	if n := reg.Count(); n != 0 {
		t.Fatalf("registry not empty after concurrent owners: %d", n)
	}
}
