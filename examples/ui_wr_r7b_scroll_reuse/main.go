// Command ui_wr_r7b_scroll_reuse is the W3 R7b real-window: 滚动少重录 cell.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_r7b_scroll_reuse -auto-only
//	go run ./examples/ui_wr_r7b_scroll_reuse                 # manual: scroll until close
//	go run ./examples/ui_wr_r7b_scroll_reuse -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=60 (U16, §2.5: 关闭 60s/观察 120). GPU window required.
//
// Evidence (U21): logic probes, pixel probes on the GPU snapshot, and a
// bitwise Golden over header + legend + body. The scroll position integrates
// dt (not bit-stable), so gate mode freezes the offset at a fixed y for the
// final 3s; the visible rows are then a pure function of the rowHeight table
// and every probe want is derived from the construction constants. Manual
// mode keeps scrolling and does not assert pixels.
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
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
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
	freezeY      = 1234.0 // gate 冻结的确定性滚动偏移（< maxY，行内容纯由 rowHeight 推出）
	freezeTail   = 3.0    // 末段冻结秒数
	itemCount    = 1200
	vpW          = 620.0
	rightW       = 250.0
	cacheRows    = 2.0  // CacheExtent = 2×fallback 行高
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
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze scroll at the end, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R7b")
	} else if *autoOnly {
		secs = closeSeconds
	}
	gateMode := secsSet
	if !gateMode {
		secs = 0 // manual: run until close (or -manual-seconds)
	}
	if gateMode && secs < closeSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R7b (§2.5 close duration)\n", closeSeconds)
		os.Exit(1)
	}
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

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r7b_scroll_reuse", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_r7b_scroll_reuse.png")
	// Manual mode scrolls forever, so its frame is not evidence. Keep the
	// gate snapshot path empty there: otherwise a later manual run would
	// overwrite the committed gate PNG that the Golden baseline was seeded from.
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
		freezeNow := gateMode && elapsed >= float64(secs)-freezeTail
		if freezeNow {
			// Gate freeze: pin the offset at a fixed y so the visible rows are
			// a pure function of the rowHeight table (bit-stable snapshot).
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
			y += dir * speed * dt
			if y >= maxY {
				y, dir = maxY, -1
			}
			if y <= 0 {
				y, dir = 0, 1
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
		bound := int64(dy/minRow) + int64(dvh/minRow) + int64(cacheBoth) + 2
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
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	rrCap := int64(3 * itemCount)

	// Pixel probes: derived from the construction constants at the frozen
	// offset. rowTop(i) is the cumulative row offset; the visible rows at
	// freezeY are a pure function of the rowHeight table.
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
		{desc: "right_cell_00", x: bx + listW + 16 + 38, y: by + 90, want: [3]float64{0.14, 0.45, 0.30}, tol: 0.06},
		{desc: "right_cell_33", x: bx + listW + 16 + 12 + 58*3 + 26, y: by + 64 + 58*3 + 26, want: [3]float64{0.14 + 0.08*3, 0.45 + 0.06*3, 0.30 + 0.08*((3+3)%3)}, tol: 0.06},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}
	inkRegions := []inkRegion{
		{desc: "right_title_ink", x: bx + listW + 16 + 12, y: by + 10, w: 210, h: 16, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 20},
		{desc: "range_label_ink", x: bx + listW + 16 + 12, y: by + 34, w: 200, h: 16, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 20},
		{desc: "static_labels_ink", x: bx + listW + 16 + 12, y: by + 310, w: 230, h: 40, bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.04, minPx: 30},
	}
	if gateMode {
		// Scrollbar thumb: Align(0.5, y/maxY) in the 10-wide track.
		frac := 0.0
		if maxY > 0 {
			frac = freezeY / maxY
		}
		thumbY := frac*(bodyH-56) + 28
		pixelChecks = append(pixelChecks, pixelCheck{desc: "thumb", x: bx + listW - 14 + 5, y: by + thumbY + 3, want: [3]float64{0.45, 0.75, 0.95}, tol: 0.06})
		// Visible rows at freezeY: probe one row of each kind (text / color /
		// image). The list viewport sits at body (0,0).
		kindProbe := func(wantKind int) *probeRow {
			for i := 0; i < itemCount; i++ {
				top := rowTop(i)
				if top+rowHeight(i) <= freezeY || top >= freezeY+bodyH {
					continue
				}
				if i%4 == wantKind {
					return &probeRow{idx: i, top: top}
				}
			}
			return nil
		}
		if r := kindProbe(0); r != nil {
			inkRegions = append(inkRegions, inkRegion{desc: fmt.Sprintf("row%04d_text_ink", r.idx), x: bx + 14, y: by + (r.top - freezeY) + 12, w: 240, h: 16, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 20})
		}
		if r := kindProbe(1); r != nil {
			i := r.idx
			baseWant := [3]float64{0.16 + 0.05*float64(i%7), 0.35 + 0.04*float64(i%5), 0.55}
			h := rowHeight(i)
			pixelChecks = append(pixelChecks, pixelCheck{desc: fmt.Sprintf("row%04d_base", i), x: bx + 14 + (listW-28)/2, y: by + (r.top - freezeY) + 10 + (h-20)/2, want: baseWant, tol: 0.06})
		}
		if r := kindProbe(3); r != nil {
			// chip checkerboard light square: ((x+y)/10)%2==0 with cell-local
			// image offset (14,10); (15,5) lands on a light square.
			pixelChecks = append(pixelChecks, pixelCheck{desc: fmt.Sprintf("row%04d_chip", r.idx), x: bx + 14 + 15, y: by + (r.top - freezeY) + 10 + 5, want: [3]float64{30.0 / 255, 160.0 / 255, 200.0 / 255}, tol: 0.06})
		}
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	// Manual mode scrolls forever; its evidence is human observation.
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
		"scroll_rerecord_le_cap",
		"rr_violations_zero",
		"boundary_skip_positive",
		"bind_ratio_ok",
	}
	// The RSS slope budget is a steady-state judgement (§2.2.4): a short
	// manual run is startup-ramp dominated and cannot judge it.
	slopeJudgeable := gateMode || *manualSeconds == 0 || *manualSeconds >= float64(closeSeconds)
	if slopeJudgeable {
		logicProbes = append(logicProbes, "rss_slope_le_budget")
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
	if slopeJudgeable && snap.RSSSlopeKBPerMin > slopeBudget {
		logicFailed = append(logicFailed, "rss_slope_le_budget")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if snap.ScrollRerecord > rrCap {
		logicFailed = append(logicFailed, "scroll_rerecord_le_cap")
	}
	if rrViolations != 0 {
		logicFailed = append(logicFailed, "rr_violations_zero")
	}
	if snap.BoundarySkip <= 0 {
		logicFailed = append(logicFailed, "boundary_skip_positive")
	}
	if !(snap.ItemCount >= itemCount && snap.BindCount > 0 && snap.BindCount*10 <= snap.ItemCount) {
		logicFailed = append(logicFailed, "bind_ratio_ok")
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R7b",
		Scenario:      "ui_wr_r7b_scroll_reuse",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
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
			"impl_correctness":            "retained cells replay from cache; only rows entering viewport(+cache) record once; per-tick Δrr stays under the entering-row bound",
			"impl_dirty":                  "gate: 9s scroll cycles for ~57s then offset frozen at y=1234 for the final 3s; manual: cycles forever",
			"impl_cache":                  "VariableVirtualList CacheExtent=2 rows; boundary_skip grows while retained cells replay",
			"impl_edge":                   "1200 variable-height rows (44/92/60/76), 4 cell kinds (text/color+accent/text+chip/image+label), right static panel with 16 boundary cells + 8 labels, Align-driven thumb",
			"impl_fail":                   "rr_violations>0 (retained cells re-recorded), scroll_rerecord>3×items, fps<55/p95>22, hitch>5/min, slope>30000, pixel/golden mismatch → FAIL",
			"impl_visible":                "HUD rr/Δ/bound + skip/bind; rows scroll then hold; range label settles at the frozen range; gate pixel/Golden evidence",
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
	fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: %s presents=%d rr=%d/%d viol=%d maxTick=%d/%d skip=%d fps=%.1f p95=%.1f hitch=%.1f/min slope=%.0fKB/min logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, presents, snap.ScrollRerecord, rrCap, rrViolations, maxTickRR, maxTickBound,
		snap.BoundarySkip, fpsInterval, snap.P95FrameIntervalMs, snap.HitchRatePerMin, snap.RSSSlopeKBPerMin,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

// probeRow names a visible row at the frozen offset (index + list top).
type probeRow struct {
	idx int
	top float64
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
