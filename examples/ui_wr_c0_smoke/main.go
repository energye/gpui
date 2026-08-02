// Command ui_wr_c0_smoke is the W0 C0 composite real-window: R0+R12+R16 集成.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
// C0 = 集成窗，只做集成，不能代替 R0/R12/R16 单窗（§3 组合表）。
// 集成验证点：开机即见静+动（首帧内容不黑）+ FullPaint 静态每帧存活 +
// 指标全族字段齐 + policy/presents/首帧。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

// §2.2.1 A–J 全族必采字段 JSON key 清单（与 R12 单窗同源：真源 §2.2.1）。
var fullFamilyKeys = []string{
	// A 帧时
	"fps_wall", "interval_avg_ms", "interval_p50_ms", "interval_p95_ms", "interval_p99_ms",
	"hitch_count", "hitch_rate_per_min", "vsync_source", "target_hz",
	// B 管线
	"frame_build_ms", "frame_raster_ms", "pipeline_depth", "pipeline_max",
	// C 脏区
	"layout_count", "paint_count", "damage_area_px", "damage_ratio", "present_mode", "present_policy",
	// D CPU
	"cpu_pct_avg", "cpu_ui_pct", "cpu_raster_pct",
	// E 内存
	"rss_start_kb", "rss_end_kb", "rss_peak_kb", "rss_slope_kb_per_min", "rss_after_close_kb",
	// F GPU
	"gpu_ops", "cpu_fallback_ops", "last_cpu_fallback", "frame_flushes",
	// G 图/文 (有 measure 数据 → 必采 hit/miss)
	"measure_cache_hit", "measure_cache_miss",
	// H 启动
	"warmup",
	// 外壳
	"ability_id", "scenario", "present_count", "frame_count", "elapsed_sec", "fps_interval", "ability_extra",
}

func checkAllKeys(raw []byte) []string {
	s := string(raw)
	var missing []string
	for _, k := range fullFamilyKeys {
		if !strings.Contains(s, `"`+k+`"`) {
			missing = append(missing, k)
		}
	}
	return missing
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "C0")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c0_smoke — R0+R12+R16 集成"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C0 R0+R12+R16 集成 — 开机即完整可用", []string{
		"R0: 静网+文+动 同屏, full_paint 每帧存活",
		"R12: 指标全族字段齐 (42 键核对)",
		"R16: 首帧 WarmUp, 开机即见内容",
		"集成窗只验证集成, 不代替单窗",
	})

	// R0 特征：密集静态网格 + 静态文（FullPaint 静态存活验证对象）。
	staticCount := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(84, 84, 0.15+0.1*float64(i%4), 0.5, 0.3+0.12*float64(j%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*92, 20+float64(j)*92)
			staticCount++
		}
	}
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("c0 static %02d", i), 12, 0.65+0.05*float64(i%5), 0.72, 0.82)
		shell.Body.Place(t, 600+float64(i%4)*95, 20+float64(i/4)*30)
		staticCount++
	}
	// R0 特征：HOT 动块 + 相位横幅。
	hot := rendering.NewRenderColorBox(120, 120, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 600, 140)
	hotX, hotY := 600.0, 140.0
	phaseLabel := wrkit.Label("PHASE: STEADY", 20, 0.95, 0.95, 0.4)
	shell.Body.Place(phaseLabel, 600, 96)
	// R12 特征：SCHEMA 字段滚动大字。
	schemaBig := wrkit.Label(fmt.Sprintf("SCHEMA: %d FIELDS", len(fullFamilyKeys)), 22, 0.4, 0.95, 0.6)
	shell.Body.Place(schemaBig, 600, 420)
	// R16 特征：首帧横幅（开机即见）。
	firstBanner := wrkit.Label("FIRST FRAME CONTENT OK", 20, 0.4, 0.95, 0.6)
	shell.Body.Place(firstBanner, 600, 380)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: close (%s)\n", win.Backend())
			}
		},
	})

	var phase string
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		switch phase {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			hotX, hotY = 600, 140
			phaseLabel.SetText("PHASE: STEADY")
			phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotX, hotY = 600, 260
			phaseLabel.SetText("PHASE: SPIKE")
			phaseLabel.SetColor(1.0, 0.8, 0.2, 1)
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			hotX, hotY = 680, 140
			phaseLabel.SetText("PHASE: RECOVER")
			phaseLabel.SetColor(0.2, 0.8, 1.0, 1)
		}
		phaseLabel.MarkNeedsPaint()
		shell.Body.Box.Place(hot, hotX, hotY)
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		// R12 特征：SCHEMA 大字实时滚动字段名。
		schemaBig.SetText(fmt.Sprintf("SCHEMA: %d FIELDS · %s", len(fullFamilyKeys),
			fullFamilyKeys[(hotTick/6)%len(fullFamilyKeys)]))
		schemaBig.MarkNeedsPaint()

		// U18: live HUD（集成状态：首帧 + paint 覆盖）。
		snapH := app.Metrics().Snapshot()
		shell.NoteHUDTick(dt)
		gateOK := snapH.Warmup && snapH.FirstPresentPaintCount > 0
		shell.UpdateHUD("C0", phase, app, gateOK,
			fmt.Sprintf("paint=%d t2f=%.0fms", snapH.PaintCount, snapH.TimeToFirstPresentMs), "")
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
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C0",
		Scenario:      "ui_wr_c0_smoke",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"integrated":                []string{"R0", "R12", "R16"},
			"present_policy":            "full_paint",
			"static_visible":            staticCount,
			"hot_repaints":              hotTick,
			"warmup_observed":           snap.Warmup,
			"time_to_first_present_ms":  snap.TimeToFirstPresentMs,
			"first_present_paint_count": snap.FirstPresentPaintCount,
			"family_keys_checked":       len(fullFamilyKeys),
			"first_frame_semantic":      "full_clear_full_paint",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// C0 gates: 集成验证点（§3 组合表）。
	//  1) policy=full_paint（R0 FullPaint 语义）
	//  2) 首帧内容不黑（R16：warmup 观测 + first_present_paint_count>0）
	//  3) 静态每帧存活（R0：paint_count ≈ presents，full_paint 全树重画）
	//  4) 指标全族字段齐（R12：42 键核对）
	//  5) 持续 tick fps（正确性类 ≥55）
	presents := float64(app.PresentCount())
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.PresentPolicy != scheduler.PresentPolicyFullPaint {
		fmt.Fprintf(os.Stderr, "FAIL: present_policy=%q want full_paint", snap.PresentPolicy)
		os.Exit(1)
	}
	if !snap.Warmup || snap.FirstPresentPaintCount <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 首帧内容不黑 (warmup=%v first_paint=%d)", snap.Warmup, snap.FirstPresentPaintCount)
		os.Exit(1)
	}
	fullPaintOK := snap.PaintCount > 0 && (presents == 0 || float64(snap.PaintCount) >= presents*0.9)
	if !fullPaintOK {
		fmt.Fprintf(os.Stderr, "FAIL: paint_count=%d not covering frames=%.0f (静态每帧存活)", snap.PaintCount, presents)
		os.Exit(1)
	}
	missing := checkAllKeys(raw)
	if err := wrgate.CheckSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: §2.2.1 full-family keys missing: %v\n", missing)
		os.Exit(1)
	}
	fpsOK := snap.AvgFrameIntervalMs > 1e-6 && 1000.0/snap.AvgFrameIntervalMs >= 55
	if !fpsOK {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick C0)", 1000.0/snap.AvgFrameIntervalMs)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: OK presents=%d paint=%d statics=%d t2f=%.0fms warmup=%v fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), snap.PaintCount, staticCount, snap.TimeToFirstPresentMs, snap.Warmup,
		1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
