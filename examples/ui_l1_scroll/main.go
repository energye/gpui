// Command ui_l1_scroll is the L1 P4 demo: VirtualList 1000 rows + auto-scroll.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_scroll
//
// Auto window backend: Wayland or X11 (auto; force via platform.Options.Backend).
// Duration: default 60s; override with RUN_SECONDS.
// GPUI_VAR_EXTENT=1 enables mixed row heights (FScroll-VAR-EXTENT MVP).
// Window is resizable; list + viewport follow client size.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(60)
	varExtent := os.Getenv("GPUI_VAR_EXTENT") == "1" || os.Getenv("GPUI_VAR_EXTENT") == "true"
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: running %ds (RUN_SECONDS); close window to exit safely\n", secs)
	fmt.Fprintln(os.Stderr, "ui_l1_scroll: window is resizable — list tracks client size")
	if varExtent {
		fmt.Fprintln(os.Stderr, "ui_l1_scroll: GPUI_VAR_EXTENT=1 — variable row heights")
	}
	const winW, winH = 400, 480
	const itemExtent = 40.0
	const itemCount = 1000

	var proc scheduler.ProcessTracker
	proc.Start()

	title := "gpui L1 scroll (P4) — resize me"
	if varExtent {
		title = "gpui L1 scroll — variable extent"
	}
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: title, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	// Explicit close after app for after_close RSS.

	host := win.Host()
	extentAt := func(i int) float64 {
		switch i % 3 {
		case 0:
			return 28
		case 1:
			return 40
		default:
			return 56
		}
	}
	var list *rendering.VirtualList
	if varExtent {
		list = rendering.NewVariableVirtualList(itemCount, itemExtent, extentAt, func(i int) rendering.RenderObject {
			h := extentAt(i)
			r, g, b := 0.35, 0.38, 0.42
			if i%2 == 0 {
				r, g, b = 0.25, 0.55, 0.85
			}
			if i%3 == 2 {
				r, g, b = 0.55, 0.35, 0.75
			}
			return rendering.NewRenderColorBox(0, h, r, g, b, 1)
		})
	} else {
		list = rendering.NewVirtualList(itemCount, itemExtent, func(i int) rendering.RenderObject {
			r, g, b := 0.35, 0.38, 0.42
			if i%2 == 0 {
				r, g, b = 0.25, 0.55, 0.85
			}
			return rendering.NewRenderColorBox(0, itemExtent, r, g, b, 1)
		})
	}
	list.CacheExtent = itemExtent * 2
	vp := rendering.NewRenderViewport(list)

	app := embedder.NewPipelineApp(host, vp, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventResize {
				fmt.Fprintf(os.Stderr, "ui_l1_scroll: resize %dx%d\n", ev.Width, ev.Height)
			}
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_l1_scroll: window close (%s) — stopping GPU before destroy\n", win.Backend())
			}
		},
	})

	var scrollY float64
	app.Scheduler().Tickers().Add(&scrollTicker{
		on: func(dt float64) {
			scrollY += 80 * dt
			_, h := host.Size()
			max := list.ContentHeight() - float64(h)
			if max < 0 {
				max = 0
			}
			if scrollY > max {
				scrollY = 0
			}
			vp.SetScrollOffset(0, scrollY)
			app.ScheduleFrame()
			proc.Sample()
		},
	})
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
	m := app.Metrics().Snapshot()
	fps := float64(app.PresentCount()) / elapsed
	fw, fh := host.Size()
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: backend=%s presents=%d ~%.1f fps over %.1fs layout_flushes=%d bind=%d scrollY=%.0f size=%dx%d\n",
		win.Backend(), app.PresentCount(), fps, elapsed, app.LayoutFlushCount(), list.BindCount, vp.ScrollOffset().Y, fw, fh)
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: avg=%.2fms max=%.2fms p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: rss start=%d end=%d peak=%d after_close=%d KB cpu_avg=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)
	if list.BindCount >= itemCount {
		fmt.Fprintln(os.Stderr, "FAIL: virtualization mounted all rows")
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

type scrollTicker struct {
	on func(dt float64)
}

func (s *scrollTicker) Tick(dt float64) bool {
	if s.on != nil {
		s.on(dt)
	}
	return true
}
