package rwgpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// Ledger accounting is pure Go: charge, refund, budget gate, double-release
// safety. No GPU needed. Serial: the ledger is process-global.
func TestVramLedgerChargeRefund(t *testing.T) {
	t.Setenv("GPUI_VRAM_BUDGET_MB", "0") // disable gate, test accounting only
	vramLedger.Lock()
	vramLedger.bytes = make(map[uintptr]uint64)
	vramLedger.total = 0
	vramLedger.Unlock()
	t.Cleanup(func() {
		vramLedger.Lock()
		vramLedger.bytes = make(map[uintptr]uint64)
		vramLedger.total = 0
		vramLedger.Unlock()
	})

	vramAdd(1, 100)
	vramAdd(2, 200)
	if got := VramLiveBytes(); got != 300 {
		t.Fatalf("live bytes = %d, want 300", got)
	}
	if got := VramLiveCount(); got != 2 {
		t.Fatalf("live count = %d, want 2", got)
	}
	vramForget(1)
	if got := VramLiveBytes(); got != 200 {
		t.Fatalf("after forget live bytes = %d, want 200", got)
	}
	// Double release must not underflow.
	vramForget(1)
	vramForget(0)
	if got := VramLiveBytes(); got != 200 {
		t.Fatalf("after double forget live bytes = %d, want 200", got)
	}
	vramForget(2)
	if got := VramLiveBytes(); got != 0 {
		t.Fatalf("after full refund live bytes = %d, want 0", got)
	}
}

func TestVramLedgerBudgetGate(t *testing.T) {
	t.Setenv("GPUI_VRAM_BUDGET_MB", "1") // 1 MiB cap
	vramLedger.Lock()
	vramLedger.bytes = make(map[uintptr]uint64)
	vramLedger.total = 0
	vramLedger.Unlock()
	t.Cleanup(func() {
		vramLedger.Lock()
		vramLedger.bytes = make(map[uintptr]uint64)
		vramLedger.total = 0
		vramLedger.Unlock()
	})

	if err := vramCheck("TestOp", 512*1024); err != nil {
		t.Fatalf("under-budget check failed: %v", err)
	}
	if err := vramCheck("TestOp", 2*1024*1024); err == nil {
		t.Fatal("over-budget check passed, want OOM error")
	} else if got := err.Error(); !containsOOM(got) {
		t.Fatalf("OOM error %q must match IsGPUOutOfMemory phrasing", got)
	}
}

func containsOOM(s string) bool {
	for _, sub := range []string{"out of memory", "not enough memory", "Out of memory", "Not enough memory", "VRAM budget exceeded"} {
		if len(s) >= len(sub) {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func TestVramTextureBytes(t *testing.T) {
	size := types.Extent3D{Width: 1200, Height: 800, DepthOrArrayLayers: 1}
	got := vramTextureBytes(size, 1, 1, 1, types.TextureFormatBGRA8Unorm)
	want := uint64(1200 * 800 * 4)
	if got != want {
		t.Fatalf("1200x800 BGRA8 = %d, want %d", got, want)
	}
	if got := vramTextureBytes(size, 1, 1, 1, types.TextureFormatR8Unorm); got != uint64(1200*800) {
		t.Fatalf("1200x800 R8 = %d, want %d", got, 1200*800)
	}
	if got := vramTextureBytes(size, 1, 1, 1, types.TextureFormatDepth24PlusStencil8); got != want {
		t.Fatalf("1200x800 depth24 = %d, want %d", got, want)
	}
	if got := vramTextureBytes(types.Extent3D{}, 1, 1, 1, types.TextureFormatBGRA8Unorm); got != 0 {
		t.Fatalf("empty extent = %d, want 0", got)
	}
}
