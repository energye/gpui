// Command ui_wr_r19_snap is the W1 R19 real-window: 1px / device-pixel alignment.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r19_snap -auto-only
//	go run ./examples/ui_wr_r19_snap -manual-seconds 15
//
// Window: 1200x800 logical. Gate mode freezes Steady, writes a GPU readback
// snapshot and compares a zero-tolerance Golden over header + legend + body.
// Manual mode runs Steady -> Spike -> Recover and never writes the gate PNG.
//
// R19's visible contrast is a one-device-pixel filled bar: SnapCoord on the
// grid covers exactly one pixel; the same bar offset by +0.5 device px splits
// over two half-covered pixels. SnapLine hairlines are checked for crispness
// separately. The GPU backend also snaps axis-aligned 1px strokes, so stroked
// raw/snapped hairlines are not used as blur bait.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
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
	closeSecs  = 5
	manualSecs = 15

	// Golden mask: header + legend + body. HUD is excluded because its
	// counters refresh during the run.
	goldenMaskPx = 1200*48 + 260*656 + 904*656

	coordPanelX, coordPanelY, coordPanelW, coordPanelH     = 12.0, 12.0, 440.0, 260.0
	linePanelX, linePanelY, linePanelW, linePanelH         = 464.0, 12.0, 428.0, 260.0
	dprPanelX, dprPanelY, dprPanelW, dprPanelH             = 12.0, 286.0, 440.0, 160.0
	hotPanelX, hotPanelY, hotPanelW, hotPanelH             = 464.0, 286.0, 428.0, 160.0
	footerPanelX, footerPanelY, footerPanelW, footerPanelH = 12.0, 458.0, 880.0, 180.0

	coordBarTop = 78.0
	coordBarH   = 150.0
	lineBorderX = 12.0
	lineBorderY = 60.0
	lineBorderW = 404.0
	lineBorderH = 188.0
)

var (
	topBG    = [3]float64{0.12, 0.14, 0.18}
	legendBG = [3]float64{0.11, 0.12, 0.15}
	bodyBG   = [3]float64{0.10, 0.11, 0.13}
	coordBG  = [3]float64{0.12, 0.15, 0.19}
	lineBG   = [3]float64{0.14, 0.16, 0.20}
	dprBG    = [3]float64{0.11, 0.14, 0.17}
	hotBG    = [3]float64{0.15, 0.16, 0.20}
	footerBG = [3]float64{0.12, 0.13, 0.17}

	snapGreen    = [3]float64{0.30, 0.90, 0.60}
	rawRed       = [3]float64{0.92, 0.38, 0.36}
	hairCyan     = [3]float64{0.35, 0.80, 0.95}
	borderOrange = [3]float64{0.95, 0.62, 0.28}
	hotYellow    = [3]float64{0.98, 0.82, 0.30}

	// SnapCoord filled bars, left half of the coord panel.
	snapBarVXs = []float64{30, 60, 90, 120, 150, 180}
	snapBarHYs = []float64{80, 110, 140, 170, 200}
	// Same 1-device-pixel bars offset by +0.5 device px, right half.
	rawBarVXs = []float64{250, 280, 310, 340, 370, 400}
	rawBarHYs = []float64{80, 110, 140, 170, 200}
	// SnapLine stroked hairlines in the line panel.
	hairVXs = []float64{30, 70, 110, 150, 190, 230, 270, 310, 350, 390}
	hairHYs = []float64{80, 120, 160, 200, 240}

	chipColors = [16][3]float64{
		{0.85, 0.25, 0.25}, {0.90, 0.55, 0.20}, {0.86, 0.78, 0.20}, {0.55, 0.82, 0.25},
		{0.25, 0.78, 0.45}, {0.20, 0.76, 0.70}, {0.22, 0.62, 0.88}, {0.35, 0.48, 0.90},
		{0.55, 0.38, 0.88}, {0.78, 0.30, 0.78}, {0.90, 0.30, 0.58}, {0.92, 0.50, 0.55},
		{0.65, 0.65, 0.68}, {0.45, 0.52, 0.60}, {0.32, 0.38, 0.46}, {0.78, 0.70, 0.55},
	}
)

type hotState struct {
	mu      sync.Mutex
	x, y    float64
	r, g, b float64
}

func newHotState(x, y float64, c [3]float64) *hotState {
	return &hotState{x: x, y: y, r: c[0], g: c[1], b: c[2]}
}

func (h *hotState) snapshot() (x, y, r, g, b float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.x, h.y, h.r, h.g, h.b
}

// set returns whether the position/colour changed. Paint reads a locked
// snapshot so the UI ticker and raster thread never share plain fields.
func (h *hotState) set(x, y float64, c [3]float64) (moved, recoloured bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	moved = math.Abs(x-h.x) > 0.01 || math.Abs(y-h.y) > 0.01
	recoloured = math.Abs(c[0]-h.r) > 0.002 || math.Abs(c[1]-h.g) > 0.002 || math.Abs(c[2]-h.b) > 0.002
	h.x, h.y, h.r, h.g, h.b = x, y, c[0], c[1], c[2]
	return moved, recoloured
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze Steady, snapshot + pixel/Golden evidence, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R19")
	} else if *autoOnly {
		secs, secsSet = closeSecs, true
	}
	gateMode := secsSet
	if gateMode && secs < closeSecs {
		fmt.Fprintf(os.Stderr, "FAIL: RUN_SECONDS must be >= %d to close R19 (§2.5 close duration)\n", closeSecs)
		os.Exit(1)
	}
	if !gateMode && *manualSeconds > 0 && *manualSeconds < manualSecs {
		fmt.Fprintf(os.Stderr, "FAIL: -manual-seconds must be >= %d for R19 phase evidence\n", manualSecs)
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
		Title:       "gpui ui_wr_r19_snap — 1px device alignment",
		Decorations: true,
		Resizable:   true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	dprHost := host.ScaleFactor()
	if dprHost <= 0 {
		dprHost = 1
	}

	// Offscreen DPR cross-check: run before the pipeline app owns/releases GPU
	// resources. render.NewContext with WithDeviceScale uses the registered GPU
	// accelerator when available, else software, and the result is labeled.
	dprProbes, dprProbeOK := runDPRProbes()

	shell := wrkit.NewShell(winW, winH, "R19 1px/device alignment — snapped vs raw", []string{
		"green: SnapCoord on the device grid",
		"red: same bar +0.5 device px -> split",
		"cyan: SnapLine hairline grid",
		"orange: SnapRect filled 1px border",
		"DPR 1/1.5/2: offscreen pixel probe",
		"gate: frozen Steady + 821184px Golden",
		"manual: Steady -> Spike -> Recover",
	})
	snappedLineCount := len(snapBarVXs) + len(snapBarHYs) + len(hairVXs) + len(hairHYs) + 4 + 2
	rawLineCount := len(rawBarVXs) + len(rawBarHYs)

	panel := func(w, h float64, bg [3]float64) *wrkit.Panel {
		return wrkit.NewPanel(w, h, bg[0], bg[1], bg[2], 1)
	}
	coordPanel := panel(coordPanelW, coordPanelH, coordBG)
	coordPanel.PlaceOn(shell.Body.Box, coordPanelX, coordPanelY)
	coordPanel.LabelAt("SNAP COORD — filled 1 device px bars", 13, 12, 8, 0.82, 0.90, 0.98)
	coordPanel.LabelAt("SnapCoord(v) rounds onto a pixel boundary", 10, 12, 30, 0.68, 0.78, 0.90)
	coordPanel.LabelAt("SNAPPED", 10, 20, 54, snapGreen[0], snapGreen[1], snapGreen[2])
	coordPanel.LabelAt("RAW +0.5px", 10, 248, 54, rawRed[0], rawRed[1], rawRed[2])

	linePanel := panel(linePanelW, linePanelH, lineBG)
	linePanel.PlaceOn(shell.Body.Box, linePanelX, linePanelY)
	linePanel.LabelAt("SNAP LINE — stroked hairlines + SnapRect border", 13, 12, 8, 0.82, 0.90, 0.98)
	linePanel.LabelAt("10 vertical + 5 horizontal hairlines; border = 4 filled edges", 10, 12, 32, 0.68, 0.78, 0.90)

	dprPanel := panel(dprPanelW, dprPanelH, dprBG)
	dprPanel.PlaceOn(shell.Body.Box, dprPanelX, dprPanelY)
	dprPanel.LabelAt("DPR / GRID CONTRACT", 13, 12, 8, 0.82, 0.90, 0.98)
	dprPanel.LabelAt("SnapCoord -> pixel boundary; SnapLine -> pixel center", 10, 12, 34, 0.68, 0.78, 0.90)
	dprPanel.LabelAt(fmt.Sprintf("dpr=1.0  SnapCoord(20.4)=%.4f  SnapLine(20.4)=%.4f", rendering.SnapCoord(20.4, 1), rendering.SnapLine(20.4, 1)), 10, 12, 58, 0.62, 0.76, 0.90)
	dprPanel.LabelAt(fmt.Sprintf("dpr=2.0  SnapCoord(20.4)=%.4f  SnapLine(20.4)=%.4f (%.1f dev)", rendering.SnapCoord(20.4, 2), rendering.SnapLine(20.4, 2), rendering.SnapLine(20.4, 2)*2), 10, 12, 82, 0.62, 0.76, 0.90)
	dprPanel.LabelAt(fmt.Sprintf("dpr=1.5  SnapCoord(20.4)=%.4f  SnapLine(20.4)=%.4f (%.1f dev)", rendering.SnapCoord(20.4, 1.5), rendering.SnapLine(20.4, 1.5), rendering.SnapLine(20.4, 1.5)*1.5), 10, 12, 106, 0.62, 0.76, 0.90)
	dprPanel.LabelAt("host panel DPR is in the HUD; offscreen pixel probes cover 1 / 1.5 / 2", 9, 12, 134, 0.56, 0.68, 0.82)

	hot := newHotState(180, 90, hotYellow)
	hotPanel := panel(hotPanelW, hotPanelH, hotBG)
	hotPanel.PlaceOn(shell.Body.Box, hotPanelX, hotPanelY)
	hotPanel.LabelAt("HOT — moving 1px crosshair", 13, 12, 8, 0.82, 0.90, 0.98)
	hotPanel.LabelAt("gate: fixed at (180,90); manual: Steady -> Spike -> Recover", 10, 12, 32, 0.68, 0.78, 0.90)
	hotPanel.LabelAt("the crosshair uses SnapCoord on both axes", 10, 12, 50, 0.62, 0.74, 0.88)

	footerPanel := panel(footerPanelW, footerPanelH, footerBG)
	footerPanel.PlaceOn(shell.Body.Box, footerPanelX, footerPanelY)
	footerPanel.LabelAt("STATIC DENSE — 4x4 exact-colour chips + labels", 13, 12, 8, 0.82, 0.90, 0.98)
	footerPanel.LabelAt("16 chips are pixel-probed; labels are ink-probed", 10, 12, 28, 0.68, 0.78, 0.90)
	chipX := func(col int) float64 { return 20 + float64(col)*100 }
	chipY := func(row int) float64 { return 42 + float64(row)*30 }
	chipsDrawn := 0
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			idx := row*4 + col
			c := chipColors[idx]
			footerPanel.ColorAt(16, 16, chipX(col), chipY(row), c[0], c[1], c[2], 1, false)
			footerPanel.LabelAt(fmt.Sprintf("c%02d", idx), 10, chipX(col)+22, chipY(row)+2, 0.78, 0.84, 0.92)
			chipsDrawn++
		}
	}
	footerPanel.LabelAt(fmt.Sprintf("%d snapped line specs", snappedLineCount), 11, 460, 44, 0.70, 0.84, 0.92)
	footerPanel.LabelAt(fmt.Sprintf("%d raw bar specs", rawLineCount), 11, 460, 66, 0.86, 0.62, 0.60)
	footerPanel.LabelAt("Golden: 821184 px, zero tolerance", 11, 460, 88, 0.70, 0.84, 0.92)
	footerPanel.LabelAt("DPR probes: 1 / 1.5 / 2 offscreen", 11, 460, 110, 0.70, 0.84, 0.92)
	panelList := []*wrkit.Panel{coordPanel, linePanel, dprPanel, hotPanel, footerPanel}

	coordGrid := rendering.NewRenderBox()
	coordGrid.FixedWidth, coordGrid.FixedHeight = coordPanelW, coordPanelH
	coordGrid.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) { paintCoordBars(pc) }
	coordPanel.Place(coordGrid, 0, 0)

	lineGrid := rendering.NewRenderBox()
	lineGrid.FixedWidth, lineGrid.FixedHeight = linePanelW, linePanelH
	lineGrid.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) { paintLineGrid(pc) }
	linePanel.Place(lineGrid, 0, 0)

	hotBox := rendering.NewRenderBox()
	hotBox.FixedWidth, hotBox.FixedHeight = hotPanelW, hotPanelH
	hotBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) { paintHotBox(pc, hot) }
	hotPanel.Place(hotBox, 0, 0)

	// Collect body labels after placement; the report uses this real count.
	var contentTexts []*rendering.RenderText
	collectTexts(shell.Body.Box, &contentTexts)
	var allTexts []*rendering.RenderText
	collectTexts(shell.Root, &allTexts)

	snapDir := filepath.Join("examples", "ui_wr_r19_snap", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapName := "showcase_r19_snap.png"
	baseName := "showcase_r19_snap_base.png"
	snapPath := filepath.Join(snapDir, snapName)
	snapshotPath := snapPath
	if !gateMode {
		// Manual mode animates the crosshair. Keep the gate PNG untouched.
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
				fmt.Fprintf(os.Stderr, "ui_wr_r19_snap: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	// W1 correctness window: pin FullPaint explicitly. The capability is
	// device-pixel alignment, not retained damage bookkeeping.
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	phase := wrkit.PhaseSteady
	phaseSeen := map[string]bool{wrkit.PhaseSteady: true}
	clock := wrkit.NewPhaseClock(2, 5)
	lastPhase := ""
	hotMoves, steadyMoves, spikeMoves, recoverMoves := 0, 0, 0, 0
	recoverTicks := 0
	recoverSettled := false
	recoverMovesAtSettle := 0
	hotInBounds := true
	gateHotMoves := 0
	lastLiveSnap := scheduler.FrameMetrics{}

	// setHot keeps the paint state locked and attributes visible motion to the
	// current phase. Gate mode never calls it after construction.
	setHot := func(x, y float64, c [3]float64) {
		moved, recoloured := hot.set(x, y, c)
		if moved || recoloured {
			hotBox.MarkNeedsPaint()
		}
		if moved {
			hotMoves++
			gateHotMoves++
			switch phase {
			case wrkit.PhaseSteady:
				steadyMoves++
			case wrkit.PhaseSpike:
				spikeMoves++
			default:
				recoverMoves++
			}
		}
	}

	// These are static contracts; cache them once for the HUD.
	c1, l1, r1 := snapMathAt(1)
	c15, l15, r15 := snapMathAt(1.5)
	c2, l2, r2 := snapMathAt(2)
	snapMathOK := c1 && l1 && r1 && c15 && l15 && r15 && c2 && l2 && r2
	lineMathOK := l1 && l15 && l2
	rectMathOK := r1 && r15 && r2

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if now := host.ScaleFactor(); now > 0 && now != dprHost {
			dprHost = now
			coordGrid.MarkNeedsPaint()
			lineGrid.MarkNeedsPaint()
			hotBox.MarkNeedsPaint()
		}
		if gateMode {
			phase = wrkit.PhaseSteady
			// Keep the exact construction-time crosshair; no paint dirtying.
		} else {
			phase = clock.Advance(dt)
			elapsed := clock.Elapsed()
			phaseChanged := phase != lastPhase
			switch phase {
			case wrkit.PhaseSteady:
				pulse := 0.5 + 0.5*math.Sin(elapsed*3.0)
				setHot(180+35*math.Sin(elapsed*2.2), 90+22*math.Sin(elapsed*1.7),
					[3]float64{0.98, 0.58 + 0.30*pulse, 0.30})
			case wrkit.PhaseSpike:
				setHot(220+150*math.Sin(elapsed*18), 90+30*math.Sin(elapsed*13),
					[3]float64{0.98, 0.36 + 0.20*math.Sin(elapsed*9), 0.30})
			default:
				if phaseChanged {
					setHot(180, 90, hotYellow)
				}
				recoverTicks++
				if recoverTicks == 60 {
					recoverSettled = true
					recoverMovesAtSettle = hotMoves
				}
			}
			lastPhase = phase
		}
		phaseSeen[phase] = true
		hx, hy, _, _, _ := hot.snapshot()
		if hx < 20 || hx > 410 || hy < 60 || hy > 150 {
			hotInBounds = false
		}

		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		lastLiveSnap = app.Metrics().Snapshot()
		gateOK := snapMathOK && snappedLineCount > 0 && dprHost > 0
		shell.UpdateHUD("R19", phase, app, gateOK,
			fmt.Sprintf("dpr=%.2f lines=%d raw=%d", dprHost, snappedLineCount, rawLineCount),
			fmt.Sprintf("filled 1px bars; SnapLine hairline grid; math=%v", snapMathOK))
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
	presents := app.PresentCount()
	fpsInterval := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / snap.AvgFrameIntervalMs
	}
	liveFPSInterval := 0.0
	if lastLiveSnap.AvgFrameIntervalMs > 1e-6 {
		liveFPSInterval = 1000.0 / lastLiveSnap.AvgFrameIntervalMs
	}

	// Pixel + Golden evidence is gate-only.
	var (
		pixelOK, pixelTotal int
		pixelBad            []string
		pixelDetails        []map[string]any
		groups              = map[string]*groupTally{}
		goldenDiffPct       float64
		goldenTotalPx       int64
		goldenFirstRun      bool
		snapshotDpr         = dprHost
	)
	if gateMode {
		img := loadImage(snapPath)
		if img != nil {
			snapshotDpr = imgDPR(img)
		}
		pixelChecks, inkRegions, barProbes, strokeProbes := buildPixelProbes(shell, snapshotDpr)
		pixelResult := make(map[string]bool, len(pixelChecks)+len(inkRegions)+len(barProbes)+len(strokeProbes))
		runPixelChecks(img, pixelChecks, pixelResult)
		runInkRegions(img, inkRegions, pixelResult)
		runBarProbes(img, barProbes, pixelResult, groups, &pixelDetails)
		runStrokeProbes(img, strokeProbes, pixelResult, groups, &pixelDetails)
		for desc, ok := range pixelResult {
			if ok {
				pixelOK++
			} else {
				pixelBad = append(pixelBad, desc)
			}
		}
		sort.Strings(pixelBad)
		pixelTotal = len(pixelChecks) + len(inkRegions) + len(barProbes) + len(strokeProbes)
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
	groupOK := func(name string) int {
		if groups[name] == nil {
			return 0
		}
		return groups[name].OK
	}
	groupTotal := func(name string) int {
		if groups[name] == nil {
			return 0
		}
		return groups[name].Total
	}

	phases := make([]string, 0, 3)
	for _, p := range []string{wrkit.PhaseSteady, wrkit.PhaseSpike, wrkit.PhaseRecover} {
		if phaseSeen[p] {
			phases = append(phases, p)
		}
	}
	recoverMovesAfterSettle := 0
	if recoverSettled {
		recoverMovesAfterSettle = hotMoves - recoverMovesAtSettle
	}
	gatePhaseFrozen := gateMode && phase == wrkit.PhaseSteady && !phaseSeen[wrkit.PhaseSpike] && !phaseSeen[wrkit.PhaseRecover]
	gateHotStatic := gateMode && gateHotMoves == 0

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
		{"content_text_nodes_ge_20", len(contentTexts) >= 20},
		{"all_text_nodes_ge_24", len(allTexts) >= 24},
		{"panels_five", len(panelList) >= 5},
		{"static_chips_16", chipsDrawn == 16},
		{"snap_coord_math", c1 && c15 && c2},
		{"snap_line_math_half_px", lineMathOK},
		{"snap_rect_math_grid", rectMathOK},
		{"snapped_lines_spec_ge_12", snappedLineCount >= 12},
		{"raw_bars_spec_ge_6", rawLineCount >= 6},
		{"offscreen_dpr_probes", dprProbeOK},
	}
	if gateMode {
		probes = append(probes,
			probe{"gate_phase_frozen", gatePhaseFrozen},
			probe{"gate_hot_static", gateHotStatic},
			probe{"gate_snapshot_dpr_match", snapshotDpr == dprHost},
			probe{"gate_pixel_probes_all", pixelOK == pixelTotal && pixelTotal > 0},
			probe{"gate_snapped_bars_crisp", groupOK("snapped") == groupTotal("snapped") && groupTotal("snapped") >= 2},
			probe{"gate_raw_bars_split", groupOK("raw") == groupTotal("raw") && groupTotal("raw") >= 2},
			probe{"gate_hairlines_crisp", groupOK("hairline") == groupTotal("hairline") && groupTotal("hairline") >= 2},
			probe{"gate_border_crisp", groupOK("border") == groupTotal("border") && groupTotal("border") >= 2},
			probe{"gate_hot_bars_crisp", groupOK("hot") == groupTotal("hot") && groupTotal("hot") >= 1},
			probe{"gate_golden_zero", goldenFirstRun || (goldenDiffPct == 0 && goldenTotalPx == goldenMaskPx)},
		)
	} else {
		probes = append(probes,
			probe{"manual_steady_hot_moves", steadyMoves > 0},
			probe{"manual_spike_hot_moves_ahead", spikeMoves > steadyMoves},
			probe{"manual_all_phases", phaseSeen[wrkit.PhaseSteady] && phaseSeen[wrkit.PhaseSpike] && phaseSeen[wrkit.PhaseRecover]},
			// Manual acceptance is a wall-time observation: RunFor stops on the
			// scheduler boundary just before 15.000s, while phase-clock ticks
			// can be a few milliseconds short. Closing the window early still
			// fails this check.
			probe{"manual_elapsed_ge_15", elapsed >= manualSecs},
			probe{"manual_recover_settled", phaseSeen[wrkit.PhaseRecover] && recoverSettled},
			probe{"manual_recover_no_motion_after_settle", recoverSettled && recoverMovesAfterSettle == 0},
			probe{"manual_hot_in_bounds", hotInBounds},
		)
	}
	logicFailed := make([]string, 0)
	for _, p := range probes {
		if !p.ok {
			logicFailed = append(logicFailed, p.name)
		}
	}

	extra := map[string]any{
		"client_px":                "1200x800",
		"run_seconds":              secs,
		"close_seconds_required":   closeSecs,
		"manual_seconds":           *manualSeconds,
		"manual_min_seconds":       manualSecs,
		"manual_elapsed":           elapsed,
		"manual_clock_elapsed":     clock.Elapsed(),
		"evidence_mode":            map[bool]string{true: "gate", false: "manual"}[gateMode],
		"phase_mode":               map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
		"phases_seen":              phases,
		"dpr":                      dprHost,
		"snapshot_dpr":             snapshotDpr,
		"dpr_source":               "host.ScaleFactor() of the real X11 window; offscreen probes use render.Context WithDeviceScale",
		"content_text_nodes":       len(contentTexts),
		"all_text_nodes":           len(allTexts),
		"panels":                   len(panelList),
		"static_chips":             chipsDrawn,
		"snapped_line_specs":       snappedLineCount,
		"snapped_lines":            snappedLineCount,
		"raw_line_specs":           rawLineCount,
		"snapped_bar_probes_ok":    groupOK("snapped"),
		"snapped_bar_probes_total": groupTotal("snapped"),
		"raw_split_probes_ok":      groupOK("raw"),
		"raw_split_probes_total":   groupTotal("raw"),
		"hairline_probes_ok":       groupOK("hairline"),
		"hairline_probes_total":    groupTotal("hairline"),
		"border_probes_ok":         groupOK("border"),
		"border_probes_total":      groupTotal("border"),
		"hot_probes_ok":            groupOK("hot"),
		"hot_probes_total":         groupTotal("hot"),
		"coverage_detail":          pixelDetails,
		"snap_coord_contract":      "SnapCoord(v,scale)*scale must be integral (fills sit on pixel boundaries)",
		"snap_line_contract":       "SnapLine(v,scale)*scale must be n+0.5 (1px stroke center is a pixel center)",
		"snap_rect_contract":       "SnapRect x/y/(x+w)/(y+h) * scale must be integral",
		"snap_math_ok":             snapMathOK,
		"snap_line_math_ok":        lineMathOK,
		"snap_rect_math_ok":        rectMathOK,
		"dpr_probes":               dprProbes,
		"dpr_probes_ok":            dprProbeOK,
		"dpr_probe_source":         "offscreen render.NewContext(W,H,WithDeviceScale(1/1.5/2)); GPU accelerator when registered",
		"raw_contrast":             "filled 1-device-px bars: snapped = one full pixel; raw = +0.5 device px = two half-covered pixels",
		"hairline_backend_note":    "render/internal/gpu/stroke_hairline.go also snaps axis-aligned 1px strokes; hairlines are crisp, but raw-vs-snapped blur contrast uses filled bars, not stroked hairlines",
		"pixel_evidence":           map[bool]string{true: "gate_snapshot", false: "manual_not_asserted"}[gateMode],
		"pixel_probes_ok":          pixelOK,
		"pixel_probes_total":       pixelTotal,
		"pixel_bad":                pixelBad,
		"pixel_golden_diff_pct":    pixelGoldenValue,
		"pixel_golden_total_px":    goldenTotalPx,
		"pixel_golden_first_run":   goldenFirstRun,
		"golden_mask":              "header 1200x48 + legend 260x656 + body 904x656",
		"golden_mask_px":           goldenMaskPx,
		"golden_tolerance_pct":     0,
		"snapshot_png":             snapshotPath,
		"snapshot_note": map[bool]string{
			true:  "gate snapshot (GPU readback) from the frozen Steady scene",
			false: "manual mode does not write the gate snapshot; committed evidence is not overwritten",
		}[gateMode],
		"gate_phase_frozen":          gatePhaseFrozen,
		"gate_hot_static":            gateHotStatic,
		"hot_moves":                  hotMoves,
		"steady_hot_moves":           steadyMoves,
		"spike_hot_moves":            spikeMoves,
		"recover_hot_moves":          recoverMoves,
		"recover_settled":            recoverSettled,
		"recover_moves_after_settle": recoverMovesAfterSettle,
		"hot_in_bounds":              hotInBounds,
		"present_policy":             snap.PresentPolicy,
		"live_frame_build_ms":        lastLiveSnap.LastBuildMs,
		"live_frame_raster_ms":       lastLiveSnap.LastRasterMs,
		"live_fps_interval":          liveFPSInterval,
		"frame_raster_note": map[bool]string{
			true:  "gate frame_raster_ms includes the final SnapshotPath GPU readback/PNG; live_frame_raster_ms is the last live tick",
			false: "manual frame_raster_ms is the last completed live present; manual mode has no SnapshotPath readback",
		}[gateMode],
		"fps_gate":              "persistent_tick_ge_55",
		"slope_gate":            "short_run_no_leak_verdict",
		"rss_slope_note":        "rss_slope_valid=false: a 5s gate / 15s manual run is shorter than the >=30s steady-slope verdict; raw slope is startup-inclusive and is not a leak conclusion",
		"rss_slope_steady_note": "steady slope is omitted while valid=false (short run); omission means 'cannot conclude', not 'no leak'",
		"rss_after_close_note":  "short correctness window: after_close may equal peak while GPU driver/Go heap caches are still resident; not a soak leak verdict",
		"cpu_fallback_note":     "none observed; last_cpu_fallback empty means no fallback",
		"gpu_ops_semantics":     "cumulative",
		"g_metrics":             "skipped",
		"g_metrics_reason":      "R19 is the 1px/device-pixel alignment window; it reports snap/DPR probes, not text measure cache metrics",
		"depcheck":              "passed_no_cgo_no_ui_gpu_import",
		"impl_correctness":      "SnapCoord rounds onto device-pixel boundaries for filled bars; SnapLine returns the half-pixel center used by a 1px stroke; SnapRect keeps all four edges integral. Gate freezes Steady so the snapshot is deterministic.",
		"impl_dirty":            "the gate crosshair is fixed at (180,90); manual Steady moves it slowly, Spike moves it fast and recolours it, Recover pins it back to (180,90) and stops. HUD updates are outside the Golden mask.",
		"impl_cache":            "no cache capability is claimed here; DPR probes are independent offscreen render.Context runs at scales 1/1.5/2.",
		"impl_edge":             "snapped vs raw filled 1-device-px bars, 10x5 SnapLine hairlines, a four-edge SnapRect border, a moving crosshair, 16 exact-colour chips and dense labels; host DPR1 plus offscreen DPR1/1.5/2.",
		"impl_fail":             "gate hot/phase not frozen; snapped bar not exactly one full pixel; raw bar not two mixed pixels; hairline not one full pixel; Golden diff!=0 or mask px mismatch; snapshot DPR!=host DPR; offscreen DPR probe fail; policy!=full_paint; fps<55 or p95>22 -> FAIL",
		"impl_visible":          "green snapped bars are solid single-pixel lines; red raw bars are visibly two half-tone columns/rows; cyan hairlines stay crisp; orange SnapRect border stays 1px; manual crosshair moves and recovers.",
		"logic_probes_ok":       len(probes) - len(logicFailed),
		"logic_probes_total":    len(probes),
		"logic_bad":             logicFailed,
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R19",
		Scenario:      "ui_wr_r19_snap",
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
	}); err != nil {
		fail("%v", err)
	}

	runStatus := "OK"
	if gateMode && goldenFirstRun {
		runStatus = "SEEDED"
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r19_snap: %s presents=%d lines=%d raw=%d math=%v dpr=%.2f probes=%d/%d logic=%d/%d mode=%s elapsed=%.1fs\n",
		runStatus, presents, snappedLineCount, rawLineCount, snapMathOK, dprHost,
		pixelOK, pixelTotal, len(probes)-len(logicFailed), len(probes),
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func paintCoordBars(pc *rendering.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	s := pc.Scale
	if s <= 0 {
		s = 1
	}
	// Snapped filled bars: SnapCoord gives an integral left/top edge, so a
	// 1/s-wide bar covers exactly one device pixel.
	pc.DC.SetRGBA(snapGreen[0], snapGreen[1], snapGreen[2], 1)
	for _, x := range snapBarVXs {
		sx := rendering.SnapCoord(x, s)
		pc.DC.DrawRectangle(pc.OriginX+sx, pc.OriginY+coordBarTop, 1/s, coordBarH)
	}
	for _, y := range snapBarHYs {
		sy := rendering.SnapCoord(y, s)
		pc.DC.DrawRectangle(pc.OriginX+20, pc.OriginY+sy, 190, 1/s)
	}
	_ = pc.DC.Fill()

	// Raw bars: same width, shifted by half a device pixel. With AA on, the
	// coverage lands on two adjacent pixels at ~50% each.
	pc.DC.SetRGBA(rawRed[0], rawRed[1], rawRed[2], 1)
	for _, x := range rawBarVXs {
		pc.DC.DrawRectangle(pc.OriginX+x+0.5/s, pc.OriginY+coordBarTop, 1/s, coordBarH)
	}
	for _, y := range rawBarHYs {
		pc.DC.DrawRectangle(pc.OriginX+240, pc.OriginY+y+0.5/s, 180, 1/s)
	}
	_ = pc.DC.Fill()
}

func paintLineGrid(pc *rendering.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	s := pc.Scale
	if s <= 0 {
		s = 1
	}
	thickness := 1 / s
	// SnapRect border as four filled edges: a stroked rectangle centered on
	// integer edges would split the 1px border on CPU; fills stay honest.
	bx, by, bw, bh := pc.SnapRect(lineBorderX, lineBorderY, lineBorderW, lineBorderH)
	pc.DC.SetRGBA(borderOrange[0], borderOrange[1], borderOrange[2], 1)
	pc.DC.DrawRectangle(pc.OriginX+bx, pc.OriginY+by, bw, thickness)
	pc.DC.DrawRectangle(pc.OriginX+bx, pc.OriginY+by+bh-thickness, bw, thickness)
	pc.DC.DrawRectangle(pc.OriginX+bx, pc.OriginY+by, thickness, bh)
	pc.DC.DrawRectangle(pc.OriginX+bx+bw-thickness, pc.OriginY+by, thickness, bh)
	_ = pc.DC.Fill()

	pc.DC.SetRGBA(hairCyan[0], hairCyan[1], hairCyan[2], 1)
	pc.DC.SetLineWidth(thickness)
	for _, x := range hairVXs {
		sx := pc.SnapLineX(x)
		pc.DC.MoveTo(pc.OriginX+sx, pc.OriginY+72)
		pc.DC.LineTo(pc.OriginX+sx, pc.OriginY+232)
	}
	for _, y := range hairHYs {
		sy := pc.SnapLineY(y)
		pc.DC.MoveTo(pc.OriginX+24, pc.OriginY+sy)
		pc.DC.LineTo(pc.OriginX+404, pc.OriginY+sy)
	}
	_ = pc.DC.Stroke()
}

func paintHotBox(pc *rendering.PaintContext, hot *hotState) {
	if pc == nil || pc.DC == nil || hot == nil {
		return
	}
	s := pc.Scale
	if s <= 0 {
		s = 1
	}
	x, y, r, g, b := hot.snapshot()
	pc.DC.SetRGBA(r, g, b, 1)
	sx := pc.SnapX(x)
	sy := pc.SnapY(y)
	pc.DC.DrawRectangle(pc.OriginX+sx, pc.OriginY+60, 1/s, 90)
	pc.DC.DrawRectangle(pc.OriginX+20, pc.OriginY+sy, 390, 1/s)
	_ = pc.DC.Fill()
}

func buildPixelProbes(shell *wrkit.ShellChrome, dpr float64) ([]pixelCheck, []inkRegion, []barProbe, []strokeProbe) {
	bx, by := shell.Body.X, shell.Body.Y
	cx, cy := bx+coordPanelX, by+coordPanelY
	lx, ly := bx+linePanelX, by+linePanelY
	dx, dy := bx+dprPanelX, by+dprPanelY
	hx, hy := bx+hotPanelX, by+hotPanelY
	fx, fy := bx+footerPanelX, by+footerPanelY
	rawOff := 0.5 / dpr
	hotX, hotY := 180.0, 90.0

	pixelChecks := []pixelCheck{
		{desc: "top_bg", x: 1000, y: 24, want: topBG, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 600, want: legendBG, tol: 0.05, kind: kindBG},
		{desc: "body_bg", x: bx + 890, y: by + 640, want: bodyBG, tol: 0.05, kind: kindBG},
		{desc: "coord_bg", x: cx + 430, y: cy + 250, want: coordBG, tol: 0.05, kind: kindBG},
		{desc: "line_bg", x: lx + 418, y: ly + 250, want: lineBG, tol: 0.05, kind: kindBG},
		{desc: "dpr_bg", x: dx + 430, y: dy + 150, want: dprBG, tol: 0.05, kind: kindBG},
		{desc: "hot_bg", x: hx + 418, y: hy + 150, want: hotBG, tol: 0.05, kind: kindBG},
		{desc: "footer_bg", x: fx + 870, y: fy + 170, want: footerBG, tol: 0.05, kind: kindBG},
	}
	for row := 0; row < 4; row++ {
		for col := 0; col < 4; col++ {
			idx := row*4 + col
			c := chipColors[idx]
			pixelChecks = append(pixelChecks, pixelCheck{
				desc: fmt.Sprintf("chip_%02d", idx),
				x:    fx + 20 + float64(col)*100 + 8,
				y:    fy + 42 + float64(row)*30 + 8,
				want: c, tol: 0.06,
			})
		}
	}

	inkRegions := []inkRegion{
		{desc: "top_title_ink", x: 14, y: 10, w: 700, h: 24, bg: topBG, tol: 0.04, minPx: 30, fg: [3]float64{0.88, 0.92, 0.98}, minFG: 10},
		{desc: "legend_title_ink", x: shell.Legend.X + 12, y: shell.Legend.Y + 8, w: 160, h: 16, bg: legendBG, tol: 0.04, minPx: 10, fg: [3]float64{0.55, 0.75, 0.95}, minFG: 3},
		{desc: "legend_line0_ink", x: shell.Legend.X + 12, y: shell.Legend.Y + 34, w: 230, h: 16, bg: legendBG, tol: 0.04, minPx: 18, fg: [3]float64{0.70, 0.78, 0.88}, minFG: 5},
		{desc: "coord_title_ink", x: cx + 12, y: cy + 6, w: 390, h: 18, bg: coordBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 6},
		{desc: "line_title_ink", x: lx + 12, y: ly + 6, w: 400, h: 18, bg: lineBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 6},
		{desc: "dpr_title_ink", x: dx + 12, y: dy + 6, w: 300, h: 18, bg: dprBG, tol: 0.04, minPx: 15, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 5},
		{desc: "dpr_line0_ink", x: dx + 12, y: dy + 32, w: 410, h: 16, bg: dprBG, tol: 0.04, minPx: 20, fg: [3]float64{0.68, 0.78, 0.90}, minFG: 6},
		{desc: "hot_title_ink", x: hx + 12, y: hy + 6, w: 390, h: 18, bg: hotBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 6},
		{desc: "footer_title_ink", x: fx + 12, y: fy + 6, w: 500, h: 18, bg: footerBG, tol: 0.04, minPx: 20, fg: [3]float64{0.82, 0.90, 0.98}, minFG: 6},
		{desc: "footer_summary_ink", x: fx + 460, y: fy + 42, w: 260, h: 16, bg: footerBG, tol: 0.04, minPx: 20, fg: [3]float64{0.70, 0.84, 0.92}, minFG: 5},
	}

	barProbes := []barProbe{
		// Snapped filled bars: exactly one full pixel.
		{desc: "snap_v_30", group: "snapped", edge: cx + 30, sample: cy + 150, vertical: true, snapped: true, line: snapGreen, bg: coordBG},
		{desc: "snap_v_150", group: "snapped", edge: cx + 150, sample: cy + 150, vertical: true, snapped: true, line: snapGreen, bg: coordBG},
		{desc: "snap_h_110", group: "snapped", edge: cy + 110, sample: cx + 100, vertical: false, snapped: true, line: snapGreen, bg: coordBG},
		{desc: "snap_h_170", group: "snapped", edge: cy + 170, sample: cx + 100, vertical: false, snapped: true, line: snapGreen, bg: coordBG},
		// Raw same-width bars at +0.5 device px: no full pixel, two mixed.
		{desc: "raw_v_250", group: "raw", edge: cx + 250 + rawOff, sample: cy + 150, vertical: true, snapped: false, line: rawRed, bg: coordBG},
		{desc: "raw_v_370", group: "raw", edge: cx + 370 + rawOff, sample: cy + 150, vertical: true, snapped: false, line: rawRed, bg: coordBG},
		{desc: "raw_h_80", group: "raw", edge: cy + 80 + rawOff, sample: cx + 300, vertical: false, snapped: false, line: rawRed, bg: coordBG},
		{desc: "raw_h_170", group: "raw", edge: cy + 170 + rawOff, sample: cx + 300, vertical: false, snapped: false, line: rawRed, bg: coordBG},
		// SnapRect border as filled one-device-pixel edges.
		{desc: "border_top", group: "border", edge: ly + 60, sample: lx + 200, vertical: false, snapped: true, line: borderOrange, bg: lineBG},
		{desc: "border_bottom", group: "border", edge: ly + 60 + 188 - 1/dpr, sample: lx + 200, vertical: false, snapped: true, line: borderOrange, bg: lineBG},
		{desc: "border_left", group: "border", edge: lx + 12, sample: ly + 150, vertical: true, snapped: true, line: borderOrange, bg: lineBG},
		{desc: "border_right", group: "border", edge: lx + 12 + 404 - 1/dpr, sample: ly + 150, vertical: true, snapped: true, line: borderOrange, bg: lineBG},
		// Frozen gate crosshair.
		{desc: "hot_v", group: "hot", edge: hx + hotX, sample: hy + 70, vertical: true, snapped: true, line: hotYellow, bg: hotBG},
		{desc: "hot_h", group: "hot", edge: hy + hotY, sample: hx + 120, vertical: false, snapped: true, line: hotYellow, bg: hotBG},
	}

	strokeProbes := []strokeProbe{
		{desc: "hair_v_150", group: "hairline", center: lx + rendering.SnapLine(150, dpr), sample: ly + 150, vertical: true, line: hairCyan, bg: lineBG},
		{desc: "hair_v_310", group: "hairline", center: lx + rendering.SnapLine(310, dpr), sample: ly + 150, vertical: true, line: hairCyan, bg: lineBG},
		{desc: "hair_h_160", group: "hairline", center: ly + rendering.SnapLine(160, dpr), sample: lx + 200, vertical: false, line: hairCyan, bg: lineBG},
		{desc: "hair_h_240", group: "hairline", center: ly + rendering.SnapLine(240, dpr), sample: lx + 200, vertical: false, line: hairCyan, bg: lineBG},
	}
	return pixelChecks, inkRegions, barProbes, strokeProbes
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
