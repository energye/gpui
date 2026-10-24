//go:build !nogpu

package render_test

// Real-GPU NV12 colour regression: a known BT.601 limited-range red frame
// must come out red through DrawVideoPlanes. If the UV plane is sampled at
// the wrong width (the old "chroma reads 0 / green cast" finding), the same
// frame turns green, so this test distinguishes "pixels shipped" from
// "colour is right".

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

func TestVideoYUVPlanes_GPUColor(t *testing.T) {
	requireNativeGPU(t)
	_ = rendgpu.ResetAccelerator()

	inst, err := webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: types.BackendsPrimary})
	if err != nil {
		t.Skipf("CreateInstance: %v", err)
	}
	adapter, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference: types.PowerPreferenceHighPerformance,
	})
	if err != nil {
		inst.Release()
		t.Skipf("RequestAdapter: %v", err)
	}
	device, err := adapter.RequestDevice(render.DeviceDescriptor("video-yuv-color"))
	if err != nil {
		adapter.Release()
		inst.Release()
		t.Skipf("RequestDevice: %v", err)
	}
	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: device, Adpt: adapter, Format: types.TextureFormatBGRA8Unorm,
	}); err != nil {
		device.Release()
		adapter.Release()
		inst.Release()
		t.Fatalf("SetDeviceProvider: %v", err)
	}
	t.Cleanup(func() {
		_ = rendgpu.ResetAccelerator()
		_ = device.WaitIdle()
		device.Release()
		adapter.Release()
		inst.Release()
	})

	// 64 is row-padded (pitch 256 > width); 256 is tight. Both used to
	// sample the UV texture at half its real width and read zero at center.
	for _, tc := range []struct {
		name string
		w, h int
	}{
		{"padded", 64, 32},
		{"aligned", 256, 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runVideoYUVColorCase(t, device, tc.w, tc.h)
		})
	}
}

func runVideoYUVColorCase(t *testing.T, device hal.Device, w, h int) {
	t.Helper()
	pool := render.NewVideoPlanePool(device)
	defer pool.Close()
	slot, err := pool.Acquire(w, h)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// BT.601 limited-range red: Y=76, U=84, V=255. If UV is dropped and
	// sampled as zero, the CPU/shader formula turns this frame green.
	y := make([]byte, w*h)
	for i := range y {
		y[i] = 76
	}
	uv := make([]byte, w*h/2)
	for i := 0; i < len(uv); i += 2 {
		uv[i], uv[i+1] = 84, 255
	}
	if err := pool.UploadPlanes(slot, y, uv); err != nil {
		t.Fatalf("UploadPlanes: %v", err)
	}

	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{B: 1, A: 1})
	dc.ResetRenderPathStats()
	if !dc.DrawVideoPlanes(slot, render.VideoDrawOptions{
		X: 0, Y: 0, DstWidth: float64(w), DstHeight: float64(h), Opacity: 1,
	}) {
		t.Fatal("DrawVideoPlanes = false, want GPU path")
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 {
		t.Fatalf("GPUOps=0: %s", st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("CPU fallback ran instead of GPU path: %s", st.LogLine())
	}
	r, g, b, a := dc.Image().At(w/2, h/2).RGBA()
	got := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	// Shader is float and the surface is BGRA; allow the usual ±3 UI
	// tolerance. Green here means the UV plane was not sampled.
	if got[0] < 252 || got[1] > 3 || got[2] > 3 || got[3] < 252 {
		t.Fatalf("NV12 red pixel=(%d,%d,%d,%d), want ~(255,0,0,255): %s",
			got[0], got[1], got[2], got[3], st.LogLine())
	}
}
