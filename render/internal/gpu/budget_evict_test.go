//go:build !nogpu

package gpu

import (
	"testing"
	"time"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	"github.com/energye/gpui/gpu/types"
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

// --- dual-tex out pool (total bytes + per-size-bucket cap) ---

func TestDualTexOutPool_ByteBudgetEviction(t *testing.T) {
	c := &dualTexBlendCache{}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Nine 1024x1024 BGRA temps (4 MiB each = 36 MiB) across two buckets
	// exceeds the 32 MiB pool budget; the largest bucket drains first.
	for i := 0; i < 5; i++ {
		c.outPool = map[[2]int][]dualTexPooledTex{}
		c.outPool[[2]int{1024, 1024}] = append(c.outPool[[2]int{1024, 1024}],
			dualTexPooledTex{}, dualTexPooledTex{}, dualTexPooledTex{}, dualTexPooledTex{}, dualTexPooledTex{})
		c.outPool[[2]int{512, 512}] = append(c.outPool[[2]int{512, 512}],
			dualTexPooledTex{}, dualTexPooledTex{}, dualTexPooledTex{}, dualTexPooledTex{})
		break
	}
	// Rebuild deterministically: 5×4MiB + 4×1MiB = 24MiB (within budget).
	c.outPool = map[[2]int][]dualTexPooledTex{
		{1024, 1024}: make([]dualTexPooledTex, 5),
		{512, 512}:   make([]dualTexPooledTex, 4),
	}
	c.enforceOutPoolBudgetLocked()
	used := 0
	for key, bucket := range c.outPool {
		used += key[0] * key[1] * 4 * len(bucket)
	}
	if used > 32<<20 {
		t.Fatalf("pooled bytes = %d, want <= 32MiB after enforce", used)
	}
	if len(c.outPool[[2]int{1024, 1024}]) != 5 || len(c.outPool[[2]int{512, 512}]) != 4 {
		t.Fatalf("within-budget state changed: %+v", c.outPool)
	}
	// Push past the budget: three more 4MiB entries (cap 8 per bucket →
	// 8×4MiB + 4×1MiB = 36MiB) — the biggest bucket drains until the total
	// fits exactly (7×4MiB + 4×1MiB = 32MiB).
	c.outPool[[2]int{1024, 1024}] = append(c.outPool[[2]int{1024, 1024}],
		dualTexPooledTex{}, dualTexPooledTex{}, dualTexPooledTex{})
	c.enforceOutPoolBudgetLocked()
	used = 0
	for key, bucket := range c.outPool {
		used += key[0] * key[1] * 4 * len(bucket)
	}
	if used != 32<<20 {
		t.Fatalf("pooled bytes = %d, want exactly 32MiB after overflow enforce", used)
	}
	if got := len(c.outPool[[2]int{1024, 1024}]); got != 7 {
		t.Fatalf("largest bucket len = %d, want 7 (drained to the byte cap)", got)
	}
}

func TestDualTexOutPool_PerBucketCap(t *testing.T) {
	dev := newNoopTestDevice(t)
	c := &dualTexBlendCache{}
	defer c.release()
	// The 9th same-size put must be rejected (destroyed, not pooled).
	var kept int
	for i := 0; i < 9; i++ {
		tex, err := dev.CreateTexture(&hal.TextureDescriptor{
			Label: "dualtex_cap_test", Size: hal.Extent3D{Width: 64, Height: 64, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm, Usage: types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatalf("CreateTexture: %v", err)
		}
		view, err := dev.CreateTextureView(tex, nil)
		if err != nil {
			tex.Destroy()
			t.Fatalf("CreateTextureView: %v", err)
		}
		c.putOutBGRA(tex, view, 64, 64)
	}
	c.mu.Lock()
	kept = len(c.outPool[[2]int{64, 64}])
	c.mu.Unlock()
	if kept != 8 {
		t.Fatalf("bucket len = %d, want cap 8", kept)
	}
}

// --- stencil scratch pool (size buckets, LRU at cap 8) ---

func TestStencilPool_LRUEviction(t *testing.T) {
	dev := newNoopTestDevice(t)
	ts := &textureSet{}
	for i := 0; i < 9; i++ {
		w := uint32(64 + i)
		if v := ts.takePooledStencil(dev, w, 64, "stencil_budget_test"); v == nil {
			t.Fatalf("take %d returned nil", i)
		}
	}
	if len(ts.stencilPool) != 8 {
		t.Fatalf("pool len = %d, want cap 8", len(ts.stencilPool))
	}
	if _, ok := ts.stencilPool[stencilPoolKey{w: 64, h: 64}]; ok {
		t.Fatal("size 64x64 (oldest) still pooled — LRU eviction did not run")
	}
	// Touch an existing entry, then overflow again: the touched one survives.
	if v := ts.takePooledStencil(dev, 65, 64, "stencil_budget_test"); v == nil {
		t.Fatal("re-take 65x64 returned nil")
	}
	if v := ts.takePooledStencil(dev, 73, 64, "stencil_budget_test"); v == nil {
		t.Fatal("take 73x64 returned nil")
	}
	if _, ok := ts.stencilPool[stencilPoolKey{w: 65, h: 64}]; !ok {
		t.Fatal("touched entry 65x64 was evicted — LRU order broken")
	}
	if _, ok := ts.stencilPool[stencilPoolKey{w: 66, h: 64}]; ok {
		t.Fatal("untouched entry 66x64 survived — wrong victim")
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
