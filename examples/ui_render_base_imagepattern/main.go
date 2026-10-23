// Command ui_render_base_imagepattern is the B-group direct real window for
// the pattern/brush API: render.NewImagePattern (package function since S2b
// C2b, was Context.CreateImagePattern), SetFillPattern,
// SetStrokePattern, ImagePattern anchors/scales/opacity/clamp/transform,
// NewSolidPattern, the Pattern<->Brush helpers, ImagePattern.Source /
// NewImagePatternFromSource, and Paint.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_render_base_imagepattern -auto-only
//	go run ./examples/ui_render_base_imagepattern   # manual: lives until window close
//	go run ./examples/ui_render_base_imagepattern -manual-seconds 15
//
// Evidence: 17 logical contract probes, 25 pixel probes on the GPU snapshot,
// and a bitwise golden over header + legend + the four cards. The pixel oracle
// is the committed testdata atlas / checker formula / card BG, not the Pattern
// implementation under test.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
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

type rect struct{ x, y, w, h float64 }

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "B-pattern")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var err error
	patternSource, err = loadPatternSource()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: testdata:", err)
		os.Exit(1)
	}

	logicProbes, logicFailed := runPatternLogic()
	logicMissing := missingProbeKeys(logicProbes)
	logicPass := len(logicFailed) == 0 && len(logicMissing) == 0
	for _, k := range logicProbes {
		fmt.Fprintf(os.Stderr, "ui_render_base_imagepattern: logic %-26s pass=%v\n", k, !contains(logicFailed, k))
	}
	if len(logicMissing) > 0 {
		fmt.Fprintf(os.Stderr, "ui_render_base_imagepattern: logic probes never ran: %v\n", logicMissing)
	}
	logicOK := len(patternProbeKeys) - len(logicFailed)

	probeSpecs := patternPixProbes()

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title:       "gpui ui_render_base_imagepattern — Pattern · ImagePattern · stroke",
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	// legendProbeLineIdx is the probe-count line: its text is derived from the
	// suite size, so it moves whenever a probe is added. The golden mask skips
	// that band (see goldenRects) rather than freezing a number.
	legendProbeLineIdx := 4
	shell := wrkit.NewShell(winW, winH, "ui_render_base_imagepattern — legacy Pattern / ImagePattern", []string{
		"A  image pattern tile (16×16 crop)",
		"B  anchor / scale / opacity / clamp",
		"C  stroke pattern (ring, non-rect)",
		"D  custom + solid Pattern + Paint",
		fmt.Sprintf("probe: %d logic + %d pixel", len(patternProbeKeys), len(probeSpecs)),
		"golden: header + legend (probe line masked out) + 4 cards",
		"gate: -auto-only / RUN_SECONDS",
	})
	buildPatternScene(shell)

	snapDir := filepath.Join("examples", "ui_render_base_imagepattern", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_imagepattern.png")

	resizeEvents := 0
	manualEvents := 0
	runFor := time.Duration(secs) * time.Second
	if !gateMode && *manualSeconds > 0 {
		runFor = time.Duration(*manualSeconds * float64(time.Second))
	}
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor:       runFor,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "ui_render_base_imagepattern: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_render_base_imagepattern — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					fmt.Fprintf(os.Stderr, "ui_render_base_imagepattern: resize %dx%d\n", ev.Width, ev.Height)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_imagepattern — %dx%d resize #%d", ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			default:
				manualEvents++
				fmt.Fprintf(os.Stderr, "ui_render_base_imagepattern: event #%d type=%v\n", manualEvents, ev.Type)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_imagepattern — event #%d type=%v", manualEvents, ev.Type))
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
		shell.UpdateHUD("B-pattern", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("logic=%d/%d t=%.1f", logicOK, len(patternProbeKeys), elapsed))
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

	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	img := loadImage(snapPath)
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	pixelOK := 0
	for _, s := range probeSpecs {
		ca := cardAt[s.card]
		ax := bx + ca[0] + s.rect.x
		ay := by + ca[1] + s.rect.y
		px := int((ax + s.rect.w/2) * dpr)
		py := int((ay + s.rect.h/2) * dpr)
		r, g, b, valid := readPixel(img, px, py)
		ok := valid && nearC(r, g, b, s.want[0], s.want[1], s.want[2], s.tol)
		fmt.Fprintf(os.Stderr, "ui_render_base_imagepattern: pixel %-16s %-30s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			s.key, s.desc, px, py, r, g, b, s.want[0], s.want[1], s.want[2], ok)
		if ok {
			pixelOK++
		}
	}

	// The legend is compared in two bands with the probe-count line's band left
	// out: that line prints len(patternProbeKeys), so freezing it would make
	// every probe addition look like a pixel regression.
	probeLineY := shell.Legend.Y + wrkit.LegendLineY(legendProbeLineIdx)
	goldenRects := []rect{
		{x: 0, y: 0, w: winW, h: 48},
		{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: probeLineY - shell.Legend.Y},
		{x: shell.Legend.X, y: probeLineY + wrkit.LegendLineH, w: shell.Legend.W,
			h: shell.Legend.Y + shell.Legend.H - (probeLineY + wrkit.LegendLineH)},
	}
	for _, ca := range cardAt {
		goldenRects = append(goldenRects, rect{x: bx + ca[0], y: by + ca[1], w: cardW, h: cardH})
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "B-pattern",
		Scenario:      "ui_render_base_imagepattern",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"logic_probes_ok":          logicOK,
			"logic_probes_total":       len(patternProbeKeys),
			"logic_bad":                logicFailed,
			"logic_keys_missing":       logicMissing,
			"pixel_probes_ok":          pixelOK,
			"pixel_probes_total":       len(probeSpecs),
			"pixel_golden_diff_pct":    goldenDiffPct,
			"pixel_golden_total_px":    goldenTotalPx,
			"pixel_golden_first_run":   goldenFirstRun,
			"pattern_source":           "./testdata/pattern_atlas.png (32×16; left quadrants, right magenta)",
			"pattern_source_sha256_16": patternSourceSHA,
			"apis":                     "render.NewImagePattern,NewImagePatternFromSource,ImagePattern.Source,SetFillPattern,SetStrokePattern,ImagePattern.SetAnchor/SetScale/SetOpacity/SetClamp/SetTransform/ColorAt,NewSolidPattern,BrushFromPattern,PatternFromBrush,Paint.SetBrush/ColorAt/GetBrush/SolidColor",
			"fps_gate":                 "off_correctness",
			"slope_gate":               "off",
			"resize_events":            resizeEvents,
			"client_px":                fmt.Sprintf("%dx%d", finalW, finalH),
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
	if !logicPass {
		fail("logic probes %d/%d bad=%v missing=%v", logicOK, len(patternProbeKeys), logicFailed, logicMissing)
	}
	if pixelOK != len(probeSpecs) || len(probeSpecs) == 0 {
		fail("pixel probes %d/%d ok", pixelOK, len(probeSpecs))
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
		"ui_render_base_imagepattern: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% resize=%d %dx%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), logicOK, len(patternProbeKeys), pixelOK, len(probeSpecs),
		goldenDiffPct, resizeEvents, finalW, finalH,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

func contains(list []string, k string) bool {
	for _, s := range list {
		if s == k {
			return true
		}
	}
	return false
}

// missingProbeKeys returns required keys that never ran.
func missingProbeKeys(ran []string) []string {
	seen := map[string]bool{}
	for _, k := range ran {
		seen[k] = true
	}
	var bad []string
	for _, k := range patternProbeKeys {
		if !seen[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func readPixel(img image.Image, px, py int) (r, g, b float64, valid bool) {
	if img == nil || px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return 0, 0, 0, false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, true
}

func nearC(r, g, b, wr, wg, wb, tol float64) bool {
	return math.Abs(r-wr) <= tol && math.Abs(g-wg) <= tol && math.Abs(b-wb) <= tol
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

func evaluateGolden(snapDir string, rects []rect) (diffPct float64, totalPx int64, firstRun bool) {
	cur := filepath.Join(snapDir, "showcase_imagepattern.png")
	base := filepath.Join(snapDir, "showcase_imagepattern_base.png")
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
