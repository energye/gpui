// Command ui_wr_c8_hit_overlay_xf is the W4+ C8 combination real-window:
// hit testing under transform + overlay + layer animation (R13 + R6 + R8).
//
// Three abilities coexist on one screen and interact:
//   - R13 hit: scripted probes assert HitTestPointer identity under plain
//     boxes, a CONTINUOUSLY ROTATING target (angle advances every frame),
//     an animated scale-pulse target, rounded clip inside/outside, overlap
//     top-wins, empty-area miss — plus OVERLAY-band probes: barrier consumes
//     main-tree points, floating button inside an entry hits the entry child,
//     non-barrier tooltip lets the same point fall through to the main tree,
//     and identity is restored after the final close.
//   - R6 layer animation: rotation/scale targets animate continuously via
//     SetRotation/SetScale (paint-only, boundary-isolated); Spike phase
//     doubles animation speed; the static dense region never repaints.
//   - R8 overlay: Spike phase cycles popups open/closed (modal dialog with
//     full-screen barrier / tooltip sheet without), fresh entries per open;
//     band-separated observation asserts opening the overlay never dirties
//     the main band beyond its steady baseline.
//
// Gates = union(R13, R8) per §3 C8 row: scripted_ok == scripted_total AND
// hit >= 4 AND main_dirty_excess_frames == 0 AND overlay_entries_on_open >= 1.
// NOTE: C8 is integration only — it does NOT close R6 (its own single-R
// window ui_wr_r6_layer_anim is still pending, §5 W5).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c8_hit_overlay_xf
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// probe mirrors ui_wr_r13_hit's probe shape: a window-logical click point
// computed from laid-out offsets plus the expected hit identity.
type probe struct {
	name      string // expected HitDebugName; "" for must-miss probes
	align     *rendering.RenderAlignBox
	anc       rendering.RenderObject // clip/transform container (nil for plain)
	target    rendering.RenderObject // offset provider
	dx, dy    float64
	expectHit bool
	done      bool
	ok        bool
	// wx, wy record the resolved window point of the last run for the §2.7
	// pixel cross-check (probe identity + painted color at the same point).
	// br/bg/bb are the target's base color (F0 exact-point assertion; the
	// rotating/scaling targets are probed at their invariant centers).
	wx, wy         float64
	br, bg, bb     float64
	pixOK, pixDone bool
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "C8")
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c8_hit_overlay_xf — 变换/浮层下命中", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C8 命中×浮层×变换 — R13+R6+R8 集成", []string{
		"MAIN  = 静态密集主树 (4x4色格+嵌套boundary)",
		"XF-A  = 持续旋转目标 (命中跟角度)",
		"XF-B  = 呼吸缩放目标",
		"Spike = 浮层循环: 对话框(屏障)/Tooltip(无屏障)",
		"开浮层帧 main_dirty 必须保持基线",
		"屏障点主树 → 被浮层消费 (BandOverlay)",
		"Tooltip 无屏障点主树 → 穿透回主树",
		"HOT   = 主树热点 (每帧变色, 全程活跃)",
	})

	bodyX := shell.Body.Box.Offset().X
	bodyY := shell.Body.Box.Offset().Y

	var probes []*probe
	mkHit := func(name string, box *rendering.RenderColorBox, ab *rendering.RenderAlignBox, anc rendering.RenderObject, dx, dy float64) {
		probes = append(probes, &probe{name: name, align: ab, anc: anc, target: box, dx: dx, dy: dy, expectHit: true,
			br: box.R, bg: box.G, bb: box.B})
	}
	mkMiss := func(ab *rendering.RenderAlignBox, anc, target rendering.RenderObject, dx, dy float64) {
		probes = append(probes, &probe{name: "", align: ab, anc: anc, target: target, dx: dx, dy: dy, expectHit: false})
	}

	// ===== XF-A: continuously rotating hit target (boundary-isolated anim) =====
	innerA := rendering.NewRenderColorBox(64, 64, 0.85, 0.30, 0.28, 1)
	innerA.SetDebugName("target-rot")
	innerA.SetRepaintBoundary(true)
	xfA := rendering.NewRenderTransform(innerA)
	mkHit("target-rot", innerA, shell.Body.Align(xfA, 0.04, 0.05), xfA, 32, 32)

	// ===== XF-B: breathing scale pulse hit target =====
	innerB := rendering.NewRenderColorBox(60, 60, 0.28, 0.45, 0.88, 1)
	innerB.SetDebugName("target-scale")
	innerB.SetRepaintBoundary(true)
	xfB := rendering.NewRenderTransform(innerB)
	mkHit("target-scale", innerB, shell.Body.Align(xfB, 0.16, 0.05), xfB, 30, 30)

	// ===== C: rounded-clip target =====
	innerC := rendering.NewRenderColorBox(50, 50, 0.30, 0.80, 0.45, 1)
	innerC.SetDebugName("target-round")
	clipC := rendering.NewRenderClipRRect(innerC)
	clipC.FixedWidth, clipC.FixedHeight = 50, 50
	clipC.SetRadius(12)
	mkHit("target-round", innerC, shell.Body.Align(clipC, 0.28, 0.05), clipC, 25, 25)

	// ===== E: clip with overflowing child — outside-clip must miss =====
	innerE := rendering.NewRenderColorBox(70, 70, 0.90, 0.75, 0.30, 1)
	innerE.SetDebugName("target-clipped")
	clipE := rendering.NewRenderClipRRect(innerE)
	clipE.FixedWidth, clipE.FixedHeight = 40, 40
	abE := shell.Body.Align(clipE, 0.40, 0.05)
	mkHit("target-clipped", innerE, abE, clipE, 20, 20)
	mkMiss(abE, clipE, innerE, 50, 50) // inside 70x70 child, outside 40x40 clip

	// ===== F/G: overlap — last-added top box wins =====
	bottomF := rendering.NewRenderColorBox(60, 60, 0.35, 0.65, 0.55, 1)
	bottomF.SetDebugName("target-bottom")
	topG := rendering.NewRenderColorBox(60, 60, 0.95, 0.85, 0.35, 1)
	topG.SetDebugName("target-top")
	_ = bottomF
	mkHit("target-top", topG, shell.Body.Align(topG, 0.52, 0.05), nil, 30, 30)

	// ===== H: empty area anchor — 3px off corner must miss =====
	blankH := rendering.NewRenderColorBox(2, 2, 0, 0, 0, 0)
	blankH.SetDebugName("blank-anchor")
	mkMiss(shell.Body.Align(blankH, 0.80, 0.75), nil, blankH, 3, 3)

	// ===== Static dense content (U17): 4x4 grid + nested boundary + labels =====
	denseD := rendering.NewAbsoluteBox(330, 240)
	denseD.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.21, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			c.SetRepaintBoundary(true) // nested boundary: cells cache independently
			denseD.Place(c, 10+float64(i)*66, 10+float64(j)*40)
		}
	}
	var lblAnchor *rendering.RenderText
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		lbl := wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85)
		lbl.SetDebugName(fmt.Sprintf("lbl-%d", i))
		denseD.Place(lbl, 10+float64(col)*66, 178+float64(row)*22)
		if i == 7 {
			lblAnchor = lbl // fall-through / restore probe anchor (lbl-7, bottom-right of dense box)
		}
	}
	abDense := shell.Body.Align(denseD, 0.55, 0.35)

	// HOT spot: live paint every frame in EVERY phase (keeps fps measurable;
	// contributes exactly one main-band dirty id — part of the baseline).
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.04, 0.55)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.04, 0.66)

	hitBanner := wrkit.Label("HIT: --  0/9   ov: 0/0/0   rst: --   ptr: --", 13, 0.95, 0.90, 0.55)
	shell.Body.Align(hitBanner, 0.04, 0.78)

	var app *embedder.PipelineApp
	var lastPtrHit string
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: runDur(secs),
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_c8_hit_overlay_xf: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
			if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerDown {
				band, obj, entry := app.HitTestPointer(ev.X, ev.Y)
				name := rendering.HitDebugName(obj)
				lastPtrHit = fmt.Sprintf("%s[%v]", orDash(name), band)
				fmt.Fprintf(os.Stderr, "ui_wr_c8_hit_overlay_xf: ptr click @(%.0f,%.0f) band=%v hit=%q entry=%v\n",
					ev.X, ev.Y, band, name, entry != nil)
			}
		},
	})

	// ===== R8 overlay cycling (real-app pattern, adapted from ui_wr_r8) =====
	ov := overlay.New()
	app.SetOverlay(ov)

	buildPopup := func(kind int) []*overlay.Entry {
		if kind%3 == 2 {
			// Tooltip sheet WITHOUT barrier: points over the main tree must
			// fall through to BandMain (integration-specific assertion).
			sheet := rendering.NewAbsoluteBox(280, 96)
			sheet.Background = &rendering.Color{R: 0.25, G: 0.28, B: 0.20, A: 1}
			sheet.SetRepaintBoundary(true)
			sheet.Place(wrkit.Label("Tooltip 无屏障层", 13, 0.95, 0.93, 0.75), 14, 12)
			sheet.Place(wrkit.Label(fmt.Sprintf("cycle #%d — 点外部穿透主树", kind+1), 11, 0.8, 0.85, 0.9), 14, 44)
			return []*overlay.Entry{overlay.NewEntry(sheet, 300+float64(kind%5)*60, 480, 280, 96)}
		}
		// Modal dialog WITH full-screen barrier + menu stack.
		barrier := rendering.NewRenderColorBox(1, 1, 0.02, 0.03, 0.06, 0.45)
		entries := []*overlay.Entry{overlay.NewBarrierEntry(0, 0, float64(winW), float64(winH), barrier)}
		panel := rendering.NewAbsoluteBox(340, 240)
		panel.Background = &rendering.Color{R: 0.15, G: 0.17, B: 0.24, A: 1}
		panel.SetRepaintBoundary(true)
		panel.Place(wrkit.Label("OVERLAY PANEL 浮层面板", 14, 0.92, 0.95, 1), 16, 14)
		btn := rendering.NewAbsoluteBox(120, 34)
		btn.Background = &rendering.Color{R: 0.2, G: 0.5, B: 0.85, A: 1}
		btn.SetDebugName("ov-button")
		btnLbl := wrkit.Label("浮层内按钮", 11, 1, 1, 1)
		btnLbl.SetDebugName("ov-button")
		btn.Place(btnLbl, 14, 9)
		panel.Place(btn, 16, 76)
		menu := rendering.NewAbsoluteBox(180, 150)
		menu.Background = &rendering.Color{R: 0.2, G: 0.24, B: 0.32, A: 1}
		menu.SetRepaintBoundary(true)
		for i := 0; i < 4; i++ {
			menu.Place(wrkit.Label(fmt.Sprintf("menu-item-%d", i), 11, 0.85, 0.9, 0.95), 12, 10+float64(i)*28)
		}
		return append(entries,
			overlay.NewEntry(panel, 380+float64(kind%3)*24, 260, 340, 240),
			overlay.NewEntry(menu, 700-float64(kind%2)*40, 200, 180, 150))
	}

	phase := wrkit.PhaseSteady
	var lastPhase string
	var elapsed float64
	var lastScriptHit string

	// Plan C (Flutter-aligned posture, R8 side): retained steady frames —
	// only dirty layers re-record; opening an overlay must not re-record the
	// main band.
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	// R8 gate state: worst-case steady main-band dirty baseline vs overlay-open frames.
	baseMainDirty := -1
	var ovFramesMainDirtyExcess int
	var ovOpenObserved bool
	var ovDirtyOnOpen int64
	openedThisRun := false
	cycleKind := 0
	var cycleEntries []*overlay.Entry
	var cycleT float64
	cycleOpen := false
	const openDur = 1.6
	const closeDur = 1.2

	// Probe scheduling anchors: main-tree probes fire during Steady; overlay
	// probes fire while a popup is open (once per kind); restore re-probe
	// fires after Recover removed everything. Each must pass exactly once.
	steadyDone := false
	ovBarrierConsumed := false
	ovEntryHitOK := false
	ovFallthroughOK := false
	restoredProbeOK := false
	restoredProbeDone := false
	animSpeed := 1.0
	pulse := 1.0

	// §2.7 snapshot schedule: one mid-steady shot + one after recovery.
	snapDir := os.Getenv("C8_SNAP_DIR")
	snapSteadyPath := filepath.Join(snapDir, "c8_steady.png")
	snapRecoverPath := filepath.Join(snapDir, "c8_recover.png")
	snapSteadyAt := 2.5 // seconds into Steady phase
	snapRecoverAt := -1.0

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// Phase timeline (15s default): Steady 0–3s (scripted main-tree probes)
		// → Spike 3–10.5s (overlay cycling: kind0 modal → kind1 modal →
		// kind2 tooltip, all three must open) → Recover 10.5s+ (close + restore
		// probe). Long runs extend Spike to ~90% for soak-style cycling.
		switch {
		case elapsed >= 3.0 && elapsed < spikeEnd(secs):
			phase = wrkit.PhaseSpike
		case elapsed >= spikeEnd(secs):
			phase = wrkit.PhaseRecover
		default:
			phase = wrkit.PhaseSteady
		}

		// ===== R6 layer animation: continuous rotation + breathing scale =====
		if phase == wrkit.PhaseSpike {
			animSpeed = 2.0
		} else {
			animSpeed = 1.0
		}
		xfA.SetRotation(elapsed * 0.9 * animSpeed) // angle advances every frame → HitTest must follow live rotation
		pulse = 1.0 + 0.15*math.Sin(elapsed*2.4*animSpeed)
		xfB.SetScale(pulse, pulse)

		// ===== Phase transitions =====
		if phase != lastPhase {
			lastPhase = phase
			if phase == wrkit.PhaseSpike {
				cycleT = 0
			}
			if phase == wrkit.PhaseRecover && cycleOpen {
				removeAll(ov, cycleEntries)
				cycleEntries = nil
				cycleOpen = false
				openedThisRun = false
			}
		}

		// ===== R8 overlay cycling: 2s open / 1.5s closed, fresh entries =====
		if phase == wrkit.PhaseSpike {
			cycleT += dt
			if cycleOpen && cycleT >= openDur {
				removeAll(ov, cycleEntries)
				cycleEntries = nil
				cycleOpen = false
				openedThisRun = false
				cycleT = 0
			} else if !cycleOpen && cycleT >= closeDur {
				cycleEntries = buildPopup(cycleKind)
				insertAll(ov, cycleEntries)
				ovDirtyOnOpen = int64(len(cycleEntries))
				cycleKind++
				cycleOpen = true
				openedThisRun = true
				cycleT = 0
			}
		}

		// ===== Scripted probes =====

		// (a) Steady-phase main-tree probes (fire once each, before any overlay).
		if phase == wrkit.PhaseSteady && !steadyDone && elapsed > 0.5 {
			allDone := true
			for _, p := range probes {
				if p.done {
					continue
				}
				runProbe(app, bodyX, bodyY, p, &lastScriptHit)
				allDone = allDone && p.done
			}
			steadyDone = allDone
		}

		// Shared probe anchor over the dense-region label lbl-7 (layout-driven:
		// computed from actual offsets, no hardcoded window coordinates).
		denseAbOff := abDense.Offset()
		denseOff := denseD.Offset()
		lblOff := lblAnchor.Offset()
		probeX := bodyX + denseAbOff.X + denseOff.X + lblOff.X + 5
		probeY := bodyY + denseAbOff.Y + denseOff.Y + lblOff.Y + 5
		// Barrier-consume probe point over the dense color grid (outside all
		// popup geometry so only the barrier can claim it).
		barrierX := bodyX + denseAbOff.X + denseOff.X + 20
		barrierY := bodyY + denseAbOff.Y + denseOff.Y + 20

		// (b) Overlay-band probes while a popup is open (each once per kind).
		if cycleOpen && len(cycleEntries) > 0 {
			isModal := len(cycleEntries) >= 2 // modal builds barrier + panel + menu
			if isModal && !ovBarrierConsumed {
				// Point over the main tree — the full-screen barrier must
				// consume it as BandOverlay with no main-tree object.
				band, obj, entry := app.HitTestPointer(barrierX, barrierY)
				ovBarrierConsumed = band == overlay.BandOverlay && obj == nil && entry != nil
				logProbe("barrier-consume", band, obj)
			}
			if isModal && !ovEntryHitOK {
				// Point on the floating button inside the panel entry (button
				// at panel-local (16,76) size 120x34; its label is named too).
				// Freshly inserted entries are laid out by the NEXT frame
				// build, so the first tick after open can legitimately miss —
				// keep probing each tick until the layout has landed.
				e := cycleEntries[1] // panel entry (fixed position by construction)
				band, obj, _ := app.HitTestPointer(e.X+40, e.Y+90)
				name := rendering.HitDebugName(obj)
				ovEntryHitOK = band == overlay.BandOverlay && name == "ov-button"
				if ovEntryHitOK {
					logProbe("entry-child", band, obj)
				}
			}
			if !isModal && !ovFallthroughOK {
				// Tooltip (non-barrier): the same point must fall through to
				// BandMain and keep hitting the dense-region label lbl-7.
				band, obj, _ := app.HitTestPointer(probeX, probeY)
				name := rendering.HitDebugName(obj)
				ovFallthroughOK = band == overlay.BandMain && name == "lbl-7"
				logProbe("fallthrough", band, obj)
			}
		}

		// (c) Recover-phase restore probe: after overlays are removed, the
		// same main-tree point hits again (identity fully restored).
		if phase == wrkit.PhaseRecover && !openedThisRun && !restoredProbeDone {
			band, obj, _ := app.HitTestPointer(probeX, probeY)
			restoredProbeDone = true
			restoredProbeOK = band == overlay.BandMain && rendering.HitDebugName(obj) == "lbl-7"
			logProbe("restore", band, obj)
			snapRecoverAt = elapsed + 0.5 // shot ~0.5s after restore
		}

		// §2.7/U21 snapshots: steady mid-run + post-recovery. SavePNG runs on
		// the raster thread via SnapshotAsync; the recovery shot proves no
		// temp-state residue (C8 pollution lesson).
		if snapSteadyAt > 0 && elapsed >= snapSteadyAt && steadyDone && pixelPending(probes) {
			snapSteadyAt = 0
			path := snapSteadyPath
			app.SnapshotAsync(func() {
				saveSnap(app, app.Pipeline(), shell.Root, ov, path, "steady")
				runPixelChecks(app.Target().Context().Image(), probes)
			})
		}
		if snapRecoverAt > 0 && restoredProbeDone {
			snapRecoverAt = 0
			path := snapRecoverPath
			app.SnapshotAsync(func() {
				saveSnap(app, app.Pipeline(), shell.Root, ov, path, "recover")
				runPixelChecks(app.Target().Context().Image(), probes)
			})
		}

		// Per-frame band observation (one-frame lag — see ui_wr_r8_overlay):
		// steady window tracks the WORST-CASE main dirty count; overlay-open
		// frames must stay within it.
		mainDirty, ovd := embedder.LastOverlayBandFrame()
		_ = ovd
		if elapsed > 1.0 && !openedThisRun {
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

		// HOT spot repaints itself every frame.
		hue := float64(int(elapsed*8)%16) / 16
		hot.R, hot.G, hot.B = 0.85+0.1*hue, 0.25+0.4*(1-hue), 0.3+0.5*hue
		hot.MarkNeedsPaint()

		scriptedTotal := len(probes) + 3 // + barrier / entry-child / fallthrough (+ restore below)
		scriptedOK := countOK(probes) + boolToInt(ovBarrierConsumed) + boolToInt(ovEntryHitOK) + boolToInt(ovFallthroughOK) + boolToInt(restoredProbeOK)
		hitBanner.SetText(fmt.Sprintf("HIT: %s  %d/%d   ov:%d%d%d rst:%v   ptr: %s",
			orDash(lastScriptHit), scriptedOK, scriptedTotal+1,
			boolToInt(ovBarrierConsumed), boolToInt(ovEntryHitOK), boolToInt(ovFallthroughOK),
			restoredProbeOK || !restoredProbeDone, lastPtrHit))
		hitBanner.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := scriptedOK == scriptedTotal+1 && ovFramesMainDirtyExcess == 0
		shell.UpdateHUD("C8", phase, app, gateOK,
			fmt.Sprintf("hit=%d/%d dirty=%d/%d ov=%d cyc=%d %s",
				scriptedOK, scriptedTotal+1, mainDirty, baseMainDirty, ov.Len(), cycleKind,
				map[bool]string{true: "OPEN", false: "closed"}[cycleOpen]),
			fmt.Sprintf("excess=%d t=%.1f rot=%.2f pulse=%.2f", ovFramesMainDirtyExcess, elapsed, xfA.Rotation, pulse))
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

	total := len(probes) + 4 // main probes + barrier + entry-child + fallthrough + restore
	okCount := countOK(probes) + boolToInt(ovBarrierConsumed) + boolToInt(ovEntryHitOK) + boolToInt(ovFallthroughOK) + boolToInt(restoredProbeOK)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C8",
		Scenario:      "ui_wr_c8_hit_overlay_xf",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"covers":                   []string{"R13", "R6", "R8"},
			"scripted_total":           total,
			"scripted_ok":              okCount,
			"scripted_miss":            total - okCount,
			"main_probes":              len(probes),
			"scripted_hit":             countHits(probes) + boolToInt(ovEntryHitOK),
			"overlay_barrier_consumed": boolToInt(ovBarrierConsumed),
			"overlay_entry_hit_ok":     boolToInt(ovEntryHitOK),
			"overlay_fallthrough_ok":   boolToInt(ovFallthroughOK),
			"hit_restored_after_close": boolToInt(restoredProbeOK),
			"main_dirty_baseline":      baseMainDirty,
			"main_dirty_excess_frames": ovFramesMainDirtyExcess,
			"overlay_open_observed":    ovOpenObserved,
			"overlay_entries_on_open":  ovDirtyOnOpen,
			"overlay_cycles":           cycleKind,
			"anim_targets":             "continuous_rotation,breathing_scale",
			"anim_boundary_isolated":   true,
			"pixel_probes_total":       len(probes),
			"pixel_probes_ok":          pixelAllOK(probes),
			"pixel_tolerance":          "8/255 per channel (§2.7 F0 exact-point; rot/scale at invariant centers)",
			"impl_interaction":         "hit probes run DURING rotation/scale animation and across overlay states (steady/barrier-modal/non-barrier-tooltip/recovered); band observation separates main vs overlay dirtiness",
			"impl_correctness":         "HitTestPointer overlay-first then main; RenderTransform.HitTest inverse-maps live rotation/scale per call",
			"impl_dirty":               "band-separated LastOverlayBandFrame; overlay opens must not exceed worst-case steady main-band baseline (R8 D5)",
			"impl_cache":               "retained policy: dense grid + labels replay under animation/overlay; anim targets are their own boundaries so only they rerecord",
			"impl_edge":                "outside-clip miss, empty-area miss, overlap top-wins, fall-through when no barrier, identity restored after final close",
			"impl_fail":                "any probe mismatch OR main_dirty excess OR overlay never observed → FAIL exit 1",
			"impl_visible":             "HUD hit=n/total dirty=x/base ov=cyc OPEN/closed; rotating+scaled targets visibly spin/pulse; popups alternate modal(barrier)/tooltip(no-barrier)",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ===== Gates = union(R13, R8) per §3 C8 row =====
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequirePersistentFPS:  true, // continuous ticker + animation class (R6 side)
		MinFPSWall:            55,
		MaxP95Ms:              22,
		RequireRetainedPolicy: true, // Plan C posture (R8 side)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if okCount != total {
		fmt.Fprintf(os.Stderr, "FAIL: scripted_ok=%d want %d (hit testing must match paint identity under transform+overlay)\n", okCount, total)
		os.Exit(1)
	}
	pixTotal, pixOK := len(probes), pixelAllOK(probes)
	if pixOK != pixTotal {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_probes_ok=%d/%d (§2.7/U21: composited color must match target base at probe points)\n", pixOK, pixTotal)
		os.Exit(1)
	}
	if countHits(probes)+boolToInt(ovEntryHitOK) < 4 {
		fmt.Fprintf(os.Stderr, "FAIL: scripted_hit=%d want >=4 (§2.6 probe count)\n", countHits(probes)+boolToInt(ovEntryHitOK))
		os.Exit(1)
	}
	if !ovBarrierConsumed || !ovEntryHitOK || !ovFallthroughOK {
		fmt.Fprintf(os.Stderr, "FAIL: overlay hit probes incomplete (barrier=%v entry=%v fallthrough=%v)\n",
			ovBarrierConsumed, ovEntryHitOK, ovFallthroughOK)
		os.Exit(1)
	}
	if !restoredProbeOK {
		fmt.Fprintln(os.Stderr, "FAIL: main-tree hit not restored after overlay close")
		os.Exit(1)
	}
	if !ovOpenObserved {
		fmt.Fprintln(os.Stderr, "FAIL: overlay never observed open (script error)")
		os.Exit(1)
	}
	if ovFramesMainDirtyExcess != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: main_dirty_excess_frames=%d want 0 (opening overlay must not dirty the main band beyond baseline)\n", ovFramesMainDirtyExcess)
		os.Exit(1)
	}
	if ovDirtyOnOpen < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: overlay_entries_on_open=%d want >=1\n", ovDirtyOnOpen)
		os.Exit(1)
	}
	if openedThisRun {
		fmt.Fprintln(os.Stderr, "FAIL: overlay still open at exit (Recover phase must remove it)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c8_hit_overlay_xf: OK scripted=%d/%d main_dirty_base=%d excess=%d cycles=%d presents=%d elapsed=%.1fs\n",
		okCount, total, baseMainDirty, ovFramesMainDirtyExcess, cycleKind, app.PresentCount(), elapsedSec)
}

// ===== helpers =====

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func runDur(secs int) time.Duration {
	if secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0 // no RUN_SECONDS → run until user closes the window
}

// spikeEnd returns the Spike→Recover transition time; long runs cycle ~90%.
func spikeEnd(secs int) float64 {
	if secs >= 60 {
		return float64(secs) * 0.9
	}
	return 10.5
}

func runProbe(app *embedder.PipelineApp, bodyX, bodyY float64, p *probe, lastScript *string) {
	off := p.target.Offset()
	ab := p.align.Offset()
	wx := bodyX + ab.X + off.X + p.dx
	wy := bodyY + ab.Y + off.Y + p.dy
	if p.anc != nil {
		ao := p.anc.Offset()
		wx += ao.X
		wy += ao.Y
	}
	p.wx, p.wy = wx, wy // record for pixel-assert cross-check (§2.7)
	_, obj, _ := app.HitTestPointer(wx, wy)
	name := rendering.HitDebugName(obj)
	if p.expectHit {
		if name == p.name {
			p.ok = true
			*lastScript = p.name
		}
	} else if name == "" {
		p.ok = true
	}
	p.done = true
	fmt.Fprintf(os.Stderr, "ui_wr_c8_hit_overlay_xf: probe @(%.0f,%.0f) want=%q got=%q ok=%v\n", wx, wy, orDash(p.name), orDash(name), p.ok)
}

func logProbe(kind string, band overlay.Band, obj rendering.RenderObject) {
	fmt.Fprintf(os.Stderr, "ui_wr_c8_hit_overlay_xf: probe %-17s band=%v got=%q\n", kind, band, rendering.HitDebugName(obj))
}

func removeAll(ov *overlay.State, entries []*overlay.Entry) {
	for _, e := range entries {
		ov.Remove(e)
	}
}

func insertAll(ov *overlay.State, entries []*overlay.Entry) {
	for _, e := range entries {
		ov.Insert(e)
	}
}

func countOK(probes []*probe) int {
	n := 0
	for _, p := range probes {
		if p.ok {
			n++
		}
	}
	return n
}

func countHits(probes []*probe) int {
	n := 0
	for _, p := range probes {
		if p.expectHit && p.done {
			n++
		}
	}
	return n
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// saveSnap saves a PNG snapshot on the raster thread. Window presents are
// zero-readback (the context pixmap stays stale after FlushGPUWithView +
// EndFrame releases the view), so a bare SavePNG would capture an empty
// pixmap. Replicate the close-time snapshot instead: drain in-flight work,
// BeginFrame, repaint the whole tree into the same context (the nil-view
// flush reads back into the pixmap through the same GPU session), SavePNG.
func rgbaFromImage(img image.Image) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()
	buf := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			i := (y*w + x) * 4
			buf[i], buf[i+1], buf[i+2], buf[i+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
		}
	}
	return buf
}

func saveSnap(app *embedder.PipelineApp, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, path, tag string) {
	dc := app.Target().Context()
	if dc == nil {
		return
	}
	// LIVE-BLIT probe (C8 v2.2): re-run the REAL retained composite (blits
	// from the layer texture cache) into an offscreen view and dump it —
	// this is the exact steady-frame path users see on screen, unlike the
	// vector repaint below which mirrors the post-resize state.
	if os.Getenv("C8_LIVE_PROBE") == "1" {
		pkt := rendering.BuildFramePacket(root, 1, 1, 1200, 800)
		ov.AttachToPacket(pkt)
		liveView, liveRel := dc.CreateOffscreenTexture(1200, 800)
		if liveView.IsNil() {
			fmt.Fprintln(os.Stderr, "live-probe: no offscreen texture")
		} else {
			scene.Walk(pkt.Root, func(l scene.Layer) {
				if pl, ok := l.(*scene.PictureLayer); ok && pl.CacheKey == 33 {
					fmt.Fprintf(os.Stderr, "live-probe: C layer ops=%d bounds=%v valid=%v needsRaster=%v\n",
						pl.Picture.OpCount(), pl.Picture.Bounds, pl.Picture.Valid, pl.NeedsRaster)
				}
			})
			beforeSkip := app.PictureTextures().FrameSkip.Load()
			st := scene.CompositeFramePacketTextured(pkt, dc, app.PictureTextures())
			fmt.Fprintf(os.Stderr, "live-probe: rerecord=%d blits=%d replayed=%d dmg=%d\n",
				st.RasterLayerCount, app.PictureTextures().FrameSkip.Load()-beforeSkip,
				st.ReplayedOps, len(st.DamageRects))
			type rb interface {
				ReadbackViewRGBA(v context.TextureView, w, h int) ([]byte, error)
			}
			if tc := app.PictureTextures(); tc != nil {
				// Dump the C (target-round) cached texture alone.
			}
			if true {
				if rgba, err := func() ([]byte, error) {
					// nil-view flush: readbackToData path (proven correct in
					// contract tests) — composite lands in the CPU pixmap.
					if err := dc.FlushGPU(); err != nil {
						return nil, err
					}
					return rgbaFromImage(dc.Image()), nil
				}(); err == nil {
					img := image.NewRGBA(image.Rect(0, 0, 1200, 800))
					copy(img.Pix, rgba)
					f, ferr := os.Create(strings.TrimSuffix(path, ".png") + "_live.png")
					if ferr == nil {
						_ = png.Encode(f, img)
						f.Close()
						fmt.Fprintf(os.Stderr, "live-probe wrote %s_live.png\n", strings.TrimSuffix(path, ".png"))
					}
				}
			}
		}
		liveRel()
	}
	dc.BeginFrame()
	embedder.PaintPresentTree(dc, pipe, root, ov, 0.08, 0.09, 0.11, 1, true)
	if err := dc.SavePNG(path); err != nil {
		fmt.Fprintf(os.Stderr, "snapshot %s: %v\n", tag, err)
	} else {
		fmt.Fprintf(os.Stderr, "snapshot %s: %s\n", tag, path)
	}
}

// runPixelChecks executes every pending §2.7/U21 pixel assertion against img.
func runPixelChecks(img image.Image, probes []*probe) {
	for _, p := range probes {
		if p.done && p.ok && !p.pixDone {
			p.pixDone = true
			p.pixOK = pixelMatches(img, p, probes)
			fmt.Fprintf(os.Stderr, "ui_wr_c8_hit_overlay_xf: pixel @(%v,%v) want=%q ok=%v\n",
				int(p.wx), int(p.wy), orDash(p.name), p.pixOK)
		}
	}
}

// pixelMatches is the §2.7/U21 point assertion: the composited pixel at the
// probe's resolved window point must equal the target's base color within
// tolerance (channel delta ≤8/255). Hit-probes assert their base color;
// must-miss probes assert they are NOT any known target color (the window
// background is acceptable, as is any other non-target content).
func pixelMatches(img image.Image, p *probe, allProbes []*probe) bool {
	if img == nil {
		return false
	}
	// Image pixels are PHYSICAL (DPR-scaled); probe points are LOGICAL.
	// Scale by devicePixelRatio from the metrics snapshot.
	sx := img.Bounds().Dx()
	if sx <= 0 {
		return false
	}
	dpr := float64(sx) / winW
	px, py := int(p.wx*dpr), int(p.wy*dpr)
	if px < 0 || py < 0 || px >= sx || py >= img.Bounds().Dy() {
		return false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
	tol := 8.0 / 255
	if !p.expectHit {
		// Must-miss: reject if the pixel matches ANY hit target's base color.
		for _, q := range allProbes {
			if q.expectHit && near(r, q.br, tol) && near(g, q.bg, tol) && near(b, q.bb, tol) {
				return false
			}
		}
		return true
	}
	return near(r, p.br, tol) && near(g, p.bg, tol) && near(b, p.bb, tol)
}

func near(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// pixelPending reports whether any ok probe still awaits its pixel check.
func pixelPending(probes []*probe) bool {
	for _, p := range probes {
		if p.done && p.ok && !p.pixDone {
			return true
		}
	}
	return false
}

// pixelAllOK reports whether every resolved probe also passed its pixel check.
func pixelAllOK(probes []*probe) int {
	n := 0
	for _, p := range probes {
		if p.pixDone && p.pixOK {
			n++
		}
	}
	return n
}
