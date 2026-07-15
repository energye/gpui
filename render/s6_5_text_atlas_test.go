//go:build !nogpu

package render_test

import (
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"golang.org/x/image/font/gofont/goregular"
)

func TestS65_PresentListScroll_NoRegress(t *testing.T) {
	p1RequireGPU(t)
	budget := s5EnvFloat("S6_MAIN_PATH_BUDGET", 16.7)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	dc.SetFont(src.Face(13))

	text.ClearShapeResultCache()
	text.ResetShapeResultCacheStats()

	p1White(dc, w, h)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	rows := []string{
		"Message subject line 00 — preview",
		"Message subject line 01 — preview",
		"Message subject line 02 — preview",
		"Message subject line 03 — preview",
		"Message subject line 04 — preview",
		"Message subject line 05 — preview",
		"Message subject line 06 — preview",
		"Message subject line 07 — preview",
	}

	var samples []float64
	for i := 0; i < 12; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		t0 := time.Now()
		dc.SetRGB(0.97, 0.97, 0.98)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
		off := i % 4
		dc.SetRGB(0.1, 0.1, 0.12)
		for r := 0; r < 8; r++ {
			dc.DrawString(rows[(off+r)%len(rows)], 16, 28+float64(r)*22)
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
		if i >= 4 {
			samples = append(samples, dt)
		}
	}
	p50 := s5Percentile(samples, 0.5)
	st := text.ShapeResultCacheStats()
	t.Logf("S6.5 list-scroll present p50=%.2fms budget=%.2f shapeHits=%d misses=%d entries=%d",
		p50, budget, st.Hits, st.Misses, st.Entries)
	if st.Hits < 1 {
		t.Fatalf("expected shape/layout cache hits after scroll frames, %+v", st)
	}
	if p50 > budget && os.Getenv("S6_ALLOW_SLOW") != "1" {
		t.Fatalf("p50 %.2f exceeds budget %.2f", p50, budget)
	}
}

func TestS65_L0_HelpersStillGreen(t *testing.T) {
	// Keep S6 L0 smoke linked to text path.
	p1RequireGPU(t)
	const w, h = 128, 96
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	dc.SetFont(src.Face(14))
	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.DrawString("S65", 20, 40)
	if err := dc.PresentFrameFull(view, uint32(w), uint32(h), nil); err != nil {
		t.Fatal(err)
	}
	if dc.RenderPathStats().GPUOps == 0 {
		t.Fatal("GPUOps==0")
	}
}
