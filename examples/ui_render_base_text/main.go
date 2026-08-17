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
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(30)
	fmt.Fprintf(os.Stderr, "ui_render_base_text: Text axis (FT-*) — %ds\n", secs)
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

	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
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

	// Gate: color pulse must not layout-storm
	if app.LayoutFlushCount() > presents/2 && presents > 10 {
		fmt.Fprintf(os.Stderr, "FAIL: layout flushes high (%d / %d) — SetColor should be paint-only\n",
			app.LayoutFlushCount(), presents)
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

type textTicker struct {
	on func(dt float64)
}

func (t *textTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
