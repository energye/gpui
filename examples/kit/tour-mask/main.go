// Command tour-mask is the G3 real window: guided tour mask plus hole plus panel.
//
// RUN_SECONDS=5 go run ./examples/kit/tour-mask
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
	clearBG  = [3]float64{0.08, 0.09, 0.11}
	noteBG   = [3]float64{0.10, 0.11, 0.13}
	anchorBG = [3]float64{0.16, 0.20, 0.28}
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

// panelCard builds the guided panel. Colors come from resolved tour theme.
func panelCard(title string, bg theme.Color, w, h float64) *rendering.AbsoluteBox {
	card := rendering.NewAbsoluteBox(w, h)
	card.Background = &rendering.Color{R: bg.R, G: bg.G, B: bg.B, A: 1}
	card.SetDebugName("g3-tour-" + title)
	card.SetRepaintBoundary(true)
	card.Place(wrkit.Label(title, 12, 0.12, 0.14, 0.16), 12, 10)
	return card
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G3-tour-mask")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit tour-mask — G3 hole+panel", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Real tour: three steps, first two carry host-written targets.
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	props := kit.DefaultTourMaskProps()
	props.Open = true
	steps := []kit.TourMaskStep{
		{Title: "step 1", Description: "highlight follows target", Target: kit.TourMaskRect{X: 584, Y: 210, W: 200, H: 80}, TargetSet: true},
		{Title: "step 2", Description: "panel re-anchors", Target: kit.TourMaskRect{X: 700, Y: 420, W: 160, H: 90}, TargetSet: true},
		{Title: "step 3", Description: "no target centers"},
	}
	tour := kit.BuildTourMask(baseCtx, props, steps)
	skinTour := kit.BuildTourMask(skinCtx, props, steps[:1])

	resolved := kit.ResolveTourMask(baseCtx.Theme)
	resolvedSkin := kit.ResolveTourMask(skinCtx.Theme)
	mask := kit.TourMaskMaskColor(baseCtx.Theme, props)
	hole := kit.ComputeTourMaskHole(steps[0].Target, kit.DefaultTourMaskGap())
	panel := kit.ComputeTourMaskPanel(winW, winH, hole, steps[0].Target, kit.TourMaskPlacementBottom, kit.ResolveTourMaskZ(props))

	shell := wrkit.NewShell(winW, winH, "G3 tour mask — hole follows target", []string{
		"anchor: host-written target",
		"hole: target plus gap 6 r2",
		"panel: bottom 520 z1001",
		"far right: reskinned holder",
		"bottom: mask strip merged",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Anchor mirrors the host-written step0 target in body-local coords.
	anchor := rendering.NewAbsoluteBox(200, 80)
	anchor.Background = &rendering.Color{R: anchorBG[0], G: anchorBG[1], B: anchorBG[2], A: 1}
	anchor.SetDebugName("g3-tour-anchor")
	anchor.SetRepaintBoundary(true)
	anchor.Place(wrkit.Label("target step 1", 12, 0.75, 0.82, 0.92), 12, 10)
	shell.Body.Place(anchor, 300, 150)
	// Hole frame visualizes the expanded highlight (border tone via label).
	holeBox := rendering.NewAbsoluteBox(hole.W, hole.H)
	holeBox.Background = &rendering.Color{R: anchorBG[0], G: anchorBG[1], B: anchorBG[2], A: 1}
	holeBox.SetDebugName("g3-tour-hole")
	holeBox.SetRepaintBoundary(true)
	holeBox.Place(wrkit.Label("hole gap6 r2", 10, 0.55, 0.72, 0.95), 8, 8)
	shell.Body.Place(holeBox, 300-6, 150-6)
	// Panel placed from the pure geometry helper (viewport -> body local).
	bodyOX, bodyOY := 284.0, 60.0
	panelLX, panelLY := panel.X-bodyOX, panel.Y-bodyOY
	if panelLX < 8 {
		panelLX = 8
	}
	if panelLY < 8 {
		panelLY = 8
	}
	if panelLX+panel.W > 896 {
		panelLX = 896 - panel.W
	}
	if panelLY+panel.H > 648 {
		panelLY = 648 - panel.H
	}
	card := panelCard("step 1 bottom", resolved.PanelBg, panel.W, 120)
	shell.Body.Place(card, panelLX, panelLY)
	card.Place(wrkit.Label("1 / 3  Previous Next", 10, 0.30, 0.32, 0.40), 12, 60)
	cS := panelCard("reskin static #722ed1", resolvedSkin.PanelBg, 220, 110)
	shell.Body.Place(cS, 660, 480)
	cS.Place(wrkit.Label(skinTour.HolderContent(), 10, 0.30, 0.22, 0.55), 12, 36)
	cS.Place(wrkit.Label("static eats theme", 10, 0.30, 0.32, 0.40), 12, 58)
	maskStrip := rendering.NewRenderColorBox(600, 36, mask.R, mask.G, mask.B, mask.A)
	maskStrip.SetRepaintBoundary(true)
	maskStrip.SetDebugName("g3-tour-mask")
	shell.Body.Place(maskStrip, 140, 440)
	capNote := rendering.NewAbsoluteBox(200, 120)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.Place(wrkit.Label("hole blocks, mask closes", 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label("esc traps focus in panel", 10, 0.55, 0.65, 0.78), 8, 30)
	shell.Body.Place(capNote, 24, 300)

	snapDir := filepath.Join("examples", "kit", "tour-mask", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptPanel := probePoint{want: [3]float64{resolved.PanelBg.R, resolved.PanelBg.G, resolved.PanelBg.B}, tol: 8.0 / 255}
	// Mask strip is mask RGBA(0,0,0,0.45) over body 0.10,0.11,0.13.
	ptMask := probePoint{want: [3]float64{0.055, 0.0605, 0.0715}, tol: 12.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		ax, ay := card.Offset().X, card.Offset().Y
		ptPanel.x = bodyX + ax + panel.W/2
		ptPanel.y = bodyY + ay + 60
		mx, my := maskStrip.Offset().X, maskStrip.Offset().Y
		ptMask.x = bodyX + mx + 300
		ptMask.y = bodyY + my + 18
		nx, ny := capNote.Offset().X, capNote.Offset().Y
		capTextBox = rect{x: bodyX + nx + 8, y: bodyY + ny + 8, w: 184, h: 60}
		hx, hy := holeBox.Offset().X, holeBox.Offset().Y
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + hx, bodyY + hy, hole.W, hole.H},
			{bodyX + ax, bodyY + ay, panel.W, 120},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_tour_mask.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit tour-mask: close (%s)\n", win.Backend())
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
		shell.UpdateHUD("G3-tour-mask", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d open=%v cur=%d", snapH.PaintCount, app.PresentCount(), tour.IsOpen(), tour.Current()),
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
		{pt: &ptPanel, desc: "tour_panel_bg"},
		{pt: &ptMask, desc: "tour_mask_strip"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_tour_mask.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G3-tour-mask",
		Scenario:      "kit_tour_mask",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"tour_open":              tour.IsOpen(),
			"tour_current":           tour.Current(),
			"tour_steps":             tour.StepCount(),
			"tour_hole":              fmt.Sprintf("%.0f,%.0f,%.0f,%.0f", hole.X, hole.Y, hole.W, hole.H),
			"tour_panel":             fmt.Sprintf("%.0f,%.0f,%.0f", panel.X, panel.Y, panel.W),
			"tour_z":                 panel.ZIndex,
			"skin_holder":            skinTour.HolderContent(),
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
	fmt.Fprintf(os.Stderr, "kit tour-mask: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
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
			fmt.Fprintf(os.Stderr, "kit tour-mask: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit tour-mask: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "showcase_tour_mask.png")
	base := filepath.Join(snapDir, "showcase_tour_mask_base.png")
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
