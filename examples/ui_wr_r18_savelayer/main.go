// Command ui_wr_r18_savelayer is the R18 gate: SaveLayer group opacity + budget reject.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer
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
	secs := runSeconds(10)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R18 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: R18 SaveLayer+budget — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r18_savelayer",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	var allowN, rejectN atomic.Int64
	sc := buildScene(float64(winW), float64(winH), &allowN, &rejectN)
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.95, ClearG: 0.95, ClearB: 0.97, ClearA: 1, // light bg so opacity blend visible
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r18_savelayer: close")
			}
		},
	})
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		if sc.panel != nil {
			sc.panel.MarkNeedsPaint()
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
	ok, rej := allowN.Load(), rejectN.Load()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R18",
		Scenario:      "ui_wr_r18_savelayer",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":        "1200x800",
			"run_seconds":      secs,
			"savelayer_allow":  ok,
			"savelayer_reject": rej,
			"savelayer_count":  ok,
			"budget":           "MaxOps=1 per paint (2nd SaveLayer rejected)",
			"note":             "semi-transparent red group over light bg; budget reject observable",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r18_savelayer: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if ok < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: savelayer_allow=%d want ≥1\n", ok)
		os.Exit(1)
	}
	if rej < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: savelayer_reject=%d want ≥1 (budget gate)\n", rej)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: PASS allow=%d reject=%d\n", ok, rej)
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
	Root  *rendering.AbsoluteBox
	panel *rendering.RenderBox
}

func buildScene(w, h float64, allow, reject *atomic.Int64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.95, G: 0.95, B: 0.97, A: 1}
	s.Root = root

	panel := rendering.NewRenderBox()
	panel.FixedWidth, panel.FixedHeight = 400, 300
	panel.SetRepaintBoundary(true)
	panel.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil {
			return
		}
		// Tight budget: only one SaveLayer allowed per paint.
		pc.LayerBudget = &rendering.SaveLayerBudget{MaxOps: 1, MaxArea: 1e9}
		if pc.SaveLayer(sz.Width, sz.Height, 0.45) {
			if allow != nil {
				allow.Add(1)
			}
			rendering.FillRect(pc, 40, 40, 200, 160, 0.95, 0.15, 0.1, 1)
			pc.Restore()
		}
		// Second attempt must reject (budget).
		if !pc.SaveLayer(100, 100, 0.5) {
			if reject != nil {
				reject.Add(1)
			}
		} else {
			pc.Restore()
		}
		// Direct yellow rect outside layer for contrast.
		rendering.FillRect(pc, 280, 60, 80, 80, 0.95, 0.85, 0.2, 1)
	}
	s.panel = panel
	root.Place(panel, 200, 150)
	return s
}
