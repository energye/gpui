// Command ui_wr_r11_dpr is the W2 R11 real-window: DPR / size cache
// invalidation. BoundaryCache must drop on programmatic size/DPR change, then
// re-record exactly one wave and return to replay (skip).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr -auto-only
//	go run ./examples/ui_wr_r11_dpr                 # manual: animate until close
//	go run ./examples/ui_wr_r11_dpr -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 is the generic floor, but a gate run below
// 15s cannot close R11 (§2.5). GPU window required.
//
// Evidence (U21): logic probes, color/ink probes on the GPU snapshot, and a
// bitwise Golden over header + legend + body (821184 logical px). Gate mode
// keeps the deterministic two-wave script (resize ~3s, cache invalidation
// ~6s) and only freezes the HOT hue (60-tick ramp then hold), so the snapshot
// shows the final re-recorded state bit-stably while the wave counters still
// gate. Manual mode keeps the animated HOT spot and does not assert pixels.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

const (
	winW, winH = 1200, 800
	closeSecs  = 15
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze HOT, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R11")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R11 (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	face, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Shared static image source (proves image-backed boundaries replay too).
	src, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: image buf:", err)
		os.Exit(1)
	}
	defer src.Dispose()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			_ = src.SetRGBA(x, y, 51, 191, 89, 255)
		}
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r11_dpr — DPR/尺寸缓存失效", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R11 DPR/尺寸缓存失效 — 一波 rerecord 回 skip", []string{
		"RESIZE-A = 尺寸变更波 (~3s, Align 布局)",
		"DPR-B   = 全量失效波 (~6s, invalidation)",
		"NEST-C  = 3 层嵌套 boundary (深)",
		"DENSE-D = 4x4 色格 + 8 标签 + 图 (静)",
		"HOT     = 每帧脏的热点 (live, 非 boundary)",
	})

	// Region A: RESIZE target. Nested 2-layer boundary (outer + inner color
	// box). Size change at ~3s must invalidate its entry → one rerecord wave
	// → back to skip.
	areaA := rendering.NewAbsoluteBox(190, 140)
	areaA.SetRepaintBoundary(true)
	areaA.Background = &rendering.Color{R: 0.16, G: 0.28, B: 0.44, A: 1}
	innerA := rendering.NewRenderColorBox(90, 70, 0.45, 0.72, 0.9, 1)
	innerA.SetRepaintBoundary(true)
	areaA.Place(innerA, 16, 16)
	areaA.Place(wrkit.Label("RESIZE-A", 11, 0.75, 0.85, 0.95), 16, 100)
	if face != nil {
		areaA.Place(wrkit.Label("size@3s", 10, 0.7, 0.8, 0.9), 116, 16)
	}
	shell.Body.Align(areaA, 0.04, 0.06)

	// Region B: DPR invalidation target. 2x2 color grid + label; never resized.
	areaB := rendering.NewAbsoluteBox(190, 140)
	areaB.SetRepaintBoundary(true)
	areaB.Background = &rendering.Color{R: 0.14, G: 0.32, B: 0.26, A: 1}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			c := rendering.NewRenderColorBox(60, 42, 0.3+float64(i)*0.2, 0.55, 0.4+float64(j)*0.3, 1)
			c.SetRepaintBoundary(true)
			areaB.Place(c, 16+float64(i)*72, 16+float64(j)*52)
		}
	}
	areaB.Place(wrkit.Label("DPR-B", 11, 0.75, 0.85, 0.95), 16, 122)
	shell.Body.Align(areaB, 0.36, 0.06)

	// Region C: deep nest — outer(boundary) → mid(boundary) → leaf(boundary).
	leafC := rendering.NewRenderColorBox(40, 40, 0.9, 0.45, 0.35, 1)
	leafC.SetRepaintBoundary(true)
	midC := rendering.NewAbsoluteBox(90, 90)
	midC.SetRepaintBoundary(true)
	midC.Background = &rendering.Color{R: 0.3, G: 0.3, B: 0.5, A: 1}
	midC.Place(leafC, 24, 24)
	areaC := rendering.NewAbsoluteBox(190, 150)
	areaC.SetRepaintBoundary(true)
	areaC.Background = &rendering.Color{R: 0.2, G: 0.2, B: 0.38, A: 1}
	areaC.Place(midC, 16, 16)
	areaC.Place(wrkit.Label("NEST-C (3层)", 11, 0.75, 0.85, 0.95), 16, 128)
	shell.Body.Align(areaC, 0.04, 0.52)

	// Region D: dense static content — 4x4 grid + 8 labels + 1 image.
	areaD := rendering.NewAbsoluteBox(420, 190)
	areaD.SetRepaintBoundary(true)
	areaD.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.2, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			areaD.Place(c, 12+float64(i)*66, 14+float64(j)*40)
		}
	}
	for i := 0; i < 4; i++ {
		areaD.Place(wrkit.Label(fmt.Sprintf("cell-%d", i), 10, 0.62, 0.72, 0.85), 12+float64(i)*66, 180)
	}
	imgD := rendering.NewRenderImage(40, 40)
	imgD.SetImage(src.Clone())
	imgD.SetRepaintBoundary(true)
	areaD.Place(imgD, 360, 14)
	areaD.Place(wrkit.Label("DENSE-D (4x4+8标签+图)", 11, 0.75, 0.85, 0.95), 12, 214)
	shell.Body.Align(areaD, 0.36, 0.52)

	// HOT spot: dirties itself every frame (live paint, no boundary). With no
	// boundary it contributes zero rerecord/skip, so steady frames show
	// FrameRerecord=0 and the wave counters measure the invalidation waves
	// alone (noise-free, deterministic gates).
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.88, 0.08)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.88, 0.20)

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r11_dpr", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, snapshotPNG)
	// Manual mode animates, so its frame is not evidence. Keep the gate
	// snapshot path empty there: otherwise a later manual run would overwrite
	// the committed gate PNG that the Golden baseline was seeded from.
	snapshotPath := snapPath
	if !gateMode {
		snapshotPath = ""
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
				fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: close (%s)\n", win.Backend())
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

	var (
		elapsed          float64
		resized          bool
		dprInvalidated   bool
		wave1, wave2     int64
		before1, before2 int64
		wave1Sampled     bool
		wave2Sampled     bool
		// Per-frame rerecord peaks inside each invalidation window: steady
		// frames show FrameRerecord=1 (only the HOT spot re-records); the
		// invalidation frame shows >=2 (HOT + re-recorded region / full wave).
		// A peak proves the wave happened without being drowned by HOT noise.
		peakRR1, peakRR2 int64
		// steadyRR: FrameRerecord of one frame ~1s after the size change —
		// must drop back to 1 (only HOT), proving the region returned to skip.
		steadyRR      int64
		steadySampled bool
	)
	hotHue := 0.0
	hotTicks := int64(0)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt

		// Phase script (§2.6): Steady → size change ~3s → Steady → DPR
		// invalidation ~6s → Recover. Each change triggers exactly one
		// rerecord wave, then skip resumes.
		phase := wrkit.PhaseSteady
		switch {
		case elapsed >= 6.0 && elapsed < 6.8:
			phase = "DPR-Spike"
		case elapsed >= 3.0 && elapsed < 3.8:
			phase = "Resize-Spike"
		case elapsed >= 8.5:
			phase = wrkit.PhaseRecover
		}

		// Change #1 (~3s): programmatic size change (layout-driven via Align).
		if !resized && elapsed >= 3.0 {
			before1 = app.Metrics().Snapshot().BoundaryRerecord
			areaA.FixedWidth, areaA.FixedHeight = 250, 170
			areaA.MarkNeedsLayout()
			resized = true
		}
		if resized && elapsed < 4.0 {
			if v := app.BoundaryCache().Stats().FrameRerecord; v > peakRR1 {
				peakRR1 = v
			}
		}
		if resized && !wave1Sampled && elapsed >= 3.0+0.8 {
			wave1 = app.Metrics().Snapshot().BoundaryRerecord - before1
			wave1Sampled = true
		}
		if resized && !steadySampled && elapsed >= 4.0 {
			steadyRR = app.BoundaryCache().Stats().FrameRerecord
			steadySampled = true
		}

		// Change #2 (~6s): programmatic DPR invalidation (full cache clear).
		if !dprInvalidated && elapsed >= 6.0 {
			before2 = app.Metrics().Snapshot().BoundaryRerecord
			app.InvalidateBoundaryCache()
			dprInvalidated = true
		}
		if dprInvalidated && elapsed < 7.0 {
			if v := app.BoundaryCache().Stats().FrameRerecord; v > peakRR2 {
				peakRR2 = v
			}
		}
		if dprInvalidated && !wave2Sampled && elapsed >= 6.0+0.8 {
			wave2 = app.Metrics().Snapshot().BoundaryRerecord - before2
			wave2Sampled = true
		}

		// HOT spot repaints itself every frame (independent boundary). In gate
		// mode the hue ramps once over 60 ticks to its frozen final value so
		// the end-of-run snapshot is bit-stable; the two invalidation waves
		// still fire on the same elapsed script.
		if gateMode {
			if n := hotTicks; n <= 60 {
				t := float64(n) / 60
				if t > 1 {
					t = 1
				}
				hotHue = 0.5 * t
			}
			hotTicks++
		} else {
			hotHue += 0.07
			if hotHue > 1 {
				hotHue = 0
			}
		}
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD — skip/rerecord/invalidations are the R11 proof.
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		inv := app.CacheInvalidations()
		gateOK := snapH.BoundarySkip >= 1 && snapH.BoundaryRerecord >= 1 && inv >= 1
		shell.UpdateHUD("R11", phase, app, gateOK,
			fmt.Sprintf("skip=%d rr=%d inv=%d", snapH.BoundarySkip, snapH.BoundaryRerecord, inv),
			fmt.Sprintf("wave1=%d wave2=%d t=%.1f", wave1, wave2, elapsed))
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
	wrkit.MergeBoundaryCache(app, &snap)
	inv := app.CacheInvalidations()
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	// Pixel probes are anchored to the body panel origin and reflect the FINAL
	// scene state (areaA already resized to 250x170, all regions re-recorded
	// and back to skip). Align offsets are ax*(parent-child) fractions.
	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "areaA_bg", x: bx + 176, y: by + 129, want: [3]float64{0.16, 0.28, 0.44}, tol: 0.06, kind: kindBG},
		{desc: "innerA", x: bx + 87, y: by + 80, want: [3]float64{0.45, 0.72, 0.90}, tol: 0.06},
		{desc: "areaB_bg", x: bx + 427, y: by + 151, want: [3]float64{0.14, 0.32, 0.26}, tol: 0.06, kind: kindBG},
		{desc: "cellB_00", x: bx + 303, y: by + 68, want: [3]float64{0.30, 0.55, 0.40}, tol: 0.06},
		{desc: "cellB_11", x: bx + 375, y: by + 120, want: [3]float64{0.50, 0.55, 0.70}, tol: 0.06},
		{desc: "areaC_bg", x: bx + 169, y: by + 398, want: [3]float64{0.20, 0.20, 0.38}, tol: 0.06, kind: kindBG},
		{desc: "midC_bg", x: bx + 125, y: by + 359, want: [3]float64{0.30, 0.30, 0.50}, tol: 0.06, kind: kindBG},
		{desc: "leafC", x: bx + 89, y: by + 323, want: [3]float64{0.90, 0.45, 0.35}, tol: 0.06},
		{desc: "areaD_bg", x: bx + 564, y: by + 422, want: [3]float64{0.12, 0.14, 0.20}, tol: 0.06, kind: kindBG},
		{desc: "cellD_00", x: bx + 215, y: by + 271, want: [3]float64{0.25, 0.40, 0.62}, tol: 0.06},
		{desc: "cellD_33", x: bx + 413, y: by + 391, want: [3]float64{0.61, 0.70, 0.62}, tol: 0.06},
		{desc: "imgD_green", x: bx + 554, y: by + 276, want: [3]float64{0.20, 0.749, 0.349}, tol: 0.06},
		{desc: "hot_spot", x: bx + 786, y: by + 63, want: [3]float64{0.90, 0.45, 0.55}, tol: 0.06},
		{desc: "body_bg", x: bx + 450, y: by + 640, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "resize_a_label_ink", x: bx + 42, y: by + 129, w: 80, h: 14, bg: [3]float64{0.16, 0.28, 0.44}, tol: 0.04, minPx: 6},
		{desc: "dpr_b_label_ink", x: bx + 273, y: by + 153, w: 60, h: 14, bg: [3]float64{0.14, 0.32, 0.26}, tol: 0.04, minPx: 6},
		{desc: "nest_c_label_ink", x: bx + 45, y: by + 391, w: 90, h: 14, bg: [3]float64{0.20, 0.20, 0.38}, tol: 0.04, minPx: 6},
		{desc: "cell_labels_ink", x: bx + 186, y: by + 422, w: 200, h: 14, bg: [3]float64{0.12, 0.14, 0.20}, tol: 0.04, minPx: 10},
		{desc: "dense_d_label_ink", x: bx + 186, y: by + 456, w: 160, h: 14, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 10},
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	// Manual mode intentionally animates; its evidence is human observation.
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
		for desc, ok := range pixelResult {
			if ok {
				pixelOK++
			} else {
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
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		pixelGoldenValue = nil
	}

	// The two-wave script needs the full 15s to fire and settle; a shorter
	// manual run cannot judge the wave probes (duration artifact, §2.5).
	fullDuration := gateMode || *manualSeconds == 0 || *manualSeconds >= float64(closeSecs)
	logicProbes := []string{
		"presents_positive",
		"fps_interval_ge_55",
		"present_policy_full",
		"cpu_fallback_zero",
		"vsync_source_present",
		"boundary_skip_positive",
		"boundary_rerecord_positive",
		"cache_invalidations_positive",
	}
	if fullDuration {
		logicProbes = append(logicProbes, "wave1_ge_1", "wave2_ge_1", "steady_rr_zero")
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if snap.PresentPolicy != scheduler.PresentPolicyFullPaint {
		logicFailed = append(logicFailed, "present_policy_full")
	}
	if snap.CPUFallbackOps != 0 {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if snap.BoundarySkip <= 0 {
		logicFailed = append(logicFailed, "boundary_skip_positive")
	}
	if snap.BoundaryRerecord <= 0 {
		logicFailed = append(logicFailed, "boundary_rerecord_positive")
	}
	if inv < 1 {
		logicFailed = append(logicFailed, "cache_invalidations_positive")
	}
	if fullDuration {
		if wave1 < 1 {
			logicFailed = append(logicFailed, "wave1_ge_1")
		}
		if wave2 < 1 {
			logicFailed = append(logicFailed, "wave2_ge_1")
		}
		if steadyRR != 0 {
			logicFailed = append(logicFailed, "steady_rr_zero")
		}
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R11",
		Scenario:      "ui_wr_r11_dpr",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"cache_invalidations":    inv,
			"invalidation_kind":      "programmatic_invalidate_boundary_cache",
			"resize_wave_rerecord":   wave1,
			"dpr_wave_rerecord":      wave2,
			"resize_wave_peak_frame": peakRR1,
			"dpr_wave_peak_frame":    peakRR2,
			"steady_rr_frame":        steadyRR,
			"static_regions":         4,
			"nested_depth":           3,
			"hot_live_paint":         true,
			"boundary_skip":          snap.BoundarySkip,
			"boundary_rerecord":      snap.BoundaryRerecord,
			"phase_mode":             map[bool]string{true: "scripted-frozen-hot", false: "animated"}[gateMode],
			"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":         map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"fps_gate":               "persistent_tick_ge_55",
			"slope_gate":             "off_15s_short_run",
			"rss_slope_note":         "rss_slope_valid=false for 15s = rss_warmup_sec; raw slope is startup-inclusive, not a leak conclusion",
			"golden_mask":            "header 1200x48 + legend 260x656 + body 904x656",
			"golden_tolerance_pct":   0,
			"logic_probes_ok":        len(logicProbes) - len(logicFailed),
			"logic_probes_total":     len(logicProbes),
			"logic_bad":              logicFailed,
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     pixelTotal,
			"pixel_bad":              pixelBad,
			"pixel_golden_diff_pct":  pixelGoldenValue,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"snapshot_png":           snapshotPath,
			"snapshot_note":          map[bool]string{true: "gate snapshot (GPU readback) used for pixel/Golden evidence", false: "manual mode does not write the gate snapshot; committed evidence is not overwritten"}[gateMode],
			"impl_correctness":       "size change (areaA 190x140→250x170 via MarkNeedsLayout) and InvalidateBoundaryCache each produce exactly one rerecord wave, then regions return to skip",
			"impl_dirty":             "gate: same elapsed script, only HOT hue frozen (60-tick ramp then hold); manual: HOT hue cycles every frame",
			"impl_cache":             "resize invalidates only the resized boundary subtree; programmatic invalidation drops the whole cache; both re-record once and replay after",
			"impl_edge":              "4 regions (resize target / dpr grid / 3-level nest / dense grid+labels+image), image-backed boundary (16x16 RGBA clone), resize re-lays the shell",
			"impl_fail":              "wave1=0 or wave2=0 (a change failed to re-record), steady_rr!=0 (region stuck dirty), pixel probe mismatch, golden diff!=0, or policy!=full_paint → FAIL",
			"impl_visible":           "HUD skip/rr/inv + wave1/wave2; areaA visibly grows at ~3s; gate pixel/Golden evidence over the final state",
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
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		// R11 gates (§2 主表: cache_invalidations>=1 + boundary_rerecord>=1 +
		// boundary_skip>=1). cache_invalidations comes from ability_extra via
		// EvaluateRetainedExtras (auto-called inside EvaluateGates).
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
			RequirePersistentFPS:   true,
			MinBoundarySkip:        1,
			MinBoundaryRerecord:    1,
			MinCacheInvalidations:  1,
		}); err != nil {
			fail("%v", err)
		}
	}

	status := "OK"
	if !logicPass {
		status = "WARN"
	}
	if gateMode && goldenFirstRun {
		status = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r11_dpr: %s skip=%d rr=%d inv=%d wave1=%d wave2=%d steadyRR=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, snap.BoundarySkip, snap.BoundaryRerecord, inv, wave1, wave2, steadyRR,
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
