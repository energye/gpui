// Command ui_render_base_scene_scenetext is the direct-drive real window for the __TITLE__ family
// (O12 batch 3, RENDER_FEATURE_COVERAGE §12 NONE).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/ui_render_base_scene_scenetext -auto-only
//	go run ./examples/ui_render_base_scene_scenetext  # manual: lives until close
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"

	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/ui/platform"
)

const (
	abilityID   = "A-scenescenetext"
	minRunLabel = "A-scenescenetext"
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	manualSecs := flag.Int("manual-seconds", 0, "manual mode timeout (0 = live until window close)")
	screenCheck := flag.Bool("screencheck", true, "grab the live window and compare with the snapshot")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, minRunLabel)
	} else if *autoOnly {
		secs, secsSet = 8, true
	}
	gateMode := secsSet
	if _, _, err := wrkit.EnsureUIFace(); err != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", err)
	}
	ensureFaces()

	app, err := wrkit.OpenDirect(winTitle, int(wrkit.DWinW), int(wrkit.DWinH))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}

	snapDir := filepath.Join("examples", "ui_render_base_scene_scenetext", "testdata")
	wrkit.EnsureSnapDir(snapDir)
	snapPath := filepath.Join(snapDir, "showcasescene_scenetext.png")

	logicOK := 0
	var logicBad []string
	check := func(nm string, ok bool, detail string) {
		if ok {
			logicOK++
			fmt.Fprintf(os.Stderr, "%s: OK %s\n", nm, detail)
		} else {
			logicBad = append(logicBad, nm+" "+detail)
			fmt.Fprintf(os.Stderr, "%s: FAIL %s\n", nm, detail)
		}
	}

	if err := app.PresentFull(func(dc *render.Context) { paintShowcase(dc, nil) }); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: P1 full present:", err)
		app.Close()
		os.Exit(1)
	}
	check("P1.full", app.LastOutcome.Mode == render.PresentModeFull,
		fmt.Sprintf("mode=%v", app.LastOutcome.Mode))

	var leg *stLeg
	app.RunGL(func() { leg = runSTLegs() })
	check("P2.offscreen", leg != nil && len(leg.logicBad) == 0,
		fmt.Sprintf("logic=%d bad=%v", legLogicCount(leg), leg.logicBad))

	if err := app.PresentFull(func(dc *render.Context) {
		paintShowcase(dc, leg)
		paintStatusLines(dc, leg)
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: P3 final full:", err)
		app.Close()
		os.Exit(1)
	}
	backend := app.GPUBackendName()

	grabFinal := ""
	if *screenCheck {
		gp := filepath.Join(os.TempDir(), "gpuiscene_scenetext_final.png")
		if _, _, gerr := app.GrabLiveWindow(gp); gerr != nil {
			fmt.Fprintf(os.Stderr, "grab skipped (%v)\n", gerr)
		} else {
			grabFinal = gp
		}
	}

	var deadline time.Time
	if gateMode {
		deadline = time.Now().Add(time.Duration(secs) * time.Second)
	} else if *manualSecs > 0 {
		deadline = time.Now().Add(time.Duration(*manualSecs) * time.Second)
	}
	for !app.QuitRequested() {
		timeout := 500 * time.Millisecond
		if !deadline.IsZero() {
			left := time.Until(deadline)
			if left <= 0 {
				break
			}
			if left < timeout {
				timeout = left
			}
		}
		for _, ev := range app.WaitEvents(timeout) {
			switch ev.Type {
			case platform.EventCloseRequested, platform.EventClose:
				fmt.Fprintf(os.Stderr, "%s: close (%s)\n", abilityID, app.Win.Backend())
				if app.Ctl != nil {
					app.Ctl.SetTitle(winTitle + " — closing")
				}
				app.Quit()
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					fmt.Fprintf(os.Stderr, "%s: resize %dx%d\n", abilityID, ev.Width, ev.Height)
					_ = app.Target.Resize(ev.Width, ev.Height, app.Host.ScaleFactor())
				}
			}
		}
	}

	if err := app.SnapshotRepaint(func(dc *render.Context) {
		paintShowcase(dc, leg)
		paintStatusLines(dc, leg)
	}, snapPath); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snapshot:", err)
		app.Close()
		os.Exit(1)
	}
	if err := app.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "WARN: close:", err)
	}

	probes := []wrkit.PixelProbe{}
	pixelErr := wrkit.CheckPixelProbes(snapPath, wrkit.DWinW, probes)

	goldenDiff, goldenPx, goldenSeeded, goldenErr := wrkit.GoldenGate(
		snapPath, filepath.Join(snapDir, "goldenscene_scenetext.png"), wrkit.DirectGoldenRects())

	screenDiff := -1.0
	if grabFinal != "" {
		screenDiff = wrkit.GrabDiffPct(grabFinal, snapPath, wrkit.DirectGoldenRects())
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     abilityID,
		Scenario:      "ui_render_base_scene_scenetext",
		Backend:       backend,
		PresentCount:  int64(app.Presents),
		ElapsedSec:    time.Since(startTime).Seconds(),
		SurfaceAreaPx: int64(wrkit.DWinW * wrkit.DWinH),
		Warmup:        true,
		Extra: map[string]any{
			"fps_gate":              "off_correctness",
			"slope_gate":            "off",
			"apis":                  "NewTextShape,DefaultTextRendererConfig,TextRendererConfig,TextRendererPool,RenderedGlyph,GlyphRunData,GlyphEntry,TextFlags(TextFlagCJK,TextFlagHinting),scene TextRenderer shape/render face,AffineFromMatrix,SolidBrush",
			"adapter_class":         backend,
			"logic_ok":              logicOK,
			"logic_bad":             logicBad,
			"offscreen_logic_ok":    legLogicCount(leg),
			"offscreen_logic_bad":   leg.logicBad,
			"offscreen_detail":      leg.dLines,
			"findings":              leg.findings,
			"presents_total":        app.Presents,
			"present_errs":          app.PresentErrs,
			"pixel_probes_total":    len(probes),
			"pixel_probes_err":      errString(pixelErr),
			"pixel_golden_diff_pct": goldenDiff,
			"pixel_golden_total_px": goldenPx,
			"pixel_golden_seeded":   goldenSeeded,
			"pixel_golden_err":      errString(goldenErr),
			"screen_grab_diff_pct":  screenDiff,
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
	if len(logicBad) > 0 {
		fail("logic probes: %v", logicBad)
	}
	if pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if goldenErr != nil && !goldenSeeded {
		fail("golden: %v", goldenErr)
	}
	if !goldenSeeded && goldenDiff != 0 {
		fail("pixel_golden_diff_pct=%.4f want 0 over %d px", goldenDiff, goldenPx)
	}
	if screenDiff >= 0 && screenDiff > 1.0 {
		fail("screen vs snapshot diff %.4f%% > 1%%", screenDiff)
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "%s: OK presents=%d logic=%d+%d pixel=%d golden=%.4f%% screen=%.4f%% backend=%s\n",
		abilityID, app.Presents, logicOK, legLogicCount(leg), len(probes), goldenDiff, screenDiff, backend)
}

var startTime = time.Now()

func paintStatusLines(dc *render.Context, leg *stLeg) {
	extra := ""
	if leg != nil && len(leg.logicBad) > 0 {
		extra = fmt.Sprintf(" bad=%v", leg.logicBad)
	}
	lines := []string{
		fmt.Sprintf("probe: logic_ok=%d%s · 状态行在黄金图区外（动态计数不入金）", legLogicCount(leg), extra),
		"manual: 常驻人工窗 — 关窗出汇总; gate: RUN_SECONDS 自退",
	}
	wrkit.PaintDirectStatus(dc, lines, faceSmall)
}

func legLogicCount(l *stLeg) int {
	if l == nil {
		return 0
	}
	return l.logicOK
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
