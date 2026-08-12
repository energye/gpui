// Command ui_wr_r3_boundary is the W1 R3 real-window: nested RepaintBoundary cache.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary
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
		wrkit.RequireMinRun(secs, "R3")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r3_boundary — 嵌套 Boundary 缓存"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R3 嵌套Boundary缓存 — 内脏不外溢", []string{
		"WHITE outline=nested boundary",
		"INNER dirty → only inner rerecord",
		"OUTER dirty → only outer rerecord",
		"skip↑ on clean frames (Replay)",
		"static label inside boundary",
	})

	// Boundary nesting: outer(240x240) → mid(160x160) → inner(80x80).
	// Each level holds static labels + a color block; inner holds the hot cell.
	outer := rendering.NewAbsoluteBox(240, 240)
	outer.SetRepaintBoundary(true)
	shell.Body.Place(outer, 20, 20)
	outer.SetDebugName("outer")

	mid := rendering.NewAbsoluteBox(180, 180)
	mid.SetRepaintBoundary(true)
	outer.Place(mid, 30, 30)

	inner := rendering.NewAbsoluteBox(120, 120)
	inner.SetRepaintBoundary(true)
	mid.Place(inner, 30, 30)

	hot := rendering.NewRenderColorBox(60, 60, 0.95, 0.2, 0.2, 1)
	inner.Place(hot, 30, 30)

	// Static labels at each nesting level (mixed content: text + color).
	for i := 0; i < 3; i++ {
		t := wrkit.Label(fmt.Sprintf("L%d static text", i+1), 11, 0.72, 0.78, 0.9)
		outer.Place(t, 5, 205+float64(i)*0)
	}
	tIn := wrkit.Label("inner label", 11, 0.8, 0.85, 0.95)
	inner.Place(tIn, 5, 5)
	tMid := wrkit.Label("mid label", 11, 0.7, 0.8, 0.9)
	mid.Place(tMid, 5, 150)
	// Static blocks inside each boundary level (color mixed with text).
	outer.Place(rendering.NewRenderColorBox(40, 40, 0.4, 0.5, 0.6, 1), 190, 190)
	mid.Place(rendering.NewRenderColorBox(30, 30, 0.5, 0.4, 0.6, 1), 140, 140)
	inner.Place(rendering.NewRenderColorBox(20, 20, 0.6, 0.4, 0.5, 1), 90, 90)

	// Second independent nest (sibling of outer): proves isolation between nests.
	outer2 := rendering.NewAbsoluteBox(180, 180)
	outer2.SetRepaintBoundary(true)
	shell.Body.Place(outer2, 300, 20)
	inner2 := rendering.NewAbsoluteBox(100, 100)
	inner2.SetRepaintBoundary(true)
	outer2.Place(inner2, 40, 40)
	hot2 := rendering.NewRenderColorBox(50, 50, 0.2, 0.6, 0.95, 1)
	inner2.Place(hot2, 25, 25)
	outer2.Place(wrkit.Label("nest2 label", 11, 0.7, 0.8, 0.9), 5, 160)
	// Third deep chain (depth 4): boundary_count/depth evidence.
	deep := rendering.NewAbsoluteBox(200, 200)
	deep.SetRepaintBoundary(true)
	shell.Body.Place(deep, 520, 300)
	d2 := rendering.NewAbsoluteBox(150, 150)
	d2.SetRepaintBoundary(true)
	deep.Place(d2, 25, 25)
	d3 := rendering.NewAbsoluteBox(100, 100)
	d3.SetRepaintBoundary(true)
	d2.Place(d3, 25, 25)
	deep.Place(wrkit.Label("deep label", 11, 0.8, 0.85, 0.95), 5, 185)

	// Static dense 4x4 grid outside boundaries (content mix).
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(40, 40, 0.3, 0.55, 0.75, 1)
			shell.Body.Place(c, 520+float64(i)*55, 20+float64(j)*55)
		}
	}
	// Second dense block: text labels outside boundaries (non-cacheable mix).
	for i := 0; i < 8; i++ {
		t := wrkit.Label(fmt.Sprintf("grid label %d", i), 11, 0.65, 0.72, 0.85)
		shell.Body.Place(t, 20+float64(i%4)*80, 300+float64(i/4)*40)
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	clock := wrkit.NewPhaseClock(2, 5)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			// inner-only dirty: hot cell in nest1 + hot2 in nest2.
			hot.R = 0.95 - 0.3*(clock.Elapsed()/2)
			hot.MarkNeedsPaint()
			hot2.B = 0.95 - 0.3*(clock.Elapsed()/2)
			hot2.MarkNeedsPaint()
		case wrkit.PhaseSpike:
			// outer-level dirty: whole nest1 boundary re-records (but nest2 stays).
			outer.Background = &rendering.Color{R: 0.35 + 0.2*float64(int(clock.Elapsed())%2), G: 0.2, B: 0.3, A: 1}
			outer.MarkNeedsPaint()
		default:
			// Recover: no dirty → pure Replay frames (skip accumulates).
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (skip / rerecord are the R3 proof on screen).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		gateOK := snapH.BoundarySkip >= 3 && snapH.BoundaryRerecord >= 2
		shell.UpdateHUD("R3", phase, app, gateOK,
			wrkit.FmtSkip(snapH.BoundarySkip, snapH.BoundaryRerecord), "")
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3",
		Scenario:      "ui_wr_r3_boundary",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"nest_depth":     3,
			"static_labels":  4,
			"phases_seen":    clock.Name(),
			"skip_to_clean":  snap.BoundarySkip,
			"rerecord_dirty": snap.BoundaryRerecord,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		MinBoundarySkip:        3,
		MinBoundaryRerecord:    2,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: OK presents=%d skip=%d rerecord=%d elapsed=%.1fs\n",
		app.PresentCount(), snap.BoundarySkip, snap.BoundaryRerecord, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
