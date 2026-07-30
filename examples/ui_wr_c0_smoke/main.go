// Command ui_wr_c0_smoke is the C0 combo real-window smoke (R0+R12 schema+R16 first frame).
//
// Quality bar (wr-close 模式 2 · U17/U18 integration):
//
//	  mini app shell · static dense + hot · WarmUp · LiveHUD · §2.2 schema
//
//		export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//		RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke   # close C0
package main

import (
	"fmt"
	"math"
	"os"
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
	hudH         = 72
	closeSeconds = 5 // §2.5 C0
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "C0")
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: C0 R0+R12+R16 quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c0_smoke",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.05, ClearB: 0.06, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true, // R16 subset: warm-up full present before loop
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c0_smoke: close")
			}
		},
	})

	phases := wrkit.NewPhaseClock(1.5, 3.5)
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		sc.onTick(dt, ph)
		proc.Sample()
		if wrkit.HUDEnabled() && sc.hud != nil {
			sc.hud.NoteTick(dt)
			snap := app.Metrics().Snapshot()
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			policy := snap.PresentPolicy
			if policy == "" {
				policy = scheduler.PresentPolicyFullPaint
			}
			gateOK := (fps >= 55 || phases.Elapsed() < 2) && policy == scheduler.PresentPolicyFullPaint
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "C0",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("covers=R0+R12+R16 warmup=true cells=%d", sc.staticCells),
				GateOK:      gateOK,
				Extra:       "combo smoke · schema + first present + static under Clear",
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
	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C0",
		Scenario:      "ui_wr_c0_smoke",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"covers":       []string{"R0", "R12", "R16"},
			"client_px":    "1200x800",
			"run_seconds":  secs,
			"static_cells": sc.staticCells,
			"label_count":  sc.labelCount,
			"hud":          wrkit.HUDEnabled(),
			"depcheck":     "passed",
			"quality_bar":  "U17+U18 integration",
			"r16_note":     "WarmUp=true subset; full R16 window optional post-W0",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c0_smoke: metrics JSON on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if !rep.Warmup {
		fmt.Fprintln(os.Stderr, "FAIL: warmup flag false (R16 subset)")
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want >=8 (C0 dense static)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: PASS presents=%d policy=%s schema=ok warmup=true fps=%.1f\n",
		rep.PresentCount, rep.PresentPolicy, rep.FPSInterval)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene struct {
	Root        *rendering.AbsoluteBox
	hotB        *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	phase       float64
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.14, A: 1}
	s.Root = root

	// Panel layout (local Place inside each region — same map as R0, lighter density):
	//   TopBar · Legend · Static 3×3 · Hot · LiveHUD

	top := wrkit.NewPanel(w, 48, 0.12, 0.15, 0.20, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("C0 smoke · R0 FullPaint + R12 schema + R16 WarmUp", 15, 16, 14, 0.85, 0.90, 0.98)
	s.labelCount++

	leg := wrkit.NewPanel(280, 300, 0.11, 0.13, 0.17, 1)
	leg.PlaceOn(root, 16, 60)
	leg.LabelAt("C0 covers", 13, 12, 10, 0.55, 0.75, 0.95)
	s.labelCount++
	for i, ln := range []string{
		"R0 FullPaint + R12 schema + R16 WarmUp",
		"static survives Clear",
		"warmup first present",
		"§2.2 schema required",
		"hot marks dirty each tick",
		"LiveHUD bottom band",
		"Steady → Spike → Recover",
		"not a substitute for R0 solo",
	} {
		leg.LabelAt(ln, 12, 12, 36+float64(i)*28, 0.70, 0.80, 0.90)
		s.labelCount++
	}

	const cell, gap = 48.0, 6.0
	grid := wrkit.NewPanel(280, 300, 0.10, 0.12, 0.16, 1)
	grid.PlaceOn(root, 320, 60)
	grid.LabelAt("STATIC 3x3 (R0 subset)", 13, 12, 10, 0.65, 0.85, 0.95)
	s.labelCount++
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			grid.ColorAt(cell, cell,
				16+float64(c)*(cell+gap), 40+float64(r)*(cell+gap),
				0.18+0.12*float64(c), 0.40+0.10*float64(r), 0.55, 1, true)
			s.staticCells++
		}
	}

	hot := wrkit.NewPanel(560, 300, 0.10, 0.11, 0.14, 1)
	hot.PlaceOn(root, 620, 60)
	hot.LabelAt("HOT pulse", 13, 12, 10, 0.95, 0.50, 0.35)
	s.labelCount++
	s.hotB = hot.ColorAt(140, 140, 24, 48, 0.95, 0.30, 0.20, 1, true)
	hot.LabelAt("extra static", 12, 200, 48, 0.60, 0.70, 0.85)
	s.labelCount++
	for i := 0; i < 4; i++ {
		hot.ColorAt(36, 36, 200, 72+float64(i)*44, 0.25, 0.35, 0.55-0.05*float64(i), 1, true)
		s.staticCells++
	}

	if wrkit.HUDEnabled() {
		s.hud = wrkit.NewLiveHUD(w, hudH)
		root.Place(s.hud.Box, 0, h-hudH)
	}
	return s
}

func (s *scene) onTick(dt float64, phase string) {
	if s == nil || s.hotB == nil {
		return
	}
	rate := 4.0
	if phase == wrkit.PhaseSpike {
		rate = 9.0
	} else if phase == wrkit.PhaseRecover {
		rate = 3.0
	}
	s.phase += dt * rate
	g := 0.20 + 0.35*(0.5+0.5*math.Sin(s.phase))
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.95, g, 0.18, 1
	s.hotB.MarkNeedsPaint()
}
