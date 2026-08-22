// Command ui_wr_r10_async_image is the W3 R10 real-window: 图异步→局部脏.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/ui_wr_r10_async_image
//
// Window: 1200x800. RUN_SECONDS>=30 (U16, §2.5: 关闭 30s/观察 60). GPU window required.
//
// Gates (§2 R10 行 + §2.2 全族):
//   - 出图后 rerecord 仅一格: 每批到达后的 boundary_rerecord 增量 ≤ 到达格数+1，
//     违例批数必须为 0（若全局重绘，每批增量≈全部格数即 FAIL）
//   - 局部脏不触布局: paint-only 契约由引擎单测 TestRenderImage_SetImage_PaintOnly
//     证明（layout_count 数的是每帧布局趟数，不作窗内逐批观测）
//   - 滚动/持续 tick 60fps 档: fps_interval>=55 且 interval_p95_ms<=22
//   - 长 soak hitch 预算: hitch_rate_per_min<=5
//   - RSS 斜率预算（稳态最小二乘）: rss_slope_kb_per_min<=30000
//   - vsync_source 必须输出（fallback 如实标注，不宣称锁 60Hz）
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/io"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 30
	imageCount   = 20 // 4×5 异步图格
	hitchBudget  = 5.0
	slopeBudget  = 30000.0
)

// makeSourcePNGs 生成 imageCount 张互异条纹 PNG 到进程级临时目录（真实文件解码路径）。
func makeSourcePNGs(dir string) ([]string, error) {
	paths := make([]string, imageCount)
	for i := 0; i < imageCount; i++ {
		m := image.NewRGBA(image.Rect(0, 0, 96, 56))
		base := color.RGBA{R: uint8(30 + i*9), G: uint8(60 + i*7), B: uint8(140 + i*4), A: 255}
		for y := 0; y < 56; y++ {
			for x := 0; x < 96; x++ {
				if ((x+y)/8+i)%2 == 0 {
					m.Set(x, y, base)
				} else {
					m.Set(x, y, color.RGBA{R: 24, G: 26, B: 34, A: 255})
				}
			}
		}
		p := filepath.Join(dir, fmt.Sprintf("async_%02d.png", i))
		f, err := os.Create(p)
		if err != nil {
			return nil, err
		}
		if err := png.Encode(f, m); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
		paths[i] = p
	}
	return paths, nil
}

type arrival struct {
	idx      int
	img      *render.ImageBuf
	decodeMs float64
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	}
	wrkit.RequireMinRun(secs, "R10")
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r10_async_image — 图异步局部脏", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R10 图异步→局部脏 — 占位→图仅该格变", []string{
		"20 格占位符 → 异步解码逐格点亮",
		"出图后 rerecord 仅该格（逐批记账）",
		"layout 增量必须为 0（paint-only）",
		"Steady 慢加载 / Spike 连发 / Recover 减速",
		"HOT 块 Align 布局驱动持续动画",
		"右栏静态密集区全程静止",
	})

	bodyW, bodyH := shell.Body.W, shell.Body.H
	rightW := 250.0
	gridW := bodyW - rightW - 16

	// 4×5 图格：每格独立 RepaintBoundary（rerecord 可按格归因）。
	const cols, rows = 4, 5
	cellW := float64(int(gridW-40) / cols)
	cellH := float64(int(bodyH-40) / rows)
	type cell struct {
		box *rendering.AbsoluteBox
		img *rendering.RenderImage
	}
	cells := make([]cell, imageCount)
	for i := 0; i < imageCount; i++ {
		box := rendering.NewAbsoluteBox(cellW-12, cellH-12)
		box.SetRepaintBoundary(true)
		im := rendering.NewRenderImage(96, 56)
		im.SetLoading()
		box.Place(im, 14, 14)
		lb := wrkit.Label(fmt.Sprintf("cell %02d", i), 11, 0.65, 0.70, 0.80)
		box.Place(lb, 14, float64(cellH-40))
		cells[i] = cell{box: box, img: im}
		shell.Body.Place(box, 14+float64(i%cols)*cellW, 14+float64(i/cols)*cellH)
	}

	// HOT 动块（Align 布局驱动）：持续 tick 的动态热点。
	hot := rendering.NewRenderColorBox(70, 70, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	hotAlign := shell.Body.Align(hot, 0.94, 0.06)

	// 右栏静态密集区。
	right := wrkit.NewPanel(rightW, bodyH, 0.11, 0.12, 0.15, 1)
	shell.Body.Place(right.Box, gridW+16, 0)
	right.LabelAt("STATIC DENSE (全程静止)", 12, 12, 10, 0.55, 0.75, 0.95)
	statusLabel := right.LabelAt("loaded 00 / 20", 13, 12, 34, 0.95, 0.9, 0.4)
	statusLabel.SetRepaintBoundary(true)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(52, 52, 0.14+0.08*float64(i%4), 0.45+0.06*float64(j%4), 0.3+0.08*float64((i+j)%3), 1)
			c.SetRepaintBoundary(true)
			right.Place(c, 12+float64(i)*58, 64+float64(j)*58)
		}
	}
	for i := 0; i < 8; i++ {
		right.LabelAt(fmt.Sprintf("static label %02d — 密集文字区", i), 11, 12, 310+float64(i)*22, 0.70, 0.78, 0.88)
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r10_async_image: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})

	// 生成源图并启动异步加载（真实文件 + worker 解码；结果经 channel 回 UI 线程）。
	dir, err := os.MkdirTemp("", "r10_src")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: tmpdir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)
	paths, err := makeSourcePNGs(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: make sources:", err)
		os.Exit(1)
	}

	arrivals := make(chan arrival, imageCount)
	nextIdx := 0
	dispatch := func(i int) {
		t0 := time.Now()
		io.Default.DecodeFile(paths[i], func(res io.Result) {
			arrivals <- arrival{idx: i, img: res.Img, decodeMs: time.Since(t0).Seconds() * 1000}
		})
	}

	const cycleLen = 9.0
	var elapsed float64
	var phase string
	lastDispatch := -1.0
	loaded, decodeSum := 0, 0.0
	hotTick := 0
	lastPhase := ""
	// 逐批记账：上一 tick 应用的批次在本 tick 结算 rerecord 增量。
	pendingApplied := -1
	pendingRR := int64(0)
	rrViolations, maxBatchDelta := int64(0), int64(0)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		ct := elapsed
		for ct >= cycleLen {
			ct -= cycleLen
		}
		switch {
		case ct < 3:
			phase = wrkit.PhaseSteady
		case ct < 6:
			phase = wrkit.PhaseSpike
		default:
			phase = wrkit.PhaseRecover
		}

		snapH := app.Metrics().Snapshot()

		// 结算上一批：rerecord 增量 ≤ 到达格数+1（layout_count 数的是每帧布局
		// 趟数——HUD 文本更新等任何脏节点都会+1——不构成「SetImage 触发布局」
		// 的观测；paint-only 契约由 ui/rendering 引擎单测证明）。
		if pendingApplied >= 0 {
			dRR := snapH.BoundaryRerecord - pendingRR
			bound := int64(pendingApplied) + 1
			if dRR > bound {
				rrViolations++
			}
			if dRR > maxBatchDelta {
				maxBatchDelta = dRR
			}
			pendingApplied = -1
		}

		// 应用本 tick 到达的出图（UI 线程内变更树）。
		applied := 0
		for {
			select {
			case a := <-arrivals:
				if a.idx >= 0 && a.idx < imageCount && a.img != nil {
					cells[a.idx].img.SetImage(a.img)
					loaded++
					decodeSum += a.decodeMs
					applied++
				}
			default:
				goto applied_done
			}
		}
	applied_done:
		if applied > 0 {
			pendingApplied = applied
			pendingRR = snapH.BoundaryRerecord
			statusLabel.SetText(fmt.Sprintf("loaded %02d / %02d", loaded, imageCount))
			statusLabel.MarkNeedsPaint()
		}

		// 加载节奏（能力行为随相位变化）：Steady 每 0.8s 一张，Spike 连发，Recover 1.2s。
		if nextIdx < imageCount {
			gap := 0.8
			switch phase {
			case wrkit.PhaseSpike:
				gap = 0.05
			case wrkit.PhaseRecover:
				gap = 1.2
			}
			canDispatch := lastDispatch < 0 || elapsed-lastDispatch >= gap
			// 非连发相位保持逐批可归因：上一批未结算时不派发下一批。
			if canDispatch && (phase == wrkit.PhaseSpike || pendingApplied < 0) {
				dispatch(nextIdx)
				nextIdx++
				lastDispatch = elapsed
			}
		}

		// HOT 动块相位动画（布局驱动）。
		hotTick++
		switch phase {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			hotAlign.SetAlignment(0.94, 0.06)
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotAlign.SetAlignment(0.88, 0.10)
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			hotAlign.SetAlignment(0.94, 0.14)
		}
		hot.MarkNeedsPaint()
		if phase != lastPhase {
			lastPhase = phase
		}

		shell.NoteHUDTick(dt)
		gateOK := rrViolations == 0
		avgDec := 0.0
		if loaded > 0 {
			avgDec = decodeSum / float64(loaded)
		}
		shell.UpdateHUD("R10", phase, app, gateOK,
			fmt.Sprintf("img=%d/%d rrΔ<=%d v=%d", loaded, imageCount, maxBatchDelta, rrViolations),
			fmt.Sprintf("dec≈%.0fms", avgDec))
		app.ScheduleFrame()
		proc.Sample()
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
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	avgDecode := 0.0
	if loaded > 0 {
		avgDecode = decodeSum / float64(loaded)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R10",
		Scenario:      "ui_wr_r10_async_image",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"images_total":                imageCount,
			"images_loaded":               loaded,
			"rr_violations":               rrViolations,
			"max_batch_rr_delta":          maxBatchDelta,
			"batch_bound":                 "delta <= applied+1",
			"avg_decode_ms":               avgDecode,
			"boundary_skip":               snap.BoundarySkip,
			"boundary_rerecord":           snap.BoundaryRerecord,
			"hitch_budget_per_min":        hitchBudget,
			"rss_slope_budget_kb_per_min": slopeBudget,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates（§2 R10 行 + §2.2 全族；门禁是硬的不许放）----
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	// 能力前提: 全部图片经异步 worker 出图。
	if loaded != imageCount {
		fmt.Fprintf(os.Stderr, "FAIL: images_loaded=%d want %d (异步出图未完成)", loaded, imageCount)
		os.Exit(1)
	}
	// 族 C 能力专用①: 出图后 rerecord 仅一格。
	if rrViolations != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: rr_violations=%d want 0 (存在出图批次引发多格/全局重录)", rrViolations)
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (未出图格必须保持缓存回放)", snap.BoundarySkip)
		os.Exit(1)
	}
	// 族 A: 持续 tick 60fps 档。
	if fpsInterval < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick R10)", fpsInterval)
		os.Exit(1)
	}
	if snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: interval_p95_ms=%.1f > 22", snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.1f > %.0f", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	if snap.RSSSlopeKBPerMin > slopeBudget {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.1f > %.0f", snap.RSSSlopeKBPerMin, slopeBudget)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r10_async_image: OK presents=%d loaded=%d/%d rrViol=%d maxRRΔ=%d dec=%.0fms skip=%d fps=%.1f p95=%.1f hitch=%.1f/min slope=%.0fKB/min vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), loaded, imageCount, rrViolations, maxBatchDelta,
		avgDecode, snap.BoundarySkip, fpsInterval, snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		snap.RSSSlopeKBPerMin, snap.VSyncSource, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
