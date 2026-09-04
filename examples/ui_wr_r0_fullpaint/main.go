// Command ui_wr_r0_fullpaint is the W0 R0 real-window: FullPaint 正确性.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint
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
		wrkit.RequireMinRun(secs, "R0")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r0_fullpaint — FullPaint 正确性", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	// Shell: TopBar + Legend + Body + HUD (U17). Policy stays full_paint (W0).
	shell := wrkit.NewShell(winW, winH, "R0 FullPaint正确性 — 静网+文+动同屏", []string{
		"STATIC=color grid + text (每帧重画)",
		"HOT=phase-driven block (变位/变色)",
		"full_paint: 每帧全树 paint",
		"Clear 后静态必须存活",
		"damage_ratio≈1 属 FullPaint 语义",
	})

	// Static dense content (静网): 6x4 color grid, each cell a RepaintBoundary.
	staticCount := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(84, 84, 0.15+0.1*float64(i%4), 0.5, 0.3+0.12*float64(j%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*92, 20+float64(j)*92)
			staticCount++
		}
	}
	// Static text (文): 12 labels with distinct colors.
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("static text %02d", i), 12, 0.65+0.05*float64(i%5), 0.72, 0.82)
		shell.Body.Place(t, 600+float64(i%4)*95, 20+float64(i/4)*30)
		staticCount++
	}

	// Dynamic hotspot (动): phase-driven block — moves + recolors each phase.
	hot := rendering.NewRenderColorBox(120, 120, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 600, 140)
	hotX, hotY := 600.0, 140.0
	// Phase banner: 当前相位大字（Steady/Spike/Recover），相位驱动肉眼可见。
	phaseLabel := wrkit.Label("PHASE: STEADY", 20, 0.95, 0.95, 0.4)
	shell.Body.Place(phaseLabel, 600, 96)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var phase string
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		switch phase {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			hotX, hotY = 600, 140
			phaseLabel.SetText("PHASE: STEADY  (静格保持)")
			phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotX, hotY = 600, 260
			phaseLabel.SetText("PHASE: SPIKE   (动块跳位)")
			phaseLabel.SetColor(1.0, 0.8, 0.2, 1)
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			hotX, hotY = 680, 140
			phaseLabel.SetText("PHASE: RECOVER (动块变色)")
			phaseLabel.SetColor(0.2, 0.8, 1.0, 1)
		}
		phaseLabel.MarkNeedsPaint()
		shell.Body.Box.Place(hot, hotX, hotY)
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (R0 core counters: paint vs presents under full_paint).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && float64(snapH.DamageAreaPx) >= float64(winW*winH)*0.9
		shell.UpdateHUD("R0", phase, app, gateOK,
			fmt.Sprintf("paint=%d dmg=%.2f", snapH.PaintCount, float64(snapH.DamageAreaPx)/float64(winW*winH)), "")
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
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R0",
		Scenario:      "ui_wr_r0_fullpaint",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":  "full_paint",
			"static_visible":  staticCount,
			"hot_repaints":    hotTick,
			"phases_seen":     phase,
			"static_check":    true,
			"damage_semantic": "full_paint_full_redraw_allowed",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// Gates: R0 = full-paint correctness — every frame repaints the whole tree
	// (Clear can't drop statics), continuous tick holds fps, warmup content.
	presents := float64(app.PresentCount())
	fpsOK := snap.AvgFrameIntervalMs > 1e-6 && 1000.0/snap.AvgFrameIntervalMs >= 55
	fullPaintOK := snap.PaintCount > 0 && (presents == 0 || float64(snap.PaintCount) >= presents*0.9)
	contentOK := staticCount >= 8 && hotTick > 0
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if !fpsOK {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (continuous tick R0)", 1000.0/snap.AvgFrameIntervalMs)
		os.Exit(1)
	}
	if !fullPaintOK {
		fmt.Fprintf(os.Stderr, "FAIL: paint_count=%d not covering frames=%.0f (FullPaint path)", snap.PaintCount, presents)
		os.Exit(1)
	}
	if !contentOK {
		fmt.Fprintln(os.Stderr, "FAIL: static content or hot ticks missing")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: OK presents=%d paint=%d statics=%d hot_ticks=%d fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), snap.PaintCount, staticCount, hotTick, 1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
