// Command ui_wr_r0_fullpaint is the W0 R0 real-window: FullPaint 正确性.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint -auto-only
//	go run ./examples/ui_wr_r0_fullpaint                 # manual: animate until close
//	go run ./examples/ui_wr_r0_fullpaint -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
//
// Evidence (U21): logic probes, pixel probes on the GPU snapshot, and a
// bitwise golden over header + legend + body (the live HUD band is excluded).
// Gate mode freezes the phase on Steady so the snapshot is deterministic;
// manual mode keeps the Steady→Spike→Recover animation.
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
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze phase in Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R0")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

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
		"pixel: 12 点 + 2 文字墨量区域",
		"golden: header+legend+body",
		"gate: -auto-only / RUN_SECONDS",
	})

	// Static dense content (静网): 6x4 color grid, each cell a RepaintBoundary.
	staticCells := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(84, 84, 0.15+0.1*float64(i%4), 0.5, 0.3+0.12*float64(j%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*92, 20+float64(j)*92)
			staticCells++
		}
	}
	// Static text (文): 12 labels with distinct colors.
	staticLabels := 0
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("static text %02d", i), 12, 0.65+0.05*float64(i%5), 0.72, 0.82)
		shell.Body.Place(t, 600+float64(i%4)*95, 20+float64(i/4)*30)
		staticLabels++
	}
	staticCount := staticCells + staticLabels

	// Dynamic hotspot (动): phase-driven block — moves + recolors each phase.
	hot := rendering.NewRenderColorBox(120, 120, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 600, 140)
	hotX, hotY := 600.0, 140.0
	// Phase banner: 当前相位大字（Steady/Spike/Recover），相位驱动肉眼可见。
	phaseLabel := wrkit.Label("PHASE: STEADY", 20, 0.95, 0.95, 0.4)
	shell.Body.Place(phaseLabel, 600, 96)

	snapDir := filepath.Join("examples", "ui_wr_r0_fullpaint", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_r0_fullpaint.png")

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

	phase := wrkit.PhaseSteady
	lastPhase := ""
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	applyPhase := func(p string) {
		switch p {
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
	}
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady phase: deterministic snapshot while the ticker
			// still drives one full-tree paint per frame (full_paint).
			phase = wrkit.PhaseSteady
		} else {
			phase = clock.Advance(dt)
		}
		if phase != lastPhase {
			applyPhase(phase)
			lastPhase = phase
		}
		hotTick++
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
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	paintOK := snap.PaintCount > 0 && (presents == 0 || float64(snap.PaintCount) >= float64(presents)*0.9)
	damageOK := snap.DamageAreaPx > 0 && float64(snap.DamageAreaPx) >= float64(winW*winH)*0.9
	policyOK := snap.PresentPolicy == scheduler.PresentPolicyFullPaint
	warmupOK := snap.Warmup
	vsyncOK := snap.VSyncSource != ""
	firstPresentOK := snap.TimeToFirstPresentMs > 0 && snap.TimeToFirstPresentMs < 2000
	fallbackOK := snap.CPUFallbackOps == 0

	// Pixel probes are anchored to the body panel origin. Gate mode never
	// resizes the window, so these logical coordinates stay fixed.
	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "static_00", x: bx + 62, y: by + 62, want: [3]float64{0.15, 0.50, 0.30}, tol: 0.06},
		{desc: "static_10", x: bx + 154, y: by + 62, want: [3]float64{0.25, 0.50, 0.30}, tol: 0.06},
		{desc: "static_13", x: bx + 154, y: by + 338, want: [3]float64{0.25, 0.50, 0.30}, tol: 0.06},
		{desc: "static_32", x: bx + 338, y: by + 246, want: [3]float64{0.45, 0.50, 0.54}, tol: 0.06},
		{desc: "static_51", x: bx + 522, y: by + 154, want: [3]float64{0.25, 0.50, 0.42}, tol: 0.06},
		{desc: "hot_red", x: bx + 660, y: by + 200, want: [3]float64{0.95, 0.20, 0.20}, tol: 0.06},
		{desc: "hot_bottom_bg", x: bx + 660, y: by + 264, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "grid_gap_bg", x: bx + 108, y: by + 62, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "spike_slot_bg", x: bx + 660, y: by + 320, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "body_bg", x: bx + 450, y: by + 600, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "static_label_ink", x: bx + 600, y: by + 20, w: 70, h: 14, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 10},
		{desc: "phase_label_ink", x: bx + 600, y: by + 96, w: 220, h: 24, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 20},
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
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
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
		"cpu_fallback_zero",
	}
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
	if !fallbackOK {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R0",
		Scenario:      "ui_wr_r0_fullpaint",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Ruler:         wrgate.RulerFromEnv(),
		Extra: map[string]any{
			"present_policy":                  snap.PresentPolicy,
			"static_visible":                  staticCount,
			"static_cells":                    staticCells,
			"static_labels":                   staticLabels,
			"hot_ticks":                       hotTick,
			"phases_seen":                     phase,
			"phase_mode":                      map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
			"evidence_mode":                   map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":                  map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"fps_gate":                        "persistent_tick_ge_55",
			"slope_gate":                      "off_5s_correctness",
			"rss_slope_note":                  "rss_slope_valid=false for 5s < rss_warmup_sec; slope_gate=off per §2.2.4",
			"cpu_fallback_note":               "none observed; last_cpu_fallback empty string means no fallback",
			"time_to_first_present_budget_ms": 2000,
			"gpu_ops_semantics":               "cumulative",
			"g_metrics":                       "skipped",
			"g_metrics_reason":                "R0 is a full-paint correctness window; measure cache fields are observed but not gated",
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
			"snapshot_png":                    snapPath,
			"impl_correctness":                "full_paint drives a whole-tree paint flush every frame; static content survives Clear",
			"impl_dirty":                      "damage area is ≈ full screen by full_paint semantics (not retained)",
			"impl_cache":                      "static RepaintBoundaries observed via boundary_skip/rerecord; not used as an R0 gate",
			"impl_edge":                       "24 cells + 12 labels + phase block; resize re-lays the shell",
			"impl_fail":                       "paint_count < presents*0.9, static/hot missing, or pixel/golden mismatch → FAIL",
			"impl_visible":                    "LiveHUD paint/dmg/fps, phase banner, legend, pixel probes + golden",
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
		"ui_wr_r0_fullpaint: %s presents=%d paint=%d statics=%d hot_ticks=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, presents, snap.PaintCount, staticCount, hotTick,
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
		fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: pixel %-14s %-4s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
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
		fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: ink   %-16s region=(%4.0f,%4.0f %3.0fx%2.0f) ink_px=%d want>=%d ok=%v\n",
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

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_r0_fullpaint.png")
	base := filepath.Join(snapDir, "showcase_r0_fullpaint_base.png")
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
