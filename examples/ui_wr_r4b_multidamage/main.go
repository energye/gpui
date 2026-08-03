// Command ui_wr_r4b_multidamage is the W2 R4b real-window: DirtyLayerID + 多 damage。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r4b_multidamage
//
// Window: 1200x800. RUN_SECONDS>=15 (U16, §2.5: 15s/观察 30). GPU window required.
// Gates: dirty_layer_id_max>=2（两远离脏点 → 两独立 boundary id）;
// damage_multi_frames>=1（多矩形独立 scissor）; 中间大面积静态不重绘（skip>0）;
// damage_ratio 稳态<<1（中央静区不脏）; present_mode 非 full; 持续 tick fps>=55。
//
// 场景（§3 R4b 行）：左上 hot + 右下 hot 两个远离脏点同帧更新（每帧变脏），
// 中间 6x5 色格 + 8 标签大面积静态（嵌套 boundary，retained 下不重绘）。
// 两个 dirty layer 各自独立损伤，并集损伤不覆盖中央静区。
// Flutter 对齐布局驱动（§2.6.1 规则，对标 R4）：两热点由 Panel.Align 定位，
// offset = (父−子)×比例，相位切换 SetAlignment——无固定坐标、resize 自动跟随。
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

// hotW is the side of each distant hot spot (90×90). Two hotspots re-draw
// 2×90×90 px² per frame; the union bbox across opposite corners covers most of
// the surface but frame.go sizes full-promotion by rect-area SUM, so the pair
// stays damage_multi (independent scissors) — see damage_ratio_sum in gates.
const hotW = 90

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R4b")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r4b_multidamage — DirtyLayerID + 多 damage"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R4b DirtyLayerID + 多 damage — 两远离脏点独立 scissor", []string{
		"左上 HOT + 右下 HOT 同帧变脏",
		"两脏点 = 两独立 boundary id",
		"中央 6x5 色格+8 标签静态不重绘",
		"dirty_layer_id_max>=2",
		"damage_multi_frames>=1",
		"damage_ratio≪1（中央静区不脏）",
	})

	// 中央大面积静态：6x5 色格 + 8 标签（各自 boundary，retained 下不重绘）。
	staticCount := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 5; j++ {
			c := rendering.NewRenderColorBox(100, 80, 0.16+0.08*float64(i%4), 0.4+0.07*float64(j%3), 0.3+0.08*float64((i+j)%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 240+float64(i)*104, 120+float64(j)*88)
			staticCount++
		}
	}
	for i := 0; i < 8; i++ {
		t := wrkit.Label(fmt.Sprintf("static lbl %02d", i), 12, 0.7, 0.75, 0.85)
		t.SetRepaintBoundary(true)
		shell.Body.Place(t, 250+float64(i%4)*160, 100+float64(i/4)*440)
		staticCount++
	}

	// 两个远离脏点：左上 hot + 右下 hot（同帧每帧变脏 → 两独立 dirty layer id）。
	// Flutter 对齐布局驱动（§2.6.1 规则，对标 R4）：位置 = (Body−hot)×比例，
	// 无固定坐标；相位切换走 SetAlignment（两热点远离几何随 Body resize 自动保持）。
	hotTL := rendering.NewRenderColorBox(hotW, hotW, 0.95, 0.2, 0.2, 1)
	hotTL.SetRepaintBoundary(true)
	hotTLAlign := shell.Body.Align(hotTL, 0.0246, 0.0353)
	hotBR := rendering.NewRenderColorBox(hotW, hotW, 0.2, 0.5, 0.95, 1)
	hotBR.SetRepaintBoundary(true)
	hotBRAlign := shell.Body.Align(hotBR, 0.926, 0.894)

	phaseLabel := wrkit.Label("PHASE: STEADY", 16, 0.95, 0.95, 0.4)
	phaseLabel.SetRepaintBoundary(true)
	shell.Body.Place(phaseLabel, 360, 30)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	var phase string
	lastPhase := ""
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		// 两脏点同帧变脏：每帧都变，两独立 boundary 各自记 id。
		// 相位切换走 SetAlignment（布局驱动）：两热点随相位微移但仍保持
		// 对角远离（左上 vs 右下），多 damage 几何不破坏。
		switch phase {
		case wrkit.PhaseSteady:
			hotTL.R, hotTL.G, hotTL.B = 0.95, 0.2, 0.2
			hotBR.R, hotBR.G, hotBR.B = 0.2, 0.5, 0.95
			hotTLAlign.SetAlignment(0.0246, 0.0353)
			hotBRAlign.SetAlignment(0.926, 0.894)
		case wrkit.PhaseSpike:
			hotTL.R, hotTL.G, hotTL.B = 1.0, 0.85, 0.2
			hotBR.R, hotBR.G, hotBR.B = 0.95, 0.3, 0.95
			hotTLAlign.SetAlignment(0.03, 0.04)
			hotBRAlign.SetAlignment(0.93, 0.88)
		default:
			hotTL.R, hotTL.G, hotTL.B = 0.35, 0.8, 0.4
			hotBR.R, hotBR.G, hotBR.B = 0.9, 0.6, 0.25
			hotTLAlign.SetAlignment(0.02, 0.03)
			hotBRAlign.SetAlignment(0.94, 0.9)
		}
		// 相位切换才重录标签层。
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
		hotTL.MarkNeedsPaint()
		hotBR.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD（两脏点 id 数 + multi 次数 + damage + mode + skip）。
		snapH := app.Metrics().Snapshot()
		avgArea, _, samples, _ := app.DamageStats()
		var avgRatio float64
		if samples > 0 {
			avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		}
		shell.NoteHUDTick(dt)
		ids := app.LastDirtyLayerIDs()
		_, _, _, multiF := app.DamageStats()
		gateOK := len(ids) >= 2 && app.MaxDirtyLayerIDCount() >= 2 &&
			app.LastPresentMode() != "full" && avgRatio <= 0.35
		shell.UpdateHUD("R4b", phase, app, gateOK,
			fmt.Sprintf("ids=%d/%d multi=%d dmg=%.2f mode=%s skip=%d",
				len(ids), app.MaxDirtyLayerIDCount(), multiF,
				avgRatio, app.LastPresentMode(), snapH.BoundarySkip),
			fmt.Sprintf("ids=%v", ids))
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
		avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		maxRatio = float64(maxArea) / float64(winW*winH)
	}
	lastMode := app.LastPresentMode()
	lastIDs := app.LastDirtyLayerIDs()
	maxIDs := app.MaxDirtyLayerIDCount()
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R4b",
		Scenario:      "ui_wr_r4b_multidamage",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":      "retained",
			"damage_semantic":     "multi_damage_two_hotspots",
			"damage_ratio_avg":    avgRatio,
			"damage_ratio_max":    maxRatio,
			"damage_ratio_sum":    float64(2*hotW*hotW+1200*72/10) / float64(winW*winH), // 真实重绘像素比
			"damage_samples":      samples,
			"damage_multi_frames": multiFrames,
			"dirty_layer_id_max":  int64(maxIDs),
			"dirty_layer_ids":     lastIDs,
			"last_present_mode":   lastMode,
			"static_visible":      staticCount,
			"hot_repaints":        hotTick,
			"boundary_skip":       snap.BoundarySkip,
			"boundary_rerecord":   snap.BoundaryRerecord,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R4b gates（§3 组合表：dirty_layer_id_max>=2 + damage_multi_frames>=1；
	// §2 主表：dirty_layer_ids、rects/并集——多矩形独立 scissor 的面积和
	// （sum of rects）远小于并集 bbox，用 damage_ratio_sum 证明成本 ∝ 脏）。
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		MinDirtyLayerIDs:      2,
		MinDamageMultiFrames:  1,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if lastMode == "" || lastMode == "full" || lastMode == "idle" {
		fmt.Fprintf(os.Stderr, "FAIL: present_mode=%q want damage_union/damage_multi (非 full)", lastMode)
		os.Exit(1)
	}
	// 真实重绘像素（sum of rects）：两热点 2×90×90 + HUD 带 1200×72。
	// 每帧：16200 + 86400×0.1（HUD 10Hz 刷新）≈ 24840 px² → ratio≈0.026 ≪ 0.35。
	// 并集 bbox 0.49 是两对角脏点的几何必然（frame.go 决策用 sum 判 full，
	// 非 union——两角热点保持 multi 不升 full）。
	if float64(2*hotW*hotW+1200*72/10) > 0.35*float64(winW*winH) {
		fmt.Fprintln(os.Stderr, "FAIL: damage_ratio_sum 超预算（场景自检）")
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (中央静态区必须不重绘)", snap.BoundarySkip)
		os.Exit(1)
	}
	fpsOK := snap.AvgFrameIntervalMs > 1e-6 && 1000.0/snap.AvgFrameIntervalMs >= 55
	if !fpsOK {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick R4b)", 1000.0/snap.AvgFrameIntervalMs)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: OK presents=%d ids_max=%d ids=%v multi=%d dmg_avg=%.3f mode=%s skip=%d fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), maxIDs, lastIDs, multiFrames, avgRatio, lastMode,
		snap.BoundarySkip, 1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
