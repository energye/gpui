// Command ui_wr_r5_picture is the W1-W2 R5 real-window: Picture record/replay
// (rewritten for the R5 quality-rework wave — Skia/Flutter display-list
// semantics: immutable ops, normalized paint, strict alpha, full Bounds).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R5")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	face, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Shared image source for the OpDrawImage contrast (green 16x16).
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

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r5_picture — 录/回放", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R5 Picture录/回放 — 直绘≡回放", []string{
		"LEFT=direct paint (每帧直绘)",
		"RIGHT=Picture replay (缓存回放)",
		"path fill + stroke + rect + text",
		"+ image 1:1 / scaled (8 ops)",
		"picture_op_count + Bounds in HUD",
	})

	// Build the recorded Picture ONCE (retained display list, 8 ops).
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		// Path fill (triangle).
		p := render.NewPath()
		p.MoveTo(20, 20)
		p.LineTo(90, 20)
		p.LineTo(55, 80)
		p.Close()
		r.FillPath(p, 0.9, 0.3, 0.2, 1)
		// Path stroke (quad).
		p2 := render.NewPath()
		p2.MoveTo(20, 120)
		p2.LineTo(90, 120)
		p2.LineTo(90, 160)
		p2.LineTo(20, 160)
		p2.Close()
		r.StrokePath(p2, 3, 0.2, 0.6, 0.9, 1)
		// Filled rect.
		r.FillRect(20, 190, 80, 40, 0.3, 0.8, 0.4, 1)
		// Stroke rect.
		r.StrokeRect(120, 190, 60, 40, 2, 0.9, 0.8, 0.3, 1)
		// Image 1:1 and scaled (both folded into Bounds by the recorder).
		r.DrawImage(src, 150, 20, 0, 0)
		r.DrawImage(src, 150, 100, 32, 32)
		// Text.
		if face != nil {
			r.DrawString("PIC-REPLAY", 20, 260, face, 0.9, 0.9, 0.95, 1)
			r.DrawString("v2", 120, 260, face, 0.9, 0.8, 0.4, 1)
		}
	})
	opCount := pic.OpCount()
	picBounds := pic.Bounds
	boundsTxt := "no-bounds"
	if !picBounds.Empty() {
		boundsTxt = fmt.Sprintf("(%d,%d %dx%d)", picBounds.Min.X, picBounds.Min.Y, picBounds.Dx(), picBounds.Dy())
	}

	// Left: direct-paint box replays the same commands fresh each frame.
	direct := rendering.NewRenderBox()
	direct.FixedWidth, direct.FixedHeight = 200, 300
	direct.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ox, oy := pc.OriginX, pc.OriginY
		// Direct path draw (same geometry as pic).
		p := render.NewPath()
		p.MoveTo(ox+20, oy+20)
		p.LineTo(ox+90, oy+20)
		p.LineTo(ox+55, oy+80)
		p.Close()
		pc.DC.SetRGBA(0.9, 0.3, 0.2, 1)
		_ = pc.DC.FillPath(p)
		p2 := render.NewPath()
		p2.MoveTo(ox+20, oy+120)
		p2.LineTo(ox+90, oy+120)
		p2.LineTo(ox+90, oy+160)
		p2.LineTo(ox+20, oy+160)
		p2.Close()
		pc.DC.SetRGBA(0.2, 0.6, 0.9, 1)
		pc.DC.SetLineWidth(3)
		_ = pc.DC.StrokePath(p2)
		pc.DC.SetRGBA(0.3, 0.8, 0.4, 1)
		pc.DC.DrawRectangle(ox+20, oy+190, 80, 40)
		_ = pc.DC.Fill()
		pc.DC.SetRGBA(0.9, 0.8, 0.3, 1)
		pc.DC.DrawRectangle(ox+120, oy+190, 60, 40)
		_ = pc.DC.Stroke()
		// Direct image draws (1:1 + scaled).
		pc.DC.DrawImage(src, ox+150, oy+20)
		pc.DC.DrawImageEx(src, render.DrawImageOptions{X: ox + 150, Y: oy + 100, DstWidth: 32, DstHeight: 32})
		if face != nil {
			pc.DC.SetFont(face)
			pc.DC.SetRGBA(0.9, 0.9, 0.95, 1)
			pc.DC.DrawString("PIC-REPLAY", ox+20, oy+260)
			pc.DC.SetRGBA(0.9, 0.8, 0.4, 1)
			pc.DC.DrawString("v2", ox+120, oy+260)
		}
	}
	// §2.6.1: capability elements are positioned layout-driven (Align).
	shell.Body.Align(direct, 0.10, 0.10)

	// Right: picture-replay box replays the retained Picture each frame.
	replay := rendering.NewRenderBox()
	replay.FixedWidth, replay.FixedHeight = 200, 300
	replay.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		pc.DC.Translate(pc.OriginX, pc.OriginY)
		pic.Replay(pc.DC)
	}
	shell.Body.Align(replay, 0.55, 0.10)

	// Labels under each region (Align, follow the boxes).
	shell.Body.Align(wrkit.Label("DIRECT (直绘)", 12, 0.8, 0.85, 0.9), 0.10, 0.75)
	shell.Body.Align(wrkit.Label("REPLAY (回放)", 12, 0.8, 0.85, 0.9), 0.55, 0.75)

	// Static dense grid to keep the frame busy (not part of picture).
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(40, 40, 0.25, 0.5, 0.7, 1)
			shell.Body.Place(c, 680+float64(i)*55, 20+float64(j)*55)
		}
	}

	snapDir := filepath.Join("examples", "ui_wr_r5_picture", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_r5_picture.png")

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
				fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: close (%s)\n", win.Backend())
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

	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			direct.MarkNeedsPaint()
			replay.MarkNeedsPaint()
		case wrkit.PhaseSpike:
			direct.MarkNeedsPaint()
			replay.MarkNeedsPaint()
		default:
			// Recover: no dirties — pure replay of layer tree.
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (op count + recorded Bounds are the R5 proof on screen).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PictureOpCount >= 3 || opCount >= 3
		shell.UpdateHUD("R5", phase, app, gateOK,
			fmt.Sprintf("picture_ops=%d", opCount),
			fmt.Sprintf("bounds=%s", boundsTxt))
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

	// R5 evidence: sample the direct and replay boxes at the same local
	// coordinates. Color probes run on the GPU snapshot; pair probes compare
	// direct vs replay directly (this replaces the old hardcoded
	// "direct_replay_eq: true").
	bx, by := shell.Body.X, shell.Body.Y
	boxW, boxH := shell.Body.W, shell.Body.H
	dx := bx + (boxW-200)*0.10
	dy := by + (boxH-300)*0.10
	rx := bx + (boxW-200)*0.55
	ry := by + (boxH-300)*0.10

	pixelChecks := []pixelCheck{
		{desc: "direct_tri", x: dx + 55, y: dy + 40, want: [3]float64{0.90, 0.30, 0.20}, tol: 0.06},
		{desc: "replay_tri", x: rx + 55, y: ry + 40, want: [3]float64{0.90, 0.30, 0.20}, tol: 0.06},
		{desc: "direct_rect", x: dx + 60, y: dy + 210, want: [3]float64{0.30, 0.80, 0.40}, tol: 0.06},
		{desc: "replay_rect", x: rx + 60, y: ry + 210, want: [3]float64{0.30, 0.80, 0.40}, tol: 0.06},
		{desc: "direct_img", x: dx + 158, y: dy + 28, want: [3]float64{0.20, 0.749, 0.349}, tol: 0.06},
		{desc: "replay_img", x: rx + 158, y: ry + 28, want: [3]float64{0.20, 0.749, 0.349}, tol: 0.06},
		{desc: "direct_stroke", x: dx + 55, y: dy + 120, want: [3]float64{0.20, 0.60, 0.90}, tol: 0.06},
		{desc: "replay_stroke", x: rx + 55, y: ry + 120, want: [3]float64{0.20, 0.60, 0.90}, tol: 0.06},
		{desc: "direct_srect", x: dx + 150, y: dy + 190, want: [3]float64{0.90, 0.80, 0.30}, tol: 0.06},
		{desc: "replay_srect", x: rx + 150, y: ry + 190, want: [3]float64{0.90, 0.80, 0.30}, tol: 0.06},
		{desc: "static_grid", x: bx + 700, y: by + 40, want: [3]float64{0.25, 0.50, 0.70}, tol: 0.06},
		{desc: "text_ink", x: dx + 50, y: dy + 255, tol: 0.04, kind: kindNotBG},
		{desc: "body_bg", x: bx + 400, y: by + 600, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	pairs := []pairCheck{
		{desc: "tri", ax: dx + 55, ay: dy + 40, bx: rx + 55, by: ry + 40, tol: 0.02},
		{desc: "rect", ax: dx + 60, ay: dy + 210, bx: rx + 60, by: ry + 210, tol: 0.02},
		{desc: "img", ax: dx + 158, ay: dy + 28, bx: rx + 158, by: ry + 28, tol: 0.02},
		{desc: "stroke", ax: dx + 55, ay: dy + 120, bx: rx + 55, by: ry + 120, tol: 0.02},
		{desc: "srect", ax: dx + 150, ay: dy + 190, bx: rx + 150, by: ry + 190, tol: 0.02},
	}

	img := loadImage(snapPath)
	pixelResult := make(map[string]bool, len(pixelChecks))
	runPixelChecks(img, pixelChecks, pixelResult)
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	pairOK, _ := runPairChecks(img, pairs)
	pairTotal := len(pairs)

	goldenRects := []rect{
		{x: 0, y: 0, w: winW, h: 48},
		{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
		{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	directReplayEq := pairOK == pairTotal && pixelOK == len(pixelChecks)
	logicProbes := []string{
		"recorded_ops_8",
		"picture_ops_ge_3",
		"direct_replay_eq",
		"presents_positive",
		"paint_count_positive",
		"static_grid_pixel",
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if opCount != 8 {
		logicFailed = append(logicFailed, "recorded_ops_8")
	}
	if opCount < 3 {
		logicFailed = append(logicFailed, "picture_ops_ge_3")
	}
	if !directReplayEq {
		logicFailed = append(logicFailed, "direct_replay_eq")
	}
	if app.PresentCount() < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if snap.PaintCount <= 0 {
		logicFailed = append(logicFailed, "paint_count_positive")
	}
	if !pixelResult["static_grid"] {
		logicFailed = append(logicFailed, "static_grid_pixel")
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R5",
		Scenario:      "ui_wr_r5_picture",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"recorded_ops":              opCount,
			"picture_bounds":            boundsTxt,
			"direct_replay_eq":          directReplayEq,
			"direct_replay_pairs_ok":    pairOK,
			"direct_replay_pairs_total": pairTotal,
			"pixel_probes_ok":           pixelOK,
			"pixel_probes_total":        len(pixelChecks),
			"pixel_golden_diff_pct":     goldenDiffPct,
			"pixel_golden_total_px":     goldenTotalPx,
			"pixel_golden_first_run":    goldenFirstRun,
			"phase_mode":                "static-content",
			"fps_gate":                  "off_correctness",
			"slope_gate":                "off",
			"logic_probes_ok":           len(logicProbes) - len(logicFailed),
			"logic_probes_total":        len(logicProbes),
			"logic_bad":                 logicFailed,
		},
	})
	// picture_op_count measures the retained display list itself: the recorded
	// ops that Replay executes each frame (scene-side, honest op count).
	report.PictureOpCount = int64(opCount)
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
	if pixelOK != len(pixelChecks) {
		fail("pixel probes %d/%d ok", pixelOK, len(pixelChecks))
	}
	if pairOK != pairTotal {
		fail("direct/replay pixel pairs %d/%d ok", pairOK, pairTotal)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	if gateMode {
		if errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			MinPictureOpCount:      3,
			RequireFullPaintPolicy: true,
		}); errG != nil {
			fail("%v", errG)
		}
		if snap.VSyncSource == "" {
			fail("vsync_source missing")
		}
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r5_picture: OK ops=%d bounds=%s presents=%d logic=%d/%d pixel=%d/%d pairs=%d/%d golden=%.4f%% mode=%s elapsed=%.1fs\n",
		opCount, boundsTxt, app.PresentCount(),
		len(logicProbes)-len(logicFailed), len(logicProbes),
		pixelOK, len(pixelChecks), pairOK, pairTotal, goldenDiffPct,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
