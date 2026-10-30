//go:build !nogpu

package gpu

import (
	"testing"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render/internal/gpu/res"
	"github.com/energye/gpui/render/text"
)

// budget_evict_test.go pins the eviction semantics of the budget points that
// had no coverage before P2.4b (evidence/P2.4b/assessment.md §0): depth-clip
// mask cache, dual-tex out pool, stencil scratch LRU, glyph upload throttle.
// The tests run against the pre-P2.4b constants and must stay green after
// the declaration-table switch — they are the equivalence proof.

// newNoopTestDevice builds a full noop device (no GPU required).
func newNoopTestDevice(t *testing.T) hal.Device {
	t.Helper()
	be := noop.NewBackend()
	inst, err := be.CreateInstance(nil)
	if err != nil {
		t.Fatalf("noop CreateInstance: %v", err)
	}
	t.Cleanup(func() { inst.Release() })
	adapters := inst.EnumerateAdapters(nil)
	if len(adapters) == 0 {
		t.Fatal("noop: no adapters")
	}
	opened, err := adapters[0].Adapter.Open(0, types.Limits{})
	if err != nil {
		t.Fatalf("noop Open: %v", err)
	}
	t.Cleanup(func() { opened.Device.Release() })
	return opened.Device
}

// --- depth-clip mask cache (count / bytes / TTL / last-entry-never) ---

func newMaskEntry(key uint64, bytes int, lastUsed time.Time) *depthMaskEntry {
	return &depthMaskEntry{key: key, bytes: bytes, lastUsed: lastUsed}
}

func TestDepthClipMaskCache_CountEviction(t *testing.T) {
	p := NewDepthClipPipeline(nil, nil, 1)
	now := time.Now()
	p.maskCacheMu.Lock()
	defer p.maskCacheMu.Unlock()
	// 1 KiB entries: the byte cap (16 MiB) never binds, so the count cap
	// (24) is the one doing the eviction.
	for i := 0; i < 25; i++ {
		p.addMaskLocked(newMaskEntry(uint64(i), 1<<10, now))
	}
	if len(p.maskCache) != 24 {
		t.Fatalf("cache len = %d, want cap 24", len(p.maskCache))
	}
	if p.maskCache[0].key != 1 {
		t.Fatalf("oldest key = %d, want 1 (key 0 evicted)", p.maskCache[0].key)
	}
	if p.maskCache[len(p.maskCache)-1].key != 24 {
		t.Fatalf("newest key = %d, want 24", p.maskCache[len(p.maskCache)-1].key)
	}
	if p.maskCacheBytes != 24<<10 {
		t.Fatalf("bytes = %d, want %d", p.maskCacheBytes, 24<<10)
	}
}

func TestDepthClipMaskCache_ByteEviction(t *testing.T) {
	p := NewDepthClipPipeline(nil, nil, 1)
	now := time.Now()
	p.maskCacheMu.Lock()
	defer p.maskCacheMu.Unlock()
	// Three 8 MiB masks: the third add pushes bytes past the 16 MiB cap and
	// the first mask is evicted; 16 MiB == cap stops the loop.
	for i := 0; i < 3; i++ {
		p.addMaskLocked(newMaskEntry(uint64(i), 8<<20, now))
	}
	if len(p.maskCache) != 2 {
		t.Fatalf("cache len = %d, want 2", len(p.maskCache))
	}
	if p.maskCache[0].key != 1 {
		t.Fatalf("oldest key = %d, want 1 (key 0 evicted by bytes)", p.maskCache[0].key)
	}
	if p.maskCacheBytes != 16<<20 {
		t.Fatalf("bytes = %d, want exactly the 16MiB cap", p.maskCacheBytes)
	}
}

func TestDepthClipMaskCache_TTLPrune(t *testing.T) {
	p := NewDepthClipPipeline(nil, nil, 1)
	now := time.Now()
	p.maskCacheMu.Lock()
	defer p.maskCacheMu.Unlock()
	p.addMaskLocked(newMaskEntry(1, 1<<20, now.Add(-6*time.Second))) // stale (> 5s TTL)
	p.addMaskLocked(newMaskEntry(2, 1<<20, now))                     // fresh
	p.pruneMaskCacheLocked(now)
	if len(p.maskCache) != 1 {
		t.Fatalf("cache len = %d, want 1 after TTL prune", len(p.maskCache))
	}
	if p.maskCache[0].key != 2 {
		t.Fatalf("surviving key = %d, want 2 (fresh)", p.maskCache[0].key)
	}
	if p.maskCacheBytes != 1<<20 {
		t.Fatalf("bytes = %d, want 1MiB", p.maskCacheBytes)
	}
}

func TestDepthClipMaskCache_LastEntryNeverEvicted(t *testing.T) {
	p := NewDepthClipPipeline(nil, nil, 1)
	now := time.Now()
	p.maskCacheMu.Lock()
	defer p.maskCacheMu.Unlock()
	// One entry larger than the whole budget: the loop guards len > 1, so
	// the only entry survives (the live clip must never drop its own mask).
	p.addMaskLocked(newMaskEntry(1, 32<<20, now))
	if len(p.maskCache) != 1 {
		t.Fatalf("cache len = %d, want 1 (last entry never evicted)", len(p.maskCache))
	}
	if p.maskCacheBytes != 32<<20 {
		t.Fatalf("bytes = %d, want 32MiB", p.maskCacheBytes)
	}
}

// --- dual-tex snap pool (device-level res.Cache: byte budget + per-size cap) ---

// newPooledDualTexCache returns a dual-tex cache bound to a fresh device
// registry, so its snaps go through the real res.Cache path (P2.4d) instead of
// the removed hand-rolled bucket map.
func newPooledDualTexCache(t *testing.T) *dualTexBlendCache {
	t.Helper()
	c := &dualTexBlendCache{}
	c.bindRes(res.NewRegistry().DeviceOwner(), nil)
	t.Cleanup(c.release)
	return c
}

// snapOut takes one pooled snap and hands it straight back.
func poolSnapInOut(t *testing.T, c *dualTexBlendCache, dev hal.Device, w, h int) dualTexViewBlendOut {
	t.Helper()
	tex, view, ref, err := c.getOutBGRA(dev, nil, w, h)
	if err != nil {
		t.Fatalf("getOutBGRA(%d,%d): %v", w, h, err)
	}
	if ref.IsNil() {
		t.Fatalf("getOutBGRA(%d,%d): snap is not pooled", w, h)
	}
	return dualTexViewBlendOut{tex: tex, view: view, ref: ref}
}

func dualTexSnapCache(t *testing.T, c *dualTexBlendCache) *res.Cache {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.outCache == nil {
		t.Fatal("dual-tex snap cache was never built")
	}
	return c.outCache
}

// TestDualTexOutPool_ByteBudgetEviction pins the domain byte budget on the
// unified cache: a frame may hold more than the budget (live entries are not
// evictable), but everything handed back above the cap is retired LRU-first
// until the pooled bytes fit again. Same 32 MiB number the pre-P2.4d
// hand-rolled enforce used.
func TestDualTexOutPool_ByteBudgetEviction(t *testing.T) {
	dev := newNoopTestDevice(t)
	c := newPooledDualTexCache(t)
	// 8 × 1024² BGRA (4 MiB) + 8 × 512² (1 MiB) = 40 MiB, all checked out.
	var held []dualTexViewBlendOut
	for i := 0; i < 8; i++ {
		held = append(held, poolSnapInOut(t, c, dev, 1024, 1024))
	}
	for i := 0; i < 8; i++ {
		held = append(held, poolSnapInOut(t, c, dev, 512, 512))
	}
	cache := dualTexSnapCache(t, c)
	if st := cache.Stats(); st.Bytes != 40<<20 || st.Evictions != 0 {
		t.Fatalf("live domain: bytes=%d evictions=%d, want 40MiB and 0 (checked-out entries are not evictable)",
			st.Bytes, st.Evictions)
	}
	// Hand everything back: the LRU head (oldest recycled 1024² snaps) is
	// retired until the pool fits the 32 MiB budget again.
	for _, o := range held {
		c.recycleOut(o, nil)
	}
	st := cache.Stats()
	if st.Bytes != 32<<20 {
		t.Fatalf("pooled bytes = %d, want exactly 32MiB after overflow", st.Bytes)
	}
	if st.Evictions != 2 {
		t.Fatalf("evictions = %d, want 2 (two 4MiB snaps retired to fit)", st.Evictions)
	}
	if st.Entries != 14 || st.Idle != 14 {
		t.Fatalf("entries=%d idle=%d, want 14/14 (6×1024² + 8×512²)", st.Entries, st.Idle)
	}
}

// TestDualTexOutPool_PerBucketCap pins the per-size-bucket cap: below the byte
// budget, an identical-size burst still cannot grow one bucket without bound.
func TestDualTexOutPool_PerBucketCap(t *testing.T) {
	dev := newNoopTestDevice(t)
	c := newPooledDualTexCache(t)
	var held []dualTexViewBlendOut
	for i := 0; i < 10; i++ {
		held = append(held, poolSnapInOut(t, c, dev, 512, 512))
	}
	cache := dualTexSnapCache(t, c)
	// Return them: the 9th and 10th past the cap of 8 are retired, not pooled.
	for _, o := range held {
		c.recycleOut(o, nil)
	}
	st := cache.Stats()
	if st.Entries != 8 || st.Idle != 8 {
		t.Fatalf("entries=%d idle=%d, want 8/8 (bucket cap 8)", st.Entries, st.Idle)
	}
	if st.Evictions != 2 {
		t.Fatalf("evictions = %d, want 2 (overflow retired)", st.Evictions)
	}
	if st.Bytes != 8<<20 {
		t.Fatalf("pooled bytes = %d, want 8MiB", st.Bytes)
	}
}

// TestDualTexOutPool_PurgeKeepsCheckedOut pins the OOM/purge outlet: an
// idle-drain frees everything pooled while a frame is still using its snaps.
func TestDualTexOutPool_PurgeKeepsCheckedOut(t *testing.T) {
	dev := newNoopTestDevice(t)
	c := newPooledDualTexCache(t)
	held := poolSnapInOut(t, c, dev, 256, 256)
	back := poolSnapInOut(t, c, dev, 256, 256)
	c.recycleOut(back, nil)
	cache := dualTexSnapCache(t, c)
	if st := cache.Stats(); st.Idle != 1 {
		t.Fatalf("idle = %d, want 1 before purge", st.Idle)
	}
	c.releasePooledVRAM()
	st := cache.Stats()
	if st.Entries != 1 || st.Idle != 0 {
		t.Fatalf("after purge: entries=%d idle=%d, want the checked-out snap only", st.Entries, st.Idle)
	}
	c.recycleOut(held, nil)
	if st := cache.Stats(); st.Entries != 1 {
		t.Fatalf("after the last release: entries=%d, want 1", st.Entries)
	}
}

// --- image cache fair-share branches (w injected via effBudgetFor) ---

func TestImageCacheFairShare_Branches(t *testing.T) {
	c := &ImageCache{}
	if got := c.effBudgetFor(0); got != 128 {
		t.Errorf("w=0: %d, want default 128", got)
	}
	if got := c.effBudgetFor(1); got != 128 {
		t.Errorf("w=1: %d, want default 128 (single-window unchanged)", got)
	}
	if got := c.effBudgetFor(2); got != 64 {
		t.Errorf("w=2: %d, want 64", got)
	}
	if got := c.effBudgetFor(8); got != 16 {
		t.Errorf("w=8: %d, want 16", got)
	}
	if got := c.effBudgetFor(32); got != 16 {
		t.Errorf("w=32: %d, want floor 16 (128/32 < floor)", got)
	}
	if got := c.effBudgetBytesFor(2); got != 32<<20 {
		t.Errorf("bytes w=2: %d, want 32MiB", got)
	}
	if got := c.effBudgetBytesFor(32); got != 8<<20 {
		t.Errorf("bytes w=32: %d, want floor 8MiB", got)
	}
	// Explicit budgets win over any share.
	e := &ImageCache{explicitBudgets: true, budget: 10, budgetBytes: 1 << 20}
	if got := e.effBudgetFor(8); got != 10 {
		t.Errorf("explicit w=8: %d, want 10", got)
	}
	if got := e.effBudgetBytesFor(8); got != 1<<20 {
		t.Errorf("explicit bytes w=8: %d, want 1MiB", got)
	}
}

// --- glyph per-frame upload budget (pure throttle) ---

func TestGlyphUploadBudget_KeepsSmallestWhenOver(t *testing.T) {
	ups := []text.GlyphMaskDirtyUpload{
		{Index: 0, W: 10, H: 10}, // 100
		{Index: 1, W: 20, H: 10}, // 200
		{Index: 2, W: 30, H: 10}, // 300
		{Index: 3, W: 40, H: 10}, // 400
	}
	// Over budget → the largest uploads are marked Deferred (they ride the
	// next frame; the list itself is never reordered or dropped).
	got := applyGlyphUploadBudget(func(int) int { return 1024 * 1024 }, ups, 500)
	if len(got) != 4 {
		t.Fatalf("list len = %d, want 4 (defer, not drop)", len(got))
	}
	for _, i := range []int{2, 3} {
		if !got[i].Deferred {
			t.Fatalf("upload %d should be deferred (largest dropped first)", i)
		}
	}
	if got[2].DeferredBytes != 300 || got[3].DeferredBytes != 400 {
		t.Fatalf("deferred bytes = %d/%d, want 300/400", got[2].DeferredBytes, got[3].DeferredBytes)
	}
	if got[0].Deferred || got[1].Deferred {
		t.Fatalf("uploads 0/1 fit the budget and must ride this frame")
	}
}

func TestGlyphUploadBudget_WithinBudgetUnchanged(t *testing.T) {
	ups := []text.GlyphMaskDirtyUpload{{Index: 0, W: 10, H: 10}, {Index: 1, W: 20, H: 10}}
	got := applyGlyphUploadBudget(func(int) int { return 1024 * 1024 }, ups, 1<<20)
	if len(got) != 2 {
		t.Fatalf("within-budget run changed the list: %d", len(got))
	}
}

func TestGlyphUploadBudget_NonPositiveBudgetNoop(t *testing.T) {
	ups := []text.GlyphMaskDirtyUpload{{Index: 0, W: 4096, H: 4096}}
	got := applyGlyphUploadBudget(func(int) int { return 1 }, ups, 0)
	if len(got) != 1 {
		t.Fatalf("budget=0 must be a no-op, got %d", len(got))
	}
}
