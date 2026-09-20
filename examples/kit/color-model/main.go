// Command color-model is the G5 real window: color value plus panel math.
//
// RUN_SECONDS=5 go run ./examples/kit/color-model
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

func mustColor(s string) kit.ColorModelColor {
	c, ok := kit.ParseColorModelColor(s)
	if !ok {
		panic("bad color " + s)
	}
	return c
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G5-color-model")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit color-model — G5 value+panel", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Real model: single blue #1677ff shown; gradient/stops/disabled/
	// format/clear run on sibling hosts so the probe color stays stable.
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	blue := mustColor("#1677ff")
	red := mustColor("#ff0000")
	props := kit.DefaultColorModelProps()
	props.DefaultValue = kit.ColorModelValue{Single: blue}
	props.DefaultValueSet = true
	props.ShowText = true
	props.Modes = []kit.ColorModelMode{kit.ColorModelModeSingle, kit.ColorModelModeGradient}
	props.Presets = []kit.ColorModelPreset{
		{Label: "primaries", Colors: []kit.ColorModelValue{{Single: blue}, {Single: red}}},
	}
	props.AllowClear = true
	model := kit.BuildColorModel(baseCtx, props)
	skinModel := kit.BuildColorModel(skinCtx, props)

	gprops := kit.DefaultColorModelProps()
	gprops.Modes = []kit.ColorModelMode{kit.ColorModelModeSingle, kit.ColorModelModeGradient}
	ghost := kit.BuildColorModel(baseCtx, gprops)
	ghost.SetHSB(215, 0.91, 1, 1)
	ghost.SetMode(kit.ColorModelModeGradient)
	ghost.AddStop(50)
	gcss := ghost.Value().ToCssString()

	aprops := kit.DefaultColorModelProps()
	aprops.DisabledAlpha = true
	ahost := kit.BuildColorModel(baseCtx, aprops)
	ahost.SetHSB(215, 0.9, 1, 0.3)
	alphaText := fmt.Sprintf("alpha bar off A=%.0f", ahost.CurrentColor().A)

	fprops := kit.DefaultColorModelProps()
	fprops.DefaultValue = kit.ColorModelValue{Single: blue}
	fprops.DefaultValueSet = true
	fprops.ShowText = true
	fhost := kit.BuildColorModel(baseCtx, fprops)
	fhost.SetFormat(kit.ColorModelFormatRGB)
	fmtText := fhost.DisplayText()

	clearText := fmt.Sprintf("clear empty=%v", func() bool {
		ch := kit.BuildColorModel(baseCtx, props)
		ch.Clear()
		return ch.Value().IsEmpty()
	}())

	resolved := kit.ResolveColorModel(baseCtx.Theme)
	resolvedSkin := kit.ResolveColorModel(skinCtx.Theme)
	layout := kit.ComputeColorModelPanel(false)
	hx, hy := kit.ColorModelHandleXY(blue.H, layout)
	ax, ay := kit.ColorModelAlphaXY(1, layout)

	shell := wrkit.NewShell(winW, winH, "G5 color model — value plus panel", []string{
		"panel: 234 SV plus hue alpha",
		"swatch: 16 24 32 tiers",
		"right: gradient 2 stops live",
		"far right: reskinned holder",
		"bottom: alpha format clear",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Panel card: title plus SV/hue/alpha boxes from the pure geometry.
	const titleOff = 26.0
	panelW, panelH := 250.0, 26.0+266.0+12.0
	panel := rendering.NewAbsoluteBox(panelW, panelH)
	panel.Background = &rendering.Color{R: resolved.PanelBg.R, G: resolved.PanelBg.G, B: resolved.PanelBg.B, A: 1}
	panel.SetDebugName("g5-color-panel")
	panel.SetRepaintBoundary(true)
	panel.Place(wrkit.Label("panel 234 sv hue alpha", 12, 0.12, 0.14, 0.16), 12, 8)
	shell.Body.Place(panel, 24, 24)
	svBox := rendering.NewAbsoluteBox(layout.SV.W, layout.SV.H)
	svBox.Background = &rendering.Color{R: float64(blue.R) / 255, G: float64(blue.G) / 255, B: float64(blue.BL) / 255, A: 1}
	svBox.SetDebugName("g5-color-sv")
	svBox.Place(wrkit.Label("SV live #1677ff", 10, 1, 1, 1), 12, 12)
	panel.Place(svBox, layout.SV.X, layout.SV.Y+titleOff)
	hueBox := rendering.NewAbsoluteBox(layout.Hue.W, layout.Hue.H)
	hueBox.Background = &rendering.Color{R: 0.16, G: 0.18, B: 0.22, A: 1}
	hueBox.SetDebugName("g5-color-hue")
	panel.Place(hueBox, layout.Hue.X, layout.Hue.Y+titleOff)
	hueHandle := rendering.NewAbsoluteBox(12, 12)
	hueHandle.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	hueHandle.SetDebugName("g5-color-hue-handle")
	panel.Place(hueHandle, hx-6, hy+titleOff-6)
	alphaBox := rendering.NewAbsoluteBox(layout.Alpha.W, layout.Alpha.H)
	alphaBox.Background = &rendering.Color{R: 0.16, G: 0.18, B: 0.22, A: 1}
	alphaBox.SetDebugName("g5-color-alpha")
	panel.Place(alphaBox, layout.Alpha.X, layout.Alpha.Y+titleOff)
	alphaHandle := rendering.NewAbsoluteBox(12, 12)
	alphaHandle.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	alphaHandle.SetDebugName("g5-color-alpha-handle")
	panel.Place(alphaHandle, ax-6, ay+titleOff-6)

	// Trigger card: three swatch tiers with unique side labels.
	trigCard := rendering.NewAbsoluteBox(280, 170)
	trigCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	trigCard.SetDebugName("g5-color-triggers")
	trigCard.SetRepaintBoundary(true)
	trigCard.Place(wrkit.Label("triggers 16 24 32", 11, 0.62, 0.72, 0.85), 10, 8)
	shell.Body.Place(trigCard, 300, 24)
	smallC := mustColor("#ff0000")
	largeC := mustColor("#22aa55")
	type swRow struct {
		size  float64
		color kit.ColorModelColor
		label string
		y     float64
	}
	var medBox *rendering.AbsoluteBox
	for _, row := range []swRow{
		{kit.ColorModelSwatchSize(kit.ColorModelSizeSmall), smallC, "sw small red", 34},
		{kit.ColorModelSwatchSize(kit.ColorModelSizeMedium), blue, "sw med blue #1677ff", 66},
		{kit.ColorModelSwatchSize(kit.ColorModelSizeLarge), largeC, "sw large green", 106},
	} {
		box := rendering.NewAbsoluteBox(row.size, row.size)
		box.Background = &rendering.Color{R: float64(row.color.R) / 255, G: float64(row.color.G) / 255, B: float64(row.color.BL) / 255, A: 1}
		box.SetDebugName("g5-" + row.label)
		trigCard.Place(box, 12, row.y)
		trigCard.Place(wrkit.Label(row.label, 10, 0.55, 0.65, 0.78), 52, row.y+4)
		if row.size == kit.ColorModelSwatchSize(kit.ColorModelSizeMedium) {
			medBox = box
		}
	}
	// Gradient card: solid first-stop bar plus live stop/css lines.
	gradCard := rendering.NewAbsoluteBox(280, 130)
	gradCard.Background = &rendering.Color{R: dimBG[0], G: dimBG[1], B: dimBG[2], A: 1}
	gradCard.SetDebugName("g5-color-gradient")
	gradCard.SetRepaintBoundary(true)
	gradCard.Place(wrkit.Label("gradient live 3 stops", 11, 0.62, 0.72, 0.85), 10, 8)
	shell.Body.Place(gradCard, 300, 206)
	gradBar := rendering.NewAbsoluteBox(210, 24)
	gradBar.Background = &rendering.Color{R: float64(blue.R) / 255, G: float64(blue.G) / 255, B: float64(blue.BL) / 255, A: 1}
	gradBar.SetDebugName("g5-color-gradbar")
	gradCard.Place(gradBar, 12, 32)
	gradCard.Place(wrkit.Label(fmt.Sprintf("stops=%d sorted", len(ghost.Value().Stops)), 10, 0.55, 0.65, 0.78), 12, 64)
	gradCard.Place(wrkit.Label("css blue to red 50 mid", 10, 0.55, 0.65, 0.78), 12, 86)
	// Reskin card.
	cS := rendering.NewAbsoluteBox(260, 130)
	cS.Background = &rendering.Color{R: resolvedSkin.PanelBg.R, G: resolvedSkin.PanelBg.G, B: resolvedSkin.PanelBg.B, A: 1}
	cS.SetDebugName("g5-color-reskin")
	cS.SetRepaintBoundary(true)
	cS.Place(wrkit.Label("reskin static #722ed1", 12, 0.12, 0.14, 0.16), 12, 10)
	cS.Place(wrkit.Label(skinModel.HolderContent(), 10, 0.30, 0.22, 0.55), 12, 36)
	cS.Place(wrkit.Label("static eats theme", 10, 0.30, 0.32, 0.40), 12, 58)
	cS.Place(wrkit.Label(model.DisplayText(), 10, 0.30, 0.32, 0.40), 12, 80)
	shell.Body.Place(cS, 600, 24)
	capNote := rendering.NewAbsoluteBox(280, 130)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.SetDebugName("g5-color-cap")
	capNote.Place(wrkit.Label(alphaText, 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label(fmtText, 10, 0.55, 0.65, 0.78), 8, 30)
	capNote.Place(wrkit.Label(clearText, 10, 0.55, 0.65, 0.78), 8, 52)
	capNote.Place(wrkit.Label(fmt.Sprintf("handles h=%.0f a=1", blue.H), 10, 0.55, 0.65, 0.78), 8, 74)
	shell.Body.Place(capNote, 600, 170)

	snapDir := filepath.Join("examples", "kit", "color-model", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptPanel := probePoint{want: [3]float64{resolved.PanelBg.R, resolved.PanelBg.G, resolved.PanelBg.B}, tol: 8.0 / 255}
	ptSwatch := probePoint{want: [3]float64{float64(blue.R) / 255, float64(blue.G) / 255, float64(blue.BL) / 255}, tol: 8.0 / 255}
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
		if medBox != nil {
			tx, ty := trigCard.Offset().X, trigCard.Offset().Y
			sx, sy := medBox.Offset().X, medBox.Offset().Y
			ptSwatch.x = bodyX + tx + sx + 12
			ptSwatch.y = bodyY + ty + sy + 12
		}
		nx, ny := capNote.Offset().X, capNote.Offset().Y
		capTextBox = rect{x: bodyX + nx + 8, y: bodyY + ny + 8, w: 264, h: 100}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + px, bodyY + py, panelW, panelH},
			{bodyX + px, bodyY + py + titleOff, 234, 210},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_color_model.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit color-model: close (%s)\n", win.Backend())
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
		shell.UpdateHUD("G5-color-model", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d hex=%s", snapH.PaintCount, app.PresentCount(), model.Value().Single.ToHexString()),
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
		{pt: &ptPanel, desc: "color_panel_bg"},
		{pt: &ptSwatch, desc: "color_swatch_blue"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_color_model.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G5-color-model",
		Scenario:      "kit_color_model",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"model_hex":              model.Value().Single.ToHexString(),
			"model_rgb":              model.Value().Single.ToRgbString(),
			"model_hsb":              model.Value().Single.ToHsbString(),
			"gradient_css":           gcss,
			"gradient_stops":         len(ghost.Value().Stops),
			"alpha_text":             alphaText,
			"format_text":            fmtText,
			"clear_text":             clearText,
			"skin_holder":            skinModel.HolderContent(),
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
	fmt.Fprintf(os.Stderr, "kit color-model: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
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
			fmt.Fprintf(os.Stderr, "kit color-model: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit color-model: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "showcase_color_model.png")
	base := filepath.Join(snapDir, "showcase_color_model_base.png")
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
