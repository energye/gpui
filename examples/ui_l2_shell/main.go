// Command ui_l2_shell smoke-tests P5 L2 packages: Tap/Pan, Focus, Overlay.
//
// Demo scene lives in this example only (scene.go) — not under ui/.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	CGO_ENABLED=0 go run ./examples/ui_l2_shell
//
// Controls:
//   - Green box: tap (flashes brighter)
//   - Slate box: drag the blue handle (pan) — slides, does not rotate
//   - Yellow focus boxes: click or Tab to focus
//   - Blue button / key O: toggle overlay; click dimmer to close
//
// Duration: default 120s; RUN_SECONDS overrides. Close window to exit safely.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	secs := runSeconds(120)
	const winW, winH = 480, 360
	fmt.Fprintf(os.Stderr, "ui_l2_shell: %ds — tap / pan / focus / overlay (example)\n", secs)
	fmt.Fprintln(os.Stderr, "  green=tap  slate+blue handle=drag pan  grey/yellow=focus  blue btn / O=overlay")

	win, err := platform.Open(platform.Options{
		Width: winW, Height: winH,
		Title:       "gpui L2 demo — tap · pan · focus · overlay",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()

	scene := NewScene(float64(winW), float64(winH))
	scene.Layout()

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(win.Host(), scene.Root, embedder.PipelineOptions{
		ClearR:  0.10,
		ClearG:  0.12,
		ClearB:  0.16,
		ClearA:  1,
		RunFor:  time.Duration(secs) * time.Second,
		WarmUp:  true,
		Overlay: scene.Overlay,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_l2_shell: close (%s)\n", win.Backend())
				return
			}
			if ev.Type == platform.EventWake || ev.Type == platform.EventExpose {
				return
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				scene.Width, scene.Height = float64(ev.Width), float64(ev.Height)
				scene.Layout()
				scene.Root.MarkNeedsPaint()
				app.ScheduleFrame()
				return
			}
			if ev.Type == platform.EventPointer || ev.Type == platform.EventKey {
				scene.HandleEvent(ev)
			}
		},
	})
	scene.OnDirty = func() {
		if scene.Root != nil {
			scene.Root.MarkNeedsPaint()
		}
		app.ScheduleFrame()
	}

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}

	scene.Layout()
	scene.Root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	app.Close()

	elapsed := time.Since(t0).Seconds()
	m := scene.SnapshotMetrics()
	fmt.Fprintf(os.Stderr, "ui_l2_shell: backend=%s elapsed=%.1fs presents=%d %s\n",
		win.Backend(), elapsed, app.PresentCount(), scene.String())
	if b, err := json.Marshal(m); err == nil {
		fmt.Println(string(b))
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "no presents")
		os.Exit(1)
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
