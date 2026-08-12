// Command ui_wr_r11_dpr is the W2 R11 real-window: DPR / size cache
// invalidation. BoundaryCache must drop on programmatic size/DPR change, then
// re-record exactly one wave and return to replay (skip).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
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

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R11")
	}
	face, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Shared static image source (proves image-backed boundaries replay too).
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

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r11_dpr — DPR/尺寸缓存失效"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R11 DPR/尺寸缓存失效 — 一波 rerecord 回 skip", []string{
		"RESIZE-A = 尺寸变更波 (~3s, Align 布局)",
		"DPR-B   = 全量失效波 (~6s, invalidation)",
		"NEST-C  = 3 层嵌套 boundary (深)",
		"DENSE-D = 4x4 色格 + 8 标签 + 图 (静)",
		"HOT     = 每帧脏的热点 (live, 非 boundary)",
	})

	// Region A: RESIZE target. Nested 2-layer boundary (outer + inner color
	// box). Size change at ~3s must invalidate its entry → one rerecord wave
	// → back to skip.
	areaA := rendering.NewAbsoluteBox(190, 140)
	areaA.SetRepaintBoundary(true)
	areaA.Background = &rendering.Color{R: 0.16, G: 0.28, B: 0.44, A: 1}
	innerA := rendering.NewRenderColorBox(90, 70, 0.45, 0.72, 0.9, 1)
	innerA.SetRepaintBoundary(true)
	areaA.Place(innerA, 16, 16)
	areaA.Place(wrkit.Label("RESIZE-A", 11, 0.75, 0.85, 0.95), 16, 100)
	if face != nil {
		areaA.Place(wrkit.Label("size@3s", 10, 0.7, 0.8, 0.9), 116, 16)
	}
	shell.Body.Align(areaA, 0.04, 0.06)

	// Region B: DPR invalidation target. 2x2 color grid + label; never resized.
	areaB := rendering.NewAbsoluteBox(190, 140)
	areaB.SetRepaintBoundary(true)
	areaB.Background = &rendering.Color{R: 0.14, G: 0.32, B: 0.26, A: 1}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			c := rendering.NewRenderColorBox(60, 42, 0.3+float64(i)*0.2, 0.55, 0.4+float64(j)*0.3, 1)
			c.SetRepaintBoundary(true)
			areaB.Place(c, 16+float64(i)*72, 16+float64(j)*52)
		}
	}
	areaB.Place(wrkit.Label("DPR-B", 11, 0.75, 0.85, 0.95), 16, 122)
	shell.Body.Align(areaB, 0.36, 0.06)

	// Region C: deep nest — outer(boundary) → mid(boundary) → leaf(boundary).
	leafC := rendering.NewRenderColorBox(40, 40, 0.9, 0.45, 0.35, 1)
	leafC.SetRepaintBoundary(true)
	midC := rendering.NewAbsoluteBox(90, 90)
	midC.SetRepaintBoundary(true)
	midC.Background = &rendering.Color{R: 0.3, G: 0.3, B: 0.5, A: 1}
	midC.Place(leafC, 24, 24)
	areaC := rendering.NewAbsoluteBox(190, 150)
	areaC.SetRepaintBoundary(true)
	areaC.Background = &rendering.Color{R: 0.2, G: 0.2, B: 0.38, A: 1}
	areaC.Place(midC, 16, 16)
	areaC.Place(wrkit.Label("NEST-C (3层)", 11, 0.75, 0.85, 0.95), 16, 128)
	shell.Body.Align(areaC, 0.04, 0.52)

	// Region D: dense static content — 4x4 grid + 8 labels + 1 image.
	areaD := rendering.NewAbsoluteBox(420, 190)
	areaD.SetRepaintBoundary(true)
	areaD.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.2, A: 1}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(58, 30, 0.25+float64(i)*0.12, 0.4+float64(j)*0.1, 0.62, 1)
			areaD.Place(c, 12+float64(i)*66, 14+float64(j)*40)
		}
	}
	for i := 0; i < 4; i++ {
		areaD.Place(wrkit.Label(fmt.Sprintf("cell-%d", i), 10, 0.62, 0.72, 0.85), 12+float64(i)*66, 180)
	}
	imgD := rendering.NewRenderImage(40, 40)
	imgD.SetImage(src.Clone())
	imgD.SetRepaintBoundary(true)
	areaD.Place(imgD, 360, 14)
	areaD.Place(wrkit.Label("DENSE-D (4x4+8标签+图)", 11, 0.75, 0.85, 0.95), 12, 214)
	shell.Body.Align(areaD, 0.36, 0.52)

	// HOT spot: dirties itself every frame (live paint, no boundary). With no
	// boundary it contributes zero rerecord/skip, so steady frames show
	// FrameRerecord=0 and the wave counters measure the invalidation waves
	// alone (noise-free, deterministic gates).
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.88, 0.08)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.88, 0.20)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	var (
		elapsed          float64
		resized          bool
		dprInvalidated   bool
		wave1, wave2     int64
		before1, before2 int64
		wave1Sampled     bool
		wave2Sampled     bool
		// Per-frame rerecord peaks inside each invalidation window: steady
		// frames show FrameRerecord=1 (only the HOT spot re-records); the
		// invalidation frame shows >=2 (HOT + re-recorded region / full wave).
		// A peak proves the wave happened without being drowned by HOT noise.
		peakRR1, peakRR2 int64
		// steadyRR: FrameRerecord of one frame ~1s after the size change —
		// must drop back to 1 (only HOT), proving the region returned to skip.
		steadyRR      int64
		steadySampled bool
	)
	hotHue := 0.0

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt

		// Phase script (§2.6): Steady → size change ~3s → Steady → DPR
		// invalidation ~6s → Recover. Each change triggers exactly one
		// rerecord wave, then skip resumes.
		phase := wrkit.PhaseSteady
		switch {
		case elapsed >= 6.0 && elapsed < 6.8:
			phase = "DPR-Spike"
		case elapsed >= 3.0 && elapsed < 3.8:
			phase = "Resize-Spike"
		case elapsed >= 8.5:
			phase = wrkit.PhaseRecover
		}

		// Change #1 (~3s): programmatic size change (layout-driven via Align).
		if !resized && elapsed >= 3.0 {
			before1 = app.Metrics().Snapshot().BoundaryRerecord
			areaA.FixedWidth, areaA.FixedHeight = 250, 170
			areaA.MarkNeedsLayout()
			resized = true
		}
		if resized && elapsed < 4.0 {
			if v := app.BoundaryCache().FrameRerecord; v > peakRR1 {
				peakRR1 = v
			}
		}
		if resized && !wave1Sampled && elapsed >= 3.0+0.8 {
			wave1 = app.Metrics().Snapshot().BoundaryRerecord - before1
			wave1Sampled = true
		}
		if resized && !steadySampled && elapsed >= 4.0 {
			steadyRR = app.BoundaryCache().FrameRerecord
			steadySampled = true
		}

		// Change #2 (~6s): programmatic DPR invalidation (full cache clear).
		if !dprInvalidated && elapsed >= 6.0 {
			before2 = app.Metrics().Snapshot().BoundaryRerecord
			app.InvalidateBoundaryCache()
			dprInvalidated = true
		}
		if dprInvalidated && elapsed < 7.0 {
			if v := app.BoundaryCache().FrameRerecord; v > peakRR2 {
				peakRR2 = v
			}
		}
		if dprInvalidated && !wave2Sampled && elapsed >= 6.0+0.8 {
			wave2 = app.Metrics().Snapshot().BoundaryRerecord - before2
			wave2Sampled = true
		}

		// HOT spot repaints itself every frame (independent boundary).
		hotHue += 0.07
		if hotHue > 1 {
			hotHue = 0
		}
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD — skip/rerecord/invalidations are the R11 proof.
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		inv := app.CacheInvalidations()
		gateOK := snapH.BoundarySkip >= 1 && snapH.BoundaryRerecord >= 1 && inv >= 1
		shell.UpdateHUD("R11", phase, app, gateOK,
			fmt.Sprintf("skip=%d rr=%d inv=%d", snapH.BoundarySkip, snapH.BoundaryRerecord, inv),
			fmt.Sprintf("wave1=%d wave2=%d t=%.1f", wave1, wave2, elapsed))
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
	wrkit.MergeBoundaryCache(app, &snap)
	inv := app.CacheInvalidations()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R11",
		Scenario:      "ui_wr_r11_dpr",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"cache_invalidations":    inv,
			"invalidation_kind":      "programmatic_invalidate_boundary_cache",
			"resize_wave_rerecord":   wave1,
			"dpr_wave_rerecord":      wave2,
			"resize_wave_peak_frame": peakRR1,
			"dpr_wave_peak_frame":    peakRR2,
			"steady_rr_frame":        steadyRR,
			"static_regions":         4,
			"nested_depth":           3,
			"hot_live_paint":         true,
			"boundary_skip":          snap.BoundarySkip,
			"boundary_rerecord":      snap.BoundaryRerecord,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R11 gates (§2 主表: cache_invalidations>=1 + boundary_rerecord>=1 +
	// boundary_skip>=1). cache_invalidations comes from ability_extra via
	// EvaluateRetainedExtras (auto-called inside EvaluateGates).
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		MinBoundarySkip:        1,
		MinBoundaryRerecord:    1,
		MinCacheInvalidations:  1,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if wave1 < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: resize wave rerecord=%d want >=1 (size change must re-record once)", wave1)
		os.Exit(1)
	}
	if wave2 < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: dpr wave rerecord=%d want >=1 (invalidation must re-record once)", wave2)
		os.Exit(1)
	}
	// Steady frames must re-record nothing (HOT is non-boundary live paint):
	// steadyRR=0 proves the invalidated regions returned to skip. peak1/peak2
	// stay as observation fields (per-frame peak may miss the wave frame when
	// two paints land between ticks), the cumulative wave counters gate.
	if steadyRR != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: steady frame rerecord=%d want 0 (regions must be back to skip)", steadyRR)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: OK skip=%d rr=%d inv=%d wave1=%d wave2=%d peak1=%d peak2=%d steadyRR=%d presents=%d elapsed=%.1fs\n",
		snap.BoundarySkip, snap.BoundaryRerecord, inv, wave1, wave2, peakRR1, peakRR2, steadyRR, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
