// Command ui_render_base_backdrop is the A-group real window for the backdrop
// layer family: PushBackdropLayer (+ ApplyBlur, PopLayer)
// (RENDER_FEATURE_COVERAGE §1 #16 / §3.1 item 7).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_render_base_backdrop -auto-only
//	go run ./examples/ui_render_base_backdrop   # manual: lives until window close
//	go run ./examples/ui_render_base_backdrop -manual-seconds 15  # manual, auto-close
//
// Evidence: 14 logical contract probes (value-level on an offscreen CPU
// context), 20 pixel probes on the GPU snapshot (oracle = palette constants +
// snapshot-dim math), a bitwise golden over header + legend + the four cards, a
// GPU-route assertion (gpu_ops > 0, cpu_fallback_ops == 0), and a live screen
// grab compared with the snapshot over the same rects.
//
// The screen check is not optional here: while closing this window the
// presented path rendered black (the layer RT's first pass cleared the uploaded
// backdrop seed) while the synthetic Close repaint stayed correct. Snapshot
// green alone would have been a false green; see README §4.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// windowTitle is the unique X11 WM_NAME used to resolve the live client
// geometry for the screen check (xwininfo -name).
const windowTitle = "gpui ui_render_base_backdrop — Backdrop 层 Push/Blur/Pop"

// Screen check tolerances: the grab and the snapshot are the same render path,
// so differences should be zero; ±2/255 absorbs compositor rounding. A black
// window or a missing card differs far above 1% of the compared pixels.
const (
	screenTol        = 2
	screenMaxDiffPct = 1.0
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var pixelResult = map[string]bool{}

// probeKeys are the logical contract probes that must all run and pass.
var probeKeys = []string{
	"backdrop_empty_is_identity",
	"backdrop_dims_content",
	"backdrop_outside_fill_untouched",
	"backdrop_pop_opacity_half",
	"backdrop_layer_child_stays_sharp",
	"backdrop_clip_keeps_outside",
	"backdrop_blur_softens_edge",
	"backdrop_blur_keeps_child_sharp",
	"backdrop_nested_sees_outer_layer",
	"backdrop_nested_band_outside_dim",
	"backdrop_sees_parent_draw_order",
	"backdrop_then_parent_draw_on_top",
	"backdrop_twice_composes",
	"backdrop_opacity_zero_noop",
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until window close)")
	logicOnly := flag.Bool("logic-only", false, "internal: run the value-level probes and exit (child of the window run)")
	screenCheck := flag.Bool("screencheck", true, "manual mode: grab the live window and compare it with the snapshot (gate mode always runs it)")
	flag.Parse()
	if *logicOnly {
		probes, failed := runLogicProbes()
		out, _ := json.Marshal(struct {
			Probes []string `json:"probes"`
			Failed []string `json:"failed"`
		}{probes, failed})
		fmt.Println(string(out))
		return
	}
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "A-backdrop")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	gateMode := secsSet
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	// Logical probes render offscreen; they run in a child of this same binary
	// with GOGPU_RENDER_MODE=cpu so their results always come from the software
	// contract. An in-process switch is not reliable once the GUI stack has
	// initialised (the accelerator is already live), and view-less GPU contexts
	// do not materialise pending ops into the pixmap. The window itself stays on
	// the caller's mode (GPU by default); its pixels are covered by the pixel
	// probes below.
	logicProbes, logicFailed, logicErr := runLogicProbesSubprocess()
	if logicErr != nil {
		fmt.Fprintln(os.Stderr, "logic probes:", logicErr)
		logicProbes, logicFailed = nil, nil
	}
	logicMissing := missingProbeKeys(logicProbes, logicFailed)
	logicPass := logicErr == nil && len(logicFailed) == 0 && len(logicMissing) == 0
	for _, k := range logicProbes {
		fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: logic %-38s pass=%v\n", k, !contains(logicFailed, k))
	}
	if len(logicMissing) > 0 {
		fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: logic probes never ran: %v\n", logicMissing)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title:       windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "ui_render_base_backdrop — backdrop layer", []string{
		"A 基面直达：压暗面板，面板外/层内后画 chip 原样",
		"B 隔离卡 Blur(6)：条纹抹平，Pop 后子内容锐利",
		"C 裁剪面板：Pop 整面贴回，面板外保留底图",
		"D 嵌套背板：内层快照外层隔离层内容",
		"probe: 14 值级契约 (CPU offscreen)",
		"pixel: 20 探针 (oracle + 通道带)",
		"golden: 表头+图例+四卡 逐位",
		"screen: 抓屏 vs 快照 同区对照",
		"gate: -auto-only / RUN_SECONDS",
	})
	buildBackdropScene(shell)

	snapDir := filepath.Join("examples", "ui_render_base_backdrop", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}
	snapPath := filepath.Join(snapDir, "showcase_backdrop.png")

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
				fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: close (%s)\n", win.Backend())
				if ctl != nil {
					ctl.SetTitle("gpui ui_render_base_backdrop — closing")
				}
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: resize %dx%d\n", ev.Width, ev.Height)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_backdrop — %dx%d resize #%d", ev.Width, ev.Height, resizeEvents))
					}
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			default:
				manualEvents++
				fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: event #%d type=%v\n", manualEvents, ev.Type)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("gpui ui_render_base_backdrop — event #%d type=%v", manualEvents, ev.Type))
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
		shell.UpdateHUD("A-backdrop", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
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
	// Screen check: the window still shows the last presented frame here, while
	// the snapshot is a synthetic repaint written at Close. Grab the live client
	// area now and compare it with the snapshot after Close — a correct snapshot
	// over a black window (the pre-fix backdrop path) must fail the gate.
	var screenGrabPath string
	var screenGrabErr error
	if gateMode || *screenCheck {
		screenGrabPath = filepath.Join(os.TempDir(), "gpui_backdrop_screen.png")
		if _, _, err := grabClientArea(windowTitle, screenGrabPath); err != nil {
			screenGrabErr = err
			screenGrabPath = ""
		}
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
	// color comes from the analytic oracle (palette constants + snapshot-dim
	// math), so a layout move can never silently misalign the expectations.
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
		fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: pixel %-16s %-52s @(%4d,%4d) got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f) ok=%v\n",
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

	// Screen check (same rects as the golden): the grab was taken while the
	// window was on screen; the snapshot is the synthetic Close repaint.
	screenRes := screenCheckResult{}
	if gateMode || *screenCheck {
		screenRes.Attempted = true
		screenRes.GrabPath = screenGrabPath
		if screenGrabErr != nil {
			screenRes.Detail = "grab failed: " + screenGrabErr.Error()
		} else if pct, n, cmpErr := compareScreenToSnapshot(screenGrabPath, snapPath, goldenRects, screenTol); cmpErr != nil {
			screenRes.Detail = cmpErr.Error()
		} else if pct > screenMaxDiffPct {
			screenRes.Detail = fmt.Sprintf("diff=%.4f%% over %d px > %.2f%% (grab vs snapshot)", pct, n, screenMaxDiffPct)
		} else {
			screenRes.OK = true
			screenRes.DiffPct = pct
			screenRes.Compared = n
			screenRes.Detail = fmt.Sprintf("diff=%.4f%% over %d px tol=±%d", pct, n, screenTol)
		}
		fmt.Fprintf(os.Stderr, "ui_render_base_backdrop: screen check %s (%s)\n", screenRes.Detail, screenRes.GrabPath)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "A-backdrop",
		Scenario:      "ui_render_base_backdrop",
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
			"apis":                   "PushBackdropLayer,ApplyBlur,PopLayer",
			"backdrop_gpu_route":     fmt.Sprintf("gpu_ops=%d cpu_fallback_ops=%d last=%q", snap.GPUOps, snap.CPUFallbackOps, snap.LastCPUFallbackReason),
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
			"screen_check_ok":        screenRes.OK,
			"screen_check_diff_pct":  screenRes.DiffPct,
			"screen_check_px":        screenRes.Compared,
			"screen_check_detail":    screenRes.Detail,
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
	if screenRes.Attempted && !screenRes.OK {
		fail("screen check: %s", screenRes.Detail)
	}
	// Backdrop layers must ride the GPU: a CPU fallback here would mean the
	// window validates a software path the accelerator never takes.
	if snap.GPUOps == 0 {
		fail("gpu_ops=0: backdrop draws did not ride the GPU")
	}
	if snap.CPUFallbackOps != 0 {
		fail("cpu_fallback_ops=%d last=%q", snap.CPUFallbackOps, snap.LastCPUFallbackReason)
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
		"ui_render_base_backdrop: OK presents=%d logic=%d/%d pixel=%d/%d golden=%.4f%% screen=%v resize=%d %dx%d mode=%s elapsed=%.1fs\n",
		app.PresentCount(), len(logicProbes)-len(logicFailed), len(logicProbes),
		pixelOK, len(pixelResult), goldenDiffPct, screenRes.OK, resizeEvents, finalW, finalH,
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

// runLogicProbesSubprocess runs the value-level probes in a child of this same
// binary with GOGPU_RENDER_MODE=cpu (see main for why). The child prints its
// probe keys as JSON on stdout; its stderr (per-probe failure details) is
// forwarded so a gate failure shows the sampled values.
func runLogicProbesSubprocess() (probes, failed []string, err error) {
	cmd := exec.Command(os.Args[0], "-logic-only")
	cmd.Env = append(os.Environ(), "GOGPU_RENDER_MODE=cpu")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if runErr := cmd.Run(); runErr != nil {
		return nil, nil, fmt.Errorf("child %v: %s", runErr, errb.String())
	}
	if errb.Len() > 0 {
		_, _ = os.Stderr.Write(errb.Bytes())
	}
	var res struct {
		Probes []string `json:"probes"`
		Failed []string `json:"failed"`
	}
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		return nil, nil, fmt.Errorf("child output: %v", err)
	}
	return res.Probes, res.Failed, nil
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
	cur := filepath.Join(snapDir, "showcase_backdrop.png")
	base := filepath.Join(snapDir, "showcase_backdrop_base.png")
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

// screenCheckResult is the live-window vs snapshot verdict. Attempted is false
// when -screencheck=false in manual mode; gate mode always runs it.
type screenCheckResult struct {
	Attempted bool
	OK        bool
	DiffPct   float64
	Compared  int
	GrabPath  string
	Detail    string
}

// grabClientArea captures the live X11 client area of the window with the given
// title. The snapshot is a synthetic repaint at Close (pipeline_app
// SnapshotPath); it can be correct while the presented window is black, so the
// real window must be grabbed while it is still on screen.
func grabClientArea(title, outPath string) (w, h int, err error) {
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		return 0, 0, fmt.Errorf("DISPLAY is empty")
	}
	info := exec.Command("xwininfo", "-name", title)
	info.Env = append(os.Environ(), "LC_ALL=C") // stable field names
	raw, err := info.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("xwininfo -name %q: %w", title, err)
	}
	var x, y, gotW, gotH int
	for _, ln := range strings.Split(string(raw), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(ln), ":")
		if !ok {
			continue
		}
		n, convErr := strconv.Atoi(strings.TrimSpace(v))
		if convErr != nil {
			continue
		}
		switch k {
		case "Absolute upper-left X":
			x = n
		case "Absolute upper-left Y":
			y = n
		case "Width":
			gotW = n
		case "Height":
			gotH = n
		}
	}
	if gotW <= 0 || gotH <= 0 {
		return 0, 0, fmt.Errorf("xwininfo: no geometry for %q", title)
	}
	// ffmpeg x11grab input: <display>+<x>,<y>, client size (no decorations).
	grab := exec.Command("ffmpeg", "-y", "-loglevel", "error",
		"-f", "x11grab", "-draw_mouse", "0",
		"-video_size", fmt.Sprintf("%dx%d", gotW, gotH),
		"-i", fmt.Sprintf("%s+%d,%d", disp, x, y),
		"-frames:v", "1", outPath)
	if out, grabErr := grab.CombinedOutput(); grabErr != nil {
		return 0, 0, fmt.Errorf("ffmpeg x11grab: %w: %s", grabErr, strings.TrimSpace(string(out)))
	}
	return gotW, gotH, nil
}

// compareScreenToSnapshot compares the grabbed client area against the snapshot
// over the golden rects (header + legend + cards). The live HUD is outside them:
// its counters change every frame, so it cannot be part of a screen assertion.
func compareScreenToSnapshot(grabPath, snapPath string, rects []rect, tol int) (diffPct float64, compared int, err error) {
	grab, snap := loadImage(grabPath), loadImage(snapPath)
	if grab == nil || snap == nil {
		return 0, 0, fmt.Errorf("decode failed (grab=%v snap=%v)", grab != nil, snap != nil)
	}
	if grab.Bounds() != snap.Bounds() {
		return 0, 0, fmt.Errorf("size mismatch grab=%v snap=%v (window resized?)", grab.Bounds(), snap.Bounds())
	}
	dpr := float64(snap.Bounds().Dx()) / winW
	var diff int
	for _, r := range rects {
		x0, y0 := int(r.x*dpr), int(r.y*dpr)
		x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
		for py := y0; py < y1 && py < snap.Bounds().Dy(); py++ {
			for px := x0; px < x1 && px < snap.Bounds().Dx(); px++ {
				gr, gg, gb, _ := grab.At(px, py).RGBA()
				sr, sg, sb, _ := snap.At(px, py).RGBA()
				compared++
				if channelDelta(gr, sr) > tol || channelDelta(gg, sg) > tol || channelDelta(gb, sb) > tol {
					diff++
				}
			}
		}
	}
	if compared == 0 {
		return 0, 0, fmt.Errorf("empty screen mask")
	}
	return float64(diff) * 100 / float64(compared), compared, nil
}

func channelDelta(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		d = -d
	}
	return d
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
