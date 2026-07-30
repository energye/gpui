// Command ui_wr_r13_hit is the R13 gate: hit test ≡ paint identity (DebugName).
// Scripted probes at known centers (no reliance on manual click alone).
//
// Quality bar (wr-close 模式 2 · §2.6 · U17/U18/U20):
//
//	wrkit Shell + Legend≥5 + LiveHUD + PhaseClock + EnsureUIFace + multi-region Panel
//	multi-color/shape hit targets + empty area + scripted probes + live pointer highlight
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r13_hit
//
// Gates: present_policy=full_paint, fps_interval≥55, scripted_ok==scripted_total,
// §2.2 full family A–J.
package main

import (
	"fmt"
	"os"
	"sync"
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
	closeSecs  = 5
)

func main() {
	secs := wrkit.RunSeconds(closeSecs)
	wrkit.RequireMinRun(secs, "R13")
	fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: R13 hit ≡ paint quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH,
		Title: "gpui ui_wr_r13_hit",
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
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
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
	// Targets placed body-local by buildScene; probe window coords computed from
	// wrkit.NewShell body origin (bodyX=12+260+12=284, bodyY=48+12=60) + hotPanel
	// offset (6,8) → hotPanel origin (290,68). box centers:
	//   green box1 @(24,36) 200×200 → (414,204)
	//   red   box2 @(260,80) 160×160 → (630,228)
	//   cyan  box3 @(180,260) 140×140 → (540,398)
	probes := []struct {
		x, y float64
		want string
	}{
		{414, 204, "green"},
		{630, 228, "red"},
		{540, 398, "cyan"},
		{50, 50, ""}, // empty background (top bar area)
	}

	phases := wrkit.NewPhaseClock(1.5, 3.0) // Steady 0–1.5 · Spike 1.5–3 · Recover
	probed := false
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
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
		// Phase-driven hit feedback: Spike cycles highlight of green to prove
		// hit ≡ paint identity under varied paint pressure.
		if ph == wrkit.PhaseSpike && app.PresentCount()%8 == 0 {
			sc.highlight("green")
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
			mu.Lock()
			nLive := len(liveHits)
			mu.Unlock()
			gateOK := fps >= 55 || phases.Elapsed() < 2
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R13",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("probes=%d/%d live=%d", probesOK, probesN, nLive),
				GateOK:      gateOK,
				Extra:       "scripted probes hit DebugName ≡ paint identity",
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
			"client_px":        "1200x800",
			"run_seconds":      secs,
			"scripted_ok":      probesOK,
			"scripted_total":   probesN,
			"pointer_hits":     hitsCopy,
			"targets":          []string{"green@ body(180,100)", "red@ (700,280)", "cyan@ (500,500)"},
			"static_cells":     sc.staticCells,
			"label_count":      sc.labelCount,
			"phases":           "Steady/Spike/Recover",
			"regions":          "TopBar/Legend/HitTargets/EmptyZone/HUD",
			"impl_correctness": "HitTestPointer DebugName ≡ paint identity; scripted probes at known centers all hit",
			"impl_dirty":       "pointer down + Spike highlight MarkNeedsPaint on hit target",
			"impl_cache":       "N/A for R13 (hit test uses tree, not BoundaryCache)",
			"impl_edge":        "empty background probe returns \"\"; Spike cycles green highlight",
			"impl_fail":        "policy≠full_paint / probes ok<total / fps<55 / target blanks",
			"impl_visible":     "LiveHUD probes/live; 3 colored hit boxes + empty zone; highlight on hit",
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
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if probesN < 4 || probesOK != probesN {
		fmt.Fprintf(os.Stderr, "FAIL: scripted hit probes ok=%d/%d\n", probesOK, probesN)
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8 (dense static proof)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r13_hit: PASS probes=%d/%d pointer_hits=%d cells=%d labels=%d\n",
		probesOK, probesN, len(hitsCopy), sc.staticCells, sc.labelCount)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene13 struct {
	Root        *rendering.AbsoluteBox
	box1        *rendering.RenderColorBox
	box2        *rendering.RenderColorBox
	box3        *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene13 {
	s := &scene13{}
	shell := wrkit.NewShell(w, h,
		"R13 hit ≡ paint identity · scripted probes · DebugName",
		[]string{
			"full_paint policy · HitTestPointer tree walk",
			"green target — DebugName=\"green\"",
			"red target — DebugName=\"red\"",
			"cyan target — DebugName=\"cyan\"",
			"empty background — probe returns \"\"",
			"scripted probes at known centers",
			"pointer down highlight target (visible)",
			"Spike cycles green highlight feedback",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	body := shell.Body
	s.labelCount = 8 // legend lines

	// --- Hit targets region (left/center, three colored DebugName boxes) ---
	hotPanel := wrkit.NewPanel(body.W*0.55, body.H-16, 0.10, 0.11, 0.14, 1)
	hotPanel.PlaceOn(body.Box, 6, 8)
	hotPanel.LabelAt("HIT TARGETS (DebugName ≡ paint identity)", 12, 10, 8, 0.95, 0.55, 0.40)
	s.labelCount++
	s.box1 = hotPanel.ColorAt(200, 200, 24, 36, 0.2, 0.55, 0.3, 1, true)
	s.box1.SetDebugName("green")
	s.box2 = hotPanel.ColorAt(160, 160, 260, 80, 0.9, 0.25, 0.15, 1, true)
	s.box2.SetDebugName("red")
	s.box3 = hotPanel.ColorAt(140, 140, 180, 260, 0.15, 0.75, 0.85, 1, true)
	s.box3.SetDebugName("cyan")
	hotPanel.LabelAt("green · red · cyan — probes hit each DebugName", 11, 16, 410, 0.55, 0.75, 0.85)
	s.labelCount++

	// --- Empty zone region (right, static + empty background proof) ---
	emptyPanel := wrkit.NewPanel(body.W*0.42, body.H-16, 0.20, 0.22, 0.26, 1)
	emptyPanel.PlaceOn(body.Box, body.W*0.57, 8)
	emptyPanel.LabelAt("EMPTY ZONE (probe returns \"\")", 12, 10, 8, 0.65, 0.80, 0.90)
	s.labelCount++
	// Dense static grid 4×4 to prove hit ≡ paint under FullPaint dense static.
	const cell, gap = 36.0, 6.0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			emptyPanel.ColorAt(cell, cell,
				12+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.20+0.08*float64(c), 0.35+0.08*float64(r), 0.50, 1, true)
			s.staticCells++
		}
	}
	emptyPanel.LabelAt("static grid — no DebugName → hit \"\"", 11, 12, 32+4*(cell+gap)+8, 0.55, 0.70, 0.80)
	s.labelCount++
	emptyPanel.LabelAt("pointer optional — scripted probes determinate", 11, 12, 32+4*(cell+gap)+28, 0.55, 0.65, 0.70)
	s.labelCount++

	return s
}

func (s *scene13) highlight(name string) {
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
