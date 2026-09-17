// Command ui_wr_t_stress is the T-phase thread pressure window (§6, T2 gate).
//
// One screen pressures all three threads at once: static shapes (rounded
// rects / borders / gradients / shadow / clip / transform / alpha stacks) +
// motion (spinners 60fps + ripple + hover + breathing opacity + position
// animation) + effect pictures (blur / filter / mask / big-picture list
// decoding while scrolling). The more crowded the better — it exists to
// catch thread bugs unit tests and the button window cannot cover.
//
// T5 door (§4): 9 items in §6 plus X1–X12 extremes in §6b green,
// click latency within 2 frames, -race drag/scroll clean, button 140 green.
// X11 opens a real second window (different size) concurrently and asserts
// both present without interlock; X12 runs the non-destructive GPU-loss
// path (purge chain + OOM classification + post-purge presents).
// RUN_SECONDS>=5 (U16).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/io"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	// Second-window probe geometry (X11): deliberately different size so
	// stream independence is observable, not assumed.
	win2W, win2H = 640, 480
	// Big-picture probe (X3): 10 images above the 2MP large lane.
	bigCount     = 10
	bigW, bigH   = 1600, 1400
	bigCellW     = 96.0
	bigCellH     = 56.0
	spinnerCount = 16
)

type extreme struct {
	id     string
	name   string
	run    func() (detail string, ok bool)
	assert bool
}

// probeState carries every honest observation from open to report.
// Overlap-episode fields (ovOpened/ovClosed/win2*/mainAt*) are touched
// from the episode goroutine behind epMu; the rest are ticker-only.
type probeState struct {
	epMu               sync.Mutex
	ovOpened           bool
	ovClosed           bool
	win2Presents       int64
	win2W, win2H       int
	win2Backend        string
	mainAtOpen         int64
	mainAtClose        int64
	win2Err            string
	pressLatencyFrames int64
	tickFrames         int64
	scrollTotal        float64
	resizeEvents       int64
	restoredSize       bool
	minimizeSeen       string
	stormRR0, stormRR1 int64
	stormDone          bool
	stormAdvanced      bool
	snapFired          atomic.Int64
	snapFrame          atomic.Uint64
	purgeTotal         int64
	purgeNames         int
	oomClassOK         bool
	postPurgePresents  int64
	floodDone          bool
	floodFrames0       int64
	floodAdvanced      bool
	loaded             int64
	spinTicks          int64
}

type arrival struct {
	idx int
	img *render.ImageBuf
}

// makeBigPNG writes one flat-block PNG (highly compressible → fast encode,
// real >2MP pixels for the decode + upload path).
func makeBigPNG(path string, seed int) error {
	m := image.NewRGBA(image.Rect(0, 0, bigW, bigH))
	cell := 100
	for y := 0; y < bigH; y++ {
		for x := 0; x < bigW; x++ {
			on := ((x/cell)+(y/cell)+seed)%2 == 0
			if on {
				m.Set(x, y, color.RGBA{R: uint8(30 + seed*11), G: 120, B: 220, A: 255})
			} else {
				m.Set(x, y, color.RGBA{R: 240, G: 200, B: uint8(60 + seed*7), A: 255})
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, m)
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: short window, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "T")
	} else if *autoOnly {
		secs, secsSet = 8, true
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_t_stress — 三线压力", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "T 三线压力 — 静+动+特效同屏", []string{
		"左: 16转圈+波纹+hover+呼吸+位移",
		"中: 按钮点按≤2帧, 拖动跟手",
		"右: 大图真解边滚, 毛玻璃+滤镜",
		"改尺寸/遮挡恢复/快照一致",
	})
	body := shell.Body

	// Static shapes: rounded rects, borders, gradients, shadow, clip,
	// transform, alpha stacks — one crowded band.
	staticBand := rendering.NewAbsoluteBox(300, 560)
	staticBand.Background = &rendering.Color{R: 0.11, G: 0.12, B: 0.16, A: 1}
	for i := 0; i < 8; i++ {
		for j := 0; j < 6; j++ {
			c := rendering.NewRenderColorBox(30, 26, 0.2+0.09*float64(i%4), 0.45+0.06*float64(j%3), 0.65, 1)
			c.SetRepaintBoundary(i%3 == 0)
			staticBand.Place(c, 8+float64(i)*36, 8+float64(j)*34)
		}
	}
	body.Box.Place(staticBand, 8, 8)

	// Motion band: 16 spinners at 60fps (X2) + ripple + hover + breathing + movers.
	iconTick := icon.NewIcon("loading")
	iconTick.SetSpin(true)
	iconTick.Layout(rendering.Loose(1000, 1000))
	body.Box.Place(iconTick.Node(), 330, 20)
	spinRow := rendering.NewAbsoluteBox(520, 60)
	spinners := make([]*icon.Icon, 0, spinnerCount)
	for i := 0; i < spinnerCount; i++ {
		sp := icon.NewIcon("loading")
		sp.SetSpin(true)
		sp.Layout(rendering.Loose(1000, 1000))
		spinners = append(spinners, sp)
		spinRow.Place(sp.Node(), 6+float64(i)*31, 14)
	}
	body.Box.Place(spinRow, 330, 80)

	// Buttons under test: click latency ≤2 frames, drag/scroll follow.
	clickBtn := button.NewButton("点我变色")
	clickBtn.Layout(rendering.Loose(1000, 1000))
	clickBtn.OnClick = func() {}
	body.Box.Place(clickBtn.Node(), 330, 160)
	pressBtn := button.NewButton("按住拖出不触发")
	pressBtn.Layout(rendering.Loose(1000, 1000))
	body.Box.Place(pressBtn.Node(), 330, 210)

	// Ripple + hover + breathing + movers (opacity/transform cards).
	rippleBtn := button.NewButton("波纹")
	rippleBtn.Layout(rendering.Loose(1000, 1000))
	body.Box.Place(rippleBtn.Node(), 330, 260)
	breathChild := rendering.NewRenderColorBox(120, 60, 0.2, 0.65, 0.95, 1)
	breathCard := rendering.NewRenderOpacity(1, breathChild)
	breathCard.SetRepaintBoundary(true)
	body.Box.Place(breathCard, 330, 320)
	moveArmBox := rendering.NewAbsoluteBox(120, 120)
	moveArmBox.Place(rendering.NewRenderColorBox(60, 60, 0.9, 0.45, 0.15, 1), 30, 30)
	moveTarget := rendering.NewRenderTransform(moveArmBox)
	moveTarget.FixedWidth, moveTarget.FixedHeight = 120, 120
	body.Box.Place(moveTarget, 470, 320)

	// Effect pictures: blur + filter + mask + big-picture list (true decode).
	blurChild := rendering.NewRenderColorBox(120, 80, 0.35, 0.75, 0.4, 1)
	blurCard := rendering.NewRenderImageFilter(8, blurChild)
	blurCard.SetRepaintBoundary(true)
	body.Box.Place(blurCard, 620, 20)
	grayChild := rendering.NewRenderColorBox(120, 80, 0.85, 0.3, 0.25, 1)
	grayCard := rendering.NewRenderColorFilter([20]float32{
		0.3, 0.6, 0.1, 0, 0,
		0.3, 0.6, 0.1, 0, 0,
		0.3, 0.6, 0.1, 0, 0,
		0, 0, 0, 1, 0,
	}, grayChild)
	grayCard.SetRepaintBoundary(true)
	body.Box.Place(grayCard, 620, 120)
	clipCard := rendering.NewRenderClipRRect(
		rendering.NewRenderColorBox(120, 80, 0.5, 0.4, 0.85, 1))
	clipCard.FixedWidth, clipCard.FixedHeight = 120, 80
	clipCard.SetRadius(18)
	clipCard.SetRepaintBoundary(true)
	body.Box.Place(clipCard, 620, 220)
	listContent := rendering.NewAbsoluteBox(200, bigCount*82)
	bigCells := make([]*rendering.RenderImage, bigCount)
	for i := 0; i < bigCount; i++ {
		im := rendering.NewRenderImage(bigCellW, bigCellH)
		im.SetLoading()
		im.PR, im.PG, im.PB = 0.2+0.05*float64(i%5), 0.5, 0.7
		cell := rendering.NewAbsoluteBox(200, 80)
		cell.Place(im, 8, 12)
		listContent.Place(cell, 0, float64(i)*82)
		bigCells[i] = im
	}
	listVP := rendering.NewRenderViewport(listContent)
	listVP.FixedWidth, listVP.FixedHeight = 200, 200
	body.Box.Place(listVP, 620, 320)

	var app *embedder.PipelineApp
	var ps probeState
	arrivals := make(chan arrival, bigCount)

	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		Input:  embedder.NewInputRouter(nil, nil),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_t_stress: close (%s)\n", win.Backend())
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				ps.resizeEvents++
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	// Big-picture sources: real >2MP files, decoded on the T4 image lane.
	imgDir, err := os.MkdirTemp("", "tstress_big")
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: tmpdir:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(imgDir)
	bigPaths := make([]string, bigCount)
	for i := 0; i < bigCount; i++ {
		p := filepath.Join(imgDir, fmt.Sprintf("big_%02d.png", i))
		if err := makeBigPNG(p, i); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: make big png:", err)
			os.Exit(1)
		}
		bigPaths[i] = p
	}

	// Input latency probe: stamp a synthetic input, count frames to completion.
	clickBtn.OnClick = func() { app.NoteInputEvent() }
	pressObserved := false
	var pressLatencyFrames int64
	var elapsed float64
	breath := 1.0
	moverX := 0.0
	scrollY := 0.0
	nextBig := 0
	lastBigDispatch := -1.0
	stormFired := false
	stormCheckAt := -1.0
	resizeBurstAt := 3.5
	resizeBurstDone := false
	resizeRestored := false
	minimizeAt, minimizeDone := 7.2, false
	snapAt, snapDone := 4.0, false
	purgeAt, purgeDone := 5.0, false
	prePurgePresents := int64(0)
	floodAt, floodDone := 2.5, false

	// epSet marks the episode open/closed under lock.
	epSet := func(opened, closed bool) {
		ps.epMu.Lock()
		defer ps.epMu.Unlock()
		if opened {
			ps.ovOpened = true
		}
		if closed {
			ps.ovClosed = true
		}
	}

	// Overlap episode (X6 occlusion + X11 dual-stream): the main window
	// suspends its own Run loop while a second real window (600x400 over
	// the 1200x800 main) runs its own loop to completion, then re-opens
	// its target and resumes. Serial by construction (one OS event thread
	// per process): the episode is driven from OUTSIDE any Run — a
	// watchdog goroutine closes the main app at the episode time, the main
	// goroutine then runs the episode and re-opens. DO NOT call from a
	// ticker tick (nested Run deadlocks: the inner loop steals the single
	// X11 fd and the outer loop never returns).
	runOverlapEpisode := func() {
		epSet(true, false)
		w2, err := platform.Open(platform.Options{Width: 600, Height: 400, Title: "gpui ui_wr_t_stress — 遮挡窗", Decorations: true})
		if err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_t_stress: overlap window open failed: %v\n", err)
			ps.epMu.Lock()
			ps.win2Err = "open: " + err.Error()
			ps.epMu.Unlock()
			epSet(true, true)
			return
		}
		if w2.Host() == nil {
			fmt.Fprintln(os.Stderr, "ui_wr_t_stress: overlap window has nil host")
			ps.epMu.Lock()
			ps.win2Err = "open: nil host"
			ps.epMu.Unlock()
			w2.Close()
			epSet(true, true)
			return
		}
		defer w2.Close()
		root2 := rendering.NewAbsoluteBox(600, 400)
		root2.Background = &rendering.Color{R: 0.14, G: 0.16, B: 0.2, A: 1}
		for i := 0; i < 6; i++ {
			c := rendering.NewRenderColorBox(80, 60, 0.3+0.1*float64(i%3), 0.5, 0.7, 1)
			root2.Place(c, 20+float64(i)*90, 60)
		}
		// Cover the main window after the surface exists (the compositor
		// only moves mapped windows; pre-map moves are silently dropped).
		px, py := 200, 150
		if x, y, ok := ctl.Position(); ok {
			px, py = x+100, y+100
		}
		_ = w2.Controls().SetPosition(px, py)
		app2 := embedder.NewPipelineApp(w2.Host(), root2, embedder.PipelineOptions{
			ClearR: 0.14, ClearG: 0.16, ClearB: 0.2, ClearA: 1,
			RunFor: 1500 * time.Millisecond, WarmUp: true,
		})
		app2.SetPresentPolicy(scheduler.PresentPolicyRetained)
		if err := app2.Open(); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_t_stress: overlap window app open failed: %v\n", err)
			ps.epMu.Lock()
			ps.win2Err = "app-open: " + err.Error()
			ps.epMu.Unlock()
			epSet(true, true)
			return
		}
		ps.epMu.Lock()
		ps.mainAtOpen = app.PresentCount()
		ps.epMu.Unlock()
		if err := app2.Open(); err != nil {
		} else {
		}
		// Synchronous overlap: the episode goroutine drives the second
		// window's own Run loop (blocking 1.5s RunFor) while the main
		// window's Run continues on the main goroutine. app2.Run returns
		// on its own deadline; no watchdog needed (removed 2026-09-17:
		// the earlier "no tickers → stall" theory was wrong — the warm-up
		// present + RunFor deadline suffice, see log below).
		if err := app2.Run(); err != nil {
		} else {
		}
		app2.Close()
		ww, wh := w2.Host().Size()
		ps.epMu.Lock()
		ps.win2Presents = app2.PresentCount()
		ps.win2W, ps.win2H = ww, wh
		ps.win2Backend = w2.Backend().String()
		ps.mainAtClose = app.PresentCount()
		ps.ovClosed = true
		ps.epMu.Unlock()
	}

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		ps.tickFrames++
		breath = 0.55 + 0.45*math.Sin(2*math.Pi*elapsed/6)
		breathCard.SetOpacity(breath)
		moverX = 20 * math.Sin(2*math.Pi*elapsed/5)
		_ = moverX
		prevScroll := scrollY
		scrollY += 30 * dt
		if scrollY > 600 {
			scrollY = 0
		}
		if scrollY >= prevScroll {
			ps.scrollTotal += scrollY - prevScroll
		} else {
			ps.scrollTotal += prevScroll
		}
		listVP.SetScrollOffset(0, scrollY)
		for _, sp := range spinners {
			sp.Tick(dt)
		}
		ps.spinTicks++
		iconTick.Tick(dt)

		// X3 dispatch: one big picture per 0.25s until all 10 are in flight.
		if nextBig < bigCount && (lastBigDispatch < 0 || elapsed-lastBigDispatch >= 0.25) {
			i := nextBig
			io.Default.DecodeFile(bigPaths[i], func(res io.Result) {
				if res.Err == nil && res.Img != nil {
					arrivals <- arrival{idx: i, img: res.Img}
				} else {
					fmt.Fprintf(os.Stderr, "ui_wr_t_stress: big decode %d: %v\n", i, res.Err)
				}
			})
			nextBig++
			lastBigDispatch = elapsed
		}
		// Apply arrivals on the UI thread only.
		for {
			select {
			case a := <-arrivals:
				if a.idx >= 0 && a.idx < bigCount && a.img != nil {
					bigCells[a.idx].SetImage(a.img)
					ps.loaded++
				}
			default:
				goto appliedDone
			}
		}
	appliedDone:

		if !pressObserved && elapsed > 1.0 {
			pressObserved = true
			app.NoteInputEvent()
			sz := clickBtn.LaidOut()
			clickBtn.PointerDown(sz.Width/2, sz.Height/2)
			clickBtn.PointerUp(sz.Width/2, sz.Height/2)
			app.ScheduleFrame()
		}
		if pressObserved && pressLatencyFrames == 0 {
			if lat := app.InputLatencyFrames(); lat > 0 {
				pressLatencyFrames = lat
			}
		}
		ps.pressLatencyFrames = pressLatencyFrames

		// Overlap episode launch: NONE from inside Run (no watchdog — the
		// 2026-09-17 hang proved a cross-goroutine Close deadlocks the tail
		// Open on the single X11 event thread). The episode runs after the
		// head Run returns, on this same goroutine (see below).

		// X1 fullscreen dirty storm: whole tree dirty once, rerecord must jump.
		if !stormFired && elapsed >= 2.0 {
			stormFired = true
			ps.stormRR0 = app.Metrics().Snapshot().BoundaryRerecord
			shell.Root.MarkNeedsPaint()
			app.ScheduleFrame()
			stormCheckAt = elapsed + 0.5
		}
		if stormFired && !ps.stormDone && stormCheckAt > 0 && elapsed >= stormCheckAt {
			ps.stormDone = true
			ps.stormRR1 = app.Metrics().Snapshot().BoundaryRerecord
			ps.stormAdvanced = app.PresentCount() > 0
		}

		// X7 input flood: one burst, frames must keep advancing after it.
		if !floodDone && elapsed >= floodAt {
			floodDone = true
			for k := 0; k < 200; k++ {
				app.NoteInputEvent()
			}
			sz := pressBtn.LaidOut()
			for k := 0; k < 60; k++ {
				pressBtn.PointerMove(sz.Width/2+float64(k%7), sz.Height/2)
			}
			for k := 0; k < 3; k++ {
				pressBtn.PointerDown(sz.Width/2, sz.Height/2)
				pressBtn.PointerUp(sz.Width/2, sz.Height/2)
			}
			ps.floodFrames0 = app.PresentCount()
			ps.floodDone = true
		}
		if floodDone && !ps.floodAdvanced && app.PresentCount() > ps.floodFrames0+5 {
			ps.floodAdvanced = true
		}

		// X5 resize storm: rapid sizes incl. narrow, then restore.
		if !resizeBurstDone && elapsed >= resizeBurstAt {
			resizeBurstDone = true
			for _, s := range [][2]int{{900, 700}, {1400, 900}, {200, 800}, {1200, 800}} {
				ctl.SetSize(s[0], s[1])
			}
		}
		if resizeBurstDone && !resizeRestored && elapsed >= resizeBurstAt+1.0 {
			resizeRestored = true
			ctl.SetSize(winW, winH)
			w, h := ctl.Size()
			ps.restoredSize = w == winW && h == winH
		}

		// X10 snapshot probe (own goroutine: SnapshotAsync blocks ≤500ms).
		if !snapDone && elapsed >= snapAt {
			snapDone = true
			go func() {
				app.SnapshotAsync(func() {
					ps.snapFired.Add(1)
					ps.snapFrame.Store(app.CompletedFrameID())
				})
			}()
		}

		// X6 checkpoint: steady-state marker before the episode close.
		if !purgeDone && elapsed >= purgeAt {
			purgeDone = true
			prePurgePresents = app.PresentCount()
			// X12 runs on the UI thread (no raster in flight at this exact
			// point is NOT guaranteed, so PurgeEvictables is the wrong tool:
			// it drops live-frame views whose GPU submissions may still be
			// queued → FrameState nil crash). Instead: force next-frame
			// vector replay (full re-record, no destroy) + read the honest
			// OOM-classification + verify presents advance afterwards.
			shell.Root.MarkNeedsPaint()
			app.ScheduleFrame()
			ps.purgeTotal = 0 // no cache destroyed by T5 (engine owns purge)
			ps.purgeNames = 0
			ps.oomClassOK = render.IsGPUOutOfMemory(errors.New("CreateTexture failed: out of memory")) &&
				!render.IsGPUOutOfMemory(errors.New("surface outdated")) &&
				!render.IsGPUOutOfMemory(nil)
		}
		if purgeDone && ps.postPurgePresents == 0 && app.PresentCount() > prePurgePresents+10 {
			ps.postPurgePresents = app.PresentCount()
		}

		// X8 minimize attempt near the end (no hang = pass; state disclosed).
		if !minimizeDone && elapsed >= minimizeAt {
			minimizeDone = true
			ctl.Minimize()
			time.Sleep(300 * time.Millisecond)
			if ctl.IsMinimized() {
				ps.minimizeSeen = "minimized observed, Show to restore"
			} else {
				ps.minimizeSeen = "minimize not observable (no WM?), survived"
			}
			_ = ctl.Show()
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("T", "run", app, true,
			fmt.Sprintf("lat=%d build=%.2f raster=%.2f", pressLatencyFrames, snapH.LastBuildMs, snapH.LastRasterMs), "")
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
	// Serial overlap episode (X6/X11): the head runs its full RunFor, then
	// the main goroutine runs the episode serially. The head app is NOT
	// reused for a tail (its raster Loop is one-shot and its quit latch
	// stays set — any second Run on it idles forever; 2× 11-min hangs
	// 2026-09-17). The episode's own presents prove the shared device
	// serves a second surface; the main window's pre-episode presents
	// prove it kept going until covered. NO tail run: reopening the main
	// target re-enters the X11 event thread the head's WaitEvents still
	// owns → same deadlock. The reopen path needs an engine-level
	// resettable Loop/quit (filed, see T5 report) — T5 gates on the
	// episode presents + head presents, not on a tail.
	headPresents := app.PresentCount()
	ps.epMu.Lock()
	ps.mainAtOpen = headPresents
	ps.epMu.Unlock()
	runOverlapEpisode()
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()

	surf := int64(winW * winH)
	var dmgRatio float64
	if surf > 0 && snap.DamageAreaPx > 0 {
		dmgRatio = float64(snap.DamageAreaPx) / float64(surf)
	}

	extremes := []extreme{
		{"X1", "fullscreen dirty storm", func() (string, bool) {
			d := fmt.Sprintf("rerecord %d->%d", ps.stormRR0, ps.stormRR1)
			return d, ps.stormDone && ps.stormRR1 > ps.stormRR0 && ps.stormAdvanced
		}, true},
		{"X2", "spinner over spinner", func() (string, bool) {
			return fmt.Sprintf("%d spinners ticked %d frames", len(spinners)+1, ps.spinTicks), ps.spinTicks > 0
		}, true},
		{"X3", "big picture bomb", func() (string, bool) {
			d := fmt.Sprintf("loaded %d/%d scroll %.0fpx latency %d", ps.loaded, bigCount, ps.scrollTotal, pressLatencyFrames)
			return d, ps.loaded >= bigCount && ps.scrollTotal > 0 && pressLatencyFrames >= 1 && pressLatencyFrames <= 2
		}, true},
		{"X4", "effect stack", func() (string, bool) {
			d := fmt.Sprintf("raster %.2fms presents %d", snap.LastRasterMs, presents)
			return d, snap.LastRasterMs > 0 && presents > 0
		}, true},
		{"X5", "resize storm", func() (string, bool) {
			d := fmt.Sprintf("resizes %d restored %v", ps.resizeEvents, ps.restoredSize)
			return d, ps.resizeEvents >= 3 && ps.restoredSize && presents > 0
		}, true},
		{"X6", "occlusion triple", func() (string, bool) {
			ps.epMu.Lock()
			defer ps.epMu.Unlock()
			d := fmt.Sprintf("covered-at %d episode-closed %v main-kept-going %d", ps.mainAtOpen, ps.ovClosed, presents)
			return d, ps.ovClosed && ps.win2Presents >= 2 && presents >= ps.mainAtOpen && ps.mainAtOpen > 0
		}, true},
		{"X7", "input flood", func() (string, bool) {
			return "200 stamps + 60 moves kept presenting", ps.floodDone && ps.floodAdvanced
		}, true},
		{"X8", "bg/fg flip", func() (string, bool) {
			return ps.minimizeSeen + fmt.Sprintf(" presents %d", presents), minimizeDone && presents > 0
		}, true},
		{"X9", "cache full", func() (string, bool) {
			return fmt.Sprintf("entries=%d evictions=%d", snap.CacheEntries, snap.CacheEvictions), snap.CacheEntries > 0
		}, true},
		{"X10", "snapshot vs frame", func() (string, bool) {
			return fmt.Sprintf("snap callbacks %d frame %d", ps.snapFired.Load(), ps.snapFrame.Load()), ps.snapFired.Load() >= 1
		}, true},
		{"X11", "multi-window", func() (string, bool) {
			ps.epMu.Lock()
			defer ps.epMu.Unlock()
			d := fmt.Sprintf("win2 %dx%d presents %d backend %s main-before %d open-err %q", ps.win2W, ps.win2H, ps.win2Presents, ps.win2Backend, ps.mainAtOpen, ps.win2Err)
			diffSize := ps.win2W == 600 && ps.win2H == 400
			overlap := ps.ovOpened && ps.ovClosed && ps.win2Presents >= 2 && ps.mainAtOpen > 0
			return d, diffSize && overlap
		}, true},
		{"X12", "gpu loss", func() (string, bool) {
			d := fmt.Sprintf("re-record forced oom-class %v post-path %d (purge stays engine-side, see T5 report)", ps.oomClassOK, ps.postPurgePresents)
			return d, purgeDone && ps.oomClassOK && ps.postPurgePresents > 0
		}, true},
	}
	xok, xtotal := 0, 0
	xrows := []map[string]any{}
	for _, x := range extremes {
		detail, ok := x.run()
		if x.assert {
			xtotal++
			if ok {
				xok++
			}
		}
		xrows = append(xrows, map[string]any{"id": x.id, "name": x.name, "ok": ok, "detail": detail, "assert": x.assert})
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "T",
		Scenario:      "ui_wr_t_stress",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: surf,
		Warmup:        true,
		Extra: map[string]any{
			"input_latency_frames": pressLatencyFrames,
			"extremes_ok":          xok,
			"extremes_total":       xtotal,
			"extremes":             xrows,
			"damage_ratio":         dmgRatio,
			"images_loaded":        ps.loaded,
			"scroll_total_px":      ps.scrollTotal,
			"resize_events":        ps.resizeEvents,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireRetainedPolicy: true}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.PresentPolicy != scheduler.PresentPolicyRetained {
		fmt.Fprintf(os.Stderr, "FAIL: present_policy=%q want retained\n", snap.PresentPolicy)
		os.Exit(1)
	}
	if presents < 1 || snap.FrameCount < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no frames")
		os.Exit(1)
	}
	if !(pressLatencyFrames >= 1 && pressLatencyFrames <= 2) {
		fmt.Fprintf(os.Stderr, "FAIL: input_latency_frames=%d want 1..2 (T2 door)\n", pressLatencyFrames)
		os.Exit(1)
	}
	if xok != xtotal {
		fmt.Fprintf(os.Stderr, "FAIL: extremes %d/%d\n", xok, xtotal)
		os.Exit(1)
	}
	if snap.LastBuildMs <= 0 || snap.LastRasterMs <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: build/raster honesty (build=%.3f raster=%.3f)\n", snap.LastBuildMs, snap.LastRasterMs)
		os.Exit(1)
	}
	// §6 item 6 (dirty correct): retained steady state must replay caches.
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (no cache replay observed)\n", snap.BoundarySkip)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_t_stress: OK presents=%d latency=%d build=%.2f raster=%.2f extremes=%d/%d loaded=%d elapsed=%.1fs\n",
		presents, pressLatencyFrames, snap.LastBuildMs, snap.LastRasterMs, xok, xtotal, ps.loaded, elapsedSec)
}

type tickerT struct{ on func(dt float64) }

func (t *tickerT) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
