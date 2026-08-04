package render

import "testing"

func TestPixmapPool_GetReuseStillWorks(t *testing.T) {
	p := newPixmapPool(2)
	pm := p.Get(10, 10)
	if pm == nil || pm.Width() != 10 || pm.Height() != 10 {
		t.Fatalf("Get returned %v", pm)
	}
	p.Put(pm)
	pm2 := p.Get(10, 10)
	if pm2 != pm {
		t.Fatal("Get after Put must reuse the pooled Pixmap")
	}
	gets, puts, hits, misses := p.Stats()
	if gets != 2 || puts != 1 || hits != 1 || misses != 1 {
		t.Fatalf("stats = g%d p%d h%d m%d, want 2 1 1 1", gets, puts, hits, misses)
	}
	p.ResetStats()
	gets, _, _, _ = p.Stats()
	if gets != 0 {
		t.Fatalf("stats after reset = %d, want 0", gets)
	}
}

func TestPixmapPool_EvictExcept_DropsStaleSizes(t *testing.T) {
	p := newPixmapPool(4)
	a := p.Get(10, 10)
	b := p.Get(20, 20)
	c := p.Get(30, 30)
	p.Put(a)
	p.Put(b)
	p.Put(c)

	if got := p.EvictExcept(20, 20); got != int64((10*10+30*30)*4) {
		t.Fatalf("EvictExcept freed %d bytes, want %d", got, int64((10*10+30*30)*4))
	}
	p.mu.Lock()
	if len(p.buckets) != 1 {
		t.Fatalf("expected 1 bucket after EvictExcept, got %d", len(p.buckets))
	}
	if _, ok := p.buckets[pixmapPoolKey{20, 20}]; !ok {
		t.Fatal("current-size bucket must survive EvictExcept")
	}
	p.mu.Unlock()
}

func TestPixmapPool_BudgetEvictsLRUBucket(t *testing.T) {
	p := newPixmapPoolBudget(8, 1000) // 10x10=400B, 12x12=576B
	a := p.Get(10, 10)
	b := p.Get(10, 10)
	c := p.Get(12, 12)
	p.Put(a)
	p.Put(b)
	p.Put(c) // total 400+400+576=1376 > 1000 -> evict LRU (10x10 bucket)
	p.mu.Lock()
	if _, ok := p.buckets[pixmapPoolKey{10, 10}]; ok {
		t.Fatal("LRU 10x10 bucket must be evicted when over budget")
	}
	if _, ok := p.buckets[pixmapPoolKey{12, 12}]; !ok {
		t.Fatal("most-recently-used 12x12 bucket must survive")
	}
	p.mu.Unlock()
	retained, budget := p.Budget()
	if retained != 576 {
		t.Fatalf("retained=%d want 576", retained)
	}
	if budget != 1000 {
		t.Fatalf("budget=%d want 1000", budget)
	}
}

func TestPixmapPool_KeepsActiveBucketOverBudget(t *testing.T) {
	p := newPixmapPoolBudget(8, 1000) // 10x10=400B; 3 puts = 1200B over budget
	a := p.Get(10, 10)
	b := p.Get(10, 10)
	c := p.Get(10, 10)
	p.Put(a)
	p.Put(b)
	p.Put(c)
	p.mu.Lock()
	if got := len(p.buckets[pixmapPoolKey{10, 10}].items); got != 3 {
		t.Fatalf("active bucket must keep all 3 items over budget, got %d", got)
	}
	p.mu.Unlock()
	if got := p.Evictions(); got != 0 {
		t.Fatalf("no eviction expected for a single active size, got %d", got)
	}
}

func TestContext_Resize_EvictsStaleLayerSizes(t *testing.T) {
	dc := NewContext(1200, 800)
	dc.PushLayerIsolated(0.55)
	dc.SetRGBA(0.5, 0.5, 0.5, 1)
	dc.DrawRectangle(0, 0, 1200, 800)
	_ = dc.Fill()
	dc.PopLayer()
	pool := dc.layerStack.pool
	if pool == nil {
		t.Fatal("layerStack.pool must exist")
	}
	pool.mu.Lock()
	if len(pool.buckets) != 1 {
		t.Fatalf("expected 1 size bucket after first frame, got %d", len(pool.buckets))
	}
	pool.mu.Unlock()

	if err := dc.Resize(900, 700); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	pool.mu.Lock()
	n := len(pool.buckets)
	pool.mu.Unlock()
	if n != 0 {
		t.Fatalf("stale 1200x800 bucket must be evicted by Resize single-slot replacement, got %d buckets", n)
	}
	if got := pool.Evictions(); got != 1 {
		t.Fatalf("Evictions=%d want 1", got)
	}
}
