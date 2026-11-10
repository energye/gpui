// Command ui_wr_p43_measure is the P4.3 measurement real window. It drives a
// *local* backdrop layer (a centered card covering ~16% of the surface) over a
// full-window static striped background, so the P4.3 "backdrop damage region"
// question can be answered with objective JSON counters: damage_area_px,
// frame_flushes, frame_raster_ms, gpu_ops, cpu_fallback_ops, present_count.
//
// It adds NO engine capability and NO public API: it only drives existing
// render.Canvas / ui-rendering public APIs. There is no golden, no
// screen-check and no pixel probe — the verdict is a JSON counter comparison
// against the full-surface backdrop baseline recorded in
// evidence/P4.3/measure/results.md §2. A no-GPU / no-present run fails loudly
// instead of producing a fake-green JSON.
//
// Usage (isolated display, single window, standard close):
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	env -u WAYLAND_DISPLAY DISPLAY=:3 RUN_SECONDS=6 go run ./examples/ui_wr_p43_measure -auto-only
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// backdropRect is a centered 40%x40% card => 16% of the window surface.
// Full-surface backdrop baseline (results.md §2): damage_area_px 960000 = whole
// 1200x800 window, frame_flushes 11. If THIS window still reports a whole-window
// damage / similar flush count, the whole-layer blit cannot be damage-cropped.
var backdropRect = [4]float64{winW * 0.30, winH * 0.30, winW * 0.40, winH * 0.40}

// noBackdrop is the A/B control: same scene, same present policy, but the
// backdrop layer is skipped. Comparing the two runs separates "full-surface
// damage caused by PresentPolicyFullPaint" from "extra cost caused by the
// backdrop whole-layer blit". Without this control the damage counter alone
// cannot attribute anything to backdrop.
var noBackdrop = os.Getenv("GPUI_P43_NO_BACKDROP") == "1"

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }

func paintScene(pc *rendering.PaintContext, w, h float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc := pc.DC

	// Full-window static striped background: no animation => steady frames, so
	// any repaint cost is attributable to the backdrop layer, not a moving scene.
	const bands = 16
	bw := w / bands
	for i := 0; i < bands; i++ {
		if i%2 == 0 {
			dc.SetRGBA(0.12, 0.42, 0.55, 1)
		} else {
			dc.SetRGBA(0.85, 0.78, 0.30, 1)
		}
		dc.DrawRectangle(float64(i)*bw, 0, bw, h)
		_ = dc.Fill()
	}

	// A LOCAL backdrop layer: clip to the centered card, snapshot the device,
	// dim inside, pop. PushBackdropLayer still seeds from the whole device; the
	// measured question is whether Pop blit / present damage can be cropped to
	// this card or stays full-surface. The A/B control skips this whole block.
	bx, by, cw, ch := backdropRect[0], backdropRect[1], backdropRect[2], backdropRect[3]
	if !noBackdrop {
		dc.Push()
		dc.ClipRect(bx, by, cw, ch)
		dc.PushBackdropLayer(render.BlendNormal, 1)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(bx, by, cw, ch)
		_ = dc.Fill()
		dc.PopLayer()
		dc.Pop()
	}

	// Metric line floats over the top-left of the content (HUD-on-content rule).
	dc.Push()
	dc.SetRGBA(0, 0, 0, 0.55)
	dc.DrawRectangle(0, 0, w, 30)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if face := wrkit.FaceAt(13); face != nil {
		dc.SetFont(face)
	}
	dc.DrawString(fmt.Sprintf("P4.3 local backdrop  card=%.0fx%.0f @%.0f,%.0f  (metrics in JSON)", cw, ch, bx, by), 10, 20)
	dc.Pop()
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "scripted run: JSON on stdout, standard close")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		if *autoOnly {
			secs = 6
		} else {
			secs = 0 // persistent manual window
		}
	}
	if _, _, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintln(os.Stderr, "WARN: UI face:", err)
	}

	win, err := platform.Open(platform.Options{
		Width: winW, Height: winH,
		Title: "gpui ui_wr_p43_measure — P4.3 local backdrop damage", Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open:", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = float64(winW), float64(winH)
	root.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		w, h := sz.Width, sz.Height
		if w <= 0 || h <= 0 {
			w, h = float64(winW), float64(winH)
		}
		paintScene(pc, w, h)
	}

	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.05, ClearG: 0.06, ClearB: 0.08, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if (ev.Type == platform.EventClose || ev.Type == platform.EventCloseRequested) && ctl != nil {
				ctl.SetTitle("gpui ui_wr_p43_measure — closing")
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) { app.ScheduleFrame() }})
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
	app.Close()
	win.Close()
	elapsed := time.Since(t0).Seconds()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P4.3-local-backdrop", Scenario: "ui_wr_p43_measure",
		Backend: win.Backend().String(),
		Snap: app.Metrics().Snapshot(), PresentCount: app.PresentCount(), ElapsedSec: elapsed,
		SurfaceAreaPx: int64(winW) * int64(winH), Warmup: true,
		Extra: map[string]any{
			"fps_gate":               "off_measurement",
			"slope_gate":             "off",
			"p43_mode":               "backdrop-local",
			"p43_no_backdrop":        noBackdrop,
			"backdrop_card_px":       fmt.Sprintf("%.0fx%.0f@%.0f,%.0f", backdropRect[2], backdropRect[3], backdropRect[0], backdropRect[1]),
			"backdrop_card_area_px":  int64(backdropRect[2] * backdropRect[3]),
			"apis":                   "PushBackdropLayer/ClipRect/PopLayer (render.Canvas public API)",
			"verdict_source":         "evidence/P4.3/measure/results.md §2 full-surface baseline: damage 960000 / flush 11",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	// Objective guards against a fake-green (no GPU / no presents) run.
	fail := false
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no presents")
		fail = true
	}
	if report.GPUOps == 0 {
		fmt.Fprintln(os.Stderr, "FAIL: gpu_ops=0: window draws did not ride the GPU")
		fail = true
	}
	if *autoOnly && fail {
		os.Exit(1)
	}
}
