// Command ui_render_jumprope 是渲染层 2D 协调动画演示真窗：
// 用 UI 渲染管线（RenderObject 树 + GPU Present）实现「两人摇长绳 + 中间一人连续跳绳」：
// 左右摇绳人交替挥臂（反相小圆轨迹），跳绳是每帧重建的单条连续柔性曲线（两端钉在手上，
// 弓形随相位上下翻转），中间小人按同一时钟在绳过脚时起跳、落地续跳，整循环无缝播放。
//
// 同步模型：绳相位、双臂角度、跳跃高度全部由同一个仿真时钟 t 的纯函数导出，
// 不存在各自独立计时——时间同步由构造保证。
// 不穿身：绳按相位做前/后分层绘制（cosθ≥0 绳在人前，否则在人后），
// 翻转发生在弓形极值点（绳远离身体），视觉无跳变。
//
// 运行（需 GPU 窗口）：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_jumprope          # 默认 20s，可调
//	SNAPSHOT=tmp/jumprope_shot.png RUN_SECONDS=5 go run ./examples/ui_render_jumprope
//
// 指标：结束打印 JSON（fps / p50 / p95 / hitch / gpu_ops / cpu_fallback / rss / cpu），
// 门禁：有帧即过；RUN_SECONDS>=10 时 fps>=30。
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs := runSeconds(20)
	fmt.Fprintf(os.Stderr, "ui_render_jumprope: 跳绳协调动画真窗 — %.1fs（RUN_SECONDS 可调）\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_render_jumprope — 双人摇绳 · 中央跳绳",
		Decorations: true,
		Resizable:   true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}

	sc := newJumpropeScene(winW, winH)
	host := win.Host()

	snapshot := os.Getenv("SNAPSHOT")

	app := embedder.NewPipelineApp(host, sc.Root, embedder.PipelineOptions{
		ClearR: 0.62, ClearG: 0.80, ClearB: 0.95, ClearA: 1,
		RunFor:       time.Duration(secs * float64(time.Second)),
		WarmUp:       true,
		SnapshotPath: snapshot,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				sc.onResize(float64(ev.Width), float64(ev.Height))
			}
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_render_jumprope: close (%s)\n", win.Backend())
			}
		},
	})

	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		sc.onTick(dt)

		// LiveHUD：底部指标带（~10Hz 节流刷新）
		snap := app.Metrics().Snapshot()
		fps := 0.0
		if snap.AvgFrameIntervalMs > 1e-6 {
			fps = 1000 / snap.AvgFrameIntervalMs
		}
		pol := snap.PresentPolicy
		if pol == "" {
			pol = scheduler.PresentPolicyFullPaint
		}
		sc.hud.NoteTick(dt)
		sc.hud.Update(wrkit.Snap{
			AbilityID:   "DEMO-JUMPROPE",
			Phase:       "loop",
			FPS:         fps,
			P95Ms:       snap.P95FrameIntervalMs,
			Policy:      pol,
			PresentMode: snap.PresentMode,
			PaintCount:  snap.PaintCount,
			Presents:    app.PresentCount(),
			Core:        "绳相位/双臂/跳跃 同一时钟 t",
			GateOK:      true,
			Extra:       fmt.Sprintf("t=%.1fs 圈数=%d", sc.sim.simT, sc.sim.count),
		})

		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	presents := app.PresentCount()
	if presents < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no frames")
		os.Exit(1)
	}
	m := app.Metrics().Snapshot()
	fps := float64(presents) / elapsed

	rssSlopePerMin := m.RSSSlopeKBPerMin

	fmt.Fprintf(os.Stderr, "ui_render_jumprope: backend=%s presents=%d ~%.1f fps\n",
		win.Backend(), presents, fps)
	fmt.Fprintf(os.Stderr, "ui_render_jumprope: avg=%.2f p50=%.2f p95=%.2f p99=%.2f hitches=%d hitch_rate=%.2f/min vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P95FrameIntervalMs, m.P99FrameIntervalMs,
		m.HitchCount, m.HitchRatePerMin, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_jumprope: rss start=%d end=%d peak=%d after_close=%d KB slope=%.1f KB/min cpu=%.1f%% (ui=%.1f raster=%.1f)\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, rssSlopePerMin, m.CPUPctAvg, m.CPUUIPct, m.CPURasterPct)
	fmt.Fprintf(os.Stderr, "ui_render_jumprope: gpu_ops=%d cpu_fallback_ops=%d last_cpu_fb=%q frame_flushes=%d damage_px=%d mode=%s\n",
		m.GPUOps, m.CPUFallbackOps, m.LastCPUFallbackReason, m.FrameFlushes, m.DamageAreaPx, m.PresentMode)

	// 门禁：有帧即过；长跑需稳定 30fps+（本场景负载远低于粒子窗）。
	if secs >= 10 && fps < 30 {
		fmt.Fprintf(os.Stderr, "FAIL: fps too low ~%.1f (<30 over %.1fs)\n", fps, secs)
		os.Exit(1)
	}

	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
	}
}

func runSeconds(def int) float64 {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return float64(def)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
