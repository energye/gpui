// Command ui_wr_r11_dpr is the R11 gate: size/DPR-class change invalidates
// boundary Picture cache — one rerecord wave, then skip recovery.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r11_dpr
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(15)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R11 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: R11 size/DPR cache invalidation — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r11_dpr",
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
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r11_dpr: close")
			}
		},
	})

	tStart := time.Now()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		elapsed := time.Since(tStart).Seconds()
		cache := app.BoundaryCache()
		ph := phase.Load()

		// ~3s: first size change + full cache clear (R11 invalidation wave).
		if ph == 0 && elapsed >= 3 {
			sc.resizeBox(220, 200)
			app.InvalidateBoundaryCache()
			invalidations.Add(1)
			phase.Store(1)
			if cache != nil {
				// Snapshot baseline after clear (next paints will rerecord).
				_ = cache
			}
		}
		// ~6s: second size change + invalidate.
		if ph == 1 && elapsed >= 6 {
			sc.resizeBox(160, 160)
			app.InvalidateBoundaryCache()
			invalidations.Add(1)
			phase.Store(2)
		}
		// Track skip/rr after first invalidation for recovery evidence.
		if ph >= 1 && cache != nil {
			postInvalRR.Store(cache.Rerecord)
			postInvalSkip.Store(cache.Skip)
		}
		// Steady paint of static (when clean → skip after re-warm).
		if sc.box != nil && !sc.box.NeedsPaint() {
			// leave clean so BoundaryCache can skip
		}
		proc.Sample()
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
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
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
	fmt.Fprintf(os.Stderr, "ui_wr_r11_dpr: PASS inval=%d rr=%d skip=%d\n", invN, rr, sk)
}

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

type demoScene struct {
	Root *rendering.AbsoluteBox
	box  *rendering.RenderColorBox
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root
	s.box = rendering.NewRenderColorBox(180, 180, 0.2, 0.55, 0.85, 1)
	s.box.SetRepaintBoundary(true)
	s.box.SetDebugName("dpr-box")
	root.Place(s.box, 80, 100)
	return s
}

func (s *demoScene) resizeBox(w, h float64) {
	if s == nil || s.box == nil {
		return
	}
	s.box.Width, s.box.Height = w, h
	s.box.MarkNeedsLayout()
	s.box.MarkNeedsPaint()
}
