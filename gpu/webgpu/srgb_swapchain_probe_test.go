//go:build linux && !(js && wasm) && !nogpu

package webgpu_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// P3.1b-0 探针：X11 surface 是否广告 sRGB swapchain 格式，以及用 sRGB 格式
// Configure / BeginFrame / clear / Present 是否真的能走通。
//
// 这是 opt-in 线性目标的呈现入口探针；像素/混合语义已由 offscreen 探针覆盖。
func TestP31bSrgbSwapchainFormatProbe(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	xw := tryOpenX11Window(t, 160, 120)
	defer xw.close()

	inst, err := webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: types.BackendsPrimary})
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	defer inst.Release()
	surf, err := inst.CreateSurfaceFromHandles(xw.display, xw.window)
	if err != nil {
		t.Fatalf("CreateSurface: %v", err)
	}
	defer surf.Release()
	adapter, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference:   types.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Fatalf("RequestAdapter: %v", err)
	}
	defer adapter.Release()
	wa, ok := adapter.(*webgpu.Adapter)
	if !ok {
		t.Fatalf("adapter type %T, want *webgpu.Adapter", adapter)
	}
	caps := wa.GetSurfaceCapabilities(surf)
	if caps == nil {
		t.Fatal("GetSurfaceCapabilities returned nil")
	}
	t.Logf("P3.1b-0 X11 surface formats: %v", caps.Formats)
	var srgb types.TextureFormat
	for _, f := range caps.Formats {
		if f == types.TextureFormatBGRA8UnormSrgb || f == types.TextureFormatRGBA8UnormSrgb {
			srgb = f
			break
		}
	}
	if srgb == 0 {
		t.Skipf("P3.1b-0: no sRGB swapchain format advertised; formats=%v", caps.Formats)
	}
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Fatalf("RequestDevice: %v", err)
	}
	defer device.Release()
	sc := webgpu.NewSwapchain(surf, device, 160, 120)
	sc.Usage = types.TextureUsageRenderAttachment
	sc.Format = srgb
	if err := sc.Configure(); err != nil {
		t.Fatalf("Configure(sRGB %v): %v", srgb, err)
	}
	defer sc.Release()
	if sc.Format != srgb {
		t.Fatalf("configured format = %v, want %v", sc.Format, srgb)
	}
	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame(sRGB): %v", err)
	}
	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_srgb_sc_clear"})
	if err != nil {
		t.Fatalf("CreateCommandEncoder: %v", err)
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "p31b_srgb_sc_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: frame.View, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0.5, G: 0.25, B: 0.1, A: 1},
		}},
	})
	if err != nil {
		t.Fatalf("BeginRenderPass(sRGB swapchain): %v", err)
	}
	if err := rp.End(); err != nil {
		t.Fatalf("EndRenderPass: %v", err)
	}
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}
	defer device.FreeCommandBuffer(cmd)
	if _, err := device.Queue().Submit(cmd); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := sc.EndFrame(frame); err != nil {
		t.Fatalf("EndFrame(sRGB swapchain): %v", err)
	}
	t.Logf("P3.1b-0 sRGB swapchain format %v configure/clear/present OK", srgb)
}
