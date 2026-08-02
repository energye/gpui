// Command ui_wr_r12b_debug_repaint is the W1 R12b real-window: debug repaint visualization.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
// WR_DEBUG_REPAINT=0 disables the overlay (off-phase gate); default on.
package main

import (
	"encoding/json"
	"fmt"
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

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R12b")
	}
	wrkit.EnsureUIFace()

	debugOn := os.Getenv("WR_DEBUG_REPAINT") != "0"

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r12b_debug_repaint — 重绘可视化"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R12b debug重绘 — magenta叠加", []string{
		"magenta flash = live repaint",
		"on: 脏区闪烁 (draws≥1)",
		"off: 无叠加 (draws==0)",
		"多区域 + 静态块对照",
	})

	// Multi-region: 3 static boundaries + 2 hot regions (different rates).
	statics := make([]*rendering.RenderColorBox, 0, 4)
	for i := 0; i < 4; i++ {
		c := rendering.NewRenderColorBox(100, 80, 0.25, 0.55, 0.8, 1)
		c.SetRepaintBoundary(true)
		shell.Body.Place(c, 20+float64(i)*130, 20)
		statics = append(statics, c)
	}
	// Dense static labels.
	for i := 0; i < 8; i++ {
		t := wrkit.Label(fmt.Sprintf("static %d", i), 11, 0.7, 0.75, 0.85)
		shell.Body.Place(t, 20+float64(i%4)*140, 140+float64(i/4)*40)
	}
	// Dense static color strip (non-boundary nodes; must never flash).
	for i := 0; i < 8; i++ {
		c := rendering.NewRenderColorBox(46, 34, 0.2+0.05*float64(i), 0.5, 0.65, 1)
		shell.Body.Place(c, 20+float64(i)*52, 230)
	}

	hotA := rendering.NewRenderColorBox(90, 90, 0.9, 0.3, 0.3, 1)
	hotA.SetRepaintBoundary(true)
	shell.Body.Place(hotA, 560, 20)
	hotB := rendering.NewRenderColorBox(60, 60, 0.3, 0.8, 0.5, 1)
	hotB.SetRepaintBoundary(true)
	shell.Body.Place(hotB, 560, 140)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r12b_debug_repaint: close (%s)\n", win.Backend())
			}
		},
	})
	app.SetDebugRepaint(debugOn)

	clock := wrkit.NewPhaseClock(2, 5)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			hotA.R = 0.7 + 0.2*float64(int(clock.Elapsed())%2)
			hotA.MarkNeedsPaint()
		case wrkit.PhaseSpike:
			hotA.MarkNeedsPaint()
			hotB.MarkNeedsPaint()
		default:
			// Recover: no dirty → no magenta flash at all.
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (flash counter proves on/off behavior live).
		shell.NoteHUDTick(dt)
		drawsH := app.DebugRepaintDraws()
		gateOK := (debugOn && drawsH >= 1) || (!debugOn && drawsH == 0)
		shell.UpdateHUD("R12b", phase, app, gateOK,
			fmt.Sprintf("on=%v flash_draws=%d", debugOn, drawsH), "")
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	draws := app.DebugRepaintDraws()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R12b",
		Scenario:      "ui_wr_r12b_debug_repaint",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"debug_repaint_on": debugOn,
			"debug_draws":      draws,
			"static_boxes":     len(statics),
		},
	})
	report.DebugRepaintDraws = draws
	report.DebugRepaintOn = debugOn
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	// R12b gate: on → draws≥1; off → draws==0.
	if debugOn && draws < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: debug on but debug_draws=0 (overlay not painted)")
		os.Exit(1)
	}
	if !debugOn && draws != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: debug off but debug_draws=%d (overlay leaked)", draws)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r12b_debug_repaint: OK on=%v draws=%d presents=%d elapsed=%.1fs\n",
		debugOn, draws, app.PresentCount(), elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
