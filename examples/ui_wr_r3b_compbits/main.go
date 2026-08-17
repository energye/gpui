// Command ui_wr_r3b_compbits is the W1 R3b real-window: compositing bits / boundary discovery.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R3b")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r3b_compbits — 合成位发现", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R3b 合成位 — 嵌套链≥3 + NeedsCompositing 可视化", []string{
		"boundary_count / max_depth in HUD",
		"NeedsCompositing=true nodes painted",
		"with bright border",
		"旁路 boundary（深度+1）",
	})

	// Chain: root body → a → b → c (3 boundaries) + sibling s (depth 2).
	a := rendering.NewAbsoluteBox(200, 200)
	a.SetRepaintBoundary(true)
	shell.Body.Place(a, 30, 30)

	b := rendering.NewAbsoluteBox(150, 150)
	b.SetRepaintBoundary(true)
	a.Place(b, 25, 25)

	c := rendering.NewAbsoluteBox(100, 100)
	c.SetRepaintBoundary(true)
	b.Place(c, 25, 25)

	hot := rendering.NewRenderColorBox(60, 60, 0.9, 0.3, 0.3, 1)
	c.Place(hot, 20, 20)

	// Sibling nest: depth 2.
	s := rendering.NewAbsoluteBox(140, 140)
	s.SetRepaintBoundary(true)
	shell.Body.Place(s, 260, 30)
	hotS := rendering.NewRenderColorBox(50, 50, 0.3, 0.5, 0.9, 1)
	s.Place(hotS, 45, 45)

	// Deep chain adds one more boundary for max_depth=3 proof.
	d := rendering.NewAbsoluteBox(60, 60)
	d.SetRepaintBoundary(true)
	c.Place(d, 20, 20)
	tD := wrkit.Label("d", 11, 0.8, 0.85, 0.9)
	d.Place(tD, 20, 20)

	// Static density: color + label mix inside every level of nest1.
	for i := 0; i < 3; i++ {
		cb := rendering.NewRenderColorBox(26, 26, 0.25+0.15*float64(i), 0.5, 0.65, 1)
		a.Place(cb, 195, 10+float64(i)*32)
		tl := wrkit.Label(fmt.Sprintf("n1-%d", i), 11, 0.7, 0.78, 0.88)
		b.Place(tl, 155, 10+float64(i)*32)
	}
	// Sibling nest static blocks.
	s.Place(rendering.NewRenderColorBox(28, 28, 0.55, 0.35, 0.25, 1), 10, 100)
	s.Place(wrkit.Label("sib static", 11, 0.75, 0.8, 0.88), 10, 118)
	// Third nest (depth 2) to widen sibling count.
	s2 := rendering.NewAbsoluteBox(120, 120)
	s2.SetRepaintBoundary(true)
	shell.Body.Place(s2, 430, 30)
	hotS2 := rendering.NewRenderColorBox(44, 44, 0.85, 0.6, 0.2, 1)
	s2.Place(hotS2, 38, 38)
	s2.Place(wrkit.Label("s2 static", 11, 0.7, 0.78, 0.88), 10, 98)
	// Non-boundary deep color row (compositingBits test on plain nodes).
	for i := 0; i < 6; i++ {
		nc := rendering.NewRenderColorBox(30, 30, 0.4+0.06*float64(i), 0.45, 0.55, 1)
		shell.Body.Place(nc, 20+float64(i)*38, 470)
	}

	// NeedsCompositing visualization: paint border on nodes whose bits are on.
	var needsComp func(n rendering.RenderObject) bool
	needsComp = func(n rendering.RenderObject) bool {
		if n == nil {
			return false
		}
		if rendering.NeedsCompositingOf(n) {
			return true
		}
		for _, ch := range n.Children() {
			if needsComp(ch) {
				return true
			}
		}
		return false
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	// Border paint hook: mark boundary nodes with bright outline when they
	// need compositing (proves bits propagate to root + sibling isolation).
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			hot.MarkNeedsPaint()
			hotS.MarkNeedsPaint()
		case wrkit.PhaseSpike:
			hotS2.MarkNeedsPaint()
			a.MarkNeedsPaint()
		default:
			d.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (count/max_depth are the R3b proof on screen).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundaryCount >= 3 && snapH.BoundaryMaxDepth >= 2
		shell.UpdateHUD("R3b", phase, app, gateOK,
			fmt.Sprintf("boundaries=%d depth=%d", snapH.BoundaryCount, snapH.BoundaryMaxDepth), "")
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3b",
		Scenario:      "ui_wr_r3b_compbits",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"root_needs_compositing": needsComp(shell.Root),
			"chain_depth":            3,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		MinBoundaryCount:       3,
		MinBoundaryMaxDepth:    2,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	if !needsComp(shell.Root) {
		fmt.Fprintln(os.Stderr, "FAIL: root NeedsCompositing=false (bits not propagated)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: OK count=%d depth=%d presents=%d\n",
		snap.BoundaryCount, snap.BoundaryMaxDepth, app.PresentCount())
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
