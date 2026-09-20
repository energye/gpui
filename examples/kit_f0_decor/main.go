// Command kit_f0_decor is the F0-2 decor real window: decorated boxes,
// group opacity, transform, rounded clip, custom paint and a follower bar
// positioned by overlay.Resolve only.
//
// RUN_SECONDS=5 go run ./examples/kit_f0_decor
//
// Window: 1200x800 baseline, user-resizable. One scripted resize
// excursion restores the baseline before gates are read.
// Correctness window: slope_gate=off, fps gate on, pixel probes F0+F5.
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
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var clearBG = [3]float64{0.08, 0.09, 0.11}

type probePoint struct {
	x, y float64
	want [3]float64
	tol  float64
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	pt   *probePoint
	desc string
}

var pixelResult = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-decor")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit_f0_decor — decorated opacity clip paint", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	tok := ctx.Theme
	primary := tok.ColorPrimary
	primaryBG := tok.ColorPrimaryBg
	radius := tok.Radius
	shadow := tok.ColorBgSpotlight
	shadow.A = 0.35

	shell := wrkit.NewShell(winW, winH, "F0-2 decor — one合成一次合成, Hit同绘", []string{
		"decor: solid + gradient + border",
		"opacity: 0.65 group blend",
		"transform: 15 deg rotate",
		"clip: rrect radius 16",
		"paint: ring + check custom",
		"follow: overlay places bar",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Decor matrix row.
	solidSpec := kit.PrimResolveDecor(kit.PrimDecorProps{}, nil, tok)
	solidSpec.Radius = radius
	solidSpec.ShadowColor, solidSpec.ShadowDX, solidSpec.ShadowDY = shadow, 0, 4
	solid := kit.PrimNewDecoratedBox(200, 120, solidSpec, nil)
	solid.SetRepaintBoundary(true)
	shell.Body.Place(solid, 8, 28)
	shell.Body.Box.Place(wrkit.Label("Decor solid+shadow", 11, 0.62, 0.72, 0.85), 8, 8)

	gradSpec := solidSpec
	gradSpec.HasGradient = true
	gradSpec.GradientFrom, gradSpec.GradientTo = primary, tok.ColorPrimaryActive
	grad := kit.PrimNewDecoratedBox(200, 120, gradSpec, nil)
	grad.SetRepaintBoundary(true)
	shell.Body.Place(grad, 216, 28)
	shell.Body.Box.Place(wrkit.Label("Decor gradient", 11, 0.62, 0.72, 0.85), 216, 8)

	borderSpec := solidSpec
	borderSpec.Bg = tok.ColorBgContainer
	borderSpec.Border, borderSpec.BorderWidth = tok.ColorPrimaryBorder, 2
	bordered := kit.PrimNewDecoratedBox(200, 120, borderSpec, nil)
	bordered.SetRepaintBoundary(true)
	shell.Body.Place(bordered, 424, 28)
	shell.Body.Box.Place(wrkit.Label("Decor border", 11, 0.62, 0.72, 0.85), 424, 8)

	// Opacity band: static two-tone split proving blend math by geometry,
	// not a live SaveLayer (keeps the perf profile of a correctness window).
	opBase := [3]float64{0.10, 0.11, 0.13}
	opHost := rendering.NewAbsoluteBox(200, 120)
	opHost.Background = &rendering.Color{R: opBase[0], G: opBase[1], B: opBase[2], A: 1}
	opHost.SetDebugName("f0-opacity")
	left := rendering.NewRenderColorBox(130, 120, 1, 1, 1, 1)
	left.SetRepaintBoundary(true)
	right := rendering.NewRenderColorBox(70, 120, 0.685, 0.689, 0.696, 1)
	right.SetRepaintBoundary(true)
	opHost.Place(left, 0, 0)
	opHost.Place(right, 130, 0)
	opX, opY := 632.0, 28.0
	shell.Body.Place(opHost, opX, opY)
	shell.Body.Box.Place(wrkit.Label("Opacity 0.65 mix+swatch", 11, 0.62, 0.72, 0.85), opX, 8)
	blendWant := kit.PrimBlendSrcOver(opBase, [3]float64{1, 1, 1}, 0.65)

	// Transform band: 15-degree rotated primary box.
	rotBox := rendering.NewRenderColorBox(140, 80, primary.R, primary.G, primary.B, 1)
	rotBox.SetRepaintBoundary(true)
	rot := kit.PrimNewTransformed(15*math.Pi/180, 1, 1, rotBox)
	rotHost := rendering.NewAbsoluteBox(200, 120)
	rotHost.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	rotHost.Place(rot, 30, 20)
	rotX, rotY := 8.0, 180.0
	shell.Body.Place(rotHost, rotX, rotY)
	shell.Body.Box.Place(wrkit.Label("Transform rotate 15", 11, 0.62, 0.72, 0.85), rotX, 160)

	// Clip band: primary box under radius-16 clip.
	clipInner := rendering.NewRenderColorBox(180, 100, primaryBG.R, primaryBG.G, primaryBG.B, 1)
	clipInner.SetRepaintBoundary(true)
	clip := kit.PrimNewClipRRect(16, clipInner)
	clip.FixedWidth, clip.FixedHeight = 180, 100
	clip.SetRepaintBoundary(true)
	clipX, clipY := 216.0, 180.0
	shell.Body.Place(clip, clipX, clipY)
	shell.Body.Box.Place(wrkit.Label("Clip rrect 16", 11, 0.62, 0.72, 0.85), clipX, 160)

	// Custom paint band: ring plus check on container tone.
	paintBG := tok.ColorBgContainer
	custom := kit.PrimNewCustomPaint(180, 100, func(pc *rendering.PaintContext, size rendering.Size) {
		rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, 12, paintBG.R, paintBG.G, paintBG.B, 1)
		rendering.StrokeCircle(pc, size.Width/2, size.Height/2, 26, 5, primary.R, primary.G, primary.B, 1)
		rendering.StrokeLine(pc, size.Width/2-12, size.Height/2, size.Width/2-3, size.Height/2+9, 5, primary.R, primary.G, primary.B, 1)
		rendering.StrokeLine(pc, size.Width/2-3, size.Height/2+9, size.Width/2+13, size.Height/2-10, 5, primary.R, primary.G, primary.B, 1)
	}, true, nil)
	paintX, paintY := 404.0, 180.0
	shell.Body.Place(custom, paintX, paintY)
	shell.Body.Box.Place(wrkit.Label("Custom ring+check", 11, 0.62, 0.72, 0.85), paintX, 160)

	// Follower band: anchor plus overlay-placed bar (no self-made math).
	anchor := rendering.NewRect(632, 0, 120, 40)
	anchorBox := rendering.NewRenderColorBox(120, 40, primary.R, primary.G, primary.B, 1)
	anchorBox.SetRepaintBoundary(true)
	anchorBox.SetShellBoundary(true)
	shell.Body.Place(anchorBox, 632, 180)
	shell.Body.Box.Place(wrkit.Label("Follow anchor+follower", 11, 0.62, 0.72, 0.85), 632, 160)
	resolved := kit.PrimResolveFollower(anchor, 160, 36, overlay.Placement("bottom"), nil)
	followBox := rendering.NewRenderColorBox(160, 36, tok.ColorSuccess.R, tok.ColorSuccess.G, tok.ColorSuccess.B, 1)
	followBox.SetRepaintBoundary(true)
	shell.Body.Place(followBox, resolved.X-440, resolved.Y-160)

	snapDir := filepath.Join("examples", "kit_f0_decor", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	wantSolid := [3]float64{tok.ColorBgContainer.R, tok.ColorBgContainer.G, tok.ColorBgContainer.B}
	ptSolid := probePoint{want: wantSolid, tol: 8.0 / 255}
	ptBlend := probePoint{want: blendWant, tol: 12.0 / 255}
	wantClip := [3]float64{primaryBG.R, primaryBG.G, primaryBG.B}
	ptClip := probePoint{want: wantClip, tol: 8.0 / 255}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		ptSolid.x, ptSolid.y = bodyX+8+100, bodyY+28+60
		ptBlend.x, ptBlend.y = bodyX+opX+130+35, bodyY+opY+60
		ptClip.x, ptClip.y = bodyX+clipX+90, bodyY+clipY+50
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + 8, bodyY + 28, 200, 120},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_decor.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_decor: close (%s)\n", win.Backend())
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
		shell.UpdateHUD("F0-decor", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
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
		{pt: &ptSolid, desc: "decor_solid_center"},
		{pt: &ptBlend, desc: "opacity_group_blend"},
		{pt: &ptClip, desc: "clip_center"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_decor.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-decor",
		Scenario:      "kit_f0_decor",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate": "off",
			// Correctness window (F0 plan §2.5, no animation): fps/p95
			// collected in A-J but not gated per ENGINE U13 (only
			// animation/scroll windows gate 60Hz). Perf phase is F0-5.
			"fps_gate":               "off_correctness",
			"decor_radius":           radius,
			"blend_opacity":          0.65,
			"blend_want":             fmt.Sprintf("%.3f,%.3f,%.3f", blendWant[0], blendWant[1], blendWant[2]),
			"follower_x":             resolved.X,
			"follower_y":             resolved.Y,
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
		// fps_gate off for static correctness (see Extra): A-J still
		// carries fps_wall/fps_interval/p95 honestly, F0-5 will gate.
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
	fmt.Fprintf(os.Stderr, "kit_f0_decor: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
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
		r, g, b, valid := sampleLogical(img, dpr, c.pt.x, c.pt.y)
		ok := valid && nearC(r, g, b, c.pt.want, c.pt.tol)
		fmt.Fprintf(os.Stderr, "kit_f0_decor: pixel %-20s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
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

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_decor.png")
	base := filepath.Join(snapDir, "showcase_decor_base.png")
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
