// Command ui_wr_r9_text_cache is the R9 gate: text measure cache hits under
// repeated layout of stable content.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache
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
	secs := runSeconds(5)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R9 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: R9 measure cache — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r9_text_cache",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var layoutPasses atomic.Int64

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r9_text_cache: close")
			}
		},
	})

	// Warm measure cache once, then repeatedly MarkNeedsLayout on the same text
	// so measureLine hits the cache (R9).
	sc.txt.ResetMeasureCacheStats()
	_ = sc.txt.Layout(rendering.Loose(400, 400))
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		// Force layout each tick without changing text → measure cache hits.
		sc.txt.MarkNeedsLayout()
		if app.Pipeline().FlushLayout(rendering.Size{Width: float64(winW), Height: float64(winH)}, false) {
			layoutPasses.Add(1)
		}
		sc.panel.MarkNeedsPaint()
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

	hits, miss := sc.txt.MeasureCacheStats()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R9",
		Scenario:      "ui_wr_r9_text_cache",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":          "1200x800",
			"run_seconds":        secs,
			"measure_cache_hit":  hits,
			"measure_cache_miss": miss,
			"layout_passes":      layoutPasses.Load(),
			"sample_text":        sc.txt.Text,
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r9_text_cache: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if hits < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: measure_cache_hit=%d want ≥1\n", hits)
		os.Exit(1)
	}
	// After warm, hits should dominate misses for stable text.
	if hits < miss {
		fmt.Fprintf(os.Stderr, "FAIL: measure_cache_hit=%d < miss=%d (cache ineffective)\n", hits, miss)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: PASS hits=%d miss=%d layouts=%d\n",
		hits, miss, layoutPasses.Load())
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

type scene struct {
	Root  *rendering.AbsoluteBox
	panel *rendering.AbsoluteBox
	txt   *rendering.RenderText
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	panel := rendering.NewAbsoluteBox(520, 360)
	panel.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.18, A: 1}
	panel.SetRepaintBoundary(true)
	s.panel = panel

	txt := rendering.NewRenderText(
		"gpui measure cache demo — repeated wrap words for hit rate " +
			"alpha beta gamma delta epsilon zeta eta theta iota kappa " +
			"alpha beta gamma delta epsilon zeta eta theta iota kappa",
	)
	txt.FontSize = 18
	txt.ApproxCharW = 0.55
	txt.SetMaxWidth(480)
	txt.R, txt.G, txt.B, txt.A = 0.92, 0.93, 0.95, 1
	txt.SetRepaintBoundary(true)
	s.txt = txt
	panel.Place(txt, 20, 24)
	root.Place(panel, 100, 120)
	return s
}
