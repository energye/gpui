package render

import "sync"

// pixmapPool reuses full-surface Pixmaps for layers and filter intermediates (S6.4).
// Keyed by (width,height). Not safe for concurrent use of the same Pixmap;
// the pool itself is mutex-protected.
type pixmapPool struct {
	mu      sync.Mutex
	buckets map[pixmapPoolKey][]*Pixmap
	maxPer  int
	gets    int
	puts    int
	hits    int
	misses  int
}

type pixmapPoolKey struct {
	w, h int
}

func newPixmapPool(maxPerBucket int) *pixmapPool {
	if maxPerBucket <= 0 {
		maxPerBucket = 8
	}
	return &pixmapPool{
		buckets: make(map[pixmapPoolKey][]*Pixmap),
		maxPer:  maxPerBucket,
	}
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
	bucket := p.buckets[key]
	if len(bucket) > 0 {
		pm := bucket[len(bucket)-1]
		p.buckets[key] = bucket[:len(bucket)-1]
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
	bucket := p.buckets[key]
	if p.maxPer > 0 && len(bucket) >= p.maxPer {
		return
	}
	// Leave pixels as-is; Get() clears on reuse.
	p.buckets[key] = append(bucket, pm)
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

// ResetStats clears counters without discarding pooled buffers.
func (p *pixmapPool) ResetStats() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.gets, p.puts, p.hits, p.misses = 0, 0, 0, 0
	p.mu.Unlock()
}
