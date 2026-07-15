//go:build !nogpu

package render_test

// T3: complex offscreen scene churn (graphics/effects/text + size/background).

import (
	"testing"

	"github.com/energye/gpui/render"
)

func TestMem_T3_ComplexOffscreen_SizeChurn(t *testing.T) {
	memRequireGPU(t)
	n := memEnvIters(36)
	rng := newMemRNG(memSeed())
	font := memFindFont(t)
	img := memMakeCheckerImage(t, 32)

	sizes := [][2]int{
		{200, 150}, {320, 200}, {400, 300}, {480, 270}, {256, 256},
		{512, 288}, {360, 400}, {128, 128},
	}
	rss := make([]int64, 0, n)

	// Phase A: create/close each iter (harder on session alloc)
	half := n / 2
	for i := 0; i < half; i++ {
		w, h := sizes[i%len(sizes)][0], sizes[i%len(sizes)][1]
		// jitter
		w += rng.intn(32) - 8
		h += rng.intn(24) - 6
		if w < 64 {
			w = 64
		}
		if h < 64 {
			h = 64
		}
		dc := render.NewContext(w, h)
		if err := dc.LoadFontFace(font, 13); err != nil {
			dc.Close()
			t.Fatalf("font: %v", err)
		}
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		lvl := memSceneMedium
		if i%3 == 0 {
			lvl = memSceneComplex
		} else if i%3 == 1 {
			lvl = memSceneSimple
		}
		memDrawScene(t, dc, w, h, i, lvl, rng, img)
		memPresentOffscreen(t, dc, w, h)
		if err := dc.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if i >= half/10 {
			rss = append(rss, memRSSKB())
		}
	}

	// Phase B: retained context with resize + complex frames
	dc := render.NewContext(400, 300)
	defer dc.Close()
	if err := dc.LoadFontFace(font, 13); err != nil {
		t.Fatalf("font: %v", err)
	}
	for i := half; i < n; i++ {
		w, h := sizes[i%len(sizes)][0], sizes[i%len(sizes)][1]
		if err := dc.Resize(w, h); err != nil {
			t.Fatalf("Resize: %v", err)
		}
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		memDrawScene(t, dc, w, h, i, memSceneComplex, rng, img)
		memPresentOffscreen(t, dc, w, h)
		rss = append(rss, memRSSKB())
	}

	delta := memEnvInt64("GPUI_MEM_RSS_DELTA_KB", 96*1024)
	memAssertSteadyRSS(t, rss, delta, "T3")
	t.Logf("T3 ComplexOffscreen ok iters=%d final_rss=%dKB", n, memRSSKB())
}

func TestMem_T3_ComplexOffscreen_EscalatingLevels(t *testing.T) {
	memRequireGPU(t)
	// fixed size, escalate simple→medium→complex repeatedly
	const w, h = 480, 320
	n := memEnvIters(24)
	rng := newMemRNG(memSeed() + 7)
	font := memFindFont(t)
	img := memMakeCheckerImage(t, 24)
	dc := render.NewContext(w, h)
	defer dc.Close()
	if err := dc.LoadFontFace(font, 13); err != nil {
		t.Fatalf("font: %v", err)
	}
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()

	rss := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		dc.BeginFrame()
		dc.ResetRenderPathStats()
		lvl := memSceneLevel(i % 3)
		memDrawScene(t, dc, w, h, i, lvl, rng, img)
		if err := dc.PresentFrame(view, uint32(w), uint32(h), nil); err != nil {
			t.Fatalf("present: %v", err)
		}
		if dc.RenderPathStats().GPUOps == 0 {
			t.Fatal("GPUOps==0")
		}
		if dc.RenderPathStats().CPUFallbackOps != 0 {
			t.Fatalf("cpu_fb=%d", dc.RenderPathStats().CPUFallbackOps)
		}
		memHardRSSCheck(t)
		if i >= n/10 {
			rss = append(rss, memRSSKB())
		}
	}
	delta := memEnvInt64("GPUI_MEM_RSS_DELTA_KB", 64*1024)
	memAssertSteadyRSS(t, rss, delta, "T3-escalating")
}
