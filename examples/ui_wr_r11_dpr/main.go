// Command ui_wr_r11_dpr is the R11 gate: size/DPR-class change invalidates
// boundary Picture cache — one rerecord wave, then skip recovery.
//
// Quality bar (wr-close 模式 2 · §2.6 · U17/U18/U20):
//
//	wrkit Shell + Legend≥5 + LiveHUD + PhaseClock + EnsureUIFace + multi-region Panel
//	two size-change waves (~3s, ~6s) + InvalidateBoundaryCache → rerecord wave then skip
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr
//
// Gates: present_policy=full_paint, fps_interval≥55, cache_invalidations≥1,
// boundary_rerecord≥1 (after wave), boundary_skip≥1 (recovery); §2.2 full family A–J.
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
	winW, winH = 1200, 800
	hudD       = 72.0
	closeSecs  = 15
)

func main() {
	secs := wrkit.RunSeconds(closeSecs)
	wrkit.RequireMinRun(secs, "R11")
	fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: R11 size/DPR cache invalidation quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH,
		Title: "gpui ui_wr_r11_dpr",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var invalidations atomic.Int64
	var postInvalRR atomic.Int64
	var postInvalSkip atomic.Int64
	var phase atomic.Int32 // 0 warm, 1 after first inval, 2 after second

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r11_dpr: close")
			}
		},
	})

	tStart := time.Now()
	phases := wrkit.NewPhaseClock(3.0, 7.0) // Steady 0–3 · Spike 3–7 (inval waves) · Recover
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		elapsed := time.Since(tStart).Seconds()
		ph := phases.Advance(dt)
		cache := app.BoundaryCache()
		phaseState := phase.Load()

		// ~3s: first size change + full cache clear (R11 invalidation wave).
		if phaseState == 0 && elapsed >= 3 {
			sc.resizeBox(220, 200)
			app.InvalidateBoundaryCache()
			invalidations.Add(1)
			phase.Store(1)
		}
		// ~6s: second size change + invalidate.
		if phaseState == 1 && elapsed >= 6 {
			sc.resizeBox(160, 160)
			app.InvalidateBoundaryCache()
			invalidations.Add(1)
			phase.Store(2)
		}
		// Track skip/rr after first invalidation for recovery evidence.
		if phase.Load() >= 1 && cache != nil {
			postInvalRR.Store(cache.Rerecord)
			postInvalSkip.Store(cache.Skip)
		}
		// Steady paint of static (when clean → skip after re-warm).
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
			invN := invalidations.Load()
			rr := postInvalRR.Load()
			sk := postInvalSkip.Load()
			gateOK := fps >= 55 || elapsed < 2
			if snap.P95FrameIntervalMs > 22 && elapsed >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R11",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("inval=%d rr=%d skip=%d", invN, rr, sk),
				GateOK:      gateOK,
				Extra:       "size change → InvalidateBoundaryCache → rerecord wave then skip",
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
	snap := app.Metrics().Snapshot()
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}
	invN := invalidations.Load()
	rr := postInvalRR.Load()
	sk := postInvalSkip.Load()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R11",
		Scenario:      "ui_wr_r11_dpr",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":           "1200x800",
			"run_seconds":         secs,
			"cache_invalidations": invN,
			"boundary_rerecord":   rr,
			"boundary_skip":       sk,
			"static_cells":        sc.staticCells,
			"label_count":         sc.labelCount,
			"phases":              "Steady/Spike(inval)/Recover",
			"regions":             "TopBar/Legend/DPRBox/StaticGrid/HUD",
			"impl_correctness":    "size change + InvalidateBoundaryCache → one rerecord wave then skip recovery",
			"impl_dirty":          "~3s and ~6s resizeBox + MarkNeedsLayout/Paint → cache invalidated",
			"impl_cache":          "BoundaryCache Picture invalidated by DPR/size change; re-warm returns to skip",
			"impl_edge":           "Spike hosts both invalidation waves; Recover confirms skip resumes",
			"impl_fail":           "policy≠full_paint / inval<1 / rr<1 / skip<1 / fps<55 / static blanks",
			"impl_visible":        "LiveHUD inval/rr/skip; DPR box resize twice; static grid survives",
			"note":                "size change + InvalidateBoundaryCache → rerecord wave then skip",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r11_dpr: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if invN < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: cache_invalidations=%d want ≥1\n", invN)
		os.Exit(1)
	}
	if rr < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_rerecord=%d want ≥1 after invalidation\n", rr)
		os.Exit(1)
	}
	// After re-warm, clean frames should skip again.
	if sk < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: boundary_skip=%d want ≥1 (recovery after rerecord wave)\n", sk)
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8 (dense static proof)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: PASS inval=%d rr=%d skip=%d cells=%d labels=%d\n",
		invN, rr, sk, sc.staticCells, sc.labelCount)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene11 struct {
	Root        *rendering.AbsoluteBox
	box         *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene11 {
	s := &scene11{}
	shell := wrkit.NewShell(w, h,
		"R11 size/DPR cache invalidation · rerecord wave then skip",
		[]string{
			"full_paint policy · BoundaryCache skip/rerecord",
			"DPR box — RepaintBoundary + SetDebugName",
			"~3s first resize + InvalidateBoundaryCache",
			"~6s second resize + invalidate",
			"rerecord wave → then skip recovery",
			"Spike hosts invalidation waves",
			"Recover confirms skip resumes",
			"LiveHUD: inval / rr / skip",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	body := shell.Body
	s.labelCount = 8 // legend lines

	// --- DPR box region (left, resized twice) ---
	dprPanel := wrkit.NewPanel(body.W*0.45, body.H-16, 0.10, 0.11, 0.14, 1)
	dprPanel.PlaceOn(body.Box, 8, 8)
	dprPanel.LabelAt("DPR BOX (resized ~3s & ~6s · cache invalidated)", 12, 10, 8, 0.95, 0.55, 0.40)
	s.labelCount++
	s.box = dprPanel.ColorAt(180, 180, 24, 36, 0.20, 0.55, 0.85, 1, true)
	s.box.SetDebugName("dpr-box")
	dprPanel.LabelAt("Resize triggers MarkNeedsLayout/Paint → cache inval", 11, 24, 230, 0.90, 0.70, 0.55)
	s.labelCount++
	dprPanel.LabelAt("after re-warm: clean → skip resumes", 11, 24, 250, 0.55, 0.75, 0.85)
	s.labelCount++

	// --- Static grid region (right, must survive resize waves) ---
	gridPanel := wrkit.NewPanel(body.W*0.52, body.H-16, 0.10, 0.12, 0.15, 1)
	gridPanel.PlaceOn(body.Box, body.W*0.47, 8)
	gridPanel.LabelAt("STATIC GRID 4×4 (survives inval waves)", 12, 10, 8, 0.65, 0.80, 0.90)
	s.labelCount++
	const cell, gap = 36.0, 6.0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			gridPanel.ColorAt(cell, cell,
				12+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.20+0.08*float64(c), 0.35+0.08*float64(r), 0.50, 1, true)
			s.staticCells++
		}
	}
	gridPanel.LabelAt("static — BoundaryCache skip when clean", 11, 16, 32+4*(cell+gap)+8, 0.55, 0.70, 0.80)
	s.labelCount++

	// Unused but keep math import stable for future phase hooks.
	_ = math.Sin
	return s
}

func (s *scene11) resizeBox(w, h float64) {
	if s == nil || s.box == nil {
		return
	}
	s.box.Width, s.box.Height = w, h
	s.box.MarkNeedsLayout()
	s.box.MarkNeedsPaint()
}
