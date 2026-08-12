// Command ui_wr_r12_metrics is the W0 R12 real-window: 帧指标字段完备 (schema).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r12_metrics
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
// gate=schema_only (R12 唯一允许): 硬 FAIL = §2.2.1 A–J 全族字段必须全部存在,
// 且仍须真窗 Present (present_count>=1)。fps 不做硬门禁 (非 R12 主判), 但如实输出。
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

// fullFamilyKeys is the §2.2.1 A–J 全族必采字段 JSON key list (R12 核对真源,
// 比 wrgate.RequiredSchemaKeys 更全: 含 A 的 interval_avg/p99、B 的 pipeline_*、
// E 的 rss_after_close_kb、F 的 last_cpu_fallback/frame_flushes、G 的 measure_*、
// H 的 warmup)。缺失任何一个 → FAIL。
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
	// G 图/文 (本窗有 measure 数据 → 必采 hit/miss)
	"measure_cache_hit", "measure_cache_miss",
	// H 启动
	"warmup",
	// 外壳
	"ability_id", "scenario", "present_count", "frame_count", "elapsed_sec", "fps_interval", "ability_extra",
}

// checkAllKeys verifies every family key exists in the marshaled JSON.
func checkAllKeys(raw []byte) []string {
	s := string(raw)
	var missing []string
	for _, k := range fullFamilyKeys {
		needle := `"` + k + `"`
		if !strings.Contains(s, needle) {
			missing = append(missing, k)
		}
	}
	return missing
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R12")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r12_metrics — 指标字段完备"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R12 帧指标字段完备 — §2.2.1 A–J 全族", []string{
		"gate=schema_only (R12 唯一允许)",
		"全族字段缺失任一 → FAIL",
		"真窗仍须 Present (present_count>=1)",
		"fps 如实输出, 不做硬门禁",
	})

	// 轻量静态内容 + 动块 (真窗 Present 场景, 非空窗)。
	staticCount := 0
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(70, 70, 0.2+0.1*float64(i%3), 0.45, 0.35+0.1*float64(j%2), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*80, 20+float64(j)*80)
			staticCount++
		}
	}
	for i := 0; i < 8; i++ {
		t := wrkit.Label(fmt.Sprintf("schema text %02d", i), 12, 0.7, 0.75, 0.85)
		shell.Body.Place(t, 360+float64(i%4)*90, 20+float64(i/4)*28)
		staticCount++
	}
	hot := rendering.NewRenderColorBox(100, 100, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 360, 120)
	hotX, hotY := 360.0, 120.0
	// SCHEMA 大字：字段核对可视化（schema 窗视觉核心）。
	schemaBig := wrkit.Label(fmt.Sprintf("SCHEMA: %d FIELDS", len(fullFamilyKeys)), 26, 0.4, 0.95, 0.6)
	shell.Body.Place(schemaBig, 520, 140)
	schemaSub := wrkit.Label("gate=schema_only · 缺一 FAIL · 真窗 Present", 12, 0.6, 0.65, 0.7)
	shell.Body.Place(schemaSub, 520, 180)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r12_metrics: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
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
			hotX, hotY = 360, 120
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotX, hotY = 360, 240
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			hotX, hotY = 440, 120
		}
		shell.Body.Box.Place(hot, hotX, hotY)
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		// SCHEMA 大字实时滚动核对中的字段名（全族键逐一展示）。
		fieldName := fullFamilyKeys[(hotTick/6)%len(fullFamilyKeys)]
		schemaBig.SetText(fmt.Sprintf("SCHEMA: %d FIELDS · %s", len(fullFamilyKeys), fieldName))
		schemaBig.MarkNeedsPaint()

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("R12", phase, app, snapH.PresentCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()), "")
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
		AbilityID:     "R12",
		Scenario:      "ui_wr_r12_metrics",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"gate":         "schema_only",
			"family_keys":  len(fullFamilyKeys),
			"static_count": staticCount,
			"hot_repaints": hotTick,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R12 gates: schema 全集存在 (wrgate 30 键 + §2.2.1 全族键) + 真窗 Present。
	missing := checkAllKeys(raw)
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents (R12 仍须真窗 Present)")
		os.Exit(1)
	}
	if err := wrgate.CheckSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: §2.2.1 full-family keys missing: %v\n", missing)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r12_metrics: OK schema_only presents=%d fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), 1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
