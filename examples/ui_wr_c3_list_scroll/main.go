// Command ui_wr_c3_list_scroll is the W3 C3 composite real-window:
// 虚拟列表(R7) + 滚复用(R7b) + 异步图格(R10) 集成（壳层静态区局部脏承 R4 语义）。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_c3_list_scroll -auto-only
//	go run ./examples/ui_wr_c3_list_scroll                 # manual: scroll until close
//	go run ./examples/ui_wr_c3_list_scroll -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=60 (U16, §2.5: 关闭 60s/观察 120). GPU required.
//
// Evidence (U21): logic probes, pixel probes on the GPU snapshot, and a
// bitwise Golden over header + legend + body. The scroll offset integrates
// dt (not bit-stable), so gate mode freezes it at a fixed y for the final 3s;
// visible rows (and the async-decoded checkerboard sources) are then a pure
// function of the construction tables and every probe want is derived from
// them. Manual mode keeps scrolling and does not assert pixels.
//
// Gates (§3 C3 行指标要点 bind/scroll_rerecord/p95 + §2.2 全族):
//   - bind_count ≪ item_count: item>=1200 且 bind<=64 且 bind*10<=item
//   - scroll_rerecord 上限: 累计 <= 3×item_count
//   - 滚复用结构门禁: 每 tick 全部重录增量 ≤ 滚动进入上界 + 出图额度池，
//     违例 tick 必须为 0（出图重录可能延迟到 cell 重入视口的 tick，由额度吸收）
//   - 异步出图完成: images_loaded == 24
//   - 持续 tick 60fps 档: fps_interval>=55 且 interval_p95_ms<=22；hitch<=5/min
//   - RSS 斜率预算（稳态最小二乘）: rss_slope_valid=true 时
//     rss_slope_steady_kb_per_min<=30000；valid=false 只记账不判
//   - vsync_source 必须输出；boundary_skip>0（静区/保留 cell 回放）
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
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
	freezeY      = 1234.0 // gate 冻结的确定性滚动偏移（< maxY，行内容纯由行高/源色表推出）
	freezeTail   = 3.0    // 末段冻结秒数
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
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze scroll at the end, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "C3")
	} else if *autoOnly {
		secs = closeSeconds
	}
	gateMode := secsSet
	if !gateMode {
		secs = 0 // manual: run until close (or -manual-seconds)
	}
	if gateMode && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close C3 (§2.5 close duration)\n", closeSeconds)
		os.Exit(1)
	}
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

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_c3_list_scroll", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_c3_list_scroll.png")
	snapshotPath := snapPath
	if !gateMode {
		snapshotPath = ""
	}
	runFor := time.Duration(0)
	if gateMode {
		runFor = time.Duration(secs) * time.Second
	} else if *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapshotPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_c3_list_scroll: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	// W6: C3's per-tick rerecord bound was calibrated on the full_paint path
	// (boundary-cache replay); the retained texture-LRU path re-records on
	// eviction by design and trips that bound. Pin the as-closed posture;
	// retained-path list scroll is covered by C9. (C3-on-retained re-close is
	// separate follow-up work, not this window's gate change.)
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

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
		freezeNow := gateMode && elapsed >= float64(secs)-freezeTail
		if freezeNow {
			// Gate freeze: pin the offset so the visible rows (and decoded
			// image sources) are a pure function of the construction tables.
			phase, speed = wrkit.PhaseRecover, 0
			y = freezeY
		} else {
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
		bound := int64(dy/minRow) + int64(dvh/minRow) + int64(cacheBoth) + 2
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
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	rrCap := int64(3 * itemCount)

	// Pixel probes: derived from the construction tables at the frozen offset.
	rowTop := func(i int) float64 {
		t := 0.0
		for k := 0; k < i; k++ {
			t += rowHeight(k)
		}
		return t
	}
	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "track_bg", x: bx + listW - 9, y: by + 400, want: [3]float64{0.05, 0.06, 0.08}, tol: 0.05, kind: kindBG},
		{desc: "gap_body_bg", x: bx + listW + 8, y: by + 300, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "right_bg", x: bx + listW + 16 + 200, y: by + 560, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "right_cell_00", x: bx + listW + 16 + 12 + 26, y: by + 88 + 26, want: [3]float64{0.14, 0.45, 0.30}, tol: 0.06},
		{desc: "right_cell_33", x: bx + listW + 16 + 12 + 58*3 + 26, y: by + 88 + 58*3 + 26, want: [3]float64{0.14 + 0.08*3, 0.45 + 0.06*3, 0.30 + 0.08*((3+3)%3)}, tol: 0.06},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "right_title_ink", x: bx + listW + 16 + 12, y: by + 10, w: 210, h: 16, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 20},
		{desc: "range_label_ink", x: bx + listW + 16 + 12, y: by + 34, w: 200, h: 16, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 20},
		{desc: "img_label_ink", x: bx + listW + 16 + 12, y: by + 58, w: 120, h: 16, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 20},
		{desc: "static_labels_ink", x: bx + listW + 16 + 12, y: by + 334, w: 230, h: 40, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 30},
	}
	if gateMode {
		frac := 0.0
		if maxY > 0 {
			frac = freezeY / maxY
		}
		thumbY := frac*(bodyH-56) + 28
		pixelChecks = append(pixelChecks, pixelCheck{desc: "thumb", x: bx + listW - 14 + 5, y: by + thumbY + 3, want: [3]float64{0.45, 0.75, 0.95}, tol: 0.06})
		kindProbe := func(wantKind int) int {
			for i := 0; i < itemCount; i++ {
				top := rowTop(i)
				if top+rowHeight(i) <= freezeY || top >= freezeY+bodyH {
					continue
				}
				if i%4 == wantKind {
					return i
				}
			}
			return -1
		}
		if i := kindProbe(0); i >= 0 {
			inkRegions = append(inkRegions, inkRegion{desc: fmt.Sprintf("row%04d_text_ink", i), x: bx + 14, y: by + (rowTop(i) - freezeY) + 12, w: 240, h: 16, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 20})
		}
		if i := kindProbe(1); i >= 0 {
			baseWant := [3]float64{0.16 + 0.05*float64(i%7), 0.35 + 0.04*float64(i%5), 0.55}
			h := rowHeight(i)
			pixelChecks = append(pixelChecks, pixelCheck{desc: fmt.Sprintf("row%04d_base", i), x: bx + 14 + (listW-28)/2, y: by + (rowTop(i) - freezeY) + 10 + (h-20)/2, want: baseWant, tol: 0.06})
		}
		if i := kindProbe(2); i >= 0 {
			chipWant := [3]float64{0.30 + 0.06*float64(i%6), 0.65, 0.35}
			pixelChecks = append(pixelChecks, pixelCheck{desc: fmt.Sprintf("row%04d_chip", i), x: bx + listW - 100 + 36, y: by + (rowTop(i) - freezeY) + 39, want: chipWant, tol: 0.06})
		}
		if i := kindProbe(3); i >= 0 {
			src := i % imageSources
			// checkerboard ((x+y)/8+src)%2; (15,5) is light when (2+src) even.
			want := [3]float64{24.0 / 255, 26.0 / 255, 34.0 / 255}
			if (2+src)%2 == 0 {
				want = [3]float64{float64(30+src*9) / 255, float64(60+src*7) / 255, float64(140+src*4) / 255}
			}
			pixelChecks = append(pixelChecks, pixelCheck{desc: fmt.Sprintf("row%04d_img_src%02d", i, src), x: bx + 14 + 15, y: by + (rowTop(i) - freezeY) + 10 + 5, want: want, tol: 0.06})
		}
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	var pixelOK int
	pixelBad := make([]string, 0, len(pixelChecks)+len(inkRegions))
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	pixelTotal := 0
	if gateMode {
		img := loadImage(snapPath)
		pixelResult := make(map[string]bool, len(pixelChecks)+len(inkRegions))
		runPixelChecks(img, pixelChecks, pixelResult)
		runInkRegions(img, inkRegions, pixelResult)
		for desc, ok := range pixelResult {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, desc)
			}
		}
		sort.Strings(pixelBad)
		pixelTotal = len(pixelChecks) + len(inkRegions)

		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		pixelGoldenValue = nil
	}

	logicProbes := []string{
		"presents_positive",
		"fps_interval_ge_55",
		"p95_le_22",
		"hitch_le_budget",
		"vsync_source_present",
		"bind_ratio_ok",
		"scroll_rerecord_le_cap",
		"tick_violations_zero",
		"boundary_skip_positive",
	}
	// The full source set only decodes as rows scroll into view; a short
	// manual run cannot judge it (duration artifact, §2.5 sets 60s).
	fullDuration := gateMode || *manualSeconds == 0 || *manualSeconds >= float64(closeSeconds)
	if fullDuration {
		logicProbes = append(logicProbes, "images_loaded_all")
	}
	slopeJudgeable := fullDuration
	if slopeJudgeable {
		logicProbes = append(logicProbes, "rss_slope_steady_le_budget")
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if snap.P95FrameIntervalMs > 22 {
		logicFailed = append(logicFailed, "p95_le_22")
	}
	if snap.HitchRatePerMin > hitchBudget {
		logicFailed = append(logicFailed, "hitch_le_budget")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if snap.ItemCount < itemCount || snap.BindCount <= 0 || snap.BindCount > 64 || snap.BindCount*10 > snap.ItemCount {
		logicFailed = append(logicFailed, "bind_ratio_ok")
	}
	if snap.ScrollRerecord > rrCap {
		logicFailed = append(logicFailed, "scroll_rerecord_le_cap")
	}
	if scrollViolations != 0 {
		logicFailed = append(logicFailed, "tick_violations_zero")
	}
	if fullDuration && loaded != imageSources {
		logicFailed = append(logicFailed, "images_loaded_all")
	}
	if snap.BoundarySkip <= 0 {
		logicFailed = append(logicFailed, "boundary_skip_positive")
	}
	if slopeJudgeable && snap.RSSSlopeValid && snap.RSSSlopeSteadyKBPerMin > slopeBudget {
		logicFailed = append(logicFailed, "rss_slope_steady_le_budget")
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C3",
		Scenario:      "ui_wr_c3_list_scroll",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
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
			"rss_slope_steady_kb_per_min": snap.RSSSlopeSteadyKBPerMin,
			"rss_slope_valid":             snap.RSSSlopeValid,
			"freeze_y":                    freezeY,
			"freeze_tail_sec":             freezeTail,
			"phase_mode":                  map[bool]string{true: "scroll-then-frozen", false: "scrolling"}[gateMode],
			"evidence_mode":               map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":              map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"golden_mask":                 "header 1200x48 + legend 260x656 + body 904x656",
			"golden_tolerance_pct":        0,
			"logic_probes_ok":             len(logicProbes) - len(logicFailed),
			"logic_probes_total":          len(logicProbes),
			"logic_bad":                   logicFailed,
			"pixel_probes_ok":             pixelOK,
			"pixel_probes_total":          pixelTotal,
			"pixel_bad":                   pixelBad,
			"pixel_golden_diff_pct":       pixelGoldenValue,
			"pixel_golden_total_px":       goldenTotalPx,
			"pixel_golden_first_run":      goldenFirstRun,
			"snapshot_png":                snapshotPath,
			"snapshot_note":               map[bool]string{true: "gate snapshot (GPU readback) used for pixel/Golden evidence", false: "manual mode does not write the gate snapshot; committed evidence is not overwritten"}[gateMode],
			"impl_correctness":            "virtualized list binds ≪ items; retained cells replay; async images all decode and bind; per-tick Δrr stays under the entering bound + image credit",
			"impl_dirty":                  "gate: 9s scroll cycles for ~57s then offset frozen at y=1234 for the final 3s; manual: cycles forever",
			"impl_cache":                  "VariableVirtualList CacheExtent=2 rows; 24 async-decoded sources shared via SetImageShared",
			"impl_edge":                   "1200 variable-height rows, 4 cell kinds (text / color+accent / text+chip / async image+label), right static panel 16 cells + 8 labels, Align thumb",
			"impl_fail":                   "bind>64 or bind*10>item, scroll_rerecord>3×items, tick_violations>0, images_loaded<24, fps<55/p95>22, pixel/golden mismatch → FAIL",
			"impl_visible":                "HUD bind/srr/viol/img + skip; rows scroll then hold; img counter reaches 24/24; gate pixel/Golden evidence",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if !logicPass {
		fail("logic probes %d/%d bad=%v", len(logicProbes)-len(logicFailed), len(logicProbes), logicFailed)
	}
	if gateMode {
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
	}

	status := "OK"
	if !logicPass {
		status = "WARN"
	}
	if gateMode && goldenFirstRun {
		status = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c3_list_scroll: %s presents=%d bind=%d/%d srr=%d/%d viol=%d img=%d/%d skip=%d fps=%.1f p95=%.1f hitch=%.1f/min logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, presents, snap.BindCount, snap.ItemCount, snap.ScrollRerecord, rrCap,
		scrollViolations, loaded, imageSources, snap.BoundarySkip, fpsInterval,
		snap.P95FrameIntervalMs, snap.HitchRatePerMin,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
