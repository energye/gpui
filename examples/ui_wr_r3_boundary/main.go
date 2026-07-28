// Command ui_wr_r3_boundary is the R3 single-ability real-window gate for
// ENGINE_UI_WIDGET_RENDER W1 Boundary true cache (Picture Replay skip).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r3_boundary   # close R3 @ 1200×800
//
// Gates: presents>=1, present_policy, §2.2 schema, boundary_skip>0,
// boundary_rerecord>0 (hot dirty path), fps when RUN_SECONDS>=5.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
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
	// §2.5: R3 close duration 10s (min 5).
	secs := runSeconds(10)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R3 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: R3 Boundary cache — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
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
	// Publish boundary discovery for JSON (leaf count).
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		sc.onTick(dt)
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

	// Final discovery snapshot (stable tree).
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
	snap := app.Metrics().Snapshot()
	cache := app.BoundaryCache()
	var lifeSkip, lifeRR int64
	if cache != nil {
		lifeSkip, lifeRR = cache.Skip, cache.Rerecord
	}
	// Prefer metrics-accumulated; fall back to cache lifetime if async missed notes.
	if snap.BoundarySkip < lifeSkip {
		// Merge lifetime into report via Extra; also patch snap fields for gates.
		snap.BoundarySkip = lifeSkip
		snap.BoundaryRerecord = lifeRR
	}

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3",
		Scenario:      "ui_wr_r3_boundary",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":           "1200x800",
			"run_seconds":         secs,
			"static_boundary":     "green 180x180 @ (48,48) — must Replay (skip)",
			"hot_boundary":        "red pulsing @ (700,280) — re-record each tick",
			"cache_lifetime_skip": lifeSkip,
			"cache_lifetime_rr":   lifeRR,
			"boundary_count":      cnt,
			"boundary_max_depth":  depth,
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
		MinBoundarySkip:        1, // clean static must Replay at least once
		MinBoundaryRerecord:    1, // hot must re-record
	}
	if err := wrgate.EvaluateGates(rep, opt); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3_boundary: PASS presents=%d skip=%d rr=%d fps=%.1f\n",
		rep.PresentCount, rep.BoundarySkip, rep.BoundaryRerecord, rep.FPSInterval)
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
	Root    *rendering.AbsoluteBox
	staticB *rendering.RenderColorBox
	hotB    *rendering.RenderColorBox
	phase   float64
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	// Static repaint boundary — after warm-up must contribute boundary_skip via Replay.
	s.staticB = rendering.NewRenderColorBox(180, 180, 0.12, 0.72, 0.28, 1)
	s.staticB.SetRepaintBoundary(true)
	root.Place(s.staticB, 48, 48)

	// Hot repaint boundary — MarkNeedsPaint every tick → boundary_rerecord.
	s.hotB = rendering.NewRenderColorBox(140, 140, 0.95, 0.22, 0.18, 1)
	s.hotB.SetRepaintBoundary(true)
	root.Place(s.hotB, 700, 280)
	return s
}

func (s *scene) onTick(dt float64) {
	if s == nil || s.hotB == nil {
		return
	}
	s.phase += dt
	g := 0.12 + 0.4*(0.5+0.5*math.Sin(s.phase*5))
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.95, g, 0.18, 1
	s.hotB.MarkNeedsPaint()
}
