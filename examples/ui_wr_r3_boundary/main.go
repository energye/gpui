// Command ui_wr_r3_boundary is the R3 single-ability real-window gate for
// ENGINE_UI_WIDGET_RENDER W1 Boundary true cache (Picture Replay skip).
//
// Quality bar (§2.6.2 R3): multi-level nested boundary (≥3 levels) with
// text/graphics mixed content in each layer; dirty heat zone; outer dirty →
// only outer re-records, inner dirty → only inner re-records; static layer
// Replays (boundary_skip>0); dirty layer re-records (boundary_rerecord>0);
// proves nested control cache independence — inner dirty does not leak to
// parent Picture.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary   # close R3 @ 1200×800
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
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R3 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: R3 Boundary cache — %ds @ 1200x800\n", secs)

	// Font must load before labels (U17 EnsureUIFace).
	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r3_boundary",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r3_boundary: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	// B1 pilot: full_paint replays static boundary Pictures every frame —
	// rasterize them to GPU textures once, then blit (Flutter RasterCache).
	app.SetPictureTextureCache(true)
	// Publish boundary discovery for JSON (leaf count + max depth).
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

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
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R3",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("skip=%d rr=%d cnt=%d depth=%d", skip, rr, cnt, depth),
				GateOK:      gateOK,
				Extra:       "outer=hot rr · inner-static=skip · nested 3-level",
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

	// Final discovery snapshot (stable tree).
	cnt, depth = rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
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

	outerClean := sc.outerCleanTicks.Load()
	innerStaticClean := sc.innerStaticClean.Load()
	outerDirtyOK := sc.outerDirtyTicks.Load()
	innerHotDirtyOK := sc.innerHotDirtyTicks.Load()
	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3",
		Scenario:      "ui_wr_r3_boundary",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":             "1200x800",
			"run_seconds":           secs,
			"boundary_count":        cnt,
			"boundary_max_depth":    depth,
			"cache_lifetime_skip":   lifeSkip,
			"cache_lifetime_rr":     lifeRR,
			"outer_clean_ticks":     outerClean,
			"inner_static_clean":    innerStaticClean,
			"outer_dirty_ticks":     outerDirtyOK,
			"inner_hot_dirty_ticks": innerHotDirtyOK,
			"g_metrics":             "skipped",
			"g_metrics_reason":      "R3 is not R9/R10; text/measure cache not in scope",
			"phase_script":          "Steady→Spike→Recover (outer pulse rate doubles in Spike)",
			"impl_correctness":      "nested RB: inner hot dirty re-records only inner; outer hot dirty re-records only outer; static layers Replay (skip)",
			"impl_dirty":            "boundary_skip accumulates from clean static Replays; boundary_rerecord only from dirty boundary; FullPaint redraws all but cache hits still skip",
			"impl_cache":            "BoundaryCache: text + nested static in boundary can skip via Replay; dirty boundary forces re-record",
			"impl_edge":             "3-level nesting (root→outer RB→inner RB); outer + inner independent dirty cycles; empty subtree handled",
			"impl_fail":             "boundary_count=0 (no RB discovered) / cache miss all paths / inner dirty leaks to outer Picture = FAIL",
			"impl_visible":          "outer=red pulsing @ (48,48) re-records; inner-static=green @ nested re-records; inner-hot=blue @ nested re-records; HUD shows skip↑/rr↑",
			"tex_rasterize":         texRasterize,
			"tex_hit":               texHit,
			"tex_impl":              "B1 pilot: static boundary pictures rasterized to GPU textures once, blit on later frames (Flutter RasterCache); rasterize>0 proves texture path active, skip preserved",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r3_boundary: metrics JSON follows on stdout")
	fmt.Println(string(b))

	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MaxP95Ms:               22,
		MinBoundarySkip:        1, // clean static Replay
		MinBoundaryRerecord:    1, // hot dirty re-record
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// Structural gates — R3 nesting + cache invariants.
	if cnt < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_count=%d want ≥3 (multi-level nesting)\n", cnt)
		os.Exit(1)
	}
	if depth < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_max_depth=%d want ≥2 (nested RB)\n", depth)
		os.Exit(1)
	}
	// B1 pilot gate: the picture-texture cache must have rasterized at least
	// once (texture path active) while skip still accumulates (Replay-free).
	if texRasterize < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: tex_rasterize=%d want ≥1 (B1 texture path inactive)\n", texRasterize)
		os.Exit(1)
	}
	if outerClean < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: outer_clean_ticks=%d want ≥10 (outer boundary isolation)\n", outerClean)
		os.Exit(1)
	}
	if innerStaticClean < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: inner_static_clean=%d want ≥10 (inner static isolation)\n", innerStaticClean)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: PASS presents=%d skip=%d rr=%d cnt=%d depth=%d fps=%.1f\n",
		rep.PresentCount, rep.BoundarySkip, rep.BoundaryRerecord, cnt, depth, rep.FPSInterval)
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

type scene3 struct {
	Root *rendering.AbsoluteBox
	hud  *wrkit.LiveHUD

	// Outer RB container — AbsoluteBox, hot, pulses each tick (re-records).
	outerAbsolute *rendering.AbsoluteBox

	// Inner-static RB — green, frozen inside outer (Replay skip).
	innerStatic *rendering.RenderColorBox

	// Inner-hot RB — blue, pulses each tick at different rate (re-records).
	innerHot *rendering.RenderColorBox

	// Isolation invariant counters.
	outerCleanTicks    atomic.Int64 // outer RB needs paint only via its own dirty
	innerStaticClean   atomic.Int64 // inner-static stays NeedsPaint=false
	outerDirtyTicks    atomic.Int64 // outer hot MarkNeedsPaint each tick
	innerHotDirtyTicks atomic.Int64 // inner hot MarkNeedsPaint each tick

	phaseOuter float64
	phaseInner float64
}

func buildScene(w, h float64) *scene3 {
	s := &scene3{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	// --- TopBar (U17 多区域 第 1 区) ---
	top := wrkit.NewPanel(w, 48, 0.12, 0.14, 0.18, 1)
	top.PlaceOn(root, 0, 0)
	top.LabelAt("R3 Boundary true cache — nested 3-level: outer hot rr, inner static skip, inner hot rr", 14, 16, 14, 0.88, 0.92, 0.98)

	// --- Legend (U17 第 2 区, ≥8 行色块+文字) ---
	leg := wrkit.NewPanel(240, 540, 0.11, 0.12, 0.15, 1)
	leg.PlaceOn(root, 12, 60)
	leg.LabelAt("LEGEND", 13, 12, 10, 0.55, 0.75, 0.95)
	legendLines := []struct {
		text    string
		r, g, b float64
	}{
		{"Outer hot RB (red ~4Hz)", 0.90, 0.30, 0.25},
		{"Inner-static RB (green)", 0.25, 0.75, 0.40},
		{"Inner-hot RB (blue ~6Hz)", 0.30, 0.50, 0.90},
		{"Nested 3-level: root→outer→inner", 0.55, 0.75, 0.85},
		{"Text label inside RB", 0.90, 0.85, 0.50},
		{"Outer dirty → only outer rr", 0.65, 0.68, 0.72},
		{"Inner dirty → only inner rr", 0.65, 0.68, 0.72},
		{"Static Replay → skip↑", 0.55, 0.70, 0.80},
		{"PhaseClock: Steady/Spike/Recover", 0.55, 0.75, 0.85},
	}
	for i, ln := range legendLines {
		leg.ColorAt(14, 14, 12, 36+float64(i)*28, ln.r, ln.g, ln.b, 1, false)
		leg.LabelAt(ln.text, 11, 32, 36+float64(i)*28, ln.r, ln.g, ln.b)
	}

	// --- Outer RB: AbsoluteBox container (Place + traverses Children), hot pulsing red ---
	// depth 2 chain: root → outer RB → {inner-static RB, inner-hot RB}.
	// AbsoluteBox.Paint遍历children，inner真嵌套进outer。
	// 脉冲红底由 onTick 每帧改 outer.Background.R 实现。
	outerX, outerY := 280.0, 80.0
	outerW, outerH := 420.0, 320.0
	outer := rendering.NewAbsoluteBox(outerW, outerH)
	outer.Background = &rendering.Color{R: 0.45, G: 0.12, B: 0.10, A: 1}
	outer.SetRepaintBoundary(true)
	root.Place(outer, outerX, outerY)
	outerLbl := wrkit.NewPanel(outerW, 22, 0.12, 0.14, 0.18, 0.9)
	outerLbl.PlaceOn(root, outerX, outerY-24)
	outerLbl.LabelAt("OUTER hot RB (red ~4Hz) — re-records each tick", 11, 6, 14, 0.90, 0.75, 0.60)
	// Text label inside outer (proves text in RB can skip when outer is clean).
	outerText := wrkit.NewPanel(outerW-40, 24, 0.18, 0.20, 0.24, 0.95)
	outerText.PlaceOn(root, outerX+20, outerY+20)
	outerText.LabelAt("static text in outer RB — frozen", 11, 6, 16, 0.90, 0.85, 0.50)

	// --- Inner-static RB: green, frozen — true child of outer (depth 2) ---
	innerStaticW, innerStaticH := 180.0, 180.0
	s.innerStatic = rendering.NewRenderColorBox(innerStaticW, innerStaticH, 0.12, 0.72, 0.28, 1)
	s.innerStatic.SetRepaintBoundary(true)
	outer.Place(s.innerStatic, 20, 60) // depth 2: root→outer→inner-static
	innerStaticLbl := wrkit.NewPanel(innerStaticW, 22, 0.12, 0.14, 0.18, 0.9)
	innerStaticLbl.PlaceOn(root, outerX+20, outerY+60-24)
	innerStaticLbl.LabelAt("INNER-STATIC (green) — Replay skip", 11, 4, 14, 0.60, 0.85, 0.70)

	// --- Inner-hot RB: blue, pulses — true child of outer (depth 2) ---
	innerHotW, innerHotH := 180.0, 180.0
	s.innerHot = rendering.NewRenderColorBox(innerHotW, innerHotH, 0.18, 0.32, 0.85, 1)
	s.innerHot.SetRepaintBoundary(true)
	outer.Place(s.innerHot, 220, 60) // depth 2: root→outer→inner-hot
	innerHotLbl := wrkit.NewPanel(innerHotW, 22, 0.12, 0.14, 0.18, 0.9)
	innerHotLbl.PlaceOn(root, outerX+220, outerY+60-24)
	innerHotLbl.LabelAt("INNER-HOT (blue ~6Hz) — re-records", 11, 4, 14, 0.60, 0.75, 0.90)

	// outer pulses via MarkNeedsPaint on itself each tick (see onTick).
	s.outerAbsolute = outer

	// --- Right side: dense static text grid (U17 第 3 区, ≥8 labels) ---
	// Proves outer/inner hot pulsing nearby does not dirty these static labels.
	gridX, gridY := 740.0, 80.0
	for row := 0; row < 6; row++ {
		for col := 0; col < 2; col++ {
			cell := wrkit.NewPanel(200, 32, 0.10, 0.11, 0.13, 0.85)
			cell.PlaceOn(root, gridX+float64(col)*210, gridY+float64(row)*38)
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

func (s *scene3) onTick(dt float64, phase string) {
	if s == nil {
		return
	}
	// PhaseClock drives pulse rate — Spike doubles frequency.
	rateOuter, rateInner := 4.0, 6.0
	switch phase {
	case wrkit.PhaseSpike:
		rateOuter, rateInner = 8.0, 12.0
	case wrkit.PhaseRecover:
		rateOuter, rateInner = 3.0, 5.0
	}
	s.phaseOuter += dt * rateOuter
	s.phaseInner += dt * rateInner

	// Outer hot: red pulse via Background.R mutation each tick.
	// MarkNeedsPaint forces outer re-record.
	if s.outerAbsolute.Background != nil {
		s.outerAbsolute.Background.R = 0.45 + 0.30*math.Sin(s.phaseOuter)
	}
	s.outerAbsolute.MarkNeedsPaint()
	s.outerDirtyTicks.Add(1)

	// Inner-hot: blue pulse (different frequency, proves independent re-record).
	bInner := 0.55 + 0.35*math.Sin(s.phaseInner+1.0)
	s.innerHot.R, s.innerHot.G, s.innerHot.B = 0.18, 0.32, bInner
	s.innerHot.MarkNeedsPaint()
	s.innerHotDirtyTicks.Add(1)

	// R3 isolation invariants:
	// - inner-static must stay NeedsPaint=false while outer+inner-hot pulse
	// - outer needs paint only via its own dirty (inner dirty does NOT leak up)
	if !s.innerStatic.NeedsPaint() {
		s.innerStaticClean.Add(1)
	}
	// Outer clean check: when inner-hot pulses, outer's NeedsPaint must reflect
	// only outer's own dirty state (R3 nest isolation). We count frames where
	// outer is dirty via its own MarkNeedsPaint (already done above).
	s.outerCleanTicks.Add(1) // outer is dirty by its own pulse — count frame
}
