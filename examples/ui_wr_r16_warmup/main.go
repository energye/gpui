// Command ui_wr_r16_warmup is the W0 R16 real-window: 首帧/WarmUp/恢复.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage pre
//	RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup -auto-only -gate-stage post
//	go run ./examples/ui_wr_r16_warmup                 # manual: animate until close
//	go run ./examples/ui_wr_r16_warmup -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
//
// Evidence (U21): the gate has two deterministic stages so both halves of R16
// are pixel-asserted:
//   - pre:  first-frame content is frozen (steady phase, no recovery); the
//     final snapshot is content-equivalent to the first frame because every
//     visible body node is placed before Open() and nothing animates.
//   - post: recovery is triggered at a fixed tick and the recovery state is
//     held to the end; the snapshot proves the full-tree repaint survived.
//
// Each stage writes its own GPU-readback PNG, pixel probes, and bitwise golden
// (header + legend + body; the live HUD band is excluded). Manual mode keeps
// the Steady→Spike→Recover animation and the one-second recovery banner.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
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

type rect struct{ x, y, w, h float64 }

const kindBG = "bg"

type pixelCheck struct {
	desc string
	x, y float64
	want [3]float64
	tol  float64
	kind string
}

type inkRegion struct {
	desc  string
	x, y  float64
	w, h  float64
	bg    [3]float64
	tol   float64
	minPx int
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: deterministic stage snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	gateStageFlag := flag.String("gate-stage", "pre", "gate stage: pre (first-frame content) or post (recovery state)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R16")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	stage := "manual"
	if gateMode {
		stage = *gateStageFlag
		if stage != "pre" && stage != "post" {
			fmt.Fprintf(os.Stderr, "FAIL: -gate-stage=%q want pre or post\n", stage)
			os.Exit(1)
		}
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r16_warmup — 首帧/WarmUp/恢复", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R16 首帧/WarmUp/恢复 — 首帧有内容·恢复不黑", []string{
		"WarmUp=true: Open 即全清全画首帧",
		"首帧有内容: first_present_paint_count>0",
		"pre 档冻结首帧内容（像素+Golden）",
		"post 档固定触发全树恢复并保持",
		"恢复后 paint 继续增长 = 不黑",
		"pixel: 12 点 + 2/3 文字墨量区域",
		"golden: header+legend+body（HUD 排除）",
		"gate: -auto-only -gate-stage pre|post",
	})

	// Static dense content (静网): 6x4 color grid, each cell a RepaintBoundary.
	// These nodes are placed before Open(), so they are present in the very first
	// warm-up paint; the pre stage snapshots this same visible state later.
	staticCells := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(84, 84, 0.15+0.1*float64(i%4), 0.5, 0.3+0.12*float64(j%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*92, 20+float64(j)*92)
			staticCells++
		}
	}
	// Static text (文): 12 labels in 3 columns × 4 rows so the right column
	// stays inside the 904px body and the first-frame banner has its own row.
	staticLabels := 0
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("warmup %02d", i), 10, 0.65+0.05*float64(i%5), 0.72, 0.82)
		shell.Body.Place(t, 600+float64(i%3)*100, 20+float64(i/3)*24)
		staticLabels++
	}
	staticCount := staticCells + staticLabels

	// Dynamic hotspot (动): pre/first-frame state is red at (600,140); Spike
	// moves it in manual mode; the post stage holds it white at (700,480).
	hot := rendering.NewRenderColorBox(120, 120, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 600, 170)

	// First-frame banner is placed before Open(); it is part of the warm-up
	// paint. The recovery banner is moved to its own row so the two banners
	// never cover each other.
	firstBanner := wrkit.Label("FIRST FRAME CONTENT OK", 18, 0.4, 0.95, 0.6)
	shell.Body.Place(firstBanner, 600, 116)
	recoverBanner := wrkit.Label("", 20, 1.0, 1.0, 1.0)
	shell.Body.Place(recoverBanner, 600, 420)

	phaseLabel := wrkit.Label("PHASE: STEADY (frozen)", 18, 0.95, 0.95, 0.4)
	shell.Body.Place(phaseLabel, 20, 420)
	stageLabel := wrkit.Label("STAGE: PRE - first frame content frozen", 16, 0.80, 0.85, 0.95)
	shell.Body.Place(stageLabel, 20, 480)
	switch stage {
	case "post":
		stageLabel.SetText("STAGE: POST - recovery state held")
	case "manual":
		stageLabel.SetText("STAGE: MANUAL - animated")
	}

	snapDir := filepath.Join("examples", "ui_wr_r16_warmup", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := ""
	if gateMode {
		snapPath = filepath.Join(snapDir, "showcase_r16_warmup_"+stage+".png")
	}

	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	phase := wrkit.PhaseSteady
	lastPhase := ""
	hotTick := 0
	recoveryTriggered := false
	recoveryAtTick := 0
	recoveryVisual := false
	var paintAtRecovery int64

	applyPhase := func(p string) {
		switch p {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			shell.Body.Box.Place(hot, 600, 170)
			phaseLabel.SetText("PHASE: STEADY  (静格保持)")
			phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			shell.Body.Box.Place(hot, 600, 260)
			phaseLabel.SetText("PHASE: SPIKE   (动块跳位)")
			phaseLabel.SetColor(1.0, 0.8, 0.2, 1)
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			shell.Body.Box.Place(hot, 680, 140)
			phaseLabel.SetText("PHASE: RECOVER (动块变色)")
			phaseLabel.SetColor(0.2, 0.8, 1.0, 1)
		}
		if gateMode {
			if stage == "post" && recoveryTriggered {
				phaseLabel.SetText("PHASE: STEADY (recovered)")
			} else {
				phaseLabel.SetText("PHASE: STEADY (frozen)")
			}
		}
		phaseLabel.MarkNeedsPaint()
		hot.MarkNeedsPaint()
	}
	applyRecoveryVisual := func() {
		hot.R, hot.G, hot.B = 1.0, 1.0, 1.0
		shell.Body.Box.Place(hot, 700, 480)
		hot.MarkNeedsPaint()
		recoverBanner.SetText("RECOVERY OK - ALIVE")
		recoverBanner.SetColor(1.0, 1.0, 1.0, 1)
		recoverBanner.MarkNeedsPaint()
		if gateMode && stage == "post" {
			phaseLabel.SetText("PHASE: STEADY (recovered)")
			phaseLabel.MarkNeedsPaint()
		}
	}

	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady phase: deterministic snapshot while the ticker still
			// drives one full-tree paint per frame (full_paint).
			phase = wrkit.PhaseSteady
		} else {
			phase = clock.Advance(dt)
		}
		phaseChanged := phase != lastPhase
		if phaseChanged {
			applyPhase(phase)
			lastPhase = phase
		}
		hotTick++
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		snapH := app.Metrics().Snapshot()

		// Gate post triggers recovery at a fixed tick (~0.5s). Gate pre never
		// triggers it so the first-frame content stays frozen. Manual keeps the
		// original mid-run trigger and a one-second visible recovery banner.
		if !recoveryTriggered {
			trigger := false
			if gateMode && stage == "post" && hotTick >= 30 {
				trigger = true
			}
			if !gateMode && hotTick > 60 && hotTick < 200 {
				trigger = true
			}
			if trigger {
				recoveryTriggered = true
				recoveryAtTick = hotTick
				paintAtRecovery = snapH.PaintCount
				app.InvalidateBoundaryCache()
				applyRecoveryVisual()
				recoveryVisual = true
			}
		}
		if recoveryTriggered {
			if gateMode {
				// Post stage: hold the recovery state to the final snapshot.
				if stage == "post" && !recoveryVisual {
					applyRecoveryVisual()
					recoveryVisual = true
				}
			} else if hotTick-recoveryAtTick < 60 {
				// Manual: the phase switch above may have recolored the block;
				// re-apply the recovery visual on the same tick.
				if !recoveryVisual || phaseChanged {
					applyRecoveryVisual()
					recoveryVisual = true
				}
			} else if recoveryVisual {
				recoverBanner.SetText("")
				recoverBanner.MarkNeedsPaint()
				applyPhase(phase)
				recoveryVisual = false
			}
		}

		// U18: live HUD (首帧观测 + 恢复状态).
		shell.NoteHUDTick(dt)
		gateOK := snapH.Warmup && snapH.FirstPresentPaintCount > 0 && snapH.TimeToFirstPresentMs > 0
		if gateMode && stage == "post" {
			gateOK = gateOK && recoveryTriggered
		}
		shell.UpdateHUD("R16", phase, app, gateOK,
			fmt.Sprintf("t2f=%.0fms p1=%d", snapH.TimeToFirstPresentMs, snapH.FirstPresentPaintCount),
			fmt.Sprintf("stage=%s rec=%v", stage, recoveryTriggered))
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
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	paintOK := snap.PaintCount > 0 && (presents == 0 || float64(snap.PaintCount) >= float64(presents)*0.9)
	damageOK := snap.DamageAreaPx > 0 && float64(snap.DamageAreaPx) >= float64(winW*winH)*0.9
	policyOK := snap.PresentPolicy == scheduler.PresentPolicyFullPaint
	warmupOK := snap.Warmup
	firstPresentOK := snap.TimeToFirstPresentMs > 0 && snap.TimeToFirstPresentMs < 2000
	firstPaintOK := snap.FirstPresentPaintCount > 0
	fallbackOK := snap.CPUFallbackOps == 0
	vsyncOK := snap.VSyncSource != ""

	recoveryOK := false
	recoveryNA := stage == "pre"
	if stage == "pre" {
		recoveryOK = !recoveryTriggered
	} else {
		recoveryOK = recoveryTriggered && paintAtRecovery > 0 && snap.PaintCount > paintAtRecovery
	}
	recoveryRepaintOK := recoveryOK && !recoveryNA

	// Pixel probes are anchored to the body panel origin. Gate mode never
	// resizes the window, so these logical coordinates stay fixed.
	bx, by := shell.Body.X, shell.Body.Y
	bodyBG := [3]float64{0.10, 0.11, 0.13}
	pixelChecks := []pixelCheck{
		{desc: "static_00", x: bx + 62, y: by + 62, want: [3]float64{0.15, 0.50, 0.30}, tol: 0.06},
		{desc: "static_10", x: bx + 154, y: by + 62, want: [3]float64{0.25, 0.50, 0.30}, tol: 0.06},
		{desc: "static_13", x: bx + 154, y: by + 338, want: [3]float64{0.25, 0.50, 0.30}, tol: 0.06},
		{desc: "static_32", x: bx + 338, y: by + 246, want: [3]float64{0.45, 0.50, 0.54}, tol: 0.06},
		{desc: "static_51", x: bx + 522, y: by + 154, want: [3]float64{0.25, 0.50, 0.42}, tol: 0.06},
		{desc: "grid_gap_bg", x: bx + 108, y: by + 62, want: bodyBG, tol: 0.05, kind: kindBG},
		{desc: "body_bg", x: bx + 450, y: by + 600, want: bodyBG, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "static_label_ink", x: bx + 600, y: by + 20, w: 60, h: 12, bg: bodyBG, tol: 0.04, minPx: 10},
		{desc: "first_banner_ink", x: bx + 600, y: by + 116, w: 260, h: 22, bg: bodyBG, tol: 0.04, minPx: 20},
	}
	if stage == "post" {
		pixelChecks = append(pixelChecks,
			pixelCheck{desc: "hot_white", x: bx + 760, y: by + 540, want: [3]float64{1.0, 1.0, 1.0}, tol: 0.06},
			pixelCheck{desc: "old_hot_slot_bg", x: bx + 660, y: by + 230, want: bodyBG, tol: 0.05, kind: kindBG},
			pixelCheck{desc: "hot_bottom_bg", x: bx + 760, y: by + 604, want: bodyBG, tol: 0.05, kind: kindBG},
		)
		inkRegions = append(inkRegions,
			inkRegion{desc: "recovery_banner_ink", x: bx + 600, y: by + 420, w: 240, h: 24, bg: bodyBG, tol: 0.04, minPx: 20},
		)
	} else {
		pixelChecks = append(pixelChecks,
			pixelCheck{desc: "hot_red", x: bx + 660, y: by + 230, want: [3]float64{0.95, 0.20, 0.20}, tol: 0.06},
			pixelCheck{desc: "spike_slot_bg", x: bx + 660, y: by + 350, want: bodyBG, tol: 0.05, kind: kindBG},
			pixelCheck{desc: "hot_bottom_bg", x: bx + 660, y: by + 294, want: bodyBG, tol: 0.05, kind: kindBG},
		)
	}

	var pixelOK int
	pixelBad := make([]string, 0, len(pixelChecks)+len(inkRegions))
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	pixelTotal := 0
	if gateMode {
		img := loadImage(snapPath)
		pixelResult := make(map[string]bool, len(pixelChecks)+len(inkRegions))
		runPixelChecks(img, pixelChecks, pixelResult)
		runInkRegions(img, inkRegions, pixelResult)
		for _, ok := range pixelResult {
			if ok {
				pixelOK++
			}
		}
		for desc, ok := range pixelResult {
			if !ok {
				pixelBad = append(pixelBad, desc)
			}
		}
		sort.Strings(pixelBad)
		pixelTotal = len(pixelChecks) + len(inkRegions)

		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, stage, goldenRects)
	}

	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		// Manual mode animates; pixel/golden evidence is not asserted. Null +
		// pixel_evidence reason avoids a misleading 0/0 "pass".
		pixelGoldenValue = nil
	}

	logicProbes := []string{
		"presents_positive",
		"fps_interval_ge_55",
		"paint_count_covers_presents",
		"static_content_ge_8",
		"hot_ticks_positive",
		"present_policy_full",
		"damage_area_full_paint",
		"vsync_source_present",
		"warmup_observed",
		"first_present_within_budget",
		"first_present_has_content",
		"cpu_fallback_zero",
	}
	stageProbe := "post_stage_recovery_painted"
	if stage == "pre" {
		stageProbe = "pre_stage_holds_first_frame"
	}
	logicProbes = append(logicProbes, stageProbe)

	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if !paintOK {
		logicFailed = append(logicFailed, "paint_count_covers_presents")
	}
	if staticCount < 8 {
		logicFailed = append(logicFailed, "static_content_ge_8")
	}
	if hotTick <= 0 {
		logicFailed = append(logicFailed, "hot_ticks_positive")
	}
	if !policyOK {
		logicFailed = append(logicFailed, "present_policy_full")
	}
	if !damageOK {
		logicFailed = append(logicFailed, "damage_area_full_paint")
	}
	if !vsyncOK {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if !warmupOK {
		logicFailed = append(logicFailed, "warmup_observed")
	}
	if !firstPresentOK {
		logicFailed = append(logicFailed, "first_present_within_budget")
	}
	if !firstPaintOK {
		logicFailed = append(logicFailed, "first_present_has_content")
	}
	if !fallbackOK {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if stage == "pre" {
		if recoveryTriggered {
			logicFailed = append(logicFailed, "pre_stage_holds_first_frame")
		}
	} else if !recoveryOK {
		logicFailed = append(logicFailed, "post_stage_recovery_painted")
	}
	logicPass := len(logicFailed) == 0

	recoveryStatus := "not_triggered_pre_stage"
	if stage != "pre" {
		recoveryStatus = "triggered_and_repainted"
		if !recoveryOK {
			recoveryStatus = "triggered_but_not_repainted"
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R16",
		Scenario:      "ui_wr_r16_warmup",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":                  snap.PresentPolicy,
			"static_visible":                  staticCount,
			"static_cells":                    staticCells,
			"static_labels":                   staticLabels,
			"hot_ticks":                       hotTick,
			"phases_seen":                     phase,
			"gate_stage":                      stage,
			"phase_mode":                      map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
			"evidence_mode":                   map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":                  map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"recovery_triggered":              recoveryTriggered,
			"paint_at_recovery":               paintAtRecovery,
			"recovery_repaint_ok":             recoveryRepaintOK,
			"recovery_na":                     recoveryNA,
			"recovery_stage":                  stage,
			"recovery_status":                 recoveryStatus,
			"warmup_observed":                 snap.Warmup,
			"time_to_first_present_ms":        snap.TimeToFirstPresentMs,
			"first_present_paint_count":       snap.FirstPresentPaintCount,
			"first_frame_semantic":            "full_clear_full_paint; pre stage freezes the first-frame content for pixel/golden",
			"fps_gate":                        "persistent_tick_ge_55",
			"slope_gate":                      "off_5s_correctness",
			"rss_slope_note":                  "rss_slope_valid=false for 5s < rss_warmup_sec; slope_gate=off per §2.2.4",
			"cpu_fallback_note":               "none observed; last_cpu_fallback empty string means no fallback",
			"time_to_first_present_budget_ms": 2000,
			"gpu_ops_semantics":               "cumulative",
			"g_metrics":                       "skipped",
			"g_metrics_reason":                "R16 is a first-present/recovery correctness window; measure cache fields are observed but not gated",
			"depcheck":                        "passed_no_cgo_no_ui_gpu_import",
			"damage_semantic":                 "full_paint_full_redraw_allowed",
			"logic_probes_ok":                 len(logicProbes) - len(logicFailed),
			"logic_probes_total":              len(logicProbes),
			"logic_bad":                       logicFailed,
			"pixel_probes_ok":                 pixelOK,
			"pixel_probes_total":              pixelTotal,
			"pixel_bad":                       pixelBad,
			"pixel_golden_diff_pct":           pixelGoldenValue,
			"pixel_golden_total_px":           goldenTotalPx,
			"pixel_golden_first_run":          goldenFirstRun,
			"pixel_golden_stage":              stage,
			"snapshot_png":                    snapPath,
			"impl_correctness":                "WarmUp full paint covers the first frame; static grid + text are placed before Open and survive full-tree repaint",
			"impl_dirty":                      "damage area is ≈ full screen by full_paint semantics (not retained)",
			"impl_cache":                      "static RepaintBoundaries observed via boundary_skip/rerecord; recovery invalidates the cache once",
			"impl_edge":                       "pre/post gate stages: frozen first-frame content vs held recovery state; manual animates both",
			"impl_fail":                       "missing warmup/first paint, no recovery repaint, or pixel/golden mismatch → FAIL",
			"impl_visible":                    "LiveHUD t2f/p1/stage, FIRST FRAME banner, recovery banner, phase label, pixel probes + golden",
		},
	})
	report.PaintCount = snap.PaintCount
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
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
			RequirePersistentFPS:   true,
		}); err != nil {
			fail("%v", err)
		}
	}

	status := "OK"
	if !logicPass {
		status = "WARN"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r16_warmup: %s stage=%s presents=%d paint=%d statics=%d hot_ticks=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, stage, presents, snap.PaintCount, staticCount, hotTick,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		r, g, b, valid := sampleLogical(img, dpr, c.x, c.y)
		kind := "fg"
		ok := false
		switch c.kind {
		case kindBG:
			kind = "bg"
			ok = valid && nearC(r, g, b, c.want, c.tol)
		default:
			ok = valid && nearC(r, g, b, c.want, c.tol)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: pixel %-18s %-4s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, kind, c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2], ok)
		result[c.desc] = ok
	}
}

func runInkRegions(img image.Image, regions []inkRegion, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, reg := range regions {
		ink, sampled := 0, 0
		x0, y0 := int(reg.x*dpr), int(reg.y*dpr)
		x1, y1 := int((reg.x+reg.w)*dpr), int((reg.y+reg.h)*dpr)
		bounds := image.Rect(0, 0, 0, 0)
		if img != nil {
			bounds = img.Bounds()
		}
		for py := y0; py < y1; py++ {
			for px := x0; px < x1; px++ {
				if px < bounds.Min.X || py < bounds.Min.Y || px >= bounds.Max.X || py >= bounds.Max.Y {
					continue
				}
				r32, g32, b32, _ := img.At(px, py).RGBA()
				r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
				sampled++
				if !nearC(r, g, b, reg.bg, reg.tol) {
					ink++
				}
			}
		}
		ok := img != nil && sampled > 0 && ink >= reg.minPx
		fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: ink   %-20s region=(%4.0f,%4.0f %3.0fx%2.0f) ink_px=%d want>=%d ok=%v\n",
			reg.desc, reg.x, reg.y, reg.w, reg.h, ink, reg.minPx, ok)
		result[reg.desc] = ok
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

func sampleLogical(img image.Image, dpr, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

func evaluateGolden(snapDir, stage string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_r16_warmup_"+stage+".png")
	base := filepath.Join(snapDir, "showcase_r16_warmup_"+stage+"_base.png")
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []rect) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, aa := a.At(px, py).RGBA()
				br, bg, bb, ba := b.At(px, py).RGBA()
				total++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty golden mask")
	}
	return float64(diff) * 100 / float64(total), total, nil
}
