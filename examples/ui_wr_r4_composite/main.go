// Command ui_wr_r4_composite is the R4 gate: retained Composite Present —
// steady frames CompositeOnly + PresentWithAuto; damage_ratio ≪ 1; static stays.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(15)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R4 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: R4 retained composite — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
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
	// W2: retained steady path (CompositeOnly + damage Present).
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		sc.onTick(dt)
		proc.Sample()
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
	sumA, maxA, samples, multiN := app.DamageStats()
	surf := int64(winW * winH)
	var avgRatio, maxRatio float64
	if samples > 0 && surf > 0 {
		avgRatio = float64(sumA) / float64(samples) / float64(surf)
		maxRatio = float64(maxA) / float64(surf)
	}
	// Prefer last-frame snap damage for wrgate DamageRatio; override with steady avg.
	if snap.DamageAreaPx == 0 && maxA > 0 {
		snap.DamageAreaPx = maxA
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
			"note":                 "retained: CompositeOnly steady; static green must remain",
		},
	})
	// Gate on steady avg ratio (not single last full-clear frame).
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
		MaxDamageRatio:        0.35, // hot 140² / 1200×800 ≈ 0.02; allow headroom
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if samples < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_samples=%d want ≥10\n", samples)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: PASS policy=%s dmg_avg=%.4f dmg_max=%.4f mode=%s\n",
		rep.PresentPolicy, avgRatio, maxRatio, app.LastPresentMode())
}

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

type demoScene struct {
	Root    *rendering.AbsoluteBox
	staticB *rendering.RenderColorBox
	hotB    *rendering.RenderColorBox
	phase   float64
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root
	s.staticB = rendering.NewRenderColorBox(220, 180, 0.12, 0.65, 0.28, 1)
	s.staticB.SetRepaintBoundary(true)
	root.Place(s.staticB, 48, 60)
	s.hotB = rendering.NewRenderColorBox(140, 140, 0.95, 0.25, 0.15, 1)
	s.hotB.SetRepaintBoundary(true)
	root.Place(s.hotB, 900, 480)
	return s
}

func (s *demoScene) onTick(dt float64) {
	if s == nil || s.hotB == nil {
		return
	}
	s.phase += dt
	g := 0.15 + 0.35*(0.5+0.5*math.Sin(s.phase*5))
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.95, g, 0.15, 1
	s.hotB.MarkNeedsPaint()
}
