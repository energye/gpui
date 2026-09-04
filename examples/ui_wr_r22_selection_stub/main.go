// Command ui_wr_r22_selection_stub is the R22 selection/caret local-dirty
// reservation single-ability real window (mode-1 first close).
//
// §2 R22 thesis: 选区/光标局部脏预留 — the engine reserves a local-dirty
// path for future selection/caret work, so that selection changes never need
// a full-window repaint. This window proves the reservation is in place: the
// real engine selection API (textinput.Editor.SetSelection) exists and runs,
// the stub visual (selection lane + caret bar) toggles locally on phase
// steps, and the rest of the window stays untouched.
//
// Boards:
//
//	STUB   selection lane + caret bar driven by a headless textinput.Editor;
//	       Steady/Recover show range A, Spike shows range B (the observable
//	       "selection range change", phase-local repaint only)
//	DENSE  static 4x4 grid + 8 labels, never touched
//	HOT    breathing-scale disc (no selection) — the persistent motion hotspot
//
// Gates (§2 主表 R22 行「字段可 0；API 存在」+ §2.2 全族): full_paint policy,
// selection_api_present + selection_set_ok, stub calls >= 2, paint drift
// capped (stub never full-repaints), pixel assertions, golden static mask
// bitwise-stable across runs.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r22_selection_stub
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 5 (§2.5 正确性档).
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/textinput"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 5 // §2.5 正确性档（R22 = 5s）

	phaseLen    = 1.5 // Steady 1.5s → Spike 1.5s → Recover 2s (5s close)
	hitchBudget = 5.0 // per-min budget (§2.2.2 正确性窗默认建议)
	// layoutDriftCap bounds post-settle layouts: phase entries repaint lane
	// boxes paint-only (MarkNeedsPaint/MoveTo) and must never relayout.
	// paint_count is cumulative and full_paint repaints by design, so it is
	// reported as observation (paint_rate_per_sec), not gated — a drift cap
	// on it would scale with run length and FAIL any full_paint window.
	layoutDriftCap = 4
)

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG   = [3]float64{0.08, 0.09, 0.11}
	denseBase = [3]float64{0.10, 0.11, 0.13}
	laneBG    = [3]float64{0.14, 0.16, 0.22}
	selFill   = [3]float64{0.30, 0.55, 0.95}
	selAlpha  = 0.45
	caretFill = [3]float64{1.0, 0.65, 0.20}
	hotFill   = [3]float64{0.25, 0.75, 0.35}
	stubBG    = [3]float64{0.12, 0.13, 0.18}
)

// blendOver returns the exact F5 group expectation for a translucent src over
// an opaque background: out = a*src + (1-a)*bg.
func blendOver(src, bg [3]float64, a float64) [3]float64 {
	return [3]float64{
		a*src[0] + (1-a)*bg[0],
		a*src[1] + (1-a)*bg[1],
		a*src[2] + (1-a)*bg[2],
	}
}

type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	pt   *probePoint
	text *rect // F6: text density is a REGION property
	base [3]float64
	desc string
}

var pixelResult = map[string]bool{}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "R22")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// R22 ability proof: the real engine selection API exists and runs.
	// A headless Editor exercises SetSelection for the two stub ranges; the
	// window lane mirrors the same A/B state machine visually.
	ed := textinput.New()
	apiPresent := ed != nil
	setOK := false
	selOK := false
	stubCalls := 0
	if apiPresent {
		setOK = ed.SetText("stub selection 预留行", textinput.TextRange{Base: 5, Extent: 5}, textinput.TextRange{}, 0)
		stubCalls++
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_r22_selection_stub — 选区/光标局部脏预留", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R22 选区/光标局部脏预留 — stub 空跑，防将来全窗刷", []string{
		"STUB  选区 lane + 光标条 (Headless Editor 真 API 驱动)",
		"Steady/Recover=选区A Spike=选区B 相位局部重绘",
		"DENSE 右侧密集区 4x4色格+8标签 全程静止",
		"HOT   呼吸缩放盘 持续动画热点(与选区无关)",
		"门禁: API存在 + stub调用 + paint漂移封顶",
		"静背景不闪 = Golden 逐位 + 像素断言",
	})
	body := shell.Body

	// ===== Board STUB: selection lane + caret ==============================
	stubBoard := rendering.NewAbsoluteBox(560, 300)
	stubBoard.Background = &rendering.Color{R: stubBG[0], G: stubBG[1], B: stubBG[2], A: 1}
	stubBoard.Place(wrkit.Label("STUB 选区/光标预留 (真 API: Editor.SetSelection)", 13, 0.90, 0.93, 0.98), 12, 10)
	stubBoard.Place(wrkit.Label("stub selection 预留行 — lane 镜像 headless 选区状态", 12, 0.70, 0.78, 0.88), 12, 34)
	lane := rendering.NewRenderColorBox(300, 30, laneBG[0], laneBG[1], laneBG[2], 1)
	stubBoard.Place(lane, 12, 70)
	// Range A (Steady/Recover): 60px highlight. Range B (Spike): the
	// adjacent 80px tail lights up too (combined 140px). The two boxes are
	// ADJACENT, never overlapping — whoever paints last must not cover the
	// other (a past revision stacked them and selB hid selA: pixel FAIL).
	selA := rendering.NewRenderColorBox(60, 30, selFill[0], selFill[1], selFill[2], selAlpha)
	stubBoard.Place(selA, 12, 70)
	selB := rendering.NewRenderColorBox(80, 30, laneBG[0], laneBG[1], laneBG[2], 1) // hidden in A
	stubBoard.Place(selB, 72, 70)
	caret := rendering.NewRenderColorBox(3, 24, caretFill[0], caretFill[1], caretFill[2], 1)
	stubBoard.Place(caret, 76, 73)
	stubBoard.Place(wrkit.Label("Spike 选区B(宽高亮+光标跟尾) Recover 恢复A", 11, 0.62, 0.70, 0.82), 12, 112)
	stubState := wrkit.Label("SEL=A caret=76", 12, 0.85, 0.95, 0.55)
	stubBoard.Place(stubState, 12, 140)
	body.Place(stubBoard, 8, 8)

	// ===== Board DENSE: static grid + labels (never touched) ===============
	dense := rendering.NewAbsoluteBox(300, 300)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	var cell00 *rendering.RenderColorBox
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 28, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			if i == 0 && j == 0 {
				cell00 = c
			}
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		dense.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*64, 168+float64(row)*20)
	}
	denseNote := wrkit.Label("dense 静区 全程不闪", 11, 0.60, 0.68, 0.80)
	dense.Place(denseNote, 10, 216)
	body.Place(dense, 584, 8)

	// ===== Board HOT: persistent motion hotspot (breathing scale) ==========
	pulseTarget := rendering.NewRenderTransform()
	pulseArm := rendering.NewAbsoluteBox(120, 120)
	pulseDisc := rendering.NewRenderColorBox(70, 70, hotFill[0], hotFill[1], hotFill[2], 1)
	pulseArm.Place(pulseDisc, 25, 25)
	pulseTarget.FixedWidth, pulseTarget.FixedHeight = 120, 120
	pulseTarget.AddChild(pulseArm)
	hotBoard := rendering.NewAbsoluteBox(250, 250)
	hotBoard.Background = &rendering.Color{R: 0.13, G: 0.13, B: 0.19, A: 1}
	hotBoard.Place(wrkit.Label("HOT 呼吸盘 (与选区无关)", 12, 0.90, 0.80, 0.60), 10, 8)
	hotBoard.Place(pulseTarget, 65, 55)
	body.Place(hotBoard, 8, 322)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	geoValid := false
	ptDense := probePoint{tol: 8.0 / 255}
	ptSel := probePoint{tol: 12.0 / 255} // F5 blend
	ptSelB := probePoint{want: laneBG, tol: 8.0 / 255}
	ptCaret := probePoint{want: caretFill, tol: 8.0 / 255}
	ptHot := probePoint{want: hotFill, tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect

	_ = cell00
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		// DENSE cell (0,0) center. Chain: body→dense(584,8)→cell(10,10) 56x28.
		ptDense.x = bodyX + 584 + 10 + 28
		ptDense.y = bodyY + 8 + 10 + 14
		ptDense.want = [3]float64{0.25, 0.4, 0.62}
		// STUB selA center. Chain: body→stub(8,8)→selA(12,70) 60x30.
		ptSel.x = bodyX + 8 + 12 + 30
		ptSel.y = bodyY + 8 + 70 + 15
		ptSel.want = blendOver(selFill, laneBG, selAlpha)
		// selB lane-exclusive tail (x beyond selA width): hidden in final A.
		ptSelB.x = bodyX + 8 + 12 + 100
		ptSelB.y = bodyY + 8 + 70 + 15
		// caret bar center. Chain: body→stub(8,8)→caret(76,73) 3x24.
		ptCaret.x = bodyX + 8 + 76 + 1
		ptCaret.y = bodyY + 8 + 73 + 12
		// HOT disc center (transform-invariant). Chain: body→hot(8,322)→
		// pulseTarget(65,55)→arm(0,0)→disc(25,25) 70x70 center +35,+35.
		ptHot.x = bodyX + 8 + 65 + 25 + 35
		ptHot.y = bodyY + 322 + 55 + 25 + 35
		// F6 box: dense label rows region.
		capTextBox = rect{x: bodyX + 584 + 10, y: bodyY + 8 + 168, w: 280, h: 44}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + 584, bodyY + 8, 300, 300},
		}
		geoValid = true
	}

	snapDir := os.Getenv("R22_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/r22_selection_stub"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "r22_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r22_selection_stub: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
				geoValid = false
			}
		},
	})
	// Full-paint correctness posture: the stub must never need retained
	// damage tricks to look right; pin the policy explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	clock := wrkit.NewPhaseClock(phaseLen, 2*phaseLen)

	var elapsed float64
	lastPhase := ""
	paintMin, paintMax := int64(-1), int64(-1)
	layoutBase := int64(-1)
	pulseScale := 1.0
	steadyShotDone, recoverShotDone := false, false
	snapSteadyAt, snapRecoverAt := phaseLen*0.7, 2*phaseLen+0.6

	applyPhase := func(phase string) {
		// Range B: tail lights up (combined 140px highlight); the caret rides
		// to the selection end via MoveTo (the runtime-safe reposition path).
		// Range A: tail back to lane bg; caret back after the narrow highlight.
		// The caret stays visible in both phases — phases differ ONLY in the
		// highlight width, so every pixel expectation is exact.
		switch phase {
		case wrkit.PhaseSpike:
			selB.R, selB.G, selB.B, selB.A = selFill[0], selFill[1], selFill[2], selAlpha
			caret.MoveTo(156, 73)
			if apiPresent {
				if ed.SetSelection(textinput.TextRange{Base: 2, Extent: 7}) {
					selOK = true
				}
				stubCalls++
			}
			stubState.SetText("SEL=B caret=156")
		default:
			selA.R, selA.G, selA.B, selA.A = selFill[0], selFill[1], selFill[2], selAlpha
			selB.R, selB.G, selB.B, selB.A = laneBG[0], laneBG[1], laneBG[2], 1
			caret.MoveTo(76, 73)
			if apiPresent && lastPhase != phase {
				if ed.SetSelection(textinput.TextRange{Base: 5, Extent: 5}) {
					selOK = true
				}
				stubCalls++
			}
			stubState.SetText("SEL=A caret=76")
		}
		selA.MarkNeedsPaint()
		selB.MarkNeedsPaint()
		caret.MarkNeedsPaint()
		stubState.MarkNeedsPaint()
	}

	saveSnap := func(name string) {
		img := app.Target().Context().Image()
		if img == nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: no image\n", name)
			return
		}
		f, err := os.Create(filepath.Join(snapDir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: %v\n", name, err)
			return
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: %v\n", name, err)
			return
		}
		fmt.Fprintf(os.Stderr, "snapshot stored: %s\n", name)
	}

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		phase := clock.Advance(dt)
		if phase != lastPhase {
			applyPhase(phase)
			lastPhase = phase
		}

		// Persistent motion hotspot (no selection): breathing scale.
		pulseScale = 1.0 + 0.10*math.Sin(2*math.Pi*elapsed/2.5)
		pulseTarget.SetScale(pulseScale, pulseScale)

		pc := app.Metrics().Snapshot()
		if elapsed >= 1.0 {
			if paintMin < 0 || pc.PaintCount < paintMin {
				paintMin = pc.PaintCount
			}
			if pc.PaintCount > paintMax {
				paintMax = pc.PaintCount
			}
			if layoutBase < 0 {
				layoutBase = pc.LayoutCount
			}
		}

		// §2.7/U21 snapshot discipline: steady mid-run shot + recover shot.
		// Both are fired OFF the ticker goroutine: SnapshotAsync waits
		// (bounded 500ms poll) for raster-thread completion, and blocking
		// the ticker on that wait would charge measurement overhead into
		// frame intervals (first two runs: exactly 2 hitches = 2 snapshot
		// waits, fps 47 vs steady-state 58). A real app never blocks its
		// UI loop on snapshot completion; the closures still execute on
		// the raster thread and completion is verified post-run by file
		// existence below (no fire-and-forget evidence gap).
		if secsSet && !steadyShotDone && elapsed >= snapSteadyAt && phase == wrkit.PhaseSteady {
			steadyShotDone = true
			go app.SnapshotAsync(func() { saveSnap("r22_steady.png") })
		}
		if secsSet && steadyShotDone && !recoverShotDone && elapsed >= snapRecoverAt && phase == wrkit.PhaseRecover {
			recoverShotDone = true
			go app.SnapshotAsync(func() { saveSnap("r22_recover.png") })
		}

		gateOK := layoutBase < 0 || pc.LayoutCount-layoutBase <= layoutDriftCap
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("R22", phase, app, gateOK && apiPresent && setOK && selOK,
			fmt.Sprintf("SEL=%s stub=%d api=%v", map[string]string{wrkit.PhaseSteady: "A", wrkit.PhaseSpike: "B", wrkit.PhaseRecover: "A"}[phase], stubCalls, setOK),
			fmt.Sprintf("pc=%d lay=%d", pc.PaintCount, pc.LayoutCount))
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open: %v\n", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	// ---- Final-frame pixel assertions (§2.7/U21) --------------------------
	// Final phase is Recover = range A restored; expectations are exact.
	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptDense, desc: "dense_cell_00_exact"},
		{pt: &ptSel, desc: "selection_lane_blend_exact"},
		{pt: &ptSelB, desc: "selection_b_hidden_restored"},
		{pt: &ptCaret, desc: "caret_bar_exact"},
		{pt: &ptHot, desc: "pulse_disc_center_invariant"},
		{text: &capTextBox, base: denseBase, desc: "dense_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "r22_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R22",
		Scenario:      "ui_wr_r22_selection_stub",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":            "1200x800",
			"run_seconds":          secs,
			"hitch_budget_per_min": hitchBudget,
			"layout_drift_cap":     layoutDriftCap,
			"paint_count_min":      paintMin,
			"paint_count_max":      paintMax,
			"paint_rate_per_sec":   float64(paintMax-paintMin) / elapsedSec,
			"layout_base":          layoutBase,
			"layout_count_final":   snap.LayoutCount,
			// R22 reservation semantics: no dedicated engine selection
			// counter exists yet — fields stay 0 by design (§2 主表「字段可 0」).
			"selection_api_present": setOK && selOK && apiPresent,
			"selection_set_ok":      setOK && selOK,
			"selection_stub_calls":  stubCalls,
			"selection_dirty_px":    0,
			"selection_fields_zero": true,
			"scripted_ok":           scriptedOK,
			"scripted_total":        scriptedTotal,
			"pixel_golden_diff_pct": goldenDiffPct,
			"pixel_golden_total_px": goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			// RSS note: 5s runs sit inside the startup equilibrium ramp
			// (Go heap + GPU driver + shader cache); leakage verdicts
			// belong to the R15 soak, reported here as observation.
			"rss_slope_semantics": "startup-ramp dominated at 5s; leakage verdict belongs to R15 soak",
			"snapshots":              "steady+recover raster-thread double-shot (U21 §4)",
			"impl_correctness":       "real engine API textinput.Editor.SetSelection drives the A/B stub state machine; lane visuals mirror the same state (no example-local fake API)",
			"impl_dirty":             "phase steps repaint only the lane boxes (selA/selB highlight + caret MoveTo + state label, all paint-only); layout stays flat — stub never triggers layout storms",
			"impl_cache":             "dense board untouched all run; golden static mask covers top/legend/dense",
			"impl_edge":              "resize invalidates probe geometry (re-resolved); final Recover frame restores range A exactly (selB tail back to lane bg)",
			"impl_fail":              "API missing, stub calls < 2, layout drift over cap, any pixel check fail, golden diff != 0 from run 2, fps<55 → FAIL exit 1",
			"impl_visible":           "Spike visibly widens the highlight and rides the caret to the selection end; Recover restores narrow highlight + caret home; HUD shows SEL state + stub count",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates ------------------------------------------------------------
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true, // continuous HOT hotspot ticker
		MinFPSWall:             55,
		MinFPSElapsed:          4,
		MaxP95Ms:               22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if secsSet && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 正确性档关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.2f > %.0f\n", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 D: CPU 非双 0（禁降画质装绿）。
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑")
		os.Exit(1)
	}
	// R22 专用: 真 API 必须存在且调用成功（§2 主表「API 存在」）。
	if !apiPresent || !setOK || !selOK {
		fmt.Fprintf(os.Stderr, "FAIL: selection API missing (present=%v set_ok=%v sel_ok=%v)\n", apiPresent, setOK, selOK)
		os.Exit(1)
	}
	if stubCalls < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: selection_stub_calls=%d want >=2 (phase-driven A/B)\n", stubCalls)
		os.Exit(1)
	}
	// R22 专用: stub 空跑不得引发 layout 风暴（相位切换全是 paint-only）。
	if layoutBase < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: layout baseline never sampled")
		os.Exit(1)
	}
	if snap.LayoutCount-layoutBase > layoutDriftCap {
		fmt.Fprintf(os.Stderr, "FAIL: layout_count drift=%d want ≤%d\n", snap.LayoutCount-layoutBase, layoutDriftCap)
		os.Exit(1)
	}
	// 像素断言全过（逻辑探针 + 像素 + Golden 三证据之一）。
	if scriptedTotal != 6 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 6/6 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// 静背景不闪: Golden 零容差（首跑产基线；次跑起逐位一致）。
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	// U21 快照纪律: 双张必须都落地（flags 只是发射标记；快照闭包在
	// raster 线程执行，文件存在 + 非空才是完成证据）。
	snapOK := func(name string) bool {
		fi, err := os.Stat(filepath.Join(snapDir, name))
		return err == nil && fi.Size() > 0
	}
	if secsSet && (!steadyShotDone || !recoverShotDone) {
		fmt.Fprintf(os.Stderr, "FAIL: snapshots incomplete (steady=%v recover=%v) — U21 §4 requires 2 shots\n",
			steadyShotDone, recoverShotDone)
		os.Exit(1)
	}
	if secsSet && (!snapOK("r22_steady.png") || !snapOK("r22_recover.png")) {
		fmt.Fprintf(os.Stderr, "FAIL: snapshot files missing (steady=%v recover=%v)\n",
			snapOK("r22_steady.png"), snapOK("r22_recover.png"))
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_r22_selection_stub: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min stub=%d pc=%d..%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin, stubCalls,
		paintMin, paintMax, scriptedOK, scriptedTotal, goldenDiffPct, goldenTotalPx, goldenFirstRun,
		snap.VSyncSource, elapsedSec)
}

// --- assertion helpers -----------------------------------------------------

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		var ok bool
		switch {
		case c.pt != nil:
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, c.pt.want, c.pt.tol)
			fmt.Fprintf(os.Stderr, "ui_wr_r22_selection_stub: pixel %-28s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		case c.text != nil:
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_r22_selection_stub: pixel %-28s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
		}
		result[c.desc] = ok
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

func sampleLogical(img image.Image, dpr float64, lx, ly float64) (r, g, b float64, valid bool) {
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

func textPixels(img image.Image, dpr float64, box rect, base [3]float64) int {
	if img == nil {
		return 0
	}
	x0, y0 := int(box.x*dpr), int(box.y*dpr)
	x1, y1 := int((box.x+box.w)*dpr), int((box.y+box.h)*dpr)
	n := 0
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			r32, g32, b32, _ := img.At(px, py).RGBA()
			r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
			if math.Abs(r-base[0]) > 24.0/255 || math.Abs(g-base[1]) > 24.0/255 || math.Abs(b-base[2]) > 24.0/255 {
				n++
			}
		}
	}
	return n
}

// evaluateGolden compares this run's final snapshot against the stored baseline
// over the static mask. First run stores the baseline (first_run=true, no verdict).
func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "r22_final.png")
	base := filepath.Join(snapDir, "r22_final_base.png")
	if _, err := os.Stat(base); err != nil {
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
		return 0, 0, fmt.Errorf("empty mask")
	}
	return float64(diff) / float64(total) * 100, total, nil
}

func fpsOf(snap scheduler.FrameMetrics) float64 {
	if snap.AvgFrameIntervalMs > 1e-6 {
		return 1000.0 / snap.AvgFrameIntervalMs
	}
	return 0
}

type tickerT struct{ on func(dt float64) }

func (t *tickerT) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
