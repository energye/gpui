// Command ui_render_base_image is the Image-axis window smoke for
// docs/ENGINE_UI_RENDER_BASE.md § FImg-* / §24.
//
// Covers:
//
//	RenderImage state machine (loading → ready / error)
//	ui/io.Pool async decode (not on Layout/Paint)
//	UI DrawImageBuf / Rect / Rounded / Circular / Nine / Atlas
//	paint-only image swap (no layout storm) + RSS metrics
//
// Does NOT claim GIF multi-frame, TextureLayer, or P6 present.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_base_image
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
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
	autoOnly := flag.Bool("auto-only", false, "gate run: freeze pulse, snapshot, pixel + Golden gate")
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
		wrkit.RequireMinRun(secs, "BASE-IMAGE")
	}
	fmt.Fprintf(os.Stderr, "ui_render_base_image: Image axis (FImg-*) — %ds gate=%v\n", secs, gateMode)
	fmt.Fprintln(os.Stderr, "ui_render_base_image: RenderImage + ui/io decode off UI; not P6")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 780, 640
	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui Image axis (FImg-*)",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}

	face, fontPath, ferr := rendering.TryLoadDefaultFace(12)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "ui_render_base_image: font skipped: %v\n", ferr)
	} else {
		fmt.Fprintf(os.Stderr, "ui_render_base_image: font %s\n", fontPath)
	}

	sc, serr := buildImageScene(float64(winW), float64(winH), face)
	if serr != nil {
		fmt.Fprintln(os.Stderr, "scene:", serr)
		os.Exit(1)
	}
	sc.frozen = gateMode

	snapPath := ""
	if gateMode {
		snapPath = os.Getenv("SNAP_PNG")
		if snapPath == "" {
			snapPath = filepath.Join("examples", "ui_render_base_image", "testdata", "showcase_image.png")
		}
		if err := os.MkdirAll(filepath.Dir(snapPath), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
			os.Exit(1)
		}
	}

	var decodeDone atomic.Int64
	var decodeErr atomic.Value // string

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_render_base_image: close (%s)\n", win.Backend())
			}
		},
	})

	// Kick async decode after open path is ready — still before Run is OK;
	// results land via channel + ticker hop onto UI thread.
	sc.startAsyncDecode(func(msg string) {
		decodeDone.Add(1)
		if msg != "" {
			decodeErr.Store(msg)
		}
		app.ScheduleFrame()
	})

	app.Scheduler().Tickers().Add(&imgTicker{on: func(dt float64) {
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
	sc.cleanup()
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
	fmt.Fprintf(os.Stderr, "ui_render_base_image: backend=%s presents=%d ~%.1f fps layout_flushes=%d raster_layers=%d\n",
		win.Backend(), presents, fps, app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount)
	fmt.Fprintf(os.Stderr, "ui_render_base_image: avg=%.2f p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_base_image: rss start=%d end=%d peak=%d after_close=%d KB cpu=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)
	fmt.Fprintf(os.Stderr, "ui_render_base_image: async_decode_callbacks=%d swaps=%d\n",
		decodeDone.Load(), sc.swapCount)

	if v := decodeErr.Load(); v != nil {
		if s, ok := v.(string); ok && s != "" {
			fmt.Fprintf(os.Stderr, "ui_render_base_image: decode note: %s\n", s)
		}
	}

	// Gate: image state / SetImage must stay paint-only (no layout storm).
	if app.LayoutFlushCount() > presents/2 && presents > 10 {
		fmt.Fprintf(os.Stderr, "FAIL: layout flushes high (%d / %d) — image updates should be paint-only\n",
			app.LayoutFlushCount(), presents)
		os.Exit(1)
	}
	// Gate: at least one async path should complete in a multi-second run.
	if secs >= 2 && decodeDone.Load() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no async decode callback — ui/io path broken?")
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
		{Desc: "root_bg", X: 770, Y: 590, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "sync_checker_blue", X: 18, Y: 74, Want: [3]float64{0.157, 0.471, 0.863}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "sync_checker_yellow", X: 30, Y: 74, Want: [3]float64{0.941, 0.784, 0.235}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "async_gradient", X: 136, Y: 74, Want: [3]float64{0.153, 0.188, 0.365}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "dc_panel_bg", X: 20, Y: 230, Want: [3]float64{0.12, 0.14, 0.18}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "dc_image_blue", X: 34, Y: 246, Want: [3]float64{0.157, 0.471, 0.863}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "pulse_bar", X: 32, Y: 391, Want: [3]float64{0.30, 0.70, 0.90}, Tol: 0.08, Kind: wrevidence.KindBG},
		{Desc: "title_ink", X: 155, Y: 5, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.04, Kind: wrevidence.KindNotBG},
	}
	pixelResult := wrevidence.CheckPixels(img, checks, "ui_render_base_image")
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := wrevidence.Golden(
		snapPath, filepath.Join("examples", "ui_render_base_image", "testdata", "showcase_image_base.png"),
		[]wrevidence.Rect{{X: 0, Y: 0, W: winW, H: winH}}, "ui_render_base_image")

	logic := map[string]bool{
		"presents_positive":  presents >= 1,
		"paint_positive":     m.PaintCount > 0,
		"no_layout_storm":    app.LayoutFlushCount() <= presents/2,
		"decode_done":        decodeDone.Load() >= 1,
		"swap_count":         sc.swapCount >= 1,
		"async_status_ready": sc.lastApplied == "ready",
		"gpu_ops_positive":   m.GPUOps > 0,
		"fallback_zero":      m.CPUFallbackOps == 0,
	}
	logicPass := 0
	for _, ok := range logic {
		if ok {
			logicPass++
		}
	}
	logicOK := logicPass == len(logic)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "BASE-IMAGE", Scenario: "ui_render_base_image",
		Backend: win.Backend().String(), Snap: m, PresentCount: presents,
		ElapsedSec: elapsed, SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"async_decode_callbacks": decodeDone.Load(),
			"swaps":                  sc.swapCount,
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
		fmt.Fprintf(os.Stderr, "FAIL: logic probes %d/%d (decode=%d swaps=%d status=%q layout_flushes=%d)\n",
			logicPass, len(logic), decodeDone.Load(), sc.swapCount, sc.lastApplied, app.LayoutFlushCount())
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
	fmt.Fprintf(os.Stderr, "ui_render_base_image: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% elapsed=%.1fs\n",
		presents, logicPass, len(logic), pixelOK, len(checks), goldenDiffPct, elapsed)
}

type imgTicker struct {
	on func(dt float64)
}

func (t *imgTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
