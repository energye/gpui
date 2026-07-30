// Command ui_wr_r4b_multidamage is the R4b gate: two distant dirty hotspots;
// DirtyLayerIDs lists both; middle static stays; damage not full-screen.
//
// Quality bar (wr-close 模式 2 · §2.6 · U17/U18/U20):
//
//	wrkit Shell + Legend≥5 + LiveHUD + PhaseClock + EnsureUIFace + multi-region Panel
//	distant dual hots (TL+BR) + middle static survives + damage_multi independent scissors
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_r4b_multidamage
//
// Gates: present_policy=retained, fps_interval≥55, dirty_layer_id_max≥2,
// damage_multi_frames≥1, static_cells≥8; §2.2 full family A–J.
// NOTE: retained/CompositeOnly 下 boundary_skip=0 是引擎正确语义——静态靠
// GPU LoadOpLoad 保像素，不靠 Picture 缓存重放；故不对 boundary_skip 设门禁。
package main

import (
	"fmt"
	"math"
	"os"
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
	wrkit.RequireMinRun(secs, "R4b")
	fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: R4b multi damage quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH,
		Title: "gpui ui_wr_r4b_multidamage",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r4b_multidamage: close")
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	phases := wrkit.NewPhaseClock(4.0, 8.0) // Steady 0–4 · Spike 4–8 · Recover
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		sc.onTick(dt, ph)
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
				policy = scheduler.PresentPolicyRetained
			}
			_, _, _, multiN := app.DamageStats()
			maxDirty := app.MaxDirtyLayerIDCount()
			gateOK := fps >= 55 || phases.Elapsed() < 2
			if snap.P95FrameIntervalMs > 22 && phases.Elapsed() >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R4b",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("dirty_max=%d multi=%d cells=%d", maxDirty, multiN, sc.staticCells),
				GateOK:      gateOK,
				Extra:       "TL+BR distant hots · mid static survives · damage_multi independent scissors",
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
	sumA, maxA, samples, multiN := app.DamageStats()
	surf := int64(winW * winH)
	var avgRatio float64
	if samples > 0 && surf > 0 {
		avgRatio = float64(sumA) / float64(samples) / float64(surf)
	}
	maxDirty := app.MaxDirtyLayerIDCount()
	lastIDs := app.LastDirtyLayerIDs()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R4b",
		Scenario:      "ui_wr_r4b_multidamage",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: surf,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":            "1200x800",
			"run_seconds":          secs,
			"damage_ratio_avg":     avgRatio,
			"damage_ratio_max":     float64(maxA) / float64(surf),
			"damage_samples":       samples,
			"damage_multi_frames":  multiN,
			"dirty_layer_id_max":   maxDirty,
			"last_dirty_layer_ids": lastIDs,
			"hotspots":             "TL orange + BR cyan; mid gray static band",
			"static_cells":         sc.staticCells,
			"label_count":          sc.labelCount,
			"phases":               "Steady/Spike/Recover",
			"regions":              "TopBar/Legend/TLHot/MidStatic/BRHot/HUD",
			"impl_correctness":     "retained dual distant hots keep independent DirtyLayerIDs; union AABB may be large but damage_multi keeps scissors separate",
			"impl_dirty":           "TL+BR each MarkNeedsPaint every tick; mid static never dirty",
			"impl_cache":           "retained/CompositeOnly下静态靠GPU LoadOpLoad保像素；boundary_skip=0是正确语义",
			"impl_edge":            "Spike doubles TL+BR pulse rate; Recover returns to Steady pace",
			"impl_fail":            "policy≠retained / dirty_max<2 / multi<1 / fps<55 / static blanks",
			"impl_visible":         "LiveHUD dirty_max/multi/cells; TL orange + BR cyan pulse; mid gray static",
		},
	})
	rep.DamageRatio = avgRatio

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r4b_multidamage: metrics JSON follows on stdout")
	fmt.Println(string(b))

	// Distant dual hots: FrameDamage *union AABB* can be large (honest metric),
	// but PlanFramePresent keeps damage_multi (sum of rects ≪ surface). Gate on
	// multi mode + dirty ids — not union coverage (see render.PlanFramePresent).
	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MinFPSElapsed:         5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if maxDirty < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: dirty_layer_id_max=%d want ≥2 (two distant hots)\n", maxDirty)
		os.Exit(1)
	}
	if multiN < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: damage_multi_frames=%d want ≥1 (independent scissors for distant dirties)\n", multiN)
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8 (mid static proof)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r4b_multidamage: PASS dirty_max=%d multi_frames=%d union_dmg_avg=%.4f cells=%d labels=%d\n",
		maxDirty, multiN, avgRatio, sc.staticCells, sc.labelCount)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene4b struct {
	Root        *rendering.AbsoluteBox
	hotTL       *rendering.RenderColorBox
	hotBR       *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	phaseTL     float64
	phaseBR     float64
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene4b {
	s := &scene4b{}
	shell := wrkit.NewShell(w, h,
		"R4b retained · distant dual hots · DirtyLayerIDs · mid static survives",
		[]string{
			"retained policy · CompositeOnly + damage",
			"TL orange hot — MarkNeedsPaint each tick",
			"BR cyan hot — MarkNeedsPaint each tick",
			"mid gray static — never dirty between hots",
			"damage_multi independent scissors (not union)",
			"dirty_layer_id_max ≥ 2 (two distant hots)",
			"Spike accelerates both hots; Recover steady",
			"LiveHUD: dirty_max / multi / cells",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	body := shell.Body
	s.labelCount = 8 // legend lines

	// Body layout: TL hot | mid static | BR hot (three regions side by side).
	regionW := body.W / 3.0
	// --- TL hot region (left) ---
	tlPanel := wrkit.NewPanel(regionW-8, body.H-16, 0.20, 0.12, 0.10, 1)
	tlPanel.PlaceOn(body.Box, 6, 8)
	tlPanel.LabelAt("TL HOT (dirty · damage source)", 12, 10, 8, 0.95, 0.55, 0.30)
	s.labelCount++
	s.hotTL = tlPanel.ColorAt(120, 120, 24, 36, 0.95, 0.45, 0.10, 1, true)
	tlPanel.LabelAt("orange pulse — DirtyLayerID", 11, 24, 168, 0.90, 0.70, 0.55)
	s.labelCount++
	for i := 0; i < 3; i++ {
		tlPanel.ColorAt(36, 36, 156, 60+float64(i)*40, 0.30, 0.20, 0.15, 1, true)
		s.staticCells++
	}
	tlPanel.LabelAt("static corners — survive retained", 11, 12, 200, 0.55, 0.65, 0.70)
	s.labelCount++

	// --- Mid static region (center, never dirty) ---
	midPanel := wrkit.NewPanel(regionW-8, body.H-16, 0.22, 0.24, 0.28, 1)
	midPanel.PlaceOn(body.Box, regionW+2, 8)
	midPanel.LabelAt("MID STATIC (never dirty)", 12, 10, 8, 0.65, 0.80, 0.90)
	s.labelCount++
	// Dense mid static grid 3×3 to prove retained preserves pixels between distant hots.
	const cell, gap = 40.0, 6.0
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			midPanel.ColorAt(cell, cell,
				16+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.40+0.05*float64(c), 0.45+0.05*float64(r), 0.50, 1, true)
			s.staticCells++
		}
	}
	midPanel.LabelAt("static band between hots — retained keeps", 11, 12, 32+3*(cell+gap)+8, 0.55, 0.75, 0.80)
	s.labelCount++

	// --- BR hot region (right) ---
	brPanel := wrkit.NewPanel(regionW-8, body.H-16, 0.10, 0.16, 0.20, 1)
	brPanel.PlaceOn(body.Box, 2*regionW+6, 8)
	brPanel.LabelAt("BR HOT (dirty · damage source)", 12, 10, 8, 0.30, 0.75, 0.85)
	s.labelCount++
	s.hotBR = brPanel.ColorAt(120, 120, 24, 36, 0.15, 0.75, 0.85, 1, true)
	brPanel.LabelAt("cyan pulse — DirtyLayerID", 11, 24, 168, 0.55, 0.85, 0.90)
	s.labelCount++
	for i := 0; i < 3; i++ {
		brPanel.ColorAt(36, 36, 156, 60+float64(i)*40, 0.15, 0.20, 0.30, 1, true)
		s.staticCells++
	}
	brPanel.LabelAt("static corners — survive retained", 11, 12, 200, 0.55, 0.65, 0.70)
	s.labelCount++

	return s
}

func (s *scene4b) onTick(dt float64, phase string) {
	if s == nil || (s.hotTL == nil && s.hotBR == nil) {
		return
	}
	rate := 4.0
	switch phase {
	case wrkit.PhaseSpike:
		rate = 10.0
	case wrkit.PhaseRecover:
		rate = 3.0
	}
	s.phaseTL += dt * rate
	s.phaseBR += dt * (rate + 1.5) // slightly off-phase to keep independent DirtyLayerIDs
	oTL := 0.5 + 0.5*math.Sin(s.phaseTL)
	oBR := 0.5 + 0.5*math.Sin(s.phaseBR)
	if s.hotTL != nil {
		s.hotTL.R, s.hotTL.G, s.hotTL.B = 0.95, 0.30+0.40*oTL, 0.10+0.10*math.Cos(s.phaseTL*0.7)
		s.hotTL.MarkNeedsPaint()
	}
	if s.hotBR != nil {
		s.hotBR.R, s.hotBR.G, s.hotBR.B = 0.10+0.10*math.Cos(s.phaseBR*0.7), 0.55+0.30*oBR, 0.85
		s.hotBR.MarkNeedsPaint()
	}
}
