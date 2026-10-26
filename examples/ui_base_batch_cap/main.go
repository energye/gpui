// Command ui_base_batch_cap is matrix row 13: primitive/vector density ceiling.
//
// One full-window scene runs two ladders, 10s per rung:
//   - prims:  1k / 5k / 10k atlas quads (2.5D sprite base).
//   - vector: 100 / 300 / 600 curved paths, 24 cubic segments each
//     (design-tool base; FillPath re-tessellation is the cost).
//
// The window reports fps/p95/CPU per rung, "60fps max N*" per mode, and the
// process resource account. Three evidence: logical curve probes, pixel
// probes, bitwise Golden for both mode snapshots.
//
// Gate mode runs one unmeasured prewarm ladder first (raising every per-density
// GPU pool/heap high-water mark), then the measured ladder:
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=200 go run ./examples/ui_base_batch_cap -auto-only
//	go run ./examples/ui_base_batch_cap                     # manual: prewarm + repeat ladders
//	go run ./examples/ui_base_batch_cap -manual-seconds 200 # manual: timed
//
// Window: 1200x800. Gate needs 200s (two prewarm ladders + 6 x 10s rungs).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	winW, winH = 1200, 800
	// stepSec is fixed by the matrix (each rung 10s).
	stepSec = 10.0
	// gateSeconds covers two unmeasured prewarm ladders plus 6 measured rungs
	// and settle/snapshot headroom.
	gateSeconds = 200
	// prewarmLadders raises every per-density GPU pool/heap high-water mark
	// before measurement. Two cycles are needed because the RSS ramp (arena +
	// glyph/SDF pools) spans more than one ladder; one cycle still leaves the
	// engine's 25%-of-run steady-slope window inside the ramp.
	prewarmLadders = 2
	// health line (matrix §4): 60fps-ish and no long tail.
	minStepFPS = 55.0
	maxStepP95 = 22.0
)

type stepResult struct {
	Mode        string  `json:"mode"`
	N           int     `json:"n"`
	Seconds     float64 `json:"seconds"`
	Frames      int64   `json:"frames"`
	FPS         float64 `json:"fps"`
	P95Ms       float64 `json:"p95_ms"`
	CPUPct      float64 `json:"cpu_pct"`
	CPUOK       bool    `json:"cpu_ok"`
	Pass60      bool    `json:"pass_60fps"`
	RSSStartMB  float64 `json:"rss_start_mb"`
	RSSEndMB    float64 `json:"rss_end_mb"`
	RSSGrowthMB float64 `json:"rss_growth_mb"`
}

// intervalCollector gathers per-present frame intervals for one rung.
type intervalCollector struct {
	mu sync.Mutex
	v  []float64
}

func (c *intervalCollector) Add(ms float64) {
	if c == nil || ms <= 0 {
		return
	}
	c.mu.Lock()
	c.v = append(c.v, ms)
	c.mu.Unlock()
}

func (c *intervalCollector) Take() []float64 {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	out := c.v
	c.v = nil
	c.mu.Unlock()
	return out
}

func pct95(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	tmp := append([]float64(nil), v...)
	sort.Float64s(tmp)
	return tmp[(len(tmp)-1)*95/100]
}

func sleepOrStop(d time.Duration, stop <-chan struct{}) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-stop:
		return false
	}
}

// waitPresents waits until present count reaches want.
func waitPresents(app *embedder.PipelineApp, want int64, timeout time.Duration, stop <-chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if app.PresentCount() >= want {
			return true
		}
		select {
		case <-stop:
			return false
		case <-time.After(5 * time.Millisecond):
		}
	}
	return false
}

// waitFrozenPaint waits until a paintBatch that observed frozen=true has
// completed after the given baseline. A paint already in flight when setFrozen
// runs does not count, so the retained packet cannot carry the animated phase.
func waitFrozenPaint(sc *scene, baseline uint64, timeout time.Duration, stop <-chan struct{}) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sc.frozenPaints.Load() > baseline {
			return true
		}
		select {
		case <-stop:
			return false
		case <-time.After(2 * time.Millisecond):
		}
	}
	return false
}

// measureRung records one 10s rung: wall fps from present deltas, p95 from the
// per-present interval samples, CPU from /proc jiffies deltas.
func measureRung(app *embedder.PipelineApp, sc *scene, col *intervalCollector, mode int32, n int, stop <-chan struct{}) (stepResult, bool) {
	name := "prims"
	if mode == modeVector {
		name = "vector"
	}
	sc.set(mode, n)
	sc.setRunning()
	app.ScheduleFrame()
	if !sleepOrStop(300*time.Millisecond, stop) { // settle at the new rung, excluded from timing
		return stepResult{}, false
	}
	col.Take()
	startP := app.PresentCount()
	cpu0, cpuOK0 := wrkit.ProcessCPUJiffies()
	rss0 := scheduler.ReadRSSKB()
	t0 := time.Now()
	if !sleepOrStop(time.Duration(stepSec*float64(time.Second)), stop) {
		return stepResult{}, false
	}
	wall := time.Since(t0).Seconds()
	if wall <= 0 {
		wall = stepSec
	}
	frames := app.PresentCount() - startP
	cpu1, cpuOK1 := wrkit.ProcessCPUJiffies()
	rss1 := scheduler.ReadRSSKB()
	res := stepResult{
		Mode: name, N: n, Seconds: wall, Frames: frames,
		CPUOK:       cpuOK0 && cpuOK1,
		RSSStartMB:  float64(rss0) / 1024,
		RSSEndMB:    float64(rss1) / 1024,
		RSSGrowthMB: float64(rss1-rss0) / 1024,
	}
	res.FPS = float64(frames) / wall
	res.P95Ms = pct95(col.Take())
	if res.CPUOK {
		res.CPUPct = (cpu1 - cpu0) / wrkit.ProcessCPUTicksPerSec() / wall * 100
	}
	res.Pass60 = res.FPS >= minStepFPS && res.P95Ms > 0 && res.P95Ms <= maxStepP95
	return res, true
}

func maxContiguous(steps []stepResult, mode string) int {
	max := 0
	for _, s := range steps {
		if s.Mode != mode {
			continue
		}
		if !s.Pass60 {
			break
		}
		max = s.N
	}
	return max
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: six 10s rungs, both snapshots, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "BASE_BATCH_CAP")
	} else if *autoOnly {
		secs, secsSet = gateSeconds, true
	}
	gateMode := secsSet
	if gateMode && secs < gateSeconds {
		fmt.Fprintf(os.Stderr, "FAIL: BASE_BATCH_CAP gate needs RUN_SECONDS>=%d (one prewarm ladder + 6 x 10s rungs)\n", gateSeconds)
		os.Exit(1)
	}
	wrkit.EnsureUIFace()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_base_batch_cap — 图元/矢量密度上限", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	sc := newScene()
	collector := &intervalCollector{}

	snapDir := filepath.Join("examples", "ui_base_batch_cap", "testdata")
	primsSnap := filepath.Join(snapDir, "batch_cap_prims.png")
	vectorSnap := filepath.Join(snapDir, "batch_cap_vector.png")
	primsBase := filepath.Join(snapDir, "batch_cap_prims_base.png")
	vectorBase := filepath.Join(snapDir, "batch_cap_vector_base.png")
	if gateMode {
		os.MkdirAll(snapDir, 0o755)
	} else {
		primsSnap, vectorSnap = "", ""
	}

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.055, G: 0.065, B: 0.085, A: 1}
	fullBox := rendering.NewRenderBox()
	fullBox.FixedWidth, fullBox.FixedHeight = winW, winH
	var app *embedder.PipelineApp
	fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		fps := 0.0
		var presents int64
		if app != nil {
			if m := app.Metrics().Snapshot(); m.AvgFrameIntervalMs > 1e-6 {
				fps = 1000.0 / m.AvgFrameIntervalMs
			}
			presents = app.PresentCount()
		}
		sc.paintBatch(pc, size, fps, presents)
	}
	root.Place(fullBox, 0, 0)

	eventsTotal := 0
	bumpTitle := func(state string) {
		if ctl != nil {
			ctl.SetTitle(fmt.Sprintf("ui_base_batch_cap %s events=%d", state, eventsTotal))
		}
	}
	bumpTitle("ready")

	runFor := time.Duration(0)
	if gateMode {
		runFor = time.Duration(secs) * time.Second
	} else if *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	app = embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: 0.055, ClearG: 0.065, ClearB: 0.085, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: vectorSnap,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_base_batch_cap: close (%s)\n", win.Backend())
			case platform.EventPointer:
				eventsTotal++
				fmt.Fprintf(os.Stderr, "ui_base_batch_cap event pointer (%d) @%.0f,%.0f\n", eventsTotal, ev.X, ev.Y)
				bumpTitle("manual")
			case platform.EventKey:
				eventsTotal++
				fmt.Fprintf(os.Stderr, "ui_base_batch_cap event key (%d) code=%d pressed=%v\n", eventsTotal, ev.KeyCode, ev.Pressed)
				bumpTitle("manual")
			case platform.EventResize:
				eventsTotal++
				fmt.Fprintf(os.Stderr, "ui_base_batch_cap event resize (%d) %dx%d\n", eventsTotal, ev.Width, ev.Height)
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					fullBox.FixedWidth, fullBox.FixedHeight = float64(ev.Width), float64(ev.Height)
					fullBox.MarkNeedsPaint()
				}
				bumpTitle("manual")
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	app.Scheduler().Tickers().Add(&ticker{app: app, sc: sc, box: fullBox, col: collector, proc: &proc})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}

	var steps []stepResult
	var warmSteps []stepResult
	var stepsMu sync.Mutex
	stop := make(chan struct{})
	ctrlDone := make(chan struct{})
	snapOK := true
	var snapMu sync.Mutex
	go func() {
		defer close(ctrlDone)
		if !waitPresents(app, 1, 10*time.Second, stop) {
			return
		}
		rungs := []struct {
			mode int32
			n    int
		}{
			{modePrims, primSteps[0]}, {modePrims, primSteps[1]}, {modePrims, primSteps[2]},
			{modeVector, vectorSteps[0]}, {modeVector, vectorSteps[1]}, {modeVector, vectorSteps[2]},
		}
		// runLadder walks the six rungs. capturePrims takes the retained
		// prims snapshot at the prims→vector boundary (measured cycle only).
		runLadder := func(capturePrims bool) ([]stepResult, bool) {
			out := make([]stepResult, 0, len(rungs))
			for i, rung := range rungs {
				res, ok := measureRung(app, sc, collector, rung.mode, rung.n, stop)
				if !ok {
					return nil, false
				}
				out = append(out, res)
				// At the prims→vector boundary: freeze and capture the prims
				// packet before the vector ladder overwrites it.
				if capturePrims && i == 2 {
					base := sc.frozenPaints.Load()
					sc.set(modePrims, primSteps[2])
					sc.setFrozen()
					app.ScheduleFrame()
					// First drain frames that started before the freeze. Then
					// drop the boundary/picture caches, so the retained replay
					// cannot pick up a layer recorded with a pre-freeze phase.
					if waitFrozenPaint(sc, base, 3*time.Second, stop) &&
						waitPresents(app, app.PresentCount()+2, 3*time.Second, stop) {
						base = sc.frozenPaints.Load()
						app.InvalidateBoundaryCache()
						app.ScheduleFrame()
						if waitFrozenPaint(sc, base, 3*time.Second, stop) &&
							waitPresents(app, app.PresentCount()+2, 3*time.Second, stop) {
							ok := app.RetainedSnapshot(primsSnap)
							snapMu.Lock()
							snapOK = snapOK && ok
							snapMu.Unlock()
						}
					}
				}
			}
			return out, true
		}

		// Unmeasured ladders first: they raise every per-density GPU pool/heap
		// high-water mark and compile the pipelines, so the measured cycle is
		// steady state. Without this the 60s RSS slope is really the arena ramp
		// (measured ~45MB/min at 60s vs ~3MB/min once warm).
		sc.setPrewarm(true)
		bumpTitle("prewarm")
		var warm []stepResult
		for i := 0; i < prewarmLadders; i++ {
			w, ok := runLadder(false)
			if !ok {
				return
			}
			warm = append(warm, w...)
		}
		sc.setPrewarm(false)
		stepsMu.Lock()
		warmSteps = warm
		stepsMu.Unlock()
		bumpTitle("measuring")

		for {
			got, ok := runLadder(gateMode)
			if !ok {
				return
			}
			stepsMu.Lock()
			steps = got
			stepsMu.Unlock()
			if gateMode {
				// Final vector snapshot: same freeze barrier as the prims
				// capture. Wait for a paint after setFrozen, then for its
				// present to land before quitting.
				base := sc.frozenPaints.Load()
				sc.set(modeVector, vectorSteps[2])
				sc.setFrozen()
				app.ScheduleFrame()
				// Drain pre-freeze frames first, then clear the retained
				// boundary/picture caches. SnapshotPath repaints the live tree;
				// clearing here guarantees that repaint re-records every layer
				// at phase 0 instead of reusing one captured before the freeze.
				if waitFrozenPaint(sc, base, 3*time.Second, stop) &&
					waitPresents(app, app.PresentCount()+2, 3*time.Second, stop) {
					base = sc.frozenPaints.Load()
					app.InvalidateBoundaryCache()
					app.ScheduleFrame()
					_ = waitFrozenPaint(sc, base, 3*time.Second, stop)
					_ = waitPresents(app, app.PresentCount()+2, 3*time.Second, stop)
				}
				bumpTitle("measured")
				app.Quit()
				return
			}
			// Manual resident mode: repeat the ladders until close.
			sc.setRunning()
		}
	}()

	t0 := time.Now()
	runErr := app.Run()
	close(stop)
	<-ctrlDone
	elapsed := time.Since(t0).Seconds()
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	if runErr != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", runErr)
		os.Exit(1)
	}

	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()

	stepsMu.Lock()
	gotSteps := append([]stepResult{}, steps...)
	gotWarm := append([]stepResult{}, warmSteps...)
	stepsMu.Unlock()
	maxPrims := maxContiguous(gotSteps, "prims")
	maxVector := maxContiguous(gotSteps, "vector")
	prewarmSec := 0.0
	for _, s := range gotWarm {
		prewarmSec += s.Seconds
	}

	// Pixel + Golden evidence for both mode snapshots (gate only).
	var pixelOK, pixelTotal int
	pixelBad := make([]string, 0)
	var primsGolden, vectorGolden float64
	var primsTotal, vectorTotal int64
	var primsFirst, vectorFirst bool
	pixelGoldenValue := any(nil)
	if gateMode {
		bg := [3]float64{0.055, 0.065, 0.085}
		checksP := []pixelCheck{
			{desc: "prims_bg", x: 5, y: 790, want: bg, tol: 0.03, kind: kindBG},
		}
		inksP := []inkRegion{
			{desc: "prims_ink", x: 60, y: 80, w: 1080, h: 640, bg: bg, tol: 0.04, minPx: 4000},
		}
		checksV := []pixelCheck{
			{desc: "vector_bg", x: 5, y: 790, want: bg, tol: 0.03, kind: kindBG},
		}
		inksV := []inkRegion{
			{desc: "vector_ink", x: 60, y: 80, w: 1080, h: 640, bg: bg, tol: 0.04, minPx: 1500},
		}
		res := make(map[string]bool)
		for _, img := range []struct {
			path   string
			checks []pixelCheck
			inks   []inkRegion
		}{
			{primsSnap, checksP, inksP},
			{vectorSnap, checksV, inksV},
		} {
			im := loadImage(img.path)
			runPixelChecks(im, img.checks, res)
			runInkRegions(im, img.inks, res)
		}
		pixelTotal = len(checksP) + len(inksP) + len(checksV) + len(inksV)
		for d, ok := range res {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, d)
			}
		}
		mask := []rect{{x: 0, y: contentTop, w: winW, h: winH - contentTop - 24}}
		primsGolden, primsTotal, primsFirst = evaluateGolden(primsSnap, primsBase, mask)
		vectorGolden, vectorTotal, vectorFirst = evaluateGolden(vectorSnap, vectorBase, mask)
		pixelGoldenValue = math.Max(primsGolden, vectorGolden)
	}

	extra := map[string]any{
		"steps":                     gotSteps,
		"prims_ladder":              primSteps,
		"vector_ladder":             vectorSteps,
		"vector_segments":           vectorSegments,
		"max_prims_n_60fps":         maxPrims,
		"max_vector_paths_60":       maxVector,
		"prims_ceiling_reached":     maxPrims == primSteps[len(primSteps)-1],
		"vector_ceiling_reached":    maxVector == vectorSteps[len(vectorSteps)-1],
		"step_sec":                  stepSec,
		"prewarm_steps":             gotWarm,
		"prewarm_sec":               prewarmSec,
		"prewarm_note":              "two unmeasured ladders raise GPU pool/heap high-water marks; measured cycle is steady state",
		"rss_slope_scope":           "whole run after max(25% elapsed, 15s) warmup; prewarm cycle included",
		"rss_slope_line_kb_per_min": 30000,
		"hitch_scope":               "ladder includes overload rungs; per-rung pass_60fps is the capacity gate",
		"health_line_fps":           minStepFPS,
		"health_line_p95_ms":        maxStepP95,
		"evidence_mode":             map[bool]string{true: "gate", false: "manual"}[gateMode],
		"pixel_probes_ok":           pixelOK,
		"pixel_probes_total":        pixelTotal,
		"pixel_bad":                 pixelBad,
		"pixel_golden_diff_pct":     pixelGoldenValue,
		"pixel_prims_golden_pct":    primsGolden,
		"pixel_vector_golden_pct":   vectorGolden,
		"pixel_prims_total_px":      primsTotal,
		"pixel_vector_total_px":     vectorTotal,
		"pixel_golden_first_run":    primsFirst || vectorFirst,
		"snapshot_prims":            primsSnap,
		"snapshot_vector":           vectorSnap,
		"golden_mask":               "content 1200x736 (metric strip excluded)",
		"golden_tolerance_pct":      0,
		"events_total":              eventsTotal,
		"rss_slope_valid":           snap.RSSSlopeValid,
	}
	if !gateMode {
		extra["pixel_golden_diff_pct"] = nil
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "base_batch_cap",
		Scenario:      "ui_base_batch_cap",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
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
	// Logical evidence.
	if presents < 1 {
		fail("no presents")
	}
	if gateMode {
		if len(gotSteps) != 6 {
			fail("rungs=%d want 6 (truncated run?)", len(gotSteps))
		}
		if maxPrims < primSteps[0] {
			fail("prims floor failed: max_60fps_n=%d want >=%d (steps=%s)", maxPrims, primSteps[0], summarizeSteps(gotSteps))
		}
		if maxVector < vectorSteps[0] {
			fail("vector floor failed: max_60fps_paths=%d want >=%d (steps=%s)", maxVector, vectorSteps[0], summarizeSteps(gotSteps))
		}
		for _, s := range gotSteps {
			if !s.CPUOK {
				fail("cpu measurement unavailable at %s/%d (health line needs /proc)", s.Mode, s.N)
			}
			if s.Pass60 && s.CPUPct > 85 {
				fail("%s/%d meets 60fps but cpu=%.1f%% > 85%% health line", s.Mode, s.N, s.CPUPct)
			}
		}
		if !snap.RSSSlopeValid || snap.RSSSlopeSteadyKBPerMin > 30000 {
			fail("rss_slope_steady=%.0fKB/min valid=%v want <=30000KB/min (health line)", snap.RSSSlopeSteadyKBPerMin, snap.RSSSlopeValid)
		}
		if snap.CPUPctAvg > 85 {
			fail("process cpu=%.1f%% > 85%% health line", snap.CPUPctAvg)
		}
		if snap.MissedVSync > 0 || snap.CPUFallbackOps > 0 {
			fail("missed_vsync=%d cpu_fallback_ops=%d want 0", snap.MissedVSync, snap.CPUFallbackOps)
		}
		snapMu.Lock()
		okSnaps := snapOK
		snapMu.Unlock()
		if !okSnaps {
			fail("prims retained snapshot failed")
		}
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !(primsFirst || vectorFirst) && (primsGolden != 0 || vectorGolden != 0) {
			fail("golden prims=%.4f%% vector=%.4f%% want 0", primsGolden, vectorGolden)
		}
	}

	status := "OK"
	if gateMode && (primsFirst || vectorFirst) {
		status = "SEEDED"
	}
	fmt.Fprintf(os.Stderr, "ui_base_batch_cap: %s presents=%d prims_max=%d vector_max=%d cpu=%.1f%% steps=%d px=%d/%d golden=%.4f%%/%.4f%% mode=%s elapsed=%.1fs\n",
		status, presents, maxPrims, maxVector, snap.CPUPctAvg, len(gotSteps),
		pixelOK, pixelTotal, primsGolden, vectorGolden,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
	for _, s := range gotSteps {
		fmt.Fprintf(os.Stderr, "  rung %-6s n=%5d fps=%.1f p95=%.1fms cpu=%.1f%% pass=%v\n",
			s.Mode, s.N, s.FPS, s.P95Ms, s.CPUPct, s.Pass60)
	}
}

func summarizeSteps(steps []stepResult) string {
	var b strings.Builder
	for i, s := range steps {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s/%d:%.0ffps:%.0fms", s.Mode, s.N, s.FPS, s.P95Ms)
	}
	return b.String()
}

type ticker struct {
	app  *embedder.PipelineApp
	sc   *scene
	box  *rendering.RenderBox
	col  *intervalCollector
	proc *scheduler.ProcessTracker
}

func (t *ticker) Tick(dt float64) bool {
	if t == nil || t.app == nil {
		return false
	}
	t.sc.advance(dt)
	t.box.MarkNeedsPaint()
	t.app.ScheduleFrame()
	if t.proc != nil {
		t.proc.Sample()
	}
	if m := t.app.Metrics().Snapshot(); m.LastFrameIntervalMs > 0 {
		t.col.Add(m.LastFrameIntervalMs)
	}
	return true
}
