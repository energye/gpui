// Command ui_render_base_image is the Image-axis window smoke for
// docs/ENGINE_UI_RENDER_BASE.md § FImg-* / §24.
//
// Covers:
//
//	RenderImage state machine (loading → ready / error)
//	ui/io.Pool async decode (not on Layout/Paint)
//	pc.DC DrawImage / DrawImageEx / DrawImageRounded / DrawImageCircular / DrawImageNine
//	SrcRect crop sample
//	paint-only image swap (no layout storm) + RSS metrics
//
// Does NOT claim GIF multi-frame, TextureLayer, or P6 present.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_render_base_image
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(30)
	fmt.Fprintf(os.Stderr, "ui_render_base_image: Image axis (FImg-*) — %ds\n", secs)
	fmt.Fprintln(os.Stderr, "ui_render_base_image: RenderImage + ui/io decode off UI; not P6")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 780, 640
	win, err := exhost.Open(exhost.Options{
		Width:  winW,
		Height: winH,
		Title:  "gpui Image axis (FImg-*)",
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

	var decodeDone atomic.Int64
	var decodeErr atomic.Value // string

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
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

type imgTicker struct {
	on func(dt float64)
}

func (t *imgTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
