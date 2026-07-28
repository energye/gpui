// Command ui_wr_r2_paint is the R2 single-ability real-window gate:
// local NeedsPaint isolation (only target node repaints under boundary).
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
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(5)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R2 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: R2 local NeedsPaint — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r2_paint",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var staticStillClean atomic.Int64
	var hotDirtyOK atomic.Int64

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
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		sc.onTick(dt)
		// After MarkNeedsPaint on hot: static must stay clean (R2 isolation).
		if !sc.staticB.NeedsPaint() {
			staticStillClean.Add(1)
		}
		if sc.hotB.NeedsPaint() {
			hotDirtyOK.Add(1)
		}
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
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}

	cleanOK := staticStillClean.Load()
	hotOK := hotDirtyOK.Load()
	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R2",
		Scenario:      "ui_wr_r2_paint",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":          "1200x800",
			"run_seconds":        secs,
			"static_clean_ticks": cleanOK,
			"hot_dirty_ticks":    hotOK,
			"paint_count":        snap.PaintCount,
			"note":               "only hot boundary MarkNeedsPaint each tick; static stays clean",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r2_paint: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
		MinBoundarySkip: 1, // static boundary Replays
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if cleanOK < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: static_clean_ticks=%d want ≥10 (NeedsPaint isolation)\n", cleanOK)
		os.Exit(1)
	}
	if hotOK < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: hot_dirty_ticks=%d want ≥10\n", hotOK)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: PASS clean_ticks=%d hot_ticks=%d paint=%d skip=%d\n",
		cleanOK, hotOK, snap.PaintCount, rep.BoundarySkip)
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

type scene struct {
	Root    *rendering.AbsoluteBox
	staticB *rendering.RenderColorBox
	hotB    *rendering.RenderColorBox
	phase   float64
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root
	s.staticB = rendering.NewRenderColorBox(200, 200, 0.15, 0.55, 0.85, 1)
	s.staticB.SetRepaintBoundary(true)
	root.Place(s.staticB, 80, 100)
	s.hotB = rendering.NewRenderColorBox(160, 160, 0.9, 0.3, 0.15, 1)
	s.hotB.SetRepaintBoundary(true)
	root.Place(s.hotB, 700, 280)
	return s
}

func (s *scene) onTick(dt float64) {
	if s == nil || s.hotB == nil {
		return
	}
	s.phase += dt
	g := 0.2 + 0.4*(0.5+0.5*math.Sin(s.phase*5))
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.95, g, 0.15, 1
	s.hotB.MarkNeedsPaint()
}
