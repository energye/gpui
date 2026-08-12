// Command ui_wr_c1_boundary_nest is the W1 C1 composite real-window:
// R2+R3+R3b+R12b integrated — nested boundaries + dirty isolation + debug overlay.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
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
		wrkit.RequireMinRun(secs, "C1")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c1_boundary_nest — 组合窗"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C1 嵌套boundary组合 — R2+R3+R3b+R12b", []string{
		"4 层嵌套 boundary",
		"内脏→只内 rerecord",
		"外脏→只外 rerecord",
		"debug 色只在脏区闪",
		"boundary_count in HUD",
	})

	// 4-level nest: L1(280) → L2(220) → L3(160) → L4(100).
	L1 := rendering.NewAbsoluteBox(280, 280)
	L1.SetRepaintBoundary(true)
	shell.Body.Place(L1, 20, 20)

	L2 := rendering.NewAbsoluteBox(220, 220)
	L2.SetRepaintBoundary(true)
	L1.Place(L2, 30, 30)

	L3 := rendering.NewAbsoluteBox(160, 160)
	L3.SetRepaintBoundary(true)
	L2.Place(L3, 30, 30)

	L4 := rendering.NewAbsoluteBox(100, 100)
	L4.SetRepaintBoundary(true)
	L3.Place(L4, 30, 30)

	hot := rendering.NewRenderColorBox(50, 50, 0.9, 0.3, 0.3, 1)
	L4.Place(hot, 25, 25)

	// Static text at every level + static blocks (content mix).
	L1.Place(wrkit.Label("L1", 12, 0.8, 0.85, 0.9), 5, 260)
	L2.Place(wrkit.Label("L2", 12, 0.8, 0.85, 0.9), 5, 200)
	L3.Place(wrkit.Label("L3", 12, 0.8, 0.85, 0.9), 5, 140)
	L4.Place(wrkit.Label("L4", 12, 0.8, 0.85, 0.9), 5, 80)
	for i := 0; i < 3; i++ {
		c := rendering.NewRenderColorBox(30, 30, 0.3, 0.55, 0.8, 1)
		L1.Place(c, 240, 20+float64(i)*40)
	}

	// Sibling nest to prove isolation (no cross-nest propagation).
	S := rendering.NewAbsoluteBox(160, 160)
	S.SetRepaintBoundary(true)
	shell.Body.Place(S, 340, 20)
	hotS := rendering.NewRenderColorBox(40, 40, 0.3, 0.8, 0.5, 1)
	S.Place(hotS, 60, 60)

	// Static dense 4x4 grid outside boundaries.
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(36, 36, 0.24, 0.45, 0.7, 1)
			shell.Body.Place(c, 540+float64(i)*48, 20+float64(j)*48)
		}
	}
	// Extra density: labels column + non-boundary color strip (mixed content).
	for i := 0; i < 6; i++ {
		t := wrkit.Label(fmt.Sprintf("c1 static %d", i), 11, 0.65, 0.72, 0.85)
		shell.Body.Place(t, 540, 230+float64(i)*26)
	}
	for i := 0; i < 8; i++ {
		c := rendering.NewRenderColorBox(40, 26, 0.3+0.05*float64(i%4), 0.5, 0.6, 1)
		shell.Body.Place(c, 540+float64(i)*52, 400)
	}
	// Sibling labels (isolation visual).
	S.Place(wrkit.Label("sib", 11, 0.75, 0.85, 0.9), 5, 140)
	L2.Place(rendering.NewRenderColorBox(26, 26, 0.5, 0.35, 0.4, 1), 170, 170)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// R12b integrated: debug overlay on live repaints (magenta flash).
	app.SetDebugRepaint(true)

	clock := wrkit.NewPhaseClock(2, 5)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			// Inner-only dirty: hot + hotS flash; outer levels stay cached.
			hot.R = 0.6 + 0.3*float64(int(clock.Elapsed())%2)
			hot.MarkNeedsPaint()
			hotS.G = 0.6 + 0.2*float64(int(clock.Elapsed())%2)
			hotS.MarkNeedsPaint()
		case wrkit.PhaseSpike:
			// Outer-level dirty: L1 re-records whole nest; S sibling stays.
			L1.Background = &rendering.Color{R: 0.3 + 0.2*float64(int(clock.Elapsed())%2), G: 0.18, B: 0.24, A: 1}
			L1.MarkNeedsPaint()
		default:
			// Recover: pure Replay — skip accumulates, no flash.
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (skip/rerecord + flash draws prove the composite live).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		drawsH := app.DebugRepaintDraws()
		gateOK := snapH.BoundarySkip >= 3 && snapH.BoundaryRerecord >= 2 && drawsH >= 1
		shell.UpdateHUD("C1", phase, app, gateOK,
			fmt.Sprintf("%s draws=%d", wrkit.FmtSkip(snapH.BoundarySkip, snapH.BoundaryRerecord), drawsH), "")
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
	wrkit.MergeBoundaryCache(app, &snap)
	draws := app.DebugRepaintDraws()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C1",
		Scenario:      "ui_wr_c1_boundary_nest",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"nest_depth":   4,
			"debug_draws":  draws,
			"phases_seen":  clock.Name(),
			"static_dense": 16,
		},
	})
	report.DebugRepaintDraws = draws
	report.DebugRepaintOn = true
	out, _ := json.Marshal(report)
	fmt.Println(string(out))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		MinBoundarySkip:        3,
		MinBoundaryRerecord:    2,
		MinBoundaryCount:       3,
		MinBoundaryMaxDepth:    2,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	if draws < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: debug_draws=0 (R12b overlay absent in composite)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: OK skip=%d rr=%d count=%d depth=%d draws=%d presents=%d\n",
		snap.BoundarySkip, snap.BoundaryRerecord, snap.BoundaryCount, snap.BoundaryMaxDepth, draws, app.PresentCount())
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
