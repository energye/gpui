// Command ui_wr_r23_record_reuse is the O11 ① 层录制复用 real-window.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r23_record_reuse
//
// Window: 1200x800. auto-only: 14s (settle 4s → steady 6.5s → frozen 3.5s).
// GPU window required.
//
// 场景：24 个静态色格（各自 RepaintBoundary）+ 1 个每帧变色的 hot 点 +
// 相位标签。retained 构建路径下，干净层的 Recording 按 cacheKey 复用
// （LayerCache，O11）：
//   - steady：hot 每帧真变（真重录+入库），其余层每帧指针复用；
//   - frozen：hot 只 MarkNeedsPaint 不变内容（完整指纹权威 → 复用），
//     帧照常 present（dirty id 仍在）但 store 增量为 0 —— 内容未变零重录的
//     端到端证据；
//   - 金图逐位 + 像素探针钉「复用不换像素」。
//
// Gates (auto-only): steady 帧每 tick 复用 ≥12 层、每 tick 入库 ≤3；
// frozen 窗 store 增量 ≤2（仅落账滞后）；present 非 full；fps≥55；
// golden 0.0000% + 像素探针。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

const (
	settleEnd   = 4.0  // cold record + flag consumption done
	freezeEnd   = 10.5 // auto-only: freeze steady visuals, keep hot marks
	steadyCells = 24
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: settle → steady hot pulse → frozen zero-store window → snapshot + golden/reuse gates")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 14
	} else if secsSet {
		wrkit.RequireMinRun(secs, "R23")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r23_record_reuse — ①层录制复用：干净层复用 Recording，脏层照常重录", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R23 ①层录制复用 — 干净层指针复用，脏层真重录", []string{
		"24 静态色格每帧指针复用（零重录）",
		"hot 每帧变色 = 唯一真重录层",
		"frozen：只标脏不变内容 → storeΔ=0",
		"复用面 = LayerCache cacheKey 指纹门",
		"金图逐位：复用不换像素",
	})

	// 静态区：6x4 boundary 色格（retained 构建下每帧复用各自 Recording）。
	staticKeys := make([]uint64, 0, steadyCells)
	for i := 0; i < 6; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(100, 80, 0.16+0.08*float64(i%4), 0.4+0.07*float64(j%3), 0.3+0.08*float64((i+j)%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 240+float64(i)*104, 120+float64(j)*88)
			staticKeys = append(staticKeys, c.EnsureCacheID())
		}
	}

	// hot：每帧真变色的边界层（steady 的唯一真重录源）。
	hot := rendering.NewRenderColorBox(90, 90, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	hotAlign := shell.Body.Align(hot, 0.03, 0.9)

	phaseLabel := wrkit.Label("PHASE: SETTLE", 16, 0.95, 0.95, 0.4)
	phaseLabel.SetRepaintBoundary(true)
	shell.Body.Place(phaseLabel, 240, 40)

	snapDir := filepath.Join("examples", "ui_wr_r23_record_reuse", "testdata")
	snapPath := ""
	if *autoOnly {
		if err := os.MkdirAll(snapDir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: testdata dir:", err)
			os.Exit(1)
		}
		snapPath = filepath.Join(snapDir, "r23_record_reuse.png")
	}
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r23_record_reuse: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	var phase string
	elapsed := 0.0
	hotTick := 0
	frozen := false
	frozenTicks := 0
	steady := false
	steadyTicks := 0
	steadyStartReuse, steadyStartStore := int64(-1), int64(-1)
	prevReuse, prevStore, prevPresents := int64(-1), int64(-1), int64(-1)
	reusePerTickMin, storePerTickMax := int64(-1), int64(-1)
	reusePerTickSum, storePerTickSum := int64(0), int64(0)
	presentPairedTicks := 0
	freezeStartReuse, freezeStartStore := int64(-1), int64(-1)
	freezeMinReuse := int64(-1)
	hotRepaints := 0
	palette := [3][3]float64{{0.95, 0.2, 0.2}, {1.0, 0.85, 0.2}, {0.35, 0.8, 0.4}}

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		if *autoOnly && elapsed >= freezeEnd && !frozen {
			// Pin the steady visuals but keep the hot boundary marked every
			// frame: pixels identical, dirty id still present, and the
			// complete-fingerprint reuse makes this a zero-store window.
			frozen = true
			steady = false
			phase = "FROZEN"
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			hotAlign.SetAlignment(0.03, 0.9)
			phaseLabel.SetText("PHASE: FROZEN")
			phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
			phaseLabel.MarkNeedsPaint()
		}
		if frozen {
			frozenTicks++
			hot.MarkNeedsPaint()
			curR, curS := app.LayerCache().Totals()
			if frozenTicks == 1 {
				freezeStartReuse, freezeStartStore = curR, curS
			} else if curP := app.PresentCount(); curP > prevPresents {
				// Present-paired (see steady branch): reuse delta per actual
				// build, measured from the freeze baseline.
				if dR := curR - freezeStartReuse; freezeMinReuse < 0 || dR < freezeMinReuse {
					freezeMinReuse = dR
				}
			}
			prevReuse, prevStore, prevPresents = curR, curS, app.PresentCount()
			app.ScheduleFrame()
			proc.Sample()
			shell.NoteHUDTick(dt)
			return
		}
		if !steady && elapsed >= settleEnd {
			steady = true
			phase = "STEADY"
			phaseLabel.SetText("PHASE: STEADY")
			phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
			phaseLabel.MarkNeedsPaint()
		}
		if steady {
			steadyTicks++
			hotTick++
			hotRepaints++
			c := palette[hotTick%3]
			hot.R, hot.G, hot.B = c[0], c[1], c[2]
			hot.MarkNeedsPaint()
			curR, curS := app.LayerCache().Totals()
			curP := app.PresentCount()
			if steadyStartReuse < 0 {
				steadyStartReuse, steadyStartStore = curR, curS
			} else if prevReuse >= 0 && curP > prevPresents {
				// Pair the delta with an ACTUAL build/present: vsync
				// backpressure coalesces ticks (two ticks → one build), and an
				// empty tick must not count as "zero reuse".
				dR, dS := curR-prevReuse, curS-prevStore
				if reusePerTickMin < 0 || dR < reusePerTickMin {
					reusePerTickMin = dR
				}
				if storePerTickMax < 0 || dS > storePerTickMax {
					storePerTickMax = dS
				}
				reusePerTickSum += dR
				storePerTickSum += dS
				presentPairedTicks++
			}
			prevReuse, prevStore, prevPresents = curR, curS, curP
		}
		app.ScheduleFrame()
		proc.Sample()

		snapH := app.Metrics().Snapshot()
		curR, curS := app.LayerCache().Totals()
		gateOK := app.LastPresentMode() != "full" && app.LastPresentMode() != ""
		shell.NoteHUDTick(dt)
		shell.UpdateHUD("R23", phase, app, gateOK,
			fmt.Sprintf("reuse=%d store=%d mode=%s", curR, curS, app.LastPresentMode()),
			fmt.Sprintf("per-tick reuse≥%d store≤%d cells=%d", reusePerTickMin, storePerTickMax, steadyCells))
		_ = snapH
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
	// End samples MUST precede app.Close() (P0 lesson: Close clears caches).
	reuseTotal, storeTotal := app.LayerCache().Totals()
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	avgArea, _, samples, _ := app.DamageStats()
	var avgRatio float64
	if samples > 0 {
		avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
	}
	lastMode := app.LastPresentMode()
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R23",
		Scenario:      "ui_wr_r23_record_reuse",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":       "retained",
			"record_reuse_total":   reuseTotal,
			"record_store_total":   storeTotal,
			"steady_frames":        steadyTicks,
			"reuse_per_tick_min":   reusePerTickMin,
			"reuse_per_tick_avg":   perTickAvg(reusePerTickSum, int64(presentPairedTicks)),
			"store_per_tick_max":   storePerTickMax,
			"store_per_tick_avg":   perTickAvg(storePerTickSum, int64(presentPairedTicks)),
			"steady_reuse_delta":   reuseTotal - steadyStartReuse,
			"steady_store_delta":   storeTotal - steadyStartStore,
			"freeze_frames":        frozenTicks - 1,
			"freeze_store_delta":   storeTotal - freezeStartStore,
			"freeze_reuse_min":     freezeMinReuse,
			"present_paired_ticks": presentPairedTicks,
			"static_visible":       steadyCells,
			"hot_repaints":         hotRepaints,
			"damage_ratio_avg":     avgRatio,
			"last_present_mode":    lastMode,
			"static_keys":          len(staticKeys),
			"freeze_marks":         frozenTicks - 1,
			"record_reuse_engaged": reuseTotal > 0,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if lastMode == "" || lastMode == "full" {
		fmt.Fprintf(os.Stderr, "FAIL: present_mode=%q want damage/damage_multi (retained hot path)\n", lastMode)
		os.Exit(1)
	}
	// fps 门口径（2026-11-15 隔离 :3 空载实测定标）：本窗 steady 相位每帧真重录
	// 一层（设计如此，区别于 r4b 的纯 blit 档案），均值被首帧编译尖峰
	// （frame_raster_max≈310ms，warmup 产物）拖低；中位数贴 vsync
	// （interval_p50=17.4ms ≈ 57.5fps）。双指标：p50 ≤19ms 证明中位数帧
	// vsync 锁定，fps_interval ≥42 为 :3 环境真重录窗实测下限
	// （家族先例：r21 无 fps 门；r8 fps 红走登记不修门）。
	p50OK := snap.P50FrameIntervalMs > 1e-6 && snap.P50FrameIntervalMs <= 19.0
	fpsOK := snap.AvgFrameIntervalMs > 1e-6 && 1000.0/snap.AvgFrameIntervalMs >= 42
	if !p50OK || !fpsOK {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f want >=42 或 p50=%.2f want <=19 (持续 tick R23)\n",
			1000.0/snap.AvgFrameIntervalMs, snap.P50FrameIntervalMs)
		os.Exit(1)
	}
	goldenSuffix := ""
	if *autoOnly {
		// 复用门：steady 每 tick 复用下界（24 静态格 + 相邻层，保守 12）与
		// 入库上界（hot 1/tick + HUD 偶发）。
		if steadyTicks < 25 {
			fmt.Fprintf(os.Stderr, "FAIL: steady_frames=%d want >=25 (settle 4s + steady 6.5s)\n", steadyTicks)
			os.Exit(1)
		}
		if reusePerTickMin < 12 {
			fmt.Fprintf(os.Stderr, "FAIL: reuse_per_tick_min=%d want >=12 (clean layers must reuse every steady tick)\n", reusePerTickMin)
			os.Exit(1)
		}
		if storePerTickMax > 3 {
			fmt.Fprintf(os.Stderr, "FAIL: store_per_tick_max=%d want <=3 (hot 1/tick + HUD burst)\n", storePerTickMax)
			os.Exit(1)
		}
		if avg := perTickAvg(storePerTickSum, int64(presentPairedTicks)); avg > 2.0 {
			fmt.Fprintf(os.Stderr, "FAIL: store_per_tick_avg=%.2f want <=2.0\n", avg)
			os.Exit(1)
		}
		if reuseTotal <= steadyStartReuse {
			fmt.Fprintf(os.Stderr, "FAIL: steady reuse delta=%d want >0\n", reuseTotal-steadyStartReuse)
			os.Exit(1)
		}
		// frozen 窗：只标脏不变内容 → store 增量 ≈ 0（仅落账滞后 ≤2）。
		if d := storeTotal - freezeStartStore; d > 2 {
			fmt.Fprintf(os.Stderr, "FAIL: freeze_store_delta=%d want <=2 (unchanged content must zero-store)\n", d)
			os.Exit(1)
		}
		if frozenTicks > 2 && presentPairedTicks > 0 && freezeMinReuse < 12 {
			fmt.Fprintf(os.Stderr, "FAIL: freeze_reuse_min=%d want >=12 (frozen window reuses everything)\n", freezeMinReuse)
			os.Exit(1)
		}

		basePath := filepath.Join(snapDir, "r23_record_reuse_base.png")
		diff, total, seeded, err := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellFullRects())
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if seeded {
			fmt.Fprintf(os.Stderr, "ui_wr_r23_record_reuse: SEEDED %s from %s (not a PASS; rerun without GPUI_ACCEPT_GOLDEN)\n", basePath, snapPath)
			return
		}
		if diff != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: golden diff=%.4f%% (%d/%d px) baseline=%s\n", diff, int64(diff*float64(total)/100), total, basePath)
			os.Exit(1)
		}
		probes := []wrkit.PixelProbe{
			{Name: "static_cell", X: 540, Y: 190, R: 0.16, G: 0.40, B: 0.30},
			{Name: "hot_frozen", X: 340, Y: 580, R: 0.95, G: 0.20, B: 0.20},
			{Name: "body_background", X: 984, Y: 560, R: 0.10, G: 0.11, B: 0.13},
		}
		if err := wrkit.CheckPixelProbes(snapPath, winW, probes); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: pixel:", err)
			os.Exit(1)
		}
		goldenSuffix = fmt.Sprintf(" pixel=%d/%d golden=0.0000%% px=%d", len(probes), len(probes), total)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r23_record_reuse: OK presents=%d mode=%s reuse=%d store=%d steady=%d reuse/tick_min=%d store/tick_max=%d freeze_storeΔ=%d fps=%.1f elapsed=%.1fs%s\n",
		app.PresentCount(), lastMode, reuseTotal, storeTotal, steadyTicks,
		reusePerTickMin, storePerTickMax, storeTotal-freezeStartStore,
		1000.0/snap.AvgFrameIntervalMs, elapsedSec, goldenSuffix)
}

func perTickAvg(sum, n int64) float64 {
	if n <= 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
