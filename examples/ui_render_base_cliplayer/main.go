// Command ui_render_base_cliplayer is the ClipLayer-axis window smoke for
// docs/ENGINE_UI_RENDER_BASE.md §24 (Clip / Opacity / Boundary / Transform).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_base_cliplayer
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
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate run: freeze clip/opacity/transform, snapshot, pixel + Golden gate")
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
		wrkit.RequireMinRun(secs, "BASE-CLIPLAYER")
	}
	fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: ClipLayer axis — %ds gate=%v\n", secs, gateMode)
	fmt.Fprintln(os.Stderr, "ui_render_base_cliplayer: Boundary+Clip+Opacity+Transform; not P6")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 720, 560
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ClipLayer axis", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}

	var face text.Face
	if f, path, e := rendering.TryLoadDefaultFace(12); e == nil {
		face = f
		fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: font %s\n", path)
	}

	sc := buildClipLayerScene(float64(winW), float64(winH), face)
	sc.frozen = gateMode

	snapPath := ""
	if gateMode {
		snapPath = os.Getenv("SNAP_PNG")
		if snapPath == "" {
			snapPath = filepath.Join("examples", "ui_render_base_cliplayer", "testdata", "showcase_cliplayer.png")
		}
		if err := os.MkdirAll(filepath.Dir(snapPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
			os.Exit(1)
		}
	}

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: close (%s)\n", win.Backend())
			}
		},
	})
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
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
	fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: backend=%s presents=%d ~%.1f fps layout_flushes=%d raster_layers=%d\n",
		win.Backend(), presents, fps, app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount)
	fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: avg=%.2f p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: rss start=%d end=%d peak=%d after_close=%d KB cpu=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)

	if app.LayoutFlushCount() > presents/2 && presents > 10 {
		fmt.Fprintf(os.Stderr, "FAIL: layout storm (%d / %d)\n", app.LayoutFlushCount(), presents)
		os.Exit(1)
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
		{Desc: "root_bg", X: 700, Y: 510, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "static_card_0", X: 42, Y: 82, Want: [3]float64{0.20, 0.35, 0.55}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "static_card_1", X: 154, Y: 82, Want: [3]float64{0.28, 0.35, 0.55}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "hot_panel", X: 420, Y: 102, Want: [3]float64{0.30, 0.55, 1.00}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "clip_stripe", X: 60, Y: 240, Want: [3]float64{0.25, 0.45, 0.70}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "clip_rrect_child", X: 72, Y: 406, Want: [3]float64{0.85, 0.35, 0.55}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "opacity_blend", X: 410, Y: 280, Want: [3]float64{0.315, 0.205, 0.2725}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "transform_child", X: 570, Y: 270, Want: [3]float64{0.30, 0.75, 0.95}, Tol: 0.06, Kind: wrevidence.KindBG},
		{Desc: "title_ink", X: 15, Y: 10, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.04, Kind: wrevidence.KindNotBG},
	}
	pixelResult := wrevidence.CheckPixels(img, checks, "ui_render_base_cliplayer")
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := wrevidence.Golden(
		snapPath, filepath.Join("examples", "ui_render_base_cliplayer", "testdata", "showcase_cliplayer_base.png"),
		[]wrevidence.Rect{{X: 0, Y: 0, W: winW, H: winH}}, "ui_render_base_cliplayer")

	logic := map[string]bool{
		"presents_positive": presents >= 1,
		"paint_positive":    m.PaintCount > 0,
		"no_layout_storm":   app.LayoutFlushCount() <= presents/2,
		"gpu_ops_positive":  m.GPUOps > 0,
		"fallback_zero":     m.CPUFallbackOps == 0,
		"boundary_count":    m.BoundaryCount >= 8,
	}
	logicPass := 0
	for _, ok := range logic {
		if ok {
			logicPass++
		}
	}
	logicOK := logicPass == len(logic)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "BASE-CLIPLAYER", Scenario: "ui_render_base_cliplayer",
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
	fmt.Fprintf(os.Stderr, "ui_render_base_cliplayer: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% elapsed=%.1fs\n",
		presents, logicPass, len(logic), pixelOK, len(checks), goldenDiffPct, elapsed)
}

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
