// Command ui_wr_r13_hit is the R13 gate: hit test ≡ paint identity (DebugName).
// Scripted probes at known centers (no reliance on manual click alone).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r13_hit
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
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
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R13 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: R13 hit ≡ paint — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r13_hit",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var (
		mu       sync.Mutex
		liveHits []string
		probesOK int
		probesN  int
	)

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintln(os.Stderr, "ui_wr_r13_hit: close")
			case platform.EventPointer:
				if ev.Pointer != platform.PointerDown {
					return
				}
				if app == nil {
					return
				}
				_, hit, _ := app.HitTestPointer(ev.X, ev.Y)
				name := rendering.HitDebugName(hit)
				mu.Lock()
				liveHits = append(liveHits, name)
				mu.Unlock()
				fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: pointer (%.0f,%.0f) → %q\n", ev.X, ev.Y, name)
				sc.highlight(name)
				app.ScheduleFrame()
			}
		},
	})

	// Scripted probes after layout (deterministic gate; pointer optional extra).
	probes := []struct {
		x, y float64
		want string
	}{
		{180, 200, "green"}, // box1 center ~ (80+100, 100+100)
		{780, 360, "red"},   // box2 @ (700,280) 160×160 → center (780,360)
		{570, 570, "cyan"},  // box3 @ (500,500) 140×140 → center (570,570)
		{50, 50, ""},        // empty background
	}

	probed := false
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		if !probed && app.PresentCount() >= 2 {
			probed = true
			for _, p := range probes {
				_, hit, _ := app.HitTestPointer(p.x, p.y)
				got := rendering.HitDebugName(hit)
				probesN++
				if got == p.want {
					probesOK++
				} else {
					fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: probe (%.0f,%.0f) got %q want %q\n", p.x, p.y, got, p.want)
				}
			}
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
	mu.Lock()
	hitsCopy := append([]string(nil), liveHits...)
	mu.Unlock()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R13",
		Scenario:      "ui_wr_r13_hit",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":      "1200x800",
			"run_seconds":    secs,
			"scripted_ok":    probesOK,
			"scripted_total": probesN,
			"pointer_hits":   hitsCopy,
			"targets":        []string{"green@ (80,100)", "red@ (700,280)", "cyan@ (500,500)"},
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r13_hit: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if probesN < 4 || probesOK != probesN {
		fmt.Fprintf(os.Stderr, "FAIL: scripted hit probes ok=%d/%d\n", probesOK, probesN)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: PASS probes=%d/%d pointer_hits=%d\n",
		probesOK, probesN, len(hitsCopy))
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
	box1 *rendering.RenderColorBox
	box2 *rendering.RenderColorBox
	box3 *rendering.RenderColorBox
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	s.box1 = rendering.NewRenderColorBox(200, 200, 0.2, 0.55, 0.3, 1)
	s.box1.SetRepaintBoundary(true)
	s.box1.SetDebugName("green")
	root.Place(s.box1, 80, 100)

	s.box2 = rendering.NewRenderColorBox(160, 160, 0.9, 0.25, 0.15, 1)
	s.box2.SetRepaintBoundary(true)
	s.box2.SetDebugName("red")
	root.Place(s.box2, 700, 280)

	s.box3 = rendering.NewRenderColorBox(140, 140, 0.15, 0.75, 0.85, 1)
	s.box3.SetRepaintBoundary(true)
	s.box3.SetDebugName("cyan")
	root.Place(s.box3, 500, 500)
	return s
}

func (s *demoScene) highlight(name string) {
	if s == nil {
		return
	}
	// Dim non-targets; brighten hit target (visible feedback).
	reset := func(b *rendering.RenderColorBox, r, g, bl float64) {
		if b == nil {
			return
		}
		b.R, b.G, b.B, b.A = r, g, bl, 1
		b.MarkNeedsPaint()
	}
	reset(s.box1, 0.15, 0.35, 0.2)
	reset(s.box2, 0.5, 0.15, 0.1)
	reset(s.box3, 0.1, 0.4, 0.5)
	switch name {
	case "green":
		reset(s.box1, 0.25, 0.9, 0.35)
	case "red":
		reset(s.box2, 1, 0.35, 0.2)
	case "cyan":
		reset(s.box3, 0.2, 0.95, 1)
	}
}
