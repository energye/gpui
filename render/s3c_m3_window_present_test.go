//go:build !nogpu

package render_test

import (
	"errors"
	"os"
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// --- S.03 PresentFrame orchestration ---

func TestS3c_M3_PresentFrame_Offscreen(t *testing.T) {
	s3cRequireGPU(t)
	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()

	view, release := dc.CreateOffscreenTexture(32, 32)
	if release == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer release()

	presented := false
	if err := dc.PresentFrame(view, 32, 32, func() error {
		presented = true
		return nil
	}); err != nil {
		t.Fatalf("PresentFrame: %v", err)
	}
	if !presented {
		t.Fatal("present callback not invoked")
	}
	stats := dc.RenderPathStats()
	t.Logf("path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("PresentFrame needs GPUOps>0: %s", stats.LogLine())
	}
}

func TestS3c_M3_PresentFrame_Guards(t *testing.T) {
	dc := render.NewContext(8, 8)
	defer dc.Close()

	if err := dc.PresentFrame(gpucontext.TextureView{}, 8, 8, nil); !errors.Is(err, render.ErrNilSurfaceView) {
		t.Fatalf("nil view err=%v want ErrNilSurfaceView", err)
	}

	view, release := dc.CreateOffscreenTexture(8, 8)
	if release == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer release()
	if err := dc.PresentFrame(view, 0, 0, nil); err == nil {
		t.Fatal("expected error for zero extent")
	}
}

func TestS3c_M3_WindowSwapchain_PresentFrame(t *testing.T) {
	// Full window path when CreateSurface(0,0) somehow works; otherwise skip.
	// Real X11 e2e lives in gpu/webgpu.TestSwapchain_WindowPresentE2E.
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	s3cRequireGPU(t)

	inst, err := webgpu.CreateInstance(&webgpu.InstanceDescriptor{Backends: webgpu.BackendsPrimary})
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	defer inst.Release()

	surf, err := inst.CreateSurface(0, 0)
	if err != nil || surf == nil {
		t.Skipf("no platform window surface (headless): %v", err)
	}
	defer surf.Release()

	adapter, err := inst.RequestAdapter(&webgpu.RequestAdapterOptions{
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Skipf("RequestAdapter: %v", err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("RequestDevice: %v", err)
	}
	defer device.Release()

	sc := webgpu.NewSwapchain(surf, device, 64, 64)
	sc.Usage = types.TextureUsageRenderAttachment
	if err := sc.ConfigureFromCapabilities(adapter); err != nil {
		t.Skipf("Configure: %v", err)
	}
	defer sc.Release()

	frame, err := sc.BeginFrame()
	if err != nil {
		t.Skipf("BeginFrame: %v", err)
	}

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.SetRGB(0, 0.5, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	_ = dc.Fill()

	if err := dc.PresentFrame(frame.Handle, frame.Width, frame.Height, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		t.Fatalf("PresentFrame window: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("window present path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("window present needs GPUOps>0: %s", stats.LogLine())
	}
}
