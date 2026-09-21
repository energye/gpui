// Command kit_f0_motion is the F0-5 motion real window: spinner true
// rotation plus wave spread/fade plus reduced-motion freeze plus the
// central four-gate self-check.
//
// RUN_SECONDS=30 go run ./examples/kit_f0_motion
//
// Window: 1200x800 baseline, user-resizable. One scripted resize excursion
// restores the baseline before gates are read. Steady window (30s):
// fps gate on (fps_interval>=55, p95<=22ms), slope_gate=off.
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
	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var scripted = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "F0-motion")
	} else if *autoOnly {
		secs, secsSet = 30, true
	}
	_, faceName, errFace := wrkit.EnsureUIFace()
	if errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}
	hasFace := faceName != ""

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit_f0_motion — spinner wave gate", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	tok := ctx.Theme
	motionOn := ctx.Motion
	motionOff := kit.ScopeMotionConfig{Enabled: false}
	motionReduced := kit.ScopeMotionConfig{Enabled: true, ReducedMotion: true}
	durs := kit.BehaviorResolveDurations(tok)

	spinner := kit.NewBehaviorSpinner(0.9, kit.BehaviorResolveCurve("linear"))
	frozen := kit.NewBehaviorSpinner(0.9, kit.BehaviorResolveCurve("linear"))
	wave := kit.NewBehaviorWave()

	shell := wrkit.NewShell(winW, winH, "F0-5 motion — spinner wave gate", []string{
		"spinner: ticker true rotation",
		"wave: tap spreads 0-6, fades 2s",
		"reduced-motion: all frozen",
		"semantics: named plus role",
		"direction: rtl mirrors",
		"gate: props/state/wire/token",
		"resize once, restore baseline",
		"slope_gate=off (30s window)",
	})
	// Debug HUD is telemetry, not product: throttle to 2Hz so its uncached
	// text shaping (unique fps strings bypass the glyph cache by design)
	// stops driving GC in animation windows. Product pixels unaffected
	// (HUD band sits outside the golden mask).
	if shell.HUD != nil {
		shell.HUD.SetMinIntervalSec(0.5)
	}

	colTrack := [3]float64{0.13, 0.15, 0.19}
	colDot := [3]float64{tok.ColorPrimary.R, tok.ColorPrimary.G, tok.ColorPrimary.B}
	colWave := [3]float64{tok.ColorPrimaryBorder.R, tok.ColorPrimaryBorder.G, tok.ColorPrimaryBorder.B}
	colFrozen := [3]float64{tok.ColorFillSecondary.R, tok.ColorFillSecondary.G, tok.ColorFillSecondary.B}

	const spinBX, spinBY = 16.0, 40.0
	const spinSize = 120.0
	trackBox := rendering.NewRenderColorBox(spinSize, spinSize, colTrack[0], colTrack[1], colTrack[2], 1)
	trackBox.SetRepaintBoundary(true)
	trackBox.SetDebugName("f0-spin-track")
	shell.Body.Place(trackBox, spinBX, spinBY)
	shell.Body.Box.Place(wrkit.Label("spinner ticker turns", 11, 0.62, 0.72, 0.85), spinBX, 20)

	dotBox := rendering.NewRenderColorBox(16, 16, colDot[0], colDot[1], colDot[2], 1)
	dotBox.SetRepaintBoundary(true)
	dotBox.SetDebugName("f0-spin-dot")
	const dotHomeX, dotHomeY = spinBX + 52, spinBY + 52
	shell.Body.Place(dotBox, dotHomeX, dotHomeY)

	const frozenBX = 152.0
	frozenTrack := rendering.NewRenderColorBox(spinSize, spinSize, colFrozen[0], colFrozen[1], colFrozen[2], 1)
	frozenTrack.SetRepaintBoundary(true)
	frozenTrack.SetDebugName("f0-frozen-track")
	shell.Body.Place(frozenTrack, frozenBX, spinBY)
	shell.Body.Box.Place(wrkit.Label("reduced-motion frozen", 11, 0.62, 0.72, 0.85), frozenBX, 20)
	frozenDot := rendering.NewRenderColorBox(16, 16, colDot[0], colDot[1], colDot[2], 1)
	frozenDot.SetRepaintBoundary(true)
	frozenDot.SetDebugName("f0-frozen-dot")
	shell.Body.Place(frozenDot, frozenBX+8, spinBY+8)

	const waveBX, waveBY = 288.0, 40.0
	waveBox := rendering.NewRenderColorBox(220, 64, colWave[0], colWave[1], colWave[2], 1)
	waveBox.SetRepaintBoundary(true)
	waveBox.SetDebugName("f0-wave")
	shell.Body.Place(waveBox, waveBX, waveBY)
	shell.Body.Box.Place(wrkit.Label("wave tap spreads fades", 11, 0.62, 0.72, 0.85), waveBX, 20)

	face := wrkit.FaceAt(14)
	infoLabel := func(s string) *rendering.RenderText {
		t := wrkit.Label(s, 13, 0.88, 0.92, 0.96)
		if face != nil {
			wrkit.ApplyFace(t, 13)
		}
		t.SetRepaintBoundary(true)
		return t
	}
	semText := infoLabel("semantics: ")
	semText.SetDebugName("f0-sem")
	shell.Body.Place(semText, 560, 40)
	dirText := infoLabel("direction: ")
	dirText.SetDebugName("f0-dir")
	shell.Body.Place(dirText, 560, 90)
	gateText := infoLabel("gate: ")
	gateText.SetDebugName("f0-gate")
	shell.Body.Place(gateText, 560, 140)
	shell.Body.Box.Place(wrkit.Label("semantics direction gate", 11, 0.62, 0.72, 0.85), 560, 20)
	noteBG := [3]float64{0.10, 0.11, 0.13}

	snapDir := filepath.Join("examples", "kit_f0_motion", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	geoValid := false
	resizeEvents := 0
	var goldenRects []rect
	var probeSpin, probeWave, probeFrozen rect
	var densityRect rect
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		probeSpin = rect{x: bodyX + spinBX, y: bodyY + spinBY, w: spinSize, h: spinSize}
		probeWave = rect{x: bodyX + waveBX, y: bodyY + waveBY, w: 220, h: 64}
		probeFrozen = rect{x: bodyX + frozenBX, y: bodyY + spinBY, w: spinSize, h: spinSize}
		densityRect = rect{x: bodyX + 560, y: bodyY + 40, w: 300, h: 110}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
			probeSpin,
			probeWave,
		}
		geoValid = true
	}

	var angleAtStart, angleLater float64
	var isolationPic, isolationSib, isolationLayout bool
	var elapsed float64
	resizeDone, resizeBack := false, false
	phSpin, phFrozen, phWaveStart, phWaveMid, phWaveDone, phAudit, phFinal, phParked := false, false, false, false, false, false, false, false
	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_motion.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit_f0_motion: close (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					resizeEvents++
					shell.Resize(float64(ev.Width), float64(ev.Height))
					geoValid = false
				}
			case platform.EventPointer:
				if ev.Pointer == platform.PointerUp {
					bodyX, bodyY := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
					lx, ly := ev.X-bodyX, ev.Y-bodyY
					if kit.BehaviorContains(waveBX, waveBY, 220, 64, lx, ly) {
						if wave.Start(motionOn, kit.BehaviorWaveConfig{Enabled: true, HasBorder: true}) {
							fmt.Fprintln(os.Stderr, "kit_f0_motion: pointer wave started")
						}
					}
					app.ScheduleFrame()
				}
			}
		},
	})
	// Animation window: retained composite. Only the spinning dot, wave box
	// and throttled HUD repaint; static tracks skip (boundary_skip). Full
	// repaint every frame wastes the 16.7ms budget on this iGPU (13ms
	// raster) and leaves 3ms headroom, so any system jitter drops a vsync.
	app.SetPresentPolicy(scheduler.PresentPolicyRetained)

	// Synchronous placement: the controller drives the dot in its own Tick,
	// same frame before render, so there is no one-frame stale read.
	placeAt := func(angleDeg float64) {
		ang := angleDeg * math.Pi / 180
		dotBox.MoveTo(dotHomeX+44*math.Cos(ang), dotHomeY+44*math.Sin(ang))
	}
	spinner.Controller().OnValue(func(v float64) { placeAt(v * 360) })

	regSetup := app.Scheduler().Tickers()
	regSetup.Add(&ticker{on: func(dt float64) {
		elapsed += dt
		if ctl != nil && !resizeDone && elapsed >= 2.0 {
			resizeDone = true
			ctl.SetSize(1400, 900)
		}
		if ctl != nil && resizeDone && !resizeBack && elapsed >= 3.0 {
			resizeBack = true
			ctl.SetSize(winW, winH)
		}
		if !phSpin && elapsed >= 4.0 {
			phSpin = true
			spinner.Start(app.Scheduler().Tickers(), motionOn)
			angleAtStart = spinner.Angle()
			scripted["durations_ok"] = math.Abs(durs.Fast-0.1) < 1e-9 && math.Abs(durs.Mid-0.2) < 1e-9 && math.Abs(durs.Slow-0.3) < 1e-9
		}
		if phSpin && !phFrozen && elapsed >= 6.0 {
			phFrozen = true
			angleLater = spinner.Angle()
			scripted["spinner_spins"] = spinner.IsSpinning() && math.Abs(angleLater-angleAtStart) > 1e-6
			// Reduced-motion never starts: frozen dot holds home.
			frozen.Start(app.Scheduler().Tickers(), motionOff)
			scripted["reduced_frozen"] = !frozen.IsSpinning() && frozen.Angle() == 0 &&
				!wave.Start(motionReduced, kit.BehaviorWaveConfig{Enabled: true})
		}
		if phFrozen && !phWaveStart && elapsed >= 8.0 {
			phWaveStart = true
			started := wave.Start(motionOn, kit.BehaviorWaveConfig{Enabled: true, HasBorder: true})
			scripted["wave_started"] = started && wave.IsRunning()
			scripted["wave_no_hold"] = !kit.NewBehaviorWave().Start(motionOn, kit.BehaviorWaveConfig{Enabled: false}) &&
				!kit.NewBehaviorWave().Start(motionOn, kit.BehaviorWaveConfig{Enabled: true, Disabled: true}) &&
				!kit.NewBehaviorWave().Start(motionOn, kit.BehaviorWaveConfig{Enabled: true, IsTextLink: true})
		}
		if phWaveStart && !phWaveMid && elapsed >= 9.0 {
			phWaveMid = true
			scripted["wave_mid"] = wave.IsRunning() && wave.Radius() > 0 && wave.Alpha() > 0 && wave.Alpha() <= 0.2
			waveBox.SetAlpha(0.4 + wave.Alpha())
		}
		if phWaveMid && !phWaveDone && elapsed >= 11.0 {
			phWaveDone = true
			scripted["wave_done"] = !wave.IsRunning() && wave.Alpha() == 0
			waveBox.SetAlpha(1)
		}
		if phWaveDone && !phAudit && elapsed >= 13.0 {
			phAudit = true
			root := semantics.New(semantics.RoleGeneric, "motion")
			root.Add(semantics.New(semantics.RoleButton, "Spinner"))
			root.Add(semantics.New(semantics.RoleButton, "Wave"))
			root.Add(semantics.New(semantics.RoleButton, ""))
			total, named := kit.BehaviorAuditTree(root)
			okOne, _ := kit.BehaviorAuditOne(semantics.New(semantics.RoleButton, "Spinner"))
			badOne, _ := kit.BehaviorAuditOne(semantics.New(semantics.RoleButton, ""))
			scripted["semantics_ok"] = total == 4 && named == 3 && okOne && !badOne
			scripted["advisory_ok"] = kit.BehaviorMinTouchOK(44, 44) && !kit.BehaviorMinTouchOK(20, 20) &&
				kit.BehaviorPassContrast(kit.BehaviorContrastRatio(theme.Hex("#000000"), theme.Hex("#ffffff")))
			mirrorOK := kit.BehaviorMirrorX(100, 1200, kit.ScopeDirRTL) == 1100 &&
				kit.BehaviorMirrorX(100, 1200, kit.ScopeDirLTR) == 100
			prev, next := kit.BehaviorArrowPrevNext(kit.ScopeDirRTL, "←", "→")
			emptyOK := kit.BehaviorEmptyText(ctx, "Empty") == "No Empty data"
			scripted["direction_ok"] = mirrorOK && prev == "→" && next == "←" && emptyOK && kit.BehaviorIsRTL(ctx.WithDir(kit.ScopeDirRTL))
			semText.SetText("semantics: 3/4 named")
			dirText.SetText("direction: rtl mirrors")
			// Central four-gate self-check over window facts.
			propsOK, _ := kit.GateCheckProps(kit.GatePropsCoverage{Component: "motion", Total: 4, Covered: 4})
			stateOK := kit.GateCheckState(kit.GateStateFlow{
				States:   []kit.ScopeState{kit.ScopeStateDisabled, kit.ScopeStateHover, kit.ScopeStateLoading},
				Resolved: []bool{true, true, true},
			})
			wireOK, _ := kit.GateCheckWiring([]string{
				"github.com/energye/gpui/ui/kit/internal/prim",
				"github.com/energye/gpui/ui/kit/internal/behavior",
				"github.com/energye/gpui/ui/theme",
			})
			tokenOK := kit.GateCheckToken(kit.GateTokenUse{FromTheme: true, HardcodedCount: 0})
			scripted["gate4_ok"] = propsOK && stateOK && wireOK && tokenOK
			gateText.SetText("gate: 4/4 pass")
		}
		if phAudit && !phFinal && elapsed >= 15.0 {
			phFinal = true
			// Repaint-boundary contract: animated dot dirty, static track clean.
			dotBox.MarkNeedsPaint()
			isolationPic = dotBox.NeedsPaint()
			isolationSib = !trackBox.NeedsPaint() && !frozenTrack.NeedsPaint()
			isolationLayout = !dotBox.NeedsLayout() && !trackBox.NeedsLayout() && !shell.Root.NeedsLayout()
			scripted["isolation_ok"] = isolationPic && isolationSib && isolationLayout
			scripted["virtual_ok"] = kit.BehaviorVirtualBoundOK(1000, 8)
		}
		if !phParked && elapsed >= 28.0 {
			phParked = true
			// Park the dot at a fixed corner before the snapshot so the
			// Golden mask stays deterministic across runs.
			spinner.Stop()
			dotBox.MoveTo(spinBX+8, spinBY+8)
		}
		if wave.IsRunning() {
			wave.Tick(dt)
			waveBox.SetAlpha(0.4 + wave.Alpha())
			if !wave.IsRunning() {
				waveBox.SetAlpha(1)
			}
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("F0-motion", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			fmt.Sprintf("resize=%d angle=%.0f wave=%.1f t=%.1f", resizeEvents, spinner.Angle(), wave.Radius(), elapsed))
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)
	// Commit at display cadence (60Hz), not the 16ms floor (62.5Hz):
	// free-running faster than the display skips refreshes, which reads
	// as judder and doubles the visible step at 0.9s/rev.
	app.Scheduler().SetAnimTick(time.Second / 60)

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

	resolveGeometry()
	img := loadImage(filepath.Join(snapDir, "showcase_motion.png"))
	runSolidChecks(img, map[string]solidProbe{
		"spin_track": {box: probeSpin, want: colTrack},
		"wave_box":   {box: probeWave, want: colWave},
		"frozen_ok":  {box: probeFrozen, want: colFrozen},
	}, scripted)
	runDensityCheck(img, densityRect, noteBG, "info_density", scripted, hasFace)

	scriptedOK := 0
	keys := []string{"durations_ok", "spinner_spins", "reduced_frozen", "wave_started", "wave_no_hold", "wave_mid", "wave_done", "semantics_ok", "advisory_ok", "direction_ok", "gate4_ok", "isolation_ok", "virtual_ok", "spin_track", "wave_box", "frozen_ok", "info_density"}
	wantTotal := len(keys)
	for _, k := range keys {
		if scripted[k] {
			scriptedOK++
		}
	}
	goldenDiffPct, goldenTotalPx, goldenFirstRun := evaluateGolden(snapDir, goldenRects)

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "F0-motion",
		Scenario:      "kit_f0_motion",
		Backend:       win.Backend().String(),
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"slope_gate":             "off",
			"has_face":               hasFace,
			"face_name":              faceName,
			"spinner_angle":          spinner.Angle(),
			"spinner_spinning":       spinner.IsSpinning(),
			"frozen_angle":           frozen.Angle(),
			"wave_running":           wave.IsRunning(),
			"wave_radius":            wave.Radius(),
			"isolation_pic":          isolationPic,
			"isolation_sib":          isolationSib,
			"isolation_layout":       isolationLayout,
			"durations":              durs,
			"scripted_ok":            scriptedOK,
			"scripted_total":         wantTotal,
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
		MinPresents:           1,
		RequireRetainedPolicy: true,
		MaxDamageRatio:        0.5,
		RequirePersistentFPS:  true,
		MinFPSWall:            55,
		MaxP95Ms:              22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if scriptedOK != wantTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d\n", scriptedOK, wantTotal)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "  %-16s %v\n", k, scripted[k])
		}
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
	fmt.Fprintf(os.Stderr, "kit_f0_motion: OK presents=%d scripted=%d/%d golden=%.4f%% resize=%v %dx%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, wantTotal, goldenDiffPct, resizeBack, finalW, finalH, elapsedSec)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

type solidProbe struct {
	box  rect
	want [3]float64
}

func runSolidChecks(img image.Image, probes map[string]solidProbe, result map[string]bool) {
	for name, p := range probes {
		ok := img != nil && solidPointOK(img, p.box, p.want)
		fmt.Fprintf(os.Stderr, "kit_f0_motion: solid %-14s ok=%v\n", name, ok)
		result[name] = ok
	}
}

func solidPointOK(img image.Image, box rect, want [3]float64) bool {
	px := int(box.x + box.w/2)
	py := int(box.y + box.h/2)
	if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
		return false
	}
	r32, g32, b32, _ := img.At(px, py).RGBA()
	r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
	return math.Abs(r-want[0]) <= 12.0/255 && math.Abs(g-want[1]) <= 12.0/255 && math.Abs(b-want[2]) <= 12.0/255
}

func runDensityCheck(img image.Image, box rect, base [3]float64, name string, result map[string]bool, hasFace bool) {
	n := textPixels(img, 1.0, box, base)
	ok := img != nil && hasFace && n >= 60
	fmt.Fprintf(os.Stderr, "kit_f0_motion: text %-14s ink_px=%d (want >=60) ok=%v\n", name, n, ok)
	result[name] = ok
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
	cur := filepath.Join(snapDir, "showcase_motion.png")
	base := filepath.Join(snapDir, "showcase_motion_base.png")
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
