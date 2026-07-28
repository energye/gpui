// Command ui_wr_c7_resize_dpr is the C7 combo: R11 size invalidation + R3 boundary
// skip recovery (+ optional 1px line for R19 awareness). Does not close solos.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c7_resize_dpr
package main

import (
	"fmt"
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
	secs := runSeconds(15)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close C7 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: C7 resize/DPR combo — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c7_resize_dpr",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var inval atomic.Int64
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c7_resize_dpr: close")
			}
		},
	})
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	tStart := time.Now()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		el := time.Since(tStart).Seconds()
		if el >= 4 && el < 4.5 && inval.Load() == 0 {
			sc.resize(240, 180)
			app.InvalidateBoundaryCache()
			inval.Add(1)
		}
		if el >= 8 && el < 8.5 && inval.Load() == 1 {
			sc.resize(180, 220)
			app.InvalidateBoundaryCache()
			inval.Add(1)
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
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C7",
		Scenario:      "ui_wr_c7_resize_dpr",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":           "1200x800",
			"run_seconds":         secs,
			"covers":              "R11+R3(+R19 line)",
			"cache_invalidations": inval.Load(),
			"boundary_count":      cnt,
			"boundary_max_depth":  depth,
			"note":                "combo only — does not close solo R11/R3",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c7_resize_dpr: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
		MinBoundarySkip: 1, MinBoundaryCount: 2,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if inval.Load() < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: cache_invalidations=%d\n", inval.Load())
		os.Exit(1)
	}
	if snap.BoundaryRerecord < 1 && rep.BoundaryRerecord < 1 {
		// lifetime may be in snap after merge
		if cache := app.BoundaryCache(); cache == nil || cache.Rerecord < 1 {
			fmt.Fprintln(os.Stderr, "FAIL: expected rerecord after invalidation")
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: PASS inval=%d skip=%d rr=%d\n",
		inval.Load(), rep.BoundarySkip, rep.BoundaryRerecord)
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
	Root *rendering.AbsoluteBox
	box  *rendering.RenderColorBox
	line *rendering.RenderColorBox // 1px-ish hairline (R19 awareness)
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.07, G: 0.08, B: 0.10, A: 1}
	s.Root = root

	s.box = rendering.NewRenderColorBox(200, 160, 0.25, 0.5, 0.85, 1)
	s.box.SetRepaintBoundary(true)
	root.Place(s.box, 100, 120)

	// Thin horizontal line (device-pixel awareness demo; not a full R19 gate).
	s.line = rendering.NewRenderColorBox(400, 1, 0.95, 0.95, 0.2, 1)
	s.line.SetRepaintBoundary(true)
	root.Place(s.line, 100, 320)

	static := rendering.NewRenderColorBox(120, 120, 0.2, 0.65, 0.3, 1)
	static.SetRepaintBoundary(true)
	root.Place(static, 500, 200)
	return s
}

func (s *demoScene) resize(w, h float64) {
	if s == nil || s.box == nil {
		return
	}
	s.box.Width, s.box.Height = w, h
	s.box.MarkNeedsLayout()
	s.box.MarkNeedsPaint()
}
