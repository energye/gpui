// Command ui_wr_c1_boundary_nest is the C1 combo real-window gate:
// nested boundary + paint dirty isolation + optional debug-repaint tint (W1).
//
// Does NOT close solo R2/R3/R3b/R12b — those need their own packages.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest
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
	secs := runSeconds(10)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close C1 (U16)")
		os.Exit(1)
	}
	debugRepaint := os.Getenv("DEBUG_REPAINT") == "1"
	fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: C1 nest+skip — %ds @ 1200x800 debug_repaint=%v\n", secs, debugRepaint)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c1_boundary_nest",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH), debugRepaint)
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.06, ClearB: 0.08, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c1_boundary_nest: close")
			}
		},
	})
	app.Metrics().SetBoundaryDiscovery(cnt, depth)
	app.Pipeline().UpdateCompositingBits()

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
		AbilityID:     "C1",
		Scenario:      "ui_wr_c1_boundary_nest",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":          "1200x800",
			"run_seconds":        secs,
			"covers":             "R2+R3+R3b(+R12b tint)",
			"debug_repaint":      debugRepaint,
			"boundary_count":     cnt,
			"boundary_max_depth": depth,
			"note":               "combo only — does not close solo R rows",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c1_boundary_nest: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MinBoundarySkip:        1,
		MinBoundaryRerecord:    1,
		MinBoundaryCount:       3,
		MinBoundaryMaxDepth:    2,
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: PASS skip=%d rr=%d count=%d\n",
		rep.BoundarySkip, rep.BoundaryRerecord, rep.BoundaryCount)
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
	Root         *rendering.AbsoluteBox
	Hot          *rendering.RenderColorBox
	debugRepaint bool
	phase        float64
}

func buildScene(w, h float64, debugRepaint bool) *scene {
	s := &scene{debugRepaint: debugRepaint}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.06, G: 0.07, B: 0.09, A: 1}
	s.Root = root

	outer := rendering.NewAbsoluteBox(500, 400)
	outer.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.22, A: 1}
	outer.SetRepaintBoundary(true)

	mid := rendering.NewAbsoluteBox(360, 280)
	mid.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.32, A: 1}
	mid.SetRepaintBoundary(true)

	static := rendering.NewRenderColorBox(100, 100, 0.15, 0.65, 0.3, 1)
	static.SetRepaintBoundary(true)
	mid.Place(static, 30, 30)

	hot := rendering.NewRenderColorBox(110, 110, 0.9, 0.2, 0.15, 1)
	hot.SetRepaintBoundary(true)
	mid.Place(hot, 200, 120)
	s.Hot = hot

	// Sibling static on outer (not under mid) — must skip independently.
	side := rendering.NewRenderColorBox(70, 200, 0.35, 0.4, 0.7, 1)
	side.SetRepaintBoundary(true)
	outer.Place(side, 400, 40)

	outer.Place(mid, 24, 40)
	root.Place(outer, 80, 80)
	return s
}

func (s *scene) onTick(dt float64) {
	if s == nil || s.Hot == nil {
		return
	}
	s.phase += dt
	g := 0.12 + 0.4*(0.5+0.5*math.Sin(s.phase*5))
	if s.debugRepaint {
		// R12b-ish tint: push magenta flash on dirty hot so repaint is obvious.
		s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = 0.95, g*0.3, 0.85, 1
	} else {
		s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = 0.95, g, 0.15, 1
	}
	s.Hot.MarkNeedsPaint()
}
