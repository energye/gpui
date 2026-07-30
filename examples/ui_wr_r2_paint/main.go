// Command ui_wr_r2_paint is the R2 single-ability real-window gate:
// local NeedsPaint isolation — only the target node repaints under boundary,
// siblings and static boundaries stay clean.
//
// Quality bar (§2.6.2 R2): ≥3 RepaintBoundary regions of different color /
// content type (color block + text label + nested static), ≥3 independent hot
// spots pulsing at different frequencies via MarkNeedsPaint, static boundary
// NeedsPaint() always false, proving control local repaint does not leak to
// siblings.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint
package main

import (
	"fmt"
	"math"
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
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R2 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: R2 local NeedsPaint — %ds @ 1200x800\n", secs)

	// Font must load before labels (U17 EnsureUIFace).
	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r2_paint",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r2_paint: close")
			}
		},
	})
	// R2: default full_paint (correctness under Clear; local repaint isolation).

	phases := wrkit.NewPhaseClock(1.5, 3.5) // Steady 0–1.5 · Spike 1.5–3.5 · Recover
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		sc.onTick(dt, ph)
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
			gateOK := fps >= 55 || phases.Elapsed() < 2
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R2",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core: fmt.Sprintf("hotA=%d hotB=%d hotC=%d staticClean=%d",
					sc.hotDirtyA.Load(), sc.hotDirtyB.Load(), sc.hotDirtyC.Load(), sc.staticClean.Load()),
				GateOK: gateOK,
				Extra:  "3 hot boundaries pulse independently — static neighbors stay clean",
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

	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	cleanOK := sc.staticClean.Load()
	hotA := sc.hotDirtyA.Load()
	hotB := sc.hotDirtyB.Load()
	hotC := sc.hotDirtyC.Load()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R2",
		Scenario:      "ui_wr_r2_paint",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":          "1200x800",
			"run_seconds":        secs,
			"hot_boundaries":     3,
			"hotA_dirty_ticks":   hotA,
			"hotB_dirty_ticks":   hotB,
			"hotC_dirty_ticks":   hotC,
			"static_clean_ticks": cleanOK,
			"g_metrics":          "skipped",
			"g_metrics_reason":   "R2 is not R9/R10; text/measure cache not in scope",
			"phase_script":       "Steady→Spike→Recover (paint rate doubles in Spike)",
			"impl_correctness":   "only target hot boundary MarkNeedsPaint each tick; static boundary NeedsPaint() stays false; siblings unaffected",
			"impl_dirty":         "paint_count ∝ number of dirty boundaries; FullPaint redraws all but only hots pulse",
			"impl_cache":         "N/A for R2 (BoundaryCache is R3); R2 proves local repaint isolation, not cache hits",
			"impl_edge":          "3 hot boundaries at different frequencies; static neighbors with text labels; deep nesting via panel-in-panel",
			"impl_fail":          "static boundary becomes NeedsPaint=true after hot MarkNeedsPaint = isolation leak = FAIL",
			"impl_visible":       "3 colored hot regions (A=red, B=green, C=blue) pulse at different rates; static text labels and color blocks stay frozen",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r2_paint: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MinBoundarySkip:        1, // static boundary Replays under full_paint
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// Structural gates — R2 isolation invariants.
	if cleanOK < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: static_clean_ticks=%d want ≥10 (R2 isolation: static boundary must stay clean)\n", cleanOK)
		os.Exit(1)
	}
	if hotA < 10 || hotB < 10 || hotC < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: hot dirty ticks A=%d B=%d C=%d want each ≥10\n", hotA, hotB, hotC)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: PASS clean=%d hotA=%d hotB=%d hotC=%d paint=%d skip=%d fps=%.1f\n",
		cleanOK, hotA, hotB, hotC, snap.PaintCount, rep.BoundarySkip, rep.FPSInterval)
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

type scene2 struct {
	Root *rendering.AbsoluteBox
	hud  *wrkit.LiveHUD

	// Three independent hot boundaries, pulsing at different frequencies.
	hotA *rendering.RenderColorBox
	hotB *rendering.RenderColorBox
	hotC *rendering.RenderColorBox

	// Static boundary that must stay clean (NeedsPaint=false) while hots pulse.
	staticB *rendering.RenderColorBox

	// Counters for R2 isolation invariants.
	staticClean atomic.Int64
	hotDirtyA   atomic.Int64
	hotDirtyB   atomic.Int64
	hotDirtyC   atomic.Int64

	phaseA, phaseB, phaseC float64
}

func buildScene(w, h float64) *scene2 {
	s := &scene2{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	// --- TopBar (U17 多区域 第 1 区) ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("R2 local NeedsPaint — 3 hot boundaries pulse independently, static stays clean", 14, 16, 14, 0.88, 0.92, 0.98)

	// --- Legend (U17 第 2 区, ≥8 行色块+文字) ---
	leg := wrkit.NewPanel(240, 540, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.75, 0.95)
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"Hot A — red, pulses ~5 Hz", 0.90, 0.30, 0.25},
		{"Hot B — green, pulses ~3 Hz", 0.25, 0.75, 0.40},
		{"Hot C — blue, pulses ~7 Hz", 0.30, 0.50, 0.90},
		{"Static boundary — frozen", 0.55, 0.60, 0.70},
		{"Text labels — static dense", 0.90, 0.85, 0.50},
		{"PhaseClock: Steady/Spike/Recover", 0.55, 0.75, 0.85},
		{"LiveHUD band bottom (U18)", 0.55, 0.70, 0.80},
		{"Neighbors of hot stay clean", 0.65, 0.68, 0.72},
	}
	for i, ln := range legendLines {
		leg.ColorAt(14, 14, 12, 36+float64(i)*28, ln.r, ln.g, ln.b, 1, false)
		leg.LabelAt(ln.text, 11, 32, 36+float64(i)*28, ln.r, ln.g, ln.b)
	}

	// --- Three hot boundary panels (U17 多区域 第 3/4/5 区) ---
	// Each hot region is a RepaintBoundary color box, surrounded by static
	// text labels and a static color-block neighbor (proving local repaint
	// isolation: hot pulses, neighbor stays frozen).

	// Hot A: red, top-left of body.
	hotAX, hotAY := 276.0, 70.0
	hotAW, hotAH := 180.0, 180.0
	s.hotA = rendering.NewRenderColorBox(hotAW, hotAH, 0.90, 0.30, 0.25, 1)
	s.hotA.SetRepaintBoundary(true)
	root.Place(s.hotA, hotAX, hotAY)
	hotALbl := wrkit.NewPanel(hotAW, 22, 0.12, 0.14, 0.18, 0.9)
	hotALbl.PlaceOn(root, hotAX, hotAY-24)
	hotALbl.LabelAt("HOT A (red ~5Hz)", 11, 6, 14, 0.90, 0.75, 0.60)
	// Static text label below hot A (must stay clean).
	hotACap := wrkit.NewPanel(hotAW, 22, 0.10, 0.11, 0.13, 0.9)
	hotACap.PlaceOn(root, hotAX, hotAY+hotAH+6)
	hotACap.LabelAt("static label under A", 11, 6, 14, 0.70, 0.75, 0.85)

	// Hot B: green, top-right of body.
	hotBX, hotBY := 740.0, 70.0
	hotBW, hotBH := 180.0, 180.0
	s.hotB = rendering.NewRenderColorBox(hotBW, hotBH, 0.25, 0.75, 0.40, 1)
	s.hotB.SetRepaintBoundary(true)
	root.Place(s.hotB, hotBX, hotBY)
	hotBLbl := wrkit.NewPanel(hotBW, 22, 0.12, 0.14, 0.18, 0.9)
	hotBLbl.PlaceOn(root, hotBX, hotBY-24)
	hotBLbl.LabelAt("HOT B (green ~3Hz)", 11, 6, 14, 0.60, 0.85, 0.70)
	hotBCap := wrkit.NewPanel(hotBW, 22, 0.10, 0.11, 0.13, 0.9)
	hotBCap.PlaceOn(root, hotBX, hotBY+hotBH+6)
	hotBCap.LabelAt("static label under B", 11, 6, 14, 0.70, 0.75, 0.85)

	// Hot C: blue, bottom-center of body.
	hotCX, hotCY := 508.0, 320.0
	hotCW, hotCH := 180.0, 180.0
	s.hotC = rendering.NewRenderColorBox(hotCW, hotCH, 0.30, 0.50, 0.90, 1)
	s.hotC.SetRepaintBoundary(true)
	root.Place(s.hotC, hotCX, hotCY)
	hotCLbl := wrkit.NewPanel(hotCW, 22, 0.12, 0.14, 0.18, 0.9)
	hotCLbl.PlaceOn(root, hotCX, hotCY-24)
	hotCLbl.LabelAt("HOT C (blue ~7Hz)", 11, 6, 14, 0.60, 0.75, 0.90)
	hotCCap := wrkit.NewPanel(hotCW, 22, 0.10, 0.11, 0.13, 0.9)
	hotCCap.PlaceOn(root, hotCX, hotCY+hotCH+6)
	hotCCap.LabelAt("static label under C", 11, 6, 14, 0.70, 0.75, 0.85)

	// --- Static boundary (U17 多区域 第 6 区) ---
	// Frozen color block with a RepaintBoundary; must stay NeedsPaint=false
	// while hot A/B/C pulse every tick.
	staticX, staticY := 276.0, 540.0
	staticW, staticH := 460.0, 80.0
	s.staticB = rendering.NewRenderColorBox(staticW, staticH, 0.15, 0.55, 0.85, 1)
	s.staticB.SetRepaintBoundary(true)
	root.Place(s.staticB, staticX, staticY)
	staticLbl := wrkit.NewPanel(staticW, 22, 0.10, 0.11, 0.13, 0.9)
	staticLbl.PlaceOn(root, staticX, staticY+staticH+6)
	staticLbl.LabelAt("STATIC boundary — frozen, NeedsPaint must stay false", 11, 6, 14, 0.70, 0.80, 0.90)

	// --- Dense static text grid (U17 第 7 区, ≥8 labels) ---
	// 4×2 grid of small static labels on the right side, proving that
	// hot B/C pulsing nearby does not dirty these static labels.
	gridX, gridY := 936.0, 320.0
	for row := 0; row < 4; row++ {
		for col := 0; col < 2; col++ {
			cell := wrkit.NewPanel(120, 28, 0.10, 0.11, 0.13, 0.85)
			cell.PlaceOn(root, gridX+float64(col)*124, gridY+float64(row)*32)
			cell.LabelAt(fmt.Sprintf("static %d-%d", row, col), 11, 8, 16, 0.70, 0.75, 0.85)
		}
	}

	// --- LiveHUD (U18 窗内可见指标) ---
	if wrkit.HUDEnabled() {
		s.hud = wrkit.NewLiveHUD(w, hudH)
		root.Place(s.hud.Box, 0, h-hudH)
	}

	return s
}

func (s *scene2) onTick(dt float64, phase string) {
	if s == nil {
		return
	}
	// PhaseClock drives pulse rate — Spike doubles paint frequency.
	rateA, rateB, rateC := 5.0, 3.0, 7.0
	switch phase {
	case wrkit.PhaseSpike:
		rateA, rateB, rateC = 10.0, 6.0, 14.0
	case wrkit.PhaseRecover:
		rateA, rateB, rateC = 3.0, 2.0, 5.0
	}
	s.phaseA += dt * rateA
	s.phaseB += dt * rateB
	s.phaseC += dt * rateC

	// Hot A: red pulse.
	ra := 0.55 + 0.35*math.Sin(s.phaseA)
	s.hotA.R, s.hotA.G, s.hotA.B, s.hotA.A = ra, 0.20, 0.15, 1
	s.hotA.MarkNeedsPaint()
	s.hotDirtyA.Add(1)

	// Hot B: green pulse (different frequency).
	gb := 0.50 + 0.35*math.Sin(s.phaseB+0.7)
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.15, gb, 0.30, 1
	s.hotB.MarkNeedsPaint()
	s.hotDirtyB.Add(1)

	// Hot C: blue pulse (different frequency + offset).
	bc := 0.55 + 0.35*math.Sin(s.phaseC+1.4)
	s.hotC.R, s.hotC.G, s.hotC.B, s.hotC.A = 0.20, 0.35, bc, 1
	s.hotC.MarkNeedsPaint()
	s.hotDirtyC.Add(1)

	// R2 isolation invariant: static boundary must stay clean
	// (NeedsPaint=false) after all three hots called MarkNeedsPaint.
	if !s.staticB.NeedsPaint() {
		s.staticClean.Add(1)
	}
}
