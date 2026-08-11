// Command ui_wr_r18_savelayer is the W2 R18 real-window: SaveLayer + budget.
// Two OnPaint groups each request a SaveLayer every frame under a per-frame
// SaveLayerBudget{MaxOps:1}: the first is allowed (offscreen compositing with
// group opacity), the second is refused — the refusal is drawn visibly
// (red "BUDGET REJECT" frame) and counted in savelayer_reject.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var proc scheduler.ProcessTracker

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R18")
	}
	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	render.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r18_savelayer — SaveLayer 预算"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R18 SaveLayer+预算 — 超预算拒批可视化", []string{
		"A   = 组1 SaveLayer 允许 (半透明合成)",
		"B   = 组2 SaveLayer 拒批 (红框)",
		"C   = 组3 (Spike 相出现, 追加拒批)",
		"D   = 静态密集 4x4+8标签",
		"HOT = 每帧动画热点",
		"budget = MaxOps=1/帧",
	})

	// DBG MARKERS: absolute root coords, pure colors — calibrate xwd mapping.
	mk := func(x, y float64, r, g, b float64) {
		c := rendering.NewRenderColorBox(10, 10, r, g, b, 1)
		shell.Root.Place(c, x, y)
	}
	mk(0, 0, 1, 0, 1)      // magenta at root (0,0)
	mk(600, 400, 1, 1, 0)  // yellow at (600,400)
	mk(1190, 790, 0, 1, 1) // cyan at (1190,790)

	drawText := func(pc *rendering.PaintContext, x, y float64, msg string, r, g, b float64) {
		if pc == nil || pc.DC == nil {
			return
		}
		if face := wrkit.FaceAt(12); face != nil {
			pc.DC.SetFont(face)
		}
		pc.DC.SetRGBA(r, g, b, 1)
		pc.DC.DrawString(msg, pc.OriginX+x, pc.OriginY+y)
	}

	// Group A: first SaveLayer every frame — allowed by MaxOps=1.
	groupA := rendering.NewRenderBox()
	groupA.FixedWidth, groupA.FixedHeight = 190, 120
	groupA.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		ok := pc.SaveLayer(size.Width, size.Height, 0.55)
		rendering.FillRect(pc, 10, 10, 90, 70, 0.85, 0.30, 0.28, 1)
		rendering.FillRect(pc, 60, 40, 90, 70, 0.28, 0.45, 0.88, 0.6)
		drawText(pc, 12, 96, "GROUP-A allowed (合成)", 0.95, 0.95, 0.98)
		pc.RestoreLayer()
		if !ok {
			drawText(pc, 12, 18, "!!REJECT!!", 1, 0.3, 0.3)
		}
	}
	shell.Body.Align(groupA, 0.04, 0.06)

	// Group B: second SaveLayer every frame — refused (budget exhausted).
	groupB := rendering.NewRenderBox()
	groupB.FixedWidth, groupB.FixedHeight = 190, 120
	groupB.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		ok := pc.SaveLayer(size.Width, size.Height, 0.55)
		rendering.FillRect(pc, 10, 10, 90, 70, 0.35, 0.65, 0.55, 1)
		rendering.FillRect(pc, 60, 40, 90, 70, 0.95, 0.85, 0.35, 0.6)
		if !ok {
			// Refused: no offscreen group — draw the refusal visibly.
			rendering.FillRect(pc, 0, 0, 6, size.Height, 1, 0.25, 0.25, 1)
			rendering.FillRect(pc, 0, 0, size.Width, 6, 1, 0.25, 0.25, 1)
			rendering.FillRect(pc, size.Width-6, 0, 6, size.Height, 1, 0.25, 0.25, 1)
			rendering.FillRect(pc, 0, size.Height-6, size.Width, 6, 1, 0.25, 0.25, 1)
			drawText(pc, 12, 96, "GROUP-B BUDGET REJECT", 1, 0.3, 0.3)
		} else {
			drawText(pc, 12, 96, "GROUP-B allowed", 0.95, 0.95, 0.98)
		}
		pc.RestoreLayer()
	}
	shell.Body.Align(groupB, 0.04, 0.30)

	// Group C: appears in the Spike phase — adds a third per-frame SaveLayer
	// request so reject accumulates visibly faster than allow (HUD).
	groupC := rendering.NewRenderBox()
	groupC.FixedWidth, groupC.FixedHeight = 190, 120
	groupCVisible := false
	groupC.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if !groupCVisible {
			return
		}
		ok := pc.SaveLayer(size.Width, size.Height, 0.5)
		rendering.FillRect(pc, 10, 10, 90, 70, 0.72, 0.35, 0.82, 1)
		if !ok {
			drawText(pc, 12, 96, "GROUP-C REJECT (spike)", 1, 0.3, 0.3)
		} else {
			drawText(pc, 12, 96, "GROUP-C allowed", 0.95, 0.95, 0.98)
		}
		pc.RestoreLayer()
	}
	shell.Body.Align(groupC, 0.28, 0.06)

	// Static dense content: 4x4 color grid + 8 labels (U17).
	denseD := rendering.NewAbsoluteBox(330, 240)
	denseD.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.21, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			denseD.Place(c, 10+float64(i)*66, 10+float64(j)*40)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		denseD.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*66, 178+float64(row)*22)
	}
	shell.Body.Align(denseD, 0.55, 0.05)

	// HOT spot: dirties itself every frame (live paint, non-boundary).
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.88, 0.05)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.88, 0.16)

	// Live SaveLayer counter inside the body.
	slBanner := wrkit.Label("SL: allow=0 reject=0", 13, 0.95, 0.90, 0.55)
	shell.Body.Align(slBanner, 0.28, 0.30)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		// Per-frame SaveLayer budget: exactly one group may composite offscreen.
		SaveLayerMaxOps: 1,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (same wiring as ui_wr_r4b_multidamage). Without this the
				// content stays at the initial 1200x800 layout while the
				// surface clears at the new size (observed: right/bottom slack).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	var elapsed float64
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt

		phase := wrkit.PhaseSteady
		switch {
		case elapsed >= 5.0 && elapsed < 8.0:
			phase = "Spike"
		case elapsed >= 8.0:
			phase = wrkit.PhaseRecover
		}

		// Spike: group C joins — reject accumulates 2/frame vs allow 1/frame.
		wantC := elapsed >= 5.0 && elapsed < 8.0
		if wantC != groupCVisible {
			groupCVisible = wantC
			groupC.MarkNeedsPaint()
		}

		// Groups re-record every frame so each frame re-requests SaveLayer.
		groupA.MarkNeedsPaint()
		groupB.MarkNeedsPaint()

		// HOT spot repaints itself every frame.
		hotHue := float64(int(elapsed*8)%16) / 16
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		al, rj := app.SaveLayerStats()
		slBanner.SetText(fmt.Sprintf("SL: allow=%d reject=%d (MaxOps=1)", al, rj))
		slBanner.MarkNeedsPaint()

		if os.Getenv("GOMEM_DIAG") != "" && int(elapsed)%5 == 0 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			slog.Debug("mem_diag", "t", int(elapsed),
				"heap_alloc_kb", m.HeapAlloc/1024, "heap_objs", m.HeapObjects, "heap_inuse_kb", m.HeapInuse/1024)
		}

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := al >= 1 && rj >= 1
		shell.UpdateHUD("R18", phase, app, gateOK,
			fmt.Sprintf("allow=%d reject=%d", al, rj),
			fmt.Sprintf("spike=%v t=%.1f", wantC, elapsed))
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

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	al, rj := app.SaveLayerStats()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R18",
		Scenario:      "ui_wr_r18_savelayer",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"savelayer_allow":       al,
			"savelayer_reject":      rj,
			"savelayer_groups":      2,
			"savelayer_spike_group": groupCVisible,
			"save_budget":           "MaxOps=1 per frame",
			"static_dense_cells":    16,
			"static_labels":         8,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R18 gates (§2 主表: savelayer_allow>=1 + savelayer_reject>=1).
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if al < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: savelayer_allow=%d want >=1 (first group must composite offscreen)\n", al)
		os.Exit(1)
	}
	if rj < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: savelayer_reject=%d want >=1 (second group must be budget-refused)\n", rj)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: OK allow=%d reject=%d presents=%d elapsed=%.1fs\n",
		al, rj, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
