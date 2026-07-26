// Command ui_l1_blank is the L1 P0 gate: window + clear present via ui embedder.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	go run ./examples/ui_l1_blank
//
// Auto window backend: Wayland or X11 (GPUI_DISPLAY=wayland|x11|auto).
// Duration: default 60s; override with RUN_SECONDS.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(60)
	fmt.Fprintf(os.Stderr, "ui_l1_blank: running %ds (RUN_SECONDS / GPUI_DISPLAY=wayland|x11|auto); close window to exit safely\n", secs)
	const winW, winH = 480, 320

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui L1 blank (P0)"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()

	app := embedder.New(win.Host(), embedder.Options{
		ClearR:          0.10,
		ClearG:          0.45,
		ClearB:          0.75,
		ClearA:          1,
		ContinuousClear: true,
		RunFor:          time.Duration(secs) * time.Second,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_l1_blank: window close (%s) — stopping GPU before destroy\n", win.Backend())
			}
		},
	})
	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open present:", err)
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
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "no presents completed")
		os.Exit(1)
	}
	m := app.Metrics().Snapshot()
	printFrameSummary("ui_l1_blank", win.Backend().String(), app.PresentCount(), elapsed, secs, m)
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

func printFrameSummary(name, backend string, presents int64, elapsed float64, capSecs int, m scheduler.FrameMetrics) {
	fps := float64(presents) / elapsed
	fmt.Fprintf(os.Stderr, "%s: backend=%s presents=%d ~%.1f fps (wall %.1fs, cap %ds) avg=%.2fms max=%.2fms last=%.2fms hitches(>%.1fms)=%d\n",
		name, backend, presents, fps, elapsed, capSecs, m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.LastFrameIntervalMs,
		scheduler.HitchThresholdMs, m.HitchCount)
}
