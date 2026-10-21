// Command ui_render_base_geometry is the Geometry-axis window smoke for
// docs/ENGINE_UI_RENDER_BASE.md §2 (FC-DRAW-*) / §24.
//
// Covers P0 render via PaintContext surface:
//
//	FillRect, FillRoundRect, FillLinearGradient,
//	StrokeRect, StrokeRoundRect, StrokeLine,
//	FillCircle, StrokeCircle, PushClipRoundRect
//
// Does NOT claim Path/Arc/Oval ui APIs (still B) or P6 dirty-rect present.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_base_geometry
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrevidence"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate run: freeze pulses, snapshot, pixel + Golden gate")
	manualSeconds := flag.Int("manual-seconds", 0, "manual run seconds (0 = 30s; manual runs do not snapshot)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	gateMode := *autoOnly
	if *manualSeconds > 0 {
		secs = *manualSeconds
		gateMode = false
	} else if gateMode {
		if !secsSet {
			secs = 5
		}
	} else if !secsSet {
		secs = 30
	}
	if gateMode {
		wrkit.RequireMinRun(secs, "BASE-GEOMETRY")
	}
	fmt.Fprintf(os.Stderr, "ui_render_base_geometry: Geometry axis (FC-DRAW-*) — %ds gate=%v\n", secs, gateMode)
	fmt.Fprintln(os.Stderr, "ui_render_base_geometry: P0+P1 draw via render (radial/path/oval/arc); not P6 present")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 660, 620
	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui Geometry axis (P0 FC-DRAW)",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}

	sc := buildGeometryScene(float64(winW), float64(winH))
	sc.frozen = gateMode
	if face, path, err := rendering.TryLoadDefaultFace(12); err != nil {
		fmt.Fprintf(os.Stderr, "ui_render_base_geometry: font skipped: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_render_base_geometry: font %s\n", path)
		// Labels are RenderText children — walk is heavy; set on title-ish by rebuilding
		// is unnecessary: DrawTextColored uses DC font if set during paint of panels.
		_ = face
	}

	snapPath := ""
	if gateMode {
		snapPath = os.Getenv("SNAP_PNG")
		if snapPath == "" {
			snapPath = filepath.Join("examples", "ui_render_base_geometry", "testdata", "showcase_geometry.png")
		}
		if err := os.MkdirAll(filepath.Dir(snapPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
			os.Exit(1)
		}
	}

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_render_base_geometry: close (%s)\n", win.Backend())
			}
		},
	})

	app.Scheduler().Tickers().Add(&geoTicker{on: func(dt float64) {
		sc.onTick(dt, app.ScheduleFrame)
		proc.Sample()
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	presents := app.PresentCount()
	if presents < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no frames")
		os.Exit(1)
	}
	m := app.Metrics().Snapshot()
	fps := float64(presents) / elapsed
	fmt.Fprintf(os.Stderr, "ui_render_base_geometry: backend=%s presents=%d ~%.1f fps layout_flushes=%d raster_layers=%d\n",
		win.Backend(), presents, fps, app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount)
	fmt.Fprintf(os.Stderr, "ui_render_base_geometry: avg=%.2f p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_base_geometry: rss start=%d end=%d peak=%d after_close=%d KB cpu=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)
	if app.LayoutFlushCount() > presents/2 && presents > 10 {
		fmt.Fprintf(os.Stderr, "warning: layout flushes high (%d / %d)\n", app.LayoutFlushCount(), presents)
	}
	if !gateMode {
		if b, err := app.Metrics().JSON(); err == nil {
			fmt.Println(string(b))
		}
		return
	}

	img := wrevidence.Load(snapPath)
	if img == nil {
		os.Exit(1)
	}
	checks := []wrevidence.PixelCheck{
		{Desc: "root_bg", X: 640, Y: 610, Want: [3]float64{0.08, 0.09, 0.11}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "panel_bg", X: 20, Y: 75, Want: [3]float64{0.16, 0.18, 0.22}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "fill_rect_blue", X: 52, Y: 110, Want: [3]float64{0.25, 0.55, 0.90}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "fill_rrect_green", X: 264, Y: 110, Want: [3]float64{0.35, 0.75, 0.45}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "stroke_panel_bg", X: 20, Y: 250, Want: [3]float64{0.14, 0.15, 0.18}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "circle_pulse_pink", X: 112, Y: 458, Want: [3]float64{0.898, 0.349, 0.549}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "clip_pulse_orange", X: 476, Y: 426, Want: [3]float64{0.95, 0.55, 0.20}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "radial_outer", X: 112, Y: 610, Want: [3]float64{0.875, 0.745, 0.345}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "top_bg", X: 300, Y: 20, Want: [3]float64{0.08, 0.09, 0.11}, Tol: 0.05, Kind: wrevidence.KindBG},
	}
	pixelResult := wrevidence.CheckPixels(img, checks, "ui_render_base_geometry")
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := wrevidence.Golden(
		snapPath, filepath.Join("examples", "ui_render_base_geometry", "testdata", "showcase_geometry_base.png"),
		[]wrevidence.Rect{{X: 0, Y: 0, W: winW, H: winH}}, "ui_render_base_geometry")

	logic := map[string]bool{
		"presents_positive": presents >= 1,
		"paint_positive":    m.PaintCount > 0,
		"no_layout_storm":   app.LayoutFlushCount() <= presents/2,
		"gpu_ops_positive":  m.GPUOps > 0,
		"fallback_zero":     m.CPUFallbackOps == 0,
		"boundary_panels":   m.BoundaryCount >= 3,
	}
	logicPass := 0
	for _, ok := range logic {
		if ok {
			logicPass++
		}
	}
	logicOK := logicPass == len(logic)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "BASE-GEOMETRY", Scenario: "ui_render_base_geometry",
		Backend: win.Backend().String(), Snap: m, PresentCount: presents,
		ElapsedSec: elapsed, SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"logic_probes_total":     len(logic),
			"logic_probes_ok":        logicPass,
			"pixel_probes_total":     len(checks),
			"pixel_probes_ok":        pixelOK,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
		},
	})
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents: 1, RequirePersistentFPS: true,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if b, err := wrgate.Marshal(report); err == nil {
		fmt.Println(string(b))
	}
	if !logicOK {
		fmt.Fprintf(os.Stderr, "FAIL: logic probes %d/%d (presents=%d paint=%d boundaries=%d)\n",
			logicPass, len(logic), presents, m.PaintCount, m.BoundaryCount)
		os.Exit(1)
	}
	if pixelOK != len(checks) {
		fmt.Fprintf(os.Stderr, "FAIL: pixel probes %d/%d\n", pixelOK, len(checks))
		os.Exit(1)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: golden diff %.4f%% over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_render_base_geometry: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% elapsed=%.1fs\n",
		presents, logicPass, len(logic), pixelOK, len(checks), goldenDiffPct, elapsed)
}

type geoTicker struct {
	on func(dt float64)
}

func (t *geoTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
