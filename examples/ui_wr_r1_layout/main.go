// Command ui_wr_r1_layout is the W2 R1 real-window: 局部 NeedsLayout isolation.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/ui_wr_r1_layout
//
// Window: 1200x800. R1 table floor is 5s (U16); close evidence uses 30s
// because the two U21 snapshot readbacks would otherwise exceed the hitch
// budget. GPU window required.
//
// R1 thesis: changing one child's layout size must dirty only its relayout
// boundary subtree — never the window root, the shell panels, or sibling
// boards. The target frame is a real engine relayout boundary; four timed
// height changes move its Align child (the amber marker), while the sibling
// chip, dense board, stripes, and persistent HOT animation stay put.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sync/atomic"
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
	closeSeconds = 30 // R1 §2.5 下限 5s，加长到 30s 以摊薄 U21 双快照读回（2 次/30s = 4/min ≤5）

	hitchBudget = 5.0 // per-min budget (§2.2.2 正确性窗默认建议)
	cpuBudget   = 85.0

	baseHeight  = 120.0
	spikeHeight = 200.0
	markerW     = 120.0
	markerH     = 48.0
)

var (
	clearBG = [3]float64{0.06, 0.07, 0.09}

	frameBG = [3]float64{0.050, 0.060, 0.080}
	bodyBG  = [3]float64{0.100, 0.110, 0.130}

	targetFill = [3]float64{0.16, 0.42, 0.82}
	markerFill = [3]float64{0.98, 0.72, 0.22}
	chipFill   = [3]float64{0.20, 0.78, 0.68}

	neighborBG = [3]float64{0.110, 0.130, 0.170}
	stripe0    = [3]float64{0.85, 0.35, 0.30}
	hotFill    = [3]float64{0.35, 0.90, 0.55}
)

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

type layoutStep struct {
	at     float64
	height float64
}

var pixelResult = map[string]bool{}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "R1")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_wr_r1_layout — 局部 NeedsLayout", Decorations: true, Resizable: false})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R1 局部 NeedsLayout — 只脏子树，不惊动整窗", []string{
		"TARGET 真引擎 relayout boundary（改高度只脏本子树）",
		"Spike 4 次高度变化 120↔200；Recover 回到 120",
		"SIBLING 同父兄弟 chip 全程不挪；像素/Golden 双证",
		"NEIGHBOR 4×4 色格 + 8 标签全程静止",
		"HOT 呼吸盘持续动画（paint-only，不触发布局）",
		"门禁: layout_delta == height_changes + 零外来脏",
	})
	body := shell.Body

	// ===== TARGET: relayout boundary + changing fixed height ==============
	// The engine's MarkNeedsLayout bubbles to the parent and stops when the
	// PARENT is a relayout boundary (see pipeline_test TestRelayoutBoundary_
	// StopsBubble: root -> mid(boundary) -> leaf). Therefore the boundary is
	// the frame, not the changing child.
	targetFrame := rendering.NewAbsoluteBox(360, 320)
	targetFrame.Background = &rendering.Color{R: frameBG[0], G: frameBG[1], B: frameBG[2], A: 1}
	targetFrame.SetRelayoutBoundary(true)
	targetFrame.SetRepaintBoundary(true)
	targetFrame.Place(wrkit.Label("TARGET FRAME (relayout boundary)", 13, 0.90, 0.93, 0.98), 12, 10)

	target := rendering.NewAbsoluteBox(320, baseHeight)
	target.Background = &rendering.Color{R: targetFill[0], G: targetFill[1], B: targetFill[2], A: 1}
	target.SetRepaintBoundary(true)
	targetFrame.Place(target, 20, 40)

	marker := rendering.NewRenderColorBox(markerW, markerH, markerFill[0], markerFill[1], markerFill[2], 1)
	marker.SetRepaintBoundary(true)
	align := rendering.NewRenderAlignBox(marker, 0.5, 0.5)
	target.Place(align, 0, 0)

	// Sibling inside the same boundary frame: it must not move when target
	// changes height. Pixel + Golden + offset probes all watch this chip.
	targetChip := rendering.NewRenderColorBox(120, 24, chipFill[0], chipFill[1], chipFill[2], 1)
	targetChip.SetRepaintBoundary(true)
	targetFrame.Place(targetChip, 20, 260)
	targetFrame.Place(wrkit.Label("SIBLING stays put", 11, 0.70, 0.80, 0.78), 160, 262)
	// Phase lamp: paint-only visual cue in the frame title row. It never
	// touches layout; its color is not part of the Golden static mask.
	phaseLamp := rendering.NewRenderColorBox(16, 16, 0.30, 0.62, 0.95, 1)
	phaseLamp.SetRepaintBoundary(true)
	targetFrame.Place(phaseLamp, 330, 10)
	body.Place(targetFrame, 24, 40)

	// ===== NEIGHBOR: dense static board (never touched) ===================
	neighbor := rendering.NewAbsoluteBox(460, 220)
	neighbor.Background = &rendering.Color{R: neighborBG[0], G: neighborBG[1], B: neighborBG[2], A: 1}
	neighbor.Place(wrkit.Label("NEIGHBOR 静区 4x4 + 8 标签", 13, 0.90, 0.93, 0.98), 10, 8)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 28, 0.25+float64(i)*0.12, 0.40+float64(j)*0.10, 0.62, 1)
			neighbor.Place(c, 10+float64(i)*64, 32+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		neighbor.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*64, 180+float64(row)*20)
	}
	body.Place(neighbor, 420, 40)

	// ===== STRIPES: second static sibling board ==========================
	stripes := rendering.NewAbsoluteBox(460, 180)
	stripes.Background = &rendering.Color{R: 0.11, G: 0.12, B: 0.16, A: 1}
	stripes.Place(wrkit.Label("STRIPES 静区 三段色带", 13, 0.90, 0.93, 0.98), 10, 8)
	stripe := rendering.NewRenderColorBox(140, 80, stripe0[0], stripe0[1], stripe0[2], 1)
	stripes.Place(stripe, 10, 50)
	stripes.Place(rendering.NewRenderColorBox(140, 80, 0.25, 0.62, 0.85, 1), 160, 50)
	stripes.Place(rendering.NewRenderColorBox(140, 80, 0.82, 0.78, 0.25, 1), 310, 50)
	body.Place(stripes, 420, 280)

	// ===== HOT: persistent animation hotspot (paint-only) ================
	pulseTransform := rendering.NewRenderTransform()
	pulseTransform.FixedWidth, pulseTransform.FixedHeight = 120, 120
	pulseTransform.SetRepaintBoundary(true)
	pulseArm := rendering.NewAbsoluteBox(120, 120)
	pulseArm.Place(rendering.NewRenderColorBox(70, 70, hotFill[0], hotFill[1], hotFill[2], 1), 25, 25)
	pulseTransform.AddChild(pulseArm)
	hotBoard := rendering.NewAbsoluteBox(200, 200)
	hotBoard.Background = &rendering.Color{R: 0.13, G: 0.13, B: 0.19, A: 1}
	hotBoard.Place(wrkit.Label("HOT 呼吸盘 (paint-only)", 12, 0.90, 0.80, 0.60), 10, 8)
	hotBoard.Place(pulseTransform, 40, 50)
	body.Place(hotBoard, 24, 380)

	// ===== Probe geometry =================================================
	geoValid := false
	ptNeighbor := probePoint{tol: 8.0 / 255}
	ptTarget := probePoint{tol: 8.0 / 255}
	ptMarker := probePoint{tol: 8.0 / 255}
	ptSlot := probePoint{tol: 8.0 / 255}
	ptChip := probePoint{tol: 8.0 / 255}
	ptStripe := probePoint{tol: 8.0 / 255}
	ptHot := probePoint{tol: 8.0 / 255}
	ptLamp := probePoint{tol: 8.0 / 255}
	ptLampSteady := probePoint{tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect

	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := body.Box.Offset().X, body.Box.Offset().Y
		// NEIGHBOR cell (0,0) center: body→neighbor(420,40)→cell(10,32) 56x28.
		ptNeighbor.x = bodyX + 420 + 10 + 28
		ptNeighbor.y = bodyY + 40 + 32 + 14
		ptNeighbor.want = [3]float64{0.25, 0.40, 0.62}

		// TARGET blue slab, above the marker in final (base) state.
		// Chain: body→frame(24,40)→target(20,40), h=120.
		ptTarget.x = bodyX + 24 + 20 + 320/2
		ptTarget.y = bodyY + 40 + 40 + 20
		ptTarget.want = targetFill

		// Marker center final: target-local (160, 60).
		ptMarker.x = bodyX + 24 + 20 + 320/2
		ptMarker.y = bodyY + 40 + 40 + baseHeight/2
		ptMarker.want = markerFill

		// Slot below the final target: frame background (target is 120 high).
		// During Spike (200 high) this point is target blue; final restore
		// must show frame background again.
		ptSlot.x = ptMarker.x
		ptSlot.y = bodyY + 40 + 40 + 160
		ptSlot.want = frameBG

		// Sibling chip center: body→frame(24,40)→chip(20,260) 120x24.
		ptChip.x = bodyX + 24 + 20 + 60
		ptChip.y = bodyY + 40 + 260 + 12
		ptChip.want = chipFill

		// First stripe center: body→stripes(420,280)→stripe(10,50) 140x80.
		ptStripe.x = bodyX + 420 + 10 + 70
		ptStripe.y = bodyY + 280 + 50 + 40
		ptStripe.want = stripe0

		// HOT disc center: body→hot(24,380)→transform(40,50)→arm(0,0)→disc(25,25) 70x70.
		ptHot.x = bodyX + 24 + 40 + 25 + 35
		ptHot.y = bodyY + 380 + 50 + 25 + 35
		ptHot.want = hotFill

		// Phase lamp center: body→frame(24,40)→lamp(330,10) 16x16.
		// Steady is blue, Recover is green; final frame is Recover.
		ptLamp.x = bodyX + 24 + 330 + 8
		ptLamp.y = bodyY + 40 + 10 + 8
		ptLamp.want = [3]float64{0.30, 0.85, 0.55}
		ptLampSteady = ptLamp
		ptLampSteady.want = [3]float64{0.30, 0.62, 0.95}

		capTextBox = rect{x: bodyX + 420 + 10, y: bodyY + 40 + 178, w: 400, h: 30}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + 420, bodyY + 40, 460, 220},          // neighbor dense
			{bodyX + 420, bodyY + 280, 460, 180},         // stripes
			{bodyX + 24 + 20, bodyY + 40 + 258, 320, 28}, // sibling chip + caption
		}
		geoValid = true
	}

	snapDir := os.Getenv("R1_SNAP_DIR")
	if snapDir == "" {
		snapDir = "/tmp/r1_layout"
	}
	os.MkdirAll(snapDir, 0o755)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: filepath.Join(snapDir, "r1_final.png"),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r1_layout: close (%s)\n", win.Backend())
			}
			// R1 is a fixed-size correctness window. Resize layout belongs to
			// R11; accepting a resize here would add an unrelated layout pass
			// and blur the "exactly four local relayouts" gate.
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	clock := wrkit.NewPhaseClock(10, 20)
	steps := []layoutStep{
		{at: 12, height: spikeHeight},
		{at: 14, height: baseHeight},
		{at: 16, height: spikeHeight},
		{at: 18, height: baseHeight},
	}

	var elapsed float64
	lastPhase := ""
	stepIdx := 0
	heightChanges := 0
	isolationViolations := 0
	layoutBase := int64(-1)
	paintMin, paintMax := int64(-1), int64(-1)
	pulseScale := 1.0
	steadyShotDone, recoverShotDone := false, false
	snapSteadyAt, snapRecoverAt := 5.0, 25.0
	// Freeze all UI-side tree mutations while the raster thread repaints a
	// snapshot: PaintPresentTree reads the same tree the ticker normally
	// mutates (pulse transform, HUD text), so the freeze makes the U21
	// repaint a stable-tree read instead of a cross-thread race.
	var snapBusy atomic.Bool
	var snapDeadline time.Time

	var frameOffset0, neighborOffset0, chipOffset0 rendering.Point

	applyPhase := func(phase string) {
		switch phase {
		case wrkit.PhaseSpike:
			phaseLamp.R, phaseLamp.G, phaseLamp.B = 0.95, 0.55, 0.18
		case wrkit.PhaseRecover:
			phaseLamp.R, phaseLamp.G, phaseLamp.B = 0.30, 0.85, 0.55
		default:
			phaseLamp.R, phaseLamp.G, phaseLamp.B = 0.30, 0.62, 0.95
		}
		phaseLamp.MarkNeedsPaint()
	}

	applyHeight := func(h float64) {
		target.FixedHeight = h
		target.MarkNeedsLayout()
		heightChanges++
		if !target.NeedsLayout() {
			isolationViolations++
		}
		if !targetFrame.NeedsLayout() {
			// The boundary parent must be marked; if not, the change did not
			// travel through the intended relayout-boundary path.
			isolationViolations++
		}
		for _, n := range []rendering.RenderObject{
			shell.Root, shell.Top.Box, shell.Legend.Box, shell.Body.Box,
			targetChip, neighbor, stripes, hotBoard, pulseTransform,
		} {
			if n.NeedsLayout() {
				isolationViolations++
			}
		}
	}

	// Mid-run snapshots must repaint the tree into the context before
	// SavePNG. Reading the post-present context Image() returns a blank/stale
	// pixmap (zero-readback present), which would put white PNGs on disk while
	// the final SnapshotPath image looks correct. PaintPresentTree is the same
	// public repaint entry C4 uses inside SnapshotAsync.
	saveSnap := func(path, tag string) {
		dc := app.Target().Context()
		if dc == nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: no gpu context\n", tag)
			return
		}
		dc.BeginFrame()
		embedder.PaintPresentTree(dc, app.Pipeline(), shell.Root, nil,
			clearBG[0], clearBG[1], clearBG[2], 1, true)
		if err := dc.SavePNG(path); err != nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: %v\n", tag, err)
			return
		}
		fmt.Fprintf(os.Stderr, "snapshot %s: %s\n", tag, path)
	}

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		if snapBusy.Load() {
			if time.Now().After(snapDeadline) {
				snapBusy.Store(false)
			} else {
				app.ScheduleFrame()
				proc.Sample()
				return
			}
		}
		elapsed += dt
		phase := clock.Advance(dt)
		if phase != lastPhase {
			applyPhase(phase)
			lastPhase = phase
		}

		// Persistent motion hotspot: paint-only, never layout.
		pulseScale = 1.0 + 0.10*math.Sin(2*math.Pi*elapsed/2.5)
		pulseTransform.SetScale(pulseScale, pulseScale)

		// Baseline after warm-up, before the first timed height change.
		if layoutBase < 0 && elapsed >= 2.0 {
			pc := app.Metrics().Snapshot()
			layoutBase = pc.LayoutCount
			frameOffset0 = targetFrame.Offset()
			neighborOffset0 = neighbor.Offset()
			chipOffset0 = targetChip.Offset()
		}
		if stepIdx < len(steps) && elapsed >= steps[stepIdx].at {
			applyHeight(steps[stepIdx].height)
			stepIdx++
		}

		pc := app.Metrics().Snapshot()
		if elapsed >= 2.0 {
			if paintMin < 0 || pc.PaintCount < paintMin {
				paintMin = pc.PaintCount
			}
			if pc.PaintCount > paintMax {
				paintMax = pc.PaintCount
			}
		}

		if !steadyShotDone && elapsed >= snapSteadyAt && phase == wrkit.PhaseSteady {
			steadyShotDone = true
			snapBusy.Store(true)
			snapDeadline = time.Now().Add(2 * time.Second)
			go app.SnapshotAsync(func() {
				defer snapBusy.Store(false)
				saveSnap(filepath.Join(snapDir, "r1_steady.png"), "steady")
			})
		}
		if steadyShotDone && !recoverShotDone && elapsed >= snapRecoverAt && phase == wrkit.PhaseRecover {
			recoverShotDone = true
			snapBusy.Store(true)
			snapDeadline = time.Now().Add(2 * time.Second)
			go app.SnapshotAsync(func() {
				defer snapBusy.Store(false)
				saveSnap(filepath.Join(snapDir, "r1_recover.png"), "recover")
			})
		}

		layoutDelta := int64(0)
		if layoutBase >= 0 {
			layoutDelta = pc.LayoutCount - layoutBase
		}
		gateOK := layoutBase >= 0 && layoutDelta <= int64(heightChanges) && isolationViolations == 0
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("R1", phase, app, gateOK,
			fmt.Sprintf("h=%d lay=%d/%d dirty=%d", int(target.Size().Height), layoutDelta, heightChanges, isolationViolations),
			fmt.Sprintf("pc=%d", pc.PaintCount))
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

	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptNeighbor, desc: "neighbor_cell00_exact"},
		{pt: &ptTarget, desc: "target_body_exact"},
		{pt: &ptMarker, desc: "target_marker_center_final"},
		{pt: &ptSlot, desc: "target_slot_restored"},
		{pt: &ptChip, desc: "sibling_chip_stays"},
		{pt: &ptStripe, desc: "stripe0_exact"},
		{pt: &ptHot, desc: "hot_center_invariant"},
		{pt: &ptLamp, desc: "recover_phase_lamp_green"},
		{text: &capTextBox, base: neighborBG, desc: "neighbor_label_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "r1_final.png")), finalChecks, pixelResult)
	// The steady snapshot is a second visible state; its lamp must be blue
	// while the final/recover lamp above is green. This also proves the
	// snapshot itself is a real rendered frame, not a blank post-present read.
	runPixelChecks(loadImage(filepath.Join(snapDir, "r1_steady.png")),
		[]pixelCheck{{pt: &ptLampSteady, desc: "steady_phase_lamp_blue"}}, pixelResult)

	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}

	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	// Cross-state static check: the steady and recover snapshots must agree
	// bit-for-bit on the same static mask. The target/HUD/phase lamp are
	// excluded; neighbor boards and the sibling chip must not move between
	// the pre-layout and post-layout states.
	snapshotStaticDiff, snapshotStaticPx, snapshotStaticErr := comparePNG(
		filepath.Join(snapDir, "r1_steady.png"),
		filepath.Join(snapDir, "r1_recover.png"), goldenRects)
	if snapshotStaticErr != nil {
		fmt.Fprintf(os.Stderr, "snapshot static compare: %v\n", snapshotStaticErr)
		snapshotStaticDiff = 100
	}

	layoutDelta := int64(-1)
	if layoutBase >= 0 {
		layoutDelta = snap.LayoutCount - layoutBase
	}
	frameOffset := targetFrame.Offset()
	neighborOffset := neighbor.Offset()
	chipOffset := targetChip.Offset()
	neighborShift := math.Abs(neighborOffset.X-neighborOffset0.X) + math.Abs(neighborOffset.Y-neighborOffset0.Y)
	frameShift := math.Abs(frameOffset.X-frameOffset0.X) + math.Abs(frameOffset.Y-frameOffset0.Y)
	chipShift := math.Abs(chipOffset.X-chipOffset0.X) + math.Abs(chipOffset.Y-chipOffset0.Y)
	targetHeight := target.Size().Height
	markerOffset := marker.Offset()
	markerWantY := (baseHeight - markerH) / 2

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R1",
		Scenario:      "ui_wr_r1_layout",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":                   "1200x800",
			"run_seconds":                 secs,
			"hitch_budget_per_min":        hitchBudget,
			"layout_base":                 layoutBase,
			"layout_count_final":          snap.LayoutCount,
			"layout_delta":                layoutDelta,
			"layout_expected":             heightChanges,
			"height_changes":              heightChanges,
			"layout_isolation_violations": isolationViolations,
			"layout_boundary_present":     targetFrame.IsRelayoutBoundary(),
			"target_needs_layout_final":   target.NeedsLayout(),
			"target_height_final":         targetHeight,
			"target_height_expected":      baseHeight,
			"marker_offset_final":         markerOffset,
			"marker_offset_expected_y":    markerWantY,
			"neighbor_shift_px":           neighborShift,
			"target_frame_shift_px":       frameShift,
			"sibling_chip_shift_px":       chipShift,
			"paint_count_min":             paintMin,
			"paint_count_max":             paintMax,
			"paint_rate_per_sec":          float64(paintMax) / elapsedSec,
			"scripted_ok":                 scriptedOK,
			"scripted_total":              scriptedTotal,
			"pixel_golden_diff_pct":       goldenDiffPct,
			"pixel_golden_total_px":       goldenTotalPx,
			"pixel_golden_first_run":      goldenFirstRun,
			"snapshot_static_diff_pct":    snapshotStaticDiff,
			"snapshot_static_total_px":    snapshotStaticPx,
			"rss_slope_semantics":         "30s correctness window is startup-ramp dominated; leakage verdict belongs to R15 soak",
			"hitch_note":                  "2 hitches = the two U21 snapshot readbacks (steady/recover); 2/30s = 4/min; no other hitches",
			"cpu_fallback_note":           "cpu_fallback_ops=1/frame is the existing text:color-layout path for static labels; last_cpu_fallback exposes it (no gate-off, not an R1 layout callback)",
			"frame_raster_note":           "final frame_raster_ms includes the post-run snapshot repaint/readback; fps gates use interval metrics",
			"snapshots":                   "steady+recover raster-thread double-shot (U21 §4)",
			"impl_correctness":            "real engine relayout boundary (targetFrame) + AbsoluteBox/Align layout; changing target.FixedHeight recomputes marker center from constraints",
			"impl_dirty":                  "four timed target height changes mark only target and its boundary frame; root/body/top/legend/sibling boards stay NeedsLayout=false",
			"impl_cache":                  "neighbor dense/stripes + top/legend + sibling chip are static and covered by the Golden mask",
			"impl_edge":                   "final Recover restores base height 120; marker recentered; slot below target shows frame background again; no layout storm",
			"impl_fail":                   "layout delta != height changes, foreign layout dirty, boundary missing, sibling shift, target/marker/slot pixel fail, golden diff != 0 from run 2, fps<55 → exit 1",
			"impl_visible":                "Spike grows the blue target slab downward and slides the amber marker with it; Recover restores base; neighbor boards and sibling chip never move; HUD shows h/lay/dirty",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates ------------------------------------------------------------
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
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
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		fmt.Fprintln(os.Stderr, "FAIL: cpu 双 0 装绿嫌疑")
		os.Exit(1)
	}
	if snap.CPUPctAvg > cpuBudget {
		fmt.Fprintf(os.Stderr, "FAIL: cpu_pct_avg=%.2f > %.0f\n", snap.CPUPctAvg, cpuBudget)
		os.Exit(1)
	}

	// R1 专用: exactly one layout pass per timed height change; the boundary
	// must be present; no foreign node may be layout-dirty; siblings must not
	// shift. This is the local-NeedsLayout thesis, not a full-window rebuild.
	if !targetFrame.IsRelayoutBoundary() {
		fmt.Fprintln(os.Stderr, "FAIL: target frame is not a relayout boundary")
		os.Exit(1)
	}
	if heightChanges != len(steps) {
		fmt.Fprintf(os.Stderr, "FAIL: height_changes=%d want %d\n", heightChanges, len(steps))
		os.Exit(1)
	}
	if layoutBase < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: layout baseline never sampled")
		os.Exit(1)
	}
	if layoutDelta != int64(heightChanges) {
		fmt.Fprintf(os.Stderr, "FAIL: layout_delta=%d want %d (one pass per local height change)\n", layoutDelta, heightChanges)
		os.Exit(1)
	}
	if isolationViolations != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: layout isolation violations=%d want 0 (foreign node dirtied)\n", isolationViolations)
		os.Exit(1)
	}
	if neighborShift > 0.01 || frameShift > 0.01 || chipShift > 0.01 {
		fmt.Fprintf(os.Stderr, "FAIL: sibling shift neighbor=%.2f frame=%.2f chip=%.2f want 0\n", neighborShift, frameShift, chipShift)
		os.Exit(1)
	}
	if math.Abs(targetHeight-baseHeight) > 0.5 {
		fmt.Fprintf(os.Stderr, "FAIL: target_height_final=%.1f want %.1f\n", targetHeight, baseHeight)
		os.Exit(1)
	}
	if math.Abs(markerOffset.Y-markerWantY) > 0.5 {
		fmt.Fprintf(os.Stderr, "FAIL: marker_offset_final.y=%.1f want %.1f\n", markerOffset.Y, markerWantY)
		os.Exit(1)
	}
	if target.NeedsLayout() {
		fmt.Fprintln(os.Stderr, "FAIL: target still layout-dirty after final frame")
		os.Exit(1)
	}
	if scriptedTotal != 10 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 10/10 (像素断言)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	if snapshotStaticErr != nil || snapshotStaticDiff != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: snapshot static diff=%.4f over %d px err=%v want 0 (static boards moved between states)\n",
			snapshotStaticDiff, snapshotStaticPx, snapshotStaticErr)
		os.Exit(1)
	}
	snapOK := func(name string) bool {
		fi, err := os.Stat(filepath.Join(snapDir, name))
		return err == nil && fi.Size() > 0
	}
	if !steadyShotDone || !recoverShotDone {
		fmt.Fprintf(os.Stderr, "FAIL: snapshots incomplete (steady=%v recover=%v) — U21 §4 requires 2 shots\n",
			steadyShotDone, recoverShotDone)
		os.Exit(1)
	}
	if !snapOK("r1_steady.png") || !snapOK("r1_recover.png") {
		fmt.Fprintf(os.Stderr, "FAIL: snapshot files missing (steady=%v recover=%v)\n",
			snapOK("r1_steady.png"), snapOK("r1_recover.png"))
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "ui_wr_r1_layout: OK presents=%d fps=%.1f p95=%.1f hitch=%.2f/min changes=%d layout=%d..%d delta=%d dirty=%d shifts=(0,0,0) scripted=%d/%d golden=%.4f%%(%dpx first=%v) snapstatic=%.4f%% vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), fpsOf(snap), snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		heightChanges, layoutBase, snap.LayoutCount, layoutDelta, isolationViolations,
		scriptedOK, scriptedTotal, goldenDiffPct, goldenTotalPx, goldenFirstRun,
		snapshotStaticDiff, snap.VSyncSource, elapsedSec)
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
			fmt.Fprintf(os.Stderr, "ui_wr_r1_layout: pixel %-32s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		case c.text != nil:
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "ui_wr_r1_layout: pixel %-32s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "r1_final.png")
	base := filepath.Join(snapDir, "r1_final_base.png")
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
