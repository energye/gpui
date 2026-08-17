// Command ui_l1_spinner is the L1 P3 gate: spinner on RepaintBoundary + JSON metrics.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_spinner
//
// Auto window backend: Wayland or X11 (ui/platform.Open; force via platform.Options.Backend).
// Duration: default 60s; override with RUN_SECONDS.
// P0 closeout: RSS/CPU process samples in JSON (rss_*_kb, cpu_pct_avg).
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(60)
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: running %ds (RUN_SECONDS); close window to exit safely\n", secs)
	const winW, winH = 480, 320

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui L1 spinner (P3)", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	// Closed explicitly after app.Close so RSS-after-close is meaningful.

	spin := rendering.NewRenderSpinner(48)
	root := rendering.NewRenderBox(spin)
	root.FixedWidth, root.FixedHeight = float64(winW), float64(winH)
	for i := 0; i < 20; i++ {
		c := rendering.NewRenderColorBox(12, 12, 0.25, 0.28, 0.32, 1)
		root.AddChild(c)
	}
	spin.SetOffset(rendering.Point{X: float64(winW)/2 - 24, Y: float64(winH)/2 - 24})

	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: 0.10, ClearG: 0.12, ClearB: 0.16, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_l1_spinner: window close (%s) — stopping GPU before destroy\n", win.Backend())
			}
		},
	})

	ctrl := animation.NewController(1.0)
	ctrl.SetRepeat(true)
	ctrl.OnValue(func(v float64) {
		spin.SetPhase(v)
		app.ScheduleFrame()
		proc.Sample()
	})
	ctrl.Start(app.Scheduler().Tickers())

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	ctrl.Stop()
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "no frames")
		os.Exit(1)
	}
	m := app.Metrics().Snapshot()
	fps := float64(app.PresentCount()) / elapsed
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: backend=%s presents=%d ~%.1f fps over %.1fs (cap %ds) layout_flushes=%d raster_layers=%d\n",
		win.Backend(), app.PresentCount(), fps, elapsed, secs, app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount)
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: avg=%.2fms max=%.2fms p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: rss start=%d end=%d peak=%d after_close=%d KB cpu_avg=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)
	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
	}
	if app.LayoutFlushCount() > app.PresentCount()/2 && app.PresentCount() > 10 {
		fmt.Fprintf(os.Stderr, "warning: layout flushes high (%d / %d presents)\n",
			app.LayoutFlushCount(), app.PresentCount())
	}
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
