//go:build !nogpu

package render_test

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
)

func TestS67_PresentImageGrid_NoRegress(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", 16.7)
	const w, h = 320, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	img := compMakeImage(t, 32, 32, 40, 120, 200)
	p1White(dc, w, h)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	var samples []float64
	for i := 0; i < 10; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		t0 := time.Now()
		dc.SetRGB(0.95, 0.96, 0.97)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		// Same GenerationID tiles → image cache hits after first frame.
		for row := 0; row < 4; row++ {
			for col := 0; col < 6; col++ {
				dc.DrawImage(img, 12+float64(col)*50, 16+float64(row)*50)
			}
		}
		if _, err := dc.PresentFrameAuto(view, uint32(w), uint32(h), nil); err != nil {
			t.Fatalf("present: %v", err)
		}
		dt := time.Since(t0).Seconds() * 1000
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
	t.Logf("S6.7 image grid present p50=%.2fms budget=%.2f", p50, budget)
	// Image grid soft budget 3× main (upload + textured quads).
	if p50 > budget*3 && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("p50 %.2f exceeds %.2f", p50, budget*3)
	}
}

func TestS67_L0_HelpersStillGreen(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 96, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()
	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	img := compMakeImage(t, 16, 16, 255, 0, 0)
	dc.DrawImage(img, 20, 20)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatal(err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
}
