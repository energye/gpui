// Command ui_wr_r3b_compbits is the R3b single-ability real-window gate for
// compositing-bits / nested boundary discovery (ENGINE_UI_WIDGET_RENDER W1).
//
// Quality bar (§2.6.2 R3b): nested boundary chain (≥3 levels) with
// NeedsCompositing state visualization; periodic dirty at different levels
// validates compositing-chain depth; boundary_count≥3 + boundary_max_depth≥2
// + NeedsCompositing propagates correctly; proves control-tree compositing
// bits discovery is correct.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
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
	winW, winH   = 1200, 800
	closeSeconds = 8
	hudH         = 72.0
)

func main() {
	secs := runSeconds(closeSeconds)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R3b (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: R3b compositing bits / nest — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r3b_compbits",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	if cnt < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: scene boundary_count=%d want >=3\n", cnt)
		os.Exit(1)
	}
	if depth < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: scene boundary_max_depth=%d want >=2\n", depth)
		os.Exit(1)
	}

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r3b_compbits: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetPictureTextureCache(true)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)
	// Force compositing-bits walk (R3b discovery path).
	app.Pipeline().UpdateCompositingBits()
	if !sc.Outer.NeedsCompositing() {
		fmt.Fprintln(os.Stderr, "FAIL: outer boundary must NeedsCompositing after UpdateCompositingBits")
		os.Exit(1)
	}
	if !sc.Mid.NeedsCompositing() {
		fmt.Fprintln(os.Stderr, "FAIL: mid boundary must NeedsCompositing after UpdateCompositingBits")
		os.Exit(1)
	}

	phases := wrkit.NewPhaseClock(2.0, 4.5) // Steady 0–2 · Spike 2–4.5 · Recover
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
			ncOuter := "no"
			if sc.Outer.NeedsCompositing() {
				ncOuter = "yes"
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R3b",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("cnt=%d depth=%d ncOuter=%s", cnt, depth, ncOuter),
				GateOK:      gateOK,
				Extra:       "outer→mid→{static,hot} nest; NeedsCompositing propagates",
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

	ncOuter := sc.Outer.NeedsCompositing()
	ncMid := sc.Mid.NeedsCompositing()

	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3b",
		Scenario:      "ui_wr_r3b_compbits",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"tex_rasterize":           texRasterize,
			"tex_hit":                 texHit,
			"tex_impl":                "B1: per-cell boundary Pictures rasterized to GPU textures once (scroll cell reuse blits instead of re-replaying commands); rasterize>0 proves texture path active",
			"client_px":               "1200x800",
			"run_seconds":             secs,
			"nest":                    "outer AbsoluteBox RB → mid AbsoluteBox RB → {static leaf ColorBox RB, hot leaf ColorBox RB}",
			"boundary_count":          cnt,
			"boundary_max_depth":      depth,
			"needs_compositing_outer": ncOuter,
			"needs_compositing_mid":   ncMid,
			"g_metrics":               "skipped",
			"g_metrics_reason":        "R3b is not R9/R10; text/measure cache not in scope",
			"phase_script":            "Steady→Spike→Recover (hot pulse rate doubles in Spike)",
			"impl_correctness":        "UpdateCompositingBits walks tree; outer + mid boundaries become NeedsCompositing (child is RB)",
			"impl_dirty":              "boundary_count≥3 + boundary_max_depth≥2 validates compositing chain depth discovery",
			"impl_cache":              "N/A for R3b (BoundaryCache is R3); R3b proves compositing bits / discovery, not cache hits",
			"impl_edge":               "3-level nesting (root→outer RB→mid RB→leaf RB); static + hot leaves independent dirty cycles",
			"impl_fail":               "boundary_count=0 (no RB discovered) / depth=1 (no nesting) / NeedsCompositing not propagating up = FAIL",
			"impl_visible":            "outer + mid boundary outlines visible; depth=3 counter in HUD; NeedsCompositing=yes",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r3b_compbits: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MinBoundaryCount:       3,
		MinBoundaryMaxDepth:    2,
		MinBoundarySkip:        1, // clean static nest contributes skip
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
	if !ncOuter || !ncMid {
		fmt.Fprintf(os.Stderr, "FAIL: NeedsCompositing propagation: outer=%v mid=%v want both true\n", ncOuter, ncMid)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: PASS count=%d depth=%d skip=%d ncOuter=%v ncMid=%v fps=%.1f\n",
		cnt, depth, snap.BoundarySkip, ncOuter, ncMid, rep.FPSInterval)
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

type scene3b struct {
	Root  *rendering.AbsoluteBox
	Outer *rendering.AbsoluteBox
	Mid   *rendering.AbsoluteBox
	Hot   *rendering.RenderColorBox
	hud   *wrkit.LiveHUD

	phase float64
}

func buildScene(w, h float64) *scene3b {
	s := &scene3b{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.07, G: 0.08, B: 0.10, A: 1}
	s.Root = root

	// --- TopBar (U17 多区域 第 1 区) ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("R3b Compositing bits — nested boundary discovery, NeedsCompositing propagation", 14, 16, 14, 0.88, 0.92, 0.98)

	// --- Legend (U17 第 2 区, ≥8 行色块+文字) ---
	leg := wrkit.NewPanel(240, 540, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.75, 0.95)
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"Outer boundary (depth 1)", 0.30, 0.50, 0.90},
		{"Mid boundary (depth 2)", 0.55, 0.75, 0.85},
		{"Static leaf (depth 3)", 0.25, 0.75, 0.40},
		{"Hot leaf (depth 3, pulses)", 0.90, 0.30, 0.25},
		{"NeedsCompositing propagates up", 0.90, 0.85, 0.50},
		{"UpdateCompositingBits walk", 0.65, 0.68, 0.72},
		{"boundary_count ≥ 3", 0.55, 0.70, 0.80},
		{"boundary_max_depth ≥ 2", 0.55, 0.75, 0.85},
		{"PhaseClock: Steady/Spike/Recover", 0.55, 0.75, 0.85},
	}
	for i, ln := range legendLines {
		leg.ColorAt(14, 14, 12, 36+float64(i)*28, ln.r, ln.g, ln.b, 1, false)
		leg.LabelAt(ln.text, 11, 32, 36+float64(i)*28, ln.r, ln.g, ln.b)
	}

	// --- Nested chain: outer RB → mid RB → {static leaf RB, hot leaf RB} ---
	outer := rendering.NewAbsoluteBox(460, 380)
	outer.Background = &rendering.Color{R: 0.15, G: 0.18, B: 0.28, A: 1}
	outer.SetRepaintBoundary(true)
	s.Outer = outer

	mid := rendering.NewAbsoluteBox(320, 280)
	mid.Background = &rendering.Color{R: 0.22, G: 0.28, B: 0.38, A: 1}
	mid.SetRepaintBoundary(true)
	s.Mid = mid

	staticLeaf := rendering.NewRenderColorBox(90, 90, 0.20, 0.70, 0.35, 1)
	staticLeaf.SetRepaintBoundary(true)
	mid.Place(staticLeaf, 24, 24)

	hot := rendering.NewRenderColorBox(100, 100, 0.95, 0.25, 0.20, 1)
	hot.SetRepaintBoundary(true)
	mid.Place(hot, 180, 130)
	s.Hot = hot

	outer.Place(mid, 60, 50)
	root.Place(outer, 280, 90)

	// Static text label inside outer (proves text in nested RB).
	outerText := wrkit.NewPanel(460-40, 24, 0.18, 0.20, 0.24, 0.95)
	outerText.PlaceOn(root, 280+20, 90+10)
	outerText.LabelAt("outer RB (depth 1) — nest root", 11, 6, 16, 0.90, 0.85, 0.50)

	// --- Right side: dense static text grid (U17 第 3 区, ≥8 labels) ---
	gridX, gridY := 780.0, 90.0
	for row := 0; row < 6; row++ {
		for col := 0; col < 2; col++ {
			cell := wrkit.NewPanel(190, 32, 0.10, 0.11, 0.13, 0.85)
			cell.PlaceOn(root, gridX+float64(col)*200, gridY+float64(row)*38)
			cell.LabelAt(fmt.Sprintf("static %d-%d frozen", row, col), 11, 8, 18, 0.70, 0.75, 0.85)
		}
	}

	// --- LiveHUD (U18 窗内可见指标) ---
	if wrkit.HUDEnabled() {
		s.hud = wrkit.NewLiveHUD(w, hudH)
		root.Place(s.hud.Box, 0, h-hudH)
	}

	return s
}

func (s *scene3b) onTick(dt float64, phase string) {
	if s == nil || s.Hot == nil {
		return
	}
	// PhaseClock drives pulse rate — Spike doubles frequency.
	rate := 4.0
	switch phase {
	case wrkit.PhaseSpike:
		rate = 8.0
	case wrkit.PhaseRecover:
		rate = 3.0
	}
	s.phase += dt * rate
	g := 0.15 + 0.35*(0.5+0.5*math.Sin(s.phase))
	s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = 0.95, g, 0.20, 1
	s.Hot.MarkNeedsPaint()
}
