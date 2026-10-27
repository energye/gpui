// Command ui_base_multiwin_cap is matrix row 10: multi-window sharing ceiling.
//
// Eight 480x320 windows are opened once, all with the same spinning pinwheel.
// A controller activates 1 -> 2 -> 4 -> 8 windows in 12s phases (single
// process) and records the CPU/window curve, per-window amortized cost and
// the maximum window count that still holds 60fps.
//
// Every app has its own X11 display connection and Run loop; inactive windows
// implement scheduler.FrameWanter so they sleep with zero frames (the row-1
// idle contract) until the controller activates them.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=75 go run ./examples/ui_base_multiwin_cap -auto-only
//	go run ./examples/ui_base_multiwin_cap -manual-seconds 100  # human watch
//
// Gate needs 75s (10s all-window prewarm + 4 x (1s settle + 12s phase)); the
// run must exceed 60s so the process RSS slope has a valid steady window.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 480, 320
	maxWindows = 8
	// phaseSec is the per-window-count measurement window; settleSec excludes
	// the activation transient (new loop waking, first frame).
	phaseSec   = 12.0
	settleSec  = 1.0
	prewarmSec = 10.0
	// gateSeconds = prewarm + 4*(settle+phase) + open/snapshot headroom.
	gateSeconds = 75
	// health line (matrix §4), applied per active window.
	healthFPS = 55.0
	healthP95 = 22.0
)

type phaseResult struct {
	Windows         int     `json:"windows"`
	Seconds         float64 `json:"seconds"`
	Frames          int64   `json:"frames"`
	FPSAvg          float64 `json:"fps_avg"`
	FPSMin          float64 `json:"fps_min"`
	FPSMax          float64 `json:"fps_max"`
	P95MaxMs        float64 `json:"p95_max_ms"`
	CPUPct          float64 `json:"cpu_pct"`
	CPUPerWindowPct float64 `json:"cpu_per_window_pct"`
	CPUmsPerPresent float64 `json:"cpu_ms_per_present"`
	MarginalCPUPct  float64 `json:"marginal_cpu_pct"`
	RSSStartMB      float64 `json:"rss_start_mb"`
	RSSEndMB        float64 `json:"rss_end_mb"`
	RSSGrowthMB     float64 `json:"rss_growth_mb"`
	Presents        []int64 `json:"presents"`
	CPUOK           bool    `json:"cpu_ok"`
	Pass60          bool    `json:"pass_60fps"`
	Steady60        bool    `json:"steady_60fps"`
}

type win struct {
	idx  int
	win  *platform.Window
	ctl  platform.WindowController
	app  *embedder.PipelineApp
	sc   *winScene
	box  *rendering.RenderBox
	path string
}

type winTicker struct {
	idx    int
	active *atomic.Int32
	sc     *winScene
	app    *embedder.PipelineApp
	box    *rendering.RenderBox
}

func (t *winTicker) Tick(dt float64) bool {
	if t == nil || t.app == nil {
		return false
	}
	if t.idx >= int(t.active.Load()) {
		return true
	}
	t.sc.advance(dt)
	t.box.MarkNeedsPaint()
	t.app.ScheduleFrame()
	return true
}

// WantsFrame keeps inactive windows fully idle: the scheduler blocks in
// WaitEvents with no cadence until the controller schedules a frame and wakes
// the host explicitly.
func (t *winTicker) WantsFrame() bool {
	return t != nil && t.idx < int(t.active.Load())
}

func waitPresents(app *embedder.PipelineApp, want int64, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if app.PresentCount() >= want {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func setActive(wins []*win, active *atomic.Int32, n int) {
	active.Store(int32(n))
	for i, w := range wins {
		state := "IDLE"
		if i < n {
			state = "ACTIVE"
			w.app.ScheduleFrame()
			if w.win.Host() != nil {
				w.win.Host().WakeUp()
			}
		}
		if w.ctl != nil {
			w.ctl.SetTitle(fmt.Sprintf("gpui ui_base_multiwin_cap %d/%d %s", i+1, maxWindows, state))
		}
	}
}

// sleepStop waits d unless the run is being torn down; false means aborted.
func sleepStop(d time.Duration, stop <-chan struct{}) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return false
	case <-t.C:
		return true
	}
}

// measurePhase samples one steady phase: wall fps from each window's present
// delta, p95 from each app's interval metrics, and process CPU/RSS from /proc.
// It returns ok=false when the run is stopped mid-phase; a truncated phase is
// never recorded as a real measurement (the 0fps artifact seen when a manual
// timeout fires during the ladder).
func measurePhase(wins []*win, n int, stop <-chan struct{}) (phaseResult, bool) {
	startP := make([]int64, n)
	for i := 0; i < n; i++ {
		startP[i] = wins[i].app.PresentCount()
	}
	cpu0, cpuOK0 := wrkit.ProcessCPUJiffies()
	rss0 := scheduler.ReadRSSKB()
	t0 := time.Now()
	if !sleepStop(time.Duration(phaseSec*float64(time.Second)), stop) {
		return phaseResult{}, false
	}
	wall := time.Since(t0).Seconds()
	if wall <= 0 {
		wall = phaseSec
	}
	cpu1, cpuOK1 := wrkit.ProcessCPUJiffies()
	rss1 := scheduler.ReadRSSKB()

	res := phaseResult{
		Windows:     n,
		Seconds:     wall,
		CPUOK:       cpuOK0 && cpuOK1,
		RSSStartMB:  float64(rss0) / 1024,
		RSSEndMB:    float64(rss1) / 1024,
		RSSGrowthMB: float64(rss1-rss0) / 1024,
		Presents:    make([]int64, n),
	}
	var total int64
	res.FPSMin = 1e9
	for i := 0; i < n; i++ {
		d := wins[i].app.PresentCount() - startP[i]
		res.Presents[i] = d
		total += d
		fps := float64(d) / wall
		if fps < res.FPSMin {
			res.FPSMin = fps
		}
		if fps > res.FPSMax {
			res.FPSMax = fps
		}
		if p := wins[i].app.Metrics().Snapshot().P95FrameIntervalMs; p > res.P95MaxMs {
			res.P95MaxMs = p
		}
	}
	res.Frames = total
	res.FPSAvg = float64(total) / float64(n) / wall
	if res.CPUOK {
		res.CPUPct = (cpu1 - cpu0) / wrkit.ProcessCPUTicksPerSec() / wall * 100
		res.CPUPerWindowPct = res.CPUPct / float64(n)
		if total > 0 {
			res.CPUmsPerPresent = res.CPUPct / 100 * wall * 1000 / float64(total)
		}
	}
	res.Pass60 = res.FPSMin >= healthFPS
	res.Steady60 = res.Pass60 && res.P95MaxMs > 0 && res.P95MaxMs <= healthP95
	return res, true
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: 4 phases, 8 snapshots, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "BASE_MULTIWIN_CAP")
	} else if *autoOnly {
		secs, secsSet = gateSeconds, true
	}
	gateMode := secsSet
	if gateMode && secs < gateSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: BASE_MULTIWIN_CAP gate needs RUN_SECONDS>=%d (prewarm + 4 phases)\n", gateSeconds)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	snapDir := filepath.Join("examples", "ui_base_multiwin_cap", "testdata")
	if gateMode {
		os.MkdirAll(snapDir, 0o755)
	}

	active := &atomic.Int32{}
	active.Store(0)
	var eventsTotal atomic.Int64
	wins := make([]*win, 0, maxWindows)
	userClosed := make(chan struct{})
	var closeOnce sync.Once

	for i := 0; i < maxWindows; i++ {
		col, row := i%4, i/4
		pos := &platform.Point{X: col * winW, Y: row * (winH + 60)}
		winTitle := fmt.Sprintf("gpui ui_base_multiwin_cap %d/%d IDLE", i+1, maxWindows)
		backend := platform.DisplayAuto
		// Measurement glue (matrix row 10): under a real GNOME Wayland session
		// the ruler harness samples via Xlib on :0 (XWayland). Auto would pick
		// native Wayland and be invisible to the sampler, so the harness sets
		// GPUI_RULER_BACKEND=x11 to force the XWayland branch for BOTH arms.
		// Window behavior semantics are unchanged.
		if os.Getenv("GPUI_RULER_BACKEND") == "x11" {
			backend = platform.DisplayX11
		}
		w, err := platform.Open(platform.Options{
			Width: winW, Height: winH, Title: winTitle,
			Position: pos, Decorations: true, Resizable: false,
			Backend: backend,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: window %d open (needs_gpu_window): %v\n", i+1, err)
			os.Exit(1)
		}
		ctl := w.Controls()
		sc := newWinScene(i, active)
		root := rendering.NewAbsoluteBox(winW, winH)
		root.Background = &rendering.Color{R: 0.10, G: 0.12, B: 0.16, A: 1}
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = winW, winH

		snap := ""
		if gateMode {
			snap = filepath.Join(snapDir, fmt.Sprintf("multiwin_win%d.png", i+1))
		}
		wi := &win{idx: i, win: w, ctl: ctl, sc: sc, box: box, path: snap}

		var app *embedder.PipelineApp
		box.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			fps := 0.0
			var presents int64
			if app != nil {
				if m := app.Metrics().Snapshot(); m.AvgFrameIntervalMs > 1e-6 {
					fps = 1000.0 / m.AvgFrameIntervalMs
				}
				presents = app.PresentCount()
			}
			sc.paint(pc, size, fps, presents)
		}
		root.Place(box, 0, 0)

		app = embedder.NewPipelineApp(w.Host(), root, embedder.PipelineOptions{
			ClearR: 0.10, ClearG: 0.12, ClearB: 0.16, ClearA: 1,
			WarmUp:       true,
			SnapshotPath: snap,
			OnEvent: func(ev platform.Event) {
				switch ev.Type {
				case platform.EventClose:
					eventsTotal.Add(1)
					fmt.Fprintf(os.Stderr, "ui_base_multiwin_cap: win %d close\n", i+1)
					closeOnce.Do(func() { close(userClosed) })
				case platform.EventPointer, platform.EventKey:
					eventsTotal.Add(1)
					fmt.Fprintf(os.Stderr, "ui_base_multiwin_cap: win %d event type=%d\n", i+1, ev.Type)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_base_multiwin_cap %d/%d EVENT", i+1, maxWindows))
					}
				}
			},
		})
		app.SetPresentPolicy(scheduler.PresentPolicyRetained)
		app.Scheduler().Tickers().Add(&winTicker{idx: i, active: active, sc: sc, app: app, box: box})
		wi.app = app
		wins = append(wins, wi)
	}

	// Open all surfaces before any Run: opening mid-ladder would charge the
	// new window's warmup (fonts/GPU pools) to the next phase.
	for _, w := range wins {
		if err := w.app.Open(); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: app %d open: %v\n", w.idx+1, err)
			os.Exit(1)
		}
	}

	var wg sync.WaitGroup
	runErr := make([]error, maxWindows)
	t0 := time.Now()
	for i, w := range wins {
		wg.Add(1)
		go func(i int, w *win) {
			defer wg.Done()
			runErr[i] = w.app.Run()
		}(i, w)
	}
	// WarmUp runs at the top of Run (not Open): wait until every window has
	// completed its first full present before charging the prewarm phase.
	for _, w := range wins {
		if !waitPresents(w.app, 1, 15*time.Second) {
			fmt.Fprintf(os.Stderr, "FAIL: app %d no warm-up present\n", w.idx+1)
			os.Exit(1)
		}
	}

	var phases []phaseResult
	var phaseMu sync.Mutex
	stop := make(chan struct{})
	var stopOnce sync.Once
	quitAll := func() {
		stopOnce.Do(func() { close(stop) })
		for _, w := range wins {
			w.app.Quit()
		}
	}
	ctrlDone := make(chan struct{})
	go func() {
		defer close(ctrlDone)
		// Prewarm all eight: every window's GPU pool and glyph cache reaches
		// its high-water mark before the ladder, so phase CPU/RSS is steady.
		setActive(wins, active, maxWindows)
		if !sleepStop(time.Duration(prewarmSec*float64(time.Second)), stop) {
			return
		}
		for _, n := range []int{1, 2, 4, 8} {
			setActive(wins, active, n)
			if !sleepStop(time.Duration(settleSec*float64(time.Second)), stop) {
				return
			}
			res, ok := measurePhase(wins, n, stop)
			if !ok {
				return
			}
			phaseMu.Lock()
			phases = append(phases, res)
			phaseMu.Unlock()
			fmt.Fprintf(os.Stderr, "ui_base_multiwin_cap: phase n=%d fps_avg=%.1f min=%.1f p95max=%.1f cpu=%.1f%% rss=%.0f->%.0fMB presents=%v\n",
				n, res.FPSAvg, res.FPSMin, res.P95MaxMs, res.CPUPct, res.RSSStartMB, res.RSSEndMB, res.Presents)
		}
		if gateMode {
			// Freeze every window at phase 0, then quit: each Run's
			// SnapshotPath repaint writes the identical frozen tree.
			for _, w := range wins {
				w.sc.freeze()
				w.box.MarkNeedsPaint()
				w.app.ScheduleFrame()
				w.win.Host().WakeUp()
			}
			time.Sleep(150 * time.Millisecond)
			quitAll()
		}
	}()

	runDone := make(chan struct{})
	go func() { wg.Wait(); close(runDone) }()

	// RSS-series sampler: measurement must not drive frames, so it runs on a
	// plain 500ms goroutine rather than a UI ticker. Without a series the
	// 60s slope has only the start/stop points and stays invalid.
	sampleStop := make(chan struct{})
	sampleDone := make(chan struct{})
	go func() {
		defer close(sampleDone)
		tk := time.NewTicker(500 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				proc.Sample()
			case <-sampleStop:
				return
			}
		}
	}()

	var timedOut bool
	if gateMode {
		select {
		case <-runDone:
		case <-time.After(time.Duration(gateSeconds+20) * time.Second):
			timedOut = true
			quitAll()
			<-runDone
		}
	} else if *manualSeconds > 0 {
		timer := time.NewTimer(time.Until(t0.Add(time.Duration(*manualSeconds * float64(time.Second)))))
		defer timer.Stop()
		select {
		case <-runDone:
		case <-timer.C:
			quitAll()
			<-runDone
		case <-userClosed:
			quitAll()
			<-runDone
		}
	} else {
		select {
		case <-runDone:
		case <-userClosed:
			quitAll()
			<-runDone
		}
	}
	<-ctrlDone
	elapsed := time.Since(t0).Seconds()
	close(sampleStop)
	<-sampleDone
	proc.Stop()
	for _, w := range wins {
		w.app.Close()
		w.win.Close()
	}
	proc.NoteAfterClose()
	proc.Apply(wins[0].app.Metrics())

	for i, err := range runErr {
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: app %d run: %v\n", i+1, err)
			if gateMode {
				os.Exit(1)
			}
		}
	}
	if timedOut {
		fmt.Fprintln(os.Stderr, "FAIL: gate timed out waiting for window loops")
		if gateMode {
			os.Exit(1)
		}
	}

	snap := wins[0].app.Metrics().Snapshot()
	var totalPresents int64
	var cpuFallback, missedVSync int64
	for _, w := range wins {
		totalPresents += w.app.PresentCount()
		m := w.app.Metrics().Snapshot()
		cpuFallback += m.CPUFallbackOps
		missedVSync += m.MissedVSync
	}

	phaseMu.Lock()
	gotPhases := append([]phaseResult{}, phases...)
	phaseMu.Unlock()
	for i := 1; i < len(gotPhases); i++ {
		dn := gotPhases[i].Windows - gotPhases[i-1].Windows
		if dn > 0 {
			gotPhases[i].MarginalCPUPct = (gotPhases[i].CPUPct - gotPhases[i-1].CPUPct) / float64(dn)
		}
	}
	maxWindows60 := 0
	maxWindowsSteady := 0
	for _, p := range gotPhases {
		if !p.Pass60 {
			break
		}
		maxWindows60 = p.Windows
		if p.Steady60 {
			maxWindowsSteady = p.Windows
		}
	}
	cpuScale8 := 0.0
	if len(gotPhases) > 0 && gotPhases[0].CPUPct > 0 {
		for _, p := range gotPhases {
			if p.Windows == maxWindows {
				cpuScale8 = p.CPUPct / gotPhases[0].CPUPct
			}
		}
	}

	// Pixel + Golden evidence: one snapshot per window (gate only).
	var pixelOK, pixelTotal int
	pixelBad := make([]string, 0)
	goldenMax := 0.0
	goldenPer := make([]float64, maxWindows)
	goldenTotalPx := int64(0)
	goldenFirst := false
	pixelGoldenValue := any(nil)
	if gateMode {
		bg := [3]float64{0.10, 0.12, 0.16}
		res := make(map[string]bool)
		for i, w := range wins {
			img := loadImage(w.path)
			prefix := fmt.Sprintf("win%d_", i+1)
			checks := []pixelCheck{
				{desc: prefix + "bg", x: 10, y: 300, want: bg, tol: 0.05, kind: kindBG},
			}
			inks := []inkRegion{
				{desc: prefix + "ink", x: 180, y: 100, w: 120, h: 120, bg: bg, tol: 0.05, minPx: 400},
			}
			runPixelChecks(img, winW, checks, res)
			runInkRegions(img, winW, inks, res)
			pixelTotal += len(checks) + len(inks)
			base := filepath.Join(snapDir, fmt.Sprintf("multiwin_win%d_base.png", i+1))
			diff, total, first := evaluateGolden(w.path, base,
				[]rect{{x: 0, y: 40, w: winW, h: winH - 40}}, winW)
			goldenPer[i] = diff
			goldenTotalPx += total
			goldenFirst = goldenFirst || first
			if diff > goldenMax {
				goldenMax = diff
			}
		}
		for d, ok := range res {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, d)
			}
		}
		pixelGoldenValue = goldenMax
	}

	extra := map[string]any{
		"phase_sec":                 phaseSec,
		"settle_sec":                settleSec,
		"prewarm_sec":               prewarmSec,
		"windows_ladder":            []int{1, 2, 4, 8},
		"phases":                    gotPhases,
		"cpu_window_curve":          curve(gotPhases),
		"max_windows_60fps":         maxWindows60,
		"max_windows_steady60fps":   maxWindowsSteady,
		"p95_health_ms":             healthP95,
		"max_windows_note":          "max_windows_60fps = every active window >=55fps; steady additionally needs interval p95 <=22ms",
		"ladder_cap":                maxWindows,
		"ceiling_reached":           maxWindows60 == maxWindows,
		"multiwin_floor_windows":    2,
		"cpu_scale_8_over_1":        cpuScale8,
		"crash_free":                !timedOut,
		"cpu_fallback_ops_total":    cpuFallback,
		"missed_vsync_total":        missedVSync,
		"presents_total":            totalPresents,
		"presents_per_sec_at_8":     presentsPerSec(gotPhases, maxWindows),
		"evidence_mode":             map[bool]string{true: "gate", false: "manual"}[gateMode],
		"pixel_probes_ok":           pixelOK,
		"pixel_probes_total":        pixelTotal,
		"pixel_bad":                 pixelBad,
		"pixel_golden_diff_pct":     pixelGoldenValue,
		"pixel_golden_max_pct":      goldenMax,
		"pixel_golden_per_window":   goldenPer,
		"pixel_golden_total_px":     goldenTotalPx,
		"pixel_golden_first_run":    goldenFirst,
		"golden_mask":               "per-window content 480x280 (metric band excluded)",
		"golden_tolerance_pct":      0,
		"rss_slope_valid":           snap.RSSSlopeValid,
		"rss_slope_scope":           "whole run after max(25% elapsed, 15s) warmup; all 8 pre-opened+warmed before the ladder",
		"rss_slope_line_kb_per_min": 30000,
		"events_total":              eventsTotal.Load(),
	}
	if !gateMode {
		extra["pixel_golden_diff_pct"] = nil
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "base_multiwin_cap",
		Scenario:      "ui_base_multiwin_cap",
		Backend:       wins[0].win.Backend().String(),
		Snap:          snap,
		PresentCount:  totalPresents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH * maxWindows,
		Warmup:        true,
		Ruler:         wrgate.RulerFromEnv(),
		Extra:         extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if totalPresents < 1 {
		fail("no presents")
	}
	if gateMode {
		if len(gotPhases) != 4 {
			fail("phases=%d want 4 (truncated run?)", len(gotPhases))
		}
		if maxWindows60 < 2 {
			fail("multi-window floor failed: max_windows_60fps=%d want >=2", maxWindows60)
		}
		for _, p := range gotPhases {
			if !p.CPUOK {
				fail("phase n=%d cpu measurement unavailable", p.Windows)
			}
			for i, d := range p.Presents {
				if d <= 0 {
					fail("phase n=%d window %d presents delta=%d want >0", p.Windows, i+1, d)
				}
			}
		}
		if gotPhases[0].CPUPct > 85 {
			fail("phase n=1 cpu=%.1f%% > 85%% health line (idle windows must not burn CPU)", gotPhases[0].CPUPct)
		}
		if !snap.RSSSlopeValid || snap.RSSSlopeSteadyKBPerMin > 30000 {
			fail("rss_slope_steady=%.0fKB/min valid=%v want <=30000KB/min (health line)", snap.RSSSlopeSteadyKBPerMin, snap.RSSSlopeValid)
		}
		if cpuFallback > 0 {
			fail("cpu_fallback_ops_total=%d want 0", cpuFallback)
		}
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirst && goldenMax != 0 {
			fail("golden max=%.4f%% want 0 per window", goldenMax)
		}
	}

	status := "OK"
	if gateMode && goldenFirst {
		status = "SEEDED"
	}
	fmt.Fprintf(os.Stderr, "ui_base_multiwin_cap: %s presents=%d phases=%d max_windows_60fps=%d steady60=%d cpu8=%.1f%% scale8/1=%.2f px=%d/%d golden=%.4f%% mode=%s elapsed=%.1fs\n",
		status, totalPresents, len(gotPhases), maxWindows60, maxWindowsSteady, lastCPU(gotPhases), cpuScale8,
		pixelOK, pixelTotal, goldenMax, map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
	for _, p := range gotPhases {
		fmt.Fprintf(os.Stderr, "  phase n=%d fps=%.1f (min %.1f) p95max=%.1fms cpu=%.1f%% cpu/win=%.1f%% cpu_ms/present=%.2fms marginal=%.1f%% rss=%.0f->%.0fMB avg60=%v steady60=%v\n",
			p.Windows, p.FPSAvg, p.FPSMin, p.P95MaxMs, p.CPUPct, p.CPUPerWindowPct, p.CPUmsPerPresent, p.MarginalCPUPct,
			p.RSSStartMB, p.RSSEndMB, p.Pass60, p.Steady60)
	}
}

func presentsPerSec(phases []phaseResult, n int) float64 {
	for _, p := range phases {
		if p.Windows == n && p.Seconds > 0 {
			return float64(p.Frames) / p.Seconds
		}
	}
	return 0
}

func lastCPU(phases []phaseResult) float64 {
	if len(phases) == 0 {
		return 0
	}
	return phases[len(phases)-1].CPUPct
}

func curve(phases []phaseResult) [][2]float64 {
	out := make([][2]float64, 0, len(phases))
	for _, p := range phases {
		out = append(out, [2]float64{float64(p.Windows), p.CPUPct})
	}
	return out
}
