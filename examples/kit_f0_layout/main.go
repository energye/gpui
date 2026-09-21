// Command kit_f0_layout is the F0-2 layout real window: Row/Column/Stack/
// Wrap plus a virtual long-list segment, all positioned by prim math.
//
// RUN_SECONDS=5 go run ./examples/kit_f0_layout
//
// Window: 1200x800 baseline, user-resizable. One scripted resize
// excursion restores the baseline before gates are read.
// Correctness window: slope_gate=off, fps gate on, pixel probes F0.
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

func colorBox(w, h float64, c [3]float64, name string) *rendering.RenderColorBox {
	b := rendering.NewRenderColorBox(w, h, c[0], c[1], c[2], 1)
	b.SetRepaintBoundary(true)
	b.SetDebugName(name)
	return b
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-layout")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit_f0_layout — row column stack wrap", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	tok := ctx.Theme
	primary := [3]float64{tok.ColorPrimary.R, tok.ColorPrimary.G, tok.ColorPrimary.B}
	success := [3]float64{tok.ColorSuccess.R, tok.ColorSuccess.G, tok.ColorSuccess.B}
	warning := [3]float64{tok.ColorWarning.R, tok.ColorWarning.G, tok.ColorWarning.B}
	primaryBG := [3]float64{tok.ColorPrimaryBg.R, tok.ColorPrimaryBg.G, tok.ColorPrimaryBg.B}
	gap := tok.SizeXS

	shell := wrkit.NewShell(winW, winH, "F0-2 layout — prim math positions every band", []string{
		"row: fixed 100 + flex 1:2 + 8 gap",
		"column: three fixed rows",
		"stack: base + positioned chip",
		"wrap: chips flow two runs",
		"virtual: 1000 rows, mounts few",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	// Row band: fixed 100x48 + flex 1 + flex 2 inside 420 wide.
	rowW := 420.0
	rowItems := []kit.PrimFlexSpec{kit.PrimFixedSpec(100, 48), kit.PrimExpandedSpec(1), kit.PrimExpandedSpec(2)}
	rowGeo := kit.PrimFlexLayout(true, rowW, 64, gap, kit.PrimDirLTR, kit.PrimCrossStart, rowItems)
	rowHost := rendering.NewAbsoluteBox(rowGeo.Outer.Width, 64)
	rowHost.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	rowHost.SetDebugName("f0-row")
	rowBox0 := colorBox(rowGeo.Boxes[0].W, 48, primary, "f0-row-fixed")
	rowHost.Place(rowBox0, rowGeo.Boxes[0].X, 8)
	rowBox1 := colorBox(rowGeo.Boxes[1].W, 48, success, "f0-row-flex1")
	rowHost.Place(rowBox1, rowGeo.Boxes[1].X, 8)
	rowBox2 := colorBox(rowGeo.Boxes[2].W, 48, warning, "f0-row-flex2")
	rowHost.Place(rowBox2, rowGeo.Boxes[2].X, 8)
	shell.Body.Place(rowHost, 8, 28)
	shell.Body.Box.Place(wrkit.Label("Row fixed+flex", 11, 0.62, 0.72, 0.85), 8, 8)

	// Column band: three fixed rows inside 190 wide.
	colItems := []kit.PrimFlexSpec{kit.PrimFixedSpec(150, 34), kit.PrimFixedSpec(120, 44), kit.PrimFixedSpec(160, 30)}
	colGeo := kit.PrimFlexLayout(false, 1e9, 190, 10, kit.PrimDirLTR, kit.PrimCrossStart, colItems)
	colHost := rendering.NewAbsoluteBox(190, colGeo.Outer.Height)
	colHost.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	colHost.SetDebugName("f0-column")
	for i, b := range colGeo.Boxes {
		var c [3]float64
		switch i {
		case 0:
			c = primary
		case 1:
			c = success
		default:
			c = warning
		}
		cb := colorBox(b.W, b.H, c, fmt.Sprintf("f0-col-%d", i))
		colHost.Place(cb, b.X, b.Y)
	}
	shell.Body.Place(colHost, 440, 28)
	shell.Body.Box.Place(wrkit.Label("Column three rows", 11, 0.62, 0.72, 0.85), 440, 8)

	// Stack band: base 190x110 + positioned 70x34 chip.
	stackLayers := []kit.PrimStackSpec{kit.PrimStackedSpec(190, 110), kit.PrimPositionedSpec(70, 34, 24, 18, 70, 34)}
	stackGeo := kit.PrimStackLayout(rendering.Constraints{MaxWidth: 400, MaxHeight: 300}, stackLayers)
	stackHost := rendering.NewAbsoluteBox(stackGeo.Outer.Width, stackGeo.Outer.Height)
	stackHost.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	stackHost.SetDebugName("f0-stack")
	stackBase := colorBox(190, 110, primaryBG, "f0-stack-base")
	stackHost.Place(stackBase, 0, 0)
	stackChip := colorBox(70, 34, warning, "f0-stack-chip")
	stackHost.Place(stackChip, 24, 18)
	shell.Body.Place(stackHost, 642, 28)
	shell.Body.Box.Place(wrkit.Label("Stack base+chip", 11, 0.62, 0.72, 0.85), 642, 8)

	// Wrap band: five 120x30 chips inside 420 wide.
	wrapKids := []rendering.Size{{Width: 120, Height: 30}, {Width: 120, Height: 30}, {Width: 120, Height: 30}, {Width: 120, Height: 30}, {Width: 120, Height: 30}}
	wrapGeo := kit.PrimWrapLayout(rendering.Constraints{MaxWidth: 420, MaxHeight: 400}, gap, 10, kit.PrimCrossStart, wrapKids)
	wrapHost := rendering.NewAbsoluteBox(wrapGeo.Outer.Width, wrapGeo.Outer.Height)
	wrapHost.Background = &rendering.Color{R: 0.10, G: 0.11, B: 0.13, A: 1}
	wrapHost.SetDebugName("f0-wrap")
	for i, b := range wrapGeo.Boxes {
		cb := colorBox(b.W, b.H, primary, fmt.Sprintf("f0-wrap-%d", i))
		wrapHost.Place(cb, b.X, b.Y)
	}
	wrapY := 28.0 + 64 + 36
	shell.Body.Place(wrapHost, 8, wrapY)
	shell.Body.Box.Place(wrkit.Label("Wrap flow", 11, 0.62, 0.72, 0.85), 8, wrapY-20)

	// Virtual band: 1000 rows, 32px each, 300x170 viewport.
	vb := kit.PrimNewViewportBox(1000, 32, func(i int) rendering.RenderObject {
		shade := 0.16 + 0.02*float64(i%3)
		return rendering.NewRenderColorBox(300, 32, shade, shade+0.02, shade+0.05, 1)
	})
	vb.Viewport.FixedWidth, vb.Viewport.FixedHeight = 300, 170
	virtX, virtY := 440.0, wrapY
	shell.Body.Place(vb.Viewport, virtX, virtY)
	shell.Body.Box.Place(wrkit.Label("Virtual 1000 rows", 11, 0.62, 0.72, 0.85), virtX, virtY-20)

	snapDir := filepath.Join("examples", "kit_f0_layout", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	ptRow := probePoint{want: primary, tol: 8.0 / 255}
	ptStack := probePoint{want: warning, tol: 8.0 / 255}
	ptWrap := probePoint{want: primary, tol: 8.0 / 255}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		rx, ry := rowHost.Offset().X, rowHost.Offset().Y
		ptRow.x = bodyX + rx + rowGeo.Boxes[0].X + rowGeo.Boxes[0].W/2
		ptRow.y = bodyY + ry + 8 + 24
		sx, sy := stackHost.Offset().X, stackHost.Offset().Y
		ptStack.x = bodyX + sx + 24 + 35
		ptStack.y = bodyY + sy + 18 + 17
		wx, wy := wrapHost.Offset().X, wrapHost.Offset().Y
		ptWrap.x = bodyX + wx + wrapGeo.Boxes[0].X + 60
		ptWrap.y = bodyY + wy + wrapGeo.Boxes[0].Y + 15
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + rx, bodyY + ry, rowGeo.Outer.Width, 64},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_layout.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_layout: close (%s)\n", win.Backend())
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
		bind, total := rendering.LastVirtualBind()
		shell.UpdateHUD("F0-layout", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("resize=%d bind=%d/%d t=%.1f", resizeEvents, bind, total, elapsed))
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
	bind, total := rendering.LastVirtualBind()

	finalW, finalH := winW, winH
	if ctl != nil {
		if w, h := ctl.Size(); w > 0 && h > 0 {
			finalW, finalH = w, h
		}
	}

	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptRow, desc: "row_fixed_center"},
		{pt: &ptStack, desc: "stack_chip_center"},
		{pt: &ptWrap, desc: "wrap_chip_center"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_layout.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-layout",
		Scenario:      "kit_f0_layout",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"row_outer":              fmt.Sprintf("%.1fx%.1f", rowGeo.Outer.Width, rowGeo.Outer.Height),
			"col_outer":              fmt.Sprintf("%.1fx%.1f", 190.0, colGeo.Outer.Height),
			"stack_outer":            fmt.Sprintf("%.1fx%.1f", stackGeo.Outer.Width, stackGeo.Outer.Height),
			"wrap_outer":             fmt.Sprintf("%.1fx%.1f", wrapGeo.Outer.Width, wrapGeo.Outer.Height),
			"virtual_bind":           bind,
			"virtual_items":          total,
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
	if total > 0 && bind >= total {
		fmt.Fprintf(os.Stderr, "FAIL: virtual bind=%d not << items=%d\n", bind, total)
		os.Exit(1)
	}
	if !resizeBack || finalW != winW || finalH != winH {
		fmt.Fprintf(os.Stderr, "FAIL: resize round trip incomplete (back=%v %dx%d)\n", resizeBack, finalW, finalH)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "kit_f0_layout: OK presents=%d scripted=%d/%d golden=%.4f%% bind=%d/%d resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, bind, total, resizeBack, finalW, finalH, elapsedSec)
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
		fmt.Fprintf(os.Stderr, "kit_f0_layout: pixel %-20s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
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
	cur := filepath.Join(snapDir, "showcase_layout.png")
	base := filepath.Join(snapDir, "showcase_layout_base.png")
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
