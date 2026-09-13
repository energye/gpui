// Command ui_polish_gallery is the Ant control preview window.
//
// Left nav (temporary plain list; replaced by Tabs when W2 lands), right
// content per control page. One tab per control; sections follow official
// examples (§6.8 P0 first). Each tab opens standalone with -tab=<name> for
// independent test/screenshot; the combined window is preview only.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	CGO_ENABLED=0 go run ./examples/ui_polish_gallery -tab=overview
//	RUN_SECONDS=15 go run ./examples/ui_polish_gallery
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	tab := flag.String("tab", "", "open one page standalone (e.g. -tab=button)")
	list := flag.Bool("list", false, "list pages and exit")
	flag.Parse()
	if *list {
		for _, p := range Pages() {
			fmt.Printf("%s\t%s\t%s (%d sections)\n", p.Name, p.Title, p.Doc, len(p.Sections))
		}
		return
	}
	selected := *tab
	if _, ok := Lookup(selected); !ok {
		selected = "overview"
	}
	standalone := *tab != ""

	secs := runSeconds(30)
	w, h := galleryW, galleryH
	title := "gpui polish gallery"
	if standalone {
		title += " — " + selected
	}
	fmt.Fprintf(os.Stderr, "ui_polish_gallery: tab=%s standalone=%v %ds %dx%d\n", selected, standalone, secs, int(w), int(h))

	win, err := platform.Open(platform.Options{
		Width: int(w), Height: int(h),
		Title:       title,
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()

	scene := NewScene(w, h, selected)
	scene.Layout()

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(win.Host(), scene.Root, embedder.PipelineOptions{
		ClearR:  0.96,
		ClearG:  0.96,
		ClearB:  0.96,
		ClearA:  1,
		RunFor:  time.Duration(secs) * time.Second,
		WarmUp:  true,
		Overlay: scene.Overlay,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				return
			}
			if ev.Type == platform.EventWake || ev.Type == platform.EventExpose {
				return
			}
			if ev.Type == platform.EventResize && ev.Width > 0 && ev.Height > 0 {
				scene.Width, scene.Height = float64(ev.Width), float64(ev.Height)
				scene.rebuild()
				scene.Root.MarkNeedsPaint()
				app.ScheduleFrame()
				return
			}
			if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerDown && !standalone {
				if name, ok := scene.HitNav(ev.X, ev.Y); ok {
					scene.Select(name)
					scene.Root.MarkNeedsPaint()
					app.ScheduleFrame()
				}
				return
			}
			if ev.Type == platform.EventKey && ev.Pressed {
				_ = rendering.Point{}
			}
		},
	})

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
	out := map[string]any{
		"scenario":   "ui_polish_gallery",
		"tab":        selected,
		"standalone": standalone,
		"pages":      len(Pages()),
		"backend":    win.Backend(),
		"elapsed_s":  elapsed,
		"presents":   app.PresentCount(),
	}
	if b, err := json.Marshal(out); err == nil {
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
