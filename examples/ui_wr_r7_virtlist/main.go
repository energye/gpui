// Command ui_wr_r7_virtlist is the W3 R7 real-window: 虚拟化宿主（VirtualList）.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_r7_virtlist
//
// Window: 1200x800. RUN_SECONDS>=60 (U16, §2.5: 关闭 60s/观察 120). GPU window required.
//
// Gates (§2 R7 行 + §2.2 全族):
//   - bind_count ≪ item_count: item_count>=1000 且 bind_count<=64 且 bind*10<=item
//   - 滚动/持续 tick 60fps 档: fps_interval>=55 且 interval_p95_ms<=22
//   - 长 soak hitch 预算: hitch_rate_per_min<=5
//   - RSS 斜率预算: rss_slope_kb_per_min<=30000
//   - vsync_source 必须输出（fallback 如实标注，不宣称锁 60Hz）
//
// 场景（§2.6）: 1200 项变高异构列表（文字行/色块/图片混排）+ 滚动条指示器 +
// 右栏静态密集区（4×4 色格 + 标签，嵌套 boundary）。相位脚本: Steady 慢滚 →
// Spike 快滑 → Recover 减速，循环往复，到顶/底 ping-pong 反向。
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 60 // §2.5 R7 关闭用时长
	itemCount    = 1200
	vpW          = 620.0  // 列表视口宽
	rightW       = 250.0  // 右栏静态密集区宽
	cacheRows    = 2.0    // CacheExtent = 2×fallback 行高
	bindCap      = 64     // 门禁: 视口行 + cache + slack 的硬上界
	hitchBudget  = 5.0    // hitch_rate_per_min 预算（§2.2.2 长 soak 默认）
	slopeBudget  = 30000.0 // rss_slope_kb_per_min 预算（§2.2.4 极端线）
)

// rowHeight 是变高行高（i%4 四类异构 item），VirtualList 前缀和按它建 index↔offset。
func rowHeight(i int) float64 {
	switch i % 4 {
	case 0:
		return 44 // 文字行
	case 1:
		return 92 // 色块对
	case 2:
		return 60 // 文字 + 色签
	default:
		return 76 // 图片 + 文字
	}
}

// makeChip 生成一张进程内条纹图（96×56），所有图片 cell 共享（SetImageShared，
// 不转移所有权，main 结束统一 Dispose）。
func makeChip() *render.ImageBuf {
	m := image.NewRGBA(image.Rect(0, 0, 96, 56))
	for y := 0; y < 56; y++ {
		for x := 0; x < 96; x++ {
			if ((x+y)/10)%2 == 0 {
				m.Set(x, y, color.RGBA{R: 30, G: 160, B: 200, A: 255})
			} else {
				m.Set(x, y, color.RGBA{R: 24, G: 26, B: 34, A: 255})
			}
		}
	}
	return render.ImageBufFromImage(m)
}

// buildCell 构造第 i 行的异构 cell（文字行 / 色块对 / 文字+色签 / 图片+文字）。
func buildCell(i int, w float64, chip *render.ImageBuf) rendering.RenderObject {
	h := rowHeight(i)
	cell := rendering.NewAbsoluteBox(w, h)
	switch i % 4 {
	case 0:
		t := wrkit.Label(fmt.Sprintf("row %04d · text line — 虚拟列表只挂视口 cell", i), 15, 0.85, 0.88, 0.95)
		cell.Place(t, 14, 12)
	case 1:
		base := rendering.NewRenderColorBox(w-28, h-20, 0.16+0.05*float64(i%7), 0.35+0.04*float64(i%5), 0.55, 1)
		base.SetRepaintBoundary(true)
		cell.Place(base, 14, 10)
		inner := rendering.NewRenderColorBox(140, 24, 0.95, 0.55, 0.15, 1)
		cell.Place(inner, 28, 10+(h-20)/2-12)
	case 2:
		t := wrkit.Label(fmt.Sprintf("row %04d · mixed text + chip", i), 14, 0.75, 0.85, 0.75)
		cell.Place(t, 14, 8)
		chipBox := rendering.NewRenderColorBox(72, 18, 0.30+0.06*float64(i%6), 0.65, 0.35, 1)
		cell.Place(chipBox, w-100, 30)
	default:
		im := rendering.NewRenderImage(96, 56)
		im.SetImageShared(chip)
		cell.Place(im, 14, 10)
		t := wrkit.Label(fmt.Sprintf("row %04d · image cell", i), 13, 0.80, 0.82, 0.90)
		cell.Place(t, 122, 28)
	}
	return cell
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds // 未设 RUN_SECONDS → 默认关闭用值
	}
	wrkit.RequireMinRun(secs, "R7")
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r7_virtlist — 虚拟化宿主", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R7 虚拟化宿主 — VirtualList 只挂视口", []string{
		"1200 项变高异构列表（文/色块/图）",
		"仅视口(+cache) cell 挂载",
		"bind_count <= 64 且 *10 <= item",
		"Steady 慢滚 / Spike 快滑 2600px/s",
		"Recover 减速 → 到顶底 ping-pong",
		"滚动条 thumb = Align 布局驱动",
		"右栏静态密集区不随滚动重绘",
	})

	bodyW, bodyH := shell.Body.W, shell.Body.H
	listW := bodyW - rightW - 16

	chip := makeChip()
	defer chip.Dispose()

	list := rendering.NewVariableVirtualList(itemCount, 60, rowHeight, func(i int) rendering.RenderObject {
		return buildCell(i, listW, chip)
	})
	list.CacheExtent = cacheRows * 60

	vp := rendering.NewRenderViewport(list)
	vp.FixedWidth = listW
	vp.FixedHeight = bodyH
	shell.Body.Place(vp, 0, 0)

	// 滚动条指示器：轨道贴视口右缘，thumb 用 Align 布局驱动（滚动比例 → ay），
	// resize 时 Layout 自动重算，无手算坐标。
	track := wrkit.NewPanel(10, bodyH, 0.05, 0.06, 0.08, 1)
	shell.Body.Place(track.Box, listW-14, 0)
	thumb := rendering.NewRenderColorBox(6, 56, 0.45, 0.75, 0.95, 1)
	thumbAlign := track.Align(thumb, 0.5, 0)

	// 右栏静态密集区：标题 + 范围标签（boundary 隔离，仅窗口变化时重录）+
	// 4×4 色格（嵌套 boundary）+ 8 条静态标签 —— U17 静态密集要求。
	right := wrkit.NewPanel(rightW, bodyH, 0.11, 0.12, 0.15, 1)
	shell.Body.Place(right.Box, listW+16, 0)
	right.LabelAt("STATIC DENSE (不随滚动重绘)", 12, 12, 10, 0.55, 0.75, 0.95)
	rangeLabel := right.LabelAt("rows ---- -- ---- / ----", 13, 12, 34, 0.95, 0.9, 0.4)
	rangeLabel.SetRepaintBoundary(true)
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
				fmt.Fprintf(os.Stderr, "ui_wr_r7_virtlist: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})

	maxY := list.ContentHeight() - bodyH
	if maxY < 0 {
		maxY = 0
	}

	// 相位脚本（循环）: Steady 3s 慢滚 160px/s → Spike 3s 快滑 2600px/s（快滑约定）
	// → Recover 3s 线性减速 → 循环；到顶/底 ping-pong 反向。
	const cycleLen = 9.0
	var elapsed, dir, y float64 = 0, 1, 0
	lastFirst, lastLast := -1, -1
	maxBind := 0
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		ct := elapsed
		for ct >= cycleLen {
			ct -= cycleLen
		}
		var phase string
		var speed float64
		switch {
		case ct < 3:
			phase, speed = wrkit.PhaseSteady, 160
		case ct < 6:
			phase, speed = wrkit.PhaseSpike, 2600
		default:
			phase = wrkit.PhaseRecover
			speed = 2600 * (1 - (ct-6)/3)
			if speed < 40 {
				speed = 40
			}
		}
		y += dir * speed * dt
		if y >= maxY {
			y, dir = maxY, -1
		}
		if y <= 0 {
			y, dir = 0, 1
		}
		vp.SetScrollOffset(0, y)

		// 滚动条 thumb：滚动比例 → Align 布局驱动（无手算像素坐标）。
		if maxY > 0 {
			thumbAlign.SetAlignment(0.5, y/maxY)
		}
		// 绑定窗口范围标签：仅窗口变化时重录（boundary 隔离）。
		f, l := list.BoundRange()
		if f != lastFirst || l != lastLast {
			lastFirst, lastLast = f, l
			rangeLabel.SetText(fmt.Sprintf("rows %04d-%04d / %d", f, l, itemCount))
			rangeLabel.MarkNeedsPaint()
		}
		if list.BindCount > maxBind {
			maxBind = list.BindCount
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.ItemCount >= itemCount && snapH.BindCount > 0 &&
			snapH.BindCount <= bindCap && snapH.BindCount*10 <= snapH.ItemCount
		shell.UpdateHUD("R7", phase, app, gateOK,
			fmt.Sprintf("bind=%d item=%d rr=%d", snapH.BindCount, snapH.ItemCount, snapH.ScrollRerecord),
			fmt.Sprintf("maxBind=%d y=%.0f", maxBind, y))
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R7",
		Scenario:      "ui_wr_r7_virtlist",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"bind_count":                  snap.BindCount,
			"item_count":                  snap.ItemCount,
			"scroll_rerecord":             snap.ScrollRerecord,
			"max_bind_count":              int64(maxBind),
			"bound_first":                 int64(lastFirst),
			"bound_last_exclusive":        int64(lastLast),
			"cache_extent_px":             list.CacheExtent,
			"row_heights_px":              "44/92/60/76 (variable prefix-sum)",
			"hitch_budget_per_min":        hitchBudget,
			"rss_slope_budget_kb_per_min": slopeBudget,
			"bind_cap":                    bindCap,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates（§2 R7 行 + §2.2 全族；门禁是硬的不许放）----
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing (必须输出，fallback 也须如实标注)")
		os.Exit(1)
	}
	// 族 C 能力专用: bind_count ≪ item_count。
	if snap.ItemCount < itemCount {
		fmt.Fprintf(os.Stderr, "FAIL: item_count=%d want >=%d (虚拟列表未按场景挂载)", snap.ItemCount, itemCount)
		os.Exit(1)
	}
	if snap.BindCount <= 0 || snap.BindCount > bindCap || snap.BindCount*10 > snap.ItemCount {
		fmt.Fprintf(os.Stderr, "FAIL: bind_count=%d item_count=%d want 0<bind<=%d 且 bind*10<=item (虚拟化失效)",
			snap.BindCount, snap.ItemCount, bindCap)
		os.Exit(1)
	}
	// 族 A: 滚动/持续 tick 60fps 档（§2.2.2）。
	if fpsInterval < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick R7)", fpsInterval)
		os.Exit(1)
	}
	if snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: interval_p95_ms=%.1f > 22", snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	// 族 A 长 soak: hitch 预算（README 声明 ≤5/min）。
	if snap.HitchRatePerMin > hitchBudget {
		fmt.Fprintf(os.Stderr, "FAIL: hitch_rate_per_min=%.1f > %.0f", snap.HitchRatePerMin, hitchBudget)
		os.Exit(1)
	}
	// 族 E: RSS 斜率预算（虚拟化必须 RSS 平稳，不随滚动爬升）。
	if snap.RSSSlopeKBPerMin > slopeBudget {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.1f > %.0f", snap.RSSSlopeKBPerMin, slopeBudget)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r7_virtlist: OK presents=%d bind=%d/%d rr=%d maxBind=%d fps=%.1f p95=%.1f hitch=%.1f/min slope=%.0fKB/min vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), snap.BindCount, snap.ItemCount, snap.ScrollRerecord, maxBind,
		fpsInterval, snap.P95FrameIntervalMs, snap.HitchRatePerMin, snap.RSSSlopeKBPerMin,
		snap.VSyncSource, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
