// Command ui_render_base_geometry is the Geometry-axis window smoke for
// docs/ENGINE_UI_RENDER_BASE.md §2 (FC-DRAW-*) / §24.
//
// Covers P0 ui/painting surface:
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
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/painting"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(30)
	fmt.Fprintf(os.Stderr, "ui_render_base_geometry: Geometry axis (FC-DRAW-*) — %ds\n", secs)
	fmt.Fprintln(os.Stderr, "ui_render_base_geometry: P0 painting only; Path/Arc ui = B; not P6 present")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 660, 520
	win, err := exhost.Open(exhost.Options{
		Width:  winW,
		Height: winH,
		Title:  "gpui Geometry axis (P0 FC-DRAW)",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}

	sc := buildGeometryScene(float64(winW), float64(winH))
	if face, path, err := painting.TryLoadDefaultFace(12); err != nil {
		fmt.Fprintf(os.Stderr, "ui_render_base_geometry: font skipped: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_render_base_geometry: font %s\n", path)
		// Labels are RenderText children — walk is heavy; set on title-ish by rebuilding
		// is unnecessary: DrawTextColored uses DC font if set during paint of panels.
		_ = face
	}

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
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
	if b, err := app.Metrics().JSON(); err == nil {
		fmt.Println(string(b))
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

type geoTicker struct {
	on func(dt float64)
}

func (t *geoTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
