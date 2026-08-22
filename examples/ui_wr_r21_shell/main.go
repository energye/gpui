// Command ui_wr_r21_shell is the W2-W4 R21 real-window: shell/content layering.
// A static shell TopBar (title + buttons, tagged SetShellBoundary) sits above a
// scrollable body (RenderViewport + VirtualList). The body scrolls every frame
// through Steady/Spike/Recover phases; the shell boundary must never re-record:
// its Picture Replays (shell skip grows) while shell rerecord stays 0.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r21_shell
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
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// TopBar height in logical px; the shell band below it hosts the scroll body.
const topBarH = 64.0

var proc scheduler.ProcessTracker

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R21")
	}
	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	render.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r21_shell — 壳/内容分层", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R21 壳/内容分层 — 体滚时顶栏 rerecord=0", []string{
		"SHELL = 顶栏壳层 (标题+按钮, 静态, SetShellBoundary)",
		"BODY  = 可滚动虚拟列表 (60 项混合行)",
		"BODY 滚动只脏体层, 壳层 Replay 不重录",
		"DENSE = 静态密集区 (4x4色格+8标签)",
		"Spike = 体快滚 (8px/帧) — 壳仍 0 重录",
	})

	// DBG MARKERS: absolute root coords, pure colors — calibrate xwd mapping.
	mk := func(x, y float64, r, g, b float64) {
		c := rendering.NewRenderColorBox(10, 10, r, g, b, 1)
		shell.Root.Place(c, x, y)
	}
	mk(0, 0, 1, 0, 1)      // magenta at root (0,0)
	mk(600, 400, 1, 1, 0)  // yellow at (600,400)
	mk(1190, 790, 0, 1, 1) // cyan at (1190,790)

	// ===== SHELL band: static top bar (title + 2 buttons + phase chip) =====
	// The whole band is one RepaintBoundary tagged SetShellBoundary: body
	// scrolling must never re-record it (shell_rerecord == 0).
	bar := rendering.NewAbsoluteBox(winW-260, topBarH)
	bar.Background = &rendering.Color{R: 0.16, G: 0.18, B: 0.26, A: 1}
	bar.SetRepaintBoundary(true)
	bar.SetShellBoundary(true)
	bar.Place(wrkit.Label("SHELL — 标题/按钮/相位 壳层", 16, 0.95, 0.96, 0.99), 14, 10)
	// Buttons are ONE RepaintBoundary each (background + text as a group):
	// a nested RB paints AFTER the parent's Picture Replay, so a RB color box
	// would cover sibling non-RB text laid over it. Grouping keeps the button
	// atomic and the shell band z-order correct.
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
	// Font-size samples (8/10/12/14/16): static text inside the shell band,
	// must keep rerecord=0 while the body scrolls.
	sx := 530.0
	for _, s := range []float64{8, 10, 12, 14, 16} {
		bar.Place(wrkit.Label(fmt.Sprintf("%dpx 样张", int(s)), s, 0.92, 0.95, 1), sx, 44)
		sx += 28 + s*3.0
	}
	shell.Root.Place(bar, 10, 6)

	// The phase chip lives OUTSIDE the shell boundary: it changes every frame
	// (dynamic), so tagging it shell would defeat the shell/content split.
	bodyPhase := wrkit.Label("PHASE=Steady bodyScr=0", 12, 0.85, 0.95, 0.55)
	shell.Root.Place(bodyPhase, 12, topBarH+2)

	// ===== BODY: scrollable virtual list =====
	bodyW := winW - 520.0
	// Viewport height stops above the LiveHUD band (winH-HUDH-gap): an unset
	// height grows the viewport to the window bottom and scrolled rows cover
	// the HUD (z-order: viewport children paint above the HUD).
	const hudGap = 12.0
	vlist := rendering.NewVirtualList(60, 44, func(i int) rendering.RenderObject {
		row := rendering.NewAbsoluteBox(bodyW, 40)
		row.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.20, A: 1}
		// Cell content: color chip + text label (cacheable → scroll skip).
		chip := rendering.NewRenderColorBox(26, 26, 0.3+float64(i%3)*0.2, 0.45+float64(i%2)*0.3, 0.6, 1)
		chip.SetRepaintBoundary(true)
		row.Place(chip, 8, 7)
		row.Place(wrkit.Label(fmt.Sprintf("item-%d 滚动内容行", i), 12, 0.85, 0.88, 0.93), 44, 11)
		return row
	})
	viewport := rendering.NewRenderViewport(vlist)
	viewport.SetScrollOffset(0, 0)
	viewport.FixedHeight = winH - topBarH - shell.HUDH - hudGap*2
	shell.Root.Place(viewport, 12, topBarH+14)

	// ===== static dense content (U17): 4x4 grid + 8 labels =====
	dense := rendering.NewAbsoluteBox(300, 300)
	dense.Background = &rendering.Color{R: 0.11, G: 0.12, B: 0.18, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(56, 28, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			dense.Place(c, 10+float64(i)*64, 10+float64(j)*38)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		dense.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*64, 168+float64(row)*20)
	}
	shell.Root.Place(dense, 470, topBarH+14)

	// HOT spot: dirties itself every frame (live paint, non-boundary).
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
				fmt.Fprintf(os.Stderr, "ui_wr_r21_shell: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	// Scroll-state gates: after the initial record (warm-up), every frame's
	// shell partition must show rerecord=0 while the body scrolls.
	var scrollShellRerecord, scrollShellSkip int64 // steady+spike+recover scroll frames
	var warmFrames int
	var scrollY float64
	phase := wrkit.PhaseSteady

	var elapsed float64
	var lastWinW, lastWinH int
	var resizeSkip int
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// Resize/DPR-change frames legally re-record the shell (size changed).
		// Exclude them from the scroll-gate sample so external WM resize during
		// a run can't false-fail the shell_rerecord_scroll==0 gate. The rerecord
		// lands on the paint after the event, so skip the next few samples.
		cw, ch := host.Size()
		if cw != lastWinW || ch != lastWinH {
			lastWinW, lastWinH = cw, ch
			resizeSkip = 3
		}
		resizeFrame := resizeSkip > 0
		if resizeSkip > 0 {
			resizeSkip--
		}
		switch {
		case elapsed >= 5.0 && elapsed < 8.0:
			phase = "Spike"
		case elapsed >= 8.0:
			phase = wrkit.PhaseRecover
		}

		// Body scroll: rate changes with phase (Steady slow, Spike fast,
		// Recover slow again) — content motion only, shell untouched.
		rate := 0.5
		if phase == "Spike" {
			rate = 8.0
		}
		scrollY += rate
		// Infinite scroll: wrap back to the top when reaching content end so
		// the list keeps scrolling without ever sticking at the bottom.
		// (maxScrollY = content - viewport; a bounce clamp pins there.)
		if scrollY >= 60*44 {
			scrollY = 0
		}
		viewport.SetScrollOffset(0, scrollY)

		// HOT spot repaints itself every frame.
		hotHue := float64(int(elapsed*8)%16) / 16
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		// Sample shell partition on every frame after warm-up.
		srr, ssk := embedder.LastShellBoundaryFrame()
		if warmFrames >= 2 && !resizeFrame { // warm-up frames may cold-record the shell
			scrollShellRerecord += srr
			scrollShellSkip += ssk
		} else if warmFrames < 2 {
			warmFrames++
		}

		bodyPhase.SetText(fmt.Sprintf("PHASE=%s bodyScr=%.0f", phase, scrollY))
		bodyPhase.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := scrollShellRerecord == 0 && scrollShellSkip > 0
		shell.UpdateHUD("R21", phase, app, gateOK,
			fmt.Sprintf("shell_rr=%d shell_skip=%d", scrollShellRerecord, scrollShellSkip),
			fmt.Sprintf("body_scroll=%.0f t=%.1f", scrollY, elapsed))
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
		AbilityID:     "R21",
		Scenario:      "ui_wr_r21_shell",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"shell_rerecord_scroll":           scrollShellRerecord, // must be 0
			"shell_skip_scroll":               scrollShellSkip,     // must be > 0
			"shell_rerecord_total":            snap.ShellRerecord,  // cold record during warm-up
			"shell_skip_total":                snap.ShellSkip,
			"body_items":                      60,
			"body_scroll_px_per_frame_steady": 0.5,
			"body_scroll_px_per_frame_spike":  8.0,
			"topbar_shell_boundary":           true,
			"static_dense_cells":              16,
			"static_labels":                   8,
			// RSS note: 15s runs sit inside the startup equilibrium ramp
			// (Go heap + GPU driver); R8 600s soak proved the plateau
			// (~167KB/min steady slope). Reported as observation here.
			"rss_slope_semantics": "startup-ramp dominated at 15s; see ENGINE_UI_WIDGET_RENDER §10 R8 场景化长跑",
			"impl_correctness":    "shell band (topbar) is one RepaintBoundary tagged SetShellBoundary; body scroll dirties only body layers — shell Picture Replays untouched (Flutter shell/content split)",
			"impl_dirty":          "scroll updates viewport offset → only body boundary re-records; LastShellBoundaryFrame samples the shell partition per frame after warm-up",
			"impl_cache":          "shell boundary cache entry survives scrolling (skip grows ~137/frame while rr stays 0); resize legally invalidates and is excluded from the gate window",
			"impl_edge":           "resize frames excluded (3-frame settle) so external WM resizes can't false-fail the gate; infinite scroll wraps at content end; HUD/phase chips live OUTSIDE the shell boundary (self-dirty every frame would defeat the split)",
			"impl_fail":           "any shell rerecord during scroll frames → FAIL (shell_rr_scroll!=0); no shell replay at all → FAIL (cache not engaged)",
			"impl_visible":        "HUD shell_rr/shell_skip live numbers; topbar pixels never change while rows scroll beneath it",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R21 gates (§2 主表: 滚体时顶栏 rerecord=0) + §2.2 全族硬线.
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true, // continuous scroll ticker
		MinFPSWall:             55,
		MaxP95Ms:               22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if scrollShellRerecord != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: shell_rerecord_scroll=%d want 0 (body scroll must not re-record shell)\n", scrollShellRerecord)
		os.Exit(1)
	}
	if scrollShellSkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: shell_skip_scroll=%d want >0 (shell Picture must replay during body scroll)\n", scrollShellSkip)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r21_shell: OK shell_rr_scroll=%d shell_skip_scroll=%d shell_rr_total=%d presents=%d elapsed=%.1fs\n",
		scrollShellRerecord, scrollShellSkip, snap.ShellRerecord, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
