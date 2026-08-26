// Command ui_render_particle 是渲染层粒子特效演示真窗：
// 用 UI 渲染管线（RenderObject 树 + GPU Present）演示传奇类游戏三类法术特效
// ——火球术（同心盘立体球 + 径向渐变光晕 + 拖尾火花）、冰霜新星（扩散环 + 放射冰晶 + 环绕粒子）、
// 魔法旋涡（螺旋粒子群 + 中心紫光）+ 星空背景。
//
// 运行（需 GPU 窗口）：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_particle          # 默认 20s，可调
//	SNAPSHOT=tmp/particle_shot.png RUN_SECONDS=5 go run ./examples/ui_render_particle  # 结束时读回一帧 PNG
//
// 指标：结束打印 JSON（fps / p50 / p95 / hitch / gpu_ops / cpu_fallback / rss / cpu），
// 门禁：有帧即过；RUN_SECONDS>=10 时 fps>=30（粒子满负荷线）。
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs := runSeconds(20)
	fmt.Fprintf(os.Stderr, "ui_render_particle: 粒子特效真窗 — %ds（RUN_SECONDS 可调）\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_render_particle — 火球 / 冰环 / 旋涡 / 星光",
		Decorations: true,
		Resizable:   true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}

	sc := newParticleScene(winW, winH)
	host := win.Host()

	snapshot := os.Getenv("SNAPSHOT")

	app := embedder.NewPipelineApp(host, sc.Root, embedder.PipelineOptions{
		ClearR: 0.04, ClearG: 0.05, ClearB: 0.09, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapshot,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_render_particle: close (%s)\n", win.Backend())
			}
		},
	})

	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		sc.onTick(dt)
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

	fmt.Fprintf(os.Stderr, "ui_render_particle: backend=%s presents=%d ~%.1f fps\n",
		win.Backend(), presents, fps)
	fmt.Fprintf(os.Stderr, "ui_render_particle: avg=%.2f p50=%.2f p95=%.2f p99=%.2f hitches=%d hitch_rate=%.2f/min vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P95FrameIntervalMs, m.P99FrameIntervalMs,
		m.HitchCount, m.HitchRatePerMin, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_particle: rss start=%d end=%d peak=%d after_close=%d KB slope=%.1f KB/min cpu=%.1f%% (ui=%.1f raster=%.1f)\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, rssSlopePerMin, m.CPUPctAvg, m.CPUUIPct, m.CPURasterPct)
	fmt.Fprintf(os.Stderr, "ui_render_particle: gpu_ops=%d cpu_fallback_ops=%d last_cpu_fb=%q frame_flushes=%d damage_px=%d mode=%s\n",
		m.GPUOps, m.CPUFallbackOps, m.LastCPUFallbackReason, m.FrameFlushes, m.DamageAreaPx, m.PresentMode)

	// 门禁：满负荷粒子线——长跑需稳定 30fps+；短跑只要求有帧。
	if secs >= 10 && fps < 30 {
		fmt.Fprintf(os.Stderr, "FAIL: fps too low ~%.1f (<30 over %ds)\n", fps, secs)
		os.Exit(1)
	}

	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
	}
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
