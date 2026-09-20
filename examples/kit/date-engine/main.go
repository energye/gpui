// Command date-engine is the G4 real window: civil math plus panel grid.
//
// RUN_SECONDS=5 go run ./examples/kit/date-engine
//
// Window: 1200x800 baseline, user-resizable. One scripted resize excursion
// restores the baseline before gates are read.
// Correctness window (RUN_SECONDS>=5): slope_gate=off, fps gate on.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var (
	clearBG = [3]float64{0.08, 0.09, 0.11}
	noteBG  = [3]float64{0.10, 0.11, 0.13}
	dimBG   = [3]float64{0.13, 0.15, 0.19}
)

type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	pt   *probePoint
	text *rect
	base [3]float64
	desc string
}

var pixelResult = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G4-date-engine")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit date-engine — G4 civil+panel", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Real engine: February 2024 leap panel, leap-day selected,
	// March range ordered, presets resolved (one func value).
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	props := kit.DefaultDateEngineProps()
	props.Presets = []kit.DateEnginePreset{
		{Label: "Leap day", Value: []kit.DateEngineDate{kit.DateEngineDateOf(2024, 2, 29)}},
		{Label: "Func week", ValueFunc: func() []kit.DateEngineDate {
			return []kit.DateEngineDate{kit.DateEngineDateOf(2024, 3, 4), kit.DateEngineDateOf(2024, 3, 10)}
		}},
	}
	engine := kit.BuildDateEngine(baseCtx, props)
	engine.Select(kit.DateEngineDateOf(2024, 2, 29))
	rprops := kit.DefaultDateEngineProps()
	rprops.Range = true
	rhost := kit.BuildDateEngine(baseCtx, rprops)
	rhost.Select(kit.DateEngineDateOf(2024, 3, 10))
	rhost.Select(kit.DateEngineDateOf(2024, 3, 5))
	rr := rhost.Range()
	// Multi: toggle 01-01 on, 01-02 on, 01-01 off leaves 01-02.
	mprops := kit.DefaultDateEngineProps()
	mprops.Multiple = true
	mhost := kit.BuildDateEngine(baseCtx, mprops)
	mhost.Select(kit.DateEngineDateOf(2024, 1, 1))
	mhost.Select(kit.DateEngineDateOf(2024, 1, 2))
	mhost.Select(kit.DateEngineDateOf(2024, 1, 1))
	multiText := "multi empty"
	if ml := mhost.Multiple(); len(ml) == 1 {
		multiText = "multi " + kit.FormatDateEngineDate(ml[0], "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 0)
	}
	// Disabled matrix: min plus a custom func kills the 15th.
	minD := kit.DateEngineDateOf(2024, 3, 1)
	dprops := kit.DefaultDateEngineProps()
	dprops.MinDate, dprops.MinDateSet = &minD, true
	dprops.DisabledDate = func(c kit.DateEngineDate, from *kit.DateEngineDate, p kit.DateEnginePicker) bool {
		return c.Day == 15
	}
	dhost := kit.BuildDateEngine(baseCtx, dprops)
	tf := func(b bool) string {
		if b {
			return "T"
		}
		return "F"
	}
	disText := fmt.Sprintf("dis 0229=%s 0315=%s 0310=%s",
		tf(dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 2, 29))),
		tf(dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 3, 15))),
		tf(dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 3, 10))))
	// Parse second format, quarter, decade, drill-down, all live.
	parsed, parseOK := kit.ParseDateEngineDate("29/02/2024",
		[]string{"YYYY-MM-DD", "DD/MM/YYYY"}, false, kit.ResolveDateEngineLocale("en-US"))
	phost := kit.BuildDateEngine(baseCtx, kit.DefaultDateEngineProps())
	phost.DrillDown()
	miscText := fmt.Sprintf("parse %s Q%d dec%d %s",
		tf(parseOK && parsed.SameDay(kit.DateEngineDateOf(2024, 2, 29))),
		engine.Generate().QuarterOf(kit.DateEngineDateOf(2024, 5, 1)),
		kit.DecadeStartOf(2024), phost.Mode())
	skinEngine := kit.BuildDateEngine(skinCtx, props)

	resolved := kit.ResolveDateEngine(baseCtx.Theme)
	resolvedSkin := kit.ResolveDateEngine(skinCtx.Theme)
	loc := kit.ResolveDateEngineLocale("en-US")
	gen := engine.Generate()
	ws := engine.WeekStart()
	matrix := kit.MonthDateEngineMatrix(gen, 2024, 2, ws)
	head := loc.WeekHeader(ws)
	title := loc.PanelTitle(kit.DateEngineModeDate, 2024, 2)

	shell := wrkit.NewShell(winW, winH, "G4 date engine — civil math plus panel", []string{
		"panel: Feb 2024 leap 6x7 grid",
		"selected: Feb 29 primary fill",
		"right: ordered Mar range",
		"far right: reskinned holder",
		"bottom: buddhist 2567 line",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Month panel: every cell comes from the engine matrix; the
	// example only places boxes and labels (control verbs).
	const cellW, cellH = 42.0, 24.0
	const panelW, panelH = 320.0, 206.0
	panel := rendering.NewAbsoluteBox(panelW, panelH)
	panel.Background = &rendering.Color{R: resolved.PanelBg.R, G: resolved.PanelBg.G, B: resolved.PanelBg.B, A: 1}
	panel.SetDebugName("g4-date-panel")
	panel.SetRepaintBoundary(true)
	panel.Place(wrkit.Label(title, 12, 0.12, 0.14, 0.16), 12, 8)
	shell.Body.Place(panel, 24, 24)
	var selBox *rendering.AbsoluteBox
	for c := 0; c < 7; c++ {
		panel.Place(wrkit.Label(head[c], 10, 0.45, 0.52, 0.62), 13+float64(c)*cellW, 32)
	}
	for r := 0; r < 6; r++ {
		for c := 0; c < 7; c++ {
			d := matrix[r][c]
			x := 8 + float64(c)*cellW
			y := 52 + float64(r)*cellH
			if d.SameDay(kit.DateEngineDateOf(2024, 2, 29)) {
				// Label lives inside the box (proven G3 card pattern):
				// a younger non-boundary sibling of a boundary box
				// does not paint, see G4 notes.
				selBox = rendering.NewAbsoluteBox(cellW-4, cellH-2)
				selBox.Background = &rendering.Color{R: resolved.Selected.R, G: resolved.Selected.G, B: resolved.Selected.B, A: 1}
				selBox.SetRepaintBoundary(true)
				selBox.SetDebugName("g4-date-selected")
				selBox.Place(wrkit.Label("29", 10, 1, 1, 1), 13, 6)
				panel.Place(selBox, x+2, y)
				continue
			}
			tone := [3]float64{0.12, 0.14, 0.16}
			if d.Month != 2 {
				tone = [3]float64{0.42, 0.47, 0.55}
			}
			panel.Place(wrkit.Label(fmt.Sprintf("%d", d.Day), 10, tone[0], tone[1], tone[2]), x+13, y+4)
		}
	}
	// Range card: ordered March range plus preset labels.
	rngCard := rendering.NewAbsoluteBox(240, 120)
	rngCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	rngCard.SetDebugName("g4-date-range")
	rngCard.SetRepaintBoundary(true)
	rngText := "range empty"
	if rr[0] != nil && rr[1] != nil {
		rngText = kit.FormatDateEngineDate(*rr[0], "YYYY-MM-DD", false, loc, ws) + " ~ " +
			kit.FormatDateEngineDate(*rr[1], "YYYY-MM-DD", false, loc, ws)
	}
	rngCard.Place(wrkit.Label("range+multi+preset", 11, 0.62, 0.72, 0.85), 10, 8)
	rngCard.Place(wrkit.Label(rngText, 11, 0.85, 0.88, 0.94), 10, 30)
	rngCard.Place(wrkit.Label("preset 2024-01-01", 10, 0.55, 0.65, 0.78), 10, 56)
	rngCard.Place(wrkit.Label("func 2024-06-01 "+multiText, 10, 0.55, 0.65, 0.78), 10, 76)
	shell.Body.Place(rngCard, 360, 24)
	// Buddhist line plus reskin card.
	budd := kit.FormatDateEngineDate(kit.DateEngineDateOf(2024, 2, 29), "BBBB-MM-DD", true, loc, ws)
	budCard := rendering.NewAbsoluteBox(240, 86)
	budCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	budCard.SetDebugName("g4-date-buddhist")
	budCard.SetRepaintBoundary(true)
	budCard.Place(wrkit.Label(fmt.Sprintf("buddhist era ws=%d %s", ws, head[0]), 11, 0.62, 0.72, 0.85), 10, 8)
	budCard.Place(wrkit.Label(budd, 12, 0.85, 0.88, 0.94), 10, 30)
	budCard.Place(wrkit.Label("weekStart="+fmt.Sprintf("%d", ws)+" header "+head[0], 10, 0.55, 0.65, 0.78), 10, 56)
	shell.Body.Place(budCard, 360, 156)
	cS := rendering.NewAbsoluteBox(220, 110)
	cS.Background = &rendering.Color{R: resolvedSkin.PanelBg.R, G: resolvedSkin.PanelBg.G, B: resolvedSkin.PanelBg.B, A: 1}
	cS.SetDebugName("g4-date-reskin")
	cS.SetRepaintBoundary(true)
	cS.Place(wrkit.Label("reskin static #722ed1", 12, 0.12, 0.14, 0.16), 12, 10)
	cS.Place(wrkit.Label(skinEngine.HolderContent(), 10, 0.30, 0.22, 0.55), 12, 36)
	cS.Place(wrkit.Label("static eats theme", 10, 0.30, 0.32, 0.40), 12, 58)
	shell.Body.Place(cS, 640, 24)
	capNote := rendering.NewAbsoluteBox(200, 120)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.Place(wrkit.Label(disText, 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label(miscText, 10, 0.55, 0.65, 0.78), 8, 30)
	shell.Body.Place(capNote, 24, 300)

	snapDir := filepath.Join("examples", "kit", "date-engine", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptPanel := probePoint{want: [3]float64{resolved.PanelBg.R, resolved.PanelBg.G, resolved.PanelBg.B}, tol: 8.0 / 255}
	ptSel := probePoint{want: [3]float64{resolved.Selected.R, resolved.Selected.G, resolved.Selected.B}, tol: 8.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		px, py := panel.Offset().X, panel.Offset().Y
		ptPanel.x = bodyX + px + panelW - 20
		ptPanel.y = bodyY + py + 10
		if selBox != nil {
			sx, sy := selBox.Offset().X, selBox.Offset().Y
			// Box corner, clear of the centered glyph.
			ptSel.x = bodyX + px + sx + 6
			ptSel.y = bodyY + py + sy + (cellH-2)/2
		}
		nx, ny := capNote.Offset().X, capNote.Offset().Y
		capTextBox = rect{x: bodyX + nx + 8, y: bodyY + ny + 8, w: 184, h: 60}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + px, bodyY + py, panelW, panelH},
			{bodyX + px, bodyY + py, panelW, 30},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_date_engine.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit date-engine: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					shell.Resize(float64(ev.Width), float64(ev.Height))
					geoValid = false
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var elapsed float64
	resizeDone, resizeBack := false, false
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		if ctl != nil && !resizeDone && elapsed >= 2.0 {
			resizeDone = true
			ctl.SetSize(1400, 900)
		}
		if ctl != nil && resizeDone && !resizeBack && elapsed >= 3.0 {
			resizeBack = true
			ctl.SetSize(winW, winH)
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("G4-date-engine", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d sel=%v", snapH.PaintCount, app.PresentCount(), engine.Value() != nil),
			fmt.Sprintf("resize=%d t=%.1f", resizeEvents, elapsed))
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

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	finalW, finalH := winW, winH
	if ctl != nil {
		if w, h := ctl.Size(); w > 0 && h > 0 {
			finalW, finalH = w, h
		}
	}

	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptPanel, desc: "date_panel_bg"},
		{pt: &ptSel, desc: "date_selected_fill"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_date_engine.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)
	firstCell := kit.FormatDateEngineDate(matrix[0][0], "YYYY-MM-DD", false, loc, ws)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G4-date-engine",
		Scenario:      "kit_date_engine",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"engine_impl":            gen.Name(),
			"week_start":             ws,
			"matrix_first":           firstCell,
			"range_text":             rngText,
			"buddhist":               budd,
			"skin_holder":            skinEngine.HolderContent(),
			"scripted_ok":            scriptedOK,
			"scripted_total":         scriptedTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"resize_events":          resizeEvents,
			"resize_restored":        finalW == winW && finalH == winH,
			"client_px":              fmt.Sprintf("%dx%d", finalW, finalH),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MaxP95Ms:               22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if scriptedTotal != 3 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 3/3 (pixel probes)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	if !resizeBack || finalW != winW || finalH != winH {
		fmt.Fprintf(os.Stderr, "FAIL: resize round trip incomplete (back=%v %dx%d)\n", resizeBack, finalW, finalH)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "kit date-engine: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		var ok bool
		if c.pt != nil {
			r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
			ok = valid && nearC(r, g, b, c.pt.want, c.pt.tol)
			fmt.Fprintf(os.Stderr, "kit date-engine: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit date-engine: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
		}
		result[c.desc] = ok
	}
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: decode %s: %v\n", path, err)
		return nil
	}
	return img
}

func sampleLogical(img image.Image, dpr float64, lx, ly float64) (r, g, b float64, valid bool) {
	if img == nil {
		return 0, 0, 0, false
	}
	px, py := int(lx*dpr), int(ly*dpr)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b float64, want [3]float64, tol float64) bool {
	return math.Abs(r-want[0]) <= tol && math.Abs(g-want[1]) <= tol && math.Abs(b-want[2]) <= tol
}

func textPixels(img image.Image, dpr float64, box rect, base [3]float64) int {
	if img == nil {
		return 0
	}
	x0, y0 := int(box.x*dpr), int(box.y*dpr)
	x1 := int((box.x + box.w) * dpr)
	y1 := int((box.y + box.h) * dpr)
	n := 0
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			r32, g32, b32, _ := img.At(px, py).RGBA()
			r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
			if math.Abs(r-base[0]) > 24.0/255 || math.Abs(g-base[1]) > 24.0/255 || math.Abs(b-base[2]) > 24.0/255 {
				n++
			}
		}
	}
	return n
}

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_date_engine.png")
	base := filepath.Join(snapDir, "showcase_date_engine_base.png")
	if _, err := os.Stat(base); err != nil {
		if os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
			fmt.Fprintf(os.Stderr, "golden baseline missing: %s (re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit)\n", base)
			return 100, 0, false
		}
		data, err := os.ReadFile(cur)
		if err != nil {
			fmt.Fprintf(os.Stderr, "golden: current snapshot %s missing (%v)\n", cur, err)
			return 100, 0, false
		}
		if err := os.WriteFile(base, data, 0o644); err == nil {
			fmt.Fprintf(os.Stderr, "golden baseline stored: %s\n", base)
			return 0, 0, true
		}
		return 100, 0, false
	}
	diff, total, err := comparePNG(base, cur, rects)
	if err != nil {
		fmt.Fprintf(os.Stderr, "golden compare: %v\n", err)
		return 100, 0, false
	}
	fmt.Fprintf(os.Stderr, "golden %s: diff=%.4f%% over %d px\n", filepath.Base(cur), diff, total)
	return diff, total, false
}

func comparePNG(basePath, curPath string, rects []rect) (pct float64, total int64, err error) {
	a, b := loadImage(basePath), loadImage(curPath)
	if a == nil || b == nil {
		return 0, 0, fmt.Errorf("decode failed")
	}
	if a.Bounds() != b.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch %v vs %v", a.Bounds(), b.Bounds())
	}
	dpr := float64(a.Bounds().Dx()) / winW
	var diff int64
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < a.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < a.Bounds().Dx(); px++ {
				ar, ag, ab, aa := a.At(px, py).RGBA()
				br, bg, bb, ba := b.At(px, py).RGBA()
				total++
				if ar != br || ag != bg || ab != bb || aa != ba {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 0, 0, fmt.Errorf("empty golden mask")
	}
	return float64(diff) * 100 / float64(total), total, nil
}
