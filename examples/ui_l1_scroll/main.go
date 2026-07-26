// Command ui_l1_scroll is the L1 P4 demo: VirtualList 1000 rows + auto-scroll.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_scroll
//
// Auto window backend: Wayland or X11 (GPUI_DISPLAY=wayland|x11|auto).
// Duration: default 60s; override with RUN_SECONDS.
// Window is resizable; list + viewport follow client size.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(60)
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: running %ds (RUN_SECONDS / GPUI_DISPLAY); close window to exit safely\n", secs)
	fmt.Fprintln(os.Stderr, "ui_l1_scroll: window is resizable — list tracks client size")
	const winW, winH = 400, 480
	const itemExtent = 40.0
	const itemCount = 1000

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui L1 scroll (P4) — resize me"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()

	host := win.Host()
	list := rendering.NewVirtualList(itemCount, itemExtent, func(i int) rendering.RenderObject {
		r, g, b := 0.35, 0.38, 0.42
		if i%2 == 0 {
			r, g, b = 0.25, 0.55, 0.85
		}
		return rendering.NewRenderColorBox(0, itemExtent, r, g, b, 1)
	})
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
			max := float64(itemCount)*itemExtent - float64(h)
			if max < 0 {
				max = 0
			}
			if scrollY > max {
				scrollY = 0
			}
			vp.SetScrollOffset(0, scrollY)
			app.ScheduleFrame()
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
	app.Close()

	elapsed := time.Since(t0).Seconds()
	if elapsed < 0.001 {
		elapsed = 0.001
	}
	m := app.Metrics().Snapshot()
	fps := float64(app.PresentCount()) / elapsed
	fw, fh := host.Size()
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: backend=%s presents=%d ~%.1f fps over %.1fs layout_flushes=%d bind=%d scrollY=%.0f size=%dx%d\n",
		win.Backend(), app.PresentCount(), fps, elapsed, app.LayoutFlushCount(), list.BindCount, vp.ScrollOffset().Y, fw, fh)
	fmt.Fprintf(os.Stderr, "ui_l1_scroll: avg=%.2fms max=%.2fms last=%.2fms hitches(>%.1fms)=%d\n",
		m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.LastFrameIntervalMs, scheduler.HitchThresholdMs, m.HitchCount)
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
