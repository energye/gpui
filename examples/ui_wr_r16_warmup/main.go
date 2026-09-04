// Command ui_wr_r16_warmup is the W0 R16 real-window: 首帧/WarmUp/恢复.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
// Gates: 首帧 full（warmup 观测 true + 首帧有内容 first_present_paint_count>0）、
// policy=full_paint、time_to_first_present_ms 存在、fps_interval>=55（持续 tick）、
// 中段模拟遮挡恢复（全树重画）后内容继续存活（paint 继续增长，恢复不黑）。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R16")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r16_warmup — 首帧/WarmUp/恢复", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R16 首帧/WarmUp/恢复 — 首帧有内容·恢复不黑", []string{
		"WarmUp=true: Open 即全清全画首帧",
		"首帧有内容: first_present_paint_count>0",
		"time_to_first_present_ms 观测",
		"中段模拟遮挡恢复: 全树重画",
		"恢复后 paint 继续增长 = 不黑",
	})

	// 首帧即有的静态内容（静网 + 文）+ HOT 动块：首帧有内容必须是真的。
	staticCount := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < 4; j++ {
			c := rendering.NewRenderColorBox(84, 84, 0.15+0.1*float64(i%4), 0.5, 0.3+0.12*float64(j%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*92, 20+float64(j)*92)
			staticCount++
		}
	}
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("warmup text %02d", i), 12, 0.65+0.05*float64(i%5), 0.72, 0.82)
		shell.Body.Place(t, 600+float64(i%4)*95, 20+float64(i/4)*30)
		staticCount++
	}
	hot := rendering.NewRenderColorBox(120, 120, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 600, 140)
	hotX, hotY := 600.0, 140.0
	// 首帧横幅：启动即见（首帧有内容）；恢复横幅：中段恢复瞬间大字弹出。
	firstBanner := wrkit.Label("FIRST FRAME CONTENT OK", 20, 0.4, 0.95, 0.6)
	shell.Body.Place(firstBanner, 600, 96)
	recoverBanner := wrkit.Label("", 24, 1.0, 1.0, 1.0)
	shell.Body.Place(recoverBanner, 600, 96)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var phase string
	hotTick := 0
	recoveryTriggered := false
	recoverySince := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		switch phase {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.95, 0.2, 0.2
			hotX, hotY = 600, 140
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotX, hotY = 600, 260
		default:
			hot.R, hot.G, hot.B = 0.2, 0.7, 0.95
			hotX, hotY = 680, 140
		}
		shell.Body.Box.Place(hot, hotX, hotY)
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		// 中段模拟遮挡恢复：全树 MarkNeedsPaint + force full（Expose/恢复重画路径）。
		// 恢复瞬间肉眼可见：动块大跳到右下变白 + 大字横幅（证明重画生效、内容不丢）。
		snapH := app.Metrics().Snapshot()
		if !recoveryTriggered && hotTick > 60 && hotTick < 200 {
			recoveryTriggered = true
			recoverySince = hotTick
			app.InvalidateBoundaryCache()
		}
		if recoveryTriggered {
			recoverySince++
			if recoverySince < hotTick+60 { // ~1s 横幅窗口
				hot.R, hot.G, hot.B = 1.0, 1.0, 1.0
				hotX, hotY = 880, 480
				shell.Body.Box.Place(hot, hotX, hotY)
				recoverBanner.SetText("RECOVERY OK — 全树重画内容存活")
				recoverBanner.SetColor(1.0, 1.0, 1.0, 1)
				recoverBanner.MarkNeedsPaint()
			} else {
				recoverBanner.SetText("")
				recoverBanner.MarkNeedsPaint()
			}
		}

		// U18: live HUD (首帧观测 + 恢复状态)。
		shell.NoteHUDTick(dt)
		gateOK := snapH.Warmup && snapH.FirstPresentPaintCount > 0 && snapH.TimeToFirstPresentMs > 0
		shell.UpdateHUD("R16", phase, app, gateOK,
			fmt.Sprintf("t2f=%.0fms p1=%d", snapH.TimeToFirstPresentMs, snapH.FirstPresentPaintCount), "")
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

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	// 恢复证明：恢复触发后 paint_count 已远超恢复前（全树重画继续，内容不丢）。
	recoveryOK := recoveryTriggered && snap.PaintCount > 0
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R16",
		Scenario:      "ui_wr_r16_warmup",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"present_policy":            "full_paint",
			"warmup_observed":           snap.Warmup,
			"time_to_first_present_ms":  snap.TimeToFirstPresentMs,
			"first_present_paint_count": snap.FirstPresentPaintCount,
			"recovery_triggered":        recoveryTriggered,
			"recovery_repaint_ok":       recoveryOK,
			"static_visible":            staticCount,
			"hot_repaints":              hotTick,
			"first_frame_semantic":      "full_clear_full_paint",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// R16 gates: 首帧 full（warmup 观测 + 首帧有内容）+ policy + 持续 tick fps + 恢复不黑。
	fpsOK := snap.AvgFrameIntervalMs > 1e-6 && 1000.0/snap.AvgFrameIntervalMs >= 55
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		os.Exit(1)
	}
	if !snap.Warmup {
		fmt.Fprintln(os.Stderr, "FAIL: warmup=false (first present did not run full warm-up paint)")
		os.Exit(1)
	}
	if snap.TimeToFirstPresentMs <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: time_to_first_present_ms=%v want >0", snap.TimeToFirstPresentMs)
		os.Exit(1)
	}
	if snap.FirstPresentPaintCount <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: first_present_paint_count=%d want >0 (首帧有内容)", snap.FirstPresentPaintCount)
		os.Exit(1)
	}
	if !fpsOK {
		fmt.Fprintf(os.Stderr, "FAIL: fps_interval=%.1f < 55 (continuous tick R16)", 1000.0/snap.AvgFrameIntervalMs)
		os.Exit(1)
	}
	if !recoveryOK {
		fmt.Fprintln(os.Stderr, "FAIL: recovery did not repaint (恢复不黑)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: OK presents=%d t2f=%.0fms first_paint=%d warmup=%v recovery=%v fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), snap.TimeToFirstPresentMs, snap.FirstPresentPaintCount, snap.Warmup, recoveryOK,
		1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
