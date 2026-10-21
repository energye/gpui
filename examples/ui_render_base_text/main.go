// Command ui_render_base_text is the Text-axis window smoke for
// docs/ENGINE_UI_RENDER_BASE.md §8 (FT-*) / §24.
//
// Covers:
//
//	RenderText Latin/CJK + optional Face
//	SetColor paint-only (no layout storm)
//	MaxWidth wrap (DrawStringWrapped)
//	Direct pc.DC DrawString / DrawStringWrapped
//
// Does NOT claim Paragraph rich text, ellipsis, selection/IME, or P6 present.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_base_text
//	// optional: text.SetDefaultFontPath("/path/to.ttf") or FontResolver.SetFontFile
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
	autoOnly := flag.Bool("auto-only", false, "gate run: freeze animation, snapshot, pixel + Golden gate")
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
		wrkit.RequireMinRun(secs, "BASE-TEXT")
	}
	fmt.Fprintf(os.Stderr, "ui_render_base_text: Text axis (FT-*) — %ds gate=%v\n", secs, gateMode)
	fmt.Fprintln(os.Stderr, "ui_render_base_text: PaintContext + render; no ui/painting; not P6")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 800, 720
	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui Text axis (FT-*)",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}

	// Example configures multi-script itself; library default is a single system UI font.
	// App/tests can also: text.NewFontResolver().SetFontFile("…").Load(16)
	face, fontPath, ferr := text.LoadMultiFace(16)
	if ferr != nil {
		// Fall back to single system default if optional script fonts are missing.
		face, fontPath, ferr = rendering.TryLoadDefaultFace(16)
	}
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "ui_render_base_text: font skipped: %v\n", ferr)
	} else {
		fmt.Fprintf(os.Stderr, "ui_render_base_text: font %s\n", fontPath)
	}

	sc := buildTextScene(float64(winW), float64(winH), face, fontPath)
	sc.frozen = gateMode

	snapPath := ""
	if gateMode {
		snapPath = os.Getenv("SNAP_PNG")
		if snapPath == "" {
			snapPath = filepath.Join("examples", "ui_render_base_text", "testdata", "showcase_text.png")
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
				fmt.Fprintf(os.Stderr, "ui_render_base_text: close (%s)\n", win.Backend())
			}
		},
	})

	app.Scheduler().Tickers().Add(&textTicker{on: func(dt float64) {
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
	fmt.Fprintf(os.Stderr, "ui_render_base_text: backend=%s presents=%d ~%.1f fps layout_flushes=%d raster_layers=%d\n",
		win.Backend(), presents, fps, app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount)
	fmt.Fprintf(os.Stderr, "ui_render_base_text: avg=%.2f p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_base_text: rss start=%d end=%d peak=%d after_close=%d KB cpu=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)

	// Gate: color pulse must not layout-storm (applies to manual and gate modes).
	if app.LayoutFlushCount() > presents/2 && presents > 10 {
		fmt.Fprintf(os.Stderr, "FAIL: layout flushes high (%d / %d) — SetColor should be paint-only\n",
			app.LayoutFlushCount(), presents)
		os.Exit(1)
	}
	if !gateMode {
		if b, err := app.Metrics().JSON(); err == nil {
			fmt.Println(string(b))
		}
		return
	}

	// Three evidence: logic probes + GPU-snapshot pixel probes + bitwise Golden.
	img := wrevidence.Load(snapPath)
	if img == nil {
		os.Exit(1)
	}
	checks := []wrevidence.PixelCheck{
		{Desc: "root_bg", X: 790, Y: 710, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "card_bg", X: 20, Y: 80, Want: [3]float64{0.14, 0.16, 0.20}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "dc_panel_bg", X: 20, Y: 545, Want: [3]float64{0.12, 0.14, 0.18}, Tol: 0.05, Kind: wrevidence.KindBG},
		{Desc: "pulse_text_ink", X: 80, Y: 542, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.04, Kind: wrevidence.KindNotBG},
		{Desc: "title_ink", X: 112, Y: 5, Want: [3]float64{0.09, 0.10, 0.12}, Tol: 0.04, Kind: wrevidence.KindNotBG},
	}
	pixelResult := wrevidence.CheckPixels(img, checks, "ui_render_base_text")
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := wrevidence.Golden(
		snapPath, filepath.Join("examples", "ui_render_base_text", "testdata", "showcase_text_base.png"),
		[]wrevidence.Rect{{X: 0, Y: 0, W: winW, H: winH}}, "ui_render_base_text")

	logic := map[string]bool{
		"presents_positive": presents >= 1,
		"paint_positive":    m.PaintCount > 0,
		"no_layout_storm":   app.LayoutFlushCount() <= presents/2,
		"gpu_ops_positive":  m.GPUOps > 0,
		"fallback_zero":     m.CPUFallbackOps == 0,
	}
	logicPass := 0
	for _, ok := range logic {
		if ok {
			logicPass++
		}
	}
	logicOK := logicPass == len(logic)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "BASE-TEXT", Scenario: "ui_render_base_text",
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
		MinPresents: 1, RequirePersistentFPS: true, MaxP95Ms: 22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if b, err := wrgate.Marshal(report); err == nil {
		fmt.Println(string(b))
	}
	if !logicOK {
		fmt.Fprintf(os.Stderr, "FAIL: logic probes %d/%d (layout_flushes=%d presents=%d)\n", logicPass, len(logic), app.LayoutFlushCount(), presents)
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
	fmt.Fprintf(os.Stderr, "ui_render_base_text: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% elapsed=%.1fs\n",
		presents, logicPass, len(logic), pixelOK, len(checks), goldenDiffPct, elapsed)
}

type textTicker struct {
	on func(dt float64)
}

func (t *textTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
