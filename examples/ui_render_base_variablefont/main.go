// Command ui_render_base_variablefont is the B-group direct real window for
// variable-font loading and variation selection:
// Context.LoadFontFaceWithVariations, Context.FontVariationAxes,
// text.FontSource.Face + WithVariations, VariationAxes and NamedInstances.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_render_base_variablefont -auto-only
//	go run ./examples/ui_render_base_variablefont   # manual: lives until window close
//	go run ./examples/ui_render_base_variablefont -manual-seconds 15
//
// Evidence: logical contract probes on the public text API, point + region
// pixel probes on the GPU snapshot, and a bitwise golden over header + legend
// + the four cards. The visual sample is SourceSans3VF-Upright.otf, committed
// under testdata; the ink oracle comes from the pixels, not from Face.Advance.
package main

import (
	"crypto/sha256"
	"encoding/hex"
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
		wrkit.RequireMinRun(secs, "B-vf")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	if err := buildVFLayout(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: variable font:", err)
		os.Exit(1)
	}
	defer lay.source.Close()

	logicProbes, logicFailed := runVFLogic()
	logicMissing := missingProbeKeys(logicProbes)
	logicPass := len(logicFailed) == 0 && len(logicMissing) == 0
	for _, k := range logicProbes {
		fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: logic %-30s pass=%v\n", k, !contains(logicFailed, k))
	}
	if len(logicMissing) > 0 {
		fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: logic probes never ran: %v\n", logicMissing)
	}
	logicOK := len(vfProbeKeys) - len(logicFailed)

	pointSpecs := vfPointProbes()
	inkSpecs := vfInkProbes()
	fontSHA := sha16(lay.path)

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title:       "gpui ui_render_base_variablefont — VF · wght · named instances",
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "ui_render_base_variablefont — variable-font axes / instances", []string{
		"A  LoadFontFaceWithVariations",
		"B  NamedInstances + Face",
		"C  default == explicit default",
		"D  clamp bounds / unknown axis",
		fmt.Sprintf("probe: %d logic + %d point + %d ink", len(vfProbeKeys), len(pointSpecs), len(inkSpecs)),
		"golden: header + legend + 4 cards",
		"gate: -auto-only / RUN_SECONDS",
	})
	buildVFScene(shell)

	snapDir := filepath.Join("examples", "ui_render_base_variablefont", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_variablefont.png")

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
				fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_render_base_variablefont — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: resize %dx%d\n", ev.Width, ev.Height)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_variablefont — %dx%d resize #%d", ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			default:
				manualEvents++
				fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: event #%d type=%v\n", manualEvents, ev.Type)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_variablefont — event #%d type=%v", manualEvents, ev.Type))
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
		shell.UpdateHUD("B-vf", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("logic=%d/%d t=%.1f", logicOK, len(vfProbeKeys), elapsed))
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

	pointOK := 0
	for _, s := range pointSpecs {
		ca := cardAt[s.card]
		ax := bx + ca[0] + s.rect.x
		ay := by + ca[1] + s.rect.y
		px := int((ax + s.rect.w/2) * dpr)
		py := int((ay + s.rect.h/2) * dpr)
		r, g, b, valid := readPixel(img, px, py)
		ok := valid && nearC(r, g, b, s.want[0], s.want[1], s.want[2], s.tol)
		fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: pixel %-12s %-26s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			s.key, s.desc, px, py, r, g, b, s.want[0], s.want[1], s.want[2], ok)
		if ok {
			pointOK++
		}
	}

	inkOK := 0
	inkCounts := map[string]int{}
	scale := dpr * dpr
	for _, s := range inkSpecs {
		ca := cardAt[s.card]
		x0 := int((bx + ca[0] + s.rect.x) * dpr)
		y0 := int((by + ca[1] + s.rect.y) * dpr)
		x1 := int((bx + ca[0] + s.rect.x + s.rect.w) * dpr)
		y1 := int((by + ca[1] + s.rect.y + s.rect.h) * dpr)
		n, valid := countInk(img, x0, y0, x1, y1)
		inkCounts[s.key] = n
		ok := valid && n >= int(float64(s.min)*scale)
		if s.greaterThan != "" {
			prev, prevOK := inkCounts[s.greaterThan]
			ok = ok && prevOK && n > prev+int(float64(s.margin)*scale)
		}
		if s.equalTo != "" {
			prev, prevOK := inkCounts[s.equalTo]
			tol := int(float64(s.equalTol) * scale)
			ok = ok && prevOK && absInt(n-prev) <= tol
		}
		fmt.Fprintf(os.Stderr, "ui_render_base_variablefont: ink   %-12s %-30s n=%4d ok=%v\n", s.key, s.desc, n, ok)
		if ok {
			inkOK++
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "B-vf",
		Scenario:      "ui_render_base_variablefont",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"logic_probes_ok":        logicOK,
			"logic_probes_total":     len(vfProbeKeys),
			"logic_bad":              logicFailed,
			"logic_keys_missing":     logicMissing,
			"point_probes_ok":        pointOK,
			"point_probes_total":     len(pointSpecs),
			"ink_probes_ok":          inkOK,
			"ink_probes_total":       len(inkSpecs),
			"pixel_golden_diff_pct":  goldenDiffPct,
			"pixel_golden_total_px":  goldenTotalPx,
			"pixel_golden_first_run": goldenFirstRun,
			"font_path":              lay.path,
			"font_sha256_16":         fontSHA,
			"axis":                   fmt.Sprintf("%s %.0f..%.0f default %.0f", lay.axisTag, lay.axis.Minimum, lay.axis.Maximum, lay.axis.Default),
			"named_instances":        len(lay.instances),
			"apis":                   "LoadFontFaceWithVariations,FontVariationAxes,FontSource.IsVariable/VariationAxes/NamedInstances/Face,text.WithVariations,SetFont,DrawString",
			"fps_gate":               "off_correctness",
			"slope_gate":             "off",
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
	if !logicPass {
		fail("logic probes %d/%d bad=%v missing=%v", logicOK, len(vfProbeKeys), logicFailed, logicMissing)
	}
	if pointOK != len(pointSpecs) || len(pointSpecs) == 0 {
		fail("point probes %d/%d ok", pointOK, len(pointSpecs))
	}
	if inkOK != len(inkSpecs) || len(inkSpecs) == 0 {
		fail("ink probes %d/%d ok", inkOK, len(inkSpecs))
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
		"ui_render_base_variablefont: OK presents=%d logic=%d/%d point=%d/%d ink=%d/%d golden=%.4f%% resize=%d %dx%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), logicOK, len(vfProbeKeys), pointOK, len(pointSpecs), inkOK, len(inkSpecs),
		goldenDiffPct, resizeEvents, finalW, finalH,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

func missingProbeKeys(ran []string) []string {
	seen := map[string]bool{}
	for _, k := range ran {
		seen[k] = true
	}
	var bad []string
	for _, k := range vfProbeKeys {
		if !seen[k] {
			bad = append(bad, k)
		}
	}
	sort.Strings(bad)
	return bad
}

func contains(list []string, k string) bool {
	for _, s := range list {
		if s == k {
			return true
		}
	}
	return false
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

// countInk counts bright text pixels. Card backgrounds are dark (0.12–0.165),
// labels live to the left of every ink rect, and the sample text is 0.94+.
func countInk(img image.Image, x0, y0, x1, y1 int) (int, bool) {
	if img == nil || x0 < 0 || y0 < 0 || x1 > img.Bounds().Dx() || y1 > img.Bounds().Dy() || x1 <= x0 || y1 <= y0 {
		return 0, false
	}
	n := 0
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			r, g, b, valid := readPixel(img, px, py)
			if valid && r > 0.5 && g > 0.5 && b > 0.5 {
				n++
			}
		}
	}
	return n, true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sha16(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
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
	cur := filepath.Join(snapDir, "showcase_variablefont.png")
	base := filepath.Join(snapDir, "showcase_variablefont_base.png")
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
