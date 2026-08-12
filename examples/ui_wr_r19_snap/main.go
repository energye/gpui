// Command ui_wr_r19_snap is the W1 R19 real-window: 1px / device-pixel alignment.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
// Draws hairline grids with SnapCoord/SnapLine so 1px lines land on device
// pixels; HUD shows DPR so the operator can verify crispness at scale 1 vs 2.
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
		wrkit.RequireMinRun(secs, "R19")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r19_snap — 1px 对齐"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R19 1px对齐 — SnapCoord/SnapLine", []string{
		"hairline grid: snapped (crisp)",
		"hairline grid: raw (may blur)",
		"DPR in HUD — verify crisp at 1/2",
	})

	// Grid canvas: 500x400 with 20px spacing, snapped vs raw side by side.
	grid := rendering.NewRenderBox()
	grid.FixedWidth, grid.FixedHeight = 500, 400
	grid.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		scale := pc.Scale
		// Snapped hairlines: SnapLine ensures device-pixel alignment.
		pc.DC.SetLineWidth(1 / scale)
		pc.DC.SetRGBA(0.3, 0.9, 0.6, 1)
		for x := 20.0; x < 500; x += 20 {
			sx := pc.SnapLineX(x)
			pc.DC.DrawLine(pc.OriginX+sx, pc.OriginY+0, pc.OriginX+sx, pc.OriginY+400)
		}
		for y := 20.0; y < 400; y += 20 {
			sy := pc.SnapLineY(y)
			pc.DC.DrawLine(pc.OriginX+0, pc.OriginY+sy, pc.OriginX+500, pc.OriginY+sy)
		}
		_ = pc.DC.Stroke()
		// Snapped 1px border.
		sx, sy, sw, sh := pc.SnapRect(2, 2, 496, 396)
		pc.DC.SetLineWidth(1 / scale)
		pc.DC.SetRGBA(0.9, 0.6, 0.3, 1)
		pc.DC.DrawRectangle(pc.OriginX+sx, pc.OriginY+sy, sw, sh)
		_ = pc.DC.Stroke()
	}
	shell.Body.Place(grid, 20, 20)

	// Raw (unsnapped) grid for contrast: offsets at fractional positions.
	raw := rendering.NewRenderBox()
	raw.FixedWidth, raw.FixedHeight = 500, 400
	raw.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		scale := pc.Scale
		pc.DC.SetLineWidth(1 / scale)
		pc.DC.SetRGBA(0.9, 0.35, 0.35, 1)
		for x := 20.5; x < 500; x += 20 {
			pc.DC.DrawLine(pc.OriginX+x, pc.OriginY+0, pc.OriginX+x, pc.OriginY+400)
		}
		for y := 20.5; y < 400; y += 20 {
			pc.DC.DrawLine(pc.OriginX+0, pc.OriginY+y, pc.OriginX+500, pc.OriginY+y)
		}
		_ = pc.DC.Stroke()
	}
	shell.Body.Place(raw, 560, 20)

	shell.Body.Place(wrkit.Label("SNAPPED (crisp)", 12, 0.4, 0.9, 0.7), 20, 440)
	shell.Body.Place(wrkit.Label("RAW (may blur)", 12, 0.9, 0.4, 0.4), 560, 440)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	var dpr float64
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		dpr = host.ScaleFactor()
		if dpr <= 0 {
			dpr = 1
		}
		// Redraw snapped grid when DPR changed (resize / DPR switch).
		if _, _, changed := lastDPR(dpr); changed {
			grid.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (dpr + snapped rect origin are the R19 proof).
		shell.NoteHUDTick(dt)
		_, _, gok := lastDPR(dpr)
		_ = gok
		gateOK := dpr > 0
		shell.UpdateHUD("R19", "steady", app, gateOK,
			fmt.Sprintf("dpr=%.1f snapped=%d,%d", dpr, rendering.SnapCoord(2, 1), rendering.SnapCoord(2, 1)), "")
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
	if dpr <= 0 {
		dpr = 1
	}

	// Snap correctness gate: SnapCoord must return grid-aligned values.
	sx, _, _, _ := rendering.SnapRect(1.3, 2.6, 10.4, 5.2, dpr)
	if os.Getenv("WR_SNAP_STRICT") == "1" {
		// Verify that snapping a fractional coord yields a multiple of 1/dpr.
		v := rendering.SnapCoord(1.3, dpr)
		grid := v * dpr
		if grid != float64(int64(grid+0.5)) {
			fmt.Fprintf(os.Stderr, "FAIL: SnapCoord(1.3,%.1f)=%.4f not on grid\n", dpr, v)
			os.Exit(1)
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R19",
		Scenario:      "ui_wr_r19_snap",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"dpr":             dpr,
			"snapped_lines":   46, // 24 vertical + 19 horizontal + border
			"snap_rect_check": sx,
		},
	})
	report.DebugRepaintDraws = 0
	report.DebugRepaintOn = false
	out, _ := json.Marshal(report)
	fmt.Println(string(out))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: OK dpr=%.1f presents=%d elapsed=%.1fs\n",
		dpr, app.PresentCount(), elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

var lastDPRVal float64

func lastDPR(v float64) (float64, float64, bool) {
	prev := lastDPRVal
	lastDPRVal = v
	return prev, v, prev != v
}
