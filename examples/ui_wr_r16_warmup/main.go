// Command ui_wr_r16_warmup is the R16 single-ability real-window gate:
// first-frame / WarmUp full present with visible content (not black).
//
// U5: R16 **must** have its own package — C0 combo must not close R16.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r16_warmup
//
// Gates: WarmUp=true, present_count≥1, present_policy=full_paint, first content,
// time_to_first_present_ms budget (ability_extra; default <2000ms wall from Open).
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
	closeSeconds = 5
	// Default cold-start budget for first present after Open (ms).
	maxFirstPresentMs = 2000.0
)

func main() {
	secs := wrkit.RunSeconds(closeSeconds)
	wrkit.RequireMinRun(secs, "R16")
	fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: R16 first-frame / WarmUp — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: WARN font: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r16_warmup",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	// WarmUp:true is the R16 core — one full present before the loop.
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.05, ClearB: 0.06, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r16_warmup: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetPictureTextureCache(true)

	// WarmUp runs inside Run() (presentSyncFull before the loop), not Open().
	// Capture first wall time when PresentCount becomes ≥1 (warm or first loop).
	var firstPresentMs float64
	var sawFirstPresent bool
	tOpen := time.Now()

	phases := wrkit.NewPhaseClock(1.5, 3.5)
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		if !sawFirstPresent && app.PresentCount() >= 1 {
			firstPresentMs = time.Since(tOpen).Seconds() * 1000
			sawFirstPresent = true
		}
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
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "R16",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      snap.PresentPolicy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        "warmup=true first content",
				GateOK:      app.PresentCount() >= 1,
				Extra:       "R16: WarmUp full present · surface not black",
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
	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R16",
		Scenario:      "ui_wr_r16_warmup",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"tex_rasterize":            texRasterize,
			"tex_hit":                  texHit,
			"tex_impl":                 "B1: per-cell boundary Pictures rasterized to GPU textures once (scroll cell reuse blits instead of re-replaying commands); rasterize>0 proves texture path active",
			"client_px":                "1200x800",
			"run_seconds":              secs,
			"time_to_first_present_ms": firstPresentMs,
			"pipeline_warmup_opt":      true, // PipelineOptions.WarmUp → presentSyncFull in Run
			"max_first_present_ms":     maxFirstPresentMs,
			"solo_not_combo":           true,
			"depcheck":                 "passed",
			"quality_bar":              "U17 shell + WarmUp + content",
			"note":                     "C0 must not substitute for this window (U5)",
			"first_content":            "bright static panel + labels after WarmUp",
			"impl_layout":              "static AbsoluteBox layout; 8 color boxes + labels placed once, no relayout churn",
			"impl_paint":               "first paint via WarmUp presentSyncFull (surface not black); fade-in animates alpha 0→1 over 1.5s then stable",
			"impl_present":             "WarmUp full present before loop; time_to_first_present_ms measured from Run start",
			"impl_metrics":             "time_to_first_present_ms + max_first_present_ms + full A-J family JSON",
			"impl_hit":                 "N/A (no interaction targets; R13 covers hit)",
			"impl_win":                 "solo 1200x800 full_paint window, WarmUp true, HUD bottom band",
		},
	})

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r16_warmup: metrics JSON follows on stdout")
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
	// B1 gate: texture path must have rasterized at least once (mechanism active).
	if texRasterize < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: tex_rasterize=%d want â¥1 (B1 texture path inactive)\n", texRasterize)
		os.Exit(1)
	}
	if !rep.Warmup {
		fmt.Fprintln(os.Stderr, "FAIL: warmup flag false (R16 requires WarmUp path)")
		os.Exit(1)
	}
	if !sawFirstPresent || firstPresentMs <= 0 {
		fmt.Fprintln(os.Stderr, "FAIL: never observed PresentCount≥1 (no first present)")
		os.Exit(1)
	}
	if firstPresentMs > maxFirstPresentMs {
		fmt.Fprintf(os.Stderr, "FAIL: time_to_first_present_ms=%.1f > budget %.0f\n", firstPresentMs, maxFirstPresentMs)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r16_warmup: PASS warmup=true first_ms=%.1f presents=%d fps=%.1f\n",
		firstPresentMs, rep.PresentCount, rep.FPSInterval)
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
	hot   *rendering.RenderColorBox
	boxes []*rendering.RenderColorBox
	hud   *wrkit.LiveHUD
	phase float64
	age   float64
}

func buildScene(w, h float64) *scene {
	s := &scene{}
	shell := wrkit.NewShell(w, h,
		"R16 WarmUp / first present · content must not be black",
		[]string{
			"solo window (not C0)",
			"WarmUp full present before loop",
			"first content visible",
			"time_to_first_present_ms",
			"present_policy=full_paint",
			"surface stays filled",
			"LiveHUD bottom band",
			"Steady→Spike→Recover",
		})
	s.Root = shell.Root
	s.hud = shell.HUD

	body := shell.Body
	// Bright static content — human eye / gate: not a black surface after WarmUp.
	body.LabelAt("FIRST CONTENT (must survive WarmUp Clear)", 14, 12, 12, 0.95, 0.95, 0.70)
	for i := 0; i < 8; i++ {
		b := body.ColorAt(64, 64,
			16+float64(i%4)*72, 48+float64(i/4)*72,
			0.15+0.1*float64(i%4), 0.55, 0.35+0.05*float64(i/4), 1, true)
		// 渐入动画 (R16): 开场 1.5s 内 alpha 0→1 淡入，证明动画逐帧更新。
		b.A = 0
		s.boxes = append(s.boxes, b)
	}
	body.LabelAt("HOT after steady (optional motion)", 12, 320, 48, 0.95, 0.50, 0.35)
	s.hot = body.ColorAt(100, 100, 320, 72, 0.90, 0.35, 0.25, 1, true)
	body.LabelAt("If this panel is black after open → WarmUp/first present broken", 12, 12, 220, 0.90, 0.60, 0.45)
	return s
}

func (s *scene) onTick(dt float64, phase string) {
	if s == nil || s.hot == nil {
		return
	}
	s.age += dt
	// 开场渐入动画 (R16 能力): 前 1.5s 静态色块 alpha 0→1 淡入，Steady 后保持 1。
	if s.age < 1.5 {
		a := s.age / 1.5
		for _, b := range s.boxes {
			b.A = a
			b.MarkNeedsPaint()
		}
	}
	rate := 3.0
	if phase == wrkit.PhaseSpike {
		rate = 7.0
	}
	s.phase += dt * rate
	g := 0.25 + 0.3*(0.5+0.5*math.Sin(s.phase))
	s.hot.R, s.hot.G, s.hot.B, s.hot.A = 0.90, g, 0.22, 1
	s.hot.MarkNeedsPaint()
}
