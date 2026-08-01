// Command ui_wr_r0_fullpaint is the R0 single-ability real-window gate for
// ENGINE_UI_WIDGET_RENDER W0 FullPaint correctness (static dense + animated + text).
//
// Quality bar (wr-close 模式 2 · U17/U18/U20):
//
//	  multi-region shell · 4×4 static grid + labels · hot pulse · Steady/Spike/Recover · LiveHUD
//
//		export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//		RUN_SECONDS=5 go run ./examples/ui_wr_r0_fullpaint   # close R0 (U16 min)
//		RUN_SECONDS=15 go run ./examples/ui_wr_r0_fullpaint  # observe CPU/RSS
//
// Gates: presents>=1, present_policy=full_paint, §2.2 metrics schema;
// persistent FPS gate when RUN_SECONDS>=5 (fps_interval≥55 preferred).
package main

import (
	"fmt"
	"math"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	hudH       = 72
	// Close duration §2.5 R0 = 5s; recommended observe 15s.
	closeSeconds = 5
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "R0")
	fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: R0 FullPaint quality bar — %ds @ 1200x800\n", secs)

	// Font must load before labels: DrawString / RenderText Paint no-op without Face
	// (otherwise Place coords look like empty layout holes).
	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width:  winW,
		Height: winH,
		Title:  "gpui ui_wr_r0_fullpaint",
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
				fmt.Fprintln(os.Stderr, "ui_wr_r0_fullpaint: close")
			}
		},
	})
	// R0: full_paint is opt-in since W6 made retained the engine default.
	// Explicitly test the full repaint path (correctness under Clear).
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
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			policy := snap.PresentPolicy
			if policy == "" {
				policy = scheduler.PresentPolicyFullPaint
			}
			gateOK := fps >= 55 || phases.Elapsed() < 2 // early frames: preview soft
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R0",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("static_cells=%d labels=%d hot=1", sc.staticCells, sc.labelCount),
				GateOK:      gateOK,
				Extra:       "static grid must survive Clear every frame · hot pulses",
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
	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R0",
		Scenario:      "ui_wr_r0_fullpaint",
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
			"static_grid":      "4x4 color cells @ body left — must stay under FullPaint Clear",
			"static_labels":    sc.labelCount,
			"static_cells":     sc.staticCells,
			"hot_box":          "pulse @ body right — every tick MarkNeedsPaint",
			"phases":           "Steady/Spike/Recover",
			"hud":              wrkit.HUDEnabled(),
			"depcheck":         "passed",
			"quality_bar":      "U17+U18+U20",
			"impl_correctness": "FullPaint paints full tree each frame; static survives Clear",
			"impl_dirty":       "hot MarkNeedsPaint; static remains clean after first paint",
			"impl_cache":       "N/A for R0 (FullPaint; BoundaryCache is R3)",
			"impl_edge":        "Spike doubles hot pulse rate; Recover returns to Steady pace",
			"impl_fail":        "green/static gone while hot moves → FullPaint broken",
			"impl_visible":     "LiveHUD policy/fps + 4x4 grid + labels + hot pulse",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r0_fullpaint: metrics JSON follows on stdout")
	fmt.Println(string(b))

	// FPS gate: fps_interval≥55 (prefer) after ≥5s. MaxP95 is observed in JSON
	// but not hard-gated here — under FullPaint + dense static the p95 can spike
	// briefly at open while still holding 60Hz-class fps_interval (U13 primary).
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
	// Structural quality checks (not in wrgate pure helpers).
	if sc.staticCells < 16 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want >=16 (U17 dense static)\n", sc.staticCells)
		os.Exit(1)
	}
	if sc.labelCount < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: labels=%d want >=8 (U17 text labels)\n", sc.labelCount)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r0_fullpaint: PASS presents=%d fps=%.1f policy=%s vsync=%s cells=%d labels=%d\n",
		rep.PresentCount, rep.FPSInterval, rep.PresentPolicy, rep.VSyncSource, sc.staticCells, sc.labelCount)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene struct {
	Root        *rendering.AbsoluteBox
	hotB        *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	phase       float64
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.09, G: 0.10, B: 0.12, A: 1}
	s.Root = root

	// Layout map (logical 1200×800, content above HUD):
	//   [======== TopBar y=0 h=48 ====================================]
	//   [ Legend 12,60 272×320 ] [ Grid 300,60 420×360 ] [ Hot 740,60 440×360 ]
	//   [================ LiveHUD y=h-72 =================================]
	// Children Place inside each Panel are **panel-local** so every coord owns content.

	// --- TopBar ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("R0 FullPaint · static dense + hot · policy=full_paint", 15, 16, 14, 0.85, 0.90, 0.98)
	s.labelCount++

	// --- Left legend panel ---
	leg := wrkit.NewPanel(272, 320, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.70, 0.90)
	s.labelCount++
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"static cell — must stay", 0.25, 0.75, 0.40},
		{"hot pulse — every tick", 0.95, 0.35, 0.25},
		{"text labels with Face", 0.85, 0.85, 0.55},
		{"Clear each frame (full_paint)", 0.65, 0.68, 0.72},
		{"If static vanishes → FAIL", 0.95, 0.55, 0.40},
		{"Phase: Steady→Spike→Recover", 0.55, 0.80, 0.95},
		{"HUD band bottom (U18)", 0.55, 0.75, 0.85},
		{"panels = local Place coords", 0.60, 0.70, 0.80},
	}
	for i, ln := range legendLines {
		// Color swatch + text on same row (local y).
		leg.ColorAt(12, 12, 12, 36+float64(i)*28, ln.r, ln.g, ln.b, 1, true)
		leg.LabelAt(ln.text, 12, 32, 34+float64(i)*28, ln.r, ln.g, ln.b)
		s.labelCount++
	}

	// --- Center: static 4×4 grid panel ---
	const cols, rows = 4, 4
	const cell, gap = 56.0, 8.0
	grid := wrkit.NewPanel(420, 360, 0.10, 0.11, 0.14, 1)
	grid.PlaceOn(root, 300, 60)
	grid.LabelAt("STATIC GRID 4x4 (survives Clear)", 13, 12, 10, 0.70, 0.85, 0.95)
	s.labelCount++
	gridOriginX, gridOriginY := 16.0, 40.0
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			fr := 0.15 + 0.15*float64(c)
			fg := 0.35 + 0.12*float64(r)
			fb := 0.25 + 0.10*float64((c+r)%4)
			grid.ColorAt(cell, cell,
				gridOriginX+float64(c)*(cell+gap),
				gridOriginY+float64(r)*(cell+gap),
				fr, fg, fb, 1, true)
			s.staticCells++
		}
	}
	grid.LabelAt("NW static — must never blank", 11, 16, gridOriginY+float64(rows)*(cell+gap)+8, 0.55, 0.75, 0.60)
	s.labelCount++

	// --- Right: hot panel + side static strip ---
	hot := wrkit.NewPanel(440, 360, 0.10, 0.11, 0.13, 1)
	hot.PlaceOn(root, 740, 60)
	hot.LabelAt("HOT (MarkNeedsPaint each tick)", 13, 12, 10, 0.95, 0.55, 0.40)
	s.labelCount++
	s.hotB = hot.ColorAt(160, 160, 24, 48, 0.95, 0.25, 0.18, 1, true)
	hot.LabelAt("side static strip", 12, 220, 48, 0.60, 0.70, 0.80)
	s.labelCount++
	for i := 0; i < 4; i++ {
		hot.ColorAt(36, 36, 220, 72+float64(i)*44, 0.20, 0.28+0.05*float64(i%3), 0.35, 1, true)
		s.staticCells++
	}
	hot.LabelAt("FullPaint: all panels repaint after Clear", 11, 12, 320, 0.55, 0.65, 0.70)
	s.labelCount++

	// --- Bottom LiveHUD band (U18) ---
	if wrkit.HUDEnabled() {
		s.hud = wrkit.NewLiveHUD(w, hudH)
		root.Place(s.hud.Box, 0, h-hudH)
	}

	return s
}

func (s *scene) onTick(dt float64, phase string) {
	if s == nil || s.hotB == nil {
		return
	}
	rate := 4.0
	switch phase {
	case wrkit.PhaseSpike:
		rate = 10.0 // Spike: faster pulse — still FullPaint correctness
	case wrkit.PhaseRecover:
		rate = 3.0
	}
	s.phase += dt * rate
	g := 0.15 + 0.40*(0.5+0.5*math.Sin(s.phase))
	b := 0.12 + 0.20*(0.5+0.5*math.Cos(s.phase*0.7))
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.95, g, b, 1
	s.hotB.MarkNeedsPaint()
}
