//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package res

import "sync"

// CacheKey identifies a reusable GPU resource class (GrResourceCache key).
type CacheKey struct {
	W, H    uint32
	Format  uint32 // opaque format tag; callers map to their own enum
	Samples uint32
	Usage   uint64 // opaque usage tag
	Role    Role
}

// CreateFn fabricates a native resource for a key (device.createTexture etc).
type CreateFn func(key CacheKey) (Native, error)

// SizeFn reports the byte size of a key's resource. Caches use it for real
// budget accounting; nil falls back to defaultEntryBytes per entry.
type SizeFn func(key CacheKey) uint64

const defaultEntryBytes uint64 = 4096

// CacheOptions configures one cache domain.
type CacheOptions struct {
	// Name labels the domain in Stats ("" = unnamed).
	Name string
	// Budget caps estimated bytes of live+pooled entries; 0 disables.
	Budget uint64
	// SizeFn gives real per-resource bytes (nil = defaultEntryBytes).
	SizeFn SizeFn
	// MaxPerKey caps idle entries kept per key; 0 = unlimited. Overflow is
	// retired immediately, so identical-size bursts cannot grow a bucket
	// without bound.
	MaxPerKey int
}

// CacheStats is a per-domain snapshot (diagnostics / real-window JSON pool
// section). Entries includes checked-out and pooled; Idle is pooled-and-idle.
type CacheStats struct {
	Name      string
	Hits      uint64
	Misses    uint64
	Evictions uint64
	Purged    uint64
	Entries   uint64
	Idle      uint64
	Bytes     uint64
	PeakBytes uint64
	Budget    uint64
}

// Cache is a GrResourceCache-style pool: keyed reuse, byte budget with LRU
// eviction, per-key idle cap, explicit idle purge, and "in-flight items are
// never lent out" safety. Entries returned by Acquire are strong Refs;
// Release returns them to the pool. Over-budget idle entries are retired
// (destroyed once no longer in-flight).
//
// Thread contract: one cache may be shared by several window threads on the
// same device (shared filter/dualtex domains), so every method locks. Lock
// order is cache → registry; the registry runs idle callbacks after releasing
// its own lock, so the callback may re-enter the cache safely.
type Cache struct {
	mu sync.Mutex

	reg   *Registry
	owner OwnerID
	opt   CacheOptions

	create CreateFn
	pool   map[CacheKey][]uint32 // key → idle registry ids (refs==0 && inflight==0)
	keyOf  map[uint32]CacheKey   // registry id → key
	sizes  map[uint32]uint64     // registry id → byte size
	used   uint64                // bytes of live+pooled entries
	peak   uint64                // high-water mark of used (diagnostics)
	lru    []uint32              // idle ids, least-recently-retired first (trim order)

	hits      uint64
	misses    uint64
	evictions uint64
	purged    uint64
}

// NewCache creates a cache whose entries are attributed to owner, so the
// owning session's teardown releases exactly them from the device-level
// registry.
func NewCache(owner Owner, create CreateFn, opt CacheOptions) *Cache {
	return &Cache{
		reg:    owner.reg,
		owner:  owner.id,
		opt:    opt,
		create: create,
		pool:   make(map[CacheKey][]uint32),
		keyOf:  make(map[uint32]CacheKey),
		sizes:  make(map[uint32]uint64),
	}
}

// Acquire returns a strong Ref for the key, reusing an idle pooled entry when
// possible. In-flight or currently-referenced entries are never lent out.
//
// The create callback runs WITHOUT the cache lock: real allocators invoke the
// OOM purge chain (PurgeAcceleratorSurfaceResources → Cache.PurgeIdle), which
// re-enters this cache. Holding the lock across create would deadlock.
func (c *Cache) Acquire(key CacheKey) (Ref, error) {
	if c == nil || c.reg == nil {
		return Ref{}, nil
	}
	c.mu.Lock()

	// Prefer an idle pooled entry (refs==0 && inflight==0; prune stale ids).
	bucket := c.pool[key]
	for len(bucket) > 0 {
		id := bucket[len(bucket)-1]
		bucket = bucket[:len(bucket)-1]
		c.pool[key] = bucket
		if ref, ok := c.reg.acquireIfIdle(id); ok {
			c.hits++
			c.mu.Unlock()
			return ref, nil
		}
		// Unusable entry (already freed/retired, or raced into use): retire
		// whatever is left so a live slot cannot leak, drop bookkeeping.
		// Counting the retirement keeps the drop counters monotonic in every
		// path that can destroy a pooled native — callers key cache-generation
		// off Evictions+Purged.
		if c.reg.retireEntry(id) {
			c.evictions++
		}
		c.forgetLocked(id)
	}
	c.misses++
	c.mu.Unlock()

	// Create a fresh resource outside the lock (see OOM re-entry above).
	native, err := c.create(key)
	if err != nil {
		return Ref{}, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	ref := Owner{reg: c.reg, id: c.owner}.Register(native)
	if ref.IsNil() {
		return Ref{}, nil
	}
	size := c.sizeOf(key)
	c.keyOf[ref.id] = key
	c.sizes[ref.id] = size
	c.used += size
	if c.used > c.peak {
		c.peak = c.used
	}
	// Re-pool through the registry idle callback: the entry returns to its
	// bucket when the last ref is released AND no submission is in flight,
	// whichever comes last (no separate pending queue).
	id := ref.id
	c.reg.SetIdleCallback(id, func() { c.repool(id, key) })
	// Enforce budget: retire the oldest idle entries until under limit.
	c.trimLocked()
	return ref, nil
}

// Release returns a Ref to the pool. The registry refcount is decremented;
// the entry becomes recombinable when its last ref is gone and it is not
// in-flight — the registry idle callback repools it at that point, so an
// entry still referenced by a submitted command buffer waits for SubmitDone.
func (c *Cache) Release(ref Ref) {
	if c == nil || ref.IsNil() || ref.reg != c.reg {
		return
	}
	c.mu.Lock()
	_, known := c.keyOf[ref.id]
	c.mu.Unlock()
	if !known {
		// Not this cache's entry (callers may hand us foreign refs): fall
		// through to the registry without cache bookkeeping.
		c.reg.Release(ref)
		return
	}
	// Do not hold the cache lock across registry.Release: the idle callback
	// may fire synchronously from an in-flight completion and re-enters the
	// cache.
	c.reg.Release(ref)
	c.mu.Lock()
	c.trimLocked()
	c.mu.Unlock()
}

// ReleaseRetire drops ref and destroys the entry instead of returning it to
// the pool. Used by owners whose resources may still be referenced by a
// directly submitted (untracked) command buffer: hand-off to a pool could
// reissue the same native while the GPU is not done, so they keep the
// pre-pooling "destroy on release" contract.
func (c *Cache) ReleaseRetire(ref Ref) {
	if c == nil || ref.IsNil() || ref.reg != c.reg {
		return
	}
	c.mu.Lock()
	if _, known := c.keyOf[ref.id]; known {
		c.forgetLocked(ref.id)
	}
	c.mu.Unlock()
	// Mark retired before dropping the ref so the last Release destroys it
	// immediately instead of firing the idle/repool callback.
	c.reg.Retire(ref)
	c.reg.Release(ref)
}

// ReleaseAll force-releases every entry (native Release) and clears the cache.
// Device-teardown path (healthy device): the device-abandon flow calls this so
// pooled natives do not outlive the device. Device loss uses InvalidateAll
// instead (no Release).
func (c *Cache) ReleaseAll() {
	if c == nil || c.reg == nil {
		return
	}
	c.mu.Lock()
	ids := make([]uint32, 0, len(c.keyOf))
	for id := range c.keyOf {
		ids = append(ids, id)
	}
	c.pool = make(map[CacheKey][]uint32)
	c.keyOf = make(map[uint32]CacheKey)
	c.sizes = make(map[uint32]uint64)
	c.used = 0
	c.lru = c.lru[:0]
	c.mu.Unlock()
	c.reg.releaseIDs(ids)
}

// repool returns an idle entry to its key bucket and the LRU queue. Runs from
// the registry idle callback, i.e. without the cache lock held by the caller.
func (c *Cache) repool(id uint32, key CacheKey) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.keyOf[id]; !ok {
		return // purged / invalidated: never resurrect a dropped entry
	}
	if c.opt.MaxPerKey > 0 && len(c.pool[key]) >= c.opt.MaxPerKey {
		if c.reg.retireEntry(id) {
			c.evictions++
		}
		c.forgetLocked(id)
		return
	}
	c.pool[key] = append(c.pool[key], id)
	c.lru = append(c.lru, id)
	c.trimLocked()
}

// HitRate returns cache hit/miss counters (diagnostics / anti-fake-green).
func (c *Cache) HitRate() (hits, misses uint64) {
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses
}

// Evictions returns the number of budget/cap-driven retirements.
func (c *Cache) Evictions() uint64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.evictions
}

// Stats returns a per-domain snapshot. Idle counts entries that are actually
// sitting in a key bucket AND still reusable per the registry — the exact set
// Acquire can lend and PurgeIdle can drain. Registry-only would over-report:
// a hold cleared by the device-loss path leaves the entry reusable in the
// registry but never queued in the cache, so it can no longer be lent.
// The registry check runs outside the cache lock (lock order cache → registry).
func (c *Cache) Stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}
	c.mu.Lock()
	st := CacheStats{
		Name:      c.opt.Name,
		Hits:      c.hits,
		Misses:    c.misses,
		Evictions: c.evictions,
		Purged:    c.purged,
		Entries:   uint64(len(c.keyOf)),
		Bytes:     c.used,
		PeakBytes: c.peak,
		Budget:    c.opt.Budget,
	}
	pooled := make([]uint32, 0, len(c.lru))
	for _, ids := range c.pool {
		for _, id := range ids {
			if _, ok := c.keyOf[id]; ok {
				pooled = append(pooled, id)
			}
		}
	}
	c.mu.Unlock()
	for _, id := range pooled {
		if c.reg.idleEntry(id) {
			st.Idle++
		}
	}
	return st
}

// PurgeIdle retires every idle pooled entry and returns how many were purged
// and how many bytes were freed. Checked-out and in-flight entries are left
// alone (they retire through their normal path). This is the OOM/purge outlet:
// caches are drained before a device-wide fallback.
func (c *Cache) PurgeIdle() (int, uint64) {
	if c == nil || c.reg == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	var freed uint64
	for len(c.lru) > 0 {
		id := c.lru[0]
		c.lru = c.lru[1:]
		if _, ok := c.keyOf[id]; !ok {
			continue
		}
		if !c.reg.idleEntry(id) {
			continue // still referenced / in-flight: repool will re-queue it
		}
		size := c.sizes[id]
		if c.reg.retireEntry(id) {
			freed += size
			n++
		}
		c.forgetLocked(id)
	}
	c.purged += uint64(n)
	return n, freed
}

// InvalidateAll clears the pool without touching native resources (device
// loss path). Registry entries are dropped by Registry.InvalidateAll /
// Owner.InvalidateOwned; counters stay for post-mortem diagnostics.
func (c *Cache) InvalidateAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.pool = make(map[CacheKey][]uint32)
	c.keyOf = make(map[uint32]CacheKey)
	c.sizes = make(map[uint32]uint64)
	c.used = 0
	c.lru = c.lru[:0]
	c.mu.Unlock()
}

// trimLocked retires the LRU-most idle entries until estimated usage fits the
// budget. Retired entries are destroyed only when no longer in-flight.
func (c *Cache) trimLocked() {
	if c.opt.Budget == 0 {
		return
	}
	for c.used > c.opt.Budget {
		id := c.popLRUIdleLocked()
		if id == 0 {
			return // nothing idle left to retire
		}
		if c.reg.retireEntry(id) {
			c.evictions++
		}
		c.forgetLocked(id)
	}
}

// popLRUIdleLocked returns the oldest idle registry id, or 0 if none idle.
func (c *Cache) popLRUIdleLocked() uint32 {
	for len(c.lru) > 0 {
		id := c.lru[0]
		c.lru = c.lru[1:]
		if _, ok := c.keyOf[id]; !ok {
			continue
		}
		if !c.reg.idleEntry(id) {
			// Checked out or in flight: drop from the trim queue; the idle
			// callback appends it again when it truly becomes idle.
			continue
		}
		return id
	}
	return 0
}

// forgetLocked drops per-entry bookkeeping and byte accounting.
func (c *Cache) forgetLocked(id uint32) {
	if size, ok := c.sizes[id]; ok {
		if size <= c.used {
			c.used -= size
		} else {
			c.used = 0
		}
		delete(c.sizes, id)
	}
	delete(c.keyOf, id)
}

func (c *Cache) sizeOf(key CacheKey) uint64 {
	if c.opt.SizeFn != nil {
		return c.opt.SizeFn(key)
	}
	return defaultEntryBytes
}

// KeyOf returns the cache key for a registry id (diagnostics).
func (c *Cache) KeyOf(id uint32) (CacheKey, bool) {
	if c == nil {
		return CacheKey{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	k, ok := c.keyOf[id]
	return k, ok
}
