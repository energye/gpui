// Command ui_wr_r4_composite is the W2 R4 real-window: 层 Composite Present (Retained).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r4_composite
//
// Window: 1200x800. RUN_SECONDS>=15 (U16, §2.5: 15s/观察 30). GPU window required.
// Gates: present_policy=retained; 稳态 damage_ratio<=0.35（远小于全屏）;
// present_mode 非 full（damage_union/multi）; boundary_skip>0（静态区不重绘）;
// 持续 tick fps>=55。Retained 下静态区域每帧不重绘、局部动画区持续变化。
//
// Flutter 对齐用法：hot 块不手算坐标，由 wrkit.Panel.Align 布局驱动——
// 位置 = (父尺寸 - 子尺寸) × 对齐比例，resize 时 Layout 自动重算 offset，
// 无 clamp 跳变（对齐 Flutter Align/FractionallySizedBox 语义）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
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
		wrkit.RequireMinRun(secs, "R4")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r4_composite — 层 Composite Present"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R4 层 Composite Present — retained 省绘", []string{
		"RETENTED: 大面积静态背景不重绘",
		"局部动画区持续变化",
		"damage_ratio<=0.35（≪全屏）",
		"present_mode 非 full（局部损伤）",
		"静态区 boundary_skip>0",
		"HOT 块 Align 布局驱动（Flutter 对齐）",
	})

	// 大面积静态背景：8x5 色格平铺左区（多 boundary 区域，retained 下不重绘）。
	// 注意：Body 起点在 surface (284,60)（legend 左栏 + top 栏），body 宽 904；
	// 格子必须落在 body 视口内，否则 damage rect 越界 → present 退化全屏。
	staticCount := 0
	for i := 0; i < 8; i++ {
		for j := 0; j < 5; j++ {
			c := rendering.NewRenderColorBox(110, 110, 0.14+0.08*float64(i%5), 0.45+0.06*float64(j%4), 0.3+0.08*float64((i+j)%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*110, 20+float64(j)*110)
			staticCount++
		}
	}
	// 大面积静态区域标签（也是静态，不重绘）。
	for i := 0; i < 4; i++ {
		t := wrkit.Label(fmt.Sprintf("static bg %02d", i), 12, 0.7, 0.75, 0.85)
		shell.Body.Place(t, 40+float64(i)*210, 640)
		staticCount++
	}
	// 局部动画区（右下）：HOT 动块 + 相位横幅（唯一持续变化区域）。
	// HOT 块用 Flutter 式 Align 布局驱动：位置 = (body - hot) × 比例，
	// resize 时 Layout 自动重算 offset —— 无 clamp、无固定坐标。
	hot := rendering.NewRenderColorBox(100, 100, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	hotAlign := shell.Body.Align(hot, 0.78, 0.86)
	phaseLabel := wrkit.Label("PHASE: STEADY", 16, 0.95, 0.95, 0.4)
	// Phase text changes at phase flips — isolate it in its own layer so the
	// MarkNeedsPaint bubble stops here instead of reaching the window root
	// (root background layer would re-record full-surface → present_mode full).
	phaseLabel.SetRepaintBoundary(true)
	shell.Body.Place(phaseLabel, 700, 520)

	// SnapshotPath saves a GPU readback PNG of the final frame at close
	// (pixel verification: xwd on the X11 backing store lags composite GPU
	// content). Override with R4_SNAPSHOT; empty disables.
	snapPath := os.Getenv("R4_SNAPSHOT")
	if snapPath == "" {
		snapPath = "r4_snapshot.png"
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: close (%s)\n", win.Backend())
			}
			// Responsive shell layout: HUD re-pins to the new bottom, panels
			// re-size (PipelineApp still does the surface/viewport work).
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	// W2 R4: retained 稳态（首帧仍 full，稳态 compositeOnly + damage present）。
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	var phase string
	lastPhase := ""
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		// 相位 → 对齐比例（布局驱动：Layout 时按 body 当前尺寸求 offset）。
		switch phase {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			hotAlign.SetAlignment(0.78, 0.86)
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotAlign.SetAlignment(0.84, 0.86)
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			hotAlign.SetAlignment(0.78, 0.97)
		}
		// 相位切换才重录标签层（稳态保持纹理 blit，损伤=文本矩形）。
		if phase != lastPhase {
			lastPhase = phase
			switch phase {
			case wrkit.PhaseSteady:
				phaseLabel.SetText("PHASE: STEADY")
				phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
			case wrkit.PhaseSpike:
				phaseLabel.SetText("PHASE: SPIKE")
				phaseLabel.SetColor(1.0, 0.8, 0.2, 1)
			default:
				phaseLabel.SetText("PHASE: RECOVER")
				phaseLabel.SetColor(0.2, 0.8, 1.0, 1)
			}
			phaseLabel.MarkNeedsPaint()
		}
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD（retained 稳态损伤比 + present_mode + boundary skip）。
		snapH := app.Metrics().Snapshot()
		avgArea, _, samples, _ := app.DamageStats()
		var avgRatio float64
		if samples > 0 {
			avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		}
		shell.NoteHUDTick(dt)
		gateOK := app.LastPresentMode() != "full" && avgRatio <= 0.35
		shell.UpdateHUD("R4", phase, app, gateOK,
			fmt.Sprintf("dmg=%.2f mode=%s skip=%d", avgRatio, app.LastPresentMode(), snapH.BoundarySkip), "")
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
	avgArea, maxArea, samples, multiFrames := app.DamageStats()
	var avgRatio, maxRatio float64
	if samples > 0 {
		// DamageStats returns the cumulative sum; average per present first.
		avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		maxRatio = float64(maxArea) / float64(winW*winH)
	}
	lastMode := app.LastPresentMode()
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R4",
		Scenario:      "ui_wr_r4_composite",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":      "retained",
			"damage_semantic":     "retained_composite_only",
			"damage_ratio_avg":    avgRatio,
			"damage_ratio_max":    maxRatio,
			"damage_samples":      samples,
			"damage_multi_frames": multiFrames,
			"last_present_mode":   lastMode,
			"static_visible":      staticCount,
			"hot_repaints":        hotTick,
			"hot_aligned":         "align_layout_driven",
			"boundary_skip":       snap.BoundarySkip,
			"boundary_rerecord":   snap.BoundaryRerecord,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R4 gates: retained 稳态省绘（§3 组合表：damage_ratio<=0.35 + present_mode 非 full）。
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.PresentPolicy != scheduler.PresentPolicyRetained {
		fmt.Fprintf(os.Stderr, "FAIL: present_policy=%q want retained", snap.PresentPolicy)
		os.Exit(1)
	}
	if samples == 0 || avgRatio > 0.35 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_ratio_avg=%.3f (samples=%d) > 0.35 (retained 必须远小于全屏)", avgRatio, samples)
		os.Exit(1)
	}
	if lastMode == "" || lastMode == "full" || lastMode == "idle" {
		fmt.Fprintf(os.Stderr, "FAIL: present_mode=%q want damage_union/damage_multi (非 full)", lastMode)
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (静态区必须不重绘)", snap.BoundarySkip)
		os.Exit(1)
	}
	fpsOK := snap.AvgFrameIntervalMs > 1e-6 && 1000.0/snap.AvgFrameIntervalMs >= 55
	if !fpsOK {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick R4)", 1000.0/snap.AvgFrameIntervalMs)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4_composite: OK presents=%d dmg_avg=%.3f mode=%s skip=%d multi=%d fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), avgRatio, lastMode, snap.BoundarySkip, multiFrames,
		1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
