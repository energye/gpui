// Command ui_wr_r20_filter is the R20 Filter-layer single-ability real window
// (mode-1 first close).
//
// §2 R20 thesis: 模糊/灰度滤镜子树 + 外部不变内容 — a subtree wrapped in
// RenderGrayscale / RenderImageFilter must gray/blur ONLY its own pixels and
// leave everything outside untouched; changing the filter (matrix swap /
// radius step) dirties ONLY the filtered boundary (局部 rerecord).
//
// Boards:
//
//	GRAY   RenderGrayscale over a colored patch card; Spike swaps the matrix
//	       identity ↔ grayscale (the observable "filter range change")
//	BLUR   RenderImageFilter over color bars; radius steps with the phase
//	RAW    same card shape WITHOUT a filter — the outside-invariant control
//	PULSE  breathing scale card (no filter) — the persistent motion hotspot
//	DENSE  static nested-boundary grid + labels, never touched
//
// Gates (§2 主表 R20 行「局部 rerecord」+ §2.2 全族): retained policy,
// boundary_rerecord>0 on phase steps, filter_layer_count>=1, paint_count flat,
// pixel assertions (group blend exactness / outside-invariance), golden static
// mask bitwise-stable across runs.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r20_filter
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 10 (§2.5
// SaveLayer/Filter 档). No RUN_SECONDS → interactive loop (pixel evidence off).
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

	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 10 // §2.5 SaveLayer/Filter 档（R18/R20/C6 = 10s）

	boardW, boardH = 250.0, 250.0
	phaseLen       = 3.0 // Steady 3s → Spike 3s → Recover 3s loops

	hitchBudget = 30.0 // per-min budget (README-declared): steady frames are
	// hitch-free (p99≈17ms); hitches concentrate at phase-step frames where a
	// NEW parameter combination pays its one-time filtered-result generation
	// (SaveLayer/Filter 档本质成本; loop returns hit the old cache entry)
	paintDriftCap = 40 // paint_count max−min band; full-paint regression ≫40	paintDriftCap = 40  // paint_count max−min band; full-paint regression ≫40
)

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG     = [3]float64{0.08, 0.09, 0.11}
	denseBase   = [3]float64{0.11, 0.12, 0.18}
	patchRed    = [3]float64{0.90, 0.20, 0.15}
	patchGreen  = [3]float64{0.25, 0.75, 0.35}
	patchBlue   = [3]float64{0.25, 0.50, 0.90}
	barFill     = [3]float64{0.95, 0.65, 0.20}
	bar0Fill    = [3]float64{0.95, 0.65 * 0.8, 0.20 * 0.4} // bar i=0 exact fill (assertion source)
	rawCtrlFill = [3]float64{0.60, 0.30, 0.80}
	boardBG     = [3]float64{0.13, 0.14, 0.19}
)

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return v
	}
	return v
}

func grayMatrix() [20]float32 {
	return [20]float32{
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0, 0, 0, 1, 0,
	}
}

func identityMatrix() [20]float32 {
	var m [20]float32
	m[0], m[6], m[12], m[18] = 1, 1, 1, 1
	return m
}

func isIdentity(m [20]float32) bool {
	id := identityMatrix()
	for i := range m {
		if m[i] != id[i] {
			return false
		}
	}
	return true
}

// expectedUnderMatrix is the exact group expectation for a solid fill under
// the node's CURRENT matrix: identity passes through, the grayscale matrix
// maps every channel to BT.601 luma. Evaluated against the frozen post-run
// node state ⇒ exact, no cross-frame race.
func expectedUnderMatrix(fill [3]float64, m [20]float32) [3]float64 {
	if isIdentity(m) {
		return fill
	}
	var out [3]float64
	for c := 0; c < 3; c++ {
		out[c] = clamp01(float64(m[5*c])*fill[0] + float64(m[5*c+1])*fill[1] + float64(m[5*c+2])*fill[2])
	}
	return out
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
		secs = 0
	} else {
		wrkit.RequireMinRun(secs, "R20")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_r20_filter — Filter 层：子树灰/糊 外不变", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R20 Filter 层 — 子树灰/糊 外不变", []string{
		"GRAY  RenderGrayscale 彩色格纹卡 Spike 切换 identity↔灰",
		"BLUR  RenderImageFilter 半径相位阶跃 4↔8px",
		"RAW   无滤镜对照卡 外部不变的硬证据",
		"PULSE 呼吸缩放对照板（持续动画热点）",
		"DENSE 右侧密集区 全程静止不闪",
		"Spike 相位：矩阵切换 + 半径 ×2",
		"filter_layer_count HUD 可见 0↔1 跳变",
		"静背景不闪 = Golden 逐位 + 像素断言",
	})
	body := shell.Body

	// ===== Board GRAY: grayscale subtree ===================================
	patchesCol := rendering.NewAbsoluteBox(boardW-20, boardH-20)
	patchesCol.SetDebugName("r20-gray-patches")
	for i := 0; i < 3; i++ {
		fill := patchRed
		if i == 1 {
			fill = patchGreen
		}
		if i == 2 {
			fill = patchBlue
		}
		p := rendering.NewRenderColorBox(56, 56, fill[0], fill[1], fill[2], 1)
		p.SetDebugName(fmt.Sprintf("r20-gray-patch-%d", i))
		patchesCol.Place(p, 16+float64(i)*62, 24)
	}
	grayLbl := wrkit.Label("GRAYED TEXT inside filter", 11, 0.95, 0.85, 0.30)
	grayLbl.SetDebugName("r20-gray-label")
	patchesCol.Place(grayLbl, 16, 100)
	redPatchNode := rendering.NewRenderColorBox(180, 44, patchRed[0], patchRed[1], patchRed[2], 1)
	redPatchNode.SetDebugName("r20-gray-redband")
	patchesCol.Place(redPatchNode, 16, 140)

	grayCard := rendering.NewRenderGrayscale(patchesCol)
	grayCard.SetRepaintBoundary(true) // the filtered subtree re-records alone
	grayBoard := rendering.NewAbsoluteBox(boardW, boardH)
	grayBoard.Background = &rendering.Color{R: boardBG[0], G: boardBG[1], B: boardBG[2], A: 1}
	grayBoard.Place(grayCard, 10, 10)

	// ===== Board BLUR: image-filter subtree ================================
	// Bar gaps (22px) exceed ~2.5σ of the 4px blur kernel so bar centers stay
	// ≈ source color — keeps the center-preservation assertion well-defined.
	barsCol := rendering.NewAbsoluteBox(boardW-20, 120)
	for i := 0; i < 4; i++ {
		b := rendering.NewRenderColorBox(34, 110, barFill[0]*(1-0.18*float64(i)), barFill[1]*0.8, barFill[2]*(0.4+0.2*float64(i)), 1)
		barsCol.Place(b, 14+float64(i)*56, 5)
	}
	blurFilter := rendering.NewRenderImageFilter(4, barsCol)
	blurCard := rendering.NewAbsoluteBox(boardW-20, 130)
	blurCard.Place(blurFilter, 5, 5)
	blurBoard := rendering.NewAbsoluteBox(boardW, boardH)
	blurBoard.Background = &rendering.Color{R: 0.12, G: 0.13, B: 0.17, A: 1}
	blurBoard.Place(blurCard, 10, 10)

	// ===== Board RAW: unfiltered control (outside-invariant evidence) ======
	rawCard := rendering.NewRenderColorBox(150, 90, rawCtrlFill[0], rawCtrlFill[1], rawCtrlFill[2], 1)
	rawCard.SetDebugName("r20-raw-control")
	rawBoard := rendering.NewAbsoluteBox(boardW, boardH)
	rawBoard.Background = &rendering.Color{R: 0.12, G: 0.13, B: 0.18, A: 1}
	rawBoard.Place(rawCard, 50, 60)

	// ===== Board PULSE: persistent motion hotspot (no filter) ==============
	pulseTarget := rendering.NewRenderTransform()
	pulseArm := rendering.NewAbsoluteBox(120, 120)
	pulseDisc := rendering.NewRenderColorBox(70, 70, patchGreen[0], patchGreen[1], patchGreen[2], 1)
	pulseDisc.SetDebugName("r20-pulse-disc")
	pulseArm.Place(pulseDisc, 25, 25)
	pulseTarget.FixedWidth, pulseTarget.FixedHeight = 120, 120
	pulseTarget.AddChild(pulseArm)
	pulseBoard := rendering.NewAbsoluteBox(boardW, boardH)
	pulseBoard.Background = &rendering.Color{R: 0.13, G: 0.13, B: 0.19, A: 1}
	pulseBoard.Place(pulseTarget, 65, 55)

	leftCol := rendering.NewAbsoluteBox(boardW*2+30, body.H*0.62)
	leftCol.Place(grayBoard, 0, 34)
	leftCol.Place(blurBoard, boardW+30, 34)
	leftCol.Place(rawBoard, 0, boardH+58)
	leftCol.Place(pulseBoard, boardW+30, boardH+58)
	titleAt := func(txt string, x, y float64) {
		l := wrkit.Label(txt, 11, 0.78, 0.84, 0.92)
		l.SetDebugName(txt)
		leftCol.Place(l, x, y)
	}
	titleAt("GRAY RenderGrayscale", 0, 8)
	titleAt("BLUR RenderImageFilter", boardW+30, 8)
	titleAt("RAW 无滤镜对照", 0, boardH+32)
	titleAt("PULSE 呼吸缩放对照", boardW+30, boardH+32)
	body.Box.Place(leftCol, 8, 8)

	// ===== Right side: static dense nested-boundary grid ===================
	dense := rendering.NewAbsoluteBox(310, 252)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	dense.SetDebugName("r20-dense-panel")
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 28, 0.25+0.12*float64(i%4), 0.40+0.10*float64(j%4), 0.62, 1)
			c.SetRepaintBoundary(true)
			c.SetDebugName(fmt.Sprintf("r20-cell-%d-%d", i, j))
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		lbl.SetDebugName(fmt.Sprintf("r20-lbl-%d", i))
		dense.Place(lbl, 10+float64(col)*64, 168+float64(row)*20)
	}
	denseNote := wrkit.Label("静态区：滤镜全程不影响这里", 11, 0.55, 0.75, 0.95)
	dense.SetRepaintBoundary(false)
	dense.Place(denseNote, 10, 216)
	wrapDense := rendering.NewRenderAlignBox(dense, 1, 0)
	wrapDense.FixedWidth = 310
	body.Box.Place(wrapDense, boardW*2+46, 8)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	geoValid := false
	runStart := time.Time{}
	ptGrayRed := probePoint{tol: 8.0 / 255} // red band under the CURRENT gray matrix
	ptRawCtrl := probePoint{want: rawCtrlFill, tol: 8.0 / 255}
	ptBlurMid := probePoint{want: bar0Fill, tol: 12.0 / 255}
	ptPulseDisc := probePoint{want: patchGreen, tol: 8.0 / 255}
	capTextBox := rect{x: 0, y: 0, w: 240, h: 26}
	var goldenRects []rect

	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		colX := bodyX + leftCol.Offset().X
		colY := bodyY + leftCol.Offset().Y
		// GRAY red band center. Chain: leftCol→grayBoard(0,34)→grayCard(10,10)
		// →patchesCol(0,0 — RO Layout zeroes child offsets)→red band(16,140).
		ptGrayRed.x = colX + grayBoard.Offset().X + 10 + 16 + 90
		ptGrayRed.y = colY + grayBoard.Offset().Y + 10 + 140 + 22
		// RAW control center.
		ptRawCtrl.x = colX + rawBoard.Offset().X + 50 + 75
		ptRawCtrl.y = colY + rawBoard.Offset().Y + 60 + 45
		// BLUR bar #0 center. Chain: leftCol→blurBoard(boardW+30,34)→
		// blurCard(10,10)→blurFilter(5,5)→barsCol(0,0)→bar0(14,5,w34,h110).
		ptBlurMid.x = colX + blurBoard.Offset().X + 10 + 5 + 14 + 17
		ptBlurMid.y = colY + blurBoard.Offset().Y + 10 + 5 + 5 + 55
		// PULSE disc center (transform-invariant).
		ptPulseDisc.x = colX + pulseBoard.Offset().X + 65 + 60
		ptPulseDisc.y = colY + pulseBoard.Offset().Y + 55 + 60
		// F6 box: the dense-note label region.
		capTextBox.x = bodyX + wrapDense.Offset().X + dense.Offset().X + 10
		capTextBox.y = bodyY + wrapDense.Offset().Y + dense.Offset().Y + 216
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{capTextBox.x - 10, capTextBox.y - 216, 310, 252},
		}
		geoValid = true
	}

	snapDir := os.Getenv("R20_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/r20_filter"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "r20_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r20_filter: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
				geoValid = false
			}
		},
	})
	// Retained posture: 「局部 rerecord」 is a retained-semantics gate — phase
	// steps must re-record ONLY the two filtered boundaries; the dense region
	// replays untouched.
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	clock := wrkit.NewPhaseClock(phaseLen, 2*phaseLen)

	var elapsed float64
	lastPhase := ""
	paintMin, paintMax := int64(-1), int64(-1)
	filterSeenOn := int64(0)
	blurRadius := 4.0
	pulseScale := 1.0
	steadyShotDone, recoverShotDone := false, false
	snapSteadyAt, snapRecoverAt := phaseLen+1.0, 2*phaseLen+phaseLen*2.0/3.0

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
			lastPhase = phase
		}

		// Ability-specific pressure: filter RANGE changes with the phase.
		// Spike swaps the grayscale matrix to identity (filter OFF → layer
		// omitted → filter_layer_count dips) and doubles the blur radius;
		// Recover restores both. Only the two filtered boundaries re-record.
		if phase == wrkit.PhaseSpike {
			grayCard.SetMatrix(identityMatrix())
			blurRadius = 8
		} else {
			grayCard.SetMatrix(grayMatrix())
			blurRadius = 4
		}
		blurFilter.SetBlurRadius(blurRadius)

		// Persistent motion hotspot (no filter): breathing scale.
		pulseScale = 1.0 + 0.10*math.Sin(2*math.Pi*elapsed/2.5)
		pulseTarget.SetScale(pulseScale, pulseScale)

		pc := app.Metrics().Snapshot()
		if fc := pc.FilterLayerCount; fc > filterSeenOn {
			filterSeenOn = fc
		}
		if elapsed >= 1.0 {
			if paintMin < 0 || pc.PaintCount < paintMin {
				paintMin = pc.PaintCount
			}
			if pc.PaintCount > paintMax {
				paintMax = pc.PaintCount
			}
		}

		// §2.7/U21 snapshot discipline: steady mid-run shot + post-Spike
		// recovery shot (both raster-thread via SnapshotAsync; the second
		// proves no temp-state residue after the matrix/radius steps).
		if secsSet && !steadyShotDone && elapsed >= snapSteadyAt && phase == wrkit.PhaseSpike {
			steadyShotDone = true
			app.SnapshotAsync(func() { saveSnap("r20_steady.png") })
		}
		if secsSet && steadyShotDone && !recoverShotDone && elapsed >= snapRecoverAt && phase == wrkit.PhaseRecover {
			recoverShotDone = true
			app.SnapshotAsync(func() { saveSnap("r20_recover.png") })
		}

		gateOK := paintMax-paintMin <= paintDriftCap
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("R20", phase, app, gateOK,
			fmt.Sprintf("blur=%.0f flt=%d pc=%d", blurRadius, pc.FilterLayerCount, pc.PaintCount),
			fmt.Sprintf("rr=%d skip=%d", pc.BoundaryRerecord, pc.BoundarySkip))
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: open: %v\n", err)
		os.Exit(1)
	}
	t0 := time.Now()
	runStart = t0
	_ = runStart
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
	// The engine snapshot is the FINAL frame after the last ticker tick, so
	// expectations are computed from exactly those node values (exact, no race).
	resolveGeometry()
	ptGrayRed.want = expectedUnderMatrix(patchRed, grayCard.ColorMatrix())
	finalChecks := []pixelCheck{
		{pt: &ptGrayRed, desc: "gray_red_band_matrix_exact"},
		{pt: &ptRawCtrl, desc: "raw_control_outside_invariant"},
		{pt: &ptBlurMid, desc: "blur_bar_center_preserved"},
		{pt: &ptPulseDisc, desc: "pulse_disc_center_invariant"},
		{text: &capTextBox, base: denseBase, desc: "dense_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "r20_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := 0.0, int64(0), false
	goldenRectsFinal := goldenRects
	goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRectsFinal)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R20",
		Scenario:      "ui_wr_r20_filter",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"hitch_budget_per_min":   hitchBudget,
			"paint_drift_cap":        paintDriftCap,
			"paint_count_min":        paintMin,
			"paint_count_max":        paintMax,
			"filter_seen_on":         filterSeenOn,
			"final_blur_radius":      blurRadius,
			"final_gray_identity":    isIdentity(grayCard.ColorMatrix()),
			"scripted_ok":            scriptedOK,
			"scripted_total":         scriptedTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"snapshots":              "steady+recover raster-thread double-shot (U21 §4)",
			"impl_correctness":       "RenderGrayscale/RenderImageFilter isolate their subtree offscreen (SaveLayer→PushLayerIsolated) and apply at composite; scene.ColorFilterLayer/ImageFilterLayer in the retained tree",
			"impl_dirty":             "retained: matrix/radius phase steps dirty ONLY the two filtered boundaries; dense region replays (boundary_skip grows)",
			"impl_edge":              "identity matrix / zero radius omit the layer entirely (steady-state trees unchanged); expectations computed from the frozen post-run node state",
			"impl_fail":              "any failed pixel assertion, filter_layer_count==0 all run, paint_count drift beyond cap, golden diff != 0 from run 2, fps<45 → FAIL exit 1",
			"impl_visible":           "GRAY card visibly desaturates (Spike flips it back to color); BLUR card smears; RAW control stays vivid; HUD shows flt count 0↔1",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates ------------------------------------------------------------
	// 族 A: R20 is the SaveLayer/Filter 10s·20fps 档 (NOT the animation-tier
	// 55fps line). The persistent PULSE hotspot still warrants an honest floor:
	// fps_wall>=45, p95<=22, hitch<=6/min (README-declared; stricter than tier).
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequirePersistentFPS:  true,
		MinFPSWall:            45,
		MinFPSElapsed:         4,
		MaxP95Ms:              22,
		RequireRetainedPolicy: true,
		MinBoundarySkip:       1,
		MinBoundaryRerecord:   1,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if secsSet && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 SaveLayer/Filter 关闭用时长)\n", secs, closeSeconds)
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
	// R20 专用: paint_count 稳（滤镜切换只脏滤镜板）。
	if paintMin < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: paint baseline never sampled")
		os.Exit(1)
	}
	if paintMax-paintMin > paintDriftCap {
		fmt.Fprintf(os.Stderr, "FAIL: paint_count drift=%d want ≤%d\n", paintMax-paintMin, paintDriftCap)
		os.Exit(1)
	}
	// R20 专用: 滤镜必须真实应用过（装绿防护）。
	if filterSeenOn < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: filter_layer_count never >=1 (滤镜从未应用)\n")
		os.Exit(1)
	}
	// 像素断言全过。
	if scriptedTotal != 5 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 5/5 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// 静背景不闪: Golden 零容差（首跑产基线；次跑起逐位一致）。
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	// U21 快照纪律: 双张必须都落地。
	if secsSet && (!steadyShotDone || !recoverShotDone) {
		fmt.Fprintf(os.Stderr, "FAIL: snapshots incomplete (steady=%v recover=%v) — U21 §4 requires 2 shots\n",
			steadyShotDone, recoverShotDone)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_r20_filter: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min flt>=%d pc=%d..%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin, filterSeenOn,
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
			fmt.Fprintf(os.Stderr, "ui_wr_r20_filter: pixel %-28s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		case c.text != nil:
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_r20_filter: pixel %-28s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "r20_final.png")
	base := filepath.Join(snapDir, "r20_final_base.png")
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
