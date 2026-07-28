// Command ui_wr_r3b_compbits is the R3b single-ability real-window gate for
// compositing-bits / nested boundary discovery (ENGINE_UI_WIDGET_RENDER W1).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits   # close R3b @ 1200×800
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
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R3b (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: R3b compositing bits / nest — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r3b_compbits",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	if cnt < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: scene boundary_count=%d want >=3\n", cnt)
		os.Exit(1)
	}
	if depth < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: scene boundary_max_depth=%d want >=2\n", depth)
		os.Exit(1)
	}

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r3b_compbits: close")
			}
		},
	})
	app.Metrics().SetBoundaryDiscovery(cnt, depth)
	// Force compositing-bits walk (R3b discovery path).
	app.Pipeline().UpdateCompositingBits()
	if !sc.Outer.NeedsCompositing() {
		fmt.Fprintln(os.Stderr, "FAIL: outer boundary must NeedsCompositing after UpdateCompositingBits")
		os.Exit(1)
	}

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
		AbilityID:     "R3b",
		Scenario:      "ui_wr_r3b_compbits",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":          "1200x800",
			"run_seconds":        secs,
			"nest":               "outer AbsoluteBox boundary → mid boundary → hot leaf",
			"boundary_count":     cnt,
			"boundary_max_depth": depth,
			"needs_compositing":  sc.Outer.NeedsCompositing(),
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r3b_compbits: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MinBoundaryCount:       3,
		MinBoundaryMaxDepth:    2,
		MinBoundarySkip:        1, // outer/static nest still contributes skip when fully clean frames occur
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: PASS count=%d depth=%d skip=%d\n",
		rep.BoundaryCount, rep.BoundaryMaxDepth, rep.BoundarySkip)
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
	Outer *rendering.AbsoluteBox
	Mid   *rendering.AbsoluteBox
	Hot   *rendering.RenderColorBox
	phase float64
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.07, G: 0.08, B: 0.10, A: 1}
	s.Root = root

	// Nested chain: outer → mid → hot leaf (all repaint boundaries).
	outer := rendering.NewAbsoluteBox(420, 360)
	outer.Background = &rendering.Color{R: 0.15, G: 0.18, B: 0.28, A: 1}
	outer.SetRepaintBoundary(true)
	s.Outer = outer

	mid := rendering.NewAbsoluteBox(280, 240)
	mid.Background = &rendering.Color{R: 0.22, G: 0.28, B: 0.38, A: 1}
	mid.SetRepaintBoundary(true)
	s.Mid = mid

	static := rendering.NewRenderColorBox(80, 80, 0.2, 0.7, 0.35, 1)
	static.SetRepaintBoundary(true)
	mid.Place(static, 24, 24)

	hot := rendering.NewRenderColorBox(90, 90, 0.95, 0.25, 0.2, 1)
	hot.SetRepaintBoundary(true)
	mid.Place(hot, 150, 100)
	s.Hot = hot

	outer.Place(mid, 40, 40)
	root.Place(outer, 120, 100)
	return s
}

func (s *scene) onTick(dt float64) {
	if s == nil || s.Hot == nil {
		return
	}
	s.phase += dt
	g := 0.15 + 0.35*(0.5+0.5*math.Sin(s.phase*4))
	s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = 0.95, g, 0.2, 1
	s.Hot.MarkNeedsPaint()
}
