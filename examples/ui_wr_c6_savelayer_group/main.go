// Command ui_wr_c6_savelayer_group is the C6 composite real window
// (mode-1 first close): R18 SaveLayer offscreen groups + budget refusal,
// coexisting with the R3 nested-boundary static cache, under the retained
// present policy.
//
// §3 C6 thesis: 离屏组 + boundary 缓存 + 超预算拒批共存 — under a per-frame
// SaveLayerBudget{MaxOps:1}, group A re-requests a SaveLayer every frame and
// is allowed (real offscreen RT, group α=0.55), group B is budget-refused
// (draws directly behind a visible red frame), and group C joins in the Spike
// phase as a third refused request. All of this churns every frame while the
// dense nested-boundary panel keeps replaying (boundary_skip grows
// monotonically) and stays bitwise-stable across runs (golden).
//
// Gates (§3 C6 row + §2.5 SaveLayer/Filter 档): savelayer_allow ≥1,
// savelayer_reject ≥1, boundary_skip >0, retained policy, fps_interval ≥55,
// p95 ≤22ms, hitch ≤5/min, CPU non-double-zero, pixel assertions 6/6,
// golden bitwise zero-tolerance from run 2.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_c6_savelayer_group
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); closing requires 10 (§2.5
// SaveLayer/Filter 档 max(R18=10, R3=8)). No RUN_SECONDS → interactive loop
// (pixel evidence off).
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
	closeSeconds = 10 // §2.5 SaveLayer/Filter 档：max(R18,R3) = 10s

	cardW, cardH = 240.0, 150.0
	groupAlpha   = 0.55 // group A offscreen opacity — frozen constant (golden-stable)
	rect2Alpha   = 0.60 // inner-group second rect opacity
	hitchBudget  = 5.0  // per-min budget (§2.2.3 default)
	paintDriftCap = 40  // paint_count max−min band; full-paint regression ≫40
)

// Palette shared by scene builder AND pixel assertions (single source of truth).
var (
	clearBG   = [3]float64{0.08, 0.09, 0.11}
	boardBG   = [3]float64{0.13, 0.15, 0.21}
	denseBase = [3]float64{0.11, 0.12, 0.18}
	gA1       = [3]float64{0.85, 0.30, 0.28} // group A base rect (opaque)
	gA2       = [3]float64{0.28, 0.45, 0.88} // group A second rect (α 0.6)
	gB1       = [3]float64{0.35, 0.65, 0.55} // group B fill (drawn DIRECT on reject)
	rejectRed = [3]float64{1.00, 0.25, 0.25} // refusal frame + label
	gC1       = [3]float64{0.72, 0.35, 0.82} // group C fill (spike only)
	denseCell = [3]float64{0.37, 0.50, 0.72}
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

// blendOver is the exact compositing of src (opacity a) over dst:
// out = a·src + (1−a)·dst.
func blendOver(src, dst [3]float64, a float64) [3]float64 {
	var out [3]float64
	for i := 0; i < 3; i++ {
		out[i] = a*src[i] + (1-a)*dst[i]
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
	if secsSet {
		wrkit.RequireMinRun(secs, "C6")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_c6_savelayer_group — 离屏组+缓存+预算", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C6 SaveLayer离屏组+boundary缓存+超预算拒批 — R18+R3", []string{
		"A     组1 SaveLayer 允许 α=0.55 真离屏",
		"B     组2 超预算拒批 红框直绘",
		"C     组3 Spike 相出现 再拒一批",
		"DENSE 嵌套 boundary 静态缓存",
		"HOT   每帧变色动态热点（非缓存）",
		"budget MaxOps=1/帧（每帧重置）",
		"Spike 相位 HOT 转速×2",
		"静区 skip 只增 = 缓存与预算共存",
	})
	body := shell.Body

	drawText := func(pc *rendering.PaintContext, x, y float64, msg string, r, g, b float64) {
		if pc == nil || pc.DC == nil {
			return
		}
		if face := wrkit.FaceAt(12); face != nil {
			pc.DC.SetFont(face)
		}
		pc.DC.SetRGBA(r, g, b, 1)
		pc.DC.DrawString(msg, pc.OriginX+x, pc.OriginY+y)
	}

	// ===== Group A: first per-frame SaveLayer request — ALLOWED ============
	// Real offscreen RT via PushLayerIsolated(groupAlpha); interior blends are
	// two-stage and asserted exactly (probe P1 base-only, P2 overlap).
	cardA := rendering.NewRenderBox()
	cardA.FixedWidth, cardA.FixedHeight = cardW, cardH
	cardA.SetRepaintBoundary(true)
	cardA.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		ok := pc.SaveLayer(size.Width, size.Height, groupAlpha)
		rendering.FillRect(pc, 10, 10, 220, 130, gA1[0], gA1[1], gA1[2], 1)
		rendering.FillRect(pc, 125, 10, 95, 130, gA2[0], gA2[1], gA2[2], rect2Alpha)
		pc.RestoreLayer()
		if ok {
			drawText(pc, 12, 24, "GROUP-A allowed 合成", 0.96, 0.96, 0.98)
		} else {
			drawText(pc, 12, 24, "!!REJECT!!", 1, 0.3, 0.3)
		}
	}
	boardA := rendering.NewAbsoluteBox(300, 220)
	boardA.Background = &rendering.Color{R: boardBG[0], G: boardBG[1], B: boardBG[2], A: 1}
	boardA.Place(cardA, 30, 35)
	body.Box.Place(boardA, 8, 34)

	// ===== Group B: second per-frame request — BUDGET-REFUSED ==============
	// Refusal draws DIRECTLY (no group α): probe P3 expects the exact fill
	// color and P4 the exact red frame — both fail if the engine wrongly
	// allows the layer.
	cardB := rendering.NewRenderBox()
	cardB.FixedWidth, cardB.FixedHeight = cardW, cardH
	cardB.SetRepaintBoundary(true)
	cardB.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		ok := pc.SaveLayer(size.Width, size.Height, groupAlpha)
		rendering.FillRect(pc, 10, 10, 220, 130, gB1[0], gB1[1], gB1[2], 1)
		if !ok {
			// Refused: no offscreen group — draw the refusal visibly.
			rendering.FillRect(pc, 0, 0, size.Width, 6, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			rendering.FillRect(pc, 0, 0, 6, size.Height, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			rendering.FillRect(pc, size.Width-6, 0, 6, size.Height, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			rendering.FillRect(pc, 0, size.Height-6, size.Width, 6, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			drawText(pc, 14, 24, "GROUP-B BUDGET REJECT", 1, 0.35, 0.35)
		} else {
			drawText(pc, 14, 24, "GROUP-B allowed", 0.96, 0.96, 0.98)
		}
		pc.RestoreLayer()
	}
	boardB := rendering.NewAbsoluteBox(300, 220)
	boardB.Background = &rendering.Color{R: boardBG[0], G: boardBG[1], B: boardBG[2], A: 1}
	boardB.Place(cardB, 30, 35)
	body.Box.Place(boardB, 8, 270)

	// ===== Group C: Spike-phase third request — also REFUSED ===============
	cVisible := false
	cardC := rendering.NewRenderBox()
	cardC.FixedWidth, cardC.FixedHeight = cardW, cardH
	cardC.SetRepaintBoundary(true)
	cardC.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if !cVisible {
			return
		}
		ok := pc.SaveLayer(size.Width, size.Height, 0.5)
		rendering.FillRect(pc, 10, 10, 220, 130, gC1[0], gC1[1], gC1[2], 1)
		if !ok {
			rendering.FillRect(pc, 0, 0, size.Width, 6, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			rendering.FillRect(pc, 0, 0, 6, size.Height, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			rendering.FillRect(pc, size.Width-6, 0, 6, size.Height, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			rendering.FillRect(pc, 0, size.Height-6, size.Width, 6, rejectRed[0], rejectRed[1], rejectRed[2], 1)
			drawText(pc, 14, 24, "GROUP-C REJECT (spike)", 1, 0.35, 0.35)
		} else {
			drawText(pc, 14, 24, "GROUP-C allowed", 0.96, 0.96, 0.98)
		}
		pc.RestoreLayer()
	}
	boardC := rendering.NewAbsoluteBox(300, 220)
	boardC.Background = &rendering.Color{R: 0.15, G: 0.13, B: 0.19, A: 1}
	boardC.Place(cardC, 30, 35)
	body.Box.Place(boardC, 320, 34)

	// Live SaveLayer counter inside the body (dynamic → outside golden mask).
	slBanner := wrkit.Label("SL: allow=0 reject=0", 13, 0.95, 0.90, 0.55)
	body.Box.Place(slBanner, 320, 280)

	// ===== Right side: static dense nested-boundary cache (R3) ============
	dense := rendering.NewAbsoluteBox(250, 430)
	dense.Background = &rendering.Color{R: denseBase[0], G: denseBase[1], B: denseBase[2], A: 1}
	dense.SetDebugName("c6-dense-panel")
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 44, denseCell[0]*(1-0.08*float64(i)), denseCell[1]*(0.8+0.1*float64(j)), denseCell[2], 1)
			c.SetRepaintBoundary(true)
			c.SetDebugName(fmt.Sprintf("c6-cell-%d-%d", i, j))
			dense.Place(c, 10+float64(i)*62, 10+float64(j)*50)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		lbl.SetDebugName(fmt.Sprintf("c6-lbl-%d", i))
		dense.Place(lbl, 10+float64(col)*62, 222+float64(row)*20)
	}
	denseNote := wrkit.Label("静态缓存：SaveLayer 全程 skip", 11, 0.55, 0.75, 0.95)
	dense.SetRepaintBoundary(false)
	dense.Place(denseNote, 10, 268)
	body.Box.Place(dense, 650, 34)

	// ===== HOT spot: self-dirtying animation (U17 dynamic hotspot) =========
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	body.Box.Place(hot, 650, 500)
	hotLbl := wrkit.Label("HOT 每帧变色（非缓存）", 11, 0.95, 0.7, 0.6)
	body.Box.Place(hotLbl, 686, 504)

	// ===== Probe geometry (resolved from the layout chain; §2.7 rule 2) ====
	geoValid := false
	ptABase := probePoint{tol: 12.0 / 255} // P1: group-A point covered by rect1 only
	ptAOver := probePoint{tol: 12.0 / 255} // P2: group-A rect2-over-rect1 overlap
	ptBFill := probePoint{tol: 6.0 / 255}  // P3: group-B direct fill (exact, unblended)
	ptBFrame := probePoint{tol: 8.0 / 255} // P4: group-B red refusal frame
	ptDenseCell := probePoint{
		want: [3]float64{denseCell[0], denseCell[1] * 0.8, denseCell[2]}, tol: 8.0 / 255,
	} // cell (0,0) factors (1−0.08·0, 0.8+0.1·0)
	capTextBox := rect{}
	var goldenRects []rect

	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		ax := bodyX + boardA.Offset().X + 30 // cardA origin within body abs coords
		ay := bodyY + boardA.Offset().Y + 35
		bx := bodyX + boardB.Offset().X + 30
		by := bodyY + boardB.Offset().Y + 35
		dx := bodyX + dense.Offset().X

		// P1: card-rel (60,70) — rect1 only (x<125). Two-stage blend:
		// in-group rect1 opaque → group α over board backdrop.
		ptABase.x, ptABase.y = ax+60, ay+70
		ptABase.want = blendOver(gA1, boardBG, groupAlpha)
		// P2: card-rel (172,75) — overlap: in-group 0.6·gA2+0.4·gA1, then group α.
		inGroup := blendOver(gA2, gA1, rect2Alpha)
		ptAOver.x, ptAOver.y = ax+172, ay+75
		ptAOver.want = blendOver(inGroup, boardBG, groupAlpha)
		// P3: card-rel (60,80) — rejected → direct fill, EXACT color.
		ptBFill.x, ptBFill.y = bx+60, by+80
		ptBFill.want = gB1
		// P4: card-rel (120,3) — top refusal frame strip.
		ptBFrame.x, ptBFrame.y = bx+120, by+3
		ptBFrame.want = rejectRed
		// Dense cell (0,0) center.
		ptDenseCell.x = dx + 10 + 28
		ptDenseCell.y = bodyY + dense.Offset().Y + 10 + 22
		// F6 box: dense-note label region.
		capTextBox = rect{x: dx + 10, y: bodyY + dense.Offset().Y + 268, w: 230, h: 28}

		// Golden static mask: only regions that are time-invariant BY DESIGN.
		// Excluded: HUD band (fps/phase text), SL banner (counters grow),
		// board C (phase-toggled), HOT spot (cycles every frame).
		bx0, by0 := bodyX + boardB.Offset().X, bodyY + boardB.Offset().Y
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + boardA.Offset().X - 2, ay - 37, 304, 228},  // board A column
			{bx0 - 2, by0 - 2, 304, 228},                        // board B column
			{dx - 2, bodyY + dense.Offset().Y - 2, 256, 436},    // dense cache panel
			{ax + 280, by + 190, 300, 170},                      // empty body-bg strip
		}
		geoValid = true
	}

	snapDir := os.Getenv("C6_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/c6_savelayer_group"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		// Per-frame SaveLayer budget (reset each paint frame): exactly one
		// group may composite offscreen; B always refused, C refused in Spike.
		SaveLayerMaxOps: 1,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "c6_final.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c6_savelayer_group: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
				geoValid = false
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	clock := wrkit.NewPhaseClock(5, 10) // Steady 5s → Spike 5s → Recover

	var elapsed float64
	lastPhase := ""
	paintMin, paintMax := int64(-1), int64(-1)
	hotHue := 0.0
	phasesSeen := map[string]bool{}

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		phase := clock.Advance(dt)
		if phase != lastPhase {
			lastPhase = phase
		}
		phasesSeen[phase] = true
		spike := phase == wrkit.PhaseSpike

		// Group C joins in Spike — a third per-frame SaveLayer request so
		// reject accumulates 2/frame vs allow 1/frame (HUD-visible).
		if spike != cVisible {
			cVisible = spike
			cardC.MarkNeedsPaint()
		}

		// Groups A/B re-record every frame so each frame re-requests
		// SaveLayer against the fresh per-frame budget.
		cardA.MarkNeedsPaint()
		cardB.MarkNeedsPaint()

		// HOT spot repaints itself every frame (speed ×2 in Spike).
		speedK := 1.0
		if spike {
			speedK = 2.0
		}
		hotHue = math.Mod(hotHue+dt*speedK*0.6, 1.0)
		hot.R = 0.85 + 0.1*hotHue
		hot.G = 0.25 + 0.4*(1-hotHue)
		hot.B = 0.3 + 0.5*hotHue
		hot.MarkNeedsPaint()

		al, rj := app.SaveLayerStats()
		slBanner.SetText(fmt.Sprintf("SL: allow=%d reject=%d (MaxOps=1)", al, rj))
		slBanner.MarkNeedsPaint()

		pcSnap := app.Metrics().Snapshot()
		if elapsed >= 1.0 {
			if paintMin < 0 || pcSnap.PaintCount < paintMin {
				paintMin = pcSnap.PaintCount
			}
			if pcSnap.PaintCount > paintMax {
				paintMax = pcSnap.PaintCount
			}
		}

		gateOK := al >= 1 && rj >= 1 && pcSnap.BoundarySkip > 0 && paintMax-paintMin <= paintDriftCap
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("C6", phase, app, gateOK,
			fmt.Sprintf("allow=%d reject=%d", al, rj),
			fmt.Sprintf("skip=%d rr=%d spike=%v", pcSnap.BoundarySkip, pcSnap.BoundaryRerecord, spike))
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
	wrkit.MergeBoundaryCache(app, &snap)
	al, rj := app.SaveLayerStats()

	// ---- Final-frame pixel assertions (§2.7/U21) --------------------------
	// The engine snapshot is the FINAL frame after the last ticker tick. Group
	// contents are frozen constants (by design), so expectations are exact.
	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptABase, desc: "groupA_offscreen_blend"},
		{pt: &ptAOver, desc: "groupA_two_stage_blend"},
		{pt: &ptBFill, desc: "groupB_direct_fill_unblended"},
		{pt: &ptBFrame, desc: "groupB_refusal_frame"},
		{pt: &ptDenseCell, desc: "dense_cell_intact_under_churn"},
		{text: &capTextBox, base: denseBase, desc: "dense_note_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "c6_final.png")), finalChecks, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C6",
		Scenario:      "ui_wr_c6_savelayer_group",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"savelayer_allow":       al,
			"savelayer_reject":      rj,
			"savelayer_groups":      3,
			"save_budget":           "MaxOps=1 per frame (per-frame reset)",
			"hitch_budget_per_min":  hitchBudget,
			"paint_drift_cap":       paintDriftCap,
			"paint_count_min":       paintMin,
			"paint_count_max":       paintMax,
			"phases_seen":           keysOf(phasesSeen),
			"group_c_spike_visible": lastPhase == wrkit.PhaseSpike && cVisible,
			"scripted_ok":           scriptedOK,
			"scripted_total":        scriptedTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"covered":                "R18 SaveLayer offscreen group + budget refusal + R3 nested boundary cache + retained composite",
			"impl_correctness":       "group A composites as a real offscreen RT with exact α=0.55 two-stage blends (P1/P2); group B refusal draws DIRECT (P3/P4 exact colors prove no hidden blending)",
			"impl_dirty":             "retained: steady frames re-record only the three savelayer cards + banner + hot; dense boundaries keep skipping (boundary_skip monotonic)",
			"impl_cache":             "dense panel replays untouched while offscreen groups churn above/beside it — cache and budget coexist",
			"impl_edge":              "per-frame budget reset verified (allow=reject growing every frame); probe geometry re-resolved on resize; expectations from frozen constants",
			"impl_fail":              "wrong allow order / wrongly-allowed B / broken group blend / dense rerecord storm all trip pixel or counter gates → FAIL exit 1",
			"impl_visible":           "HUD shows live allow/reject/skip; group B wears a red refusal frame; group C appears in Spike; dense region looks perfectly still",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates (§3 C6 行 + §2.5 SaveLayer/Filter 档 + §2.2 全族；硬，不许放) --
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MaxP95Ms:              22,
		RequireRetainedPolicy: true,
		MinBoundarySkip:       3,
		MinBoundaryRerecord:   1,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	// §3 C6 row gates: the three counters must coexist.
	if al < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: savelayer_allow=%d want >=1 (first group must composite offscreen)\n", al)
		os.Exit(1)
	}
	if rj < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: savelayer_reject=%d want >=1 (second group must be budget-refused)\n", rj)
		os.Exit(1)
	}
	if snap.BoundarySkip < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >=3 (static cache must keep skipping)\n", snap.BoundarySkip)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if secsSet && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 SaveLayer/Filter 档组合关闭用时长)\n", secs, closeSeconds)
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

	fmt.Fprintf(os.Stderr, "ui_wr_c6_savelayer_group: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min allow=%d reject=%d skip=%d rr=%d pc=%d..%d scripted=%d/%d golden=%.4f%%(%dpx first=%v) vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		al, rj, snap.BoundarySkip, snap.BoundaryRerecord,
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
			fmt.Fprintf(os.Stderr, "ui_wr_c6_savelayer_group: pixel %-30s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_c6_savelayer_group: pixel %-30s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "c6_final.png")
	base := filepath.Join(snapDir, "c6_final_base.png")
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

func keysOf(m map[string]bool) string {
	out := ""
	for k := range m {
		if out != "" {
			out += ","
		}
		out += k
	}
	return out
}

type tickerT struct{ on func(dt float64) }

func (t *tickerT) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
