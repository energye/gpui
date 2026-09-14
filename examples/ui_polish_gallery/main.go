// Command ui_polish_gallery is the Ant control preview window.
//
// Left nav (temporary plain list; replaced by Tabs when W2 lands), right
// content per control page. One tab per control; sections follow official
// examples (§6.8 P0 first). Each tab opens standalone with -tab=<name> for
// independent test/screenshot; the combined window is preview only.
//
// Live widgets (widgets.go) mount one P0 representative per W1 page on the
// section strips, so the GPU draws the real component.
//
// Modes (same shape as ui_pf_x11_dnd):
//
//	go run ./examples/ui_polish_gallery -tab=button -auto-only
//	  headless selftest for all pages, then a short real window for -tab
//	  (JSON on stdout, exit 1 on fail; gate mode).
//	go run ./examples/ui_polish_gallery -tab=button
//	  selftest, then manual until close (click nav, close X to finish).
//	go run ./examples/ui_polish_gallery -tab=button -manual-seconds 30
//	  manual for 30s, then summary JSON.
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

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"

	_ "github.com/energye/gpui/render/gpu"
)

func main() {
	tab := flag.String("tab", "", "open one page standalone (e.g. -tab=button)")
	list := flag.Bool("list", false, "list pages and exit")
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
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

	// Headless selftest first: layouts every page, no GPU needed.
	rows := selftestWidgets()
	ok := pfkit.Report(rows)
	if !*autoOnly {
		if !ok {
			fmt.Fprintln(os.Stderr, "ui_polish_gallery: selftest FAIL, not opening window")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "ui_polish_gallery: selftest done, entering manual phase")
		fmt.Fprintln(os.Stderr, "  click nav entries to switch pages;")
		fmt.Fprintln(os.Stderr, "  close the window (X) to finish, or wait for -manual-seconds.")
	} else if !ok {
		emitGateJSON(selected, "unknown", rows, false, 0, manualSummary{Note: "headless selftest failed"})
		os.Exit(1)
	}

	// Window durations: gate mode runs short (proves backend + presents);
	// manual runs until close or -manual-seconds / RUN_SECONDS.
	var secs int
	if *autoOnly {
		secs = runSeconds(5)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}
	w, h := galleryW, galleryH
	title := "gpui polish gallery"
	if standalone {
		title += " — " + selected
	}
	idleTitle := title
	fmt.Fprintf(os.Stderr, "ui_polish_gallery: tab=%s standalone=%v runfor=%v %dx%d auto-only=%v\n",
		selected, standalone, runFor, int(w), int(h), *autoOnly)

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
	ctl := win.Controls()

	scene := NewScene(w, h, selected)
	scene.Layout()

	var summary manualSummary
	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(win.Host(), scene.Root, embedder.PipelineOptions{
		ClearR:  0.96,
		ClearG:  0.96,
		ClearB:  0.96,
		ClearA:  1,
		RunFor:  runFor,
		WarmUp:  true,
		Overlay: scene.Overlay,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				return
			case platform.EventPointer:
				summary.Pointer++
				// Interaction backbone first: hover/press/release reach the
				// real components (button/alert/tag/float-button wired).
				// Nav switching keeps working underneath.
				dirty := false
				switch ev.Pointer {
				case platform.PointerMove:
					if label := scene.DispatchMove(ev.X, ev.Y); label != "" {
						dirty = true
					}
				case platform.PointerDown:
					fmt.Fprintf(os.Stderr, "pointer down (%.0f,%.0f)\n", ev.X, ev.Y)
					if label, ok := scene.DispatchDown(ev.X, ev.Y); ok {
						summary.Activate++
						fmt.Fprintf(os.Stderr, "activate press %s\n", label)
						dirty = true
					}
					if name, ok := scene.HitNav(ev.X, ev.Y); ok && !standalone {
						scene.Select(name)
						summary.Nav++
						fmt.Fprintf(os.Stderr, "nav -> %s\n", name)
						if ctl != nil {
							ctl.SetTitle(idleTitle + " — " + name)
						}
						dirty = true
					}
				case platform.PointerUp:
					if label, ok := scene.DispatchUp(ev.X, ev.Y); ok {
						summary.Activate++
						fmt.Fprintf(os.Stderr, "activate release %s\n", label)
						dirty = true
					}
				}
				if dirty {
					scene.Root.MarkNeedsPaint()
					app.ScheduleFrame()
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					fmt.Fprintf(os.Stderr, "key %+v\n", ev)
					if key := galleryKey(ev); key != "" {
						if label, ok := scene.DispatchKey(key); ok {
							fmt.Fprintf(os.Stderr, "key activate %s via %s\n", label, key)
							scene.Root.MarkNeedsPaint()
							app.ScheduleFrame()
						} else if key == "Tab" {
							scene.Root.MarkNeedsPaint()
							app.ScheduleFrame()
						}
					}
				}
				_ = rendering.Point{}
				return
			case platform.EventWake, platform.EventExpose:
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					scene.Width, scene.Height = float64(ev.Width), float64(ev.Height)
					scene.rebuild()
					scene.Root.MarkNeedsPaint()
					app.ScheduleFrame()
				}
				return
			default:
				return
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
	presents := app.PresentCount()
	backend := win.Backend().String()
	pass := ok && presents >= 1
	if backend == "" || backend == "auto" {
		pass = false
	}
	if *autoOnly {
		emitGateJSON(selected, backend, rows, pass, presents, manualSummary{Note: "gate"})
		if !pass {
			fmt.Fprintf(os.Stderr, "gate FAIL: backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "gate PASS: backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
		return
	}
	summary.Timed = secs > 0
	rows = append(rows, summary.rows()...)
	ok = pfkit.Report(rows)
	emitGateJSON(selected, backend, rows, ok && presents >= 1, presents, summary)
	fmt.Fprintf(os.Stderr, "ui_polish_gallery backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
	if !(ok && presents >= 1) {
		os.Exit(1)
	}
}

type manualSummary struct {
	Pointer  int
	Key      int
	Resize   int
	Nav      int
	Activate int
	Timed    bool
	Note     string
}

func (s manualSummary) rows() []pfkit.ResultRow {
	return []pfkit.ResultRow{{
		Name: "ManualGallery",
		OK:   true,
		Detail: fmt.Sprintf("pointer=%d key=%d resize=%d nav=%d activate=%d timed=%v %s",
			s.Pointer, s.Key, s.Resize, s.Nav, s.Activate, s.Timed, s.Note),
	}}
}

func emitGateJSON(tab, backend string, rows []pfkit.ResultRow, pass bool, presents int64, m manualSummary) {
	b, _ := json.Marshal(map[string]any{
		"scenario": "ui_polish_gallery",
		"tab":      tab,
		"backend":  backend,
		"rows":     rows,
		"manual": map[string]any{
			"pointer": m.Pointer, "key": m.Key, "resize": m.Resize,
			"nav": m.Nav, "activate": m.Activate, "timed": m.Timed, "note": m.Note,
		},
		"presents": presents,
		"pass":     pass,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
