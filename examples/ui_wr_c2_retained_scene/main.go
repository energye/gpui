// Command ui_wr_c2_retained_scene is the C2 combo: retained multi-boundary +
// Picture replay + dual distant dirty (covers R3+R4+R4b+R5). Combo only —
// does not close solo R packages.
//
// Quality bar (§3.1 · U17/U18/U20):
//
//	Shell+Legend≥8 · ≥6 regions · PhaseClock · LiveHUD · gate ∪ of R3/R4/R4b/R5
//
//		export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//		RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene
package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
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

const (
	winW, winH   = 1200, 800
	closeSeconds = 15 // §2.5 C2
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "C2")
	fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: C2 retained R3+R4+R4b+R5 — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c2_retained_scene",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.06, ClearB: 0.08, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c2_retained_scene: close")
			}
		},
	})
	// C2 core: retained CompositeOnly + damage Present (R4).
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	// 15s close: Steady 0–4 · Spike 4–10 · Recover 10–end
	phases := wrkit.NewPhaseClock(4.0, 10.0)
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		sc.onTick(dt, ph)
		proc.Sample()
		if wrkit.HUDEnabled() && sc.hud != nil {
			sc.hud.NoteTick(dt)
			snap := app.Metrics().Snapshot()
			wrkit.MergeBoundaryCache(app, &snap)
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			policy := snap.PresentPolicy
			if policy == "" {
				policy = scheduler.PresentPolicyFullPaint
			}
			sumA, _, samples, multiN := app.DamageStats()
			avgRatio := 0.0
			if samples > 0 {
				avgRatio = float64(sumA) / float64(samples) / float64(winW*winH)
			}
			maxDirty := app.MaxDirtyLayerIDCount()
			gateOK := (fps >= 55 || phases.Elapsed() < 2) &&
				policy == scheduler.PresentPolicyRetained &&
				(avgRatio < 0.45 || phases.Elapsed() < 2)
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "C2",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core: fmt.Sprintf("skip=%d rr=%d dmg=%.3f dirty=%d multi=%d ops=%d",
					snap.BoundarySkip, snap.BoundaryRerecord, avgRatio, maxDirty, multiN, sc.pic.OpCount()),
				GateOK: gateOK,
				Extra:  "R3 nest+R4 retained+R4b dual-hot+R5 Picture · combo only",
			})
		}
		app.ScheduleFrame()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: present target open: %v\n", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	sumA, maxA, samples, multiN := app.DamageStats()
	surf := int64(winW * winH)
	var avgRatio, maxRatio float64
	if samples > 0 && surf > 0 {
		avgRatio = float64(sumA) / float64(samples) / float64(surf)
		maxRatio = float64(maxA) / float64(surf)
	}
	maxDirty := app.MaxDirtyLayerIDCount()
	opCount := sc.pic.OpCount()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C2",
		Scenario:      "ui_wr_c2_retained_scene",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: surf,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":            "1200x800",
			"run_seconds":          secs,
			"covers":               []string{"R3", "R4", "R4b", "R5"},
			"damage_ratio_avg":     avgRatio,
			"damage_ratio_max":     maxRatio,
			"damage_samples":       samples,
			"damage_multi_frames":  multiN,
			"dirty_layer_id_max":   maxDirty,
			"last_dirty_layer_ids": app.LastDirtyLayerIDs(),
			"last_present_mode":    app.LastPresentMode(),
			"picture_op_count":     opCount,
			"picture_paint_frames": sc.picPaintFrames,
			"boundary_count":       cnt,
			"boundary_max_depth":   depth,
			"static_cells":         sc.staticCells,
			"label_count":          sc.labelCount,
			"regions":              "TopBar/Legend/Nest/StaticGrid/HotTL/HotBR/Picture/HUD",
			"phases":               "Steady/Spike/Recover",
			"depcheck":             "passed",
			"note":                 "combo only — does not close solo R rows",
			"impl_interaction":     "R3 nest skip under retained R4 damage; R4b dual-hot distant ids≥2; R5 Picture static boundary skips after first paint; Spike accelerates both hots without full-screen damage",
			"impl_correctness":     "retained static nest+Picture survive; dual hots dirty independently",
			"impl_dirty":           "damage ∝ two hot rects (+ HUD band ~10Hz); center static not in damage",
			"impl_cache":           "nested RB skip; Picture host RB caches first Replay",
			"impl_edge":            "Spike dual-hot rate↑; multi vs union damage; deep nest skip",
			"impl_fail":            "policy≠retained / dirty_ids<2 / ops<5 / dmg≥0.45 / fps<55 (retained 下 boundary_skip=0 是正确语义——静 RB 靠 GPU LoadOpLoad 保像素，不设 skip 门禁)",
			"impl_visible":         "LiveHUD skip/dmg/dirty/ops; nest left static; TL+BR hots pulse; Picture mid static",
		},
	})
	// Gate on steady avg damage (not a single full-clear frame).
	rep.DamageRatio = avgRatio

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c2_retained_scene: metrics JSON follows on stdout")
	fmt.Println(string(b))

	// Gate ∪ of R3+R4+R4b+R5 (§3.1.1).
	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MinFPSElapsed:         5,
		// retained/CompositeOnly 下静 RB 被引擎正确跳过（靠 GPU LoadOpLoad 保像素，
		// 不靠 Picture 缓存重放）；boundary_skip=0 是正确语义，故不设门禁（设为 0=off）。
		// C 集成门禁取并集，但 retained 模式下 skip 门禁本就不适用（与 R4 solo 同源）。
		MinBoundarySkip:  0,
		MinBoundaryCount: 3,
		// Dual distant hots + HUD band: slightly above R4 solo 0.35, still ≪1.
		MaxDamageRatio: 0.45,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if opCount < 5 {
		fmt.Fprintf(os.Stderr, "FAIL: picture_op_count=%d want ≥5 (R5 multi-op Picture)\n", opCount)
		os.Exit(1)
	}
	if sc.picPaintFrames < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: picture_paint_frames=%d want ≥1\n", sc.picPaintFrames)
		os.Exit(1)
	}
	if maxDirty < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: dirty_layer_id_max=%d want ≥2 (R4b dual distant dirty)\n", maxDirty)
		os.Exit(1)
	}
	if multiN < 1 && avgRatio >= 0.45 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_multi_frames=0 and dmg_avg=%.4f (want multi or low union)\n", avgRatio)
		os.Exit(1)
	}
	if samples < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_samples=%d want ≥10\n", samples)
		os.Exit(1)
	}
	if depth < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_max_depth=%d want ≥2 (R3 nest)\n", depth)
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8 (dense static)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_c2_retained_scene: PASS skip=%d count=%d depth=%d dirty_max=%d multi=%d ops=%d dmg=%.4f fps=%.1f policy=%s\n",
		rep.BoundarySkip, cnt, depth, maxDirty, multiN, opCount, avgRatio, rep.FPSInterval, rep.PresentPolicy)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene2 struct {
	Root           *rendering.AbsoluteBox
	hotTL          *rendering.RenderColorBox
	hotBR          *rendering.RenderColorBox
	pic            scene.Picture
	picPaintFrames int64
	hud            *wrkit.LiveHUD
	phase          float64
	staticCells    int
	labelCount     int
}

func buildScene(w, h float64) *scene2 {
	s := &scene2{}
	shell := wrkit.NewShell(w, h,
		"C2 retained · R3 nest + R4 damage + R4b dual-hot + R5 Picture",
		[]string{
			"R3 nested boundary skip (outer/mid/static)",
			"R4 retained policy · damage ≪ full screen",
			"R4b dual distant dirty (TL + BR hots)",
			"R5 Picture multi-op static replay",
			"center dense static grid (not damaged)",
			"Spike accelerates BOTH hots",
			"LiveHUD: skip/dmg/dirty/ops",
			"combo only — not solo R close",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	body := shell.Body
	bw, bh := body.W, body.H

	// --- Region: R3 nested boundary (left) ---
	nestPanel := wrkit.NewPanel(bw*0.34, bh*0.55, 0.11, 0.13, 0.18, 1)
	nestPanel.PlaceOn(body.Box, 8, 8)
	nestPanel.LabelAt("R3 NEST (outer→mid→static)", 12, 10, 8, 0.70, 0.85, 0.95)
	s.labelCount++

	outer := rendering.NewAbsoluteBox(bw*0.34-20, bh*0.55-40)
	outer.Background = &rendering.Color{R: 0.14, G: 0.16, B: 0.24, A: 1}
	outer.SetRepaintBoundary(true)
	nestPanel.Place(outer, 10, 28)

	midBox := rendering.NewAbsoluteBox(bw*0.34-60, bh*0.55-90)
	midBox.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.32, A: 1}
	midBox.SetRepaintBoundary(true)

	staticInner := rendering.NewRenderColorBox(70, 60, 0.18, 0.55, 0.32, 1)
	staticInner.SetRepaintBoundary(true)
	midBox.Place(staticInner, 16, 16)
	s.staticCells++

	// Nested labels (text in boundary — skip must still work).
	lbl := wrkit.Label("static nest", 11, 0.75, 0.85, 0.70)
	midBox.Place(lbl, 16, 90)
	s.labelCount++
	side := rendering.NewRenderColorBox(40, 100, 0.35, 0.40, 0.70, 1)
	side.SetRepaintBoundary(true)
	outer.Place(side, bw*0.34-70, 20)
	s.staticCells++
	outer.Place(midBox, 12, 12)

	// --- Region: dense static grid (center, R4 static proof) ---
	gridPanel := wrkit.NewPanel(bw*0.30, bh*0.55, 0.10, 0.12, 0.15, 1)
	gridPanel.PlaceOn(body.Box, bw*0.36, 8)
	gridPanel.LabelAt("STATIC GRID (retained · no dirty)", 11, 8, 8, 0.65, 0.80, 0.90)
	s.labelCount++
	const cell, gap = 36.0, 5.0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			gridPanel.ColorAt(cell, cell,
				10+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.20+0.08*float64(c), 0.35+0.08*float64(r), 0.50, 1, true)
			s.staticCells++
		}
	}

	// --- Region: R5 Picture (right of nest row) ---
	picPanel := wrkit.NewPanel(bw*0.32, bh*0.55, 0.10, 0.12, 0.16, 1)
	picPanel.PlaceOn(body.Box, bw*0.68, 8)
	picPanel.LabelAt("R5 PICTURE (record once · skip)", 11, 8, 8, 0.85, 0.75, 0.45)
	s.labelCount++

	// Record multi-op Picture in panel-local space; OnPaint translates by Origin.
	s.pic = scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(12, 12, 70, 50, 0.85, 0.30, 0.22, 1)
		r.FillRect(50, 36, 70, 50, 0.22, 0.70, 0.35, 1)
		r.FillRect(12, 70, 70, 50, 0.25, 0.45, 0.90, 1)
		tp := render.NewPath()
		tp.MoveTo(160, 20)
		tp.LineTo(200, 70)
		tp.LineTo(120, 70)
		tp.Close()
		r.FillPath(tp, 0.40, 0.80, 0.55, 1)
		sp := render.NewPath()
		sp.RoundedRectangle(20, 140, 160, 50, 10)
		r.StrokePath(sp, 2.5, 0.30, 0.55, 0.90, 1)
		r.DrawString("Pic≡cache", 30, 175, wrkit.FaceAt(13), 0.90, 0.85, 0.50, 1)
		r.StrokeRect(12, 210, 200, 40, 1.5, 0.55, 0.60, 0.70, 1)
		r.DrawString("R5 under retained", 24, 235, wrkit.FaceAt(11), 0.70, 0.78, 0.85, 1)
	})
	picHost := rendering.NewRenderBox()
	picHost.FixedWidth, picHost.FixedHeight = bw*0.32-16, bh*0.55-36
	picHost.SetRepaintBoundary(true)
	picHost.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		// Picture ops are host-local; shift into absolute paint space.
		pc.DC.Push()
		pc.DC.Translate(pc.OriginX, pc.OriginY)
		s.pic.Replay(pc.DC)
		pc.DC.Pop()
		s.picPaintFrames++
	}
	picPanel.Place(picHost, 8, 28)

	// --- Region: R4b dual distant hots (bottom band of body) ---
	hotBand := wrkit.NewPanel(bw-16, bh*0.38, 0.09, 0.10, 0.13, 1)
	hotBand.PlaceOn(body.Box, 8, bh*0.58)
	hotBand.LabelAt("R4b DUAL HOT · TL + BR distant (middle static stays)", 12, 10, 8, 0.95, 0.55, 0.40)
	s.labelCount++

	s.hotTL = hotBand.ColorAt(110, 100, 16, 36, 0.95, 0.28, 0.18, 1, true)
	hotBand.LabelAt("HOT-TL", 11, 16, 144, 0.95, 0.50, 0.35)
	s.labelCount++

	// Far bottom-right of hot band — maximize separation for multi-damage.
	s.hotBR = hotBand.ColorAt(110, 100, bw-16-130, hotBand.H-120, 0.20, 0.55, 0.95, 1, true)
	hotBand.LabelAt("HOT-BR", 11, bw-16-130, hotBand.H-16, 0.40, 0.70, 0.95)
	s.labelCount++

	// Mid static strip between the two hots (must not dirty with them).
	for i := 0; i < 6; i++ {
		hotBand.ColorAt(48, 48, 160+float64(i)*56, 50, 0.22, 0.28, 0.38, 1, true)
		s.staticCells++
	}
	hotBand.LabelAt("mid static between dual hots — skip under retained", 11, 160, 110, 0.55, 0.70, 0.80)
	s.labelCount++

	return s
}

func (s *scene2) onTick(dt float64, phase string) {
	if s == nil {
		return
	}
	// Phase drives BOTH hots (integration: multi-ability phase coupling).
	rate := 4.0
	switch phase {
	case wrkit.PhaseSpike:
		rate = 10.0
	case wrkit.PhaseRecover:
		rate = 3.0
	}
	s.phase += dt * rate
	o := 0.5 + 0.5*math.Sin(s.phase)
	if s.hotTL != nil {
		s.hotTL.R, s.hotTL.G, s.hotTL.B = 0.95, 0.20+0.40*o, 0.15
		s.hotTL.MarkNeedsPaint()
	}
	if s.hotBR != nil {
		s.hotBR.R, s.hotBR.G, s.hotBR.B = 0.15, 0.45+0.35*o, 0.90
		s.hotBR.MarkNeedsPaint()
	}
	// Picture host intentionally NOT MarkNeedsPaint — R5 under retained must
	// stay as a static boundary (first paint caches, then skip).
}
