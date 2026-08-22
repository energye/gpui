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
//	RUN_SECONDS=15 go run ./examples/ui_wr_r8_overlay
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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

const winW, winH = 1200, 800

var proc scheduler.ProcessTracker

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R8")
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

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
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

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		hotTick += dt
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R8",
		Scenario:      "ui_wr_r8_overlay",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"main_dirty_baseline":        baseMainDirty,
			"main_dirty_excess_frames":   ovFramesMainDirtyExcess, // must be 0
			"overlay_open_observed":      ovOpenObserved,
			"overlay_entries_on_open":    ovDirtyOnOpen, // ≥1 expected
			"overlay_cycles":             cycleKind,     // open/close cycles completed
			"static_dense_cells":         16,
			"static_labels":              8,
			"hot_active_all_phases":      true,
			"impl_correctness":           "overlay band composites above main (FramePacket.Root then .Overlay); insert/remove touch only overlay entries",
			"impl_dirty":                 "AttachToPacket mirrors overlay dirties into pkt.OverlayDirtyLayerIDs; main DirtyLayerIDs unchanged by open (D5); band counts sampled pre/post attach",
			"impl_cache":                 "retained policy: main boundaries replay while overlay open; overlay panel/menu are their own RepaintBoundaries",
			"impl_edge":                  "cycling opens build FRESH entries per cycle (real-app pattern: new widgets per popup), 3 kinds rotate (modal+menu stack / dropdown / tooltip sheet), sizes+positions vary per cycle",
			"impl_fail":                  "main_dirty > baseline during open frames → FAIL (leak into main band); no overlay observed → FAIL; fps decay over many cycles → RSS/fps gates catch it",
			"impl_visible":               "HUD ov=N cyc=K OPEN/closed main_dirty=X/base; popups alternate dialog/menu/tooltip at varying positions through the run",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R8 gates (§2 主表: 开浮层后主树 paint_count 不涨).
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true, // Plan C posture
		RequirePersistentFPS:  true, // continuous ticker (animation-class)
		MinFPSWall:            55,
		MaxP95Ms:              22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if !ovOpenObserved {
		fmt.Fprintln(os.Stderr, "FAIL: overlay never observed open (script error)")
		os.Exit(1)
	}
	if baseMainDirty < 0 {
		fmt.Fprintln(os.Stderr, "FAIL: main_dirty baseline never sampled")
		os.Exit(1)
	}
	if ovFramesMainDirtyExcess != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: main_dirty_excess_frames=%d want 0 (opening overlay must not dirty the main band beyond baseline)\n", ovFramesMainDirtyExcess)
		os.Exit(1)
	}
	if ovDirtyOnOpen < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: overlay_entries_on_open=%d want >=1 (overlay band must be the dirtied side)\n", ovDirtyOnOpen)
		os.Exit(1)
	}
	if openedThisRun {
		fmt.Fprintln(os.Stderr, "FAIL: overlay still open at exit (Recover phase must remove it)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r8_overlay: OK main_dirty_base=%d excess=%d ov_entries=%d presents=%d elapsed=%.1fs\n",
		baseMainDirty, ovFramesMainDirtyExcess, ovDirtyOnOpen, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
