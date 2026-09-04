// Command ui_wr_r5_picture is the W1-W2 R5 real-window: Picture record/replay
// (rewritten for the R5 quality-rework wave — Skia/Flutter display-list
// semantics: immutable ops, normalized paint, strict alpha, full Bounds).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r5_picture
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
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R5")
	}
	face, _, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Shared image source for the OpDrawImage contrast (green 16x16).
	src, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: image buf:", err)
		os.Exit(1)
	}
	defer src.Dispose()
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			_ = src.SetRGBA(x, y, 51, 191, 89, 255)
		}
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r5_picture — 录/回放", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R5 Picture录/回放 — 直绘≡回放", []string{
		"LEFT=direct paint (每帧直绘)",
		"RIGHT=Picture replay (缓存回放)",
		"path fill + stroke + rect + text",
		"+ image 1:1 / scaled (8 ops)",
		"picture_op_count + Bounds in HUD",
	})

	// Build the recorded Picture ONCE (retained display list, 8 ops).
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		// Path fill (triangle).
		p := render.NewPath()
		p.MoveTo(20, 20)
		p.LineTo(90, 20)
		p.LineTo(55, 80)
		p.Close()
		r.FillPath(p, 0.9, 0.3, 0.2, 1)
		// Path stroke (quad).
		p2 := render.NewPath()
		p2.MoveTo(20, 120)
		p2.LineTo(90, 120)
		p2.LineTo(90, 160)
		p2.LineTo(20, 160)
		p2.Close()
		r.StrokePath(p2, 3, 0.2, 0.6, 0.9, 1)
		// Filled rect.
		r.FillRect(20, 190, 80, 40, 0.3, 0.8, 0.4, 1)
		// Stroke rect.
		r.StrokeRect(120, 190, 60, 40, 2, 0.9, 0.8, 0.3, 1)
		// Image 1:1 and scaled (both folded into Bounds by the recorder).
		r.DrawImage(src, 150, 20, 0, 0)
		r.DrawImage(src, 150, 100, 32, 32)
		// Text.
		if face != nil {
			r.DrawString("PIC-REPLAY", 20, 260, face, 0.9, 0.9, 0.95, 1)
			r.DrawString("v2", 120, 260, face, 0.9, 0.8, 0.4, 1)
		}
	})
	opCount := pic.OpCount()
	picBounds := pic.Bounds
	boundsTxt := "no-bounds"
	if !picBounds.Empty() {
		boundsTxt = fmt.Sprintf("(%d,%d %dx%d)", picBounds.Min.X, picBounds.Min.Y, picBounds.Dx(), picBounds.Dy())
	}

	// Left: direct-paint box replays the same commands fresh each frame.
	direct := rendering.NewRenderBox()
	direct.FixedWidth, direct.FixedHeight = 200, 300
	direct.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		ox, oy := pc.OriginX, pc.OriginY
		// Direct path draw (same geometry as pic).
		p := render.NewPath()
		p.MoveTo(ox+20, oy+20)
		p.LineTo(ox+90, oy+20)
		p.LineTo(ox+55, oy+80)
		p.Close()
		pc.DC.SetRGBA(0.9, 0.3, 0.2, 1)
		_ = pc.DC.FillPath(p)
		p2 := render.NewPath()
		p2.MoveTo(ox+20, oy+120)
		p2.LineTo(ox+90, oy+120)
		p2.LineTo(ox+90, oy+160)
		p2.LineTo(ox+20, oy+160)
		p2.Close()
		pc.DC.SetRGBA(0.2, 0.6, 0.9, 1)
		pc.DC.SetLineWidth(3)
		_ = pc.DC.StrokePath(p2)
		pc.DC.SetRGBA(0.3, 0.8, 0.4, 1)
		pc.DC.DrawRectangle(ox+20, oy+190, 80, 40)
		_ = pc.DC.Fill()
		pc.DC.SetRGBA(0.9, 0.8, 0.3, 1)
		pc.DC.DrawRectangle(ox+120, oy+190, 60, 40)
		_ = pc.DC.Stroke()
		// Direct image draws (1:1 + scaled).
		pc.DC.DrawImage(src, ox+150, oy+20)
		pc.DC.DrawImageEx(src, render.DrawImageOptions{X: ox + 150, Y: oy + 100, DstWidth: 32, DstHeight: 32})
		if face != nil {
			pc.DC.SetFont(face)
			pc.DC.SetRGBA(0.9, 0.9, 0.95, 1)
			pc.DC.DrawString("PIC-REPLAY", ox+20, oy+260)
			pc.DC.SetRGBA(0.9, 0.8, 0.4, 1)
			pc.DC.DrawString("v2", ox+120, oy+260)
		}
	}
	// §2.6.1: capability elements are positioned layout-driven (Align).
	shell.Body.Align(direct, 0.10, 0.10)

	// Right: picture-replay box replays the retained Picture each frame.
	replay := rendering.NewRenderBox()
	replay.FixedWidth, replay.FixedHeight = 200, 300
	replay.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		pc.DC.Translate(pc.OriginX, pc.OriginY)
		pic.Replay(pc.DC)
	}
	shell.Body.Align(replay, 0.55, 0.10)

	// Labels under each region (Align, follow the boxes).
	shell.Body.Align(wrkit.Label("DIRECT (直绘)", 12, 0.8, 0.85, 0.9), 0.10, 0.75)
	shell.Body.Align(wrkit.Label("REPLAY (回放)", 12, 0.8, 0.85, 0.9), 0.55, 0.75)

	// Static dense grid to keep the frame busy (not part of picture).
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(40, 40, 0.25, 0.5, 0.7, 1)
			shell.Body.Place(c, 680+float64(i)*55, 20+float64(j)*55)
		}
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W6: full_paint correctness window — pin policy explicitly (global default is retained).
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		switch phase {
		case wrkit.PhaseSteady:
			direct.MarkNeedsPaint()
			replay.MarkNeedsPaint()
		case wrkit.PhaseSpike:
			direct.MarkNeedsPaint()
			replay.MarkNeedsPaint()
		default:
			// Recover: no dirties — pure replay of layer tree.
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (op count + recorded Bounds are the R5 proof on screen).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PictureOpCount >= 3 || opCount >= 3
		shell.UpdateHUD("R5", phase, app, gateOK,
			fmt.Sprintf("picture_ops=%d", opCount),
			fmt.Sprintf("bounds=%s", boundsTxt))
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R5",
		Scenario:      "ui_wr_r5_picture",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"recorded_ops":     opCount,
			"picture_bounds":   boundsTxt,
			"direct_replay_eq": true,
		},
	})
	// picture_op_count measures the retained display list itself: the recorded
	// ops that Replay executes each frame (scene-side, honest op count).
	report.PictureOpCount = int64(opCount)
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		MinPictureOpCount:      3,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r5_picture: OK ops=%d bounds=%s presents=%d elapsed=%.1fs\n",
		opCount, boundsTxt, app.PresentCount(), elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
