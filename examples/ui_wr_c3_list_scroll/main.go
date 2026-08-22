// Command ui_wr_c3_list_scroll is the W3 C3 composite real-window:
// 虚拟列表(R7) + 滚复用(R7b) + 异步图格(R10) 集成（壳层静态区局部脏承 R4 语义）。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_c3_list_scroll
//
// Window: 1200x800. RUN_SECONDS>=60 (U16, §2.5: 关闭 60s/观察 120). GPU required.
//
// Gates (§3 C3 行指标要点 bind/scroll_rerecord/p95 + §2.2 全族):
//   - bind_count ≪ item_count: item>=1200 且 bind<=64 且 bind*10<=item
//   - scroll_rerecord 上限: 累计 <= 3×item_count
//   - 滚复用结构门禁: 每 tick 全部重录增量 ≤ 滚动进入上界 + 出图额度池，
//     违例 tick 必须为 0（出图重录可能延迟到 cell 重入视口的 tick，由额度吸收）
//   - 异步出图完成: images_loaded == 24
//   - 持续 tick 60fps 档: fps_interval>=55 且 interval_p95_ms<=22；hitch<=5/min
//   - RSS 斜率预算（稳态最小二乘）: rss_slope_kb_per_min<=30000
//   - vsync_source 必须输出；boundary_skip>0（静区/保留 cell 回放）
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
	closeSeconds = 60
	itemCount    = 1200
	vpW          = 620.0
	rightW       = 250.0
	cacheRows    = 2.0
	minRow       = 44.0
	imageSources = 24 // 异步解码的唯一源图数（图片行按 i%24 共享）
	hitchBudget  = 5.0
	slopeBudget  = 30000.0
)

func rowHeight(i int) float64 {
	switch i % 4 {
	case 0:
		return 44
	case 1:
		return 92
	case 2:
		return 60
	default:
		return 76
	}
}

func makeSourcePNGs(dir string) ([]string, error) {
	paths := make([]string, imageSources)
	for i := 0; i < imageSources; i++ {
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
		p := filepath.Join(dir, fmt.Sprintf("c3_src_%02d.png", i))
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

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	}
	wrkit.RequireMinRun(secs, "C3")
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c3_list_scroll — 虚拟列表+滚复用+异步图格", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C3 组合 — 虚拟列表+滚复用+异步图格", []string{
		"1200 项变高异构列表持续滚动",
		"仅视口(+cache) cell 挂载（R7）",
		"静 cell 回放不重录（R7b）",
		"图片行异步解码逐格点亮（R10）",
		"Δrr ≤ 滚动进入上界 + 出图额度",
		"右栏静态密集区全程静止（R4 语义）",
	})

	bodyW, bodyH := shell.Body.W, shell.Body.H
	listW := bodyW - rightW - 16

	// 异步图源注册表：decoded 常驻（SetImageShared 共享，退出统一 Dispose）；
	// liveNodes 记录各源当前挂载的 RenderImage（UI 线程内读写，无锁）。
	decoded := make(map[int]*render.ImageBuf, imageSources)
	liveNodes := make(map[int]*rendering.RenderImage, imageSources)

	list := rendering.NewVariableVirtualList(itemCount, 60, rowHeight, func(i int) rendering.RenderObject {
		h := rowHeight(i)
		cell := rendering.NewAbsoluteBox(listW, h)
		switch i % 4 {
		case 0:
			t := wrkit.Label(fmt.Sprintf("row %04d · text line — C3 组合窗", i), 15, 0.85, 0.88, 0.95)
			cell.Place(t, 14, 12)
		case 1:
			base := rendering.NewRenderColorBox(listW-28, h-20, 0.16+0.05*float64(i%7), 0.35+0.04*float64(i%5), 0.55, 1)
			base.SetRepaintBoundary(true)
			cell.Place(base, 14, 10)
			inner := rendering.NewRenderColorBox(140, 24, 0.95, 0.55, 0.15, 1)
			cell.Place(inner, 28, 10+(h-20)/2-12)
		case 2:
			t := wrkit.Label(fmt.Sprintf("row %04d · mixed text + chip", i), 14, 0.75, 0.85, 0.75)
			cell.Place(t, 14, 8)
			chipBox := rendering.NewRenderColorBox(72, 18, 0.30+0.06*float64(i%6), 0.65, 0.35, 1)
			cell.Place(chipBox, listW-100, 30)
		default:
			src := i % imageSources
			im := rendering.NewRenderImage(96, 56)
			if buf := decoded[src]; buf != nil {
				im.SetImageShared(buf)
			} else {
				im.SetLoading()
				liveNodes[src] = im
			}
			cell.Place(im, 14, 10)
			t := wrkit.Label(fmt.Sprintf("row %04d · image #%02d", i, src), 13, 0.80, 0.82, 0.90)
			cell.Place(t, 122, 28)
		}
		return cell
	})
	list.CacheExtent = cacheRows * 60

	vp := rendering.NewRenderViewport(list)
	vp.FixedWidth = listW
	vp.FixedHeight = bodyH
	shell.Body.Place(vp, 0, 0)

	track := wrkit.NewPanel(10, bodyH, 0.05, 0.06, 0.08, 1)
	shell.Body.Place(track.Box, listW-14, 0)
	thumb := rendering.NewRenderColorBox(6, 56, 0.45, 0.75, 0.95, 1)
	thumbAlign := track.Align(thumb, 0.5, 0)

	right := wrkit.NewPanel(rightW, bodyH, 0.11, 0.12, 0.15, 1)
	shell.Body.Place(right.Box, listW+16, 0)
	right.LabelAt("STATIC DENSE (不随滚动重绘)", 12, 12, 10, 0.55, 0.75, 0.95)
	rangeLabel := right.LabelAt("rows ---- -- ---- / ----", 13, 12, 34, 0.95, 0.9, 0.4)
	rangeLabel.SetRepaintBoundary(true)
	imgLabel := right.LabelAt("img 00 / 24", 13, 12, 58, 0.4, 0.9, 0.6)
	imgLabel.SetRepaintBoundary(true)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(52, 52, 0.14+0.08*float64(i%4), 0.45+0.06*float64(j%4), 0.3+0.08*float64((i+j)%3), 1)
			c.SetRepaintBoundary(true)
			right.Place(c, 12+float64(i)*58, 88+float64(j)*58)
		}
	}
	for i := 0; i < 8; i++ {
		right.LabelAt(fmt.Sprintf("static label %02d — 密集文字区", i), 11, 12, 334+float64(i)*22, 0.70, 0.78, 0.88)
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c3_list_scroll: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})

	dir, err := os.MkdirTemp("", "c3_src")
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

	arrivals := make(chan struct {
		idx int
		buf *render.ImageBuf
	}, imageSources)
	dispatch := func(i int) {
		io.Default.DecodeFile(paths[i], func(res io.Result) {
			arrivals <- struct {
				idx int
				buf *render.ImageBuf
			}{idx: i, buf: res.Img}
		})
	}

	maxY := list.ContentHeight() - bodyH
	if maxY < 0 {
		maxY = 0
	}

	const cycleLen = 9.0
	var elapsed, dirScroll, y float64 = 0, 1, 0
	lastFirst, lastLast := -1, -1
	prevY, prevVpH := 0.0, bodyH
	lastBoundaryRR := int64(-1) // -1 = 未建立基线（首 tick 采样后跳过判定，排除 warmup 全量重录）
	loaded := 0
	scrollViolations, maxTickExcess := int64(0), int64(0)
	credit := int64(0) // 出图额度池：每张已应用出图 1 点，吸收延迟到重入 tick 的重录（不过期，总量 ≤ imageSources）

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
		y += dirScroll * speed * dt
		if y >= maxY {
			y, dirScroll = maxY, -1
		}
		if y <= 0 {
			y, dirScroll = 0, 1
		}
		vp.SetScrollOffset(0, y)

		if maxY > 0 {
			thumbAlign.SetAlignment(0.5, y/maxY)
		}
		f, l := list.BoundRange()
		if f != lastFirst || l != lastLast {
			lastFirst, lastLast = f, l
			rangeLabel.SetText(fmt.Sprintf("rows %04d-%04d / %d", f, l, itemCount))
			rangeLabel.MarkNeedsPaint()
		}

		snapH := app.Metrics().Snapshot()

		// 应用本 tick 到达的出图（UI 线程）：写入 decoded 缓存 + 点亮当前挂载节点。
	applyLoop:
		for {
			select {
			case a := <-arrivals:
				if a.buf == nil || decoded[a.idx] != nil {
					continue // 重复到达/空结果保护
				}
				decoded[a.idx] = a.buf
				if node := liveNodes[a.idx]; node != nil {
					node.SetImageShared(a.buf)
				}
				loaded++
				credit++
				imgLabel.SetText(fmt.Sprintf("img %02d / %02d", loaded, imageSources))
				imgLabel.MarkNeedsPaint()
			default:
				break applyLoop
			}
		}

		// 逐 tick 结构门禁：全部重录增量 ≤ 滚动进入上界 + 出图额度。
		nowRR := snapH.BoundaryRerecord
		dRR := nowRR - lastBoundaryRR
		if lastBoundaryRR < 0 {
			// 首 tick：只建立基线（warmup 全量重录不计入任何 tick 的增量）。
			lastBoundaryRR = nowRR
			dRR = 0
		}
		lastBoundaryRR = nowRR
		vpH := vp.Size().Height
		dy, dvh := y-prevY, vpH-prevVpH
		if dy < 0 {
			dy = -dy
		}
		if dvh < 0 {
			dvh = -dvh
		}
		prevY, prevVpH = y, vpH
		cacheBoth := 2 * cacheRows * 60 / minRow
		bound := int64(dy/minRow)+int64(dvh/minRow)+int64(cacheBoth)+2
		excess := dRR - bound
		if excess > 0 {
			if excess <= credit {
				credit -= excess
			} else {
				scrollViolations++
				if excess-credit > maxTickExcess {
					maxTickExcess = excess - credit
				}
				credit = 0
			}
		}

		shell.NoteHUDTick(dt)
		gateOK := scrollViolations == 0 && snapH.ItemCount >= itemCount &&
			snapH.BindCount > 0 && snapH.BindCount <= 64 && snapH.BindCount*10 <= snapH.ItemCount
		shell.UpdateHUD("C3", phase, app, gateOK,
			fmt.Sprintf("bind=%d item=%d srr=%d", snapH.BindCount, snapH.ItemCount, snapH.ScrollRerecord),
			fmt.Sprintf("img=%d/%d v=%d skip=%d", loaded, imageSources, scrollViolations, snapH.BoundarySkip))
		app.ScheduleFrame()
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}

	// 出图派发节奏（窗口打开后开始）：Steady 相位附近每 0.6s 一张，跑完 24 张。
	go func() {
		for i := 0; i < imageSources; i++ {
			time.Sleep(600 * time.Millisecond)
			dispatch(i)
		}
	}()

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

	for _, b := range decoded {
		b.Dispose()
	}

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	rrCap := int64(3 * itemCount)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C3",
		Scenario:      "ui_wr_c3_list_scroll",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"bind_count":                  snap.BindCount,
			"item_count":                  snap.ItemCount,
			"scroll_rerecord":             snap.ScrollRerecord,
			"scroll_rerecord_cap":         rrCap,
			"tick_violations":             scrollViolations,
			"max_tick_excess":             maxTickExcess,
			"images_loaded":               loaded,
			"images_total":                imageSources,
			"boundary_skip":               snap.BoundarySkip,
			"boundary_rerecord":           snap.BoundaryRerecord,
			"hitch_budget_per_min":        hitchBudget,
			"rss_slope_budget_kb_per_min": slopeBudget,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates（§3 C3 行 + §2.2 全族；门禁是硬的不许放）----
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	// R7: bind ≪ item。
	if snap.ItemCount < itemCount || snap.BindCount <= 0 || snap.BindCount > 64 || snap.BindCount*10 > snap.ItemCount {
		fmt.Fprintf(os.Stderr, "FAIL: bind=%d item=%d want 0<bind<=64 且 bind*10<=item>=%d (虚拟化失效)",
			snap.BindCount, snap.ItemCount, itemCount)
		os.Exit(1)
	}
	// R7b: scroll_rerecord 上限 + 结构门禁。
	if snap.ScrollRerecord > rrCap {
		fmt.Fprintf(os.Stderr, "FAIL: scroll_rerecord=%d > cap=%d", snap.ScrollRerecord, rrCap)
		os.Exit(1)
	}
	if scrollViolations != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: tick_violations=%d want 0 (存在 tick 重录超滚动进入上界且无出图额度可吸收)", scrollViolations)
		os.Exit(1)
	}
	// R10: 异步出图全部完成。
	if loaded != imageSources {
		fmt.Fprintf(os.Stderr, "FAIL: images_loaded=%d want %d (异步出图未完成)", loaded, imageSources)
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0", snap.BoundarySkip)
		os.Exit(1)
	}
	if fpsInterval < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick C3)", fpsInterval)
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
	fmt.Fprintf(os.Stderr, "ui_wr_c3_list_scroll: OK presents=%d bind=%d/%d srr=%d/%d viol=%d img=%d/%d skip=%d fps=%.1f p95=%.1f hitch=%.1f/min slope=%.0fKB/min vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), snap.BindCount, snap.ItemCount, snap.ScrollRerecord, rrCap,
		scrollViolations, loaded, imageSources, snap.BoundarySkip, fpsInterval,
		snap.P95FrameIntervalMs, snap.HitchRatePerMin, snap.RSSSlopeKBPerMin, snap.VSyncSource, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
