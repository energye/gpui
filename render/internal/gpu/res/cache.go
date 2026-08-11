package res

// CacheKey identifies a reusable GPU resource class (GrResourceCache key).
type CacheKey struct {
	W, H    uint32
	Format  uint32 // opaque format tag; callers map to their own enum
	Samples uint32
	Usage   uint64 // opaque usage tag
	Role    Role
}

// cacheEntry tracks one pooled resource: registry id plus reuse bookkeeping.
type cacheEntry struct {
	key CacheKey
	id  uint32
}

// CreateFn fabricates a native resource for a key (device.createTexture etc).
type CreateFn func(key CacheKey) (Native, error)

// Cache is a GrResourceCache-style pool: keyed reuse, byte budget with LRU
// eviction, and "in-flight items are never lent out" safety. Entries returned
// by Acquire are strong Refs; Release returns them to the pool. Over-budget
// idle entries are retired (destroyed once no longer in-flight).
type Cache struct {
	reg       *Registry
	create    CreateFn
	pool      map[CacheKey][]uint32 // key → idle registry ids (refs==0 && inflight==0)
	keyOf     map[uint32]CacheKey   // registry id → key
	pending   []uint32              // released while in-flight; re-pooled on submission finish
	used      uint64                // estimated bytes of live+pooled entries
	budget    uint64
	hits      uint64
	misses    uint64
	evictions uint64
	lru       []uint32 // idle ids, least-recently-retired first (trim order)
}

// NewCache creates a cache bound to reg. budget==0 disables byte budgeting.
func NewCache(reg *Registry, create CreateFn, budget uint64) *Cache {
	return &Cache{
		reg:    reg,
		create: create,
		pool:   make(map[CacheKey][]uint32),
		keyOf:  make(map[uint32]CacheKey),
		budget: budget,
	}
}

// Acquire returns a strong Ref for the key, reusing an idle pooled entry when
// possible. In-flight or currently-referenced entries are never lent out.
func (c *Cache) Acquire(key CacheKey) (Ref, error) {
	if c == nil || c.reg == nil {
		return Ref{}, nil
	}
	// Prefer an idle pooled entry (refs==0 && inflight==0; prune stale ids).
	bucket := c.pool[key]
	for len(bucket) > 0 {
		id := bucket[len(bucket)-1]
		bucket = bucket[:len(bucket)-1]
		c.pool[key] = bucket
		if s, ok := c.reg.slots[id]; ok && s.refs == 0 && s.inflight == 0 && !s.retire {
			c.hits++
			return c.reg.Acquire(id), nil
		}
		// Stale entry (already freed or retired): drop and try next.
		delete(c.keyOf, id)
	}
	// Create a fresh resource.
	c.misses++
	native, err := c.create(key)
	if err != nil {
		return Ref{}, err
	}
	ref := c.reg.Register(native)
	c.keyOf[ref.id] = key
	// Enforce budget: retire the oldest idle entries until under limit.
	c.trimLocked()
	return ref, nil
}

// Release returns a Ref to the pool. The registry refcount is decremented;
// the entry becomes recombinable once references reach zero and the resource
// is not in-flight. If the entry is still in-flight (submitted but not
// finished), it waits in the pending queue until OnSubmissionFinished.
func (c *Cache) Release(ref Ref) {
	if c == nil || ref.IsNil() || ref.reg != c.reg {
		return
	}
	id := ref.id
	key, ok := c.keyOf[id]
	if !ok {
		c.reg.Release(ref)
		return
	}
	c.reg.Release(ref)
	s, ok := c.reg.slots[id]
	if !ok {
		delete(c.keyOf, id)
		return
	}
	if s.retire {
		return // registry already owns destruction (removeIfReady fired)
	}
	if s.refs == 0 && s.inflight == 0 {
		c.repool(id, key)
	} else if s.inflight > 0 {
		c.pending = append(c.pending, id)
	}
	c.trimLocked()
}

// OnSubmissionFinished is called at submission-completion points: pending
// entries whose in-flight state has cleared become recombinable.
func (c *Cache) OnSubmissionFinished() {
	if c == nil {
		return
	}
	for _, id := range c.pending {
		s, ok := c.reg.slots[id]
		if !ok {
			delete(c.keyOf, id)
			continue
		}
		if s.refs == 0 && s.inflight == 0 && !s.retire {
			key, _ := c.keyOf[id]
			c.repool(id, key)
		}
	}
	c.pending = c.pending[:0]
	c.trimLocked()
}

// repool returns an idle entry to its key bucket and the LRU queue.
func (c *Cache) repool(id uint32, key CacheKey) {
	c.pool[key] = append(c.pool[key], id)
	c.lru = append(c.lru, id)
}

// HitRate returns cache hit/miss counters (diagnostics / anti-fake-green).
func (c *Cache) HitRate() (hits, misses uint64) {
	if c == nil {
		return 0, 0
	}
	return c.hits, c.misses
}

// Evictions returns the number of budget-driven retirements.
func (c *Cache) Evictions() uint64 {
	if c == nil {
		return 0
	}
	return c.evictions
}

// InvalidateAll clears the pool without touching native resources (device
// loss path). Registry entries are dropped by Registry.InvalidateAll.
func (c *Cache) InvalidateAll() {
	if c == nil {
		return
	}
	c.pool = make(map[CacheKey][]uint32)
	c.keyOf = make(map[uint32]CacheKey)
	c.lru = c.lru[:0]
	c.pending = c.pending[:0]
}

// trimLocked retires the LRU-most idle entries until estimated usage fits the
// budget. Retired entries are destroyed only when no longer in-flight.
func (c *Cache) trimLocked() {
	if c.budget == 0 {
		return
	}
	for c.estimatedBytes() > c.budget {
		id := c.popLRUIdle()
		if id == 0 {
			return // nothing idle left to retire
		}
		if s, ok := c.reg.slots[id]; ok {
			s.retire = true
			c.evictions++
			c.reg.removeIfReady(s)
		}
		delete(c.keyOf, id)
	}
}

// popLRUIdle returns the oldest idle registry id, or 0 if none idle.
func (c *Cache) popLRUIdle() uint32 {
	for len(c.lru) > 0 {
		id := c.lru[0]
		c.lru = c.lru[1:]
		s, ok := c.reg.slots[id]
		if !ok || s.refs != 0 || s.inflight != 0 {
			continue
		}
		return id
	}
	return 0
}

// estimatedBytes sums per-entry estimates for live+pooled resources.
// Entries do not carry a size today; we approximate by counting distinct keys
// against a fixed cost so the budget path is exercised deterministically.
func (c *Cache) estimatedBytes() uint64 {
	if c == nil {
		return 0
	}
	// Each live+pooled entry counts one fixed unit (entries are size-tracked
	// by the caller when precise byte accounting is needed).
	return uint64(len(c.keyOf)) * 4096
}

// KeyOf returns the cache key for a registry id (diagnostics).
func (c *Cache) KeyOf(id uint32) (CacheKey, bool) {
	if c == nil {
		return CacheKey{}, false
	}
	k, ok := c.keyOf[id]
	return k, ok
}