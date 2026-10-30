//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/hal/noop"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// surface_cache_test.go pins the device-level surface trio cache (P2.4d): the
// pool is keyed by size + samples + mode, one owner holds one trio at a time,
// degraded trios never re-enter the pool, and the budget / per-key cap / idle
// purge are the three outlets that keep a resize storm from pinning VRAM.
// Runs on the noop device — no GPU, no skip.

func newSurfaceCacheForTest(t *testing.T, budget uint64, maxPerKey int) (hal.Device, *surfaceCache, res.Owner) {
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

	reg := res.NewRegistry()
	owner := reg.DeviceOwner()
	c := newSurfaceCacheWithLimits(owner, "test-surface", budget, maxPerKey)
	c.setDevice(opened.Device)
	return opened.Device, c, owner
}

// Two owners requesting the same size must never be handed the same native:
// the strong Ref is the exclusivity contract (D1). Once one releases, the next
// request for that key hits instead of allocating.
func TestSurfaceCache_TwoOwnersSameSizeGetDistinctTrios(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	key := surfaceKey(64, 32, 1, false, false)

	refA, trioA, err := c.acquire(device, key)
	if err != nil {
		t.Fatalf("owner A acquire: %v", err)
	}
	if trioA.stencilView == nil {
		t.Fatal("trio A must carry a depth/stencil view")
	}
	if trioA.resolveTex != nil {
		t.Fatal("surface-mode trio must not allocate a resolve target")
	}

	refB, trioB, err := c.acquire(device, key)
	if err != nil {
		t.Fatalf("owner B acquire: %v", err)
	}
	if trioB == trioA {
		t.Fatal("two owners must not share one trio while both hold refs")
	}

	c.release(refA)
	if st := c.stats(); st.Entries != 2 || st.Idle != 1 {
		t.Fatalf("after A release: entries=%d idle=%d want 2/1", st.Entries, st.Idle)
	}

	refC, trioC, err := c.acquire(device, key)
	if err != nil {
		t.Fatalf("owner C acquire: %v", err)
	}
	if trioC != trioA {
		t.Fatal("released trio must be handed to the next same-key request")
	}
	if st := c.stats(); st.Hits != 1 || st.Misses != 2 {
		t.Fatalf("hits=%d misses=%d want 1/2", st.Hits, st.Misses)
	}
	c.release(refB)
	c.release(refC)
}

// The resolve flag is part of the key: a readback (offscreen) owner must never
// be served a surface trio without resolve, and vice versa.
func TestSurfaceCache_ResolveModeIsDistinctKey(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	surfaceKeyV := surfaceKey(48, 40, 1, false, false)
	offscreenKeyV := surfaceKey(48, 40, 1, true, false)
	if surfaceKeyV == offscreenKeyV {
		t.Fatal("resolve mode must change the cache key")
	}

	refS, trioS, err := c.acquire(device, surfaceKeyV)
	if err != nil {
		t.Fatalf("surface acquire: %v", err)
	}
	if trioS.resolveTex != nil {
		t.Fatal("surface trio must have no resolve target")
	}
	c.release(refS)

	refO, trioO, err := c.acquire(device, offscreenKeyV)
	if err != nil {
		t.Fatalf("offscreen acquire: %v", err)
	}
	if trioO == trioS {
		t.Fatal("an offscreen request must not reuse the surface trio")
	}
	if trioO.resolveTex == nil || trioO.resolveView == nil {
		t.Fatal("offscreen trio must carry a resolve target for CPU readback")
	}
	if st := c.stats(); st.Hits != 0 || st.Misses != 2 {
		t.Fatalf("cross-mode reuse must miss: hits=%d misses=%d want 0/2", st.Hits, st.Misses)
	}
	c.release(refO)
}

// MSAA sample count is part of the key too: a 4x trio must not be served to a
// 1x owner (the depth/stencil sample counts would mismatch at render pass
// creation).
func TestSurfaceCache_SampleCountIsDistinctKey(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	ref1, trio1, err := c.acquire(device, surfaceKey(32, 16, 1, false, false))
	if err != nil {
		t.Fatalf("1x acquire: %v", err)
	}
	if trio1.msaaTex != nil {
		t.Fatal("1x trio must not allocate MSAA color")
	}
	c.release(ref1)

	ref4, trio4, err := c.acquire(device, surfaceKey(32, 16, 4, false, false))
	if err != nil {
		t.Fatalf("4x acquire: %v", err)
	}
	if trio4 == trio1 {
		t.Fatal("4x request must not reuse the 1x trio")
	}
	if trio4.msaaTex == nil || trio4.msaaView == nil {
		t.Fatal("4x trio must allocate MSAA color")
	}
	c.release(ref4)
}

// A degraded (1x1 depth) trio is a fallback, not a cached class: releasing it
// must destroy it so the next full-size success is never served a 1x1 depth.
func TestSurfaceCache_DegradedReleaseRetires(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	key := surfaceKey(64, 64, 1, false, true)

	ref, trio, err := c.acquire(device, key)
	if err != nil {
		t.Fatalf("degraded acquire: %v", err)
	}
	if !trio.degraded {
		t.Fatal("trio must report degraded")
	}
	if trio.stencilTex == nil {
		t.Fatal("degraded trio still allocates the 1x1 depth")
	}
	c.releaseRetire(ref)

	if st := c.stats(); st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("retired trio must leave the pool: entries=%d bytes=%d", st.Entries, st.Bytes)
	}
	if ref2, trio2, err := c.acquire(device, key); err != nil {
		t.Fatalf("re-acquire after retire: %v", err)
	} else {
		defer c.release(ref2)
		if trio2 == trio {
			t.Fatal("a retired trio must never be handed out again")
		}
	}
}

// The per-key cap is the resize-storm outlet: a burst of same-key
// acquire/release cycles cannot grow the idle bucket without bound.
func TestSurfaceCache_MaxPerKeyCapsIdleBucket(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 2)
	key := surfaceKey(20, 20, 1, false, false)

	refs := make([]res.Ref, 0, 4)
	for i := 0; i < 4; i++ {
		ref, _, err := c.acquire(device, key)
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		refs = append(refs, ref)
	}
	for _, ref := range refs {
		c.release(ref)
	}
	st := c.stats()
	if st.Idle > 2 || st.Entries > 2 {
		t.Fatalf("idle=%d entries=%d want <=2 (per-key cap)", st.Idle, st.Entries)
	}
	if st.Evictions != 2 {
		t.Fatalf("evictions=%d want 2 (cap overflow retired)", st.Evictions)
	}
}

// The byte budget is the second outlet: with a budget that fits exactly one
// trio, releasing the older one evicts it instead of pooling it.
func TestSurfaceCache_ByteBudgetEvictsLRU(t *testing.T) {
	keyA := surfaceKey(32, 32, 1, false, false)
	one := surfaceTrioBytes(keyA)
	device, c, _ := newSurfaceCacheForTest(t, one, 0)

	ref1, _, err := c.acquire(device, keyA)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	ref2, _, err := c.acquire(device, keyA)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if st := c.stats(); st.Bytes != 2*one {
		t.Fatalf("bytes=%d want %d (over budget by design, nothing idle yet)", st.Bytes, 2*one)
	}
	c.release(ref1)
	st := c.stats()
	if st.Evictions == 0 {
		t.Fatal("releasing an over-budget idle entry must evict")
	}
	if st.Bytes > one {
		t.Fatalf("bytes=%d must fit the %d budget after eviction", st.Bytes, one)
	}
	c.release(ref2)
}

// The OOM outlet: purge frees idle trios and leaves anything still held (or
// still in flight) alone.
func TestSurfaceCache_PurgeIdleKeepsCheckedOut(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	kIdle := surfaceKey(24, 12, 1, false, false)
	kHeld := surfaceKey(80, 60, 1, true, false)

	refIdle, _, err := c.acquire(device, kIdle)
	if err != nil {
		t.Fatalf("idle acquire: %v", err)
	}
	refHeld, _, err := c.acquire(device, kHeld)
	if err != nil {
		t.Fatalf("held acquire: %v", err)
	}
	c.release(refIdle)

	before := c.stats()
	if want := surfaceTrioBytes(kIdle) + surfaceTrioBytes(kHeld); before.Bytes != want {
		t.Fatalf("before purge: bytes=%d want %d (both trios live)", before.Bytes, want)
	}
	n, freed := c.purgeIdle()
	if n != 1 || freed != surfaceTrioBytes(kIdle) {
		t.Fatalf("purge=%d freed=%d want 1/%d (only the idle trio)", n, freed, surfaceTrioBytes(kIdle))
	}
	st := c.stats()
	if st.Entries != 1 || st.Bytes != surfaceTrioBytes(kHeld) {
		t.Fatalf("after purge: entries=%d bytes=%d want 1/%d", st.Entries, st.Bytes, surfaceTrioBytes(kHeld))
	}
	if st.Purged != 1 {
		t.Fatalf("purged=%d want 1", st.Purged)
	}
	c.release(refHeld)
	if n, _ := c.purgeIdle(); n != 1 {
		t.Fatalf("second purge must take the released trio, got %d", n)
	}
}

// Device teardown: releaseAll destroys pooled and checked-out trios exactly
// once; a later release of the abandoned ref must not double-destroy.
func TestSurfaceCache_ReleaseAllDestroysEveryTrio(t *testing.T) {
	device, c, owner := newSurfaceCacheForTest(t, 0, 0)
	k := surfaceKey(16, 16, 1, false, false)
	ref1, trio1, err := c.acquire(device, k)
	if err != nil {
		t.Fatalf("acquire 1: %v", err)
	}
	ref2, trio2, err := c.acquire(device, k)
	if err != nil {
		t.Fatalf("acquire 2: %v", err)
	}
	c.release(ref1) // pooled
	c.releaseAll()  // device teardown

	for i, trio := range []*surfaceTrio{trio1, trio2} {
		if trio.stencilTex != nil || trio.stencilView != nil {
			t.Fatalf("trio %d must be released by releaseAll", i)
		}
	}
	if owner.Count() != 0 {
		t.Fatalf("registry must be empty after releaseAll, count=%d", owner.Count())
	}
	c.release(ref2) // late release of a force-dropped ref: no-op, no panic
	if st := c.stats(); st.Entries != 0 {
		t.Fatalf("entries=%d want 0 after late release", st.Entries)
	}
}

// Device loss: bookkeeping drops without touching the natives — the abandon
// flow owns teardown, and a second release outlet would double-destroy.
func TestSurfaceCache_InvalidateAllLeavesNativesToAbandon(t *testing.T) {
	device, c, owner := newSurfaceCacheForTest(t, 0, 0)
	k := surfaceKey(12, 9, 1, true, false)
	ref, trio, err := c.acquire(device, k)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	c.release(ref)
	c.invalidateAll()

	if st := c.stats(); st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("invalidate must drop bookkeeping: entries=%d bytes=%d", st.Entries, st.Bytes)
	}
	if trio.stencilTex == nil {
		t.Fatal("invalidate must not release the native (abandon flow owns it)")
	}
	owner.InvalidateOwned()
	if owner.Count() != 0 {
		t.Fatalf("registry must be empty after invalidate, count=%d", owner.Count())
	}
	if trio.stencilTex == nil {
		t.Fatal("invalidate owned must still not release natives")
	}
}

// The probe出口 (D7): every declared budget domain surfaces its own counters
// under its own name, so a real-window JSON pool section can be pinned to it.
func TestSurfaceCache_StatsProbePerDomain(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	k := surfaceKey(8, 8, 1, false, false)
	ref, _, err := c.acquire(device, k)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	c.release(ref)
	ref2, _, err := c.acquire(device, k)
	if err != nil {
		t.Fatalf("reuse acquire: %v", err)
	}
	c.release(ref2)

	st := c.stats()
	if st.Name != "test-surface" {
		t.Fatalf("name=%q want test-surface", st.Name)
	}
	if st.Hits != 1 || st.Misses != 1 {
		t.Fatalf("hits=%d misses=%d want 1/1", st.Hits, st.Misses)
	}
	if st.Entries != 1 || st.Idle != 1 {
		t.Fatalf("entries=%d idle=%d want 1/1", st.Entries, st.Idle)
	}
	want := surfaceTrioBytes(k)
	if st.Bytes != want || st.PeakBytes != want {
		t.Fatalf("bytes=%d peak=%d want %d", st.Bytes, st.PeakBytes, want)
	}
	if st.Budget != 0 {
		t.Fatalf("budget=%d want 0 (limits disabled in this fixture)", st.Budget)
	}
}

// The production limits come from the single budget declaration table: the
// trio cache takes BudgetSurfacePool bytes and BudgetStencilScratch per-key
// entries, and the estimate must match what the cache actually charges.
func TestSurfaceCache_ProductionLimitsMatchBudget(t *testing.T) {
	reg := res.NewRegistry()
	c := newSurfaceCache(reg.DeviceOwner(), "device-surface")
	if c.cache == nil {
		t.Fatal("production cache must bind to the registry")
	}
	st := c.stats()
	want := uint64(render.DefaultBudget(render.BudgetSurfacePool).MaxBytes)
	if st.Budget != want {
		t.Fatalf("budget=%d want %d (BudgetSurfacePool)", st.Budget, want)
	}

	// 1x surface trio = depth only; 4x = MSAA color + 4x depth; offscreen adds
	// a single-sample resolve target.
	if got := surfaceTrioBytes(surfaceKey(10, 20, 1, false, false)); got != 10*20*4 {
		t.Fatalf("1x surface bytes=%d want %d", got, 10*20*4)
	}
	if got := surfaceTrioBytes(surfaceKey(10, 20, 4, false, false)); got != 10*20*4*4+10*20*4*4 {
		t.Fatalf("4x surface bytes=%d want %d", got, 10*20*4*4+10*20*4*4)
	}
	if got := surfaceTrioBytes(surfaceKey(10, 20, 1, true, false)); got != 10*20*4+10*20*4 {
		t.Fatalf("1x offscreen bytes=%d want %d", got, 10*20*4+10*20*4)
	}
	if got := surfaceTrioBytes(surfaceKey(10, 20, 1, false, true)); got != 4 {
		t.Fatalf("degraded bytes=%d want 4 (one 1x1 depth texel)", got)
	}
}

// The owner-side routing, pinned end to end (a cache-only test would not catch
// a textureSet that pools what it must destroy): a full-size trio returns to
// the pool, a degraded trio and an owner that submits outside res.Submission
// (retireOnRelease: shared stencil renderer / textured cover caches) destroy.
func TestTextureSet_ReleaseRoutingPoolsOrRetires(t *testing.T) {
	device, c, _ := newSurfaceCacheForTest(t, 0, 0)
	full := surfaceKey(40, 24, 1, false, false)
	degraded := surfaceKey(40, 24, 1, false, true)

	// 1) session-shaped owner: full-size trio pools.
	ts := &textureSet{surfaceCache: c}
	if err := ts.acquireSurface(device, full, "pool"); err != nil {
		t.Fatalf("acquire full: %v", err)
	}
	ts.destroyTextures()
	if st := c.stats(); st.Entries != 1 || st.Idle != 1 {
		t.Fatalf("full-size release must pool: entries=%d idle=%d want 1/1", st.Entries, st.Idle)
	}

	// 2) degraded trio must destroy, never join the pool.
	dts := &textureSet{surfaceCache: c}
	if err := dts.acquireSurface(device, degraded, "degraded"); err != nil {
		t.Fatalf("acquire degraded: %v", err)
	}
	dts.destroyTextures()
	if st := c.stats(); st.Entries != 1 {
		t.Fatalf("degraded release must retire: entries=%d want 1 (only the pooled full trio)", st.Entries)
	}

	// 3) retireOnRelease owners (submitted outside res.Submission): the take
	// hits the trio pooled in step 1, the release destroys it — the pool ends
	// empty, matching the pre-pooling destroy-on-resize contract.
	before := c.stats()
	rts := &textureSet{surfaceCache: c, retireOnRelease: true}
	if err := rts.acquireSurface(device, full, "retire"); err != nil {
		t.Fatalf("acquire for retire owner: %v", err)
	}
	if cur := c.stats(); cur.Hits == before.Hits {
		t.Fatal("step 3 must be served from the pool (hit counter must advance)")
	}
	if cur := c.stats(); cur.Idle != 0 {
		t.Fatalf("a checked-out trio must not stay idle: idle=%d want 0", cur.Idle)
	}
	rts.destroyTextures()
	if st := c.stats(); st.Entries != 0 {
		t.Fatalf("retireOnRelease must destroy the trio: entries=%d want 0", st.Entries)
	}
}
