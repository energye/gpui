// Command kit_f0_scope is the F0-1 real-window: theme plus scope.
//
// RUN_SECONDS=5 go run ./examples/kit_f0_scope
//
// Window: 1200x800 baseline, user-resizable. The run performs one
// scripted resize excursion and restores the baseline, so layout
// follows real EventResize traffic before any gate is read.
// Correctness window (RUN_SECONDS>=5): slope_gate=off, fps gate on
// (continuous tick), pixel probes F0+F6, golden zero-tolerance.
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
	panelBG = [3]float64{0.10, 0.11, 0.13}
	noteBG  = [3]float64{0.10, 0.11, 0.13}
	// disabledA is the exact blend of white under the
	// container-disabled token (alpha 0.04 over white).
	disabledA = [3]float64{0.96, 0.96, 0.96}
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

// themeColumn builds one vertical theme column: title, resolved swatch,
// disabled swatch, and a text-density caption. Colors come from the
// Ctx passed in, never from literals in this file.
func themeColumn(ctx kit.ScopeCtx, title string, bg theme.Color) *rendering.AbsoluteBox {
	col := rendering.NewAbsoluteBox(288, 560)
	col.Background = &rendering.Color{R: panelBG[0], G: panelBG[1], B: panelBG[2], A: 1}
	col.SetDebugName("f0-scope-" + title)
	col.Place(wrkit.Label(title, 12, 0.75, 0.85, 0.95), 12, 8)

	sw := rendering.NewRenderColorBox(264, 150, bg.R, bg.G, bg.B, 1)
	sw.SetRepaintBoundary(true)
	sw.SetDebugName("f0-scope-swatch-" + title)
	col.Place(sw, 12, 34)

	// Disabled row: white base plus the container-disabled token as a
	// translucent overlay, so the GPU blends it for real. Probing the
	// center proves the disabled token resolved and composited.
	white := rendering.NewRenderColorBox(264, 90, 1, 1, 1, 1)
	white.SetRepaintBoundary(true)
	white.SetDebugName("f0-scope-disabled-base-" + title)
	col.Place(white, 12, 196)
	d := ctx.Theme.ColorBgContainerDisabled
	dim := rendering.NewRenderColorBox(264, 90, d.R, d.G, d.B, d.A)
	dim.SetRepaintBoundary(true)
	dim.SetDebugName("f0-scope-disabled-" + title)
	col.Place(dim, 12, 196)

	capNote := rendering.NewAbsoluteBox(264, 250)
	capNote.Background = &rendering.Color{R: noteBG[0], G: noteBG[1], B: noteBG[2], A: 1}
	capNote.Place(wrkit.Label(title+" theme resolves", 11, 0.62, 0.72, 0.85), 8, 8)
	capNote.Place(wrkit.Label("props > ctx theme > seed", 10, 0.55, 0.65, 0.78), 8, 30)
	capNote.Place(wrkit.Label("size "+string(ctx.Size)+" dir "+string(ctx.Dir), 10, 0.55, 0.65, 0.78), 8, 52)
	col.Place(capNote, 12, 298)
	return col
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-scope")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit_f0_scope — theme plus scope", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// Three Ctx values, one per column: default, reskinned, compact.
	baseCtx := kit.DefaultScopeCtx()
	skinTokens := baseCtx.Theme
	skinTokens.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := baseCtx.WithTheme(skinTokens)
	compactCtx := baseCtx.WithSize(kit.ScopeSizeSmall)

	shell := wrkit.NewShell(winW, winH, "F0-1 theme plus scope — one switch, whole tree", []string{
		"left: default seed theme",
		"mid: reskinned primary",
		"right: compact size tier",
		"disable-all: whole tree dims",
		"resize once, restore baseline",
		"slope_gate=off (5s window)",
	})

	colA := themeColumn(baseCtx, "default", baseCtx.Theme.ColorPrimaryBg)
	colB := themeColumn(skinCtx, "reskin", skinCtx.Theme.ColorPrimaryBg)
	colC := themeColumn(compactCtx, "compact", compactCtx.Theme.ColorBgContainer)
	shell.Body.Place(colA, 8, 8)
	shell.Body.Place(colB, 304, 8)
	shell.Body.Place(colC, 600, 8)

	snapDir := filepath.Join("examples", "kit_f0_scope", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	// Probe geometry resolves from the layout chain after first layout.
	geoValid := false
	resizeEvents := 0
	ptSwatch := probePoint{want: [3]float64{baseCtx.Theme.ColorPrimaryBg.R, baseCtx.Theme.ColorPrimaryBg.G, baseCtx.Theme.ColorPrimaryBg.B}, tol: 8.0 / 255}
	ptDisabled := probePoint{want: [3]float64{disabledA[0], disabledA[1], disabledA[2]}, tol: 12.0 / 255}
	capTextBox := rect{}
	var goldenRects []rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		ax, ay := colA.Offset().X, colA.Offset().Y
		ptSwatch.x = bodyX + ax + 12 + 132
		ptSwatch.y = bodyY + ay + 34 + 75
		ptDisabled.x = bodyX + ax + 12 + 132
		ptDisabled.y = bodyY + ay + 196 + 45
		capTextBox = rect{x: bodyX + ax + 12 + 8, y: bodyY + ay + 298 + 8, w: 248, h: 70}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			{bodyX + ax, bodyY + ay, 288, 290},
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_scope.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_scope: close (%s)\n", win.Backend())
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
		// Scripted resize excursion: leave baseline, then restore it.
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
		shell.UpdateHUD("F0-scope", wrkit.PhaseSteady, app, gateOK,
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

	// Resize evidence: the scripted excursion ran and the window is back.
	finalW, finalH := winW, winH
	if ctl != nil {
		if w, h := ctl.Size(); w > 0 && h > 0 {
			finalW, finalH = w, h
		}
	}

	resolveGeometry()
	finalChecks := []pixelCheck{
		{pt: &ptSwatch, desc: "default_bg_swatch"},
		{pt: &ptDisabled, desc: "disabled_container_dim"},
		{text: &capTextBox, base: noteBG, desc: "caption_text_density"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_scope.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-scope",
		Scenario:      "kit_f0_scope",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"scope_default_size":     string(baseCtx.Size),
			"scope_skin_primary":     "#722ed1",
			"scope_compact_size":     string(compactCtx.Size),
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
	fmt.Fprintf(os.Stderr, "kit_f0_scope: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
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
			fmt.Fprintf(os.Stderr, "kit_f0_scope: pixel %-24s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
				c.desc, c.pt.x, c.pt.y, r, g, b, c.pt.want[0], c.pt.want[1], c.pt.want[2], ok)
		} else if c.text != nil {
			n := textPixels(img, dpr, *c.text, c.base)
			ok = img != nil && n >= 120
			fmt.Fprintf(os.Stderr, "kit_f0_scope: pixel %-24s text_px=%d (want >=120) ok=%v\n", c.desc, n, ok)
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
	cur := filepath.Join(snapDir, "showcase_scope.png")
	base := filepath.Join(snapDir, "showcase_scope_base.png")
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
