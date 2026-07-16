package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP13_LayerPop_BatchesUntilFinalFlush verifies F.03 / P1-3:
// nested Normal layers composite via GPU texture draws without mid-frame
// FlushGPU per Pop; a single end-of-frame Flush materializes everything.
func TestP13_LayerPop_BatchesUntilFinalFlush(t *testing.T) {
	requireNativeGPU(t)

	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.BeginFrame()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Nested translucent layers — each Pop previously forced a base FlushGPU.
	dc.PushLayer(render.BlendNormal, 0.5)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 24, 24)
	_ = dc.Fill()
	dc.PushLayer(render.BlendNormal, 0.5)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(20, 20, 24, 24)
	_ = dc.Fill()
	dc.PopLayer() // must NOT require mid-frame flush for correctness at present
	dc.PopLayer()

	mid := dc.RenderPathStats().FrameFlushes
	t.Logf("after pops (before final flush) frame_flushes=%d %s", mid, dc.RenderPathStats().LogLine())
	// Layer RT finish may flush to the offscreen view (1–2), but must not
	// explode to one full base submit per Pop (old path ≈ 2+ base flushes).
	if mid > 4 {
		t.Fatalf("too many mid-frame flushes after layer pops: %d", mid)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("final flush: %v", err)
	}
	stats := dc.RenderPathStats()
	t.Logf("final %s", stats.LogLine())
	if stats.GPUOps == 0 {
		t.Fatalf("expected GPUOps>0: %s", stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("cpu_fb: %s", stats.LogLine())
	}
	// Final flush counted.
	if stats.FrameFlushes < mid {
		t.Fatalf("final flush not counted: mid=%d final=%d", mid, stats.FrameFlushes)
	}

	// Visual: overlap region not pure white
	r, g, b, _ := sampleRGBA(dc, 28, 28)
	t.Logf("overlap=%d,%d,%d", r, g, b)
	if int(r)+int(g)+int(b) > 750 {
		t.Fatalf("expected layer composite ink, got %d,%d,%d", r, g, b)
	}
}

// TestP13_PresentFrameAuto_DamageIdle verifies F.02: idle frames skip work;
// partial dirty uses damage path not forced Full when coverage is small.
func TestP13_PresentFrameAuto_DamageIdle(t *testing.T) {
	requireNativeGPU(t)

	const w, h = 200, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.SetDamageTracking(true)

	// Bootstrap full frame.
	dc.BeginFrame()
	dc.MarkFullRedraw()
	dc.SetRGB(0.9, 0.9, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	// Offscreen present target via FlushGPU (no real swapchain).
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	// Idle: no damage → plan idle.
	dc.BeginFrame()
	plan := dc.PlanPresent(w, h)
	if plan.Mode != render.PresentModeIdle {
		t.Fatalf("empty damage want idle, got %v", plan.Mode)
	}

	// Small dirty rect → damage (not full at 85% threshold).
	dc.BeginFrame()
	dc.Invalidate(image.Rect(10, 10, 40, 40))
	plan = dc.PlanPresent(w, h)
	t.Logf("small dirty mode=%v union=%v", plan.Mode, plan.Union)
	if plan.Mode == render.PresentModeFull {
		t.Fatalf("small dirty must not promote to full")
	}
	if plan.Mode == render.PresentModeIdle {
		t.Fatalf("small dirty must not be idle")
	}

	// Large dirty ≥85% → full.
	dc.BeginFrame()
	dc.Invalidate(image.Rect(0, 0, 190, 190))
	plan = dc.PlanPresent(w, h)
	t.Logf("large dirty mode=%v", plan.Mode)
	if plan.Mode != render.PresentModeFull {
		t.Fatalf("large dirty want full, got %v", plan.Mode)
	}
}
