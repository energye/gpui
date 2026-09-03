package text

import (
	"math"
	"sync"
	"sync/atomic"
)

// advanceKey identifies one cached advance. Every parameter that can change
// the advance must be in the key — a missing field is a collision:
//   - vertical: vmtx heights vs hmtx widths are different values;
//   - tab: tab-stop advance shares the space GID with a different advance;
//   - hinted: TT-hinted widths differ from raw advances for the same glyph;
//   - size is stored exact (not quantized): hinted widths key on integer
//     ppem, so rounding the size could return another ppem's widths.
type advanceKey struct {
	fontID   uint64
	gid      uint32
	sizeBits uint64
	hinting  Hinting
	varHash  uint64
	flags    uint8
}

const (
	advanceFlagVertical uint8 = 1 << iota
	advanceFlagTab
	advanceFlagHinted
)

// AdvanceCacheStats reports global advance cache counters.
type AdvanceCacheStats struct {
	Hits      uint64
	Misses    uint64
	Entries   int
	SoftLimit int
	Evictions uint64
}

type advanceCacheEntry struct {
	advance float64
	atime   int64
}

// advanceCache is a process-wide soft-LRU for per-glyph advances, same
// discipline as shapeResultCache (map + tick/atime, never-hit fast lane,
// 3/4-target oldest eviction). Values are plain float64.
type advanceCache struct {
	mu        sync.Mutex
	entries   map[advanceKey]*advanceCacheEntry
	softLimit int
	tick      int64
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

const defaultAdvanceSoftLimit = 32768

var globalAdvanceCache = newAdvanceCache(defaultAdvanceSoftLimit)

func newAdvanceCache(softLimit int) *advanceCache {
	if softLimit <= 0 {
		softLimit = defaultAdvanceSoftLimit
	}
	return &advanceCache{
		entries:   make(map[advanceKey]*advanceCacheEntry),
		softLimit: softLimit,
	}
}

// AdvanceCacheStats returns counters for the global advance cache.
func AdvanceCacheSnapshot() AdvanceCacheStats {
	return globalAdvanceCache.stats()
}

// ResetAdvanceCacheStats clears hit/miss/eviction counters (entries retained).
func ResetAdvanceCacheStats() {
	globalAdvanceCache.resetStats()
}

// ClearAdvanceCache drops all cached advances and resets stats.
func ClearAdvanceCache() {
	globalAdvanceCache.clear()
}

func (c *advanceCache) stats() AdvanceCacheStats {
	c.mu.Lock()
	n := len(c.entries)
	lim := c.softLimit
	c.mu.Unlock()
	return AdvanceCacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Entries:   n,
		SoftLimit: lim,
		Evictions: c.evictions.Load(),
	}
}

func (c *advanceCache) resetStats() {
	c.hits.Store(0)
	c.misses.Store(0)
	c.evictions.Store(0)
}

func (c *advanceCache) clear() {
	c.mu.Lock()
	c.entries = make(map[advanceKey]*advanceCacheEntry)
	c.tick = 0
	c.mu.Unlock()
	c.resetStats()
}

func (c *advanceCache) get(key advanceKey) (float64, bool) {
	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		return 0, false
	}
	c.tick++
	e.atime = c.tick
	v := e.advance
	c.mu.Unlock()
	c.hits.Add(1)
	return v, true
}

func (c *advanceCache) set(key advanceKey, v float64) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.tick++
		e.atime = c.tick
		e.advance = v
		c.mu.Unlock()
		c.hits.Add(1)
		return
	}
	c.tick++
	c.entries[key] = &advanceCacheEntry{advance: v, atime: 0}
	if c.softLimit > 0 && len(c.entries) > c.softLimit {
		c.evictOldestLocked()
	}
	c.mu.Unlock()
	c.misses.Add(1)
}

func (c *advanceCache) getOrCreate(key advanceKey, create func() float64) float64 {
	if v, ok := c.get(key); ok {
		return v
	}
	v := create()
	c.set(key, v)
	return v
}

func (c *advanceCache) evictOldestLocked() {
	target := c.softLimit * 3 / 4
	if target < 1 {
		target = 1
	}
	for k, e := range c.entries {
		if e.atime == 0 && len(c.entries) > target {
			delete(c.entries, k)
			c.evictions.Add(1)
		}
	}
	if len(c.entries) <= target {
		return
	}
	toEvict := len(c.entries) - target
	for i := 0; i < toEvict; i++ {
		var oldestKey advanceKey
		var oldestAtime int64 = math.MaxInt64
		found := false
		for k, e := range c.entries {
			if e.atime < oldestAtime {
				oldestAtime = e.atime
				oldestKey = k
				found = true
			}
		}
		if !found {
			break
		}
		delete(c.entries, oldestKey)
		c.evictions.Add(1)
	}
}

// advanceCacheKey builds the lookup key for one advance computation.
func advanceCacheKey(fontID uint64, gid uint16, size float64, hinting Hinting, varHash uint64, vertical, tab, hinted bool) advanceKey {
	var flags uint8
	if vertical {
		flags |= advanceFlagVertical
	}
	if tab {
		flags |= advanceFlagTab
	}
	if hinted {
		flags |= advanceFlagHinted
	}
	return advanceKey{
		fontID:   fontID,
		gid:      uint32(gid),
		sizeBits: math.Float64bits(size),
		hinting:  hinting,
		varHash:  varHash,
		flags:    flags,
	}
}

// advanceLookupKey builds the lookup key for one advance computation.
func (f *sourceFace) advanceLookupKey(gid uint16, r rune, hinted bool) advanceKey {
	var varHash uint64
	if len(f.config.variations) > 0 {
		varHash = HashFontVariations(f.config.variations)
	}
	return advanceCacheKey(
		f.cachedFontID(),
		gid,
		f.size,
		f.config.hinting,
		varHash,
		f.config.direction.IsVertical(),
		r == '\t',
		hinted,
	)
}

// lookupAdvance hits the cache without allocating (hot path).
func (f *sourceFace) lookupAdvance(gid uint16, r rune, hinted bool) (float64, bool) {
	return globalAdvanceCache.get(f.advanceLookupKey(gid, r, hinted))
}

// storeAdvance inserts a computed advance.
func (f *sourceFace) storeAdvance(gid uint16, r rune, hinted bool, v float64) {
	globalAdvanceCache.set(f.advanceLookupKey(gid, r, hinted), v)
}

// tabAdvanceCached returns the cached tab-stop advance (space GID carrier).
func (f *sourceFace) tabAdvanceCached(parsed ParsedFont, r rune) (uint16, float64) {
	gid, _ := tabAdvance(parsed, f.size)
	if v, ok := f.lookupAdvance(gid, r, false); ok {
		return gid, v
	}
	gid, adv := tabAdvance(parsed, f.size)
	f.storeAdvance(gid, r, false, adv)
	return gid, adv
}

// rawAdvanceCached returns the cached unhinted advance (vertical- and
// variation-aware via glyphAdvance).
func (f *sourceFace) rawAdvanceCached(parsed ParsedFont, gid uint16, r rune, varProvider VariableAdvanceProvider) float64 {
	if v, ok := f.lookupAdvance(gid, r, false); ok {
		return v
	}
	adv, _ := f.glyphAdvance(parsed, gid, varProvider)
	f.storeAdvance(gid, r, false, adv)
	return adv
}
