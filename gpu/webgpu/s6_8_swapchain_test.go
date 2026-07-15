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
