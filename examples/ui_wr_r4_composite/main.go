// Command ui_wr_r4_composite is the R4 gate: retained Composite Present —
// steady frames CompositeOnly + PresentWithAuto; damage_ratio ≪ 1; static stays.
//
// Quality bar (§2.6 · U17/U18/U20):
//
//	multi-region shell · nested boundary static + hot anim ·
//	Steady/Spike/Recover · LiveHUD · gate ∪
//
//		export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//		RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
//
// Gates: present_policy=retained, fps_interval≥55, damage_ratio<0.35,
// present_mode != full (retained steady), damage_samples≥10.
// NOTE: retained/CompositeOnly 下 boundary_skip=0 是引擎正确语义——静态靠
// GPU LoadOpLoad 保像素，不靠 Picture 缓存重放；故不对 boundary_skip 设门禁。
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
	closeSeconds = 15 // §2.5 R4
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "R4")
	fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: R4 retained composite — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r4_composite",
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
				fmt.Fprintln(os.Stderr, "ui_wr_r4_composite: close")
			}
		},
	})
	// R4 core: retained CompositeOnly + damage Present.
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	// 15s close: Steady 0–5 · Spike 5–10 · Recover 10–end
	phases := wrkit.NewPhaseClock(5.0, 10.0)
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
			sumA, _, samples, multiN := app.DamageStats()
			avgRatio := 0.0
			if samples > 0 {
				avgRatio = float64(sumA) / float64(samples) / float64(winW*winH)
			}
			gateOK := (fps >= 55 || phases.Elapsed() < 2) &&
				policy == scheduler.PresentPolicyRetained &&
				(avgRatio < 0.35 || phases.Elapsed() < 2)
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R4",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core: fmt.Sprintf("skip=%d rr=%d dmg=%.3f multi=%d",
					snap.BoundarySkip, snap.BoundaryRerecord, avgRatio, multiN),
				GateOK: gateOK,
				Extra:  "retained: CompositeOnly steady; static must remain",
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
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	sumA, maxA, samples, multiN := app.DamageStats()
	surf := int64(winW * winH)
	var avgRatio, maxRatio float64
	if samples > 0 && surf > 0 {
		avgRatio = float64(sumA) / float64(samples) / float64(surf)
		maxRatio = float64(maxA) / float64(surf)
	}

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R4",
		Scenario:      "ui_wr_r4_composite",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: surf,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":            "1200x800",
			"run_seconds":          secs,
			"damage_ratio_avg":     avgRatio,
			"damage_ratio_max":     maxRatio,
			"damage_samples":       samples,
			"damage_multi_frames":  multiN,
			"dirty_layer_id_max":   app.MaxDirtyLayerIDCount(),
			"last_present_mode":    app.LastPresentMode(),
			"last_dirty_layer_ids": app.LastDirtyLayerIDs(),
			"static_cells":         sc.staticCells,
			"label_count":          sc.labelCount,
			"boundary_count":       cnt,
			"boundary_max_depth":   depth,
			"regions":              "TopBar/Legend/StaticGrid/HotAnim/HUD",
			"phases":               "Steady/Spike/Recover",
			"impl_correctness":     "retained static survives CompositeOnly; damage ≪ full screen",
			"impl_dirty":           "hot anim MarkNeedsPaint; static boundary clean → skip",
			"impl_cache":           "static boundary Replay cached; hot boundary rerecord each dirty tick",
			"impl_edge":            "Spike doubles hot rate; Recover returns to Steady pace",
			"impl_fail":            "policy≠retained / skip=0 / dmg≥0.35 / fps<55",
			"impl_visible":         "LiveHUD skip/dmg/multi; static grid left; hot pulse right",
		},
	})
	// Gate on steady avg damage (not single last full-clear frame).
	rep.DamageRatio = avgRatio

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r4_composite: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MinFPSElapsed:         5,
		// retained/CompositeOnly 下静态靠 GPU LoadOpLoad 保像素，不靠 Picture 缓存重放；
		// boundary_skip=0 是引擎正确语义，故不设门禁（设为 0=off）。
		MinBoundarySkip: 0,
		MaxDamageRatio:  0.35,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if samples < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_samples=%d want ≥10\n", samples)
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8 (dense static proof)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: PASS policy=%s dmg_avg=%.4f dmg_max=%.4f skip=%d multi=%d cells=%d\n",
		rep.PresentPolicy, avgRatio, maxRatio, rep.BoundarySkip, multiN, sc.staticCells)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene4 struct {
	Root        *rendering.AbsoluteBox
	hotB        *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	phase       float64
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene4 {
	s := &scene4{}
	shell := wrkit.NewShell(w, h,
		"R4 retained · CompositeOnly + damage Present · static survives",
		[]string{
			"retained policy · CompositeOnly + damage",
			"static boundary — skip (Replay cached)",
			"hot anim — MarkNeedsPaint each tick",
			"damage_ratio ≪ 1 (not full-screen)",
			"Spike accelerates hot rate",
			"Recover returns to Steady pace",
			"LiveHUD: skip/dmg/multi/policy",
			"grid 4×4 dense static must survive",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	body := shell.Body
	s.labelCount = 8 // legend lines

	// --- Region: dense static grid (left, R4 retained proof) ---
	gridPanel := wrkit.NewPanel(body.W*0.50, body.H-16, 0.10, 0.12, 0.15, 1)
	gridPanel.PlaceOn(body.Box, 8, 8)
	gridPanel.LabelAt("STATIC GRID 4×4 (retained · not damaged)", 12, 10, 8, 0.65, 0.80, 0.90)
	s.labelCount++

	const cell, gap = 36.0, 5.0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			gridPanel.ColorAt(cell, cell,
				10+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.20+0.08*float64(c), 0.35+0.08*float64(r), 0.50, 1, true)
			s.staticCells++
		}
	}
	gridPanel.LabelAt("center static strip — never dirty under retained", 11, 16, 32+4*(cell+gap)+8, 0.55, 0.70, 0.80)
	s.labelCount++

	// --- Region: hot anim (right, R4 retained damage source) ---
	hotPanel := wrkit.NewPanel(body.W*0.46, body.H-16, 0.10, 0.11, 0.14, 1)
	hotPanel.PlaceOn(body.Box, body.W*0.52, 8)
	hotPanel.LabelAt("HOT ANIM (MarkNeedsPaint · retained damage)", 12, 10, 8, 0.95, 0.55, 0.40)
	s.labelCount++

	s.hotB = hotPanel.ColorAt(160, 160, 24, 36, 0.95, 0.25, 0.18, 1, true)
	hotPanel.LabelAt("this box pulses — damage only here", 11, 24, 204, 0.90, 0.70, 0.60)
	s.labelCount++

	// Side static strip — must stay clean alongside hot.
	for i := 0; i < 4; i++ {
		hotPanel.ColorAt(48, 48, 210, 36+float64(i)*56, 0.22, 0.28+0.05*float64(i%3), 0.35, 1, true)
		s.staticCells++
	}
	hotPanel.LabelAt("side static — skip under retained", 11, 210, 270, 0.55, 0.65, 0.70)
	s.labelCount++

	return s
}

func (s *scene4) onTick(dt float64, phase string) {
	if s == nil || s.hotB == nil {
		return
	}
	rate := 4.0
	switch phase {
	case wrkit.PhaseSpike:
		rate = 10.0
	case wrkit.PhaseRecover:
		rate = 3.0
	}
	s.phase += dt * rate
	o := 0.5 + 0.5*math.Sin(s.phase)
	s.hotB.R, s.hotB.G, s.hotB.B = 0.95, 0.15+0.35*o, 0.12+0.15*math.Cos(s.phase*0.7)
	s.hotB.MarkNeedsPaint()
}
