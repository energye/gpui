package render

import "sync"

// pixmapPool reuses full-surface Pixmaps for layers and filter intermediates (S6.4).
// Keyed by (width,height). Not safe for concurrent use of the same Pixmap;
// the pool itself is mutex-protected.
//
// Memory policy aligns with Skia's GrResourceCache semantics:
//   - EvictExcept: single-slot replacement on window resize — the context only
//     ever requests the current window size, so stale sizes are dropped at once
//     (Skia layer cache: key = size, resize purges the old key).
//   - Byte budget: budgeted resources are tracked against maxBytes (Skia
//     fMaxBytes). Put evicts the least-recently-used size bucket while over
//     budget, keeping at least the active bucket to avoid self-destruction.
type pixmapPool struct {
	mu         sync.Mutex
	buckets    map[pixmapPoolKey]*poolBucket
	maxPer     int
	maxBytes   int64
	totalBytes int64
	seq        int64
	evictions  int64
	gets       int
	puts       int
	hits       int
	misses     int
}

type pixmapPoolKey struct {
	w, h int
}

type poolBucket struct {
	items    []*Pixmap
	lastUsed int64
}

// defaultPixmapPoolBudgetBytes matches the Skia GrResourceCache default
// (kDefaultMaxBytes ≈ 96 MB) scale for CPU-side offscreen surfaces.
const defaultPixmapPoolBudgetBytes = 96 << 20

func newPixmapPool(maxPerBucket int) *pixmapPool {
	return newPixmapPoolBudget(maxPerBucket, defaultPixmapPoolBudgetBytes)
}

func newPixmapPoolBudget(maxPerBucket int, maxBytes int64) *pixmapPool {
	if maxPerBucket <= 0 {
		maxPerBucket = 8
	}
	return &pixmapPool{
		buckets:  make(map[pixmapPoolKey]*poolBucket),
		maxPer:   maxPerBucket,
		maxBytes: maxBytes,
	}
}

func pixmapBytes(pm *Pixmap) int64 {
	return int64(pm.Width()) * int64(pm.Height()) * 4
}

// Get returns a cleared transparent Pixmap of the given size.
func (p *pixmapPool) Get(w, h int) *Pixmap {
	return p.get(w, h, true)
}

// GetForOverwrite returns a Pixmap that will be fully overwritten by the caller
// (skips Clear). Used by backdrop snapshot and filter intermediates (S6.4).
func (p *pixmapPool) GetForOverwrite(w, h int) *Pixmap {
	return p.get(w, h, false)
}

func (p *pixmapPool) get(w, h int, clear bool) *Pixmap {
	if p == nil || w <= 0 || h <= 0 {
		pm := NewPixmap(w, h)
		if clear {
			pm.Clear(Transparent)
		}
		return pm
	}
	key := pixmapPoolKey{w: w, h: h}
	p.mu.Lock()
	p.gets++
	p.seq++
	bucket := p.buckets[key]
	if bucket != nil && len(bucket.items) > 0 {
		pm := bucket.items[len(bucket.items)-1]
		bucket.items = bucket.items[:len(bucket.items)-1]
		bucket.lastUsed = p.seq
		p.hits++
		p.mu.Unlock()
		if clear {
			pm.Clear(Transparent)
		}
		return pm
	}
	p.misses++
	p.mu.Unlock()
	pm := NewPixmap(w, h)
	// NewPixmap is zero-filled; Clear(Transparent) is redundant for fresh buffers.
	return pm
}

// Put returns a Pixmap to the pool. pm must not be used after Put.
func (p *pixmapPool) Put(pm *Pixmap) {
	if p == nil || pm == nil {
		return
	}
	key := pixmapPoolKey{w: pm.Width(), h: pm.Height()}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.puts++
	p.seq++
	bucket := p.buckets[key]
	if bucket == nil {
		bucket = &poolBucket{}
		p.buckets[key] = bucket
	}
	if p.maxPer > 0 && len(bucket.items) >= p.maxPer {
		return
	}
	bucket.items = append(bucket.items, pm)
	bucket.lastUsed = p.seq
	p.totalBytes += pixmapBytes(pm)
	p.evictLocked()
}

// evictLocked drops the least-recently-used size bucket while over budget.
// The most recently used bucket is never evicted, so an actively rendered
// size cannot thrash against the budget.
func (p *pixmapPool) evictLocked() {
	for p.totalBytes > p.maxBytes && len(p.buckets) > 1 {
		var oldestKey pixmapPoolKey
		var oldestSeq int64 = 1<<63 - 1
		for key, bucket := range p.buckets {
			if len(bucket.items) == 0 {
				continue
			}
			if bucket.lastUsed < oldestSeq {
				oldestSeq = bucket.lastUsed
				oldestKey = key
			}
		}
		if oldestSeq == p.seq || oldestSeq == 1<<63-1 {
			// Only the just-active bucket (or nothing) remains: stop.
			return
		}
		p.dropBucketLocked(oldestKey)
	}
}

// EvictExcept drops every size bucket other than (w,h) — the window-resize
// single-slot replacement (Skia layer cache semantics): stale sizes are never
// requested again, so they are released immediately.
func (p *pixmapPool) EvictExcept(w, h int) int64 {
	if p == nil {
		return 0
	}
	keep := pixmapPoolKey{w: w, h: h}
	p.mu.Lock()
	defer p.mu.Unlock()
	var freed int64
	for key := range p.buckets {
		if key != keep {
			freed += p.dropBucketLocked(key)
		}
	}
	return freed
}

func (p *pixmapPool) dropBucketLocked(key pixmapPoolKey) int64 {
	bucket := p.buckets[key]
	if bucket == nil {
		return 0
	}
	var freed int64
	for _, pm := range bucket.items {
		freed += pixmapBytes(pm)
	}
	p.totalBytes -= freed
	delete(p.buckets, key)
	p.evictions++
	return freed
}

// Stats returns pool counters (for tests / S6.4 diagnostics).
func (p *pixmapPool) Stats() (gets, puts, hits, misses int) {
	if p == nil {
		return 0, 0, 0, 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gets, p.puts, p.hits, p.misses
}

// Budget reports retained bytes and the byte budget (Skia fMaxBytes).
func (p *pixmapPool) Budget() (retainedBytes int64, budgetBytes int64) {
	if p == nil {
		return 0, 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.totalBytes, p.maxBytes
}

// Evictions reports how many size buckets were dropped by budget eviction or
// EvictExcept (diagnostics only).
func (p *pixmapPool) Evictions() int64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.evictions
}

// ResetStats clears counters without discarding pooled buffers.
func (p *pixmapPool) ResetStats() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.gets, p.puts, p.hits, p.misses = 0, 0, 0, 0
	p.mu.Unlock()
}
