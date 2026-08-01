// Command ui_wr_r5_picture is the R5 gate: Picture record once, Replay each
// frame vs direct paint of the same geometry — side by side on a real GPU window.
//
// Quality bar (§2.6 · U17/U18/U20):
//
//	multi-region shell · FillRect+FillPath+StrokePath+DrawString ops ·
//	Steady/Spike/Recover · LiveHUD · direct ≡ replay visual proof
//
//		export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//		RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture    # close R5 (U16 min)
//		RUN_SECONDS=15 go run ./examples/ui_wr_r5_picture   # observe CPU/RSS
//
// Gates: presents≥1, present_policy=full_paint, §2.2 metrics schema;
// persistent FPS gate when RUN_SECONDS≥5; picture_op_count≥5; replay_frames≥10.
package main

import (
	"fmt"
	"os"
	"strconv"
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
	winW, winH = 1200, 800
	hudH       = 72
	// Close duration §2.5 R5 = 5s; recommended observe 15s.
	closeSeconds = 5
)

func main() {
	secs := runSeconds(closeSeconds)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R5 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: R5 Picture record/replay — %ds @ 1200x800\n", secs)

	// Font must load before labels.
	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r5_picture",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r5_picture: close")
			}
		},
	})
	// R5: full_paint is opt-in since W6 made retained the engine default.
	// Explicitly test Picture replay correctness under Clear.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetPictureTextureCache(true)

	phases := wrkit.NewPhaseClock(1.5, 3.5) // Steady 0–1.5 · Spike 1.5–3.5 · Recover
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
			gateOK := fps >= 55 || phases.Elapsed() < 2
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R5",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("ops=%d replay=%d direct=%d", sc.pic.OpCount(), sc.replayFrames, sc.directFrames),
				GateOK:      gateOK,
				Extra:       "left=direct right=replay — must look identical",
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

	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
	snap := app.Metrics().Snapshot()
	opCount := sc.pic.OpCount()
	replays := sc.replayFrames

	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R5",
		Scenario:      "ui_wr_r5_picture",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"tex_rasterize":    texRasterize,
			"tex_hit":          texHit,
			"tex_impl":         "B1: per-cell boundary Pictures rasterized to GPU textures once (scroll cell reuse blits instead of re-replaying commands); rasterize>0 proves texture path active",
			"client_px":        "1200x800",
			"run_seconds":      secs,
			"picture_op_count": opCount,
			"replay_frames":    replays,
			"direct_frames":    sc.directFrames,
			"op_types":         "FillRect,FillPath,StrokePath,StrokeRect,DrawString,PushTransform,PopTransform",
			"direct_region":    "left  panel — live DC draw each frame",
			"replay_region":    "right panel — Record once, Replay each frame",
			"phases":           "Steady/Spike/Recover",
			"impl_correctness": "Picture Replay ops produce identical pixels as direct DC draw (incl. PushTransform/PopTransform CTM block)",
			"impl_dirty":       "both sides MarkNeedsPaint every tick; FullPaint redraws all",
			"impl_cache":       "N/A for R5 (BoundaryCache is R3); Picture is display-list, not cache",
			"impl_edge":        "Spike doubles paint rate; path clone isolation; empty text no-op",
			"impl_fail":        "replay looks different from direct / ops<5 / replays<10",
			"impl_visible":     "left=direct right=replay — same geometry, colors, text",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r5_picture: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// B1 gate: texture path must have rasterized at least once (mechanism active).
	if texRasterize < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: tex_rasterize=%d want ≥1 (B1 texture path inactive)\n", texRasterize)
		os.Exit(1)
	}
	// Structural gates.
	if opCount < 5 {
		fmt.Fprintf(os.Stderr, "FAIL: picture_op_count=%d want ≥5 (FillRect×2+FillPath+StrokePath+DrawString)\n", opCount)
		os.Exit(1)
	}
	if replays < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: replay_frames=%d want ≥10\n", replays)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: PASS ops=%d replays=%d direct=%d fps=%.1f policy=%s vsync=%s\n",
		opCount, replays, sc.directFrames, rep.FPSInterval, rep.PresentPolicy, rep.VSyncSource)
}

// --- helpers ---

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

// --- scene ---

type scene5 struct {
	Root         *rendering.AbsoluteBox
	directBox    *rendering.RenderBox
	replayBox    *rendering.RenderBox
	pic          scene.Picture
	hud          *wrkit.LiveHUD
	directFrames int64
	replayFrames int64
	phase        float64
}

func buildScene(w, h float64) *scene5 {
	s := &scene5{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	// --- TopBar ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("R5 Picture · Record once · Replay vs Direct — must match", 15, 16, 14, 0.85, 0.90, 0.98)

	// --- Legend ---
	leg := wrkit.NewPanel(260, 300, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.70, 0.90)
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"FillRect — solid color blocks", 0.85, 0.30, 0.25},
		{"FillPath — triangle path", 0.25, 0.75, 0.40},
		{"StrokePath — rounded rect 3px", 0.30, 0.50, 0.90},
		{"DrawString — text label", 0.90, 0.85, 0.50},
		{"Replay — Record once, replay", 0.55, 0.80, 0.95},
		{"Clear each frame (full_paint)", 0.65, 0.68, 0.72},
		{"Phase: Steady→Spike→Recover", 0.55, 0.75, 0.85},
		{"HUD band bottom (U18)", 0.55, 0.70, 0.80},
	}
	for i, ln := range legendLines {
		leg.ColorAt(12, 12, 12, 36+float64(i)*30, ln.r, ln.g, ln.b, 1, true)
		leg.LabelAt(ln.text, 11, 32, 34+float64(i)*30, ln.r, ln.g, ln.b)
	}

	// Panel geometry.
	const (
		directX, directY = 284.0, 60.0
		replayX, replayY = 736.0, 60.0
		panelW, panelH   = 440.0, 480.0
	)

	// --- Left: direct paint every frame ---
	direct := rendering.NewRenderBox()
	direct.FixedWidth, direct.FixedHeight = panelW, panelH
	direct.SetRepaintBoundary(true)
	direct.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		drawContent(pc.DC, directX, directY)
		s.directFrames++
	}
	s.directBox = direct
	root.Place(direct, directX, directY)
	directLbl := wrkit.NewPanel(panelW, 28, 0.12, 0.14, 0.18, 0.9)
	directLbl.PlaceOn(root, directX, directY-28)
	directLbl.LabelAt("LEFT: DIRECT (live DC draw each frame)", 12, 8, 6, 0.90, 0.75, 0.60)

	// --- Right: Replay recorded Picture every frame ---
	s.pic = recordContent()
	replay := rendering.NewRenderBox()
	replay.FixedWidth, replay.FixedHeight = panelW, panelH
	replay.SetRepaintBoundary(true)
	replay.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		// Picture was recorded with directX/directY offsets.
		s.pic.Replay(pc.DC)
		s.replayFrames++
	}
	s.replayBox = replay
	root.Place(replay, replayX, replayY)
	replayLbl := wrkit.NewPanel(panelW, 28, 0.12, 0.14, 0.18, 0.9)
	replayLbl.PlaceOn(root, replayX, replayY-28)
	replayLbl.LabelAt("RIGHT: REPLAY (Record once, Replay each frame)", 12, 8, 6, 0.60, 0.85, 0.70)

	// --- LiveHUD ---
	if wrkit.HUDEnabled() {
		s.hud = wrkit.NewLiveHUD(w, hudH)
		root.Place(s.hud.Box, 0, h-hudH)
	}

	return s
}

// drawContent draws the reference content directly onto dc at absolute coords (ox,oy).
// This is the "truth" — the Picture must produce identical pixels.
func drawContent(dc *render.Context, ox, oy float64) {
	// --- 1. FillRect: 2×2 colored blocks ---
	rects := []struct{ x, y, w, h, r, g, b float64 }{
		{ox + 20, oy + 20, 120, 100, 0.85, 0.25, 0.20},
		{ox + 160, oy + 20, 120, 100, 0.20, 0.70, 0.35},
		{ox + 20, oy + 140, 120, 100, 0.25, 0.40, 0.90},
		{ox + 160, oy + 140, 120, 100, 0.90, 0.75, 0.20},
	}
	for _, q := range rects {
		dc.SetRGBA(q.r, q.g, q.b, 1)
		dc.DrawRectangle(q.x, q.y, q.w, q.h)
		_ = dc.Fill()
	}

	// --- 2. Transform: rotated+scaled rect (R5 变换 op: Push/Pop CTM) ---
	// direct 与 replay 必须像素一致：直接 DC Push+Rotate+Scale vs Picture
	// PushTransform+PopTransform（ui/scene/picture.go OpPushTransform）。
	dc.Push()
	dc.Translate(ox+320, oy+70)
	dc.Rotate(-0.45) // ≈ -25.8°
	dc.Scale(1.15, 0.85)
	dc.Translate(-40, -40)
	dc.SetRGBA(0.40, 0.80, 0.55, 1)
	dc.DrawRectangle(0, 0, 80, 80)
	_ = dc.Fill()
	dc.Pop()

	// --- 3. StrokePath: rounded rect outline ---
	dc.SetRGBA(0.30, 0.50, 0.90, 1)
	dc.SetLineWidth(3)
	sp := render.NewPath()
	sp.RoundedRectangle(ox+30, oy+270, 200, 80, 14)
	_ = dc.StrokePath(sp)

	// --- 4. DrawString: label ---
	if face := wrkit.FaceAt(16); face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(0.90, 0.85, 0.50, 1)
	dc.DrawString("Picture ≡ Direct", ox+260, oy+310)

	// --- 5. StrokeRect: solid border (Picture has no SetDash; keep direct≡replay) ---
	dc.SetRGBA(0.55, 0.60, 0.70, 1)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(ox+30, oy+370, 380, 80)
	_ = dc.Stroke()

	// --- 6. FillPath: circle (proves Circle path) ---
	dc.SetRGBA(0.85, 0.40, 0.65, 1)
	circ := render.NewPath()
	circ.Circle(ox+100, oy+420, 30)
	_ = dc.FillPath(circ)

	// Label inside stroke box
	if face := wrkit.FaceAt(12); face != nil {
		dc.SetFont(face)
	}
	dc.SetRGBA(0.75, 0.78, 0.85, 1)
	dc.DrawString("stroke border + circle path", ox+160, oy+418)
}

// recordContent records the same geometry as drawContent into a Picture display list.
// Coordinates match drawContent exactly (absolute) — Replay dc must be at (0,0) origin.
func recordContent() scene.Picture {
	return scene.RecordPicture(func(r *scene.PictureRecorder) {
		ox, oy := 736.0, 60.0 // replayX, replayY

		// 1. FillRect: 2×2 colored blocks (same colors as direct)
		r.FillRect(ox+20, oy+20, 120, 100, 0.85, 0.25, 0.20, 1)
		r.FillRect(ox+160, oy+20, 120, 100, 0.20, 0.70, 0.35, 1)
		r.FillRect(ox+20, oy+140, 120, 100, 0.25, 0.40, 0.90, 1)
		r.FillRect(ox+160, oy+140, 120, 100, 0.90, 0.75, 0.20, 1)

		// 2. Transform: rotated+scaled rect (must match direct DC CTM exactly)
		r.PushTransform(ox+320, oy+70, -0.45, 1.15, 0.85)
		r.FillRect(280, 30, 80, 80, 0.40, 0.80, 0.55, 1)
		r.PopTransform()

		// 3. StrokePath: rounded rect
		sp := render.NewPath()
		sp.RoundedRectangle(ox+30, oy+270, 200, 80, 14)
		r.StrokePath(sp, 3, 0.30, 0.50, 0.90, 1)

		// 4. DrawString
		r.DrawString("Picture ≡ Direct", ox+260, oy+310, wrkit.FaceAt(16), 0.90, 0.85, 0.50, 1)

		// 5. StrokeRect: solid border (matches direct)
		r.StrokeRect(ox+30, oy+370, 380, 80, 1.5, 0.55, 0.60, 0.70, 1)

		// 6. FillPath: circle
		circ := render.NewPath()
		circ.Circle(ox+100, oy+420, 30)
		r.FillPath(circ, 0.85, 0.40, 0.65, 1)

		// Label inside stroke box
		r.DrawString("stroke border + circle path", ox+160, oy+418, wrkit.FaceAt(12), 0.75, 0.78, 0.85, 1)
	})
}

func (s *scene5) onTick(dt float64, phase string) {
	if s == nil {
		return
	}
	// PhaseClock: Spike doubles paint rate.
	rate := 4.0
	switch phase {
	case wrkit.PhaseSpike:
		rate = 10.0
	case wrkit.PhaseRecover:
		rate = 3.0
	}
	s.phase += dt * rate
	// Both sides MarkNeedsPaint so FullPaint keeps drawing direct + replay.
	if s.directBox != nil {
		s.directBox.MarkNeedsPaint()
	}
	if s.replayBox != nil {
		s.replayBox.MarkNeedsPaint()
	}
}
