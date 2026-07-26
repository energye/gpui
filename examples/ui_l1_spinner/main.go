// Command ui_l1_spinner is the L1 P3 gate: spinner on RepaintBoundary + JSON metrics.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_spinner
//
// Auto window backend: Wayland or X11 (see examples/exhost, GPUI_DISPLAY=wayland|x11|auto).
// Duration: default 60s; override with RUN_SECONDS.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/ui/animation"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(60)
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: running %ds (RUN_SECONDS / GPUI_DISPLAY=wayland|x11|auto); close window to exit safely\n", secs)
	const winW, winH = 480, 320

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui L1 spinner (P3)"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	// GPU must be released before native window destroy.
	defer win.Close()

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
	app.Close()

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
	fmt.Fprintf(os.Stderr, "ui_l1_spinner: avg=%.2fms max=%.2fms last=%.2fms hitches(>%.1fms)=%d\n",
		m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.LastFrameIntervalMs, scheduler.HitchThresholdMs, m.HitchCount)
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
