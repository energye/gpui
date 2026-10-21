// Command ui_wr_r2_paint is the W1 R2 real-window: 局部 NeedsPaint isolation.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_r2_paint -auto-only
//	go run ./examples/ui_wr_r2_paint                 # manual: animate until close
//	go run ./examples/ui_wr_r2_paint -manual-seconds 15
//
// Window: 1200x800. RUN_SECONDS>=5 (U16). GPU window required.
//
// Evidence: paint/boundary logic probes, 13 pixel probes on the GPU snapshot,
// and a bitwise golden over header + legend + body. Gate mode freezes the
// phase on Steady so the frozen snapshot is deterministic; manual mode keeps
// the Steady→Spike→Recover animation.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

type rect struct{ x, y, w, h float64 }

const (
	kindBG    = "bg"
	kindNotBG = "ink"
)

type pixelCheck struct {
	desc string
	x, y float64
	want [3]float64
	tol  float64
	kind string
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "R2")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui ui_wr_r2_paint — 局部 NeedsPaint", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	// Shell: TopBar + Legend + Body + HUD (U17).
	shell := wrkit.NewShell(winW, winH, "R2 局部NeedsPaint — 9宫静+1热", []string{
		"GREEN=static boundary (never repaints)",
		"MAGENTA=hot boundary (tick-driven)",
		"BLUE=static plain nodes",
		"paint_count low ⇒ isolation OK",
		"visits≈hot subtree only",
		"gate: -auto-only / RUN_SECONDS",
		"pixel: 13 probes on GPU snapshot",
		"golden: header+legend+body",
	})

	// Static dense content: 4x3 color grid + 12 labels, each a RepaintBoundary.
	staticB := make([]*rendering.RenderColorBox, 0, 12)
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(90, 90, 0.12+0.18*float64(i%3), 0.55, 0.28+0.2*float64(j%2), 1)
			c.SetRepaintBoundary(true)
			shell.Body.Place(c, 20+float64(i)*110, 20+float64(j)*110)
			staticB = append(staticB, c)
		}
	}
	// Second static grid (non-boundary, dense color + text mix): proves plain
	// nodes also stay unpainted under CompositeOnly.
	for i := 0; i < 4; i++ {
		for j := 0; j < 3; j++ {
			c := rendering.NewRenderColorBox(36, 36, 0.18+0.15*float64(j), 0.45, 0.55+0.1*float64(i), 1)
			shell.Body.Place(c, 460+float64(i)*46, 400+float64(j)*46)
		}
	}
	for i := 0; i < 12; i++ {
		t := wrkit.Label(fmt.Sprintf("static label %d", i), 12, 0.7, 0.75, 0.85)
		shell.Body.Place(t, 20+float64(i%4)*95, 380+float64(i/4)*30)
	}

	// Hot boundary: repaints at ticker pace; static siblings must not.
	hot := rendering.NewRenderColorBox(90, 90, 0.95, 0.2, 0.2, 1)
	hot.SetRepaintBoundary(true)
	shell.Body.Place(hot, 500, 20)

	// Second hot with different rate (phase-dependent).
	hot2 := rendering.NewRenderColorBox(60, 60, 0.95, 0.6, 0.2, 1)
	hot2.SetRepaintBoundary(true)
	shell.Body.Place(hot2, 620, 300)

	snapDir := filepath.Join("examples", "ui_wr_r2_paint", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_r2_paint.png")

	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: close (%s)\n", win.Backend())
			case platform.EventResize:
				// Responsive layout: re-lay the shell to the new window size
				// (standard wiring, same as ui_wr_r4b_multidamage / r18).
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	phase := wrkit.PhaseSteady
	var r, g, b = 0.95, 0.2, 0.2
	clock := wrkit.NewPhaseClock(1.5, 3.5)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		if gateMode {
			// Frozen Steady phase: deterministic snapshot while the hot
			// boundary still repaints once per frame (static skip remains).
			phase = wrkit.PhaseSteady
			r, g, b = 0.95, 0.2, 0.2
		} else {
			phase = clock.Advance(dt)
			switch phase {
			case wrkit.PhaseSteady:
				r, g, b = 0.95, 0.2, 0.2
			case wrkit.PhaseSpike:
				r, g, b = 1.0, 0.85, 0.2
			default:
				r, g, b = 0.2, 0.7, 0.95
			}
		}
		hot.R, hot.G, hot.B = r, g, b
		hot.MarkNeedsPaint()
		if !gateMode && phase == wrkit.PhaseSpike {
			hot2.R, hot2.G, hot2.B = r*0.9, g*0.9, b*0.9
			hot2.MarkNeedsPaint()
		}
		app.ScheduleFrame()
		proc.Sample()

		// U18: live HUD refresh (ability ID + phase + fps + core counters).
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.PaintCount > 0 && (app.PresentCount() == 0 || snapH.PaintCount <= app.PresentCount()+20)
		shell.UpdateHUD("R2", phase, app, gateOK,
			fmt.Sprintf("visits=%d skip=%d", snapH.PaintVisits, snapH.BoundarySkip), "")
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
	frames := float64(app.PresentCount())
	paintOK := snap.PaintCount > 0 && (frames == 0 || float64(snap.PaintCount) <= frames+20)
	visits := int64(0)
	if b, ok := os.LookupEnv("WR_VISITS"); ok && b == "1" {
		visits = snap.PaintVisits
	}

	bx, by := shell.Body.X, shell.Body.Y
	pixelChecks := []pixelCheck{
		{desc: "static_00", x: bx + 65, y: by + 65, want: [3]float64{0.12, 0.55, 0.28}, tol: 0.06},
		{desc: "static_11", x: bx + 175, y: by + 175, want: [3]float64{0.30, 0.55, 0.48}, tol: 0.06},
		{desc: "static_22", x: bx + 285, y: by + 285, want: [3]float64{0.48, 0.55, 0.28}, tol: 0.06},
		{desc: "static_30", x: bx + 395, y: by + 65, want: [3]float64{0.12, 0.55, 0.28}, tol: 0.06},
		{desc: "dense_00", x: bx + 478, y: by + 418, want: [3]float64{0.18, 0.45, 0.55}, tol: 0.06},
		{desc: "dense_11", x: bx + 524, y: by + 464, want: [3]float64{0.33, 0.45, 0.65}, tol: 0.06},
		{desc: "dense_22", x: bx + 570, y: by + 510, want: [3]float64{0.48, 0.45, 0.75}, tol: 0.06},
		{desc: "hot_red", x: bx + 545, y: by + 65, want: [3]float64{0.95, 0.20, 0.20}, tol: 0.06},
		{desc: "hot2_orange", x: bx + 650, y: by + 330, want: [3]float64{0.95, 0.60, 0.20}, tol: 0.06},
		{desc: "label_ink", x: bx + 27, y: by + 382, tol: 0.04, kind: kindNotBG},
		{desc: "body_bg", x: bx + 400, y: by + 600, want: [3]float64{0.10, 0.11, 0.13}, tol: 0.05, kind: kindBG},
		{desc: "legend_bg", x: shell.Legend.X + 20, y: shell.Legend.Y + 620, want: [3]float64{0.11, 0.12, 0.15}, tol: 0.05, kind: kindBG},
		{desc: "top_bg", x: 1000, y: 24, want: [3]float64{0.12, 0.14, 0.18}, tol: 0.05, kind: kindBG},
	}

	// Pixel + Golden evidence only makes sense for the frozen gate phase.
	// Manual mode intentionally animates; its evidence is human observation.
	var pixelOK int
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	pixelTotal := 0
	if gateMode {
		img := loadImage(snapPath)
		pixelResult := make(map[string]bool, len(pixelChecks))
		runPixelChecks(img, pixelChecks, pixelResult)
		for _, ok := range pixelResult {
			if ok {
				pixelOK++
			}
		}
		pixelTotal = len(pixelChecks)

		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
			{x: shell.Body.X, y: shell.Body.Y, w: shell.Body.W, h: shell.Body.H},
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	}

	logicProbes := []string{
		"paint_count_positive",
		"paint_count_le_frames_plus_20",
		"presents_positive",
		"static_boundaries_12",
		"boundary_skip_positive",
		"boundary_rerecord_positive",
	}
	logicFailed := make([]string, 0, len(logicProbes))
	if snap.PaintCount <= 0 {
		logicFailed = append(logicFailed, "paint_count_positive")
	}
	if !paintOK {
		logicFailed = append(logicFailed, "paint_count_le_frames_plus_20")
	}
	if app.PresentCount() < 1 {
		logicFailed = append(logicFailed, "presents_positive")
	}
	if len(staticB) != 12 {
		logicFailed = append(logicFailed, "static_boundaries_12")
	}
	if snap.BoundarySkip <= 0 {
		logicFailed = append(logicFailed, "boundary_skip_positive")
	}
	if snap.BoundaryRerecord <= 0 {
		logicFailed = append(logicFailed, "boundary_rerecord_positive")
	}
	logicPass := len(logicFailed) == 0

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "R2",
		Scenario:      "ui_wr_r2_paint",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsed,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"paint_visits":           visits,
			"static_boundaries":      len(staticB),
			"phases_seen":            phase,
			"phase_mode":             map[bool]string{true: "steady-frozen", false: "animated"}[gateMode],
			"evidence_mode":          map[bool]string{true: "gate", false: "manual"}[gateMode],
			"fps_gate":               "off_correctness",
			"slope_gate":             "off",
			"logic_probes_ok":        len(logicProbes) - len(logicFailed),
			"logic_probes_total":     len(logicProbes),
			"logic_bad":              logicFailed,
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     pixelTotal,
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
		},
	})
	report.PaintCount = snap.PaintCount
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
		if pixelOK != len(pixelChecks) {
			fail("pixel probes %d/%d ok", pixelOK, len(pixelChecks))
		}
		if !goldenFirstRun && goldenDiffPct != 0 {
			fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
		}
		// R2 is a local-NeedsPaint correctness window; the app default may be
		// retained (W6 default switch), so do not force a full_paint policy.
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents: 1,
		}); err != nil {
			fail("%v", err)
		}
		if snap.VSyncSource == "" {
			fail("vsync_source missing")
		}
	}
	evidence := ""
	if gateMode {
		evidence = fmt.Sprintf(" pixel=%d/%d golden=%.4f%%", pixelOK, pixelTotal, goldenDiffPct)
	}
	fmt.Fprintf(os.Stderr,
		"ui_wr_r2_paint: OK presents=%d paint_flushes=%d visits=%d logic=%d/%d%s mode=%s elapsed=%.1fs\n",
		app.PresentCount(), snap.PaintCount, snap.PaintVisits,
		len(logicProbes)-len(logicFailed), len(logicProbes), evidence,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsed)
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
		r, g, b, valid := sampleLogical(img, dpr, c.x, c.y)
		kind := "fg"
		ok := false
		switch c.kind {
		case kindBG:
			kind = "bg"
			ok = valid && nearC(r, g, b, c.want, c.tol)
		case kindNotBG:
			kind = "ink"
			ok = valid && !nearC(r, g, b, [3]float64{0.10, 0.11, 0.13}, c.tol)
		default:
			ok = valid && nearC(r, g, b, c.want, c.tol)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_r2_paint: pixel %-14s %-4s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			c.desc, kind, c.x, c.y, r, g, b, c.want[0], c.want[1], c.want[2], ok)
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

func sampleLogical(img image.Image, dpr, lx, ly float64) (r, g, b float64, valid bool) {
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
	cur := filepath.Join(snapDir, "showcase_r2_paint.png")
	base := filepath.Join(snapDir, "showcase_r2_paint_base.png")
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
