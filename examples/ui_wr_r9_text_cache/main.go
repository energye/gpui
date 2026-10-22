// Command ui_wr_r9_text_cache is the W1 R9 real-window: text measure cache.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache -auto-only
//	go run ./examples/ui_wr_r9_text_cache -manual-seconds 15
//
// Window: 1200x800. R9 close duration is 5s; a gate run below 5s is rejected.
//
// Evidence: gate mode freezes Steady, forces a layout every tick, and resets
// the per-text counters only after the first layout has warmed every width.
// From that point on the same text/style must hit (miss stays 0) and every
// text node must keep its size. Manual mode runs Steady -> Spike -> Recover;
// Spike changes the two hot strings every tick (cold misses) while every
// static text keeps hitting, and Recover freezes the hot strings and stops
// the miss delta. Pixel probes and a zero-tolerance Golden baseline cover the
// rendered scene; manual mode never writes the gate snapshot.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	rtext "github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	closeSecs  = 5

	// Golden mask: header + legend + body. HUD is intentionally excluded
	// because its counters refresh during the run.
	goldenMaskPx = 1200*48 + 260*656 + 904*656

	styleX, styleY, styleW, styleH     = 20.0, 20.0, 520.0, 210.0
	paraX, paraY, paraW, paraH         = 20.0, 245.0, 520.0, 220.0
	denseX, denseY, denseW, denseH     = 560.0, 20.0, 324.0, 210.0
	statusX, statusY, statusW, statusH = 560.0, 245.0, 324.0, 220.0
	footerX, footerY, footerW, footerH = 20.0, 480.0, 520.0, 150.0
	hotX, hotY, hotW, hotH             = 560.0, 480.0, 324.0, 150.0
)

var (
	topBG    = [3]float64{0.12, 0.14, 0.18}
	legendBG = [3]float64{0.11, 0.12, 0.15}
	bodyBG   = [3]float64{0.10, 0.11, 0.13}
	styleBG  = [3]float64{0.12, 0.15, 0.19}
	paraBG   = [3]float64{0.13, 0.14, 0.18}
	denseBG  = [3]float64{0.14, 0.16, 0.20}
	statusBG = [3]float64{0.11, 0.14, 0.17}
	footerBG = [3]float64{0.12, 0.13, 0.17}
	hotBG    = [3]float64{0.15, 0.16, 0.20}

	chipColors = [][3]float64{
		{0.85, 0.25, 0.25},
		{0.25, 0.75, 0.45},
		{0.25, 0.45, 0.85},
		{0.90, 0.75, 0.20},
	}
)

type textSpec struct {
	text    string
	size    float64
	r, g, b float64
	weight  rtext.FontWeight
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R9")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R9 (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintf(os.Stderr, "wrkit: font: %v\n", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_wr_r9_text_cache — text measure cache",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R9 text measure cache — forced relayout, hits>=miss", []string{
		"style matrix: 8 text styles",
		"wrapped paragraphs: many lines",
		"dense labels: small sizes/colors",
		"every tick MarkNeedsLayout",
		"gate: hit grows, miss stays 0",
		"gate: each text size stable",
		"manual Spike: hot text changes",
		"manual Recover: miss stops",
	})
	shell.Top.LabelAt("forced relayout every tick", 12, 560, 14, 0.72, 0.82, 0.94)

	contentTexts := make([]*rendering.RenderText, 0, 32)
	addText := func(p *wrkit.Panel, s string, size, x, y, r, g, b float64, weight rtext.FontWeight, maxW float64) *rendering.RenderText {
		t := p.LabelAt(s, size, x, y, r, g, b)
		if maxW > 0 {
			t.SetMaxWidth(maxW)
		}
		if weight != 0 {
			t.SetFontWeight(weight)
		}
		contentTexts = append(contentTexts, t)
		return t
	}
	mkPanel := func(w, h float64, bg [3]float64, name string) *wrkit.Panel {
		p := wrkit.NewPanel(w, h, bg[0], bg[1], bg[2], 1)
		p.Box.SetDebugName(name)
		return p
	}

	// STYLE MATRIX: same strings, different sizes/weights/colours. These are
	// the primary cache-hit producers when layout is forced repeatedly.
	stylePanel := mkPanel(styleW, styleH, styleBG, "style_panel")
	stylePanel.PlaceOn(shell.Body.Box, styleX, styleY)
	stylePanel.LabelAt("STYLE MATRIX — 8 styles, same text", 13, 12, 8, 0.82, 0.90, 0.98)
	// Labels stop at the weight grade: real Bold cuts have wider advances than
	// the old synthetic mask dilation, and the trailing "measure" made the
	// 20px row collide with the second column.
	styleSpecs := []textSpec{
		{"12px regular", 12, 0.88, 0.92, 0.98, rtext.WeightRegular},
		{"13px medium", 13, 0.55, 0.86, 0.66, rtext.WeightMedium},
		{"14px semibold", 14, 0.80, 0.68, 0.96, rtext.WeightSemiBold},
		{"15px bold", 15, 0.96, 0.77, 0.48, rtext.WeightBold},
		{"16px regular", 16, 0.58, 0.78, 0.98, rtext.WeightRegular},
		{"18px medium", 18, 0.94, 0.68, 0.78, rtext.WeightMedium},
		{"20px semibold", 20, 0.92, 0.84, 0.52, rtext.WeightSemiBold},
		{"22px bold", 22, 0.56, 0.90, 0.86, rtext.WeightBold},
	}
	for i, spec := range styleSpecs {
		addText(stylePanel, spec.text, spec.size, 12+float64(i%2)*260, 40+float64(i/2)*38,
			spec.r, spec.g, spec.b, spec.weight, 0)
	}

	// WRAPPED PARAGRAPHS: each forced layout walks several lines through
	// measureLine; the cache must return the same widths, so wrapping cannot
	// drift between frames.
	paraPanel := mkPanel(paraW, paraH, paraBG, "paragraph_panel")
	paraPanel.PlaceOn(shell.Body.Box, paraX, paraY)
	paraPanel.LabelAt("WRAPPED PARAGRAPHS — repeated measureLine calls", 13, 12, 8, 0.82, 0.90, 0.98)
	addText(paraPanel,
		"measure cache paragraph one: the same words are measured again on every forced layout, so repeated width lookups should hit the cache while the text remains unchanged.",
		12, 12, 38, 0.76, 0.86, 0.98, 0, 240)
	addText(paraPanel,
		"second paragraph at a different width: a different set of line breaks exercises more measureLine keys and makes the hit/miss split visible in the HUD counters.",
		12, 268, 38, 0.78, 0.90, 0.82, 0, 232)
	paraPanel.LabelAt("gate: hit grows, miss stays 0; manual Spike changes only hot text", 11, 12, 164, 0.66, 0.74, 0.86)

	// DENSE LABELS: many small labels with different sizes/colours. This is
	// the static dense content, not a two-colour-box scene.
	densePanel := mkPanel(denseW, denseH, denseBG, "dense_panel")
	densePanel.PlaceOn(shell.Body.Box, denseX, denseY)
	densePanel.LabelAt("DENSE LABELS — static small text", 13, 12, 8, 0.82, 0.90, 0.98)
	for i := 0; i < 12; i++ {
		size := 9.0 + float64(i%3)
		col := 0.54 + 0.07*float64(i%4)
		addText(densePanel, fmt.Sprintf("dense %02d  %.0fpx", i, size), size,
			12+float64(i%2)*158, 40+float64(i/2)*25,
			col, 0.78, 0.92, 0, 0)
	}

	// STATUS: the implementation contract, visible to the operator.
	statusPanel := mkPanel(statusW, statusH, statusBG, "status_panel")
	statusPanel.PlaceOn(shell.Body.Box, statusX, statusY)
	statusPanel.LabelAt("MEASURE CACHE CONTRACT", 13, 12, 8, 0.82, 0.90, 0.98)
	statusLines := []string{
		"cache key = text + size + spacing + weight",
		"same key -> hit; changed text -> cold miss",
		"forced layout every tick exercises the cache",
		"stale width must never move a line",
		"Spike changes only the two hot strings",
		"Recover freezes them and misses stop",
	}
	for i, line := range statusLines {
		statusPanel.LabelAt(line, 11, 12, 38+float64(i)*24, 0.68, 0.78, 0.90)
	}

	// FOOTER: four exact colour chips give simple, high-signal pixel probes
	// alongside the text ink probes.
	footerPanel := mkPanel(footerW, footerH, footerBG, "footer_panel")
	footerPanel.PlaceOn(shell.Body.Box, footerX, footerY)
	footerPanel.LabelAt("CACHE KEY / STYLE CHIPS", 13, 12, 8, 0.82, 0.90, 0.98)
	footerPanel.LabelAt("same text/style => stable width", 11, 12, 36, 0.68, 0.78, 0.90)
	footerPanel.LabelAt("changed text => one cold miss", 11, 12, 58, 0.68, 0.78, 0.90)
	for i, c := range chipColors {
		footerPanel.ColorAt(110, 32, 12+float64(i)*125, 90, c[0], c[1], c[2], 1, false)
	}

	// HOT TEXT: manual-only dynamic content. Gate keeps both strings fixed;
	// Steady pulses only the colour (paint dirty, no measure miss); Spike
	// changes the strings every tick; Recover installs a final string once.
	hotPanel := mkPanel(hotW, hotH, hotBG, "hot_panel")
	hotPanel.PlaceOn(shell.Body.Box, hotX, hotY)
	hotPanel.LabelAt("HOT TEXT — manual Spike changes content", 13, 12, 8, 0.82, 0.90, 0.98)
	hotA := addText(hotPanel, "hotA: stable text (cache hit)", 16, 12, 38, 0.96, 0.72, 0.46, 0, 290)
	hotB := addText(hotPanel, "hotB: stable text (cache hit)", 14, 12, 76, 0.58, 0.90, 0.68, 0, 290)
	hotPanel.LabelAt("Steady: colour pulse; Spike: content; Recover: stable", 10, 12, 114, 0.66, 0.74, 0.86)

	allTexts := make([]*rendering.RenderText, 0, len(contentTexts)+16)
	collectTexts(shell.Root, &allTexts)
	hotSet := map[*rendering.RenderText]bool{hotA: true, hotB: true}
	realWeightOK, realWeightChecked, realWeightFaces, realWeightBad := weightFaceProof(allTexts)

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r9_text_cache", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapName := "showcase_r9_text_cache.png"
	baseName := "showcase_r9_text_cache_base.png"
	snapPath := filepath.Join(snapDir, snapName)
	snapshotPath := snapPath
	if !gateMode {
		// Manual mode animates hot strings, so its frame is not evidence.
		// Keep the gate snapshot path empty: a manual run must not overwrite
		// the committed gate PNG or the Golden baseline seeded from it.
		snapshotPath = ""
	}

	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
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
				fmt.Fprintf(os.Stderr, "ui_wr_r9_text_cache: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W1 correctness window: pin FullPaint explicitly. The capability is the
	// measure cache, not retained damage bookkeeping.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{wrkit.PhaseSteady: true}
	clock := wrkit.NewPhaseClock(2, 5)
	lastPhase := ""
	tickCount := int64(0)

	// Counters are reset only after every text has been laid out once. Gate
	// then requires miss==0; manual attributes hit/miss deltas to phases.
	const statsResetTick = 20
	statsReset := false
	baselineSizes := make(map[*rendering.RenderText]rendering.Size, len(allTexts))
	sizeBad := make([]string, 0)
	lastHits, lastMiss := int64(0), int64(0)
	hitsSteady, hitsSpike, hitsRecover := int64(0), int64(0), int64(0)
	missSteady, missSpike, missRecover := int64(0), int64(0), int64(0)
	recoverTicks := 0
	recoverSettled := false
	recoverMissAtSettle := int64(0)
	lastLiveSnap := scheduler.FrameMetrics{}

	acc := func() (int64, int64) {
		h, m := sumTextStats(allTexts)
		if statsReset {
			if h >= lastHits {
				d := h - lastHits
				if d > 0 {
					switch phase {
					case wrkit.PhaseSteady:
						hitsSteady += d
					case wrkit.PhaseSpike:
						hitsSpike += d
					default:
						hitsRecover += d
					}
				}
				lastHits = h
			}
			if m >= lastMiss {
				d := m - lastMiss
				if d > 0 {
					switch phase {
					case wrkit.PhaseSteady:
						missSteady += d
					case wrkit.PhaseSpike:
						missSpike += d
					default:
						missRecover += d
					}
				}
				lastMiss = m
			}
		}
		return h, m
	}

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		tickCount++
		if gateMode {
			phase = wrkit.PhaseSteady
			hotA.SetColor(0.96, 0.72, 0.46, 1)
			hotB.SetColor(0.58, 0.90, 0.68, 1)
		} else {
			phase = clock.Advance(dt)
			phaseChanged := phase != lastPhase
			switch phase {
			case wrkit.PhaseSteady:
				// Colour pulse only: paint dirty, measure cache stays warm.
				pulse := 0.5 + 0.5*math.Sin(clock.Elapsed()*3)
				hotA.SetColor(0.96, 0.58+0.30*pulse, 0.40, 1)
			case wrkit.PhaseSpike:
				if phaseChanged {
					hotA.SetColor(0.96, 0.60, 0.40, 1)
					hotB.SetColor(0.58, 0.92, 0.68, 1)
				}
				// Change both hot strings every tick. A once-per-second value
				// would let the same-footprint/content path hide the miss.
				hotA.SetText(fmt.Sprintf("hotA spike tick %d", tickCount))
				hotB.SetText(fmt.Sprintf("hotB spike tick %d", tickCount))
			default:
				if phaseChanged {
					hotA.SetText("hotA recovered: cache hits resume")
					hotB.SetText("hotB recovered: no more misses")
					hotA.SetColor(0.96, 0.72, 0.46, 1)
					hotB.SetColor(0.58, 0.90, 0.68, 1)
				}
			}
			lastPhase = phase
		}
		phaseSeen[phase] = true

		if !statsReset && tickCount >= statsResetTick {
			resetTextStats(allTexts)
			for _, t := range allTexts {
				baselineSizes[t] = t.Size()
			}
			statsReset = true
			lastHits, lastMiss = 0, 0
		}

		// Force relayout every tick. Unchanged text/styles must hit; only
		// the hot strings above may cold-miss.
		for _, t := range allTexts {
			t.MarkNeedsLayout()
		}

		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)

		snapH := app.Metrics().Snapshot()
		lastLiveSnap = snapH
		totalHits, totalMiss := acc()
		if !gateMode && phase == wrkit.PhaseRecover {
			recoverTicks++
			if recoverTicks == 60 {
				recoverSettled = true
				recoverMissAtSettle = totalMiss
			}
		}
		gateOK := totalHits > 0 && totalHits >= totalMiss
		shell.UpdateHUD("R9", phase, app, gateOK,
			fmt.Sprintf("hit=%d miss=%d", totalHits, totalMiss),
			fmt.Sprintf("texts=%d layout=%d", len(allTexts), snapH.LayoutCount))
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
	totalHits, totalMiss := acc()
	metricsHits, metricsMiss := snap.MeasureCacheHit, snap.MeasureCacheMiss
	// FrameBuildStats copies the same per-text counters into MetricsStore on
	// every built frame. The last scheduled forced layout can land after the
	// last present, so the snapshot may be one partial frame stale. Sync the
	// store from the authoritative tree counters before building the report;
	// keep the pre-sync values in ability_extra as an honest diagnostic.
	app.Metrics().SetMeasureCacheStats(totalHits, totalMiss)
	snap = app.Metrics().Snapshot()
	measureMetricsMatch := snap.MeasureCacheHit == totalHits && snap.MeasureCacheMiss == totalMiss

	// Classification is based on all RenderText nodes, including shell chrome
	// labels, because FrameBuildStats sums the whole tree.
	hotMiss := int64(0)
	for t := range hotSet {
		_, m := t.MeasureCacheStats()
		hotMiss += m
	}
	staticMiss := totalMiss - hotMiss
	for _, t := range allTexts {
		if !gateMode && hotSet[t] {
			continue
		}
		b, ok := baselineSizes[t]
		if !ok || b.Width <= 0 || b.Height <= 0 {
			sizeBad = append(sizeBad, fmt.Sprintf("%q no-baseline", t.Text))
			continue
		}
		now := t.Size()
		if now.Width != b.Width || now.Height != b.Height {
			sizeBad = append(sizeBad, fmt.Sprintf("%q %.3fx%.3f -> %.3fx%.3f", t.Text, b.Width, b.Height, now.Width, now.Height))
		}
	}
	sizeStable := statsReset && len(sizeBad) == 0

	distinctSizes := map[int]bool{}
	distinctWeights := map[int]bool{}
	distinctColors := map[string]bool{}
	wrappedNodes := 0
	for _, t := range allTexts {
		if t.MaxWidth > 0 {
			wrappedNodes++
		}
		distinctSizes[int(t.FontSize*100+0.5)] = true
		w := int(t.FontWeight)
		if w == 0 {
			w = int(rtext.WeightRegular)
		}
		distinctWeights[w] = true
		distinctColors[fmt.Sprintf("%.2f/%.2f/%.2f", t.R, t.G, t.B)] = true
	}

	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	liveFPSInterval := 0.0
	if lastLiveSnap.AvgFrameIntervalMs > 1e-6 {
		liveFPSInterval = 1000.0 / lastLiveSnap.AvgFrameIntervalMs
	}

	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "top_bg", x: 1000, y: 24, want: topBG, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: legendBG, tol: 0.05, kind: kindBG},
		{desc: "body_bg", x: bx + 890, y: by + 640, want: bodyBG, tol: 0.05, kind: kindBG},
		{desc: "style_bg", x: bx + styleX + 510, y: by + styleY + 200, want: styleBG, tol: 0.05, kind: kindBG},
		{desc: "para_bg", x: bx + paraX + 510, y: by + paraY + 205, want: paraBG, tol: 0.05, kind: kindBG},
		{desc: "dense_bg", x: bx + denseX + 315, y: by + denseY + 200, want: denseBG, tol: 0.05, kind: kindBG},
		{desc: "status_bg", x: bx + statusX + 315, y: by + statusY + 205, want: statusBG, tol: 0.05, kind: kindBG},
		{desc: "footer_bg", x: bx + footerX + 510, y: by + footerY + 140, want: footerBG, tol: 0.05, kind: kindBG},
		{desc: "hot_bg", x: bx + hotX + 315, y: by + hotY + 140, want: hotBG, tol: 0.05, kind: kindBG},
	}
	for i, c := range chipColors {
		pixelChecks = append(pixelChecks, pixelCheck{
			desc: fmt.Sprintf("chip_%d", i),
			x:    bx + footerX + 12 + float64(i)*125 + 55,
			y:    by + footerY + 90 + 16,
			want: c, tol: 0.06,
		})
	}
	inkRegions := []inkRegion{
		{desc: "header_title_ink", x: 16, y: 14, w: 520, h: 18, bg: topBG, tol: 0.04, minPx: 40, fg: [3]float64{0.88, 0.92, 0.98}, minFG: 20},
		{desc: "legend_line0_ink", x: shell.Legend.X + 12, y: shell.Legend.Y + 36, w: 230, h: 14, bg: legendBG, tol: 0.04, minPx: 20, fg: [3]float64{0.70, 0.78, 0.88}, minFG: 10},
		{desc: "style_title_ink", x: bx + styleX + 12, y: by + styleY + 8, w: 300, h: 14, bg: styleBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 10},
		{desc: "style_label0_ink", x: bx + styleX + 12, y: by + styleY + 40, w: 220, h: 20, bg: styleBG, tol: 0.04, minPx: 8, fg: [3]float64{0.88, 0.92, 0.98}, minFG: 5},
		{desc: "para_title_ink", x: bx + paraX + 12, y: by + paraY + 8, w: 320, h: 14, bg: paraBG, tol: 0.04, minPx: 20},
		{desc: "para_line_ink", x: bx + paraX + 12, y: by + paraY + 38, w: 240, h: 90, bg: paraBG, tol: 0.04, minPx: 30, fg: [3]float64{0.76, 0.86, 0.98}, minFG: 10},
		{desc: "para2_line_ink", x: bx + paraX + 268, y: by + paraY + 38, w: 232, h: 90, bg: paraBG, tol: 0.04, minPx: 30, fg: [3]float64{0.78, 0.90, 0.82}, minFG: 10},
		{desc: "dense_title_ink", x: bx + denseX + 12, y: by + denseY + 8, w: 250, h: 14, bg: denseBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 10},
		// Small 9-11px glyphs have very few pure interior pixels; the dense
		// panel's 13px title above carries the F6 main-colour-share check.
		// This region still proves the small label rendered (ink >= 8px).
		{desc: "dense_label_ink", x: bx + denseX + 12, y: by + denseY + 115, w: 140, h: 18, bg: denseBG, tol: 0.04, minPx: 8},
		{desc: "status_title_ink", x: bx + statusX + 12, y: by + statusY + 8, w: 250, h: 14, bg: statusBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 10},
		{desc: "status_line0_ink", x: bx + statusX + 12, y: by + statusY + 38, w: 290, h: 14, bg: statusBG, tol: 0.04, minPx: 10, fg: [3]float64{0.68, 0.78, 0.90}, minFG: 5},
		{desc: "footer_title_ink", x: bx + footerX + 12, y: by + footerY + 8, w: 260, h: 14, bg: footerBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 10},
		{desc: "footer_line_ink", x: bx + footerX + 12, y: by + footerY + 36, w: 400, h: 14, bg: footerBG, tol: 0.04, minPx: 10, fg: [3]float64{0.68, 0.78, 0.90}, minFG: 5},
		{desc: "hot_title_ink", x: bx + hotX + 12, y: by + hotY + 8, w: 300, h: 14, bg: hotBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 10},
		{desc: "hotA_ink", x: bx + hotX + 12, y: by + hotY + 38, w: 290, h: 22, bg: hotBG, tol: 0.04, minPx: 10, fg: [3]float64{0.96, 0.72, 0.46}, minFG: 10},
		{desc: "hotB_ink", x: bx + hotX + 12, y: by + hotY + 76, w: 290, h: 20, bg: hotBG, tol: 0.04, minPx: 10, fg: [3]float64{0.58, 0.90, 0.68}, minFG: 10},
	}

	var pixelOK int
	pixelBad := make([]string, 0, len(pixelChecks)+len(inkRegions))
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	pixelTotal := 0
	if gateMode {
		img := loadImage(snapPath)
		pixelResult := make(map[string]bool, len(pixelChecks)+len(inkRegions))
		runPixelChecks(img, pixelChecks, pixelResult)
		runInkRegions(img, inkRegions, pixelResult)
		for desc, ok := range pixelResult {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, desc)
			}
		}
		sort.Strings(pixelBad)
		pixelTotal = len(pixelChecks) + len(inkRegions)

		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, snapName, baseName, goldenRects)
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		pixelGoldenValue = nil
	}

	hitsPhaseSum := hitsSteady + hitsSpike + hitsRecover
	missPhaseSum := missSteady + missSpike + missRecover
	phases := make([]string, 0, 3)
	for _, p := range []string{wrkit.PhaseSteady, wrkit.PhaseSpike, wrkit.PhaseRecover} {
		if phaseSeen[p] {
			phases = append(phases, p)
		}
	}

	type probe struct {
		name string
		ok   bool
	}
	probes := []probe{
		{"presents_positive", presents >= 1},
		{"fps_interval_ge_55", fpsInterval >= 55},
		{"p95_measured_le_22", snap.P95FrameIntervalMs > 0 && snap.P95FrameIntervalMs <= 22},
		{"paint_count_positive", snap.PaintCount > 0},
		{"layout_count_positive", snap.LayoutCount > 0},
		{"present_policy_full", snap.PresentPolicy == scheduler.PresentPolicyFullPaint},
		{"cpu_paths_recorded", snap.CPUPctAvg > 0 && (snap.CPUUIPct > 0 || snap.CPURasterPct > 0)},
		{"cpu_fallback_zero", snap.CPUFallbackOps == 0},
		{"vsync_source_present", snap.VSyncSource != ""},
		{"text_nodes_ge_24", len(allTexts) >= 24},
		{"content_text_nodes_ge_20", len(contentTexts) >= 20},
		{"wrapped_nodes_ge_2", wrappedNodes >= 2},
		{"distinct_sizes_ge_4", len(distinctSizes) >= 4},
		{"distinct_weights_ge_2", len(distinctWeights) >= 2},
		{"distinct_colors_ge_3", len(distinctColors) >= 3},
		{"stats_reset_observed", statsReset},
		{"measure_hits_after_reset_ge_1000", totalHits >= 1000},
		{"measure_hits_ge_miss", totalHits >= totalMiss},
		{"text_sizes_stable", sizeStable},
		{"static_text_miss_zero", staticMiss == 0},
		{"real_weight_14_15_no_synthetic", realWeightOK},
	}
	if gateMode {
		probes = append(probes,
			probe{"gate_phase_frozen", phase == wrkit.PhaseSteady && !phaseSeen[wrkit.PhaseSpike] && !phaseSeen[wrkit.PhaseRecover]},
			probe{"gate_measure_miss_zero", totalMiss == 0},
			probe{"gate_hot_static", hotMiss == 0},
			probe{"gate_hits_phase_sum", hitsPhaseSum == totalHits},
			probe{"gate_miss_phase_sum", missPhaseSum == totalMiss},
			probe{"gate_pixel_probes_all", pixelOK == pixelTotal && pixelTotal > 0},
			probe{"gate_golden_zero", goldenFirstRun || (goldenDiffPct == 0 && goldenTotalPx == goldenMaskPx)},
		)
	} else {
		probes = append(probes,
			probe{"manual_steady_hits_positive", hitsSteady > 0},
			probe{"manual_steady_miss_zero", missSteady == 0},
			probe{"manual_spike_miss_positive", missSpike > 0},
			probe{"manual_hot_miss_positive", hotMiss > 0},
			probe{"manual_recover_settled", phaseSeen[wrkit.PhaseRecover] && recoverSettled},
		)
		if phaseSeen[wrkit.PhaseRecover] && recoverSettled {
			probes = append(probes, probe{"manual_recover_miss_zero", totalMiss == recoverMissAtSettle})
		}
		if *manualSeconds >= 15 {
			probes = append(probes, probe{"manual_all_phases", phaseSeen[wrkit.PhaseSteady] && phaseSeen[wrkit.PhaseSpike] && phaseSeen[wrkit.PhaseRecover]})
		}
	}
	logicFailed := make([]string, 0)
	for _, p := range probes {
		if !p.ok {
			logicFailed = append(logicFailed, p.name)
		}
	}

	recoverMissAfterSettle := int64(0)
	if recoverSettled {
		recoverMissAfterSettle = totalMiss - recoverMissAtSettle
	}

	extra := map[string]any{
		"client_px":                        "1200x800",
		"run_seconds":                      secs,
		"close_seconds_required":           closeSecs,
		"manual_seconds":                   *manualSeconds,
		"evidence_mode":                    map[bool]string{true: "gate", false: "manual"}[gateMode],
		"phase_mode":                       map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
		"phases_seen":                      phases,
		"text_nodes":                       len(allTexts),
		"content_text_nodes":               len(contentTexts),
		"wrapped_nodes":                    wrappedNodes,
		"distinct_sizes":                   len(distinctSizes),
		"distinct_weights":                 len(distinctWeights),
		"distinct_colors":                  len(distinctColors),
		"stats_reset_tick":                 statsResetTick,
		"measure_cache_hit_total":          totalHits,
		"measure_cache_miss_total":         totalMiss,
		"measure_cache_hit_after_reset":    totalHits,
		"measure_cache_miss_after_reset":   totalMiss,
		"measure_metrics_match_tree":       measureMetricsMatch,
		"measure_metrics_pre_sync_hit":     metricsHits,
		"measure_metrics_pre_sync_miss":    metricsMiss,
		"measure_metrics_pre_sync_lag_hit": totalHits - metricsHits,
		"measure_metrics_source":           "TreeMeasureCacheStats after close, synced into MetricsStore before report",
		"hits_steady":                      hitsSteady,
		"hits_spike":                       hitsSpike,
		"hits_recover":                     hitsRecover,
		"hits_phase_sum":                   hitsPhaseSum,
		"miss_steady":                      missSteady,
		"miss_spike":                       missSpike,
		"miss_recover":                     missRecover,
		"miss_phase_sum":                   missPhaseSum,
		"hot_text_miss":                    hotMiss,
		"static_text_miss":                 staticMiss,
		"text_sizes_stable":                sizeStable,
		"real_weight_14_15_ok":             realWeightOK,
		"real_weight_14_15_checked":        realWeightChecked,
		"real_weight_14_15_faces":          realWeightFaces,
		"real_weight_14_15_bad":            realWeightBad,
		"real_weight_14_15_contract":       "14px semibold and 15px bold must resolve to real font cuts (or wght variable outlines), never post-raster mask dilation",
		"size_bad":                         sizeBad,
		"recover_settled":                  recoverSettled,
		"recover_miss_after_settle":        recoverMissAfterSettle,
		"measure_cache_contract":           "reset after warmup; forced relayout must hit; only changed hot strings may miss",
		"present_policy":                   snap.PresentPolicy,
		"live_frame_build_ms":              lastLiveSnap.LastBuildMs,
		"live_frame_raster_ms":             lastLiveSnap.LastRasterMs,
		"live_fps_interval":                liveFPSInterval,
		"frame_raster_note": map[bool]string{
			true:  "gate frame_raster_ms includes the final SnapshotPath GPU readback/PNG; live_frame_raster_ms is the last live tick",
			false: "manual frame_raster_ms is the last completed live present; manual mode has no SnapshotPath readback",
		}[gateMode],
		"pixel_evidence":         map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
		"pixel_probes_ok":        pixelOK,
		"pixel_probes_total":     pixelTotal,
		"pixel_bad":              pixelBad,
		"pixel_golden_diff_pct":  pixelGoldenValue,
		"pixel_golden_total_px":  goldenTotalPx,
		"pixel_golden_first_run": goldenFirstRun,
		"golden_mask":            "header 1200x48 + legend 260x656 + body 904x656",
		"golden_mask_px":         goldenMaskPx,
		"golden_tolerance_pct":   0,
		"snapshot_png":           snapshotPath,
		"snapshot_note": map[bool]string{
			true:  "gate snapshot (GPU readback) from the frozen Steady scene",
			false: "manual mode does not write the gate snapshot; committed evidence is not overwritten",
		}[gateMode],
		"fps_gate":              "persistent_tick_ge_55",
		"slope_gate":            "short_run_no_leak_verdict",
		"rss_slope_note":        "rss_slope_valid=false: a 5s gate / 15s manual run is shorter than the >=30s steady-slope verdict; raw slope is startup-inclusive and is not a leak conclusion",
		"rss_slope_steady_note": "steady slope is omitted while valid=false (short run); omission means 'cannot conclude', not 'no leak' (TestBuildReport_ShortRunSlopeInvalidIsHonest)",
		"rss_after_close_note":  "short correctness window: after_close may equal peak while GPU driver/Go heap caches are still resident; not a soak leak verdict",
		"cpu_fallback_note":     "none observed; last_cpu_fallback empty means no fallback",
		"gpu_ops_semantics":     "cumulative",
		"g_metrics":             "measured",
		"g_metrics_reason":      "R9 is the text measure cache window; measure_cache_hit/miss are gated",
		"depcheck":              "passed_no_cgo_no_ui_gpu_import",
		"impl_correctness":      "RenderText.measureLine caches width by text+size+spacing+weight; forced layout every tick hits the same keys and returns the same width. The gate resets stats after warmup, so hits prove reuse rather than first-layout misses.",
		"impl_dirty":            "all text nodes are marked needs-layout every tick; static text must hit with miss=0. Manual Steady changes only hotA colour (paint dirty, no measure miss); Spike changes hotA/hotB strings every tick (real cold misses); Recover installs final strings once and then stops.",
		"impl_cache":            "same text/style keeps its width cache; SetText/SetFace/SetFontSize/SetFontWeight invalidate it. The manual static-text miss count must stay zero while only the two hot strings miss.",
		"impl_edge":             "multiple sizes, weights, colours, two different wrap widths, dense small labels, and a full shell; gate is frozen Steady; manual runs Steady -> Spike -> Recover; gate snapshot and Golden are separate from manual.",
		"impl_fail":             "gate hit<1000 or miss!=0; size drift; static text miss>0; manual Spike miss==0 or Recover miss keeps growing; pixel probe fail; Golden diff!=0 or mask px mismatch; policy!=full_paint; fps<55 or p95>22 -> FAIL",
		"impl_visible":          "style matrix, wrapped paragraphs, dense labels and colour chips stay fixed; HUD shows hit/miss/layout/phase; manual Spike visibly changes hot strings, Recover freezes them.",
		"logic_probes_ok":       len(probes) - len(logicFailed),
		"logic_probes_total":    len(probes),
		"logic_bad":             logicFailed,
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R9",
		Scenario:      "ui_wr_r9_text_cache",
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
		os.Exit(1)
	}
	if len(logicFailed) > 0 {
		fail("logic probes %d/%d bad=%v", len(probes)-len(logicFailed), len(probes), logicFailed)
	}
	if gateMode {
		if pixelOK != pixelTotal {
			fail("pixel probes %d/%d ok bad=%v", pixelOK, pixelTotal, pixelBad)
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		if !goldenFirstRun && goldenTotalPx != goldenMaskPx {
			fail("golden mask px=%d want %d", goldenTotalPx, goldenMaskPx)
		}
	}
	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MinFPSElapsed:          5,
		MaxP95Ms:               22,
		MinMeasureCacheHit:     1000,
		RequireHitsGEMMiss:     true,
	}); err != nil {
		fail("%v", err)
	}

	runStatus := "OK"
	if gateMode && goldenFirstRun {
		runStatus = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%% hit=%d miss=%d", pixelOK, pixelTotal, goldenDiffPct, totalHits, totalMiss)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r9_text_cache: %s presents=%d layouts=%d hit=%d miss=%d static_miss=%d hot_miss=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		runStatus, presents, snap.LayoutCount, totalHits, totalMiss, staticMiss, hotMiss,
		len(probes)-len(logicFailed), len(probes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func collectTexts(root rendering.RenderObject, out *[]*rendering.RenderText) {
	if root == nil {
		return
	}
	if t, ok := root.(*rendering.RenderText); ok {
		*out = append(*out, t)
	}
	for _, ch := range root.Children() {
		collectTexts(ch, out)
	}
}

// weightFaceProof checks the R9 14px/15px style-matrix rows against the
// resolved face. It must see a real weight cut (or a wght-variable outline),
// never a synthetic post-raster mask dilation. The old 1px mask dilation at
// these exact sizes was the visible blur this window now guards against.
func weightFaceProof(texts []*rendering.RenderText) (ok bool, checked int, faces []map[string]any, bad []string) {
	for _, t := range texts {
		if t == nil {
			continue
		}
		if !((t.FontSize == 14 && t.FontWeight == rtext.WeightSemiBold) ||
			(t.FontSize == 15 && t.FontWeight == rtext.WeightBold)) {
			continue
		}
		checked++
		face := rtext.DeriveFace(t.Face, t.FontSize, t.FontWeight)
		if face == nil {
			bad = append(bad, fmt.Sprintf("%q %.0fpx w%d: nil face", t.Text, t.FontSize, t.FontWeight))
			continue
		}
		source := ""
		if src := face.Source(); src != nil {
			source = src.Parsed().FullName()
		}
		entry := map[string]any{
			"text":       t.Text,
			"size":       t.FontSize,
			"weight":     int(t.FontWeight),
			"embolden":   rtext.FaceEmbolden(face),
			"face_type":  fmt.Sprintf("%T", face),
			"source":     source,
			"variations": face.Variations(),
		}
		faces = append(faces, entry)
		if rtext.FaceEmbolden(face) {
			bad = append(bad, fmt.Sprintf("%q %.0fpx w%d: synthetic embolden still active", t.Text, t.FontSize, t.FontWeight))
		}
		if face.Weight() != t.FontWeight {
			bad = append(bad, fmt.Sprintf("%q %.0fpx w%d: resolved weight=%d", t.Text, t.FontSize, t.FontWeight, face.Weight()))
		}
	}
	if checked != 2 {
		bad = append(bad, fmt.Sprintf("checked=%d want 2 (14px semibold + 15px bold)", checked))
	}
	return checked == 2 && len(bad) == 0, checked, faces, bad
}

func sumTextStats(ts []*rendering.RenderText) (hits, misses int64) {
	for _, t := range ts {
		h, m := t.MeasureCacheStats()
		hits += h
		misses += m
	}
	return hits, misses
}

func resetTextStats(ts []*rendering.RenderText) {
	for _, t := range ts {
		t.ResetMeasureCacheStats()
	}
}
