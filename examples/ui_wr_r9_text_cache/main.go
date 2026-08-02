// Command ui_wr_r9_text_cache is the W1 R9 real-window: text measure cache.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/exhost"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R9")
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := exhost.Open(exhost.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r9_text_cache — measure 缓存"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R9 measure缓存 — 每tick重布局 文字不变", []string{
		"多字号/颜色/粗细样式",
		"每 tick MarkNeedsLayout",
		"measure_cache_hit in HUD",
		"hits≥miss ⇒ cache有效",
	})

	// Multiple text styles: sizes 10/14/18/24, colors, wrapped paragraphs.
	texts := make([]*rendering.RenderText, 0, 10)
	sizes := []float64{10, 14, 18, 24}
	colors := [][3]float64{
		{0.9, 0.9, 0.95}, {0.4, 0.8, 0.6}, {0.8, 0.6, 0.9}, {0.9, 0.75, 0.4},
	}
	for i := 0; i < 8; i++ {
		sz := sizes[i%len(sizes)]
		t := wrkit.Label(fmt.Sprintf("measure cache sample %d with some longer text to wrap lines", i), sz, colors[i%len(colors)][0], colors[i%len(colors)][1], colors[i%len(colors)][2])
		t.SetMaxWidth(280)
		shell.Body.Place(t, 20+float64(i%2)*310, 20+float64(i/2)*85)
		texts = append(texts, t)
	}
	// Wrapped long paragraph (many measureLine calls per layout).
	para := wrkit.Label("long wrapped paragraph: measure cache words repeated many times in this sentence so wrapping produces multiple measure calls per layout pass and the cache pays off on the next forced layout", 12, 0.7, 0.78, 0.9)
	para.SetMaxWidth(380)
	shell.Body.Place(para, 20, 370)
	texts = append(texts, para)
	// Second paragraph: different width → different line-break set (density).
	para2 := wrkit.Label("second paragraph at a different width forces a different set of line breaks which doubles the per-layout measure traffic and makes the hit/miss split visible in the hud counters", 12, 0.75, 0.8, 0.92)
	para2.SetMaxWidth(300)
	shell.Body.Place(para2, 440, 370)
	texts = append(texts, para2)
	// Dense label column: many small texts with distinct sizes/colors.
	for i := 0; i < 8; i++ {
		size := 9 + float64(i%3)*2
		col := 0.45 + 0.06*float64(i%4)
		t := wrkit.Label(fmt.Sprintf("dense %d @%.0fpx", i, size), size, col, 0.7, 0.85)
		shell.Body.Place(t, 780+float64(i%2)*150, 20+float64(i/2)*45)
		texts = append(texts, t)
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: close (%s)\n", win.Backend())
			}
		},
	})

	clock := wrkit.NewPhaseClock(1.5, 3.5)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase := clock.Advance(dt)
		// Force layout every tick but text unchanged → measure cache must hit.
		for _, t := range texts {
			t.MarkNeedsLayout()
		}
		if phase == wrkit.PhaseSpike {
			// One text changes → only its cache invalidates; others hit.
			texts[2].SetText(fmt.Sprintf("spike changed text at %.0f", clock.Elapsed()))
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD (hit/miss are the R9 proof on screen).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.MeasureCacheHit >= 1 && snapH.MeasureCacheHit >= snapH.MeasureCacheMiss
		shell.UpdateHUD("R9", phase, app, gateOK,
			fmt.Sprintf("hit=%d miss=%d", snapH.MeasureCacheHit, snapH.MeasureCacheMiss), "")
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R9",
		Scenario:      "ui_wr_r9_text_cache",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"text_styles": len(texts),
			"phases_seen": clock.Name(),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	errG := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		MinMeasureCacheHit:     1,
		RequireHitsGEMMiss:     true,
		RequireFullPaintPolicy: true,
	})
	if errG != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", errG)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: OK hit=%d miss=%d presents=%d elapsed=%.1fs\n",
		snap.MeasureCacheHit, snap.MeasureCacheMiss, app.PresentCount(), elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
