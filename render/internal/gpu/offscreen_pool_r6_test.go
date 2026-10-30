//go:build !nogpu

package gpu

import "testing"

// TestOffscreenCache_ReuseAndAbandon locks the R6-3 quarantine by construction
// (needs a native device; skips headless like the other native tests): a
// released layer RT of the same size is handed out again, a different size
// allocates fresh, and a device switch releases pooled natives instead of
// handing them out on the new device.
func TestOffscreenCache_ReuseAndAbandon(t *testing.T) {
	device, _, cleanup := createNativeTestDevice(t)
	defer cleanup()
	shared := NewGPUShared()
	shared.device = device
	shared.deviceReady = true
	rc := shared.NewRenderContext()
	defer rc.Close()

	v1, rel1 := rc.CreateOffscreenTexture(64, 32)
	if v1.IsNil() || rel1 == nil {
		t.Fatal("first alloc must succeed")
	}
	rel1() // back to the cache (not natively destroyed)

	cache, _ := rc.offscreenCacheFor()
	if hits, _ := cache.HitRate(); hits != 0 {
		t.Fatalf("first take must miss, hits=%d", hits)
	}

	v2, rel2 := rc.CreateOffscreenTexture(64, 32)
	if v2.IsNil() || rel2 == nil {
		t.Fatal("pooled take must succeed")
	}
	if unpackView(v1) != unpackView(v2) {
		t.Fatal("same-size release must hand the native back (reuse)")
	}
	if hits, _ := cache.HitRate(); hits != 1 {
		t.Fatalf("same-size take must hit, hits=%d", hits)
	}
	rel2()

	// Size flip is a different key: never served from the 64x32 bucket.
	v3, rel3 := rc.CreateOffscreenTexture(32, 64)
	if v3.IsNil() || rel3 == nil {
		t.Fatal("size-flip alloc must succeed")
	}
	rel3()

	// Device switch: pooled entries must be released, never handed out again.
	rc.abandonOffscreenCache(false)
	if st := cache.Stats(); st.Entries != 0 {
		t.Fatalf("abandon must release pooled entries, entries=%d", st.Entries)
	}
	v4, rel4 := rc.CreateOffscreenTexture(64, 32)
	if v4.IsNil() || rel4 == nil {
		t.Fatal("take after abandon must allocate fresh")
	}
	defer rel4()
	if unpackView(v4) == unpackView(v2) {
		t.Fatal("a native released by abandon must not be handed out")
	}
}

// TestOffscreenCache_NoPoolAllocatesFresh locks GPUI_NO_OFFSCREEN_POOL=1:
// always-fresh allocation, release retires instead of pooling.
func TestOffscreenCache_NoPoolAllocatesFresh(t *testing.T) {
	t.Setenv("GPUI_NO_OFFSCREEN_POOL", "1")
	device, _, cleanup := createNativeTestDevice(t)
	defer cleanup()
	shared := NewGPUShared()
	shared.device = device
	shared.deviceReady = true
	rc := shared.NewRenderContext()
	defer rc.Close()

	v1, rel1 := rc.CreateOffscreenTexture(64, 32)
	if v1.IsNil() || rel1 == nil {
		t.Fatal("first alloc must succeed")
	}
	rel1()

	v2, rel2 := rc.CreateOffscreenTexture(64, 32)
	if v2.IsNil() || rel2 == nil {
		t.Fatal("second alloc must succeed")
	}
	rel2()
	if unpackView(v1) == unpackView(v2) {
		t.Fatal("no-pool mode must not reuse a released native")
	}
	cache, _ := rc.offscreenCacheFor()
	if st := cache.Stats(); st.Entries != 0 {
		t.Fatalf("no-pool release must retire, entries=%d", st.Entries)
	}
}
