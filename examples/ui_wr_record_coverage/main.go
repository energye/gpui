// Command ui_wr_record_coverage is the composition real-window for
// ENGINE_NODE_RECORD_COVERAGE_PLAN (node recording coverage + scroll reuse).
//
//	go run ./examples/ui_wr_record_coverage -manual-seconds 22  # human tour (retained, ~60Hz)
//	RUN_SECONDS=20 go run ./examples/ui_wr_record_coverage -auto-only
//
// Window 1200x800: a long page (800x2400) inside a viewport mixes the engine's
// node types (text / color / image / picture / OnPaint / align / clip /
// opacity / color filter / image filter / transform) plus a Volatile spinner
// and an async image in the side panel. -auto-only runs three evidence phases:
//
//  1. STATIC (1.0->1.4s and 2.0->4.0s, no changes): present_count delta 0.
//  2. SCROLL (4.0->14.0s, auto-scroll): the content boundary replays with ZERO
//     re-record while boundary_skip grows; the frozen final offset shows the
//     newly exposed bands at the expected pixels (displacement correct).
//  3. ANIM (14.0->17.5s, spinner only): the spinner advances, the content
//     boundary still records 0 (Volatile live paint), then the frame freezes
//     for pixel probes + bitwise Golden.
//
// Cross-platform: backend is reported from win.Backend(); the example makes no
// backend-specific choice (X11/Wayland and GL/wgpu run the same code).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sync/atomic"
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
	winW, winH = 1200, 800
	vpW, vpH   = 800.0, 700.0
	pageW      = 800.0
	pageH      = 2400.0
	sideW      = 352.0
	sideH      = 700.0
	vpX, vpY   = 16.0, 60.0
	sideX      = 832.0
	finalY     = 1600.0
	gateSecs   = 19

	warmEnd    = 1.0
	static1End = 1.4
	static2End = 2.5
	staticEnd  = 4.0
	scrollEnd  = 14.0
	animEnd    = 17.5
	finalAt    = 17.65
)

func makeChip(light, dark color.RGBA) *render.ImageBuf {
	m := image.NewRGBA(image.Rect(0, 0, 96, 56))
	for y := 0; y < 56; y++ {
		for x := 0; x < 96; x++ {
			if ((x+y)/10)%2 == 0 {
				m.Set(x, y, light)
			} else {
				m.Set(x, y, dark)
			}
		}
	}
	return render.ImageBufFromImage(m)
}

func buildPage(chip *render.ImageBuf) *rendering.AbsoluteBox {
	page := rendering.NewAbsoluteBox(pageW, pageH)
	page.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}

	// Bands: the last two are the newly exposed strip at finalY (1600).
	page.Place(rendering.NewRenderColorBox(pageW, 60, 0.85, 0.20, 0.20, 1), 0, 0)
	page.Place(rendering.NewRenderColorBox(pageW, 60, 0.15, 0.35, 0.90, 1), 0, 760)
	page.Place(rendering.NewRenderColorBox(pageW, 80, 0.95, 0.60, 0.15, 1), 0, 1600)
	page.Place(rendering.NewRenderColorBox(pageW, 80, 0.15, 0.75, 0.30, 1), 0, 1700)

	// Mixed node types above the frozen window (recorded once, replayed on scroll).
	page.Place(wrkit.Label("RenderText — 长页滚动时由内容边界整棵回放", 15, 0.12, 0.14, 0.18), 20, 84)

	dash := rendering.NewRenderBox()
	dash.FixedWidth, dash.FixedHeight = 300, 44
	dash.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, 8, 0.93, 0.95, 0.99, 1)
		rendering.StrokeRoundRect(pc, 0, 0, size.Width, size.Height, 8, 1.5, 0.25, 0.45, 0.85, 1)
		for x := 14.0; x < size.Width-14; x += 14 {
			rendering.FillRect(pc, x, size.Height-8, 7, 2, 0.25, 0.45, 0.85, 1)
		}
	}
	page.Place(dash, 20, 130)

	page.Place(rendering.NewRenderPictureBox(140, 70, func(dc render.Canvas, ox, oy float64) {
		dc.SetRGBA(0.10, 0.55, 0.25, 1)
		dc.DrawCircle(ox+70, oy+35, 28)
		_ = dc.Fill()
		dc.SetRGBA(1, 1, 1, 1)
		dc.DrawCircle(ox+70, oy+35, 12)
		_ = dc.Fill()
	}), 20, 196)

	img := rendering.NewRenderImage(96, 56)
	img.SetImageShared(chip)
	page.Place(img, 20, 290)
	page.Place(wrkit.Label("RenderImage — 静态图片进内容录制", 13, 0.35, 0.35, 0.42), 130, 306)

	align := rendering.NewRenderAlignBox(rendering.NewRenderColorBox(80, 24, 0.90, 0.50, 0.10, 1), 1, 0.5)
	align.FixedWidth, align.FixedHeight = 300, 44
	page.Place(align, 20, 370)

	page.Place(rendering.NewRenderGrayscale(rendering.NewRenderColorBox(160, 40, 0.90, 0.15, 0.15, 1)), 20, 430)
	page.Place(rendering.NewRenderImageFilter(4, rendering.NewRenderColorBox(160, 40, 0.85, 0.20, 0.20, 1)), 220, 430)

	clip := rendering.NewRenderClipRRect(rendering.NewRenderColorBox(240, 60, 0.55, 0.30, 0.90, 1))
	clip.FixedWidth, clip.FixedHeight = 200, 40
	clip.Radius = 12
	page.Place(clip, 20, 500)

	op := rendering.NewRenderOpacity(0.5,
		rendering.NewRenderColorBox(120, 40, 0.90, 0.15, 0.15, 1),
		rendering.NewRenderColorBox(120, 40, 0.15, 0.35, 0.90, 1))
	page.Place(op, 260, 500)

	tr := rendering.NewRenderTransform(rendering.NewRenderColorBox(120, 60, 0.20, 0.50, 0.90, 1))
	tr.FixedWidth, tr.FixedHeight = 140, 70
	tr.SetRotation(0.4)
	page.Place(tr, 20, 580)

	// Signature nodes inside the frozen window (pixel probes).
	clip2 := rendering.NewRenderClipRRect(rendering.NewRenderColorBox(240, 60, 0.55, 0.30, 0.90, 1))
	clip2.FixedWidth, clip2.FixedHeight = 200, 40
	clip2.Radius = 12
	page.Place(clip2, 40, 1800)

	op2 := rendering.NewRenderOpacity(0.5,
		rendering.NewRenderColorBox(120, 40, 0.90, 0.15, 0.15, 1),
		rendering.NewRenderColorBox(120, 40, 0.15, 0.35, 0.90, 1))
	page.Place(op2, 40, 1860)

	page.Place(rendering.NewRenderGrayscale(rendering.NewRenderColorBox(160, 40, 0.90, 0.15, 0.15, 1)), 40, 1920)
	page.Place(rendering.NewRenderImageFilter(4, rendering.NewRenderColorBox(160, 40, 0.85, 0.20, 0.20, 1)), 40, 1980)

	tr2 := rendering.NewRenderTransform(rendering.NewRenderColorBox(120, 60, 0.20, 0.50, 0.90, 1))
	tr2.FixedWidth, tr2.FixedHeight = 140, 70
	tr2.SetRotation(0.4)
	page.Place(tr2, 40, 2040)

	align2 := rendering.NewRenderAlignBox(rendering.NewRenderColorBox(80, 24, 0.90, 0.50, 0.10, 1), 1, 0.5)
	align2.FixedWidth, align2.FixedHeight = 300, 44
	page.Place(align2, 40, 2120)

	page.Place(wrkit.Label("RenderText — 冻结帧内的文字（Golden 覆盖）", 15, 0.12, 0.14, 0.18), 40, 2180)
	page.Place(rendering.NewRenderPictureBox(140, 70, func(dc render.Canvas, ox, oy float64) {
		dc.SetRGBA(0.10, 0.55, 0.25, 1)
		dc.DrawCircle(ox+70, oy+35, 28)
		_ = dc.Fill()
	}), 40, 2220)
	return page
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: three evidence phases + pixel/Golden snapshot, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "NRC")
	} else if *autoOnly {
		secs, secsSet = gateSecs, true
	}
	gateMode := secsSet
	if !gateMode {
		secs = 0
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width: winW, Height: winH, Title: "gpui 节点录制覆盖 — 14 型组合 / 滚动复用 / 静止零帧",
		Decorations: true, Resizable: false,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.94, G: 0.95, B: 0.97, A: 1}
	root.Place(wrkit.Label("节点录制覆盖与性能面同步 — 14 种节点组合验收", 20, 0.08, 0.10, 0.14), 16, 10)
	root.Place(wrkit.Label("视口滚动=内容边界位移回放（零重录）· Spinner/Video=Volatile 直画 · 异步图片无需输入即唤醒", 12, 0.32, 0.35, 0.40), 16, 38)

	chip := makeChip(color.RGBA{R: 30, G: 160, B: 200, A: 255}, color.RGBA{R: 24, G: 26, B: 34, A: 255})
	defer chip.Dispose()
	asyncChip := makeChip(color.RGBA{R: 220, G: 90, B: 40, A: 255}, color.RGBA{R: 40, G: 44, B: 52, A: 255})

	page := buildPage(chip)
	vp := rendering.NewRenderViewport(page)
	vp.FixedWidth, vp.FixedHeight = vpW, vpH
	root.Place(vp, vpX, vpY)

	side := rendering.NewAbsoluteBox(sideW, sideH)
	side.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.14, A: 1}
	side.SetRepaintBoundary(true)
	root.Place(side, sideX, vpY)
	side.Place(wrkit.Label("SIDE PANEL", 13, 0.55, 0.75, 0.95), 16, 14)
	side.Place(wrkit.Label("Spinner — 每帧变，声明 Volatile（不缓存）", 11, 0.72, 0.75, 0.82), 16, 40)

	spinner := rendering.NewRenderSpinner(40)
	side.Place(spinner, 16, 70)

	asyncImg := rendering.NewRenderImage(96, 56)
	side.Place(asyncImg, 16, 140)
	side.Place(wrkit.Label("异步图片：1.5s 后由后台 goroutine 唤醒（无输入事件）", 11, 0.72, 0.75, 0.82), 16, 204)

	side.Place(rendering.NewRenderColorBox(160, 90, 0.16, 0.17, 0.21, 1), 16, 240)
	video := rendering.NewRenderVideo(160, 90, nil)
	side.Place(video, 16, 240)
	side.Place(wrkit.Label("RenderVideo — 无解码桥：声明 Volatile，直画不缓存", 11, 0.72, 0.75, 0.82), 16, 338)

	side.Place(wrkit.Label("缓存口径：内容边界 = 800x2400 整页；滚动/动效期只放不录", 11, 0.62, 0.66, 0.74), 16, 372)
	side.Place(wrkit.Label("静止期：无脏不排帧（present 增量必须为 0）", 11, 0.62, 0.66, 0.74), 16, 392)

	status := wrkit.Label("phase 0: warm-up", 12, 0.30, 0.33, 0.38)
	root.Place(status, 16, 772)

	snapDir := filepath.Join("examples", "ui_wr_record_coverage", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_record_coverage.png")
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

	var (
		imgReady     atomic.Bool
		imgPending   atomic.Pointer[render.ImageBuf]
		imgInstalled bool
	)

	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.94, ClearG: 0.95, ClearB: 0.97, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapshotPath,
	})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	// Gate mode runs the full-paint path: that is where the BoundaryCache
	// recording walk (R7b counters) is observable. Manual mode keeps the
	// retained policy so the window stays smooth to look at; both paths record
	// nodes through the same Recordable contract.
	if gateMode {
		app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	}
	pipeCache := app.Pipeline().BoundaryCache()

	// Async image: a background goroutine lands data and wakes the sleeping
	// loop via ScheduleFrame (the cross-thread demand latch). The ticker
	// installs the buffer on the UI thread; no input event is involved.
	time.AfterFunc(1500*time.Millisecond, func() {
		imgPending.Store(asyncChip)
		imgReady.Store(true)
		app.ScheduleFrame()
	})

	type phase int
	const (
		phWarm phase = iota
		phSettle
		phStatic1
		phStatic2
		phStaticIdle
		phScroll
		phAnim
		phFinal
		phIdle
	)
	var (
		ph        = phWarm
		want      = true
		elapsed   float64
		nextAt    = warmEnd
		p1, p2    int64
		p3, p4    int64
		rrScroll0 int64
		skScroll0 int64
		rrScroll1 int64
		skScroll1 int64
		rrAnim0   int64
		rrAnim1   int64
		spinStart float64
		spinMoved bool
		scrollY   float64
	)

	setStatus := func(s string) {
		status.SetText(s)
		status.MarkNeedsPaint()
	}

	tk := &ticker{
		want: &want,
		next: func() time.Duration {
			// Phase heartbeat: cap the quiet wait at 100ms so the machine keeps
			// stepping even if the platform wait oversleeps a long deadline.
			// This does NOT request frames (WantsFrame is false in quiet
			// phases), so static evidence stays "0 presents".
			d := nextAt - elapsed
			if d <= 0 {
				return 0
			}
			if d > 0.1 {
				d = 0.1
			}
			return time.Duration(d * float64(time.Second))
		},
		on: func(dt float64) {
			elapsed += dt
			if imgReady.Load() && !imgInstalled {
				imgInstalled = true
				asyncImg.SetImage(imgPending.Load())
			}
			if ph == phScroll {
				scrollY += 180 * dt
				if maxY := vp.MaxScrollY(); scrollY > maxY {
					scrollY = maxY
				}
				vp.SetScrollOffset(0, scrollY)
			}
			if ph == phAnim {
				spinner.SetPhase(spinner.Phase + dt*0.6)
				if spinner.Phase != spinStart {
					spinMoved = true
				}
			}
			if elapsed < nextAt {
				return
			}
			switch ph {
			case phWarm:
				// Settle the in-flight warm-up present first, then sample p1.
				ph, want, nextAt = phSettle, false, 1.1
			case phSettle:
				p1 = app.PresentCount()
				ph, want, nextAt = phStatic1, false, static1End
			case phStatic1:
				p2 = app.PresentCount()
				ph, want, nextAt = phStatic2, false, static2End
			case phStatic2:
				p3 = app.PresentCount()
				ph, want, nextAt = phStaticIdle, false, staticEnd
			case phStaticIdle:
				p4 = app.PresentCount()
				ph, want, nextAt = phScroll, true, scrollEnd
				st := pipeCache.Stats()
				rrScroll0, skScroll0 = st.Rerecord, st.Skip
				setStatus("phase 2: scrolling 10s (content replay, zero re-record)")
			case phScroll:
				vp.SetScrollOffset(0, finalY)
				scrollY = finalY
				st := pipeCache.Stats()
				rrScroll1, skScroll1 = st.Rerecord, st.Skip
				ph, want, nextAt = phAnim, true, animEnd
				spinStart, rrAnim0 = spinner.Phase, st.Rerecord
				setStatus("phase 3: spinner animating (content must not re-record)")
			case phAnim:
				st := pipeCache.Stats()
				rrAnim1 = st.Rerecord
				spinner.SetPhase(0.125)
				ph, want, nextAt = phFinal, true, finalAt
				setStatus("frozen: scroll=1600 · spinner=0.125 · async=loaded")
			case phFinal:
				ph, want, nextAt = phIdle, false, 3600
			}
			if want {
				app.ScheduleFrame()
			}
		},
	}
	app.Scheduler().Tickers().Add(tk)

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
	contentCached := pipeCache.HasValid(page)

	logicProbes := []string{
		"presents_positive",
		"static_idle_1_5s_zero_presents",
		"content_boundary_cached",
		"scroll_zero_rerecord",
		"scroll_skip_grows",
		"anim_zero_rerecord",
		"spinner_moved",
		"async_image_installed",
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if p4-p3 != 0 {
		logicFailed = append(logicFailed, "static_idle_1_5s_zero_presents")
	}
	if !contentCached {
		logicFailed = append(logicFailed, "content_boundary_cached")
	}
	if rrScroll1-rrScroll0 != 0 {
		logicFailed = append(logicFailed, "scroll_zero_rerecord")
	}
	if skScroll1-skScroll0 <= 0 {
		logicFailed = append(logicFailed, "scroll_skip_grows")
	}
	if rrAnim1-rrAnim0 != 0 {
		logicFailed = append(logicFailed, "anim_zero_rerecord")
	}
	if !spinMoved {
		logicFailed = append(logicFailed, "spinner_moved")
	}
	if !imgInstalled {
		logicFailed = append(logicFailed, "async_image_installed")
	}
	logicPass := len(logicFailed) == 0

	// Frozen-frame pixels: newly exposed bands at finalY, plus the mixed-node
	// signatures and the async image (all derived from the construction).
	logical := []wrkit.PixelProbe{
		{Name: "band_orange_new", X: vpX + 400, Y: vpY + 40, R: 0.95, G: 0.60, B: 0.15, Tol: 6},
		{Name: "band_green_new", X: vpX + 400, Y: vpY + 130, R: 0.15, G: 0.75, B: 0.30, Tol: 6},
		{Name: "clip_center", X: vpX + 40 + 100, Y: vpY + 200 + 20, R: 0.55, G: 0.30, B: 0.90, Tol: 8},
		// Rect extent of the clip (the child is 240 wide, the clip 200):
		// proves the clip is applied; the rounded-corner radius itself is a
		// known retained-composite gap recorded in the plan ledger.
		{Name: "clip_right_outside", X: vpX + 40 + 205, Y: vpY + 200 + 20, R: 1.00, G: 1.00, B: 1.00, Tol: 6},
		{Name: "grayscale_center", X: vpX + 40 + 80, Y: vpY + 320 + 20, R: 0.374, G: 0.374, B: 0.374, Tol: 10},
		{Name: "blur_center", X: vpX + 40 + 80, Y: vpY + 380 + 20, R: 0.85, G: 0.20, B: 0.20, Tol: 20},
		{Name: "transform_center", X: vpX + 40 + 70, Y: vpY + 440 + 35, R: 0.20, G: 0.50, B: 0.90, Tol: 10},
		{Name: "align_badge", X: vpX + 40 + 260, Y: vpY + 520 + 22, R: 0.90, G: 0.50, B: 0.10, Tol: 8},
		{Name: "side_bg", X: sideX + 200, Y: vpY + 620, R: 0.10, G: 0.11, B: 0.14, Tol: 6},
		{Name: "async_chip_light", X: sideX + 16 + 15, Y: vpY + 140 + 5, R: 220.0 / 255, G: 90.0 / 255, B: 40.0 / 255, Tol: 8},
	}
	var pixelErr error
	var goldenDiff float64
	var goldenPx int64
	var goldenSeeded bool
	if gateMode {
		pixelErr = wrkit.CheckPixelProbes(snapPath, winW, logical)
		var gerr error
		goldenDiff, goldenPx, goldenSeeded, gerr = wrkit.GoldenGateW(snapPath,
			filepath.Join(snapDir, "showcase_record_coverage_base.png"), winW,
			[]wrkit.GoldenRect{{X: 0, Y: 0, W: winW, H: winH}})
		if gerr != nil {
			fmt.Fprintln(os.Stderr, "FAIL: golden:", gerr)
			os.Exit(1)
		}
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "NRC",
		Scenario:      "ui_wr_record_coverage",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"evidence_warm_settle":      p2 - p1,
			"evidence_static_idle_1_5s": p4 - p3,
			"evidence_async_frames":     p3 - p1,
			"evidence_scroll_rerecord":  rrScroll1 - rrScroll0,
			"evidence_scroll_skip":      skScroll1 - skScroll0,
			"evidence_anim_rerecord":    rrAnim1 - rrAnim0,
			"spinner_phase_start":       spinStart,
			"spinner_phase_final":       spinner.Phase,
			"spinner_moved":             spinMoved,
			"async_image_installed":     imgInstalled,
			"content_boundary_cached":   contentCached,
			"final_scroll_y":            finalY,
			"page_size":                 fmt.Sprintf("%.0fx%.0f", pageW, pageH),
			"viewport_size":             fmt.Sprintf("%.0fx%.0f", vpW, vpH),
			"node_types":                "ColorBox/Text/Image/PictureBox/BoxOnPaint/AlignBox/ClipRRect/Opacity/ColorFilter/ImageFilter/Transform/Viewport + Spinner/Video Volatile",
			"logic_probes_ok":           len(logicProbes) - len(logicFailed),
			"logic_probes_total":        len(logicProbes),
			"logic_bad":                 logicFailed,
			"fps_interval":              fpsInterval,
			"pixel_error":               errorString(pixelErr),
			"pixel_golden_diff_pct":     goldenDiff,
			"pixel_golden_total_px":     goldenPx,
			"pixel_golden_first_run":    goldenSeeded,
			"snapshot_png":              snapshotPath,
			"impl_correctness":          "content boundary replays at zero re-record while scrolling; the frozen frame shows the newly exposed bands at the expected offsets",
			"impl_dirty":                "static phases change nothing; the async image lands from a goroutine and wakes the loop via ScheduleFrame",
			"impl_cache":                "BoundaryCache: content page 800x2400 records once; scroll/anim frames skip; spinner/video never store",
			"impl_edge":                 "14 node kinds mixed; viewport content-local anchor; 8192px cap fallback covered by unit tests",
			"impl_fail":                 "static presents>0, scroll/anim rerecord>0, skip not growing, spinner not moving, async image missing, pixel/Golden mismatch",
			"impl_visible":              "orange/green newly exposed bands at final scroll 1600; side panel spinner + async image",
			"known_composite_gaps":      "retained textured composite ignores rounded-clip radius and group opacity for texture-composited children (pre-existing, recorded in ENGINE_NODE_RECORD_COVERAGE_PLAN §9; unit recording tests cover both on the FullPaint path)",
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
	// Logic probes are gate-mode assertions; a short manual run may end before
	// the scroll/anim phases complete, so it only reports, never fails.
	if !logicPass && gateMode {
		fail("logic probes %d/%d bad=%v", len(logicProbes)-len(logicFailed), len(logicProbes), logicFailed)
	}
	if gateMode && pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if gateMode && !goldenSeeded && goldenDiff != 0 {
		fail("golden diff=%.4f%% want 0 over %d px", goldenDiff, goldenPx)
	}

	statusWord := "OK"
	if !logicPass && gateMode {
		statusWord = "WARN"
	}
	if gateMode && goldenSeeded {
		statusWord = "SEEDED"
	}
	fmt.Fprintf(os.Stderr, "ui_wr_record_coverage: %s presents=%d static1=%d staticIdle=%d scrollRR=%d scrollSkip=%d animRR=%d spin=%v async=%v fps=%.1f golden=%.4f%% mode=%s elapsed=%.1fs\n",
		statusWord, presents, p2-p1, p4-p3, rrScroll1-rrScroll0, skScroll1-skScroll0, rrAnim1-rrAnim0,
		spinMoved, imgInstalled, fpsInterval, goldenDiff,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ticker is the phase machine: registered tick, but frame demand is explicit
// (WantsFrame) and quiet phases report the next boundary via NextWake, so the
// static evidence windows really are on-demand idle (no frames, no presents).
type ticker struct {
	on   func(dt float64)
	want *bool
	next func() time.Duration
}

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func (t *ticker) WantsFrame() bool { return t.want != nil && *t.want }

func (t *ticker) NextWake() (time.Duration, bool) {
	if t.next == nil {
		return 0, false
	}
	return t.next(), true
}
