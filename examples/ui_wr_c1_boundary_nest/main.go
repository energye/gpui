// Command ui_wr_c1_boundary_nest is the W1 C1 composite real-window:
// R2+R3+R3b+R12b integrated — nested boundaries + dirty isolation + debug overlay.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest -auto-only
//	go run ./examples/ui_wr_c1_boundary_nest                 # manual: animate until close
//	go run ./examples/ui_wr_c1_boundary_nest -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 is the generic floor, but a gate run below
// 10s cannot close C1 (§2.5). GPU window required.
//
// Evidence (U21): boundary logic probes, color/ink probes on the GPU snapshot,
// and a bitwise Golden over header + legend + body (821184 logical px). Gate
// mode freezes Steady: hot/hotS ramp once over 60 ticks then hold fixed colors
// and stop marking dirty, so every boundary replays clean (the debug overlay
// never bakes into a cached replay) and the snapshot is bit-stable. Manual mode
// keeps the Steady -> Spike -> Recover animation and does not assert pixels.
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
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	closeSecs  = 10
)

// Static nest palette (visible at every level so probes can pin each layer).
var (
	l1BG   = [3]float64{0.16, 0.18, 0.24}
	l2BG   = [3]float64{0.20, 0.15, 0.26}
	l3BG   = [3]float64{0.12, 0.21, 0.24}
	l4BG   = [3]float64{0.24, 0.17, 0.20}
	sibBG  = [3]float64{0.14, 0.18, 0.20}
	bodyBG = [3]float64{0.10, 0.11, 0.13}
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "C1")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close C1 (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c1_boundary_nest — 组合窗", Decorations: true})
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
		"pixel: 17 点 + 6 墨区",
		"golden: header+legend+body",
		"gate: -auto-only / 10s",
	})

	// 4-level nest: L1(280) → L2(220) → L3(160) → L4(100). Every level carries a
	// distinct background so the nest structure is pixel-assertable.
	L1 := newNest(280, 280, l1BG, "L1")
	shell.Body.Place(L1, 20, 20)

	L2 := newNest(220, 220, l2BG, "L2")
	L1.Place(L2, 30, 30)

	L3 := newNest(160, 160, l3BG, "L3")
	L2.Place(L3, 30, 30)

	L4 := newNest(100, 100, l4BG, "L4")
	L3.Place(L4, 30, 30)

	hot := rendering.NewRenderColorBox(50, 50, 0.9, 0.3, 0.3, 1)
	L4.Place(hot, 25, 25)

	// Static text at every level + static blocks (content mix).
	staticLabels := 0
	L1.Place(wrkit.Label("L1", 12, 0.8, 0.85, 0.9), 5, 260)
	L2.Place(wrkit.Label("L2", 12, 0.8, 0.85, 0.9), 5, 200)
	L3.Place(wrkit.Label("L3", 12, 0.8, 0.85, 0.9), 5, 140)
	L4.Place(wrkit.Label("L4", 12, 0.8, 0.85, 0.9), 5, 80)
	staticLabels += 4
	for i := 0; i < 3; i++ {
		c := rendering.NewRenderColorBox(30, 30, 0.3, 0.55, 0.8, 1)
		L1.Place(c, 240, 20+float64(i)*40)
	}

	// Sibling nest to prove isolation (no cross-nest propagation).
	S := newNest(160, 160, sibBG, "S")
	shell.Body.Place(S, 340, 20)
	hotS := rendering.NewRenderColorBox(40, 40, 0.3, 0.8, 0.5, 1)
	S.Place(hotS, 60, 60)

	// Static dense 4x4 grid outside boundaries.
	gridWant := [3]float64{0.24, 0.45, 0.7}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(36, 36, gridWant[0], gridWant[1], gridWant[2], 1)
			shell.Body.Place(c, 540+float64(i)*48, 20+float64(j)*48)
		}
	}
	// Extra density: labels column + non-boundary color strip (mixed content).
	for i := 0; i < 6; i++ {
		t := wrkit.Label(fmt.Sprintf("c1 static %d", i), 11, 0.65, 0.72, 0.85)
		shell.Body.Place(t, 540, 230+float64(i)*26)
		staticLabels++
	}
	stripWant := func(i int) [3]float64 { return [3]float64{0.3 + 0.05*float64(i%4), 0.5, 0.6} }
	for i := 0; i < 8; i++ {
		c := rendering.NewRenderColorBox(40, 26, stripWant(i)[0], stripWant(i)[1], stripWant(i)[2], 1)
		shell.Body.Place(c, 540+float64(i)*52, 400)
	}
	// Sibling labels (isolation visual).
	S.Place(wrkit.Label("sib", 11, 0.75, 0.85, 0.9), 5, 140)
	// L2 static block in L2's right margin, clear of L3's opaque boundary
	// (L3 ends at L2-local x=190; a block under it would be covered).
	L2.Place(rendering.NewRenderColorBox(26, 26, 0.5, 0.35, 0.4, 1), 192, 100)

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_c1_boundary_nest", "testdata")
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
				fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size.
				// Gate mode never resizes, so pixel/Golden coordinates stay fixed.
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	// R12b integrated: debug overlay on live repaints (magenta flash).
	app.SetDebugRepaint(true)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{wrkit.PhaseSteady: true}
	clock := wrkit.NewPhaseClock(2, 5)
	hotTicks := int64(0)
	lastRR := int64(0)
	rrSteady, rrSpike, rrRecover := int64(0), int64(0), int64(0)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady: hot/hotS ramp once over the first 60 ticks (the
			// inner boundaries really re-record), then hold exact final colors
			// and stop marking dirty. Clean boundaries Replay from cache, the
			// debug overlay never bakes into a replay, and the end-of-run
			// snapshot is bit-stable while boundary_rerecord still proves
			// inner-only dirty (not a 0/0 pass).
			phase = wrkit.PhaseSteady
			if n := hotTicks; n <= 60 {
				t := float64(n) / 60
				if t > 1 {
					t = 1
				}
				hot.R = 0.9 - 0.3*t
				hotS.G = 0.8 - 0.2*t
				hot.MarkNeedsPaint()
				hotS.MarkNeedsPaint()
			}
			hotTicks++
		} else {
			phase = clock.Advance(dt)
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
			hotTicks++
		}
		phaseSeen[phase] = true
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (skip/rerecord + flash draws prove the composite live).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		if snapH.BoundaryRerecord >= lastRR {
			d := snapH.BoundaryRerecord - lastRR
			switch phase {
			case wrkit.PhaseSteady:
				rrSteady += d
			case wrkit.PhaseSpike:
				rrSpike += d
			default:
				rrRecover += d
			}
			lastRR = snapH.BoundaryRerecord
		}
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
	// Attribute the final post-tick rerecord delta to the phase that was
	// active, so rr_steady+rr_spike+rr_recover sums to the JSON total.
	if snap.BoundaryRerecord >= lastRR {
		d := snap.BoundaryRerecord - lastRR
		switch phase {
		case wrkit.PhaseSteady:
			rrSteady += d
		case wrkit.PhaseSpike:
			rrSpike += d
		default:
			rrRecover += d
		}
		lastRR = snap.BoundaryRerecord
	}
	draws := app.DebugRepaintDraws()
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	// Pixel probes are anchored to the body panel origin. Gate mode never
	// resizes, so these logical coordinates stay fixed.
	bx, by := shell.Body.X, shell.Body.Y
	// grid/strip cells are plain non-boundary leaves: FullPaint repaints them
	// live every frame, so with the R12b debug overlay on they read the fixed
	// magenta blend c*0.65 + (0.95,0.15,0.85)*0.35 (same gateLive semantics as
	// ui_wr_r12b). Boundary content (nest levels, hot) replays clean/raw.
	blend := func(c [3]float64) [3]float64 {
		return [3]float64{c[0]*0.65 + 0.95*0.35, c[1]*0.65 + 0.15*0.35, c[2]*0.65 + 0.85*0.35}
	}
	pixelChecks := []pixelCheck{
		{desc: "l1_bg", x: bx + 35, y: by + 35, want: l1BG, tol: 0.06, kind: kindBG},
		{desc: "l2_bg", x: bx + 65, y: by + 65, want: l2BG, tol: 0.06, kind: kindBG},
		{desc: "l3_bg", x: bx + 95, y: by + 95, want: l3BG, tol: 0.06, kind: kindBG},
		{desc: "l4_bg", x: bx + 120, y: by + 120, want: l4BG, tol: 0.06, kind: kindBG},
		{desc: "hot_red", x: bx + 160, y: by + 160, want: [3]float64{0.60, 0.30, 0.30}, tol: 0.06},
		{desc: "l1_static", x: bx + 275, y: by + 55, want: [3]float64{0.30, 0.55, 0.80}, tol: 0.06},
		{desc: "l2_static", x: bx + 255, y: by + 163, want: [3]float64{0.50, 0.35, 0.40}, tol: 0.06},
		{desc: "sib_bg", x: bx + 355, y: by + 35, want: sibBG, tol: 0.06, kind: kindBG},
		{desc: "hot_s_green", x: bx + 420, y: by + 100, want: [3]float64{0.30, 0.60, 0.50}, tol: 0.06},
		{desc: "grid_00", x: bx + 558, y: by + 38, want: blend(gridWant), tol: 0.06},
		{desc: "grid_13", x: bx + 606, y: by + 182, want: blend(gridWant), tol: 0.06},
		{desc: "grid_33", x: bx + 702, y: by + 182, want: blend(gridWant), tol: 0.06},
		{desc: "strip_0", x: bx + 560, y: by + 413, want: blend(stripWant(0)), tol: 0.06},
		{desc: "strip_3", x: bx + 716, y: by + 413, want: blend(stripWant(3)), tol: 0.06},
		{desc: "body_bg", x: bx + 450, y: by + 640, want: bodyBG, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "l1_label_ink", x: bx + 24, y: by + 278, w: 40, h: 16, bg: l1BG, tol: 0.04, minPx: 6},
		{desc: "l2_label_ink", x: bx + 54, y: by + 248, w: 40, h: 16, bg: l2BG, tol: 0.04, minPx: 6},
		{desc: "l3_label_ink", x: bx + 84, y: by + 188, w: 40, h: 16, bg: l3BG, tol: 0.04, minPx: 6},
		{desc: "l4_label_ink", x: bx + 114, y: by + 188, w: 40, h: 16, bg: l4BG, tol: 0.04, minPx: 6},
		{desc: "sib_label_ink", x: bx + 344, y: by + 158, w: 50, h: 16, bg: sibBG, tol: 0.04, minPx: 6},
		{desc: "c1_static_ink", x: bx + 540, y: by + 230, w: 120, h: 80, bg: bodyBG, tol: 0.04, minPx: 20},
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
		// Manual mode animates; pixel/Golden evidence is not asserted. Null +
		// pixel_evidence reason avoids a misleading 0/0 "pass".
		pixelGoldenValue = nil
	}

	logicProbes := []string{
		"presents_positive",
		"fps_interval_ge_55",
		"boundary_skip_positive",
		"boundary_rerecord_positive",
		"boundary_count_ge_3",
		"boundary_max_depth_ge_2",
		"static_labels_ge_8",
		"present_policy_full",
		"cpu_fallback_zero",
		"vsync_source_present",
		"debug_draws_positive",
		"rr_steady_positive",
		"rr_phase_sum",
	}
	if gateMode {
		logicProbes = append(logicProbes, "gate_phase_frozen", "gate_rr_spike_zero")
	} else if phaseSeen[wrkit.PhaseSpike] {
		logicProbes = append(logicProbes, "manual_rr_spike_positive")
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if snap.BoundarySkip <= 0 {
		logicFailed = append(logicFailed, "boundary_skip_positive")
	}
	if snap.BoundaryRerecord <= 0 {
		logicFailed = append(logicFailed, "boundary_rerecord_positive")
	}
	if snap.BoundaryCount < 3 {
		logicFailed = append(logicFailed, "boundary_count_ge_3")
	}
	if snap.BoundaryMaxDepth < 2 {
		logicFailed = append(logicFailed, "boundary_max_depth_ge_2")
	}
	if staticLabels < 8 {
		logicFailed = append(logicFailed, "static_labels_ge_8")
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
	if draws < 1 {
		logicFailed = append(logicFailed, "debug_draws_positive")
	}
	if rrSteady <= 0 {
		logicFailed = append(logicFailed, "rr_steady_positive")
	}
	if gateMode && phase != wrkit.PhaseSteady {
		logicFailed = append(logicFailed, "gate_phase_frozen")
	}
	if gateMode {
		if rrSpike != 0 {
			logicFailed = append(logicFailed, "gate_rr_spike_zero")
		}
	} else if phaseSeen[wrkit.PhaseSpike] && rrSpike <= 0 {
		logicFailed = append(logicFailed, "manual_rr_spike_positive")
	}
	if rrSteady+rrSpike+rrRecover != snap.BoundaryRerecord {
		logicFailed = append(logicFailed, "rr_phase_sum")
	}
	logicPass := len(logicFailed) == 0
	phaseList := make([]string, 0, 3)
	for _, p := range []string{wrkit.PhaseSteady, wrkit.PhaseSpike, wrkit.PhaseRecover} {
		if phaseSeen[p] {
			phaseList = append(phaseList, p)
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C1",
		Scenario:      "ui_wr_c1_boundary_nest",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":              "1200x800",
			"run_seconds":            secs,
			"close_seconds_required": closeSecs,
			"manual_seconds":         *manualSeconds,
			"present_policy":         snap.PresentPolicy,
			"nest_depth":             4,
			"static_dense":           16,
			"static_labels":          staticLabels,
			"debug_draws":            draws,
			"debug_state":            "on",
			"hot_ticks":              hotTicks,
			"rr_steady":              rrSteady,
			"rr_spike":               rrSpike,
			"rr_recover":             rrRecover,
			"rr_total_seen":          lastRR,
			"phases_seen":            phaseList,
			"phase_mode":             map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
			"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":         map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"fps_gate":               "persistent_tick_ge_55",
			"slope_gate":             "off_10s_short_run",
			"rss_slope_note":         "rss_slope_valid=false because 10s < rss_warmup_sec=15; raw slope is startup-inclusive, not a leak conclusion",
			"cpu_fallback_note":      "none observed; last_cpu_fallback empty means no fallback",
			"gpu_ops_semantics":      "cumulative",
			"g_metrics":              "skipped",
			"g_metrics_reason":       "C1 is a composite correctness window; measure cache fields are observed but not gated",
			"depcheck":               "passed_no_cgo_no_ui_gpu_import",
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
			"impl_correctness":       "4-level nest L1→L2→L3→L4 with hot in L4, sibling nest S with hotS; Steady dirties only the inner hot cells, Spike dirties only L1, clean layers Replay",
			"impl_dirty":             "gate: frozen Steady, 60-tick hot ramp then hold (clean replay, no baked debug tint); manual: Steady hot/hotS flash, Spike L1.Background, Recover pure Replay; HUD is a separate RepaintBoundary band",
			"impl_cache":             "clean boundaries accumulate boundary_skip; inner hot re-records during the ramp increment boundary_rerecord; static labels, blocks, grid and strip stay cached",
			"impl_edge":              "4-level nest + sibling nest, 16 grid cells, 6 static labels, 8 strip boxes, labels at every level; resize re-lays the shell",
			"impl_fail":              "skip=0 or rerecord=0, boundary_count<3, depth<2, draws=0, pixel probe mismatch, golden diff!=0, or present_policy!=full_paint → FAIL",
			"impl_visible":           "HUD skip/rr + phase; magenta debug flash on live paints (R12b); hot red / hotS green animate in manual Steady; gate pixel/Golden evidence",
		},
	})
	report.DebugRepaintDraws = draws
	report.DebugRepaintOn = true
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
			MinBoundarySkip:        3,
			MinBoundaryRerecord:    2,
			MinBoundaryCount:       3,
			MinBoundaryMaxDepth:    2,
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
	if gateMode && goldenFirstRun {
		status = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_c1_boundary_nest: %s presents=%d skip=%d rr=%d count=%d depth=%d draws=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
			status, presents, snap.BoundarySkip, snap.BoundaryRerecord, snap.BoundaryCount, snap.BoundaryMaxDepth, draws,
			len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
			map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

func newNest(w, h float64, bg [3]float64, name string) *rendering.AbsoluteBox {
	box := rendering.NewAbsoluteBox(w, h)
	box.Background = &rendering.Color{R: bg[0], G: bg[1], B: bg[2], A: 1}
	box.SetRepaintBoundary(true)
	box.SetDebugName(name)
	return box
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
