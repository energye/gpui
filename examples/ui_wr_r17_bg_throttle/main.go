// Command ui_wr_r17_bg_throttle is the R17 real-window: invisible windows
// must not burn frames (stop, not throttle).
//
// Phase script (30s close duration, wall clock in the tick pump):
//
//	FG  0–22s  normal persistent rendering, presents flow
//	BG  22–30s self-minimize via WindowController → minimized latch stops
//	           the frame loop: presents must freeze while latched
//
// The script deliberately ends minimized: several WMs (verified Mutter)
// ignore a bare map request for restoring an iconic window — programmatic
// restore is not a portable primitive (Wayland has no unminimize request
// at all). Restore is covered by unit test (embedder MinimizedLatch,
// latch→unlatch→demand) and by the user-driven path (taskbar click →
// MapNotify → StateChanged, platform x11_linux.go).
//
// Stop semantics (user-confirmed direction): the gate is NOT "background
// interval > 2x foreground" (that wording assumes a slow heartbeat). A
// stopped loop produces no interval samples at all, so the honest proof is
// presents frozen inside the latched span. fps_wall over the whole run is
// therefore low by design — read fps_interval (steady foreground pacing)
// instead.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/ui_wr_r17_bg_throttle
//
// Window: 1200x800. RUN_SECONDS>=5 (U16); <30 cannot complete the phase
// script and FAILs honestly. GPU window required. The run ends while
// minimized (no programmatic restore primitive); see the phase note above.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"log/slog"
	"math"
	"os"
	"path/filepath"
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

// Close duration: the FG/BG/RC script needs the full span.
const closeSecs = 30

// Phase boundaries (seconds, wall clock in the tick pump).
const minimizeAt = 22.0

var proc scheduler.ProcessTracker

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSecs
	} else {
		wrkit.RequireMinRun(secs, "R17")
	}

	_, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	render.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r17_bg_throttle — 不可见停帧", Resizable: true, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()
	if ctl == nil {
		fmt.Fprintln(os.Stderr, "FAIL: no WindowController (cannot self-minimize/restore)")
		os.Exit(1)
	}

	shell := wrkit.NewShell(winW, winH, "R17 不可见停帧 — 最小化零提交", []string{
		"FG  = 前台正常渲染 (0-22s, presents 流动)",
		"BG  = 最小化后台 (22-30s, 锁存内零提交)",
		"D   = 静态密集 4x4+8标签",
		"HOT = 每帧动画热点",
		"BG段 fps_wall 低是设计使然 (停帧无采样)",
		"恢复由单测覆盖 (窗内以 MapNotify 用户路径为准)",
	})

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

	// Foreground content: a panel that visibly advances while frames flow.
	fgBox := rendering.NewRenderBox()
	fgBox.FixedWidth, fgBox.FixedHeight = 260, 150
	var fgTick int64
	fgBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		rendering.FillRect(pc, 0, 0, size.Width, size.Height, 0.16, 0.35, 0.30, 1)
		bar := float64(fgTick%100) / 100 * (size.Width - 20)
		rendering.FillRect(pc, 10, 60, bar, 24, 0.30, 0.85, 0.55, 1)
		drawText(pc, 12, 20, fmt.Sprintf("FG frame %d", fgTick), 0.92, 0.97, 0.93)
		drawText(pc, 12, 110, "前台流动 / 后台冻结", 0.75, 0.88, 0.78)
	}
	shell.Body.Align(fgBox, 0.04, 0.06)

	// Static dense content: 4x4 color grid + 8 labels (U17).
	denseD := rendering.NewAbsoluteBox(330, 240)
	denseD.Background = &rendering.Color{R: 0.13, G: 0.15, B: 0.21, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			denseD.Place(c, 10+float64(i)*66, 10+float64(j)*40)
		}
	}
	for i := 0; i < 8; i++ {
		row, col := i/4, i%4
		denseD.Place(wrkit.Label(fmt.Sprintf("lbl-%d", i), 10, 0.62, 0.72, 0.85), 10+float64(col)*66, 178+float64(row)*22)
	}
	shell.Body.Align(denseD, 0.55, 0.05)

	// HOT spot: dirties itself every frame (live paint, non-boundary).
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.88, 0.05)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.88, 0.16)

	// Status banner: latch state + per-phase present counts (U18 core).
	banner := wrkit.Label("R17: init", 13, 0.95, 0.90, 0.55)
	shell.Body.Align(banner, 0.28, 0.30)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r17_bg_throttle: close (%s)\n", win.Kind())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	// Phase observation state (written by the tick pump, read at report).
	var elapsed float64
	phase := wrkit.PhaseSteady
	var minimizeCalled, latchSeen bool
	var latchPresents int64
	var latchT float64
	var fgPresents, bgPresents int64
	// Pixel proof (§2.7/U21, coordinate-free): two FG re-render snapshots
	// must both show content (anti-black) and differ only slightly
	// (liveness, anti-frozen) with the bulk identical (anti-flicker).
	// Snapshots run on the raster thread via SnapshotAsync (C4 pattern).
	pxDir, err := os.MkdirTemp("", "r17px")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: pixel tempdir:", err)
		os.Exit(1)
	}
	snapA := filepath.Join(pxDir, "fg_a.png")
	snapB := filepath.Join(pxDir, "fg_b.png")
	var snapADone, snapBDone bool

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		presents := app.PresentCount()
		minimized := app.Minimized()
		occluded := app.Occluded()

		switch {
		case elapsed < minimizeAt:
			phase = wrkit.PhaseSteady
			fgPresents = presents
		default:
			phase = "Background"
			if !minimizeCalled {
				minimizeCalled = true
				ctl.Minimize()
				fmt.Fprintf(os.Stderr, "ui_wr_r17_bg_throttle: minimize called at t=%.1f\n", elapsed)
			}
			// The background span is measured from the latch edge, not
			// from the controller call: WM round-trips legitimately
			// land a few transition frames.
			if minimized && !latchSeen {
				latchSeen = true
				latchT = elapsed
				latchPresents = presents
				fmt.Fprintf(os.Stderr, "ui_wr_r17_bg_throttle: minimized latch observed at t=%.1f presents=%d\n", elapsed, presents)
			}
			if latchSeen {
				bgPresents = presents - latchPresents
			}
		}

		// FG pixel snapshots at two distant steady points (both well
		// before minimizeAt): A≈6s, B≈14s.
		if !snapADone && elapsed >= 6.0 {
			snapADone = true
			takeSnap(app, shell, snapA, "A")
		}
		if !snapBDone && elapsed >= 14.0 {
			snapBDone = true
			takeSnap(app, shell, snapB, "B")
		}

		// Foreground animation demand (frozen automatically while latched:
		// the loop clears demand at the visibility gate, no presents).
		fgTick++
		fgBox.MarkNeedsPaint()
		hotHue := float64(int(elapsed*8)%16) / 16
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		banner.SetText(fmt.Sprintf("R17 %s min=%v occ=%v fg=%d bg=%d", phase, minimized, occluded, fgPresents, bgPresents))
		banner.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		gateOK := fgPresents >= 60 && (!latchSeen || bgPresents <= 2)
		shell.UpdateHUD("R17", phase, app, gateOK,
			fmt.Sprintf("fg=%d bg=%d min=%v", fgPresents, bgPresents, minimized),
			fmt.Sprintf("t=%.1f latch=%v", elapsed, latchSeen))
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

	// Pixel verdicts (coordinate-free frame comparison).
	pxDiff, pxContent, pxErr := compareSnaps(snapA, snapB)
	if pxErr != nil {
		fmt.Fprintf(os.Stderr, "FAIL: pixel snapshots: %v (files kept at %s)\n", pxErr, pxDir)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r17_bg_throttle: pixel diff_frac=%.4f content_frac=%.4f\n", pxDiff, pxContent)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R17",
		Scenario:      "ui_wr_r17_bg_throttle",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"backend":            win.Kind().String(),
			"fg_presents":        fgPresents,
			"bg_presents":        bgPresents,
			"minimized_observed": latchSeen,
			"minimize_t":         latchT,
			"pixel_diff_frac":    pxDiff,
			"pixel_content_frac": pxContent,
			"restore_coverage":   "unit test TestHandleLifecycle_MinimizedLatch + user-driven MapNotify path (no portable programmatic restore)",
			"stop_semantics":     "presents frozen while latched (no heartbeat frames)",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// Schema gate first (full A–J family must be present).
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	// R17 stop gates (phase script needs the close duration).
	if secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS=%d < %d: phase script incomplete, cannot close R17\n", secs, closeSecs)
		os.Exit(1)
	}
	if !minimizeCalled {
		fmt.Fprintln(os.Stderr, "FAIL: minimize was never attempted (script did not reach BG phase)")
		os.Exit(1)
	}
	if !latchSeen {
		fmt.Fprintln(os.Stderr, "FAIL: minimized latch never observed (WM did not report iconify)")
		os.Exit(1)
	}
	if fgPresents < 60 {
		fmt.Fprintf(os.Stderr, "FAIL: fg_presents=%d want >=60 (foreground must flow before minimize)\n", fgPresents)
		os.Exit(1)
	}
	if bgPresents > 2 {
		fmt.Fprintf(os.Stderr, "FAIL: bg_presents=%d want <=2 (no frames while latched)\n", bgPresents)
		os.Exit(1)
	}
	// diff==0 means the second snapshot never showed new content
	// (frozen pipe presented as flow); diff>5% means the steady scene
	// itself is unstable (flicker). Content must clear the clear-color
	// floor or the window rendered blank.
	if pxDiff <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel diff_frac=%.4f want >0 (FG snapshots 8s apart must show motion)\n", pxDiff)
		os.Exit(1)
	}
	if pxDiff > 0.05 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel diff_frac=%.4f want <=0.05 (steady scene unstable)\n", pxDiff)
		os.Exit(1)
	}
	// content_frac is the share of pixels clearly off the clear color
	// (any channel >24/255 away, C4 textPixels convention): a blank
	// clear-colored frame scores ~0, this scene several percent.
	if pxContent < 0.01 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel content_frac=%.4f want >=0.01 (frame looks blank)\n", pxContent)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r17_bg_throttle: OK fg=%d bg=%d presents=%d elapsed=%.1fs\n",
		fgPresents, bgPresents, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

// takeSnap re-renders the live tree offscreen and writes a PNG. Runs on the
// raster thread via SnapshotAsync (C4 pattern); the caller blocks until the
// raster drain executes it.
func takeSnap(app *embedder.PipelineApp, shell *wrkit.ShellChrome, path, tag string) {
	app.SnapshotAsync(func() {
		dc := app.Target().Context()
		if dc == nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: no gpu context\n", tag)
			return
		}
		dc.BeginFrame()
		embedder.PaintPresentTree(dc, app.Pipeline(), shell.Root, nil, 0.08, 0.09, 0.11, 1, true)
		if err := dc.SavePNG(path); err != nil {
			fmt.Fprintf(os.Stderr, "snapshot %s: %v\n", tag, err)
		}
	})
}

// compareSnaps loads the two FG snapshots and returns the fraction of
// differing pixels plus the fraction of pixels clearly off the clear color
// (any channel >24/255 away — the C4 textPixels convention, robust for dark
// scenes where a mean-deviation metric would sit near its floor).
// Same bounds required (a mid-run resize invalidates the comparison).
func compareSnaps(pathA, pathB string) (diffFrac, contentFrac float64, err error) {
	a, err := loadImage(pathA)
	if err != nil {
		return 0, 0, err
	}
	b, err := loadImage(pathB)
	if err != nil {
		return 0, 0, err
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v (window resized mid-run?)", a.Bounds(), b.Bounds())
	}
	var diff, content, total int64
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			total++
			if ar != br || ag != bg || ab != bb {
				diff++
			}
			fr, fg, fb := float64(ar>>8)/255, float64(ag>>8)/255, float64(ab>>8)/255
			if math.Abs(fr-0.08) > 24.0/255 || math.Abs(fg-0.09) > 24.0/255 || math.Abs(fb-0.11) > 24.0/255 {
				content++
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty snapshot")
	}
	return float64(diff) / float64(total), float64(content) / float64(total), nil
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}
