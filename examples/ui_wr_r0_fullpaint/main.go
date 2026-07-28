// Command ui_wr_r0_fullpaint is the R0 single-ability real-window gate for
// ENGINE_UI_WIDGET_RENDER W0 FullPaint correctness (static + animated content).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint   # min 5s, 1200×800
//
// Gates: presents>=1, present_policy=full_paint, §2.2 metrics schema;
// persistent FPS gate when RUN_SECONDS>=5 (fps_interval>=55 preferred).
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
	secs := runSeconds(5)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R0 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: R0 FullPaint static+animated — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	// U15: standard real-window client size for all ui_wr_* ability tests.
	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width:  winW,
		Height: winH,
		Title:  "gpui ui_wr_r0_fullpaint",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r0_fullpaint: close")
			}
		},
	})
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
	presents := app.PresentCount()
	snap := app.Metrics().Snapshot()
	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R0",
		Scenario:      "ui_wr_r0_fullpaint",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"static_box": "green 160x160 @ (40,40)",
			"anim_box":   "red pulsing @ (600,300)",
			"client_px":  "1200x800",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r0_fullpaint: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5, // U16: only gate FPS after full min run
		MaxP95Ms:               22,
	}
	// Very short smokes still require presents + policy; FPS only if long enough.
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: PASS presents=%d fps=%.1f policy=%s vsync=%s\n",
		rep.PresentCount, rep.FPSWall, rep.PresentPolicy, rep.VSyncSource)
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
	animB   *rendering.RenderColorBox
	phase   float64
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.1, G: 0.1, B: 0.12, A: 1}
	s.Root = root

	// Static green box — must remain visible every frame under FullPaint.
	s.staticB = rendering.NewRenderColorBox(160, 160, 0.1, 0.75, 0.2, 1)
	s.staticB.SetRepaintBoundary(true)
	root.Place(s.staticB, 40, 40)

	// Animated red box — marks dirty each tick.
	s.animB = rendering.NewRenderColorBox(120, 120, 0.95, 0.2, 0.15, 1)
	s.animB.SetRepaintBoundary(true)
	root.Place(s.animB, 600, 300)
	return s
}

func (s *scene) onTick(dt float64) {
	if s == nil || s.animB == nil {
		return
	}
	s.phase += dt
	// Pulse alpha-ish via color channel so paint is visibly dirty each frame.
	g := 0.15 + 0.35*(0.5+0.5*math.Sin(s.phase*4))
	s.animB.R, s.animB.G, s.animB.B, s.animB.A = 0.95, g, 0.15, 1
	s.animB.MarkNeedsPaint()
}
