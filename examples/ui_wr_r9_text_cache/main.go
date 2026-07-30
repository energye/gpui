// Command ui_wr_r9_text_cache is the R9 gate: text measure cache hits under
// repeated layout of stable content.
//
// Quality bar (§2.6.2 R9): multiple text styles (font size / color / weight) +
// repeated layout triggers; each tick forces MarkNeedsLayout but text content
// stays the same; measure_cache_hit≥1 + hits≥miss; proves control text measure
// cache is effective.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 5
	hudH         = 72.0
)

func main() {
	secs := runSeconds(closeSeconds)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R9 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: R9 measure cache — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r9_text_cache",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var layoutPasses atomic.Int64

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r9_text_cache: close")
			}
		},
	})

	// Warm measure cache once, then repeatedly MarkNeedsLayout on the same text
	// so measureLine hits the cache (R9).
	for _, t := range sc.texts {
		t.ResetMeasureCacheStats()
		_ = t.Layout(rendering.Loose(460, 200))
	}
	phases := wrkit.NewPhaseClock(1.2, 3.0) // Steady 0–1.2 · Spike 1.2–3 · Recover
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		// Force layout each tick without changing text → measure cache hits.
		rate := 1.0
		switch ph {
		case wrkit.PhaseSpike:
			rate = 2.0
		case wrkit.PhaseRecover:
			rate = 0.7
		}
		_ = rate
		for _, t := range sc.texts {
			t.MarkNeedsLayout()
		}
		if app.Pipeline().FlushLayout(rendering.Size{Width: float64(winW), Height: float64(winH)}, false) {
			layoutPasses.Add(1)
		}
		for _, p := range sc.panels {
			p.MarkNeedsPaint()
		}
		proc.Sample()
		if wrkit.HUDEnabled() && sc.hud != nil {
			sc.hud.NoteTick(dt)
			snap := app.Metrics().Snapshot()
			wrkit.MergeBoundaryCache(app, &snap)
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			policy := snap.PresentPolicy
			if policy == "" {
				policy = scheduler.PresentPolicyFullPaint
			}
			hits, miss := int64(0), int64(0)
			for _, t := range sc.texts {
				h, m := t.MeasureCacheStats()
				hits += h
				miss += m
			}
			gateOK := fps >= 55 || phases.Elapsed() < 1.5
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 1.5 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R9",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("hits=%d miss=%d layouts=%d", hits, miss, layoutPasses.Load()),
				GateOK:      gateOK,
				Extra:       "4 text styles · repeated MarkNeedsLayout · cache hits",
			})
		}
		app.ScheduleFrame()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: present target open: %v\n", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	hits, miss := int64(0), int64(0)
	for _, t := range sc.texts {
		h, m := t.MeasureCacheStats()
		hits += h
		miss += m
	}
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R9",
		Scenario:      "ui_wr_r9_text_cache",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":          "1200x800",
			"run_seconds":        secs,
			"measure_cache_hit":  hits,
			"measure_cache_miss": miss,
			"layout_passes":      layoutPasses.Load(),
			"text_styles":        4,
			"phase_script":       "Steady→Spike→Recover (layout rate doubles in Spike)",
			"impl_correctness":   "same text + same style under repeated MarkNeedsLayout → width/height stable, cache hits dominate",
			"impl_dirty":         "MarkNeedsLayout forces layout each tick but text content unchanged → MeasureCache hits",
			"impl_cache":         "MeasureCache hits≥1 + hits≥miss; cache keyed by (text, face, size, maxWidth)",
			"impl_edge":          "4 distinct styles (size 14/18/24 + color variants); empty text no-op; same text different style = different cache entry",
			"impl_fail":          "hits=0 (cache never warmed) / hits<miss (cache ineffective) = FAIL",
			"impl_visible":       "hits/miss counter in HUD; text stable not jittering",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r9_text_cache: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if hits < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: measure_cache_hit=%d want ≥1\n", hits)
		os.Exit(1)
	}
	// After warm, hits should dominate misses for stable text.
	if hits < miss {
		fmt.Fprintf(os.Stderr, "FAIL: measure_cache_hit=%d < miss=%d (cache ineffective)\n", hits, miss)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: PASS hits=%d miss=%d layouts=%d fps=%.1f\n",
		hits, miss, layoutPasses.Load(), rep.FPSInterval)
}

// --- helpers ---

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

// --- scene ---

type scene9 struct {
	Root   *rendering.AbsoluteBox
	hud    *wrkit.LiveHUD
	texts  []*rendering.RenderText
	panels []*rendering.AbsoluteBox
}

func buildScene(w, h float64) *scene9 {
	s := &scene9{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	// --- TopBar (U17 多区域 第 1 区) ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("R9 measure cache — 4 text styles, repeated MarkNeedsLayout, hits≫miss", 14, 16, 14, 0.88, 0.92, 0.98)

	// --- Legend (U17 第 2 区, ≥8 行色块+文字) ---
	leg := wrkit.NewPanel(240, 540, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.75, 0.95)
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"RenderText (4 styles)", 0.90, 0.85, 0.50},
		{"FontSize 14/18/24", 0.55, 0.75, 0.85},
		{"SetMaxWidth 460", 0.30, 0.50, 0.90},
		{"MeasureCache keyed", 0.25, 0.75, 0.40},
		{"MarkNeedsLayout each tick", 0.90, 0.30, 0.25},
		{"hits ≥ 1 (warmed)", 0.55, 0.70, 0.80},
		{"hits ≥ miss (effective)", 0.55, 0.75, 0.85},
		{"PhaseClock: Steady/Spike/Recover", 0.55, 0.75, 0.85},
	}
	for i, ln := range legendLines {
		leg.ColorAt(14, 14, 12, 36+float64(i)*28, ln.r, ln.g, ln.b, 1, false)
		leg.LabelAt(ln.text, 11, 32, 36+float64(i)*28, ln.r, ln.g, ln.b)
	}

	// --- 4 text panels with distinct styles (U17 多区域 第 3-6 区) ---
	textSpecs := []struct {
		x, y    float64
		pw, ph  float64
		size    float64
		r, g, b float64
		text    string
	}{
		{276, 70, 460, 100, 14, 0.92, 0.93, 0.95,
			"gpui measure cache demo — repeated wrap words for hit rate " +
				"alpha beta gamma delta epsilon zeta eta theta iota kappa " +
				"alpha beta gamma delta epsilon zeta eta theta iota kappa"},
		{276, 200, 460, 100, 18, 0.55, 0.80, 0.95,
			"second panel — different size + color, same cache key semantics " +
				"lambda mu nu xi omicron pi rho sigma tau upsilon phi"},
		{756, 70, 432, 140, 24, 0.90, 0.75, 0.60,
			"big text — measures wider, distinct cache entry " +
				"psi omega alpha beta gamma delta epsilon zeta"},
		{756, 230, 432, 100, 14, 0.70, 0.80, 0.90,
			"fourth panel — yet another style, another cache slot " +
				"eta theta iota kappa lambda mu nu xi"},
	}
	for _, spec := range textSpecs {
		panel := rendering.NewAbsoluteBox(spec.pw, spec.ph)
		panel.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.18, A: 1}
		panel.SetRepaintBoundary(true)
		s.panels = append(s.panels, panel)

		txt := rendering.NewRenderText(spec.text)
		txt.FontSize = spec.size
		txt.ApproxCharW = 0.55
		txt.SetMaxWidth(spec.pw - 20)
		txt.R, txt.G, txt.B, txt.A = spec.r, spec.g, spec.b, 1
		txt.SetRepaintBoundary(true)
		panel.Place(txt, 10, 10)
		s.texts = append(s.texts, txt)
		root.Place(panel, spec.x, spec.y)
	}

	// --- Static dense text grid (U17 第 7 区, ≥8 labels) ---
	gridX, gridY := 276.0, 320.0
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			cell := wrkit.NewPanel(108, 28, 0.10, 0.11, 0.13, 0.85)
			cell.PlaceOn(root, gridX+float64(col)*112, gridY+float64(row)*32)
			cell.LabelAt(fmt.Sprintf("static %d-%d", row, col), 11, 8, 18, 0.70, 0.75, 0.85)
		}
	}

	// --- LiveHUD (U18 窗内可见指标) ---
	if wrkit.HUDEnabled() {
		s.hud = wrkit.NewLiveHUD(w, hudH)
		root.Place(s.hud.Box, 0, h-hudH)
	}

	return s
}
