// Command ui_wr_r7b_scroll_reuse is the W3 R7b real-window: 滚动少重录 cell.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_r7b_scroll_reuse
//
// Window: 1200x800. RUN_SECONDS>=60 (U16, §2.5: 关闭 60s/观察 120). GPU window required.
//
// Gates (§2 R7b 行 + §2.2 全族):
//   - scroll_rerecord 有上限: 累计 ≤ 3×item_count（每 cell 进入窗口只录一次）
//   - 静 cell 保持（结构门禁）: 每 tick 重录增量 ≤ 该 tick 新进入行数上界
//     (⌊|dy|/minRow⌋ + ⌊|dvh|/minRow⌋ + cache 两侧 + 边缘)，违例 tick 数必须为 0
//     ——若每帧全量重录挂载 cell（≈11–15 个），稳态 tick 增量恒超上界即 FAIL
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
	closeSeconds = 60
	itemCount    = 1200
	vpW          = 620.0
	rightW       = 250.0
	cacheRows    = 2.0 // CacheExtent = 2×fallback 行高
	minRow       = 44.0 // 最小行高（变高行的下界，用于进入行数上界）
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

func buildCell(i int, w float64, chip *render.ImageBuf) rendering.RenderObject {
	h := rowHeight(i)
	cell := rendering.NewAbsoluteBox(w, h)
	switch i % 4 {
	case 0:
		t := wrkit.Label(fmt.Sprintf("row %04d · text line — 静 cell 缓存回放不重录", i), 15, 0.85, 0.88, 0.95)
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
		secs = closeSeconds
	}
	wrkit.RequireMinRun(secs, "R7b")
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r7b_scroll_reuse — 滚动少重录", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R7b 滚动少重录 — 静 cell 保持", []string{
		"持续滚动 60s（Steady/Spike/Recover 循环）",
		"静 cell Picture 回放，不重录",
		"新入视口(+cache) cell 才录一次",
		"逐 tick 门禁: Δrr ≤ 新进入行数上界",
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

	track := wrkit.NewPanel(10, bodyH, 0.05, 0.06, 0.08, 1)
	shell.Body.Place(track.Box, listW-14, 0)
	thumb := rendering.NewRenderColorBox(6, 56, 0.45, 0.75, 0.95, 1)
	thumbAlign := track.Align(thumb, 0.5, 0)

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
				fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: close (%s)\n", win.Backend())
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

	const cycleLen = 9.0
	var elapsed, dir, y float64 = 0, 1, 0
	lastFirst, lastLast := -1, -1
	// 逐 tick 结构门禁状态：Δrr ≤ ⌊|dy|/minRow⌋ + ⌊|dvh|/minRow⌋ + cache 两侧 + 边缘。
	prevY, prevVpH := 0.0, bodyH
	lastRR := rendering.ScrollRerecordTotal()
	rrViolations, maxTickRR, maxTickBound := int64(0), int64(0), int64(0)

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

		if maxY > 0 {
			thumbAlign.SetAlignment(0.5, y/maxY)
		}
		f, l := list.BoundRange()
		if f != lastFirst || l != lastLast {
			lastFirst, lastLast = f, l
			rangeLabel.SetText(fmt.Sprintf("rows %04d-%04d / %d", f, l, itemCount))
			rangeLabel.MarkNeedsPaint()
		}

		// 逐 tick 结构门禁：本 tick 重录增量必须 ≤ 新进入（视口+cache）行数上界。
		nowRR := rendering.ScrollRerecordTotal()
		dRR := nowRR - lastRR
		lastRR = nowRR
		vpH := vp.Size().Height
		dy, dvh := y-prevY, vpH-prevVpH
		if dy < 0 {
			dy = -dy
		}
		if dvh < 0 {
			dvh = -dvh
		}
		prevY, prevVpH = y, vpH
		cacheBoth := 2 * cacheRows * 60 / minRow // cache 两侧以最小行高计
		bound := int64(dy/minRow)+int64(dvh/minRow)+int64(cacheBoth)+2
		if bound > maxTickBound {
			maxTickBound = bound
		}
		if dRR > maxTickRR {
			maxTickRR = dRR
		}
		if dRR > bound {
			rrViolations++
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := rrViolations == 0 && snapH.ItemCount >= itemCount &&
			snapH.BindCount > 0 && snapH.BindCount*10 <= snapH.ItemCount
		shell.UpdateHUD("R7b", phase, app, gateOK,
			fmt.Sprintf("rr=%d Δ=%d/%d v=%d", nowRR, dRR, bound, rrViolations),
			fmt.Sprintf("skip=%d bind=%d", snapH.BoundarySkip, snapH.BindCount))
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
	rrCap := int64(3 * itemCount)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R7b",
		Scenario:      "ui_wr_r7b_scroll_reuse",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"scroll_rerecord":             snap.ScrollRerecord,
			"scroll_rerecord_cap":         rrCap,
			"rr_violations":               rrViolations,
			"max_tick_rr":                 maxTickRR,
			"max_tick_bound":              maxTickBound,
			"tick_gate":                   "Δrr <= floor(|dy|/44)+floor(|dvh|/44)+cache_both+2",
			"bind_count":                  snap.BindCount,
			"item_count":                  snap.ItemCount,
			"boundary_skip":               snap.BoundarySkip,
			"boundary_rerecord":           snap.BoundaryRerecord,
			"hitch_budget_per_min":        hitchBudget,
			"rss_slope_budget_kb_per_min": slopeBudget,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// ---- Gates（§2 R7b 行 + §2.2 全族；门禁是硬的不许放）----
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	// 族 C 能力专用①: scroll_rerecord 累计上限。
	if snap.ScrollRerecord > rrCap {
		fmt.Fprintf(os.Stderr, "FAIL: scroll_rerecord=%d > cap=%d (=3×item_count)", snap.ScrollRerecord, rrCap)
		os.Exit(1)
	}
	// 族 C 能力专用②: 静 cell 保持的结构证明——逐 tick 增量不超进入上界。
	if rrViolations != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: rr_violations=%d want 0 (存在 tick 重录数超过新进入行数上界 — 静 cell 未保持)", rrViolations)
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (保留 cell 必须走缓存回放)", snap.BoundarySkip)
		os.Exit(1)
	}
	// 族 A: 滚动/持续 tick 60fps 档。
	if fpsInterval < 55 {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (持续 tick R7b)", fpsInterval)
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
	// 族 E: RSS 斜率预算（稳态语义）。
	if snap.RSSSlopeKBPerMin > slopeBudget {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.1f > %.0f", snap.RSSSlopeKBPerMin, slopeBudget)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: OK presents=%d rr=%d/%d viol=%d maxTick=%d/%d skip=%d fps=%.1f p95=%.1f hitch=%.1f/min slope=%.0fKB/min vsync=%s elapsed=%.1fs\n",
		app.PresentCount(), snap.ScrollRerecord, rrCap, rrViolations, maxTickRR, maxTickBound,
		snap.BoundarySkip, fpsInterval, snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		snap.RSSSlopeKBPerMin, snap.VSyncSource, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
