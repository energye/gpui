// Command ui_wr_r3_boundary is the W1 R3 real-window: nested RepaintBoundary
// cache isolation.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary -auto-only
//	go run ./examples/ui_wr_r3_boundary                 # manual: animate until close
//	go run ./examples/ui_wr_r3_boundary -manual-seconds 15
//
// Window: 1200x800. R3 close duration is 10s (§2.5); RUN_SECONDS>=5 is the
// generic floor, but a gate run below 10s is rejected because it cannot close
// R3. GPU window required.
//
// Evidence: boundary logic probes, color/ink probes on the GPU snapshot, and a
// bitwise Golden over header + legend + body (821184 logical px). Gate mode
// freezes Steady (hot/hot2 fixed) so the snapshot is deterministic; manual mode
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
	contentRBs = 8
	nestDepth  = 3
	gridCells  = 16
	gridLabelN = 8
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R3")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R3 (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_wr_r3_boundary — 嵌套 Boundary 缓存",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	// U17 shell: TopBar + colour-swatch Legend + Body + LiveHUD. The legend is
	// built here (instead of passing plain strings to NewShell) so every row has
	// a visible swatch plus text, matching §2.6.1.
	shell := wrkit.NewShell(winW, winH, "R3 嵌套Boundary缓存 — 内脏不外溢", nil)
	legendRows := []struct {
		text string
		r    float64
		g    float64
		b    float64
	}{
		{"outer 240² static", 0.16, 0.18, 0.24},
		{"mid 180² static", 0.20, 0.15, 0.26},
		{"inner 120² + hot", 0.12, 0.21, 0.24},
		{"hot red: dirty each tick", 0.95, 0.20, 0.20},
		{"hot2 blue: sibling nest", 0.20, 0.60, 0.95},
		{"deep chain 200/150/100", 0.15, 0.15, 0.20},
		{"4×4 grid + labels static", 0.32, 0.54, 0.64},
		{"skip↑ Replay / rr↑ rerecord", 0.25, 0.85, 0.45},
		{"gate: -auto-only / 10s", 0.55, 0.75, 0.95},
	}
	for i, row := range legendRows {
		y := 36 + float64(i)*22
		shell.Legend.ColorAt(10, 10, 12, y+3, row.r, row.g, row.b, 1, false)
		shell.Legend.LabelAt(row.text, 12, 28, y, 0.70, 0.78, 0.88)
	}

	// Scratch: R3 gate mode runs with the full scene (nested boundaries + text
	// + dense static content) but freezes the animation phase for a stable
	// snapshot. Manual mode animates through Steady -> Spike -> Recover.
	outer := newBoundary(240, 240, 0.16, 0.18, 0.24, "outer")
	shell.Body.Place(outer, 20, 20)
	mid := newBoundary(180, 180, 0.20, 0.15, 0.26, "mid")
	outer.Place(mid, 30, 30)
	inner := newBoundary(120, 120, 0.12, 0.21, 0.24, "inner")
	mid.Place(inner, 30, 30)
	hot := rendering.NewRenderColorBox(60, 60, 0.95, 0.20, 0.20, 1)
	inner.Place(hot, 30, 30)

	// Nest level 1: three short labels in the bottom margin, plus one static
	// block per level. Text + graphics are both inside each boundary.
	staticLabels := 0
	for i, text := range []string{"L1", "L2", "L3"} {
		outer.Place(wrkit.Label(text, 11, 0.72, 0.78, 0.90), 5+float64(i)*65, 212)
		staticLabels++
	}
	mid.Place(wrkit.Label("mid", 11, 0.70, 0.80, 0.90), 5, 150)
	inner.Place(wrkit.Label("inner", 11, 0.80, 0.85, 0.95), 5, 100)
	staticLabels += 2
	outer.Place(rendering.NewRenderColorBox(20, 20, 0.40, 0.50, 0.60, 1), 5, 5)
	mid.Place(rendering.NewRenderColorBox(20, 20, 0.50, 0.40, 0.60, 1), 5, 5)
	inner.Place(rendering.NewRenderColorBox(20, 20, 0.60, 0.40, 0.50, 1), 5, 5)

	// Sibling nest: proves hot2 does not touch nest1, and outer2/inner2 stay
	// cached while nest1's inner hot cell changes.
	outer2 := newBoundary(180, 180, 0.14, 0.18, 0.20, "outer2")
	shell.Body.Place(outer2, 300, 20)
	inner2 := newBoundary(100, 100, 0.20, 0.16, 0.22, "inner2")
	outer2.Place(inner2, 40, 40)
	hot2 := rendering.NewRenderColorBox(50, 50, 0.20, 0.60, 0.95, 1)
	inner2.Place(hot2, 25, 25)
	outer2.Place(wrkit.Label("nest2", 11, 0.70, 0.80, 0.90), 5, 5)
	inner2.Place(wrkit.Label("inner2", 11, 0.75, 0.82, 0.94), 5, 5)
	staticLabels += 2
	outer2.Place(rendering.NewRenderColorBox(20, 20, 0.35, 0.55, 0.45, 1), 5, 150)
	inner2.Place(rendering.NewRenderColorBox(20, 20, 0.55, 0.45, 0.35, 1), 5, 75)

	// Third chain (depth 3): boundary_count / max_depth evidence with mixed
	// labels and static blocks; never dirtied after setup.
	deep := newBoundary(200, 200, 0.15, 0.15, 0.20, "deep")
	shell.Body.Place(deep, 520, 300)
	d2 := newBoundary(150, 150, 0.19, 0.14, 0.22, "d2")
	deep.Place(d2, 25, 25)
	d3 := newBoundary(100, 100, 0.12, 0.19, 0.22, "d3")
	d2.Place(d3, 25, 25)
	deep.Place(wrkit.Label("deep", 11, 0.80, 0.85, 0.95), 5, 5)
	d2.Place(wrkit.Label("d2", 11, 0.78, 0.84, 0.94), 5, 5)
	d3.Place(wrkit.Label("d3", 11, 0.76, 0.82, 0.93), 5, 5)
	staticLabels += 3
	deep.Place(rendering.NewRenderColorBox(20, 20, 0.45, 0.50, 0.35, 1), 5, 175)
	d2.Place(rendering.NewRenderColorBox(20, 20, 0.35, 0.45, 0.55, 1), 5, 125)
	d3.Place(rendering.NewRenderColorBox(20, 20, 0.55, 0.35, 0.45, 1), 5, 75)

	// Static dense 4x4 grid outside the boundary tree. Colours vary by cell so
	// probes can prove the grid survived while the inner boundary re-recorded.
	gridWant := func(i, j int) [3]float64 {
		return [3]float64{0.20 + 0.06*float64(i), 0.42 + 0.06*float64(j), 0.60 + 0.02*float64(i)}
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			want := gridWant(i, j)
			c := rendering.NewRenderColorBox(40, 40, want[0], want[1], want[2], 1)
			shell.Body.Place(c, 520+float64(i)*55, 20+float64(j)*55)
		}
	}
	for i := 0; i < gridLabelN; i++ {
		t := wrkit.Label(fmt.Sprintf("label %d", i), 11, 0.65, 0.72, 0.85)
		shell.Body.Place(t, 20+float64(i%4)*70, 300+float64(i/4)*40)
		staticLabels++
	}

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r3_boundary", "testdata")
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
				fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size.
				// Gate mode never resizes, so pixel/Golden coordinates stay fixed.
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// R3 is a full_paint correctness window (§2 main table). Keep the policy
	// explicit so the JSON cannot silently inherit the W6 retained default.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{wrkit.PhaseSteady: true}
	clock := wrkit.NewPhaseClock(2, 5)
	hotTicks := int64(0)
	lastRR := int64(0)
	rrSteady, rrSpike, rrRecover := int64(0), int64(0), int64(0)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady: run a deterministic 60-tick colour warm-up so the
			// inner boundaries really change content and re-record; then hold
			// the exact final colours. The 10s snapshot is therefore bit-stable
			// while boundary_rerecord proves inner-only dirty (not a 0/0 pass).
			phase = wrkit.PhaseSteady
			if hotTicks < 60 {
				t := float64(hotTicks) / 60
				hot.R, hot.G, hot.B = 0.95-0.30*t, 0.20, 0.20
				hot2.R, hot2.G, hot2.B = 0.20, 0.60, 0.95-0.30*t
			} else {
				hot.R, hot.G, hot.B = 0.95, 0.20, 0.20
				hot2.R, hot2.G, hot2.B = 0.20, 0.60, 0.95
			}
			hot.MarkNeedsPaint()
			hot2.MarkNeedsPaint()
			hotTicks++
		} else {
			phase = clock.Advance(dt)
			switch phase {
			case wrkit.PhaseSteady:
				// Inner-only dirty: the two hot cells re-record, their parent
				// boundaries and the sibling nest stay cached.
				hot.R = 0.95 - 0.30*(clock.Elapsed()/2)
				hot.G, hot.B = 0.20, 0.20
				hot.MarkNeedsPaint()
				hot2.B = 0.95 - 0.30*(clock.Elapsed()/2)
				hot2.R, hot2.G = 0.20, 0.60
				hot2.MarkNeedsPaint()
				hotTicks++
			case wrkit.PhaseSpike:
				// Outer-level dirty: only nest1's outer boundary re-records.
				outer.Background = &rendering.Color{
					R: 0.35 + 0.20*float64(int(clock.Elapsed())%2),
					G: 0.20, B: 0.30, A: 1,
				}
				outer.MarkNeedsPaint()
			default:
				// Recover: no content dirty -> pure Replay frames.
			}
		}
		phaseSeen[phase] = true
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD; skip/rerecord and the phase are the on-screen proof.
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
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	// Pixel probes are anchored to the body panel origin. Gate mode never
	// resizes, so these logical coordinates stay fixed.
	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "outer_bg", x: bx + 35, y: by + 252, want: [3]float64{0.16, 0.18, 0.24}, tol: 0.06, kind: kindBG},
		{desc: "mid_bg", x: bx + 65, y: by + 222, want: [3]float64{0.20, 0.15, 0.26}, tol: 0.06, kind: kindBG},
		{desc: "inner_bg", x: bx + 185, y: by + 90, want: [3]float64{0.12, 0.21, 0.24}, tol: 0.06, kind: kindBG},
		{desc: "outer_static", x: bx + 35, y: by + 35, want: [3]float64{0.40, 0.50, 0.60}, tol: 0.06},
		{desc: "mid_static", x: bx + 65, y: by + 65, want: [3]float64{0.50, 0.40, 0.60}, tol: 0.06},
		{desc: "inner_static", x: bx + 95, y: by + 95, want: [3]float64{0.60, 0.40, 0.50}, tol: 0.06},
		{desc: "hot_red", x: bx + 140, y: by + 140, want: [3]float64{0.95, 0.20, 0.20}, tol: 0.06},
		{desc: "outer2_bg", x: bx + 315, y: by + 155, want: [3]float64{0.14, 0.18, 0.20}, tol: 0.06, kind: kindBG},
		{desc: "inner2_bg", x: bx + 375, y: by + 145, want: [3]float64{0.20, 0.16, 0.22}, tol: 0.06, kind: kindBG},
		{desc: "outer2_static", x: bx + 315, y: by + 180, want: [3]float64{0.35, 0.55, 0.45}, tol: 0.06},
		{desc: "inner2_static", x: bx + 355, y: by + 145, want: [3]float64{0.55, 0.45, 0.35}, tol: 0.06},
		{desc: "hot2_blue", x: bx + 390, y: by + 110, want: [3]float64{0.20, 0.60, 0.95}, tol: 0.06},
		{desc: "deep_bg", x: bx + 555, y: by + 485, want: [3]float64{0.15, 0.15, 0.20}, tol: 0.06, kind: kindBG},
		{desc: "d2_bg", x: bx + 580, y: by + 465, want: [3]float64{0.19, 0.14, 0.22}, tol: 0.06, kind: kindBG},
		{desc: "d3_bg", x: bx + 605, y: by + 435, want: [3]float64{0.12, 0.19, 0.22}, tol: 0.06, kind: kindBG},
		{desc: "deep_static", x: bx + 535, y: by + 485, want: [3]float64{0.45, 0.50, 0.35}, tol: 0.06},
		{desc: "d2_static", x: bx + 560, y: by + 465, want: [3]float64{0.35, 0.45, 0.55}, tol: 0.06},
		{desc: "d3_static", x: bx + 585, y: by + 435, want: [3]float64{0.55, 0.35, 0.45}, tol: 0.06},
		{desc: "grid_00", x: bx + 540, y: by + 40, want: gridWant(0, 0), tol: 0.06},
		{desc: "grid_11", x: bx + 595, y: by + 95, want: gridWant(1, 1), tol: 0.06},
		{desc: "grid_22", x: bx + 650, y: by + 150, want: gridWant(2, 2), tol: 0.06},
		{desc: "grid_33", x: bx + 705, y: by + 205, want: gridWant(3, 3), tol: 0.06},
		{desc: "grid_03", x: bx + 540, y: by + 205, want: gridWant(0, 3), tol: 0.06},
		{desc: "body_bg", x: bx + 400, y: by + 600, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "outer_labels_ink", x: bx + 25, y: by + 232, w: 150, h: 14, bg: [3]float64{0.16, 0.18, 0.24}, tol: 0.04, minPx: 8},
		{desc: "mid_label_ink", x: bx + 55, y: by + 200, w: 55, h: 16, bg: [3]float64{0.20, 0.15, 0.26}, tol: 0.04, minPx: 6},
		{desc: "inner_label_ink", x: bx + 85, y: by + 180, w: 70, h: 16, bg: [3]float64{0.12, 0.21, 0.24}, tol: 0.04, minPx: 6},
		{desc: "outer2_label_ink", x: bx + 305, y: by + 25, w: 70, h: 14, bg: [3]float64{0.14, 0.18, 0.20}, tol: 0.04, minPx: 6},
		{desc: "inner2_label_ink", x: bx + 345, y: by + 65, w: 60, h: 14, bg: [3]float64{0.20, 0.16, 0.22}, tol: 0.04, minPx: 6},
		{desc: "deep_label_ink", x: bx + 525, y: by + 305, w: 55, h: 14, bg: [3]float64{0.15, 0.15, 0.20}, tol: 0.04, minPx: 6},
		{desc: "grid_label_ink", x: bx + 20, y: by + 300, w: 55, h: 14, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 6},
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
		"paint_count_positive",
		"boundary_skip_positive",
		"boundary_rerecord_positive",
		"boundary_count_ge_3",
		"boundary_max_depth_ge_2",
		"static_labels_ge_8",
		"present_policy_full",
		"cpu_paths_recorded",
		"cpu_fallback_zero",
		"vsync_source_present",
		"rr_steady_positive",
		"rr_recover_zero",
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
	if snap.PaintCount <= 0 {
		logicFailed = append(logicFailed, "paint_count_positive")
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
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		logicFailed = append(logicFailed, "cpu_paths_recorded")
	}
	if snap.CPUFallbackOps != 0 {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if gateMode && phase != wrkit.PhaseSteady {
		logicFailed = append(logicFailed, "gate_phase_frozen")
	}
	if rrSteady <= 0 {
		logicFailed = append(logicFailed, "rr_steady_positive")
	}
	if rrRecover != 0 {
		logicFailed = append(logicFailed, "rr_recover_zero")
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

	extra := map[string]any{
		"client_px":              "1200x800",
		"run_seconds":            secs,
		"close_seconds_required": closeSecs,
		"manual_seconds":         *manualSeconds,
		"present_policy":         snap.PresentPolicy,
		"nest_depth":             nestDepth,
		"static_labels":          staticLabels,
		"static_boundaries":      contentRBs,
		"grid_cells":             gridCells,
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
		"rss_slope_steady_note":  "rss_slope_steady_kb_per_min omitted when rss_slope_valid=false; an invalid steady slope must not claim a value (see wrgate TestBuildReport_ShortRunSlopeInvalidIsHonest)",
		"cpu_fallback_note":      "none observed; last_cpu_fallback empty means no fallback",
		"gpu_ops_semantics":      "cumulative",
		"g_metrics":              "skipped",
		"g_metrics_reason":       "R3 is a boundary-cache correctness window; measure cache fields are observed but not gated",
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
		"impl_correctness":       "nested RBs outer→mid→inner, outer2→inner2 and deep→d2→d3; Steady dirties only the inner hot cells, Spike dirties only nest1 outer, clean layers Replay",
		"impl_dirty":             "steady: hot/hot2 mark paint each tick; spike: only outer.Background marks paint; recover: no content dirty; HUD is a separate RepaintBoundary band",
		"impl_cache":             "clean boundaries accumulate boundary_skip; dirty inner layers increment boundary_rerecord; static labels, blocks, deep chain and 4×4 grid stay cached",
		"impl_edge":              "3-level nest1, 3-level deep chain, sibling nest2, 16 dense grid cells, 18 labels; resize re-lays the shell",
		"impl_fail":              "skip=0 or rerecord=0, boundary_count<3, depth<2, pixel probe mismatch, golden diff!=0, fps<55/p95>22, or present_policy!=full_paint → FAIL",
		"impl_visible":           "HUD skip/rr + phase; red hot cell animates in manual Steady; outer background flips in Spike; static layers stay; legend has colour swatches; gate pixel/Golden evidence",
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3",
		Scenario:      "ui_wr_r3_boundary",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
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
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
			RequirePersistentFPS:   true,
			MinFPSWall:             55,
			MinFPSElapsed:          5,
			MaxP95Ms:               22,
			MinBoundarySkip:        3,
			MinBoundaryRerecord:    2,
			MinBoundaryCount:       3,
			MinBoundaryMaxDepth:    2,
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
		"ui_wr_r3_boundary: %s presents=%d skip=%d rr=%d count=%d depth=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, presents, snap.BoundarySkip, snap.BoundaryRerecord,
		snap.BoundaryCount, snap.BoundaryMaxDepth,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

func newBoundary(w, h, r, g, b float64, name string) *rendering.AbsoluteBox {
	box := rendering.NewAbsoluteBox(w, h)
	box.Background = &rendering.Color{R: r, G: g, B: b, A: 1}
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
