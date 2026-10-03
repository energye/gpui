// Command ui_render_pelican 是渲染层 2D 场景动画演示真窗：
// 按 pelican-bike.html 参考实现 1:1 移植「鹈鹕骑自行车」动画——
// 天空渐变 + 旋转光芒太阳 + 三朵漂移云 + 两只扇翅飞鸟 + 远山/树木/车道线/草丛
// 四层视差滚动 + 辐条车轮 + 曲柄踏频 + 双腿两段式逆运动学踩踏 + 鹈鹕身体起伏、
// 扇翅、围巾飘动、后轮扬尘，右下角暂停/调速控制条（空格 / ↑↓ 可控）。
//
// 同步模型与参考实现一致：环境时钟 t 驱动 CSS/SMIL 类动画（云/鸟/太阳/围巾/
// 翅膀/扬尘），骑行时钟驱动视差/车轮/曲柄/腿/身体起伏；暂停只冻结骑行时钟，
// 环境动画继续（与浏览器里 CSS 动画不受 JS 暂停影响的行为一致）。
//
// 多窗：经 ui/application 标准壳跑（每窗独立 scene + 独立循环，共享一块 GPU
// 设备）；右下调速条右端的 + 钮点一下多开一扇同样的窗，开到显存满为止
// （开不出时报一句人话，已有的窗不受影响）。
//
// 运行（需 GPU 窗口）：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_render_pelican              # 不限时长，关窗退出
//	RUN_SECONDS=15 go run ./examples/ui_render_pelican   # 定时退出
//	PELICAN_WINDOWS=2 RUN_SECONDS=12 go run ./examples/ui_render_pelican # 预开两窗同跑
//	SNAPSHOT=tmp/pelican_shot.png RUN_SECONDS=5 go run ./examples/ui_render_pelican
//
// 指标：结束打印 JSON（fps / p50 / p95 / hitch / gpu_ops / cpu_fallback / rss），
// 门禁：有帧即过；RUN_SECONDS>=10 时主窗 fps>=30。
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/ui/application"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 700 // 默认窗口 = SVG 舞台 1:1

// pelicanWins 给后开的窗编号（主窗是 #1）。
var pelicanWins atomic.Int64

func pelicanTitle(n int64) string {
	if n <= 1 {
		return "gpui ui_render_pelican — 鹈鹕骑行 · Pelican Rider"
	}
	return fmt.Sprintf("gpui ui_render_pelican #%d — 鹈鹕骑行 · Pelican Rider", n)
}

// runWindowCount PELICAN_WINDOWS=预开窗数（默认 1；>1 时启动即多窗同跑，
// 走的与 + 钮同一条 SpawnWindow 路，用于自动化验证多窗并发呈现）。
func runWindowCount() int {
	if v := os.Getenv("PELICAN_WINDOWS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 1 {
			return n
		}
	}
	return 1
}

// openPelicanWindow 开一扇鹈鹕窗：独立 scene + 独立循环，内容接线（改尺寸、
// 按键、指针、+ 钮开窗）经窗口私有观察者走，不经过 App 全局回调。
func openPelicanWindow(app *application.App, title string) (*application.Window, error) {
	sc := newPelicanScene(winW, winH)
	w, err := app.SpawnWindow(application.WindowOptions{
		Width:       winW,
		Height:      winH,
		Title:       title,
		Decorations: os.Getenv("PELICAN_NODECOR") != "1", // PELICAN_NODECOR=1: 无 CSD 单 surface（合成路径对照）
		Resizable:   true,
	}, sc.Root)
	if err != nil {
		return nil, err
	}
	w.SetEventObserver(func(ev platform.Event) {
		switch ev.Type {
		case platform.EventResize:
			if ev.Width > 0 && ev.Height > 0 {
				sc.onResize(float64(ev.Width), float64(ev.Height))
			}
		case platform.EventKey:
			sc.onKey(ev)
		case platform.EventPointer:
			sc.onPointer(ev)
		case platform.EventClose:
			fmt.Fprintf(os.Stderr, "ui_render_pelican: close %q (%s)\n", title, w.Platform().Backend())
		}
	})
	pipe := w.Pipeline()
	pipe.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		sc.onTick(dt)
		pipe.ScheduleFrame()
	}})
	pipe.Scheduler().SetMode(scheduler.ModePersistent)
	// retained 呈现（R4）：持续动画只重画脏层，raster ~16→5ms——为块2
	// vsync 在途门控腾帧预算（raster 超过半个刷新周期时互斥会掉帧）。
	pipe.SetPresentPolicy(scheduler.PresentPolicyRetained)
	sc.OnOpenWindow = func() {
		n := pelicanWins.Add(1)
		if _, err := openPelicanWindow(app, pelicanTitle(n)); err != nil {
			fmt.Fprintf(os.Stderr, "ui_render_pelican: open window #%d: %v (close windows or free GPU memory and retry)\n", n, err)
		} else {
			fmt.Fprintf(os.Stderr, "ui_render_pelican: opened window #%d\n", n)
		}
	}
	return w, nil
}

func main() {
	secs := runSeconds(0) // 0 = 不限时，关窗退出
	if secs <= 0 {
		if os.Getenv("SNAPSHOT") != "" {
			secs = 5 // 快照模式给个默认时限
		}
	}
	fmt.Fprintf(os.Stderr, "ui_render_pelican: 鹈鹕骑行真窗 — %s（RUN_SECONDS 可调，0=关窗退出）\n", runLabel(secs))

	// 离屏真值模式：GOLDEN_PNG=path 时不开窗，用软件光栅渲染 PELICAN_T
	// 时刻的单帧并保存（用于与 GPU 上屏内容做逐像素对比诊断）。
	if gp := os.Getenv("GOLDEN_PNG"); gp != "" {
		newPelicanScene(winW, winH).renderGolden(gp)
		return
	}

	app := application.New(application.Config{
		Name:    "gpui ui_render_pelican",
		Backend: runBackend(),
		// 与天空顶色同族，仅冷启动首帧兜底
		ClearR: 0.47, ClearG: 0.78, ClearB: 0.96, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: os.Getenv("SNAPSHOT"), // 单窗验证用；多窗同跑时以主窗为准
	})

	pelicanWins.Store(1)
	mainWin, err := openPelicanWindow(app, pelicanTitle(1))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	// 预开多窗（与 + 钮同一条 SpawnWindow 路）：启动即多窗同跑。
	for i := 2; i <= runWindowCount(); i++ {
		pelicanWins.Store(int64(i))
		if _, err := openPelicanWindow(app, pelicanTitle(int64(i))); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
			os.Exit(1)
		}
	}

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	app.Close()

	elapsed := time.Since(t0).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	stepDiagReport()
	mainPipe := mainWin.Pipeline()
	presents := mainPipe.PresentCount()
	fps := float64(presents) / elapsed

	m := mainPipe.Metrics().Snapshot()
	fmt.Fprintf(os.Stderr, "ui_render_pelican: backend=%s windows=%d presents=%d ~%.1f fps\n",
		mainWin.Platform().Backend(), app.WindowCount(), presents, fps)
	fmt.Fprintf(os.Stderr, "ui_render_pelican: avg=%.2f p50=%.2f p95=%.2f p99=%.2f hitches=%d gpu_ops=%d cpu_fallback_ops=%d\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P95FrameIntervalMs, m.P99FrameIntervalMs,
		m.HitchCount, m.GPUOps, m.CPUFallbackOps)

	// 门禁：定时长跑需稳定 30fps+；不限时/短跑只要求有帧。
	if secs >= 10 && fps < 30 {
		fmt.Fprintf(os.Stderr, "FAIL: fps too low ~%.1f (<30 over %ds)\n", fps, secs)
		os.Exit(1)
	}

	if b, err := mainPipe.Metrics().JSON(); err == nil {
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

// runBackend GPUI_BACKEND=x11|wayland 可强制显示后端（默认自动检测）。
func runBackend() platform.DisplayBackend {
	switch os.Getenv("GPUI_BACKEND") {
	case "x11":
		return platform.DisplayX11
	case "wayland":
		return platform.DisplayWayland
	}
	return platform.DisplayAuto
}

func runLabel(secs int) string {
	if secs <= 0 {
		return "不限时"
	}
	return fmt.Sprintf("%ds", secs)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
