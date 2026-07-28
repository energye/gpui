// Command ui_wr_r12b_debug_repaint is the R12b gate: debug repaint overlay
// on live paints (not cache Replay).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint
//	DEBUG_REPAINT=0 …  # off path must draw 0 overlays
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
	secs := runSeconds(8)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R12b (U16)")
		os.Exit(1)
	}
	// Default ON for the close gate; DEBUG_REPAINT=0 exercises off path.
	debugOn := os.Getenv("DEBUG_REPAINT") != "0"
	fmt.Fprintf(os.Stderr, "ui_wr_r12b_debug_repaint: R12b — %ds @ 1200x800 debug=%v\n", secs, debugOn)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r12b_debug_repaint",
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
				fmt.Fprintln(os.Stderr, "ui_wr_r12b_debug_repaint: close")
			}
		},
	})
	app.SetDebugRepaint(debugOn)

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
	draws := app.DebugRepaintDraws()
	snap := app.Metrics().Snapshot()
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R12b",
		Scenario:      "ui_wr_r12b_debug_repaint",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":           "1200x800",
			"run_seconds":         secs,
			"debug_repaint":       debugOn,
			"debug_repaint_draws": draws,
			"visible":             "magenta flash on hot live re-paints when debug=1",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r12b_debug_repaint: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if debugOn {
		if draws < 1 {
			fmt.Fprintf(os.Stderr, "FAIL: debug_repaint_draws=%d want ≥1 when DEBUG_REPAINT on\n", draws)
			os.Exit(1)
		}
	} else if draws != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: debug_repaint_draws=%d want 0 when DEBUG_REPAINT=0\n", draws)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r12b_debug_repaint: PASS draws=%d debug=%v\n", draws, debugOn)
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
	Root  *rendering.AbsoluteBox
	Hot   *rendering.RenderColorBox
	phase float64
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.1, G: 0.11, B: 0.14, A: 1}
	s.Root = root
	static := rendering.NewRenderColorBox(180, 180, 0.2, 0.55, 0.3, 1)
	static.SetRepaintBoundary(true)
	root.Place(static, 60, 80)
	s.Hot = rendering.NewRenderColorBox(200, 200, 0.3, 0.3, 0.35, 1)
	s.Hot.SetRepaintBoundary(true)
	root.Place(s.Hot, 500, 250)
	return s
}

func (s *scene) onTick(dt float64) {
	if s == nil || s.Hot == nil {
		return
	}
	s.phase += dt
	// Subtle base color change; debug overlay (magenta) is the visible repaint mark.
	v := 0.25 + 0.15*(0.5+0.5*math.Sin(s.phase*3))
	s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = v, v, v+0.05, 1
	s.Hot.MarkNeedsPaint()
}
