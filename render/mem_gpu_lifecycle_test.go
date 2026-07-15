//go:build !nogpu

package render_test

// T0–T2 memory lifecycle gates. Plan: docs/MEM_LEAK_TEST_PLAN.md

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
)

func memDrawPresentSimple(t *testing.T, w, h int) {
	t.Helper()
	dc := render.NewContext(w, h)
	defer func() {
		if err := dc.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer rel()

	dc.ResetRenderPathStats()
	dc.SetRGB(0.2, 0.4, 0.7)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	dc.SetRGB(1, 0.8, 0.2)
	dc.DrawRoundedRectangle(8, 8, float64(w)/2, float64(h)/3, 6)
	_ = dc.Fill()
	if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("PresentFrame %dx%d: %v", w, h, err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatalf("GPUOps==0 at %dx%d", w, h)
	}
	if dc.RenderPathStats().CPUFallbackOps != 0 {
		t.Fatalf("cpu_fallback_ops=%d", dc.RenderPathStats().CPUFallbackOps)
	}
	memHardRSSCheck(t)
}

// TestMem_T0_CreateClose — each iter: NewContext → Present → Close, multi-size.
func TestMem_T0_CreateClose(t *testing.T) {
	memRequireGPU(t)
	n := memEnvIters(40)
	sizes := [][2]int{{64, 64}, {128, 96}, {256, 160}, {320, 200}, {480, 270}}
	rss := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		sz := sizes[i%len(sizes)]
		memDrawPresentSimple(t, sz[0], sz[1])
		if i >= n/10 {
			rss = append(rss, memRSSKB())
		}
	}
	delta := memEnvInt64("GPUI_MEM_RSS_DELTA_KB", 48*1024)
	memAssertSteadyRSS(t, rss, delta, "T0")
	t.Logf("T0 CreateClose ok iters=%d", n)
}

// TestMem_T1_RetainedMultiSize — long-lived Context, Resize + new offscreen each iter.
func TestMem_T1_RetainedMultiSize(t *testing.T) {
	memRequireGPU(t)
	n := memEnvIters(30)
	dc := render.NewContext(512, 512)
	defer dc.Close()
	sizes := [][2]int{{128, 128}, {256, 144}, {400, 300}, {512, 288}, {64, 64}}
	rss := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		w, h := sizes[i%len(sizes)][0], sizes[i%len(sizes)][1]
		if err := dc.Resize(w, h); err != nil {
			t.Fatalf("Resize: %v", err)
		}
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		dc.SetRGB(0.3, 0.6, 0.95)
		rad := float64(w)
		if h < w {
			rad = float64(h)
		}
		dc.DrawCircle(float64(w)/2, float64(h)/2, rad/4)
		_ = dc.Fill()
		memPresentOffscreen(t, dc, w, h)
		if i >= n/10 {
			rss = append(rss, memRSSKB())
		}
	}
	delta := memEnvInt64("GPUI_MEM_RSS_DELTA_KB", 48*1024)
	memAssertSteadyRSS(t, rss, delta, "T1")
	t.Logf("T1 RetainedMultiSize ok iters=%d", n)
}

// TestMem_T2_ResetAccelerator — pressure then global reset reclaim.
func TestMem_T2_ResetAccelerator(t *testing.T) {
	// Prefer 1x samples so suite reclaim is testable on tight VRAM.
	_ = os.Setenv("GPUI_SURFACE_SAMPLE_COUNT", "1")
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	// Start from a clean accelerator so the require-probe present (if any) does
	// not leave a second device-generation on the stack for this tight-VRAM host.
	if err := rendgpu.ResetAccelerator(); err != nil {
		t.Fatalf("ResetAccelerator start: %v", err)
	}
	// Light pressure then hard reset; prove post-reset present still works.
	// Keep sizes modest: go test packages are large and leave less headroom for
	// Vulkan heaps than standalone go run probes on integrated GPUs.
	for i := 0; i < 2; i++ {
		memDrawPresentSimple(t, 96+i*16, 72+i*12)
	}
	if err := rendgpu.ResetAccelerator(); err != nil {
		t.Fatalf("ResetAccelerator: %v", err)
	}
	// Post-reset: new device/session must allocate again without native OOM.
	memDrawPresentSimple(t, 128, 96)
	memDrawPresentSimple(t, 160, 120)
	if err := rendgpu.ResetAccelerator(); err != nil {
		t.Fatalf("ResetAccelerator 2: %v", err)
	}
	memDrawPresentSimple(t, 144, 108)
	t.Log("T2 ResetAccelerator reclaim ok")
}

// TestMem_T5_StressCreateClose — optional high iters.
func TestMem_T5_StressCreateClose(t *testing.T) {
	if os.Getenv("GPUI_MEM_STRESS") != "1" {
		t.Skip("set GPUI_MEM_STRESS=1 for high-iter stress")
	}
	memRequireGPU(t)
	n := memEnvIters(200)
	for i := 0; i < n; i++ {
		memDrawPresentSimple(t, 320, 200)
		if i%50 == 49 {
			t.Logf("T5 stress %d/%d rss=%dKB", i+1, n, memRSSKB())
		}
	}
}

// Keep old names as aliases so prior docs/commands still match.
func TestMem_GPUContextLifecycle_CreateClose(t *testing.T) {
	TestMem_T0_CreateClose(t)
}
func TestMem_GPUContextLifecycle_RetainedMultiSize(t *testing.T) {
	TestMem_T1_RetainedMultiSize(t)
}
func TestMem_GPUContextLifecycle_ResetAccelerator(t *testing.T) {
	TestMem_T2_ResetAccelerator(t)
}
