package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/webgpu"
)

// TestOffscreenPool_ReuseAndStaleGuard locks the R6-3 re-enable (needs a
// native device; skips headless like the other native tests). One shared
// device for all steps: separate native devices per step trip driver
// reclaim lag ("Not enough memory left") and flake.
//
//   - a released texture of the same size is handed out again instead of
//     allocating fresh (reuse);
//   - pooled items from a previous device are destroyed, never handed out
//     on the new device (shared-recovery guard).
func TestOffscreenPool_ReuseAndStaleGuard(t *testing.T) {
	device, _, cleanup := createNativeTestDevice(t)
	defer cleanup()
	shared := &GPUShared{device: device, deviceReady: true}
	rc := &GPURenderContext{shared: shared}

	v1, rel1 := rc.CreateOffscreenTexture(64, 32)
	if v1.IsNil() || rel1 == nil {
		t.Fatal("first alloc must succeed")
	}
	rel1() // back to the pool (not natively destroyed)

	v2, rel2 := rc.CreateOffscreenTexture(64, 32)
	if v2.IsNil() || rel2 == nil {
		t.Fatal("pooled take must succeed")
	}
	rc.offscreenPoolMu.Lock()
	n := len(rc.offscreenPool[[2]int{64, 32}])
	rc.offscreenPoolMu.Unlock()
	if n != 0 {
		t.Fatalf("pool bucket=%d want 0 after reuse take", n)
	}
	rel2()

	// Plant a stale entry tagged with no device (simulates a pre-recovery
	// texture); the take must destroy it and allocate fresh.
	rc.offscreenPoolMu.Lock()
	rc.offscreenPool = map[[2]int][]offscreenPooled{
		{64, 32}: {{tex: &webgpu.Texture{}, view: &webgpu.TextureView{}, dev: nil}},
	}
	rc.offscreenPoolMu.Unlock()
	v3, rel3 := rc.CreateOffscreenTexture(64, 32)
	if v3.IsNil() || rel3 == nil {
		t.Fatal("take past a stale entry must allocate fresh")
	}
	defer rel3()
	rc.offscreenPoolMu.Lock()
	n = len(rc.offscreenPool[[2]int{64, 32}])
	rc.offscreenPoolMu.Unlock()
	if n != 0 {
		t.Fatalf("stale entry must be destroyed, bucket=%d", n)
	}
}
