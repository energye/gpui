// Command ui_wr_c7_resize_dpr is the C7 combo: R11 size invalidation + R3 boundary
// skip recovery (+ optional 1px line for R19 awareness). Does not close solos.
//
// Quality bar (wr-close 模式 2 · §3.1 集成加强 · U17/U18/U20):
//
//	wrkit Shell + Legend≥8 + LiveHUD + PhaseClock + EnsureUIFace + ≥6 Panel regions
//	covers R11+R3(+R19 line); resize waves + boundary rerecord/skip + 1px hairline
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c7_resize_dpr
//
// Gates: present_policy=full_paint, fps_interval≥55, cache_invalidations≥1,
// boundary_rerecord≥1 (after wave), boundary_skip≥1 (recovery), boundary_count≥2,
// §2.2 full family A–J. Combo only — does NOT close solo R11/R3/R19.
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
	closeSecs  = 15
)

func main() {
	secs := wrkit.RunSeconds(closeSecs)
	wrkit.RequireMinRun(secs, "C7")
	fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: C7 resize/DPR combo quality bar — %ds @ 1200x800\n", secs)

	if _, path, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: WARN font: %v — labels may be blank\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: font ready (%s)\n", path)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH,
		Title: "gpui ui_wr_c7_resize_dpr",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	var inval atomic.Int64
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c7_resize_dpr: close")
			}
		},
	})

	// W6: retained is the engine default; this window's capability is
	// defined on the full_paint path, so opt back in explicitly.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.SetPictureTextureCache(true)
	cnt, depth := rendering.CountRepaintBoundaries(sc.Root)
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	tStart := time.Now()
	phases := wrkit.NewPhaseClock(4.0, 9.0) // Steady 0–4 · Spike 4–9 (resize waves) · Recover
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
		ph := phases.Advance(dt)
		el := time.Since(tStart).Seconds()
		// resize waves during Spike (R11 invalidation integration).
		if el >= 4 && el < 4.5 && inval.Load() == 0 {
			sc.resize(240, 180)
			app.InvalidateBoundaryCache()
			inval.Add(1)
		}
		if el >= 8 && el < 8.5 && inval.Load() == 1 {
			sc.resize(180, 220)
			app.InvalidateBoundaryCache()
			inval.Add(1)
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
			cache := app.BoundaryCache()
			sk, rr := int64(0), int64(0)
			if cache != nil {
				sk, rr = cache.Skip, cache.Rerecord
			}
			gateOK := fps >= 55 || el < 2
			if snap.P95FrameIntervalMs > 22 && el >= 2 {
				gateOK = false
			}
			sc.hud.Update(wrkit.Snap{
				AbilityID:   "C7",
				Phase:       ph,
				FPS:         fps,
				P95Ms:       snap.P95FrameIntervalMs,
				Policy:      policy,
				PresentMode: snap.PresentMode,
				PaintCount:  snap.PaintCount,
				Presents:    app.PresentCount(),
				Core:        fmt.Sprintf("inval=%d skip=%d rr=%d cnt=%d", inval.Load(), sk, rr, cnt),
				GateOK:      gateOK,
				Extra:       "R11 resize+R3 boundary skip recovery+R19 1px line · combo only",
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
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}

	texRasterize, texHit := app.PictureTextureCacheStats()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C7",
		Scenario:      "ui_wr_c7_resize_dpr",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"tex_rasterize":       texRasterize,
			"tex_hit":             texHit,
			"tex_impl":            "B1: per-cell boundary Pictures rasterized to GPU textures once (scroll cell reuse blits instead of re-replaying commands); rasterize>0 proves texture path active",
			"client_px":           "1200x800",
			"run_seconds":         secs,
			"covers":              []string{"R11", "R3", "R19(line)"},
			"cache_invalidations": inval.Load(),
			"boundary_count":      cnt,
			"boundary_max_depth":  depth,
			"static_cells":        sc.staticCells,
			"label_count":         sc.labelCount,
			"phases":              "Steady/Spike(resize)/Recover",
			"regions":             "TopBar/Legend/DPRBox/StaticGrid/1pxLine/HUD",
			"impl_interaction":    "R11 resize+InvalidateBoundaryCache triggers R3 boundary rerecord wave then skip recovery; R19 1px hairline static across resize",
			"impl_correctness":    "combo only — resize invalidates boundary cache; recovery returns to skip; 1px line survives",
			"impl_dirty":          "~4s and ~8s resize+MarkNeedsLayout/Paint → cache invalidated",
			"impl_cache":          "BoundaryCache Picture invalidated by DPR/size change; re-warm returns to skip",
			"impl_edge":           "Spike hosts both resize waves; Recover confirms skip resumes; 1px line snap",
			"impl_fail":           "policy≠full_paint / inval<1 / rr<1 / skip<1 / cnt<2 / fps<55",
			"impl_visible":        "LiveHUD inval/skip/rr/cnt; DPR box resize twice; static grid + 1px line survive",
			"note":                "combo only — does not close solo R11/R3/R19",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c7_resize_dpr: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MinBoundarySkip:        1,
		MinBoundaryCount:       2,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	// B1 gate: texture path must have rasterized at least once (mechanism active).
	if texRasterize < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: tex_rasterize=%d want â¥1 (B1 texture path inactive)\n", texRasterize)
		os.Exit(1)
	}
	// C7 特有集成不变式（GateOptions 并集的补充，非代替）：
	// B1 纹理路径激活 + 失效后重录 + 密集静态证明。能力门禁（skip/rr/cnt/depth）
	// 已在 GateOptions 并集中。
	if inval.Load() < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: cache_invalidations=%d\n", inval.Load())
		os.Exit(1)
	}
	if snap.BoundaryRerecord < 1 && rep.BoundaryRerecord < 1 {
		// lifetime may be in snap after merge
		if cache := app.BoundaryCache(); cache == nil || cache.Rerecord < 1 {
			fmt.Fprintln(os.Stderr, "FAIL: expected rerecord after invalidation")
			os.Exit(1)
		}
	}
	if sc.staticCells < 8 {
		fmt.Fprintf(os.Stderr, "FAIL: static_cells=%d want ≥8\n", sc.staticCells)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c7_resize_dpr: PASS inval=%d skip=%d rr=%d cnt=%d depth=%d cells=%d labels=%d\n",
		inval.Load(), rep.BoundarySkip, rep.BoundaryRerecord, cnt, depth, sc.staticCells, sc.labelCount)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type scene7 struct {
	Root        *rendering.AbsoluteBox
	box         *rendering.RenderColorBox
	line        *rendering.RenderColorBox // 1px-ish hairline (R19 awareness)
	hud         *wrkit.LiveHUD
	staticCells int
	labelCount  int
}

func buildScene(w, h float64) *scene7 {
	s := &scene7{}
	shell := wrkit.NewShell(w, h,
		"C7 resize/DPR combo · R11+R3(+R19 line) · combo only",
		[]string{
			"full_paint policy · BoundaryCache skip/rerecord",
			"R11: DPR box resized ~4s and ~8s + InvalidateBoundaryCache",
			"R3: boundary rerecord wave then skip recovery",
			"R19: 1px hairline static across resize (snap awareness)",
			"combo only — does NOT close solo R11/R3/R19",
			"Spike hosts resize waves; Recover confirms skip",
			"static grid survives resize invalidation",
			"LiveHUD: inval / skip / rr / cnt",
		},
	)
	s.Root = shell.Root
	s.hud = shell.HUD
	body := shell.Body
	s.labelCount = 8 // legend lines

	// --- DPR box region (left, resized twice) ---
	dprPanel := wrkit.NewPanel(body.W*0.45, body.H-16, 0.10, 0.11, 0.14, 1)
	dprPanel.PlaceOn(body.Box, 6, 8)
	dprPanel.LabelAt("DPR BOX (R11 resize ~4s & ~8s)", 12, 10, 8, 0.95, 0.55, 0.40)
	s.labelCount++
	s.box = dprPanel.ColorAt(200, 160, 24, 36, 0.25, 0.5, 0.85, 1, true)
	dprPanel.LabelAt("resize + InvalidateBoundaryCache → rerecord wave", 11, 24, 220, 0.90, 0.70, 0.55)
	s.labelCount++

	// --- R19 1px hairline (left, below DPR box) ---
	s.line = rendering.NewRenderColorBox(280, 1, 0.95, 0.95, 0.2, 1)
	s.line.SetRepaintBoundary(true)
	dprPanel.Place(s.line, 24, 260)
	dprPanel.LabelAt("R19 1px hairline — static across resize", 11, 24, 275, 0.85, 0.85, 0.40)
	s.labelCount++

	// --- Static grid region (right, must survive resize waves) ---
	gridPanel := wrkit.NewPanel(body.W*0.52, body.H-16, 0.10, 0.12, 0.15, 1)
	gridPanel.PlaceOn(body.Box, body.W*0.47, 8)
	gridPanel.LabelAt("STATIC GRID 4×4 (R3 boundary skip recovery)", 12, 10, 8, 0.65, 0.80, 0.90)
	s.labelCount++
	const cell, gap = 36.0, 6.0
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			gridPanel.ColorAt(cell, cell,
				12+float64(c)*(cell+gap), 32+float64(r)*(cell+gap),
				0.20+0.08*float64(c), 0.35+0.08*float64(r), 0.50, 1, true)
			s.staticCells++
		}
	}
	gridPanel.LabelAt("static — BoundaryCache skip when clean (after re-warm)", 11, 16, 32+4*(cell+gap)+8, 0.55, 0.70, 0.80)
	s.labelCount++
	gridPanel.LabelAt("combo only — does NOT close solo R11/R3/R19", 11, 16, 32+4*(cell+gap)+28, 0.55, 0.65, 0.70)
	s.labelCount++

	return s
}

func (s *scene7) resize(w, h float64) {
	if s == nil || s.box == nil {
		return
	}
	s.box.Width, s.box.Height = w, h
	s.box.MarkNeedsLayout()
	s.box.MarkNeedsPaint()
}
