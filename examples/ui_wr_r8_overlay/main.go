// Command ui_wr_r8_overlay is the W4 R8 real-window: overlay independent
// compositing. A dense static main tree (grid + labels + nested boundary +
// HOT live-paint spot) runs under a scripted overlay stack: Steady (no
// overlay) → Spike (panel + barrier + inner interactive button stacked) →
// Recover (overlay removed). Opening/closing the overlay must dirty ONLY the
// overlay band — main-band dirty ids stay at their pre-overlay baseline and
// pipe paint_count does not grow from the overlay open.
//
// Retained policy (Plan C, Flutter-aligned): steady frames CompositeOnly +
// damage present; clean boundaries replay, only dirty layers re-record.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r8_overlay -auto-only
//	R8_GATE_STATE=closed RUN_SECONDS=15 go run ./examples/ui_wr_r8_overlay -auto-only
//	go run ./examples/ui_wr_r8_overlay                 # manual: cycling until close
//	go run ./examples/ui_wr_r8_overlay -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 is the generic floor, but a gate run below
// 15s cannot close R8 (§2.5). GPU window required.
//
// Evidence (U21): logic probes, two-state pixel probes on the GPU snapshot
// (R8_GATE_STATE=open: fixed modal popup stacked above the dimming barrier;
// closed: main band only), and a bitwise Golden per state over header + legend
// + body. Gate mode freezes the script — hot converges over 60 ticks then
// holds, the overlay opens once after the 1.2s baseline window and stays; the
// main-dirty isolation gate still runs against the frozen open. Manual mode
// keeps the Steady → SpikeCycle → Recover popup cycling and does not assert
// pixels.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	// §2.5 Overlay/命中组合关闭用时长: R8 = 15（Steady5 + SpikeCycle5 + Recover沉降；
	// 短于此时弹窗循环走不完，退出时浮层仍开属于时长误用而非引擎问题）。
	closeSeconds = 15
)

var proc scheduler.ProcessTracker

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze script, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	gateState := os.Getenv("R8_GATE_STATE")
	if gateState == "" {
		gateState = "open"
	}
	if gateState != "open" && gateState != "closed" {
		fmt.Fprintf(os.Stderr, "FAIL: R8_GATE_STATE=%q want open|closed\n", gateState)
		os.Exit(1)
	}
	snapshotPNG := "showcase_r8_overlay_" + gateState + ".png"
	baselinePNG := snapshotPNG[:len(snapshotPNG)-len(".png")] + "_base.png"

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R8")
		if secs < closeSeconds {
			fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 Overlay组合关闭用时长)\n", secs, closeSeconds)
			os.Exit(1)
		}
	} else if *autoOnly {
		secs, secsSet = closeSeconds, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d want >=%d (§2.5 Overlay组合关闭用时长)\n", secs, closeSeconds)
		os.Exit(1)
	}
	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	render.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r8_overlay — Overlay 独立合成", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R8 Overlay 独立合成 — 开浮层主树不重绘", []string{
		"MAIN  = 静态密集主树 (4x4色格+8标签+嵌套boundary)",
		"HOT   = 主树热点 (每帧变色, 全程活跃)",
		"Spike = 开浮层: 面板+屏障+内嵌按钮",
		"开浮层帧 main_dirty 必须保持基线",
		"Recover = 关浮层, 主树照常",
	})

	// ===== MAIN band: static dense tree (U17) =====
	dense := rendering.NewAbsoluteBox(560, 420)
	dense.Background = &rendering.Color{R: 0.11, G: 0.12, B: 0.18, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(96, 64,
				0.22+float64(i)*0.14, 0.38+float64(j)*0.11, 0.55+float64((i+j)%3)*0.12, 1)
			c.SetRepaintBoundary(true) // nested boundary: grid cells cache independently
			dense.Place(c, 12+float64(i)*108, 12+float64(j)*74)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		dense.Place(wrkit.Label(fmt.Sprintf("main-lbl-%d 静态文字", i), 11, 0.72, 0.8, 0.9),
			16+float64(col)*140, 320+float64(row)*26)
	}
	shell.Body.Box.Place(dense, 10, 10)

	// HOT spot: live-paint (non-boundary), active EVERY phase including overlay
	// open (user choice). Gate = main dirty count stays at its pre-overlay
	// baseline (HOT contributes exactly one id).
	hot := rendering.NewRenderColorBox(30, 30, 0.95, 0.3, 0.25, 1)
	shell.Body.Box.Place(hot, 480, 350)
	shell.Body.Box.Place(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 480, 386)

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r8_overlay", "testdata")
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
				fmt.Fprintf(os.Stderr, "ui_wr_r8_overlay: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// Plan C (Flutter-aligned posture): retained steady frames — only dirty
	// layers re-record; opening an overlay must not re-record the main band.
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	ov := overlay.New()
	app.SetOverlay(ov)

	// ===== Realistic app-scenario overlay cycling =====
	// Real apps open/close popups repeatedly, each open building FRESH
	// widgets (a new dialog is new RenderObjects). Three popup kinds rotate
	// with different sizes/positions so every cycle allocates fresh textures;
	// the phase clock paces: Steady (baseline) → Spike (cycles: 2s open /
	// 1.5s closed, kind rotates per cycle) → Recover (final close, settle).
	// This is the stress that exposes per-open leaks the single-open script
	// cannot see.
	buildPopup := func(kind int) []*overlay.Entry {
		barrier := rendering.NewRenderColorBox(1, 1, 0.02, 0.03, 0.06, 0.45)
		entries := []*overlay.Entry{overlay.NewBarrierEntry(0, 0, float64(winW), float64(winH), barrier)}
		switch kind % 3 {
		case 0: // modal dialog
			panel := rendering.NewAbsoluteBox(340, 240)
			panel.Background = &rendering.Color{R: 0.15, G: 0.17, B: 0.24, A: 1}
			panel.SetRepaintBoundary(true)
			panel.Place(wrkit.Label("OVERLAY PANEL 浮层面板", 14, 0.92, 0.95, 1), 16, 14)
			panel.Place(wrkit.Label(fmt.Sprintf("第 %d 次打开 · 对话框", kind+1), 11, 0.7, 0.78, 0.88), 16, 44)
			btn := rendering.NewAbsoluteBox(120, 34)
			btn.Background = &rendering.Color{R: 0.2, G: 0.5, B: 0.85, A: 1}
			btn.SetRepaintBoundary(true)
			btn.Place(wrkit.Label("浮层内按钮", 11, 1, 1, 1), 14, 9)
			panel.Place(btn, 16, 76)
			// Stacked second layer above the dialog: menu list.
			menu := rendering.NewAbsoluteBox(180, 150)
			menu.Background = &rendering.Color{R: 0.2, G: 0.24, B: 0.32, A: 1}
			menu.SetRepaintBoundary(true)
			for i := 0; i < 4; i++ {
				menu.Place(wrkit.Label(fmt.Sprintf("menu-item-%d", i), 11, 0.85, 0.9, 0.95), 12, 10+float64(i)*28)
			}
			entries = append(entries,
				overlay.NewEntry(panel, 380+float64(kind%3)*24, 260, 340, 240),
				overlay.NewEntry(menu, 700-float64(kind%2)*40, 200, 180, 150))
		case 1: // dropdown menu (no barrier — click-through allowed outside)
			drop := rendering.NewAbsoluteBox(220, 210)
			drop.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.30, A: 1}
			drop.SetRepaintBoundary(true)
			for i := 0; i < 6; i++ {
				row := rendering.NewAbsoluteBox(200, 26)
				row.Background = &rendering.Color{R: 0.14 + 0.03*float64(i%2), G: 0.18, B: 0.26, A: 1}
				row.SetRepaintBoundary(true)
				row.Place(wrkit.Label(fmt.Sprintf("下拉项-%d 选择", i), 11, 0.85, 0.9, 0.95), 10, 5)
				drop.Place(row, 10, 10+float64(i)*32)
			}
			entries = append(entries, overlay.NewEntry(drop, 420+float64(kind%4)*30, 180, 220, 210))
		default: // tooltip-ish sheet
			sheet := rendering.NewAbsoluteBox(280, 96)
			sheet.Background = &rendering.Color{R: 0.25, G: 0.28, B: 0.20, A: 1}
			sheet.SetRepaintBoundary(true)
			sheet.Place(wrkit.Label("Tooltip 提示层", 13, 0.95, 0.93, 0.75), 14, 12)
			sheet.Place(wrkit.Label(fmt.Sprintf("cycle #%d — 提示内容随开随建", kind+1), 11, 0.8, 0.85, 0.9), 14, 44)
			entries = append(entries, overlay.NewEntry(sheet, 300+float64(kind%5)*60, 480, 280, 96))
		}
		return entries
	}

	phase := wrkit.PhaseSteady
	var lastPhase string
	var elapsed float64
	var hotTick float64
	// Baseline/gate state: main-band dirty count sampled per frame via the
	// engine's band-separated observation (LastOverlayBandFrame). The steady
	// main band legitimately fluctuates (HOT every frame + HUD refresh at
	// ~10Hz + phase chip), so the baseline is the MAX over a steady window —
	// overlay-open frames must stay ≤ that worst case, not one lucky frame.
	baseMainDirty := -1
	var ovFramesMainDirtyExcess int
	var ovOpenObserved bool
	var ovDirtyOnOpen int64
	openedThisRun := false
	// Cycling state (realistic app usage): Spike phase opens for 2s, closes
	// for 1.5s, kind rotates each cycle; fresh entries per open.
	cycleKind := 0
	var cycleEntries []*overlay.Entry
	var cycleT float64
	cycleOpen := false
	const openDur = 2.0
	const closeDur = 1.5
	hotTicks := int64(0)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		hotTick += dt
		if gateMode {
			// Frozen script: phase pinned to Steady, hot converges over 60
			// ticks then holds its final colour and stops marking dirty, and
			// (open state) the modal popup opens once after the 1.2s
			// main-dirty baseline window and stays open. The main-dirty
			// isolation gate therefore still runs against a real open frame,
			// while the end-of-run snapshot is bit-stable.
			phase = wrkit.PhaseSteady
			if n := hotTicks; n <= 60 {
				t := float64(n) / 60
				if t > 1 {
					t = 1
				}
				h := 0.5 * t
				hot.R, hot.G, hot.B = 0.85+0.1*h, 0.25+0.4*(1-h), 0.3+0.5*h
				hot.MarkNeedsPaint()
			}
			hotTicks++
			if gateState == "open" && elapsed > 1.2 && cycleEntries == nil {
				cycleEntries = buildPopup(0)
				for _, e := range cycleEntries {
					ov.Insert(e)
				}
				ovDirtyOnOpen = int64(len(cycleEntries))
				cycleKind = 1
				cycleOpen = true
				openedThisRun = true
			}
		} else {
			switch {
			case elapsed >= 5.0 && elapsed < 10.0:
				phase = wrkit.PhaseSpike
			case elapsed >= 10.0:
				phase = wrkit.PhaseRecover
			default:
				phase = wrkit.PhaseSteady
			}
			// Spike phase: leave it cycling for the WHOLE spike window (long runs
			// extend the spike — set RUN_SECONDS=150 to stress many cycles).
			spikeEnd := 10.0
			if secs >= 60 {
				spikeEnd = float64(secs) * 0.9 // long validation runs cycle ~90% of the time
			}
			switch {
			case elapsed >= 5.0 && elapsed < spikeEnd:
				phase = "SpikeCycle"
			case elapsed >= spikeEnd:
				phase = wrkit.PhaseRecover
			}

			// HOT repaints itself every frame in every phase.
			hue := float64(int(hotTick*8)%16) / 16
			hot.R, hot.G, hot.B = 0.85+0.1*hue, 0.25+0.4*(1-hue), 0.3+0.5*hue
			hot.MarkNeedsPaint()

			if phase != lastPhase {
				lastPhase = phase
				if phase == "SpikeCycle" {
					// Enter cycling mode: first open happens on the next tick
					// (cycleT starts counting inside SpikeCycle below).
					cycleT = 0
				}
				if phase == wrkit.PhaseRecover && cycleOpen {
					for _, e := range cycleEntries {
						ov.Remove(e)
					}
					cycleEntries = nil
					cycleOpen = false
					openedThisRun = false
				}
			}
			// Cycling: 2s open / 1.5s closed, fresh entries + rotating kind each
			// open (real apps build new widgets per popup).
			if phase == "SpikeCycle" {
				cycleT += dt
				if cycleOpen && cycleT >= openDur {
					for _, e := range cycleEntries {
						ov.Remove(e)
					}
					cycleEntries = nil
					cycleOpen = false
					openedThisRun = false
					cycleT = 0
				} else if !cycleOpen && cycleT >= closeDur {
					cycleEntries = buildPopup(cycleKind)
					for _, e := range cycleEntries {
						ov.Insert(e)
					}
					ovDirtyOnOpen = int64(len(cycleEntries))
					cycleKind++
					cycleOpen = true
					openedThisRun = true
					cycleT = 0
				}
			}
		}

		// Per-frame band observation: LastOverlayBandFrame reflects the LAST
		// built frame (one-frame lag — this tick's frame is built after).
		mainDirty, ovd := embedder.LastOverlayBandFrame()
		if elapsed > 1.0 && !openedThisRun {
			// Steady window: track the WORST-CASE main dirty count.
			if mainDirty > baseMainDirty {
				baseMainDirty = mainDirty
			}
		}
		if openedThisRun {
			ovOpenObserved = true
			if baseMainDirty >= 0 && mainDirty > baseMainDirty {
				ovFramesMainDirtyExcess++
			}
		}

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := ovFramesMainDirtyExcess == 0
		shell.UpdateHUD("R8", phase, app, gateOK,
			fmt.Sprintf("ov=%d main_dirty=%d/%d ov_dirty=%d cyc=%d %s", ov.Len(), mainDirty, baseMainDirty, ovd, cycleKind, map[bool]string{true: "OPEN", false: "closed"}[cycleOpen]),
			fmt.Sprintf("excess_frames=%d t=%.1f", ovFramesMainDirtyExcess, elapsed))
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

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	// Pixel probes are anchored to the body panel origin (main band) and to
	// window coordinates (overlay entries sit above the shell). Gate mode
	// never resizes, so these logical coordinates stay fixed.
	bx, by := shell.Body.X, shell.Body.Y
	gridWant := func(i, j int) [3]float64 {
		return [3]float64{0.22 + float64(i)*0.14, 0.38 + float64(j)*0.11, 0.55 + float64((i+j)%3)*0.12}
	}
	// Gate hot converges to hue=0.5: (0.90, 0.45, 0.55).
	hotWant := [3]float64{0.90, 0.45, 0.55}
	// Grid cell (i,j) center in body coords: (70+108i, 54+74j). The modal
	// panel (window 380..720, 260..500) covers body x 96..436 y 200..440, so
	// cell (3,3) is only visible in the closed state; (0,3) stays clear.
	pixelChecks := []pixelCheck{
		{desc: "grid_00", x: bx + 70, y: by + 54, want: gridWant(0, 0), tol: 0.06},
		{desc: "grid_11", x: bx + 178, y: by + 128, want: gridWant(1, 1), tol: 0.06},
		{desc: "grid_20", x: bx + 286, y: by + 54, want: gridWant(2, 0), tol: 0.06},
		{desc: "grid_03", x: bx + 70, y: by + 276, want: gridWant(0, 3), tol: 0.06},
		{desc: "dense_bg", x: bx + 120, y: by + 54, want: [3]float64{0.11, 0.12, 0.18}, tol: 0.05, kind: kindBG},
		{desc: "hot_spot", x: bx + 495, y: by + 365, want: hotWant, tol: 0.06},
		{desc: "body_bg", x: bx + 700, y: by + 500, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	if gateMode && gateState == "open" {
		// The barrier content is a 1x1 box and does not stretch, so the main
		// band reads its raw colours under the modal; the modal panel (with
		// its inner button) and the stacked menu composite above it.
		pixelChecks = append(pixelChecks,
			pixelCheck{desc: "ov_panel", x: 420, y: 310, want: [3]float64{0.15, 0.17, 0.24}, tol: 0.06},
			pixelCheck{desc: "ov_button", x: 440, y: 350, want: [3]float64{0.20, 0.50, 0.85}, tol: 0.06},
			pixelCheck{desc: "ov_menu", x: 760, y: 250, want: [3]float64{0.20, 0.24, 0.32}, tol: 0.06},
		)
	} else {
		pixelChecks = append(pixelChecks,
			pixelCheck{desc: "grid_33", x: bx + 394, y: by + 276, want: gridWant(3, 3), tol: 0.06},
		)
	}
	inkRegions := []inkRegion{
		{desc: "main_label_ink", x: bx + 26, y: by + 340, w: 180, h: 50, bg: [3]float64{0.11, 0.12, 0.18}, tol: 0.04, minPx: 20},
		{desc: "hot_label_ink", x: bx + 480, y: by + 386, w: 40, h: 14, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 6},
	}
	if gateMode && gateState == "open" {
		inkRegions = append(inkRegions,
			inkRegion{desc: "ov_panel_ink", x: 396, y: 274, w: 220, h: 18, bg: [3]float64{0.15, 0.17, 0.24}, tol: 0.04, minPx: 10},
			inkRegion{desc: "ov_menu_ink", x: 712, y: 210, w: 120, h: 60, bg: [3]float64{0.20, 0.24, 0.32}, tol: 0.04, minPx: 10},
		)
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	// Manual mode intentionally cycles popups; its evidence is human obs.
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
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, snapshotPNG, baselinePNG, goldenRects)
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
		"present_policy_retained",
		"cpu_fallback_zero",
		"vsync_source_present",
		"main_dirty_excess_zero",
	}
	if gateMode {
		logicProbes = append(logicProbes, "gate_overlay_state_match")
		if gateState == "open" {
			logicProbes = append(logicProbes, "gate_overlay_open_observed", "gate_baseline_sampled", "gate_overlay_entries_on_open")
		}
	} else {
		logicProbes = append(logicProbes, "manual_overlay_open_observed", "manual_baseline_sampled")
		// The scripted Recover (which closes the popup) only arrives after
		// 10s, so a shorter manual run cannot pass the closed-at-exit probe —
		// that is a duration artifact, not a regression (§2.5 sets 15s).
		if *manualSeconds == 0 || *manualSeconds >= float64(closeSeconds) {
			logicProbes = append(logicProbes, "manual_overlay_closed_at_exit")
		}
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if snap.PresentPolicy != scheduler.PresentPolicyRetained {
		logicFailed = append(logicFailed, "present_policy_retained")
	}
	if snap.CPUFallbackOps != 0 {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if ovFramesMainDirtyExcess != 0 {
		logicFailed = append(logicFailed, "main_dirty_excess_zero")
	}
	if gateMode {
		if gateState == "open" {
			if !(ovOpenObserved && ov.Len() == 3) {
				logicFailed = append(logicFailed, "gate_overlay_state_match")
			}
			if !ovOpenObserved {
				logicFailed = append(logicFailed, "gate_overlay_open_observed")
			}
			if baseMainDirty < 0 {
				logicFailed = append(logicFailed, "gate_baseline_sampled")
			}
			if ovDirtyOnOpen < 1 {
				logicFailed = append(logicFailed, "gate_overlay_entries_on_open")
			}
		} else if ov.Len() != 0 || ovOpenObserved {
			logicFailed = append(logicFailed, "gate_overlay_state_match")
		}
	} else {
		if !ovOpenObserved {
			logicFailed = append(logicFailed, "manual_overlay_open_observed")
		}
		if baseMainDirty < 0 {
			logicFailed = append(logicFailed, "manual_baseline_sampled")
		}
		if openedThisRun && (*manualSeconds == 0 || *manualSeconds >= float64(closeSeconds)) {
			logicFailed = append(logicFailed, "manual_overlay_closed_at_exit")
		}
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R8",
		Scenario:      "ui_wr_r8_overlay",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"gate_state":               gateState,
			"main_dirty_baseline":      baseMainDirty,
			"main_dirty_excess_frames": ovFramesMainDirtyExcess, // must be 0
			"overlay_open_observed":    ovOpenObserved,
			"overlay_entries_on_open":  ovDirtyOnOpen, // ≥1 expected
			"overlay_cycles":           cycleKind,     // open/close cycles completed
			"static_dense_cells":       16,
			"static_labels":            8,
			"hot_active_all_phases":    true,
			"phase_mode":               map[bool]string{true: "frozen-open/closed", false: "cycling"}[gateMode],
			"evidence_mode":            map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":           map[bool]string{true: "gate_snapshot_" + gateState, false: "manual_not_asserted"}[gateMode],
			"fps_gate":                 "persistent_tick_ge_55",
			"slope_gate":               "off_15s_short_run",
			"rss_slope_note":           "rss_slope_valid=false for 15s = rss_warmup_sec; raw slope is startup-inclusive, not a leak conclusion",
			"cpu_fallback_note":        "none observed; last_cpu_fallback empty means no fallback",
			"gpu_ops_semantics":        "cumulative",
			"golden_mask":              "header 1200x48 + legend 260x656 + body 904x656",
			"golden_tolerance_pct":     0,
			"logic_probes_ok":          len(logicProbes) - len(logicFailed),
			"logic_probes_total":       len(logicProbes),
			"logic_bad":                logicFailed,
			"pixel_probes_ok":          pixelOK,
			"pixel_probes_total":       pixelTotal,
			"pixel_bad":                pixelBad,
			"pixel_golden_diff_pct":    pixelGoldenValue,
			"pixel_golden_total_px":    goldenTotalPx,
			"pixel_golden_first_run":   goldenFirstRun,
			"snapshot_png":             snapshotPath,
			"snapshot_note":            map[bool]string{true: "gate snapshot (GPU readback) used for pixel/Golden evidence", false: "manual mode does not write the gate snapshot; committed evidence is not overwritten"}[gateMode],
			"impl_correctness":         "overlay band composites above main (FramePacket.Root then .Overlay); insert/remove touch only overlay entries",
			"impl_dirty":               "AttachToPacket mirrors overlay dirties into pkt.OverlayDirtyLayerIDs; main DirtyLayerIDs unchanged by open (D5); band counts sampled pre/post attach",
			"impl_cache":               "retained policy: main boundaries replay while overlay open; overlay panel/menu are their own RepaintBoundaries",
			"impl_edge":                "cycling opens build FRESH entries per cycle (real-app pattern: new widgets per popup), 3 kinds rotate (modal+menu stack / dropdown / tooltip sheet), sizes+positions vary per cycle",
			"impl_fail":                "main_dirty > baseline during open frames → FAIL (leak into main band); no overlay observed → FAIL; fps decay over many cycles → RSS/fps gates catch it",
			"impl_visible":             "HUD ov=N cyc=K OPEN/closed main_dirty=X/base; gate open state shows modal+menu over dimmed main; manual popups alternate dialog/menu/tooltip",
		},
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
		// R8 gates (§2 主表: 开浮层后主树 paint_count 不涨).
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:           1,
			RequireRetainedPolicy: true, // Plan C posture
			RequirePersistentFPS:  true, // continuous ticker (animation-class)
			MinFPSWall:            55,
			MaxP95Ms:              22,
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
		"ui_wr_r8_overlay: %s state=%s main_dirty_base=%d excess=%d ov_entries=%d presents=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, gateState, baseMainDirty, ovFramesMainDirtyExcess, ovDirtyOnOpen, presents,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
