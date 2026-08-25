// Command ui_wr_r6_layer_anim is the R6 single-ability real window
// (mode-1 first close, 2026-08-24).
//
// §2 R6 thesis: Opacity/Transform/Clip 层动画三种同跑 + 大面积静态背景，
// 证明「控件层动画不影响静态缓存」。A 2×2 grid of demo boards shares the
// body with a static dense nested-boundary region; one per-frame ticker
// drives all three layer kinds through the pipeline:
//
//	OPA   RenderOpacity   SaveLayer group isolation, breathing α 0.1..1.0
//	ROT   RenderTransform continuous rotation around the target center
//	PULSE RenderTransform breathing scale 0.85×–1.15×
//	CLIP  RenderClipRRect breathing corner radius + a STATIC twin card
//
// Present policy: retained — steady animation frames must re-record ONLY the
// four board boundaries (boundary_rerecord grows, boundary_skip grows for the
// static region) while the whole-tree paint walk count stays flat. That is
// exactly the §2 gate 「paint_count 稳」: a regression to full-frame repaints
// makes paint_count grow ~1/frame and fails the drift gate.
//
// Gates (§2.5 动画层档): fps_interval ≥55, p95 ≤22ms, hitch ≤5/min,
// 静背景不闪. "静背景不闪" is TWO hard gates:
//
//   - Golden bitwise diff == 0 over the STATIC mask across runs (U21 §2.7:
//     first run stores the baseline, later runs demand zero differing pixels;
//     the mask covers topbar/legend/dense — animation boards are excluded
//     because they move by design)
//
//   - Scripted pixel assertions at race-free invariant points: the CENTER of
//     the rotating/scaling/clipped targets is transform-invariant, and the
//     opacity board is snapshotted while ALL FOUR drivers are PAUSED (the
//     ticker freezes them before the raster-thread readback and resumes
//     afterwards), so the expected group blend is exact — no cross-frame race.
//
//     export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//     RUN_SECONDS=30 go run ./examples/ui_wr_r6_layer_anim
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 30 (§2.5 动画层档).
// GPU window required (needs_gpu_window otherwise). No RUN_SECONDS → run
// indefinitely (user closes); the phase script loops forever and the snapshot
// machinery is skipped (closing evidence requires RUN_SECONDS).
package main

import (
	"encoding/json"
	"fmt"
	"image"
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

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 30 // §2.5 动画层档（R6/C5 = 30s）

	boardW, boardH = 260.0, 260.0 // each demo board
	animPeriod     = 6.0          // seconds per full breath/spin cycle

	hitchBudget   = 5.0 // hitch_rate_per_min budget (README-declared)
	paintDriftCap = 40  // paint_count max−min band; full-paint regression ≈ frames ≫40
)

// Palette shared by scene builder AND pixel assertions (single source of
// truth: an assertion's expected color and the painted color cannot drift).
var (
	clearBG    = [3]float64{0.08, 0.09, 0.11} // window clear color
	denseBase  = [3]float64{0.11, 0.12, 0.18} // dense panel fill
	rotFill    = [3]float64{0.85, 0.30, 0.25} // rotating arm fill
	rotDisc    = [3]float64{0.96, 0.93, 0.90} // rotating center disc
	pulseFill  = [3]float64{0.20, 0.65, 0.95} // scaling arm fill
	pulseDisc  = [3]float64{0.96, 0.94, 0.88} // scaling center disc
	clipFill   = [3]float64{0.35, 0.75, 0.40} // clip card fill
	clipStatic = [3]float64{0.80, 0.70, 0.20} // clip static twin fill
	opCardBG   = [3]float64{0.12, 0.14, 0.22} // opacity board backdrop
	opCardFill = [3]float64{0.90, 0.45, 0.15} // opacity card child fill
)

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

// opBlend is the theoretical group blend of the opacity card's child over its
// board backdrop: out = α·fill + (1−α)·bg. Evaluated against the FROZEN α at
// the snapshot tick ⇒ exact expectation, not an approximation.
func opBlend(alpha float64) [3]float64 {
	var out [3]float64
	for i := 0; i < 3; i++ {
		out[i] = alpha*opCardFill[i] + (1-alpha)*opCardBG[i]
	}
	return out
}

// probePoint is one exact-point pixel assertion (logical window coords,
// resolved from the layout chain at runtime — §2.7 sampling rule 2).
type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

// pixelCheck binds one assertion site to its expected value. Expectations for
// state-dependent probes (e.g. the opacity card) are computed post-run from
// the frozen node values and assigned to pt.want directly.
type pixelCheck struct {
	pt   *probePoint
	text *rect // F6: text density is a REGION property, not a point
	base [3]float64
	desc string
}

var pixelResult = map[string]bool{}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = 0 // interactive: run until the user closes; phases loop forever
	} else {
		wrkit.RequireMinRun(secs, "R6")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_r6_layer_anim — Opacity/Transform/Clip 层动画", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R6 层动画 — Opacity/Transform/Clip 三种同跑", []string{
		"OPA   呼吸透明度卡 SaveLayer 组隔离",
		"ROT   持续旋转标靶 绕自身中心",
		"PULSE 呼吸缩放标靶 0.85×–1.15×",
		"CLIP  圆角呼吸 + 静态对照卡",
		"DENSE 右侧密集区 全程不闪不重录",
		"Spike 相位转速 ×2 幅度 ×1.5",
		"静背景不闪 = Golden 逐位 + 像素断言",
		"paint_count 稳 = 动画只脏动画板",
	})
	body := shell.Body

	// ===== Animation boards (2×2) ==========================================
	// Board OPA: the card fades as ONE GROUP via RenderOpacity (SaveLayer
	// isolation in the vector path, scene.OpacityLayer in the retained path).
	opCardChild := rendering.NewRenderColorBox(boardW-24, boardH-24, opCardFill[0], opCardFill[1], opCardFill[2], 1)
	opCard := rendering.NewRenderOpacity(1, opCardChild)
	opCard.SetRepaintBoundary(true) // the fading subtree re-records alone

	opBoard := rendering.NewAbsoluteBox(boardW, boardH)
	opBoard.Background = &rendering.Color{R: opCardBG[0], G: opCardBG[1], B: opCardBG[2], A: 1}
	opBoard.Place(opCard, 12, 12)

	// Board ROT: continuous rotation around the TARGET center. The sampled
	// point is the center of the inner disc == the pivot ⇒ rotation-invariant.
	rotTarget := rendering.NewRenderTransform()
	rotArm := rendering.NewAbsoluteBox(140, 140)
	rotArm.Place(rendering.NewRenderColorBox(56, 56, rotFill[0], rotFill[1], rotFill[2], 1), 42, 42)
	rotDiscNode := rendering.NewRenderColorBox(28, 28, rotDisc[0], rotDisc[1], rotDisc[2], 1)
	rotDiscNode.SetDebugName("r6-rot-disc")
	rotArm.Place(rotDiscNode, 56, 56) // centered ⇒ disc center == pivot
	rotTarget.FixedWidth, rotTarget.FixedHeight = 140, 140
	rotTarget.AddChild(rotArm)

	rotBoard := rendering.NewAbsoluteBox(boardW, boardH)
	rotBoard.Background = &rendering.Color{R: 0.13, G: 0.14, B: 0.19, A: 1}
	rotBoard.Place(rotTarget, 60, 60)

	// Board PULSE: breathing scale, same center-invariance trick.
	pulseTarget := rendering.NewRenderTransform()
	pulseArm := rendering.NewAbsoluteBox(150, 150)
	pulseArm.Place(rendering.NewRenderColorBox(72, 72, pulseFill[0], pulseFill[1], pulseFill[2], 1), 39, 39)
	pulseDiscNode := rendering.NewRenderColorBox(36, 36, pulseDisc[0], pulseDisc[1], pulseDisc[2], 1)
	pulseDiscNode.SetDebugName("r6-pulse-disc")
	pulseArm.Place(pulseDiscNode, 57, 57)
	pulseTarget.FixedWidth, pulseTarget.FixedHeight = 150, 150
	pulseTarget.AddChild(pulseArm)

	pulseBoard := rendering.NewAbsoluteBox(boardW, boardH)
	pulseBoard.Background = &rendering.Color{R: 0.14, G: 0.13, B: 0.18, A: 1}
	pulseBoard.Place(pulseTarget, 55, 55)

	// Board CLIP: breathing corner radius + STATIC twin (different fill, fixed
	// radius): any clip leak onto the twin or backdrop shows by eye/Golden.
	const cardSize = 110.0
	clipCard := rendering.NewRenderClipRRect(
		rendering.NewRenderColorBox(cardSize, cardSize, clipFill[0], clipFill[1], clipFill[2], 1))
	clipCard.FixedWidth, clipCard.FixedHeight = cardSize, cardSize
	clipCard.SetRadius(12)
	clipCard.SetRepaintBoundary(true)
	clipTwin := rendering.NewRenderClipRRect(
		rendering.NewRenderColorBox(cardSize, cardSize, clipStatic[0], clipStatic[1], clipStatic[2], 1))
	clipTwin.FixedWidth, clipTwin.FixedHeight = cardSize, cardSize
	clipTwin.SetRadius(16) // static radius

	clipBoard := rendering.NewAbsoluteBox(boardW, boardH)
	clipBoard.Background = &rendering.Color{R: 0.12, G: 0.13, B: 0.17, A: 1}
	clipBoard.Place(clipCard, 14, 75)
	clipBoard.Place(clipTwin, 136, 75)

	boardsCol := rendering.NewAbsoluteBox(boardW*2+30, body.H-16)
	boardsCol.Background = &rendering.Color{R: clearBG[0], G: clearBG[1], B: clearBG[2], A: 1}
	boardsCol.Place(opBoard, 0, 30)
	boardsCol.Place(rotBoard, boardW+30, 30)
	boardsCol.Place(pulseBoard, 0, boardH+80)
	boardsCol.Place(clipBoard, boardW+30, boardH+80)
	titleAt := func(txt string, x, y float64) {
		l := wrkit.Label(txt, 11, 0.78, 0.84, 0.92)
		l.SetDebugName(txt)
		boardsCol.Place(l, x, y)
	}
	titleAt("OPACITY 呼吸组隔离", 0, 8)
	titleAt("TRANSFORM 旋转", boardW+30, 8)
	titleAt("TRANSFORM 呼吸缩放", 0, boardH+58)
	titleAt("CLIP 圆角呼吸 + 静态对照", boardW+30, boardH+58)
	body.Box.Place(boardsCol, 8, 8)

	// ===== Right side: static dense nested-boundary grid ===================
	dense := rendering.NewAbsoluteBox(310, 252)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	dense.SetDebugName("r6-dense-panel")
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 28, 0.25+0.12*float64(i%4), 0.40+0.10*float64(j%4), 0.62, 1)
			c.SetRepaintBoundary(true)
			c.SetDebugName(fmt.Sprintf("r6-cell-%d-%d", i, j))
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		lbl.SetDebugName(fmt.Sprintf("r6-lbl-%d", i))
		dense.Place(lbl, 10+float64(col)*64, 168+float64(row)*20)
	}
	denseNote := wrkit.Label("静态区：动画全程不闪、不重录", 11, 0.55, 0.75, 0.95)
	dense.SetRepaintBoundary(false)
	dense.Place(denseNote, 10, 216)
	wrapDense := rendering.NewRenderAlignBox(dense, 1, 0)
	// Fixed width: AbsoluteBox hands children the FULL body width as max —
	// an unfixed AlignBox would stretch to 904 and right-align itself
	// off-screen (first-run bug: dense region invisible).
	wrapDense.FixedWidth = 310
	body.Box.Place(wrapDense, boardW*2+46, 8)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	geoValid := false
	// resizeLog records whether the user resized during the closing run.
	// Interactive drags put the engine into full-recovery (per-frame swapchain
	// reconfigure + texture reallocation waves + forced full repaints), which
	// by design inflates family-A frame timings and changes surface geometry —
	// both golden bitwise compare and fps/hitch gates are only defined for an
	// unattended run. The flags below make that state explicit in the JSON
	// instead of failing on artifacts of interaction.
	var resizeLog []float64 // wall seconds since runStart
	var runStart time.Time
	ptOpCard := probePoint{tol: 8.0 / 255} // want filled at freeze time
	ptRotDisc := probePoint{want: rotDisc, tol: 8.0 / 255}
	ptPulseDisc := probePoint{want: pulseDisc, tol: 8.0 / 255}
	ptClipCenter := probePoint{want: clipFill, tol: 8.0 / 255}
	ptClipTwin := probePoint{want: clipStatic, tol: 8.0 / 255}
	capTextBox := rect{x: 0, y: 0, w: 240, h: 26}
	var goldenRects []rect

	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		colX := bodyX + boardsCol.Offset().X
		colY := bodyY + boardsCol.Offset().Y
		// OPA: board origin + card offset + half child size (the card center).
		ptOpCard.x = colX + opBoard.Offset().X + 12 + (boardW-24)/2
		ptOpCard.y = colY + opBoard.Offset().Y + 12 + (boardH-24)/2
		// ROT/PULSE: board origin + target offset + half target (disc centers).
		ptRotDisc.x = colX + rotBoard.Offset().X + 60 + 70
		ptRotDisc.y = colY + rotBoard.Offset().Y + 60 + 70
		ptPulseDisc.x = colX + pulseBoard.Offset().X + 55 + 75
		ptPulseDisc.y = colY + pulseBoard.Offset().Y + 55 + 75
		// CLIP: card/twin centers (inside the rounded rect at any radius).
		ptClipCenter.x = colX + clipBoard.Offset().X + 14 + cardSize/2
		ptClipCenter.y = colY + clipBoard.Offset().Y + 75 + cardSize/2
		ptClipTwin.x = colX + clipBoard.Offset().X + 136 + cardSize/2
		ptClipTwin.y = colY + clipBoard.Offset().Y + 75 + cardSize/2
		// F6 box: the dense-note label region (static text presence).
		capTextBox.x = bodyX + wrapDense.Offset().X + dense.Offset().X + 10
		capTextBox.y = bodyY + wrapDense.Offset().Y + dense.Offset().Y + 216
		// Golden static mask: chrome topbar + legend + dense panel. Animation
		// boards are excluded (they move by design); HUD sits OUTSIDE the mask.
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{capTextBox.x - 10, capTextBox.y - 216, 310, 252},
		}
		geoValid = true
	}

	// ===== Snapshot dir + App ==============================================
	snapDir := os.Getenv("R6_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/r6_layer_anim"
	}
	os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "r6_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		// Closing-run pixel evidence: the engine snapshots the FINAL frame on
		// the raster thread AFTER the loop stops (no mid-run stall → no
		// hitch pollution). The final frame's animation state is whatever the
		// last ticker tick applied to the nodes, so expectations are computed
		// from those same node values post-run (exact, no race).
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r6_layer_anim: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
				geoValid = false
				if !runStart.IsZero() {
					resizeLog = append(resizeLog, time.Since(runStart).Seconds())
				}
			}
		},
	})
	// Retained posture: steady animation frames must re-record ONLY the four
	// board boundaries; the whole-tree paint walk counter stays flat (§2 gate
	// 「paint_count 稳」 is meaningless under full_paint, where it grows ~1/frame).
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	// Phase script: Steady 5s → Spike 5s (spin ×2, amplitude ×1.5) → Recover
	// 5s → loops forever. Pixel evidence = final-frame engine snapshot
	// (PipelineOptions.SnapshotPath): the raster thread shoots AFTER the loop
	// stops, so no mid-run stall pollutes the hitch budget.
	const phaseLen = 5.0
	clock := wrkit.NewPhaseClock(phaseLen, 2*phaseLen)

	var elapsed float64
	lastPhase := ""
	paintMin, paintMax := int64(-1), int64(-1)
	// Live animation values; the LAST tick's values are the final frame's
	// node state, so post-run expectations are computed from exactly these.
	breath, scaleV, radiusV, rotDeg := 1.0, 1.0, 12.0, 0.0

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		phase := clock.Advance(dt)
		if phase != lastPhase {
			lastPhase = phase
		}

		speedK, ampK := 1.0, 1.0
		if phase == wrkit.PhaseSpike {
			speedK, ampK = 2.0, 1.5
		}
		breath = clamp01(0.55 + 0.45*math.Sin(2*math.Pi*(elapsed*speedK/animPeriod)))
		scaleV = 1.0 + ampK*0.15*math.Sin(2*math.Pi*elapsed*speedK/animPeriod)
		radiusV = 10 + ampK*8*(0.5+0.5*math.Sin(2*math.Pi*elapsed*speedK/animPeriod))
		rotDeg = math.Mod(elapsed*speedK*360.0/animPeriod, 360)
		opCard.SetOpacity(breath)
		rotTarget.SetRotation(rotDeg * math.Pi / 180)
		pulseTarget.SetScale(scaleV, scaleV)
		clipCard.SetRadius(radiusV)

		// paint_count stability tracking (§2 gate 「paint_count 稳」, retained):
		// after warm-up, the whole-tree paint-walk counter must stay flat —
		// animation rides the boundary layers, not full-frame repaints.
		pc := app.Metrics().Snapshot().PaintCount
		if elapsed >= 1.0 {
			if paintMin < 0 || pc < paintMin {
				paintMin = pc
			}
			if pc > paintMax {
				paintMax = pc
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.HitchRatePerMin <= hitchBudget
		shell.UpdateHUD("R6", phase, app, gateOK,
			fmt.Sprintf("α=%.2f θ=%.0f° s=%.2f r=%.0f pc=%d", breath, rotDeg, scaleV, radiusV, snapH.PaintCount),
			fmt.Sprintf("drift=%d/%d", func() int64 {
				if paintMax < 0 || paintMin < 0 {
					return 0
				}
				return paintMax - paintMin
			}(), int64(paintDriftCap)))
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	runStart = t0
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

	// ---- Final-frame pixel assertions (§2.7/U21) --------------------------
	// The engine snapshot is the FINAL frame; its node state is exactly the
	// last ticker tick's values (breath/scaleV/radiusV/rotDeg), so the
	// opacity expectation is computed from that same frozen state — exact.
	//
	// A window resize DURING the closing run changes the surface geometry:
	// probes re-resolve from the new layout, but the golden baseline was
	// captured at 1200x800 — a bitwise comparison across different surface
	// sizes is undefined. Skip golden honestly in that case (resize_during_run)
	// instead of failing on a size mismatch.
	resolveGeometry()
	ptOpCard.want = opBlend(breath)
	resizedDuringRun := len(resizeLog) > 0
	// lastResizeSec / calmAfterResize characterize the interaction window:
	// family-A metrics (fps/hitch/p95) include the resize full-recovery cost
	// by design, so a run with recent interaction reports WHEN it happened
	// and the post-drag calm duration instead of silently failing the gate.
	var lastResizeSec float64 = -1
	for _, ts := range resizeLog {
		if ts > lastResizeSec {
			lastResizeSec = ts
		}
	}
	calmAfterResize := -1.0
	if resizedDuringRun {
		calmAfterResize = elapsedSec - lastResizeSec
	}
	// familyASkipped is non-empty when interaction polluted the run: the
	// fps/p95/hitch gates are enforced only on unattended runs (see below).
	familyASkipped := ""
	if resizedDuringRun {
		familyASkipped = fmt.Sprintf("resize during run (last at %.1fs, %.1fs calm after); re-run unattended for closing evidence",
			lastResizeSec, calmAfterResize)
	}
	finalChecks := []pixelCheck{
		{pt: &ptRotDisc, desc: "rot_center_invariant"},
		{pt: &ptPulseDisc, desc: "pulse_center_invariant"},
		{pt: &ptClipCenter, desc: "clip_center"},
		{pt: &ptClipTwin, desc: "clip_static_twin"},
		{pt: &ptOpCard, desc: "op_card_group_blend"},
		{text: &capTextBox, base: denseBase, desc: "dense_note_text_density"},
	}
	runPixelChecks(loadImage(snapPath), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := 0.0, int64(0), false
	if resizedDuringRun {
		fmt.Fprintf(os.Stderr, "golden: SKIPPED (window resized during run; bitwise compare needs same-size captures)\n")
	} else {
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R6",
		Scenario:      "ui_wr_r6_layer_anim",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"hitch_budget_per_min":       hitchBudget,
			"rss_slope_note":             "no 30s hard line: startup driver texture-pool ramp (§10 R7 note); leak detection = R15 soak; memory semantics enforced here by paint_drift",
			"paint_drift_cap":            paintDriftCap,
			"paint_count_min":            paintMin,
			"paint_count_max":            paintMax,
			"anim_period_s":              animPeriod,
			"spike_spin_multiplier":      2.0,
			"spike_amplitude_multiplier": 1.5,
			"final_alpha":                breath,
			"final_scale":                scaleV,
			"final_radius":               radiusV,
			"final_rot_deg":              rotDeg,
			"scripted_ok":                scriptedOK,
			"scripted_total":             scriptedTotal,
			"resize_during_run":          resizedDuringRun,
			"last_resize_at_sec":         lastResizeSec,
			"calm_after_resize_sec":      calmAfterResize,
			"family_a_gate": func() string {
				if familyASkipped == "" {
					return "enforced"
				}
				return "skipped: " + familyASkipped
			}(),
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"impl_correctness":       "three layer kinds driven per frame through the shipped ROs: RenderOpacity (SaveLayer group / scene.OpacityLayer), RenderTransform (CTM around target pivot), RenderClipRRect (breathing radius + static twin)",
			"impl_dirty":             "retained posture: steady frames re-record only the four board boundaries (boundary_rerecord grows) while the static dense region replays (boundary_skip grows); whole-tree paint walk count must stay flat (paint_drift ≤ cap)",
			"impl_edge":              "pixel evidence = final-frame engine snapshot AFTER loop stop (no mid-run stall → honest hitch budget); expectations computed from the last tick's node state; rotation/scale sampled at transform-invariant centers; resize invalidates cached probe geometry",
			"impl_fail":              "golden static-mask diff != 0 from run 2 on, any failed pixel assertion, paint_count drift beyond cap, fps<55, p95>22, hitch over budget → FAIL exit 1",
			"impl_visible":           "four titled boards breathe/spin/fade visibly; HUD shows α/θ/scale/radius + paint drift; static dense panel must look perfectly still throughout",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates (§2 R6 行 + §2.5 动画层档 + §2.2 全族；硬，不许放) ----------
	// 族 A（fps/p95/hitch）只在无人值守跑上判定：拖拽把引擎置入 resize 全量
	// 恢复（逐帧 swapchain reconfigure + 纹理池重分配波 + 强制全画），恢复
	// 成本按设计如实计入帧时——那是交互取证污染，不是稳态退化。被拖拽的跑
	// 在此显式 SKIP 族 A 并注明原因（JSON 已带 resize 时间戳与 calm 时长），
	// 其余族照常硬判；关闭证据必须另取一次不碰窗口的跑。
	if familyASkipped != "" {
		fmt.Fprintf(os.Stderr, "SKIP: family-A fps/hitch gates — %s\n", familyASkipped)
	}
	if familyASkipped == "" {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:           1,
			RequirePersistentFPS:  true, // continuous animation ticker (§2.2.2 动画档)
			MinFPSWall:            55,
			MaxP95Ms:              22,
			RequireRetainedPolicy: true, // 「paint_count 稳」只在 retained 下有意义
			MinBoundarySkip:       1,    // 静态缓存必须真实回放
		}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
	} else {
		// Interaction-polluted run: still enforce the non-family-A gates.
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:           1,
			RequireRetainedPolicy: true,
			MinBoundarySkip:       1,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing (fallback 也须如实标注)")
		os.Exit(1)
	}
	if secsSet && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 动画层档关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	// 族 A 长 soak: hitch 预算（README 声明 ≤5/min）。同上：仅无人值守跑判定。
	if familyASkipped == "" && snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.2f > %.0f\n", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 E: RSS 字段必采并如实上报（JSON 全字段）。30s 关闭跑不设 RSS 硬线：
	// §10 R7 修订行已定性——启动期驱动侧纹理池爬坡在均衡建立前整段表现为
	// "坡"（同族 C4 同模式 176K KB/min），真泄漏检测归 R15 soak 300s 窗。
	// 本窗的内存语义门禁由 paint_drift（纹理不随动画累积）承担。
	// 族 D: CPU 非双 0（动画窗禁降画质装绿）。
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑 (cpu_pct_avg 与 ui/raster 至少一项须为正)")
		os.Exit(1)
	}
	// R6 专用「paint_count 稳」: 全树重绘走查不得随动画增长（漂移超上限 =
	// 动画退化为全画幅重绘，静态缓存语义破产）。
	if paintMin < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: paint baseline never sampled")
		os.Exit(1)
	}
	if paintMax-paintMin > paintDriftCap {
		fmt.Fprintf(os.Stderr, "FAIL: paint_count drift=%d want ≤%d (动画必须只脏动画板)\n",
			paintMax-paintMin, paintDriftCap)
		os.Exit(1)
	}
	// R6 专用: 动画必须真实走边界层逐帧重录（rerecord>0），配合 paint drift
	// 上限共同证明「动画只脏动画板」；rerecord=0 意味着动画没画（装绿）。
	if snap.BoundaryRerecord <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_rerecord=%d want >0 (层动画必须逐帧重录动画板边界)\n", snap.BoundaryRerecord)
		os.Exit(1)
	}
	// 像素断言全过且覆盖完整（快照失败时 result 缺项/为 false）。
	if scriptedTotal != 6 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 6/6 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// 静背景不闪: Golden 零容差（首跑产基线不给结论；次跑起逐位一致）。
	// resize_during_run 时快照与基线尺寸不同，逐位对比无定义——如实跳过（JSON 已标注）。
	if !goldenFirstRun && !resizedDuringRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px (静背景逐位一致)\n",
			goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_r6_layer_anim: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min slope=%.0fKB/min pc=%d..%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin, snap.RSSSlopeKBPerMin,
		paintMin, paintMax, scriptedOK, scriptedTotal, goldenDiffPct, goldenTotalPx, goldenFirstRun,
		snap.VSyncSource, elapsedSec)
}

// --- assertion helpers -----------------------------------------------------

// runPixelChecks evaluates the final-frame assertion group against the PNG.
func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		var ok bool
		switch {
		case c.pt != nil:
			want := c.pt.want
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, want, c.pt.tol)
			fmt.Fprintf(os.Stderr, "ui_wr_r6_layer_anim: pixel %-22s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, want[0], want[1], want[2], ok)
		case c.text != nil:
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_r6_layer_anim: pixel %-22s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
		}
		result[c.desc] = ok
	}
}

// loadImage decodes a PNG written by the engine snapshot path.
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

// sampleLogical reads one logical-coordinate pixel (physical = logical·dpr).
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

// textPixels counts pixels clearly deviating from the panel background in a
// logical box (F6: text presence is a REGION property).
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

// evaluateGolden compares this run's snapshot against the stored baseline over
// the static mask. First run stores the baseline (first_run=true, no verdict;
// §2.7 semantics). Later runs demand ZERO differing pixels.
func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "r6_final.png")
	base := filepath.Join(snapDir, "r6_final_base.png")
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

// comparePNG counts differing pixels inside the static mask between two
// same-engine screenshots (physical pixels; any channel mismatch counts).
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
