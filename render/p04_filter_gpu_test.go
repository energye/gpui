package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestP04_ApplyBlurGPU verifies L.04: ApplyBlur uses GPU filter graph
// (GPUOps increases, soft edge spill, no CPU fallback counter).
func TestP04_ApplyBlurGPU(t *testing.T) {
	requireNativeGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	if !render.GPUFilterGraphRegistered() {
		// Force GPU init — blank-import gpu registers graph after shared ensure.
		dc0 := render.NewContext(8, 8)
		dc0.SetRGB(1, 0, 0)
		dc0.DrawRectangle(0, 0, 8, 8)
		_ = dc0.Fill()
		_ = dc0.FlushGPU()
		_ = dc0.Close()
	}
	if !render.GPUFilterGraphRegistered() {
		t.Fatal("GPU filter graph not registered")
	}

	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.1, 0.2, 0.95)
	dc.DrawRoundedRectangle(20, 20, 24, 24, 4)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush base: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.ApplyBlur(3)
	stats := dc.RenderPathStats()
	t.Logf("blur %s base=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("ApplyBlur must record GPUOps (GPU filter path): base=%d after=%d %s", base, stats.GPUOps, stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Fatalf("ApplyBlur cpu_fb: %s", stats.LogLine())
	}

	// Blur should soften edges: outside the hard rect, not pure white.
	er, eg, eb, _ := sampleRGBA(dc, 16, 32)
	t.Logf("near-edge=%d,%d,%d", er, eg, eb)
	if er > 254 && eg > 254 && eb > 254 {
		er, eg, eb, _ = sampleRGBA(dc, 17, 32)
		t.Logf("near-edge2=%d,%d,%d", er, eg, eb)
	}
	if er > 254 && eg > 254 && eb > 254 {
		t.Fatalf("expected blur spill near card edge")
	}
	// Center remains blue-ish
	cr, cg, cb, _ := sampleRGBA(dc, 32, 32)
	t.Logf("center=%d,%d,%d", cr, cg, cb)
	if cb < 100 {
		t.Fatalf("center lost blue after blur: %d,%d,%d", cr, cg, cb)
	}
}

// TestP04_ApplyDropShadowGPU verifies DropShadow single-op GPU route.
// Transparent canvas is required: shadow is extracted from alpha (Skia/CSS model).
func TestP04_ApplyDropShadowGPU(t *testing.T) {
	requireNativeGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	// ensure graph
	dc0 := render.NewContext(4, 4)
	_ = dc0.FlushGPU()
	_ = dc0.Close()
	if !render.GPUFilterGraphRegistered() {
		t.Fatal("GPU filter graph not registered")
	}

	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.Transparent)
	dc.SetRGB(0.9, 0.2, 0.2)
	dc.DrawRectangle(24, 20, 28, 28)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	base := dc.RenderPathStats().GPUOps

	dc.ApplyDropShadow(4, 4, 3, render.RGBA{R: 0, G: 0, B: 0, A: 0.55})
	stats := dc.RenderPathStats()
	t.Logf("shadow %s base=%d", stats.LogLine(), base)
	if stats.GPUOps <= base {
		t.Fatalf("ApplyDropShadow must GPUOps: %s", stats.LogLine())
	}
	// Content still red-ish
	cr, cg, cb, ca := sampleRGBA(dc, 38, 34)
	t.Logf("content=%d,%d,%d,%d", cr, cg, cb, ca)
	if cr < 150 || ca < 200 {
		t.Fatalf("content lost after drop shadow: %d,%d,%d,%d", cr, cg, cb, ca)
	}
	// Shadow SE of rect: original covers [24,52)x[20,48); offset +4,+4 silhouette ~[28,56)x[24,52)
	// Sample outside original, inside offset silhouette.
	found := false
	for y := 48; y < 56 && !found; y++ {
		for x := 52; x < 60; x++ {
			sr, sg, sb, sa := sampleRGBA(dc, x, y)
			if sa > 20 && int(sr)+int(sg)+int(sb) < 200 {
				t.Logf("shadow sample at %d,%d = %d,%d,%d,%d", x, y, sr, sg, sb, sa)
				found = true
				break
			}
		}
	}
	if !found {
		// fallback single sample for log
		sr, sg, sb, sa := sampleRGBA(dc, 54, 50)
		t.Fatalf("expected drop shadow ink outside original rect, sample 54,50=%d,%d,%d,%d", sr, sg, sb, sa)
	}
}

// TestP04_ApplyGrayscaleGPU verifies color matrix style ops on GPU.
func TestP04_ApplyGrayscaleGPU(t *testing.T) {
	requireNativeGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	dc0 := render.NewContext(4, 4)
	_ = dc0.FlushGPU()
	_ = dc0.Close()

	dc := render.NewContext(40, 40)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 1)
	dc.DrawRectangle(8, 8, 24, 24)
	_ = dc.Fill()
	_ = dc.FlushGPU()
	base := dc.RenderPathStats().GPUOps
	dc.ApplyGrayscale()
	if dc.RenderPathStats().GPUOps <= base {
		t.Fatalf("ApplyGrayscale must GPUOps")
	}
	r, g, b, _ := sampleRGBA(dc, 20, 20)
	t.Logf("gray=%d,%d,%d", r, g, b)
	// Not pure saturated blue
	if b > r+50 && b > g+50 {
		t.Fatalf("still saturated blue: %d,%d,%d", r, g, b)
	}
}
