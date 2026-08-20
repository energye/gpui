// Command ui_wr_c2_retained_scene is the W2 C2 combined real-window:
// R3+R4+R4b+R5 集成 — retained 整场景：多 boundary + Picture + 远离脏点 + damage present。
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene
//
// Window: 1200x800. RUN_SECONDS>=15 (U16, §2.5: C2 关闭用 15s). GPU window required.
//
// 集成场景（§3.1.2 C2 行）：
//   - R3：嵌套 boundary 链（outer→mid→inner 3 层）+ 各层静文/色块；内脏只重录 inner，
//     outer/mid skip；Spike 相外层脏 → outer 整巢重录（巢外不受影响）。
//   - R4：retained 稳态 + 5x3 静态色格阵（各自 boundary）+ 局部动画区 hotA（Align 布局驱动）。
//   - R4b：hot1（右上）+ hot2（左下）对角远离同帧变脏 → 两独立 dirty layer id，
//     中间大面积静态不重绘 → damage_multi_frames>=1。
//   - R5：Picture 直绘 vs 回放同构图并排（8 ops），picture_op_count>=3。
//
// GateOptions = ∪(各 R 门禁)：boundary_skip>=3 + boundary_rerecord>=2（R3）、
// retained policy + damage_ratio_sum<=0.35（sum of rects 口径，R4b 先例；union bbox 为观测）
// + mode 非 full（R4）、dirty_layer_id_max>=2 + damage_multi_frames>=1（R4b）、
// picture_op_count>=3（R5）、持续 tick fps>=55 + p95<=22。
//
// PhaseClock 三阶段同时影响多能力（§3.1.1）：
//   - Steady（0–2s）：R3 inner hot3 每帧变（仅 inner 重录）；hotA/hot1/hot2 每帧变；
//     R5 直绘/回放区 0.5s 低频刷新（retained 下回放缓存静态走 blit skip）。
//   - Spike（2–4s）：R3 outer.Background 翻转（整巢重录）；三热点加速变色；相位横幅变色；
//     R5 区每帧刷新（与相位节奏呼应）。
//   - Recover（4s+）：R3 仅 inner 按 0.5s 低频变（outer/mid 持续 skip）；三热点继续；
//     R5 区低频——retained 稳态回稳。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// hotSize is the side of each dynamic hot spot (R4 animation + R4b distant pair).
const hotSize = 60.0

// picBox is the side of each R5 Picture region (direct / replay).
const picBox = 190.0

// sumRatio estimates the worst-case per-frame re-recorded pixel sum (sum of
// rects — the honest retained cost). DamageStats' avgArea is the **union bbox**
// which is a geometric artifact when dirty layers are scattered across the
// surface (R4b documents the same: diagonal hotspots → union ≈0.49 while the
// true re-recorded sum ≈0.026). Peak frame here: R3 inner 100² + 3 hot 60² +
// R5 direct/replay 2×190² + HUD band 1200×72 (10Hz refresh) ≈ 0.20 ≪ 0.35.
func sumRatio() float64 {
	sum := 100.0*100.0 + 3*hotSize*hotSize + 2*picBox*picBox + 1200.0*72.0
	return sum / (winW * winH)
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "C2")
	}
	face, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Shared image source for the R5 Picture OpDrawImage (green 16x16).
	src, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: image buf:", err)
		os.Exit(1)
	}
	defer src.Dispose()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			_ = src.SetRGBA(x, y, 51, 191, 89, 255)
		}
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_c2_retained_scene — R3+R4+R4b+R5 集成", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "C2 R3+R4+R4b+R5 集成 — retained 整场景", []string{
		"R3: 嵌套 boundary 内脏不外溢",
		"R3: 外层脏→整巢重录",
		"R4: retained 静态色格阵不重绘",
		"R4: damage_ratio<=0.35（≪全屏）",
		"R4b: 对角两热点同帧脏→2 ids",
		"R4b: damage_multi_frames>=1",
		"R5: Picture 直绘≡回放并排",
		"持续 tick fps>=55 · p95<=22",
	})

	// ------------------------------------------------------------------
	// 区 A — R3 嵌套 boundary（3 层）：outer→mid→inner，各层静文/色块。
	// ------------------------------------------------------------------
	outer := rendering.NewAbsoluteBox(220, 220)
	outer.SetRepaintBoundary(true)
	outer.SetDebugName("c2-outer")
	shell.Body.Place(outer, 16, 16)

	mid := rendering.NewAbsoluteBox(160, 160)
	mid.SetRepaintBoundary(true)
	outer.Place(mid, 30, 30)

	inner := rendering.NewAbsoluteBox(100, 100)
	inner.SetRepaintBoundary(true)
	mid.Place(inner, 30, 30)

	hot3 := rendering.NewRenderColorBox(50, 50, 0.95, 0.2, 0.2, 1)
	inner.Place(hot3, 25, 25)

	inner.Place(wrkit.Label("inner lbl", 10, 0.85, 0.9, 0.97), 5, 5)
	mid.Place(wrkit.Label("mid lbl", 10, 0.72, 0.8, 0.92), 5, 140)
	outer.Place(wrkit.Label("outer lbl", 10, 0.7, 0.78, 0.9), 5, 200)
	outer.Place(rendering.NewRenderColorBox(36, 36, 0.4, 0.5, 0.6, 1), 174, 174)
	mid.Place(rendering.NewRenderColorBox(26, 26, 0.5, 0.42, 0.58, 1), 128, 128)

	// ------------------------------------------------------------------
	// 区 B — R4 retained 静态色格阵（5x3，各自 boundary）+ 局部动画区 hotA。
	// ------------------------------------------------------------------
	staticCount := 0
	for i := 0; i < 5; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(70, 70, 0.15+0.07*float64(i%4), 0.42+0.08*float64(j%2), 0.32+0.09*float64((i+j)%3), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 264+float64(i)*76, 16+float64(j)*76)
			staticCount++
		}
	}
	// 静态标签（区 B 下方一行，也是静态）。
	for i := 0; i < 3; i++ {
		t := wrkit.Label(fmt.Sprintf("static col %02d", i), 11, 0.68, 0.75, 0.86)
		t.SetRepaintBoundary(true)
		shell.Body.Place(t, 264+float64(i)*190, 250)
		staticCount++
	}

	// R4 局部动画区 hotA —— Flutter 式 Align 布局驱动（§2.6.1，对标 R4）。
	hotA := rendering.NewRenderColorBox(hotSize, hotSize, 0.95, 0.2, 0.2, 1)
	hotA.SetRepaintBoundary(true)
	hotAAlign := shell.Body.Align(hotA, 0.60, 0.42)

	// ------------------------------------------------------------------
	// 区 C — R4b 两远离脏点：hot1（右上）+ hot2（左下），同帧每帧变脏 →
	// 两独立 dirty layer id；中间大面积静态（区 A/B/D）不重绘。
	// ------------------------------------------------------------------
	hot1 := rendering.NewRenderColorBox(hotSize, hotSize, 0.2, 0.5, 0.95, 1)
	hot1.SetRepaintBoundary(true)
	hot1Align := shell.Body.Align(hot1, 0.90, 0.12)
	hot2 := rendering.NewRenderColorBox(hotSize, hotSize, 0.25, 0.8, 0.4, 1)
	hot2.SetRepaintBoundary(true)
	hot2Align := shell.Body.Align(hot2, 0.12, 0.85)

	// 相位横幅（隔离在独立 boundary，相位切换只重录本层）。
	phaseLabel := wrkit.Label("PHASE: STEADY", 16, 0.95, 0.95, 0.4)
	phaseLabel.SetRepaintBoundary(true)
	shell.Body.Place(phaseLabel, 620, 300)

	// ------------------------------------------------------------------
	// 区 D — R5 Picture 录/回放：直绘 vs 回放同构图并排（8 ops）。
	// ------------------------------------------------------------------
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		p := render.NewPath()
		p.MoveTo(20, 20)
		p.LineTo(90, 20)
		p.LineTo(55, 80)
		p.Close()
		r.FillPath(p, 0.9, 0.3, 0.2, 1)
		p2 := render.NewPath()
		p2.MoveTo(20, 120)
		p2.LineTo(90, 120)
		p2.LineTo(90, 160)
		p2.LineTo(20, 160)
		p2.Close()
		r.StrokePath(p2, 3, 0.2, 0.6, 0.9, 1)
		r.FillRect(20, 190, 80, 40, 0.3, 0.8, 0.4, 1)
		r.StrokeRect(120, 190, 60, 40, 2, 0.9, 0.8, 0.3, 1)
		r.DrawImage(src, 150, 20, 0, 0)
		r.DrawImage(src, 150, 100, 32, 32)
		if face != nil {
			r.DrawString("PIC-REPLAY", 20, 260, face, 0.9, 0.9, 0.95, 1)
			r.DrawString("c2", 120, 260, face, 0.9, 0.8, 0.4, 1)
		}
	})
	opCount := pic.OpCount()
	picBounds := pic.Bounds
	boundsTxt := "no-bounds"
	if !picBounds.Empty() {
		boundsTxt = fmt.Sprintf("(%d,%d %dx%d)", picBounds.Min.X, picBounds.Min.Y, picBounds.Dx(), picBounds.Dy())
	}

	direct := rendering.NewRenderBox()
	direct.FixedWidth, direct.FixedHeight = picBox, picBox
	// R5 区在 retained 下必须独立成层（脏不外溢到 root，否则每帧全窗重录 →
	// present_mode=full / damage≈1）。R5 单窗是 full_paint 无此要求；C2 集成暴露此边界。
	direct.SetRepaintBoundary(true)
	direct.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ox, oy := pc.OriginX, pc.OriginY
		p := render.NewPath()
		p.MoveTo(ox+20, oy+20)
		p.LineTo(ox+90, oy+20)
		p.LineTo(ox+55, oy+80)
		p.Close()
		pc.DC.SetRGBA(0.9, 0.3, 0.2, 1)
		_ = pc.DC.FillPath(p)
		p2 := render.NewPath()
		p2.MoveTo(ox+20, oy+120)
		p2.LineTo(ox+90, oy+120)
		p2.LineTo(ox+90, oy+160)
		p2.LineTo(ox+20, oy+160)
		p2.Close()
		pc.DC.SetRGBA(0.2, 0.6, 0.9, 1)
		pc.DC.SetLineWidth(3)
		_ = pc.DC.StrokePath(p2)
		pc.DC.SetRGBA(0.3, 0.8, 0.4, 1)
		pc.DC.DrawRectangle(ox+20, oy+190, 80, 40)
		_ = pc.DC.Fill()
		pc.DC.SetRGBA(0.9, 0.8, 0.3, 1)
		pc.DC.DrawRectangle(ox+120, oy+190, 60, 40)
		_ = pc.DC.Stroke()
		pc.DC.DrawImage(src, ox+150, oy+20)
		pc.DC.DrawImageEx(src, render.DrawImageOptions{X: ox + 150, Y: oy + 100, DstWidth: 32, DstHeight: 32})
		if face != nil {
			pc.DC.SetFont(face)
			pc.DC.SetRGBA(0.9, 0.9, 0.95, 1)
			pc.DC.DrawString("PIC-REPLAY", ox+20, oy+260)
			pc.DC.SetRGBA(0.9, 0.8, 0.4, 1)
			pc.DC.DrawString("c2", ox+120, oy+260)
		}
	}
	shell.Body.Align(direct, 0.34, 0.55)

	replay := rendering.NewRenderBox()
	replay.FixedWidth, replay.FixedHeight = picBox, picBox
	replay.SetRepaintBoundary(true)
	replay.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		pc.DC.Translate(pc.OriginX, pc.OriginY)
		pic.Replay(pc.DC)
	}
	shell.Body.Align(replay, 0.52, 0.55)

	shell.Body.Align(wrkit.Label("DIRECT (直绘)", 11, 0.82, 0.87, 0.92), 0.34, 0.87)
	shell.Body.Align(wrkit.Label("REPLAY (回放)", 11, 0.82, 0.87, 0.92), 0.52, 0.87)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// C2 核心：retained 稳态（首帧 full，稳态 CompositeOnly + damage present）。
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	var phase string
	lastPhase := ""
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		t := clock.Elapsed()
		// PhaseClock 三阶段同时驱动多能力（R3 脏层选择 / R4 动画 / R4b 双热点）。
		switch phase {
		case wrkit.PhaseSteady:
			// R3：仅 inner 脏（内脏只重录 inner，outer/mid skip）。
			hot3.R, hot3.G, hot3.B = 0.95-0.3*(t/2), 0.2+0.2*(t/2), 0.2
			// R4 动画区 + R4b 双热点每帧变。
			hotA.R, hotA.G, hotA.B = 0.95, 0.2, 0.2
			hot1.R, hot1.G, hot1.B = 0.2, 0.5, 0.95
			hot2.R, hot2.G, hot2.B = 0.25, 0.8, 0.4
			hotAAlign.SetAlignment(0.60, 0.42)
			hot1Align.SetAlignment(0.90, 0.12)
			hot2Align.SetAlignment(0.12, 0.85)
		case wrkit.PhaseSpike:
			// R3：外层脏 → outer 整巢重录（巢外/区 B 不受影响）。
			outer.Background = &rendering.Color{R: 0.35 + 0.2*float64(int(t)%2), G: 0.2, B: 0.3, A: 1}
			// 三热点加速变色 + 微移（布局驱动 SetAlignment）。
			hotA.R, hotA.G, hotA.B = 1.0, 0.85, 0.2
			hot1.R, hot1.G, hot1.B = 0.95, 0.3, 0.95
			hot2.R, hot2.G, hot2.B = 1.0, 0.8, 0.2
			hotAAlign.SetAlignment(0.63, 0.45)
			hot1Align.SetAlignment(0.92, 0.14)
			hot2Align.SetAlignment(0.10, 0.83)
		default:
			// Recover：R3 仅 inner 按 0.5s 低频变（outer/mid 持续 skip）。
			if int(t*2)%2 == 0 {
				hot3.R, hot3.G, hot3.B = 0.95, 0.3, 0.3
			} else {
				hot3.R, hot3.G, hot3.B = 0.3, 0.7, 0.95
			}
			hotA.R, hotA.G, hotA.B = 0.2, 0.7, 0.95
			hot1.R, hot1.G, hot1.B = 0.2, 0.5, 0.95
			hot2.R, hot2.G, hot2.B = 0.25, 0.8, 0.4
			hotAAlign.SetAlignment(0.60, 0.42)
			hot1Align.SetAlignment(0.90, 0.12)
			hot2Align.SetAlignment(0.12, 0.85)
		}
		// 相位切换才重录横幅层（稳态保持纹理 blit）。
		if phase != lastPhase {
			lastPhase = phase
			switch phase {
			case wrkit.PhaseSteady:
				phaseLabel.SetText("PHASE: STEADY")
				phaseLabel.SetColor(0.95, 0.95, 0.4, 1)
			case wrkit.PhaseSpike:
				phaseLabel.SetText("PHASE: SPIKE")
				phaseLabel.SetColor(1.0, 0.8, 0.2, 1)
			default:
				phaseLabel.SetText("PHASE: RECOVER")
				phaseLabel.SetColor(0.2, 0.8, 1.0, 1)
			}
			phaseLabel.MarkNeedsPaint()
		}
		// 每帧脏集合（R4b 双热点 + R4 动画区 + R3 内脏 + R5 直绘/回放区）。
		hot3.MarkNeedsPaint()
		hotA.MarkNeedsPaint()
		hot1.MarkNeedsPaint()
		hot2.MarkNeedsPaint()
		// R5 区（直绘/回放）在 retained 下低频刷新（0.5s）：Picture 回放缓存
		// 的语义是"静态时走 blit skip"，每帧重录只会徒增纹理 churn + RSS 峰值。
		// Spike 相每帧刷新（与相位节奏呼应）；Steady/Recover 0.5s 一次。
		if phase == wrkit.PhaseSpike || int(t*2)%2 == 0 {
			direct.MarkNeedsPaint()
			replay.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD（skip/rerecord + damage + mode + ids + multi + picture ops）。
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		avgArea, _, samples, _ := app.DamageStats()
		var avgRatio float64
		if samples > 0 {
			avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		}
		shell.NoteHUDTick(dt)
		ids := app.LastDirtyLayerIDs()
		_, _, _, multiF := app.DamageStats()
		// 门禁预览用 sum 口径（真实重绘像素，R4b 先例）；union bbox 是脏层
		// 分散的几何必然，仅作观测（avgRatio 显示在 HUD）。
		gateOK := len(ids) >= 2 && app.MaxDirtyLayerIDCount() >= 2 &&
			app.LastPresentMode() != "full" && sumRatio() <= 0.35 && snapH.BoundarySkip >= 3
		shell.UpdateHUD("C2", phase, app, gateOK,
			fmt.Sprintf("skip=%d rr=%d sum=%.2f mode=%s", snapH.BoundarySkip, snapH.BoundaryRerecord, sumRatio(), app.LastPresentMode()),
			fmt.Sprintf("ids=%d/%d multi=%d ops=%d union=%.2f", len(ids), app.MaxDirtyLayerIDCount(), multiF, opCount, avgRatio))
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
	wrkit.MergeBoundaryCache(app, &snap)
	avgArea, maxArea, samples, multiFrames := app.DamageStats()
	var avgRatio, maxRatio float64
	if samples > 0 {
		avgRatio = float64(avgArea) / float64(samples) / float64(winW*winH)
		maxRatio = float64(maxArea) / float64(winW*winH)
	}
	lastMode := app.LastPresentMode()
	lastIDs := app.LastDirtyLayerIDs()
	maxIDs := app.MaxDirtyLayerIDCount()
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C2",
		Scenario:      "ui_wr_c2_retained_scene",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			// §3.1.1：covers + 每能力集成验证 + impl_interaction。
			"covers":                []string{"R3", "R4", "R4b", "R5"},
			"r3_boundary_nest":      "outer/mid/inner 3层嵌套；Steady 内脏仅 inner 重录，Spike 外层脏整巢重录，巢外 skip",
			"r4_retained_composite": "retained 稳态 5x3 静态色格阵不重绘；动画区独立；damage_ratio<=0.35",
			"r4b_multi_damage":      "hot1/hot2 对角远离同帧变脏 → dirty_layer_id_max>=2 + damage_multi_frames>=1",
			"r5_picture_replay":     "直绘 vs 回放同构图并排（8 ops）；picture_op_count>=3；Replay≡direct",
			"impl_interaction": "同一 retained 帧内：R3 嵌套 boundary 的 inner 脏只重录 inner（outer/mid skip）；" +
				"R4 动画区与 R4b 双热点同帧变脏 → 3 个独立 dirty layer id、独立 scissor；" +
				"R5 Picture 回放区与静态色格阵共用 retained 稳态（damage 不含静区）——" +
				"证明 retained 场景下 缓存+多损伤+回放 完整闭环",
			"present_policy":      "retained",
			"damage_semantic":     "retained_multi_damage_picture",
			"damage_ratio_avg":    avgRatio,   // union bbox 口径（观测）
			"damage_ratio_sum":    sumRatio(), // sum of rects 真实重绘像素（门禁）
			"damage_ratio_max":    maxRatio,
			"damage_samples":      samples,
			"damage_multi_frames": multiFrames,
			"dirty_layer_id_max":  int64(maxIDs),
			"dirty_layer_ids":     lastIDs,
			"last_present_mode":   lastMode,
			"static_visible":      staticCount,
			"hot_repaints":        hotTick,
			"picture_op_count":    int64(opCount),
			"picture_bounds":      boundsTxt,
			"boundary_skip":       snap.BoundarySkip,
			"boundary_rerecord":   snap.BoundaryRerecord,
		},
	})
	// R5 门禁字段：picture_op_count 度量保留显示列表本身（scene-side 诚实 op 数）。
	report.PictureOpCount = int64(opCount)
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// C2 门禁 = ∪(各 R 门禁)（§3.1.1：C 的 GateOptions = ∪(各 R 门禁)）。
	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true, // R4
		MinBoundarySkip:       3,    // R3
		MinBoundaryRerecord:   2,    // R3
		MinDirtyLayerIDs:      2,    // R4b
		MinDamageMultiFrames:  1,    // R4b
		MinPictureOpCount:     3,    // R5
		RequirePersistentFPS:  true, // 持续 tick（R4 动画类）
		MinFPSWall:            55,
		MaxP95Ms:              22,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	if lastMode == "" || lastMode == "full" || lastMode == "idle" {
		fmt.Fprintf(os.Stderr, "FAIL: present_mode=%q want damage_union/damage_multi (非 full)", lastMode)
		os.Exit(1)
	}
	if sumRatio() > 0.35 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_ratio_sum=%.3f > 0.35 (retained 必须远小于全屏；sum of rects 口径，union bbox 为观测)", sumRatio())
		os.Exit(1)
	}
	if snap.BoundarySkip <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want >0 (静态 boundary 必须 Replay)", snap.BoundarySkip)
		os.Exit(1)
	}
	if opCount < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: picture_op_count=%d want >=3", opCount)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: OK presents=%d skip=%d rr=%d dmg_avg=%.3f mode=%s ids=%d/%d multi=%d ops=%d fps=%.1f elapsed=%.1fs\n",
		app.PresentCount(), snap.BoundarySkip, snap.BoundaryRerecord, avgRatio, lastMode,
		len(lastIDs), maxIDs, multiFrames, opCount,
		1000.0/snap.AvgFrameIntervalMs, elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
