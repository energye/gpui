//go:build linux && !nogpu

package render_test

// V-3 real-window negative: a video slot allocated on device A is drawn into
// a live window session on device B. The direct path must refuse it, record
// video:device-mismatch, and the CPU fallback must still put pixels on the
// real X11 surface.

import (
	"os"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

func TestV3_VideoCrossDeviceRealWindow(t *testing.T) {
	if os.Getenv("GPUI_FORCE_NO_X11") == "1" {
		t.Skip("GPUI_FORCE_NO_X11=1")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	memRequireGPU(t)
	_ = rendgpu.ResetAccelerator()

	const winW, winH = 160, 120
	xw := memOpenX11(t, winW, winH)

	instB, err := webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: types.BackendsPrimary})
	if err != nil {
		xw.close()
		t.Skipf("CreateInstance(B): %v", err)
	}
	surf, err := instB.CreateSurfaceFromHandles(xw.display, xw.window)
	if err != nil {
		instB.Release()
		xw.close()
		t.Fatalf("CreateSurface: %v", err)
	}
	adapterB, err := instB.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference:   types.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		surf.Release()
		instB.Release()
		xw.close()
		t.Fatalf("RequestAdapter(B): %v", err)
	}
	deviceB, err := adapterB.RequestDevice(render.DeviceDescriptor("v3-window-b"))
	if err != nil {
		adapterB.Release()
		surf.Release()
		instB.Release()
		xw.close()
		t.Fatalf("RequestDevice(B): %v", err)
	}
	sc := webgpu.NewSwapchain(surf, deviceB, winW, winH)
	sc.Usage = types.TextureUsageRenderAttachment
	sc.SetPreferVSync()
	if err := sc.ConfigureFromCapabilities(adapterB); err != nil {
		deviceB.Release()
		adapterB.Release()
		surf.Release()
		instB.Release()
		xw.close()
		t.Fatalf("Configure: %v", err)
	}
	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: deviceB, Adpt: adapterB, Format: sc.Format,
	}); err != nil {
		sc.Release()
		deviceB.Release()
		adapterB.Release()
		surf.Release()
		instB.Release()
		xw.close()
		t.Fatalf("SetDeviceProvider: %v", err)
	}
	dc := render.NewContext(winW, winH)

	// Device A is deliberately a second native device; only the slot texture
	// is used from it, never queued into B's session.
	instA, err := webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: types.BackendsPrimary})
	if err != nil {
		_ = dc.Close()
		_ = rendgpu.ResetAccelerator()
		sc.Release()
		deviceB.Release()
		adapterB.Release()
		surf.Release()
		instB.Release()
		xw.close()
		t.Skipf("CreateInstance(A): %v", err)
	}
	adapterA, err := instA.RequestAdapter(nil)
	if err != nil {
		instA.Release()
		_ = dc.Close()
		_ = rendgpu.ResetAccelerator()
		sc.Release()
		deviceB.Release()
		adapterB.Release()
		surf.Release()
		instB.Release()
		xw.close()
		t.Fatalf("RequestAdapter(A): %v", err)
	}
	deviceA, err := adapterA.RequestDevice(render.DeviceDescriptor("v3-slot-a"))
	if err != nil {
		adapterA.Release()
		instA.Release()
		_ = dc.Close()
		_ = rendgpu.ResetAccelerator()
		sc.Release()
		deviceB.Release()
		adapterB.Release()
		surf.Release()
		instB.Release()
		xw.close()
		t.Fatalf("RequestDevice(A): %v", err)
	}
	poolA := render.NewVideoTexturePool(deviceA)

	t.Cleanup(func() { xw.close() })
	t.Cleanup(func() {
		sc.Release()
		_ = deviceB.WaitIdle()
		deviceB.Release()
		adapterB.Release()
		surf.Release()
		instB.Release()
	})
	t.Cleanup(func() {
		_ = dc.Close()
		_ = rendgpu.ResetAccelerator()
	})
	t.Cleanup(func() {
		poolA.Close()
		_ = deviceA.WaitIdle()
		deviceA.Release()
		adapterA.Release()
		instA.Release()
	})

	const vw, vh = 64, 32
	slot, _, err := poolA.AcquireForFrame(nil, vw, vh)
	if err != nil {
		t.Fatalf("AcquireForFrame(A): %v", err)
	}
	pix := make([]byte, vw*vh*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+3] = 255, 255
	}
	if err := poolA.Upload(slot, pix); err != nil {
		t.Fatalf("Upload(A): %v", err)
	}

	dc.BeginFrame()
	dc.ResetRenderPathStats()
	opts := render.VideoDrawOptions{X: 10, Y: 10, DstWidth: vw, DstHeight: vh, Opacity: 1}
	if dc.DrawVideoSlot(slot, opts) {
		t.Fatal("cross-device video slot was accepted by the live window session")
	}
	mismatchReason := dc.LastCPUFallbackReason()
	if mismatchReason != "video:device-mismatch" {
		t.Fatalf("fallback reason=%q, want video:device-mismatch", mismatchReason)
	}
	if st := dc.RenderPathStats(); st.CPUFallbackOps == 0 {
		t.Fatalf("cross-device rejection did not count a CPU fallback: %s", st.LogLine())
	}

	// The caller's fallback must still land: draw the same frame through the
	// generic image path on B, then read the live context back.
	img, err := render.NewImageBuf(vw, vh, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer img.Dispose()
	copy(img.Data(), pix)
	img.MarkPixelsDirty()
	if !dc.DrawVideoFrame(img, opts) {
		t.Fatal("DrawVideoFrame fallback returned false")
	}
	got := dc.Image()
	r, g, b, _ := got.At(20, 20).RGBA()
	if r>>8 < 200 || g>>8 > 60 || b>>8 > 60 {
		t.Fatalf("fallback pixel=(%d,%d,%d), want red", r>>8, g>>8, b>>8)
	}
	pathStats := dc.RenderPathStats()

	frame, err := sc.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame: %v", err)
	}
	if err := dc.PresentFrameFull(frame.Handle, frame.Width, frame.Height, func() error {
		return sc.EndFrame(frame)
	}); err != nil {
		sc.DiscardFrame(frame)
		t.Fatalf("PresentFrameFull: %v", err)
	}
	if st := sc.Stats(); st.Presents == 0 {
		t.Fatalf("real window presented nothing: %+v", st)
	} else {
		t.Logf("V-3 real window: device A slot rejected by device B session; reason=%q cpu_fb=%d pixel=(%d,%d,%d); presents=%d",
			mismatchReason, pathStats.CPUFallbackOps, r>>8, g>>8, b>>8, st.Presents)
	}
}
