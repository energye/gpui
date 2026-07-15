//go:build !nogpu

package render_test

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
)

func TestS63_PresentMainPath_NoRegress(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", 16.7)
	const w, h = 400, 240
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
		// Several same-style rects (SDF batch path).
		for k := 0; k < 8; k++ {
			dc.SetRGB(0.2, 0.5, 0.85)
			dc.DrawRectangle(12+float64(k*18), 40, 14, 22)
			_ = dc.Fill()
		}
		out, err := dc.PresentFrameAuto(view, uint32(w), uint32(h), nil)
		dt := time.Since(t0).Seconds() * 1000
		if err != nil {
			t.Fatalf("present: %v", err)
		}
		if out.Idle {
			t.Fatal("unexpected idle")
		}
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatal("GPUOps==0")
		}
		if dc.RenderPathStats().CPUFallbackOps != 0 {
			t.Fatalf("cpu_fallback=%d", dc.RenderPathStats().CPUFallbackOps)
		}
		if i >= 3 {
			samples = append(samples, dt)
		}
	}
	p50 := s5Percentile(samples, 0.5)
	t.Logf("S6.3 multi-rect present p50=%.2fms budget=%.2f", p50, budget)
	if p50 > budget && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("p50 %.2f exceeds budget %.2f", p50, budget)
	}
}
