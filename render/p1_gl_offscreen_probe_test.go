//go:build linux && !nogpu

package render_test

import (
	"os"
	"runtime"
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"

	// Register the pure-Go GL backend (P1 temporary path).
	_ "github.com/energye/gpui/gpu/gwgpu/gles"
	// Register the render GPU accelerator (provider injection target).
	_ "github.com/energye/gpui/render/gpu"
)

// p1ProbeProvider carries a hal.Device through the boxed encoding.
type p1ProbeProvider struct {
	dev hal.Device
	ad  hal.Adapter
}

func (p *p1ProbeProvider) Device() gpucontext.Device {
	if p == nil || p.dev == nil {
		return gpucontext.Device{}
	}
	return gpucontext.PackDevice(p.dev)
}

func (p *p1ProbeProvider) Queue() gpucontext.Queue {
	if p == nil || p.dev == nil {
		return gpucontext.Queue{}
	}
	return gpucontext.PackQueue(p.dev.Queue())
}

func (p *p1ProbeProvider) SurfaceFormat() gputypes.TextureFormat {
	return gputypes.TextureFormatBGRA8Unorm
}

func (p *p1ProbeProvider) Adapter() gpucontext.Adapter {
	if p == nil || p.ad == nil {
		return gpucontext.Adapter{}
	}
	return gpucontext.PackAdapter(p.ad)
}

func (p *p1ProbeProvider) AdapterInfo() gpucontext.AdapterInfo {
	if p == nil || p.ad == nil {
		return gpucontext.AdapterInfo{Type: gpucontext.AdapterTypeUnknown}
	}
	info := p.ad.Info()
	ai := gpucontext.AdapterInfo{Name: info.Name}
	switch info.DeviceType {
	case gputypes.DeviceTypeDiscreteGPU:
		ai.Type = gpucontext.AdapterTypeDiscrete
	case gputypes.DeviceTypeIntegratedGPU:
		ai.Type = gpucontext.AdapterTypeIntegrated
	case gputypes.DeviceTypeCPU:
		ai.Type = gpucontext.AdapterTypeSoftware
	default:
		ai.Type = gpucontext.AdapterTypeUnknown
	}
	return ai
}

// p1GLDevice opens a boxed GL device using a minimal X11 window as the
// context provider (surfaceless RequestDevice is rejected by the backend).
// Pixels are validated offscreen; the window is never presented to.
// EGL has thread affinity: pin the whole test to one OS thread so context
// creation, shader compile, and queue submit never hop threads mid-test
// (unpinned rapid device churn intermittently negotiates GLSL 330 and fails
// SDF compile with an empty info log).
func p1GLDevice(t *testing.T) (hal.Device, hal.Adapter) {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY")
	}
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	be, ok := hal.GetBackend(gputypes.BackendGL)
	if !ok || be == nil {
		t.Skip("BackendGL not registered")
	}
	inst, err := be.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		t.Skipf("GL CreateInstance unavailable: %v", err)
	}
	t.Cleanup(inst.Release)

	// GL context provider: minimal X11 window (not presented to).
	xw := memOpenX11(t, 64, 64)
	t.Cleanup(xw.close)
	surf, err := inst.CreateSurface(hal.SurfaceTarget{
		Kind:          hal.SurfaceTargetXlibWindow,
		DisplayHandle: xw.display,
		WindowHandle:  xw.window,
	})
	if err != nil {
		t.Skipf("GL CreateSurface(Xlib) unavailable: %v", err)
	}
	t.Cleanup(surf.Destroy)

	ad, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		t.Skipf("GL RequestAdapter unavailable: %v", err)
	}
	t.Cleanup(ad.Release)

	dev, err := ad.RequestDevice(render.DeviceDescriptor("p1-probe"))
	if err != nil {
		t.Skipf("GL RequestDevice unavailable: %v", err)
	}
	t.Cleanup(dev.Release)
	return dev, ad
}

// TestP1GLOffscreenSolidProbe is the P1-1b first probe: solid fill through
// the boxed GL device must take the GPU path with zero CPU fallback and
// produce the exact pixel. No X11/EGL/GPU => Skip (no fake green).
func TestP1GLOffscreenSolidProbe(t *testing.T) {
	dev, ad := p1GLDevice(t)

	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}

	stats := dc.RenderPathStats()
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d reason=%q (GL path fell back to CPU)",
			stats.CPUFallbackOps, dc.LastCPUFallbackReason())
	}
	if stats.GPUOps == 0 {
		t.Fatalf("gpu_ops=0 (nothing took the GPU path)")
	}

	img := dc.Image()
	r, g, b, a := img.At(32, 32).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Fatalf("center pixel = (%d,%d,%d,%d), want (255,0,0,255)",
			r>>8, g>>8, b>>8, a>>8)
	}
}

// TestP1GLOffscreenSolidCorner locks offscreen readback orientation: a 16x16
// rect at (0,0) must land top-left, not bottom-left. Center-only probes stay
// green under a Y-flip; this corner probe caught the CopyTextureToBuffer
// extra-flip (P1-1b-2) where (8,8) was transparent and (8,55) was red.
func TestP1GLOffscreenSolidCorner(t *testing.T) {
	dev, ad := p1GLDevice(t)

	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.Transparent)
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, 16, 16)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	out := dc.Image()
	r, g, b, a := out.At(8, 8).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Fatalf("top-left (8,8) = (%d,%d,%d,%d), want (255,0,0,255)",
			r>>8, g>>8, b>>8, a>>8)
	}
	r, g, b, a = out.At(8, 55).RGBA()
	if r>>8 != 0 || g>>8 != 0 || b>>8 != 0 || a>>8 != 0 {
		t.Fatalf("bottom (8,55) = (%d,%d,%d,%d), want (0,0,0,0)",
			r>>8, g>>8, b>>8, a>>8)
	}
}

// TestP1GLOffscreenSolidDoubleFlush checks whether an explicit FlushGPU
// followed by Image()'s internal flush wipes the frame on GL. The textured
// probe does DrawImage + Fill + FlushGPU + Image (two flushes) while the
// solid probe does Fill + Image (one flush) — this isolates the flush count.
func TestP1GLOffscreenSolidDoubleFlush(t *testing.T) {
	dev, ad := p1GLDevice(t)

	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	out := dc.Image()
	r, g, b, a := out.At(32, 32).RGBA()
	t.Logf("double-flush solid=(%d,%d,%d,%d)", r>>8, g>>8, b>>8, a>>8)
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Fatalf("double-flush solid = (%d,%d,%d,%d), want (255,0,0,255)",
			r>>8, g>>8, b>>8, a>>8)
	}
}

// TestP1GLOffscreenTexturedQuad is the P1-1b texture-path isolate: a solid
// red ImageBuf drawn via DrawImage must take the GPU path and read back red.
// If this fails while the solid-fill probe passes, the textured-quad pipeline
// (QueueImageDraw: ramp/image uploads) is broken on GL, not the gradient math.
func TestP1GLOffscreenTexturedQuad(t *testing.T) {
	dev, ad := p1GLDevice(t)

	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	img, err := render.NewImageBuf(64, 64, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	img.Fill(255, 0, 0, 255)

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ClearWithColor(render.Transparent)
	dc.DrawImage(img, 0, 0)
	// Co-draw: solid red rect at top-left (0,0,16,16), overlapping the image.
	// If it shows while the rest of the image stays transparent, the image
	// draw is a silent no-op inside an otherwise healthy pass.
	dc.SetRGBA(1, 0, 0, 1)
	dc.DrawRectangle(0, 0, 16, 16)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	stats := dc.RenderPathStats()
	t.Logf("GL image %s", stats.LogLine())
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("GL image must not CPU-fallback: %s reason=%q",
			stats.LogLine(), dc.LastCPUFallbackReason())
	}

	out := dc.Image()
	r, g, b, a := out.At(32, 32).RGBA()
	rb, gb, bb, ab := out.At(2, 61).RGBA()
	sr, sg, sb, sa := out.At(8, 8).RGBA()
	t.Logf("image center=(%d,%d,%d,%d) bottom=(%d,%d,%d,%d) solid=(%d,%d,%d,%d)",
		r>>8, g>>8, b>>8, a>>8, rb>>8, gb>>8, bb>>8, ab>>8, sr>>8, sg>>8, sb>>8, sa>>8)
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Fatalf("image pixel = (%d,%d,%d,%d), want (255,0,0,255)",
			r>>8, g>>8, b>>8, a>>8)
	}
}

// is a multi-stop linear gradient, so the gradient pipeline (WGSL->GLSL +
// ramp upload) must take the GPU path with zero fallback and ramp the right
// way. Thresholds mirror TestP02_LinearGradientNativeGPU (red falls L->R).
func TestP1GLOffscreenLinearGradient(t *testing.T) {
	dev, ad := p1GLDevice(t)

	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	dc := render.NewContext(128, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.Black)

	grad := render.NewLinearGradientBrush(0, 0, 128, 0).
		AddColorStop(0, render.RGB(1, 0, 0)).
		AddColorStop(0.5, render.RGB(0, 1, 0)).
		AddColorStop(1, render.RGB(0, 0, 1))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, 128, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	stats := dc.RenderPathStats()
	t.Logf("GL linear %s", stats.LogLine())
	t.Logf("GL linear bootstrap=%d reason=%q", stats.BrushBootstrapOps, stats.LastBrushBootstrapReason)
	if stats.GPUOps == 0 {
		t.Fatalf("GL linear GPUOps==0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("GL linear must not CPU-fallback: %s reason=%q",
			stats.LogLine(), dc.LastCPUFallbackReason())
	}

	img := dc.Image()
	at := func(x, y int) (int, int, int) {
		rr, gg, bb, _ := img.At(x, y).RGBA()
		return int(rr >> 8), int(gg >> 8), int(bb >> 8)
	}
	for _, x := range []int{0, 16, 32, 48, 64, 80, 96, 112, 127} {
		rr, gg, bb := at(x, 32)
		t.Logf("x=%d rgb=%d,%d,%d", x, rr, gg, bb)
	}
	rL, gL, bL := at(4, 32)
	rM, gM, _ := at(64, 32)
	rR, _, bR := at(124, 32)
	t.Logf("L=%d,%d,%d M=%d,%d,%d R=%d,%d,%d", rL, gL, bL, rM, gM, 0, rR, 0, bR)
	if rL < 180 || gL > 80 {
		t.Fatalf("left expected red-ish, got %d,%d,%d", rL, gL, bL)
	}
	if gM < 120 {
		t.Fatalf("mid expected green contribution, got %d,%d,%d", rM, gM, 0)
	}
	if bR < 180 || rR > 80 {
		t.Fatalf("right expected blue-ish, got %d,%d,%d", rR, 0, bR)
	}
	if rL <= rR {
		t.Fatalf("expected red decrease L->R: L=%d R=%d", rL, rR)
	}
}
