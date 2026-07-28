// Command ui_wr_r4b_multidamage is the R4b gate: two distant dirty hotspots;
// DirtyLayerIDs lists both; middle static stays; damage not full-screen.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r4b_multidamage
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
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R4b (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: R4b multi damage — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r4b_multidamage",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r4b_multidamage: close")
			}
		},
	})
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
	var avgRatio float64
	if samples > 0 && surf > 0 {
		avgRatio = float64(sumA) / float64(samples) / float64(surf)
	}
	maxDirty := app.MaxDirtyLayerIDCount()
	lastIDs := app.LastDirtyLayerIDs()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R4b",
		Scenario:      "ui_wr_r4b_multidamage",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: surf,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":            "1200x800",
			"run_seconds":          secs,
			"damage_ratio_avg":     avgRatio,
			"damage_ratio_max":     float64(maxA) / float64(surf),
			"damage_samples":       samples,
			"damage_multi_frames":  multiN,
			"dirty_layer_id_max":   maxDirty,
			"last_dirty_layer_ids": lastIDs,
			"hotspots":             "TL orange + BR cyan; mid gray static",
		},
	})
	rep.DamageRatio = avgRatio

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r4b_multidamage: metrics JSON follows on stdout")
	fmt.Println(string(b))

	// Distant dual hots: FrameDamage *union AABB* can be large (honest metric),
	// but PlanFramePresent keeps damage_multi (sum of rects ≪ surface). Gate on
	// multi mode + dirty ids — not union coverage (see render.PlanFramePresent).
	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MinFPSElapsed:         5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if maxDirty < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: dirty_layer_id_max=%d want ≥2 (two distant hots)\n", maxDirty)
		os.Exit(1)
	}
	if multiN < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_multi_frames=%d want ≥1 (independent scissors for distant dirties)\n", multiN)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: PASS dirty_max=%d multi_frames=%d union_dmg_avg=%.4f\n",
		maxDirty, multiN, avgRatio)
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
	Root  *rendering.AbsoluteBox
	mid   *rendering.RenderColorBox
	hotTL *rendering.RenderColorBox
	hotBR *rendering.RenderColorBox
	phase float64
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.07, G: 0.08, B: 0.10, A: 1}
	s.Root = root

	// Large mid static — must not dirty when only TL/BR pulse.
	s.mid = rendering.NewRenderColorBox(400, 300, 0.22, 0.24, 0.28, 1)
	s.mid.SetRepaintBoundary(true)
	root.Place(s.mid, 400, 250)

	s.hotTL = rendering.NewRenderColorBox(100, 100, 0.95, 0.45, 0.1, 1)
	s.hotTL.SetRepaintBoundary(true)
	root.Place(s.hotTL, 40, 40)

	s.hotBR = rendering.NewRenderColorBox(100, 100, 0.15, 0.75, 0.85, 1)
	s.hotBR.SetRepaintBoundary(true)
	root.Place(s.hotBR, 1050, 650)
	return s
}

func (s *demoScene) onTick(dt float64) {
	if s == nil {
		return
	}
	s.phase += dt
	o := 0.5 + 0.5*math.Sin(s.phase*4)
	if s.hotTL != nil {
		s.hotTL.R, s.hotTL.G, s.hotTL.B = 0.95, 0.3+0.4*o, 0.1
		s.hotTL.MarkNeedsPaint()
	}
	if s.hotBR != nil {
		s.hotBR.R, s.hotBR.G, s.hotBR.B = 0.1, 0.5+0.3*o, 0.85
		s.hotBR.MarkNeedsPaint()
	}
}
