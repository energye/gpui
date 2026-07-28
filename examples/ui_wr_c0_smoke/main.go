// Command ui_wr_c0_smoke is the C0 combo real-window smoke (R0+R12 schema+R16 first frame).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke   # min 5s, 1200×800
package main

import (
	"fmt"
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
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close C0 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: C0 R0+R12+R16 smoke — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c0_smoke",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	root := rendering.NewAbsoluteBox(float64(winW), float64(winH))
	root.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.18, A: 1}
	static := rendering.NewRenderColorBox(200, 200, 0.2, 0.6, 0.9, 1)
	static.SetRepaintBoundary(true)
	root.Place(static, 100, 100)

	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.05, ClearB: 0.06, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true, // R16: warm-up full present before loop
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c0_smoke: close")
			}
		},
	})
	// Keep presenting so schema has non-zero frame/present counts.
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
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
		AbilityID:     "C0",
		Scenario:      "ui_wr_c0_smoke",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"covers": []string{"R0", "R12", "R16"},
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c0_smoke: metrics JSON on stdout")
	fmt.Println(string(b))

	// C0: schema + presents + full_paint; FPS after full U16 min run.
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
		fmt.Fprintln(os.Stderr, "FAIL: warmup flag false (R16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: PASS presents=%d policy=%s schema=ok\n",
		rep.PresentCount, rep.PresentPolicy)
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
