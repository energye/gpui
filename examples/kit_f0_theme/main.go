// Command kit_f0_theme is the F0 theme-switch real window: one button row
// under three token sets (default/skinned/compact), one global switch,
// whole-tree disable and size switch.
//
// THEME=default|skinned|compact RUN_SECONDS=10 go run ./examples/kit_f0_theme
//
// Every run starts on the default set, then applies $THEME once via a
// single theme.Default.SetBase call. All three buttons re-resolve from
// the provider; no component code changes per theme (re-skinning by
// editing component code is a FAIL per F0 §5.6).
//
// Window: 1200x800 baseline, user-resizable. One scripted resize excursion
// restores the baseline before gates are read. Correctness window:
// slope_gate=off, fps gate on when steady.
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

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var scripted = map[string]bool{}

// themeSets builds the three token sets. Default is the shipped seed;
// skinned swaps the primary family (purple); compact shrinks type and
// control height. All values flow through the provider; components only
// read theme.Default.Current().
func themeSets() map[string]theme.Tokens {
	base := theme.DefaultTokens()
	skinned := base
	purple := theme.Hex("#722ed2")
	skinned.ColorPrimary = purple
	skinned.ColorPrimaryHover = theme.Hex("#9254de")
	skinned.ColorPrimaryActive = theme.Hex("#531dab")
	skinned.ColorPrimaryBorder = theme.Hex("#d3adf7")
	compact := base
	compact.FontSize = 12
	compact.ControlHeight = 24
	compact.ControlHeightSM = 16
	compact.Padding = 12
	return map[string]theme.Tokens{
		"default": base,
		"skinned": skinned,
		"compact": compact,
	}
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-theme")
	} else if *autoOnly {
		secs, secsSet = 10, true
	}
	themeName := os.Getenv("THEME")
	sets := themeSets()
	if _, ok := sets[themeName]; !ok {
		themeName = "default"
	}
	_, faceName, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	hasFace := faceName != ""

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit_f0_theme — " + themeName, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "F0 theme — one switch, whole tree follows ("+themeName+")", []string{
		"3 buttons x 1 global theme",
		"switch = single SetBase",
		"whole-tree disable switch",
		"size switch follows tokens",
		"resize once, restore baseline",
		"slope_gate=off (10s window)",
	})

	cur := theme.DefaultTokens()
	normalH := cur.ControlHeight
	setBox := func(b *rendering.RenderColorBox, c theme.Color) {
		b.R, b.G, b.B, b.A = c.R, c.G, c.B, 1
		b.MarkNeedsPaint()
	}

	// One behavior machine per button: disabled must swallow taps.
	btns := make([]*kit.BehaviorInteractive, 3)
	boxes := make([]*rendering.RenderColorBox, 3)
	btnX := []float64{16, 252, 488}
	const btnY, btnW = 40.0, 220.0
	for i := range btns {
		btns[i] = kit.NewBehaviorInteractive(kit.BehaviorInteractiveConfig{Focusable: false, Label: fmt.Sprintf("btn%d", i)})
		boxes[i] = rendering.NewRenderColorBox(btnW, normalH, cur.ColorPrimary.R, cur.ColorPrimary.G, cur.ColorPrimary.B, 1)
		boxes[i].SetRepaintBoundary(true)
		boxes[i].SetDebugName(fmt.Sprintf("f0-theme-btn%d", i))
		shell.Body.Place(boxes[i], btnX[i], btnY)
	}
	shell.Body.Box.Place(wrkit.Label("same button x 3, one global theme", 11, 0.62, 0.72, 0.85), 16, 20)

	face := wrkit.FaceAt(14)
	infoLabel := func(s string) *rendering.RenderText {
		t := wrkit.Label(s, 13, 0.88, 0.92, 0.96)
		if face != nil {
			wrkit.ApplyFace(t, 13)
		}
		t.SetRepaintBoundary(true)
		return t
	}
	themeText := infoLabel("theme: default")
	themeText.SetDebugName("f0-theme-name")
	shell.Body.Place(themeText, 760, 40)
	stateText := infoLabel("state: enabled")
	stateText.SetDebugName("f0-theme-state")
	shell.Body.Place(stateText, 760, 90)
	sizeText := infoLabel(fmt.Sprintf("size: %.0f", normalH))
	sizeText.SetDebugName("f0-theme-size")
	shell.Body.Place(sizeText, 760, 140)
	shell.Body.Box.Place(wrkit.Label("theme disable size", 11, 0.62, 0.72, 0.85), 760, 20)
	noteBG := [3]float64{0.10, 0.11, 0.13}

	snapDir := filepath.Join("examples", "kit_f0_theme", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	var goldenRects []rect
	var probeRects []rect
	var densityRect rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		tok := theme.Default.Current()
		h := tok.ControlHeight
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		probeRects = probeRects[:0]
		for i := range boxes {
			probeRects = append(probeRects, rect{x: bodyX + btnX[i], y: bodyY + btnY, w: btnW, h: h})
		}
		densityRect = rect{x: bodyX + 760, y: bodyY + 40, w: 300, h: 110}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			probeRects[0], probeRects[1], probeRects[2],
		}
		geoValid = true
	}

	var elapsed float64
	resizeDone, resizeBack := false, false
	phTheme, phDisable, phEnable, phSmall, phRestore := false, false, false, false, false
	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_theme_"+themeName+".png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_theme: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					shell.Resize(float64(ev.Width), float64(ev.Height))
					geoValid = false
				}
			case platform.EventPointer:
				if ev.Pointer == platform.PointerUp {
					for _, b := range btns {
						b.DirectTap()
					}
					fmt.Fprintf(os.Stderr, "kit_f0_theme: pointer up taps=%d\n", btns[0].Clicks())
				}
				app.ScheduleFrame()
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	// setHeight resizes a box the runtime-safe way: field write plus
	// layout and paint dirty flags (absolute children never relayout
	// themselves).
	setHeight := func(b *rendering.RenderColorBox, h float64) {
		b.Height = h
		b.MarkNeedsLayout()
		b.MarkNeedsPaint()
	}

	// reResolve reads the provider once and applies it to the whole tree.
	// This is the only theme path: no per-button constants anywhere.
	// Colors and control height both follow; size-switch phases run
	// later and override height until restore.
	reResolve := func() {
		tok := theme.Default.Current()
		for i := range boxes {
			if btns[i].States().Has(kit.ScopeStateDisabled) {
				setBox(boxes[i], tok.ColorBgContainerDisabled)
			} else {
				setBox(boxes[i], tok.ColorPrimary)
			}
			setHeight(boxes[i], tok.ControlHeight)
		}
		geoValid = false
	}

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
		if !phTheme && elapsed >= 4.0 {
			phTheme = true
			// The single global switch: one SetBase, whole tree follows.
			theme.Default.SetBase(sets[themeName])
			reResolve()
			themeText.SetText("theme: " + themeName)
			tok := theme.Default.Current()
			allMatch := true
			for i := range boxes {
				c := tok.ColorPrimary
				b := boxes[i]
				if math.Abs(b.R-c.R) > 1e-9 || math.Abs(b.G-c.G) > 1e-9 || math.Abs(b.B-c.B) > 1e-9 {
					allMatch = false
				}
			}
			scripted["theme_global"] = allMatch
			scripted["size_follows_theme"] = boxes[0].Height == tok.ControlHeight
		}
		if phTheme && !phDisable && elapsed >= 6.0 {
			phDisable = true
			for _, b := range btns {
				b.SetDisabled(true)
			}
			reResolve()
			stateText.SetText("state: disabled")
			for _, b := range btns {
				b.DirectTap()
			}
			covered := true
			for _, b := range btns {
				if b.Clicks() != 0 {
					covered = false
				}
			}
			scripted["disable_covers"] = covered
		}
		if phDisable && !phEnable && elapsed >= 7.0 {
			phEnable = true
			for _, b := range btns {
				b.SetDisabled(false)
			}
			reResolve()
			stateText.SetText("state: enabled")
			btns[0].DirectTap()
			scripted["reenable_counts"] = btns[0].Clicks() == 1
		}
		if phEnable && !phSmall && elapsed >= 8.0 {
			phSmall = true
			tok := theme.Default.Current()
			for _, b := range boxes {
				setHeight(b, tok.ControlHeightSM)
			}
			sizeText.SetText(fmt.Sprintf("size: %.0f", tok.ControlHeightSM))
			scripted["size_switch"] = boxes[0].Height == tok.ControlHeightSM
			geoValid = false
		}
		if phSmall && !phRestore && elapsed >= 9.0 {
			phRestore = true
			tok := theme.Default.Current()
			for _, b := range boxes {
				setHeight(b, tok.ControlHeight)
			}
			sizeText.SetText(fmt.Sprintf("size: %.0f", tok.ControlHeight))
			scripted["size_restored"] = boxes[0].Height == tok.ControlHeight
			geoValid = false
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("F0-theme", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("resize=%d theme=%s t=%.1f", resizeEvents, themeName, elapsed))
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
	img := loadImage(filepath.Join(snapDir, "showcase_theme_"+themeName+".png"))
	tok := theme.Default.Current()
	probes := map[string]solidProbe{}
	for i, r := range probeRects {
		probes[fmt.Sprintf("btn%d_%s", i, themeName)] = solidProbe{box: r, want: [3]float64{tok.ColorPrimary.R, tok.ColorPrimary.G, tok.ColorPrimary.B}}
	}
	// Collapse to three stable keys for the gate count.
	runSolidChecks(img, map[string]solidProbe{
		"theme_btn0": probes[fmt.Sprintf("btn0_%s", themeName)],
		"theme_btn1": probes[fmt.Sprintf("btn1_%s", themeName)],
		"theme_btn2": probes[fmt.Sprintf("btn2_%s", themeName)],
	}, scripted)
	runDensityCheck(img, densityRect, noteBG, "info_density", scripted, hasFace)

	keys := []string{"theme_global", "size_follows_theme", "disable_covers", "reenable_counts", "size_switch", "size_restored", "theme_btn0", "theme_btn1", "theme_btn2", "info_density"}
	wantTotal := len(keys)
	scriptedOK := 0
	for _, k := range keys {
		if scripted[k] {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, themeName, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-theme-" + themeName,
		Scenario:      "kit_f0_theme",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"has_face":               hasFace,
			"face_name":              faceName,
			"theme":                  themeName,
			"primary_hex":            fmt.Sprintf("%.2f,%.2f,%.2f", tok.ColorPrimary.R, tok.ColorPrimary.G, tok.ColorPrimary.B),
			"control_height":         tok.ControlHeight,
			"scripted_ok":            scriptedOK,
			"scripted_total":         wantTotal,
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
	if scriptedOK != wantTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d\n", scriptedOK, wantTotal)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "  %-18s %v\n", k, scripted[k])
		}
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
	fmt.Fprintf(os.Stderr, "kit_f0_theme(%s): OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		themeName, app.PresentCount(), scriptedOK, wantTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type solidProbe struct {
	box  rect
	want [3]float64
}

func runSolidChecks(img image.Image, probes map[string]solidProbe, result map[string]bool) {
	for name, p := range probes {
		ok := img != nil && solidPointOK(img, p.box, p.want)
		fmt.Fprintf(os.Stderr, "kit_f0_theme: solid %-14s ok=%v\n", name, ok)
		result[name] = ok
	}
}

func solidPointOK(img image.Image, box rect, want [3]float64) bool {
	px := int(box.x + box.w/2)
	py := int(box.y + box.h/2)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
	return math.Abs(r-want[0]) <= 12.0/255 && math.Abs(g-want[1]) <= 12.0/255 && math.Abs(b-want[2]) <= 12.0/255
}

func runDensityCheck(img image.Image, box rect, base [3]float64, name string, result map[string]bool, hasFace bool) {
	n := textPixels(img, 1.0, box, base)
	ok := img != nil && hasFace && n >= 60
	fmt.Fprintf(os.Stderr, "kit_f0_theme: text %-14s ink_px=%d (want >=60) ok=%v\n", name, n, ok)
	result[name] = ok
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

func evaluateGolden(snapDir, themeName string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_theme_"+themeName+".png")
	base := filepath.Join(snapDir, "showcase_theme_"+themeName+"_base.png")
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
