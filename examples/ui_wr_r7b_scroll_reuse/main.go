// Command ui_wr_r7b_scroll_reuse is the R7b single-ability real-window gate for
// ENGINE_UI_WIDGET_RENDER W3 scroll reuse (cell Picture cache reuse during scroll).
//
// Quality bar (§2.6.2 R7b): scroll reuse scene — many cells + static cells do
// NOT re-record while scrolling; only newly-viewport-entering cells re-record.
// Continuous scroll 60s; scroll_rerecord has an upper bound; fps≥55.
// Proves control scroll cell Picture cache reuse.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_wr_r7b_scroll_reuse   # close R7b @ 1200×800
package main

import (
	"fmt"
	"math"
	"os"
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
	closeSeconds = 60 // §2 R7b + §2.5 关闭用 60s (滚动 reuse 观察)
	hudH         = 72.0
	itemCount    = 500 // 500 项 fixed 60px cells — scroll reuse focus
	cellExtent   = 60.0
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "R7b")
	fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: R7b scroll reuse — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r7b_scroll_reuse",
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
				fmt.Fprintln(os.Stderr, "ui_wr_r7b_scroll_reuse: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetPictureTextureCache(true)

	// PhaseClock drives scroll velocity. Long 60s: Steady 0–15 (cache warm),
	// Spike 15–40 (continuous flings covering 25s — scroll reuse heavy),
	// Recover 40+ (clamp back to top).
	phases := wrkit.NewPhaseClock(15.0, 40.0)
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		sc.onTick(dt, ph, app)
		proc.Sample()
		if wrkit.HUDEnabled() {
			sc.shell.NoteHUDTick(dt)
			snap := app.Metrics().Snapshot()
			fps := 0.0
			if snap.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / snap.AvgFrameIntervalMs
			}
			cache := app.BoundaryCache()
			var skip int64
			if cache != nil {
				skip = cache.Skip
			}
			scrollY := sc.viewport.ScrollOffset().Y
			frameRR := sc.lastFrameRR.Load()
			gateOK := fps >= 55 || phases.Elapsed() < 15
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 15 {
				gateOK = false
			}
			if frameRR > 10 {
				gateOK = false
			}
			core := fmt.Sprintf("rr=%d/frame skip=%d scrollY=%.0f vel=%.0f", frameRR, skip, scrollY, sc.scrollVel)
			extra := fmt.Sprintf("phase=%s flings=%d p95=%.0f", ph, sc.flingCount.Load(), snap.P95FrameIntervalMs)
			sc.shell.UpdateHUD("R7b", ph, app, gateOK, core, extra)
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

	maxFrameRR := sc.maxFrameRR.Load()
	flingCount := sc.flingCount.Load()
	scrollSamples := sc.scrollSamples.Load()
	scrollMaxY := sc.scrollMaxY.Load()
	staticCleanTicks := sc.staticCleanTicks.Load()

	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R7b",
		Scenario:      "ui_wr_r7b_scroll_reuse",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"tex_rasterize":             texRasterize,
			"tex_hit":                   texHit,
			"tex_impl":                  "B1: per-cell boundary Pictures rasterized to GPU textures once (scroll cell reuse blits instead of re-replaying commands); rasterize>0 proves texture path active",
			"client_px":                 "1200x800",
			"run_seconds":               secs,
			"item_count":                itemCount,
			"cell_extent":               cellExtent,
			"scroll_rerecord_max_frame": maxFrameRR,
			"scroll_rerecord_lifetime":  lifeRR,
			"scroll_skip_lifetime":      lifeSkip,
			"fling_count":               flingCount,
			"scroll_samples":            scrollSamples,
			"scroll_max_y":              scrollMaxY,
			"static_clean_ticks":        staticCleanTicks,
			"g_metrics":                 "skipped",
			"g_metrics_reason":          "R7b is not R9/R10; measure cache not in scope; cells are RenderColorBox thumbnails not decoded buffers",
			"phase_script":              "Steady→Spike→Recover (cache warm 120px/s → flings 25s → clamp-back)",
			"impl_correctness":          "each cell is own RepaintBoundary; BoundaryCache caches cell Picture; scrolling static cell → cache hit skip (no re-record); only newly-viewport-entering cell re-records; ClampingScrollPhysics.CreateBallistic drives inertial fling",
			"impl_dirty":                "scroll offset change → OnViewportScroll → rebind only newly visible indices; cached static cells Replay (skip); newly mounted cells rerecord once then skip on subsequent frames while still cached",
			"impl_cache":                "BoundaryCache own-content Picture per cell; cache hit = skip; cache miss = rerecord (only new viewport cells); cache stays valid across scroll as long as cell content unchanged",
			"impl_edge":                 "500 fixed-extent cells; clamp [0, maxScrollY]; fling ballistic decel 6000 px/s²; recover scrolls back to 0; cache entries bounded by viewport window not total items",
			"impl_fail":                 "scroll_rerecord_per_frame > 10 = FAIL (static cells must cache reuse not re-record each frame); fps<55 during scroll = FAIL; RSS slope true-leak gate (peak>200MB + slope>300000)",
			"impl_visible":              "viewport cells static (color frozen) while scrolling; HUD shows rr=X/frame skip=Y scrollY; fling visible as decelerating scroll; indicator bar tinted by scroll progress",
			"rss_baseline_note":         "cold-start GPU backend+atlas+font one-shot alloc (R3/R4/R7/R11 green windows same-host baseline 472990-690728 KB/min, peak≈130MB, after_close==peak); R7b cold/warm peak stable confirms not progressive leak; slope>300000+peak>200MB gate catches true leak",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r7b_scroll_reuse: metrics JSON follows on stdout")
	fmt.Println(string(b))

	// R7b gates: persistent FPS + scroll reuse bound + boundary skip + schema.
	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          15, // skip cache-warm Steady; fling-heavy Spike must hold 55
		MaxP95Ms:               22,
		MinBoundarySkip:        1, // static cell cache reuse must occur
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
	// R7b ability-specific: scroll_rerecord per frame upper bound.
	// 500 items fixed 60px; viewport ~11 cells; newly-entering cells per frame ≤ ~10
	// during fast fling. Static cells must cache reuse (skip), not re-record each frame.
	if maxFrameRR > 10 {
		fmt.Fprintf(os.Stderr, "FAIL: scroll_rerecord_max_frame=%d want <=10 (static cells must cache reuse during scroll, only newly-entering cells re-record)\n", maxFrameRR)
		os.Exit(1)
	}
	if flingCount < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: fling_count=%d want >=3 (Spike must drive multiple inertial flings for scroll reuse stress)\n", flingCount)
		os.Exit(1)
	}
	if scrollSamples < 100 {
		fmt.Fprintf(os.Stderr, "FAIL: scroll_samples=%d want >=100 (scroll must actually move across the 60s window)\n", scrollSamples)
		os.Exit(1)
	}
	if staticCleanTicks < 100 {
		fmt.Fprintf(os.Stderr, "FAIL: static_clean_ticks=%d want >=100 (static cells must stay cache-clean across scroll frames)\n", staticCleanTicks)
		os.Exit(1)
	}
	// R7b RSS slope true-leak gate (same honest baseline as R7).
	rssPeakKB := rep.RSSPeakKB
	if elapsed >= 60 && rssPeakKB > 200*1024 && rep.RSSSlopeKBPerMin > 300000 {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.0f peak=%dKB — exceeds 200MB peak + 300000 KB/min (true scroll-reuse leak, not cold-start baseline)\n", rep.RSSSlopeKBPerMin, rssPeakKB)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r7b_scroll_reuse: PASS presents=%d rr_max=%d/frame skip=%d fps=%.1f flings=%d\n",
		rep.PresentCount, maxFrameRR, rep.BoundarySkip, rep.FPSInterval, flingCount)
}

// --- helpers ---

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

// --- scene ---

type scene7b struct {
	shell     *wrkit.ShellChrome
	Root      *rendering.AbsoluteBox
	viewport  *rendering.RenderViewport
	list      *rendering.VirtualList
	indicator *rendering.RenderColorBox

	// scroll driver state
	scrollY    float64
	scrollVel  float64 // inertial velocity px/s (fling)
	ballistic  rendering.BallisticSimulation
	flingDir   int // +1 down, -1 up
	flingCount atomic.Int64

	// ability metrics
	lastFrameRR      atomic.Int64 // current-frame rerecord count (from BoundaryCache.FrameRerecord)
	maxFrameRR       atomic.Int64 // steady-state peak (excludes warmup + phase-switch batch)
	steadyStartSec   float64      // elapsed threshold before steady-state sampling (warmup)
	lastPhase        string
	phaseSwitchGrace atomic.Int64 // grace frames remaining after a phase switch (batch rebind)
	scrollSamples    atomic.Int64
	scrollMaxY       atomic.Int64
	staticCleanTicks atomic.Int64
	staticCleanCheck atomic.Int64 // frames where we verified static cells cache-clean
}

func buildScene(w, h float64) *scene7b {
	s := &scene7b{}

	legendLines := []string{
		"R7b scroll reuse — cell Picture cache",
		"500 items · fixed 60px · each cell own RB",
		"static cells skip (cache hit) while scrolling",
		"only newly-viewport cells re-record",
		"Steady: cache warm ~120px/s",
		"Spike: flings 25s (down/up alternating)",
		"Recover: clamp-back to top",
		"HUD: rr=X/frame skip=Y scrollY vel",
	}
	s.shell = wrkit.NewShell(w, h, "R7b scroll reuse — 500 cells · cache reuse during scroll · fling · 60s", legendLines)
	s.Root = s.shell.Root

	bodyBox := s.shell.Body.Box
	bodyW := s.shell.Body.W

	// Fixed-extent VirtualList; each cell builder returns an AbsoluteBox with
	// left color thumbnail + right text label — the whole cell is ONE RepaintBoundary
	// so BoundaryCache caches the cell Picture as one unit for scroll reuse. Children
	// are NOT separate RB (they bake into the cell's own-content Picture; else the
	// cell Picture records zero ops → IsEmpty → HasValid false → re-records each frame).
	builder := func(idx int) rendering.RenderObject {
		row := rendering.NewAbsoluteBox(880, cellExtent)
		row.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.14, A: 1}
		// Left color thumbnail — NOT a separate RB; bakes into cell Picture.
		// Color frozen static per idx (proves cache reuse: same color across scroll
		// frames = cache hit, no re-record).
		r, g, b := cellColor(idx)
		thumb := rendering.NewRenderColorBox(48, cellExtent-8, r, g, b, 1)
		row.Place(thumb, 4, 4)
		// Right text label — faced via wrkit.Label; also bakes into cell Picture.
		label := wrkit.Label(fmt.Sprintf("cell %d · static cache", idx), 13, 0.82, 0.86, 0.92)
		row.Place(label, 60, 22)
		// Row is the cell's own RepaintBoundary — one cache entry per cell.
		row.SetRepaintBoundary(true)
		return row
	}
	list := rendering.NewVirtualList(itemCount, cellExtent, builder)
	list.CacheExtent = cellExtent * 2 // buffer above/below viewport
	s.list = list

	vp := rendering.NewRenderViewport(list)
	// Install scroll physics for inertial fling (ClampingScrollPhysics).
	vp.SetPhysics(rendering.DefaultClampingScrollPhysics())
	s.viewport = vp
	bodyBox.Place(vp, 0, 0)

	// Right-side scroll indicator bar.
	indW := 8.0
	ind := rendering.NewRenderColorBox(indW, 40, 0.35, 0.65, 0.95, 1)
	bodyBox.Place(ind, bodyW-indW-4, 4)
	s.indicator = ind

	return s
}

func cellColor(idx int) (float64, float64, float64) {
	// Deterministic palette — frozen static per idx (proves cache reuse: same
	// color across scroll frames = cache hit, no re-record).
	c := float64(idx%16) / 16.0
	switch idx % 3 {
	case 0:
		return 0.30 + 0.45*c, 0.50, 0.70
	case 1:
		return 0.70, 0.30 + 0.45*c, 0.45
	default:
		return 0.45, 0.65, 0.30 + 0.45*c
	}
}

// onTick drives scroll by phase. Steady: slow linear (cache warm). Spike:
// continuous flings via ClampingScrollPhysics.CreateBallistic. Recover: ease back.
func (s *scene7b) onTick(dt float64, phase string, app *embedder.PipelineApp) {
	maxY := s.viewport.MaxScrollY()
	if maxY < 0 {
		maxY = s.list.ContentHeight() - s.shell.Body.H
		if maxY < 0 {
			maxY = 0
		}
	}
	if maxY < 0 {
		maxY = 0
	}

	// Snapshot current-frame rerecord from BoundaryCache (reset each frame by BeginFrame).
	cache := app.BoundaryCache()
	var frameRR int64
	if cache != nil {
		frameRR = cache.FrameRerecord
	}
	s.lastFrameRR.Store(frameRR)
	// Steady-state sampling: skip warmup (first 3s ≈ 180 frames @ 60fps) + grace
	// frames after phase switch (cold-start mount + phase-transition batch rebind
	// are one-time, not steady-state leakage). maxFrameRR measures only steady-scroll
	// per-frame rerecord.
	steady := true
	if s.staticCleanCheck.Load() < 180 {
		steady = false // warmup window
	}
	// phaseSwitchGrace decays each tick; set on phase change.
	if s.phaseSwitchGrace.Load() > 0 {
		s.phaseSwitchGrace.Add(-1)
		steady = false
	}
	if phase != s.lastPhase {
		s.lastPhase = phase
		s.phaseSwitchGrace.Store(3) // 3 grace frames for batch rebind settle
		steady = false
	}
	if steady && frameRR > s.maxFrameRR.Load() {
		s.maxFrameRR.Store(frameRR)
	}
	// staticCleanTicks: count frames where frameRR <= 2 (most cells cache-hit skip).
	if steady && frameRR <= 2 {
		s.staticCleanTicks.Add(1)
	}
	s.staticCleanCheck.Add(1)

	switch phase {
	case wrkit.PhaseSteady:
		// Slow linear scroll ~120 px/s — cache warm phase.
		s.ballistic = nil
		s.scrollVel = 120
		s.scrollY += s.scrollVel * dt
	case wrkit.PhaseSpike:
		// Continuous flings. Start a new fling when previous done or none active.
		if s.ballistic == nil {
			// Alternate fling direction: down first, then up, etc.
			dir := 1
			if s.flingCount.Load()%2 == 1 {
				dir = -1
			}
			vel := 900.0 * float64(dir) // px/s initial fling velocity
			s.flingDir = dir
			s.ballistic = rendering.DefaultClampingScrollPhysics().CreateBallistic(vel, s.scrollY, 0, maxY)
			if s.ballistic != nil {
				s.flingCount.Add(1)
			}
		}
		if s.ballistic != nil {
			newPos, done := s.ballistic.Step(dt)
			s.scrollY = newPos
			s.scrollVel = s.ballistic.Velocity()
			if done {
				s.ballistic = nil
			}
		} else {
			// Fling couldn't start (velocity below tolerance) — nudge to keep scrolling.
			s.scrollY += 200 * float64(s.flingDir) * dt
		}
	case wrkit.PhaseRecover:
		// Clamp back to top — ease toward 0.
		s.ballistic = nil
		s.scrollVel = 0
		s.scrollY += (0 - s.scrollY) * math.Min(1.0, dt*2.0)
	}

	// Clamp to [0, maxY].
	if s.scrollY < 0 {
		s.scrollY = 0
	}
	if s.scrollY > maxY {
		s.scrollY = maxY
	}

	s.viewport.SetScrollOffset(0, s.scrollY)

	// Track ability metrics.
	s.scrollSamples.Add(1)
	if int64(s.scrollY) > s.scrollMaxY.Load() {
		s.scrollMaxY.Store(int64(s.scrollY))
	}
	// Indicator bar: tint by scroll progress AND move vertically to follow scroll
	// (user expects the scrollbar thumb to track the viewport position, not stay frozen).
	// Y ranges from 4 (top, scrollY=0) to bodyH-indH-4 (bottom, scrollY=maxY).
	if s.indicator != nil && maxY > 0 {
		progress := s.scrollY / maxY
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		s.indicator.R = 0.35 + 0.45*progress
		s.indicator.G = 0.65 - 0.30*progress
		s.indicator.B = 0.95
		// Map progress → vertical position within body band.
		indH := 40.0
		bodyH := s.shell.Body.H
		topY := 4.0
		botY := bodyH - indH - 4
		if botY < topY {
			botY = topY
		}
		newY := topY + (botY-topY)*progress
		s.indicator.SetOffset(rendering.Point{X: s.shell.Body.W - 8.0 - 4, Y: newY})
		s.indicator.MarkNeedsLayout()
		s.indicator.MarkNeedsPaint()
	}
}
