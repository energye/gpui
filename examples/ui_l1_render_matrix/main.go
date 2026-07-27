// Command ui_l1_render_matrix is a multi-axis L1 render verification smoke (M1–M5).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/ui_l1_render_matrix
//
// Alignment claim: PARTIAL Flutter-style L1 contracts (boundaries, CompositeOnly stats,
// DirtyLayerIDs, async present, virtual list). NOT full Engine parity — true dirty-RECT
// present / Picture display lists / per-layer GPU RT are P6 and intentionally unclaimed.
//
// P0 closeout: optional default font on text nodes; RSS/CPU in metrics JSON.
// See README.md in this directory for metric interpretation.
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
	fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: PARTIAL Flutter L1 contract smoke — %ds (RUN_SECONDS / GPUI_DISPLAY)\n", secs)
	fmt.Fprintln(os.Stderr, "ui_l1_render_matrix: axes M1 spinner | M2 multi-BD | M3 list | M4 text | M5 gradient")
	fmt.Fprintln(os.Stderr, "ui_l1_render_matrix: does NOT claim P6 dirty-rect partial present")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 720, 400
	win, err := exhost.Open(exhost.Options{
		Width:  winW,
		Height: winH,
		Title:  "gpui L1 render matrix (PARTIAL)",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	// Explicit close after app for after_close RSS (no defer).

	sc := buildMatrixScene(float64(winW), float64(winH))
	if face, path, err := painting.TryLoadDefaultFace(16); err != nil {
		fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: default font skipped: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: default font %s\n", path)
		if sc.Latin != nil {
			sc.Latin.SetFace(face)
		}
		if sc.CJK != nil {
			// same face may lack CJK glyphs; still better than no face for Latin measure.
			sc.CJK.SetFace(face)
		}
	}

	host := win.Host()
	app := embedder.NewPipelineApp(host, sc.Root, embedder.PipelineOptions{
		ClearR: 0.07, ClearG: 0.08, ClearB: 0.10, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: window close (%s)\n", win.Backend())
			}
		},
	})

	app.Scheduler().Tickers().Add(&matrixTicker{
		on: func(dt float64) {
			sc.onTick(dt, app.ScheduleFrame)
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
	presents := app.PresentCount()
	if presents < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: no frames presented")
		os.Exit(1)
	}
	m := app.Metrics().Snapshot()
	fps := float64(presents) / elapsed
	bind := 0
	if sc.List != nil {
		bind = sc.List.BindCount
	}
	fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: backend=%s presents=%d ~%.1f fps over %.1fs layout_flushes=%d raster_layers=%d bind=%d scrollY=%.0f\n",
		win.Backend(), presents, fps, elapsed, app.LayoutFlushCount(), app.LastRasterStats().RasterLayerCount, bind, sc.scrollY)
	fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: avg=%.2fms max=%.2fms p50=%.2f p99=%.2f hitches=%d vsync=%s\n",
		m.AvgFrameIntervalMs, m.MaxFrameIntervalMs, m.P50FrameIntervalMs, m.P99FrameIntervalMs, m.HitchCount, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_l1_render_matrix: rss start=%d end=%d peak=%d after_close=%d KB cpu_avg=%.1f%%\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, m.CPUPctAvg)
	fmt.Fprintln(os.Stderr, "ui_l1_render_matrix: note raster_layer_count is DirtyLayer stats — present path is still full clear+paint (P6)")

	if sc.List != nil && sc.List.BindCount >= sc.itemCount {
		fmt.Fprintln(os.Stderr, "FAIL: virtualization mounted all rows")
		os.Exit(1)
	}
	if app.LayoutFlushCount() > presents/2 && presents > 10 {
		fmt.Fprintf(os.Stderr, "warning: layout flushes high (%d / %d presents) — check dirty locality\n",
			app.LayoutFlushCount(), presents)
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

type matrixTicker struct {
	on func(dt float64)
}

func (t *matrixTicker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
