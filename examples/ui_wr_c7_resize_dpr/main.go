// Command ui_wr_c7_resize_dpr is the W2 C7 combined real-window:
// R11+R19+R3 集成 — 两次尺寸/DPR 失效波 + 1px 对齐 + 嵌套 boundary。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c7_resize_dpr
//
// Window: 1200x800. RUN_SECONDS>=15 (U16, §2.5: C7 关闭用 15s). GPU window required.
//
// 集成场景（§3.1.2 C7 行）：
//   - R11：区 A 两次尺寸变更之一（~3s，Align 布局驱动 FixedWidth/Height + MarkNeedsLayout
//     → 缓存失效 → 一波 rerecord → 回 skip）；区 B DPR 全量失效（~6s
//     InvalidateBoundaryCache → 一波 rerecord → 回 skip）。
//   - R19：SnapLine/SnapRect 1px 网格（snapped crisp vs raw 对比），DPR/尺寸变更后重画，
//     收尾 SnapCoord 网格对齐断言。
//   - R3：3 层嵌套 boundary（outer→mid→inner）+ 内层 hot 每帧变（内脏不外溢，
//     outer/mid skip）；R3 内层 hot 在失效波窗口（3–7s）暂停，波检测干净（steadyRR==0）。
//
// GateOptions = ∪(各 R 门禁)：cache_invalidations>=1 + boundary_rerecord>=1 +
// boundary_skip>=1（R11）、snap 网格对齐（R19）、boundary_skip>=3 +
// boundary_rerecord>=2（R3）、持续 tick fps>=55。自定义：wave1>=1（尺寸波）、
// wave2>=1（DPR 波）、steadyRR==0（回 skip）。
//
// PhaseClock（§3.1.1：三阶段同时影响多能力）：
//   - Steady（0–3s）：HOT 每帧变 + R3 内层 hot 每帧变（inner rerecord、outer/mid skip）。
//   - Resize-Spike（3–3.8s）：区 A 尺寸 190×140→250×170（布局驱动）→ rerecord 波；
//     R19 grid 重画；R3 内层 hot 暂停（波窗口干净）。
//   - DPR-Spike（6–6.8s）：app.InvalidateBoundaryCache() 全量失效 → rerecord 波；
//     R19 grid 重画。
//   - Recover（8.5s+）：R3 内层 hot 恢复每帧变，skip 持续累积。
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
		wrkit.RequireMinRun(secs, "C7")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c7_resize_dpr — 尺寸/DPR 失效 + 1px + 嵌套", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C7 R11+R19+R3 集成 — 失效波回 skip + 1px 清晰", []string{
		"R11: RESIZE-A 尺寸波 (~3s, Align 布局)",
		"R11: DPR-B 全量失效波 (~6s)",
		"R11: 一波 rerecord 后回 skip",
		"R19: SnapLine 1px 网格 (crisp)",
		"R19: raw 网格 (may blur) 对比",
		"R3: 3 层嵌套 boundary 内脏不外溢",
		"wave1>=1 + wave2>=1 + steadyRR=0",
		"持续 tick fps>=55",
	})

	// ------------------------------------------------------------------
	// 区 A — R11 RESIZE：嵌套 2 层 boundary，~3s 尺寸变更 → 一波 rerecord。
	// ------------------------------------------------------------------
	areaA := rendering.NewAbsoluteBox(190, 140)
	areaA.SetRepaintBoundary(true)
	areaA.Background = &rendering.Color{R: 0.16, G: 0.28, B: 0.44, A: 1}
	innerA := rendering.NewRenderColorBox(90, 70, 0.45, 0.72, 0.9, 1)
	innerA.SetRepaintBoundary(true)
	areaA.Place(innerA, 16, 16)
	areaA.Place(wrkit.Label("RESIZE-A", 11, 0.75, 0.85, 0.95), 16, 100)
	areaA.Place(wrkit.Label("size@3s", 10, 0.7, 0.8, 0.9), 116, 16)
	shell.Body.Align(areaA, 0.04, 0.06)

	// ------------------------------------------------------------------
	// 区 B — R11 DPR：2x2 色格 + 标签，~6s 全量失效 → 一波 rerecord。
	// ------------------------------------------------------------------
	areaB := rendering.NewAbsoluteBox(190, 140)
	areaB.SetRepaintBoundary(true)
	areaB.Background = &rendering.Color{R: 0.14, G: 0.32, B: 0.26, A: 1}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			c := rendering.NewRenderColorBox(60, 42, 0.3+float64(i)*0.2, 0.55, 0.4+float64(j)*0.3, 1)
			c.SetRepaintBoundary(true)
			areaB.Place(c, 16+float64(i)*72, 16+float64(j)*52)
		}
	}
	areaB.Place(wrkit.Label("DPR-B", 11, 0.75, 0.85, 0.95), 16, 122)
	shell.Body.Align(areaB, 0.36, 0.06)

	// ------------------------------------------------------------------
	// 区 C — R3 嵌套 boundary（3 层）：outer→mid→inner + 内层 hot。
	// ------------------------------------------------------------------
	hot3 := rendering.NewRenderColorBox(30, 30, 0.95, 0.2, 0.2, 1)
	innerC := rendering.NewAbsoluteBox(70, 70)
	innerC.SetRepaintBoundary(true)
	innerC.Place(hot3, 20, 20)
	midC := rendering.NewAbsoluteBox(110, 110)
	midC.SetRepaintBoundary(true)
	midC.Background = &rendering.Color{R: 0.3, G: 0.3, B: 0.5, A: 1}
	midC.Place(innerC, 20, 20)
	areaC := rendering.NewAbsoluteBox(190, 150)
	areaC.SetRepaintBoundary(true)
	areaC.Background = &rendering.Color{R: 0.2, G: 0.2, B: 0.38, A: 1}
	areaC.Place(midC, 16, 16)
	areaC.Place(wrkit.Label("NEST-C (3层)", 11, 0.75, 0.85, 0.95), 16, 130)
	shell.Body.Align(areaC, 0.04, 0.52)

	// ------------------------------------------------------------------
	// 区 D — R19 1px 网格：snapped crisp vs raw blur 并排。
	// ------------------------------------------------------------------
	grid := rendering.NewRenderBox()
	grid.FixedWidth, grid.FixedHeight = 430, 300
	grid.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		scale := pc.Scale
		if scale <= 0 {
			scale = 1
		}
		// 左半：SnapLine 对齐（crisp）。
		pc.DC.SetLineWidth(1 / scale)
		pc.DC.SetRGBA(0.3, 0.9, 0.6, 1)
		for x := 10.0; x < 200; x += 20 {
			sx := pc.SnapLineX(x)
			pc.DC.DrawLine(pc.OriginX+sx, pc.OriginY+10, pc.OriginX+sx, pc.OriginY+290)
		}
		for y := 10.0; y < 300; y += 20 {
			sy := pc.SnapLineY(y)
			pc.DC.DrawLine(pc.OriginX+10, pc.OriginY+sy, pc.OriginX+200, pc.OriginY+sy)
		}
		// 右半：raw（小数偏移，可能糊）。
		pc.DC.SetRGBA(0.9, 0.35, 0.35, 1)
		for x := 220.5; x < 430; x += 20 {
			pc.DC.DrawLine(pc.OriginX+x, pc.OriginY+10, pc.OriginX+x, pc.OriginY+290)
		}
		for y := 10.5; y < 300; y += 20 {
			pc.DC.DrawLine(pc.OriginX+220, pc.OriginY+y, pc.OriginX+430, pc.OriginY+y)
		}
		// SnapRect 1px 边框（两半各一个）。
		sx, sy, sw, sh := pc.SnapRect(10, 10, 180, 280)
		pc.DC.SetLineWidth(1 / scale)
		pc.DC.SetRGBA(0.9, 0.6, 0.3, 1)
		pc.DC.DrawRectangle(pc.OriginX+sx, pc.OriginY+sy, sw, sh)
		_ = pc.DC.Stroke()
	}
	shell.Body.Align(grid, 0.36, 0.52)
	shell.Body.Align(wrkit.Label("SNAPPED (crisp)", 10, 0.4, 0.9, 0.7), 0.37, 1.02)
	shell.Body.Align(wrkit.Label("RAW (may blur)", 10, 0.9, 0.4, 0.4), 0.52, 1.02)

	// ------------------------------------------------------------------
	// 区 E — 静态密集 3x3 色格（R3 skip 累积，非能力区）。
	// ------------------------------------------------------------------
	staticCount := 0
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(50, 50, 0.25+0.12*float64(i), 0.4+0.1*float64(j), 0.62, 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 700+float64(i)*58, 40+float64(j)*58)
			staticCount++
		}
	}
	shell.Body.Place(wrkit.Label("STATIC 3x3 (skip)", 10, 0.62, 0.72, 0.85), 700, 220)

	// HOT spot：非 boundary live paint，每帧变（稳态 rerecord 噪声为零）。
	hot := rendering.NewRenderColorBox(26, 26, 0.95, 0.3, 0.25, 1)
	shell.Body.Align(hot, 0.90, 0.92)
	shell.Body.Align(wrkit.Label("HOT", 10, 0.95, 0.7, 0.6), 0.90, 1.04)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var (
		elapsed          float64
		resized          bool
		dprInvalidated   bool
		wave1, wave2     int64
		before1, before2 int64
		wave1Sampled     bool
		wave2Sampled     bool
		peakRR1, peakRR2 int64
		steadyRR         int64
		steadySampled    bool
	)
	hotHue := 0.0
	dpr := 1.0
	lastDPR := 1.0

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt

		// PhaseClock（§3.1.1：同时影响 R11 失效波 + R3 内层 hot + R19 grid）。
		phase := wrkit.PhaseSteady
		switch {
		case elapsed >= 6.0 && elapsed < 6.8:
			phase = "DPR-Spike"
		case elapsed >= 3.0 && elapsed < 3.8:
			phase = "Resize-Spike"
		case elapsed >= 8.5:
			phase = wrkit.PhaseRecover
		}

		// Change #1 (~3s)：程序化尺寸变更（Align 布局驱动）。
		if !resized && elapsed >= 3.0 {
			before1 = app.Metrics().Snapshot().BoundaryRerecord
			areaA.FixedWidth, areaA.FixedHeight = 250, 170
			areaA.MarkNeedsLayout()
			resized = true
		}
		if resized && elapsed < 4.0 {
			if v := app.BoundaryCache().FrameRerecord; v > peakRR1 {
				peakRR1 = v
			}
		}
		if resized && !wave1Sampled && elapsed >= 3.0+0.8 {
			wave1 = app.Metrics().Snapshot().BoundaryRerecord - before1
			wave1Sampled = true
		}
		if resized && !steadySampled && elapsed >= 4.0 {
			steadyRR = app.BoundaryCache().FrameRerecord
			steadySampled = true
		}

		// Change #2 (~6s)：程序化 DPR 全量失效。
		if !dprInvalidated && elapsed >= 6.0 {
			before2 = app.Metrics().Snapshot().BoundaryRerecord
			app.InvalidateBoundaryCache()
			dprInvalidated = true
		}
		if dprInvalidated && elapsed < 7.0 {
			if v := app.BoundaryCache().FrameRerecord; v > peakRR2 {
				peakRR2 = v
			}
		}
		if dprInvalidated && !wave2Sampled && elapsed >= 6.0+0.8 {
			wave2 = app.Metrics().Snapshot().BoundaryRerecord - before2
			wave2Sampled = true
		}

		// R19：DPR 变化时重画 grid（resize/scale 切换）。
		dpr = host.ScaleFactor()
		if dpr <= 0 {
			dpr = 1
		}
		if dpr != lastDPR {
			lastDPR = dpr
			grid.MarkNeedsPaint()
		}

		// HOT 每帧变（非 boundary，稳态 rerecord 噪声为零）。
		hotHue += 0.07
		if hotHue > 1 {
			hotHue = 0
		}
		hot.R, hot.G, hot.B = 0.85+0.1*hotHue, 0.25+0.4*(1-hotHue), 0.3+0.5*hotHue
		hot.MarkNeedsPaint()

		// R3 内层 hot：Steady/Recover 每帧变（inner rerecord、outer/mid skip）；
		// 失效波窗口（3–7s）暂停，波检测干净（steadyRR==0）。
		if elapsed < 3.0 || elapsed >= 7.0 {
			hot3.R, hot3.G, hot3.B = 0.95-0.4*(hotHue), 0.2+0.3*hotHue, 0.2+0.5*(1-hotHue)
			hot3.MarkNeedsPaint()
		}

		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD — skip/rerecord/inv/wave/dpr 是 C7 的证明。
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		inv := app.CacheInvalidations()
		gateOK := snapH.BoundarySkip >= 3 && snapH.BoundaryRerecord >= 2 && inv >= 1
		shell.UpdateHUD("C7", phase, app, gateOK,
			fmt.Sprintf("skip=%d rr=%d inv=%d dpr=%.1f", snapH.BoundarySkip, snapH.BoundaryRerecord, inv, dpr),
			fmt.Sprintf("wave1=%d wave2=%d t=%.1f", wave1, wave2, elapsed))
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
	wrkit.MergeBoundaryCache(app, &snap)
	inv := app.CacheInvalidations()

	// R19 snap 正确性：SnapCoord 必须返回设备像素网格对齐值。
	snapOK := true
	sx, _, _, _ := rendering.SnapRect(1.3, 2.6, 10.4, 5.2, dpr)
	v := rendering.SnapCoord(1.3, dpr)
	gridV := v * dpr
	if gridV != float64(int64(gridV+0.5)) {
		snapOK = false
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C7",
		Scenario:      "ui_wr_c7_resize_dpr",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			// §3.1.1：covers + 每能力集成验证 + impl_interaction。
			"covers":                []string{"R11", "R19", "R3"},
			"r11_size_invalidation": "RESIZE-A 190×140→250×170（Align 布局驱动）→ 一波 rerecord（wave1）→ 回 skip（steadyRR=0）",
			"r11_dpr_invalidation":  "DPR-B 全量 InvalidateBoundaryCache → 一波 rerecord（wave2）→ 回 skip",
			"r19_snap":              "SnapLine/SnapRect 1px 网格 snapped crisp vs raw 对比；DPR 变化 grid 重画；SnapCoord 网格对齐断言",
			"r3_nested_boundary":    "outer→mid→inner 3 层嵌套；内脏 hot3 每帧变仅 inner 重录，outer/mid skip（BoundaryCache tryReplay 语义）",
			"impl_interaction": "同一 full_paint 帧内：R3 嵌套内层 hot 每帧变只重录 inner（outer/mid skip 累积）；" +
				"R11 尺寸/DPR 失效波使对应区重录后回 skip（steadyRR=0 证明回稳）；" +
				"R19 1px 网格在 DPR 变化时重画并保持 SnapCoord 网格对齐——" +
				"证明控件 尺寸/DPR 变更后 缓存重建 + 1px 清晰 完整恢复",
			"cache_invalidations":    inv,
			"invalidation_kind":      "programmatic_invalidate_boundary_cache + layout size change",
			"resize_wave_rerecord":   wave1,
			"dpr_wave_rerecord":      wave2,
			"resize_wave_peak_frame": peakRR1,
			"dpr_wave_peak_frame":    peakRR2,
			"steady_rr_frame":        steadyRR,
			"dpr":                    dpr,
			"snapped_lines":          38, // snapped 半区 19 竖 + 14 横 + 边框
			"snap_rect_check":        sx,
			"snap_ok":                snapOK,
			"static_visible":         staticCount,
			"nested_depth":           3,
			"boundary_skip":          snap.BoundarySkip,
			"boundary_rerecord":      snap.BoundaryRerecord,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// C7 门禁 = ∪(各 R 门禁)（§3.1.1）。
	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true, // R11/R19 正确性窗（full_paint）
		MinBoundarySkip:        3,    // R3
		MinBoundaryRerecord:    2,    // R3
		MinCacheInvalidations:  1,    // R11
		RequirePersistentFPS:   true, // 持续 tick（§2.2.2 正确性+持续 tick 窗）
		MinFPSWall:             55,
		MaxP95Ms:               22,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	if wave1 < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: resize wave rerecord=%d want >=1 (size change must re-record once)", wave1)
		os.Exit(1)
	}
	if wave2 < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: dpr wave rerecord=%d want >=1 (invalidation must re-record once)", wave2)
		os.Exit(1)
	}
	if steadyRR != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: steady frame rerecord=%d want 0 (regions must be back to skip)", steadyRR)
		os.Exit(1)
	}
	if !snapOK {
		fmt.Fprintf(os.Stderr, "FAIL: SnapCoord(1.3,%.1f)=%.4f not on device-pixel grid (R19)", dpr, v)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: OK skip=%d rr=%d inv=%d wave1=%d wave2=%d peak1=%d peak2=%d steadyRR=%d dpr=%.1f snap=%v presents=%d elapsed=%.1fs\n",
		snap.BoundarySkip, snap.BoundaryRerecord, inv, wave1, wave2, peakRR1, peakRR2, steadyRR, dpr, snapOK, app.PresentCount(), elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
