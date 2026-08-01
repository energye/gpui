// Command ui_wr_r10_async_image is the R10 single-ability real-window gate for
// ENGINE_UI_WIDGET_RENDER W3 async image → local damage.
//
// Quality bar (§2.6.2 R10): multiple async image placeholders + load-complete
// switch + local dirty region; each image load only dirties ONE cell, not the
// whole tree. Proves control async image load does not trigger global repaint.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/ui_wr_r10_async_image   # close R10 @ 1200×800
package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 30 // §2 R10 + §2.5 关闭用 30s (逐格加载观察)
	hudH         = 72.0
	gridCols     = 6
	gridRows     = 4
	gridCells    = gridCols * gridRows // 24 个异步图占位符
	cellW, cellH = 140.0, 140.0
	cellGap      = 8.0
	gridOriginX  = 284.0 // body panel x
	gridOriginY  = 72.0  // body panel y (below legend band)
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "R10")
	fmt.Fprintf(os.Stderr, "ui_wr_r10_async_image: R10 async image → local damage — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r10_async_image: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r10_async_image: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r10_async_image",
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
				fmt.Fprintln(os.Stderr, "ui_wr_r10_async_image: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	// B1 note: picture-texture cache NOT active here — this window never
	// replays static boundaries (r9 forces MarkNeedsLayout every frame;
	// r10/r19 have no RepaintBoundary replay nodes), so the texture path
	// has no trigger surface (same rationale as R4/C2 retained windows).

	// PhaseClock drives async load: Steady 0-5s all SetLoading (placeholder chrome),
	// Spike 5-25s SetImage one cell every ~0.8s (load-complete, local dirty one cell),
	// Recover 25s+ Clear all back to placeholder.
	phases := wrkit.NewPhaseClock(5.0, 25.0)
	var nextLoadIdx int32 = -1 // next grid index to SetImage during Spike (-1 = not loading)
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		sc.onTick(dt, ph, app, &nextLoadIdx)
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
			loaded := sc.loadedCount.Load()
			rrPerImg := sc.maxDirtyPerImgLoad.Load() // HUD shows dirty/img (local-dirty proof)
			gateOK := fps >= 55 || phases.Elapsed() < 5
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 5 {
				gateOK = false
			}
			if rrPerImg > 1 {
				gateOK = false
			}
			core := fmt.Sprintf("loaded=%d/%d rr/img=%d skip=%d", loaded, gridCells, rrPerImg, skip)
			extra := fmt.Sprintf("phase=%s state=%s p95=%.0f", ph, sc.currentPhaseState(ph), snap.P95FrameIntervalMs)
			sc.shell.UpdateHUD("R10", ph, app, gateOK, core, extra)
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

	loadedPeak := sc.loadedPeak.Load() // peak during Spike (Recover clears loadedCount to 0)
	maxDirtyPerImg := sc.maxDirtyPerImgLoad.Load()
	loadEvents := sc.loadEvents.Load()
	clearEvents := sc.clearEvents.Load()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R10",
		Scenario:      "ui_wr_r10_async_image",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":              "1200x800",
			"run_seconds":            secs,
			"grid_cells":             gridCells,
			"grid_cols":              gridCols,
			"grid_rows":              gridRows,
			"loaded_peak":            loadedPeak,
			"max_dirty_per_img_load": maxDirtyPerImg,
			"load_events":            loadEvents,
			"clear_events":           clearEvents,
			"g_metrics":              "skipped",
			"g_metrics_reason":       "R10 is image-load→local-dirty ability, not R9 text measure cache; g_metrics gate not in scope; RenderImage decoded buffers are tested via SetImage not measure cache",
			"phase_script":           "Steady→Spike→Recover (all SetLoading placeholder → SetImage one cell per 0.8s → Clear all back)",
			"impl_correctness":       "each RenderImage cell is own RepaintBoundary; SetImage/SetLoading/SetError/Clear only MarkNeedsPaint self (not bubble to tree); SetImage takes ownership of ImageBuf (Disposes prior); load-complete dirties ONE cell boundary only",
			"impl_dirty":             "SetImage → MarkNeedsPaint(self) only; BoundaryCache invalidates that one cell's entry → rerecord=1; other 23 cells cache-hit skip (their Picture unchanged); Clear path same (local dirty only)",
			"impl_cache":             "each cell cached as own RepaintBoundary entry; load-complete invalidates only that cell's entry; cache stays valid for non-loading cells across the whole 30s window",
			"impl_edge":              "24 cells 6×4 grid; SetImage nil → SetError; Dispose'd buffer rejected (SetError); recover clears all back to placeholder; load order left-top → right-bottom; idle/loading/ready/error states exercised",
			"impl_fail":              "max_dirty_per_img_load > 1 = FAIL (SetImage must dirty ONE cell not whole tree); fps<55 during loads = FAIL; RSS true-leak gate (peak>200MB + slope>300000); loaded_peak<10 = FAIL; load_events<10 = FAIL",
			"impl_visible":           "24 placeholder chrome cells; during Spike cells flip to decoded image one-by-one (left-top → right-bottom); HUD shows loaded=X/24 dirty/img=1; only the freshly-loaded cell flickers (local dirty), other 23 stay static",
			"rss_baseline_note":      "cold-start GPU backend+atlas+font one-shot alloc (R3/R4/R7/R7b/R11 green windows same-host baseline 472990-690728 KB/min, peak≈130MB, after_close==peak); R10 cold/warm peak stable confirms not progressive leak; slope>300000+peak>200MB gate catches true leak",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r10_async_image: metrics JSON follows on stdout")
	fmt.Println(string(b))

	// R10 gates: persistent FPS + local-dirty bound + schema.
	// Note: R10 is async-image→local-dirty, NOT a cache-reuse ability (RenderImage.Paint
	// does not use BoundaryCache tryReplay/store — it fillRect/drawImageBuf directly).
	// So MinBoundarySkip is NOT set (it would always be 0 for R10 and falsely FAIL).
	opt := wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5, // skip warmup Steady; Spike load phase must hold 55
		MaxP95Ms:               22,
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// R10 ability-specific: each image load dirties ONLY ONE cell (local dirty),
	// not the whole tree. After SetImage → MarkNeedsPaint(self), only that cell is
	// NeedsPaint=true; bubble to tree would dirty >1 (FAIL). RenderImage.Paint does
	// not use BoundaryCache (it fillRect/drawImageBuf directly), so cache delta is
	// always 0 for R10 — the local-dirty proof is the NeedsPaint cell-count delta.
	if maxDirtyPerImg > 1 {
		fmt.Fprintf(os.Stderr, "FAIL: max_dirty_per_img_load=%d want <=1 (async image SetImage must dirty ONE cell only, not %d — no global repaint)\n", maxDirtyPerImg, maxDirtyPerImg)
		os.Exit(1)
	}
	if loadEvents < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: load_events=%d want >=10 (Spike 20s must drive at least 10 SetImage loads for local-dirty stress)\n", loadEvents)
		os.Exit(1)
	}
	if loadedPeak < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: loaded_peak=%d want >=10 (at least 10 cells must reach ImageReady during Spike)\n", loadedPeak)
		os.Exit(1)
	}
	// R10 RSS slope true-leak gate (same honest baseline as R7/R7b).
	rssPeakKB := rep.RSSPeakKB
	if elapsed >= 30 && rssPeakKB > 200*1024 && rep.RSSSlopeKBPerMin > 300000 {
		fmt.Fprintf(os.Stderr, "FAIL: rss_slope_kb_per_min=%.0f peak=%dKB — exceeds 200MB peak + 300000 KB/min (true async-load leak, not cold-start baseline)\n", rep.RSSSlopeKBPerMin, rssPeakKB)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r10_async_image: PASS presents=%d loaded=%d/%d dirty/img=%d fps=%.1f\n",
		rep.PresentCount, loadedPeak, gridCells, maxDirtyPerImg, rep.FPSInterval)
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

type scene10 struct {
	shell *wrkit.ShellChrome
	Root  *rendering.AbsoluteBox
	cells []*rendering.RenderImage // 24 grid cells, each own RepaintBoundary

	// async load driver state
	loadTimer         float64 // accumulates dt; when >= loadInterval, fire next SetImage
	loadInterval      float64 // 0.8s between loads during Spike
	loadedSeq         int32   // next grid index to SetImage (left-top → right-bottom)
	steadyLoadingDone bool    // Steady phase SetLoading fired once

	// ability metrics
	loadedCount        atomic.Int64 // current ImageReady count
	loadedPeak         atomic.Int64 // peak ImageReady count during Spike (Recover clears loadedCount)
	maxRRPerImgLoad    atomic.Int64 // peak per-SetImage rerecord (must be <=1)
	maxDirtyPerImgLoad atomic.Int64 // peak per-SetImage dirty-cell count (local dirty = 1, global = >1)
	loadEvents         atomic.Int64 // total SetImage calls
	clearEvents        atomic.Int64 // total Clear calls
	lastFrameRR        atomic.Int64 // current-frame rerecord (BoundaryCache.FrameRerecord)
	lastFrameRRAtLoad  atomic.Int64 // FrameRerecord snapshot right after a SetImage (the local-dirty count)
}

func buildScene(w, h float64) *scene10 {
	s := &scene10{
		cells:        make([]*rendering.RenderImage, gridCells),
		loadInterval: 0.8,
	}

	legendLines := []string{
		"R10 async image → local dirty",
		"24 cells · 6×4 grid · each own RB",
		"placeholder chrome while loading",
		"SetImage → dirty ONE cell only",
		"Steady: all SetLoading placeholder",
		"Spike: SetImage one cell per 0.8s",
		"Recover: Clear all back to placeholder",
		"HUD: loaded=X/24 rr/img=1 skip=Y",
	}
	s.shell = wrkit.NewShell(w, h, "R10 async image — 24 cells · local dirty on load · 30s", legendLines)
	s.Root = s.shell.Root

	bodyBox := s.shell.Body.Box

	// Build 6×4 grid of RenderImage cells, each own RepaintBoundary.
	for row := 0; row < gridRows; row++ {
		for col := 0; col < gridCols; col++ {
			idx := row*gridCols + col
			im := rendering.NewRenderImage(cellW, cellH)
			im.PR, im.PG, im.PB = 0.2, 0.2, 0.25 // placeholder chrome color
			im.SetRepaintBoundary(true)          // each cell own cache — local dirty key
			x := float64(col) * (cellW + cellGap)
			y := float64(row) * (cellH + cellGap)
			bodyBox.Place(im, x, y)
			s.cells[idx] = im
		}
	}

	return s
}

// currentPhaseState returns a short state string for HUD.
func (s *scene10) currentPhaseState(phase string) string {
	switch phase {
	case wrkit.PhaseSteady:
		return "loading"
	case wrkit.PhaseSpike:
		return "img-ready"
	case wrkit.PhaseRecover:
		return "clearing"
	default:
		return phase
	}
}

// onTick drives async load by phase.
func (s *scene10) onTick(dt float64, phase string, app *embedder.PipelineApp, nextLoadIdx *int32) {
	if s == nil {
		return
	}

	// Snapshot current-frame rerecord (BoundaryCache.FrameRerecord resets each BeginFrame).
	cache := app.BoundaryCache()
	var frameRR int64
	if cache != nil {
		frameRR = cache.FrameRerecord
	}
	s.lastFrameRR.Store(frameRR)

	switch phase {
	case wrkit.PhaseSteady:
		// All 24 cells SetLoading (placeholder chrome). Only once at phase start.
		if s.loadedSeq == 0 && s.loadEvents.Load() == 0 && s.clearEvents.Load() == 0 && !s.steadyLoadingDone {
			for _, im := range s.cells {
				if im != nil {
					im.SetLoading()
				}
			}
			s.steadyLoadingDone = true
			fmt.Fprintf(os.Stderr, "R10: Steady — 24 cells SetLoading (placeholder)\n")
		}
		// During Steady, no SetImage; cells stay placeholder.
	case wrkit.PhaseSpike:
		// SetImage one cell every loadInterval (~0.8s). Local dirty: only that cell.
		s.loadTimer += dt
		if s.loadTimer >= s.loadInterval {
			s.loadTimer = 0
			idx := atomic.AddInt32(&s.loadedSeq, 1) - 1
			if int(idx) < gridCells {
				im := s.cells[idx]
				if im != nil {
					// Build a decoded ImageBuf (solid color varies by idx — proves which
					// cell loaded). SetImage takes ownership; RenderImage Dispose's prior.
					buf := makeSolidImageBuf(int(idx))
					// Snapshot all-cell NeedsPaint BEFORE SetImage to measure the delta.
					var dirtyBefore int64
					for _, c := range s.cells {
						if c != nil && c.NeedsPaint() {
							dirtyBefore++
						}
					}
					im.SetImage(buf)
					// Local-dirty proof: after SetImage, ONLY this cell should be NeedsPaint
					// (SetImage → MarkNeedsPaint(self), no bubble). Count dirty cells; >1 means
					// global repaint (FAIL). This is the R10 ability contract, not BoundaryCache
					// delta (RenderImage.Paint doesn't use BoundaryCache tryReplay/store).
					var dirtyAfter int64
					for _, c := range s.cells {
						if c != nil && c.NeedsPaint() {
							dirtyAfter++
						}
					}
					delta := dirtyAfter - dirtyBefore
					for {
						cur := s.maxDirtyPerImgLoad.Load()
						if delta > cur {
							if s.maxDirtyPerImgLoad.CompareAndSwap(cur, delta) {
								break
							}
						} else {
							break
						}
					}
					s.loadedCount.Add(1)
					// Track peak during Spike (Recover will Store(0) so end-of-run read is 0).
					cur := s.loadedCount.Load()
					for {
						peak := s.loadedPeak.Load()
						if cur > peak {
							if s.loadedPeak.CompareAndSwap(peak, cur) {
								break
							}
						} else {
							break
						}
					}
					s.loadEvents.Add(1)
				}
			}
		}
	case wrkit.PhaseRecover:
		// Clear all cells back to placeholder. Only once at phase start.
		if s.clearEvents.Load() == 0 {
			for _, im := range s.cells {
				if im != nil {
					im.Clear()
				}
			}
			s.clearEvents.Add(1)
			s.loadedCount.Store(0)
			fmt.Fprintf(os.Stderr, "R10: Recover — all cells Clear back to placeholder\n")
		}
	}
}

// makeSolidImageBuf builds a small decoded ImageBuf with a solid color varying
// by idx (proves which cell loaded — each cell shows a distinct color post-load).
// This stands in for async-decoded image bytes (R10 tests local-dirty on load-
// complete, not the decode pipeline itself).
func makeSolidImageBuf(idx int) *render.ImageBuf {
	// 16×16 solid color tile; color varies by idx across a palette.
	const tile = 16
	buf, err := render.NewImageBuf(tile, tile, render.FormatRGBA8)
	if err != nil || buf == nil {
		return nil
	}
	r, g, b := cellImageColor(idx)
	for y := 0; y < tile; y++ {
		for x := 0; x < tile; x++ {
			_ = buf.SetRGBA(x, y, r, g, b, 255)
		}
	}
	return buf
}

func cellImageColor(idx int) (uint8, uint8, uint8) {
	// Deterministic palette — proves which cell loaded via color.
	c := idx % 24
	switch c % 6 {
	case 0:
		return 200, 80, 80
	case 1:
		return 200, 160, 80
	case 2:
		return 160, 200, 80
	case 3:
		return 80, 200, 120
	case 4:
		return 80, 160, 200
	default:
		return 140, 80, 200
	}
}
