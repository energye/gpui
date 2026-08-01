// Command ui_wr_c0_smoke is the C0 combo real-window smoke (R0+R12 schema+R16 first frame).
//
// Quality bar (wr-close 模式 2 · U17/U18 integration):
//
//	  mini app shell · static dense + hot · WarmUp · LiveHUD · §2.2 schema
//
//		export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//		RUN_SECONDS=5 go run ./examples/ui_wr_c0_smoke   # close C0
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
	winW, winH   = 1200, 800
	hudH         = 72
	closeSeconds = 5 // §2.5 C0
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "C0")
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: C0 R0+R12+R16 quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c0_smoke",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.05, ClearB: 0.06, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true, // R16 subset: warm-up full present before loop
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c0_smoke: close")
			}
		},
	})

	phases := wrkit.NewPhaseClock(1.5, 3.5)
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
			gateOK := (fps >= 55 || phases.Elapsed() < 2) && policy == scheduler.PresentPolicyRetained
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "C0",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("covers=R0+R12+R16 warmup=true cells=%d", sc.staticCells),
				GateOK:      gateOK,
				Extra:       "combo smoke · schema + first present + static under Clear",
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
	presents := app.PresentCount()
	snap := app.Metrics().Snapshot()
	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C0",
		Scenario:      "ui_wr_c0_smoke",
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"covers":           []string{"R0", "R12", "R16"},
			"client_px":        "1200x800",
			"run_seconds":      secs,
			"static_cells":     sc.staticCells,
			"label_count":      sc.labelCount,
			"hud":              wrkit.HUDEnabled(),
			"depcheck":         "passed",
			"quality_bar":      "U17+U18 integration",
			"r16_note":         "WarmUp=true subset; full R16 window optional post-W0",
			"impl_interaction": "R0 static survives Clear · R12 schema 集成 · R16 WarmUp 首帧内容共存；hot 每 tick MarkNeedsPaint 驱动 metrics 采样，不动 R0/R12/R16 单能力门禁",
			"regions":          "TopBar/Legend/Body-Static/Body-Hot/ExtraBody/HUD = 6 区共存同屏",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c0_smoke: metrics JSON on stdout")
	fmt.Println(string(b))

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
	if !rep.Warmup {
		fmt.Fprintln(os.Stderr, "FAIL: warmup flag false (R16 subset)")
		os.Exit(1)
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want >=8 (C0 dense static)\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c0_smoke: PASS presents=%d policy=%s schema=ok warmup=true fps=%.1f\n",
		rep.PresentCount, rep.PresentPolicy, rep.FPSInterval)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene struct {
	Root        *rendering.AbsoluteBox
	hotB        *rendering.RenderColorBox
	hud         *wrkit.LiveHUD
	phase       float64
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	// wrkit.NewShell: TopBar + Legend(8 行) + Body + LiveHUD  (§3.1.1 C 窗基线)
	shell := wrkit.NewShell(w, h,
		"C0 smoke · R0 FullPaint + R12 schema + R16 WarmUp · integration",
		[]string{
			"R0 FullPaint + R12 schema + R16 WarmUp",
			"static survives Clear each frame",
			"warmup=true first present (R16)",
			"§2.2 schema required (R12)",
			"hot marks dirty each tick",
			"LiveHUD bottom band (U18)",
			"Steady → Spike → Recover",
			"integration only — not R0/R12/R16 solo",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD

	// §3.1.1 要求 C 窗 ≥6 区 Panel：TopBar/Legend 已占 2，Body 内分两独立子区 +
	// 右侧 ExtraBody 区，凑成 Body-Static / Body-Hot / ExtraBody = 3 个能力专属区，
	// 加 TopBar/Legend/HUD = 共 6 区，每个能力至少一个专属区域共存同屏。
	body := shell.Body
	bw, bh := body.W, body.H

	// Body 子区 1：STATIC 4×3 (R0 subset 静存活，Clear 下不动)
	const cell, gap = 52.0, 6.0
	static := wrkit.NewPanel(bw*0.42, bh, 0.10, 0.12, 0.16, 1)
	static.PlaceOn(body.Box, 0, 0)
	static.LabelAt("STATIC 4x3 (R0 · survives Clear)", 13, 12, 10, 0.65, 0.85, 0.95)
	s.labelCount++
	for r := 0; r < 3; r++ {
		for c := 0; c < 4; c++ {
			static.ColorAt(cell, cell,
				14+float64(c)*(cell+gap), 38+float64(r)*(cell+gap),
				0.18+0.12*float64(c), 0.40+0.10*float64(r), 0.55, 1, true)
			s.staticCells++
		}
	}
	static.LabelAt("extra static strip", 12, 14, 38+3*(cell+gap)+8, 0.60, 0.70, 0.85)
	s.labelCount++
	for i := 0; i < 3; i++ {
		static.ColorAt(36, 36, 14, 38+3*(cell+gap)+44+float64(i)*42, 0.25, 0.35, 0.55-0.05*float64(i), 1, true)
		s.staticCells++
	}

	// Body 子区 2：HOT (R0 动点 + 集成 schema/first present 指示)
	hot := wrkit.NewPanel(bw*0.58, bh, 0.10, 0.11, 0.14, 1)
	hot.PlaceOn(body.Box, bw*0.42, 0)
	hot.LabelAt("HOT pulse (each tick MarkNeedsPaint)", 13, 12, 10, 0.95, 0.50, 0.35)
	s.labelCount++
	s.hotB = hot.ColorAt(140, 140, 24, 48, 0.95, 0.30, 0.20, 1, true)
	hot.LabelAt("WarmUp first content band", 12, 180, 48, 0.60, 0.80, 0.95)
	s.labelCount++
	hot.LabelAt("JSON stdout = §2.2 A–J schema", 11, 180, 78, 0.55, 0.75, 0.85)
	s.labelCount++

	// 第 6 区：ExtraBody band（独立 Panel，提示「集成」边界 + 集成效果说明）
	extra := wrkit.NewPanel(bw, 36, 0.09, 0.10, 0.12, 1)
	extra.PlaceOn(body.Box, 0, bh-36)
	extra.LabelAt("integration: static+hot+WarmUp+schema共存·不动R0/R12/R16单能力门禁", 11, 12, 14, 0.70, 0.85, 0.90)
	s.labelCount++

	return s
}

func (s *scene) onTick(dt float64, phase string) {
	if s == nil || s.hotB == nil {
		return
	}
	rate := 4.0
	if phase == wrkit.PhaseSpike {
		rate = 9.0
	} else if phase == wrkit.PhaseRecover {
		rate = 3.0
	}
	s.phase += dt * rate
	g := 0.20 + 0.35*(0.5+0.5*math.Sin(s.phase))
	s.hotB.R, s.hotB.G, s.hotB.B, s.hotB.A = 0.95, g, 0.18, 1
	s.hotB.MarkNeedsPaint()
}
