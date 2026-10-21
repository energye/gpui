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
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

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
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R18")
		if secs < 10 {
			fmt.Fprintln(os.Stderr, "FAIL: R18 close duration must be >=10s (§2.5)")
			os.Exit(1)
		}
	} else if *autoOnly {
		secs, secsSet = 10, true
	}
	gateMode := secsSet
	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	render.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r18_savelayer — SaveLayer 预算", Resizable: true, Decorations: true})
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

	snapDir := filepath.Join("examples", "ui_wr_r18_savelayer", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_r18_savelayer.png")

	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		// Per-frame SaveLayer budget: exactly one group may composite offscreen.
		SaveLayerMaxOps: 1,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: close (%s)\n", win.Kind())
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
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

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
		// Gate mode keeps it hidden so the 10s snapshot is deterministic and
		// the allow/reject pair stays exactly one allowed + one rejected.
		wantC := !gateMode && elapsed >= 5.0 && elapsed < 8.0
		if wantC != groupCVisible {
			groupCVisible = wantC
			groupC.MarkNeedsPaint()
		}

		// Groups re-record every frame so each frame re-requests SaveLayer.
		groupA.MarkNeedsPaint()
		groupB.MarkNeedsPaint()

		// HOT + banner are animated in manual mode; gate mode freezes both so
		// the Golden covers the whole body deterministically. HUD stays live.
		al, rj := app.SaveLayerStats()
		if gateMode {
			hot.R, hot.G, hot.B = 0.95, 0.30, 0.25
			slBanner.SetText("SL: gate frozen (HUD live)")
		} else {
			hotHue := float64(int(elapsed*8)%16) / 16
			hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
			slBanner.SetText(fmt.Sprintf("SL: allow=%d reject=%d (MaxOps=1)", al, rj))
		}
		hot.MarkNeedsPaint()
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

	// R18 evidence origins (layout-driven Align; same arithmetic as the scene).
	bx, by := shell.Body.X, shell.Body.Y
	bodyW, bodyH := shell.Body.W, shell.Body.H
	aox := bx + (bodyW-190)*0.04
	aoy := by + (bodyH-120)*0.06
	box := bx + (bodyW-190)*0.04
	boy := by + (bodyH-120)*0.30
	cox := bx + (bodyW-190)*0.28
	coy := by + (bodyH-120)*0.06
	dox := bx + (bodyW-330)*0.55
	doy := by + (bodyH-240)*0.05
	hox := bx + (bodyW-26)*0.88
	hoy := by + (bodyH-26)*0.05

	// Logic evidence (always): allow/reject counters and group count.
	logicProbes := []string{
		"savelayer_allow_ge_1",
		"savelayer_reject_ge_1",
		"savelayer_groups_2",
		"presents_positive",
		"paint_count_positive",
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if al < 1 {
		logicFailed = append(logicFailed, "savelayer_allow_ge_1")
	}
	if rj < 1 {
		logicFailed = append(logicFailed, "savelayer_reject_ge_1")
	}
	if groupCVisible {
		// Gate mode keeps group C hidden; manual Spike may show it.
		if gateMode {
			logicFailed = append(logicFailed, "savelayer_groups_2")
		}
	}
	if app.PresentCount() < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if snap.PaintCount <= 0 {
		logicFailed = append(logicFailed, "paint_count_positive")
	}
	logicPass := len(logicFailed) == 0

	// Pixel + Golden evidence only in gate mode (manual animates HOT/banner/C).
	var pixelOK, pixelTotal int
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	if gateMode {
		pixelChecks := []pixelCheck{
			// Group A is allowed: 0.55 group opacity over body BG. Raw red
			// would be 0.85; measured ~0.514 proves offscreen compositing.
			{desc: "groupA_red_group_alpha", x: aox + 30, y: aoy + 30, want: [3]float64{0.514, 0.212, 0.212}, tol: 0.03},
			// Group A blue rect alpha=0.6 inside 0.55 group → ~0.33 over BG.
			{desc: "groupA_blue_group_alpha", x: aox + 140, y: aoy + 50, want: [3]float64{0.159, 0.232, 0.376}, tol: 0.03},
			// Group B is refused: draws direct, no 0.55 group alpha.
			{desc: "groupB_green_raw", x: box + 30, y: boy + 30, want: [3]float64{0.35, 0.65, 0.55}, tol: 0.03},
			// Visibly drawn refusal frame (red, 6px).
			{desc: "groupB_reject_frame", x: box + 3, y: boy + 60, want: [3]float64{1.0, 0.25, 0.25}, tol: 0.03},
			// Group C must be absent in the 10s gate snapshot.
			{desc: "groupC_absent_bg", x: cox + 30, y: coy + 30, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.03, kind: kindBG},
			{desc: "dense_grid", x: dox + 39, y: doy + 25, want: [3]float64{0.25, 0.40, 0.62}, tol: 0.03},
			{desc: "hot_frozen", x: hox + 13, y: hoy + 13, want: [3]float64{0.95, 0.30, 0.25}, tol: 0.03},
			{desc: "body_bg", x: bx + 400, y: by + 600, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
			{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
			{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
		}
		img := loadImage(snapPath)
		pixelResult := make(map[string]bool, len(pixelChecks))
		runPixelChecks(img, pixelChecks, pixelResult)
		for _, ok := range pixelResult {
			if ok {
				pixelOK++
			}
		}
		pixelTotal = len(pixelChecks)

		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R18",
		Scenario:      "ui_wr_r18_savelayer",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"savelayer_allow":        al,
			"savelayer_reject":       rj,
			"savelayer_groups":       2,
			"savelayer_spike_group":  groupCVisible,
			"save_budget":            "MaxOps=1 per frame",
			"static_dense_cells":     16,
			"static_labels":          8,
			"phase_mode":             "steady-frozen-hot-banner",
			"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     pixelTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"fps_gate":               "off_correctness",
			"slope_gate":             "off",
			"logic_probes_ok":        len(logicProbes) - len(logicFailed),
			"logic_probes_total":     len(logicProbes),
			"logic_bad":              logicFailed,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if !logicPass {
		fail("logic probes %d/%d bad=%v", len(logicProbes)-len(logicFailed), len(logicProbes), logicFailed)
	}
	if gateMode {
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok", pixelOK, pixelTotal)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
		}); err != nil {
			fail("%v", err)
		}
		if al < 1 {
			fail("savelayer_allow=%d want >=1 (first group must composite offscreen)", al)
		}
		if rj < 1 {
			fail("savelayer_reject=%d want >=1 (second group must be budget-refused)", rj)
		}
		if snap.VSyncSource == "" {
			fail("vsync_source missing")
		}
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: OK allow=%d reject=%d presents=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		al, rj, app.PresentCount(),
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
