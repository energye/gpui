// Command ui_wr_c5_anim_over_static is the C5 composite real window
// (mode-1 first close): R6 layer animations composited OVER the R3 static
// nested-boundary cache, under the R4 retained present policy.
//
// §3 C5 thesis: 层动画盖在静态缓存上 — a semi-transparent floating card
// (FLOAT) animates position+opacity ON TOP of the static dense region while
// three animation boards (OPA breathing opacity / ROT rotation / CLIP radius)
// run alongside. The static cache must keep replaying untouched: damage stays
// inside the animated boundaries, the dense region keeps skipping, and the
// golden static mask is bitwise-stable across runs.
//
// Gates (§3 C5 row + §2.2 族 A 动画层档): fps_interval ≥55, p95 ≤22ms,
// hitch ≤5/min, retained policy, boundary skip/rerecord split, paint_count
// flat, pixel assertions, golden bitwise zero-tolerance.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/ui_wr_c5_anim_over_static
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 30 (§2.5 动画层档,
// max(R6=30, R3=8, R4=15)). No RUN_SECONDS → interactive loop (pixel evidence
// off).
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
	closeSeconds = 30 // §2.5 动画层档：max(R6,R3,R4) = 30s

	boardW, boardH = 250.0, 250.0
	animPeriod     = 6.0 // seconds per full breath/spin cycle
	phaseLen       = 5.0 // Steady 5s → Spike 5s → Recover 5s loops

	hitchBudget   = 5.0 // per-min budget (§2.2.3 default, same as R6 动画层档)
	paintDriftCap = 40  // paint_count max−min band; full-paint regression ≫40
)

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG    = [3]float64{0.08, 0.09, 0.11}
	denseBase  = [3]float64{0.11, 0.12, 0.18}
	rotFill    = [3]float64{0.85, 0.30, 0.25}
	rotDisc    = [3]float64{0.96, 0.93, 0.90}
	opaFill    = [3]float64{0.20, 0.65, 0.95}
	opaBoardBG = [3]float64{0.12, 0.14, 0.22}
	clipFill   = [3]float64{0.35, 0.75, 0.40}
	floatFill  = [3]float64{0.90, 0.45, 0.15}
	denseCell  = [3]float64{0.37, 0.50, 0.72}
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

// blendOver is the exact group blend of a card over a solid backdrop:
// out = α·fill + (1−α)·bg. Evaluated against the frozen post-run α ⇒ exact.
func blendOver(fill, bg [3]float64, alpha float64) [3]float64 {
	var out [3]float64
	for i := 0; i < 3; i++ {
		out[i] = alpha*fill[i] + (1-alpha)*bg[i]
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
		wrkit.RequireMinRun(secs, "C5")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_c5_anim_over_static — 层动画盖在静态缓存上", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C5 层动画盖静态缓存 — R6+R3+R4", []string{
		"OPA   呼吸透明度卡 RenderOpacity",
		"ROT   持续旋转标靶 RenderTransform",
		"CLIP  圆角呼吸 RenderClipRRect",
		"FLOAT 半透明浮动卡 盖在 DENSE 缓存上方",
		"DENSE 右侧嵌套 boundary 静态缓存",
		"Spike 相位转速 ×2 幅度 ×1.3",
		"damage 只在动画区 静区照常 skip",
		"静背景不闪 = Golden 逐位 + 像素断言",
	})
	body := shell.Body

	// ===== Left column: R6 layer-animation boards ==========================
	boardsCol := rendering.NewAbsoluteBox(boardW*2+30, body.H-16)
	boardsCol.Background = &rendering.Color{R: clearBG[0], G: clearBG[1], B: clearBG[2], A: 1}

	// Board OPA: breathing group opacity (RenderOpacity).
	opaChild := rendering.NewRenderColorBox(boardW-40, boardH-40, opaFill[0], opaFill[1], opaFill[2], 1)
	opaCard := rendering.NewRenderOpacity(1, opaChild)
	opaCard.SetRepaintBoundary(true)
	opaBoard := rendering.NewAbsoluteBox(boardW, boardH)
	opaBoard.Background = &rendering.Color{R: opaBoardBG[0], G: opaBoardBG[1], B: opaBoardBG[2], A: 1}
	opaBoard.Place(opaCard, 20, 20)
	boardsCol.Place(opaBoard, 0, 34)
	titleAt := func(txt string, x, y float64) {
		l := wrkit.Label(txt, 11, 0.78, 0.84, 0.92)
		l.SetDebugName(txt)
		boardsCol.Place(l, x, y)
	}
	titleAt("OPA 呼吸组隔离", 0, 8)

	// Board ROT: continuous rotation around target center (transform-invariant probe).
	rotTarget := rendering.NewRenderTransform()
	rotArm := rendering.NewAbsoluteBox(140, 140)
	rotArm.Place(rendering.NewRenderColorBox(56, 56, rotFill[0], rotFill[1], rotFill[2], 1), 42, 42)
	rotDiscNode := rendering.NewRenderColorBox(28, 28, rotDisc[0], rotDisc[1], rotDisc[2], 1)
	rotArm.Place(rotDiscNode, 56, 56)
	rotTarget.FixedWidth, rotTarget.FixedHeight = 140, 140
	rotTarget.AddChild(rotArm)
	rotBoard := rendering.NewAbsoluteBox(boardW, boardH)
	rotBoard.Background = &rendering.Color{R: 0.13, G: 0.14, B: 0.19, A: 1}
	rotBoard.Place(rotTarget, 55, 55)
	boardsCol.Place(rotBoard, boardW+30, 34)
	titleAt("ROT 持续旋转", boardW+30, 8)

	// Board CLIP: breathing corner radius.
	const clipSize = 130.0
	clipCard := rendering.NewRenderClipRRect(
		rendering.NewRenderColorBox(clipSize, clipSize, clipFill[0], clipFill[1], clipFill[2], 1))
	clipCard.FixedWidth, clipCard.FixedHeight = clipSize, clipSize
	clipCard.SetRadius(12)
	clipCard.SetRepaintBoundary(true)
	clipBoard := rendering.NewAbsoluteBox(boardW, boardH)
	clipBoard.Background = &rendering.Color{R: 0.12, G: 0.13, B: 0.17, A: 1}
	clipBoard.Place(clipCard, 60, 60)
	boardsCol.Place(clipBoard, 0, boardH+58)
	titleAt("CLIP 圆角呼吸", 0, boardH+32)

	// Board HUD-note placeholder (keeps the 2×2 rhythm; live numbers live in
	// the bottom HUD bar).
	noteLbl := wrkit.Label("稳态帧只重录三块动画板\n静区 skip 只增", 11, 0.62, 0.72, 0.85)
	boardsCol.Place(noteLbl, boardW+45, boardH+80)

	body.Box.Place(boardsCol, 8, 8)

	// ===== Right side: static dense cache (R3), with FLOAT card on top =====
	dense := rendering.NewAbsoluteBox(310, 420)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	dense.SetDebugName("c5-dense-panel")
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 44, denseCell[0]*(1-0.08*float64(i)), denseCell[1]*(0.8+0.1*float64(j)), denseCell[2], 1)
			c.SetRepaintBoundary(true)
			c.SetDebugName(fmt.Sprintf("c5-cell-%d-%d", i, j))
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*52)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		lbl.SetDebugName(fmt.Sprintf("c5-lbl-%d", i))
		dense.Place(lbl, 10+float64(col)*64, 232+float64(row)*20)
	}
	denseNote := wrkit.Label("静态缓存：动画全程不闪、不重录", 11, 0.55, 0.75, 0.95)
	dense.SetRepaintBoundary(false)
	dense.Place(denseNote, 10, 280)
	wrapDense := rendering.NewRenderAlignBox(dense, 1, 0)
	wrapDense.FixedWidth = 310

	// FLOAT card: the C5-specific pressure — a semi-transparent animated card
	// layered ON TOP of the dense cache (z-order above). It covers solid color
	// cells only (no text under it) so the group blend has an exact backdrop;
	// its damage must stay inside its own band, and the golden mask routes
	// around its rect.
	floatHost := rendering.NewAbsoluteBox(310, 420)
	floatHost.Place(wrapDense, 0, 0)
	floatChild := rendering.NewRenderColorBox(130, 80, floatFill[0], floatFill[1], floatFill[2], 1)
	floatCard := rendering.NewRenderOpacity(0.75, floatChild)
	floatCard.SetRepaintBoundary(true)
	floatHost.Place(floatCard, 170, 60)
	floatHost.SetDebugName("c5-float-host")

	body.Box.Place(floatHost, boardW*2+46, 8)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	geoValid := false
	ptFloatCard := probePoint{tol: 12.0 / 255} // want filled from frozen α post-run
	ptRotDisc := probePoint{want: rotDisc, tol: 8.0 / 255}
	ptOpaCard := probePoint{tol: 12.0 / 255}
	ptClipCenter := probePoint{want: clipFill, tol: 8.0 / 255}
	// Dense cell (0,0) center — outside the FLOAT overlap area. Cell color
	// factors are (1-0.08*i, 0.8+0.1*j); at i=0,j=0 the green channel is ×0.8.
	ptDenseCell := probePoint{want: [3]float64{denseCell[0], denseCell[1] * 0.8, denseCell[2]}, tol: 8.0 / 255}
	capTextBox := rect{x: 0, y: 0, w: 240, h: 26}
	var goldenRects []rect

	// Backdrop colors for the exact group-blend expectations.
	cell31 := [3]float64{denseCell[0] * (1 - 0.08*3), denseCell[1] * (0.8 + 0.1*1), denseCell[2]} // under FLOAT

	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		colX := bodyX + boardsCol.Offset().X
		colY := bodyY + boardsCol.Offset().Y
		hostX := bodyX + floatHost.Offset().X
		hostY := bodyY + floatHost.Offset().Y

		// ROT disc center (board origin +55/+55, target pivot at 70,70).
		ptRotDisc.x = colX + rotBoard.Offset().X + 55 + 70
		ptRotDisc.y = colY + rotBoard.Offset().Y + 55 + 70
		// OPA card center (board origin +20, child half size).
		ptOpaCard.x = colX + opaBoard.Offset().X + 20 + (boardW-40)/2
		ptOpaCard.y = colY + opaBoard.Offset().Y + 20 + (boardH-40)/2
		// CLIP card center.
		ptClipCenter.x = colX + clipBoard.Offset().X + 60 + clipSize/2
		ptClipCenter.y = colY + clipBoard.Offset().Y + 60 + clipSize/2
		// FLOAT card center over solid cell (3,1): card at host +(170,60), 130×80.
		ptFloatCard.x = hostX + 170 + 65
		ptFloatCard.y = hostY + 60 + 40
		// DENSE cell (0,0) center — outside the FLOAT overlap area.
		ptDenseCell.x = hostX + 10 + 28
		ptDenseCell.y = hostY + 10 + 22
		// F6 box: the dense-note label region.
		capTextBox.x = hostX + 10
		capTextBox.y = hostY + 280
		// Golden static mask routes AROUND the FLOAT card rect (it breathes by
		// design): top band above the card, left strip, right sliver, bottom band.
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{hostX, hostY, 310, 60},
			{hostX, hostY, 170, 420},
			{hostX + 300, hostY, 10, 420},
			{hostX, hostY + 140, 310, 280},
		}
		geoValid = true
	}

	snapDir := os.Getenv("C5_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/c5_anim_over_static"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "c5_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c5_anim_over_static: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
				geoValid = false
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	clock := wrkit.NewPhaseClock(phaseLen, 2*phaseLen)

	var elapsed float64
	lastPhase := ""
	paintMin, paintMax := int64(-1), int64(-1)
	breath, rotDeg := 1.0, 0.0
	lastFloatAlpha, lastOpaAlpha := 0.75, 1.0 // frozen post-run node state

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		phase := clock.Advance(dt)
		if phase != lastPhase {
			lastPhase = phase
		}

		speedK, ampK := 1.0, 1.0
		if phase == wrkit.PhaseSpike {
			speedK, ampK = 2.0, 1.3
		}
		breath = clamp01(0.55 + 0.45*math.Sin(2*math.Pi*(elapsed*speedK/animPeriod)))
		rotDeg = math.Mod(elapsed*speedK*360.0/animPeriod, 360)
		lastOpaAlpha = breath
		opaCard.SetOpacity(breath)
		rotTarget.SetRotation(rotDeg * math.Pi / 180)
		clipCard.SetRadius(12 + ampK*8*(0.5+0.5*math.Sin(2*math.Pi*elapsed*speedK/animPeriod)))
		lastFloatAlpha = clamp01(0.55 + 0.25*math.Sin(2*math.Pi*elapsed*speedK/(animPeriod*1.5)))
		floatCard.SetOpacity(lastFloatAlpha)

		pc := app.Metrics().Snapshot()
		if elapsed >= 1.0 {
			if paintMin < 0 || pc.PaintCount < paintMin {
				paintMin = pc.PaintCount
			}
			if pc.PaintCount > paintMax {
				paintMax = pc.PaintCount
			}
		}

		gateOK := paintMax-paintMin <= paintDriftCap
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("C5", phase, app, gateOK,
			fmt.Sprintf("α=%.2f θ=%.0f° pc=%d", breath, rotDeg, pc.PaintCount),
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
	// expectations are computed from exactly those frozen node values.
	resolveGeometry()
	ptFloatCard.want = blendOver(floatFill, cell31, lastFloatAlpha)
	ptOpaCard.want = blendOver(opaFill, opaBoardBG, lastOpaAlpha)
	finalChecks := []pixelCheck{
		{pt: &ptFloatCard, desc: "float_card_group_blend"},
		{pt: &ptRotDisc, desc: "rot_center_invariant"},
		{pt: &ptOpaCard, desc: "opa_card_breathing"},
		{pt: &ptClipCenter, desc: "clip_center"},
		{pt: &ptDenseCell, desc: "dense_cell_under_float_host"},
		{text: &capTextBox, base: denseBase, desc: "dense_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "c5_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C5",
		Scenario:      "ui_wr_c5_anim_over_static",
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
			"scripted_ok":            scriptedOK,
			"scripted_total":         scriptedTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"covered":                "R6 layer anims (opacity/transform/clip) + R3 nested boundary cache + R4 retained composite",
			"impl_correctness":       "FLOAT card composites over the dense cache with exact group blend; animations ride their own boundaries; dense region replays untouched",
			"impl_dirty":             "retained: steady frames re-record only the animated boundaries; dense region boundary_skip grows monotonically",
			"impl_edge":              "expectations computed from frozen post-run node state; rotation sampled at transform-invariant centers; resize re-resolves probe geometry",
			"impl_fail":              "golden diff != 0 from run 2, any failed pixel assertion, paint drift beyond cap, fps<55, p95>22 → FAIL exit 1",
			"impl_visible":           "three boards animate continuously; orange FLOAT card breathes above the dense grid; dense region looks perfectly still",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates (§3 C5 行 + §3.1.2 + §2.2 族 A 动画层档；硬，不许放) --------
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
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
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 动画层档组合关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.2f > %.0f\n", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 D: CPU 非双 0。
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑")
		os.Exit(1)
	}
	// paint_count 稳。
	if paintMin < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: paint baseline never sampled")
		os.Exit(1)
	}
	if paintMax-paintMin > paintDriftCap {
		fmt.Fprintf(os.Stderr, "FAIL: paint_count drift=%d want ≤%d\n", paintMax-paintMin, paintDriftCap)
		os.Exit(1)
	}
	// 像素断言全过。
	if scriptedTotal != 6 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 6/6 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	// Golden 零容差（首跑产基线；次跑起逐位一致）。
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_c5_anim_over_static: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min pc=%d..%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
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
		if c.pt != nil {
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, c.pt.want, c.pt.tol)
			fmt.Fprintf(os.Stderr, "ui_wr_c5_anim_over_static: pixel %-28s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_c5_anim_over_static: pixel %-28s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "c5_final.png")
	base := filepath.Join(snapDir, "c5_final_base.png")
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
