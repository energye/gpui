// Command ui_base_idle is the base resource matrix row 1 real-window:
// idle (no dirty, zero frames). A static content window that is opened,
// allowed to paint once (warmup), then left alone — the framework must
// present zero additional frames and burn ~0 CPU for the remainder.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=60 go run ./examples/ui_base_idle -auto-only
//	go run ./examples/ui_base_idle                 # manual: idle until close
//
// Window: 1200×800. RUN_SECONDS>=60 (long enough to prove sustained idle).
//
// Evidence: presents_after_warmup ≤ 2, cpu_pct ≤ 2% (60s steady window),
// vsync_source present, no cpu_fallback. Golden over header+legend+body
// (static content, bitwise).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: idle 60s, snapshot + evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "BASE_IDLE")
	} else if *autoOnly {
		secs, secsSet = 60, true
	}
	gateMode := secsSet
	wrkit.EnsureUIFace()

	var snapshotPath string
	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui ui_base_idle — 空闲零帧", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open:", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "BASE_IDLE 空闲零帧 — 开窗后不碰", []string{
		"静态内容开窗后不再变化",
		"retained 模式：无脏 → 无帧",
		"量门：presents_after_warmup ≤ 2",
		"量门：CPU ≤ 2%（60s 稳态）",
		"对比 Flutter 静止页",
	})

	snapDir := filepath.Join("examples", "ui_base_idle", "testdata")
	if gateMode {
		os.MkdirAll(snapDir, 0o755)
		snapshotPath = filepath.Join(snapDir, "showcase_idle.png")
	}
	runFor := time.Duration(0)
	if gateMode {
		runFor = time.Duration(secs) * time.Second
	} else if *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapshotPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_base_idle: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	// No ticker: the idle window must NOT call ScheduleFrame after warmup.
	// The on-demand scheduler only renders when ScheduleFrame is called —
	// a static window with no ticker naturally stops presenting.

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()

	// Boundary re-record delta probe (dirty-region P0): the idle window must
	// not re-record ANY layer key between a mid-run sample and the end, even
	// if a stray present slipped past the presents_after_warmup ≤ 2 gate (a
	// present without re-record would pass that gate but still burn GPU).
	// Sampling runs on a plain timer goroutine (mutex-consistent snapshot
	// reads, same posture as the RSS sampler), never drives frames, and hands
	// the sample over a channel — the main thread reads it after Run returns.
	type idleProbeSample struct {
		keys             map[uint64]map[string]int64
		boundaryRerecord int64
	}
	probeCh := make(chan idleProbeSample, 1)
	time.AfterFunc(10*time.Second, func() {
		probeCh <- idleProbeSample{
			keys:             app.PictureTextures().RerecordByKeySnapshot(),
			boundaryRerecord: app.Metrics().Snapshot().BoundaryRerecord,
		}
	})

	// RSS-series sampler: measurement must not drive frames, so it runs on a
	// plain goroutine (reads /proc), never on a UI ticker. A 60s run is the
	// O3 screening window: rss_slope_valid=false is expected (INCONCLUSIVE),
	// so the gate requires RSS availability and no over-line verdict instead
	// of demanding a 120s verdict from a 60s run.
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
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	// Dirty-region P0 sample B: MUST be taken before app.Close() — Close
	// clears the texture-cache attribution map, so a post-Close read would be
	// empty and the delta==0 gate would pass vacuously.
	idleSampleB := app.PictureTextures().RerecordByKeySnapshot()
	idleMetricsB := app.Metrics().Snapshot().BoundaryRerecord
	close(sampleStop)
	<-sampleDone
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()

	// Core idle assertions: after warmup the framework must NOT keep painting.
	// Warm-up presents exactly one frame (presentSyncFull), so the idle count
	// is the total minus that one bootstrap present — the doc gate is on the
	// post-warmup increment, not on the bootstrap frame.
	presentsAfterWarmup := presents - 1
	if presentsAfterWarmup < 0 {
		presentsAfterWarmup = 0
	}
	presentsOK := presentsAfterWarmup <= 2
	cpuOK := snap.CPUPctAvg <= 2.0
	fallbackOK := snap.CPUFallbackOps == 0
	rssAvailable := snap.RSSStartKB != 0 || snap.RSSEndKB != 0
	rssOK := rssAvailable && (!snap.RSSSlopeValid || snap.RSSSlopeSteadyKBPerMin <= 30000)

	// Dirty-region P0: boundary re-record delta over the idle tail must be 0 —
	// no layer key gained re-records and the cumulative boundary counter did
	// not move. The probe must have engaged (warm-up fills the texture cache);
	// a disengaged probe is reported as a failure, not a silent pass.
	var probeSnapA map[uint64]map[string]int64
	var probeMetricsA int64
	probeFired := false
	select {
	case s := <-probeCh:
		probeSnapA, probeMetricsA, probeFired = s.keys, s.boundaryRerecord, true
	default:
	}
	probeEngaged := probeFired
	rerecordDeltaKeys := map[uint64]int64{}
	textureLegLive := len(probeSnapA) > 0
	if textureLegLive {
		for k, causes := range idleSampleB {
			prevCauses := probeSnapA[k]
			gained := int64(0)
			for cause, n := range causes {
				if d := n - prevCauses[cause]; d > 0 {
					gained += d
				}
			}
			if gained > 0 {
				rerecordDeltaKeys[k] = gained
			}
		}
	}
	boundaryRerecordDelta := idleMetricsB - probeMetricsA
	// Primary gate = the display-list leg (BoundaryCache cumulative delta):
	// the idle window's single warmup frame is a vector full present, so the
	// GPU texture cache never initializes (pictureTex stays nil — established
	// 2026-11-14) and the texture-leg key attribution is structurally
	// unavailable here; the key-level check applies only when that leg is live.
	rerecordDeltaOK := probeEngaged && boundaryRerecordDelta == 0 && len(rerecordDeltaKeys) == 0

	logicProbes := []string{"idle_presents_after_warmup_le_2", "cpu_idle_le_2pct", "cpu_fallback_zero", "rss_no_leak_or_inconclusive"}
	if gateMode {
		logicProbes = append(logicProbes, "boundary_rerecord_delta_zero")
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if !presentsOK {
		logicFailed = append(logicFailed, "idle_presents_after_warmup_le_2")
	}
	if gateMode && !rerecordDeltaOK {
		logicFailed = append(logicFailed, "boundary_rerecord_delta_zero")
	}
	if !cpuOK {
		logicFailed = append(logicFailed, "cpu_idle_le_2pct")
	}
	if !fallbackOK {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if !rssOK {
		logicFailed = append(logicFailed, "rss_no_leak_or_inconclusive")
	}
	logicPass := len(logicFailed) == 0

	var pixelOK, pixelTotal int
	pixelBad := make([]string, 0)
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	if gateMode {
		img := loadImage(snapshotPath)
		checks := []pixelCheck{
			{desc: "body_bg", x: 600, y: 400, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
			{desc: "legend_bg", x: 20, y: 300, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
			{desc: "top_bg", x: 600, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
		}
		inks := []inkRegion{
			// A page that only painted backgrounds would still match the three
			// bg probes; these regions prove the title/legend text is on screen.
			{desc: "title_ink", x: 14, y: 16, w: 260, h: 18,
				bg: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, minPx: 12},
			{desc: "legend_ink", x: 24, y: 84, w: 230, h: 18,
				bg: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, minPx: 8},
		}
		res := make(map[string]bool, len(checks)+len(inks))
		runPixelChecks(img, checks, res)
		runInkRegions(img, inks, res)
		for d, ok := range res {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, d)
			}
		}
		pixelTotal = len(checks) + len(inks)
		// HUD band (bottom 72px) is deliberately outside the static mask: it is
		// a meter, not tested content. Top + legend + body must be bitwise.
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir,
			[]rect{{x: 0, y: 0, w: winW, h: winH - 72}})
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		pixelGoldenValue = nil
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "base_idle",
		Scenario:      "ui_base_idle",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  presents,
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"presents_total":              presents,
			"presents_after_warmup":       presentsAfterWarmup,
			"warmup_presents":             presents - presentsAfterWarmup,
			"boundary_rerecord_total_end": snap.BoundaryRerecord,
			"boundary_rerecord_delta":     boundaryRerecordDelta,
			"rerecord_probe_engaged":      probeEngaged,
			"rerecord_texture_leg":        map[bool]string{true: "live", false: "unavailable_no_textured_present"}[textureLegLive],
			"rerecord_delta_keys":         rerecordDeltaKeys,
			"evidence_mode":               map[bool]string{true: "gate", false: "manual"}[gateMode],
			"pixel_evidence":              map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
			"logic_probes_ok":             len(logicProbes) - len(logicFailed),
			"logic_probes_total":          len(logicProbes),
			"logic_bad":                   logicFailed,
			"pixel_probes_ok":             pixelOK,
			"pixel_probes_total":          pixelTotal,
			"pixel_bad":                   pixelBad,
			"pixel_golden_diff_pct":       pixelGoldenValue,
			"pixel_golden_total_px":       goldenTotalPx,
			"pixel_golden_first_run":      goldenFirstRun,
			"snapshot_png":                snapshotPath,
			"golden_mask":                 "top+legend+body 1200x728 (HUD band excluded)",
			"golden_tolerance_pct":        0,
			"slope_gate":                  "off_60s_static",
			"rss_slope_steady_kb_per_min": snap.RSSSlopeSteadyKBPerMin,
			"rss_slope_valid":             snap.RSSSlopeValid,
			"rss_warmup_sec":              snap.RSSWarmupSec,
			"vsync_probe":                 "not_asserted_zero_frames_no_pacing",
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if !logicPass {
		fail("logic probes %d/%d bad=%v", len(logicProbes)-len(logicFailed), len(logicProbes), logicFailed)
	}
	if gateMode {
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
	}

	status := "OK"
	if !logicPass {
		status = "WARN"
	}
	if gateMode && goldenFirstRun {
		status = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr, "ui_base_idle: %s presents=%d idle_presents=%d cpu=%.1f%% logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		status, presents, presentsAfterWarmup, snap.CPUPctAvg, len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
