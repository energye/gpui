//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !nogpu

package hal

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// Ledger accounting is pure Go: charge, refund, budget gate, double-release
// safety. No GPU needed. Serial: the ledger is process-global.
func TestVramLedgerChargeRefund(t *testing.T) {
	t.Setenv("GPUI_VRAM_BUDGET_MB", "0") // disable gate, test accounting only
	VramTestReset()
	t.Cleanup(VramTestReset)

	VramAdd(1, 100)
	VramAdd(2, 200)
	if got := VramLiveBytes(); got != 300 {
		t.Fatalf("live bytes = %d, want 300", got)
	}
	if got := VramLiveCount(); got != 2 {
		t.Fatalf("live count = %d, want 2", got)
	}
	VramForget(1)
	if got := VramLiveBytes(); got != 200 {
		t.Fatalf("after forget live bytes = %d, want 200", got)
	}
	// Double release must not underflow.
	VramForget(1)
	VramForget(0)
	if got := VramLiveBytes(); got != 200 {
		t.Fatalf("after double forget live bytes = %d, want 200", got)
	}
	VramForget(2)
	if got := VramLiveBytes(); got != 0 {
		t.Fatalf("after full refund live bytes = %d, want 0", got)
	}
}

func TestVramLedgerBudgetGate(t *testing.T) {
	t.Setenv("GPUI_VRAM_BUDGET_MB", "1") // 1 MiB cap
	VramTestReset()
	t.Cleanup(VramTestReset)

	if err := VramCheck("TestOp", 512*1024); err != nil {
		t.Fatalf("under-budget check failed: %v", err)
	}
	if err := VramCheck("TestOp", 2*1024*1024); err == nil {
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
	got := VramTextureBytes(size, 1, 1, 1, types.TextureFormatBGRA8Unorm)
	want := uint64(1200 * 800 * 4)
	if got != want {
		t.Fatalf("1200x800 BGRA8 = %d, want %d", got, want)
	}
	if got := VramTextureBytes(size, 1, 1, 1, types.TextureFormatR8Unorm); got != uint64(1200*800) {
		t.Fatalf("1200x800 R8 = %d, want %d", got, 1200*800)
	}
	if got := VramTextureBytes(size, 1, 1, 1, types.TextureFormatDepth24PlusStencil8); got != want {
		t.Fatalf("1200x800 depth24 = %d, want %d", got, want)
	}
	if got := VramTextureBytes(types.Extent3D{}, 1, 1, 1, types.TextureFormatBGRA8Unorm); got != 0 {
		t.Fatalf("empty extent = %d, want 0", got)
	}
}

// TestVramLedgerNonTextureKinds locks the ledger scope: pipelines,
// samplers, and swapchain surfaces must all charge and refund through the
// same account — no blind categories. Pure Go, no GPU needed.
func TestVramLedgerNonTextureKinds(t *testing.T) {
	t.Setenv("GPUI_VRAM_BUDGET_MB", "0") // disable gate, test accounting only
	VramTestReset()
	t.Cleanup(VramTestReset)

	if VramSamplerBytes == 0 || VramPipelineBytes == 0 {
		t.Fatal("fixed estimates must be non-zero or the category is untracked")
	}
	// 1200x800 BGRA8 surface ≈ one 3.66MB presented frame.
	if got, want := VramSurfaceBytes(1200, 800, types.TextureFormatBGRA8Unorm), uint64(1200*800*4); got != want {
		t.Fatalf("surface bytes = %d, want %d", got, want)
	}
	if got := VramSurfaceBytes(0, 800, types.TextureFormatBGRA8Unorm); got != 0 {
		t.Fatalf("zero-extent surface = %d, want 0", got)
	}

	VramAdd(11, VramSamplerBytes)
	VramAdd(12, VramPipelineBytes)
	VramAdd(13, VramPipelineBytes)
	VramAdd(14, VramSurfaceBytes(1200, 800, types.TextureFormatBGRA8Unorm))
	wantTotal := uint64(VramSamplerBytes + 2*VramPipelineBytes + 1200*800*4)
	if got := VramLiveBytes(); got != wantTotal {
		t.Fatalf("live bytes = %d, want %d", got, wantTotal)
	}
	if got := VramLiveCount(); got != 4 {
		t.Fatalf("live count = %d, want 4", got)
	}
	// Re-charge on the same handle (surface re-Configure) must replace,
	// never double-count.
	VramAdd(14, VramSurfaceBytes(640, 480, types.TextureFormatBGRA8Unorm))
	wantTotal = uint64(VramSamplerBytes+2*VramPipelineBytes) + uint64(640*480*4)
	if got := VramLiveBytes(); got != wantTotal {
		t.Fatalf("after re-configure live bytes = %d, want %d", got, wantTotal)
	}
	VramForget(11)
	VramForget(12)
	VramForget(13)
	VramForget(14)
	if got := VramLiveBytes(); got != 0 {
		t.Fatalf("after full refund live bytes = %d, want 0", got)
	}
}
