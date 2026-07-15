//go:build !nogpu

package render_test

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
)

func TestS66_PresentPathStrokeDash_NoRegress(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", 16.7)
	// B12-like cloud is heavier than U01; soft budget allows heavier path scenes.
	// Gate is relative: warm frames after caches should stay under 4× main budget.
	soft := budget * 4
	const w, h = 480, 360
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

	drawCloud := func() {
		dc.SetRGB(0.98, 0.98, 0.99)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		dc.SetLineWidth(1.5)
		for i := 0; i < 40; i++ {
			p := render.NewPath()
			x0 := 20 + float64(i%8)*55
			y0 := 30 + float64(i/8)*60
			p.MoveTo(x0, y0)
			p.LineTo(x0+40, y0+10)
			p.LineTo(x0+30, y0+40)
			p.LineTo(x0-5, y0+35)
			p.Close()
			dc.SetRGB(float64(i%5)/6, 0.25, float64(i%7)/8)
			dc.SetDash(4, 3)
			dc.AppendPath(p)
			_ = dc.Stroke()
		}
		dc.SetDash()
		// Mix convex fills (triangle cloud) for convex-cache path.
		for i := 0; i < 12; i++ {
			p := render.NewPath()
			x0 := 30 + float64(i%6)*70
			y0 := 280 + float64(i/6)*30
			p.MoveTo(x0, y0)
			p.LineTo(x0+24, y0)
			p.LineTo(x0+12, y0+20)
			p.Close()
			dc.SetRGB(0.2, 0.45, 0.85)
			dc.AppendPath(p)
			_ = dc.Fill()
		}
	}

	var samples []float64
	for i := 0; i < 10; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		t0 := time.Now()
		drawCloud()
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
	t.Logf("S6.6 path/stroke/dash present p50=%.2fms soft=%.2f main=%.2f", p50, soft, budget)
	if p50 > soft && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("p50 %.2f exceeds soft path budget %.2f", p50, soft)
	}
}

func TestS66_PresentRetainedStroke_NoRegress(t *testing.T) {
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

	// Same geometry every frame → stroke + tess caches should stay hot.
	draw := func() {
		dc.SetRGB(0.96, 0.97, 0.98)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		dc.SetLineWidth(2)
		dc.SetRGB(0.15, 0.35, 0.8)
		for i := 0; i < 16; i++ {
			p := render.NewPath()
			x0 := 16 + float64(i%4)*70
			y0 := 20 + float64(i/4)*40
			p.MoveTo(x0, y0)
			p.LineTo(x0+40, y0+4)
			p.LineTo(x0+30, y0+28)
			p.Close()
			dc.AppendPath(p)
			_ = dc.Stroke()
		}
	}

	var samples []float64
	for i := 0; i < 10; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		t0 := time.Now()
		draw()
		if _, err := dc.PresentFrameAuto(view, uint32(w), uint32(h), nil); err != nil {
			t.Fatalf("present: %v", err)
		}
		dt := time.Since(t0).Seconds() * 1000
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatal("GPUOps==0")
		}
		if i >= 3 {
			samples = append(samples, dt)
		}
	}
	p50 := s5Percentile(samples, 0.5)
	t.Logf("S6.6 retained stroke present p50=%.2fms budget=%.2f", p50, budget)
	// Retained medium stroke cloud — allow 3× main path for expand/stencil cost.
	if p50 > budget*3 && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("p50 %.2f exceeds retained stroke budget %.2f", p50, budget*3)
	}
}

func TestS66_L0_HelpersStillGreen(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 128, 96
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
	p := render.NewPath()
	p.MoveTo(20, 20)
	p.LineTo(100, 30)
	p.LineTo(60, 70)
	p.Close()
	dc.SetRGB(0.2, 0.4, 0.9)
	dc.SetLineWidth(2)
	dc.AppendPath(p)
	_ = dc.Stroke()
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatal(err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
}
