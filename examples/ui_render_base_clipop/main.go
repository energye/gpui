// Command ui_render_base_clipop is the A-group real window for clipping
// operations: ClipRectOp / ClipPathOp with ClipOpDifference and ClipOpReplace,
// plus ClipRect / ResetClip / SetFillRule propagation into clip paths
// (RENDER_FEATURE_COVERAGE §1 #13 / §3.1 item 5).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_render_base_clipop -auto-only
//	go run ./examples/ui_render_base_clipop   # manual: lives until window close
//	go run ./examples/ui_render_base_clipop -manual-seconds 15  # manual, auto-close
//
// Evidence: 14 logical contract probes (value-level analytic coverage on an
// offscreen CPU context), 23 pixel probes on the GPU snapshot (oracle =
// device-space analytic coverage blend over the card background, plus
// independent channel bands), and a bitwise golden over header + legend +
// the four cards.
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

// probeKeys are the logical contract probes that must all run and pass.
var probeKeys = []string{
	"rect_diff_hole_int",
	"rect_diff_frac_left25",
	"rect_diff_frac_right75",
	"rect_diff_frac_corner",
	"rect_diff_union_overlap",
	"rect_replace_discards",
	"path_diff_nonzero_nested",
	"path_diff_evenodd_nested",
	"path_replace_disjoint",
	"path_replace_open_triangle",
	"path_diff_disjoint_kept",
	"reset_clip_restores",
	"clip_rect_intersect",
	"rect_diff_from_no_base",
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "A-clipop")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Logical probes render offscreen; force the software rasterizer for the
	// probe phase so it never spins up a GPU device before the window session
	// opens. The window itself runs with the caller's mode (GPU by default).
	prevMode, hadMode := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	logicProbes, logicFailed := runLogicProbes()
	if hadMode {
		_ = os.Setenv("GOGPU_RENDER_MODE", prevMode)
	} else {
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
	}
	logicMissing := missingProbeKeys(logicProbes, logicFailed)
	logicPass := len(logicFailed) == 0 && len(logicMissing) == 0
	for _, k := range logicProbes {
		fmt.Fprintf(os.Stderr, "ui_render_base_clipop: logic %-28s pass=%v\n", k, !contains(logicFailed, k))
	}
	if len(logicMissing) > 0 {
		fmt.Fprintf(os.Stderr, "ui_render_base_clipop: logic probes never ran: %v\n", logicMissing)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title:       "gpui ui_render_base_clipop — 裁剪运算 difference·replace",
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "ui_render_base_clipop — clip ops", []string{
		"A ClipRectOp difference：整洞 + 分数洞(AA 边)",
		"B ClipRectOp replace：旧裁剪被丢弃",
		"C ClipPathOp difference × NonZero(左)/EvenOdd(右)",
		"D ClipPathOp replace：不相交子路径 + 开放三角形",
		"probe: 14 值级契约 (解析覆盖)",
		"pixel: 23 探针 (oracle + 通道带)",
		"golden: 表头+图例+四卡 逐位",
		"gate: -auto-only / RUN_SECONDS",
	})
	buildClipOpScene(shell)

	snapDir := filepath.Join("examples", "ui_render_base_clipop", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_clipop.png")

	resizeEvents := 0
	manualEvents := 0
	// Manual mode (no RUN_SECONDS, no -auto-only) lives until the window is
	// closed or -manual-seconds elapses; real events are logged and mirrored
	// into the title, and the summary + JSON print at the end (AGENTS.md
	// real-window manual pass).
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
				fmt.Fprintf(os.Stderr, "ui_render_base_clipop: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_render_base_clipop — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					fmt.Fprintf(os.Stderr, "ui_render_base_clipop: resize %dx%d\n", ev.Width, ev.Height)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_clipop — %dx%d resize #%d", ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			default:
				manualEvents++
				fmt.Fprintf(os.Stderr, "ui_render_base_clipop: event #%d type=%v\n", manualEvents, ev.Type)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_clipop — event #%d type=%v", manualEvents, ev.Type))
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
		shell.UpdateHUD("A-clipop", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("logic=%d/%d t=%.1f", len(logicProbes)-len(logicFailed), len(logicProbes), elapsed))
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

	// Probe geometry resolves against the live body origin, and every wanted
	// color comes from the analytic coverage oracle at the sampled device
	// pixel (card-local hole geometry resolved in device space), so a layout
	// move can never silently misalign the expectations.
	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	img := loadImage(snapPath)
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	pixelOK := 0
	for _, s := range probeSpecs {
		ca := cardAt[s.card]
		ox, oy := bx+ca[0]+contentX0, by+ca[1]+contentY0
		px, py := int((ox+s.pt.X)*dpr), int((oy+s.pt.Y)*dpr)
		wr, wg, wb := s.want(ox, oy, dpr, px, py)
		r, g, b, valid := readPixel(img, px, py)
		ok := valid && nearC(r, g, b, wr, wg, wb, tolSolid) && bandsOK(r, g, b, s.bands)
		fmt.Fprintf(os.Stderr, "ui_render_base_clipop: pixel %-16s %-52s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
			s.key, s.desc, px, py, r, g, b, wr, wg, wb, ok)
		pixelResult[s.key] = ok
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

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "A-clipop",
		Scenario:      "ui_render_base_clipop",
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
			"apis":                   "ClipRectOp(difference/replace),ClipPathOp(difference/replace),ClipRect,ResetClip,SetFillRule(FillRuleNonZero/EvenOdd)",
			"logic_pass":             logicPass,
			"logic_probes_ok":        len(logicProbes) - len(logicFailed),
			"logic_probes_total":     len(logicProbes),
			"logic_bad":              logicFailed,
			"logic_keys_missing":     logicMissing,
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
	if !logicPass {
		fail("logic probes %d/%d bad=%v missing=%v", len(logicProbes)-len(logicFailed), len(logicProbes), logicFailed, logicMissing)
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
		"ui_render_base_clipop: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% resize=%d %dx%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), len(logicProbes)-len(logicFailed), len(logicProbes),
		pixelOK, len(pixelResult), goldenDiffPct, resizeEvents, finalW, finalH,
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

// missingProbeKeys returns required keys that never ran (typo / dropped check).
func missingProbeKeys(ran, failed []string) []string {
	seen := map[string]bool{}
	for _, k := range ran {
		seen[k] = true
	}
	_ = failed
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

func bandsOK(r, g, b float64, bands []band) bool {
	v := [3]float64{r, g, b}
	for _, bd := range bands {
		if v[bd.ch] < bd.min || v[bd.ch] > bd.max {
			return false
		}
	}
	return true
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
	cur := filepath.Join(snapDir, "showcase_clipop.png")
	base := filepath.Join(snapDir, "showcase_clipop_base.png")
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
