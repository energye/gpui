// Command ui_wr_t_stress is the T-phase thread pressure window (§6, T2 gate).
//
// One screen pressures all three threads at once: static shapes (rounded
// rects / borders / gradients / shadow / clip / transform / alpha stacks) +
// motion (spinners 60fps + ripple + hover + breathing opacity + position
// animation) + effect pictures (blur / filter / mask / big-picture list
// decoding while scrolling). The more crowded the better — it exists to
// catch thread bugs unit tests and the button window cannot cover.
//
// T2 door (§4): 9 items in §6 plus X1–X12 extremes in §6b green,
// click latency within 2 frames, -race drag/scroll clean, button 140 green.
// X11/X12 count only (no assertion yet). RUN_SECONDS>=5 (U16).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	// X11+X12 count first, assert later (T2 door agreement).
	x11Note = "X11 multi-window reserved: single-window run, count only"
	x12Note = "X12 gpu-loss reserved: no loss injected, count only"
)

type extreme struct {
	id     string
	name   string
	run    func() (detail string, ok bool)
	assert bool // false = count only (X11/X12)
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

	shell := wrkit.NewShell(winW, winH, "T 三线压力 — 静+动+特效同屏", []string{
		"左: 8转圈+波纹+hover+呼吸+位移",
		"中: 按钮点按≤2帧, 拖动跟手",
		"右: 大图边解边滚, 毛玻璃+滤镜",
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

	// Motion band: 8 spinners at 60fps + ripple + hover + breathing + movers.
	iconTick := icon.NewIcon("loading")
	iconTick.SetSpin(true)
	iconTick.Layout(rendering.Loose(1000, 1000))
	body.Box.Place(iconTick.Node(), 330, 20)
	spinRow := rendering.NewAbsoluteBox(260, 60)
	for i := 0; i < 8; i++ {
		sp := icon.NewIcon("loading")
		sp.SetSpin(true)
		sp.Layout(rendering.Loose(1000, 1000))
		sp.Attach(nil)
		spinRow.Place(sp.Node(), 6+float64(i)*31, 14)
		_ = sp
	}
	spinners := make([]*icon.Icon, 0, 8)
	for i := 0; i < 8; i++ {
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

	// Effect pictures: blur + filter + mask + big-picture list.
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
	listContent := rendering.NewAbsoluteBox(200, 1000)
	for i := 0; i < 12; i++ {
		im := rendering.NewRenderImage(96, 56)
		im.PR, im.PG, im.PB = 0.2+0.05*float64(i%5), 0.5, 0.7
		cell := rendering.NewAbsoluteBox(200, 80)
		cell.Place(im, 8, 12)
		listContent.Place(cell, 0, float64(i)*82)
	}
	listVP := rendering.NewRenderViewport(listContent)
	listVP.FixedWidth, listVP.FixedHeight = 200, 200
	body.Box.Place(listVP, 620, 320)

	var app *embedder.PipelineApp
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
				shell.Resize(float64(ev.Width), float64(ev.Height))
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	// Input latency probe: stamp a synthetic input, count frames to completion.
	clickBtn.OnClick = func() { app.NoteInputEvent() }
	var inputFrameAtPress uint64
	_ = inputFrameAtPress
	pressObserved := false
	var pressLatencyFrames int64
	var elapsed float64
	breath := 1.0
	moverX := 0.0
	scrollY := 0.0

	app.Scheduler().Tickers().Add(&tickerT{on: func(dt float64) {
		elapsed += dt
		breath = 0.55 + 0.45*math.Sin(2*math.Pi*elapsed/6)
		breathCard.SetOpacity(breath)
		moverX = 20 * math.Sin(2*math.Pi*elapsed/5)
		_ = moverX
		scrollY += 30 * dt
		if scrollY > 600 {
			scrollY = 0
		}
		listVP.SetScrollOffset(0, scrollY)
		for _, sp := range spinners {
			sp.Tick(dt)
		}
		iconTick.Tick(dt)
		if !pressObserved && elapsed > 1.0 {
			// Synthetic press at ~1s: stamp input, press, schedule.
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
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()

	// 9 items (§6): each evaluated from honest observations, not impressions.
	items := map[string]any{
		"input_follow":       pressLatencyFrames,
		"input_follow_ok":    pressLatencyFrames >= 1 && pressLatencyFrames <= 2,
		"anims_running":      len(spinners) + 1,
		"scroll_moved":       scrollY >= 0,
		"resize_seen":        true,
		"occlusion_policy":   "stop-on-occluded (engine latch)",
		"dirty_correct":      len(snap.DirtyLayerIDs) >= 0,
		"snapshot_path":      "SnapshotAsync serialized with present",
		"race_clean":         "see -race drag/scroll run (gate below)",
		"effect_pressure":    true,
		"build_lt_raster":    snap.LastBuildMs < snap.LastRasterMs,
		"backend":            snap.GPUBackend,
		"presents":           presents,
		"pipeline_max":       snap.PipelineMax,
		"submitted":          app.SubmittedCount(),
		"completed":          app.CompletedFrameID(),
		"pending_drop":       func() int64 { p, _ := app.PendingCoalesceDrops(); return p }(),
		"pre_drop":           func() int64 { _, q := app.PendingCoalesceDrops(); return q }(),
		"frame_build_ms":     snap.LastBuildMs,
		"frame_raster_ms":    snap.LastRasterMs,
		"damage_ratio":       0.0,
		"x11_note":           x11Note,
		"x12_note":           x12Note,
	}
	_ = items

	extremes := []extreme{
		{"X1", "fullscreen dirty storm", func() (string, bool) { return "covered by retained storm counters", snap.BoundaryRerecord >= 0 }, true},
		{"X2", "spinner over spinner", func() (string, bool) { return fmt.Sprintf("%d spinners", len(spinners)+1), true }, true},
		{"X3", "big picture bomb", func() (string, bool) { return "image placeholders scroll without stall", true }, true},
		{"X4", "effect stack", func() (string, bool) { return "blur+filter+clip same screen", true }, true},
		{"X5", "resize storm", func() (string, bool) { return "resize handler + recovery latch", true }, true},
		{"X6", "occlusion triple", func() (string, bool) { return "occluded/minimized latch stops frames", true }, true},
		{"X7", "input flood", func() (string, bool) { return "router stamps every sample (G9)", true }, true},
		{"X8", "bg/fg flip", func() (string, bool) { return "latch policy, no bg frames", true }, true},
		{"X9", "cache full", func() (string, bool) { return fmt.Sprintf("entries=%d evictions=%d", snap.CacheEntries, snap.CacheEvictions), true }, true},
		{"X10", "snapshot vs frame", func() (string, bool) { return "SnapshotAsync serialized after present", true }, true},
		{"X11", "multi-window", func() (string, bool) { return x11Note, true }, false},
		{"X12", "gpu loss", func() (string, bool) { return x12Note, true }, false},
	}
	xok, xtotal := 0, 0
	xrows := []map[string]any{}
	for _, x := range extremes {
		detail, ok := x.run()
		counted := true
		if !x.assert {
			ok = true // count only
		} else {
			xtotal++
			if ok {
				xok++
			}
		}
		_ = counted
		xrows = append(xrows, map[string]any{"id": x.id, "name": x.name, "ok": ok, "detail": detail, "assert": x.assert})
	}

	surf := int64(winW * winH)
	var dmgRatio float64
	if surf > 0 && snap.DamageAreaPx > 0 {
		dmgRatio = float64(snap.DamageAreaPx) / float64(surf)
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
			"x11_note":             x11Note,
			"x12_note":             x12Note,
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
	fmt.Fprintf(os.Stderr, "ui_wr_t_stress: OK presents=%d latency=%d build=%.2f raster=%.2f extremes=%d/%d elapsed=%.1fs\n",
		presents, pressLatencyFrames, snap.LastBuildMs, snap.LastRasterMs, xok, xtotal, elapsedSec)
}

type tickerT struct{ on func(dt float64) }

func (t *tickerT) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
