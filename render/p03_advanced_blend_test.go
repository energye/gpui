package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP03_MultiplyDualTexGPU verifies G.06: solid Multiply uses dual-tex GPU
// (extra GPUOps, dark result over yellow base).
func TestP03_MultiplyDualTexGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 0)
	dc.DrawRectangle(0, 0, 64, 64)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	base := dc.RenderPathStats().GPUOps

	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(8, 8, 48, 48)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("%s base_gpu=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("expected dual-tex GPUOps>base")
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("cpu_fb: %s", stats.LogLine())
	}
	r, g, b, _ := sampleRGBA(dc, 32, 32)
	t.Logf("multiply rgba=%d,%d,%d", r, g, b)
	if int(r)+int(g)+int(b) > 80 {
		t.Fatalf("expected dark multiply got %d,%d,%d", r, g, b)
	}
}

// TestP03_AdvancedLayerDualTexGPU verifies PushLayer(Multiply) content draws on
// GPU RT and Pop dual-tex composites onto parent (P0-3).
func TestP03_AdvancedLayerDualTexGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(48, 48)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	_ = dc.FlushGPU()

	dc.PushLayer(render.BlendMultiply, 1.0)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()
	dc.PopLayer()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("layer multiply %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("expected GPUOps>0 for advanced layer path")
	}
	r, g, b, _ := sampleRGBA(dc, 24, 24)
	t.Logf("result rgba=%d,%d,%d", r, g, b)
	// Multiply red over white ≈ red
	if r < 100 {
		t.Fatalf("expected red-ish after multiply layer, got %d,%d,%d", r, g, b)
	}
	if g > 40 || b > 40 {
		t.Fatalf("unexpected green/blue: %d,%d,%d", r, g, b)
	}
}

// TestP03_LargeMultiplyTiled keeps dual-tex on a larger surface (G.07 tiling).
func TestP03_LargeMultiplyTiled(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 0)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	base := dc.RenderPathStats().GPUOps

	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("large %s base=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("large multiply should add GPUOps")
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("cpu_fb on large: %s", stats.LogLine())
	}
	r, g, b, _ := sampleRGBA(dc, w/2, h/2)
	if int(r)+int(g)+int(b) > 80 {
		t.Fatalf("center not dark: %d,%d,%d", r, g, b)
	}
}
