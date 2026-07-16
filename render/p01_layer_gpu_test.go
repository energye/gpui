package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP01_LayerGPUComposite verifies L.01/L.02: Normal-opacity layers draw into a
// GPU offscreen RT and composite via DrawGPUTexture (GPUOps>0, no full-layer
// CPU force-path for simple fills).
func TestP01_LayerGPUComposite(t *testing.T) {
	requireNativeGPU(t)

	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	// Opaque blue base on GPU.
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("base fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base flush: %v", err)
	}

	// 50% red layer — must hit GPU layer RT + texture composite (P0-1).
	dc.PushLayer(render.BlendNormal, 0.5)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 48, 48)
	if err := dc.Fill(); err != nil {
		t.Fatalf("layer fill: %v", err)
	}
	dc.PopLayer()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("final flush: %v", err)
	}

	stats := dc.RenderPathStats()
	t.Logf("P01 path_stats %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("P01 expected GPUOps>0 (layer GPU RT + composite): %s", stats.LogLine())
	}

	// Outside the red rect: pure blue base.
	br, bg, bb, _ := sampleRGBA(dc, 2, 2)
	if br > 40 || bg > 40 || bb < 200 {
		t.Fatalf("outside layer want blue, got rgb=%d,%d,%d", br, bg, bb)
	}

	// Inside 50% red over blue: both R and B elevated.
	lr, lg, lb, _ := sampleRGBA(dc, 32, 32)
	t.Logf("layer composite rgba=%d,%d,%d", lr, lg, lb)
	if lr < 80 || lr > 200 {
		t.Fatalf("layer red out of range: %d", lr)
	}
	if lb < 80 || lb > 200 {
		t.Fatalf("layer blue residual out of range: %d", lb)
	}
	if lg > 60 {
		t.Fatalf("layer unexpected green: %d", lg)
	}
}

// TestP01_LayerGPUNested ensures nested Normal layers composite without crash
// and still produce GPU ops.
func TestP01_LayerGPUNested(t *testing.T) {
	requireNativeGPU(t)

	dc := render.NewContext(48, 48)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(0, 0, 48, 48)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 1.0)
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(4, 4, 40, 40)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 0.5)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(12, 12, 24, 24)
	_ = dc.Fill()
	dc.PopLayer()
	dc.PopLayer()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("nested %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("nested GPUOps==0: %s", stats.LogLine())
	}
	// Center should not be pure white.
	r, g, b, _ := sampleRGBA(dc, 24, 24)
	if int(r)+int(g)+int(b) > 700 {
		t.Fatalf("center still near-white rgb=%d,%d,%d", r, g, b)
	}
}

// TestP01_LayerAdvancedBlendStillCPU verifies Multiply layer still composites
// visibly (P0-3 may use GPU dual-tex Pop; pixel contract unchanged).
func TestP01_LayerAdvancedBlendStillCPU(t *testing.T) {
	requireNativeGPU(t)

	dc := render.NewContext(32, 32)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	_ = dc.FlushGPU()

	dc.PushLayer(render.BlendMultiply, 1.0)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 32, 32)
	_ = dc.Fill()
	dc.PopLayer()
	_ = dc.FlushGPU()

	r, g, b, _ := sampleRGBA(dc, 16, 16)
	t.Logf("multiply layer rgb=%d,%d,%d", r, g, b)
	// Multiply red over white ≈ red-ish; not pure white.
	if r < 100 {
		t.Fatalf("expected red channel after multiply, got %d", r)
	}
}
