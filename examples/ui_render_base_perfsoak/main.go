// Command ui_render_base_perfsoak is the PerfSoak axis for
// docs/ENGINE_UI_RENDER_BASE.md §20 / §24 — sustained frames + RSS/CPU/hitch.
//
// Default run is short-friendly (30s); use RUN_SECONDS=300 for real soak.
// Combines: multi RepaintBoundary spinners + Transform pulse + layout gate.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_render_base_perfsoak
package main

import (
	"encoding/json"
	"fmt"
	"math"
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
	secs := runSeconds(30)
	fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: PerfSoak axis — %ds (set RUN_SECONDS for longer)\n", secs)
	fmt.Fprintln(os.Stderr, "ui_render_base_perfsoak: multi-boundary + transform; metrics JSON; not P6")

	var proc scheduler.ProcessTracker
	proc.Start()

	const winW, winH = 640, 480
	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui PerfSoak", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}

	sc := buildSoakScene(float64(winW), float64(winH))
	app := embedder.NewPipelineApp(win.Host(), sc.Root, embedder.PipelineOptions{
		ClearR: 0.06, ClearG: 0.07, ClearB: 0.09, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: close (%s)\n", win.Backend())
			}
		},
	})
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) {
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

	// M-RSS-SLOPE is KB/min from ProcessTracker.Apply (also in JSON).
	// KB/s kept for human stderr only.
	rssSlopePerMin := m.RSSSlopeKBPerMin
	rssSlopePerSec := 0.0
	if m.RSSElapsedSec > 0 {
		rssSlopePerSec = float64(m.RSSEndKB-m.RSSStartKB) / m.RSSElapsedSec
	} else if elapsed > 0 && m.RSSEndKB > 0 && m.RSSStartKB > 0 {
		rssSlopePerSec = float64(m.RSSEndKB-m.RSSStartKB) / elapsed
	}

	fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: backend=%s presents=%d ~%.1f fps layout_flushes=%d\n",
		win.Backend(), presents, fps, app.LayoutFlushCount())
	fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: avg=%.2f p50=%.2f p95=%.2f p99=%.2f hitches=%d hitch_rate=%.2f/min vsync=%s\n",
		m.AvgFrameIntervalMs, m.P50FrameIntervalMs, m.P95FrameIntervalMs, m.P99FrameIntervalMs,
		m.HitchCount, m.HitchRatePerMin, m.VSyncSource)
	fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: rss start=%d end=%d peak=%d after_close=%d KB slope=%.1f KB/min (%.1f KB/s) cpu=%.1f%% (ui=%.1f raster=%.1f path-proxy)\n",
		m.RSSStartKB, m.RSSEndKB, m.RSSPeakKB, m.RSSAfterCloseKB, rssSlopePerMin, rssSlopePerSec, m.CPUPctAvg, m.CPUUIPct, m.CPURasterPct)
	fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: gpu_ops=%d cpu_fallback_ops=%d last_cpu_fb=%q frame_flushes=%d damage_px=%d mode=%s\n",
		m.GPUOps, m.CPUFallbackOps, m.LastCPUFallbackReason, m.FrameFlushes, m.DamageAreaPx, m.PresentMode)

	// Optional: BASELINE_JSON=path compares against a saved metrics snapshot (M-BASELINE-DELTA).
	if bp := os.Getenv("BASELINE_JSON"); bp != "" {
		base, err := scheduler.LoadBaseline(bp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: load baseline %s: %v\n", bp, err)
			os.Exit(1)
		}
		delta := scheduler.CompareToBaseline(m, base, scheduler.DefaultBaselineTolerance())
		if b, err := json.Marshal(delta); err == nil {
			fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: baseline_delta %s\n", b)
		}
		if !delta.OK {
			fmt.Fprintf(os.Stderr, "FAIL: baseline regression: %v\n", delta.Failures)
			os.Exit(1)
		}
	}
	// Optional: write this run as a new baseline artifact.
	if sp := os.Getenv("SAVE_BASELINE_JSON"); sp != "" {
		if err := scheduler.SaveBaseline(sp, m); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: save baseline: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "ui_render_base_perfsoak: wrote baseline %s\n", sp)
	}

	// Gates (soft for short runs; stricter when long)
	if app.LayoutFlushCount() > presents/2 && presents > 30 {
		fmt.Fprintf(os.Stderr, "FAIL: layout storm (%d / %d)\n", app.LayoutFlushCount(), presents)
		os.Exit(1)
	}
	if secs >= 10 && fps < 30 {
		fmt.Fprintf(os.Stderr, "FAIL: fps too low ~%.1f (<30 over %ds)\n", fps, secs)
		os.Exit(1)
	}
	// Huge RSS growth on short smoke is often GPU init; only flag extreme slope on long soak.
	// Gate uses KB/min (M-RSS-SLOPE): 500 KB/s ≈ 30000 KB/min.
	if secs >= 60 && rssSlopePerMin > 30000 {
		fmt.Fprintf(os.Stderr, "FAIL: RSS slope %.1f KB/min too high over %ds\n", rssSlopePerMin, secs)
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

type tick struct{ on func(dt float64) }

func (t *tick) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type soakScene struct {
	Root     *rendering.AbsoluteBox
	spinners []*rendering.RenderSpinner
	xform    *rendering.RenderTransform
	phase    float64
}

func buildSoakScene(winW, winH float64) *soakScene {
	s := &soakScene{}
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.11, A: 1}
	s.Root = root

	title := rendering.NewRenderText("PerfSoak — multi spinner + transform · §20 metrics")
	title.FontSize = 13
	title.R, title.G, title.B, title.A = 0.7, 0.75, 0.8, 1
	root.Place(title, 12, 10)

	// Grid of spinners (each RepaintBoundary by default)
	const n = 12
	for i := 0; i < n; i++ {
		sp := rendering.NewRenderSpinner(28)
		col := i % 4
		row := i / 4
		root.Place(sp, 40+float64(col)*140, 60+float64(row)*100)
		s.spinners = append(s.spinners, sp)
	}

	box := rendering.NewRenderColorBox(48, 48, 0.95, 0.55, 0.2, 1)
	s.xform = rendering.NewRenderTransform(box)
	s.xform.FixedWidth, s.xform.FixedHeight = 48, 48
	s.xform.SetRepaintBoundary(true)
	root.Place(s.xform, winW-90, winH-90)

	return s
}

func (s *soakScene) onTick(dt float64, schedule func()) {
	if s == nil {
		return
	}
	s.phase += dt
	for i, sp := range s.spinners {
		if sp != nil {
			// staggered phases
			sp.SetPhase(s.phase*0.4 + float64(i)*0.08)
		}
	}
	if s.xform != nil {
		s.xform.SetRotation(s.phase * 1.5)
		scale := 0.85 + 0.25*math.Sin(s.phase*2)
		s.xform.SetScale(scale, scale)
	}
	if schedule != nil {
		schedule()
	}
}
