// Command ui_base_vram_cap is matrix row 17: VRAM budget pressure and
// device-recovery resource discipline (RENDER_BASE_RESOURCE_MATRIX §2 row 17).
//
// Mode A (default) — budget-refusal ladder: windows are OPENED INCREMENTALLY
// (2 -> 4 -> 8) because the shared-budget fair share divides by the live
// window count at open time; each window carries 160 static repaint-boundary
// cards, so once the shared fair share (budget / windows / 4) drops below the
// per-window working set the picture-texture cache must refuse automatic
// growth and re-record instead. The gate asserts refusals actually fire at
// 4 and 8 windows, correctness survives them (bitwise Golden), and the
// process VRAM ledger never crosses the waterline.
//
// Mode B (-oom-probe) — OOM injection recovery: one window, light scene, run
// with GPUI_FAULT_OOM="after:1" (set by the harness). The probe parses the
// same env, samples hal.VramLiveBytes() around the injected submit failure
// and asserts the app survives (no 3-frame persistent-OOM quit) and the
// ledger returns to its pre-injection level: whatever the failed present
// path allocated must be refunded (资源谁建谁放).
//
// Mode A is a multi-window batch: run it in the isolated Wayland session
// (AGENTS GPU safety; desktop forbids 8-window batches). Mode B is a single
// window and may run on the desktop.
//
//	# mode A (isolated Wayland session)
//	GPUI_RULER_BACKEND=x11 RUN_SECONDS=60 go run ./examples/ui_base_vram_cap -auto-only
//	# mode B (desktop ok)
//	GPUI_FAULT_OOM=180:1 RUN_SECONDS=15 go run ./examples/ui_base_vram_cap -oom-probe -auto-only
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 640, 480
	// ladderSteps is the registered row-17 plan: 2/4/8 windows with the
	// texture load stepping up as windows join the shared device.
	ladderSteps = 8
	// openSettle lets a freshly opened batch finish its warmup (fonts, GPU
	// pools, first card recordings) before the settle+measure window.
	openSettle = 3.0
	settleSec  = 1.0
	phaseSec   = 10.0
	// gateSeconds = 3 batches*(open+settle) + 3*(settle+phase) + hold.
	// The hold segment (fixed 8 windows, saturated caches, continuous
	// presenting) is where the leak line applies; 300s matches the matrix
	// soak closing duration (row 11 protocol).
	gateSeconds = 300
	// ledgerWaterlinePct caps live ledger bytes as a fraction of the budget:
	// the fair share clamps cache demand to budget/(windows*4) by design, so
	// crossing ~90% of budget would mean the clamp leaked.
	ledgerWaterlinePct = 0.9
	healthFPS          = 40.0 // per-window floor for the low-pressure 2-window phase
)

type phaseResult struct {
	Windows         int     `json:"windows"`
	SharedCount     int     `json:"shared_count"`
	Seconds         float64 `json:"seconds"`
	Presents        []int64 `json:"presents"`
	FPSMin          float64 `json:"fps_min"`
	FPSAvg          float64 `json:"fps_avg"`
	RefusalDelta    int64   `json:"refusal_delta"`
	RefusalsTotal   int64   `json:"refusals_total"`
	LiveMBEnd       float64 `json:"live_mb_end"`
	LiveMBMax       float64 `json:"live_mb_max"`
	LiveMBStart     float64 `json:"live_mb_start"`
	RSSStartMB      float64 `json:"rss_start_mb"`
	RSSEndMB        float64 `json:"rss_end_mb"`
	RSSGrowthMB     float64 `json:"rss_growth_mb"`
	FairShareBytes  uint64  `json:"fair_share_bytes_per_win"`
	WithinWaterline bool    `json:"within_waterline"`
}

// resSample is one timestamped process snapshot from the series sampler.
// The hold-segment RSS slope (fixed 8 windows, saturated caches) is the
// window's real leak signal — the whole-run slope is structurally inflated
// by the incremental ladder itself (each opened window adds its driver/
// runtime overhead and cache fill), so the health line judges the hold.
type resSample struct {
	T      time.Time `json:"-"`
	RSSKB  int64     `json:"rss_kb"`
	LiveMB float64   `json:"live_mb"`
}

type vwin struct {
	idx  int
	win  *platform.Window
	ctl  platform.WindowController
	app  *embedder.PipelineApp
	hud  *hudScene
	box  *rendering.RenderBox // HUD box (ticker repaints only this)
	path string
}

// hudTicker keeps presenting (retained steady frames re-run EnsureCapacity,
// which is where refusals accrue) and refreshes the HUD band.
type hudTicker struct {
	app *embedder.PipelineApp
	hud *hudScene
	box *rendering.RenderBox
}

func (t *hudTicker) Tick(dt float64) bool {
	if t == nil || t.app == nil || t.hud == nil {
		return false
	}
	if !t.hud.frozen.Load() {
		if m := t.app.Metrics().Snapshot(); m.AvgFrameIntervalMs > 1e-6 {
			fps := 1000.0 / m.AvgFrameIntervalMs
			t.hud.fps.Store(uint64(fps * 100))
		}
		t.hud.presents.Store(t.app.PresentCount())
		t.hud.refusals.Store(t.app.Metrics().Snapshot().BudgetRefusals)
		t.hud.liveMB.Store(uint64(float64(render.VramLiveBytes())/(1024*1024) * 100))
	}
	t.box.MarkNeedsPaint()
	t.app.ScheduleFrame()
	return true
}

func (t *hudTicker) WantsFrame() bool { return true }

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

func openWindow(idx, cards int, snapPath string, gateMode bool, eventsTotal *atomic.Int64, userClosed chan struct{}, closeOnce *sync.Once) (*vwin, error) {
	pos := &platform.Point{X: (idx % 4) * winW, Y: (idx / 4) * (winH + 60)}
	backend := platform.DisplayAuto
	if os.Getenv("GPUI_RULER_BACKEND") == "x11" {
		backend = platform.DisplayX11
	}
	w, err := platform.Open(platform.Options{
		Width: winW, Height: winH,
		Title: fmt.Sprintf("gpui ui_base_vram_cap %d/%d", idx+1, ladderSteps),
		Position: pos, Decorations: true, Resizable: false,
		Backend: backend,
	})
	if err != nil {
		return nil, err
	}
	ctl := w.Controls()
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.08, G: 0.09, B: 0.12, A: 1}
	buildCards(root, idx, cards)

	hud := newHudScene(idx, float64(render.VramBudgetMB()))
	hudBox := rendering.NewRenderBox()
	hudBox.FixedWidth, hudBox.FixedHeight = winW, hudH
	hudBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		hud.paint(pc, size)
	}
	root.Place(hudBox, 0, 0)

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(w.Host(), root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.12, ClearA: 1,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				eventsTotal.Add(1)
				fmt.Fprintf(os.Stderr, "ui_base_vram_cap: win %d close\n", idx+1)
				closeOnce.Do(func() { close(userClosed) })
			case platform.EventPointer, platform.EventKey:
				eventsTotal.Add(1)
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)
	app.Scheduler().Tickers().Add(&hudTicker{app: app, hud: hud, box: hudBox})
	return &vwin{idx: idx, win: w, ctl: ctl, app: app, hud: hud, box: hudBox, path: snapPath}, nil
}

// measurePhase samples one steady phase: per-window present deltas, the
// cumulative refusal counter delta, and the process ledger around the phase.
func measurePhase(wins []*vwin, stop <-chan struct{}, bumpLiveMax func(float64), getLiveMax func() float64, budgetMB float64) (phaseResult, bool) {
	startRef := int64(0)
	startP := make([]int64, len(wins))
	for i, w := range wins {
		startRef += w.app.Metrics().Snapshot().BudgetRefusals
		startP[i] = w.app.PresentCount()
	}
	live0 := float64(render.VramLiveBytes()) / (1024 * 1024)
	rss0 := scheduler.ReadRSSKB()
	t0 := time.Now()
	if !sleepStop(time.Duration(phaseSec*float64(time.Second)), stop) {
		return phaseResult{}, false
	}
	wall := time.Since(t0).Seconds()
	if wall <= 0 {
		wall = phaseSec
	}
	live1 := float64(render.VramLiveBytes()) / (1024 * 1024)
	bumpLiveMax(live1)
	rss1 := scheduler.ReadRSSKB()

	res := phaseResult{
		Windows:     len(wins),
		SharedCount: render.SharedWindowCount(),
		Seconds:     wall,
		LiveMBStart: live0,
		LiveMBEnd:   live1,
		LiveMBMax:   getLiveMax(),
		RSSStartMB:  float64(rss0) / 1024,
		RSSEndMB:    float64(rss1) / 1024,
		Presents:    make([]int64, len(wins)),
	}
	res.RSSGrowthMB = res.RSSEndMB - res.RSSStartMB
	res.FairShareBytes = fairShareBytes(res.SharedCount)
	res.WithinWaterline = getLiveMax() <= budgetMB*ledgerWaterlinePct
	endRef := int64(0)
	var total int64
	res.FPSMin = 1e9
	for i, w := range wins {
		endRef += w.app.Metrics().Snapshot().BudgetRefusals
		d := w.app.PresentCount() - startP[i]
		res.Presents[i] = d
		total += d
		fps := float64(d) / wall
		if fps < res.FPSMin {
			res.FPSMin = fps
		}
		res.FPSAvg = res.FPSAvg + fps
	}
	res.FPSAvg /= float64(len(wins))
	res.RefusalDelta = endRef - startRef
	res.RefusalsTotal = endRef
	return res, true
}

// fairShareBytes mirrors render.PictureCacheFairMax's byte math (budget /
// windows / pictureCacheBudgetDivisor) for the JSON evidence: the cap the
// refusal path enforces per window.
func fairShareBytes(windows int) uint64 {
	budget := render.VramBudgetMB()
	if windows <= 1 || budget <= 0 {
		return 0
	}
	return uint64(budget) * 1024 * 1024 / uint64(windows) / 4
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: ladder or OOM probe, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (mode A)")
	oomProbe := flag.Bool("oom-probe", false, "mode B: single-window GPUI_FAULT_OOM injection recovery probe")
	flag.Parse()

	// Mode B has its own simpler flow.
	if *oomProbe {
		runOOMProbe(*autoOnly)
		return
	}

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "BASE_VRAM_CAP")
	} else if *autoOnly {
		secs, secsSet = gateSeconds, true
	}
	gateMode := secsSet
	if gateMode && secs < gateSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: BASE_VRAM_CAP gate needs RUN_SECONDS>=%d (3 open batches + 3 phases)\n", gateSeconds)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	snapDir := filepath.Join("examples", "ui_base_vram_cap", "testdata")
	if gateMode {
		os.MkdirAll(snapDir, 0o755)
	}

	budgetMB := float64(render.VramBudgetMB())
	var eventsTotal atomic.Int64
	userClosed := make(chan struct{})
	var closeOnce sync.Once
	var wins []*vwin
	var runErrs []error
	var wg sync.WaitGroup
	runErrMu := sync.Mutex{}
	// liveMaxMu guards the process ledger high-water seen by the sampler
	// goroutine and the controller goroutine (phase measurement reads it).
	liveMaxMu := sync.Mutex{}
	liveMax := 0.0
	bumpLiveMax := func(mb float64) {
		liveMaxMu.Lock()
		if mb > liveMax {
			liveMax = mb
		}
		liveMaxMu.Unlock()
	}
	getLiveMax := func() float64 {
		liveMaxMu.Lock()
		defer liveMaxMu.Unlock()
		return liveMax
	}

	// openBatch opens count more windows, starts their Run loops and waits
	// for each first present (surface registered -> SharedWindowCount grows).
	openBatch := func(from, count int) {
		for i := from; i < from+count; i++ {
			snap := ""
			if gateMode {
				snap = filepath.Join(snapDir, fmt.Sprintf("vram_win%d.png", i+1))
			}
			w, err := openWindow(i, cardsPerWin, snap, gateMode, &eventsTotal, userClosed, &closeOnce)
			if err != nil {
				fmt.Fprintf(os.Stderr, "FAIL: window %d open (needs_gpu_window): %v\n", i+1, err)
				os.Exit(1)
			}
			wins = append(wins, w)
			if err := w.app.Open(); err != nil {
				fmt.Fprintf(os.Stderr, "FAIL: app %d open: %v\n", i+1, err)
				os.Exit(1)
			}
			wg.Add(1)
			go func(i int, w *vwin) {
				defer wg.Done()
				if err := w.app.Run(); err != nil {
					runErrMu.Lock()
					runErrs = append(runErrs, fmt.Errorf("app %d: %w", i+1, err))
					runErrMu.Unlock()
				}
			}(i, w)
		}
		for i := from; i < from+count; i++ {
			if !waitPresents(wins[i].app, 1, 15*time.Second) {
				fmt.Fprintf(os.Stderr, "FAIL: app %d no warm-up present\n", i+1)
				os.Exit(1)
			}
		}
	}

	stop := make(chan struct{})
	var stopOnce sync.Once
	quitAll := func() {
		stopOnce.Do(func() { close(stop) })
		for _, w := range wins {
			w.app.Quit()
		}
	}

	t0 := time.Now()
	var runDeadline time.Time
	if gateMode {
		runDeadline = t0.Add(time.Duration(secs) * time.Second)
	}
	var phases []phaseResult
	var phaseMu sync.Mutex
	holdStart := time.Time{}
	ctrlDone := make(chan struct{})
	go func() {
		defer close(ctrlDone)
		// Incremental open IS the pressure ladder (registered row-17 plan:
		// 2/4/8 windows): the shared fair share divides by the live window
		// count, so each step raises shared pressure past the next
		// threshold (2 windows fit, 4 and 8 must refuse cache growth).
		ladder := []int{2, 4, 8}
		prev := 0
		for _, n := range ladder {
			openBatch(prev, n-prev)
			prev = n
			if !sleepStop(time.Duration(openSettle*float64(time.Second)), stop) {
				return
			}
			if !sleepStop(time.Duration(settleSec*float64(time.Second)), stop) {
				return
			}
			res, ok := measurePhase(wins, stop, bumpLiveMax, getLiveMax, budgetMB)
			if !ok {
				return
			}
			phaseMu.Lock()
			phases = append(phases, res)
			phaseMu.Unlock()
			fmt.Fprintf(os.Stderr, "ui_base_vram_cap: phase n=%d shared=%d fps_min=%.1f refusals=+%d (total %d) live=%.1fMB (max %.1f) fair/win=%.1fMB\n",
				res.Windows, res.SharedCount, res.FPSMin, res.RefusalDelta, res.RefusalsTotal, res.LiveMBEnd, res.LiveMBMax,
				float64(res.FairShareBytes)/(1024*1024))
		}
		// Hold: fixed 8 windows, caches saturated, continuous presenting
		// until the run window closes — the leak line judges this segment.
		holdStart = time.Now()
		if gateMode && !runDeadline.IsZero() {
			if d := time.Until(runDeadline.Add(-500 * time.Millisecond)); d > 0 {
				if !sleepStop(d, stop) {
					return
				}
			}
		}
		if gateMode {
			for _, w := range wins {
				w.hud.freeze()
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

	// Ledger + RSS series sampler: measurement must not drive frames, so it
	// runs on a plain 500ms goroutine. The series feeds the hold-segment
	// slope (the window's real leak signal).
	sampleStop := make(chan struct{})
	sampleDone := make(chan struct{})
	series := make([]resSample, 0, 1024)
	var seriesMu sync.Mutex
	go func() {
		defer close(sampleDone)
		tk := time.NewTicker(500 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				proc.Sample()
				mb := float64(render.VramLiveBytes()) / (1024 * 1024)
				bumpLiveMax(mb)
				seriesMu.Lock()
				series = append(series, resSample{T: time.Now(), RSSKB: scheduler.ReadRSSKB(), LiveMB: mb})
				seriesMu.Unlock()
			case <-sampleStop:
				return
			}
		}
	}()

	var timedOut bool
	if gateMode {
		select {
		case <-runDone:
		case <-time.After(time.Duration(gateSeconds+25) * time.Second):
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
	if len(wins) > 0 {
		proc.Apply(wins[0].app.Metrics())
	}

	runErrMu.Lock()
	errs := append([]error{}, runErrs...)
	runErrMu.Unlock()
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		if gateMode {
			os.Exit(1)
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
	var refusalsTotal int64
	for _, w := range wins {
		totalPresents += w.app.PresentCount()
		refusalsTotal += w.app.Metrics().Snapshot().BudgetRefusals
	}

	phaseMu.Lock()
	gotPhases := append([]phaseResult{}, phases...)
	phaseMu.Unlock()

	// Pixel + Golden evidence: one snapshot per window (gate only).
	var pixelOK, pixelTotal int
	pixelBad := make([]string, 0)
	goldenMax := 0.0
	goldenPer := make([]float64, len(wins))
	goldenTotalPx := int64(0)
	goldenFirst := false
	pixelGoldenValue := any(nil)
	if gateMode {
		bg := [3]float64{0.08, 0.09, 0.12}
		res := make(map[string]bool)
		for i, w := range wins {
			img := loadImage(w.path)
			prefix := fmt.Sprintf("win%d_", i+1)
			checks := []pixelCheck{
				{desc: prefix + "bg", x: 10, y: winH - 10, want: bg, tol: 0.05, kind: kindBG},
			}
			inks := []inkRegion{
				{desc: prefix + "ink", x: 40, y: 120, w: 200, h: 200, bg: bg, tol: 0.05, minPx: 400},
			}
			runPixelChecks(img, winW, checks, res)
			runInkRegions(img, winW, inks, res)
			pixelTotal += len(checks) + len(inks)
			base := filepath.Join(snapDir, fmt.Sprintf("vram_win%d_base.png", i+1))
			diff, total, first := evaluateGolden(w.path, base,
				[]rect{{x: 0, y: hudH, w: winW, h: winH - hudH}}, winW)
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

	livePeakMB := float64(render.VramPeakBytes()) / (1024 * 1024)
	// Hold-segment slope: the window's real leak line. Judged over the
	// fixed-8-window hold (caches saturated); the whole-run slope is
	// structurally inflated by the incremental ladder and stays
	// informational only.
	holdSec := 0.0
	holdSlope := 0.0
	holdValid := false
	if gateMode && !holdStart.IsZero() {
		seriesMu.Lock()
		var first, last resSample
		for _, s := range series {
			if s.T.After(holdStart) {
				if first.T.IsZero() {
					first = s
				}
				last = s
			}
		}
		seriesMu.Unlock()
		if !first.T.IsZero() && last.T.Sub(first.T).Minutes() >= 0.5 {
			holdSec = last.T.Sub(holdStart).Seconds()
			holdSlope = float64(last.RSSKB-first.RSSKB) / last.T.Sub(first.T).Minutes()
			holdValid = true
		}
	}
	extra := map[string]any{
		"mode":                     "budget_ladder",
		"windows_ladder":           []int{2, 4, 8},
		"phases":                   gotPhases,
		"cards_per_window":         cardsPerWin,
		"card_px":                  fmt.Sprintf("%dx%d", cardW, cardH),
		"vram_budget_mb":           budgetMB,
		"ledger_waterline_pct":     ledgerWaterlinePct,
		"ledger_peak_mb":           livePeakMB,
		"refusals_total":           refusalsTotal,
		"refusal_curve":            refusalCurve(gotPhases),
		"fair_share_rule":          "budget / windows / 4 (render.PictureCacheFairMax) clamps automatic picture-texture cache growth",
		"presents_total":           totalPresents,
		"events_total":             eventsTotal.Load(),
		"crash_free":               !timedOut,
		"evidence_mode":            map[bool]string{true: "gate", false: "manual"}[gateMode],
		"pixel_probes_ok":          pixelOK,
		"pixel_probes_total":       pixelTotal,
		"pixel_bad":                pixelBad,
		"pixel_golden_diff_pct":    pixelGoldenValue,
		"pixel_golden_max_pct":     goldenMax,
		"pixel_golden_per_window":  goldenPer,
		"pixel_golden_total_px":    goldenTotalPx,
		"pixel_golden_first_run":   goldenFirst,
		"golden_mask":              "per-window card content below the HUD band",
		"golden_tolerance_pct":     0,
		"rss_slope_valid":           snap.RSSSlopeValid,
		"rss_slope_scope":           "whole-run slope is structural (incremental ladder adds windows+caches); the health line judges the hold segment below",
		"rss_slope_line_kb_per_min": 30000,
		"hold_sec":                  holdSec,
		"hold_slope_kb_per_min":     holdSlope,
		"hold_slope_valid":          holdValid,
		"hold_scope":                "fixed 8 windows, saturated caches, continuous presenting; leak line 30000KB/min",
	}
	if !gateMode {
		extra["pixel_golden_diff_pct"] = nil
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "base_vram_cap",
		Scenario:      "ui_base_vram_cap",
		Backend:       wins[0].win.Backend().String(),
		Snap:          snap,
		PresentCount:  totalPresents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: int64(winW * winH * len(wins)),
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
		if len(gotPhases) != 3 {
			fail("phases=%d want 3 (2/4/8 ladder truncated?)", len(gotPhases))
		}
		for _, p := range gotPhases {
			if p.SharedCount != p.Windows {
				fail("phase n=%d shared_window_count=%d want %d (incremental open precondition)", p.Windows, p.SharedCount, p.Windows)
			}
			for i, d := range p.Presents {
				if d <= 0 {
					fail("phase n=%d window %d presents delta=%d want >0", p.Windows, i+1, d)
				}
			}
		}
		if len(gotPhases) >= 2 && gotPhases[1].RefusalDelta <= 0 {
			fail("phase n=4 refusal_delta=%d want >0 (shared budget fair share did not bite)", gotPhases[1].RefusalDelta)
		}
		if len(gotPhases) >= 3 && gotPhases[2].RefusalDelta <= 0 {
			fail("phase n=8 refusal_delta=%d want >0", gotPhases[2].RefusalDelta)
		}
		if refusalsTotal <= 0 {
			fail("refusals_total=%d want >0", refusalsTotal)
		}
		if gotPhases[0].FPSMin < healthFPS {
			fail("phase n=2 fps_min=%.1f < %.1f (low-pressure phase must stay smooth; high-pressure phases record fps without gating)", gotPhases[0].FPSMin, healthFPS)
		}
		for _, p := range gotPhases {
			if !p.WithinWaterline {
				fail("phase n=%d live=%.1fMB max crossed waterline %.1fMB (budget clamp leaked)", p.Windows, p.LiveMBMax, budgetMB*ledgerWaterlinePct)
			}
		}
		if !holdValid {
			fail("hold segment missing or too short (hold_sec=%.0f): the leak line needs the fixed-8-window hold", holdSec)
		} else if holdSlope > 30000 {
			fail("hold rss slope=%.0fKB/min > 30000KB/min health line (leak in the refused/re-record steady state)", holdSlope)
		}
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirst && goldenMax != 0 {
			fail("golden max=%.4f%% want 0 per window (refusal path must not break correctness)", goldenMax)
		}
	}

	status := "OK"
	if gateMode && goldenFirst {
		status = "SEEDED"
	}
	fmt.Fprintf(os.Stderr, "ui_base_vram_cap: %s presents=%d refusals=%d ledger_peak=%.1fMB px=%d/%d golden=%.4f%% mode=%s elapsed=%.1fs\n",
		status, totalPresents, refusalsTotal, livePeakMB, pixelOK, pixelTotal, goldenMax,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

func refusalCurve(phases []phaseResult) [][3]any {
	out := make([][3]any, 0, len(phases))
	for _, p := range phases {
		out = append(out, [3]any{p.Windows, p.RefusalDelta, p.RefusalsTotal})
	}
	return out
}

// ---- Mode B: OOM injection recovery probe ----

type ledgerSample struct {
	Presents int64   `json:"presents"`
	LiveMB   float64 `json:"live_mb"`
}

type oomVerdict struct {
	Spec        string  `json:"inject_spec"`
	After       int64   `json:"inject_after_submit"`
	Count       int64   `json:"inject_count"`
	PreMB       float64 `json:"pre_inject_live_mb"`
	PostMB      float64 `json:"post_recovery_live_mb"`
	DeltaMB     float64 `json:"delta_mb"`
	ToleranceMB float64 `json:"tolerance_mb"`
	Recovered   bool    `json:"ledger_recovered"`
	Survived    bool    `json:"app_survived"`
}

func runOOMProbe(gateMode bool) {
	const (
		gateSeconds = 15
		// pre/post sample windows are in completed presents relative to the
		// injected submit; margins absorb the submit-vs-present off-by-one.
		preLo, preHi   = int64(48), int64(8)
		postLo, postHi = int64(40), int64(120)
	)
	secs := 0
	if v, set := wrkit.RunSecondsOpt(); set {
		wrkit.RequireMinRun(v, "BASE_VRAM_CAP_OOM")
		secs = v
	} else if gateMode {
		secs = gateSeconds
	}
	gateMode = secs > 0
	spec := os.Getenv("GPUI_FAULT_OOM")
	after, count := int64(0), int64(0)
	if spec != "" {
		parts := strings.SplitN(spec, ":", 2)
		after, _ = strconv.ParseInt(parts[0], 10, 64)
		if len(parts) == 2 {
			count, _ = strconv.ParseInt(parts[1], 10, 64)
		} else {
			count = 1
		}
	}
	if after <= 0 || count != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: oom-probe needs GPUI_FAULT_OOM=\"<after>:1\" (single injection), got %q\n", spec)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()
	snapDir := filepath.Join("examples", "ui_base_vram_cap", "testdata")
	if gateMode {
		os.MkdirAll(snapDir, 0o755)
	}

	var eventsTotal atomic.Int64
	userClosed := make(chan struct{})
	var closeOnce sync.Once
	snap := ""
	if gateMode {
		snap = filepath.Join(snapDir, "vram_oom_probe.png")
	}
	w, err := openWindow(0, 12, snap, gateMode, &eventsTotal, userClosed, &closeOnce)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: window open: %v\n", err)
		os.Exit(1)
	}
	if err := w.app.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: app open: %v\n", err)
		os.Exit(1)
	}
	done := make(chan error, 1)
	go func() { done <- w.app.Run() }()

	// Ledger sampler keyed by present count: pre window right before the
	// injected submit, post window well after the single failed present.
	samples := make([]ledgerSample, 0, 512)
	var samplesMu sync.Mutex
	sampleStop := make(chan struct{})
	sampleDone := make(chan struct{})
	go func() {
		defer close(sampleDone)
		tk := time.NewTicker(50 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				proc.Sample()
				samplesMu.Lock()
				samples = append(samples, ledgerSample{
					Presents: w.app.PresentCount(),
					LiveMB:   float64(render.VramLiveBytes()) / (1024 * 1024),
				})
				samplesMu.Unlock()
			case <-sampleStop:
				return
			}
		}
	}()

	// quit is idempotent; Run's completion is consumed exactly once by the
	// selects below (never inside quit — a consume here would starve the
	// later wait on the same buffered channel).
	var quitOnce sync.Once
	quit := func() { quitOnce.Do(func() { w.app.Quit() }) }
	t0 := time.Now()
	var timedOut bool
	if gateMode {
		// Normal end: the run window closes at RUN_SECONDS (+3s startup
		// margin) — freeze the HUD, let the final repaint commit the
		// snapshot, then quit and wait for the loop. Only exceeding the
		// window by 15s counts as a timeout.
		runTimer := time.NewTimer(time.Duration(secs+3) * time.Second)
		defer runTimer.Stop()
		select {
		case <-done:
		case <-runTimer.C:
			w.hud.freeze()
			w.box.MarkNeedsPaint()
			time.Sleep(150 * time.Millisecond)
			quit()
		}
		deadline := time.NewTimer(15 * time.Second)
		defer deadline.Stop()
		select {
		case <-done:
		case <-deadline.C:
			timedOut = true
			quit()
		}
	} else {
		select {
		case <-done:
		case <-userClosed:
			quit()
		}
	}
	elapsed := time.Since(t0).Seconds()
	close(sampleStop)
	<-sampleDone
	proc.Stop()
	w.app.Close()
	w.win.Close()
	proc.NoteAfterClose()
	proc.Apply(w.app.Metrics())

	// Aggregate pre/post medians from the sample series. Pre window sits
	// below the injected submit, post window above it.
	medianBetween := func(lo, hi int64) (float64, int) {
		var vals []float64
		samplesMu.Lock()
		defer samplesMu.Unlock()
		for _, s := range samples {
			if s.Presents >= lo && s.Presents <= hi {
				vals = append(vals, s.LiveMB)
			}
		}
		if len(vals) == 0 {
			return 0, 0
		}
		for i := 0; i < len(vals); i++ {
			for j := i + 1; j < len(vals); j++ {
				if vals[j] < vals[i] {
					vals[i], vals[j] = vals[j], vals[i]
				}
			}
		}
		return vals[len(vals)/2], len(vals)
	}
	preMB, preN := medianBetween(after-preLo, after-preHi)
	postMB, postN := medianBetween(after+postLo, after+postHi)
	presents := w.app.PresentCount()
	tolMB := preMB * 0.05
	if tolMB < 4 {
		tolMB = 4
	}
	verdict := oomVerdict{
		Spec: spec, After: after, Count: count,
		PreMB: preMB, PostMB: postMB,
		DeltaMB: postMB - preMB, ToleranceMB: tolMB,
		Recovered: preN >= 5 && postN >= 5 && abs(postMB-preMB) <= tolMB,
		Survived:  presents >= after+postHi,
	}

	// Golden over the single window (content below the HUD band).
	goldenMax := 0.0
	goldenFirst := false
	pixelGoldenValue := any(nil)
	if gateMode {
		base := filepath.Join(snapDir, "vram_oom_probe_base.png")
		diff, _, first := evaluateGolden(w.path, base,
			[]rect{{x: 0, y: hudH, w: winW, h: winH - hudH}}, winW)
		goldenMax, goldenFirst = diff, first
		pixelGoldenValue = goldenMax
	}

	extra := map[string]any{
		"mode":                   "oom_recovery_probe",
		"oom":                     verdict,
		"presents_total":         presents,
		"events_total":           eventsTotal.Load(),
		"timed_out":              timedOut,
		"crash_free":             !timedOut,
		"vram_budget_mb":         float64(render.VramBudgetMB()),
		"ledger_peak_mb":         float64(render.VramPeakBytes()) / (1024 * 1024),
		"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
		"pixel_golden_diff_pct":  pixelGoldenValue,
		"pixel_golden_max_pct":   goldenMax,
		"pixel_golden_first_run": goldenFirst,
		"golden_mask":            "card content below the HUD band",
		"golden_tolerance_pct":   0,
	}
	if !gateMode {
		extra["pixel_golden_diff_pct"] = nil
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "base_vram_cap_oom",
		Scenario:      "ui_base_vram_cap -oom-probe",
		Backend:       w.win.Backend().String(),
		Snap:          w.app.Metrics().Snapshot(),
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
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
	if gateMode {
		if timedOut {
			fail("gate timed out")
		}
		if presents < after+postHi {
			fail("presents=%d want >=%d (run must outlive the injected submit by the recovery window)", presents, after+postHi)
		}
		if preN < 5 || postN < 5 {
			fail("ledger samples pre=%d post=%d want >=5 each (sampler starved?)", preN, postN)
		}
		if !verdict.Survived {
			fail("app did not survive the single injected OOM (presents=%d)", presents)
		}
		if !verdict.Recovered {
			fail("ledger did not return to pre-injection level: pre=%.2fMB post=%.2fMB delta=%.2fMB tol=%.2fMB (资源谁建谁放)", preMB, postMB, verdict.DeltaMB, tolMB)
		}
		if !goldenFirst && goldenMax != 0 {
			fail("golden max=%.4f%% want 0", goldenMax)
		}
	}
	status := "OK"
	if gateMode && goldenFirst {
		status = "SEEDED"
	}
	fmt.Fprintf(os.Stderr, "ui_base_vram_cap oom-probe: %s spec=%s presents=%d pre=%.2fMB post=%.2fMB delta=%.2fMB recovered=%v survived=%v mode=%s elapsed=%.1fs\n",
		status, spec, presents, preMB, postMB, verdict.DeltaMB, verdict.Recovered, verdict.Survived,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
