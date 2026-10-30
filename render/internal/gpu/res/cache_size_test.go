package res

import (
	"sync"
	"testing"
)

// sizeOfPixels reports W*H*4 bytes — the surface/stencil domains' real size.
func sizeOfPixels(k CacheKey) uint64 {
	return uint64(k.W) * uint64(k.H) * 4
}

func newSizedCache(budget uint64, maxPerKey int) (*Registry, *Cache, *fakeNativeTracker) {
	reg := NewRegistry()
	tracker := &fakeNativeTracker{}
	c := NewCache(reg.DeviceOwner(), tracker.create, CacheOptions{
		Name:      "test",
		Budget:    budget,
		SizeFn:    sizeOfPixels,
		MaxPerKey: maxPerKey,
	})
	return reg, c, tracker
}

// Real byte sizes drive the budget: two 10x10 entries (400B each) fit a 600B
// budget; releasing the oldest idle entry evicts exactly the overage.
func TestCache_SizeFnDrivesBudget(t *testing.T) {
	_, c, tracker := newSizedCache(600, 0)
	k1 := CacheKey{W: 10, H: 10, Role: RoleLayerRT}
	k2 := CacheKey{W: 10, H: 10, Role: RoleLayerRT, Usage: 1}
	k3 := CacheKey{W: 10, H: 10, Role: RoleLayerRT, Usage: 2}

	r1, _ := c.Acquire(k1)
	r2, _ := c.Acquire(k2)
	if st := c.Stats(); st.Bytes != 800 {
		t.Fatalf("bytes=%d want 800 (two 400B entries)", st.Bytes)
	}
	// Nothing idle yet: over budget but no eviction.
	if st := c.Stats(); st.Evictions != 0 {
		t.Fatalf("must not evict checked-out entries, evictions=%d", st.Evictions)
	}

	c.Release(r1) // idle 400B; used still 800 > 600 → evict r1
	if ev := c.Evictions(); ev != 1 {
		t.Fatalf("evictions=%d want 1", ev)
	}
	if tracker.created[0].released != 1 {
		t.Fatalf("evicted entry native must be released, got %d", tracker.created[0].released)
	}
	if st := c.Stats(); st.Bytes != 400 {
		t.Fatalf("bytes=%d want 400 after eviction", st.Bytes)
	}

	r3, _ := c.Acquire(k3) // 400 + 400 = 800 > 600, r2 still checked out
	c.Release(r2)          // evicts one idle entry, back under budget
	c.Release(r3)
	if st := c.Stats(); st.Bytes > 600 {
		t.Fatalf("bytes=%d must stay within budget 600 after releases", st.Bytes)
	}
}

// MaxPerKey caps the idle bucket: identical-size bursts cannot grow the pool
// without bound; overflow is retired on release.
func TestCache_MaxPerKeyCapsIdleBucket(t *testing.T) {
	reg, c, _ := newSizedCache(0, 1)
	k := CacheKey{W: 8, H: 8, Role: RoleSessionStencil}

	r1, _ := c.Acquire(k)
	r2, _ := c.Acquire(k) // second entry before any release → miss
	if hits, misses := c.HitRate(); hits != 0 || misses != 2 {
		t.Fatalf("hits=%d misses=%d want 0/2", hits, misses)
	}
	c.Release(r1)
	c.Release(r2) // bucket already holds r1 → r2 retired
	if ev := c.Evictions(); ev != 1 {
		t.Fatalf("evictions=%d want 1 (per-key cap)", ev)
	}
	st := c.Stats()
	if st.Idle != 1 || st.Entries != 1 {
		t.Fatalf("idle=%d entries=%d want 1/1 (only one pooled per key)", st.Idle, st.Entries)
	}
	if got := reg.Count(); got != 1 {
		t.Fatalf("registry count=%d want 1", got)
	}
}

// PurgeIdle is the OOM outlet: every idle entry retires, bytes go to zero,
// and the next Acquire misses instead of reusing a dropped id.
func TestCache_PurgeIdle(t *testing.T) {
	reg, c, tracker := newSizedCache(0, 0)
	k1 := CacheKey{W: 16, H: 16, Role: RoleSessionMSAA}
	k2 := CacheKey{W: 16, H: 16, Role: RoleSessionResolve}
	r1, _ := c.Acquire(k1)
	r2, _ := c.Acquire(k2)
	c.Release(r1)
	c.Release(r2)

	if st := c.Stats(); st.Idle != 2 || st.Bytes != 2048 {
		t.Fatalf("before purge: idle=%d bytes=%d want 2/2048", st.Idle, st.Bytes)
	}
	n, freed := c.PurgeIdle()
	if n != 2 || freed != 2048 {
		t.Fatalf("purge n=%d freed=%d want 2/2048", n, freed)
	}
	for i, f := range tracker.created {
		if f.released != 1 {
			t.Fatalf("purged native %d released=%d want 1", i, f.released)
		}
	}
	if st := c.Stats(); st.Entries != 0 || st.Idle != 0 || st.Bytes != 0 {
		t.Fatalf("after purge: entries=%d idle=%d bytes=%d want 0", st.Entries, st.Idle, st.Bytes)
	}
	if got := reg.Count(); got != 0 {
		t.Fatalf("registry count=%d want 0", got)
	}

	r3, _ := c.Acquire(k1) // must allocate fresh, not reuse a purged id
	if _, misses := c.HitRate(); misses != 3 {
		t.Fatalf("misses=%d want 3 after re-acquire", misses)
	}
	if n, _ := c.PurgeIdle(); n != 0 {
		t.Fatalf("purge with checked-out entry must purge 0, got %d", n)
	}
	c.Release(r3)
	if n, _ := c.PurgeIdle(); n != 1 {
		t.Fatalf("second purge must retire the released entry, got %d", n)
	}
}

// ReleaseRetire drops the ref and destroys the entry instead of pooling it.
// Owners whose native may still be referenced by a directly submitted command
// buffer use this to keep the "destroy on release" contract.
func TestCache_ReleaseRetireDoesNotPool(t *testing.T) {
	reg, c, tracker := newSizedCache(0, 0)
	k := CacheKey{W: 32, H: 32, Role: RoleSessionStencil}

	r1, _ := c.Acquire(k)
	c.ReleaseRetire(r1)
	if tracker.created[0].released != 1 {
		t.Fatalf("retired native released=%d want 1", tracker.created[0].released)
	}
	if st := c.Stats(); st.Entries != 0 || st.Idle != 0 || st.Bytes != 0 {
		t.Fatalf("after retire: entries=%d idle=%d bytes=%d want 0", st.Entries, st.Idle, st.Bytes)
	}
	if got := reg.Count(); got != 0 {
		t.Fatalf("registry count=%d want 0", got)
	}
	r2, _ := c.Acquire(k) // must allocate fresh
	if hits, misses := c.HitRate(); hits != 0 || misses != 2 {
		t.Fatalf("hits=%d misses=%d want 0/2", hits, misses)
	}
	c.Release(r2)
}

// ReleaseRetire still waits for the in-flight gate: a submitted command
// buffer keeps the native alive until SubmitDone, then destroys it (no pool).
func TestCache_ReleaseRetireWaitsInflight(t *testing.T) {
	reg, c, tracker := newSizedCache(0, 0)
	sub := NewSubmission(reg)
	k := CacheKey{W: 32, H: 32, Role: RoleSessionMSAA}

	r1, _ := c.Acquire(k)
	sub.Track(r1)
	c.ReleaseRetire(r1)
	if tracker.created[0].released != 0 {
		t.Fatalf("in-flight native must not be released yet")
	}
	sub.SubmitDone()
	if tracker.created[0].released != 1 {
		t.Fatalf("native released=%d want 1 after SubmitDone", tracker.created[0].released)
	}
	if got := reg.Count(); got != 0 {
		t.Fatalf("registry count=%d want 0", got)
	}
}

// ReleaseAll is the device-teardown outlet: pooled and checked-out entries are
// force-released and the cache/registry bookkeeping is empty.
func TestCache_ReleaseAll(t *testing.T) {
	reg, c, tracker := newSizedCache(0, 0)
	k1 := CacheKey{W: 16, H: 16, Role: RoleSessionResolve}
	k2 := CacheKey{W: 16, H: 16, Role: RoleSessionMSAA}

	r1, _ := c.Acquire(k1)
	r2, _ := c.Acquire(k2)
	c.Release(r1) // k1 pooled
	c.ReleaseAll()
	for i, f := range tracker.created {
		if f.released != 1 {
			t.Fatalf("native %d released=%d want 1", i, f.released)
		}
	}
	if st := c.Stats(); st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("after ReleaseAll: entries=%d bytes=%d want 0", st.Entries, st.Bytes)
	}
	if got := reg.Count(); got != 0 {
		t.Fatalf("registry count=%d want 0", got)
	}
	c.Release(r2) // releasing a force-dropped ref must not panic/release twice
	if tracker.created[1].released != 1 {
		t.Fatalf("double release after ReleaseAll, got %d", tracker.created[1].released)
	}
}

// Per-domain stats carry the domain name and the pooled/checked-out split.
func TestCache_StatsPerDomain(t *testing.T) {
	_, c, _ := newSizedCache(4096, 0)
	k := CacheKey{W: 4, H: 4, Role: RoleSessionResolve}

	r1, _ := c.Acquire(k)
	c.Release(r1)
	r2, _ := c.Acquire(k) // hit
	c.Release(r2)

	st := c.Stats()
	if st.Name != "test" {
		t.Fatalf("name=%q want test", st.Name)
	}
	if st.Hits != 1 || st.Misses != 1 {
		t.Fatalf("hits=%d misses=%d want 1/1", st.Hits, st.Misses)
	}
	if st.Entries != 1 || st.Idle != 1 {
		t.Fatalf("entries=%d idle=%d want 1/1", st.Entries, st.Idle)
	}
	if st.Bytes != 64 || st.PeakBytes != 64 || st.Budget != 4096 {
		t.Fatalf("bytes=%d peak=%d budget=%d want 64/64/4096", st.Bytes, st.PeakBytes, st.Budget)
	}

	// Checked-out entries are Entries but not Idle.
	r3, _ := c.Acquire(k)
	st = c.Stats()
	if st.Entries != 1 || st.Idle != 0 {
		t.Fatalf("checked-out: entries=%d idle=%d want 1/0", st.Entries, st.Idle)
	}
	c.Release(r3)
}

// Concurrent windows drawing with the same size: acquire/release must stay
// race-free and never double-release (run with -race in the gate).
func TestCache_ConcurrentAcquireRelease(t *testing.T) {
	_, c, _ := newSizedCache(0, 2)
	keys := []CacheKey{
		{W: 8, H: 8, Role: RoleSessionMSAA},
		{W: 8, H: 8, Role: RoleSessionResolve},
	}
	const workers, loops = 4, 100
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				k := keys[(w+i)%len(keys)]
				ref, err := c.Acquire(k)
				if err != nil {
					t.Errorf("acquire: %v", err)
					return
				}
				c.Release(ref)
			}
		}(w)
	}
	wg.Wait()
	st := c.Stats()
	if st.Entries > uint64(len(keys)*(2)) {
		t.Fatalf("entries=%d exceeded per-key cap accounting", st.Entries)
	}
	if n, _ := c.PurgeIdle(); n != int(st.Entries) {
		t.Fatalf("purge=%d want all %d idle entries", n, st.Entries)
	}
}
