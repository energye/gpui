// Command ui_wr_r5_picture is the R5 gate: Picture record once, Replay each frame
// vs direct paint of the same geometry (side-by-side).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(5)
	if secs < 5 {
		fmt.Fprintln(os.Stderr, "FAIL: RUN_SECONDS must be >= 5 to close R5 (U16)")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: R5 Picture record/replay — %ds @ 1200x800\n", secs)

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 1200, 800
	win, err := exhost.Open(exhost.Options{
		Width: winW, Height: winH, Title: "gpui ui_wr_r5_picture",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open (needs_gpu_window): %v\n", err)
		os.Exit(1)
	}

	sc := buildScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintln(os.Stderr, "ui_wr_r5_picture: close")
			}
		},
	})
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

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	opCount := sc.pic.OpCount()
	replays := sc.replayFrames

	rep := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R5",
		Scenario:      "ui_wr_r5_picture",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH),
		Warmup:        true,
		Extra: map[string]any{
			"client_px":        "1200x800",
			"run_seconds":      secs,
			"picture_op_count": opCount,
			"replay_frames":    replays,
			"direct_region":    "left  panels — live FillRect each frame",
			"replay_region":    "right panels — Record once, Replay each frame",
		},
	})
	b, err := wrgate.Marshal(rep)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, "ui_wr_r5_picture: metrics JSON follows on stdout")
	fmt.Println(string(b))

	if err := wrgate.EvaluateGates(rep, wrgate.GateOptions{
		MinPresents: 1, RequireFullPaintPolicy: true,
		RequirePersistentFPS: true, MinFPSWall: 55, MinFPSElapsed: 5,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if opCount < 3 {
		fmt.Fprintf(os.Stderr, "FAIL: picture_op_count=%d want ≥3\n", opCount)
		os.Exit(1)
	}
	if replays < 10 {
		fmt.Fprintf(os.Stderr, "FAIL: replay_frames=%d want ≥10\n", replays)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: PASS ops=%d replays=%d\n", opCount, replays)
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
	directBox    *rendering.RenderBox
	replayBox    *rendering.RenderBox
	pic          scene.Picture
	replayFrames int64
}

func buildScene(w, h float64) *demoScene {
	s := &demoScene{}
	root := rendering.NewAbsoluteBox(w, h)
	root.Background = &rendering.Color{R: 0.09, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	// Record once: three colored rects (same geometry as direct side, offset for right).
	const ox, oy = 620.0, 120.0
	s.pic = scene.RecordPicture(func(r *scene.PictureRecorder) {
		r.FillRect(ox+0, oy+0, 160, 120, 0.85, 0.25, 0.2, 1)
		r.FillRect(ox+40, oy+40, 160, 120, 0.2, 0.7, 0.35, 1)
		r.FillRect(ox+80, oy+80, 160, 120, 0.25, 0.4, 0.9, 1)
	})

	// Left: direct paint every frame via OnPaint.
	direct := rendering.NewRenderBox()
	direct.FixedWidth, direct.FixedHeight = 400, 400
	direct.SetRepaintBoundary(true)
	direct.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		drawDirect(pc.DC, 80, 120)
	}
	s.directBox = direct
	root.Place(direct, 40, 40)

	// Right: Replay recorded Picture every frame.
	replay := rendering.NewRenderBox()
	replay.FixedWidth, replay.FixedHeight = 400, 400
	replay.SetRepaintBoundary(true)
	replay.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		s.pic.Replay(pc.DC)
		s.replayFrames++
	}
	s.replayBox = replay
	root.Place(replay, 40, 40) // OnPaint uses absolute coords in picture; box is just a dirty host
	return s
}

func drawDirect(dc *render.Context, ox, oy float64) {
	if dc == nil {
		return
	}
	rects := []struct{ x, y, w, h, r, g, b float64 }{
		{ox + 0, oy + 0, 160, 120, 0.85, 0.25, 0.2},
		{ox + 40, oy + 40, 160, 120, 0.2, 0.7, 0.35},
		{ox + 80, oy + 80, 160, 120, 0.25, 0.4, 0.9},
	}
	for _, q := range rects {
		dc.SetRGBA(q.r, q.g, q.b, 1)
		dc.DrawRectangle(q.x, q.y, q.w, q.h)
		_ = dc.Fill()
	}
}

func (s *demoScene) onTick(dt float64) {
	if s == nil {
		return
	}
	// Both sides MarkNeedsPaint so FullPaint keeps drawing direct + Replay.
	if s.directBox != nil {
		s.directBox.MarkNeedsPaint()
	}
	if s.replayBox != nil {
		s.replayBox.MarkNeedsPaint()
	}
}
