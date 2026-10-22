// Command ui_wr_r12b_debug_repaint is the W1 R12b real-window: debug repaint
// visualization.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint -auto-only
//	WR_DEBUG_REPAINT=0 RUN_SECONDS=8 go run ./examples/ui_wr_r12b_debug_repaint -auto-only
//	go run ./examples/ui_wr_r12b_debug_repaint -manual-seconds 15
//
// Window: 1200x800. R12b close duration is 8s; a gate run below 8s is rejected.
//
// Evidence: the overlay is a real PaintContext.NoteDebugRepaint stroke on live
// color-box paints, not an example-side drawing. Gate mode freezes Steady and
// deterministically exercises both sides of the contract:
//   - gateLive is a plain (non-boundary) leaf, so FullPaint repaints it every
//     frame; its centre is a fixed magenta blend in the on run and raw red in
//     the off run.
//   - hotA/hotB and all static panels are clean RepaintBoundaries, so they
//     Replay from cache and stay raw-coloured (no magenta).
//
// On/off are separate gate runs with separate snapshots and Golden baselines
// (showcase_r12b_debug_on*.png / showcase_r12b_debug_off*.png). Manual mode keeps
// Steady -> Spike -> Recover animation and does not assert pixels.
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
	closeSecs  = 8

	staticBlockN = 4
	cachedStripN = 8
	gridCells    = 16
	footerCellN  = 8

	goldenMaskPx = 1200*48 + 260*656 + 904*656

	blockX0, blockY, blockW, blockH, blockStep = 20.0, 20.0, 100.0, 80.0, 130.0
	cachedX, cachedY, cachedW, cachedH         = 20.0, 120.0, 520.0, 190.0
	gridX, gridY, gridW, gridH                 = 20.0, 330.0, 300.0, 200.0
	footerX, footerY, footerW, footerH         = 20.0, 550.0, 520.0, 86.0
	guideX, guideY, guideW, guideH             = 560.0, 270.0, 320.0, 300.0
	hotAX, hotAY, hotAW, hotAH                 = 580.0, 24.0, 110.0, 110.0
	hotBX, hotBY, hotBW, hotBH                 = 580.0, 160.0, 80.0, 80.0
	gateLiveX, gateLiveY, gateLiveW, gateLiveH = 720.0, 24.0, 100.0, 100.0
)

var (
	blockColor     = [3]float64{0.24, 0.55, 0.82}
	cachedPanelBG  = [3]float64{0.13, 0.16, 0.20}
	gridPanelBG    = [3]float64{0.12, 0.14, 0.18}
	footerPanelBG  = [3]float64{0.12, 0.15, 0.19}
	guidePanelBG   = [3]float64{0.14, 0.15, 0.19}
	hotABase       = [3]float64{0.95, 0.60, 0.15}
	hotBBase       = [3]float64{0.24, 0.76, 0.44}
	gateLiveBase   = [3]float64{0.90, 0.20, 0.20}
	overlayMagenta = [3]float64{0.95, 0.15, 0.85}
	overlayAlpha   = 0.35
	bodyBG         = [3]float64{0.10, 0.11, 0.13}
	legendBG       = [3]float64{0.11, 0.12, 0.15}
	topBG          = [3]float64{0.12, 0.14, 0.18}
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R12b")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R12b (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	debugOn := os.Getenv("WR_DEBUG_REPAINT") != "0"
	debugState := map[bool]string{true: "on", false: "off"}[debugOn]
	stateSuffix := map[bool]string{true: "on", false: "off"}[debugOn]

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_wr_r12b_debug_repaint — 重绘可视化 (" + debugState + ")",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R12b debug重绘 — magenta=live paint", []string{
		"magenta = live paint",
		"raw = clean boundary Replay",
		"on: draws>=1 (overlay)",
		"off: draws==0 (no overlay)",
		"gate: Steady frozen",
		"gate live leaf always paints",
		"manual: Steady/Spike/Recover",
		"Golden: 821184 px, 0 tolerance",
	})

	boundaries := make(map[rendering.RenderObject]bool)
	staticLabels := 0
	staticColorBoxes := 0

	addBodyLabel := func(text string, size, x, y, r, g, b float64) *rendering.RenderText {
		t := wrkit.Label(text, size, r, g, b)
		shell.Body.Place(t, x, y)
		staticLabels++
		return t
	}
	mkBoundaryPanel := func(w, h, r, g, b float64, name string) *wrkit.Panel {
		p := wrkit.NewPanel(w, h, r, g, b, 1)
		p.Box.SetRepaintBoundary(true)
		p.Box.SetDebugName(name)
		boundaries[p.Box] = true
		return p
	}
	mkStaticBox := func(w, h, r, g, b float64, name string) *rendering.RenderColorBox {
		c := rendering.NewRenderColorBox(w, h, r, g, b, 1)
		c.SetRepaintBoundary(true)
		c.SetDebugName(name)
		boundaries[c] = true
		return c
	}

	// 4x4 static grid inside a clean boundary: cached as one Picture, never
	// magenta after the first record.
	gridPanel := mkBoundaryPanel(gridW, gridH, gridPanelBG[0], gridPanelBG[1], gridPanelBG[2], "grid_panel")
	gridPanel.PlaceOn(shell.Body.Box, gridX, gridY)
	gridPanel.LabelAt("4x4 STATIC GRID — cached", 12, 12, 8, 0.70, 0.80, 0.92)
	staticLabels++
	gridWant := func(i, j int) [3]float64 {
		return [3]float64{0.22 + 0.06*float64(i), 0.40 + 0.05*float64(j), 0.58 + 0.02*float64(i)}
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			want := gridWant(i, j)
			gridPanel.ColorAt(30, 30, 14+float64(i)*36, 34+float64(j)*36, want[0], want[1], want[2], 1, false)
			staticColorBoxes++
		}
	}
	for i, line := range []string{"cells stay raw", "panel = boundary", "no dirty mark", "Replay every frame"} {
		gridPanel.LabelAt(line, 11, 170, 42+float64(i)*28, 0.68, 0.76, 0.88)
		staticLabels++
	}

	// 4 static boundary blocks: clean boundary controls. They record once,
	// then Replay; no overlay after the first frame.
	staticBlocks := make([]*rendering.RenderColorBox, 0, staticBlockN)
	for i := 0; i < staticBlockN; i++ {
		c := mkStaticBox(blockW, blockH, blockColor[0], blockColor[1], blockColor[2], fmt.Sprintf("static_block_%d", i))
		shell.Body.Place(c, blockX0+float64(i)*blockStep, blockY)
		staticBlocks = append(staticBlocks, c)
		staticColorBoxes++
	}

	// Static cached panel: 8 labels + 8 non-boundary strips. The strips are
	// plain children, but the panel boundary bakes them into one Picture, so a
	// clean frame never live-paints (or overlays) them.
	cached := mkBoundaryPanel(cachedW, cachedH, cachedPanelBG[0], cachedPanelBG[1], cachedPanelBG[2], "cached_panel")
	cached.PlaceOn(shell.Body.Box, cachedX, cachedY)
	cached.LabelAt("STATIC CACHED PANEL — boundary Replay, no magenta", 12, 12, 8, 0.70, 0.80, 0.92)
	staticLabels++
	for i := 0; i < 8; i++ {
		cached.LabelAt(fmt.Sprintf("static label %d", i), 11,
			12+float64(i%4)*126, 38+float64(i/4)*28, 0.70, 0.78, 0.88)
		staticLabels++
	}
	stripWant := func(i int) [3]float64 {
		return [3]float64{0.25 + 0.05*float64(i), 0.52 + 0.03*float64(i%3), 0.62 + 0.02*float64(i%2)}
	}
	for i := 0; i < cachedStripN; i++ {
		want := stripWant(i)
		cached.ColorAt(50, 34, 12+float64(i)*62, 108, want[0], want[1], want[2], 1, false)
		staticColorBoxes++
	}
	cached.LabelAt("strips are plain children, cached with the panel", 11, 12, 154, 0.62, 0.70, 0.82)
	staticLabels++

	// Static footer: one more cached strip row, close to the body edge.
	footer := mkBoundaryPanel(footerW, footerH, footerPanelBG[0], footerPanelBG[1], footerPanelBG[2], "footer_panel")
	footer.PlaceOn(shell.Body.Box, footerX, footerY)
	footer.LabelAt("STATIC FOOTER — cached", 11, 12, 6, 0.70, 0.78, 0.90)
	staticLabels++
	footerWant := func(i int) [3]float64 {
		return [3]float64{0.30 + 0.04*float64(i), 0.45 + 0.04*float64(i%4), 0.66 - 0.02*float64(i)}
	}
	for i := 0; i < footerCellN; i++ {
		want := footerWant(i)
		footer.ColorAt(50, 34, 12+float64(i)*62, 34, want[0], want[1], want[2], 1, false)
		staticColorBoxes++
	}

	// Hot boundary controls. In gate mode both stay clean (raw colours); in
	// manual mode hotA is dirtied every Steady/Spike tick and hotB in Spike.
	hotA := rendering.NewRenderColorBox(hotAW, hotAH, hotABase[0], hotABase[1], hotABase[2], 1)
	hotA.SetRepaintBoundary(true)
	hotA.SetDebugName("hotA")
	boundaries[hotA] = true
	shell.Body.Place(hotA, hotAX, hotAY)

	hotB := rendering.NewRenderColorBox(hotBW, hotBH, hotBBase[0], hotBBase[1], hotBBase[2], 1)
	hotB.SetRepaintBoundary(true)
	hotB.SetDebugName("hotB")
	boundaries[hotB] = true
	shell.Body.Place(hotB, hotBX, hotBY)

	// gateLive is deliberately a plain (non-boundary) leaf. FullPaint repaints
	// it every frame, so its snapshot overlay is deterministic: on = magenta
	// blend, off = raw red. It is gate-only; manual Recover must be able to
	// stop all overlay, and a plain leaf would keep painting under FullPaint.
	var gateLive *rendering.RenderColorBox
	if gateMode {
		gateLive = rendering.NewRenderColorBox(gateLiveW, gateLiveH, gateLiveBase[0], gateLiveBase[1], gateLiveBase[2], 1)
		gateLive.SetDebugName("gate_live_leaf")
		shell.Body.Place(gateLive, gateLiveX, gateLiveY)
	}

	hotALabelText := "hotA: clean (raw)"
	hotBLabelText := "hotB: clean (raw green)"
	if !gateMode {
		hotALabelText = "hotA: dirty every tick"
		hotBLabelText = "hotB: Spike only"
	}
	addBodyLabel(hotALabelText, 11, hotAX, 6, 0.88, 0.80, 0.90)
	addBodyLabel(hotBLabelText, 11, hotBX, hotBY-18, 0.80, 0.88, 0.84)
	if gateLive != nil {
		addBodyLabel("gate live leaf (always painted)", 11, gateLiveX, 6, 0.94, 0.74, 0.78)
	}

	guide := mkBoundaryPanel(guideW, guideH, guidePanelBG[0], guidePanelBG[1], guidePanelBG[2], "guide_panel")
	guide.PlaceOn(shell.Body.Box, guideX, guideY)
	guide.LabelAt("PROBE GUIDE", 13, 12, 10, 0.78, 0.86, 0.96)
	staticLabels++
	guideLines := []string{
		"magenta = a live color-box paint",
		"raw colour = clean boundary Replay",
		"static panels are boundaries",
		"hotA/hotB clean in gate",
		"gate live leaf is non-boundary",
		"FullPaint repaints it every frame",
		"on: live leaf = magenta blend",
		"off: live leaf = raw red",
		"manual Recover stops dirty marks",
	}
	if !gateMode {
		guideLines[4] = "manual: hotA Steady live"
		guideLines[5] = "Spike adds hotB; Recover stops"
	}
	for i, line := range guideLines {
		guide.LabelAt(line, 11, 12, 38+float64(i)*22, 0.68, 0.76, 0.88)
		staticLabels++
	}

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r12b_debug_repaint", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapName := "showcase_r12b_debug_" + stateSuffix + ".png"
	baseName := "showcase_r12b_debug_" + stateSuffix + "_base.png"
	snapPath := filepath.Join(snapDir, snapName)
	snapshotPath := snapPath
	if !gateMode {
		// Manual mode animates, so its frame is not evidence. Keep the gate
		// snapshot path empty there: a later manual run must not overwrite the
		// committed gate PNG the Golden baseline was seeded from.
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
				fmt.Fprintf(os.Stderr, "ui_wr_r12b_debug_repaint: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Gate mode never resizes, so pixel/Golden coordinates stay fixed.
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// FullPaint correctness window: a plain leaf is painted every frame, which
	// is exactly what makes the gate live leaf's overlay deterministic. Retained
	// policy would not present every plain leaf and R12b would not be observable.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetDebugRepaint(debugOn)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{wrkit.PhaseSteady: true}
	clock := wrkit.NewPhaseClock(2, 5)
	hotTicks := int64(0)

	// Debug draw accounting after the first-frame static records. The overlay
	// counter is cumulative; per-phase deltas are observational evidence, not a
	// frame-exact count (pipeline presents are asynchronous).
	drawsAtWarmup := int64(-1)
	lastDraws := int64(0)
	drawsSteady, drawsSpike, drawsRecover := int64(0), int64(0), int64(0)
	recoverTicks := 0
	recoverSettled := false
	recoverDrawsAtSettle := int64(0)

	lastRR := int64(0)
	rrSteady, rrSpike, rrRecover := int64(0), int64(0), int64(0)
	lastLiveSnap := scheduler.FrameMetrics{}
	lastPhaseLog := ""

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady: hotA/hotB stay clean boundaries. gateLive is a
			// plain leaf, so FullPaint paints it every frame with a fixed
			// colour; the snapshot is therefore bit-stable and still shows a
			// real live-paint overlay in the on run.
			phase = wrkit.PhaseSteady
			hotA.R, hotA.G, hotA.B = hotABase[0], hotABase[1], hotABase[2]
			hotB.R, hotB.G, hotB.B = hotBBase[0], hotBBase[1], hotBBase[2]
			hotTicks++
		} else {
			phase = clock.Advance(dt)
			switch phase {
			case wrkit.PhaseSteady:
				// hotA is a RepaintBoundary: dirty every tick, live overlay.
				hotA.R = hotABase[0] - 0.20*(clock.Elapsed()/2)
				hotA.G, hotA.B = hotABase[1], hotABase[2]
				hotA.MarkNeedsPaint()
			case wrkit.PhaseSpike:
				// Two independent dirty boundaries, both changing content
				// every tick. A once-per-second value would let the content
				// key reuse the cached Picture and hide most spike frames.
				pulse := float64(hotTicks%60) / 60.0
				hotA.R = hotABase[0] - 0.22*pulse
				hotA.MarkNeedsPaint()
				hotB.G = hotBBase[1] - 0.18*pulse
				hotB.MarkNeedsPaint()
			default:
				// Recover: no dirty marks. The first clean frame may still
				// carry the previous dirty state; after that every boundary
				// Replays and no new overlay is drawn.
			}
			hotTicks++
		}

		phaseSeen[phase] = true
		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		lastLiveSnap = snapH
		drawsNow := app.DebugRepaintDraws()
		if !gateMode && (phase != lastPhaseLog || hotTicks%120 == 0) {
			fmt.Fprintf(os.Stderr,
				"ui_wr_r12b_debug_repaint: phase=%s tick=%d clip=%.2fs draws=%d rr=%d hotA_dirty=%v hotB_dirty=%v\n",
				phase, hotTicks, clock.Elapsed(), drawsNow, snapH.BoundaryRerecord,
				hotA.NeedsPaint(), hotB.NeedsPaint())
			lastPhaseLog = phase
		}

		if drawsAtWarmup < 0 && hotTicks >= 10 {
			drawsAtWarmup = drawsNow
			lastDraws = drawsNow
		}
		if drawsAtWarmup >= 0 && drawsNow >= lastDraws {
			d := drawsNow - lastDraws
			if d > 0 {
				switch phase {
				case wrkit.PhaseSteady:
					drawsSteady += d
				case wrkit.PhaseSpike:
					drawsSpike += d
				default:
					drawsRecover += d
				}
			}
			lastDraws = drawsNow
		}
		if !gateMode && phase == wrkit.PhaseRecover {
			recoverTicks++
			if recoverTicks == 60 {
				recoverSettled = true
				recoverDrawsAtSettle = drawsNow
			}
		}

		if snapH.BoundaryRerecord >= lastRR {
			d := snapH.BoundaryRerecord - lastRR
			if d > 0 {
				switch phase {
				case wrkit.PhaseSteady:
					rrSteady += d
				case wrkit.PhaseSpike:
					rrSpike += d
				default:
					rrRecover += d
				}
			}
			lastRR = snapH.BoundaryRerecord
		}

		gateOK := (debugOn && drawsNow >= 1) || (!debugOn && drawsNow == 0)
		shell.UpdateHUD("R12b", phase, app, gateOK,
			wrkit.FmtSkip(snapH.BoundarySkip, snapH.BoundaryRerecord),
			fmt.Sprintf("debug=%s draws=%d", debugState, drawsNow))
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
	draws := app.DebugRepaintDraws()

	if snap.BoundaryRerecord >= lastRR {
		d := snap.BoundaryRerecord - lastRR
		if d > 0 {
			switch phase {
			case wrkit.PhaseSteady:
				rrSteady += d
			case wrkit.PhaseSpike:
				rrSpike += d
			default:
				rrRecover += d
			}
		}
		lastRR = snap.BoundaryRerecord
	}
	if drawsAtWarmup >= 0 && draws >= lastDraws {
		d := draws - lastDraws
		if d > 0 {
			switch phase {
			case wrkit.PhaseSteady:
				drawsSteady += d
			case wrkit.PhaseSpike:
				drawsSpike += d
			default:
				drawsRecover += d
			}
		}
		lastDraws = draws
	}
	drawsAfterWarmup := int64(0)
	if drawsAtWarmup >= 0 {
		drawsAfterWarmup = draws - drawsAtWarmup
	}
	recoverDrawsAfterSettle := int64(0)
	if recoverSettled {
		recoverDrawsAfterSettle = draws - recoverDrawsAtSettle
	}

	// Structural truth: every static colour box and hot control is a boundary;
	// only gateLive is a plain leaf. HUD is a boundary too.
	boundaryCountExpected := int64(len(boundaries))
	if shell.HUD != nil {
		boundaryCountExpected++
	}
	staticNodes := []rendering.RenderObject{
		gridPanel.Box, cached.Box, footer.Box, guide.Box,
	}
	for _, c := range staticBlocks {
		staticNodes = append(staticNodes, c)
	}
	staticClean := true
	for _, n := range staticNodes {
		if n == nil || n.NeedsPaint() {
			staticClean = false
			break
		}
	}
	hotAClean := !hotA.NeedsPaint()
	hotBClean := !hotB.NeedsPaint()
	gateLiveClean := gateLive == nil || !gateLive.NeedsPaint()

	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	// The gate's final SnapshotPath readback is not a live present. Keep the
	// last live tick's build/raster read so the 120ms readback does not get
	// misread as a steady-frame raster regression.
	liveFPSInterval := 0.0
	if lastLiveSnap.AvgFrameIntervalMs > 1e-6 {
		liveFPSInterval = 1000.0 / lastLiveSnap.AvgFrameIntervalMs
	}

	bx, by := shell.Body.X, shell.Body.Y
	blendOnBase := func(base [3]float64) [3]float64 {
		return [3]float64{
			overlayMagenta[0]*overlayAlpha + base[0]*(1-overlayAlpha),
			overlayMagenta[1]*overlayAlpha + base[1]*(1-overlayAlpha),
			overlayMagenta[2]*overlayAlpha + base[2]*(1-overlayAlpha),
		}
	}
	pixelChecks := make([]pixelCheck, 0, 32)
	pixelChecks = append(pixelChecks,
		pixelCheck{desc: "static_block_0", x: bx + 70, y: by + 60, want: blockColor, tol: 0.06},
		pixelCheck{desc: "static_block_3", x: bx + 460, y: by + 60, want: blockColor, tol: 0.06},
		pixelCheck{desc: "cached_panel_bg", x: bx + 530, y: by + 300, want: cachedPanelBG, tol: 0.05, kind: kindBG},
		pixelCheck{desc: "cached_strip_0", x: bx + 57, y: by + 245, want: stripWant(0), tol: 0.06},
		pixelCheck{desc: "cached_strip_3", x: bx + 243, y: by + 245, want: stripWant(3), tol: 0.06},
		pixelCheck{desc: "cached_strip_7", x: bx + 491, y: by + 245, want: stripWant(7), tol: 0.06},
		pixelCheck{desc: "grid_bg", x: bx + 310, y: by + 520, want: gridPanelBG, tol: 0.05, kind: kindBG},
		pixelCheck{desc: "grid_cell_00", x: bx + 49, y: by + 379, want: gridWant(0, 0), tol: 0.06},
		pixelCheck{desc: "grid_cell_11", x: bx + 85, y: by + 415, want: gridWant(1, 1), tol: 0.06},
		pixelCheck{desc: "grid_cell_22", x: bx + 121, y: by + 451, want: gridWant(2, 2), tol: 0.06},
		pixelCheck{desc: "grid_cell_33", x: bx + 157, y: by + 487, want: gridWant(3, 3), tol: 0.06},
		pixelCheck{desc: "grid_cell_30", x: bx + 157, y: by + 379, want: gridWant(3, 0), tol: 0.06},
		pixelCheck{desc: "footer_bg", x: bx + 530, y: by + 630, want: footerPanelBG, tol: 0.05, kind: kindBG},
		pixelCheck{desc: "footer_cell_0", x: bx + 57, y: by + 601, want: footerWant(0), tol: 0.06},
		pixelCheck{desc: "footer_cell_7", x: bx + 491, y: by + 601, want: footerWant(7), tol: 0.06},
		pixelCheck{desc: "guide_bg", x: bx + 870, y: by + 560, want: guidePanelBG, tol: 0.05, kind: kindBG},
		pixelCheck{desc: "hotA_clean_orange", x: bx + hotAX + hotAW/2, y: by + hotAY + hotAH/2, want: hotABase, tol: 0.06},
		pixelCheck{desc: "hotA_corner_clean", x: bx + hotAX + 8, y: by + hotAY + 8, want: hotABase, tol: 0.06},
		pixelCheck{desc: "hotB_clean_green", x: bx + hotBX + hotBW/2, y: by + hotBY + hotBH/2, want: hotBBase, tol: 0.06},
		pixelCheck{desc: "hotB_corner_clean", x: bx + hotBX + 8, y: by + hotBY + 8, want: hotBBase, tol: 0.06},
		pixelCheck{desc: "body_bg", x: bx + 890, y: by + 640, want: bodyBG, tol: 0.05, kind: kindBG},
		pixelCheck{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: legendBG, tol: 0.05, kind: kindBG},
		pixelCheck{desc: "top_bg", x: 1000, y: 24, want: topBG, tol: 0.05, kind: kindBG},
	)
	if gateLive != nil {
		liveWant := gateLiveBase
		liveTol := 0.06
		if debugOn {
			liveWant = blendOnBase(gateLiveBase)
			liveTol = 0.08
		}
		pixelChecks = append(pixelChecks,
			pixelCheck{desc: "gate_live_center", x: bx + gateLiveX + gateLiveW/2, y: by + gateLiveY + gateLiveH/2, want: liveWant, tol: liveTol},
			pixelCheck{desc: "gate_live_corner", x: bx + gateLiveX + 8, y: by + gateLiveY + 8, want: liveWant, tol: liveTol},
		)
	}

	inkRegions := []inkRegion{
		{desc: "header_title_ink", x: 16, y: 14, w: 420, h: 18, bg: topBG, tol: 0.04, minPx: 40},
		{desc: "legend_line0_ink", x: shell.Legend.X + 12, y: shell.Legend.Y + 36, w: 220, h: 14, bg: legendBG, tol: 0.04, minPx: 20},
		{desc: "cached_title_ink", x: bx + 32, y: by + 128, w: 340, h: 14, bg: cachedPanelBG, tol: 0.04, minPx: 20},
		{desc: "cached_label0_ink", x: bx + 32, y: by + 158, w: 90, h: 14, bg: cachedPanelBG, tol: 0.04, minPx: 8},
		{desc: "grid_title_ink", x: bx + 32, y: by + 338, w: 180, h: 14, bg: gridPanelBG, tol: 0.04, minPx: 10},
		{desc: "grid_side_label_ink", x: bx + 190, y: by + 368, w: 100, h: 14, bg: gridPanelBG, tol: 0.04, minPx: 8},
		{desc: "footer_title_ink", x: bx + 32, y: by + 556, w: 220, h: 14, bg: footerPanelBG, tol: 0.04, minPx: 10},
		{desc: "guide_title_ink", x: bx + 572, y: by + 280, w: 120, h: 14, bg: guidePanelBG, tol: 0.04, minPx: 8},
		{desc: "hotA_label_ink", x: bx + hotAX, y: by + 6, w: 170, h: 12, bg: bodyBG, tol: 0.04, minPx: 8},
		{desc: "hotB_label_ink", x: bx + hotBX, y: by + hotBY - 18, w: 130, h: 12, bg: bodyBG, tol: 0.04, minPx: 8},
	}
	if gateLive != nil {
		inkRegions = append(inkRegions, inkRegion{desc: "gate_live_label_ink", x: bx + gateLiveX, y: by + 6, w: 180, h: 12, bg: bodyBG, tol: 0.04, minPx: 8})
	}

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
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, snapName, baseName, goldenRects)
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		pixelGoldenValue = nil
	}

	drawsPhaseSum := drawsSteady + drawsSpike + drawsRecover
	rrPhaseSum := rrSteady + rrSpike + rrRecover
	phases := make([]string, 0, 3)
	for _, p := range []string{wrkit.PhaseSteady, wrkit.PhaseSpike, wrkit.PhaseRecover} {
		if phaseSeen[p] {
			phases = append(phases, p)
		}
	}

	type probe struct {
		name string
		ok   bool
	}
	probes := []probe{
		{"presents_positive", presents >= 1},
		{"fps_interval_ge_55", fpsInterval >= 55},
		{"paint_count_positive", snap.PaintCount > 0},
		{"boundary_skip_positive", snap.BoundarySkip > 0},
		{"boundary_rerecord_positive", snap.BoundaryRerecord > 0},
		{"boundary_count_matches_scene", snap.BoundaryCount == boundaryCountExpected},
		{"present_policy_full", snap.PresentPolicy == scheduler.PresentPolicyFullPaint},
		{"cpu_paths_recorded", snap.CPUPctAvg > 0 && (snap.CPUUIPct > 0 || snap.CPURasterPct > 0)},
		{"cpu_fallback_zero", snap.CPUFallbackOps == 0},
		{"vsync_source_present", snap.VSyncSource != ""},
		{"static_labels_ge_8", staticLabels >= 8},
		{"static_color_boxes_ge_30", staticColorBoxes >= 30},
		{"debug_state_matches_env", (debugOn && draws > 0) || (!debugOn && draws == 0)},
		{"debug_draws_after_warmup_state", drawsAtWarmup >= 0 && ((debugOn && drawsAfterWarmup > 0) || (!debugOn && drawsAfterWarmup == 0))},
		{"draws_phase_sum", drawsAtWarmup >= 0 && drawsPhaseSum == drawsAfterWarmup},
		{"rr_phase_sum", rrPhaseSum == snap.BoundaryRerecord},
	}
	if gateMode {
		probes = append(probes,
			probe{"gate_phase_frozen", phase == wrkit.PhaseSteady},
			probe{"gate_rr_spike_zero", rrSpike == 0},
			probe{"gate_rr_recover_zero", rrRecover == 0},
			probe{"gate_hotA_clean", hotAClean},
			probe{"gate_hotB_clean", hotBClean},
			probe{"gate_static_clean", staticClean},
			probe{"gate_live_leaf_clean", gateLiveClean},
			probe{"gate_pixel_probes_all", pixelOK == pixelTotal && pixelTotal > 0},
			probe{"gate_golden_zero", goldenFirstRun || (goldenDiffPct == 0 && goldenTotalPx == goldenMaskPx)},
		)
	} else {
		probes = append(probes,
			probe{"manual_steady_draws_positive", drawsSteady > 0},
			probe{"manual_recover_settled", phaseSeen[wrkit.PhaseRecover] && recoverSettled},
		)
		if phaseSeen[wrkit.PhaseSpike] {
			probes = append(probes, probe{"manual_spike_draws_positive", drawsSpike > 0})
		}
		if phaseSeen[wrkit.PhaseRecover] && recoverSettled {
			probes = append(probes, probe{"manual_recover_draws_zero", recoverDrawsAfterSettle == 0})
		}
		if *manualSeconds >= 15 {
			probes = append(probes, probe{"manual_all_phases", phaseSeen[wrkit.PhaseSteady] && phaseSeen[wrkit.PhaseSpike] && phaseSeen[wrkit.PhaseRecover]})
		}
	}
	logicFailed := make([]string, 0)
	for _, p := range probes {
		if !p.ok {
			logicFailed = append(logicFailed, p.name)
		}
	}

	extra := map[string]any{
		"client_px":              "1200x800",
		"run_seconds":            secs,
		"close_seconds_required": closeSecs,
		"manual_seconds":         *manualSeconds,
		"debug_state":            debugState,
		"debug_repaint_on":       debugOn,
		"debug_draws":            draws,
		"debug_draws_at_warmup":  drawsAtWarmup,
		"debug_draws_after":      drawsAfterWarmup,
		"draws_steady":           drawsSteady,
		"draws_spike":            drawsSpike,
		"draws_recover":          drawsRecover,
		"draws_phase_sum":        drawsPhaseSum,
		"recover_settled":        recoverSettled,
		"recover_draws_after":    recoverDrawsAfterSettle,
		"hot_ticks":              hotTicks,
		"rr_steady":              rrSteady,
		"rr_spike":               rrSpike,
		"rr_recover":             rrRecover,
		"rr_phase_sum":           rrPhaseSum,
		"static_boxes":           staticColorBoxes,
		"static_labels":          staticLabels,
		"static_clean":           staticClean,
		"hotA_clean":             hotAClean,
		"hotB_clean":             hotBClean,
		"gate_live_leaf":         gateLive != nil,
		"gate_live_clean":        gateLiveClean,
		"boundary_count_scene":   boundaryCountExpected,
		"present_policy":         snap.PresentPolicy,
		"live_frame_build_ms":    lastLiveSnap.LastBuildMs,
		"live_frame_raster_ms":   lastLiveSnap.LastRasterMs,
		"live_fps_interval":      liveFPSInterval,
		"frame_raster_note": map[bool]string{
			true:  "gate frame_raster_ms includes the final SnapshotPath GPU readback/PNG; live_frame_raster_ms is the last live tick and is the steady-frame figure",
			false: "manual frame_raster_ms is the last completed live present; manual mode has no SnapshotPath readback",
		}[gateMode],
		"phases_seen":            phases,
		"phase_mode":             map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
		"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
		"pixel_evidence":         map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
		"pixel_probes_ok":        pixelOK,
		"pixel_probes_total":     pixelTotal,
		"pixel_bad":              pixelBad,
		"pixel_golden_diff_pct":  pixelGoldenValue,
		"pixel_golden_total_px":  goldenTotalPx,
		"pixel_golden_first_run": goldenFirstRun,
		"golden_mask":            "header 1200x48 + legend 260x656 + body 904x656",
		"golden_mask_px":         goldenMaskPx,
		"golden_tolerance_pct":   0,
		"snapshot_png":           snapshotPath,
		"snapshot_note": map[bool]string{
			true:  "gate snapshot (GPU readback); SnapshotPath now carries the R12b debug flag, so the overlay is in the evidence PNG",
			false: "manual mode does not write the gate snapshot; committed evidence is not overwritten",
		}[gateMode],
		"debug_overlay_semantics": "NoteDebugRepaint draws only on live color/picture paints; clean RepaintBoundary Replay bypasses it",
		"fps_gate":                "persistent_tick_ge_55",
		"slope_gate":              map[bool]string{true: "off_8s_short_run_allowed", false: "off_manual_observation_not_a_soak"}[gateMode],
		"rss_slope_note":          "rss_slope_valid=false: an 8s gate/15s manual run is shorter than the >=30s steady-slope verdict; raw slope is startup-inclusive and is not a leak conclusion",
		"rss_slope_steady_note":   "steady slope is omitted while valid=false (short run); omission means 'cannot conclude', not 'no leak' (TestBuildReport_ShortRunSlopeInvalidIsHonest)",
		"cpu_fallback_note":       "none observed; last_cpu_fallback empty means no fallback",
		"gpu_ops_semantics":       "cumulative",
		"g_metrics":               "skipped",
		"g_metrics_reason":        "R12b is a debug-overlay correctness window; measure cache fields are observed but not gated",
		"depcheck":                "passed_no_cgo_no_ui_gpu_import",
		"logic_probes_ok":         len(probes) - len(logicFailed),
		"logic_probes_total":      len(probes),
		"logic_bad":               logicFailed,
		"impl_correctness":        "overlay is drawn by PaintContext.NoteDebugRepaint inside the live color-box path; clean boundary Replay returns before the overlay. Gate live leaf is a plain leaf painted every FullPaint frame, so on=blend/off=raw is deterministic; hotA/hotB/static boundaries stay raw.",
		"impl_dirty":              "gate: frozen Steady, no boundary dirt (static/hotA/hotB clean); gate live leaf is non-boundary and FullPaint repaints it every frame. manual: Steady hotA changes every tick; Spike hotA+hotB change every tick; Recover stops dirty marks after the last Spike frame.",
		"impl_cache":              "static blocks/panels are RepaintBoundaries: first live paint records, later frames Replay (boundary_skip) with no overlay; their pixels stay raw. hotA/hotB are clean boundary controls in gate.",
		"impl_edge":               "on/off separate gate runs and separate Golden baselines; 36 static color boxes, 20+ static labels, 4x4 grid, cached strips/footer, clean hotA/hotB controls, gate-only live leaf; FullPaint pinned; manual Steady->Spike->Recover.",
		"impl_fail":               "on debug_draws=0 or off debug_draws>0; live leaf pixel not blend/raw; hot/static pixel shows overlay; golden diff!=0 or mask px mismatch; logic fail; policy!=full_paint; fps<55 or p95>22 -> FAIL",
		"impl_visible":            "on: live leaf magenta blend, hotA orange/hotB green/static raw; off: live leaf raw red and no overlay anywhere; HUD shows debug state, flash draws, skip/rr and phase; manual cycles Steady/Spike/Recover.",
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R12b",
		Scenario:      "ui_wr_r12b_debug_repaint",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         extra,
	})
	report.DebugRepaintDraws = draws
	report.DebugRepaintOn = debugOn
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		os.Exit(1)
	}
	if len(logicFailed) > 0 {
		fail("logic probes %d/%d bad=%v", len(probes)-len(logicFailed), len(probes), logicFailed)
	}
	if gateMode {
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		if !goldenFirstRun && goldenTotalPx != goldenMaskPx {
			fail("golden mask px=%d want %d", goldenTotalPx, goldenMaskPx)
		}
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
			RequirePersistentFPS:   true,
			MinFPSWall:             55,
			MinFPSElapsed:          5,
			MaxP95Ms:               22,
			MinBoundarySkip:        1,
			MinBoundaryRerecord:    1,
			MinBoundaryCount:       10,
			RequireDebugOn:         debugOn,
		}); err != nil {
			fail("%v", err)
		}
	}

	runStatus := "OK"
	if gateMode && goldenFirstRun {
		runStatus = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%% state=%s draws=%d", pixelOK, pixelTotal, goldenDiffPct, debugState, draws)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r12b_debug_repaint: %s presents=%d skip=%d rr=%d count=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		runStatus, presents, snap.BoundarySkip, snap.BoundaryRerecord,
		snap.BoundaryCount, len(probes)-len(logicFailed), len(probes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
