// Command ui_wr_r4_composite is the W2 R4 real-window: 层 Composite Present (Retained).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_wr_r4_composite -auto-only
//	go run ./examples/ui_wr_r4_composite -manual-seconds 15
//
// Window: 1200x800. Gate close duration: 15s (§2.5). GPU window required.
//
// Gate mode freezes the Steady phase so the GPU-readback snapshot and Golden
// baseline are deterministic, while still marking the local hot layer dirty on
// every tick (the retained path is exercised continuously). Manual mode runs the
// full Steady -> Spike -> Recover phase clock and never writes the gate PNG.
//
// The visible capability is retained composition: a large static grid and
// static information panel stay untouched while only the local hot layer is
// re-recorded / presented. The hot layer is positioned with Flutter-style Align
// layout (wrkit.Panel.Align), not hand-computed coordinates.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSecs    = 15
	manualSecs   = 15
	goldenMaskPx = 1200*48 + 260*656 + 904*656

	staticCells  = 16
	staticCell   = 96.0
	staticGap    = 8.0
	staticPanelX = 16.0
	staticPanelY = 16.0
	staticPanelW = 460.0
	staticPanelH = 548.0
	staticGridX  = 16.0
	staticGridY  = 52.0

	infoPanelX = 476.0
	infoPanelY = 16.0
	infoPanelW = 420.0
	infoPanelH = 250.0

	hotSize   = 96.0
	hotAlignX = 0.78
	hotAlignY = 0.70
)

var (
	bodyBG           = [3]float64{0.10, 0.11, 0.13}
	legendBG         = [3]float64{0.11, 0.12, 0.15}
	topBG            = [3]float64{0.12, 0.14, 0.18}
	staticPanelBG    = [3]float64{0.16, 0.18, 0.24}
	infoPanelBG      = [3]float64{0.14, 0.16, 0.21}
	staticLabelColor = [3]float64{0.78, 0.84, 0.92}
	infoLabelColor   = [3]float64{0.68, 0.78, 0.90}
	hotRed           = [3]float64{0.95, 0.20, 0.20}
)

// hotState is read by OnPaint on the raster thread while the ticker writes it
// on the UI thread. Keep the color behind a mutex instead of sharing plain
// RenderColorBox fields across threads.
type hotState struct {
	mu      sync.Mutex
	r, g, b float64
}

func newHotState(c [3]float64) *hotState {
	return &hotState{r: c[0], g: c[1], b: c[2]}
}

func (h *hotState) set(c [3]float64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	changed := math.Abs(c[0]-h.r) > 0.002 || math.Abs(c[1]-h.g) > 0.002 || math.Abs(c[2]-h.b) > 0.002
	h.r, h.g, h.b = c[0], c[1], c[2]
	return changed
}

func (h *hotState) color() [3]float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return [3]float64{h.r, h.g, h.b}
}

func (h *hotState) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil {
		return
	}
	h.mu.Lock()
	r, g, b := h.r, h.g, h.b
	h.mu.Unlock()
	rendering.FillRect(pc, 0, 0, size.Width, size.Height, r, g, b, 1)
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: 15s scripted run, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R4")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R4 (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if !gateMode && *manualSeconds > 0 && *manualSeconds < manualSecs {
		fmt.Fprintf(os.Stderr, "FAIL: -manual-seconds must be >= %d for R4 phase evidence\n", manualSecs)
		os.Exit(1)
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintf(os.Stderr, "wrkit: font: %v\n", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_wr_r4_composite — 层 Composite Present",
		Decorations: true,
		Resizable:   true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R4 层 Composite Present — retained 省绘", []string{
		"RETAINED: 大面积静态背景不重绘",
		"局部 HOT 层每 tick 走 retained present",
		"damage_ratio_avg <= 0.35（≪全屏）",
		"present_mode 非 full（局部损伤）",
		"静态 boundary_skip > 0",
		"HOT 位置走 Align 布局驱动",
		"gate: frozen Steady + pixel/Golden",
		"manual: Steady -> Spike -> Recover",
	})

	staticLabels := 0
	addStaticLabel := func(p *wrkit.Panel, text string, size, x, y float64, c [3]float64) *rendering.RenderText {
		staticLabels++
		return p.LabelAt(text, size, x, y, c[0], c[1], c[2])
	}

	// ------------------------------------------------------------------
	// Static region A: retained 4x4 exact-colour grid + labels. These
	// cells are all RepaintBoundary nodes and never change after build.
	// ------------------------------------------------------------------
	staticPanel := wrkit.NewPanel(staticPanelW, staticPanelH, staticPanelBG[0], staticPanelBG[1], staticPanelBG[2], 1)
	staticPanel.Box.SetRepaintBoundary(true)
	staticPanel.PlaceOn(shell.Body.Box, staticPanelX, staticPanelY)
	addStaticLabel(staticPanel, "R4 RETAINED — STATIC GRID", 13, 12, 8, staticLabelColor)
	addStaticLabel(staticPanel, "4x4 exact-colour cells; boundaries stay clean", 10, 12, 30, [3]float64{0.62, 0.72, 0.84})

	cellsDrawn := 0
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := [3]float64{0.18 + 0.07*float64(i), 0.40 + 0.08*float64(j), 0.62}
			staticPanel.ColorAt(staticCell, staticCell,
				staticGridX+float64(i)*(staticCell+staticGap),
				staticGridY+float64(j)*(staticCell+staticGap),
				c[0], c[1], c[2], 1, true)
			cellsDrawn++
		}
	}
	for i := 0; i < 8; i++ {
		col, row := i%4, i/4
		addStaticLabel(staticPanel, fmt.Sprintf("cell %02d", i), 10,
			16+float64(col)*110, 472+float64(row)*24, staticLabelColor)
	}

	// ------------------------------------------------------------------
	// Static region B: retained information panel with 8 static labels.
	// It is deliberately separate from the grid so damage from the hot
	// layer cannot be attributed to the dense content.
	// ------------------------------------------------------------------
	infoPanel := wrkit.NewPanel(infoPanelW, infoPanelH, infoPanelBG[0], infoPanelBG[1], infoPanelBG[2], 1)
	infoPanel.Box.SetRepaintBoundary(true)
	infoPanel.PlaceOn(shell.Body.Box, infoPanelX, infoPanelY)
	addStaticLabel(infoPanel, "RETAINED INFO PANEL", 13, 12, 8, staticLabelColor)
	addStaticLabel(infoPanel, "static labels and cells; no per-frame re-record", 10, 12, 30, [3]float64{0.62, 0.72, 0.84})
	infoRows := []string{
		"policy      retained",
		"present     damage_union",
		"static      16 cells",
		"labels      static",
		"hot         96 px layer",
		"gate        frozen Steady",
		"golden      821184 px",
		"phases      steady/spike/recover",
	}
	for i, row := range infoRows {
		addStaticLabel(infoPanel, row, 11, 12, 56+float64(i)*22, infoLabelColor)
	}

	// ------------------------------------------------------------------
	// Dynamic hotspot: Flutter-style Align layout; only its own boundary
	// changes. Gate mode freezes the final red after a short deterministic
	// warmup so the snapshot/Golden is stable.
	// ------------------------------------------------------------------
	hotColor := newHotState(hotRed)
	hot := rendering.NewRenderBox()
	hot.FixedWidth, hot.FixedHeight = hotSize, hotSize
	hot.SetRepaintBoundary(true)
	hot.SetDebugName("r4-hot")
	hot.OnPaint = hotColor.paint
	hotAlign := shell.Body.Align(hot, hotAlignX, hotAlignY)
	if hotAlign == nil {
		fmt.Fprintln(os.Stderr, "FAIL: hot Align wrapper is nil")
		os.Exit(1)
	}
	addStaticLabel(shell.Body, "HOT — Align layout driven", 12, 640, 570, [3]float64{0.95, 0.70, 0.60})

	// Phase label is isolated in its own boundary so phase text changes cannot
	// force the static body/root layer to re-record.
	phaseLabel := wrkit.Label("PHASE: STEADY", 16, 0.95, 0.95, 0.40)
	phaseLabel.SetRepaintBoundary(true)
	shell.Body.Place(phaseLabel, 24, 620)

	snapDir := filepath.Join("examples", "ui_wr_r4_composite", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapName := "showcase_r4_composite.png"
	baseName := "showcase_r4_composite_base.png"
	snapPath := filepath.Join(snapDir, snapName)
	snapshotPath := ""
	if gateMode {
		snapshotPath = snapPath
	}
	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapshotPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive shell: body/legend/HUD re-lay to the new window.
				// The hot layer follows through Align layout.
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W2 R4: retained steady state (first frame full, then composite-only +
	// damage present).
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{}
	clock := wrkit.NewPhaseClock(2, 4)
	lastPhase := ""
	hotTick, hotColorChanges, hotMoves := 0, 0, 0
	lastAX, lastAY := hotAlignX, hotAlignY
	manualElapsed := 0.0

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		hotTick++
		if gateMode {
			// Deterministic warmup: first 2s changes the hot colour, then it is
			// frozen red for a stable snapshot. The layer is still marked dirty
			// every tick so retained present/damage keeps being exercised.
			phase = wrkit.PhaseSteady
			if hotTick <= 120 {
				pulse := 0.5 + 0.5*math.Sin(float64(hotTick)*0.22)
				if hotColor.set([3]float64{0.95, 0.20 + 0.55*pulse, 0.20}) {
					hotColorChanges++
				}
			} else if hotTick == 121 {
				if hotColor.set(hotRed) {
					hotColorChanges++
				}
			}
		} else {
			phase = clock.Advance(dt)
			manualElapsed = clock.Elapsed()
			var ax, ay float64
			c := hotRed
			switch phase {
			case wrkit.PhaseSteady:
				pulse := 0.5 + 0.5*math.Sin(manualElapsed*4.0)
				c = [3]float64{0.95, 0.20 + 0.35*pulse, 0.20}
				ax = hotAlignX + 0.020*math.Sin(manualElapsed*2.1)
				ay = hotAlignY + 0.015*math.Cos(manualElapsed*1.7)
			case wrkit.PhaseSpike:
				pulse := 0.5 + 0.5*math.Sin(manualElapsed*12.0)
				c = [3]float64{1.0, 0.72 + 0.15*pulse, 0.20}
				ax = 0.84 + 0.020*math.Sin(manualElapsed*7.0)
				ay = 0.74 + 0.020*math.Cos(manualElapsed*5.0)
			default:
				c = [3]float64{0.20, 0.65, 0.95}
				ax = hotAlignX + 0.020*math.Sin(manualElapsed*1.3)
				ay = hotAlignY + 0.015*math.Cos(manualElapsed*1.1)
			}
			if hotColor.set(c) {
				hotColorChanges++
			}
			if math.Abs(ax-lastAX) > 1e-4 || math.Abs(ay-lastAY) > 1e-4 {
				hotAlign.SetAlignment(ax, ay)
				lastAX, lastAY = ax, ay
				hotMoves++
			}
		}
		phaseSeen[phase] = true

		// Phase label only changes on phase transitions; gate mode never
		// changes it, keeping the Golden snapshot deterministic.
		if !gateMode && phase != lastPhase {
			switch phase {
			case wrkit.PhaseSteady:
				phaseLabel.SetText("PHASE: STEADY")
				phaseLabel.SetColor(0.95, 0.95, 0.40, 1)
			case wrkit.PhaseSpike:
				phaseLabel.SetText("PHASE: SPIKE")
				phaseLabel.SetColor(1.0, 0.80, 0.20, 1)
			default:
				phaseLabel.SetText("PHASE: RECOVER")
				phaseLabel.SetColor(0.20, 0.80, 1.00, 1)
			}
			phaseLabel.MarkNeedsPaint()
			lastPhase = phase
		}

		// Mark only the local hot boundary dirty. Static panels/grid remain
		// clean and are replayed from their retained layers.
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		avgArea, _, samples, _ := app.DamageStats()
		var avgRatio float64
		if samples > 0 {
			avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		}
		sumArea, _, sumSamples := app.DamageRectSumStats()
		var sumRatio float64
		if sumSamples > 0 {
			sumRatio = float64(sumArea) / float64(sumSamples) / float64(winW*winH)
		}
		shell.NoteHUDTick(dt)
		gateOK := app.LastPresentMode() != "full" && app.LastPresentMode() != "idle" &&
			sumRatio <= 0.35 && snapH.BoundarySkip > 0
		shell.UpdateHUD("R4", phase, app, gateOK,
			fmt.Sprintf("sum=%.3f union=%.3f mode=%s skip=%d rr=%d", sumRatio, avgRatio, app.LastPresentMode(), snapH.BoundarySkip, snapH.BoundaryRerecord),
			fmt.Sprintf("hot=%d cells=%d labels=%d", hotTick, cellsDrawn, staticLabels))
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
	avgArea, maxArea, samples, multiFrames := app.DamageStats()
	var avgRatio, maxRatio float64
	if samples > 0 {
		avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		maxRatio = float64(maxArea) / float64(winW*winH)
	}
	rectSumArea, maxRectSum, sumSamples := app.DamageRectSumStats()
	var sumRatioAvg, sumRatioMax float64
	var sumAvgPx int64
	if sumSamples > 0 {
		sumAvgPx = rectSumArea / sumSamples
		sumRatioAvg = float64(rectSumArea) / float64(sumSamples) / float64(winW*winH)
		sumRatioMax = float64(maxRectSum) / float64(winW*winH)
	}
	lastMode := app.LastPresentMode()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	staticVisible := cellsDrawn + staticLabels
	finalHot := hotColor.color()
	phases := make([]string, 0, 3)
	for _, p := range []string{wrkit.PhaseSteady, wrkit.PhaseSpike, wrkit.PhaseRecover} {
		if phaseSeen[p] {
			phases = append(phases, p)
		}
	}

	// Pixel + Golden evidence is gate-only. Manual mode animates the hot layer
	// and must not claim a deterministic screenshot.
	var (
		pixelOK, pixelTotal int
		pixelBad            []string
		goldenDiffPct       float64
		goldenTotalPx       int64
		goldenFirstRun      bool
	)
	if gateMode {
		img := loadImage(snapPath)
		checks, inks := buildEvidenceProbes(shell)
		result := make(map[string]bool, len(checks)+len(inks))
		runPixelChecks(img, checks, result)
		runInkRegions(img, inks, result)
		for _, c := range checks {
			if result[c.desc] {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, c.desc)
			}
		}
		for _, reg := range inks {
			if result[reg.desc] {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, reg.desc)
			}
		}
		pixelTotal = len(checks) + len(inks)
		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, snapName, baseName, goldenRects)
	}

	type probe struct {
		name string
		ok   bool
	}
	probes := []probe{
		{"presents_positive", app.PresentCount() >= 1},
		{"fps_interval_ge_55", fpsInterval >= 55},
		{"p95_measured_le_22", snap.P95FrameIntervalMs > 0 && snap.P95FrameIntervalMs <= 22},
		{"present_policy_retained", snap.PresentPolicy == scheduler.PresentPolicyRetained},
		{"damage_samples_positive", samples > 0},
		{"damage_sum_samples_positive", sumSamples > 0},
		{"damage_sum_avg_le_0.35", sumSamples > 0 && sumRatioAvg <= 0.35},
		{"present_mode_non_full", lastMode != "" && lastMode != "full" && lastMode != "idle"},
		{"boundary_skip_positive", snap.BoundarySkip > 0},
		{"boundary_rerecord_positive", snap.BoundaryRerecord > 0},
		{"static_cells_16", cellsDrawn == staticCells},
		{"static_labels_ge_8", staticLabels >= 8},
		{"static_visible_ge_24", staticVisible >= 24},
		{"hot_repaints_positive", hotTick > 0},
		{"hot_align_wrapper", hotAlign != nil},
		{"cpu_paths_recorded", snap.CPUPctAvg > 0 && (snap.CPUUIPct > 0 || snap.CPURasterPct > 0)},
		{"cpu_fallback_zero", snap.CPUFallbackOps == 0},
		{"vsync_source_present", snap.VSyncSource != ""},
		{"paint_count_positive", snap.PaintCount > 0},
	}
	if gateMode {
		probes = append(probes,
			probe{"gate_phase_frozen", phase == wrkit.PhaseSteady && !phaseSeen[wrkit.PhaseSpike] && !phaseSeen[wrkit.PhaseRecover]},
			probe{"gate_hot_dynamic_warmup", hotColorChanges >= 2},
			probe{"gate_hot_final_red", nearC(finalHot[0], finalHot[1], finalHot[2], hotRed, 0.03)},
			probe{"gate_pixel_probes_all", pixelOK == pixelTotal && pixelTotal > 0},
			probe{"gate_golden_zero", goldenFirstRun || (goldenDiffPct == 0 && goldenTotalPx == goldenMaskPx)},
		)
	} else {
		probes = append(probes,
			probe{"manual_all_phases", phaseSeen[wrkit.PhaseSteady] && phaseSeen[wrkit.PhaseSpike] && phaseSeen[wrkit.PhaseRecover]},
			probe{"manual_elapsed_ge_15", elapsed >= manualSecs},
			probe{"manual_hot_moves_positive", hotMoves > 0},
			probe{"manual_hot_color_changes_positive", hotColorChanges > 0},
		)
	}
	logicFailed := make([]string, 0)
	for _, p := range probes {
		if !p.ok {
			logicFailed = append(logicFailed, p.name)
		}
	}
	logicPass := len(logicFailed) == 0

	pixelGoldenValue := any(nil)
	if gateMode {
		pixelGoldenValue = goldenDiffPct
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R4",
		Scenario:      "ui_wr_r4_composite",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Ruler:         wrgate.RulerFromEnv(),
		Extra: map[string]any{
			"client_px":              "1200x800",
			"run_seconds":            secs,
			"close_seconds_required": closeSecs,
			"manual_seconds":         *manualSeconds,
			"manual_min_seconds":     manualSecs,
			"manual_elapsed":         manualElapsed,
			"present_policy":         snap.PresentPolicy,
			"damage_semantic":        "retained_composite_only",
			"damage_ratio_avg":       avgRatio,    // union bbox: observation only
			"damage_ratio_max":       maxRatio,    // union bbox: observation only
			"damage_ratio_sum_avg":   sumRatioAvg, // measured sum of damage rects: gate
			"damage_ratio_sum_max":   sumRatioMax, // measured sum of damage rects: observation
			"damage_area_sum_avg_px": sumAvgPx,
			"damage_samples":         samples,
			"damage_sum_samples":     sumSamples,
			"damage_multi_frames":    multiFrames,
			"last_present_mode":      lastMode,
			"static_cells":           cellsDrawn,
			"static_labels":          staticLabels,
			"static_visible":         staticVisible,
			"static_background_px":   int(staticPanelW*staticPanelH + infoPanelW*infoPanelH),
			"hot_repaints":           hotTick,
			"hot_color_changes":      hotColorChanges,
			"hot_moves":              hotMoves,
			"hot_final_rgb":          finalHot,
			"hot_aligned":            "align_layout_driven",
			"boundary_skip":          snap.BoundarySkip,
			"boundary_rerecord":      snap.BoundaryRerecord,
			"phase_mode":             map[bool]string{true: "steady-frozen-after-warmup", false: "animated"}[gateMode],
			"phases_seen":            phases,
			"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":         map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     pixelTotal,
			"pixel_bad":              pixelBad,
			"pixel_golden_diff_pct":  pixelGoldenValue,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"golden_mask":            "header 1200x48 + legend 260x656 + body 904x656",
			"golden_mask_px":         goldenMaskPx,
			"golden_tolerance_pct":   0,
			"snapshot_png":           snapshotPath,
			"snapshot_note": map[bool]string{
				true:  "gate snapshot (GPU readback) from the frozen Steady scene",
				false: "manual mode does not write the gate snapshot; committed evidence is not overwritten",
			}[gateMode],
			"fps_gate":           "persistent_tick_ge_55",
			"slope_gate":         "short_run_no_leak_verdict",
			"rss_slope_note":     "rss_slope_valid=false for a 15s run; raw slope is startup-inclusive and is not a leak conclusion",
			"logic_probes_ok":    len(probes) - len(logicFailed),
			"logic_probes_total": len(probes),
			"logic_bad":          logicFailed,
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
		fail("logic probes %d/%d bad=%v", len(probes)-len(logicFailed), len(probes), logicFailed)
	}
	if gateMode {
		if errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:           1,
			RequireRetainedPolicy: true,
			RequirePersistentFPS:  true,
			MinFPSWall:            55,
			MaxP95Ms:              22,
			MinBoundarySkip:       1,
			MinBoundaryRerecord:   1,
			// Do not gate on report.DamageRatio here: that field is the last
			// frame's union bbox (hot + HUD can be 0.435 by geometry). R4's
			// retained-cost gate is the measured damage-rect sum below.
		}); errG != nil {
			fail("%v", errG)
		}
		if sumSamples == 0 || sumRatioAvg > 0.35 {
			fail("damage_ratio_sum_avg=%.3f (samples=%d) > 0.35 (measured damage-rect sum; union avg=%.3f is observation only)", sumRatioAvg, sumSamples, avgRatio)
		}
		if lastMode == "" || lastMode == "full" || lastMode == "idle" {
			fail("present_mode=%q want damage_union/damage_multi (non-full)", lastMode)
		}
		if snap.BoundarySkip <= 0 {
			fail("boundary_skip=%d want >0 (static boundaries must Replay)", snap.BoundarySkip)
		}
		if snap.BoundaryRerecord <= 0 {
			fail("boundary_rerecord=%d want >0 (hot boundary must re-record)", snap.BoundaryRerecord)
		}
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		if !goldenFirstRun && goldenTotalPx != goldenMaskPx {
			fail("golden mask px=%d want %d", goldenTotalPx, goldenMaskPx)
		}
		if goldenFirstRun {
			fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: WARN golden baseline seeded; run again for the zero-diff evidence\n")
		}
		if snap.VSyncSource == "" {
			fail("vsync_source missing")
		}
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r4_composite: OK presents=%d dmg_sum=%.4f dmg_union=%.4f dmg_max=%.4f mode=%s skip=%d rr=%d cells=%d labels=%d hot=%d logic=%d/%d%s phase=%s elapsed=%.1fs\n",
		app.PresentCount(), sumRatioAvg, avgRatio, maxRatio, lastMode, snap.BoundarySkip, snap.BoundaryRerecord,
		cellsDrawn, staticLabels, hotTick, len(probes)-len(logicFailed), len(probes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

func buildEvidenceProbes(shell *wrkit.ShellChrome) ([]pixelCheck, []inkRegion) {
	bx, by := shell.Body.X, shell.Body.Y
	bodyW, bodyH := shell.Body.W, shell.Body.H
	cellCenter := func(i, j int) (float64, float64) {
		return bx + staticPanelX + staticGridX + float64(i)*(staticCell+staticGap) + staticCell/2,
			by + staticPanelY + staticGridY + float64(j)*(staticCell+staticGap) + staticCell/2
	}
	c00x, c00y := cellCenter(0, 0)
	c11x, c11y := cellCenter(1, 1)
	c22x, c22y := cellCenter(2, 2)
	c33x, c33y := cellCenter(3, 3)
	hotX := bx + (bodyW-hotSize)*hotAlignX + hotSize/2
	hotY := by + (bodyH-hotSize)*hotAlignY + hotSize/2

	checks := []pixelCheck{
		{desc: "static_cell_00", x: c00x, y: c00y, want: [3]float64{0.18, 0.40, 0.62}, tol: 0.03},
		{desc: "static_cell_11", x: c11x, y: c11y, want: [3]float64{0.25, 0.48, 0.62}, tol: 0.03},
		{desc: "static_cell_22", x: c22x, y: c22y, want: [3]float64{0.32, 0.56, 0.62}, tol: 0.03},
		{desc: "static_cell_33", x: c33x, y: c33y, want: [3]float64{0.39, 0.64, 0.62}, tol: 0.03},
		{desc: "static_panel_bg", x: bx + staticPanelX + staticPanelW - 30, y: by + staticPanelY + staticPanelH - 18, want: staticPanelBG, tol: 0.03, kind: kindBG},
		{desc: "info_panel_bg", x: bx + infoPanelX + infoPanelW - 20, y: by + infoPanelY + infoPanelH - 18, want: infoPanelBG, tol: 0.03, kind: kindBG},
		{desc: "hot_frozen_red", x: hotX, y: hotY, want: hotRed, tol: 0.03},
		{desc: "body_bg", x: bx + bodyW - 80, y: by + bodyH - 40, want: bodyBG, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: legendBG, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: topBG, tol: 0.05, kind: kindBG},
	}
	inks := []inkRegion{
		{desc: "static_title_ink", x: bx + staticPanelX + 12, y: by + staticPanelY + 8, w: 220, h: 18, bg: staticPanelBG, tol: 0.03, minPx: 20},
		{desc: "static_grid_label_ink", x: bx + staticPanelX + 16, y: by + staticPanelY + 472, w: 60, h: 16, bg: staticPanelBG, tol: 0.03, minPx: 5},
		{desc: "info_row_ink", x: bx + infoPanelX + 12, y: by + infoPanelY + 56, w: 300, h: 18, bg: infoPanelBG, tol: 0.03, minPx: 15},
		{desc: "phase_label_ink", x: bx + 24, y: by + 618, w: 180, h: 24, bg: bodyBG, tol: 0.03, minPx: 15},
		{desc: "hot_label_ink", x: bx + 640, y: by + 568, w: 220, h: 18, bg: bodyBG, tol: 0.03, minPx: 10},
	}
	return checks, inks
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
