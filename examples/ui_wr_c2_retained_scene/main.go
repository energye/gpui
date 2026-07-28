// Command ui_wr_c2_retained_scene is the C2 combo: retained multi-boundary +
// Picture replay (covers R3+R4+R4b+R5). Does not close solo R packages.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_wr_c2_retained_scene
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
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(15)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close C2 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: C2 retained scene — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_c2_retained_scene",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.06, ClearB: 0.08, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_c2_retained_scene: close")
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)
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
	app.Metrics().SetBoundaryDiscovery(cnt, depth)

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	if cache := app.BoundaryCache(); cache != nil {
		if snap.BoundarySkip < cache.Skip {
			snap.BoundarySkip = cache.Skip
			snap.BoundaryRerecord = cache.Rerecord
		}
	}
	sumA, _, samples, multiN := app.DamageStats()
	surf := int64(winW * winH)
	var avgRatio float64
	if samples > 0 && surf > 0 {
		avgRatio = float64(sumA) / float64(samples) / float64(surf)
	}
	maxDirty := app.MaxDirtyLayerIDCount()

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "C2",
		Scenario:      "ui_wr_c2_retained_scene",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: surf,
		Warmup:        true,
		Extra: map[string]any{
			"client_px":            "1200x800",
			"run_seconds":          secs,
			"covers":               "R3+R4+R4b+R5",
			"damage_ratio_avg":     avgRatio,
			"damage_multi_frames":  multiN,
			"dirty_layer_id_max":   maxDirty,
			"last_dirty_layer_ids": app.LastDirtyLayerIDs(),
			"picture_op_count":     sc.pic.OpCount(),
			"replay_frames":        sc.replayFrames,
			"boundary_count":       cnt,
			"boundary_max_depth":   depth,
			"note":                 "combo only — does not close solo R rows",
		},
	})
	// Union AABB can be large with distant hots; report avg but do not gate on it alone.
	rep.DamageRatio = avgRatio

	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_c2_retained_scene: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents:           1,
		RequireRetainedPolicy: true,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MinFPSElapsed:         5,
		MinBoundaryCount:      3,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if sc.pic.OpCount() < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: picture_op_count=%d\n", sc.pic.OpCount())
		os.Exit(1)
	}
	if maxDirty < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: dirty_layer_id_max=%d want ≥2\n", maxDirty)
		os.Exit(1)
	}
	if multiN < 1 && avgRatio >= 0.5 {
		// Prefer multi scissors; if planner used union only, require modest coverage.
		fmt.Fprintf(os.Stderr, "FAIL: damage_multi_frames=0 and union dmg_avg=%.4f (want multi or low union)\n", avgRatio)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_c2_retained_scene: PASS dirty_max=%d multi=%d ops=%d union_dmg=%.4f\n",
		maxDirty, multiN, sc.pic.OpCount(), avgRatio)
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
	Root         *rendering.AbsoluteBox
	hot          *rendering.RenderColorBox
	hot2         *rendering.RenderColorBox
	picHost      *rendering.RenderBox
	pic          scene.Picture
	replayFrames int64
	phase        float64
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.06, G: 0.07, B: 0.09, A: 1}
	s.Root = root

	outer := rendering.NewAbsoluteBox(500, 360)
	outer.Background = &rendering.Color{R: 0.12, G: 0.14, B: 0.20, A: 1}
	outer.SetRepaintBoundary(true)

	static := rendering.NewRenderColorBox(120, 100, 0.2, 0.55, 0.3, 1)
	static.SetRepaintBoundary(true)
	outer.Place(static, 24, 24)

	s.hot = rendering.NewRenderColorBox(100, 100, 0.9, 0.25, 0.15, 1)
	s.hot.SetRepaintBoundary(true)
	outer.Place(s.hot, 280, 180)
	root.Place(outer, 40, 40)

	s.hot2 = rendering.NewRenderColorBox(90, 90, 0.2, 0.6, 0.9, 1)
	s.hot2.SetRepaintBoundary(true)
	root.Place(s.hot2, 1000, 600)

	s.pic = scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(620, 80, 100, 80, 0.8, 0.3, 0.2, 1)
		r.FillRect(660, 110, 100, 80, 0.25, 0.65, 0.35, 1)
	})
	s.picHost = rendering.NewRenderBox()
	s.picHost.FixedWidth, s.picHost.FixedHeight = 300, 250
	s.picHost.SetRepaintBoundary(true)
	s.picHost.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		s.pic.Replay(pc.DC)
		s.replayFrames++
	}
	root.Place(s.picHost, 40, 40)
	return s
}

func (s *demoScene) onTick(dt float64) {
	if s == nil {
		return
	}
	s.phase += dt
	o := 0.5 + 0.5*math.Sin(s.phase*5)
	if s.hot != nil {
		s.hot.R, s.hot.G, s.hot.B = 0.95, 0.2+0.4*o, 0.15
		s.hot.MarkNeedsPaint()
	}
	if s.hot2 != nil {
		s.hot2.R, s.hot2.G, s.hot2.B = 0.15, 0.5+0.3*o, 0.9
		s.hot2.MarkNeedsPaint()
	}
	if s.picHost != nil {
		s.picHost.MarkNeedsPaint()
	}
}
