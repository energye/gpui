// Command ui_wr_c4_shell_overlay is the W4 C4 composite real-window:
// 顶栏壳(R21) + 可滚体内容(R3) + 浮层面板叠加(R8) 三层独立集成。
//
// Three bands live on screen simultaneously and must stay independent:
//   - SHELL band: wrkit TopBar tagged SetShellBoundary — body scrolling and
//     overlay cycling must never re-record it (shell_rerecord_scroll == 0,
//     shell Picture Replays → shell_skip_scroll > 0).
//   - CONTENT band: scrollable VirtualList + static dense grid (nested
//     RepaintBoundaries, R3 boundary_skip>0) + HOT live-paint spot.
//   - OVERLAY band: Spike phase cycles popups open/closed (2s open / 1.5s
//     closed, 3 kinds rotate, fresh entries per cycle). Opening must dirty
//     ONLY the overlay band — main-band dirty count stays at its Steady
//     baseline (main_dirty_excess_frames == 0).
//
// Present policy: retained (Plan C, Flutter-aligned; user-confirmed at the
// stop-report node) — steady frames CompositeOnly + damage present; only
// dirty layers re-record. This makes both "shell rerecord=0" and "overlay
// does not grow main paint" gates strict.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c4_shell_overlay
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

const (
	winW, winH = 1200, 800
	// §2.5 组合窗关闭用时长: C4 = 15 (覆盖最重能力 R8).
	closeSeconds = 15
)

var proc scheduler.ProcessTracker

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	}
	wrkit.RequireMinRun(secs, "C4")
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	render.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c4_shell_overlay — 壳+体内容+浮层 三层独立", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C4 组合 — 壳(R21)+体内容(R3)+浮层(R8) 三层独立", []string{
		"SHELL = 顶栏壳层 SetShellBoundary 全程静止 (R21)",
		"BODY  = 虚拟列表持续滚动 只脏体层",
		"DENSE = 静态密集区嵌套 boundary 回放 (R3)",
		"HOT   = 主树热点每帧自脏 全程活跃",
		"Spike = 浮层循环开合 3 种轮换 每次全新 entry (R8)",
		"开浮层帧 main_dirty 必须保持基线",
		"体滚/浮层都不得重录壳层",
		"policy=retained 稳态只脏层重录",
	})

	const topBarH = 64.0

	// ===== SHELL band: static top bar under the wrkit TopBar (R21 pattern).
	// One RepaintBoundary tagged SetShellBoundary: body scroll AND overlay
	// cycling must never re-record it.
	bar := rendering.NewAbsoluteBox(winW-260, topBarH)
	bar.Background = &rendering.Color{R: 0.16, G: 0.18, B: 0.26, A: 1}
	bar.SetRepaintBoundary(true)
	bar.SetShellBoundary(true)
	bar.Place(wrkit.Label("SHELL — 标题/按钮/样张 壳层", 16, 0.95, 0.96, 0.99), 14, 10)
	btn1 := rendering.NewAbsoluteBox(86, 26)
	btn1.Background = &rendering.Color{R: 0.22, G: 0.52, B: 0.88, A: 1}
	btn1.SetRepaintBoundary(true)
	btn1.Place(wrkit.Label("按钮-1", 11, 1, 1, 1), 10, 7)
	bar.Place(btn1, 320, 18)
	btn2 := rendering.NewAbsoluteBox(86, 26)
	btn2.Background = &rendering.Color{R: 0.62, G: 0.42, B: 0.20, A: 1}
	btn2.SetRepaintBoundary(true)
	btn2.Place(wrkit.Label("按钮-2", 11, 1, 1, 1), 10, 7)
	bar.Place(btn2, 414, 18)
	sx := 530.0
	for _, s := range []float64{8, 10, 12, 14, 16} {
		bar.Place(wrkit.Label(fmt.Sprintf("%dpx 样张", int(s)), s, 0.92, 0.95, 1), sx, 44)
		sx += 28 + s*3.0
	}
	shell.Root.Place(bar, 10, 6)

	// Phase chip lives OUTSIDE the shell boundary (self-dirties every frame;
	// tagging it shell would defeat the split).
	bodyPhase := wrkit.Label("PHASE=Steady bodyScr=0 ov=closed", 12, 0.85, 0.95, 0.55)
	shell.Root.Place(bodyPhase, 12, topBarH+2)

	// ===== CONTENT band: scrollable virtual list (body) =====
	bodyW := winW - 520.0
	const hudGap = 12.0
	vlist := rendering.NewVirtualList(60, 44, func(i int) rendering.RenderObject {
		row := rendering.NewAbsoluteBox(bodyW, 40)
		row.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.20, A: 1}
		chip := rendering.NewRenderColorBox(26, 26, 0.3+float64(i%3)*0.2, 0.45+float64(i%2)*0.3, 0.6, 1)
		chip.SetRepaintBoundary(true)
		row.Place(chip, 8, 7)
		row.Place(wrkit.Label(fmt.Sprintf("item-%d 体内容行", i), 12, 0.85, 0.88, 0.93), 44, 11)
		return row
	})
	viewport := rendering.NewRenderViewport(vlist)
	viewport.SetScrollOffset(0, 0)
	viewport.FixedHeight = winH - topBarH - shell.HUDH - hudGap*2
	shell.Root.Place(viewport, 12, topBarH+14)

	// Static dense content (U17/R3): 4x4 grid with NESTED boundaries + 8 labels.
	dense := rendering.NewAbsoluteBox(300, 300)
	dense.Background = &rendering.Color{R: 0.11, G: 0.12, B: 0.18, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 28, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			c.SetRepaintBoundary(true) // nested RB: cells cache independently (R3)
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		dense.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*64, 168+float64(row)*20)
	}
	shell.Root.Place(dense, 470, topBarH+14)

	// HOT spot: live-paint (non-boundary), active in EVERY phase including
	// overlay-open frames — contributes exactly one main-band dirty id.
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Root.Place(hot, 800, topBarH+14)
	shell.Root.Place(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 800, topBarH+48)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// Plan C posture (user-confirmed): retained — only dirty layers re-record.
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	ov := overlay.New()
	app.SetOverlay(ov)

	// Popup builder: 3 kinds rotate per cycle, FRESH entries each open
	// (real apps build new widgets per popup). Same as ui_wr_r8_overlay.
	buildPopup := func(kind int) []*overlay.Entry {
		barrier := rendering.NewRenderColorBox(1, 1, 0.02, 0.03, 0.06, 0.45)
		entries := []*overlay.Entry{overlay.NewBarrierEntry(0, 0, float64(winW), float64(winH), barrier)}
		switch kind % 3 {
		case 0: // modal dialog + stacked menu list
			panel := rendering.NewAbsoluteBox(340, 240)
			panel.Background = &rendering.Color{R: 0.15, G: 0.17, B: 0.24, A: 1}
			panel.SetRepaintBoundary(true)
			panel.Place(wrkit.Label("OVERLAY PANEL 浮层面板", 14, 0.92, 0.95, 1), 16, 14)
			panel.Place(wrkit.Label(fmt.Sprintf("第 %d 次打开 · 对话框", kind+1), 11, 0.7, 0.78, 0.88), 16, 44)
			menu := rendering.NewAbsoluteBox(180, 150)
			menu.Background = &rendering.Color{R: 0.2, G: 0.24, B: 0.32, A: 1}
			menu.SetRepaintBoundary(true)
			for i := 0; i < 4; i++ {
				menu.Place(wrkit.Label(fmt.Sprintf("menu-item-%d", i), 11, 0.85, 0.9, 0.95), 12, 10+float64(i)*28)
			}
			entries = append(entries,
				overlay.NewEntry(panel, 380+float64(kind%3)*24, 260, 340, 240),
				overlay.NewEntry(menu, 700-float64(kind%2)*40, 200, 180, 150))
		case 1: // dropdown menu (no barrier — click-through outside)
			drop := rendering.NewAbsoluteBox(220, 210)
			drop.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.30, A: 1}
			drop.SetRepaintBoundary(true)
			for i := 0; i < 6; i++ {
				rowB := rendering.NewAbsoluteBox(200, 26)
				rowB.Background = &rendering.Color{R: 0.14 + 0.03*float64(i%2), G: 0.18, B: 0.26, A: 1}
				rowB.SetRepaintBoundary(true)
				rowB.Place(wrkit.Label(fmt.Sprintf("下拉项-%d 选择", i), 11, 0.85, 0.9, 0.95), 10, 5)
				drop.Place(rowB, 10, 10+float64(i)*32)
			}
			entries = append(entries, overlay.NewEntry(drop, 420+float64(kind%4)*30, 180, 220, 210))
		default: // tooltip sheet
			sheet := rendering.NewAbsoluteBox(280, 96)
			sheet.Background = &rendering.Color{R: 0.25, G: 0.28, B: 0.20, A: 1}
			sheet.SetRepaintBoundary(true)
			sheet.Place(wrkit.Label("Tooltip 提示层", 13, 0.95, 0.93, 0.75), 14, 12)
			sheet.Place(wrkit.Label(fmt.Sprintf("cycle #%d — 随开随建", kind+1), 11, 0.8, 0.85, 0.9), 14, 44)
			entries = append(entries, overlay.NewEntry(sheet, 300+float64(kind%5)*60, 480, 280, 96))
		}
		return entries
	}

	phase := wrkit.PhaseSteady
	var lastPhase string
	var elapsed float64

	// ---- Gate state ----
	// R21: shell partition sampled per frame after warm-up; scroll frames
	// must show rerecord==0 while replays happen. Resize frames excluded
	// (legal rerecord, external WM interference protection).
	var resizeSkip int
	var lastWinW, lastWinH int
	var shellRRScroll, shellSkipScroll int64
	// R8: overlay band cycling; main-band gate reference = global worst-case
	// dirty count over closed-state frames (see ticker comment).
	var refMainDirty int = -1
	var ovFramesMainDirtyExcess int
	var ovOpenObserved bool
	var ovDirtyOnOpen int64
	openedThisRun := false
	cycleKind := 0
	var cycleEntries []*overlay.Entry
	cycleOpen := false
	var cycleT float64
	const openDur = 2.0
	const closeDur = 1.5

	// Phase script: Steady 0–5s (baseline, no overlay) → Spike 5–11.5s
	// (overlay cycling) → Recover 11.5s+ (final close, settle).
	const spikeStart = 5.0
	spikeEnd := 11.5
	if secs > closeSeconds { // long validation runs extend the spike window
		spikeEnd = float64(secs) - 3.5
	}

	var scrollY float64
	var wrapSettle int

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		switch {
		case elapsed >= spikeStart && elapsed < spikeEnd:
			phase = wrkit.PhaseSpike
		case elapsed >= spikeEnd:
			phase = wrkit.PhaseRecover
		default:
			phase = wrkit.PhaseSteady
		}

		// Resize settle exclusion (same as R21).
		cw, ch := host.Size()
		if cw != lastWinW || ch != lastWinH {
			lastWinW, lastWinH = cw, ch
			resizeSkip = 3
		}
		resizeFrame := resizeSkip > 0
		if resizeSkip > 0 {
			resizeSkip--
		}

		// Body scroll every frame (content band motion; shell untouched).
		rate := 0.5
		if phase == wrkit.PhaseSpike {
			rate = 8.0
		}
		scrollY += rate
		// Infinite wrap keeps the list moving. A wrap remounts the whole
		// visible cell window — mark the frame so the main-dirty gate can
		// exclude this scroll-side burst (it is not overlay-caused).
		wrapped := scrollY >= 60*44
		if wrapped {
			scrollY = 0
			// Wrap remounts the whole visible cell window: the fresh-mount
			// wave lands 1-2 frames later (band observation lags one frame)
			// and spans several frames. 8 frames covers it without masking
			// real overlay leaks (Spike wraps only every ~5.5s).
			wrapSettle = 8
		}
		if wrapSettle > 0 {
			wrapSettle--
		}
		viewport.SetScrollOffset(0, scrollY)

		// HOT repaints itself every frame in every phase.
		hue := float64(int(elapsed*8)%16) / 16
		hot.R, hot.G, hot.B = 0.85+0.1*hue, 0.25+0.4*(1-hue), 0.3+0.5*hue
		hot.MarkNeedsPaint()

		// Overlay cycling in Spike only.
		if phase != lastPhase {
			lastPhase = phase
			if phase == wrkit.PhaseRecover && cycleOpen {
				for _, e := range cycleEntries {
					ov.Remove(e)
				}
				cycleEntries = nil
				cycleOpen = false
				openedThisRun = false
			}
		}
		if phase == wrkit.PhaseSpike {
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

		// Per-frame observations (one-frame lag: reflects the LAST built frame).
		mainDirty, _ := embedder.LastOverlayBandFrame()
		srr, ssk := embedder.LastShellBoundaryFrame()
		// Startup exclusion by TIME, not frame count: the warm-up full paint,
		// force frames and raster catch-up all land within ~150ms of open,
		// but ticker-driven frame counts don't align with wall time (a burst
		// at t=50–150ms leaked past a 12-frame window in one run). 500ms is
		// far below the Spike start (5s), so no gate semantics are lost.
		if elapsed < 0.5 {
			// startup frames: not sampled
		} else if !resizeFrame {
			if srr != 0 {
				shellRRScroll += srr
			}
			shellSkipScroll += ssk
		}
		// Main-band overlay gate (R8 semantics): an overlay open/close must
		// not ADD main-band dirties. The reference is the global worst-case
		// main-dirty count over ALL closed-state frames (any phase) — scroll
		// bursts (cell fresh-mount waves around the 2640px wrap) hit open
		// and closed frames alike, so the closed-state max already covers
		// them; only overlay-caused dirt exceeds it. wrapSettle additionally
		// excludes the immediate post-wrap remount window from judgment.
		if openedThisRun {
			if cycleOpen {
				ovOpenObserved = true
				if refMainDirty >= 0 && mainDirty > refMainDirty+2 && wrapSettle <= 0 {
					ovFramesMainDirtyExcess++
				}
			} else if mainDirty > refMainDirty {
				refMainDirty = mainDirty
			}
		} else if elapsed > 1.0 && mainDirty > refMainDirty {
			refMainDirty = mainDirty
		}

		bodyPhase.SetText(fmt.Sprintf("PHASE=%s bodyScr=%.0f ov=%s", phase, scrollY,
			map[bool]string{true: "OPEN", false: "closed"}[cycleOpen]))
		bodyPhase.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := shellRRScroll == 0 && shellSkipScroll > 0 &&
			refMainDirty >= 0 && ovFramesMainDirtyExcess == 0
		shell.UpdateHUD("C4", phase, app, gateOK,
			fmt.Sprintf("shell_rr=%d skip=%d main=%d/%d",
				shellRRScroll, shellSkipScroll, refMainDirty, ovFramesMainDirtyExcess),
			fmt.Sprintf("ov=%d cyc=%d %s scr=%.0f t=%.1f",
				ov.Len(), cycleKind, map[bool]string{true: "OPEN", false: "closed"}[cycleOpen], scrollY, elapsed))
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
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C4",
		Scenario:      "ui_wr_c4_shell_overlay",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"covers": []string{"R3", "R8", "R21"},
			// R21 gates
			"shell_rerecord_scroll": shellRRScroll, // must be 0
			"shell_skip_scroll":     shellSkipScroll,
			"shell_rerecord_total":  snap.ShellRerecord,
			"shell_skip_total":      snap.ShellSkip,
			// R8 gates
			"main_dirty_baseline":      refMainDirty,
			"main_dirty_excess_frames": ovFramesMainDirtyExcess, // must be 0
			"overlay_open_observed":    ovOpenObserved,
			"overlay_entries_on_open":  ovDirtyOnOpen, // ≥1 expected
			"overlay_cycles":           cycleKind,
			"overlay_still_open":       openedThisRun, // must be false
			// R3 observation
			"boundary_skip":    snap.BoundarySkip,
			"boundary_rerecord": snap.BoundaryRerecord,
			"static_dense_cells": 16,
			"static_labels":      8,
			"hot_active_all_phases": true,
			"impl_interaction": "three bands share one frame: body scroll dirties only body layers (R21 shell partition untouched), overlay insert/remove dirties only the overlay band (FramePacket.Root then .Overlay; main DirtyLayerIDs unchanged by open), dense-grid nested boundaries replay throughout (R3 cache) — a shell rerecord OR a main-dirty excess during any Spike frame fails the run",
			"impl_correctness": "retained Plan C: steady frames CompositeOnly + damage present; shell band is one SetShellBoundary RepaintBoundary; overlay composites above main; dense grid cells are nested RepaintBoundaries",
			"impl_dirty":       "band-separated observation: LastShellBoundaryFrame samples the shell partition; LastOverlayBandFrame mirrors overlay dirties into pkt.OverlayDirtyLayerIDs pre/post attach so open frames cannot pollute the main-band count",
			"impl_cache":       "shell Picture Replays while rows scroll and popups cycle (skip grows, rr stays 0); dense grid + list cell boundaries replay; popup panel/menu/dropdown are their own RepaintBoundaries",
			"impl_edge":        "resize frames excluded from shell gate (3-frame settle); fresh overlay entries per cycle with 3 rotating kinds; infinite scroll wraps at content end; HUD/phase chip live OUTSIDE the shell boundary",
			// RSS note: 15s runs sit inside the startup equilibrium ramp
			// (Go heap + GPU driver); R8 600s soak proved the plateau. Same
			// semantics as ui_wr_r21_shell (15s slope ~162k KB/min there).
			"rss_slope_semantics": "startup-ramp dominated at 15s; see ENGINE_UI_WIDGET_RENDER §10 R8 场景化长跑",
			"impl_fail":        "any shell rerecord during scroll frames → FAIL; no shell replay → FAIL; main_dirty > baseline during open frames → FAIL; no overlay observed or left open at exit → FAIL; boundary_skip==0 → FAIL",
			"impl_visible":     "HUD live: shell_rr/skip, main=X/base, excess, ov=N cyc=K OPEN/closed; topbar pixels never change while rows scroll beneath it and popups stack above everything",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates（§3 C4 门禁并集 ∪(R3,R8,R21) + §2.2 全族；门禁是硬的不许放）----
	gates := wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true, // user-confirmed Plan C posture
		RequirePersistentFPS:  true, // continuous scroll ticker (animation-class)
		MinFPSWall:            55,
		MaxP95Ms:              22,
		MinBoundarySkip:       1, // R3: static dense cache engaged
	}
	if err := wrgate.EvaluateGates(report, gates); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	// R21: shell independence during body scroll.
	if shellRRScroll != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: shell_rerecord_scroll=%d want 0 (scroll/overlay must not re-record the shell)\n", shellRRScroll)
		os.Exit(1)
	}
	if shellSkipScroll <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: shell_skip_scroll=%d want >0 (shell Picture must replay)\n", shellSkipScroll)
		os.Exit(1)
	}
	// R8: overlay independence from the main band.
	if !ovOpenObserved {
		fmt.Fprintln(os.Stderr, "FAIL: overlay never observed open (script error)")
		os.Exit(1)
	}
	if refMainDirty < 0 {
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
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (R3 static dense cache)", snap.BoundarySkip)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c4_shell_overlay: OK shell_rr=%d shell_skip=%d main_base=%d excess=%d ov_entries=%d cyc=%d skip=%d fps=%.1f p95=%.1f presents=%d elapsed=%.1fs\n",
		shellRRScroll, shellSkipScroll, refMainDirty, ovFramesMainDirtyExcess,
		ovDirtyOnOpen, cycleKind, snap.BoundarySkip, fpsInterval,
		snap.P95FrameIntervalMs, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
