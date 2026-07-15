//go:build !nogpu

package render_test

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters" // register blur/shadow
)

func TestS64_LayerPool_ReusesSurfaces(t *testing.T) {
	dc := render.NewContext(128, 96)
	defer dc.Close()
	dc.ResetLayerPoolStats()

	for i := 0; i < 6; i++ {
		dc.PushLayer(render.BlendNormal, 0.9)
		dc.SetRGB(0.2, 0.4, 0.8)
		dc.DrawRectangle(10, 10, 40, 30)
		_ = dc.Fill()
		dc.PopLayer()
	}
	gets, puts, hits, misses := dc.LayerPoolStats()
	t.Logf("layer pool gets=%d puts=%d hits=%d misses=%d", gets, puts, hits, misses)
	if gets < 6 || puts < 6 {
		t.Fatalf("expected get/put per push/pop, got gets=%d puts=%d", gets, puts)
	}
	if hits < 4 {
		t.Fatalf("expected reuse hits after first alloc, hits=%d misses=%d", hits, misses)
	}
	if misses < 1 {
		t.Fatalf("expected at least one cold miss, misses=%d", misses)
	}
}

func TestS64_FilterPool_ReusesIntermediates(t *testing.T) {
	if !render.FiltersRegistered() {
		t.Skip("filters not registered")
	}
	render.ResetFilterPoolStats()
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, 64, 64)
	_ = dc.Fill()

	for i := 0; i < 5; i++ {
		dc.ApplyBlur(1.5)
	}
	gets, puts, hits, misses := render.FilterPoolStats()
	t.Logf("filter pool gets=%d puts=%d hits=%d misses=%d", gets, puts, hits, misses)
	if gets < 5 || puts < 5 {
		t.Fatalf("gets=%d puts=%d", gets, puts)
	}
	if hits < 3 {
		t.Fatalf("expected intermediate reuse hits=%d misses=%d", hits, misses)
	}
}

func TestS64_CoalesceColorMatrixNodes(t *testing.T) {
	// Identity ∘ scale should merge to one node.
	a := render.ImageFilterNode{Kind: render.ImageFilterColorMatrix, Matrix: identityColorMatrix()}
	b := render.ImageFilterNode{Kind: render.ImageFilterColorMatrix, Matrix: scaleColorMatrix(0.5)}
	// Use exported behavior via ApplyImageFilterGraph path indirectly through package tests in render.
	// Coalesce is unexported; verify via graph apply correctness instead of node count.
	dc := render.NewContext(8, 8)
	defer dc.Close()
	dc.ClearWithColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
	if !render.FiltersRegistered() {
		t.Skip("filters not registered")
	}
	// Two matrices: grayscale-like then invert may both run; here two scales.
	dc.ApplyImageFilterGraph(a, b)
	// Pixel should be darkened ~0.5 red channel (premul).
	px := dc.Image().At(4, 4)
	r, _, _, _ := px.RGBA()
	// 0-65535 scale; ~0.5 of white-ish red
	if r > 40000 {
		t.Fatalf("expected darkened red after 0.5 matrix, got r=%d", r)
	}
}

func identityColorMatrix() [20]float32 {
	return [20]float32{
		1, 0, 0, 0, 0,
		0, 1, 0, 0, 0,
		0, 0, 1, 0, 0,
		0, 0, 0, 1, 0,
	}
}

func scaleColorMatrix(s float32) [20]float32 {
	return [20]float32{
		s, 0, 0, 0, 0,
		0, s, 0, 0, 0,
		0, 0, s, 0, 0,
		0, 0, 0, 1, 0,
	}
}

func TestS64_LayerPresent_NoRegress(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", 16.7)
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	p1White(dc, w, h)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	var samples []float64
	for i := 0; i < 10; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		t0 := time.Now()
		// Nested layer shell (U05-lite / B09-like).
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		dc.PushLayer(render.BlendNormal, 0.92)
		dc.SetRGB(0.2, 0.45, 0.85)
		dc.DrawRoundedRectangle(40, 40, 200, 100, 12)
		_ = dc.Fill()
		dc.PopLayer()
		if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
			t.Fatalf("present: %v", err)
		}
		dt := time.Since(t0).Seconds() * 1000
		if dc.RenderPathStats().GPUOps == 0 {
			// Layer may be CPU composite; still require no silent fallback flag path if GPU was used for fills
			t.Logf("note: GPUOps==0 on layer frame (CPU composite path possible)")
		}
		if i >= 3 {
			samples = append(samples, dt)
		}
	}
	p50 := s5Percentile(samples, 0.5)
	t.Logf("S6.4 layer present p50=%.2fms budget=%.2f", p50, budget)
	if p50 > budget*4 && os.Getenv("S6_ALLOW_SLOW") != "1" {
		// Layer+CPU composite can be heavier than damage-only UI; soft ceiling 4× main budget.
		t.Fatalf("p50 %.2f exceeds soft layer budget %.2f", p50, budget*4)
	}
}

func TestS64_BackdropSnapshot_UsesPool(t *testing.T) {
	dc := render.NewContext(80, 60)
	defer dc.Close()
	dc.SetRGB(0.1, 0.2, 0.3)
	dc.DrawRectangle(0, 0, 80, 60)
	_ = dc.Fill()
	dc.ResetLayerPoolStats()
	dc.PushBackdropLayer(render.BlendNormal, 0.95)
	dc.SetRGBA(1, 1, 1, 0.3)
	dc.DrawRectangle(10, 10, 40, 30)
	_ = dc.Fill()
	dc.PopLayer()
	gets, puts, _, _ := dc.LayerPoolStats()
	if gets < 1 || puts < 1 {
		t.Fatalf("backdrop should use layer pool gets=%d puts=%d", gets, puts)
	}
}
