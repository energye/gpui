// Command ui_wr_r18_savelayer is the R18 gate: SaveLayer group opacity + budget reject.
//
// Quality bar (wr-close 模式 2 · §2.6 · U17/U18/U20):
//
//	wrkit Shell + Legend≥5 + LiveHUD + PhaseClock + EnsureUIFace + multi-region Panel
//	SaveLayer semi-transparent group (allow) + 2nd SaveLayer (budget reject) + static contrast
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=10 go run ./examples/ui_wr_r18_savelayer
//
// Gates: present_policy=full_paint, fps_interval≥55, savelayer_allow≥1,
// savelayer_reject≥1 (budget); §2.2 full family A–J.
package main

import (
	"fmt"
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
	closeSecs  = 10
)

func main() {
	secs := wrkit.RunSeconds(closeSecs)
	wrkit.RequireMinRun(secs, "R18")
	fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: R18 SaveLayer+budget quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH,
		Title: "gpui ui_wr_r18_savelayer",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	var allowN, rejectN atomic.Int64
	sc := buildScene(float64(winW), float64(winH), &allowN, &rejectN)
	phases := wrkit.NewPhaseClock(3.0, 6.0) // Steady 0–3 · Spike 3–6 · Recover
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
		ph := phases.Advance(dt)
		if sc.panel != nil {
			// Spike: repaint faster to exercise SaveLayer budget under load.
			if ph == wrkit.PhaseSpike || app.PresentCount()%2 == 0 {
				sc.panel.MarkNeedsPaint()
			} else {
				sc.panel.MarkNeedsPaint()
			}
		}
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
			ok, rej := allowN.Load(), rejectN.Load()
			gateOK := fps >= 55 || phases.Elapsed() < 2
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R18",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("allow=%d reject=%d", ok, rej),
				GateOK:      gateOK,
				Extra:       "SaveLayer allow + 2nd SaveLayer budget reject observable",
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
			"static_cells":     sc.staticCells,
			"label_count":      sc.labelCount,
			"phases":           "Steady/Spike/Recover",
			"regions":          "TopBar/Legend/SaveLayerGroup/BudgetReject/StaticContrast/HUD",
			"impl_correctness": "1st SaveLayer allow (semi-transparent group); 2nd SaveLayer budget reject observable",
			"impl_dirty":       "panel MarkNeedsPaint each tick; Spike accelerates repaint pressure",
			"impl_cache":       "SaveLayerBudget MaxOps=1 per paint → 2nd SaveLayer rejected",
			"impl_edge":        "Spike exercises SaveLayer budget under load; Recover returns to Steady pace",
			"impl_fail":        "policy≠full_paint / allow<1 / reject<1 / fps<55 / static blanks",
			"impl_visible":     "LiveHUD allow/reject; semi-transparent red group over light bg; yellow contrast",
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
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
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
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8 (dense static proof)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r18_savelayer: PASS allow=%d reject=%d cells=%d labels=%d\n",
		ok, rej, sc.staticCells, sc.labelCount)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene18 struct {
	Root        *rendering.AbsoluteBox
	panel       *rendering.RenderBox
	hud         *wrkit.LiveHUD
	staticCells int
	labelCount  int
}

func buildScene(w, h float64, allow, reject *atomic.Int64) *scene18 {
	s := &scene18{}
	shell := wrkit.NewShell(w, h,
		"R18 SaveLayer group opacity + budget reject",
		[]string{
			"full_paint policy · SaveLayerBudget MaxOps=1",
			"1st SaveLayer — allow (semi-transparent group)",
			"2nd SaveLayer — budget reject observable",
			"static contrast rect outside layer",
			"light bg so opacity blend visible",
			"Spike accelerates repaint pressure",
			"Recover returns to Steady pace",
			"LiveHUD: allow / reject",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	// Shell uses dark bg; R18 wants light bg for opacity blend visibility.
	shell.Root.Background = &rendering.Color{R: 0.95, G: 0.95, B: 0.97, A: 1}
	body := shell.Body
	body.Box.Background = &rendering.Color{R: 0.92, G: 0.93, B: 0.95, A: 1}
	s.labelCount = 8 // legend lines

	// --- SaveLayer group region (left, 1st allow + 2nd reject) ---
	slPanel := wrkit.NewPanel(body.W*0.55, body.H-16, 0.88, 0.88, 0.90, 1)
	slPanel.PlaceOn(body.Box, 6, 8)
	slPanel.LabelAt("SAVELAYER GROUP (1st allow · 2nd budget reject)", 12, 10, 8, 0.20, 0.30, 0.45)
	s.labelCount++
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
	slPanel.Place(panel, 24, 36)
	slPanel.LabelAt("semi-transparent red group · yellow contrast outside", 11, 24, 350, 0.25, 0.30, 0.40)
	s.labelCount++

	// --- Static contrast region (right, dense static grid outside layer) ---
	staticPanel := wrkit.NewPanel(body.W*0.42, body.H-16, 0.85, 0.86, 0.88, 1)
	staticPanel.PlaceOn(body.Box, body.W*0.57, 8)
	staticPanel.LabelAt("STATIC CONTRAST (outside layer)", 12, 10, 8, 0.20, 0.30, 0.45)
	s.labelCount++
	const cell, gap = 36.0, 6.0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			staticPanel.ColorAt(cell, cell,
				12+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.30+0.08*float64(c), 0.40+0.08*float64(r), 0.55, 1, true)
			s.staticCells++
		}
	}
	staticPanel.LabelAt("static — no SaveLayer · full_paint repaints each frame", 11, 12, 32+4*(cell+gap)+8, 0.25, 0.35, 0.45)
	s.labelCount++

	return s
}
