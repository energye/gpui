// Command ui_wr_r2_paint is the W1 R2 real-window: 局部 NeedsPaint isolation.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

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
		wrkit.RequireMinRun(secs, "R2")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r2_paint — 局部 NeedsPaint", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	// Shell: TopBar + Legend + Body + HUD (U17).
	shell := wrkit.NewShell(winW, winH, "R2 局部NeedsPaint — 9宫静+1热", []string{
		"GREEN=static boundary (never repaints)",
		"MAGENTA=hot boundary (tick-driven)",
		"BLUE=static plain nodes",
		"paint_count low ⇒ isolation OK",
		"visits≈hot subtree only",
	})

	// Static dense content: 4x4 color grid + 8 labels, each a RepaintBoundary (9-grid).
	staticB := make([]*rendering.RenderColorBox, 0, 24)
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(90, 90, 0.12+0.18*float64(i%3), 0.55, 0.28+0.2*float64(j%2), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*110, 20+float64(j)*110)
			staticB = append(staticB, c)
		}
	}
	// Second static grid (non-boundary, dense color + text mix): proves plain
	// nodes also stay unpainted under CompositeOnly.
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(36, 36, 0.18+0.15*float64(j), 0.45, 0.55+0.1*float64(i), 1)
			shell.Body.Place(c, 460+float64(i)*46, 400+float64(j)*46)
		}
	}
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("static label %d", i), 12, 0.7, 0.75, 0.85)
		shell.Body.Place(t, 20+float64(i%4)*95, 380+float64(i/4)*30)
	}

	// Hot boundary: repaints at ticker pace; static siblings must not.
	hot := rendering.NewRenderColorBox(90, 90, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 500, 20)

	// Second hot with different rate (phase-dependent).
	hot2 := rendering.NewRenderColorBox(60, 60, 0.95, 0.6, 0.2, 1)
	hot2.SetRepaintBoundary(true)
	shell.Body.Place(hot2, 620, 300)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	var phase string
	var r, g, b float64
	clock := wrkit.NewPhaseClock(1.5, 3.5)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			r, g, b = 0.95, 0.2, 0.2
		case wrkit.PhaseSpike:
			r, g, b = 1.0, 0.85, 0.2
		default:
			r, g, b = 0.2, 0.7, 0.95
		}
		hot.R, hot.G, hot.B = r, g, b
		hot.MarkNeedsPaint()
		if phase == wrkit.PhaseSpike {
			hot2.R, hot2.G, hot2.B = r*0.9, g*0.9, b*0.9
			hot2.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD refresh (ability ID + phase + fps + core counters).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && (app.PresentCount() == 0 || snapH.PaintCount <= app.PresentCount()+20)
		shell.UpdateHUD("R2", phase, app, gateOK,
			fmt.Sprintf("visits=%d skip=%d", snapH.PaintVisits, snapH.BoundarySkip), "")
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
	// R2 gate: paint_count must not equal frames — static boundaries skip.
	frames := float64(app.PresentCount())
	okPaintCount := snap.PaintCount > 0 && (frames == 0 || float64(snap.PaintCount) <= frames+20)
	visits := int64(0)
	if b, ok := os.LookupEnv("WR_VISITS"); ok && b == "1" {
		visits = snap.PaintVisits
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R2",
		Scenario:      "ui_wr_r2_paint",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"paint_visits":      visits,
			"static_boundaries": len(staticB),
			"phases_seen":       phase,
		},
	})
	report.PaintCount = snap.PaintCount
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// Gates: R2 = paint_count visible + static not repainting (HUD paint vs hot).
	if !okPaintCount {
		fmt.Fprintf(os.Stderr, "FAIL: paint_count=%d exceeds frames+20 (static boundaries repainting)", snap.PaintCount)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: OK presents=%d paint_flushes=%d visits=%d elapsed=%.1fs\n",
		app.PresentCount(), snap.PaintCount, snap.PaintVisits, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
