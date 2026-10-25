package gpu

import "sync"

// brushFieldCacheKey identifies one cached ColorAt field: brush seed +
// field geometry (bounds + resolution). See fillColorAtFieldMaskedGPU (C1):
// a stable brush re-sampled every frame paid the full nw*nh ColorAt sweep
// plus a fresh buffer per call (measured 1.56MB / 140 allocs per fill at
// 512x320 before memoization).
type brushFieldCacheKey struct {
	seed uint64
	geom uint64
	w, h int
}

// fieldCache is a small drop-oldest memo (capacity 8) for ColorAt fields on
// the shared GPU state. Fields are large (up to 512²×4 ≈ 1MB), so capacity
// stays small; entries are reused in place (no per-frame allocation on the
// steady state).
type fieldCache struct {
	mu      sync.Mutex
	entries map[brushFieldCacheKey][]byte
	stamps  map[brushFieldCacheKey]uint64
	stamp   uint64
}

func (c *fieldCache) get(k brushFieldCacheKey, size int) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	buf, ok := c.entries[k]
	if !ok || len(buf) != size {
		return nil, false
	}
	c.stamp++
	c.stamps[k] = c.stamp
	return buf, true
}

func (c *fieldCache) put(k brushFieldCacheKey, buf []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[brushFieldCacheKey][]byte, 8)
		c.stamps = make(map[brushFieldCacheKey]uint64, 8)
	}
	c.stamp++
	c.stamps[k] = c.stamp
	c.entries[k] = buf
	if len(c.entries) > 8 {
		oldest := k
		var oldestStamp uint64 = ^uint64(0)
		for kk, st := range c.stamps {
			if st < oldestStamp {
				oldestStamp = st
				oldest = kk
			}
		}
		delete(c.entries, oldest)
		delete(c.stamps, oldest)
	}
}
