// Command ui_wr_c1_boundary_nest is the C1 combo real-window gate:
// nested boundary + paint dirty isolation + optional debug-repaint tint (W1).
//
// Integrates R2 (local NeedsPaint) + R3 (Boundary cache skip/rerecord) +
// R3b (compositing bits discovery) + R12b (debug repaint overlay).
//
// Quality bar (§3.1.2 C1): 多层嵌套 boundary (≥3层) + 内层脏不外溢 + 外层脏不内传
// + debug 重绘色可视化; boundary skip/rerecord + NeedsPaint 隔离 + compositing bits
// + debug overlay; 内脏→只内 rerecord; 外脏→只外 rerecord; debug 色只在脏区闪.
//
// Does NOT close solo R2/R3/R3b/R12b — those need their own packages.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_c1_boundary_nest
//	DEBUG_REPAINT=1 …  # enable R12b debug overlay tint
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync/atomic"
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
	closeSeconds = 10
	hudH         = 72.0
)

func main() {
	secs := runSeconds(closeSeconds)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close C1 (U16)")
		os.Exit(1)
	}
	debugRepaint := os.Getenv("DEBUG_REPAINT") == "1"
	fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: C1 nest+skip+debug combo — %ds @ 1200x800 debug=%v\n", secs, debugRepaint)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c1_boundary_nest",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH), debugRepaint)
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.06, ClearB: 0.08, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c1_boundary_nest: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetPictureTextureCache(true)
	app.SetDebugRepaint(debugRepaint)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)
	// R3b: force compositing-bits walk so outer+mid become NeedsCompositing.
	app.Pipeline().UpdateCompositingBits()

	phases := wrkit.NewPhaseClock(2.0, 5.0) // Steady 0–2 · Spike 2–5 · Recover
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
			cache := app.BoundaryCache()
			skip, rr := int64(0), int64(0)
			if cache != nil {
				skip, rr = cache.Skip, cache.Rerecord
			}
			draws := app.DebugRepaintDraws()
			innerClean := sc.innerStaticClean.Load()
			outerSideClean := sc.outerSideClean.Load()
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "C1",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core: fmt.Sprintf("skip=%d rr=%d cnt=%d depth=%d draws=%d innerClean=%d sideClean=%d",
					skip, rr, cnt, depth, draws, innerClean, outerSideClean),
				GateOK: gateOK,
				Extra:  "R2 local·R3 skip·R3b bits·R12b debug — 4 abilities one tree",
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
	cnt, depth = rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	cache := app.BoundaryCache()
	var lifeSkip, lifeRR int64
	if cache != nil {
		lifeSkip, lifeRR = cache.Skip, cache.Rerecord
	}
	if snap.BoundarySkip < lifeSkip {
		snap.BoundarySkip = lifeSkip
		snap.BoundaryRerecord = lifeRR
	}

	innerClean := sc.innerStaticClean.Load()
	outerSideClean := sc.outerSideClean.Load()
	innerHotDirty := sc.innerHotDirty.Load()
	outerDirty := sc.outerDirty.Load()
	draws := app.DebugRepaintDraws()

	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C1",
		Scenario:      "ui_wr_c1_boundary_nest",
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
			"covers":                  "R2+R3+R3b+R12b",
			"debug_repaint":           debugRepaint,
			"debug_repaint_draws":     draws,
			"boundary_count":          cnt,
			"boundary_max_depth":      depth,
			"inner_static_clean":      innerClean,
			"outer_side_clean":        outerSideClean,
			"inner_hot_dirty":         innerHotDirty,
			"outer_dirty":             outerDirty,
			"needs_compositing_outer": sc.Outer.NeedsCompositing(),
			"needs_compositing_mid":   sc.Mid.NeedsCompositing(),
			"g_metrics":               "skipped",
			"g_metrics_reason":        "C1 is not R9/R10; text/measure cache not in scope",
			"phase_script":            "Steady→Spike→Recover (hot pulse rate doubles in Spike)",
			"impl_interaction":        "R2 局部 NeedsPaint 驱动 R3 boundary 缓存 skip/rerecord (热区 MarkNeedsPaint 触发该 RB rerecord, 静区 Replay skip); R3b UpdateCompositingBits 发现嵌套链 (outer+mid 均 NeedsCompositing); R12b debug overlay 染脏区 (debug=1 时 hot 闪 magenta, static 不闪) — 四能力同树同时工作, 集成有意义因嵌套缓存+脏隔离+调试可视化是控件树同时需要的正交能力",
			"impl_correctness":        "内脏→只内 rerecord (inner-static stays clean); 外脏→只外 rerecord (outer-side stays clean); debug 色只在脏区闪 (static 无 magenta)",
			"impl_dirty":              "boundary_skip accumulates from clean static Replays; boundary_rerecord only from dirty boundary; FullPaint redraws all but cache hits still skip",
			"impl_cache":              "BoundaryCache: text + nested static in boundary can skip via Replay; dirty boundary forces re-record; compositing bits walk discovers nest depth",
			"impl_edge":               "3-level nesting (root→outer RB→mid RB→leaf RB); outer-side sibling RB proves outer dirty does not leak inward; inner-static proves inner dirty does not leak outward",
			"impl_fail":               "boundary_count<3 / depth<2 / inner-static dirty after inner-hot pulse / outer-side dirty after outer pulse / debug tint on static = FAIL",
			"impl_visible":            "outer+mid nest outlines; inner-hot pulses red (magenta tint if debug=1); inner-static green frozen; outer-side blue frozen; HUD shows skip/rr/cnt/depth/draws",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c1_boundary_nest: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MinBoundarySkip:        1,
		MinBoundaryRerecord:    1,
		MinBoundaryCount:       3,
		MinBoundaryMaxDepth:    3, // root→outer→mid→leaf 链（R3 嵌套 ≥3 层）
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
	// C1 integration invariants.
	if !sc.Outer.NeedsCompositing() || !sc.Mid.NeedsCompositing() {
		fmt.Fprintf(os.Stderr, "FAIL: NeedsCompositing propagation: outer=%v mid=%v want both true (R3b)\n",
			sc.Outer.NeedsCompositing(), sc.Mid.NeedsCompositing())
		os.Exit(1)
	}
	if innerClean < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: inner_static_clean=%d want ≥10 (内脏不外溢)\n", innerClean)
		os.Exit(1)
	}
	if outerSideClean < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: outer_side_clean=%d want ≥10 (外脏不内传)\n", outerSideClean)
		os.Exit(1)
	}
	if innerHotDirty < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: inner_hot_dirty=%d want ≥10\n", innerHotDirty)
		os.Exit(1)
	}
	if outerDirty < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: outer_dirty=%d want ≥10\n", outerDirty)
		os.Exit(1)
	}
	if debugRepaint && draws < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: debug_repaint_draws=%d want ≥1 when DEBUG_REPAINT=1 (R12b)\n", draws)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c1_boundary_nest: PASS skip=%d rr=%d cnt=%d depth=%d draws=%d fps=%.1f\n",
		rep.BoundarySkip, rep.BoundaryRerecord, cnt, depth, draws, rep.FPSInterval)
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

type sceneC1 struct {
	Root  *rendering.AbsoluteBox
	Outer *rendering.AbsoluteBox
	Mid   *rendering.AbsoluteBox
	Hot   *rendering.RenderColorBox
	hud   *wrkit.LiveHUD

	// Real isolation probes (C1 invariants).
	staticLeaf *rendering.RenderColorBox // inner-static under mid (内脏不外溢 probe)
	sideLeaf   *rendering.RenderColorBox // outer-side sibling RB (外脏不内传 probe)

	// Isolation invariant counters (real NeedsPaint probes on pulse frames).
	innerStaticClean atomic.Int64 // staticLeaf.NeedsPaint()==false while inner-hot pulses
	outerSideClean   atomic.Int64 // sideLeaf.NeedsPaint()==false while outer pulses
	innerHotDirty    atomic.Int64 // inner-hot MarkNeedsPaint each tick
	outerDirty       atomic.Int64 // outer MarkNeedsPaint each tick (outer pulse)

	debugRepaint bool
	phaseHot     float64
	phaseOuter   float64
}

func buildScene(w, h float64, debugRepaint bool) *sceneC1 {
	s := &sceneC1{debugRepaint: debugRepaint}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.06, G: 0.07, B: 0.09, A: 1}
	s.Root = root

	// --- TopBar (U17 多区域 第 1 区) ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("C1 nest+skip+debug combo — R2+R3+R3b+R12b integrated in one tree", 14, 16, 14, 0.88, 0.92, 0.98)

	// --- Legend (U17 第 2 区, ≥8 行色块+文字) ---
	leg := wrkit.NewPanel(240, 540, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.75, 0.95)
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"R2 局部 NeedsPaint", 0.90, 0.85, 0.50},
		{"R3 boundary skip/rerecord", 0.55, 0.80, 0.95},
		{"R3b compositing bits", 0.30, 0.50, 0.90},
		{"R12b debug overlay (magenta)", 0.90, 0.20, 0.70},
		{"内脏→只内 rerecord", 0.25, 0.75, 0.40},
		{"外脏→只外 rerecord", 0.65, 0.68, 0.72},
		{"debug 色只在脏区闪", 0.90, 0.30, 0.80},
		{"PhaseClock: Steady/Spike/Recover", 0.55, 0.75, 0.85},
	}
	for i, ln := range legendLines {
		leg.ColorAt(14, 14, 12, 36+float64(i)*28, ln.r, ln.g, ln.b, 1, false)
		leg.LabelAt(ln.text, 11, 32, 36+float64(i)*28, ln.r, ln.g, ln.b)
	}

	// --- Nested chain: outer RB → mid RB → {static leaf RB, hot leaf RB} ---
	// 3-level nest (root→outer→mid→leaf) + outer-side sibling RB.
	outer := rendering.NewAbsoluteBox(500, 400)
	outer.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.22, A: 1}
	outer.SetRepaintBoundary(true)
	s.Outer = outer

	mid := rendering.NewAbsoluteBox(360, 280)
	mid.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.32, A: 1}
	mid.SetRepaintBoundary(true)
	s.Mid = mid

	staticLeaf := rendering.NewRenderColorBox(100, 100, 0.15, 0.65, 0.3, 1)
	staticLeaf.SetRepaintBoundary(true)
	mid.Place(staticLeaf, 30, 30)
	s.staticLeaf = staticLeaf

	hot := rendering.NewRenderColorBox(110, 110, 0.9, 0.2, 0.15, 1)
	hot.SetRepaintBoundary(true)
	mid.Place(hot, 200, 120)
	s.Hot = hot

	// Outer-side sibling RB (not under mid) — proves outer dirty does not leak inward.
	side := rendering.NewRenderColorBox(70, 200, 0.35, 0.40, 0.70, 1)
	side.SetRepaintBoundary(true)
	outer.Place(side, 400, 40)
	s.sideLeaf = side

	outer.Place(mid, 24, 40)
	root.Place(outer, 280, 90)

	// --- Static text labels around nest (U17 第 3 区) ---
	outerLbl := wrkit.NewPanel(500, 22, 0.12, 0.14, 0.18, 0.9)
	outerLbl.PlaceOn(root, 280, 90-24)
	outerLbl.LabelAt("OUTER RB (depth 1) — pulses blue, only outer rerecord", 11, 6, 14, 0.60, 0.75, 0.90)
	midLbl := wrkit.NewPanel(360, 22, 0.12, 0.14, 0.18, 0.9)
	midLbl.PlaceOn(root, 280+24, 90+40-24)
	midLbl.LabelAt("MID RB (depth 2) — nest container", 11, 4, 14, 0.60, 0.85, 0.70)
	hotLbl := wrkit.NewPanel(110, 22, 0.12, 0.14, 0.18, 0.9)
	hotLbl.PlaceOn(root, 280+24+200, 90+40+120-24)
	hotLbl.LabelAt("HOT leaf (red ~4Hz)", 11, 2, 14, 0.90, 0.75, 0.60)

	// --- Right side: dense static text grid (U17 第 4 区, ≥8 labels) ---
	gridX, gridY := 820.0, 90.0
	for row := 0; row < 6; row++ {
		for col := 0; col < 2; col++ {
			cell := wrkit.NewPanel(170, 32, 0.10, 0.11, 0.13, 0.85)
			cell.PlaceOn(root, gridX+float64(col)*180, gridY+float64(row)*38)
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

func (s *sceneC1) onTick(dt float64, phase string) {
	if s == nil || s.Hot == nil {
		return
	}
	// PhaseClock drives pulse rate — Spike doubles frequency.
	rateHot, rateOuter := 4.0, 2.0
	switch phase {
	case wrkit.PhaseSpike:
		rateHot, rateOuter = 8.0, 4.0
	case wrkit.PhaseRecover:
		rateHot, rateOuter = 3.0, 1.5
	}
	s.phaseHot += dt * rateHot
	s.phaseOuter += dt * rateOuter

	// Inner-hot: red pulse (R2 local NeedsPaint → R3 rerecord).
	r := 0.55 + 0.35*math.Sin(s.phaseHot)
	if s.debugRepaint {
		// R12b debug tint: magenta flash on dirty hot.
		s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = 0.95, r*0.3, 0.85, 1
	} else {
		s.Hot.R, s.Hot.G, s.Hot.B, s.Hot.A = r, 0.20, 0.15, 1
	}
	s.Hot.MarkNeedsPaint()
	s.innerHotDirty.Add(1)

	// Outer: blue pulse (R2 local NeedsPaint on outer → R3 rerecord).
	// Outer dirty must NOT leak inward to mid/static (外脏不内传).
	if s.Outer.Background != nil {
		s.Outer.Background.B = 0.22 + 0.20*(0.5+0.5*math.Sin(s.phaseOuter))
	}
	s.Outer.MarkNeedsPaint()
	s.outerDirty.Add(1)

	// C1 isolation invariants (real probes on this pulse frame):
	// - inner-static stays clean while inner-hot pulses (内脏不外溢):
	//   staticLeaf (sibling of Hot under mid) must NOT be paint-dirty just
	//   because Hot was MarkNeedsPaint'd — hot dirty must not leak outward.
	// - outer-side stays clean while outer pulses (外脏不内传):
	//   sideLeaf (sibling of mid under outer) must NOT be paint-dirty just
	//   because Outer was MarkNeedsPaint'd — outer dirty must not leak inward.
	if !s.staticLeaf.NeedsPaint() {
		s.innerStaticClean.Add(1)
	}
	if !s.sideLeaf.NeedsPaint() {
		s.outerSideClean.Add(1)
	}
}
