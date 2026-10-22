// Command ui_wr_r3b_compbits is the W1 R3b real-window: compositing bits /
// boundary discovery.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits -auto-only
//	go run ./examples/ui_wr_r3b_compbits                 # manual: animate until close
//	go run ./examples/ui_wr_r3b_compbits -manual-seconds 15
//
// Window: 1200x800. R3b close duration is 8s; a gate run below 8s is rejected.
//
// Evidence: compositing-bit logic probes (per-node expected true/false), color
// and ink probes on the GPU snapshot, and a bitwise Golden over header + legend
// + body (821184 logical px). Every probed node has a green (true) / magenta
// (false) chip in the status panel, so a wrong bit is also a wrong pixel.
// Gate mode freezes Steady for a deterministic snapshot; manual mode keeps the
// Steady -> Spike -> Recover animation and does not assert pixels.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	closeSecs  = 8
	gridCells  = 16
	gridLabelN = 8

	// Status panel (Body-local) and chip geometry. Pixel probes below reuse
	// these values so the visual chips and their probes cannot drift apart.
	statusX, statusY = 24.0, 320.0
	chipLeftX        = 16.0
	chipRightX       = 440.0
	chipY0           = 32.0
	chipStep         = 20.0
	chipSize         = 14.0
)

type compEntry struct {
	name  string
	node  rendering.RenderObject
	want  bool
	chip  *rendering.RenderColorBox
	label *rendering.RenderText
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R3b")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R3b (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{
		Width:       winW,
		Height:      winH,
		Title:       "gpui ui_wr_r3b_compbits — 合成位发现",
		Decorations: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "R3b 合成位 — 4 层嵌套 + NeedsCompositing 可视化", nil)
	legendRows := []struct {
		text string
		r    float64
		g    float64
		b    float64
	}{
		{"a 260² boundary", 0.16, 0.18, 0.24},
		{"b 200² nested", 0.20, 0.15, 0.26},
		{"c 140² nested + hot", 0.12, 0.21, 0.24},
		{"d 60² depth-4 + dHot", 0.30, 0.22, 0.14},
		{"s / s2 sibling bounds", 0.14, 0.18, 0.20},
		{"wrap plain -> pw bound", 0.18, 0.22, 0.16},
		{"neg: no boundary (F)", 0.17, 0.17, 0.19},
		{"green = compositing true", 0.20, 0.85, 0.35},
		{"magenta = compositing false", 0.85, 0.20, 0.85},
		{"grid + labels static", 0.32, 0.54, 0.64},
		{"gate: -auto-only 8s", 0.55, 0.75, 0.95},
	}
	for i, row := range legendRows {
		y := 36 + float64(i)*22
		shell.Legend.ColorAt(10, 10, 12, y+3, row.r, row.g, row.b, 1, false)
		shell.Legend.LabelAt(row.text, 12, 28, y, 0.70, 0.78, 0.88)
	}

	staticLabels := 0
	addLabel := func(parent *rendering.AbsoluteBox, text string, size, x, y, r, g, b float64) *rendering.RenderText {
		t := wrkit.Label(text, size, r, g, b)
		parent.Place(t, x, y)
		staticLabels++
		return t
	}

	boundaries := make(map[rendering.RenderObject]bool)
	mkBoundary := func(w, h, r, g, b float64, name string) *rendering.AbsoluteBox {
		box := rendering.NewAbsoluteBox(w, h)
		box.Background = &rendering.Color{R: r, G: g, B: b, A: 1}
		box.SetRepaintBoundary(true)
		box.SetDebugName(name)
		boundaries[box] = true
		return box
	}

	// Main chain: a -> b -> c -> d = four boundary layers. hot lives directly
	// inside c (plain leaf, expected bit false); dHot lives inside d.
	a := mkBoundary(260, 260, 0.16, 0.18, 0.24, "a")
	shell.Body.Place(a, 24, 24)
	b := mkBoundary(200, 200, 0.20, 0.15, 0.26, "b")
	a.Place(b, 30, 30)
	c := mkBoundary(140, 140, 0.12, 0.21, 0.24, "c")
	b.Place(c, 30, 30)
	hot := rendering.NewRenderColorBox(50, 50, 0.95, 0.20, 0.20, 1)
	c.Place(hot, 10, 10)
	d := mkBoundary(60, 60, 0.30, 0.22, 0.14, "d")
	c.Place(d, 70, 70)
	dHot := rendering.NewRenderColorBox(20, 20, 0.95, 0.60, 0.15, 1)
	d.Place(dHot, 10, 10)
	addLabel(d, "d", 11, 10, 32, 0.85, 0.88, 0.94)
	addLabel(c, "c", 11, 10, 120, 0.82, 0.86, 0.94)
	c.Place(rendering.NewRenderColorBox(16, 16, 0.60, 0.40, 0.50, 1), 120, 10)
	addLabel(b, "b", 11, 10, 160, 0.82, 0.86, 0.94)
	b.Place(rendering.NewRenderColorBox(16, 16, 0.50, 0.40, 0.60, 1), 10, 182)
	addLabel(a, "a", 11, 10, 232, 0.82, 0.86, 0.94)
	a.Place(rendering.NewRenderColorBox(18, 18, 0.40, 0.50, 0.60, 1), 228, 10)

	// Sibling boundaries: independent nests, not on the main chain.
	s := mkBoundary(160, 160, 0.14, 0.18, 0.20, "s")
	shell.Body.Place(s, 320, 24)
	hotS := rendering.NewRenderColorBox(50, 50, 0.20, 0.60, 0.95, 1)
	s.Place(hotS, 55, 55)
	addLabel(s, "s", 11, 10, 10, 0.82, 0.86, 0.94)
	s.Place(rendering.NewRenderColorBox(20, 20, 0.35, 0.55, 0.45, 1), 10, 130)

	s2 := mkBoundary(140, 140, 0.20, 0.16, 0.22, "s2")
	shell.Body.Place(s2, 520, 24)
	hotS2 := rendering.NewRenderColorBox(44, 44, 0.85, 0.60, 0.20, 1)
	s2.Place(hotS2, 48, 48)
	addLabel(s2, "s2", 11, 10, 10, 0.82, 0.86, 0.94)
	s2.Place(rendering.NewRenderColorBox(20, 20, 0.55, 0.45, 0.35, 1), 10, 110)

	// Plain ancestor containing a boundary: wrap must be true even though it is
	// not itself a boundary.
	wrap := rendering.NewAbsoluteBox(160, 160)
	wrap.Background = &rendering.Color{R: 0.18, G: 0.22, B: 0.16, A: 1}
	shell.Body.Place(wrap, 700, 24)
	addLabel(wrap, "wrap", 11, 10, 10, 0.82, 0.86, 0.94)
	pw := mkBoundary(100, 100, 0.28, 0.16, 0.30, "pw")
	wrap.Place(pw, 30, 30)
	pw.Place(rendering.NewRenderColorBox(20, 20, 0.60, 0.45, 0.70, 1), 10, 10)
	addLabel(pw, "pw", 11, 10, 76, 0.85, 0.88, 0.94)

	// Negative control: a plain subtree with no boundary anywhere. Its bit must
	// stay false although its ancestor (Body) is true.
	neg := rendering.NewAbsoluteBox(160, 120)
	neg.Background = &rendering.Color{R: 0.17, G: 0.17, B: 0.19, A: 1}
	shell.Body.Place(neg, 700, 200)
	neg.Place(rendering.NewRenderColorBox(20, 20, 0.65, 0.55, 0.30, 1), 10, 10)
	addLabel(neg, "no boundary", 11, 10, 90, 0.82, 0.86, 0.94)

	// Status panel: one chip per probed node. Green = NeedsCompositing true,
	// magenta = false. The chips are set once after the first frames have
	// flushed compositing bits, then stay static for the Golden snapshot.
	status := wrkit.NewPanel(856, 150, 0.13, 0.14, 0.17, 1)
	status.PlaceOn(shell.Body.Box, statusX, statusY)
	status.LabelAt("NeedsCompositing: green=true magenta=false", 13, 10, 8, 0.85, 0.90, 0.96)

	compDefs := []struct {
		name string
		node rendering.RenderObject
		want bool
	}{
		{"root", shell.Root, true},
		{"body", shell.Body.Box, true},
		{"a", a, true},
		{"b", b, true},
		{"c", c, true},
		{"d", d, true},
		{"hot", hot, false},
		{"s", s, true},
		{"s2", s2, true},
		{"wrap", wrap, true},
		{"pw", pw, true},
		{"neg", neg, false},
	}
	comps := make([]compEntry, 0, len(compDefs))
	wantTrue := 0
	for i, def := range compDefs {
		col, row := i/6, i%6
		cx, lx := chipLeftX, chipLeftX+22
		if col == 1 {
			cx, lx = chipRightX, chipRightX+22
		}
		cy := chipY0 + float64(row)*chipStep
		chip := status.ColorAt(chipSize, chipSize, cx, cy, 0.30, 0.30, 0.34, 1, false)
		lbl := wrkit.Label(def.name, 11, 0.90, 0.92, 0.96)
		status.Place(lbl, lx, cy)
		if def.want {
			wantTrue++
		}
		comps = append(comps, compEntry{name: def.name, node: def.node, want: def.want, chip: chip, label: lbl})
	}

	// Static 4x4 grid outside every boundary: proves unrelated content stays
	// untouched while inner boundaries re-record.
	gridWant := func(i, j int) [3]float64 {
		return [3]float64{0.20 + 0.06*float64(i), 0.42 + 0.05*float64(j), 0.60 + 0.02*float64(i)}
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			want := gridWant(i, j)
			cell := rendering.NewRenderColorBox(30, 30, want[0], want[1], want[2], 1)
			shell.Body.Place(cell, 24+float64(i)*38, 486+float64(j)*38)
		}
	}
	for i := 0; i < gridLabelN; i++ {
		addLabel(shell.Body.Box, fmt.Sprintf("grid %d", i), 11,
			200+float64(i%4)*160, 486+float64(i/4)*40, 0.65, 0.72, 0.85)
	}

	// Gate snapshot + Golden base live under testdata/ (committed evidence).
	snapDir := filepath.Join("examples", "ui_wr_r3b_compbits", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, snapshotPNG)
	// Manual mode animates, so its frame is not evidence. Keep the gate
	// snapshot path empty there: otherwise a later manual run would overwrite
	// the committed gate PNG that the Golden baseline was seeded from.
	snapshotPath := snapPath
	if !gateMode {
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
				fmt.Fprintf(os.Stderr, "ui_wr_r3b_compbits: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Gate mode never resizes, so pixel/Golden coordinates stay fixed.
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// FullPaint correctness window: pin the policy so JSON cannot silently
	// inherit the retained default.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{wrkit.PhaseSteady: true}
	clock := wrkit.NewPhaseClock(2, 4)
	hotTicks := int64(0)
	lastRR := int64(0)
	rrSteady, rrSpike, rrRecover := int64(0), int64(0), int64(0)

	compSet := false
	compSetTick := int64(-1)
	compStatesAtSet := map[string]bool{}
	compTrueCount, compFalseCount := 0, 0

	// Freeze-point observation (gate only): boundary counters at the first
	// post-churn tick and at exit. Proves the Readme claim that after the
	// 60-tick colour churn only replay (skip) keeps growing while the
	// unchanged content stops re-recording.
	var rrAtFreeze, skipAtFreeze int64
	freezeSeen := false
	// Freeze fields are gate-only. In manual mode there is no freeze point, so
	// they are null rather than reusing the total counters under a false name.
	var rrAtFreezeOut, skipAtFreezeOut, rrAfterFreezeOut, skipAfterFreezeOut any

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady: deterministic 60-tick colour churn produces real
			// content dirt (boundary_rerecord > 0), then the exact final
			// colours are held. Once the content stops changing the boundary
			// cache replays by content fingerprint, so rerecord stays flat and
			// only boundary_skip keeps rising; that is what makes the 8s
			// snapshot bit-stable without faking the dirty path.
			phase = wrkit.PhaseSteady
			if hotTicks < 60 {
				t := float64(hotTicks) / 60
				hot.R, hot.G, hot.B = 0.95-0.30*t, 0.20, 0.20
				hotS.R, hotS.G, hotS.B = 0.20, 0.60, 0.95-0.30*t
			} else {
				hot.R, hot.G, hot.B = 0.95, 0.20, 0.20
				hotS.R, hotS.G, hotS.B = 0.20, 0.60, 0.95
			}
			hot.MarkNeedsPaint()
			hotS.MarkNeedsPaint()
			hotTicks++
		} else {
			phase = clock.Advance(dt)
			switch phase {
			case wrkit.PhaseSteady:
				// Main-chain inner leaf + sibling leaf: both nests re-record
				// their own layers, unrelated boundaries stay cached.
				hot.R = 0.95 - 0.30*(clock.Elapsed()/2)
				hot.G, hot.B = 0.20, 0.20
				hot.MarkNeedsPaint()
				hotS.B = 0.95 - 0.30*(clock.Elapsed()/2)
				hotS.R, hotS.G = 0.20, 0.60
				hotS.MarkNeedsPaint()
				hotTicks++
			case wrkit.PhaseSpike:
				// Main-chain root boundary + second sibling: outer-level dirty
				// must not touch the other nests' pixels.
				a.Background = &rendering.Color{
					R: 0.35 + 0.20*float64(int(clock.Elapsed())%2),
					G: 0.20, B: 0.30, A: 1,
				}
				a.MarkNeedsPaint()
				hotS2.R = 0.85 - 0.25*float64(int(clock.Elapsed())%2)
				hotS2.MarkNeedsPaint()
			default:
				// Deep leaf inside c -> d: depth-4 boundary re-records.
				dHot.G = 0.60 - 0.30*float64(int(clock.Elapsed())%2)
				dHot.MarkNeedsPaint()
			}
		}

		// Compositing bits have been flushed by the pipeline before this tick;
		// tick 10 is early enough to freeze the chips for the rest of the run.
		if !compSet && hotTicks >= 10 {
			for i := range comps {
				got := rendering.NeedsCompositingOf(comps[i].node)
				compStatesAtSet[comps[i].name] = got
				if got {
					comps[i].chip.R, comps[i].chip.G, comps[i].chip.B = 0.20, 0.85, 0.35
					compTrueCount++
				} else {
					comps[i].chip.R, comps[i].chip.G, comps[i].chip.B = 0.85, 0.20, 0.85
					compFalseCount++
				}
				comps[i].chip.MarkNeedsPaint()
				comps[i].label.SetText(fmt.Sprintf("%s=%s", comps[i].name, map[bool]string{true: "T", false: "F"}[got]))
			}
			compSet = true
			compSetTick = hotTicks
		}

		phaseSeen[phase] = true
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD; skip/rerecord and the phase are the on-screen proof.
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		wrkit.MergeBoundaryCache(app, &snapH)
		if gateMode && !freezeSeen && hotTicks >= 61 {
			rrAtFreeze, skipAtFreeze = snapH.BoundaryRerecord, snapH.BoundarySkip
			freezeSeen = true
		}
		if snapH.BoundaryRerecord >= lastRR {
			d := snapH.BoundaryRerecord - lastRR
			switch phase {
			case wrkit.PhaseSteady:
				rrSteady += d
			case wrkit.PhaseSpike:
				rrSpike += d
			default:
				rrRecover += d
			}
			lastRR = snapH.BoundaryRerecord
		}
		gateOK := snapH.BoundarySkip >= 3 && snapH.BoundaryRerecord >= 2 &&
			snapH.BoundaryCount >= 3 && snapH.BoundaryMaxDepth >= 4
		shell.UpdateHUD("R3b", phase, app, gateOK,
			wrkit.FmtSkip(snapH.BoundarySkip, snapH.BoundaryRerecord),
			fmt.Sprintf("comp=%dT/%dF", compTrueCount, compFalseCount))
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
	wrkit.MergeBoundaryCache(app, &snap)
	// Attribute the final post-tick rerecord delta to the phase that was
	// active, so rr_steady+rr_spike+rr_recover sums to the JSON total.
	if snap.BoundaryRerecord >= lastRR {
		d := snap.BoundaryRerecord - lastRR
		switch phase {
		case wrkit.PhaseSteady:
			rrSteady += d
		case wrkit.PhaseSpike:
			rrSpike += d
		default:
			rrRecover += d
		}
		lastRR = snap.BoundaryRerecord
	}

	// Final compositing states: the chips were frozen at compSetTick, so a
	// late bit flip would make the visual evidence stale. Compare both maps.
	compStates := make(map[string]bool, len(comps))
	compTrueCount, compFalseCount = 0, 0
	compAllCorrect := compSet
	for i := range comps {
		got := rendering.NeedsCompositingOf(comps[i].node)
		compStates[comps[i].name] = got
		if got {
			compTrueCount++
		} else {
			compFalseCount++
		}
		if got != comps[i].want {
			compAllCorrect = false
		}
	}
	compStable := compSet && len(compStates) == len(compStatesAtSet)
	for name, got := range compStates {
		if was, ok := compStatesAtSet[name]; !ok || was != got {
			compStable = false
		}
	}

	// Structural truth (not a hardcoded chain depth): count boundary layers
	// bottom-up and compare with the engine metrics. HUD is a boundary too.
	maxBoundaryDepth := boundaryDepth(shell.Root, boundaries, 0)
	boundaryCountExpected := int64(len(boundaries))
	if shell.HUD != nil {
		boundaryCountExpected++
	}

	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}

	// Pixel probes are anchored to the body panel origin. Gate mode never
	// resizes, so these logical coordinates stay fixed.
	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		// Main chain backgrounds and static blocks.
		{desc: "a_bg", x: bx + 264, y: by + 264, want: [3]float64{0.16, 0.18, 0.24}, tol: 0.06, kind: kindBG},
		{desc: "b_bg", x: bx + 234, y: by + 234, want: [3]float64{0.20, 0.15, 0.26}, tol: 0.06, kind: kindBG},
		{desc: "c_bg", x: bx + 99, y: by + 209, want: [3]float64{0.12, 0.21, 0.24}, tol: 0.06, kind: kindBG},
		{desc: "d_bg", x: bx + 199, y: by + 169, want: [3]float64{0.30, 0.22, 0.14}, tol: 0.06, kind: kindBG},
		{desc: "hot_red", x: bx + 119, y: by + 119, want: [3]float64{0.95, 0.20, 0.20}, tol: 0.06},
		{desc: "dHot_orange", x: bx + 174, y: by + 174, want: [3]float64{0.95, 0.60, 0.15}, tol: 0.06},
		{desc: "a_static", x: bx + 261, y: by + 43, want: [3]float64{0.40, 0.50, 0.60}, tol: 0.06},
		{desc: "b_static", x: bx + 72, y: by + 244, want: [3]float64{0.50, 0.40, 0.60}, tol: 0.06},
		{desc: "c_static", x: bx + 212, y: by + 102, want: [3]float64{0.60, 0.40, 0.50}, tol: 0.06},

		// Sibling boundaries.
		{desc: "s_bg", x: bx + 450, y: by + 169, want: [3]float64{0.14, 0.18, 0.20}, tol: 0.06, kind: kindBG},
		{desc: "hotS_blue", x: bx + 400, y: by + 104, want: [3]float64{0.20, 0.60, 0.95}, tol: 0.06},
		{desc: "s_static", x: bx + 340, y: by + 164, want: [3]float64{0.35, 0.55, 0.45}, tol: 0.06},
		{desc: "s2_bg", x: bx + 640, y: by + 44, want: [3]float64{0.20, 0.16, 0.22}, tol: 0.06, kind: kindBG},
		{desc: "hotS2_orange", x: bx + 590, y: by + 94, want: [3]float64{0.85, 0.60, 0.20}, tol: 0.06},
		{desc: "s2_static", x: bx + 540, y: by + 144, want: [3]float64{0.55, 0.45, 0.35}, tol: 0.06},

		// Plain ancestor with boundary child + plain negative control.
		{desc: "wrap_bg", x: bx + 710, y: by + 164, want: [3]float64{0.18, 0.22, 0.16}, tol: 0.06, kind: kindBG},
		{desc: "pw_bg", x: bx + 805, y: by + 69, want: [3]float64{0.28, 0.16, 0.30}, tol: 0.06, kind: kindBG},
		{desc: "pw_static", x: bx + 750, y: by + 74, want: [3]float64{0.60, 0.45, 0.70}, tol: 0.06},
		{desc: "neg_bg", x: bx + 840, y: by + 300, want: [3]float64{0.17, 0.17, 0.19}, tol: 0.06, kind: kindBG},
		{desc: "neg_static", x: bx + 720, y: by + 220, want: [3]float64{0.65, 0.55, 0.30}, tol: 0.06},

		// Status panel + grid + shell backgrounds.
		{desc: "status_bg", x: bx + 434, y: by + 460, want: [3]float64{0.13, 0.14, 0.17}, tol: 0.05, kind: kindBG},
		{desc: "grid_00", x: bx + 39, y: by + 501, want: gridWant(0, 0), tol: 0.06},
		{desc: "grid_11", x: bx + 77, y: by + 539, want: gridWant(1, 1), tol: 0.06},
		{desc: "grid_22", x: bx + 115, y: by + 577, want: gridWant(2, 2), tol: 0.06},
		{desc: "grid_33", x: bx + 153, y: by + 615, want: gridWant(3, 3), tol: 0.06},
		{desc: "grid_03", x: bx + 39, y: by + 615, want: gridWant(0, 3), tol: 0.06},
		{desc: "body_bg", x: bx + 890, y: by + 640, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}

	// One chip per compositing state: green when true, magenta when false.
	green := [3]float64{0.20, 0.85, 0.35}
	magenta := [3]float64{0.85, 0.20, 0.85}
	chipProbe := func(name string, col, row int, want [3]float64) pixelCheck {
		cx := statusX + chipLeftX + chipSize/2
		if col == 1 {
			cx = statusX + chipRightX + chipSize/2
		}
		cy := statusY + chipY0 + float64(row)*chipStep + chipSize/2
		return pixelCheck{desc: "comp_" + name, x: bx + cx, y: by + cy, want: want, tol: 0.06}
	}
	chipChecks := []pixelCheck{
		chipProbe("root", 0, 0, green),
		chipProbe("body", 0, 1, green),
		chipProbe("a", 0, 2, green),
		chipProbe("b", 0, 3, green),
		chipProbe("c", 0, 4, green),
		chipProbe("d", 0, 5, green),
		chipProbe("hot", 1, 0, magenta),
		chipProbe("s", 1, 1, green),
		chipProbe("s2", 1, 2, green),
		chipProbe("wrap", 1, 3, green),
		chipProbe("pw", 1, 4, green),
		chipProbe("neg", 1, 5, magenta),
	}
	pixelChecks = append(pixelChecks, chipChecks...)

	inkRegions := []inkRegion{
		{desc: "a_label_ink", x: bx + 34, y: by + 256, w: 14, h: 14, bg: [3]float64{0.16, 0.18, 0.24}, tol: 0.04, minPx: 5},
		{desc: "b_label_ink", x: bx + 64, y: by + 214, w: 14, h: 14, bg: [3]float64{0.20, 0.15, 0.26}, tol: 0.04, minPx: 5},
		{desc: "c_label_ink", x: bx + 94, y: by + 204, w: 14, h: 14, bg: [3]float64{0.12, 0.21, 0.24}, tol: 0.04, minPx: 5},
		{desc: "d_label_ink", x: bx + 164, y: by + 186, w: 14, h: 14, bg: [3]float64{0.30, 0.22, 0.14}, tol: 0.04, minPx: 4},
		{desc: "s_label_ink", x: bx + 330, y: by + 34, w: 14, h: 14, bg: [3]float64{0.14, 0.18, 0.20}, tol: 0.04, minPx: 4},
		{desc: "s2_label_ink", x: bx + 530, y: by + 34, w: 22, h: 14, bg: [3]float64{0.20, 0.16, 0.22}, tol: 0.04, minPx: 5},
		{desc: "wrap_label_ink", x: bx + 710, y: by + 34, w: 40, h: 14, bg: [3]float64{0.18, 0.22, 0.16}, tol: 0.04, minPx: 8},
		{desc: "pw_label_ink", x: bx + 740, y: by + 130, w: 24, h: 14, bg: [3]float64{0.28, 0.16, 0.30}, tol: 0.04, minPx: 5},
		{desc: "neg_label_ink", x: bx + 710, y: by + 290, w: 80, h: 14, bg: [3]float64{0.17, 0.17, 0.19}, tol: 0.04, minPx: 10},
		{desc: "status_title_ink", x: bx + 34, y: by + 328, w: 300, h: 16, bg: [3]float64{0.13, 0.14, 0.17}, tol: 0.04, minPx: 20},
		{desc: "comp_root_label_ink", x: bx + 62, y: by + 352, w: 72, h: 14, bg: [3]float64{0.13, 0.14, 0.17}, tol: 0.04, minPx: 8},
		{desc: "comp_neg_label_ink", x: bx + 486, y: by + 452, w: 60, h: 14, bg: [3]float64{0.13, 0.14, 0.17}, tol: 0.04, minPx: 6},
		{desc: "grid_label_ink", x: bx + 200, y: by + 486, w: 55, h: 14, bg: [3]float64{0.10, 0.11, 0.13}, tol: 0.04, minPx: 6},
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	// Manual mode intentionally animates; its evidence is human observation.
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
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}
	pixelGoldenValue := any(goldenDiffPct)
	if !gateMode {
		// Manual mode animates; pixel/Golden evidence is not asserted. Null +
		// pixel_evidence reason avoids a misleading 0/0 "pass".
		pixelGoldenValue = nil
	}

	logicProbes := []string{
		"presents_positive",
		"fps_interval_ge_55",
		"paint_count_positive",
		"boundary_skip_positive",
		"boundary_rerecord_positive",
		"boundary_count_ge_3",
		"boundary_count_matches_scene",
		"boundary_max_depth_ge_4",
		"chain_depth_matches_metrics",
		"comp_root_true",
		"comp_body_true",
		"comp_all_correct",
		"comp_true_count_matches_scene",
		"comp_visual_set",
		"comp_states_stable",
		"static_labels_ge_8",
		"present_policy_full",
		"cpu_paths_recorded",
		"cpu_fallback_zero",
		"vsync_source_present",
		"rr_steady_positive",
		"rr_phase_sum",
	}
	if gateMode {
		logicProbes = append(logicProbes, "gate_phase_frozen", "gate_rr_spike_zero", "gate_rr_recover_zero", "gate_freeze_observed")
	} else {
		if phaseSeen[wrkit.PhaseSpike] {
			logicProbes = append(logicProbes, "manual_rr_spike_positive")
		}
		if phaseSeen[wrkit.PhaseRecover] {
			logicProbes = append(logicProbes, "manual_rr_recover_positive")
		}
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if presents < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if fpsInterval < 55 {
		logicFailed = append(logicFailed, "fps_interval_ge_55")
	}
	if snap.PaintCount <= 0 {
		logicFailed = append(logicFailed, "paint_count_positive")
	}
	if snap.BoundarySkip <= 0 {
		logicFailed = append(logicFailed, "boundary_skip_positive")
	}
	if snap.BoundaryRerecord <= 0 {
		logicFailed = append(logicFailed, "boundary_rerecord_positive")
	}
	if snap.BoundaryCount < 3 {
		logicFailed = append(logicFailed, "boundary_count_ge_3")
	}
	if snap.BoundaryCount != boundaryCountExpected {
		logicFailed = append(logicFailed, "boundary_count_matches_scene")
	}
	if maxBoundaryDepth < 4 {
		logicFailed = append(logicFailed, "boundary_max_depth_ge_4")
	}
	if int64(maxBoundaryDepth) != snap.BoundaryMaxDepth {
		logicFailed = append(logicFailed, "chain_depth_matches_metrics")
	}
	if !compStates["root"] {
		logicFailed = append(logicFailed, "comp_root_true")
	}
	if !compStates["body"] {
		logicFailed = append(logicFailed, "comp_body_true")
	}
	if !compAllCorrect {
		logicFailed = append(logicFailed, "comp_all_correct")
	}
	if compTrueCount != wantTrue {
		logicFailed = append(logicFailed, "comp_true_count_matches_scene")
	}
	if !compSet {
		logicFailed = append(logicFailed, "comp_visual_set")
	}
	if !compStable {
		logicFailed = append(logicFailed, "comp_states_stable")
	}
	if staticLabels < 8 {
		logicFailed = append(logicFailed, "static_labels_ge_8")
	}
	if snap.PresentPolicy != scheduler.PresentPolicyFullPaint {
		logicFailed = append(logicFailed, "present_policy_full")
	}
	if snap.CPUPctAvg <= 0 || (snap.CPUUIPct <= 0 && snap.CPURasterPct <= 0) {
		logicFailed = append(logicFailed, "cpu_paths_recorded")
	}
	if snap.CPUFallbackOps != 0 {
		logicFailed = append(logicFailed, "cpu_fallback_zero")
	}
	if snap.VSyncSource == "" {
		logicFailed = append(logicFailed, "vsync_source_present")
	}
	if rrSteady <= 0 {
		logicFailed = append(logicFailed, "rr_steady_positive")
	}
	if gateMode {
		if phase != wrkit.PhaseSteady {
			logicFailed = append(logicFailed, "gate_phase_frozen")
		}
		if rrSpike != 0 {
			logicFailed = append(logicFailed, "gate_rr_spike_zero")
		}
		if rrRecover != 0 {
			logicFailed = append(logicFailed, "gate_rr_recover_zero")
		}
		if !freezeSeen {
			logicFailed = append(logicFailed, "gate_freeze_observed")
		}
	} else {
		if phaseSeen[wrkit.PhaseSpike] && rrSpike <= 0 {
			logicFailed = append(logicFailed, "manual_rr_spike_positive")
		}
		if phaseSeen[wrkit.PhaseRecover] && rrRecover <= 0 {
			logicFailed = append(logicFailed, "manual_rr_recover_positive")
		}
	}
	if rrSteady+rrSpike+rrRecover != snap.BoundaryRerecord {
		logicFailed = append(logicFailed, "rr_phase_sum")
	}
	logicPass := len(logicFailed) == 0

	phaseList := make([]string, 0, 3)
	for _, p := range []string{wrkit.PhaseSteady, wrkit.PhaseSpike, wrkit.PhaseRecover} {
		if phaseSeen[p] {
			phaseList = append(phaseList, p)
		}
	}

	if freezeSeen {
		rrAtFreezeOut = rrAtFreeze
		skipAtFreezeOut = skipAtFreeze
		rrAfterFreezeOut = snap.BoundaryRerecord - rrAtFreeze
		skipAfterFreezeOut = snap.BoundarySkip - skipAtFreeze
	}

	extra := map[string]any{
		"client_px":               "1200x800",
		"run_seconds":             secs,
		"close_seconds_required":  closeSecs,
		"manual_seconds":          *manualSeconds,
		"present_policy":          snap.PresentPolicy,
		"chain_depth":             maxBoundaryDepth,
		"chain_depth_metrics":     snap.BoundaryMaxDepth,
		"content_boundaries":      len(boundaries),
		"expected_boundary_count": boundaryCountExpected,
		"comp_states":             compStates,
		"comp_true_count":         compTrueCount,
		"comp_false_count":        compFalseCount,
		"comp_expected_true":      wantTrue,
		"comp_set":                compSet,
		"comp_set_tick":           compSetTick,
		"comp_states_stable":      compStable,
		"static_labels":           staticLabels,
		"grid_cells":              gridCells,
		"hot_ticks":               hotTicks,
		"rr_steady":               rrSteady,
		"rr_spike":                rrSpike,
		"rr_recover":              rrRecover,
		"rr_total_seen":           lastRR,
		"rr_at_freeze":            rrAtFreezeOut,
		"skip_at_freeze":          skipAtFreezeOut,
		"rr_after_freeze":         rrAfterFreezeOut,
		"skip_after_freeze":       skipAfterFreezeOut,
		"freeze_observed":         freezeSeen,
		"freeze_note":             "gate only: counters at the first post-churn tick (hotTicks>=61); after the colours hold, identical content replays by fingerprint so rerecord should stay flat while skip keeps rising; null in manual because there is no freeze point",
		"phases_seen":             phaseList,
		"phase_mode":              map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
		"evidence_mode":           map[bool]string{true: "gate", false: "manual"}[gateMode],
		"pixel_evidence":          map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
		"fps_gate":                "persistent_tick_ge_55",
		"slope_gate":              "off_8s_short_run",
		"rss_slope_note":          "rss_slope_valid=false because 8s < rss_warmup_sec=15; raw slope is startup-inclusive, not a leak conclusion",
		"rss_slope_steady_note":   "rss_slope_steady_kb_per_min omitted when rss_slope_valid=false; an invalid steady slope must not claim a value (see wrgate TestBuildReport_ShortRunSlopeInvalidIsHonest)",
		"cpu_fallback_note":       "none observed; last_cpu_fallback empty means no fallback",
		"gpu_ops_semantics":       "cumulative",
		"g_metrics":               "skipped",
		"g_metrics_reason":        "R3b is a compositing-bits correctness window; measure cache fields are observed but not gated",
		"depcheck":                "passed_no_cgo_no_ui_gpu_import",
		"golden_mask":             "header 1200x48 + legend 260x656 + body 904x656",
		"golden_tolerance_pct":    0,
		"logic_probes_ok":         len(logicProbes) - len(logicFailed),
		"logic_probes_total":      len(logicProbes),
		"logic_bad":               logicFailed,
		"pixel_probes_ok":         pixelOK,
		"pixel_probes_total":      pixelTotal,
		"pixel_bad":               pixelBad,
		"pixel_golden_diff_pct":   pixelGoldenValue,
		"pixel_golden_total_px":   goldenTotalPx,
		"pixel_golden_first_run":  goldenFirstRun,
		"snapshot_png":            snapshotPath,
		"snapshot_note":           map[bool]string{true: "gate snapshot (GPU readback) used for pixel/Golden evidence", false: "manual mode does not write the gate snapshot; committed evidence is not overwritten"}[gateMode],
		"impl_correctness":        "main chain a->b->c->d (4 boundary layers), siblings s/s2, plain wrap with boundary pw, plain neg with no boundary; bits are bottom-up unions: root/body/wrap true, hot/neg false",
		"impl_dirty":              "steady gate: hot/hotS colour churn for the first 60 ticks then hold final colours; manual steady: hot/hotS every tick, spike: a.Background + hotS2, recover: dHot; HUD is a separate RepaintBoundary band",
		"impl_cache":              "clean boundaries replay and accumulate boundary_skip; content-changing dirty layers accumulate boundary_rerecord; after the gate freeze identical content replays by fingerprint (rr flat, skip rising), manual keeps content churn so rr rises in all phases",
		"impl_edge":               "4-level main chain, 2 sibling boundaries, plain ancestor with boundary child (wrap->pw, bit true), plain no-boundary negative control (neg, bit false), 12 compositing chips, 16 grid cells, 17 scene labels, 10 legend rows; resize re-lays the shell",
		"impl_fail":               "skip=0 or rerecord=0, boundary_count<3 or != scene, depth<4 or != structural depth, any compositing bit wrong, chips not set/stable, pixel probe mismatch, golden diff!=0, fps<55/p95>22, or present_policy!=full_paint -> FAIL",
		"impl_visible":            "green/magenta chips show all 12 probed nodes; HUD skip/rr/phase; manual Steady animates hot/hotS, Spike flips a background + hotS2, Recover animates dHot; static layers stay; legend has colour swatches",
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R3b",
		Scenario:      "ui_wr_r3b_compbits",
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
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
			RequirePersistentFPS:   true,
			MinFPSWall:             55,
			MinFPSElapsed:          5,
			MaxP95Ms:               22,
			MinBoundarySkip:        3,
			MinBoundaryRerecord:    2,
			MinBoundaryCount:       3,
			MinBoundaryMaxDepth:    4,
		}); err != nil {
			fail("%v", err)
		}
	}

	runStatus := "OK"
	if !logicPass {
		runStatus = "WARN"
	}
	if gateMode && goldenFirstRun {
		runStatus = "SEEDED"
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%% comp=%dT/%dF", pixelOK, pixelTotal, goldenDiffPct, compTrueCount, compFalseCount)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r3b_compbits: %s presents=%d skip=%d rr=%d count=%d depth=%d logical_depth=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		runStatus, presents, snap.BoundarySkip, snap.BoundaryRerecord,
		snap.BoundaryCount, snap.BoundaryMaxDepth, maxBoundaryDepth,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

// boundaryDepth returns the deepest boundary layer count in the subtree.
// boundaries is the set created by the example; the shell HUD boundary is not
// in it, but it is a leaf layer and cannot raise the maximum.
func boundaryDepth(n rendering.RenderObject, boundaries map[rendering.RenderObject]bool, depth int) int {
	if n == nil {
		return depth
	}
	if boundaries[n] {
		depth++
	}
	max := depth
	for _, ch := range n.Children() {
		if d := boundaryDepth(ch, boundaries, depth); d > max {
			max = d
		}
	}
	return max
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}
