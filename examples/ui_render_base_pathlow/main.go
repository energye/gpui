// Command ui_render_base_pathlow is the A-group real window for the path
// low-frequency APIs: FillPreserve / StrokePreserve / GetCurrentPoint
// (RENDER_FEATURE_COVERAGE §1 #3 / §3.1 item 1).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_render_base_pathlow   # gate run
//	go run ./examples/ui_render_base_pathlow                 # manual: lives until window close
//	go run ./examples/ui_render_base_pathlow -auto-only      # gate mode: JSON + exit code
//
// Evidence: logical probes (path state across ops; exact current point in
// device space), 7 pixel probes on the GPU snapshot, and a bitwise golden over
// header + legend + the four cards.
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
	"github.com/energye/gpui/ui/platform"
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
		wrkit.RequireMinRun(secs, "A-pathlow")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title:       "gpui ui_render_base_pathlow — FillPreserve · StrokePreserve · GetCurrentPoint",
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	probe := newPaintProbe()
	shell := wrkit.NewShell(winW, winH, "ui_render_base_pathlow — path low-frequency APIs", []string{
		"A FillPreserve: fill keeps path, stroke same path",
		"B StrokePreserve: stroke keeps path, fill on top",
		"C GetCurrentPoint: exact coords read back, dots",
		"D Fill consumes path, next Stroke draws nothing",
		"probe: path state + exact current point (device)",
		"pixel: 7 probes (fills, ring, dots, absence)",
		"golden: header+legend+4 cards bitwise",
		"gate: -auto-only / RUN_SECONDS, JSON + exit code",
	})
	buildPathLowScene(shell, probe)

	snapDir := filepath.Join("examples", "ui_render_base_pathlow", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_pathlow.png")

	resizeEvents := 0
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_render_base_pathlow: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_render_base_pathlow — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					fmt.Fprintf(os.Stderr, "ui_render_base_pathlow: resize %dx%d\n", ev.Width, ev.Height)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_pathlow — %dx%d resize #%d", ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var elapsed float64
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		_, okMap, cpCalls, cpExact := probe.snapshot()
		shell.UpdateHUD("A-pathlow", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("cp=%d/%d probe_ok=%d t=%.1f", cpExact, cpCalls, len(okMap), elapsed))
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

	// Probe geometry resolves against the live body origin, so resize cannot
	// silently point the samples somewhere else.
	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	checks := make([]pixelCheck, 0, len(probeSpecs))
	for _, s := range probeSpecs {
		ca := cardAt[s.card]
		checks = append(checks, pixelCheck{
			pt:   &probePoint{x: bx + ca[0] + s.lx, y: by + ca[1] + s.ly, want: s.want, tol: s.tol},
			desc: s.key,
		})
	}
	runPixelChecks(loadImage(snapPath), checks, pixelResult)
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}

	goldenRects := []rect{
		{x: 0, y: 0, w: winW, h: 48},
		{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
	}
	for _, ca := range cardAt {
		goldenRects = append(goldenRects, rect{x: bx + ca[0], y: by + ca[1], w: cardW, h: cardH})
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	probeRan, probeOKMap, cpCalls, cpExact := probe.snapshot()
	badKeys := failingKeys(probeRan, probeOKMap)
	probePass := len(badKeys) == 0 && cpCalls > 0 && cpExact == cpCalls

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "A-pathlow",
		Scenario:      "ui_render_base_pathlow",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			// Correctness window (no animation): fps/p95 are carried by A-J
			// but not gated here; the perf phase gates them.
			"fps_gate":               "off_correctness",
			"slope_gate":             "off",
			"apis":                   "FillPreserve,StrokePreserve,GetCurrentPoint",
			"probe_pass":             probePass,
			"probe_bad":              badKeys,
			"cp_calls":               cpCalls,
			"cp_exact":               cpExact,
			"pixel_probes_ok":        pixelOK,
			"pixel_probes_total":     len(pixelResult),
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"resize_events":          resizeEvents,
			"client_px":              fmt.Sprintf("%dx%d", finalW, finalH),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if !probePass {
		fail("path probes cp=%d/%d bad=%v", cpExact, cpCalls, badKeys)
	}
	if pixelOK != len(pixelResult) || len(pixelResult) == 0 {
		fail("pixel probes %d/%d ok", pixelOK, len(pixelResult))
	}
	if !goldenFirstRun && goldenDiffPct != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
			MinPresents:            1,
			RequireFullPaintPolicy: true,
		}); err != nil {
			fail("%v", err)
		}
		if snap.VSyncSource == "" {
			fail("vsync_source missing")
		}
	}
	fmt.Fprintf(os.Stderr,
		"ui_render_base_pathlow: OK presents=%d probes=%d/%d cp=%d/%d pixel=%d/%d golden=%.4f%% resize=%d %dx%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), len(probeKeys)-len(badKeys), len(probeKeys), cpExact, cpCalls,
		pixelOK, len(pixelResult), goldenDiffPct, resizeEvents, finalW, finalH,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
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
		fmt.Fprintf(os.Stderr, "ui_render_base_pathlow: pixel %-12s @(%4.0f,%4.0f) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
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
	cur := filepath.Join(snapDir, "showcase_pathlow.png")
	base := filepath.Join(snapDir, "showcase_pathlow_base.png")
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
