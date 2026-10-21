// Command ui_render_base_emoji is the B-group direct real window for
// render/text/emoji: CBDT/CBLC bitmap extraction, Segment run splitting
// (ZWJ / flag / skin tone / keycap / VS16), and COLR layer compositing.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_render_base_emoji -auto-only
//	go run ./examples/ui_render_base_emoji   # manual: lives until window close
//	go run ./examples/ui_render_base_emoji -manual-seconds 15  # manual, auto-close
//
// Evidence: 14 logical contract probes, 12 pixel probes on the GPU snapshot
// (8 are font-independent: segmentation boxes + synthetic COLR), and a bitwise
// golden over header + legend + the four cards. A/B cards need a CBDT color
// font; when absent those probes are reported as skipped, never silently green.
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

var pixelResult = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "B-emoji")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	emojiFace12 = wrkit.FaceAt(12)
	emojiFace18 = wrkit.FaceAt(18)

	// Assets + corpus load before the shell so the logic probes, the paint
	// callbacks and the pixel probe geometry all read one set of values.
	emojiAssets = loadEmojiAssets()
	if emojiAssets.err != nil {
		fmt.Fprintf(os.Stderr, "ui_render_base_emoji: color font SKIP: %v\n", emojiAssets.err)
	}
	segLines, err := loadSegLines()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: testdata:", err)
		os.Exit(1)
	}
	emojiSegLines = segLines
	emojiCOLR = buildCOLR()
	if emojiCOLR == nil {
		fmt.Fprintln(os.Stderr, "ui_render_base_emoji: COLR synthetic render failed")
	}

	logicProbes, logicFailed, logicSkipped := runLogicProbes()
	logicMissing := missingProbeKeys(logicProbes, logicSkipped)
	logicPass := len(logicFailed) == 0 && len(logicMissing) == 0
	for _, k := range logicProbes {
		fmt.Fprintf(os.Stderr, "ui_render_base_emoji: logic %-24s pass=%v\n", k, !contains(logicFailed, k))
	}
	for _, k := range logicSkipped {
		fmt.Fprintf(os.Stderr, "ui_render_base_emoji: logic %-24s SKIP (no CBDT color font)\n", k)
	}
	if len(logicMissing) > 0 {
		fmt.Fprintf(os.Stderr, "ui_render_base_emoji: logic probes never ran: %v\n", logicMissing)
	}
	logicOK := len(probeKeys) - len(logicFailed) - len(logicSkipped)

	probeSpecs := buildPixProbes()
	var pixelSkipped []string
	if emojiAssets == nil || emojiAssets.err != nil {
		pixelSkipped = []string{"A_grid_ink", "A_grid_bg", "B_sample_ink", "B_sample_bg"}
	}
	fontBase := "missing"
	if emojiAssets != nil && emojiAssets.fontPath != "" {
		fontBase = filepath.Base(emojiAssets.fontPath)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title:       "gpui ui_render_base_emoji — CBDT · COLR · segmentation",
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "ui_render_base_emoji — color emoji extraction + segmentation", []string{
		"A  6×6 CBDT bitmap grid",
		"B  one glyph at natural size",
		"C  Segment: ZWJ/flag/tone/keycap",
		"D  synthetic COLR: red / green / blue",
		fmt.Sprintf("probe: %d logic + %d pixel", len(probeKeys), len(probeSpecs)),
		"golden: header + legend + 4 cards",
		"font: " + fontBase,
		"gate: -auto-only / RUN_SECONDS",
	})
	buildEmojiScene(shell)

	snapDir := filepath.Join("examples", "ui_render_base_emoji", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_emoji.png")

	resizeEvents := 0
	manualEvents := 0
	// Manual mode lives until the window is closed or -manual-seconds elapses.
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
				fmt.Fprintf(os.Stderr, "ui_render_base_emoji: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_render_base_emoji — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					fmt.Fprintf(os.Stderr, "ui_render_base_emoji: resize %dx%d\n", ev.Width, ev.Height)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_emoji — %dx%d resize #%d", ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			default:
				manualEvents++
				fmt.Fprintf(os.Stderr, "ui_render_base_emoji: event #%d type=%v\n", manualEvents, ev.Type)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_emoji — event #%d type=%v", manualEvents, ev.Type))
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
		shell.UpdateHUD("B-emoji", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("logic=%d/%d t=%.1f", logicOK, len(probeKeys), elapsed))
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
		if s.kind == "ink" {
			x0, y0 := int(ax*dpr), int(ay*dpr)
			x1, y1 := int((ax+s.rect.w)*dpr), int((ay+s.rect.h)*dpr)
			ink, chroma, total := scanRect(img, x0, y0, x1, y1)
			frac := 0.0
			if total > 0 {
				frac = float64(ink) / float64(total)
			}
			ok := total > 0 && frac >= s.minInk && chroma >= s.minChroma
			fmt.Fprintf(os.Stderr, "ui_render_base_emoji: pixel %-14s %-28s ink=%.3f chroma=%d total=%d ok=%v\n",
				s.key, s.desc, frac, chroma, total, ok)
			pixelResult[s.key] = ok
			if ok {
				pixelOK++
			}
			continue
		}
		px := int((ax + s.rect.w/2) * dpr)
		py := int((ay + s.rect.h/2) * dpr)
		r, g, b, valid := readPixel(img, px, py)
		ok := valid && nearC(r, g, b, s.want[0], s.want[1], s.want[2], s.tol)
		fmt.Fprintf(os.Stderr, "ui_render_base_emoji: pixel %-14s %-28s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			s.key, s.desc, px, py, r, g, b, s.want[0], s.want[1], s.want[2], ok)
		pixelResult[s.key] = ok
		if ok {
			pixelOK++
		}
	}

	// The golden locks the deterministic chrome, the segmentation boxes and
	// the synthetic COLR image. A/B cards need the system color font; without
	// it the whole golden is skipped (and reported), never silently passed.
	goldenSkipped := emojiAssets == nil || emojiAssets.err != nil
	var goldenDiffPct float64
	var goldenTotalPx int64
	var goldenFirstRun bool
	if !goldenSkipped {
		goldenRects := []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{x: shell.Legend.X, y: shell.Legend.Y, w: shell.Legend.W, h: shell.Legend.H},
		}
		for _, ca := range cardAt {
			goldenRects = append(goldenRects, rect{x: bx + ca[0], y: by + ca[1], w: cardW, h: cardH})
		}
		goldenDiffPct, goldenTotalPx, goldenFirstRun = evaluateGolden(snapDir, goldenRects)
	} else {
		fmt.Fprintln(os.Stderr, "ui_render_base_emoji: golden SKIP (no CBDT color font)")
	}

	assetsExtra := map[string]any{
		"font_path":              "",
		"font_sha256_16":         "",
		"font_missing":           true,
		"cbdt_ppem":              0,
		"cbdt_glyphs":            0,
		"cbdt_sample_gid":        0,
		"colr_layers":            0,
		"seg_lines":              len(emojiSegLines),
		"seg_emoji_runs":         countEmojiRuns(emojiSegLines),
		"logic_probes_ok":        logicOK,
		"logic_probes_total":     len(probeKeys),
		"logic_bad":              logicFailed,
		"logic_skipped":          logicSkipped,
		"logic_keys_missing":     logicMissing,
		"pixel_probes_ok":        pixelOK,
		"pixel_probes_total":     len(probeSpecs) + len(pixelSkipped),
		"pixel_skipped":          pixelSkipped,
		"pixel_golden_diff_pct":  goldenDiffPct,
		"pixel_golden_total_px":  goldenTotalPx,
		"pixel_golden_first_run": goldenFirstRun,
		"pixel_golden_skipped":   goldenSkipped,
	}
	if emojiAssets != nil {
		assetsExtra["font_path"] = emojiAssets.fontPath
		assetsExtra["font_sha256_16"] = emojiAssets.fontSHA
		assetsExtra["font_missing"] = emojiAssets.err != nil
		assetsExtra["cbdt_ppem"] = int(emojiAssets.ppem)
		assetsExtra["cbdt_glyphs"] = len(emojiAssets.cells)
		if emojiAssets.sample != nil {
			assetsExtra["cbdt_sample_gid"] = int(emojiAssets.sample.gid)
		}
	}
	if emojiCOLR != nil {
		assetsExtra["colr_layers"] = len(emojiCOLR.layers)
	}
	assetsExtra["apis"] = "emoji.Segment,IsEmojiVariation,IsTextPresentation,IsValidSequence,GetFlagCode,CBDTExtractor,COLRGlyph,RenderCOLRToImage"
	assetsExtra["fps_gate"] = "off_correctness"
	assetsExtra["slope_gate"] = "off"
	assetsExtra["resize_events"] = resizeEvents
	assetsExtra["client_px"] = fmt.Sprintf("%dx%d", finalW, finalH)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "B-emoji",
		Scenario:      "ui_render_base_emoji",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra:         assetsExtra,
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
		fail("logic probes %d/%d bad=%v missing=%v skipped=%v", logicOK, len(probeKeys), logicFailed, logicMissing, logicSkipped)
	}
	if pixelOK != len(probeSpecs) || len(probeSpecs) == 0 {
		fail("pixel probes %d/%d ok (skipped=%v)", pixelOK, len(probeSpecs), pixelSkipped)
	}
	if !goldenSkipped && !goldenFirstRun && goldenDiffPct != 0 {
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
		"ui_render_base_emoji: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% skip=%d resize=%d %dx%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), logicOK, len(probeKeys), pixelOK, len(probeSpecs),
		goldenDiffPct, len(logicSkipped)+len(pixelSkipped)+(map[bool]int{true: 1, false: 0}[goldenSkipped]),
		resizeEvents, finalW, finalH,
		map[bool]string{true: "gate", false: "manual"}[gateMode], elapsedSec)
}

func countEmojiRuns(lines []segLine) int {
	n := 0
	for _, l := range lines {
		for _, r := range l.runs {
			if r.IsEmoji {
				n++
			}
		}
	}
	return n
}

func contains(list []string, k string) bool {
	for _, s := range list {
		if s == k {
			return true
		}
	}
	return false
}

// missingProbeKeys returns required keys that neither ran nor were skipped.
func missingProbeKeys(ran, skipped []string) []string {
	seen := map[string]bool{}
	for _, k := range ran {
		seen[k] = true
	}
	for _, k := range skipped {
		seen[k] = true
	}
	var bad []string
	for _, k := range probeKeys {
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

// scanRect counts pixels that differ from the card background (ink) and
// pixels with a visible chroma spread. It is used for the font bitmaps, where
// the exact glyph pixels are font-version-dependent; the golden still locks
// the layout and the synthetic COLR image.
func scanRect(img image.Image, x0, y0, x1, y1 int) (ink, chroma, total int) {
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			r, g, b, ok := readPixel(img, px, py)
			if !ok {
				continue
			}
			total++
			d := math.Max(math.Abs(r-colCardBG[0]), math.Max(math.Abs(g-colCardBG[1]), math.Abs(b-colCardBG[2])))
			if d > 0.05 {
				ink++
			}
			if math.Max(r, math.Max(g, b))-math.Min(r, math.Min(g, b)) > 0.10 {
				chroma++
			}
		}
	}
	return ink, chroma, total
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
	cur := filepath.Join(snapDir, "showcase_emoji.png")
	base := filepath.Join(snapDir, "showcase_emoji_base.png")
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
