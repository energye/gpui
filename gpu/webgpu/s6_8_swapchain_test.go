//go:build !(js && wasm) && !nogpu

package webgpu_test

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

func TestS68_Swapchain_PreferPresentModes(t *testing.T) {
	sc := webgpu.NewSwapchain(nil, nil, 100, 80)
	sc.SetPreferVSync()
	if len(sc.PreferPresentModes) < 1 || sc.PreferPresentModes[0] != webgpu.PresentModeFifo {
		t.Fatalf("vsync prefer: %v", sc.PreferPresentModes)
	}
	sc.SetPreferLowLatency()
	if sc.PreferPresentModes[0] != webgpu.PresentModeMailbox {
		t.Fatalf("low latency prefer: %v", sc.PreferPresentModes)
	}
	// 块3: Wayland steady non-blocking preference (FifoRelaxed first, never Fifo).
	sc.SetPreferFifoRelaxed()
	if len(sc.PreferPresentModes) < 1 || sc.PreferPresentModes[0] != webgpu.PresentModeFifoRelaxed {
		t.Fatalf("fifo-relaxed prefer: %v", sc.PreferPresentModes)
	}
	for _, m := range sc.PreferPresentModes {
		if m == webgpu.PresentModeFifo {
			t.Fatalf("fifo-relaxed prefer must not include blocking Fifo: %v", sc.PreferPresentModes)
		}
	}
	if sc.PresentMode != webgpu.PresentModeFifo {
		t.Fatalf("default mode %v", sc.PresentMode)
	}
	if sc.PresentModeName() != "fifo" {
		t.Fatalf("name %s", sc.PresentModeName())
	}
}

func TestS68_Swapchain_StatsDefaults(t *testing.T) {
	sc := webgpu.NewSwapchain(nil, nil, 64, 64)
	st := sc.Stats()
	if st.Acquires != 0 || st.Presents != 0 {
		t.Fatalf("%+v", st)
	}
	sc.MarkNeedsReconfigure()
	sc.ResetStats()
	st = sc.Stats()
	if st.Reconfigures != 0 {
		t.Fatalf("reset failed: %+v", st)
	}
}

func TestS68_Swapchain_NewDefaultsStillValid(t *testing.T) {
	sc := webgpu.NewSwapchain(nil, nil, 100, 50)
	if sc.Usage != types.TextureUsageRenderAttachment {
		t.Fatal(sc.Usage)
	}
	if sc.PresentMode != webgpu.PresentModeFifo {
		t.Fatal(sc.PresentMode)
	}
}

// TestS68_Swapchain_PresentModeForVsync verifies the runtime vsync-switch
// mode picker: on → Fifo always; off → Mailbox when supported, else
// FifoRelaxed/Immediate, else Fifo. Regression for the interactive-resize
// content-lag fix (content must present without waiting a Fifo vblank).
func TestS68_Swapchain_PresentModeForVsync(t *testing.T) {
	sc := webgpu.NewSwapchain(nil, nil, 100, 50)

	// On → Fifo regardless of what the surface supports.
	sc.SetSupportedPresentModesForTest([]types.PresentMode{webgpu.PresentModeImmediate, webgpu.PresentModeMailbox})
	if m := sc.PresentModeForVsync(true); m != webgpu.PresentModeFifo {
		t.Fatalf("on → %v, want fifo", m)
	}

	// Off → Mailbox (latest-frame-wins, no tearing) when available.
	if m := sc.PresentModeForVsync(false); m != webgpu.PresentModeMailbox {
		t.Fatalf("off w/ mailbox → %v, want mailbox", m)
	}

	// No Mailbox → FifoRelaxed, then Immediate.
	sc.SetSupportedPresentModesForTest([]types.PresentMode{webgpu.PresentModeFifo, webgpu.PresentModeFifoRelaxed})
	if m := sc.PresentModeForVsync(false); m != webgpu.PresentModeFifoRelaxed {
		t.Fatalf("off w/o mailbox → %v, want fiforelaxed", m)
	}
	sc.SetSupportedPresentModesForTest([]types.PresentMode{webgpu.PresentModeFifo, webgpu.PresentModeImmediate})
	if m := sc.PresentModeForVsync(false); m != webgpu.PresentModeImmediate {
		t.Fatalf("off w/ immediate → %v, want immediate", m)
	}

	// Only Fifo → stays Fifo.
	sc.SetSupportedPresentModesForTest([]types.PresentMode{webgpu.PresentModeFifo})
	if m := sc.PresentModeForVsync(false); m != webgpu.PresentModeFifo {
		t.Fatalf("off w/ fifo only → %v, want fifo", m)
	}
}
