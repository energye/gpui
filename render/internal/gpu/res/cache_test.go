package res

import "testing"

func newTestCache(budget uint64) (*Registry, *Cache, *fakeNativeTracker) {
	reg := NewRegistry()
	tracker := &fakeNativeTracker{}
	c := NewCache(reg, tracker.create, budget)
	return reg, c, tracker
}

type fakeNativeTracker struct {
	created []*fakeNative
}

func (t *fakeNativeTracker) create(key CacheKey) (Native, error) {
	f := &fakeNative{tag: len(t.created)}
	t.created = append(t.created, f)
	return f, nil
}

func TestCache_AcquireReuse(t *testing.T) {
	reg, c, _ := newTestCache(0)
	k := CacheKey{W: 10, H: 10, Role: RoleLayerRT}

	r1, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	n1 := reg.NativeOf(r1).(*fakeNative)
	c.Release(r1)

	r2, err := c.Acquire(k)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	n2 := reg.NativeOf(r2).(*fakeNative)
	if n1 != n2 {
		t.Fatalf("cache must reuse the pooled entry (got different native)")
	}
	hits, misses := c.HitRate()
	if hits != 1 || misses != 1 {
		t.Fatalf("expected hits=1 misses=1, got hits=%d misses=%d", hits, misses)
	}
	c.Release(r2)
}

func TestCache_InflightNotLentAndRepooledAfterSubmit(t *testing.T) {
	reg, c, _ := newTestCache(0)
	sub := NewSubmission(reg)
	k := CacheKey{W: 20, H: 40, Role: RoleFrameScratch}

	r1, _ := c.Acquire(k)
	n1 := reg.NativeOf(r1).(*fakeNative)
	sub.Track(r1)   // submitted CB references the entry
	c.Release(r1)   // refs==0 but inflight==1 → must go to pending, not pool

	r2, _ := c.Acquire(k) // in-flight entries are never lent out
	n2 := reg.NativeOf(r2).(*fakeNative)
	if n1 == n2 {
		t.Fatalf("in-flight entry was lent out; must allocate fresh")
	}
	if hits, misses := c.HitRate(); hits != 0 || misses != 2 {
		t.Fatalf("expected misses=2 hits=0, got hits=%d misses=%d", hits, misses)
	}
	if n1.released != 0 {
		t.Fatalf("in-flight entry must not be released")
	}

	sub.SubmitDone()       // fence arrives
	c.OnSubmissionFinished() // pending entry becomes recombinable

	r3, _ := c.Acquire(k)
	n3 := reg.NativeOf(r3).(*fakeNative)
	if n1 != n3 {
		t.Fatalf("entry must be repooled and reused after submission finished")
	}
	c.Release(r2)
	c.Release(r3)
}

func TestCache_BudgetRetireIdleAndNotInFlight(t *testing.T) {
	reg, c, tracker := newTestCache(2 * 4096) // room for 2 entries
	k1 := CacheKey{W: 1, H: 1, Role: RoleLayerRT}
	k2 := CacheKey{W: 2, H: 2, Role: RoleLayerRT}
	k3 := CacheKey{W: 3, H: 3, Role: RoleLayerRT}

	r1, _ := c.Acquire(k1)
	r2, _ := c.Acquire(k2)
	r3, _ := c.Acquire(k3) // used=3 units > budget(2); nothing idle yet → no evict
	if ev := c.Evictions(); ev != 0 {
		t.Fatalf("eviction must wait for refs==0, got %d", ev)
	}

	c.Release(r1) // r1 idle → trim retires the LRU-most idle (r1)
	if ev := c.Evictions(); ev != 1 {
		t.Fatalf("expected 1 eviction after releasing idle over budget, got %d", ev)
	}
	if tracker.created[0].released != 1 {
		t.Fatalf("over-budget idle entry must be released (when not in-flight)")
	}

	r4, _ := c.Acquire(k1) // retired entry not in pool → fresh allocation
	n4 := reg.NativeOf(r4).(*fakeNative)
	if n4 == tracker.created[0] {
		t.Fatalf("retired entry must not be reused")
	}
	c.Release(r2)
	c.Release(r3)
	c.Release(r4)
}

func TestCache_InvalidateAll(t *testing.T) {
	reg, c, tracker := newTestCache(0)
	k := CacheKey{W: 8, H: 8, Role: RoleExternal}
	r1, _ := c.Acquire(k)
	c.Release(r1)

	c.InvalidateAll()
	reg.InvalidateAll() // abandon flow calls both, in this order
	for i, f := range tracker.created {
		if f.released != 0 {
			t.Fatalf("InvalidateAll must not release native (entry %d): %d", i, f.released)
		}
	}
	// Registry no longer knows any of this — consistent with abandon.
	if reg.Count() != 0 {
		t.Fatalf("expected empty registry after InvalidateAll, got %d", reg.Count())
	}
	_ = r1
}